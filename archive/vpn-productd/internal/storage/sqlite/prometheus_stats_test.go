package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/internal/domain"
)

func TestPrometheusStats(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "prom.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	p := domain.Profile{
		ID:          "p1",
		Name:        "p",
		Enabled:     true,
		RouteMode:   domain.RouteModeSplit,
		Endpoints:   []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001"}},
		PreferredID: "main",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second,
		},
	}
	require.NoError(t, store.UpsertProfile(ctx, p))
	require.NoError(t, store.AddTrafficUsage(ctx, "p1", 100, 200))

	now := time.Now().UTC()
	exp := now.Add(48 * time.Hour)
	_, err = store.CreateSubscription(ctx, domain.Subscription{
		ID:             "s1",
		Name:           "n",
		UserID:         "u1",
		Token:          "secret-token",
		ProfileIDs:     []string{},
		Status:         "active",
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessAt:   &now,
		ExpiresAt:      &exp,
	})
	require.NoError(t, err)

	st, err := store.PrometheusStats(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, st.TotalUploadBytes, int64(100))
	require.GreaterOrEqual(t, st.TotalDistinctUsers, int64(1))
}
