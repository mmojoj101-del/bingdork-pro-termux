// Package metrics provides instrumentation and statistics for BingDork Pro.
package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
)

// Collector collects and exposes application metrics.
type Collector struct {
	mu     sync.RWMutex
	log    *logger.Logger
	cfg    *core.MetricsConfig

	// Counters
	queriesTotal      int64
	queriesSuccess    int64
	queriesFailed     int64
	resultsTotal      int64
	resultsFiltered   int64
	resultsDeduped    int64
	uniqueDomains     int64
	cacheHits         int64
	cacheMisses       int64
	retriesTotal      int64
	captchaDetected   int64
	captchaSolved     int64
	captchaFailed     int64

	// Timings
	avgResponseTime   int64 // nanoseconds
	totalResponseTime int64
	responseCount     int64

	// Provider-specific stats
	providerStats map[string]*ProviderStats

	// Application
	startTime       time.Time
	lastErrorTime   time.Time
	lastError       string

	// Listeners for real-time updates
	listeners []MetricsListener
}

// ProviderStats holds per-provider metrics.
type ProviderStats struct {
	Queries      int64
	Success      int64
	Failed       int64
	Results      int64
	AvgLatency   time.Duration
	TotalLatency time.Duration
	Errors       int64
	RateLimited  int64
}

// MetricsSnapshot is a point-in-time snapshot of all metrics.
type MetricsSnapshot struct {
	QueriesTotal      int64            `json:"queries_total"`
	QueriesSuccess    int64            `json:"queries_success"`
	QueriesFailed     int64            `json:"queries_failed"`
	ResultsTotal      int64            `json:"results_total"`
	ResultsFiltered   int64            `json:"results_filtered"`
	ResultsDeduped    int64            `json:"results_deduped"`
	UniqueDomains     int64            `json:"unique_domains"`
	CacheHits         int64            `json:"cache_hits"`
	CacheMisses       int64            `json:"cache_misses"`
	RetriesTotal      int64            `json:"retries_total"`
	CAPTCHADetected   int64            `json:"captcha_detected"`
	CAPTCHASolved     int64            `json:"captcha_solved"`
	CAPTCHAFailed     int64            `json:"captcha_failed"`
	AvgResponseTime   time.Duration    `json:"avg_response_time"`
	Uptime            time.Duration    `json:"uptime"`
	ProviderStats     map[string]*ProviderStats `json:"provider_stats"`
}

// MetricsListener receives real-time metric updates.
type MetricsListener interface {
	OnMetricsUpdate(snapshot *MetricsSnapshot)
}

// NewCollector creates a new metrics collector.
func NewCollector(cfg *core.MetricsConfig, log *logger.Logger) *Collector {
	return &Collector{
		log:           log,
		cfg:           cfg,
		startTime:     time.Now(),
		providerStats: make(map[string]*ProviderStats),
	}
}

// RecordQuery records a search query execution.
func (mc *Collector) RecordQuery(provider string, success bool, duration time.Duration, results int) {
	atomic.AddInt64(&mc.queriesTotal, 1)
	if success {
		atomic.AddInt64(&mc.queriesSuccess, 1)
	} else {
		atomic.AddInt64(&mc.queriesFailed, 1)
	}
	atomic.AddInt64(&mc.resultsTotal, int64(results))

	// Response time
	atomic.AddInt64(&mc.totalResponseTime, duration.Nanoseconds())
	atomic.AddInt64(&mc.responseCount, 1)

	// Provider stats
	mc.mu.Lock()
	ps, ok := mc.providerStats[provider]
	if !ok {
		ps = &ProviderStats{}
		mc.providerStats[provider] = ps
	}
	ps.Queries++
	ps.TotalLatency += duration
	ps.AvgLatency = ps.TotalLatency / time.Duration(ps.Queries)
	if success {
		ps.Success++
		ps.Results += int64(results)
	} else {
		ps.Failed++
	}
	mc.mu.Unlock()

	mc.notifyListeners()
}

