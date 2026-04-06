package configgen

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultGeoSiteURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
	defaultGeoIPURL   = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
)

var (
	geoSiteURL = defaultGeoSiteURL
	geoIPURL   = defaultGeoIPURL
	httpClient = http.DefaultClient
)

func EnsureGeoAssets(ctx context.Context, targetDir string, maxAge time.Duration) error {
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return err
	}
	for _, item := range []struct {
		name string
		url  string
	}{
		{name: "geosite.dat", url: geoSiteURL},
		{name: "geoip.dat", url: geoIPURL},
	} {
		path := filepath.Join(targetDir, item.name)
		fresh, err := isFresh(path, maxAge)
		if err == nil && fresh {
			continue
		}
		if err := downloadFile(ctx, item.url, path); err != nil {
			return fmt.Errorf("download %s: %w", item.name, err)
		}
	}
	return nil
}

// ForceRefreshGeoAssets downloads geoip.dat and geosite.dat regardless of file age.
func ForceRefreshGeoAssets(ctx context.Context, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return err
	}
	for _, item := range []struct {
		name string
		url  string
	}{
		{name: "geosite.dat", url: geoSiteURL},
		{name: "geoip.dat", url: geoIPURL},
	} {
		path := filepath.Join(targetDir, item.name)
		if err := downloadFile(ctx, item.url, path); err != nil {
			return fmt.Errorf("download %s: %w", item.name, err)
		}
	}
	return nil
}

func isFresh(path string, maxAge time.Duration) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return time.Since(info.ModTime()) <= maxAge, nil
}

func downloadFile(ctx context.Context, url string, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	tmpPath := path + ".tmp"
	// #nosec G304 -- tmpPath is produced internally from a controlled base path.
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
