package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureColumnExistingAndNew(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "migrate.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	require.NoError(t, store.ensureColumn(ctx, "profile_quota", "traffic_limit_gb", "INTEGER NOT NULL DEFAULT 0"))
	require.NoError(t, store.ensureColumn(ctx, "runtime_state", "extra_col", "TEXT"))
}
