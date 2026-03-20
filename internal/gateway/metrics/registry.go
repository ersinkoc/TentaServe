package metrics

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

// MetricType represents the type of metric.
type MetricType string

const (
	// Counter is a monotonically increasing value.
	Counter MetricType = "counter"
	// Gauge is a value that can go up and down.
	Gauge MetricType = "gauge"
	// Histogram samples observations into buckets.
	Histogram MetricType = "histogram"
)

// Metric is a single metric.
type Metric struct {
	Name   string
	Help   string
	Type   MetricType
	Labels []string
}

// Registry holds all metrics.
type Registry struct {
	mu         sync.RWMutex
	metrics    map[string]*Metric
	counters   map[string]*CounterVec
	gauges     map[string]*GaugeVec
	histograms map[string]*HistogramVec
}

// NewRegistry creates a new metrics registry.
func NewRegistry() *Registry {
	return &Registry{
		metrics:    make(map[string]*Metric),
		counters:   make(map[string]*CounterVec),
		gauges:     make(map[string]*GaugeVec),
		histograms: make(map[string]*HistogramVec),
	}
}

// RegisterCounter registers a counter metric.
func (r *Registry) RegisterCounter(name, help string, labels ...string) *CounterVec {
	r.mu.Lock()
	defer r.mu.Unlock()

	m := &Metric{
		Name:   name,
		Help:   help,
		Type:   Counter,
		Labels: labels,
	}
	r.metrics[name] = m

	cv := &CounterVec{
		metric: m,
		values: make(map[string]*CounterValue),
	}
	r.counters[name] = cv
	return cv
}

// RegisterGauge registers a gauge metric.
func (r *Registry) RegisterGauge(name, help string, labels ...string) *GaugeVec {
	r.mu.Lock()
	defer r.mu.Unlock()

	m := &Metric{
		Name:   name,
		Help:   help,
		Type:   Gauge,
		Labels: labels,
	}
	r.metrics[name] = m

	gv := &GaugeVec{
		metric: m,
		values: make(map[string]*GaugeValue),
	}
	r.gauges[name] = gv
	return gv
}

// RegisterHistogram registers a histogram metric.
func (r *Registry) RegisterHistogram(name, help string, buckets []float64, labels ...string) *HistogramVec {
	r.mu.Lock()
	defer r.mu.Unlock()

	m := &Metric{
		Name:   name,
		Help:   help,
		Type:   Histogram,
		Labels: labels,
	}
	r.metrics[name] = m

	hv := &HistogramVec{
		metric:  m,
		buckets: buckets,
		values:  make(map[string]*HistogramValue),
	}
	r.histograms[name] = hv
	return hv
}

// Get retrieves a metric by name.
func (r *Registry) Get(name string) *Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.metrics[name]
}

// Handler returns an HTTP handler for Prometheus exposition.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		r.WriteMetrics(w)
	})
}

// WriteMetrics writes metrics in Prometheus format.
func (r *Registry) WriteMetrics(w io.Writer) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Write counters
	for _, cv := range r.counters {
		cv.writeTo(w)
	}

	// Write gauges
	for _, gv := range r.gauges {
		gv.writeTo(w)
	}

	// Write histograms
	for _, hv := range r.histograms {
		hv.writeTo(w)
	}
}

// CounterVec is a collection of counters with labels.
type CounterVec struct {
	metric *Metric
	mu     sync.RWMutex
	values map[string]*CounterValue
}

