package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"encoding/hex"
	"strconv"
	"time"

	// Register modernc sqlite driver.
	_ "modernc.org/sqlite"

	"github.com/xtls/xray-core/internal/domain"
	perrors "github.com/xtls/xray-core/internal/errors"
	"github.com/xtls/xray-core/internal/profile"
	"github.com/xtls/xray-core/internal/telemetry"
)

const driverName = "sqlite"

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, dbErr("open", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, dbErr("ping", err)
	}
	// WAL improves concurrency and reduces SQLITE_BUSY for mixed read/write workloads.
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, dbErr("wal", err)
	}
	// Wait for locks instead of failing fast.
	_, _ = db.ExecContext(ctx, "PRAGMA busy_timeout=5000")
	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, dbErr("migrate", err)
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) UpsertProfile(ctx context.Context, p domain.Profile) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return dbErr("upsert_profile", err)
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO profiles(id, name, enabled, protocol, profile_json, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name,
		   enabled = excluded.enabled,
		   protocol = excluded.protocol,
		   profile_json = excluded.profile_json,
		   updated_at = excluded.updated_at`,
		p.ID,
		p.Name,
		boolToInt(p.Enabled),
		primaryProtocol(p),
		string(raw),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO profile_quota(profile_id, limit_mb, used_upload_bytes, used_download_bytes, expires_at, traffic_limit_gb, traffic_used_bytes, blocked, updated_at)
		 VALUES(?, ?, 0, 0, ?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id) DO UPDATE SET
		   limit_mb = excluded.limit_mb,
		   expires_at = excluded.expires_at,
		   traffic_limit_gb = excluded.traffic_limit_gb,
		   blocked = excluded.blocked,
		   updated_at = excluded.updated_at`,
		p.ID,
		p.TrafficLimitMB,
		timeToText(p.SubscriptionExpiresAt),
		p.TrafficLimitGB,
		p.TrafficUsedBytes,
		boolToInt(p.Blocked),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return dbErr("upsert_profile_quota", err)
	}
	return nil
}

func (s *Store) GetProfile(ctx context.Context, id string) (domain.Profile, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT profile_json FROM profiles WHERE id = ?`, id).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Profile{}, profile.ErrNotFound
		}
		return domain.Profile{}, dbErr("get_profile", err)
	}
	var p domain.Profile
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return domain.Profile{}, fmt.Errorf("decode profile: %w", err)
	}
	if err := s.fillQuota(ctx, &p); err != nil {
		return domain.Profile{}, err
	}
	return p, nil
}

func (s *Store) ListProfiles(ctx context.Context) ([]domain.Profile, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT p.profile_json,
		        COALESCE(q.limit_mb, 0),
		        COALESCE(q.used_upload_bytes, 0),
		        COALESCE(q.used_download_bytes, 0),
		        q.expires_at,
		        COALESCE(q.traffic_limit_gb, 0),
		        COALESCE(q.traffic_used_bytes, 0),
		        COALESCE(q.blocked, 0)
		 FROM profiles p
		 LEFT JOIN profile_quota q ON q.profile_id = p.id
		 ORDER BY p.name`,
	)
	if err != nil {
		return nil, dbErr("list_profiles_query", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.Profile
	for rows.Next() {
		var raw string
		var limitMB, up, down int64
		var (
			expiresRaw string
			limitGB    int64
			usedBytes  int64
			blocked    int
		)
		if err := rows.Scan(&raw, &limitMB, &up, &down, &expiresRaw, &limitGB, &usedBytes, &blocked); err != nil {
			return nil, dbErr("list_profiles_scan", err)
		}
		var p domain.Profile
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			return nil, err
		}
		p.TrafficLimitMB = limitMB
		p.TrafficUsedUp = up
		p.TrafficUsedDown = down
		p.SubscriptionExpiresAt = parseNullableTime(expiresRaw)
		p.TrafficLimitGB = limitGB
		p.TrafficUsedBytes = usedBytes
		p.Blocked = blocked == 1
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, dbErr("list_profiles_rows", err)
	}
	return out, nil
}

