package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ActiveUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vpn_product",
		Name:      "active_users_total",
		Help:      "Количество активных пользователей с действующей подпиской",
	})

	OnlineUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vpn_product",
		Name:      "online_users_total",
		Help:      "Онлайн-пользователи: Xray StatsService.GetAllOnlineUsers при VPN_PRODUCT_XRAY_STATS_ADDR, иначе эвристика last_access_at подписок",
	})

	TotalUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vpn_product",
		Name:      "users_total",
		Help:      "Общее количество уникальных user_id в подписках",
	})

	UserTrafficUpload = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "vpn_product",
		Name:      "user_traffic_upload_bytes",
		Help:      "Загруженный трафик (upload) по пользователю/профилю в байтах",
	}, []string{"user_id", "email"})

	UserTrafficDownload = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "vpn_product",
		Name:      "user_traffic_download_bytes",
		Help:      "Скачанный трафик (download) по пользователю/профилю в байтах",
	}, []string{"user_id", "email"})

	TrafficTotalUpload = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "vpn_product",
		Name:      "traffic_upload_bytes_total",
		Help:      "Суммарный upload по профилям (накопление прироста между опросами)",
	})

	TrafficTotalDownload = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "vpn_product",
		Name:      "traffic_download_bytes_total",
		Help:      "Суммарный download по профилям (накопление прироста между опросами)",
	})

	SubscriptionsByStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "vpn_product",
		Name:      "subscriptions_by_status",
		Help:      "Количество подписок по статусу",
	}, []string{"status"})

	SubscriptionsExpiringSoon = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vpn_product",
		Name:      "subscriptions_expiring_24h",
		Help:      "Подписки, истекающие в ближайшие 24 часа",
	})

	ServerUptime = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vpn_product",
		Name:      "server_uptime_seconds",
		Help:      "Время работы vpn-productd в секундах",
	})

	XrayStatus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vpn_product",
		Name:      "xray_running",
		Help:      "Статус x-ui/xray: 1 = active (systemctl), 0 = иначе или не Linux",
	})

	XuiSyncStatus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vpn_product",
		Name:      "xui_sync_last_success_timestamp",
		Help:      "Unix timestamp последней успешной синхронизации с 3x-ui (heartbeat)",
	})

	XuiSyncErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "vpn_product",
		Name:      "xui_sync_errors_total",
		Help:      "Количество ошибок синхронизации с 3x-ui (зарезервировано)",
	})

	APIRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "vpn_product",
		Name:      "api_requests_total",
		Help:      "Общее количество API-запросов",
	}, []string{"method", "path", "status_code"})

	APIRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "vpn_product",
		Name:      "api_request_duration_seconds",
		Help:      "Длительность API-запросов",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path"})
)

// SetXUISyncLastSuccess sets unix timestamp of last successful sync heartbeat.
func SetXUISyncLastSuccess(unix float64) {
	XuiSyncStatus.Set(unix)
}
