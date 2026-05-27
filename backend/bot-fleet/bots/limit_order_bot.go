package bots

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var rdbLimit = redis.NewClient(&redis.Options{Addr: getRedisAddrLimit()})

func getRedisAddrLimit() string {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return "localhost:6379"
	}
	return addr
}

func publishLatencyLimit(latencyMs float64) {
	ctx := context.Background()
	rdbLimit.XAdd(ctx, &redis.XAddArgs{
		Stream: "telemetry:orders",
		Values: map[string]interface{}{
			"latency_ms": strconv.FormatFloat(latencyMs, 'f', 3, 64),
			"timestamp":  time.Now().UnixNano(),
			"submission_id": os.Getenv("SUBMISSION_ID"),
		},
	})
}

func RunLimitOrderBot(botID int, targetURL string, count int) {
	client := &http.Client{Timeout: 5 * time.Second}

	for i := 0; i < count; i++ {
		price := 95.0 + rand.Float64()*10
		payload := map[string]interface{}{
			"side":     randomSide(),
			"type":     "limit",
			"price":    price,
			"quantity": rand.Intn(100) + 1,
		}

		body, _ := json.Marshal(payload)
		start := time.Now()

		resp, err := client.Post(targetURL+"/order", "application/json", bytes.NewBuffer(body))
		latency := time.Since(start).Seconds() * 1000

		if err != nil {
			log.Printf("[LimitBot-%d] ERROR: %v", botID, err)
			continue
		}
		resp.Body.Close()

		publishLatencyLimit(latency)
		fmt.Printf("[LimitBot-%d] order %d → %s in %.2fms\n", botID, i+1, resp.Status, latency)
	}
}