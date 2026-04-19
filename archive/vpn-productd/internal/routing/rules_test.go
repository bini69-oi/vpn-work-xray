package routing

import (
	"path/filepath"
	"testing"
)

func TestBuildRUPresetRulesIncludesWarpCatchAll(t *testing.T) {
	wd := DefaultWarpDomains()
	rules := BuildRUPresetRules(GeoAssets{GeoSite: true, GeoIP: true}, wd, nil)
	if len(rules) < 2 {
		t.Fatalf("expected several rules, got %d", len(rules))
	}
	last := rules[len(rules)-1]
	if last["outboundTag"] != "proxy" {
		t.Fatalf("expected last rule to proxy, got %#v", last)
	}
}

func TestBuildRUPresetRulesWithoutGeoAddsWarnings(t *testing.T) {
	var w []string
	BuildRUPresetRules(GeoAssets{}, DefaultWarpDomains(), &w)
	if len(w) == 0 {
		t.Fatal("expected warnings when geo assets are missing")
	}
}

func TestLoadSaveRoutingRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing_rules.json")
	doc := &RoutingConfigFile{
		DomainStrategy: "IPIfNonMatch",
		DomainMatcher:  "hybrid",
		Rules: []map[string]any{
			{"type": "field", "outboundTag": "direct", "domain": []string{"geosite:private"}},
		},
	}
	if err := SaveRoutingConfigFile(path, doc); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRoutingConfigFile(path)
	if err != nil || loaded == nil {
		t.Fatalf("load: err=%v doc=%v", err, loaded)
	}
	if len(loaded.Rules) != 1 {
		t.Fatal(len(loaded.Rules))
	}
}

func TestWarpSocksOutbound(t *testing.T) {
	t.Setenv("WARP_MODE", "socks")
	t.Setenv("WARP_SOCKS_ADDR", "127.0.0.1:1080")
	o, err := BuildWarpOutbound()
	if err != nil {
		t.Fatal(err)
	}
	if o["tag"] != "warp" {
		t.Fatal(o)
	}
}
