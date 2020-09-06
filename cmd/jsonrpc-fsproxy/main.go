package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/evsamsonov/json-rpc-file-proxy/pkg/jsonrpc"
	"go.uber.org/zap"
)

var (
	defaultInputFilePath  = "dev/rpcin"
	defaultOutputFilePath = "dev/rpcout"
)

func main() {
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		log.Fatal("Environment var RPC_URL is empty")
	}
	inputFilePath := os.Getenv("INPUT_FILE_PATH")
	if inputFilePath == "" {
		inputFilePath = defaultInputFilePath
	}
	outputFilePath := os.Getenv("OUTPUT_FILE_PATH")
	if outputFilePath == "" {
		outputFilePath = defaultOutputFilePath
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	proxy, err := jsonrpc.NewFSProxy(
		rpcURL,
		inputFilePath,
		outputFilePath,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to create proxy", zap.Error(err))
	}
	defer func() {
		if err := proxy.Close(); err != nil {
			logger.Warn("Failed to close proxy", zap.Error(err))
		}
	}()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := proxy.Run(ctx); err != nil {
			logger.Fatal("Failed to run proxy", zap.Error(err))
		}
	}()

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()
	wg.Wait()
}