func (s *Store) DeleteProfile(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, id)
	if err != nil {
		return dbErr("delete_profile", err)
	}
	return nil
}

func (s *Store) SetActiveProfile(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dbErr("set_active_begin_tx", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM runtime_state WHERE key = 'active_profile'`); err != nil {
		return dbErr("set_active_delete", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO runtime_state(key, value, updated_at) VALUES('active_profile', ?, ?)`, id, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return dbErr("set_active_insert", err)
	}
	if err := tx.Commit(); err != nil {
		return dbErr("set_active_commit", err)
	}
	return nil
}

func (s *Store) GetActiveProfile(ctx context.Context) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM runtime_state WHERE key = 'active_profile'`).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", dbErr("get_active_profile", err)
	}
	return id, nil
}

func (s *Store) SetTrafficLimit(ctx context.Context, profileID string, limitMB int64) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO profile_quota(profile_id, limit_mb, used_upload_bytes, used_download_bytes, expires_at, traffic_limit_gb, traffic_used_bytes, blocked, updated_at)
		 VALUES(?, ?, 0, 0, NULL, 0, 0, 0, ?)
		 ON CONFLICT(profile_id) DO UPDATE SET
		   limit_mb = excluded.limit_mb,
		   updated_at = excluded.updated_at`,
		profileID,
		limitMB,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return dbErr("set_traffic_limit", err)
	}
	return nil
}

func (s *Store) AddTrafficUsage(ctx context.Context, profileID string, uploadBytes, downloadBytes int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dbErr("add_usage_begin_tx", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO profile_quota(profile_id, limit_mb, used_upload_bytes, used_download_bytes, expires_at, traffic_limit_gb, traffic_used_bytes, blocked, updated_at)
		 VALUES(?, 0, ?, ?, NULL, 0, ?, 0, ?)
		 ON CONFLICT(profile_id) DO UPDATE SET
		   used_upload_bytes = profile_quota.used_upload_bytes + excluded.used_upload_bytes,
		   used_download_bytes = profile_quota.used_download_bytes + excluded.used_download_bytes,
		   traffic_used_bytes = profile_quota.traffic_used_bytes + excluded.traffic_used_bytes,
		   updated_at = excluded.updated_at`,
		profileID,
		uploadBytes,
		downloadBytes,
		uploadBytes+downloadBytes,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return dbErr("add_usage_upsert", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE profile_quota
		 SET blocked = CASE
		   WHEN limit_mb > 0 AND (used_upload_bytes + used_download_bytes) >= limit_mb * 1024 * 1024 THEN 1
		   ELSE blocked
		 END,
		 updated_at = ?
		 WHERE profile_id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano),
		profileID,
	); err != nil {
		return dbErr("add_usage_block_update", err)
	}
	if err := tx.Commit(); err != nil {
		return dbErr("add_usage_commit", err)
	}
	return nil
}

func (s *Store) SetBlocked(ctx context.Context, profileID string, blocked bool) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO profile_quota(profile_id, limit_mb, used_upload_bytes, used_download_bytes, expires_at, traffic_limit_gb, traffic_used_bytes, blocked, updated_at)
		 VALUES(?, 0, 0, 0, NULL, 0, 0, ?, ?)
		 ON CONFLICT(profile_id) DO UPDATE SET
		   blocked = excluded.blocked,
		   updated_at = excluded.updated_at`,
		profileID,
		boolToInt(blocked),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return dbErr("set_blocked", err)
	}
	return nil
}

