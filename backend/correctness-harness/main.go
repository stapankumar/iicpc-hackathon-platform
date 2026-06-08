package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// OrderResponse is what we expect the contestant's orderbook to return
type OrderResponse struct {
	OrderID   string  `json:"order_id"`
	Status    string  `json:"status"`
	FillPrice float64 `json:"fill_price"`
	FilledQty int     `json:"filled_qty"`
}

var (
	targetURL    = mustEnv("TARGET_URL")
	submissionID = mustEnv("SUBMISSION_ID")
	redisAddr    = envOr("REDIS_ADDR", "localhost:6379")
	client       = &http.Client{Timeout: 5 * time.Second}
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("[HARNESS] %s not set", key)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// postOrder sends a POST /order and returns the parsed response + raw status code
func postOrder(side, orderType string, price float64, qty int) (OrderResponse, int, error) {
	payload := map[string]interface{}{
		"side":     side,
		"type":     orderType,
		"price":    price,
		"quantity": qty,
	}
	body, _ := json.Marshal(payload)
	resp, err := client.Post(targetURL+"/order", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return OrderResponse{}, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var ack OrderResponse
	json.Unmarshal(raw, &ack)
	return ack, resp.StatusCode, nil
}

// cancelOrder sends DELETE /order?order_id=X and returns status code
func cancelOrder(orderID string) (int, error) {
	req, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/order?order_id=%s", targetURL, orderID), nil)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

// getOrderbook calls GET /orderbook
func getOrderbook() (map[string]interface{}, error) {
	resp, err := client.Get(targetURL + "/orderbook")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

// --- Scenarios ---
// Each returns true if passed, false if failed.
// They are sequential — each waits for the previous to complete.
// A fresh sandbox is assumed at start. We reset between scenarios
// by design: each scenario uses unique prices far apart so
// leftover resting orders from prior scenarios do not interfere.

// Scenario 1: Basic price cross — buy at 105 should fill against resting sell at 100
func scenario1() bool {
	log.Println("[HARNESS] Scenario 1: Basic price cross")

	// Place resting sell at 100
	sell, code, err := postOrder("sell", "limit", 100.0, 10)
	if err != nil || code != 200 || sell.OrderID == "" {
		log.Printf("[HARNESS] S1 FAIL: sell placement failed code=%d err=%v", code, err)
		return false
	}

	// Buy at 105 — must cross and fill against the sell at 100
	buy, code, err := postOrder("buy", "limit", 105.0, 10)
	if err != nil || code != 200 || buy.OrderID == "" {
		log.Printf("[HARNESS] S1 FAIL: buy placement failed code=%d err=%v", code, err)
		return false
	}

	// Fill price must be <= buy limit (105) and >= sell limit (100)
	if buy.FilledQty != 10 {
		log.Printf("[HARNESS] S1 FAIL: expected filled_qty=10 got=%d", buy.FilledQty)
		return false
	}
	if buy.FillPrice < 100.0 || buy.FillPrice > 105.0 {
		log.Printf("[HARNESS] S1 FAIL: fill_price=%.2f out of range [100, 105]", buy.FillPrice)
		return false
	}

	log.Println("[HARNESS] S1 PASS")
	return true
}

// Scenario 2: No cross — buy at 90 should NOT fill against sell at 95
func scenario2() bool {
	log.Println("[HARNESS] Scenario 2: No cross — orders should rest in book")

	buy, code, err := postOrder("buy", "limit", 90.0, 5)
	if err != nil || code != 200 || buy.OrderID == "" {
		log.Printf("[HARNESS] S2 FAIL: buy placement failed code=%d err=%v", code, err)
		return false
	}

	sell, code, err := postOrder("sell", "limit", 95.0, 5)
	if err != nil || code != 200 || sell.OrderID == "" {
		log.Printf("[HARNESS] S2 FAIL: sell placement failed code=%d err=%v", code, err)
		return false
	}

	// Neither should be filled
	if buy.FilledQty != 0 {
		log.Printf("[HARNESS] S2 FAIL: buy should not fill, got filled_qty=%d", buy.FilledQty)
		return false
	}
	if sell.FilledQty != 0 {
		log.Printf("[HARNESS] S2 FAIL: sell should not fill, got filled_qty=%d", sell.FilledQty)
		return false
	}

	log.Println("[HARNESS] S2 PASS")
	return true
}

// Scenario 3: Cancel prevents fill
func scenario3() bool {
	log.Println("[HARNESS] Scenario 3: Cancel prevents fill")

	// Place buy at 200 (high price, will definitely cross any sell)
	buy, code, err := postOrder("buy", "limit", 200.0, 10)
	if err != nil || code != 200 || buy.OrderID == "" {
		log.Printf("[HARNESS] S3 FAIL: buy placement failed code=%d err=%v", code, err)
		return false
	}

	// Cancel it
	cancelCode, err := cancelOrder(buy.OrderID)
	if err != nil || cancelCode != 200 {
		log.Printf("[HARNESS] S3 FAIL: cancel failed code=%d err=%v", cancelCode, err)
		return false
	}

	// Now place a sell that would have matched — should NOT fill
	sell, code, err := postOrder("sell", "limit", 180.0, 10)
	if err != nil || code != 200 || sell.OrderID == "" {
		log.Printf("[HARNESS] S3 FAIL: sell placement failed code=%d err=%v", code, err)
		return false
	}

	if sell.FilledQty != 0 {
		log.Printf("[HARNESS] S3 FAIL: sell matched cancelled order, filled_qty=%d", sell.FilledQty)
		return false
	}

	log.Println("[HARNESS] S3 PASS")
	return true
}

// Scenario 4: Partial fill
func scenario4() bool {
	log.Println("[HARNESS] Scenario 4: Partial fill")

	// Resting sell of qty 3
	sell, code, err := postOrder("sell", "limit", 150.0, 3)
	if err != nil || code != 200 || sell.OrderID == "" {
		log.Printf("[HARNESS] S4 FAIL: sell placement failed code=%d err=%v", code, err)
		return false
	}

	// Buy qty 10 — should partially fill 3, rest 7
	buy, code, err := postOrder("buy", "limit", 155.0, 10)
	if err != nil || code != 200 || buy.OrderID == "" {
		log.Printf("[HARNESS] S4 FAIL: buy placement failed code=%d err=%v", code, err)
		return false
	}

	if buy.FilledQty != 3 {
		log.Printf("[HARNESS] S4 FAIL: expected partial fill of 3, got=%d", buy.FilledQty)
		return false
	}

	log.Println("[HARNESS] S4 PASS")
	return true
}

// Scenario 5: Market order fills immediately against resting liquidity
func scenario5() bool {
	log.Println("[HARNESS] Scenario 5: Market order fills immediately")

	// Place resting sell at 160
	sell, code, err := postOrder("sell", "limit", 160.0, 10)
	if err != nil || code != 200 || sell.OrderID == "" {
		log.Printf("[HARNESS] S5 FAIL: sell placement failed code=%d err=%v", code, err)
		return false
	}

	// Market buy — should fill immediately against the resting sell
	buy, code, err := postOrder("buy", "market", 0, 5)
	if err != nil || code != 200 || buy.OrderID == "" {
		log.Printf("[HARNESS] S5 FAIL: market buy failed code=%d err=%v", code, err)
		return false
	}

	if buy.FilledQty != 5 {
		log.Printf("[HARNESS] S5 FAIL: market order should fill 5, got=%d", buy.FilledQty)
		return false
	}

	log.Println("[HARNESS] S5 PASS")
	return true
}

// Scenario 6: Orderbook endpoint returns valid structure
func scenario6() bool {
	log.Println("[HARNESS] Scenario 6: GET /orderbook returns valid structure")

	book, err := getOrderbook()
	if err != nil {
		log.Printf("[HARNESS] S6 FAIL: GET /orderbook error: %v", err)
		return false
	}

	// Must have bids and asks keys
	_, hasBids := book["bids"]
	_, hasAsks := book["asks"]
	if !hasBids || !hasAsks {
		log.Printf("[HARNESS] S6 FAIL: /orderbook missing bids/asks keys, got: %v", book)
		return false
	}

	log.Println("[HARNESS] S6 PASS")
	return true
}

func main() {
	log.Printf("[HARNESS] starting correctness harness for submission %s → %s", submissionID, targetURL)

	// Wait for sandbox to be accepting requests
	log.Println("[HARNESS] waiting for sandbox to be ready...")
	for i := 0; i < 30; i++ {
		resp, err := client.Get(targetURL + "/orderbook")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			log.Println("[HARNESS] sandbox ready")
			break
		}
		if i == 29 {
			log.Fatalf("[HARNESS] sandbox did not become ready after 30 attempts")
		}
		time.Sleep(2 * time.Second)
	}

	scenarios := []func() bool{
		scenario1,
		scenario2,
		scenario3,
		scenario4,
		scenario5,
		scenario6,
	}

	passed := 0
	total := len(scenarios)

	for i, s := range scenarios {
		// Small gap between scenarios so resting orders from prior
		// scenario don't bleed into the next one.
		// Each scenario uses distinct price ranges so this is a
		// safety net, not a requirement.
		if i > 0 {
			time.Sleep(500 * time.Millisecond)
		}
		if s() {
			passed++
		}
	}

	correctness := float64(passed) / float64(total)
	log.Printf("[HARNESS] result: %d/%d scenarios passed → correctness=%.2f", passed, total, correctness)

	// Publish correctness score to Redis so telemetry can read it during FinalizeSubmission
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	ctx := context.Background()
	key := "correctness:" + submissionID
	err := rdb.Set(ctx, key, strconv.FormatFloat(correctness, 'f', 4, 64), 2*time.Hour).Err()
	if err != nil {
		log.Fatalf("[HARNESS] failed to publish correctness to Redis: %v", err)
	}

	log.Printf("[HARNESS] correctness=%.4f published to Redis key=%s", correctness, key)
}
