package metrics

import (
	"strings"

	perrors "github.com/xtls/xray-core/internal/errors"
)

// RecordXUIIntegrationError increments xui_sync_errors_total when err is a 3x-ui integration failure (VPN_3XUI_*).
func RecordXUIIntegrationError(err error) {
	if err == nil {
		return
	}
	code := perrors.CodeOf(err)
	if strings.HasPrefix(code, "VPN_3XUI_") {
		XuiSyncErrors.Inc()
	}
}

// RecordExternalSyncFailure increments the counter when an external sync job reports failure (e.g. shell trap).
func RecordExternalSyncFailure() {
	XuiSyncErrors.Inc()
}
