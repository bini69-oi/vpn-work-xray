package configgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xtls/xray-core/internal/domain"
)

func TestGenerateCreatesArtifact(t *testing.T) {
	gen := NewGenerator(t.TempDir())
	profile := domain.Profile{
		ID:        "p1",
		Name:      "secure",
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{Name: "main", Address: "vpn.example.com", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"},
		},
		PreferredID: "main",
		DNS: domain.DNSOptions{
			Primary: []string{"https://1.1.1.1/dns-query"},
		},
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries:  3,
			BaseBackoff: time.Second,
			MaxBackoff:  5 * time.Second,
		},
	}

	artifact, err := gen.Generate(profile, "main")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if artifact.Path == "" || len(artifact.Config) == 0 {
		t.Fatal("artifact should include path and config")
	}
}

func TestGenerateWithoutGeoAssetsDoesNotEmitGeoRules(t *testing.T) {
	outDir := t.TempDir()
	gen := NewGenerator(outDir, WithAssetSearchPaths(t.TempDir()))
	profile := domain.Profile{
		ID:        "p2",
		Name:      "safe",
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{Name: "main", Address: "vpn.example.com", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"},
		},
		PreferredID: "main",
		DNS: domain.DNSOptions{
			Primary: []string{"https://1.1.1.1/dns-query"},
		},
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries:  3,
			BaseBackoff: time.Second,
			MaxBackoff:  5 * time.Second,
		},
	}

	artifact, err := gen.Generate(profile, "main")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	cfg := string(artifact.Config)
	if strings.Contains(cfg, "geosite:") || strings.Contains(cfg, "geoip:") {
		t.Fatalf("expected no geo asset references, got config: %s", cfg)
	}
	if len(artifact.Warnings) == 0 {
		t.Fatal("expected warnings when geo assets are missing")
	}
}

func TestGenerateWithGeoAssetsEmitsGeoRules(t *testing.T) {
	assetsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(assetsDir, "geosite.dat"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "geoip.dat"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(t.TempDir(), WithAssetSearchPaths(assetsDir))
	profile := domain.Profile{
		ID:        "p3",
		Name:      "geo",
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{Name: "main", Address: "vpn.example.com", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"},
		},
		PreferredID: "main",
		DNS: domain.DNSOptions{
			Primary: []string{"https://1.1.1.1/dns-query"},
		},
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries:  3,
			BaseBackoff: time.Second,
			MaxBackoff:  5 * time.Second,
		},
	}
	artifact, err := gen.Generate(profile, "main")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	cfg := string(artifact.Config)
	if !strings.Contains(cfg, "geosite:cn") || !strings.Contains(cfg, "geoip:cn") {
		t.Fatalf("expected geo rules in config, got: %s", cfg)
	}
}

func TestGenerateRUPresetWithWarpSocks(t *testing.T) {
	t.Setenv("VPN_PRODUCT_ROUTING_PRESET", "ru_warp")
	t.Setenv("WARP_MODE", "socks")
	t.Setenv("WARP_SOCKS_ADDR", "127.0.0.1:40000")
	t.Setenv("VPN_PRODUCT_ROUTING_RULES_PATH", filepath.Join(t.TempDir(), "routing_rules.json"))

	assetsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(assetsDir, "geosite.dat"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "geoip.dat"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(t.TempDir(), WithAssetSearchPaths(assetsDir))
	profile := domain.Profile{
		ID:        "p-ru-warp",
		Name:      "ru",
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{Name: "main", Address: "vpn.example.com", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"},
		},
		PreferredID: "main",
		DNS: domain.DNSOptions{
			Primary: []string{"https://1.1.1.1/dns-query"},
		},
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries:  3,
			BaseBackoff: time.Second,
			MaxBackoff:  5 * time.Second,
		},
	}
	artifact, err := gen.Generate(profile, "main")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	cfg := string(artifact.Config)
	if !strings.Contains(cfg, `"tag": "warp"`) || !strings.Contains(cfg, "geosite:category-ads-all") {
		t.Fatalf("expected warp outbound and RU preset rules: %s", cfg)
	}
}