func (s *Store) fillQuota(ctx context.Context, p *domain.Profile) error {
	var limitMB, up, down, limitGB, usedBytes int64
	var expiresRaw string
	var blocked int
	err := s.db.QueryRowContext(
		ctx,
		`SELECT limit_mb, used_upload_bytes, used_download_bytes, expires_at, traffic_limit_gb, traffic_used_bytes, blocked
		 FROM profile_quota
		 WHERE profile_id = ?`,
		p.ID,
	).Scan(&limitMB, &up, &down, &expiresRaw, &limitGB, &usedBytes, &blocked)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return dbErr("fill_quota", err)
	}
	p.TrafficLimitMB = limitMB
	p.TrafficUsedUp = up
	p.TrafficUsedDown = down
	p.SubscriptionExpiresAt = parseNullableTime(expiresRaw)
	p.TrafficLimitGB = limitGB
	p.TrafficUsedBytes = usedBytes
	p.Blocked = blocked == 1
	return nil
}

func (s *Store) UpsertPanelUser(ctx context.Context, u domain.PanelUser) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO panel_users(id, panel, external_id, username, profile_id, status, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   panel = excluded.panel,
		   external_id = excluded.external_id,
		   username = excluded.username,
		   profile_id = excluded.profile_id,
		   status = excluded.status,
		   updated_at = excluded.updated_at`,
		u.ID, u.Panel, u.ExternalID, u.Username, u.ProfileID, u.Status, time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return dbErr("upsert_panel_user", err)
	}
	return nil
}

func (s *Store) ListPanelUsers(ctx context.Context, panel string) ([]domain.PanelUser, error) {
	query := `SELECT id, panel, external_id, username, profile_id, status, updated_at FROM panel_users`
	args := []any{}
	if panel != "" {
		query += ` WHERE panel = ?`
		args = append(args, panel)
	}
	query += ` ORDER BY username`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, dbErr("list_panel_users_query", err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.PanelUser{}
	for rows.Next() {
		var item domain.PanelUser
		var updatedRaw string
		if err := rows.Scan(&item.ID, &item.Panel, &item.ExternalID, &item.Username, &item.ProfileID, &item.Status, &updatedRaw); err != nil {
			return nil, dbErr("list_panel_users_scan", err)
		}
		if ts, err := time.Parse(time.RFC3339Nano, updatedRaw); err == nil {
			item.UpdatedAt = ts
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, dbErr("list_panel_users_rows", err)
	}
	return out, nil
}

func (s *Store) SelfCheck(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dbErr("selfcheck_begin_tx", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS selfcheck_runtime (id TEXT PRIMARY KEY, updated_at TEXT NOT NULL)`); err != nil {
		return dbErr("selfcheck_create_table", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO selfcheck_runtime(id, updated_at) VALUES('probe', ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return dbErr("selfcheck_insert", err)
	}
	var id string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM selfcheck_runtime WHERE id = 'probe'`).Scan(&id); err != nil {
		return dbErr("selfcheck_select", err)
	}
	if err := tx.Commit(); err != nil {
		return dbErr("selfcheck_commit", err)
	}
	return nil
}

func (s *Store) CheckAccess(ctx context.Context, profileID string) error {
	var (
		blocked    int
		expiresRaw string
		limitGB    int64
		usedBytes  int64
	)
	err := s.db.QueryRowContext(
		ctx,
		`SELECT COALESCE(blocked,0), COALESCE(expires_at,''), COALESCE(traffic_limit_gb,0), COALESCE(traffic_used_bytes,0)
		 FROM profile_quota WHERE profile_id = ?`,
		profileID,
	).Scan(&blocked, &expiresRaw, &limitGB, &usedBytes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return dbErr("check_access", err)
	}
	if blocked == 1 {
		return perrors.New("VPN_ACCESS_BLOCKED_001", "profile is blocked")
	}
	if expiresRaw != "" {
		expiresAt, parseErr := time.Parse(time.RFC3339Nano, expiresRaw)
		if parseErr == nil && time.Now().UTC().After(expiresAt) {
			return perrors.New("VPN_ACCESS_EXPIRED_001", "subscription expired")
		}
	}
	if limitGB > 0 && usedBytes >= limitGB*1024*1024*1024 {
		return perrors.New("VPN_ACCESS_QUOTA_001", "traffic quota exhausted")
	}
	return nil
}

func (s *Store) SetRuntimeState(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO runtime_state(key, value, updated_at) VALUES(?, ?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return dbErr("set_runtime_state", err)
	}
	return nil
}

func (s *Store) GetRuntimeState(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM runtime_state WHERE key = ?`, key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", dbErr("get_runtime_state", err)
	}
	return value, nil
}

