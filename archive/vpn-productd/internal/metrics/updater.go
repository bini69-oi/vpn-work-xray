package metrics

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/xtls/xray-core/internal/storage/sqlite"
)

// UpdaterOptions configures the background metrics refresh loop.
type UpdaterOptions struct {
	Interval  time.Duration
	XrayProbe func(ctx context.Context) float64
	// XrayStatsAddr is the Xray gRPC API address (host:port) for StatsService.GetAllOnlineUsers.
	// Empty: use SQLite heuristic (subscriptions last_access_at) for vpn_product_online_users_total.
	XrayStatsAddr string
	// XrayStatsDialTimeout bounds the gRPC dial + RPC (default 3s).
	XrayStatsDialTimeout time.Duration
}

// StartUpdater runs a goroutine that refreshes gauges from SQLite and optional probes.
func StartUpdater(ctx context.Context, store *sqlite.Store, startTime time.Time, opts UpdaterOptions) {
	if opts.Interval <= 0 {
		opts.Interval = 30 * time.Second
	}
	if opts.XrayProbe == nil {
		opts.XrayProbe = defaultXrayProbe
	}
	u := &updater{
		store:     store,
		startTime: startTime,
		opts:      opts,
	}
	go u.loop(ctx)
}

type updater struct {
	store     *sqlite.Store
	startTime time.Time
	opts      UpdaterOptions

	initTraffic     bool
	lastUpload      int64
	lastDownload    int64
	prevTopLabels   map[string]labelPair
	prevSubStatuses map[string]struct{}
}

type labelPair struct {
	user string
	mail string
}

func (u *updater) loop(ctx context.Context) {
	t := time.NewTicker(u.opts.Interval)
	defer t.Stop()
	u.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			u.tick(ctx)
		}
	}
}

func (u *updater) tick(ctx context.Context) {
	stats, err := u.store.PrometheusStats(ctx)
	if err != nil {
		return
	}
	ActiveUsers.Set(float64(stats.ActiveUsers))
	online := stats.OnlineUsers
	if strings.TrimSpace(u.opts.XrayStatsAddr) != "" {
		to := u.opts.XrayStatsDialTimeout
		if to <= 0 {
			to = 3 * time.Second
		}
		if n, err := QueryXrayOnlineUserCount(ctx, strings.TrimSpace(u.opts.XrayStatsAddr), to); err == nil {
			online = n
		}
	}
	OnlineUsers.Set(float64(online))
	TotalUsers.Set(float64(stats.TotalDistinctUsers))
	SubscriptionsExpiringSoon.Set(float64(stats.ExpiringWithin24h))

	for st, n := range stats.SubscriptionsByStatus {
		SubscriptionsByStatus.WithLabelValues(st).Set(float64(n))
	}
	for st := range u.prevSubStatuses {
		if _, ok := stats.SubscriptionsByStatus[st]; !ok {
			SubscriptionsByStatus.DeleteLabelValues(st)
		}
	}
	u.prevSubStatuses = make(map[string]struct{}, len(stats.SubscriptionsByStatus))
	for st := range stats.SubscriptionsByStatus {
		u.prevSubStatuses[st] = struct{}{}
	}

	ServerUptime.Set(time.Since(u.startTime).Seconds())
	XrayStatus.Set(u.opts.XrayProbe(ctx))

	u.applyTrafficCounters(stats.TotalUploadBytes, stats.TotalDownloadBytes)
	u.applyTopTraffic(stats.TopTraffic)
}

func (u *updater) applyTrafficCounters(totalUp, totalDown int64) {
	if !u.initTraffic {
		u.lastUpload = totalUp
		u.lastDownload = totalDown
		u.initTraffic = true
		return
	}
	dUp := totalUp - u.lastUpload
	dDown := totalDown - u.lastDownload
	if dUp > 0 {
		TrafficTotalUpload.Add(float64(dUp))
	}
	if dDown > 0 {
		TrafficTotalDownload.Add(float64(dDown))
	}
	u.lastUpload = totalUp
	u.lastDownload = totalDown
}

func (u *updater) applyTopTraffic(rows []sqlite.TrafficTopRow) {
	nextKeys := make(map[string]labelPair, len(rows))
	for _, row := range rows {
		key := row.UserID + "\x00" + row.Email
		nextKeys[key] = labelPair{user: row.UserID, mail: row.Email}
		UserTrafficUpload.WithLabelValues(row.UserID, row.Email).Set(float64(row.Upload))
		UserTrafficDownload.WithLabelValues(row.UserID, row.Email).Set(float64(row.Download))
	}
	for k, pair := range u.prevTopLabels {
		if _, ok := nextKeys[k]; !ok {
			UserTrafficUpload.DeleteLabelValues(pair.user, pair.mail)
			UserTrafficDownload.DeleteLabelValues(pair.user, pair.mail)
		}
	}
	u.prevTopLabels = nextKeys
}

func defaultXrayProbe(ctx context.Context) float64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	out, err := exec.CommandContext(ctx, "systemctl", "is-active", "x-ui").Output()
	if err != nil {
		return 0
	}
	if strings.TrimSpace(string(out)) == "active" {
		return 1
	}
	return 0
}
