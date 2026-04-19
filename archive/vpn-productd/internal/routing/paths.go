package routing

import (
	"os"
	"path/filepath"
	"strings"
)

// DataDir returns VPN_PRODUCT_DATA_DIR or the default product state directory.
func DataDir() string {
	d := strings.TrimSpace(os.Getenv("VPN_PRODUCT_DATA_DIR"))
	if d == "" {
		return "/var/lib/vpn-product"
	}
	return d
}

// GeodataDir is where geoip.dat / geosite.dat are read for routing status (API).
// VPN_PRODUCT_GEODATA_DIR overrides; otherwise VPN_PRODUCT_XRAY_BIN_DIR or /usr/local/x-ui/bin.
func GeodataDir() string {
	if d := strings.TrimSpace(os.Getenv("VPN_PRODUCT_GEODATA_DIR")); d != "" {
		return d
	}
	if d := strings.TrimSpace(os.Getenv("VPN_PRODUCT_XRAY_BIN_DIR")); d != "" {
		return d
	}
	return "/usr/local/x-ui/bin"
}

// RoutingRulesPath is the persisted custom routing JSON for PUT/GET API.
func RoutingRulesPath() string {
	if p := strings.TrimSpace(os.Getenv("VPN_PRODUCT_ROUTING_RULES_PATH")); p != "" {
		return p
	}
	return filepath.Join(DataDir(), "routing_rules.json")
}

// WarpDomainsPath is the JSON list of domains and geosite tags for the WARP routing rule.
func WarpDomainsPath() string {
	if p := strings.TrimSpace(os.Getenv("VPN_PRODUCT_WARP_DOMAINS_PATH")); p != "" {
		return p
	}
	return filepath.Join("/etc/vpn-product", "warp_domains.json")
}

// WarpEnvPath is the env file with WARP WireGuard keys (wgcf / manual).
func WarpEnvPath() string {
	if p := strings.TrimSpace(os.Getenv("VPN_PRODUCT_WARP_ENV_FILE")); p != "" {
		return p
	}
	return "/etc/vpn-product/warp.env"
}
