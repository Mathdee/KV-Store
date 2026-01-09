package server

import (
	"sort"
	"sync"
	"time"
)

// Metrics will collect performance data from the server.
// We're making it thread-safe by using a mutex so multiple goroutines can record metrics at the same time.

type Metrics struct {
	mu            sync.Mutex
	totalRequests int64
	successCount  int64
	failCount     int64
	latencies     []time.Duration
	startTime     time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{
		latencies: make([]time.Duration, 0, 10000),
		startTime: time.Now(),
	}
}

func (m *Metrics) RecordSuccess(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRequests++
	m.successCount++
	m.latencies = append(m.latencies, latency)
}

func (m *Metrics) RecordFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRequests++
	m.failCount++
}

func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRequests = 0
	m.successCount = 0
	m.failCount = 0
	m.latencies = make([]time.Duration, 0, 10000)
	m.startTime = time.Now()
}

// Send the data collected to the dashboard.

type MetricsSnapshot struct {
	TotalRequests int64   `json:"totalRequests"`
	SuccessCount  int64   `json:"successCount"`
	FailCount     int64   `json:"failCount"`
	Throughput    float64 `json:"throughput"`    // requests per second
	LatencyAvg    float64 `json:"latencyAvgMs"`  // average in milliseconds
	LatencyP50    float64 `json:"latencyP50Ms"`  // median
	LatencyP95    float64 `json:"latencyP95Ms"`  // 95th percentile
	LatencyP99    float64 `json:"latencyP99Ms"`  // 99th percentile
	UptimeSeconds float64 `json:"uptimeSeconds"` // time since reset
}

//Calculate all metrics and return a snapshot.

func (m *Metrics) GetSnapshot() MetricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	uptime := time.Since(m.startTime).Seconds()

	snap := MetricsSnapshot{
		TotalRequests: m.totalRequests,
		SuccessCount:  m.successCount,
		FailCount:     m.failCount,
		UptimeSeconds: uptime,
	}

	//For throughput, we divide success count by uptime.
	if uptime > 0 {
		snap.Throughput = float64(m.successCount) / uptime
	}

	//For latency metrics, we need to sort the latencies and calculate percentiles.
	if len(m.latencies) > 0 {
		sorted := make([]time.Duration, len(m.latencies))
		copy(sorted, m.latencies) // copy them because we dont want to modify the original slice.
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i] < sorted[j]
		})

		var total time.Duration
		for _, l := range sorted {
			total += l
		}

		snap.LatencyAvg = float64(total.Microseconds()) / float64(len(sorted)) / 1000.0

		//Percentiles calculations.
		snap.LatencyP50 = float64(sorted[len(sorted)*50/100].Microseconds()) / 1000.0
		snap.LatencyP95 = float64(sorted[len(sorted)*95/100].Microseconds()) / 1000.0
		p99Idx := len(sorted) * 99 / 100
		if p99Idx >= len(sorted) {
			p99Idx = len(sorted) - 1
		}
		snap.LatencyP99 = float64(sorted[p99Idx].Microseconds()) / 1000.0

	}
	return snap

}
