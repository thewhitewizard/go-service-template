// Package observability owns everything this service exposes about itself.
//
// It is the only package allowed to import prometheus/client_golang, which the
// architecture test in internal/arch enforces. Callers depend on the narrow
// methods below instead of on prometheus types, so swapping the metrics backend
// stays a change confined to this package.
package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// durationBuckets are declared explicitly rather than reusing
// prometheus.DefBuckets.
//
// The defaults are calibrated for short HTTP requests and say nothing useful
// about a handler waiting on a slow dependency: everything above 10 s lands in
// +Inf and the p99 becomes unreadable exactly when you need it. This range
// covers an in-memory handler (5 ms) up to a call about to time out (10 s).
//
// Narrow it once you know your SLO. Every extra bucket is one more time series
// per label combination.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// Metrics holds the registry and the collectors of this service.
type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewMetrics builds a dedicated registry rather than using the global default one.
//
// A dedicated registry keeps tests independent (no collector leaks from one test
// to the next, no duplicate-registration panic) and makes it explicit which
// collectors this service publishes.
func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	m := &Metrics{
		registry: registry,
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests, by method, route pattern and status code.",
			},
			// The label set is deliberately small and every member is bounded:
			// method by the HTTP spec, status by the response codes in use, and
			// route by the allowlist of registered patterns applied upstream in
			// the transport middleware. Adding a caller-controlled label here
			// (an identifier, a free-form name, a raw path) is what takes a
			// metrics backend down.
			[]string{"method", "route", "status"},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency in seconds, by method and route pattern.",
				Buckets: durationBuckets,
			},
			// No status label here on purpose: a histogram costs len(buckets)+2
			// series per label combination, so multiplying it by the status set
			// buys detail nobody queries at a cost everybody pays.
			[]string{"method", "route"},
		),
	}

	registry.MustRegister(m.requests, m.duration)

	return m
}

// ObserveRequest records one finished HTTP request.
//
// route must already be a bounded value (a registered route pattern or the
// caller's constant for unmatched requests); this function does not and cannot
// validate that.
func (m *Metrics) ObserveRequest(method, route string, status int, d time.Duration) {
	statusLabel := strconv.Itoa(status)

	m.requests.WithLabelValues(method, route, statusLabel).Inc()
	m.duration.WithLabelValues(method, route).Observe(d.Seconds())
}

// Handler serves the Prometheus exposition format for this registry.
//
// It returns a stdlib http.Handler so this package stays free of any web
// framework: the transport layer adapts it.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// CountSeries returns how many time series the named metric family currently has.
//
// It exists so the cardinality guardrail can be tested from the transport layer
// without leaking prometheus types across the package boundary. A guardrail with
// no test that fails when it is removed is a guardrail that has never verified
// anything.
//
// It returns 0 when the family has not been observed yet, and -1 if the registry
// cannot be gathered.
func (m *Metrics) CountSeries(name string) int {
	families, err := m.registry.Gather()
	if err != nil {
		return -1
	}

	for _, family := range families {
		if family.GetName() == name {
			return len(family.GetMetric())
		}
	}

	return 0
}
