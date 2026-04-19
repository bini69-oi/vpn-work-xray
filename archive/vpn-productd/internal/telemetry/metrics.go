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
	SubscriptionIssueTotal *prometheus.CounterVec
	Apply3XUITotal         *prometheus.CounterVec
	Apply3XUILatency       *prometheus.HistogramVec
	API5xxTotal            *prometheus.CounterVec
	SyncLagSeconds         *prometheus.GaugeVec
	SyncLastSuccessUnix    *prometheus.GaugeVec
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
			SubscriptionIssueTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "vpn_subscription_issue_total",
				Help: "Issue-link pipeline counters by result",
			}, []string{"result"}),
			Apply3XUITotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "vpn_apply_3xui_total",
				Help: "Apply-to-3x-ui counters by result",
			}, []string{"result"}),
			Apply3XUILatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "vpn_apply_3xui_latency_seconds",
				Help:    "Apply-to-3x-ui latency in seconds",
				Buckets: prometheus.DefBuckets,
			}, []string{"result"}),
			API5xxTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "vpn_api_5xx_total",
				Help: "Total API 5xx responses",
			}, []string{"method", "path", "status"}),
			SyncLagSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "vpn_sync_lag_seconds",
				Help: "Lag in seconds from sync start to success",
			}, []string{"sync_name"}),
			SyncLastSuccessUnix: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "vpn_sync_last_success_unix",
				Help: "Unix timestamp of last successful sync",
			}, []string{"sync_name"}),
		}
		prometheus.MustRegister(
			m.ActiveSessions,
			m.TrafficBytes,
			m.XrayStatus,
			m.APIResponseTime,
			m.DBErrorsTotal,
			m.SubscriptionIssueTotal,
			m.Apply3XUITotal,
			m.Apply3XUILatency,
			m.API5xxTotal,
			m.SyncLagSeconds,
			m.SyncLastSuccessUnix,
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

func ObserveApply3XUI(result string, started time.Time) {
	Default().Apply3XUILatency.WithLabelValues(result).Observe(time.Since(started).Seconds())
	Default().Apply3XUITotal.WithLabelValues(result).Inc()
}

func MarkSyncSuccess(name string, lagSeconds float64) {
	Default().SyncLastSuccessUnix.WithLabelValues(name).Set(float64(time.Now().UTC().Unix()))
	if lagSeconds >= 0 {
		Default().SyncLagSeconds.WithLabelValues(name).Set(lagSeconds)
	}
}
