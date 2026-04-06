package routing

import "github.com/xtls/xray-core/internal/domain"

type AssetsAvailability struct {
	GeoSite bool
	GeoIP   bool
}

func RulesForMode(mode domain.RouteMode, assets AssetsAvailability) []map[string]any {
	privateCIDRs := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}

	directDomains := []string{}
	if assets.GeoSite {
		directDomains = append(directDomains, "geosite:private")
	}
	directIPs := append([]string{}, privateCIDRs...)
	if assets.GeoIP {
		directIPs = append(directIPs, "geoip:private")
	}

	if mode == domain.RouteModeFull {
		rules := []map[string]any{}
		if len(directDomains) > 0 {
			rules = append(rules, map[string]any{
				"type":        "field",
				"outboundTag": "direct",
				"domain":      directDomains,
			})
		}
		rules = append(rules, map[string]any{
			"type":        "field",
			"outboundTag": "direct",
			"ip":          directIPs,
		})
		return rules
	}

	rules := []map[string]any{}
	if assets.GeoSite {
		rules = append(rules, map[string]any{
			"type":        "field",
			"outboundTag": "direct",
			"domain":      []string{"geosite:cn", "geosite:private"},
		})
	}
	if assets.GeoIP {
		rules = append(rules, map[string]any{
			"type":        "field",
			"outboundTag": "direct",
			"ip":          []string{"geoip:private", "geoip:cn"},
		})
	} else {
		rules = append(rules, map[string]any{
			"type":        "field",
			"outboundTag": "direct",
			"ip":          directIPs,
		})
	}
	return rules
}
