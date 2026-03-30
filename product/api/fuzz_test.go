package api

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/xtls/xray-core/product/diagnostics"
	"github.com/xtls/xray-core/product/domain"
	"github.com/xtls/xray-core/product/health"
	"github.com/xtls/xray-core/product/logging"
)

func FuzzProfilesUpsertJSON(f *testing.F) {
	handler := mustBuildFuzzServer(f)
	f.Add(`{"id":"p1","name":"x","enabled":true}`)
	f.Add(`{"id":"1'; DROP TABLE profiles;--"}`)
	f.Add(`{`)
	f.Fuzz(func(t *testing.T, body string) {
		req := httptest.NewRequest(http.MethodPost, "/v1/profiles/upsert", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusInternalServerError {
			t.Fatalf("unexpected 500 for body %q", body)
		}
	})
}

func mustBuildFuzzServer(f *testing.F) http.Handler {
	f.Helper()
	logger, err := logging.New("")
	if err != nil {
		f.Fatalf("logger init: %v", err)
	}
	conn := &connectionMock{}
	profiles := &profileMock{}
	db := &dbCheckerMock{}

	status := domain.RuntimeStatus{State: domain.StateIdle}
	conn.On("Status", mock.Anything).Return(status)
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

	tempDir, err := os.MkdirTemp("", "vpn-api-fuzz-*")
	if err != nil {
		f.Fatalf("temp dir: %v", err)
	}
	f.Cleanup(func() { _ = os.RemoveAll(tempDir) })
	diag := diagnostics.NewService(conn, health.StaticProber{Default: health.ProbeResult{Healthy: true}}, profiles, db, tempDir)
	return NewServer(conn, profiles, diag, "token", logger, nil, nil).Handler()
}
