package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MessagesConsumed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messaging_consumed_total",
			Help: "Total number of consumed messages",
		},
		[]string{"queue", "status"},
	)

	MessagesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messaging_published_total",
			Help: "Total number of published messages",
		},
		[]string{"exchange", "routing_key", "status"},
	)

	MessageProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "messaging_processing_duration_seconds",
			Help:    "Message processing duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"queue"},
	)

	OutboxEventsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "outbox_events_processed_total",
			Help: "Total number of outbox events processed",
		},
		[]string{"status"},
	)

	WorkerCycleDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_cycle_duration_seconds",
			Help:    "Worker cycle duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"worker"},
	)

	MidtransSnapTokenCreated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "midtrans_snap_token_created_total",
			Help: "Total number of Midtrans Snap tokens successfully created",
		},
	)

	MidtransSnapTokenFailed = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "midtrans_snap_token_failed_total",
			Help: "Total number of failed Midtrans Snap token creation attempts",
		},
	)

	MidtransWebhook = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "midtrans_webhook_total",
			Help: "Total number of Midtrans webhooks by type and outcome",
		},
		[]string{"type", "outcome"},
	)
)
