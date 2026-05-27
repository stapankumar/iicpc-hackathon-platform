package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"

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

type submissionData struct {
	latencies []float64
	total     int
}

type MetricStore struct {
	mu     sync.Mutex
	stores map[string]*submissionData
	rdb    *redis.Client
}

func NewMetricStore() *MetricStore {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	return &MetricStore{
		stores: make(map[string]*submissionData),
		rdb: redis.NewClient(&redis.Options{
			Addr: redisAddr,
		}),
	}
}

func (m *MetricStore) Record(submissionID string, latencyMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.stores[submissionID]; !ok {
		m.stores[submissionID] = &submissionData{}
	}
	m.stores[submissionID].latencies = append(m.stores[submissionID].latencies, latencyMs)
	m.stores[submissionID].total++
}

func (m *MetricStore) PrintStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for subID, data := range m.stores {
		if len(data.latencies) == 0 || data.total%100 != 0 {
			continue
		}
		sorted := make([]float64, len(data.latencies))
		copy(sorted, data.latencies)
		sort.Float64s(sorted)

		p50 := percentile(sorted, 50)
		p90 := percentile(sorted, 90)
		p99 := percentile(sorted, 99)
		tps := float64(data.total) / 10.0
		score := (tps * 0.4) + ((1.0 / p99) * 1000 * 0.4) + (100.0 * 0.2)

		fmt.Printf("\n=== TELEMETRY STATS [%s] (n=%d) ===\n", subID, data.total)
		fmt.Printf("  p50: %.2f ms  p90: %.2f ms  p99: %.2f ms\n", p50, p90, p99)
		fmt.Printf("  TPS: %.0f  Score: %.2f\n", tps, score)
		fmt.Println("==============================")

		m.pushToLeaderboard(Score{
			SubmissionID: subID,
			P50:          p50, P90: p90, P99: p99,
			TPS: tps, Score: score,
		})
	}
}

func (m *MetricStore) pushToLeaderboard(s Score) {
	ctx := context.Background()

	data, err := json.Marshal(s)
	if err != nil {
		log.Printf("[LEADERBOARD] failed to marshal score: %v", err)
		return
	}

	err = m.rdb.ZAdd(ctx, "leaderboard", redis.Z{
		Score:  s.Score,
		Member: string(data),
	}).Err()

	if err != nil {
		log.Printf("[LEADERBOARD] failed to push score: %v", err)
		return
	}

	log.Printf("[LEADERBOARD] score pushed → submission: %s score: %.2f",
		s.SubmissionID, s.Score)
}

// Add this method to MetricStore
func (m *MetricStore) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for subID, data := range m.stores {
		if len(data.latencies) == 0 {
			log.Println("[TELEMETRY] No data to flush")
			return
		}

		sorted := make([]float64, len(data.latencies))
		copy(sorted, data.latencies)
		sort.Float64s(sorted)

		p50 := percentile(sorted, 50)
		p90 := percentile(sorted, 90)
		p99 := percentile(sorted, 99)
		tps := float64(data.total) / 10.0
		score := (tps * 0.4) + ((1.0 / p99) * 1000 * 0.4) + (100.0 * 0.2)

		log.Printf("[TELEMETRY] Flushing final score on shutdown — n=%d score=%.2f", data.total, score)

		m.pushToLeaderboard(Score{
			SubmissionID: subID,
			P50:          p50,
			P90:          p90,
			P99:          p99,
			TPS:          tps,
			Score:        score,
		})
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)) * p / 100)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
