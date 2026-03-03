package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	CatalogRequestsTotal   *prometheus.CounterVec
	CatalogRequestDuration *prometheus.HistogramVec
	GRPCRequestsTotal      *prometheus.CounterVec
	GRPCRequestDuration    *prometheus.HistogramVec
	CacheRequestsTotal     *prometheus.CounterVec
	CacheRequestDuration   *prometheus.HistogramVec
}

var (
	metricsInstance *Metrics
	metricsOnce     sync.Once
)

func MustMetrics() *Metrics {
	metricsOnce.Do(func() {
		metricsInstance = newMetrics()
	})
	return metricsInstance
}

func newMetrics() *Metrics {
	m := &Metrics{
		CatalogRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "catalog",
				Subsystem: "service",
				Name:      "requests_total",
				Help:      "Total number of catalog service requests.",
			},
			[]string{"method", "status"},
		),
		CatalogRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "catalog",
				Subsystem: "service",
				Name:      "request_duration_seconds",
				Help:      "Catalog service request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method"},
		),
		GRPCRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "catalog",
				Subsystem: "grpc",
				Name:      "requests_total",
				Help:      "Total number of gRPC requests handled by catalog transport.",
			},
			[]string{"method", "code"},
		),
		GRPCRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "catalog",
				Subsystem: "grpc",
				Name:      "request_duration_seconds",
				Help:      "gRPC request duration in seconds for catalog transport.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method"},
		),
		CacheRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "catalog",
				Subsystem: "cache",
				Name:      "requests_total",
				Help:      "Total number of cache requests.",
			},
			[]string{"method", "result"},
		),
		CacheRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "catalog",
				Subsystem: "cache",
				Name:      "request_duration_seconds",
				Help:      "Cache request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "operation"},
		),
	}

	prometheus.MustRegister(
		m.CatalogRequestsTotal,
		m.CatalogRequestDuration,
		m.GRPCRequestsTotal,
		m.GRPCRequestDuration,
		m.CacheRequestsTotal,
		m.CacheRequestDuration,
	)

	return m
}
