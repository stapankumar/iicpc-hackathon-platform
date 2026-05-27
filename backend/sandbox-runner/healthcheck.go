package main

import (
	"fmt"
	"net/http"
	"time"
)

func healthcheck(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("healthcheck failed: %v\n", err)
		return false
	}
	return resp.StatusCode == 200
}