// With returns a counter for the given label values.
func (cv *CounterVec) With(labelValues ...string) *CounterValue {
	key := labelsKey(cv.metric.Labels, labelValues)

	cv.mu.RLock()
	v, exists := cv.values[key]
	cv.mu.RUnlock()

	if exists {
		return v
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()

	// Double-check
	if v, exists = cv.values[key]; exists {
		return v
	}

	v = &CounterValue{
		labels:      cv.metric.Labels,
		labelValues: labelValues,
	}
	cv.values[key] = v
	return v
}

// Inc increments the counter by 1.
func (cv *CounterVec) Inc(labelValues ...string) {
	cv.With(labelValues...).Inc()
}

// Add adds a value to the counter.
func (cv *CounterVec) Add(value float64, labelValues ...string) {
	cv.With(labelValues...).Add(value)
}

func (cv *CounterVec) writeTo(w io.Writer) {
	fmt.Fprintf(w, "# HELP %s %s\n", cv.metric.Name, cv.metric.Help)
	fmt.Fprintf(w, "# TYPE %s counter\n", cv.metric.Name)

	cv.mu.RLock()
	defer cv.mu.RUnlock()

	for _, v := range cv.values {
		v.writeTo(w, cv.metric.Name)
	}
}

// CounterValue is a single counter value.
type CounterValue struct {
	labels      []string
	labelValues []string
	value       atomic.Uint64
}

// Inc increments by 1.
func (cv *CounterValue) Inc() {
	cv.value.Add(1)
}

// Add adds a value.
func (cv *CounterValue) Add(delta float64) {
	cv.value.Add(uint64(delta))
}

// Value returns the current value.
func (cv *CounterValue) Value() uint64 {
	return cv.value.Load()
}

func (cv *CounterValue) writeTo(w io.Writer, name string) {
	labels := formatLabels(cv.labels, cv.labelValues)
	fmt.Fprintf(w, "%s%s %d\n", name, labels, cv.value.Load())
}

// GaugeVec is a collection of gauges with labels.
type GaugeVec struct {
	metric *Metric
	mu     sync.RWMutex
	values map[string]*GaugeValue
}

// With returns a gauge for the given label values.
func (gv *GaugeVec) With(labelValues ...string) *GaugeValue {
	key := labelsKey(gv.metric.Labels, labelValues)

	gv.mu.RLock()
	v, exists := gv.values[key]
	gv.mu.RUnlock()

	if exists {
		return v
	}

	gv.mu.Lock()
	defer gv.mu.Unlock()

	if v, exists = gv.values[key]; exists {
		return v
	}

	v = &GaugeValue{
		labels:      gv.metric.Labels,
		labelValues: labelValues,
	}
	gv.values[key] = v
	return v
}

// Set sets the gauge value.
func (gv *GaugeVec) Set(value float64, labelValues ...string) {
	gv.With(labelValues...).Set(value)
}

// Inc increments the gauge by 1.
func (gv *GaugeVec) Inc(labelValues ...string) {
	gv.With(labelValues...).Inc()
}

// Dec decrements the gauge by 1.
func (gv *GaugeVec) Dec(labelValues ...string) {
	gv.With(labelValues...).Dec()
}

// Add adds a value to the gauge.
func (gv *GaugeVec) Add(value float64, labelValues ...string) {
	gv.With(labelValues...).Add(value)
}

func (gv *GaugeVec) writeTo(w io.Writer) {
	fmt.Fprintf(w, "# HELP %s %s\n", gv.metric.Name, gv.metric.Help)
	fmt.Fprintf(w, "# TYPE %s gauge\n", gv.metric.Name)

	gv.mu.RLock()
	defer gv.mu.RUnlock()

	for _, v := range gv.values {
		v.writeTo(w, gv.metric.Name)
	}
}

// GaugeValue is a single gauge value.
type GaugeValue struct {
	labels      []string
	labelValues []string
	value       atomic.Int64 // Store as int64 for atomic ops, treat as float64
}

// Set sets the value.
func (gv *GaugeValue) Set(value float64) {
	gv.value.Store(int64(value * 1000)) // Store with 3 decimal precision
}

// Inc increments by 1.
func (gv *GaugeValue) Inc() {
	gv.value.Add(1000)
}

// Dec decrements by 1.
func (gv *GaugeValue) Dec() {
	gv.value.Add(-1000)
}

// Add adds a value.
func (gv *GaugeValue) Add(delta float64) {
	gv.value.Add(int64(delta * 1000))
}

// Value returns the current value.
func (gv *GaugeValue) Value() float64 {
	return float64(gv.value.Load()) / 1000
}

func (gv *GaugeValue) writeTo(w io.Writer, name string) {
	labels := formatLabels(gv.labels, gv.labelValues)
	fmt.Fprintf(w, "%s%s %.3f\n", name, labels, gv.Value())
}

// HistogramVec is a collection of histograms with labels.
type HistogramVec struct {
	metric  *Metric
	buckets []float64
	mu      sync.RWMutex
	values  map[string]*HistogramValue
}

// With returns a histogram for the given label values.
func (hv *HistogramVec) With(labelValues ...string) *HistogramValue {
	key := labelsKey(hv.metric.Labels, labelValues)

	hv.mu.RLock()
	v, exists := hv.values[key]
	hv.mu.RUnlock()

	if exists {
		return v
	}

	hv.mu.Lock()
	defer hv.mu.Unlock()

	if v, exists = hv.values[key]; exists {
		return v
	}

	v = newHistogramValue(hv.buckets)
	hv.values[key] = v
	return v
}

// Observe records a value.
func (hv *HistogramVec) Observe(value float64, labelValues ...string) {
	hv.With(labelValues...).Observe(value)
}

func (hv *HistogramVec) writeTo(w io.Writer) {
	fmt.Fprintf(w, "# HELP %s %s\n", hv.metric.Name, hv.metric.Help)
	fmt.Fprintf(w, "# TYPE %s histogram\n", hv.metric.Name)

	hv.mu.RLock()
	defer hv.mu.RUnlock()

	for _, v := range hv.values {
		v.writeTo(w, hv.metric.Name)
	}
}

// HistogramValue is a single histogram value.
type HistogramValue struct {
	buckets []float64
	counts  []atomic.Uint64
	sum     atomic.Uint64
	count   atomic.Uint64
}

func newHistogramValue(buckets []float64) *HistogramValue {
	return &HistogramValue{
		buckets: buckets,
		counts:  make([]atomic.Uint64, len(buckets)),
	}
}

// Observe records a value.
func (hv *HistogramValue) Observe(value float64) {
	// Increment count
	hv.count.Add(1)
	hv.sum.Add(uint64(value * 1000))

	// Increment buckets
	for i, bucket := range hv.buckets {
		if value <= bucket {
			hv.counts[i].Add(1)
		}
	}
}

func (hv *HistogramValue) writeTo(w io.Writer, name string) {
	// Write buckets
	for i, bucket := range hv.buckets {
		bucketName := fmt.Sprintf("%s_bucket{le=\"%.3f\"}", name, bucket)
		fmt.Fprintf(w, "%s %d\n", bucketName, hv.counts[i].Load())
	}

	// Write +Inf bucket
	fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n", name, hv.count.Load())

	// Write sum and count
	fmt.Fprintf(w, "%s_sum %.3f\n", name, float64(hv.sum.Load())/1000)
	fmt.Fprintf(w, "%s_count %d\n", name, hv.count.Load())
}

// DefaultBuckets are standard histogram buckets (in seconds).
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// labelsKey creates a key from label names and values.
func labelsKey(names, values []string) string {
	if len(values) == 0 {
		return ""
	}
	key := ""
	for i, v := range values {
		if i > 0 {
			key += "\x00"
		}
		key += v
	}
	return key
}

// formatLabels formats labels for Prometheus output.
func formatLabels(names, values []string) string {
	if len(names) == 0 || len(values) == 0 {
		return ""
	}
	result := "{"
	for i, name := range names {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%s=\"%s\"", name, escapeLabel(values[i]))
	}
	result += "}"
	return result
}

// escapeLabel escapes special characters in label values.
func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return value
}