// RecordResultFiltered records a filtered result.
func (mc *Collector) RecordResultFiltered() {
	atomic.AddInt64(&mc.resultsFiltered, 1)
}

// RecordResultDeduped records a deduplicated result.
func (mc *Collector) RecordResultDeduped() {
	atomic.AddInt64(&mc.resultsDeduped, 1)
}

// RecordUniqueDomain records a unique domain.
func (mc *Collector) RecordUniqueDomain() {
	atomic.AddInt64(&mc.uniqueDomains, 1)
}

// RecordCacheHit records a cache hit.
func (mc *Collector) RecordCacheHit() {
	atomic.AddInt64(&mc.cacheHits, 1)
}

// RecordCacheMiss records a cache miss.
func (mc *Collector) RecordCacheMiss() {
	atomic.AddInt64(&mc.cacheMisses, 1)
}

// RecordRetry records a retry attempt.
func (mc *Collector) RecordRetry() {
	atomic.AddInt64(&mc.retriesTotal, 1)
}

// RecordCAPTCHA records a CAPTCHA event.
func (mc *Collector) RecordCAPTCHA(detected bool, solved bool) {
	if detected {
		atomic.AddInt64(&mc.captchaDetected, 1)
	}
	if solved {
		atomic.AddInt64(&mc.captchaSolved, 1)
	}
	if detected && !solved {
		atomic.AddInt64(&mc.captchaFailed, 1)
	}
}

// RecordError records an application error.
func (mc *Collector) RecordError(err error) {
	mc.mu.Lock()
	mc.lastErrorTime = time.Now()
	mc.lastError = err.Error()
	mc.mu.Unlock()
}

// RecordRateLimited records a rate-limited event for a provider.
func (mc *Collector) RecordRateLimited(provider string) {
	mc.mu.Lock()
	if ps, ok := mc.providerStats[provider]; ok {
		ps.RateLimited++
	}
	mc.mu.Unlock()
}

// Snapshot returns a point-in-time metrics snapshot.
func (mc *Collector) Snapshot() *MetricsSnapshot {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	avgNs := int64(0)
	if mc.responseCount > 0 {
		avgNs = mc.totalResponseTime / mc.responseCount
	}

	providerCopy := make(map[string]*ProviderStats, len(mc.providerStats))
	for k, v := range mc.providerStats {
		providerCopy[k] = &ProviderStats{
			Queries:      v.Queries,
			Success:      v.Success,
			Failed:       v.Failed,
			Results:      v.Results,
			AvgLatency:   v.AvgLatency,
			TotalLatency: v.TotalLatency,
			Errors:       v.Errors,
			RateLimited:  v.RateLimited,
		}
	}

	return &MetricsSnapshot{
		QueriesTotal:     atomic.LoadInt64(&mc.queriesTotal),
		QueriesSuccess:   atomic.LoadInt64(&mc.queriesSuccess),
		QueriesFailed:    atomic.LoadInt64(&mc.queriesFailed),
		ResultsTotal:     atomic.LoadInt64(&mc.resultsTotal),
		ResultsFiltered:  atomic.LoadInt64(&mc.resultsFiltered),
		ResultsDeduped:   atomic.LoadInt64(&mc.resultsDeduped),
		UniqueDomains:    atomic.LoadInt64(&mc.uniqueDomains),
		CacheHits:        atomic.LoadInt64(&mc.cacheHits),
		CacheMisses:      atomic.LoadInt64(&mc.cacheMisses),
		RetriesTotal:     atomic.LoadInt64(&mc.retriesTotal),
		CAPTCHADetected:  atomic.LoadInt64(&mc.captchaDetected),
		CAPTCHASolved:    atomic.LoadInt64(&mc.captchaSolved),
		CAPTCHAFailed:    atomic.LoadInt64(&mc.captchaFailed),
		AvgResponseTime:  time.Duration(avgNs),
		Uptime:           time.Since(mc.startTime),
		ProviderStats:    providerCopy,
	}
}