func (s *Store) CreateSubscription(ctx context.Context, item domain.Subscription) (domain.Subscription, error) {
	rawIDs, err := json.Marshal(item.ProfileIDs)
	if err != nil {
		return domain.Subscription{}, dbErr("create_subscription_encode", err)
	}
	tokenHash := hashToken(item.Token)
	item.TokenHint = tokenHint(item.Token)
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO subscriptions(id, name, user_id, token, token_hash, token_hint, profile_ids_json, status, revoked, revoked_at, rotated_at, rotation_count, last_access_at, expires_at, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID,
		item.Name,
		item.UserID,
		"__hashed__:"+item.ID, // retain uniqueness without storing raw token
		tokenHash,
		item.TokenHint,
		string(rawIDs),
		item.Status,
		boolToInt(item.Revoked),
		timeToText(item.RevokedAt),
		timeToText(item.RotatedAt),
		item.RotationCount,
		timeToText(item.LastAccessAt),
		timeToText(item.ExpiresAt),
		item.CreatedAt.UTC().Format(time.RFC3339Nano),
		item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return domain.Subscription{}, dbErr("create_subscription_insert", err)
	}
	return item, nil
}

func (s *Store) GetSubscription(ctx context.Context, id string) (domain.Subscription, error) {
	return s.getSubscriptionBy(ctx, "id", id)
}

func (s *Store) GetSubscriptionByToken(ctx context.Context, token string) (domain.Subscription, error) {
	hashed := hashToken(token)
	return s.getSubscriptionByTokenHash(ctx, hashed)
}

func (s *Store) RevokeSubscription(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `UPDATE subscriptions SET status = 'revoked', revoked = 1, revoked_at = ?, updated_at = ? WHERE id = ?`, now, now, id)
	if err != nil {
		return dbErr("revoke_subscription", err)
	}
	return nil
}

