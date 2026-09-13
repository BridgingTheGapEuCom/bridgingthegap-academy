package platform

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestHTTPMetricsBoundClientSuppliedMethodLabels(t *testing.T) {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_bounded_requests_total"}, []string{"route", "method", "status_class"})
	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_bounded_duration_seconds"}, []string{"route", "method"})
	registry := prometheus.NewRegistry()
	registry.MustRegister(requests, latency)
	router := newRouter(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), requests, latency, nil)
	for i := range 100 {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("ATTACK"+strconv.Itoa(i), "/health/live", nil))
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if len(family.GetMetric()) != 1 {
			t.Fatal("client-supplied HTTP methods created unbounded metric series")
		}
		for _, label := range family.GetMetric()[0].GetLabel() {
			if label.GetName() == "method" && label.GetValue() != "OTHER" {
				t.Fatal("unrecognized HTTP method was used as a metric label")
			}
		}
	}
}
