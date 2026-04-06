package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/xtls/xray-core/internal/api"
	"github.com/xtls/xray-core/internal/app"
	"github.com/xtls/xray-core/internal/metrics"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "vpn-productd error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		addr     = flag.String("listen", "127.0.0.1:8080", "daemon listen address")
		dataDir  = flag.String("data-dir", "./var/vpn-product", "product data directory")
		dbPath   = flag.String("db-path", "", "optional SQLite database path")
		logFile  = flag.String("log-file", "", "optional product log file")
		apiToken = flag.String("api-token", "", "static bearer token for API")
	)
	flag.Parse()

	token := *apiToken
	if token == "" {
		token = os.Getenv("VPN_PRODUCT_API_TOKEN")
	}
	if token == "" {
		return errors.New("api token is required (set --api-token or VPN_PRODUCT_API_TOKEN)")
	}
	adminToken := strings.TrimSpace(os.Getenv("VPN_PRODUCT_ADMIN_TOKEN"))
	adminAllowlist := strings.TrimSpace(os.Getenv("VPN_PRODUCT_ADMIN_ALLOWLIST"))
	trustedProxyCIDRs := strings.TrimSpace(os.Getenv("VPN_PRODUCT_TRUST_PROXY_CIDRS"))
	if adminAllowlist == "" {
		adminAllowlist = "127.0.0.1,::1"
	}
	xuiDBPath := strings.TrimSpace(os.Getenv("VPN_PRODUCT_3XUI_DB_PATH"))
	xuiInboundPort := 8443
	limitIP := 3
	if raw := strings.TrimSpace(os.Getenv("VPN_PRODUCT_3XUI_INBOUND_PORT")); raw != "" {
		if n, convErr := strconv.Atoi(raw); convErr == nil && n > 0 {
			xuiInboundPort = n
		}
	}
	if raw := strings.TrimSpace(os.Getenv("VPN_PRODUCT_LIMIT_IP")); raw != "" {
		if n, convErr := strconv.Atoi(raw); convErr == nil {
			limitIP = n
		}
	}
	publicBaseURL := strings.TrimSpace(os.Getenv("VPN_PRODUCT_PUBLIC_BASE_URL"))
	if err := validatePublicBaseURL(publicBaseURL); err != nil {
		return err
	}
	issueStrict := resolveIssueStrict()

	ctx := context.Background()
	if err := os.MkdirAll(*dataDir, 0o750); err != nil {
		return err
	}
	resolvedDBPath := strings.TrimSpace(*dbPath)
	if resolvedDBPath == "" {
		resolvedDBPath = strings.TrimSpace(os.Getenv("VPN_PRODUCT_DB_PATH"))
	}
	productApp, err := app.Build(ctx, app.Options{
		DataDir: *dataDir,
		DBPath:  resolvedDBPath,
		LogFile: *logFile,
	})
	if err != nil {
		return err
	}
	defer func() { _ = productApp.Store.Close() }()

	startTime := time.Now()
	metricsListen := resolveMetricsListen()
	xrayStatsAddr := strings.TrimSpace(os.Getenv("VPN_PRODUCT_XRAY_STATS_ADDR"))
	if xrayStatsAddr == "" {
		xrayStatsAddr = strings.TrimSpace(os.Getenv("VPN_PRODUCT_XRAY_API_ADDR"))
	}
	var xrayStatsTimeout time.Duration
	if raw := strings.TrimSpace(os.Getenv("VPN_PRODUCT_XRAY_STATS_TIMEOUT")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			xrayStatsTimeout = d
		}
	}
	metrics.StartUpdater(context.Background(), productApp.Store, startTime, metrics.UpdaterOptions{
		XrayStatsAddr:        xrayStatsAddr,
		XrayStatsDialTimeout: xrayStatsTimeout,
	})

	productApp.Diagnostics.SetNetworkTargets(resolveHealthTargets(*addr)...)
	server := api.NewServer(productApp.Connection, productApp.Profiles, productApp.Diagnostics, token, productApp.Logger, nil, productApp.Subscriptions).
		WithAdminToken(adminToken).
		WithAdminAllowlist(adminAllowlist).
		WithTrustedProxyCIDRs(trustedProxyCIDRs).
		With3XUI(xuiDBPath, xuiInboundPort).
		WithClientLimitIP(limitIP).
		WithIssueStrict(issueStrict).
		WithPublicMetrics(metricsListen == "")

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	var metricsServer *http.Server
	if metricsListen != "" {
		metricsServer = &http.Server{
			Addr:              metricsListen,
			Handler:           metrics.Handler(),
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			productApp.Logger.Infof("metrics listen %s", metricsListen)
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				productApp.Logger.Errorf("metrics server failed: %v", err)
			}
		}()
	}

	productApp.Logger.Infof("starting daemon at %s", *addr)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			productApp.Logger.Errorf("server failed: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if metricsServer != nil {
		_ = metricsServer.Shutdown(shutdownCtx)
	}
	_ = httpServer.Shutdown(shutdownCtx)
	return productApp.Connection.Disconnect(shutdownCtx)
}

func validatePublicBaseURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("VPN_PRODUCT_PUBLIC_BASE_URL is required and must point to your public https endpoint")
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid VPN_PRODUCT_PUBLIC_BASE_URL: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return errors.New("VPN_PRODUCT_PUBLIC_BASE_URL must start with http:// or https://")
	}
	if strings.TrimSpace(u.Host) == "" {
		return errors.New("VPN_PRODUCT_PUBLIC_BASE_URL must include host")
	}
	return nil
}

func resolveIssueStrict() bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("VPN_PRODUCT_ISSUE_STRICT")))
	if raw == "" {
		return true
	}
	switch raw {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

// resolveMetricsListen returns a dedicated listen address for Prometheus when set via env.
// Empty means expose /metrics on the main API server only.
func resolveMetricsListen() string {
	raw := strings.TrimSpace(os.Getenv("VPN_PRODUCT_METRICS_LISTEN"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("METRICS_PORT"))
	}
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, ":") {
		return "0.0.0.0" + raw
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return fmt.Sprintf("0.0.0.0:%d", n)
	}
	return raw
}

func resolveHealthTargets(listenAddr string) []string {
	override := strings.TrimSpace(os.Getenv("VPN_PRODUCT_HEALTH_TARGETS"))
	if override == "" {
		if strings.TrimSpace(listenAddr) == "" {
			return nil
		}
		return []string{listenAddr}
	}

	parts := strings.Split(override, ",")
	targets := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		targets = append(targets, item)
	}
	if len(targets) == 0 && strings.TrimSpace(listenAddr) != "" {
		return []string{listenAddr}
	}
	return targets
}