func (s *Store) RevokeActiveSubscriptionsByUser(ctx context.Context, userID string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `UPDATE subscriptions SET status = 'revoked', revoked = 1, revoked_at = ?, updated_at = ? WHERE user_id = ? AND revoked = 0`, now, now, userID)
	if err != nil {
		return 0, dbErr("revoke_subscriptions_by_user", err)
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

func (s *Store) GetActiveSubscriptionByUser(ctx context.Context, userID string) (domain.Subscription, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, user_id, COALESCE(token,''), COALESCE(token_hash,''), COALESCE(token_hint,''), profile_ids_json, COALESCE(status,''), COALESCE(revoked,0), COALESCE(revoked_at,''), COALESCE(rotated_at,''), COALESCE(rotation_count,0), COALESCE(last_access_at,''), COALESCE(expires_at,''), COALESCE(created_at,''), COALESCE(updated_at,'')
		 FROM subscriptions
		 WHERE user_id = ? AND revoked = 0 AND status = 'active'
		 ORDER BY updated_at DESC
		 LIMIT 1`,
		userID,
	)
	var (
		item            domain.Subscription
		rawToken        string
		tokenHashStored string
		tokenHintStored string
		rawIDs          string
		status          string
		revoked         int
		revokedRaw      string
		rotatedRaw      string
		rotationCount   int
		accessRaw       string
		expiresRaw      string
		createdRaw      string
		updatedRaw      string
	)
	if err := row.Scan(&item.ID, &item.Name, &item.UserID, &rawToken, &tokenHashStored, &tokenHintStored, &rawIDs, &status, &revoked, &revokedRaw, &rotatedRaw, &rotationCount, &accessRaw, &expiresRaw, &createdRaw, &updatedRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Subscription{}, profile.ErrNotFound
		}
		return domain.Subscription{}, dbErr("get_active_subscription_by_user", err)
	}
	if err := json.Unmarshal([]byte(rawIDs), &item.ProfileIDs); err != nil {
		return domain.Subscription{}, dbErr("get_active_subscription_by_user_decode", err)
	}
	item.Revoked = revoked == 1
	item.Status = status
	if tokenHintStored != "" {
		item.TokenHint = tokenHintStored
	} else if rawToken != "" {
		item.TokenHint = tokenHint(rawToken)
	}
	_ = tokenHashStored
	item.RotatedAt = parseNullableTime(rotatedRaw)
	item.RotationCount = rotationCount
	item.RevokedAt = parseNullableTime(revokedRaw)
	item.LastAccessAt = parseNullableTime(accessRaw)
	item.ExpiresAt = parseNullableTime(expiresRaw)
	if ts, err := time.Parse(time.RFC3339Nano, createdRaw); err == nil {
		item.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339Nano, updatedRaw); err == nil {
		item.UpdatedAt = ts
	}
	return item, nil
}

func (s *Store) GetLastSubscriptionByUser(ctx context.Context, userID string) (domain.Subscription, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, user_id, COALESCE(token,''), COALESCE(token_hash,''), COALESCE(token_hint,''), profile_ids_json, COALESCE(status,''), COALESCE(revoked,0), COALESCE(revoked_at,''), COALESCE(rotated_at,''), COALESCE(rotation_count,0), COALESCE(last_access_at,''), COALESCE(expires_at,''), COALESCE(created_at,''), COALESCE(updated_at,'')
		 FROM subscriptions
		 WHERE user_id = ?
		 ORDER BY created_at DESC
		 LIMIT 1`,
		userID,
	)
	var (
		item            domain.Subscription
		rawToken        string
		tokenHashStored string
		tokenHintStored string
		rawIDs          string
		status          string
		revoked         int
		revokedRaw      string
		rotatedRaw      string
		rotationCount   int
		accessRaw       string
		expiresRaw      string
		createdRaw      string
		updatedRaw      string
	)
	if err := row.Scan(&item.ID, &item.Name, &item.UserID, &rawToken, &tokenHashStored, &tokenHintStored, &rawIDs, &status, &revoked, &revokedRaw, &rotatedRaw, &rotationCount, &accessRaw, &expiresRaw, &createdRaw, &updatedRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Subscription{}, profile.ErrNotFound
		}
		return domain.Subscription{}, dbErr("get_last_subscription_by_user", err)
	}
	if err := json.Unmarshal([]byte(rawIDs), &item.ProfileIDs); err != nil {
		return domain.Subscription{}, dbErr("get_last_subscription_by_user_decode", err)
	}
	item.Revoked = revoked == 1
	item.Status = status
	if tokenHintStored != "" {
		item.TokenHint = tokenHintStored
	} else if rawToken != "" {
		item.TokenHint = tokenHint(rawToken)
	}
	_ = tokenHashStored
	item.RotatedAt = parseNullableTime(rotatedRaw)
	item.RotationCount = rotationCount
	item.RevokedAt = parseNullableTime(revokedRaw)
	item.LastAccessAt = parseNullableTime(accessRaw)
	item.ExpiresAt = parseNullableTime(expiresRaw)
	if ts, err := time.Parse(time.RFC3339Nano, createdRaw); err == nil {
		item.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339Nano, updatedRaw); err == nil {
		item.UpdatedAt = ts
	}
	return item, nil
}

func (s *Store) SetSubscriptionExpiration(ctx context.Context, id string, expiresAt *time.Time) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE subscriptions
		 SET expires_at = ?, status = 'active', revoked = 0, revoked_at = NULL, updated_at = ?
		 WHERE id = ?`,
		timeToText(expiresAt),
		time.Now().UTC().Format(time.RFC3339Nano),
		id,
	)
	if err != nil {
		return dbErr("set_subscription_expiration", err)
	}
	return nil
}

func (s *Store) ReactivateSubscription(ctx context.Context, id string, expiresAt *time.Time) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE subscriptions
		 SET status = 'active',
		     revoked = 0,
		     revoked_at = NULL,
		     expires_at = ?,
		     updated_at = ?
		 WHERE id = ?`,
		timeToText(expiresAt),
		time.Now().UTC().Format(time.RFC3339Nano),
		id,
	)
	if err != nil {
		return dbErr("reactivate_subscription", err)
	}
	return nil
}

