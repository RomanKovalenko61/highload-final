package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal общее количество запросов
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// RequestDuration продолжительность запросов
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// MetricsReceived метрики получены
	MetricsReceived = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "metrics_received_total",
			Help: "Total number of metrics received",
		},
	)

	// AnomaliesDetected обнаруженные аномалии
	AnomaliesDetected = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "anomalies_detected_total",
			Help: "Total number of anomalies detected",
		},
		[]string{"type", "device_id"},
	)

	// AnalysisLatency задержка анализа
	AnalysisLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "analysis_latency_seconds",
			Help:    "Analysis processing latency in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	// CurrentZScore текущий z-score (gauge)
	CurrentZScore = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "current_zscore",
			Help: "Current z-score for devices",
		},
		[]string{"device_id", "metric_type"},
	)

	// RollingAverage текущее скользящее среднее
	RollingAverage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rolling_average",
			Help: "Current rolling average for metrics",
		},
		[]string{"device_id", "metric_type"},
	)

	// ActiveDevices активные устройства
	ActiveDevices = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_devices",
			Help: "Number of currently active devices",
		},
	)

	// QueueSize размер очереди обработки
	QueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "processing_queue_size",
			Help: "Current size of the processing queue",
		},
	)

	// RedisOperations операции с Redis
	RedisOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_operations_total",
			Help: "Total number of Redis operations",
		},
		[]string{"operation", "status"},
	)

	// CacheHitRate коэффициент попаданий в кэш
	CacheHitRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_hit_rate",
			Help: "Cache hit rate",
		},
		[]string{"cache_type"},
	)
)
