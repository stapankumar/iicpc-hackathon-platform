package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/iicpc/telemetry-service/ingester"
	"github.com/iicpc/telemetry-service/metrics"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("Telemetry service started")
	log.Println("Consuming from Redis Streams...")

	store := metrics.NewMetricStore()

	// Graceful shutdown — flush scores before exit
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("Shutting down — flushing final scores...")
		store.Flush()
		cancel()
	}()

	ingester.Consume(ctx, store)
}
