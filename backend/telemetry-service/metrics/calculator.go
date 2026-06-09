package metrics

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

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
}

type submissionData struct {
	latencies []float64
	total     int
	startTime time.Time
	teamName  string
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

func (m *MetricStore) Record(submissionID string, latencyMs float64, teamName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.stores[submissionID]; !ok {
		m.stores[submissionID] = &submissionData{}
	}
	d := m.stores[submissionID]
	d.latencies = append(d.latencies, latencyMs)
	d.total++
	if d.total == 1 {
		d.startTime = time.Now() // Record start time on first record
	}
	if d.teamName == "" && teamName != "" {
		d.teamName = teamName
	}
}

// FinalizeSubmission is called once when the bot fleet sends the done signal.
// It computes the final score over all recorded latencies, reads the correctness
// score published by the correctness harness from Redis, then pushes one entry
// to the leaderboard. Cleans up memory after.
func (m *MetricStore) FinalizeSubmission(submissionID string, teamName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, ok := m.stores[submissionID]
	if !ok || len(data.latencies) == 0 {
		log.Printf("[TELEMETRY] no latency data for %s, skipping finalize", submissionID)
		return
	}

	sorted := make([]float64, len(data.latencies))
	copy(sorted, data.latencies)
	sort.Float64s(sorted)

	p50 := percentile(sorted, 50)
	p90 := percentile(sorted, 90)
	p99 := percentile(sorted, 99)
	elapsed := time.Since(data.startTime).Seconds()
	if elapsed < 1 {
		elapsed = 1
	}
	tps := float64(data.total) / elapsed

	// Read correctness score published by the correctness harness job.
	// If not found (harness failed or timed out), default to 0.
	correctness := 0.0
	ctx := context.Background()
	key := "correctness:" + submissionID
	val, err := m.rdb.Get(ctx, key).Result()
	if err != nil {
		log.Printf("[TELEMETRY] correctness key not found for %s, defaulting to 0.0", submissionID)
	} else {
		correctness, err = strconv.ParseFloat(val, 64)
		if err != nil {
			log.Printf("[TELEMETRY] failed to parse correctness for %s: %v", submissionID, err)
			correctness = 0.0
		}
	}

	// Score formula: 40% TPS + 40% latency (inverse p99) + 20% correctness
	score := (tps * 0.4) + ((1.0 / p99) * 1000 * 0.4) + (correctness * 100 * 0.2)

	log.Printf("[TELEMETRY] FINAL — %s n=%d p50=%.2fms p90=%.2fms p99=%.2fms tps=%.0f correctness=%.2f score=%.2f",
		submissionID, data.total, p50, p90, p99, tps, correctness, score)

	member := teamName
	if member == "" {
		member = data.teamName
	}
	if member == "" {
		member = submissionID
	}

	m.rdb.Incr(ctx, "attempts:"+member)
	m.rdb.Set(ctx, "submission:status:"+submissionID, "SCORED", 24*time.Hour)

	existing, err := m.rdb.ZScore(ctx, "leaderboard", member).Result()
	if err == nil && existing >= score {
		log.Printf("[TELEMETRY] score %.2f not better than existing %.2f for %s, skipping", score, existing, member)
		delete(m.stores, submissionID)
		return
	}

	m.pushToLeaderboard(Score{
		SubmissionID: submissionID,
		TeamName:     member,
		P50:          p50,
		P90:          p90,
		P99:          p99,
		TPS:          tps,
		Correctness:  correctness,
		Score:        score,
	})

	// Clean up — this submission is done, free the memory
	delete(m.stores, submissionID)

	// Clean up the correctness key from Redis too
	m.rdb.Del(ctx, key)
}

// Flush is a shutdown safety net — finalizes any submission that never got a done signal.
func (m *MetricStore) Flush() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.stores))
	for id := range m.stores {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		log.Printf("[TELEMETRY] flush on shutdown for %s", id)
		m.FinalizeSubmission(id, "")
	}
}

func (m *MetricStore) pushToLeaderboard(s Score) {
	ctx := context.Background()

	data, err := json.Marshal(s)
	if err != nil {
		log.Printf("[LEADERBOARD] failed to marshal score: %v", err)
		return
	}

	pipe := m.rdb.Pipeline()

	// SubmissionID as member — ZAdd updates in place, one entry per submission
	pipe.ZAdd(ctx, "leaderboard", redis.Z{
		Score:  s.Score,
		Member: s.TeamName,
	})

	// Full score details in a hash for the leaderboard service to read
	pipe.HSet(ctx, "leaderboard:details", s.TeamName, string(data))

	pipe.Set(ctx, "submission:status:"+s.SubmissionID, "SCORED", 24*time.Hour)
	pipe.Set(ctx, "submission:score:"+s.SubmissionID, strconv.FormatFloat(s.Score, 'f', 2, 64), 24*time.Hour)

	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Printf("[LEADERBOARD] failed to push score: %v", err)
		return
	}

	log.Printf("[LEADERBOARD] pushed → %s correctness=%.2f score=%.2f",
		s.SubmissionID, s.Correctness, s.Score)
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
