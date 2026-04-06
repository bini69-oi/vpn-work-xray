package sqlite

import (
	"context"
	"time"
)

// PrometheusStats aggregates product.db figures for Prometheus gauges.
type PrometheusStats struct {
	TotalUploadBytes        int64
	TotalDownloadBytes      int64
	ActiveUsers             int64
	TotalDistinctUsers      int64
	OnlineUsers             int64
	SubscriptionsByStatus   map[string]int64
	ExpiringWithin24h       int64
	TopTraffic              []TrafficTopRow
}

// TrafficTopRow is one row for top-N traffic gauges (labels user_id, email).
type TrafficTopRow struct {
	UserID   string
	Email    string
	Upload   int64
	Download int64
}

// PrometheusStats reads subscription, quota, and panel user data for metrics export.
func (s *Store) PrometheusStats(ctx context.Context) (*PrometheusStats, error) {
	out := &PrometheusStats{
		SubscriptionsByStatus: map[string]int64{},
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(used_upload_bytes),0), COALESCE(SUM(used_download_bytes),0) FROM profile_quota`,
	).Scan(&out.TotalUploadBytes, &out.TotalDownloadBytes); err != nil {
		return nil, dbErr("prometheus_stats_totals", err)
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM subscriptions WHERE revoked = 0 AND status = 'active'
		 AND (expires_at IS NULL OR expires_at = '' OR expires_at > ?)`,
		time.Now().UTC().Format(time.RFC3339Nano),
	).Scan(&out.ActiveUsers); err != nil {
		return nil, dbErr("prometheus_stats_active_users", err)
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM subscriptions`,
	).Scan(&out.TotalDistinctUsers); err != nil {
		return nil, dbErr("prometheus_stats_total_users", err)
	}

	onlineSince := time.Now().UTC().Add(-15 * time.Minute).Format(time.RFC3339Nano)
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM subscriptions
		 WHERE revoked = 0 AND last_access_at IS NOT NULL AND last_access_at != '' AND last_access_at > ?`,
		onlineSince,
	).Scan(&out.OnlineUsers); err != nil {
		return nil, dbErr("prometheus_stats_online_users", err)
	}

	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(status,''), COUNT(*) FROM subscriptions GROUP BY status`)
	if err != nil {
		return nil, dbErr("prometheus_stats_sub_status", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var st string
		var n int64
		if err := rows.Scan(&st, &n); err != nil {
			return nil, dbErr("prometheus_stats_sub_status_scan", err)
		}
		out.SubscriptionsByStatus[st] = n
	}
	if err := rows.Err(); err != nil {
		return nil, dbErr("prometheus_stats_sub_status_rows", err)
	}

	now := time.Now().UTC()
	until := now.Add(24 * time.Hour)
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM subscriptions
		 WHERE revoked = 0 AND status = 'active'
		 AND expires_at IS NOT NULL AND expires_at != ''
		 AND expires_at > ? AND expires_at <= ?`,
		now.Format(time.RFC3339Nano),
		until.Format(time.RFC3339Nano),
	).Scan(&out.ExpiringWithin24h); err != nil {
		return nil, dbErr("prometheus_stats_expiring", err)
	}

	topRows, err := s.db.QueryContext(ctx,
		`SELECT pq.profile_id,
		        COALESCE(pu.id, ''),
		        COALESCE(pu.username, ''),
		        COALESCE(pq.used_upload_bytes, 0),
		        COALESCE(pq.used_download_bytes, 0)
		 FROM profile_quota pq
		 LEFT JOIN panel_users pu ON pu.profile_id = pq.profile_id
		 ORDER BY (COALESCE(pq.used_upload_bytes,0) + COALESCE(pq.used_download_bytes,0)) DESC
		 LIMIT 10`,
	)
	if err != nil {
		return nil, dbErr("prometheus_stats_top_traffic", err)
	}
	defer func() { _ = topRows.Close() }()
	for topRows.Next() {
		var profileID, panelUserID, username string
		var up, down int64
		if err := topRows.Scan(&profileID, &panelUserID, &username, &up, &down); err != nil {
			return nil, dbErr("prometheus_stats_top_traffic_scan", err)
		}
		uid := panelUserID
		if uid == "" {
			uid = profileID
		}
		out.TopTraffic = append(out.TopTraffic, TrafficTopRow{
			UserID:   uid,
			Email:    username,
			Upload:   up,
			Download: down,
		})
	}
	if err := topRows.Err(); err != nil {
		return nil, dbErr("prometheus_stats_top_traffic_rows", err)
	}

	return out, nil
}
