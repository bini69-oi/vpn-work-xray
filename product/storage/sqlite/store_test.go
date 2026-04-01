package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/product/domain"
)

func TestStoreProfileLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	p := domain.Profile{
		ID:        "p1",
		Name:      "profile one",
		Enabled:   true,
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001"},
		},
		PreferredID: "main",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries:  3,
			BaseBackoff: time.Second,
			MaxBackoff:  5 * time.Second,
		},
	}
	require.NoError(t, store.UpsertProfile(ctx, p))
	got, err := store.GetProfile(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, p.ID, got.ID)
	require.NoError(t, store.SetActiveProfile(ctx, "p1"))
	active, err := store.GetActiveProfile(ctx)
	require.NoError(t, err)
	assert.Equal(t, "p1", active)
	require.NoError(t, store.SetTrafficLimit(ctx, "p1", 1))
	require.NoError(t, store.AddTrafficUsage(ctx, "p1", 700000, 500000))
	got, err = store.GetProfile(ctx, "p1")
	require.NoError(t, err)
	assert.True(t, got.Blocked)
}

func TestStoreMigrationsAndIdempotentOpen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store1, err := Open(ctx, dbPath)
	require.NoError(t, err)
	require.NoError(t, store1.Close())

	store2, err := Open(ctx, dbPath)
	require.NoError(t, err)
	defer func() { _ = store2.Close() }()

	require.NoError(t, store2.SelfCheck(ctx))
}

func TestStoreListAndPanelUsers(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	for i := 0; i < 2; i++ {
		p := domain.Profile{
			ID:          fmt.Sprintf("p%d", i),
			Name:        fmt.Sprintf("profile-%d", i),
			Enabled:     true,
			RouteMode:   domain.RouteModeSplit,
			Endpoints:   []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001"}},
			PreferredID: "main",
			ReconnectPolicy: domain.ReconnectPolicy{
				MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second,
			},
		}
		require.NoError(t, store.UpsertProfile(ctx, p))
	}

	items, err := store.ListProfiles(ctx)
	require.NoError(t, err)
	assert.Len(t, items, 2)

	require.NoError(t, store.UpsertPanelUser(ctx, domain.PanelUser{ID: "u1", Panel: "3x-ui", ProfileID: "p0", Username: "alice"}))
	require.NoError(t, store.UpsertPanelUser(ctx, domain.PanelUser{ID: "u2", Panel: "another", ProfileID: "p1", Username: "bob"}))

	panelUsers, err := store.ListPanelUsers(ctx, "3x-ui")
	require.NoError(t, err)
	require.Len(t, panelUsers, 1)
	assert.Equal(t, "u1", panelUsers[0].ID)
}

func TestStoreDeleteProfile(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	p := domain.Profile{
		ID:          "to-delete",
		Name:        "delete me",
		Enabled:     true,
		RouteMode:   domain.RouteModeSplit,
		Endpoints:   []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001"}},
		PreferredID: "main",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second,
		},
	}
	require.NoError(t, store.UpsertProfile(ctx, p))
	require.NoError(t, store.DeleteProfile(ctx, p.ID))
	_, err = store.GetProfile(ctx, p.ID)
	require.Error(t, err)
}

func TestStoreCheckAccess_SubscriptionAndQuota(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	expiresPast := time.Now().UTC().Add(-time.Hour)
	p := domain.Profile{
		ID:                    "premium1",
		Name:                  "premium",
		Enabled:               true,
		RouteMode:             domain.RouteModeSplit,
		Endpoints:             []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001"}},
		PreferredID:           "main",
		SubscriptionExpiresAt: &expiresPast,
		TrafficLimitGB:        1,
		TrafficUsedBytes:      2 * 1024 * 1024 * 1024,
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second,
		},
	}
	require.NoError(t, store.UpsertProfile(ctx, p))

	err = store.CheckAccess(ctx, p.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscription expired")

	future := time.Now().UTC().Add(24 * time.Hour)
	p.SubscriptionExpiresAt = &future
	require.NoError(t, store.UpsertProfile(ctx, p))
	err = store.CheckAccess(ctx, p.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "traffic quota exhausted")
}

func TestStoreRuntimeStateAndWriteFailure(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)

	require.NoError(t, store.SetRuntimeState(ctx, "k1", "v1"))
	v, err := store.GetRuntimeState(ctx, "k1")
	require.NoError(t, err)
	assert.Equal(t, "v1", v)

	require.NoError(t, store.Close())
	err = store.SetRuntimeState(ctx, "k2", "v2")
	require.Error(t, err)
}

