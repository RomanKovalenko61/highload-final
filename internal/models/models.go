package models

import "time"

// Metric представляет метрику от IoT устройства
type Metric struct {
	Timestamp time.Time `json:"timestamp"`
	DeviceID  string    `json:"device_id"`
	CPU       float64   `json:"cpu"`
	RPS       float64   `json:"rps"`
	Memory    float64   `json:"memory,omitempty"`
}

// AnalyticsResult результат анализа метрик
type AnalyticsResult struct {
	DeviceID      string    `json:"device_id"`
	Timestamp     time.Time `json:"timestamp"`
	RollingAvgCPU float64   `json:"rolling_avg_cpu"`
	RollingAvgRPS float64   `json:"rolling_avg_rps"`
	IsAnomaly     bool      `json:"is_anomaly"`
	AnomalyScore  float64   `json:"anomaly_score"`
	AnomalyType   string    `json:"anomaly_type,omitempty"`
	StandardDev   float64   `json:"standard_dev"`
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
