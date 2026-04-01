package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/product/diagnostics"
	"github.com/xtls/xray-core/product/domain"
	"github.com/xtls/xray-core/product/health"
	"github.com/xtls/xray-core/product/logging"
)

type connectionMock struct{ mock.Mock }

func (m *connectionMock) Connect(ctx context.Context, profileID string) error {
	args := m.Called(ctx, profileID)
	return args.Error(0)
}

func (m *connectionMock) Disconnect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *connectionMock) Status(ctx context.Context) domain.RuntimeStatus {
	args := m.Called(ctx)
	return args.Get(0).(domain.RuntimeStatus)
}

type profileMock struct{ mock.Mock }

func (m *profileMock) List(ctx context.Context) ([]domain.Profile, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Profile), args.Error(1)
}

func (m *profileMock) Get(ctx context.Context, id string) (domain.Profile, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.Profile), args.Error(1)
}

func (m *profileMock) Save(ctx context.Context, p domain.Profile) (domain.Profile, error) {
	args := m.Called(ctx, p)
	return args.Get(0).(domain.Profile), args.Error(1)
}

func (m *profileMock) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *profileMock) SetTrafficLimit(ctx context.Context, profileID string, limitMB int64) error {
	args := m.Called(ctx, profileID, limitMB)
	return args.Error(0)
}

func (m *profileMock) AddTrafficUsage(ctx context.Context, profileID string, uploadBytes, downloadBytes int64) error {
	args := m.Called(ctx, profileID, uploadBytes, downloadBytes)
	return args.Error(0)
}

func (m *profileMock) SetBlocked(ctx context.Context, profileID string, blocked bool) error {
	args := m.Called(ctx, profileID, blocked)
	return args.Error(0)
}

func (m *profileMock) UpsertPanelUser(ctx context.Context, user domain.PanelUser) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *profileMock) ListPanelUsers(ctx context.Context, panel string) ([]domain.PanelUser, error) {
	args := m.Called(ctx, panel)
	return args.Get(0).([]domain.PanelUser), args.Error(1)
}

func (m *profileMock) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

type dbCheckerMock struct{ mock.Mock }

