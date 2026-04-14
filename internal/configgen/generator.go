package configgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xtls/xray-core/internal/configgen/routing"
	"github.com/xtls/xray-core/internal/domain"
	vpnrouting "github.com/xtls/xray-core/internal/routing"
)

type Artifact struct {
	ProfileID string
	Path      string
	Config    []byte
	Warnings  []string
}

type Generator struct {
	outputDir        string
	assetSearchPaths []string
}

type Option func(*Generator)

func WithAssetSearchPaths(paths ...string) Option {
	return func(g *Generator) {
		g.assetSearchPaths = append(g.assetSearchPaths, paths...)
	}
}

func NewGenerator(outputDir string, opts ...Option) *Generator {
	g := &Generator{outputDir: outputDir}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

func (g *Generator) Generate(profile domain.Profile, activeEndpoint string) (Artifact, error) {
	selected, ok := selectEndpoint(profile, activeEndpoint)
	if !ok {
		return Artifact{}, fmt.Errorf("endpoint not found: %s", activeEndpoint)
	}
	assets, warnings := g.detectAssets()

	customDoc, err := vpnrouting.LoadRoutingConfigFile(vpnrouting.RoutingRulesPath())
	if err != nil {
		return Artifact{}, fmt.Errorf("routing rules file: %w", err)
	}
	preset := strings.TrimSpace(os.Getenv("VPN_PRODUCT_ROUTING_PRESET"))

	proxyOutbound, err := outboundFromEndpoint(selected)
	if err != nil {
		return Artifact{}, err
	}
	directOutbound := map[string]any{"tag": "direct", "protocol": "freedom"}
	blockOutbound := map[string]any{"tag": "block", "protocol": "blackhole"}
	outbounds := []map[string]any{proxyOutbound, directOutbound, blockOutbound}
	if preset == "ru_warp" || preset == "ru-warp" {
		warpOut, wErr := vpnrouting.BuildWarpOutbound()
		if wErr != nil {
			return Artifact{}, fmt.Errorf("warp outbound: %w", wErr)
		}
		outbounds = []map[string]any{proxyOutbound, warpOut, directOutbound, blockOutbound}
	}

	cfg := map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": buildInbounds(profile),
		"outbounds": outbounds,
		"dns": map[string]any{
			"queryStrategy": queryStrategy(profile.DNS.QueryIPv6),
			"servers":       append(profile.DNS.Primary, profile.DNS.Fallback...),
		},
		"routing": g.buildVPNRouting(profile, assets, customDoc, preset, &warnings),
	}

	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return Artifact{}, err
	}
	if err := os.MkdirAll(g.outputDir, 0o750); err != nil {
		return Artifact{}, err
	}
	path := filepath.Join(g.outputDir, profile.ID+".json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return Artifact{}, err
	}
	return Artifact{
		ProfileID: profile.ID,
		Path:      path,
		Config:    raw,
		Warnings:  warnings,
	}, nil
}

func (g *Generator) detectAssets() (routing.AssetsAvailability, []string) {
	paths := append([]string{}, g.assetSearchPaths...)
	cwd, err := os.Getwd()
	if err == nil && cwd != "" {
		paths = append(paths, cwd)
	}
	paths = append(paths,
		g.outputDir,
		filepath.Dir(g.outputDir),
		filepath.Join(filepath.Dir(g.outputDir), "assets"),
		"/usr/local/share/xray",
		"/usr/share/xray",
	)

	geosite := fileExistsInPaths("geosite.dat", paths)
	geoip := fileExistsInPaths("geoip.dat", paths)
	warnings := []string{}
	if !geosite {
		warnings = append(warnings, "geosite.dat not found, using fallback routing rules")
	}
	if !geoip {
		warnings = append(warnings, "geoip.dat not found, using CIDR-only routing rules")
	}
	return routing.AssetsAvailability{
		GeoSite: geosite,
		GeoIP:   geoip,
	}, warnings
}

