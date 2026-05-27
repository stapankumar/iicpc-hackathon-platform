package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/iicpc/leaderboard-service/db"
)

func HandleScores(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	scores, err := db.GetTopScores(50)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch scores"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scores)
}