package handlers

import (
	"encoding/json"
	"net/http"
)

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

	status, exists := submissions[submissionID]
	if !exists {
		http.Error(w, `{"error":"submission not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"submission_id": submissionID,
		"status":        status,
	})
}