// Reset clears all metrics.
func (mc *Collector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	atomic.StoreInt64(&mc.queriesTotal, 0)
	atomic.StoreInt64(&mc.queriesSuccess, 0)
	atomic.StoreInt64(&mc.queriesFailed, 0)
	atomic.StoreInt64(&mc.resultsTotal, 0)
	atomic.StoreInt64(&mc.resultsFiltered, 0)
	atomic.StoreInt64(&mc.resultsDeduped, 0)
	atomic.StoreInt64(&mc.uniqueDomains, 0)
	atomic.StoreInt64(&mc.cacheHits, 0)
	atomic.StoreInt64(&mc.cacheMisses, 0)
	atomic.StoreInt64(&mc.retriesTotal, 0)
	atomic.StoreInt64(&mc.captchaDetected, 0)
	atomic.StoreInt64(&mc.captchaSolved, 0)
	atomic.StoreInt64(&mc.captchaFailed, 0)
	atomic.StoreInt64(&mc.totalResponseTime, 0)
	atomic.StoreInt64(&mc.responseCount, 0)

	mc.providerStats = make(map[string]*ProviderStats)
	mc.startTime = time.Now()
}

// RegisterListener adds a metrics listener.
func (mc *Collector) RegisterListener(listener MetricsListener) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.listeners = append(mc.listeners, listener)
}

// notifyListeners pushes updates to all registered listeners.
func (mc *Collector) notifyListeners() {
	mc.mu.RLock()
	listeners := make([]MetricsListener, len(mc.listeners))
	copy(listeners, mc.listeners)
	mc.mu.RUnlock()

	if len(listeners) == 0 {
		return
	}

	snapshot := mc.Snapshot()
	for _, l := range listeners {
		l.OnMetricsUpdate(snapshot)
	}
}

// PrometheusExporter exports metrics in Prometheus format.
type PrometheusExporter struct {
	collector *Collector
	log       *logger.Logger
	cfg       *core.MetricsConfig
}

// NewPrometheusExporter creates a new Prometheus metrics exporter.
func NewPrometheusExporter(collector *Collector, cfg *core.MetricsConfig, log *logger.Logger) *PrometheusExporter {
	return &PrometheusExporter{
		collector: collector,
		log:       log,
		cfg:       cfg,
	}
}

// Export returns metrics in Prometheus text format.
func (pe *PrometheusExporter) Export() string {
	snapshot := pe.collector.Snapshot()
	return formatPrometheus(snapshot)
}

