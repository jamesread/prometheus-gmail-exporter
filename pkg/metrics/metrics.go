package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Registry manages Gmail Prometheus gauges with names matching the Python exporter.
type Registry struct {
	mu         sync.Mutex
	prom       *prometheus.Registry
	gauges     map[string]prometheus.Gauge
	vectors    map[string]*prometheus.GaugeVec
}

func NewRegistry() *Registry {
	return &Registry{
		prom:    prometheus.NewRegistry(),
		gauges:  make(map[string]prometheus.Gauge),
		vectors: make(map[string]*prometheus.GaugeVec),
	}
}

func (r *Registry) PrometheusRegistry() *prometheus.Registry {
	return r.prom
}

func (r *Registry) gaugeName(name string) string {
	return "gmail_" + name
}

func (r *Registry) LabelTotal(labelID, labelName string) prometheus.Gauge {
	return r.getGauge(labelID+"_total", labelName+" Total")
}

func (r *Registry) LabelUnread(labelID, labelName string) prometheus.Gauge {
	return r.getGauge(labelID+"_unread", labelName+" Unread")
}

func (r *Registry) LabelSender(labelID string) *prometheus.GaugeVec {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := labelID + "_sender"
	if vec, ok := r.vectors[key]; ok {
		return vec
	}

	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: r.gaugeName(key),
		Help: "Label sender info",
	}, []string{"sender"})
	r.prom.MustRegister(vec)
	r.vectors[key] = vec
	return vec
}

func (r *Registry) CustomQuery(name string) prometheus.Gauge {
	return r.getGauge(name, name)
}

func (r *Registry) getGauge(name, help string) prometheus.Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()

	if gauge, ok := r.gauges[name]; ok {
		return gauge
	}

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: r.gaugeName(name),
		Help: help,
	})
	r.prom.MustRegister(gauge)
	r.gauges[name] = gauge
	return gauge
}

// MetricName returns the fully-qualified Prometheus metric name for tests.
func MetricName(shortName string) string {
	return "gmail_" + shortName
}
