package bots

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var rdb = redis.NewClient(&redis.Options{Addr: getRedisAddrMarket()})

func getRedisAddrMarket() string {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return "localhost:6379"
	}
	return addr
}

func publishLatency(latencyMs float64) {
	ctx := context.Background()
	rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "telemetry:orders",
		Values: map[string]interface{}{
			"latency_ms": strconv.FormatFloat(latencyMs, 'f', 3, 64),
			"timestamp":  time.Now().UnixNano(),
			"submission_id": os.Getenv("SUBMISSION_ID"),
		},
	})
}

func RunMarketOrderBot(botID int, targetURL string, count int) {
	client := &http.Client{Timeout: 5 * time.Second}

	for i := 0; i < count; i++ {
		payload := map[string]interface{}{
			"side":     randomSide(),
			"type":     "market",
			"price":    0,
			"quantity": rand.Intn(50) + 1,
		}

		body, _ := json.Marshal(payload)
		start := time.Now()

		resp, err := client.Post(targetURL+"/order", "application/json", bytes.NewBuffer(body))
		latency := time.Since(start).Seconds() * 1000

		if err != nil {
			log.Printf("[MarketBot-%d] ERROR: %v", botID, err)
			continue
		}
		resp.Body.Close()

		publishLatency(latency)
		fmt.Printf("[MarketBot-%d] order %d → %s in %.2fms\n", botID, i+1, resp.Status, latency)
	}
}