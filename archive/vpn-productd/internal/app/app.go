package app

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xtls/xray-core/internal/configgen"
	"github.com/xtls/xray-core/internal/connection"
	"github.com/xtls/xray-core/internal/delivery"
	"github.com/xtls/xray-core/internal/diagnostics"
	"github.com/xtls/xray-core/internal/health"
	"github.com/xtls/xray-core/internal/integration/xui"
	"github.com/xtls/xray-core/internal/logging"
	"github.com/xtls/xray-core/internal/profile"
	"github.com/xtls/xray-core/internal/reconnect"
	"github.com/xtls/xray-core/internal/storage/sqlite"
	"github.com/xtls/xray-core/internal/subscription"
)

type App struct {
	Store       *sqlite.Store
	Profiles    *profile.Service
	Connection  *connection.Manager
	Subscriptions *subscription.Service
	Diagnostics *diagnostics.Service
	Logger      *logging.Logger
}

type Options struct {
	DataDir string
	DBPath  string
	LogFile string
}

func Build(ctx context.Context, opts Options) (*App, error) {
	log, err := logging.New(opts.LogFile)
	if err != nil {
		return nil, err
	}
	dbPath := opts.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(opts.DataDir, "product.db")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return nil, err
	}
	store, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	profiles := profile.NewService(store)
	if err := bootstrapProfiles(ctx, profiles); err != nil {
		return nil, err
	}
	if err := profiles.RotateRealityShortIDs(ctx, 7*24*time.Hour); err != nil {
		log.WithModule("profile").Warnf("shortId rotation skipped: %v", err)
	}
	assetsDir := filepath.Join(opts.DataDir, "assets")
	assetsCtx, cancelAssets := context.WithTimeout(ctx, 5*time.Second)
	if err := configgen.EnsureGeoAssets(assetsCtx, assetsDir, 24*time.Hour); err != nil {
		log.WithModule("configgen").Warnf("geo assets update skipped: %v", err)
	}
	cancelAssets()
	gen := configgen.NewGenerator(
		filepath.Join(opts.DataDir, "runtime", "generated"),
		configgen.WithAssetSearchPaths(opts.DataDir, assetsDir),
	)
	runtime := connection.NewXrayRuntime()
	reconnectEngine := reconnect.NewEngine(time.Now().UnixNano())
	conn := connection.NewManager(profiles, gen, runtime, reconnectEngine, log.WithModule("connection"), log.WithModule("configgen"))
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, store, assetsDir)
	xuiDBPath := strings.TrimSpace(os.Getenv("VPN_PRODUCT_3XUI_DB_PATH"))
	if xuiDBPath == "" {
		xuiDBPath = "/etc/x-ui/x-ui.db"
	}
	xuiInboundPort := 8443
	if raw := strings.TrimSpace(os.Getenv("VPN_PRODUCT_3XUI_INBOUND_PORT")); raw != "" {
		if n, convErr := strconv.Atoi(raw); convErr == nil && n > 0 {
			xuiInboundPort = n
		}
	}
	diag.SetXUIChecker(xui.NewChecker(xuiDBPath, xuiInboundPort))
	subs := subscription.NewService(store, profiles, delivery.NewService())

	return &App{
		Store:       store,
		Profiles:    profiles,
		Connection:  conn,
		Subscriptions: subs,
		Diagnostics: diag,
		Logger:      log,
	}, nil
}

func bootstrapProfiles(ctx context.Context, profiles *profile.Service) error {
	count, err := profiles.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	// Do not auto-create placeholder profiles; force explicit secure provisioning.
	return nil
}
