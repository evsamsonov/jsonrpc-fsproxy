package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/evsamsonov/jsonrpc-fsproxy/pkg/jsonrpc"
	"go.uber.org/zap"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: jsonrpc-fsproxy [INPUT_FILE_PATH] [OUTPUT_FILE_PATH] [RPC_URL]")
		os.Exit(1)
	}

	inputFilePath, outputFilePath, rpcURL := os.Args[1], os.Args[2], os.Args[3]

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
