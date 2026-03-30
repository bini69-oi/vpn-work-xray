package routing

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/xtls/xray-core/product/domain"
)

func TestRulesForMode_NoAssets_NoGeoReferences(t *testing.T) {
	rules := RulesForMode(domain.RouteModeSplit, AssetsAvailability{})
	raw, err := json.Marshal(rules)
	if err != nil {
		t.Fatalf("marshal rules: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "geosite:") {
		t.Fatalf("unexpected geosite reference: %s", text)
	}
	if strings.Contains(text, "geoip:") {
		t.Fatalf("unexpected geoip reference: %s", text)
	}
}

func TestRulesForMode_WithAssets_ContainsGeoReferences(t *testing.T) {
	rules := RulesForMode(domain.RouteModeSplit, AssetsAvailability{GeoSite: true, GeoIP: true})
	raw, err := json.Marshal(rules)
	if err != nil {
		t.Fatalf("marshal rules: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "geosite:cn") {
		t.Fatalf("missing geosite rule: %s", text)
	}
	if !strings.Contains(text, "geoip:cn") {
		t.Fatalf("missing geoip rule: %s", text)
	}
}
