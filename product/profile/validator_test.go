package profile

import (
	"testing"
	"time"

	"github.com/xtls/xray-core/product/domain"
)

func TestValidateProfile(t *testing.T) {
	ok := domain.Profile{
		ID:        "p1",
		Name:      "default",
		Enabled:   true,
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{
				Name: "primary", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy",
				UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub-key", RealityShortID: "a1b2c3d4",
			},
		},
		PreferredID: "primary",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries:       3,
			BaseBackoff:      time.Second,
			MaxBackoff:       5 * time.Second,
			DegradedFailures: 2,
		},
	}
	if err := ValidateProfile(ok); err != nil {
		t.Fatalf("expected profile to be valid: %v", err)
	}

	bad := ok
	bad.PreferredID = "missing"
	if err := ValidateProfile(bad); err == nil {
		t.Fatal("expected validation error for missing preferred endpoint")
	}
}

func TestValidateProfile_HysteriaAndPlaceholders(t *testing.T) {
	ok := domain.Profile{
		ID:        "hy1",
		Name:      "hy",
		Enabled:   true,
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{
				Name: "primary", Address: "hy.example.com", Port: 443, Protocol: domain.ProtocolHysteria, ServerTag: "proxy",
				HysteriaPassword: "secret-pass",
			},
		},
		PreferredID: "primary",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second,
		},
	}
	if err := ValidateProfile(ok); err != nil {
		t.Fatalf("expected hysteria profile to be valid: %v", err)
	}

	bad := ok
	bad.Endpoints[0].HysteriaPassword = "replace-me"
	if err := ValidateProfile(bad); err == nil {
		t.Fatal("expected placeholder password validation error")
	}
}

func TestValidateProfile_VLESSRequiredCrypto(t *testing.T) {
	base := domain.Profile{
		ID:        "v1",
		Name:      "v",
		Enabled:   true,
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{
				Name: "primary", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy",
				UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub-key", RealityShortID: "abcd1234",
			},
		},
		PreferredID: "primary",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second,
		},
	}
	if err := ValidateProfile(base); err != nil {
		t.Fatalf("expected valid vless profile: %v", err)
	}

	missingPK := base
	missingPK.Endpoints[0].RealityPublicKey = ""
	if err := ValidateProfile(missingPK); err == nil {
		t.Fatal("expected missing reality public key error")
	}
}
