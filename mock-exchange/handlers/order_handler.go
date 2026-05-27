package handlers

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// --- Data Models ---

type Order struct {
	ID        string  `json:"order_id"`
	Side      string  `json:"side"`       // "buy" or "sell"
	Type      string  `json:"type"`       // "limit" or "market"
	Price     float64 `json:"price"`      // 0 for market orders
	Quantity  int     `json:"quantity"`
	Timestamp int64   `json:"timestamp"`
}

type OrderAck struct {
	OrderID   string `json:"order_id"`
	Status    string `json:"status"`    // "ACK" or "REJECT"
	FilledQty int    `json:"filled_qty"`
	Timestamp int64  `json:"timestamp"`
}

type OrderbookSnapshot struct {
	Bids []PriceLevel `json:"bids"` // buy side, descending price
	Asks []PriceLevel `json:"asks"` // sell side, ascending price
}

type PriceLevel struct {
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

// --- In-memory store ---

var (
	mu     sync.RWMutex
	orders = make(map[string]Order)
)

// --- Handlers ---

func HandleOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {

	case http.MethodPost:
		// Place a new order
		var o Order
		if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
			http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
			return
		}

		o.ID = uuid.NewString()
		o.Timestamp = time.Now().UnixNano()

		// Simulate processing latency (real exchange would match here)
		time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)

		mu.Lock()
		orders[o.ID] = o
		mu.Unlock()

		log.Printf("[ORDER] %s %s %.2f x%d → ACK %s", o.Side, o.Type, o.Price, o.Quantity, o.ID)

		ack := OrderAck{
			OrderID:   o.ID,
			Status:    "ACK",
			FilledQty: rand.Intn(o.Quantity + 1), // simulate partial fill
			Timestamp: time.Now().UnixNano(),
		}
		json.NewEncoder(w).Encode(ack)

	case http.MethodDelete:
		// Cancel an existing order
		orderID := r.URL.Query().Get("order_id")
		mu.Lock()
		delete(orders, orderID)
		mu.Unlock()

		log.Printf("[CANCEL] order %s cancelled", orderID)
		json.NewEncoder(w).Encode(map[string]string{
			"order_id": orderID,
			"status":   "CANCELLED",
		})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func HandleOrderbook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Build a fake snapshot from current orders
	mu.RLock()
	defer mu.RUnlock()

	snapshot := OrderbookSnapshot{
		Bids: []PriceLevel{{Price: 99.5, Quantity: 100}, {Price: 99.0, Quantity: 200}},
		Asks: []PriceLevel{{Price: 100.5, Quantity: 150}, {Price: 101.0, Quantity: 300}},
	}

	json.NewEncoder(w).Encode(snapshot)
}