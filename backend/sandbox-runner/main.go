package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	zipPath := os.Getenv("SUBMISSION_ZIP")
	if zipPath == "" {
		log.Fatal("SUBMISSION_ZIP env var not set")
	}

	log.Printf("Sandbox runner starting for: %s", zipPath)

	// Step 1 — unzip submission
	extractDir := "/tmp/contestant"
	err := unzip(zipPath, extractDir)
	if err != nil {
		log.Fatalf("failed to unzip: %v", err)
	}
	log.Println("Unzipped submission successfully")

	// Step 2 — run their binary (assumed name: server)
	binaryPath := filepath.Join(extractDir, "server")
	cmd := exec.Command(binaryPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	if err != nil {
		log.Fatalf("failed to start contestant binary: %v", err)
	}
	log.Printf("Contestant server started with PID %d", cmd.Process.Pid)

	// Step 3 — healthcheck until server is ready
	ready := waitForReady("http://localhost:8080/orderbook", 30)
	if !ready {
		log.Fatal("contestant server failed healthcheck — did not respond in 30s")
	}
	log.Println("Contestant server is healthy and ready")

	// Step 4 — keep running until K8s Job TTL kills us
	cmd.Wait()
}

func waitForReady(url string, timeoutSecs int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < timeoutSecs; i++ {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == 200 {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func unzip(src, dest string) error {
	os.MkdirAll(dest, 0755)
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
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
			return err
		}
		io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		fmt.Printf("extracted: %s\n", fpath)
	}
	return nil
}