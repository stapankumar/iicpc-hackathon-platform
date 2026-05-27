package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID        string  `json:"order_id"`
	Side      string  `json:"side"`
	Type      string  `json:"type"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	FilledQty int     `json:"filled_qty"`
	Timestamp int64   `json:"timestamp"`
}

type OrderBook struct {
	mu     sync.Mutex
	bids   []*Order // sorted high to low
	asks   []*Order // sorted low to high
	orders map[string]*Order
}

var book = &OrderBook{orders: make(map[string]*Order)}

func main() {
	http.HandleFunc("/order", handleOrder)
	http.HandleFunc("/orderbook", handleOrderBook)
	log.Println("Orderbook running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodPost:
		placeOrder(w, r)
	case http.MethodDelete:
		cancelOrder(w, r)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func placeOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Side     string  `json:"side"`
		Type     string  `json:"type"`
		Price    float64 `json:"price"`
		Quantity int     `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	o := &Order{
		ID:        uuid.NewString(),
		Side:      req.Side,
		Type:      req.Type,
		Price:     req.Price,
		Quantity:  req.Quantity,
		Timestamp: time.Now().UnixNano(),
	}

	book.mu.Lock()
	filled := book.match(o)
	o.FilledQty = filled
	if o.Quantity > 0 && o.Type == "limit" {
		book.insert(o)
	}
	book.orders[o.ID] = o
	book.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"order_id":   o.ID,
		"status":     "ACK",
		"filled_qty": filled,
		"timestamp":  o.Timestamp,
	})
}

func cancelOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	book.mu.Lock()
	o, exists := book.orders[orderID]
	if exists {
		book.remove(o)
		delete(book.orders, orderID)
	}
	book.mu.Unlock()

	if !exists {
		http.Error(w, `{"error":"order not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{
		"order_id": orderID,
		"status":   "CANCELLED",
	})
}

func handleOrderBook(w http.ResponseWriter, r *http.Request) {
	book.mu.Lock()
	defer book.mu.Unlock()

	type Level struct {
		Price    float64 `json:"price"`
		Quantity int     `json:"quantity"`
	}

	bids := []Level{}
	asks := []Level{}
	for _, o := range book.bids {
		bids = append(bids, Level{o.Price, o.Quantity})
	}
	for _, o := range book.asks {
		asks = append(asks, Level{o.Price, o.Quantity})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bids": bids,
		"asks": asks,
	})
}

// match returns filled quantity — price-time priority
func (b *OrderBook) match(o *Order) int {
	filled := 0
	if o.Side == "buy" {
		for len(b.asks) > 0 && o.Quantity > 0 {
			best := b.asks[0]
			if o.Type == "limit" && o.Price < best.Price {
				break
			}
			qty := min(o.Quantity, best.Quantity)
			o.Quantity -= qty
			best.Quantity -= qty
			filled += qty
			if best.Quantity == 0 {
				b.asks = b.asks[1:]
			}
		}
	} else {
		for len(b.bids) > 0 && o.Quantity > 0 {
			best := b.bids[0]
			if o.Type == "limit" && o.Price > best.Price {
				break
			}
			qty := min(o.Quantity, best.Quantity)
			o.Quantity -= qty
			best.Quantity -= qty
			filled += qty
			if best.Quantity == 0 {
				b.bids = b.bids[1:]
			}
		}
	}
	return filled
}

func (b *OrderBook) insert(o *Order) {
	if o.Side == "buy" {
		inserted := false
		for i, existing := range b.bids {
			if o.Price > existing.Price || (o.Price == existing.Price && o.Timestamp < existing.Timestamp) {
				b.bids = append(b.bids[:i], append([]*Order{o}, b.bids[i:]...)...)
				inserted = true
				break
			}
		}
		if !inserted {
			b.bids = append(b.bids, o)
		}
	} else {
		inserted := false
		for i, existing := range b.asks {
			if o.Price < existing.Price || (o.Price == existing.Price && o.Timestamp < existing.Timestamp) {
				b.asks = append(b.asks[:i], append([]*Order{o}, b.asks[i:]...)...)
				inserted = true
				break
			}
		}
		if !inserted {
			b.asks = append(b.asks, o)
		}
	}
}

func (b *OrderBook) remove(o *Order) {
	if o.Side == "buy" {
		for i, existing := range b.bids {
			if existing.ID == o.ID {
				b.bids = append(b.bids[:i], b.bids[i+1:]...)
				return
			}
		}
	} else {
		for i, existing := range b.asks {
			if existing.ID == o.ID {
				b.asks = append(b.asks[:i], b.asks[i+1:]...)
				return
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
