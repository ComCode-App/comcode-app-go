// Package metrics exposes a small, dependency-free Prometheus text endpoint.
//
// It is disabled by default: the API and worker only listen on METRICS_ADDR when it
// is set, on a separate (internal) port so /metrics is never routed publicly.
// Label sets are fixed and low-cardinality (method, status class, task type).
package metrics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type counterVec struct {
	mu     sync.Mutex
	values map[string]*atomic.Uint64
}

func (c *counterVec) inc(labels string) {
	c.mu.Lock()
	if c.values == nil {
		c.values = map[string]*atomic.Uint64{}
	}
	v, ok := c.values[labels]
	if !ok {
		v = &atomic.Uint64{}
		c.values[labels] = v
	}
	c.mu.Unlock()
	v.Add(1)
}

func (c *counterVec) snapshot() map[string]uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]uint64, len(c.values))
	for k, v := range c.values {
		out[k] = v.Load()
	}
	return out
}

type histogram struct {
	mu     sync.Mutex
	counts []uint64 // per bucket, non-cumulative; last = +Inf
	sum    float64
	count  uint64
}

func (h *histogram) observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.counts == nil {
		h.counts = make([]uint64, len(durationBuckets)+1)
	}
	i := sort.SearchFloat64s(durationBuckets, v)
	h.counts[i]++
	h.sum += v
	h.count++
}

var (
	httpRequests   counterVec
	httpDuration   histogram
	queueTasks     counterVec
	queueDurations histogram
)

// ObserveHTTP records one finished HTTP request.
func ObserveHTTP(method string, status int, d time.Duration) {
	httpRequests.inc(fmt.Sprintf(`method=%q,code=%q`, normalizeMethod(method), statusClass(status)))
	httpDuration.observe(d.Seconds())
}

// ObserveTask records one finished Asynq task.
func ObserveTask(taskType string, err error, d time.Duration) {
	result := "success"
	if err != nil {
		result = "failure"
	}
	queueTasks.inc(fmt.Sprintf(`type=%q,result=%q`, taskType, result))
	queueDurations.observe(d.Seconds())
}

// Handler writes all metrics in the Prometheus text format.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		Write(w)
	})
}

// Write renders the exposition text.
func Write(w io.Writer) {
	writeCounter(w, "http_requests_total", "HTTP requests by method and status class.", httpRequests.snapshot())
	writeHistogram(w, "http_request_duration_seconds", "HTTP request latency.", &httpDuration)
	writeCounter(w, "queue_tasks_total", "Asynq tasks processed by type and result.", queueTasks.snapshot())
	writeHistogram(w, "queue_task_duration_seconds", "Asynq task processing time.", &queueDurations)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	writeGauge(w, "go_goroutines", "Number of goroutines.", float64(runtime.NumGoroutine()))
	writeGauge(w, "go_memstats_heap_alloc_bytes", "Heap bytes allocated and in use.", float64(ms.HeapAlloc))
	writeGauge(w, "go_memstats_sys_bytes", "Bytes obtained from the OS.", float64(ms.Sys))
}

// Serve listens on addr until ctx is cancelled. Empty addr disables metrics.
func Serve(ctx context.Context, addr string, log *slog.Logger) {
	if addr == "" {
		return
	}
	// Internal listener only (not the API router, so it is not part of the OpenAPI contract).
	metricsHandler := Handler()
	only := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			http.NotFound(w, r)
			return
		}
		metricsHandler.ServeHTTP(w, r)
	})
	srv := &http.Server{Addr: addr, Handler: only, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	go func() {
		if log != nil {
			log.Info("metrics_listen", "addr", addr)
		}
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) && log != nil {
			log.Error("metrics_listen_failed", "addr", addr, "error", err)
		}
	}()
}

func writeCounter(w io.Writer, name, help string, values map[string]uint64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", name, help, name)
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = fmt.Fprintf(w, "%s{%s} %d\n", name, k, values[k])
	}
}

func writeGauge(w io.Writer, name, help string, v float64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %s\n", name, help, name, name, formatFloat(v))
}

func writeHistogram(w io.Writer, name, help string, h *histogram) {
	h.mu.Lock()
	counts := append([]uint64(nil), h.counts...)
	sum, count := h.sum, h.count
	h.mu.Unlock()
	if counts == nil {
		counts = make([]uint64, len(durationBuckets)+1)
	}
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s histogram\n", name, help, name)
	var cum uint64
	for i, le := range durationBuckets {
		cum += counts[i]
		_, _ = fmt.Fprintf(w, "%s_bucket{le=%q} %d\n", name, formatFloat(le), cum)
	}
	_, _ = fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n", name, count)
	_, _ = fmt.Fprintf(w, "%s_sum %s\n%s_count %d\n", name, formatFloat(sum), name, count)
}

func formatFloat(v float64) string {
	if math.IsInf(v, 1) {
		return "+Inf"
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func statusClass(status int) string {
	if status < 100 || status > 599 {
		return "other"
	}
	return strconv.Itoa(status/100) + "xx"
}

func normalizeMethod(m string) string {
	switch m = strings.ToUpper(m); m {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodHead, http.MethodOptions:
		return m
	default:
		return "OTHER"
	}
}
