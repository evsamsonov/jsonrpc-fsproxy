package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/evsamsonov/json-rpc-file-proxy/pkg/jsonrpcfile"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	proxy, err := jsonrpcfile.NewProxy(
		"http://127.0.0.1:8080/rpc",
		"request.pipe",
		"response.pipe",
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
			logger.Fatal("Crash proxy", zap.Error(err))
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
