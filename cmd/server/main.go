package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"highload-final/internal/analytics"
	"highload-final/internal/cache"
	"highload-final/internal/handlers"
	"highload-final/internal/metrics"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	log.Println("Starting IoT Metrics Processing Service...")

	// Конфигурация из environment variables
	config := loadConfig()

	// Инициализация Redis
	redisCache, err := cache.NewRedisCache(
		config.RedisAddr,
		config.RedisPassword,
		config.RedisDB,
		config.MetricsRetention,
	)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisCache.Close()
	log.Println("Connected to Redis")

	// Инициализация анализатора
	analyzer := analytics.NewAnalyzer(config.WindowSize, config.AnomalyThreshold)
	analyzer.Start(4) // 4 worker goroutines
	defer analyzer.Stop()
	log.Printf("Analyzer started with window size: %d, threshold: %.2f\n",
		config.WindowSize, config.AnomalyThreshold)

	// Запускаем goroutine для обработки результатов анализа
	go processAnalysisResults(analyzer, redisCache)

	// Инициализация HTTP handlers
	handler := handlers.NewHandler(analyzer, redisCache)

	// Настройка HTTP router
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/metrics", handler.SubmitMetric)
	mux.HandleFunc("/metrics/batch", handler.BatchSubmitMetrics)
	mux.HandleFunc("/analytics", handler.GetAnalytics)
	mux.HandleFunc("/health", handler.HealthCheck)
	mux.HandleFunc("/stats", handler.GetStats)

	// Prometheus metrics endpoint
	mux.Handle("/prometheus", promhttp.Handler())

	// HTTP сервер
	server := &http.Server{
		Addr:         ":" + config.ServerPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Server listening on port %s\n", config.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Периодическое обновление метрик
	go updateMetrics(analyzer)

	// Ожидание сигнала завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// Config конфигурация приложения
type Config struct {
	ServerPort       string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	WindowSize       int
	AnomalyThreshold float64
	MetricsRetention time.Duration
}

// loadConfig загружает конфигурацию из environment
func loadConfig() Config {
	return Config{
		ServerPort:       getEnv("SERVER_PORT", "8080"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          getEnvAsInt("REDIS_DB", 0),
		WindowSize:       getEnvAsInt("WINDOW_SIZE", 50),
		AnomalyThreshold: getEnvAsFloat("ANOMALY_THRESHOLD", 2.0),
		MetricsRetention: time.Duration(getEnvAsInt("METRICS_RETENTION_HOURS", 1)) * time.Hour,
	}
}

// getEnv получает environment variable или возвращает default
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvAsInt получает environment variable как int
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	var value int
	if _, err := fmt.Sscanf(valueStr, "%d", &value); err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsFloat получает environment variable как float64
func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	var value float64
	if _, err := fmt.Sscanf(valueStr, "%f", &value); err != nil {
		return defaultValue
	}
	return value
}

// processAnalysisResults обрабатывает результаты анализа
func processAnalysisResults(analyzer *analytics.Analyzer, redisCache *cache.RedisCache) {
	resultsChan := analyzer.GetResultsChan()

	for result := range resultsChan {
		start := time.Now()

		// Обновляем Prometheus метрики
		metrics.RollingAverage.WithLabelValues(result.DeviceID, "cpu").Set(result.RollingAvgCPU)
		metrics.RollingAverage.WithLabelValues(result.DeviceID, "rps").Set(result.RollingAvgRPS)
		metrics.CurrentZScore.WithLabelValues(result.DeviceID, "combined").Set(result.AnomalyScore)

		// Сохраняем результат анализа в Redis
		go func(r analytics.AnalysisResult) {
			if err := redisCache.StoreAnalysis(r.DeviceID, r.Timestamp, r); err == nil {
				metrics.RedisOperations.WithLabelValues("store_analysis", "success").Inc()
			} else {
				metrics.RedisOperations.WithLabelValues("store_analysis", "error").Inc()
			}
		}(result)

		// Если обнаружена аномалия
		if result.IsAnomaly {
			metrics.AnomaliesDetected.WithLabelValues(result.AnomalyType, result.DeviceID).Inc()

			// Сохраняем аномалию
			go func(r analytics.AnalysisResult) {
				if err := redisCache.StoreAnomaly(r.DeviceID, r.Timestamp, r); err == nil {
					metrics.RedisOperations.WithLabelValues("store_anomaly", "success").Inc()
					log.Printf("ANOMALY DETECTED: Device=%s, Type=%s, Score=%.2f, CPU=%.2f, RPS=%.2f\n",
						r.DeviceID, r.AnomalyType, r.AnomalyScore, r.RollingAvgCPU, r.RollingAvgRPS)
				} else {
					metrics.RedisOperations.WithLabelValues("store_anomaly", "error").Inc()
				}
			}(result)
		}

		// Записываем задержку анализа
		metrics.AnalysisLatency.Observe(time.Since(start).Seconds())
	}
}

// updateMetrics периодически обновляет метрики
func updateMetrics(analyzer *analytics.Analyzer) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := analyzer.GetStats()

		if devicesTracked, ok := stats["devices_tracked"].(int); ok {
			metrics.ActiveDevices.Set(float64(devicesTracked))
		}

		if queueSize, ok := stats["queue_size"].(int); ok {
			metrics.QueueSize.Set(float64(queueSize))
		}
	}
}
