package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/iicpc/submission-service/k8s"
)

// in-memory store of submissions
// key = submissionID, value = status
var submissions = make(map[string]string)

func HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(50 << 20)
	if err != nil {
		http.Error(w, `{"error":"file too large or invalid"}`, http.StatusBadRequest)
		return
	}

	teamName := r.FormValue("team_name")
	if teamName == "" {
		teamName = "anonymous"
	}
	log.Printf("[SUBMIT] team_name received: %s", teamName)

	file, _, err := r.FormFile("submission")
	if err != nil {
		http.Error(w, `{"error":"missing submission file"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	submissionID := uuid.NewString()
	id8 := submissionID[:8]

	// Save zip to disk first
	uploadDir := "/tmp/submissions"
	os.MkdirAll(uploadDir, 0755)
	zipPath := filepath.Join(uploadDir, id8+".zip")

	dst, err := os.Create(zipPath)
	if err != nil {
		http.Error(w, `{"error":"failed to save file"}`, http.StatusInternalServerError)
		return
	}
	io.Copy(dst, file)
	dst.Close()

	// Extract zip to workspace directory for Kaniko
	workspaceDir := filepath.Join(uploadDir, id8)
	if err := extractZip(zipPath, workspaceDir); err != nil {
		submissions[submissionID] = "FAILED"
		log.Printf("[SUBMIT] failed to extract zip: %v", err)
		http.Error(w, `{"error":"failed to extract submission"}`, http.StatusBadRequest)
		return
	}

	// Verify Dockerfile exists
	if _, err := os.Stat(filepath.Join(workspaceDir, "Dockerfile")); err != nil {
		submissions[submissionID] = "FAILED"
		http.Error(w, `{"error":"Dockerfile not found in submission"}`, http.StatusBadRequest)
		return
	}

	log.Printf("[SUBMIT] submission %s extracted to %s", id8, workspaceDir)
	submissions[submissionID] = "RECEIVED"

	// Spawn pipeline in background — response returns immediately
	go func() {
		submissions[submissionID] = "BUILDING"
		if err := k8s.SpawnSandboxJob(submissionID, workspaceDir, teamName); err != nil {
			submissions[submissionID] = "FAILED"
			log.Printf("[SUBMIT] pipeline failed for %s: %v", id8, err)
			return
		}
		submissions[submissionID] = "RUNNING"
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"submission_id": submissionID,
		"status":        "BUILDING",
		"team_name":     teamName,
		"message":       fmt.Sprintf("Submission received, building image for %s", submissionID),
	})
}

func extractZip(zipPath, destDir string) error {
	os.MkdirAll(destDir, 0755)

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(fpath), 0755)
		outFile, err := os.Create(fpath)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}
		io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}
	return nil
}
