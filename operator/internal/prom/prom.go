package prom

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	CRDRequestHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "elasti_operator_CRD_counter",
			Help:    "Tracks the number of active elasti service",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"crd_name", "error"},
	)

	CRDFinalizerCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "elasti_operator_CRD_finalizer_counter",
			Help: "Counter for CRD finalizer",
		},
		[]string{"crd_name", "error"},
	)

	CRDUpdateCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "elasti_operator_CRD_update_counter",
			Help: "Counter for CRD updates",
		},
		[]string{"crd_name", "mode", "error"},
	)

	InformerGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "elasti_operator_informer_count",
			Help: "Gauge for informer count",
		},
		[]string{"informer_name"},
	)

	InformerCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "elasti_operator_informer_counter",
			Help: "Counter for informer",
		},
		[]string{"crd_name", "action", "error"},
	)
)
