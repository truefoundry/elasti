package prom

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HostExtractionCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "elasti_resolver_host_extraction_count",
			Help: "Counter for host extraction",
		},
		[]string{
			"extractionType",
			"source",
			"hostHeader",
			"error",
		},
	)

	QueuedRequestGague = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "elasti_resolver_queued_count",
			Help: "Gauge for queued requests",
		},
		[]string{},
	)

	IncomingRequestHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "elasti_resolver_incoming_requests",
			Help:    "Histogram of response latency (seconds) for every request resolved",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"source",
			"target",
			"sourceHost",
			"targetHost",
			"namespace",
			"method",
			"requestURI",
			"status",
			"error"},
	)

	TrafficSwitchCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "elasti_resolver_traffic_switch_count",
			Help: "Counter for traffic switch",
		},
		[]string{"source", "enabled"},
	)

	OperatorRPCCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "elasti_resolver_operator_rpc_count",
			Help: "Counter for operator RPC",
		},
		[]string{"error"},
	)
)
