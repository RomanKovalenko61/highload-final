package analytics

import (
	"math"
	"sync"
	"time"
)

// MetricWindow хранит скользящее окно метрик
type MetricWindow struct {
	cpuValues  []float64
	rpsValues  []float64
	timestamps []time.Time
	mu         sync.RWMutex
	maxSize    int
}

// Analyzer анализатор метрик с rolling average и z-score
type Analyzer struct {
	windows          map[string]*MetricWindow
	mu               sync.RWMutex
	windowSize       int
	anomalyThreshold float64
	metricsChan      chan MetricData
	resultsChan      chan AnalysisResult
	stopChan         chan struct{}
	wg               sync.WaitGroup
}

// MetricData данные для анализа
type MetricData struct {
	DeviceID  string
	Timestamp time.Time
	CPU       float64
	RPS       float64
}

// AnalysisResult результат анализа
type AnalysisResult struct {
	DeviceID      string
	Timestamp     time.Time
	RollingAvgCPU float64
	RollingAvgRPS float64
	IsAnomaly     bool
	AnomalyScore  float64
	AnomalyType   string
	StandardDev   float64
}

// NewAnalyzer создает новый анализатор
func NewAnalyzer(windowSize int, anomalyThreshold float64) *Analyzer {
	return &Analyzer{
		windows:          make(map[string]*MetricWindow),
		windowSize:       windowSize,
		anomalyThreshold: anomalyThreshold,
		metricsChan:      make(chan MetricData, 1000),
		resultsChan:      make(chan AnalysisResult, 1000),
		stopChan:         make(chan struct{}),
	}
}

// Start запускает обработчики в goroutines
func (a *Analyzer) Start(workers int) {
	for i := 0; i < workers; i++ {
		a.wg.Add(1)
		go a.processMetrics()
	}
}

// Stop останавливает анализатор
func (a *Analyzer) Stop() {
	close(a.stopChan)
	a.wg.Wait()
	close(a.metricsChan)
	close(a.resultsChan)
}

// AddMetric добавляет метрику для анализа
func (a *Analyzer) AddMetric(data MetricData) {
	select {
	case a.metricsChan <- data:
	default:
		// Если канал полон, пропускаем метрику
	}
}

// GetResultsChan возвращает канал с результатами
func (a *Analyzer) GetResultsChan() <-chan AnalysisResult {
	return a.resultsChan
}

// processMetrics обрабатывает метрики из канала
func (a *Analyzer) processMetrics() {
	defer a.wg.Done()

	for {
		select {
		case <-a.stopChan:
			return
		case data := <-a.metricsChan:
			result := a.analyze(data)
			select {
			case a.resultsChan <- result:
			default:
				// Канал результатов полон
			}
		}
	}
}

// analyze выполняет анализ метрики
func (a *Analyzer) analyze(data MetricData) AnalysisResult {
	a.mu.Lock()
	window, exists := a.windows[data.DeviceID]
	if !exists {
		window = &MetricWindow{
			cpuValues:  make([]float64, 0, a.windowSize),
			rpsValues:  make([]float64, 0, a.windowSize),
			timestamps: make([]time.Time, 0, a.windowSize),
			maxSize:    a.windowSize,
		}
		a.windows[data.DeviceID] = window
	}
	a.mu.Unlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	// Добавляем новые значения
	window.cpuValues = append(window.cpuValues, data.CPU)
	window.rpsValues = append(window.rpsValues, data.RPS)
	window.timestamps = append(window.timestamps, data.Timestamp)

	// Ограничиваем размер окна
	if len(window.cpuValues) > window.maxSize {
		window.cpuValues = window.cpuValues[1:]
		window.rpsValues = window.rpsValues[1:]
		window.timestamps = window.timestamps[1:]
	}

	// Вычисляем rolling average
	avgCPU := calculateAverage(window.cpuValues)
	avgRPS := calculateAverage(window.rpsValues)

	// Вычисляем стандартное отклонение
	stdDevCPU := calculateStdDev(window.cpuValues, avgCPU)
	stdDevRPS := calculateStdDev(window.rpsValues, avgRPS)

	// Вычисляем z-score для текущих значений
	var zScoreCPU, zScoreRPS float64
	if stdDevCPU > 0 {
		zScoreCPU = (data.CPU - avgCPU) / stdDevCPU
	}
	if stdDevRPS > 0 {
		zScoreRPS = (data.RPS - avgRPS) / stdDevRPS
	}

	// Определяем аномалию
	isAnomaly := false
	anomalyType := ""
	maxZScore := math.Max(math.Abs(zScoreCPU), math.Abs(zScoreRPS))

	if math.Abs(zScoreCPU) > a.anomalyThreshold {
		isAnomaly = true
		if zScoreCPU > 0 {
			anomalyType = "CPU_SPIKE"
		} else {
			anomalyType = "CPU_DROP"
		}
	}

	if math.Abs(zScoreRPS) > a.anomalyThreshold {
		isAnomaly = true
		if anomalyType != "" {
			anomalyType = "MULTIPLE_ANOMALY"
		} else if zScoreRPS > 0 {
			anomalyType = "RPS_SPIKE"
		} else {
			anomalyType = "RPS_DROP"
		}
	}

	return AnalysisResult{
		DeviceID:      data.DeviceID,
		Timestamp:     data.Timestamp,
		RollingAvgCPU: avgCPU,
		RollingAvgRPS: avgRPS,
		IsAnomaly:     isAnomaly,
		AnomalyScore:  maxZScore,
		AnomalyType:   anomalyType,
		StandardDev:   math.Max(stdDevCPU, stdDevRPS),
	}
}

// calculateAverage вычисляет среднее значение
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateStdDev вычисляет стандартное отклонение
func calculateStdDev(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}

	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	return math.Sqrt(variance)
}

// GetStats возвращает статистику анализатора
func (a *Analyzer) GetStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"devices_tracked": len(a.windows),
		"window_size":     a.windowSize,
		"threshold":       a.anomalyThreshold,
		"queue_size":      len(a.metricsChan),
	}
}