func fileExistsInPaths(name string, paths []string) bool {
	for _, base := range paths {
		if base == "" {
			continue
		}
		p := filepath.Join(base, name)
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func buildInbounds(profile domain.Profile) []map[string]any {
	inbounds := []map[string]any{
		{
			"tag":      "socks-in",
			"port":     10808,
			"listen":   "127.0.0.1",
			"protocol": "socks",
			"settings": map[string]any{"udp": true},
		},
	}
	if !profile.TUN.Enabled {
		return inbounds
	}

	iface := profile.TUN.Interface
	if iface == "" {
		iface = "utun0"
	}
	mtu := profile.TUN.MTU
	if mtu <= 0 {
		mtu = 1500
	}
	ipv4 := profile.TUN.IPv4Address
	if ipv4 == "" {
		ipv4 = "172.19.0.1/30"
	}

	inbounds = append(inbounds, map[string]any{
		"tag":      "tun-in",
		"protocol": "tun",
		"settings": map[string]any{
			"name":        iface,
			"mtu":         mtu,
			"stack":       "gvisor",
			"autoRoute":   true,
			"strictRoute": true,
			"address":     []string{ipv4},
		},
		"sniffing": map[string]any{
			"enabled":      true,
			"destOverride": []string{"http", "tls", "quic"},
		},
	})
	return inbounds
}

func (g *Generator) buildRoutingRules(profile domain.Profile, assets routing.AssetsAvailability, warnings *[]string) []map[string]any {
	hasSmartLists := len(profile.DirectDomains) > 0 || len(profile.ProxyDomains) > 0 || len(profile.DirectCIDRs) > 0 || len(profile.ProxyCIDRs) > 0 || len(profile.DirectCountries) > 0 || len(profile.ProxyCountries) > 0
	if !hasSmartLists {
		return routing.RulesForMode(profile.RouteMode, assets)
	}

	rules := make([]map[string]any, 0, 8)
	otherTag := "direct"
	if profile.InvertRouting {
		otherTag = "proxy"
	}

	if len(profile.ProxyDomains) > 0 {
		rules = append(rules, map[string]any{
			"type":        "field",
			"outboundTag": pickTag(profile.InvertRouting, "proxy", "direct"),
			"domain":      profile.ProxyDomains,
		})
	}
	if len(profile.ProxyCIDRs) > 0 {
		rules = append(rules, map[string]any{
			"type":        "field",
			"outboundTag": pickTag(profile.InvertRouting, "proxy", "direct"),
			"ip":          profile.ProxyCIDRs,
		})
	}
	if len(profile.DirectDomains) > 0 {
		rules = append(rules, map[string]any{
			"type":        "field",
			"outboundTag": pickTag(profile.InvertRouting, "direct", "proxy"),
			"domain":      profile.DirectDomains,
		})
	}
	if len(profile.DirectCIDRs) > 0 {
		rules = append(rules, map[string]any{
			"type":        "field",
			"outboundTag": pickTag(profile.InvertRouting, "direct", "proxy"),
			"ip":          profile.DirectCIDRs,
		})
	}

	if len(profile.ProxyCountries) > 0 {
		countryDomains, countryIPs := buildCountryRules(profile.ProxyCountries, assets, warnings)
		if len(countryDomains) > 0 {
			rules = append(rules, map[string]any{"type": "field", "outboundTag": pickTag(profile.InvertRouting, "proxy", "direct"), "domain": countryDomains})
		}
		if len(countryIPs) > 0 {
			rules = append(rules, map[string]any{"type": "field", "outboundTag": pickTag(profile.InvertRouting, "proxy", "direct"), "ip": countryIPs})
		}
	}
	if len(profile.DirectCountries) > 0 {
		countryDomains, countryIPs := buildCountryRules(profile.DirectCountries, assets, warnings)
		if len(countryDomains) > 0 {
			rules = append(rules, map[string]any{"type": "field", "outboundTag": pickTag(profile.InvertRouting, "direct", "proxy"), "domain": countryDomains})
		}
		if len(countryIPs) > 0 {
			rules = append(rules, map[string]any{"type": "field", "outboundTag": pickTag(profile.InvertRouting, "direct", "proxy"), "ip": countryIPs})
		}
	}

	// Final catch-all: either send all remaining traffic direct, or through VPN.
	rules = append(rules, map[string]any{
		"type":        "field",
		"outboundTag": otherTag,
		"network":     "tcp,udp",
	})

	// Keep a private traffic bypass rule at the top for safety.
	base := routing.RulesForMode(domain.RouteModeFull, assets)
	if len(base) > 0 {
		return append(base, rules...)
	}

	return rules
}

func (g *Generator) buildVPNRouting(profile domain.Profile, assets routing.AssetsAvailability, custom *vpnrouting.RoutingConfigFile, preset string, warnings *[]string) map[string]any {
	if custom != nil && len(custom.Rules) > 0 {
		ds := custom.DomainStrategy
		if ds == "" {
			ds = "IPIfNonMatch"
		}
		dm := custom.DomainMatcher
		if dm == "" {
			dm = "hybrid"
		}
		return map[string]any{
			"domainStrategy": ds,
			"domainMatcher":  dm,
			"rules":          custom.Rules,
		}
	}
	if preset == "ru_warp" || preset == "ru-warp" {
		geo := vpnrouting.GeoAssets{GeoSite: assets.GeoSite, GeoIP: assets.GeoIP}
		wd := vpnrouting.LoadWarpDomainsOrDefault(vpnrouting.WarpDomainsPath())
		rules := vpnrouting.BuildRUPresetRules(geo, wd, warnings)
		if strings.TrimSpace(os.Getenv("VPN_PRODUCT_BLOCK_QUIC")) == "1" {
			rules = append(rules, map[string]any{
				"type":        "field",
				"comment":     "Block QUIC to force TCP/TLS fallback",
				"port":        "443",
				"network":     "udp",
				"outboundTag": "block",
			})
		}
		return map[string]any{
			"domainStrategy": "IPIfNonMatch",
			"domainMatcher":  "hybrid",
			"rules":          rules,
		}
	}
	rules := g.buildRoutingRules(profile, assets, warnings)
	if strings.TrimSpace(os.Getenv("VPN_PRODUCT_BLOCK_QUIC")) == "1" {
		rules = append(rules, map[string]any{
			"type":        "field",
			"comment":     "Block QUIC to force TCP/TLS fallback",
			"port":        "443",
			"network":     "udp",
			"outboundTag": "block",
		})
	}
	return map[string]any{
		"domainStrategy": "IPIfNonMatch",
		"rules":          rules,
	}
}

func buildCountryRules(countries []string, assets routing.AssetsAvailability, warnings *[]string) ([]string, []string) {
	domains := []string{}
	ips := []string{}
	for _, country := range countries {
		cc := strings.ToLower(strings.TrimSpace(country))
		if cc == "" {
			continue
		}
		if assets.GeoSite {
			domains = append(domains, "geosite:"+cc)
		} else {
			*warnings = append(*warnings, "geosite.dat not available, country-domain routing skipped for "+cc)
		}
		if assets.GeoIP {
			ips = append(ips, "geoip:"+cc)
		} else {
			*warnings = append(*warnings, "geoip.dat not available, country-ip routing skipped for "+cc)
		}
	}
	return domains, ips
}

func pickTag(invert bool, normal string, inverted string) string {
	if invert {
		return inverted
	}
	return normal
}

func queryStrategy(ipv6 bool) string {
	if ipv6 {
		return "UseIP"
	}
	return "UseIPv4"
}

func selectEndpoint(profile domain.Profile, endpointName string) (domain.Endpoint, bool) {
	for _, ep := range profile.Endpoints {
		if ep.Name == endpointName {
			return ep, true
		}
	}
	return domain.Endpoint{}, false
}

func outboundFromEndpoint(ep domain.Endpoint) (map[string]any, error) {
	switch ep.Protocol {
	case domain.ProtocolHysteria:
		if strings.TrimSpace(ep.HysteriaPassword) == "" {
			return nil, fmt.Errorf("hysteria password is required for endpoint %s", ep.Name)
		}
		server := map[string]any{
			"address":  ep.Address,
			"port":     ep.Port,
			"password": ep.HysteriaPassword,
		}
		if ep.HysteriaUpMbps > 0 {
			server["upMbps"] = ep.HysteriaUpMbps
		}
		if ep.HysteriaDownMbps > 0 {
			server["downMbps"] = ep.HysteriaDownMbps
		}
		settings := map[string]any{
			"servers": []map[string]any{server},
		}
		if ep.HysteriaObfs != "" {
			settings["obfs"] = map[string]any{
				"type":     ep.HysteriaObfs,
				"password": ep.HysteriaObfsPass,
			}
		}
		return map[string]any{
			"tag":      "proxy",
			"protocol": "hysteria2",
			"settings": settings,
			"streamSettings": map[string]any{
				"network": "udp",
			},
		}, nil
	case domain.ProtocolWG:
		if strings.TrimSpace(ep.WireGuardSecretKey) == "" || strings.TrimSpace(ep.WireGuardPublicKey) == "" {
			return nil, fmt.Errorf("wireguard keys are required for endpoint %s", ep.Name)
		}
		localIP := ep.WireGuardLocalIP
		if localIP == "" {
			localIP = "10.0.0.2/32"
		}
		return map[string]any{
			"tag":      "proxy",
			"protocol": "wireguard",
			"settings": map[string]any{
				"secretKey":      ep.WireGuardSecretKey,
				"address":        []string{localIP},
				"peers":          []map[string]any{{"endpoint": fmt.Sprintf("%s:%d", ep.Address, ep.Port), "publicKey": ep.WireGuardPublicKey}},
				"mtu":            1420,
				"reserved":       []int{0, 0, 0},
				"workers":        2,
				"domainStrategy": "ForceIP",
			},
		}, nil
	default:
		// Default secure-performance protocol baseline.
		uuid := ep.UUID
		if uuid == "" {
			return nil, fmt.Errorf("uuid is required for endpoint %s", ep.Name)
		}
		flow := ep.Flow
		if flow == "" {
			flow = "xtls-rprx-vision"
		}
		serverName := ep.ServerName
		if serverName == "" {
			serverName = "example.com"
		}
		fingerprint := ep.Fingerprint
		if fingerprint == "" {
			fingerprint = "chrome"
		}
		publicKey := ep.RealityPublicKey
		if publicKey == "" {
			return nil, fmt.Errorf("reality public key is required for endpoint %s", ep.Name)
		}
		shortIDs := make([]string, 0, len(ep.RealityShortIDs)+1)
		for _, candidate := range ep.RealityShortIDs {
			if candidate == "" {
				continue
			}
			shortIDs = append(shortIDs, candidate)
		}
		if ep.RealityShortID != "" && !containsString(shortIDs, ep.RealityShortID) {
			shortIDs = append([]string{ep.RealityShortID}, shortIDs...)
		}
		if len(ep.RealityServerNames) > 0 {
			serverName = ep.RealityServerNames[0]
		}
		spiderX := ep.RealitySpiderX
		if spiderX == "" {
			spiderX = "/"
		}
		realitySettings := map[string]any{
			"show":        false,
			"serverName":  serverName,
			"publicKey":   publicKey,
			"fingerprint": fingerprint,
			"spiderX":     spiderX,
		}
		if ep.RealityDest != "" {
			realitySettings["dest"] = ep.RealityDest
		}
		if len(shortIDs) > 0 {
			realitySettings["shortId"] = shortIDs[0]
			realitySettings["shortIds"] = shortIDs
		}
		streamSettings := map[string]any{
			"network":         "tcp",
			"security":        "reality",
			"realitySettings": realitySettings,
			"sockopt": map[string]any{
				"happyEyeballs": map[string]any{
					"prioritizeIpv6":  false,
					"interleave":      1,
					"tryDelayMs":      200,
					"maxConcurrentTry": 2,
				},
			},
		}
		if len(ep.ALPN) > 0 {
			streamSettings["tlsSettings"] = map[string]any{
				"alpn": ep.ALPN,
			}
		}
		return map[string]any{
			"tag":      "proxy",
			"protocol": "vless",
			"settings": map[string]any{
				"vnext": []map[string]any{
					{
						"address": ep.Address,
						"port":    ep.Port,
						"users": []map[string]any{
							{
								"id":         uuid,
								"encryption": "none",
								"flow":       flow,
							},
						},
					},
				},
			},
			"streamSettings": streamSettings,
		}, nil
	}
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
