package observ

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

// HTTPHandler exposes /metrics and records the HTTP golden signals for a
// service. Route labels use net/http ServeMux patterns rather than raw paths so
// identifiers and unbounded user input never become Prometheus labels.
func HTTPHandler(service string, next http.Handler) http.Handler {
	service = strings.TrimSpace(service)
	if service == "" {
		service = "unknown-service"
	}
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "auraedu_http_requests_total",
		Help: "Total AuraEDU HTTP requests by service, method, canonical route and status.",
	}, []string{"service", "method", "route", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "auraedu_http_request_duration_seconds",
		Help:    "AuraEDU HTTP request duration by service, method and canonical route.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 0.75, 1, 1.5, 2.5, 5, 10},
	}, []string{"service", "method", "route"})
	inFlight := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "auraedu_http_requests_in_flight",
		Help: "Current AuraEDU HTTP requests in flight by service.",
	}, []string{"service"})
	reg.MustRegister(requests, duration, inFlight)
	metrics := promhttp.HandlerFor(reg, promhttp.HandlerOpts{EnableOpenMetrics: true})
	metricsToken := strings.TrimSpace(os.Getenv("METRICS_BEARER_TOKEN"))
	application := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inFlight.WithLabelValues(service).Inc()
		defer inFlight.WithLabelValues(service).Dec()
		started := time.Now()
		status := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(status, r)
		route := strings.TrimSpace(r.Pattern)
		if route == "" {
			route = "unmatched"
		}
		method := strings.ToUpper(r.Method)
		requests.WithLabelValues(service, method, route, strconv.Itoa(status.code)).Inc()
		duration.WithLabelValues(service, method, route).Observe(time.Since(started).Seconds())
	})
	traced := otelhttp.NewHandler(application, service+".http")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			if metricsToken != "" && !validMetricsToken(r, metricsToken) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			metrics.ServeHTTP(w, r)
			return
		}
		traced.ServeHTTP(w, r)
	})
}

func validMetricsToken(r *http.Request, want string) bool {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

type statusWriter struct {
	http.ResponseWriter
	code  int
	wrote bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.wrote {
		return
	}
	w.code = code
	w.wrote = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

// Unwrap lets http.ResponseController retain streaming, hijacking and deadline
// behavior provided by the underlying server writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
