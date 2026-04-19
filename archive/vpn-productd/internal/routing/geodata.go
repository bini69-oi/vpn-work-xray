package routing

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GeoAssets mirrors configgen geo file presence for rule building.
type GeoAssets struct {
	GeoSite bool
	GeoIP   bool
}

// GeoFileInfo is returned by GeodataStatus.
type GeoFileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// GeodataStatus returns stat for geoip.dat and geosite.dat under dir.
func GeodataStatus(dir string) ([]GeoFileInfo, error) {
	if dir == "" {
		dir = GeodataDir()
	}
	out := make([]GeoFileInfo, 0, 2)
	for _, name := range []string{"geoip.dat", "geosite.dat"} {
		p := filepath.Join(dir, name)
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				out = append(out, GeoFileInfo{Name: name, Path: p, Size: -1})
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", p, err)
		}
		out = append(out, GeoFileInfo{
			Name:    name,
			Path:    p,
			Size:    info.Size(),
			ModTime: info.ModTime().UTC(),
		})
	}
	return out, nil
}