func TestStoreAccessAndStateHelpers(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	p := domain.Profile{
		ID:          "state-access",
		Name:        "state-access",
		Enabled:     true,
		RouteMode:   domain.RouteModeSplit,
		Endpoints:   []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001"}},
		PreferredID: "main",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second,
		},
	}
	require.NoError(t, store.UpsertProfile(ctx, p))
	require.NoError(t, store.SetBlocked(ctx, p.ID, true))
	err = store.CheckAccess(ctx, p.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")

	unknown, err := store.GetRuntimeState(ctx, "missing")
	require.NoError(t, err)
	assert.Empty(t, unknown)
	require.NoError(t, store.CheckAccess(ctx, "no-row"))
}

func TestStoreHelpersCoverage(t *testing.T) {
	assert.Equal(t, 1, boolToInt(true))
	assert.Equal(t, 0, boolToInt(false))

	ts := time.Now().UTC()
	assert.Equal(t, ts.Format(time.RFC3339Nano), timeToText(&ts))
	assert.Empty(t, timeToText(nil))

	assert.Nil(t, parseNullableTime(""))
	rfcRaw := ts.Format(time.RFC3339Nano)
	require.NotNil(t, parseNullableTime(rfcRaw))
	unixRaw := fmt.Sprintf("%d", ts.Unix())
	require.NotNil(t, parseNullableTime(unixRaw))
	assert.Nil(t, parseNullableTime("bad-time"))

	assert.Equal(t, "", primaryProtocol(domain.Profile{}))
	assert.Equal(t, "vless", primaryProtocol(domain.Profile{
		PreferredID: "main",
		Endpoints: []domain.Endpoint{
			{Name: "main", Protocol: domain.ProtocolVLESS},
			{Name: "backup", Protocol: domain.ProtocolHysteria},
		},
	}))
	assert.Equal(t, "hysteria2", primaryProtocol(domain.Profile{Endpoints: []domain.Endpoint{{Name: "x", Protocol: domain.ProtocolHysteria}}}))
}

func TestStoreSetActiveAndSelfCheckFailurePaths(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	require.NoError(t, store.Close())

	err = store.SetActiveProfile(ctx, "x")
	require.Error(t, err)
	_, err = store.GetActiveProfile(ctx)
	require.Error(t, err)
	err = store.SelfCheck(ctx)
	require.Error(t, err)
}

func TestStoreTrafficLimitAndUsageAndPanelErrors(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	require.NoError(t, store.Close())

	require.Error(t, store.SetTrafficLimit(ctx, "p", 1))
	require.Error(t, store.AddTrafficUsage(ctx, "p", 1, 1))
	require.Error(t, store.UpsertPanelUser(ctx, domain.PanelUser{ID: "u1", Panel: "3x-ui", ProfileID: "p"}))
	_, err = store.ListPanelUsers(ctx, "3x-ui")
	require.Error(t, err)
}

func TestStoreCorruptJSONBranches(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "corrupt.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	_, err = store.db.ExecContext(ctx, `INSERT INTO profiles(id, name, enabled, protocol, profile_json, updated_at) VALUES('bad','bad',1,'vless','{broken',?)`, time.Now().UTC().Format(time.RFC3339Nano))
	require.NoError(t, err)

	_, err = store.GetProfile(ctx, "bad")
	require.Error(t, err)

	_, err = store.ListProfiles(ctx)
	require.Error(t, err)
}

func TestStoreOpenAndMigrationFailureBranches(t *testing.T) {
	ctx := context.Background()
	_, err := Open(ctx, t.TempDir())
	require.Error(t, err)

	store, err := Open(ctx, filepath.Join(t.TempDir(), "migrate-fail.db"))
	require.NoError(t, err)
	require.NoError(t, store.Close())
	require.Error(t, store.migrate(ctx))
}

func TestStoreUpsertAndDeleteErrorBranches(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "upsert-fail.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	_, err = store.db.ExecContext(ctx, `DROP TABLE profile_quota`)
	require.NoError(t, err)
	p := domain.Profile{
		ID: "p1", Name: "x", RouteMode: domain.RouteModeSplit,
		Endpoints:   []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001"}},
		PreferredID: "main",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second,
		},
	}
	require.Error(t, store.UpsertProfile(ctx, p))

	require.NoError(t, store.Close())
	require.Error(t, store.DeleteProfile(ctx, "x"))
}

