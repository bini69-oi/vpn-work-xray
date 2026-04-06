package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/internal/diagnostics"
	"github.com/xtls/xray-core/internal/domain"
	"github.com/xtls/xray-core/internal/health"
	"github.com/xtls/xray-core/internal/logging"
)

type fakeSubs struct {
	item domain.Subscription
}

func (f *fakeSubs) Create(_ context.Context, item domain.Subscription) (domain.Subscription, error) {
	if item.ID == "" {
		item.ID = "sub-1"
	}
	item.Token = "tok-1"
	f.item = item
	return item, nil
}
func (f *fakeSubs) Get(_ context.Context, _ string) (domain.Subscription, error) { return f.item, nil }
func (f *fakeSubs) Revoke(_ context.Context, _ string) error {
	f.item.Revoked = true
	f.item.Status = "revoked"
	return nil
}
func (f *fakeSubs) Rotate(_ context.Context, _ string) (domain.Subscription, error) {
	f.item.Token = "tok-rotated"
	return f.item, nil
}
func (f *fakeSubs) BuildContentByToken(_ context.Context, token string) (string, domain.Subscription, error) {
	if token == "missing" || token == "revoked" || token == "expired" || token == "" || f.item.Revoked {
		return "", domain.Subscription{}, context.Canceled
	}
	return "vless://demo\nh2://demo", f.item, nil
}
func (f *fakeSubs) IssueLink30Days(_ context.Context, userID string, profileIDs []string, name string, _ string) (domain.Subscription, error) {
	item := domain.Subscription{
		ID:         "sub-issued",
		Name:       name,
		UserID:     userID,
		Token:      "tok-issued",
		ProfileIDs: profileIDs,
		Status:     "active",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	f.item = item
	return item, nil
}
func (f *fakeSubs) ListIssues(_ context.Context, userID string, _ int) ([]domain.SubscriptionIssue, error) {
	return []domain.SubscriptionIssue{{
		ID:             "issue-1",
		UserID:         userID,
		SubscriptionID: "sub-issued",
		TokenHint:      "tok-issu",
		IssuedAt:       time.Now().UTC(),
		ExpiresAt:      time.Now().UTC().Add(30 * 24 * time.Hour),
	}}, nil
}
func (f *fakeSubs) AssignProfiles(_ context.Context, _ string, profileIDs []string) error {
	f.item.ProfileIDs = profileIDs
	return nil
}
func (f *fakeSubs) GetActiveByUser(_ context.Context, userID string) (domain.Subscription, error) {
	if f.item.UserID == userID {
		return f.item, nil
	}
	return domain.Subscription{}, context.Canceled
}
func (f *fakeSubs) ExtendActiveByUser(_ context.Context, userID string, _ int) (domain.Subscription, error) {
	if f.item.UserID != userID {
		return domain.Subscription{}, context.Canceled
	}
	t := time.Now().UTC().Add(30 * 24 * time.Hour)
	f.item.ExpiresAt = &t
	return f.item, nil
}
func (f *fakeSubs) BlockActiveByUser(_ context.Context, userID string) (domain.Subscription, error) {
	if f.item.UserID != userID {
		return domain.Subscription{}, context.Canceled
	}
	f.item.Revoked = true
	f.item.Status = "revoked"
	return f.item, nil
}

func TestAdminAndPublicRouteSeparation(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	conn := fakeConn{}
	profiles := fakeProfiles{}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	subs := &fakeSubs{item: domain.Subscription{ID: "sub-1", Token: "tok-1", ProfileIDs: []string{"p1"}, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	h := NewServer(conn, profiles, diag, "token", logger, nil, subs).Handler()

	req := httptest.NewRequest(http.MethodGet, "/admin/profiles", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/public/subscriptions/tok-1", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "vless://")
}

func TestSubscriptionLifecycleEndpoints(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	conn := fakeConn{}
	profiles := fakeProfiles{}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	subs := &fakeSubs{item: domain.Subscription{ID: "sub-1", Token: "tok-1", ProfileIDs: []string{"p1"}, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	h := NewServer(conn, profiles, diag, "token", logger, nil, subs).Handler()

	req := httptest.NewRequest(http.MethodPost, "/admin/subscriptions", bytes.NewBufferString(`{"name":"n","userId":"u1","profileIds":["p1"]}`))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodPost, "/admin/subscriptions/sub-1/rotate", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer token")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodPost, "/admin/subscriptions/sub-1/revoke", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer token")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestPublicSubscriptionInvalidTokenSanitized(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	conn := fakeConn{}
	profiles := fakeProfiles{}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	subs := &fakeSubs{item: domain.Subscription{ID: "sub-1", Token: "tok-1", ProfileIDs: []string{"p1"}, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	h := NewServer(conn, profiles, diag, "token", logger, nil, subs).Handler()

	req := httptest.NewRequest(http.MethodGet, "/public/subscriptions/missing", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "subscription not found")
}

func TestPublicSubscriptionRevokedAndExpiredSanitized(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	conn := fakeConn{}
	profiles := fakeProfiles{}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	subs := &fakeSubs{item: domain.Subscription{ID: "sub-1", Token: "tok-1", ProfileIDs: []string{"p1"}, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	h := NewServer(conn, profiles, diag, "token", logger, nil, subs).Handler()

	for _, token := range []string{"revoked", "expired"} {
		req := httptest.NewRequest(http.MethodGet, "/public/subscriptions/"+token, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)
		require.Contains(t, rec.Body.String(), "subscription not found")
	}
}

func TestLegacyV1RouteHasDeprecationHeaders(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	conn := fakeConn{}
	profiles := fakeProfiles{}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	subs := &fakeSubs{item: domain.Subscription{ID: "sub-1", Token: "tok-1", ProfileIDs: []string{"p1"}, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	h := NewServer(conn, profiles, diag, "token", logger, nil, subs).Handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/profiles", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "true", rec.Header().Get("Deprecation"))
}

func TestApplyTo3XUIValidation(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	conn := fakeConn{}
	profiles := fakeProfiles{}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	subs := &fakeSubs{item: domain.Subscription{ID: "sub-1", UserID: "u1", Token: "tok-1", ProfileIDs: []string{"p1"}, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	h := NewServer(conn, profiles, diag, "token", logger, nil, subs).Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/issue/apply-to-3xui", bytes.NewBufferString(`{"userId":"u1"}`))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestIssueLinkStrictAbortsAndRevokesOnApplyFailure(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	conn := fakeConn{}
	profiles := fakeProfiles{}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	subs := &fakeSubs{item: domain.Subscription{ID: "sub-1", UserID: "u1", Token: "tok-1", ProfileIDs: []string{"p1"}, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	h := NewServer(conn, profiles, diag, "token", logger, nil, subs).WithIssueStrict(true).Handler()

	req := httptest.NewRequest(http.MethodPost, "/admin/issue/link", bytes.NewBufferString(`{"userId":"u1"}`))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.True(t, subs.item.Revoked)
}

func TestIssueLinkNonStrictReturnsResponseOnApplyFailure(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	conn := fakeConn{}
	profiles := fakeProfiles{}
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, fakeDBChecker{}, t.TempDir())
	subs := &fakeSubs{item: domain.Subscription{ID: "sub-1", UserID: "u1", Token: "tok-1", ProfileIDs: []string{"p1"}, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	h := NewServer(conn, profiles, diag, "token", logger, nil, subs).WithIssueStrict(false).Handler()

	req := httptest.NewRequest(http.MethodPost, "/admin/issue/link", bytes.NewBufferString(`{"userId":"u1"}`))
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, false, out["appliedTo3xui"])
	require.NotEmpty(t, out["applyError"])
	require.False(t, subs.item.Revoked)
}

