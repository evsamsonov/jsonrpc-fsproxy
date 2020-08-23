package jsonrpcfile

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Proxy struct {
	inputFilePath   string
	inputFile       *os.File
	watchTimeout    time.Duration
	outputFilePath  string
	outputFile      *os.File
	outputFileMutex sync.Mutex
	logger          *zap.Logger
	rpcUrl          string
	errorStream     chan error
}

func NewProxy(
	rpcUrl string,
	inputFilePath string,
	outputFilePath string,
	watchTimeout time.Duration,
	logger *zap.Logger,
) (*Proxy, error) {
	var inputFile *os.File
	if _, err := os.Stat(inputFilePath); os.IsNotExist(err) {
		if inputFile, err = os.Create(inputFilePath); err != nil {
			return nil, fmt.Errorf("create input file: %w", err)
		}
	} else {
		if inputFile, err = os.Open(inputFilePath); err != nil {
			return nil, fmt.Errorf("open input file: %w", err)
		}
	}

	var outputFile *os.File
	if _, err := os.Stat(outputFilePath); os.IsNotExist(err) {
		if outputFile, err = os.Create(outputFilePath); err != nil {
			return nil, fmt.Errorf("create output file: %w", err)
		}
	} else {
		if outputFile, err = os.OpenFile(outputFilePath, os.O_APPEND|os.O_WRONLY, os.ModeAppend); err != nil {
			return nil, fmt.Errorf("open output file: %w", err)
		}
	}

	return &Proxy{
		rpcUrl:         rpcUrl,
		inputFile:      inputFile,
		inputFilePath:  inputFilePath,
		outputFile:     outputFile,
		outputFilePath: outputFilePath,
		watchTimeout:   watchTimeout,
		logger:         logger,
		errorStream:    make(chan error),
	}, nil
}

func (w *Proxy) Run(ctx context.Context) error {
	defer w.closeFiles()

	var wg sync.WaitGroup
	lineStream := w.watchInput(ctx, &wg)
	w.processLines(ctx, &wg, lineStream)

	waitStream := make(chan struct{})
	go func() {
		wg.Wait()
		waitStream <- struct{}{}
	}()

	select {
	case <-waitStream:
		return nil
	case err := <-w.errorStream:
		return err
	}
}

func (w *Proxy) watchInput(ctx context.Context, wg *sync.WaitGroup) <-chan string {
	lineStream := make(chan string)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(lineStream)
		var size int64
		for {
			stat, err := os.Stat(w.inputFilePath)
			if err != nil {
				w.errorStream <- fmt.Errorf("os stat input: %w", err)
				return
			}
			if size == stat.Size() {
				select {
				case <-ctx.Done():
					return
				case <-time.After(w.watchTimeout):
					continue
				}
			}
			if size == 0 {
				// Skip old lines
				_, err = w.inputFile.Seek(stat.Size(), io.SeekStart)
				if err != nil {
					w.errorStream <- fmt.Errorf("seek input: %w", err)
					return
				}
			}
			size = stat.Size()

			scanner := bufio.NewScanner(w.inputFile)
			for scanner.Scan() {
				line := scanner.Text()
				lineStream <- line
				w.logger.Info("Sent new line", zap.String("line", line))
			}
		}
	}()
	return lineStream
}

func (w *Proxy) processLines(ctx context.Context, wg *sync.WaitGroup, lineStream <-chan string) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-lineStream:
				if !ok {
					return
				}
				wg.Add(1)
				func() {
					defer wg.Done()
					w.processLine(line)
				}()
			}
		}
	}()
}

func (w *Proxy) processLine(line string) {
	resp, err := http.Post(w.rpcUrl, "Content-Type: application/json", strings.NewReader(line))
	if err != nil {
		w.logger.Error("Failed to send request", zap.Error(err))
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			w.logger.Warn("Failed to close response body", zap.Error(err))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		w.logger.Error("Response status code not OK", zap.Error(err))
		return
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		w.logger.Error("Failed to read response body", zap.Error(err))
		return
	}
	w.logger.Info("Got response", zap.ByteString("response", bodyBytes))

	w.outputFileMutex.Lock()
	defer w.outputFileMutex.Unlock()
	if _, err := w.outputFile.Write(bodyBytes); err != nil {
		w.logger.Error("Failed to write response", zap.Error(err))
		return
	}
}

func (w *Proxy) closeFiles() {
	if err := w.inputFile.Close(); err != nil {
		w.logger.Error("Failed to close input file", zap.Error(err))
	}
	if err := w.outputFile.Close(); err != nil {
		w.logger.Error("Failed to close output file", zap.Error(err))
	}
}
