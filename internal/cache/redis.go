package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache обертка для Redis клиента
type RedisCache struct {
	client *redis.Client
	ctx    context.Context
	ttl    time.Duration
}

// NewRedisCache создает новый Redis кэш
func NewRedisCache(addr, password string, db int, ttl time.Duration) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     100,
		MinIdleConns: 10,
		MaxRetries:   3,
	})

	ctx := context.Background()

	// Проверяем подключение
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: client,
		ctx:    ctx,
		ttl:    ttl,
	}, nil
}

// StoreMetric сохраняет метрику в Redis
func (r *RedisCache) StoreMetric(deviceID string, timestamp time.Time, data interface{}) error {
	key := fmt.Sprintf("metric:%s:%d", deviceID, timestamp.Unix())

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}

	return r.client.Set(r.ctx, key, jsonData, r.ttl).Err()
}

// StoreAnalysis сохраняет результат анализа
func (r *RedisCache) StoreAnalysis(deviceID string, timestamp time.Time, data interface{}) error {
	key := fmt.Sprintf("analysis:%s:%d", deviceID, timestamp.Unix())

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal analysis: %w", err)
	}

	return r.client.Set(r.ctx, key, jsonData, r.ttl).Err()
}

// StoreAnomaly сохраняет аномалию (с более длительным TTL)
func (r *RedisCache) StoreAnomaly(deviceID string, timestamp time.Time, data interface{}) error {
	key := fmt.Sprintf("anomaly:%s:%d", deviceID, timestamp.Unix())

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal anomaly: %w", err)
	}

	// Аномалии хранятся дольше
	anomalyTTL := r.ttl * 24 // 24 часа если базовый TTL = 1 час

	// Добавляем в sorted set для легкого извлечения
	score := float64(timestamp.Unix())
	listKey := fmt.Sprintf("anomaly_list:%s", deviceID)

	pipe := r.client.Pipeline()
	pipe.Set(r.ctx, key, jsonData, anomalyTTL)
	pipe.ZAdd(r.ctx, listKey, redis.Z{Score: score, Member: key})
	pipe.Expire(r.ctx, listKey, anomalyTTL)

	_, err = pipe.Exec(r.ctx)
	return err
}

// GetRecentMetrics получает последние N метрик для устройства
func (r *RedisCache) GetRecentMetrics(deviceID string, limit int) ([]string, error) {
	pattern := fmt.Sprintf("metric:%s:*", deviceID)

	var keys []string
	iter := r.client.Scan(r.ctx, 0, pattern, int64(limit)).Iterator()

	for iter.Next(r.ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan metrics: %w", err)
	}

	return keys, nil
}

// GetRecentAnomalies получает последние аномалии для устройства
func (r *RedisCache) GetRecentAnomalies(deviceID string, limit int) ([]string, error) {
	listKey := fmt.Sprintf("anomaly_list:%s", deviceID)

	// Получаем последние аномалии из sorted set
	results, err := r.client.ZRevRange(r.ctx, listKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get anomalies: %w", err)
	}

	return results, nil
}

// IncrementCounter увеличивает счетчик
func (r *RedisCache) IncrementCounter(key string) error {
	return r.client.Incr(r.ctx, key).Err()
}

// GetCounter получает значение счетчика
func (r *RedisCache) GetCounter(key string) (int64, error) {
	val, err := r.client.Get(r.ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// Close закрывает соединение с Redis
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// Ping проверяет доступность Redis
func (r *RedisCache) Ping() error {
	return r.client.Ping(r.ctx).Err()
}

// GetStats возвращает статистику Redis
func (r *RedisCache) GetStats() map[string]interface{} {
	stats := r.client.PoolStats()

	return map[string]interface{}{
		"hits":        stats.Hits,
		"misses":      stats.Misses,
		"timeouts":    stats.Timeouts,
		"total_conns": stats.TotalConns,
		"idle_conns":  stats.IdleConns,
		"stale_conns": stats.StaleConns,
	}
}