func (s *Store) RotateSubscriptionToken(ctx context.Context, id, token string) (domain.Subscription, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE subscriptions
		 SET token = '__hashed__:' || id,
		     token_hash = ?,
		     token_hint = ?,
		     status = 'active',
		     revoked = 0,
		     revoked_at = NULL,
		     rotated_at = ?,
		     rotation_count = rotation_count + 1,
		     updated_at = ?
		 WHERE id = ?`,
		hashToken(token),
		tokenHint(token),
		now,
		now,
		id,
	)
	if err != nil {
		return domain.Subscription{}, dbErr("rotate_subscription", err)
	}
	return s.GetSubscription(ctx, id)
}

func (s *Store) TouchSubscriptionAccess(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `UPDATE subscriptions SET last_access_at = ?, updated_at = ? WHERE id = ?`, now, now, id)
	if err != nil {
		return dbErr("touch_subscription_access", err)
	}
	return nil
}

func (s *Store) UpdateSubscriptionProfiles(ctx context.Context, id string, profileIDs []string) error {
	rawIDs, err := json.Marshal(profileIDs)
	if err != nil {
		return dbErr("update_subscription_profiles_encode", err)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE subscriptions SET profile_ids_json = ?, updated_at = ? WHERE id = ?`, string(rawIDs), time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return dbErr("update_subscription_profiles", err)
	}
	return nil
}

