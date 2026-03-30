package subscription

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/product/domain"
	"github.com/xtls/xray-core/product/profile"
	"github.com/xtls/xray-core/product/storage/sqlite"
)

func TestCreateAndBuildContent(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "subs.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	profiles := profile.NewService(store)
	_, err = profiles.Save(ctx, domain.Profile{
		ID:        "p1",
		Name:      "P1",
		Enabled:   true,
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{Name: "v", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "11111111-2222-3333-4444-555555555555", ServerName: "sni.example.com", RealityPublicKey: "pk", RealityShortID: "ab12"},
		},
		PreferredID: "v",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second,
		},
	})
	require.NoError(t, err)

	s := NewService(store, profiles, nil)
	sub, err := s.Create(ctx, domain.Subscription{
		Name:       "happ",
		UserID:     "user-1",
		ProfileIDs: []string{"p1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, sub.Token)

	content, _, err := s.BuildContentByToken(ctx, sub.Token)
	require.NoError(t, err)
	require.Contains(t, content, "vless://")
}

func TestBuildContentDeniedForRevokedAndExpired(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "subs-deny.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()
	profiles := profile.NewService(store)
	_, err = profiles.Save(ctx, domain.Profile{
		ID: "p1", Name: "P1", Enabled: true, RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{{Name: "h", Address: "hy.example.com", Port: 443, Protocol: domain.ProtocolHysteria, ServerTag: "proxy", HysteriaPassword: "pass"}},
		PreferredID: "h",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second,
		},
	})
	require.NoError(t, err)
	s := NewService(store, profiles, nil)
	sub, err := s.Create(ctx, domain.Subscription{Name: "x", UserID: "u1", ProfileIDs: []string{"p1"}})
	require.NoError(t, err)
	require.NoError(t, s.Revoke(ctx, sub.ID))
	_, _, err = s.BuildContentByToken(ctx, sub.Token)
	require.Error(t, err)

	exp := time.Now().UTC().Add(-time.Hour)
	sub2, err := s.Create(ctx, domain.Subscription{Name: "x2", UserID: "u2", ProfileIDs: []string{"p1"}, ExpiresAt: &exp})
	require.NoError(t, err)
	_, _, err = s.BuildContentByToken(ctx, sub2.Token)
	require.Error(t, err)
}

func TestBuildContentFiltersInvalidProfiles(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "subs-filter.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()
	profiles := profile.NewService(store)
	_, err = profiles.Save(ctx, domain.Profile{
		ID: "good", Name: "good", Enabled: true, RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{{Name: "v", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "11111111-2222-3333-4444-555555555555", ServerName: "sni.example.com", RealityPublicKey: "pk", RealityShortID: "ab12"}},
		PreferredID: "v",
		ReconnectPolicy: domain.ReconnectPolicy{MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second},
	})
	require.NoError(t, err)
	_, err = profiles.Save(ctx, domain.Profile{
		ID: "bad", Name: "bad", Enabled: true, Blocked: true, RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{{Name: "h", Address: "hy.example.com", Port: 8443, Protocol: domain.ProtocolHysteria, ServerTag: "proxy", HysteriaPassword: "pass"}},
		PreferredID: "h",
		ReconnectPolicy: domain.ReconnectPolicy{MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second},
	})
	require.NoError(t, err)

	s := NewService(store, profiles, nil)
	sub, err := s.Create(ctx, domain.Subscription{Name: "f", UserID: "u", ProfileIDs: []string{"good", "bad"}})
	require.NoError(t, err)
	content, _, err := s.BuildContentByToken(ctx, sub.Token)
	require.NoError(t, err)
	require.Contains(t, content, "vless://")
	require.NotContains(t, content, "h2://")
}

func TestRotateInvalidatesOldTokenAndUpdatesMetadata(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "subs-rotate.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()
	profiles := profile.NewService(store)
	_, err = profiles.Save(ctx, domain.Profile{
		ID: "p1", Name: "good", Enabled: true, RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{{Name: "v", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "11111111-2222-3333-4444-555555555555", ServerName: "sni.example.com", RealityPublicKey: "pk", RealityShortID: "ab12"}},
		PreferredID: "v",
		ReconnectPolicy: domain.ReconnectPolicy{MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second},
	})
	require.NoError(t, err)
	s := NewService(store, profiles, nil)
	sub, err := s.Create(ctx, domain.Subscription{Name: "r", UserID: "u", ProfileIDs: []string{"p1"}})
	require.NoError(t, err)
	oldToken := sub.Token

	rotated, err := s.Rotate(ctx, sub.ID)
	require.NoError(t, err)
	require.NotEmpty(t, rotated.Token)
	require.NotEqual(t, oldToken, rotated.Token)
	require.NotNil(t, rotated.RotatedAt)
	require.GreaterOrEqual(t, rotated.RotationCount, 1)

	_, _, err = s.BuildContentByToken(ctx, oldToken)
	require.Error(t, err)

	content, _, err := s.BuildContentByToken(ctx, rotated.Token)
	require.NoError(t, err)
	require.Contains(t, content, "vless://")
}

func TestLifecycleByUser(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "subs-lifecycle.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()
	profiles := profile.NewService(store)
	_, err = profiles.Save(ctx, domain.Profile{
		ID: "p1", Name: "good", Enabled: true, RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{{Name: "v", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "11111111-2222-3333-4444-555555555555", ServerName: "sni.example.com", RealityPublicKey: "pk", RealityShortID: "ab12"}},
		PreferredID: "v",
		ReconnectPolicy: domain.ReconnectPolicy{MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second},
	})
	require.NoError(t, err)
	s := NewService(store, profiles, nil)
	sub, err := s.IssueLink30Days(ctx, "u-lc", []string{"p1"}, "vpn", "test")
	require.NoError(t, err)
	require.NotEmpty(t, sub.ID)

	renewed, err := s.ExtendActiveByUser(ctx, "u-lc", 10)
	require.NoError(t, err)
	require.NotNil(t, renewed.ExpiresAt)

	blocked, err := s.BlockActiveByUser(ctx, "u-lc")
	require.NoError(t, err)
	require.True(t, blocked.Revoked)
}

