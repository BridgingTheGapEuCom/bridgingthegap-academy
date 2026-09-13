package platform

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestLivenessAndCorrelationID(t *testing.T) {
	r := newRouter(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_requests_total"}, []string{"route", "method", "status_class"}), prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_duration_seconds"}, []string{"route", "method"}), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing correlation ID")
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatal("missing JSON content type")
	}
}

func TestUnimplementedAPIUsesProblemDetails(t *testing.T) {
	r := newRouter(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_requests_total"}, []string{"route", "method", "status_class"}), prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_duration_seconds"}, []string{"route", "method"}), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/courses", nil))
	if w.Code != http.StatusNotFound || w.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("unexpected API response: %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	var body struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.RequestID == "" || body.RequestID != w.Header().Get("X-Request-ID") {
		t.Fatal("problem correlation ID does not match response header")
	}
}
