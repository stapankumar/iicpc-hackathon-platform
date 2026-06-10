package handlers

import (
	"encoding/json"
	"net/http"
)

func HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"platform": "IICPC Distributed Benchmarking Platform",
		"status":   "running",
		"submit":   "http://iicpc.local/submit",
		"scores":   "http://iicpc.local/scores",
		"frontend": "http://localhost:5173",
	})
}
