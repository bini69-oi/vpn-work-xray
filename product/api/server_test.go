package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	v1 "github.com/xtls/xray-core/product/api/v1"
	"github.com/xtls/xray-core/product/diagnostics"
	"github.com/xtls/xray-core/product/domain"
	"github.com/xtls/xray-core/product/health"
	"github.com/xtls/xray-core/product/logging"
)

type fakeConn struct{}

func (fakeConn) Connect(context.Context, string) error { return nil }
func (fakeConn) Disconnect(context.Context) error      { return nil }
func (fakeConn) Status(context.Context) domain.RuntimeStatus {
	return domain.RuntimeStatus{State: domain.StateIdle, UpdatedAt: time.Now().UTC()}
}

type fakeProfiles struct{}
type fakeDBChecker struct{}

func (fakeProfiles) List(context.Context) ([]domain.Profile, error) { return []domain.Profile{}, nil }
func (fakeProfiles) Get(context.Context, string) (domain.Profile, error) {
	return domain.Profile{}, nil
}
func (fakeProfiles) Count(context.Context) (int, error)             { return 0, nil }
func (fakeProfiles) Save(_ context.Context, p domain.Profile) (domain.Profile, error) {
	return p, nil
}
func (fakeProfiles) Delete(context.Context, string) error { return nil }
func (fakeProfiles) SetTrafficLimit(context.Context, string, int64) error {
	return nil
}
func (fakeProfiles) AddTrafficUsage(context.Context, string, int64, int64) error {
	return nil
}
func (fakeProfiles) SetBlocked(context.Context, string, bool) error {
	return nil
}
func (fakeProfiles) UpsertPanelUser(context.Context, domain.PanelUser) error {
	return nil
}
func (fakeProfiles) ListPanelUsers(context.Context, string) ([]domain.PanelUser, error) {
	return []domain.PanelUser{}, nil
}
func (fakeDBChecker) SelfCheck(context.Context) error { return nil }

func TestServerStatusEndpoint(t *testing.T) {
	conn := fakeConn{}
	profiles := fakeProfiles{}
	logger, err := logging.New("")
	if err != nil {
		t.Fatal(err)
	}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	srv := NewServer(conn, profiles, diag, "token", logger, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestServerRejectsMissingToken(t *testing.T) {
	conn := fakeConn{}
	profiles := fakeProfiles{}
	logger, err := logging.New("")
	if err != nil {
		t.Fatal(err)
	}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	srv := NewServer(conn, profiles, diag, "token", logger, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", w.Code)
	}
}

func TestAdminUsesDedicatedTokenWhenConfigured(t *testing.T) {
	conn := fakeConn{}
	profiles := fakeProfiles{}
	logger, err := logging.New("")
	if err != nil {
		t.Fatal(err)
	}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	srv := NewServer(conn, profiles, diag, "api-token", logger, nil, nil).WithAdminToken("admin-token")

	req := httptest.NewRequest(http.MethodGet, "/admin/health", nil)
	req.Header.Set("Authorization", "Bearer api-token")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for admin with api token, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/health", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected admin auth to pass, got %d body=%s", w.Code, strings.TrimSpace(w.Body.String()))
	}
}

func TestV1AdminRoutesRespectAllowlist(t *testing.T) {
	conn := fakeConn{}
	profiles := fakeProfiles{}
	logger, err := logging.New("")
	if err != nil {
		t.Fatal(err)
	}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	srv := NewServer(conn, profiles, diag, "api-token", logger, nil, nil).
		WithAdminToken("admin-token").
		WithAdminAllowlist("10.0.0.0/24")

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Header.Set("Authorization", "Bearer api-token")
	req.RemoteAddr = "203.0.113.10:45555"
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected v1 admin route to respect allowlist and return forbidden, got %d", w.Code)
	}
}

func TestSafeSubscriptionURLFromRequestPrefersPublicBaseURL(t *testing.T) {
	t.Setenv("VPN_PRODUCT_PUBLIC_BASE_URL", "https://198-13-186-190.sslip.io")
	req := httptest.NewRequest(http.MethodPost, "/admin/issue/link", nil)
	req.Host = "127.0.0.1:8080"
	got := safeSubscriptionURLFromRequest(req, "tok123")
	want := "https://198-13-186-190.sslip.io/public/subscriptions/tok123"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSafeSubscriptionURLFromRequestRejectsLoopbackHostWithoutPublicBaseURL(t *testing.T) {
	t.Setenv("VPN_PRODUCT_PUBLIC_BASE_URL", "")
	req := httptest.NewRequest(http.MethodPost, "/admin/issue/link", nil)
	req.Host = "127.0.0.1:8080"
	got := safeSubscriptionURLFromRequest(req, "tok123")
	if got != "" {
		t.Fatalf("expected empty URL for loopback host, got %q", got)
	}
}

func TestRequestClientIPDoesNotTrustXFFWithoutTrustedProxyList(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.RemoteAddr = "198.51.100.10:4444"
	req.Header.Set("X-Forwarded-For", "10.0.0.7")
	got := requestClientIP(req, nil)
	if got != "198.51.100.10" {
		t.Fatalf("expected remote addr when no trusted proxies configured, got %q", got)
	}
}

func TestIssueCacheCompactionBoundsGrowth(t *testing.T) {
	conn := fakeConn{}
	profiles := fakeProfiles{}
	logger, err := logging.New("")
	if err != nil {
		t.Fatal(err)
	}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	srv := NewServer(conn, profiles, diag, "token", logger, nil, nil)
	for i := 0; i < 4300; i++ {
		srv.setIssueCache("u|"+strconv.Itoa(i), v1.IssueLinkResponse{})
	}
	if len(srv.issueByIDKey) > 4096 {
		t.Fatalf("expected bounded cache size <= 4096, got %d", len(srv.issueByIDKey))
	}
}
