package routing

import "strings"

// BuildRUPresetRules builds routing rules for VPN_PRODUCT_ROUTING_PRESET=ru_warp (Russia split + WARP for blocked sites).
func BuildRUPresetRules(assets GeoAssets, wd WarpDomainsFile, warnings *[]string) []map[string]any {
	rules := make([]map[string]any, 0, 8)

	rules = append(rules, map[string]any{
		"type":        "field",
		"comment":     "DNS-запросы через proxy (защита от подмены)",
		"port":        "53",
		"outboundTag": "proxy",
	})

	if assets.GeoSite {
		rules = append(rules, map[string]any{
			"type":        "field",
			"comment":     "Блокировка рекламы",
			"domain":      []string{"geosite:category-ads-all"},
			"outboundTag": "block",
		})
	} else if warnings != nil {
		*warnings = append(*warnings, "geosite.dat not found: skipping category-ads-all block rule")
	}

	if assets.GeoSite {
		rules = append(rules, map[string]any{
			"type":        "field",
			"comment":     "Российские домены — напрямую (не через VPN)",
			"domain": []string{
				"geosite:category-ru",
				"geosite:yandex",
				"geosite:mailru",
				"geosite:vk",
			},
			"outboundTag": "direct",
		})
	} else if warnings != nil {
		*warnings = append(*warnings, "geosite.dat not found: skipping RU direct domain rules")
	}

	if assets.GeoIP {
		rules = append(rules, map[string]any{
			"type":        "field",
			"comment":     "Российские IP — напрямую",
			"ip":          []string{"geoip:ru"},
			"outboundTag": "direct",
		})
	} else if warnings != nil {
		*warnings = append(*warnings, "geoip.dat not found: skipping geoip:ru rule")
	}

	if assets.GeoIP {
		rules = append(rules, map[string]any{
			"type":        "field",
			"comment":     "Приватные сети — напрямую",
			"ip":          []string{"geoip:private"},
			"outboundTag": "direct",
		})
	} else if warnings != nil {
		*warnings = append(*warnings, "geoip.dat not found: skipping geoip:private rule")
	}

	warpDomains := make([]string, 0, len(wd.Domains)+len(wd.GeositeTags))
	for _, d := range wd.Domains {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if strings.Contains(d, ":") {
			warpDomains = append(warpDomains, d)
			continue
		}
		warpDomains = append(warpDomains, "domain:"+d)
	}
	for _, tag := range wd.GeositeTags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if strings.HasPrefix(tag, "geosite:") {
			warpDomains = append(warpDomains, tag)
			continue
		}
		warpDomains = append(warpDomains, "geosite:"+tag)
	}
	if len(warpDomains) == 0 {
		return BuildRUPresetRules(assets, DefaultWarpDomains(), warnings)
	}

	rules = append(rules, map[string]any{
		"type":        "field",
		"comment":     "Домены заблокированные в РФ — через WARP",
		"domain":      warpDomains,
		"outboundTag": "warp",
	})

	rules = append(rules, map[string]any{
		"type":        "field",
		"comment":     "Всё остальное — через proxy (основной outbound)",
		"port":        "0-65535",
		"outboundTag": "proxy",
	})

	return rules
}