func TestStoreAddTrafficUsageErrorBranch(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "usage-fail.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()
	_, err = store.db.ExecContext(ctx, `DROP TABLE profile_quota`)
	require.NoError(t, err)
	require.Error(t, store.AddTrafficUsage(ctx, "p1", 10, 20))
}

func TestEnsureColumnErrorBranch(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "ensure-column.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()
	err = store.ensureColumn(ctx, "runtime_state", "bad_col", "TEXT NOT NULL DEFAULT")
	require.Error(t, err)
}

func TestGetProfileNoRowsBranch(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "no-rows.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()
	_, err = store.GetProfile(ctx, "missing")
	require.Error(t, err)
	assert.NotEqual(t, sql.ErrNoRows, err)
}

func TestSubscriptionTokenHashLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "subs-hash.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	now := time.Now().UTC()
	item, err := store.CreateSubscription(ctx, domain.Subscription{
		ID:         "s1",
		Name:       "sub",
		UserID:     "u1",
		Token:      "plain-token-value",
		ProfileIDs: []string{"p1"},
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	require.NoError(t, err)
	require.NotEmpty(t, item.TokenHint)

	gotByToken, err := store.GetSubscriptionByToken(ctx, "plain-token-value")
	require.NoError(t, err)
	require.Equal(t, "s1", gotByToken.ID)

	rotated, err := store.RotateSubscriptionToken(ctx, "s1", "new-token-value")
	require.NoError(t, err)
	require.GreaterOrEqual(t, rotated.RotationCount, 1)
	require.NotNil(t, rotated.RotatedAt)

	_, err = store.GetSubscriptionByToken(ctx, "plain-token-value")
	require.Error(t, err)
	_, err = store.GetSubscriptionByToken(ctx, "new-token-value")
	require.NoError(t, err)

	require.NoError(t, store.RevokeSubscription(ctx, "s1"))
	revoked, err := store.GetSubscription(ctx, "s1")
	require.NoError(t, err)
	require.True(t, revoked.Revoked)
	require.NotNil(t, revoked.RevokedAt)
	require.Equal(t, "revoked", revoked.Status)

	require.NoError(t, store.TouchSubscriptionAccess(ctx, "s1"))
	accessed, err := store.GetSubscription(ctx, "s1")
	require.NoError(t, err)
	require.NotNil(t, accessed.LastAccessAt)
}

func TestSubscriptionUserRevokeAndAssignProfiles(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "subs-user.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	now := time.Now().UTC()
	_, err = store.CreateSubscription(ctx, domain.Subscription{
		ID:         "s1",
		Name:       "sub1",
		UserID:     "u1",
		Token:      "tok1",
		ProfileIDs: []string{"p1"},
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	require.NoError(t, err)
	_, err = store.CreateSubscription(ctx, domain.Subscription{
		ID:         "s2",
		Name:       "sub2",
		UserID:     "u1",
		Token:      "tok2",
		ProfileIDs: []string{"p2"},
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	require.NoError(t, err)
	affected, err := store.RevokeActiveSubscriptionsByUser(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, int64(2), affected)

	require.NoError(t, store.UpdateSubscriptionProfiles(ctx, "s1", []string{"new-profile"}))
	got, err := store.GetSubscription(ctx, "s1")
	require.NoError(t, err)
	require.Equal(t, []string{"new-profile"}, got.ProfileIDs)
	require.True(t, got.Revoked)
}

func TestRevokeActiveSubscriptionsByUserRevokesNonActiveStatusesToo(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "subs-user-status.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	now := time.Now().UTC()
	_, err = store.CreateSubscription(ctx, domain.Subscription{
		ID:         "s-active",
		Name:       "active",
		UserID:     "u2",
		Token:      "tok-active",
		ProfileIDs: []string{"p1"},
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	require.NoError(t, err)
	_, err = store.CreateSubscription(ctx, domain.Subscription{
		ID:         "s-paused",
		Name:       "paused",
		UserID:     "u2",
		Token:      "tok-paused",
		ProfileIDs: []string{"p2"},
		Status:     "paused",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	require.NoError(t, err)

	affected, err := store.RevokeActiveSubscriptionsByUser(ctx, "u2")
	require.NoError(t, err)
	require.Equal(t, int64(2), affected)

	active, err := store.GetSubscription(ctx, "s-active")
	require.NoError(t, err)
	require.True(t, active.Revoked)
	require.Equal(t, "revoked", active.Status)

	paused, err := store.GetSubscription(ctx, "s-paused")
	require.NoError(t, err)
	require.True(t, paused.Revoked)
	require.Equal(t, "revoked", paused.Status)
}
