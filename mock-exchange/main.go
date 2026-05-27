package main

import (
	"log"
	"net/http"

	"github.com/iicpc/mock-exchange/handlers"
)

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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
	mux.HandleFunc("/order", corsMiddleware(handlers.HandleOrder))
	mux.HandleFunc("/orderbook", corsMiddleware(handlers.HandleOrderbook))

	log.Println("Mock Exchange running on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}