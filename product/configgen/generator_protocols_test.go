package configgen

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/product/domain"
)

func TestOutboundFromEndpoint_Protocols(t *testing.T) {
	tests := []struct {
		name     string
		endpoint domain.Endpoint
		assertFn func(t *testing.T, outbound map[string]any)
	}{
		{
			name: "vless reality with full fields",
			endpoint: domain.Endpoint{
				Name:               "v",
				Address:            "example.com",
				Port:               443,
				Protocol:           domain.ProtocolVLESS,
				UUID:               "11111111-2222-3333-4444-555555555555",
				Flow:               "xtls-rprx-vision",
				ServerName:         "sni.example.com",
				Fingerprint:        "chrome",
				RealityPublicKey:   "pub-key",
				RealityShortID:     "aa",
				RealityShortIDs:    []string{"bb", "cc"},
				RealityDest:        "cdn.example.com:443",
				RealityServerNames: []string{"from-array.example.com"},
				ALPN:               []string{"h2", "http/1.1"},
			},
			assertFn: func(t *testing.T, outbound map[string]any) {
				require.Equal(t, "vless", outbound["protocol"])
				stream := outbound["streamSettings"].(map[string]any)
				reality := stream["realitySettings"].(map[string]any)
				assert.Equal(t, "cdn.example.com:443", reality["dest"])
				assert.Equal(t, "from-array.example.com", reality["serverName"])
				assert.Equal(t, "aa", reality["shortId"])
				assert.Equal(t, []string{"aa", "bb", "cc"}, toStringSlice(reality["shortIds"]))
				tlsSettings := stream["tlsSettings"].(map[string]any)
				assert.Equal(t, []string{"h2", "http/1.1"}, toStringSlice(tlsSettings["alpn"]))
			},
		},
		{
			name: "hysteria2 with bandwidth and obfs",
			endpoint: domain.Endpoint{
				Name:             "hy2",
				Address:          "hy.example.com",
				Port:             8443,
				Protocol:         domain.ProtocolHysteria,
				HysteriaPassword: "pass",
				HysteriaUpMbps:   100,
				HysteriaDownMbps: 200,
				HysteriaObfs:     "salamander",
				HysteriaObfsPass: "obfs-pass",
			},
			assertFn: func(t *testing.T, outbound map[string]any) {
				require.Equal(t, "hysteria2", outbound["protocol"])
				settings := outbound["settings"].(map[string]any)
				servers := settings["servers"].([]map[string]any)
				assert.Equal(t, 100, int(servers[0]["upMbps"].(int)))
				assert.Equal(t, 200, int(servers[0]["downMbps"].(int)))
				obfs := settings["obfs"].(map[string]any)
				assert.Equal(t, "salamander", obfs["type"])
				assert.Equal(t, "obfs-pass", obfs["password"])
			},
		},
		{
			name: "wireguard fallback config",
			endpoint: domain.Endpoint{
				Name:     "wg",
				Address:  "10.0.0.1",
				Port:     51820,
				Protocol: domain.ProtocolWG,
				WireGuardSecretKey: "secret",
				WireGuardPublicKey: "public",
			},
			assertFn: func(t *testing.T, outbound map[string]any) {
				require.Equal(t, "wireguard", outbound["protocol"])
				settings := outbound["settings"].(map[string]any)
				assert.Contains(t, settings, "secretKey")
				assert.Contains(t, settings, "peers")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outbound, err := outboundFromEndpoint(tt.endpoint)
			require.NoError(t, err)
			tt.assertFn(t, outbound)
		})
	}
}

func TestGenerate_BoundaryInputsStillProduceJSON(t *testing.T) {
	gen := NewGenerator(t.TempDir(), WithAssetSearchPaths(t.TempDir()))
	profile := domain.Profile{
		ID:        "boundary",
		Name:      "boundary",
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{
				Name:      "main",
				Address:   "not-an-ip-@@@",
				Port:      443,
				Protocol:  domain.ProtocolVLESS,
				ServerTag: "proxy",
				UUID:      "11111111-2222-3333-4444-555555555555",
				ServerName: "sni.example.com",
				RealityPublicKey: "public-key",
				RealityShortID: "abcd1234",
			},
		},
		PreferredID: "main",
		DNS: domain.DNSOptions{
			Primary: []string{"https://1.1.1.1/dns-query"},
		},
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries:  1,
			BaseBackoff: time.Second,
			MaxBackoff:  2 * time.Second,
		},
	}

	artifact, err := gen.Generate(profile, "main")
	require.NoError(t, err)
	assert.True(t, json.Valid(artifact.Config))

	var cfg map[string]any
	require.NoError(t, json.Unmarshal(artifact.Config, &cfg))
	outbounds := cfg["outbounds"].([]any)
	proxy := outbounds[0].(map[string]any)
	settings := proxy["settings"].(map[string]any)
	vnext := settings["vnext"].([]any)
	user := vnext[0].(map[string]any)["users"].([]any)[0].(map[string]any)
	assert.Equal(t, "11111111-2222-3333-4444-555555555555", user["id"])
}

func TestGenerate_EndpointNotFound(t *testing.T) {
	gen := NewGenerator(t.TempDir())
	profile := domain.Profile{
		ID:        "p1",
		Name:      "x",
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "a"}},
	}
	_, err := gen.Generate(profile, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint not found")
}

func toStringSlice(value any) []string {
	items := value.([]string)
	return items
}
