package dns_test

import (
	"net/url"
	"testing"

	. "github.com/xtls/xray-core/app/dns"
	"github.com/xtls/xray-core/common/net"
)

func TestNewQUICNameServerConstruction(t *testing.T) {
	t.Parallel()
	u, err := url.Parse("quic://dns.adguard-dns.com")
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewQUICNameServer(u, false, false, 0, net.IP(nil))
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("server is nil")
	}
	if got := s.Name(); got != "quic://dns.adguard-dns.com" {
		t.Errorf("Name: got %q", got)
	}
	if s.IsDisableCache() {
		t.Error("expected cache enabled")
	}

	s2, err := NewQUICNameServer(u, true, false, 0, net.IP(nil))
	if err != nil {
		t.Fatal(err)
	}
	if !s2.IsDisableCache() {
		t.Error("expected cache disabled when disableCache=true")
	}
}

func TestNewQUICNameServerExplicitPort(t *testing.T) {
	t.Parallel()
	u, err := url.Parse("quic://dns.adguard-dns.com:853")
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewQUICNameServer(u, false, false, 0, net.IP(nil))
	if err != nil {
		t.Fatal(err)
	}
	if s.Name() != "quic://dns.adguard-dns.com:853" {
		t.Errorf("unexpected name %q", s.Name())
	}
}

func TestNewQUICNameServerInvalidPort(t *testing.T) {
	t.Parallel()
	// Port > 65535 is rejected by PortFromString / PortFromInt.
	u, err := url.Parse("quic://dns.adguard-dns.com:70000")
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewQUICNameServer(u, false, false, 0, net.IP(nil))
	if err == nil {
		t.Fatal("expected error for out-of-range port")
	}
}
