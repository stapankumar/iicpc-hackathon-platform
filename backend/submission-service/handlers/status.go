package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
)

var rdb = redis.NewClient(&redis.Options{
	Addr: func() string {
		if a := os.Getenv("REDIS_ADDR"); a != "" {
			return a
		}
		return "localhost:6379"
	}(),
})

func HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	submissionID := r.URL.Query().Get("submission_id")
	if submissionID == "" {
		http.Error(w, `{"error":"missing submission_id"}`, http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	status, err := rdb.Get(ctx, "submission:status:"+submissionID).Result()
	if err != nil {
		// Not in Redis yet — check in-memory (still BUILDING/RECEIVED/RUNNING)
		var exists bool
		status, exists = submissions[submissionID]
		if !exists {
			http.Error(w, `{"error":"submission not found"}`, http.StatusNotFound)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"submission_id": submissionID,
		"status":        status,
	})
}
