package models

// Order represents a trading order submitted by a bot
type Order struct {
	ID        string  `json:"order_id"`
	Side      string  `json:"side"`      // "buy" or "sell"
	Type      string  `json:"type"`      // "limit" or "market"
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Timestamp int64   `json:"timestamp"`
}

// OrderAck is the response from contestant's exchange
type OrderAck struct {
	OrderID   string `json:"order_id"`
	Status    string `json:"status"`     // "ACK" or "REJECT"
	FilledQty int    `json:"filled_qty"`
	Timestamp int64  `json:"timestamp"`
}

// Metric is a single telemetry data point
type Metric struct {
	SubmissionID string  `json:"submission_id"`
	LatencyMs    float64 `json:"latency_ms"`
	Timestamp    int64   `json:"timestamp"`
}