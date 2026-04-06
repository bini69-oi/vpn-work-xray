package main

import (
	"testing"
)

func TestResolveHealthTargetsDefault(t *testing.T) {
	t.Setenv("VPN_PRODUCT_HEALTH_TARGETS", "")
	out := resolveHealthTargets("127.0.0.1:8080")
	if len(out) != 1 || out[0] != "127.0.0.1:8080" {
		t.Fatalf("unexpected targets: %#v", out)
	}
}

func TestResolveHealthTargetsOverride(t *testing.T) {
	t.Setenv("VPN_PRODUCT_HEALTH_TARGETS", "127.0.0.1:8080, 127.0.0.1:10808,")
	out := resolveHealthTargets("127.0.0.1:8080")
	if len(out) != 2 {
		t.Fatalf("unexpected count: %#v", out)
	}
	if out[0] != "127.0.0.1:8080" || out[1] != "127.0.0.1:10808" {
		t.Fatalf("unexpected targets: %#v", out)
	}
}

func TestResolveHealthTargetsEmptyListen(t *testing.T) {
	t.Setenv("VPN_PRODUCT_HEALTH_TARGETS", "")
	out := resolveHealthTargets("")
	if len(out) != 0 {
		t.Fatalf("unexpected targets: %#v", out)
	}
}

func TestValidatePublicBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "empty", in: "", wantErr: true},
		{name: "missing host", in: "https://", wantErr: true},
		{name: "unsupported scheme", in: "ftp://example.com", wantErr: true},
		{name: "http ok", in: "http://example.com", wantErr: false},
		{name: "https ok", in: "https://vpn.example.com", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePublicBaseURL(tt.in)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveIssueStrict(t *testing.T) {
	t.Setenv("VPN_PRODUCT_ISSUE_STRICT", "")
	if !resolveIssueStrict() {
		t.Fatalf("expected strict=true by default")
	}
	t.Setenv("VPN_PRODUCT_ISSUE_STRICT", "false")
	if resolveIssueStrict() {
		t.Fatalf("expected strict=false for false")
	}
	t.Setenv("VPN_PRODUCT_ISSUE_STRICT", "0")
	if resolveIssueStrict() {
		t.Fatalf("expected strict=false for 0")
	}
	t.Setenv("VPN_PRODUCT_ISSUE_STRICT", "1")
	if !resolveIssueStrict() {
		t.Fatalf("expected strict=true for 1")
	}
}
