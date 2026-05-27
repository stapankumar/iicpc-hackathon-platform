package db

import (
	"context"
	"encoding/json"
	"os"
	"log"

	"github.com/redis/go-redis/v9"
)

type Score struct {
	SubmissionID string  `json:"submission_id"`
	P50          float64 `json:"p50_ms"`
	P90          float64 `json:"p90_ms"`
	P99          float64 `json:"p99_ms"`
	TPS          float64 `json:"tps"`
	Score        float64 `json:"score"`
}

var rdb = redis.NewClient(&redis.Options{
	Addr: getRedisAddr(),
})

func getRedisAddr() string {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return "localhost:6379" // local dev default
	}
	return addr
}

func SaveScore(s Score) error {
	ctx := context.Background()
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	// Store in Redis sorted set — score is the rank key
	return rdb.ZAdd(ctx, "leaderboard", redis.Z{
		Score:  s.Score,
		Member: string(data),
	}).Err()
}

func GetTopScores(limit int) ([]Score, error) {
	ctx := context.Background()

	// Get top N scores descending
	results, err := rdb.ZRevRangeWithScores(ctx, "leaderboard", 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	var scores []Score
	for _, r := range results {
		var s Score
		if err := json.Unmarshal([]byte(r.Member.(string)), &s); err != nil {
			log.Printf("failed to unmarshal score: %v", err)
			continue
		}
		scores = append(scores, s)
	}
	return scores, nil
}