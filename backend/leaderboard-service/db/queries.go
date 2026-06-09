package db

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type Score struct {
	SubmissionID string  `json:"submission_id"`
	P50          float64 `json:"p50_ms"`
	P90          float64 `json:"p90_ms"`
	P99          float64 `json:"p99_ms"`
	TPS          float64 `json:"tps"`
	Correctness  float64 `json:"correctness"`
	Score        float64 `json:"score"`
	TeamName     string  `json:"team_name"`
	Attempts     int     `json:"attempts"`
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

	// Members are now submissionIDs, not JSON
	results, err := rdb.ZRevRangeWithScores(ctx, "leaderboard", 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	var scores []Score
	for _, r := range results {
		submissionID := r.Member.(string)

		// Fetch full score JSON from the details hash
		val, err := rdb.HGet(ctx, "leaderboard:details", submissionID).Result()
		if err != nil {
			log.Printf("failed to fetch details for %s: %v", submissionID, err)
			continue
		}

		var s Score
		if err := json.Unmarshal([]byte(val), &s); err != nil {
			log.Printf("failed to unmarshal score for %s: %v", submissionID, err)
			continue
		}

		attemptsStr, _ := rdb.Get(ctx, "attempts:"+s.TeamName).Result()
		s.Attempts, _ = strconv.Atoi(attemptsStr)

		scores = append(scores, s)
	}
	return scores, nil
}
