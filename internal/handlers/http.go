package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"highload-final/internal/analytics"
	"highload-final/internal/cache"
	"highload-final/internal/metrics"
	"highload-final/internal/models"
)

// Handler обработчик HTTP запросов
type Handler struct {
	analyzer *analytics.Analyzer
	cache    *cache.RedisCache
}

// NewHandler создает новый обработчик
func NewHandler(analyzer *analytics.Analyzer, cache *cache.RedisCache) *Handler {
	return &Handler{
		analyzer: analyzer,
		cache:    cache,
	}
}

// SubmitMetric обрабатывает POST /metrics
func (h *Handler) SubmitMetric(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		metrics.RequestDuration.WithLabelValues(r.Method, "/metrics").Observe(duration)
	}()

	if r.Method != http.MethodPost {
		metrics.RequestsTotal.WithLabelValues(r.Method, "/metrics", "405").Inc()
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var metric models.Metric
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		metrics.RequestsTotal.WithLabelValues(r.Method, "/metrics", "400").Inc()
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Устанавливаем timestamp если не указан
	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now()
	}

	// Валидация
	if metric.DeviceID == "" {
		metrics.RequestsTotal.WithLabelValues(r.Method, "/metrics", "400").Inc()
		http.Error(w, "device_id is required", http.StatusBadRequest)
		return
	}

	// Сохраняем в Redis (асинхронно, не блокируем ответ)
	go func() {
		if err := h.cache.StoreMetric(metric.DeviceID, metric.Timestamp, metric); err == nil {
			metrics.RedisOperations.WithLabelValues("store_metric", "success").Inc()
		} else {
			metrics.RedisOperations.WithLabelValues("store_metric", "error").Inc()
		}
	}()

	// Отправляем на анализ
	h.analyzer.AddMetric(analytics.MetricData{
		DeviceID:  metric.DeviceID,
		Timestamp: metric.Timestamp,
		CPU:       metric.CPU,
		RPS:       metric.RPS,
	})

	metrics.MetricsReceived.Inc()
	metrics.RequestsTotal.WithLabelValues(r.Method, "/metrics", "200").Inc()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "accepted",
		"device_id": metric.DeviceID,
	})
}

// GetAnalytics обрабатывает GET /analytics
func (h *Handler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		metrics.RequestDuration.WithLabelValues(r.Method, "/analytics").Observe(duration)
	}()

	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		metrics.RequestsTotal.WithLabelValues(r.Method, "/analytics", "400").Inc()
		http.Error(w, "device_id parameter is required", http.StatusBadRequest)
		return
	}

	// Получаем последние аномалии из кэша
	anomalyKeys, err := h.cache.GetRecentAnomalies(deviceID, 10)
	if err != nil {
		metrics.RequestsTotal.WithLabelValues(r.Method, "/analytics", "500").Inc()
		http.Error(w, "Failed to retrieve analytics", http.StatusInternalServerError)
		return
	}

	metrics.RedisOperations.WithLabelValues("get_anomalies", "success").Inc()
	metrics.RequestsTotal.WithLabelValues(r.Method, "/analytics", "200").Inc()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"device_id":     deviceID,
		"anomaly_count": len(anomalyKeys),
		"anomalies":     anomalyKeys,
	})
}

// HealthCheck обрабатывает GET /health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Проверяем Redis
	redisOK := h.cache.Ping() == nil

	status := "healthy"
	httpStatus := http.StatusOK

	if !redisOK {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    status,
		"redis":     redisOK,
		"timestamp": time.Now(),
	})
}

// GetStats обрабатывает GET /stats
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		metrics.RequestDuration.WithLabelValues(r.Method, "/stats").Observe(duration)
	}()

	analyzerStats := h.analyzer.GetStats()
	redisStats := h.cache.GetStats()

	metrics.RequestsTotal.WithLabelValues(r.Method, "/stats", "200").Inc()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"analyzer":  analyzerStats,
		"redis":     redisStats,
		"timestamp": time.Now(),
	})
}

// BatchSubmitMetrics обрабатывает POST /metrics/batch
func (h *Handler) BatchSubmitMetrics(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		metrics.RequestDuration.WithLabelValues(r.Method, "/metrics/batch").Observe(duration)
	}()

	if r.Method != http.MethodPost {
		metrics.RequestsTotal.WithLabelValues(r.Method, "/metrics/batch", "405").Inc()
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var batchMetrics []models.Metric
	if err := json.NewDecoder(r.Body).Decode(&batchMetrics); err != nil {
		metrics.RequestsTotal.WithLabelValues(r.Method, "/metrics/batch", "400").Inc()
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	accepted := 0
	for _, metric := range batchMetrics {
		if metric.DeviceID == "" {
			continue
		}

		if metric.Timestamp.IsZero() {
			metric.Timestamp = time.Now()
		}

		// Асинхронное сохранение в Redis
		go h.cache.StoreMetric(metric.DeviceID, metric.Timestamp, metric)

		// Отправляем на анализ
		h.analyzer.AddMetric(analytics.MetricData{
			DeviceID:  metric.DeviceID,
			Timestamp: metric.Timestamp,
			CPU:       metric.CPU,
			RPS:       metric.RPS,
		})

		metrics.MetricsReceived.Inc()
		accepted++
	}

	metrics.RequestsTotal.WithLabelValues(r.Method, "/metrics/batch", "200").Inc()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "accepted",
		"total":    len(batchMetrics),
		"accepted": accepted,
	})
}
