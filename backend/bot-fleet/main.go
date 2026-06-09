package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/iicpc/bot-fleet/bots"
	"github.com/redis/go-redis/v9"
)

func main() {
	targetURL := os.Getenv("TARGET_URL")
	if targetURL == "" {
		targetURL = "http://localhost:8080" // default for local testing
	}

	numBots := 500
	ordersPerBot := 100

	if v := os.Getenv("NUM_BOTS"); v != "" {
		numBots, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("ORDERS_PER_BOT"); v != "" {
		ordersPerBot, _ = strconv.Atoi(v)
	}

	log.Printf("Launching %d bots → %s", numBots, targetURL)

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numBots; i++ {
		wg.Add(1)
		go func(botID int) {
			defer wg.Done()
			switch botID % 3 {
			case 0:
				bots.RunMarketOrderBot(botID, targetURL, ordersPerBot)
			case 1:
				bots.RunLimitOrderBot(botID, targetURL, ordersPerBot)
			case 2:
				bots.RunCancelBot(botID, targetURL, ordersPerBot)
			}
		}(i)
	}

	wg.Wait()

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	sid := os.Getenv("SUBMISSION_ID")
	rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "telemetry:orders",
		Values: map[string]interface{}{
			"submission_id": sid,
			"event":         "done",
			"latency_ms":    "0",
			"team_name":     os.Getenv("TEAM_NAME"),
		},
	})
	log.Printf("[BOT-FLEET] done signal sent for submission %s", sid)

	elapsed := time.Since(start)
	totalOrders := numBots * ordersPerBot
	log.Printf("Done! %d orders in %s → %.0f orders/sec",
		totalOrders, elapsed, float64(totalOrders)/elapsed.Seconds())
}
