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

var rdbCancel = redis.NewClient(&redis.Options{Addr: getRedisAddrCancel()})

func getRedisAddrCancel() string {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return "localhost:6379"
	}
	return addr
}

func randomSide() string {
	if rand.Intn(2) == 0 {
		return "buy"
	}
	return "sell"
}

func publishLatencyCancel(latencyMs float64) {
	ctx := context.Background()
	rdbCancel.XAdd(ctx, &redis.XAddArgs{
		Stream: "telemetry:orders",
		Values: map[string]interface{}{
			"latency_ms": strconv.FormatFloat(latencyMs, 'f', 3, 64),
			"timestamp":  time.Now().UnixNano(),
			"submission_id": os.Getenv("SUBMISSION_ID"),
		},
	})
}

func RunCancelBot(botID int, targetURL string, count int) {
	client := &http.Client{Timeout: 5 * time.Second}

	for i := 0; i < count; i++ {
		// Place order first
		payload := map[string]interface{}{
			"side":     randomSide(),
			"type":     "limit",
			"price":    100.0,
			"quantity": rand.Intn(20) + 1,
		}
		body, _ := json.Marshal(payload)
		resp, err := client.Post(targetURL+"/order", "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Printf("[CancelBot-%d] place ERROR: %v", botID, err)
			continue
		}

		var ack map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&ack)
		resp.Body.Close()

		orderID, ok := ack["order_id"].(string)
		if !ok {
			continue
		}

		// Cancel and measure
		start := time.Now()
		req, _ := http.NewRequest(http.MethodDelete,
			fmt.Sprintf("%s/order?order_id=%s", targetURL, orderID), nil)
		resp2, err := client.Do(req)
		latency := time.Since(start).Seconds() * 1000

		if err != nil {
			log.Printf("[CancelBot-%d] cancel ERROR: %v", botID, err)
			continue
		}
		resp2.Body.Close()

		publishLatencyCancel(latency)
		fmt.Printf("[CancelBot-%d] cancel %d → %s in %.2fms\n", botID, i+1, resp2.Status, latency)
	}
}