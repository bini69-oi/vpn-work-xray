package diagnostics

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/xtls/xray-core/internal/domain"
	"github.com/xtls/xray-core/internal/health"
)

type Snapshot struct {
	Runtime      domain.RuntimeStatus `json:"runtime"`
	Health       health.ProbeResult   `json:"health"`
	ProfileCount int                  `json:"profileCount"`
	GeneratedAt  time.Time            `json:"generatedAt"`
}

type StatusProvider interface {
	Status(ctx context.Context) domain.RuntimeStatus
}

type ProfilesProvider interface {
	Count(ctx context.Context) (int, error)
}

type Service struct {
	status         StatusProvider
	prober         health.Prober
	profiles       ProfilesProvider
	dbChecker      DBChecker
	assetsDir      string
	networkTargets []string
}

type DBChecker interface {
	SelfCheck(ctx context.Context) error
}

type HealthReport struct {
	Status  string         `json:"status"`
	Details map[string]any `json:"details"`
}

func (r HealthReport) Healthy() bool {
	return r.Status == "healthy"
}

func NewService(status StatusProvider, prober health.Prober, profiles ProfilesProvider, dbChecker DBChecker, assetsDir string) *Service {
	return &Service{
		status:    status,
		prober:    prober,
		profiles:  profiles,
		dbChecker: dbChecker,
		assetsDir: assetsDir,
	}
}

func (s *Service) SetNetworkTargets(targets ...string) {
	out := make([]string, 0, len(targets))
	for _, item := range targets {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	s.networkTargets = out
}

func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	count, err := s.profiles.Count(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		Runtime:      s.status.Status(ctx),
		Health:       s.prober.Probe(ctx),
		ProfileCount: count,
		GeneratedAt:  time.Now().UTC(),
	}, nil
}

func (s *Service) SelfCheck(ctx context.Context) HealthReport {
	report := HealthReport{
		Status:  "healthy",
		Details: map[string]any{},
	}
	if err := s.dbChecker.SelfCheck(ctx); err != nil {
		report.Status = "unhealthy"
		report.Details["db"] = map[string]any{"ok": false, "error": err.Error()}
	} else {
		report.Details["db"] = map[string]any{"ok": true}
	}

	runtimeStatus := s.status.Status(ctx)
	runtimeOK, runtimeInfo := runtimeProcessCheck(ctx, runtimeStatus)
	report.Details["runtime"] = runtimeInfo
	if !runtimeOK {
		report.Status = "unhealthy"
	}

	assetsOK, assetsInfo := checkAssets(s.assetsDir)
	report.Details["assets"] = assetsInfo
	if !assetsOK {
		report.Status = "unhealthy"
	}

	netOK, netInfo := checkNetworkTargets(s.networkTargets)
	report.Details["network"] = netInfo
	if !netOK {
		report.Status = "unhealthy"
	}

	report.Details["runtime_status"] = runtimeStatus
	report.Details["checked_at"] = time.Now().UTC()
	return report
}

func runtimeProcessCheck(ctx context.Context, status domain.RuntimeStatus) (bool, map[string]any) {
	cmd := exec.CommandContext(ctx, "ps", "-axo", "pid=,command=")
	output, err := cmd.Output()
	if err != nil {
		return false, map[string]any{"ok": false, "error": err.Error()}
	}
	needle := "xray"
	embeddedNeedle := "vpn-productd"
	lines := strings.Split(string(output), "\n")
	found := false
	foundEmbedded := false
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, needle) {
			found = true
		}
		if strings.Contains(lower, embeddedNeedle) {
			foundEmbedded = true
		}
	}
	ok := found || foundEmbedded || status.State == domain.StateConnected || status.State == domain.StateConnecting || status.State == domain.StateReconnecting
	return ok, map[string]any{
		"ok":                ok,
		"xray_process_seen": found,
		"embedded_seen":     foundEmbedded,
		"mode":              "embedded-xray-runtime",
	}
}

func checkAssets(assetsDir string) (bool, map[string]any) {
	required := []string{"geoip.dat", "geosite.dat"}
	missing := []string{}
	for _, item := range required {
		if _, err := os.Stat(filepath.Join(assetsDir, item)); err != nil {
			missing = append(missing, item)
		}
	}
	return len(missing) == 0, map[string]any{
		"ok":      len(missing) == 0,
		"dir":     assetsDir,
		"missing": missing,
	}
}

func checkNetworkTargets(targets []string) (bool, map[string]any) {
	type targetStatus struct {
		Target string `json:"target"`
		OK     bool   `json:"ok"`
		Error  string `json:"error,omitempty"`
	}
	items := make([]targetStatus, 0, len(targets))
	allOK := true
	for _, target := range targets {
		conn, err := net.DialTimeout("tcp", target, 800*time.Millisecond)
		if err != nil {
			items = append(items, targetStatus{Target: target, OK: false, Error: err.Error()})
			allOK = false
			continue
		}
		_ = conn.Close()
		items = append(items, targetStatus{Target: target, OK: true})
	}
	return allOK, map[string]any{
		"ok":      allOK,
		"targets": items,
	}
}