func (s *Store) CreateSubscriptionIssue(ctx context.Context, item domain.SubscriptionIssue) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO subscription_issues(id, user_id, subscription_id, token_hint, source, issued_at, expires_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)`,
		item.ID,
		item.UserID,
		item.SubscriptionID,
		item.TokenHint,
		item.Source,
		item.IssuedAt.UTC().Format(time.RFC3339Nano),
		item.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return dbErr("create_subscription_issue", err)
	}
	return nil
}

func (s *Store) ListSubscriptionIssues(ctx context.Context, userID string, limit int) ([]domain.SubscriptionIssue, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, user_id, subscription_id, COALESCE(token_hint,''), COALESCE(source,''), issued_at, expires_at
		 FROM subscription_issues
		 WHERE user_id = ?
		 ORDER BY issued_at DESC
		 LIMIT ?`,
		userID,
		limit,
	)
	if err != nil {
		return nil, dbErr("list_subscription_issues_query", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.SubscriptionIssue, 0, limit)
	for rows.Next() {
		var item domain.SubscriptionIssue
		var issuedRaw, expiresRaw string
		if err := rows.Scan(&item.ID, &item.UserID, &item.SubscriptionID, &item.TokenHint, &item.Source, &issuedRaw, &expiresRaw); err != nil {
			return nil, dbErr("list_subscription_issues_scan", err)
		}
		if ts, err := time.Parse(time.RFC3339Nano, issuedRaw); err == nil {
			item.IssuedAt = ts
		}
		if ts, err := time.Parse(time.RFC3339Nano, expiresRaw); err == nil {
			item.ExpiresAt = ts
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, dbErr("list_subscription_issues_rows", err)
	}
	return out, nil
}

func (s *Store) CleanupExpired(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	mod := fmt.Sprintf("-%d days", retentionDays)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, dbErr("cleanup_begin_tx", err)
	}
	defer func() { _ = tx.Rollback() }()

	var affected int64
	res, err := tx.ExecContext(ctx, `DELETE FROM subscription_issues WHERE julianday(issued_at) < julianday('now', ?)`, mod)
	if err != nil {
		return 0, dbErr("cleanup_issues", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		affected += n
	}
	res, err = tx.ExecContext(ctx, `DELETE FROM subscriptions WHERE revoked = 1 AND julianday(updated_at) < julianday('now', ?)`, mod)
	if err != nil {
		return 0, dbErr("cleanup_subscriptions", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		affected += n
	}
	if err := tx.Commit(); err != nil {
		return 0, dbErr("cleanup_commit", err)
	}
	return affected, nil
}

func (s *Store) getSubscriptionBy(ctx context.Context, field string, value string) (domain.Subscription, error) {
	if field != "id" && field != "token" && field != "token_hash" {
		return domain.Subscription{}, dbErr("get_subscription_field", errors.New("invalid field"))
	}
	row := s.db.QueryRowContext(
		ctx,
		fmt.Sprintf(`SELECT id, name, user_id, COALESCE(token,''), COALESCE(token_hash,''), COALESCE(token_hint,''), profile_ids_json, COALESCE(status,''), COALESCE(revoked,0), COALESCE(revoked_at,''), COALESCE(rotated_at,''), COALESCE(rotation_count,0), COALESCE(last_access_at,''), COALESCE(expires_at,''), COALESCE(created_at,''), COALESCE(updated_at,'') FROM subscriptions WHERE %s = ?`, field),
		value,
	)
	var (
		item       domain.Subscription
		rawToken   string
		tokenHashStored string
		tokenHintStored string
		rawIDs     string
		status     string
		revoked    int
		revokedRaw string
		rotatedRaw string
		rotationCount int
		accessRaw  string
		expiresRaw string
		createdRaw string
		updatedRaw string
	)
	if err := row.Scan(&item.ID, &item.Name, &item.UserID, &rawToken, &tokenHashStored, &tokenHintStored, &rawIDs, &status, &revoked, &revokedRaw, &rotatedRaw, &rotationCount, &accessRaw, &expiresRaw, &createdRaw, &updatedRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Subscription{}, profile.ErrNotFound
		}
		return domain.Subscription{}, dbErr("get_subscription_scan", err)
	}
	if err := json.Unmarshal([]byte(rawIDs), &item.ProfileIDs); err != nil {
		return domain.Subscription{}, dbErr("get_subscription_decode", err)
	}
	item.Revoked = revoked == 1
	item.Status = status
	if tokenHintStored != "" {
		item.TokenHint = tokenHintStored
	} else if rawToken != "" {
		item.TokenHint = tokenHint(rawToken)
	}
	_ = tokenHashStored
	item.RotatedAt = parseNullableTime(rotatedRaw)
	item.RotationCount = rotationCount
	item.RevokedAt = parseNullableTime(revokedRaw)
	item.LastAccessAt = parseNullableTime(accessRaw)
	item.ExpiresAt = parseNullableTime(expiresRaw)
	if ts, err := time.Parse(time.RFC3339Nano, createdRaw); err == nil {
		item.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339Nano, updatedRaw); err == nil {
		item.UpdatedAt = ts
	}
	return item, nil
}

func (s *Store) getSubscriptionByTokenHash(ctx context.Context, tokenHash string) (domain.Subscription, error) {
	return s.getSubscriptionBy(ctx, "token_hash", tokenHash)
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func tokenHint(raw string) string {
	trimmed := raw
	if len(trimmed) > 8 {
		trimmed = trimmed[:8]
	}
	return trimmed
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func primaryProtocol(p domain.Profile) string {
	for _, ep := range p.Endpoints {
		if ep.Name == p.PreferredID {
			return string(ep.Protocol)
		}
	}
	if len(p.Endpoints) == 0 {
		return ""
	}
	return string(p.Endpoints[0].Protocol)
}

func dbErr(op string, err error) error {
	if err == nil {
		return nil
	}
	telemetry.Default().DBErrorsTotal.Inc()
	return perrors.Wrap(perrors.ErrDB.Code, fmt.Sprintf("db %s failed", op), err)
}

func timeToText(ts *time.Time) string {
	if ts == nil {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func parseNullableTime(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		if unix, convErr := strconv.ParseInt(raw, 10, 64); convErr == nil {
			parsed := time.Unix(unix, 0).UTC()
			return &parsed
		}
		return nil
	}
	return &ts
}
