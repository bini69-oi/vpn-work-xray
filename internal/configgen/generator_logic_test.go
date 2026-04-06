package configgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/internal/configgen/routing"
	"github.com/xtls/xray-core/internal/domain"
)

func TestBuildInbounds_WithAndWithoutTUN(t *testing.T) {
	without := buildInbounds(domain.Profile{})
	require.Len(t, without, 1)
	assert.Equal(t, "socks", without[0]["protocol"])

	with := buildInbounds(domain.Profile{TUN: domain.TUNOptions{Enabled: true, Interface: "tun88", MTU: 1400, IPv4Address: "172.19.0.1/30"}})
	require.Len(t, with, 2)
	tun := with[1]
	assert.Equal(t, "tun", tun["protocol"])
	settings := tun["settings"].(map[string]any)
	assert.Equal(t, "tun88", settings["name"])
	assert.Equal(t, 1400, settings["mtu"])
}

func TestBuildInbounds_TUNDefaults(t *testing.T) {
	with := buildInbounds(domain.Profile{TUN: domain.TUNOptions{Enabled: true}})
	require.Len(t, with, 2)
	settings := with[1]["settings"].(map[string]any)
	assert.Equal(t, "utun0", settings["name"])
	assert.Equal(t, 1500, settings["mtu"])
	assert.Equal(t, []string{"172.19.0.1/30"}, settings["address"])
}

func TestBuildRoutingRules_SmartListsAndInvertAndCountryFallback(t *testing.T) {
	g := NewGenerator(t.TempDir())
	p := domain.Profile{
		RouteMode:       domain.RouteModeSplit,
		InvertRouting:   true,
		ProxyDomains:    []string{"youtube.com"},
		ProxyCIDRs:      []string{"1.1.1.0/24"},
		DirectDomains:   []string{"example.com"},
		DirectCIDRs:     []string{"8.8.8.0/24"},
		ProxyCountries:  []string{"CN"},
		DirectCountries: []string{"US"},
	}
	warnings := []string{}
	rules := g.buildRoutingRules(p, routing.AssetsAvailability{GeoSite: false, GeoIP: false}, &warnings)
	require.NotEmpty(t, rules)
	assert.NotEmpty(t, warnings)
	last := rules[len(rules)-1]
	assert.Equal(t, "proxy", last["outboundTag"])
}

func TestBuildRoutingRules_NoSmartListsFallsBackToPresets(t *testing.T) {
	g := NewGenerator(t.TempDir())
	p := domain.Profile{RouteMode: domain.RouteModeFull}
	warnings := []string{}
	rules := g.buildRoutingRules(p, routing.AssetsAvailability{GeoSite: true, GeoIP: true}, &warnings)
	require.NotEmpty(t, rules)
	assert.Equal(t, "direct", rules[0]["outboundTag"])
}

func TestBuildCountryRules_WithAssetsAndWithoutAssets(t *testing.T) {
	warnings := []string{}
	domains, ips := buildCountryRules([]string{"CN", " "}, routing.AssetsAvailability{GeoSite: true, GeoIP: true}, &warnings)
	assert.Equal(t, []string{"geosite:cn"}, domains)
	assert.Equal(t, []string{"geoip:cn"}, ips)
	assert.Empty(t, warnings)

	warnings = []string{}
	domains, ips = buildCountryRules([]string{"US"}, routing.AssetsAvailability{}, &warnings)
	assert.Empty(t, domains)
	assert.Empty(t, ips)
	assert.NotEmpty(t, warnings)
}

func TestPickTagAndQueryStrategyAndSelectEndpoint(t *testing.T) {
	assert.Equal(t, "a", pickTag(false, "a", "b"))
	assert.Equal(t, "b", pickTag(true, "a", "b"))
	assert.Equal(t, "UseIP", queryStrategy(true))
	assert.Equal(t, "UseIPv4", queryStrategy(false))

	ep, ok := selectEndpoint(domain.Profile{Endpoints: []domain.Endpoint{{Name: "main"}}}, "main")
	assert.True(t, ok)
	assert.Equal(t, "main", ep.Name)
	_, ok = selectEndpoint(domain.Profile{Endpoints: []domain.Endpoint{{Name: "main"}}}, "none")
	assert.False(t, ok)
}

func TestFileExistsAndDetectAssets(t *testing.T) {
	assetsDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "geoip.dat"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "geosite.dat"), []byte("x"), 0o600))
	assert.True(t, fileExistsInPaths("geoip.dat", []string{assetsDir}))
	assert.False(t, fileExistsInPaths("geoip.dat", []string{t.TempDir()}))

	g := NewGenerator(t.TempDir(), WithAssetSearchPaths(assetsDir))
	assets, warnings := g.detectAssets()
	assert.True(t, assets.GeoIP)
	assert.True(t, assets.GeoSite)
	assert.Empty(t, warnings)
}
