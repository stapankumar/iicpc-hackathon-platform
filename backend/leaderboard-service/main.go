package main

import (
	"log"
	"net/http"

	"github.com/iicpc/leaderboard-service/handlers"
)

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", corsMiddleware(handlers.HandleHealth))
	mux.HandleFunc("/scores", corsMiddleware(handlers.HandleScores))
	mux.HandleFunc("/ws", handlers.HandleWebSocket) // SSE handles CORS internally already
	log.Println("Leaderboard service running on :8082")
	log.Fatal(http.ListenAndServe(":8082", mux))
}