// formatPrometheus converts a snapshot to Prometheus exposition format.
func formatPrometheus(s *MetricsSnapshot) string {
	sb := stringsBuilderPool.Get().(*stringsBuilderWrapper)
	sb.Reset()
	defer stringsBuilderPool.Put(sb)

	writeMetric(sb, "# HELP bingdork_queries_total Total number of search queries executed")
	writeMetric(sb, "# TYPE bingdork_queries_total counter")
	writeInt(sb, "bingdork_queries_total", s.QueriesTotal)

	writeMetric(sb, "# HELP bingdork_queries_success Total successful queries")
	writeMetric(sb, "# TYPE bingdork_queries_success counter")
	writeInt(sb, "bingdork_queries_success", s.QueriesSuccess)

	writeMetric(sb, "# HELP bingdork_queries_failed Total failed queries")
	writeMetric(sb, "# TYPE bingdork_queries_failed counter")
	writeInt(sb, "bingdork_queries_failed", s.QueriesFailed)

	writeMetric(sb, "# HELP bingdork_results_total Total results collected")
	writeMetric(sb, "# TYPE bingdork_results_total counter")
	writeInt(sb, "bingdork_results_total", s.ResultsTotal)

	writeMetric(sb, "# HELP bingdork_cache_hits Total cache hits")
	writeMetric(sb, "# TYPE bingdork_cache_hits counter")
	writeInt(sb, "bingdork_cache_hits", s.CacheHits)

	writeMetric(sb, "# HELP bingdork_cache_misses Total cache misses")
	writeMetric(sb, "# TYPE bingdork_cache_misses counter")
	writeInt(sb, "bingdork_cache_misses", s.CacheMisses)

	writeMetric(sb, "# HELP bingdork_avg_response_time Average response time in seconds")
	writeMetric(sb, "# TYPE bingdork_avg_response_time gauge")
	writeFloat(sb, "bingdork_avg_response_time", s.AvgResponseTime.Seconds())

	writeMetric(sb, "# HELP bingdork_uptime_seconds Application uptime")
	writeMetric(sb, "# TYPE bingdork_uptime_seconds counter")
	writeFloat(sb, "bingdork_uptime_seconds", s.Uptime.Seconds())

	writeMetric(sb, "# HELP bingdork_captcha_detected CAPTCHA challenges detected")
	writeMetric(sb, "# TYPE bingdork_captcha_detected counter")
	writeInt(sb, "bingdork_captcha_detected", s.CAPTCHADetected)

	writeMetric(sb, "# HELP bingdork_captcha_solved CAPTCHA challenges solved")
	writeMetric(sb, "# TYPE bingdork_captcha_solved counter")
	writeInt(sb, "bingdork_captcha_solved", s.CAPTCHASolved)

	writeMetric(sb, "# HELP bingdork_retries_total Total retry attempts")
	writeMetric(sb, "# TYPE bingdork_retries_total counter")
	writeInt(sb, "bingdork_retries_total", s.RetriesTotal)

	// Provider-specific metrics
	for provider, ps := range s.ProviderStats {
		writeMetric(sb, fmt.Sprintf("# HELP bingdork_provider_queries_%s Queries for provider", provider))
		writeMetric(sb, fmt.Sprintf("# TYPE bingdork_provider_queries_%s counter", provider))
		writeIntWithLabels(sb, "bingdork_provider_queries", ps.Queries, "provider", provider)
	}

	return sb.String()
}

func writeMetric(sb *stringsBuilderWrapper, line string) {
	sb.WriteString(line)
	sb.WriteByte('\n')
}

func writeInt(sb *stringsBuilderWrapper, name string, value int64) {
	sb.WriteString(name)
	sb.WriteByte(' ')
	sb.writeInt64(value)
	sb.WriteByte('\n')
}

func writeFloat(sb *stringsBuilderWrapper, name string, value float64) {
	sb.WriteString(name)
	sb.WriteByte(' ')
	sb.writeFloat64(value)
	sb.WriteByte('\n')
}

func writeIntWithLabels(sb *stringsBuilderWrapper, name string, value int64, labelKey, labelValue string) {
	sb.WriteString(name)
	sb.WriteByte('{')
	sb.WriteString(labelKey)
	sb.WriteString(`="`)
	sb.WriteString(labelValue)
	sb.WriteString(`"}`)
	sb.WriteByte(' ')
	sb.writeInt64(value)
	sb.WriteByte('\n')
}

// stringsBuilderWrapper wraps strings.Builder with number formatting.
type stringsBuilderWrapper struct {
	stringsBuilder interface{} // placeholder, use strings.Builder directly in real code
	buf           []byte
}

func (w *stringsBuilderWrapper) WriteString(s string) {
	w.buf = append(w.buf, s...)
}

func (w *stringsBuilderWrapper) WriteByte(b byte) {
	w.buf = append(w.buf, b)
}

func (w *stringsBuilderWrapper) writeInt64(v int64) {
	w.buf = append(w.buf, fmt.Sprintf("%d", v)...)
}

func (w *stringsBuilderWrapper) writeFloat64(v float64) {
	w.buf = append(w.buf, fmt.Sprintf("%f", v)...)
}

func (w *stringsBuilderWrapper) Reset() {
	w.buf = w.buf[:0]
}

func (w *stringsBuilderWrapper) String() string {
	return string(w.buf)
}

var stringsBuilderPool = sync.Pool{
	New: func() interface{} {
		return &stringsBuilderWrapper{}
	},
}