func (m *dbCheckerMock) SelfCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestAuthMiddlewareVariants(t *testing.T) {
	handler := buildServerForTest(t, nil)
	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "missing token", token: "", wantStatus: http.StatusUnauthorized},
		{name: "invalid token", token: "wrong", wantStatus: http.StatusUnauthorized},
		{name: "valid token", token: "token", wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestAPIEndpointsFunctionalAndInjectionSafety(t *testing.T) {
	handler := buildServerForTest(t, errors.New("db unavailable"))
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{name: "profiles list", method: http.MethodGet, path: "/v1/profiles", wantStatus: http.StatusOK},
		{name: "profile upsert invalid json", method: http.MethodPost, path: "/v1/profiles/upsert", body: "{bad", wantStatus: http.StatusBadRequest},
		{name: "profile delete validation", method: http.MethodPost, path: "/v1/profiles/delete", body: `{}`, wantStatus: http.StatusBadRequest},
		{name: "connect sql-injection payload", method: http.MethodPost, path: "/v1/connect", body: `{"profileId":"1'; DROP TABLE profiles;--"}`, wantStatus: http.StatusBadRequest},
		{name: "disconnect", method: http.MethodPost, path: "/v1/disconnect", wantStatus: http.StatusOK},
		{name: "snapshot", method: http.MethodGet, path: "/v1/diagnostics/snapshot", wantStatus: http.StatusOK},
		{name: "account unknown", method: http.MethodGet, path: "/v1/account", wantStatus: http.StatusOK},
		{name: "account active", method: http.MethodGet, path: "/v1/account?profileId=p-active", wantStatus: http.StatusOK},
		{name: "account expired blocked", method: http.MethodGet, path: "/v1/account?profileId=p-blocked", wantStatus: http.StatusOK},
		{name: "account profile not found", method: http.MethodGet, path: "/v1/account?profileId=nope", wantStatus: http.StatusNotFound},
		{name: "quota set", method: http.MethodPost, path: "/v1/quota/set", body: `{"profileId":"p1","limitMb":1}`, wantStatus: http.StatusOK},
		{name: "quota add", method: http.MethodPost, path: "/v1/quota/add", body: `{"profileId":"p1","uploadBytes":10,"downloadBytes":11}`, wantStatus: http.StatusOK},
		{name: "quota block", method: http.MethodPost, path: "/v1/quota/block", body: `{"profileId":"p1","blocked":true}`, wantStatus: http.StatusOK},
		{name: "profile stats", method: http.MethodGet, path: "/v1/stats/profiles", wantStatus: http.StatusOK},
		{name: "panel upsert", method: http.MethodPost, path: "/v1/integration/3xui/users/upsert", body: `{"id":"u1","profileId":"p1"}`, wantStatus: http.StatusOK},
		{name: "panel list", method: http.MethodGet, path: "/v1/integration/3xui/users", wantStatus: http.StatusOK},
		{name: "panel limit ip get", method: http.MethodGet, path: "/v1/integration/3xui/limit-ip", wantStatus: http.StatusOK},
		{name: "panel limit ip set", method: http.MethodPost, path: "/v1/integration/3xui/limit-ip", body: `{"limitIp":5,"applyExisting":false}`, wantStatus: http.StatusOK},
		{name: "health unhealthy", method: http.MethodGet, path: "/v1/health", wantStatus: http.StatusServiceUnavailable},
		{name: "delivery links", method: http.MethodGet, path: "/v1/delivery/links?profileId=p1", wantStatus: http.StatusOK},
		{name: "metrics", method: http.MethodGet, path: "/v1/metrics", wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *bytes.Reader
			if tt.body == "" {
				body = bytes.NewReader(nil)
			} else {
				body = bytes.NewReader([]byte(tt.body))
			}
			req := httptest.NewRequest(tt.method, tt.path, body)
			req.Header.Set("Authorization", "Bearer token")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestMetricsEndpointRequiresToken(t *testing.T) {
	handler := buildServerForTest(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func buildServerForTest(t *testing.T, dbErr error) http.Handler {
	t.Helper()
	logger, err := logging.New("")
	require.NoError(t, err)

	conn := &connectionMock{}
	profiles := &profileMock{}
	db := &dbCheckerMock{}

	status := domain.RuntimeStatus{State: domain.StateIdle, UpdatedAt: time.Now().UTC()}
	conn.On("Status", mock.Anything).Return(status)
	conn.On("Connect", mock.Anything, mock.Anything).Return(errors.New("connect failed"))
	conn.On("Disconnect", mock.Anything).Return(nil)

	listProfiles := []domain.Profile{
		{
			ID:              "p1",
			Name:            "Profile 1",
			TrafficUsedUp:   100,
			TrafficUsedDown: 200,
			Endpoints:       []domain.Endpoint{{Name: "main", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS, UUID: "11111111-2222-3333-4444-555555555555", RealityPublicKey: "pk", RealityShortID: "ab12", ServerName: "sni.example.com"}},
		},
		{ID: "p-active", Name: "Active", Blocked: false},
		{ID: "p-blocked", Name: "Blocked", Blocked: true},
	}
	profiles.On("List", mock.Anything).Return(listProfiles, nil)
	profiles.On("Get", mock.Anything, mock.Anything).Return(domain.Profile{ID: "p1", Name: "Profile 1", PreferredID: "main", Endpoints: []domain.Endpoint{{Name: "main", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS, UUID: "11111111-2222-3333-4444-555555555555", RealityPublicKey: "pk", RealityShortID: "ab12", ServerName: "sni.example.com"}}}, nil)
	profiles.On("Save", mock.Anything, mock.Anything).Return(domain.Profile{}, nil)
	profiles.On("Delete", mock.Anything, mock.Anything).Return(nil)
	profiles.On("SetTrafficLimit", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	profiles.On("AddTrafficUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	profiles.On("SetBlocked", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	profiles.On("UpsertPanelUser", mock.Anything, mock.Anything).Return(nil)
	profiles.On("ListPanelUsers", mock.Anything, mock.Anything).Return([]domain.PanelUser{}, nil)
	profiles.On("Count", mock.Anything).Return(1, nil)

	db.On("SelfCheck", mock.Anything).Return(dbErr)
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, db, t.TempDir())
	diag.SetNetworkTargets("127.0.0.1:1")

	s := NewServer(conn, profiles, diag, "token", logger, nil, nil)
	return s.Handler()
}

func TestErrorResponseHasCode(t *testing.T) {
	handler := buildServerForTest(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/profiles", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.NotEmpty(t, out["code"])
}

func TestAPIHandlerErrorBranches(t *testing.T) {
	t.Run("profiles list error", func(t *testing.T) {
		srv, _, profiles, _ := newMockServer(t)
		profiles.ExpectedCalls = nil
		profiles.On("List", mock.Anything).Return([]domain.Profile{}, errors.New("boom"))
		req := httptest.NewRequest(http.MethodGet, "/v1/profiles", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("upsert save error", func(t *testing.T) {
		srv, _, profiles, _ := newMockServer(t)
		profiles.ExpectedCalls = nil
		profiles.On("Save", mock.Anything, mock.Anything).Return(domain.Profile{}, errors.New("save failed"))
		req := httptest.NewRequest(http.MethodPost, "/v1/profiles/upsert", bytes.NewBufferString(`{"id":"p1","name":"x"}`))
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("delete service error", func(t *testing.T) {
		srv, _, profiles, _ := newMockServer(t)
		profiles.ExpectedCalls = nil
		profiles.On("Delete", mock.Anything, "p1").Return(errors.New("delete failed"))
		req := httptest.NewRequest(http.MethodPost, "/v1/profiles/delete", bytes.NewBufferString(`{"profileId":"p1"}`))
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("connect success", func(t *testing.T) {
		srv, conn, _, _ := newMockServer(t)
		conn.ExpectedCalls = nil
		conn.On("Connect", mock.Anything, "p1").Return(nil)
		conn.On("Status", mock.Anything).Return(domain.RuntimeStatus{State: domain.StateConnected})
		req := httptest.NewRequest(http.MethodPost, "/v1/connect", bytes.NewBufferString(`{"profileId":"p1"}`))
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("disconnect error", func(t *testing.T) {
		srv, conn, _, _ := newMockServer(t)
		conn.ExpectedCalls = nil
		conn.On("Disconnect", mock.Anything).Return(errors.New("disconnect failed"))
		conn.On("Status", mock.Anything).Return(domain.RuntimeStatus{State: domain.StateIdle})
		req := httptest.NewRequest(http.MethodPost, "/v1/disconnect", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("quota endpoints service errors", func(t *testing.T) {
		srv, _, profiles, _ := newMockServer(t)
		profiles.ExpectedCalls = nil
		profiles.On("SetTrafficLimit", mock.Anything, "p1", int64(1)).Return(errors.New("q"))
		profiles.On("AddTrafficUsage", mock.Anything, "p1", int64(1), int64(1)).Return(errors.New("q"))
		profiles.On("SetBlocked", mock.Anything, "p1", true).Return(errors.New("q"))

		for _, tc := range []struct {
			path string
			body string
		}{
			{"/v1/quota/set", `{"profileId":"p1","limitMb":1}`},
			{"/v1/quota/add", `{"profileId":"p1","uploadBytes":1,"downloadBytes":1}`},
			{"/v1/quota/block", `{"profileId":"p1","blocked":true}`},
		} {
			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Authorization", "Bearer token")
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("panel service errors", func(t *testing.T) {
		srv, _, profiles, _ := newMockServer(t)
		profiles.ExpectedCalls = nil
		profiles.On("UpsertPanelUser", mock.Anything, mock.Anything).Return(errors.New("panel upsert failed"))
		profiles.On("ListPanelUsers", mock.Anything, "3x-ui").Return([]domain.PanelUser{}, errors.New("panel list failed"))

		req := httptest.NewRequest(http.MethodPost, "/v1/integration/3xui/users/upsert", bytes.NewBufferString(`{"id":"u1","profileId":"p1"}`))
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		req = httptest.NewRequest(http.MethodGet, "/v1/integration/3xui/users", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec = httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("snapshot count error", func(t *testing.T) {
		srv, _, profiles, _ := newMockServer(t)
		profiles.ExpectedCalls = nil
		profiles.On("Count", mock.Anything).Return(0, errors.New("count failed"))
		req := httptest.NewRequest(http.MethodGet, "/v1/diagnostics/snapshot", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("delivery links branches", func(t *testing.T) {
		srv, _, profiles, _ := newMockServer(t)
		profiles.ExpectedCalls = nil
		profiles.On("List", mock.Anything).Return([]domain.Profile{}, nil)
		req := httptest.NewRequest(http.MethodGet, "/v1/delivery/links?profileId=p1", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)

		req = httptest.NewRequest(http.MethodGet, "/v1/delivery/links", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec = httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		req = httptest.NewRequest(http.MethodPost, "/v1/delivery/links?profileId=p1", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec = httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)

		badProfile := domain.Profile{
			ID:        "p1",
			Name:      "bad",
			Endpoints: []domain.Endpoint{{Name: "x", Address: "1.1.1.1", Port: 443, Protocol: domain.Protocol("custom")}},
		}
		profiles.ExpectedCalls = nil
		profiles.On("List", mock.Anything).Return([]domain.Profile{badProfile}, nil)
		req = httptest.NewRequest(http.MethodGet, "/v1/delivery/links?profileId=p1", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec = httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestAPIValidationAndMethodBranches(t *testing.T) {
	handler := buildServerForTest(t, nil)
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{name: "status wrong method", method: http.MethodPost, path: "/v1/status", wantStatus: http.StatusMethodNotAllowed},
		{name: "snapshot wrong method", method: http.MethodPost, path: "/v1/diagnostics/snapshot", wantStatus: http.StatusMethodNotAllowed},
		{name: "health wrong method", method: http.MethodPost, path: "/v1/health", wantStatus: http.StatusMethodNotAllowed},
		{name: "disconnect wrong method", method: http.MethodGet, path: "/v1/disconnect", wantStatus: http.StatusMethodNotAllowed},
		{name: "quota set wrong method", method: http.MethodGet, path: "/v1/quota/set", wantStatus: http.StatusMethodNotAllowed},
		{name: "quota add wrong method", method: http.MethodGet, path: "/v1/quota/add", wantStatus: http.StatusMethodNotAllowed},
		{name: "quota block wrong method", method: http.MethodGet, path: "/v1/quota/block", wantStatus: http.StatusMethodNotAllowed},
		{name: "profile stats wrong method", method: http.MethodPost, path: "/v1/stats/profiles", wantStatus: http.StatusMethodNotAllowed},
		{name: "connect invalid json", method: http.MethodPost, path: "/v1/connect", body: "{", wantStatus: http.StatusBadRequest},
		{name: "connect missing profile", method: http.MethodPost, path: "/v1/connect", body: `{}`, wantStatus: http.StatusBadRequest},
		{name: "quota set invalid payload", method: http.MethodPost, path: "/v1/quota/set", body: `{"profileId":"","limitMb":-1}`, wantStatus: http.StatusBadRequest},
		{name: "quota add invalid payload", method: http.MethodPost, path: "/v1/quota/add", body: `{"profileId":"p1","uploadBytes":-1,"downloadBytes":0}`, wantStatus: http.StatusBadRequest},
		{name: "quota block invalid payload", method: http.MethodPost, path: "/v1/quota/block", body: `{"profileId":""}`, wantStatus: http.StatusBadRequest},
		{name: "panel upsert missing fields", method: http.MethodPost, path: "/v1/integration/3xui/users/upsert", body: `{}`, wantStatus: http.StatusBadRequest},
		{name: "panel users default panel", method: http.MethodGet, path: "/v1/integration/3xui/users?panel=", wantStatus: http.StatusOK},
		{name: "panel limit ip invalid body", method: http.MethodPost, path: "/v1/integration/3xui/limit-ip", body: "{", wantStatus: http.StatusBadRequest},
		{name: "profiles delete invalid json", method: http.MethodPost, path: "/v1/profiles/delete", body: "{", wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			req.Header.Set("Authorization", "Bearer token")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestAPISuccessAndErrorMixBranches(t *testing.T) {
	srv, conn, profiles, _ := newMockServer(t)
	conn.ExpectedCalls = nil
	profiles.ExpectedCalls = nil
	conn.On("Status", mock.Anything).Return(domain.RuntimeStatus{State: domain.StateIdle})
	conn.On("Connect", mock.Anything, "p1").Return(nil)
	conn.On("Disconnect", mock.Anything).Return(nil)
	profiles.On("Save", mock.Anything, mock.Anything).Return(domain.Profile{ID: "p1", Name: "n"}, nil)
	profiles.On("Delete", mock.Anything, "p1").Return(nil)
	profiles.On("SetTrafficLimit", mock.Anything, "p1", int64(10)).Return(nil)
	profiles.On("AddTrafficUsage", mock.Anything, "p1", int64(10), int64(10)).Return(nil)
	profiles.On("SetBlocked", mock.Anything, "p1", true).Return(nil)
	profiles.On("UpsertPanelUser", mock.Anything, mock.Anything).Return(nil)
	profiles.On("ListPanelUsers", mock.Anything, "3x-ui").Return([]domain.PanelUser{{ID: "u1"}}, nil)
	profiles.On("List", mock.Anything).Return([]domain.Profile{{ID: "p1", Name: "n", Endpoints: []domain.Endpoint{{Name: "main", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS, UUID: "11111111-2222-3333-4444-555555555555", RealityPublicKey: "pk", RealityShortID: "ab12", ServerName: "sni.example.com"}}}}, nil)
	profiles.On("Count", mock.Anything).Return(1, nil)

	cases := []struct {
		method string
		path   string
		body   string
		code   int
	}{
		{http.MethodPost, "/v1/profiles/upsert", `{"id":"p1","name":"n"}`, http.StatusOK},
		{http.MethodPost, "/v1/profiles/delete", `{"profileId":"p1"}`, http.StatusOK},
		{http.MethodPost, "/v1/connect", `{"profileId":"p1"}`, http.StatusOK},
		{http.MethodPost, "/v1/disconnect", `{}`, http.StatusOK},
		{http.MethodGet, "/v1/status", ``, http.StatusOK},
		{http.MethodGet, "/v1/diagnostics/snapshot", ``, http.StatusOK},
		{http.MethodPost, "/v1/quota/set", `{"profileId":"p1","limitMb":10}`, http.StatusOK},
		{http.MethodPost, "/v1/quota/add", `{"profileId":"p1","uploadBytes":10,"downloadBytes":10}`, http.StatusOK},
		{http.MethodPost, "/v1/quota/block", `{"profileId":"p1","blocked":true}`, http.StatusOK},
		{http.MethodGet, "/v1/stats/profiles", ``, http.StatusOK},
		{http.MethodPost, "/v1/integration/3xui/users/upsert", `{"id":"u1","profileId":"p1"}`, http.StatusOK},
		{http.MethodGet, "/v1/integration/3xui/users?panel=3x-ui", ``, http.StatusOK},
		{http.MethodGet, "/v1/integration/3xui/limit-ip", ``, http.StatusOK},
		{http.MethodPost, "/v1/integration/3xui/limit-ip", `{"limitIp":4,"applyExisting":false}`, http.StatusOK},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, tc.code, rec.Code)
	}
}

func TestRequestHelperCoverage(t *testing.T) {
	assert.Equal(t, "system", requestIDFromContext(context.Background()))
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	req.RemoteAddr = "127.0.0.1"
	assert.Equal(t, "127.0.0.1", requestRemoteAddr(req))
}

func TestRateLimitExceeded(t *testing.T) {
	handler := buildServerForTest(t, nil)
	var code int
	for i := 0; i < 130; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
		req.RemoteAddr = "10.10.10.10:12345"
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		code = rec.Code
	}
	assert.Equal(t, http.StatusTooManyRequests, code)
}

func newMockServer(t *testing.T) (*Server, *connectionMock, *profileMock, *dbCheckerMock) {
	t.Helper()
	logger, err := logging.New("")
	require.NoError(t, err)
	conn := &connectionMock{}
	profiles := &profileMock{}
	db := &dbCheckerMock{}
	conn.On("Status", mock.Anything).Return(domain.RuntimeStatus{State: domain.StateIdle})
	conn.On("Connect", mock.Anything, mock.Anything).Return(errors.New("connect failed"))
	conn.On("Disconnect", mock.Anything).Return(nil)
	profiles.On("List", mock.Anything).Return([]domain.Profile{}, nil)
	profiles.On("Get", mock.Anything, mock.Anything).Return(domain.Profile{}, nil)
	profiles.On("Save", mock.Anything, mock.Anything).Return(domain.Profile{}, nil)
	profiles.On("Delete", mock.Anything, mock.Anything).Return(nil)
	profiles.On("SetTrafficLimit", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	profiles.On("AddTrafficUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	profiles.On("SetBlocked", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	profiles.On("UpsertPanelUser", mock.Anything, mock.Anything).Return(nil)
	profiles.On("ListPanelUsers", mock.Anything, mock.Anything).Return([]domain.PanelUser{}, nil)
	profiles.On("Count", mock.Anything).Return(0, nil)
	db.On("SelfCheck", mock.Anything).Return(nil)
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, db, t.TempDir())
	s := NewServer(conn, profiles, diag, "token", logger, nil, nil)
	return s, conn, profiles, db
}
