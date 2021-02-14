package jsonrpc

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

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

type FSProxy struct {
	inputFilePath   string
	inputFile       *os.File
	outputFilePath  string
	outputFile      *os.File
	outputFileMutex sync.Mutex
	logger          *zap.Logger
	rpcURL          string
	errorStream     chan error
	watcher         *fsnotify.Watcher
}

func NewFSProxy(
	rpcURL string,
	inputFilePath string,
	outputFilePath string,
	logger *zap.Logger,
) (*FSProxy, error) {
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

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("new watcher: %w", err)
	}
	err = watcher.Add(inputFilePath)
	if err != nil {
		return nil, fmt.Errorf("watcher add: %w", err)
	}

	return &FSProxy{
		rpcURL:         rpcURL,
		inputFile:      inputFile,
		inputFilePath:  inputFilePath,
		outputFile:     outputFile,
		outputFilePath: outputFilePath,
		logger:         logger,
		errorStream:    make(chan error),
		watcher:        watcher,
	}, nil
}

func (w *FSProxy) Run(ctx context.Context) error {
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

func (w *FSProxy) Close() error {
	err := w.inputFile.Close()
	if err != nil {
		return fmt.Errorf("close input file: %w", err)
	}
	err = w.outputFile.Close()
	if err != nil {
		return fmt.Errorf("close output file: %w", err)
	}
	err = w.watcher.Close()
	if err != nil {
		return fmt.Errorf("close watcher: %w", err)
	}
	return nil
}

func (w *FSProxy) watchInput(ctx context.Context, wg *sync.WaitGroup) <-chan string {
	lineStream := make(chan string)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(lineStream)

		// Skip old lines
		_, err := w.inputFile.Seek(0, io.SeekEnd)
		if err != nil {
			w.errorStream <- fmt.Errorf("seek input: %w", err)
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					// Wait until the lock is free
					for {
						if _, err := os.Stat(w.inputFilePath + ".lock"); os.IsNotExist(err) {
							break
						}
						<-time.After(100 * time.Millisecond)
					}
					scanner := bufio.NewScanner(w.inputFile)
					for scanner.Scan() {
						line := scanner.Text()
						lineStream <- line
						w.logger.Info("Got new line", zap.String("line", line))
					}
				}
			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				w.errorStream <- fmt.Errorf("watcher errors: %w", err)
				return
			}
		}
	}()
	return lineStream
}

func (w *FSProxy) processLines(ctx context.Context, wg *sync.WaitGroup, lineStream <-chan string) {
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
				go func() {
					defer wg.Done()
					w.processLine(line)
				}()
			}
		}
	}()
}

func (w *FSProxy) processLine(line string) {
	resp, err := http.Post(w.rpcURL, "Content-Type: application/json", strings.NewReader(line))
	if err != nil {
		w.logger.Error("Failed to send request", zap.Error(err))
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			w.logger.Warn("Failed to Close response body", zap.Error(err))
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
