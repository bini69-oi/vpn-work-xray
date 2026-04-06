package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler serves the default Prometheus registry (includes vpn_product_* and telemetry vpn_*).
func Handler() http.Handler {
	return promhttp.Handler()
}
