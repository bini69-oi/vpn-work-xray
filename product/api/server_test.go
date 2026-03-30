package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
