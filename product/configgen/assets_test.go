package configgen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureGeoAssetsDownloadsWhenMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))
	defer server.Close()

	origGeoSiteURL := geoSiteURL
	origGeoIPURL := geoIPURL
	defer func() {
		geoSiteURL = origGeoSiteURL
		geoIPURL = origGeoIPURL
	}()
	geoSiteURL = server.URL + "/geosite.dat"
	geoIPURL = server.URL + "/geoip.dat"

	dir := t.TempDir()
	require.NoError(t, EnsureGeoAssets(context.Background(), dir, time.Minute))
	assert.FileExists(t, filepath.Join(dir, "geosite.dat"))
	assert.FileExists(t, filepath.Join(dir, "geoip.dat"))
}

func TestEnsureGeoAssetsSkipsFreshFiles(t *testing.T) {
	dir := t.TempDir()
	sitePath := filepath.Join(dir, "geosite.dat")
	ipPath := filepath.Join(dir, "geoip.dat")
	require.NoError(t, os.WriteFile(sitePath, []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(ipPath, []byte("old"), 0o600))

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatalf("server should not be called for fresh files")
	}))
	defer server.Close()

	origGeoSiteURL := geoSiteURL
	origGeoIPURL := geoIPURL
	defer func() {
		geoSiteURL = origGeoSiteURL
		geoIPURL = origGeoIPURL
	}()
	geoSiteURL = server.URL + "/geosite.dat"
	geoIPURL = server.URL + "/geoip.dat"

	require.NoError(t, EnsureGeoAssets(context.Background(), dir, 24*time.Hour))
}

func TestDownloadFileNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	err := downloadFile(context.Background(), server.URL, filepath.Join(t.TempDir(), "x.dat"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code")
}

func TestIsFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asset.dat")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	ok, err := isFresh(path, time.Hour)
	require.NoError(t, err)
	assert.True(t, ok)
}
