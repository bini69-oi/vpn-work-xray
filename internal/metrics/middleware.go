package metrics

import (
	"strconv"
	"strings"
	"time"
)

// NormalizePathForMetrics collapses dynamic path segments for stable Prometheus labels.
func NormalizePathForMetrics(path string) string {
	if strings.HasPrefix(path, "/public/subscriptions/") {
		return "/public/subscriptions/:token"
	}
	if strings.HasPrefix(path, "/s/") {
		return "/s/:token"
	}
	return path
}

// RecordAPIRequest records vpn_product API counters and histograms (excluding /metrics noise at call site).
func RecordAPIRequest(method, path string, statusCode int, elapsed time.Duration) {
	safe := NormalizePathForMetrics(path)
	APIRequestsTotal.WithLabelValues(method, safe, strconv.Itoa(statusCode)).Inc()
	APIRequestDuration.WithLabelValues(method, safe).Observe(elapsed.Seconds())
}
