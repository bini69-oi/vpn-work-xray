package telemetry

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	ActiveSessions  prometheus.Gauge
	TrafficBytes    *prometheus.CounterVec
	XrayStatus      prometheus.Gauge
	APIResponseTime *prometheus.HistogramVec
	DBErrorsTotal   prometheus.Counter
}

var (
	once           sync.Once
	defaultMetrics *Metrics
)

func Default() *Metrics {
	once.Do(func() {
		m := &Metrics{
			ActiveSessions: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "vpn_active_sessions_count",
				Help: "Current number of active VPN sessions",
			}),
			TrafficBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "vpn_traffic_bytes_total",
				Help: "Total VPN traffic bytes by direction",
			}, []string{"direction"}),
			XrayStatus: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "vpn_xray_status",
				Help: "Xray runtime status (1=running, 0=stopped)",
			}),
			APIResponseTime: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "vpn_api_response_time_seconds",
				Help:    "API response latency in seconds",
				Buckets: prometheus.DefBuckets,
			}, []string{"method", "path", "status"}),
			DBErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "vpn_database_errors_total",
				Help: "Total number of database errors",
			}),
		}
		prometheus.MustRegister(
			m.ActiveSessions,
			m.TrafficBytes,
			m.XrayStatus,
			m.APIResponseTime,
			m.DBErrorsTotal,
		)
		defaultMetrics = m
	})
	return defaultMetrics
}

func Handler() http.Handler {
	return promhttp.Handler()
}

func ObserveAPILatency(method string, path string, statusCode int, started time.Time) {
	Default().APIResponseTime.WithLabelValues(method, path, strconv.Itoa(statusCode)).Observe(time.Since(started).Seconds())
}
