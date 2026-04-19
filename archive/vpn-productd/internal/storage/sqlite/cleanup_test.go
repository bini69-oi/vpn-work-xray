package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/internal/domain"
)

func TestCleanupExpired(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "cleanup.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	now := time.Now().UTC()
	old := now.Add(-40 * 24 * time.Hour).UTC()

	// Old + recent issues.
	require.NoError(t, store.CreateSubscriptionIssue(ctx, domain.SubscriptionIssue{
		ID:             "issue-old",
		UserID:         "u1",
		SubscriptionID: "sub-old",
		TokenHint:      "hint",
		Source:         "test",
		IssuedAt:       old,
		ExpiresAt:      old.Add(30 * 24 * time.Hour),
	}))
	require.NoError(t, store.CreateSubscriptionIssue(ctx, domain.SubscriptionIssue{
		ID:             "issue-new",
		UserID:         "u1",
		SubscriptionID: "sub-new",
		TokenHint:      "hint",
		Source:         "test",
		IssuedAt:       now,
		ExpiresAt:      now.Add(30 * 24 * time.Hour),
	}))

	// Old revoked subscription + recent revoked subscription + active old subscription.
	s1, err := store.CreateSubscription(ctx, domain.Subscription{
		ID:         "sub-rev-old",
		Name:       "x",
		UserID:     "u1",
		Token:      "tok-1",
		ProfileIDs: []string{"p1"},
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	require.NoError(t, err)
	require.NoError(t, store.RevokeSubscription(ctx, s1.ID))
	_, err = store.db.ExecContext(ctx, `UPDATE subscriptions SET updated_at = ? WHERE id = ?`, old.Format(time.RFC3339Nano), s1.ID)
	require.NoError(t, err)

	s2, err := store.CreateSubscription(ctx, domain.Subscription{
		ID:         "sub-rev-new",
		Name:       "y",
		UserID:     "u1",
		Token:      "tok-2",
		ProfileIDs: []string{"p1"},
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	require.NoError(t, err)
	require.NoError(t, store.RevokeSubscription(ctx, s2.ID))

	_, err = store.CreateSubscription(ctx, domain.Subscription{
		ID:         "sub-active-old",
		Name:       "z",
		UserID:     "u1",
		Token:      "tok-3",
		ProfileIDs: []string{"p1"},
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  old,
	})
	require.NoError(t, err)

	deleted, revokedStale, err := store.CleanupExpired(ctx, 30, 45)
	require.NoError(t, err)
	require.Equal(t, int64(0), revokedStale)
	require.GreaterOrEqual(t, deleted, int64(2))

	var issuesLeft int
	require.NoError(t, store.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM subscription_issues`).Scan(&issuesLeft))
	require.Equal(t, 1, issuesLeft)

	var subsLeft int
	require.NoError(t, store.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM subscriptions`).Scan(&subsLeft))
	require.Equal(t, 2, subsLeft) // sub-rev-new + sub-active-old
}

func TestCleanupExpired_RevokesStaleByLastAccess(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "cleanup-stale.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	now := time.Now().UTC()
	oldAccess := now.Add(-60 * 24 * time.Hour).UTC()

	_, err = store.CreateSubscription(ctx, domain.Subscription{
		ID:           "sub-stale",
		Name:         "stale",
		UserID:       "u1",
		Token:        "tok-stale",
		ProfileIDs:   []string{"p1"},
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
		LastAccessAt: &oldAccess,
	})
	require.NoError(t, err)

	deleted, revokedStale, err := store.CleanupExpired(ctx, 30, 45)
	require.NoError(t, err)
	require.Equal(t, int64(0), deleted)
	require.Equal(t, int64(1), revokedStale)

	var revoked int
	require.NoError(t, store.db.QueryRowContext(ctx, `SELECT revoked FROM subscriptions WHERE id = ?`, "sub-stale").Scan(&revoked))
	require.Equal(t, 1, revoked)
}
