package main

import (
	"log"
	"net/http"

	"github.com/iicpc/submission-service/handlers"
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

	mux.HandleFunc("/submit", corsMiddleware(handlers.HandleSubmit))
	mux.HandleFunc("/status", corsMiddleware(handlers.HandleStatus))

	log.Println("Submission service running on :8081")
	log.Fatal(http.ListenAndServe(":8081", mux))
}
