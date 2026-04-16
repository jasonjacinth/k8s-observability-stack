package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Custom Prometheus metrics for the task-processing application.
var (
	tasksInQueue = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tasks_in_queue",
		Help: "Current number of tasks waiting in the processing queue.",
	})

	tasksProcessedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tasks_processed_total",
		Help: "Total number of tasks that have been processed and dequeued.",
	})

	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"handler", "method"})
)

func init() {
	prometheus.MustRegister(tasksInQueue)
	prometheus.MustRegister(tasksProcessedTotal)
	prometheus.MustRegister(httpRequestDuration)
}

// instrumentHandler wraps an http.HandlerFunc with duration tracking.
func instrumentHandler(name string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timer := prometheus.NewTimer(httpRequestDuration.WithLabelValues(name, r.Method))
		defer timer.ObserveDuration()
		next(w, r)
	}
}

// queueMu guards the in-memory queue counter to prevent race conditions
// when concurrent enqueue/dequeue requests arrive.
var queueMu sync.Mutex
var queueSize int

func enqueueHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed. Use POST.", http.StatusMethodNotAllowed)
		return
	}

	queueMu.Lock()
	queueSize++
	tasksInQueue.Set(float64(queueSize))
	current := queueSize
	queueMu.Unlock()

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, "Task enqueued. Queue size: %d\n", current)
	log.Printf("task enqueued -- queue_size=%d", current)
}

func dequeueHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed. Use POST.", http.StatusMethodNotAllowed)
		return
	}

	queueMu.Lock()
	if queueSize <= 0 {
		queueMu.Unlock()
		http.Error(w, "Queue is empty. Nothing to dequeue.", http.StatusConflict)
		return
	}
	queueSize--
	tasksInQueue.Set(float64(queueSize))
	tasksProcessedTotal.Inc()
	current := queueSize
	queueMu.Unlock()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Task dequeued. Queue size: %d\n", current)
	log.Printf("task dequeued -- queue_size=%d", current)
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func readyzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/enqueue", instrumentHandler("enqueue", enqueueHandler))
	mux.HandleFunc("/dequeue", instrumentHandler("dequeue", dequeueHandler))
	mux.HandleFunc("/healthz", instrumentHandler("healthz", healthzHandler))
	mux.HandleFunc("/readyz", instrumentHandler("readyz", readyzHandler))
	mux.Handle("/metrics", promhttp.Handler())

	addr := ":8080"
	log.Printf("task-processor starting on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
