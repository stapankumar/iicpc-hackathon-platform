package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/iicpc/leaderboard-service/db"
)

func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE (Server-Sent Events)
	// Simpler than WebSocket — no extra library needed
	// Browser connects and receives live score updates
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	log.Println("[SSE] client connected to leaderboard stream")

	// Push scores every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			log.Println("[SSE] client disconnected")
			return
		case <-ticker.C:
			scores, err := db.GetTopScores(50)
			if err != nil {
				continue
			}
			data, _ := json.Marshal(scores)
			// SSE format
			w.Write([]byte("data: " + string(data) + "\n\n"))
			flusher.Flush()
		}
	}
}