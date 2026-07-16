package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Registry manages Gmail Prometheus gauges using labeled metric names.
type Registry struct {
	mu sync.Mutex
	prom *prometheus.Registry

	labelTotal   *prometheus.GaugeVec
	labelUnread  *prometheus.GaugeVec
	labelSender  *prometheus.GaugeVec
	customQuery  *prometheus.GaugeVec
}

func NewRegistry() *Registry {
	r := &Registry{
		prom: prometheus.NewRegistry(),
		labelTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gmail_label_total",
			Help: "Total threads for a Gmail label",
		}, []string{"id", "name"}),
		labelUnread: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gmail_label_unread",
			Help: "Unread threads for a Gmail label",
		}, []string{"id", "name"}),
		labelSender: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gmail_label_sender",
			Help: "Unread threads for a Gmail label by sender",
		}, []string{"id", "name", "sender"}),
		customQuery: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gmail_custom_query",
			Help: "Result size estimate for a configured Gmail search query",
		}, []string{"name"}),
	}

	r.prom.MustRegister(r.labelTotal, r.labelUnread, r.labelSender, r.customQuery)
	return r
}

func (r *Registry) PrometheusRegistry() *prometheus.Registry {
	return r.prom
}

func (r *Registry) SetLabelTotal(id, name string, value float64) {
	r.labelTotal.WithLabelValues(id, name).Set(value)
}

func (r *Registry) SetLabelUnread(id, name string, value float64) {
	r.labelUnread.WithLabelValues(id, name).Set(value)
}

func (r *Registry) SetLabelSender(id, name, sender string, value float64) {
	r.labelSender.WithLabelValues(id, name, sender).Set(value)
}

func (r *Registry) SetCustomQuery(name string, value float64) {
	r.customQuery.WithLabelValues(name).Set(value)
}

func (r *Registry) LabelTotal(id, name string) prometheus.Gauge {
	return r.labelTotal.WithLabelValues(id, name)
}

func (r *Registry) LabelUnread(id, name string) prometheus.Gauge {
	return r.labelUnread.WithLabelValues(id, name)
}

func (r *Registry) LabelSender(id, name string) *labelSenderVec {
	return &labelSenderVec{vec: r.labelSender, id: id, name: name}
}

func (r *Registry) CustomQuery(name string) prometheus.Gauge {
	return r.customQuery.WithLabelValues(name)
}

// labelSenderVec adapts GaugeVec to set sender counts for a fixed label id/name.
type labelSenderVec struct {
	vec  *prometheus.GaugeVec
	id   string
	name string
}

func (v *labelSenderVec) WithLabelValues(sender string) prometheus.Gauge {
	return v.vec.WithLabelValues(v.id, v.name, sender)
}

// DeleteLabel removes total/unread/sender series for a Gmail label.
func (r *Registry) DeleteLabel(id, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.labelTotal.DeleteLabelValues(id, name)
	r.labelUnread.DeleteLabelValues(id, name)
	r.labelSender.DeletePartialMatch(prometheus.Labels{"id": id, "name": name})
}

// DeleteCustomQuery removes the series for a custom query name.
func (r *Registry) DeleteCustomQuery(name string) {
	r.customQuery.DeleteLabelValues(name)
}

// ResetAll removes every Gmail metric series from the registry vectors.
func (r *Registry) ResetAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.labelTotal.Reset()
	r.labelUnread.Reset()
	r.labelSender.Reset()
	r.customQuery.Reset()
}
