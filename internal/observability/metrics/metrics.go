package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Metrics struct {
	HTTPRequestTotal    *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	TripCreateTotal    *prometheus.CounterVec
	TripCreateDuration *prometheus.HistogramVec

	TripPublishTotal        *prometheus.CounterVec
	TripPublishDuration     *prometheus.HistogramVec
	ContractRequestTotal    *prometheus.CounterVec
	ContractRequestDuration *prometheus.HistogramVec
	OutboxPending           prometheus.Gauge
	OutboxPublishTotal      *prometheus.CounterVec
	OutboxPublishFailed     prometheus.Counter
}

func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		ContractRequestTotal:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "sharetrip_contract_request_total", Help: "Contract permission checks, including retries."}, []string{"result"}),
		ContractRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "sharetrip_contract_request_duration_seconds", Help: "Contract permission check duration, including retries."}, []string{"result"}),
		OutboxPending:           prometheus.NewGauge(prometheus.GaugeOpts{Name: "sharetrip_outbox_pending", Help: "Pending outbox rows at the last successful sample."}),
		OutboxPublishTotal:      prometheus.NewCounterVec(prometheus.CounterOpts{Name: "sharetrip_outbox_publish_total", Help: "Outbox publishing attempts; success means Kafka acknowledged the event."}, []string{"result"}),
		OutboxPublishFailed:     prometheus.NewCounter(prometheus.CounterOpts{Name: "sharetrip_outbox_publish_failed_total", Help: "Failed outbox publishing attempts, including retries."}),
		HTTPRequestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "sharetrip",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "route", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "sharetrip",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "route", "status"},
		),

		TripCreateTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "sharetrip",
				Subsystem: "trip",
				Name:      "create_total",
				Help:      "Total number of create trip attempts",
			},
			[]string{"result"},
		),
		TripCreateDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "sharetrip",
				Subsystem: "trip",
				Name:      "create_duration_seconds",
				Help:      "Create trip use-case duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"result"},
		),

		TripPublishTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "sharetrip",
				Subsystem: "trip",
				Name:      "publish_total",
				Help:      "Total number of publish trip attempts",
			},
			[]string{"result"},
		),
		TripPublishDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "sharetrip",
				Subsystem: "trip",
				Name:      "publish_duration_seconds",
				Help:      "Publish trip use-case duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"result"},
		),
	}

	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.HTTPRequestTotal,
		m.HTTPRequestDuration,
		m.TripCreateTotal,
		m.TripCreateDuration,
		m.TripPublishTotal,
		m.TripPublishDuration,
		m.ContractRequestTotal, m.ContractRequestDuration,
		m.OutboxPending, m.OutboxPublishTotal, m.OutboxPublishFailed,
	)
	for _, result := range []string{"allowed", "denied", "error"} {
		m.ContractRequestTotal.WithLabelValues(result)
		m.ContractRequestDuration.WithLabelValues(result)
	}
	for _, result := range []string{"success", "error"} {
		m.OutboxPublishTotal.WithLabelValues(result)
	}
	for _, result := range []string{"success", "already_published", "forbidden", "not_found", "conflict", "validation_error", "internal_error"} {
		m.TripPublishTotal.WithLabelValues(result)
		m.TripPublishDuration.WithLabelValues(result)
	}

	return m
}
