package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func resetQueue() {
	queueMu.Lock()
	queueSize = 0
	tasksInQueue.Set(0)
	queueMu.Unlock()
}

func TestHealthzHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	healthzHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body %q, got %q", "ok", rec.Body.String())
	}
}

func TestReadyzHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	readyzHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body %q, got %q", "ok", rec.Body.String())
	}
}

func TestEnqueueHandler(t *testing.T) {
	t.Run("POST increments queue", func(t *testing.T) {
		resetQueue()
		req := httptest.NewRequest(http.MethodPost, "/enqueue", nil)
		rec := httptest.NewRecorder()

		enqueueHandler(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Errorf("expected status %d, got %d", http.StatusAccepted, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Queue size: 1") {
			t.Errorf("expected body to contain 'Queue size: 1', got %q", rec.Body.String())
		}
		queueMu.Lock()
		defer queueMu.Unlock()
		if queueSize != 1 {
			t.Errorf("expected queueSize 1, got %d", queueSize)
		}
	})

	t.Run("GET returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/enqueue", nil)
		rec := httptest.NewRecorder()

		enqueueHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("multiple enqueues accumulate", func(t *testing.T) {
		resetQueue()
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodPost, "/enqueue", nil)
			rec := httptest.NewRecorder()
			enqueueHandler(rec, req)
		}
		queueMu.Lock()
		defer queueMu.Unlock()
		if queueSize != 5 {
			t.Errorf("expected queueSize 5 after 5 enqueues, got %d", queueSize)
		}
	})
}

func TestDequeueHandler(t *testing.T) {
	t.Run("POST decrements queue", func(t *testing.T) {
		resetQueue()
		queueMu.Lock()
		queueSize = 3
		tasksInQueue.Set(3)
		queueMu.Unlock()

		req := httptest.NewRequest(http.MethodPost, "/dequeue", nil)
		rec := httptest.NewRecorder()

		dequeueHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Queue size: 2") {
			t.Errorf("expected body to contain 'Queue size: 2', got %q", rec.Body.String())
		}
		queueMu.Lock()
		defer queueMu.Unlock()
		if queueSize != 2 {
			t.Errorf("expected queueSize 2, got %d", queueSize)
		}
	})

	t.Run("empty queue returns conflict", func(t *testing.T) {
		resetQueue()
		req := httptest.NewRequest(http.MethodPost, "/dequeue", nil)
		rec := httptest.NewRecorder()

		dequeueHandler(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d", http.StatusConflict, rec.Code)
		}
	})

	t.Run("GET returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dequeue", nil)
		rec := httptest.NewRecorder()

		dequeueHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}
