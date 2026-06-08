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

	log.Println("[TELEMETRY] listening on Redis Stream:", streamKey)

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

					subID, _ := msg.Values["submission_id"].(string)
					if subID == "" {
						subID = "unknown"
					}

					// Done signal from bot-fleet — compute and push final score
					if event, _ := msg.Values["event"].(string); event == "done" {
						log.Printf("[TELEMETRY] done signal received for %s", subID)
						rdb.XAck(ctx, streamKey, "telemetry-group", msg.ID)
						store.FinalizeSubmission(subID)
						continue
					}

					// Normal latency record
					latencyStr, ok := msg.Values["latency_ms"].(string)
					if !ok {
						rdb.XAck(ctx, streamKey, "telemetry-group", msg.ID)
						continue
					}
					latency, err := strconv.ParseFloat(latencyStr, 64)
					if err != nil {
						rdb.XAck(ctx, streamKey, "telemetry-group", msg.ID)
						continue
					}

					store.Record(subID, latency)
					rdb.XAck(ctx, streamKey, "telemetry-group", msg.ID)
				}
			}
			// NOTE: PrintStats removed — score is computed once on done signal only
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
