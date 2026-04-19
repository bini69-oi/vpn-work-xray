package sqlite

import (
	"context"
	"fmt"
)

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS profiles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			protocol TEXT NOT NULL,
			profile_json TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS runtime_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS profile_quota (
			profile_id TEXT PRIMARY KEY,
			limit_mb INTEGER NOT NULL DEFAULT 0,
			used_upload_bytes INTEGER NOT NULL DEFAULT 0,
			used_download_bytes INTEGER NOT NULL DEFAULT 0,
			expires_at TEXT,
			traffic_limit_gb INTEGER NOT NULL DEFAULT 0,
			traffic_used_bytes INTEGER NOT NULL DEFAULT 0,
			blocked INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(profile_id) REFERENCES profiles(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_profile_quota_blocked ON profile_quota(blocked)`,
		`CREATE TABLE IF NOT EXISTS panel_users (
			id TEXT PRIMARY KEY,
			panel TEXT NOT NULL,
			external_id TEXT NOT NULL,
			username TEXT NOT NULL,
			profile_id TEXT NOT NULL,
			status TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_panel_users_profile ON panel_users(profile_id)`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			token_hash TEXT,
			token_hint TEXT,
			profile_ids_json TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			revoked INTEGER NOT NULL DEFAULT 0,
			revoked_at TEXT,
			rotated_at TEXT,
			rotation_count INTEGER NOT NULL DEFAULT 0,
			last_access_at TEXT,
			expires_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_subscriptions_token_hash ON subscriptions(token_hash)`,
		`CREATE TABLE IF NOT EXISTS subscription_issues (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			subscription_id TEXT NOT NULL,
			token_hint TEXT,
			source TEXT,
			issued_at TEXT NOT NULL,
			expires_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_subscription_issues_user ON subscription_issues(user_id, issued_at DESC)`,
		`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(1, datetime('now'))`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if err := s.ensureColumn(ctx, "profile_quota", "expires_at", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "profile_quota", "traffic_limit_gb", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "profile_quota", "traffic_used_bytes", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "subscriptions", "status", "TEXT NOT NULL DEFAULT 'active'"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "subscriptions", "revoked_at", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "subscriptions", "last_access_at", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "subscriptions", "token_hash", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "subscriptions", "token_hint", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "subscriptions", "rotated_at", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "subscriptions", "rotation_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "subscriptions", "token_secret", "TEXT"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(
		ctx,
		`UPDATE subscriptions
		 SET token_hash = lower(hex(sha256(token))),
		     token = '__hashed__:' || id
		 WHERE COALESCE(token_hash, '') = '' AND COALESCE(token, '') <> ''`,
	); err != nil {
		// Older SQLite builds may not support sha256 extension; keep runtime compatible.
		_ = err
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, tableName string, columnName string, columnDef string) error {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notNull   int
			dfltValue any
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnDef))
	return err
}
