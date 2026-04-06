package metrics

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	perrors "github.com/xtls/xray-core/internal/errors"
)

func TestRecordXUIIntegrationError(t *testing.T) {
	before := testutil.ToFloat64(XuiSyncErrors)
	RecordXUIIntegrationError(nil)
	require.Equal(t, before, testutil.ToFloat64(XuiSyncErrors))

	RecordXUIIntegrationError(perrors.New("VPN_SUBS_404", "not found"))
	require.Equal(t, before, testutil.ToFloat64(XuiSyncErrors))

	RecordXUIIntegrationError(perrors.Wrap("VPN_3XUI_001", "x", errors.New("cause")))
	require.Equal(t, before+1, testutil.ToFloat64(XuiSyncErrors))
}
