package delivery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/product/domain"
)

func TestGenerateLinkProtocols(t *testing.T) {
	service := NewService()
	profile := domain.Profile{
		Name: "Premium Plan",
		Endpoints: []domain.Endpoint{
			{Name: "v", Address: "example.com", Port: 443, Protocol: domain.ProtocolVLESS, UUID: "u1", RealityPublicKey: "pk", RealityShortID: "ab12", ServerName: "sni.example.com"},
			{Name: "h", Address: "hy.example.com", Port: 8443, Protocol: domain.ProtocolHysteria, HysteriaPassword: "pass", ServerName: "hy.example.com"},
			{Name: "w", Address: "wg.example.com", Port: 51820, Protocol: domain.ProtocolWG},
		},
	}

	v, err := service.GenerateLink(profile, "v")
	require.NoError(t, err)
	assert.Contains(t, v, "vless://")
	assert.Contains(t, v, "pbk=pk")
	assert.Contains(t, v, "#Premium+Plan")

	h, err := service.GenerateLink(profile, "h")
	require.NoError(t, err)
	assert.Contains(t, h, "h2://")

	w, err := service.GenerateLink(profile, "w")
	require.NoError(t, err)
	assert.Contains(t, w, "wireguard://")
}

func TestGenerateLinksAndErrors(t *testing.T) {
	service := NewService()
	profile := domain.Profile{
		Name:      "X",
		Endpoints: []domain.Endpoint{{Name: "v", Address: "x", Port: 443, Protocol: domain.ProtocolVLESS, UUID: "u", RealityPublicKey: "pk", RealityShortID: "ab12", ServerName: "sni.example.com"}},
	}
	all, err := service.GenerateLinks(profile)
	require.NoError(t, err)
	assert.Len(t, all, 1)

	_, err = service.GenerateLink(profile, "none")
	require.Error(t, err)
}

func TestGenerateHappImportLinksSkipsUnsupportedProtocols(t *testing.T) {
	service := NewService()
	profile := domain.Profile{
		Name: "X",
		Endpoints: []domain.Endpoint{
			{Name: "wg", Address: "wg.example.com", Port: 51820, Protocol: domain.ProtocolWG},
			{Name: "h", Address: "hy.example.com", Port: 443, Protocol: domain.ProtocolHysteria, HysteriaPassword: "pass"},
		},
	}
	links, err := service.GenerateHappImportLinks(profile)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Contains(t, links[0], "h2://")
}
