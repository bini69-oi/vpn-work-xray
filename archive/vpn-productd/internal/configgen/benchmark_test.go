package configgen

import (
	"testing"
	"time"

	"github.com/xtls/xray-core/internal/domain"
)

func BenchmarkGenerateComplexVLESSReality(b *testing.B) {
	gen := NewGenerator(b.TempDir(), WithAssetSearchPaths(b.TempDir()))
	profile := domain.Profile{
		ID:        "bench-vless",
		Name:      "bench",
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{
				Name:               "primary",
				Address:            "example.com",
				Port:               443,
				Protocol:           domain.ProtocolVLESS,
				ServerTag:          "proxy",
				UUID:               "11111111-2222-3333-4444-555555555555",
				Flow:               "xtls-rprx-vision",
				ServerName:         "example.com",
				Fingerprint:        "chrome",
				RealityPublicKey:   "replace-me",
				RealityShortID:     "ab12",
				RealityShortIDs:    []string{"ab12", "cd34", "ef56"},
				RealitySpiderX:     "/",
				RealityDest:        "cdn.example.com:443",
				RealityServerNames: []string{"sni1.example.com", "sni2.example.com"},
				ALPN:               []string{"h2", "http/1.1"},
			},
			{Name: "fallback-hy2", Address: "example.com", Port: 8443, Protocol: domain.ProtocolHysteria, ServerTag: "proxy", HysteriaPassword: "pass"},
		},
		PreferredID: "primary",
		DirectDomains: []string{
			"geosite:private", "example.com", "internal.corp",
		},
		ProxyDomains: []string{"youtube.com", "instagram.com"},
		DirectCIDRs:  []string{"10.0.0.0/8", "192.168.0.0/16"},
		ProxyCIDRs:   []string{"1.1.1.0/24", "8.8.8.0/24"},
		DNS:          domain.DNSOptions{Primary: []string{"https://1.1.1.1/dns-query"}, Fallback: []string{"8.8.8.8"}, QueryIPv6: false},
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 5, BaseBackoff: time.Second, MaxBackoff: 30 * time.Second,
		},
		TUN: domain.TUNOptions{Enabled: true, Interface: "tun0", MTU: 1500, IPv4Address: "172.19.0.1/30"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := gen.Generate(profile, "primary"); err != nil {
			b.Fatalf("generate failed: %v", err)
		}
	}
}
