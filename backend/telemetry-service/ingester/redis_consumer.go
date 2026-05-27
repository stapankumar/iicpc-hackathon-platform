package ingester

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/iicpc/telemetry-service/metrics"
	"github.com/redis/go-redis/v9"
)

const streamKey = "telemetry:orders"

func Consume(ctx context.Context, store *metrics.MetricStore) {
	rdb := redis.NewClient(&redis.Options{
		Addr: getRedisAddr(),
	})
	defer rdb.Close()

	rdb.XGroupCreateMkStream(ctx, streamKey, "telemetry-group", "$")

	log.Println("Listening on Redis Stream:", streamKey)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    "telemetry-group",
				Consumer: "telemetry-consumer-1",
				Streams:  []string{streamKey, ">"},
				Count:    100,
				Block:    500,
			}).Result()

			if err != nil {
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					latencyStr, ok := msg.Values["latency_ms"].(string)
					if !ok {
						continue
					}
					latency, err := strconv.ParseFloat(latencyStr, 64)
					if err != nil {
						continue
					}

					subID, _ := msg.Values["submission_id"].(string)
					if subID == "" {
						subID = "unknown"
					}
					store.Record(subID, latency)
					rdb.XAck(ctx, streamKey, "telemetry-group", msg.ID)
				}
			}

			store.PrintStats()
		}
	}
}

func getRedisAddr() string {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return "localhost:6379"
	}
	return addr
}
