package observ

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func Registry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

func Counter(reg prometheus.Registerer, name, help string, labels ...string) *prometheus.CounterVec {
	return promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: help,
	}, labels)
}

func Histogram(reg prometheus.Registerer, name, help string, labels ...string) *prometheus.HistogramVec {
	return promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: prometheus.DefBuckets,
	}, labels)
}
