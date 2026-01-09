package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mathdee/KV-Store/internal/raft"
	"github.com/mathdee/KV-Store/internal/store"
)

type HTTPServer struct {
	raft    *raft.Consensus // this turns into a pointer to the consensus struct in the file raft.go
	metrics *Metrics
	store   *store.Store
}

type StatusResponse struct {
	State       string `json:"state"`       //leader, follower, candidate
	Term        int    `json:"term"`        // current term number
	ID          string `json:"id"`          // ID of curr server
	LogLength   int    `json:"logLength"`   // number of log entries
	CommitIndex int    `json:"commitIndex"` // index of commited entries
	Paused      bool   `json:"paused"`      // true if node is paused
}

type BenchmarkResult struct {
	TotalRequests int64   `json:"totalRequests"`
	Successful    int64   `json:"successful"`
	Failed        int64   `json:"failed"`
	DurationMs    float64 `json:"durationMs"`
	Throughput    float64 `json:"throughput"`
	LatencyAvgMs  float64 `json:"latencyAvgMs"`
	LatencyP50Ms  float64 `json:"latencyP50Ms"`
	LatencyP95Ms  float64 `json:"latencyP95Ms"`
	LatencyP99Ms  float64 `json:"latencyP99Ms"`
}

func NewHTTPServer(r *raft.Consensus, m *Metrics, s *store.Store) *HTTPServer {
	return &HTTPServer{raft: r, metrics: m, store: s}
}

func (h *HTTPServer) Start(port string) {
	mux := http.NewServeMux()

	// GET /status - returns node status in a json.
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		status := StatusResponse{
			State:       h.raft.GetState(),
			Term:        h.raft.GetTerm(),
			ID:          h.raft.ID,
			LogLength:   h.raft.GetLogLength(),
			CommitIndex: h.raft.GetCommitIndex(),
			Paused:      h.raft.IsPaused(), // include paused state in response
		}
		json.NewEncoder(w).Encode(status)

	})

	// GET /pause - pauses node for failover demo
	mux.HandleFunc("/pause", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // allow dashboard cross-origin requests
		h.raft.Pause()                                     // call Pause method on raft
		w.Write([]byte("Node paused"))                     // send confirmation to client response
	})

	// GET /resume - resumes paused node operation
	mux.HandleFunc("/resume", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // allow dashboard cross-origin requests
		h.raft.Resume()                                    // call Resume method on raft
		w.Write([]byte("Node resumed"))                    // send confirmation to client response
	})

	// GET /metrics - returns performance metrics in json.
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		snapshot := h.metrics.GetSnapshot()
		json.NewEncoder(w).Encode(snapshot)
	})

	// POST /metrics/reset - clears metrics for fresh benchmark
	mux.HandleFunc("/metrics/reset", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.metrics.Reset()
		w.Write([]byte("Metrics reset"))
	})

	// POST /clear - clears data and metrics for fresh benchmark
	mux.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.raft.ClearLog() // Clear Raft log
		h.metrics.Reset() // Reset metrics
		w.Write([]byte("Data cleared"))
	})

	mux.HandleFunc("/benchmark", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		reqCount, _ := strconv.Atoi(r.URL.Query().Get("requests"))
		concurrency, _ := strconv.Atoi(r.URL.Query().Get("concurrency"))

		if reqCount <= 0 {
			reqCount = 10000
		}
		if concurrency <= 0 {
			concurrency = 100
		}

		// Direct benchmark - no TCP overhead
		result := h.runDirectBenchmark(reqCount, concurrency)
		json.NewEncoder(w).Encode(result)
	})

	fmt.Printf("HTTP status server on %s\n", port)
	http.ListenAndServe(port, mux) // listens on port and serves requests using mux router.
}

func (h *HTTPServer) runDirectBenchmark(numRequests int, concurrency int) BenchmarkResult {
	// Must be leader to run benchmark
	if h.raft.GetState() != "Leader" {
		return BenchmarkResult{
			TotalRequests: int64(numRequests),
			Failed:        int64(numRequests),
		}
	}

	var wg sync.WaitGroup
	var successCount int64
	var failCount int64
	var latencies []time.Duration
	var latencyMu sync.Mutex
	var stopped int32 // Atomic flag to stop workers

	requestsPerWorker := numRequests / concurrency
	start := time.Now()

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < requestsPerWorker; i++ {
				// Check if we should stop (no longer leader or paused)
				if atomic.LoadInt32(&stopped) == 1 {
					atomic.AddInt64(&failCount, int64(requestsPerWorker-i))
					return
				}

				// Periodically check leadership (every 100 ops)
				if i%100 == 0 {
					if h.raft.IsPaused() || h.raft.GetState() != "Leader" {
						atomic.StoreInt32(&stopped, 1)
						atomic.AddInt64(&failCount, int64(requestsPerWorker-i))
						return
					}
				}

				key := fmt.Sprintf("bench_%d_%d", workerID, i)
				value := fmt.Sprintf("value_%d_%d", workerID, i)

				opStart := time.Now()
				h.store.Set(key, value)
				h.raft.AddLogEntry("SET " + key + " " + value)
				latency := time.Since(opStart)

				atomic.AddInt64(&successCount, 1)
				latencyMu.Lock()
				latencies = append(latencies, latency)
				latencyMu.Unlock()
			}
		}(w)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Build result with whatever we completed
	result := BenchmarkResult{
		TotalRequests: int64(numRequests),
		Successful:    successCount,
		Failed:        failCount,
		DurationMs:    float64(elapsed.Milliseconds()),
		Throughput:    float64(successCount) / elapsed.Seconds(),
	}

	// Calculate latencies only for successful ops
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		result.LatencyAvgMs = float64(total.Microseconds()) / float64(len(latencies)) / 1000.0
		result.LatencyP50Ms = float64(latencies[len(latencies)*50/100].Microseconds()) / 1000.0
		result.LatencyP95Ms = float64(latencies[len(latencies)*95/100].Microseconds()) / 1000.0
		p99Idx := len(latencies) * 99 / 100
		if p99Idx >= len(latencies) {
			p99Idx = len(latencies) - 1
		}
		result.LatencyP99Ms = float64(latencies[p99Idx].Microseconds()) / 1000.0
	}

	return result
}
