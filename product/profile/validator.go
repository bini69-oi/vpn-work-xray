package profile

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/xtls/xray-core/product/domain"
)

func ValidateProfile(p domain.Profile) error {
	if p.ID == "" {
		return errors.New("profile id is required")
	}
	if p.Name == "" {
		return errors.New("profile name is required")
	}
	if p.RouteMode != domain.RouteModeSplit && p.RouteMode != domain.RouteModeFull {
		return fmt.Errorf("unsupported route mode: %s", p.RouteMode)
	}
	if len(p.Endpoints) == 0 {
		return errors.New("at least one endpoint is required")
	}

	if p.Security.Level == "" {
		p.Security.Level = domain.SecurityLevelStandard
	}
	if p.Security.Level != domain.SecurityLevelStandard && p.Security.Level != domain.SecurityLevelStrict {
		return fmt.Errorf("unsupported security level: %s", p.Security.Level)
	}
	if p.Security.Level == domain.SecurityLevelStrict {
		p.Security.DisableProtocolDowngrade = true
		p.Security.RestrictDiagnostics = true
	}

	seen := make(map[string]struct{}, len(p.Endpoints))
	for _, ep := range p.Endpoints {
		if ep.Name == "" || ep.Address == "" || ep.Port <= 0 {
			return fmt.Errorf("invalid endpoint: %+v", ep)
		}
		if ep.Protocol != domain.ProtocolVLESS && ep.Protocol != domain.ProtocolHysteria && ep.Protocol != domain.ProtocolWG {
			return fmt.Errorf("unsupported protocol: %s", ep.Protocol)
		}
		if ep.ServerTag == "" {
			return fmt.Errorf("serverTag is required for endpoint: %s", ep.Name)
		}
		if ep.Protocol == domain.ProtocolVLESS && ep.UUID == "" {
			return fmt.Errorf("uuid is required for vless endpoint: %s", ep.Name)
		}
		if strings.Contains(strings.ToLower(ep.Address), "replace-me") {
			return fmt.Errorf("placeholder address is not allowed for endpoint: %s", ep.Name)
		}
		switch ep.Protocol {
		case domain.ProtocolVLESS:
			if isPlaceholder(ep.UUID) {
				return fmt.Errorf("placeholder uuid is not allowed for endpoint: %s", ep.Name)
			}
			if isPlaceholder(ep.RealityPublicKey) || ep.RealityPublicKey == "" {
				return fmt.Errorf("reality public key is required for endpoint: %s", ep.Name)
			}
			if ep.ServerName == "" {
				return fmt.Errorf("serverName is required for vless reality endpoint: %s", ep.Name)
			}
			if ep.RealityShortID == "" && len(ep.RealityShortIDs) == 0 {
				return fmt.Errorf("reality shortId is required for endpoint: %s", ep.Name)
			}
		case domain.ProtocolHysteria:
			if ep.HysteriaPassword == "" || isPlaceholder(ep.HysteriaPassword) {
				return fmt.Errorf("hysteria password is required for endpoint: %s", ep.Name)
			}
		case domain.ProtocolWG:
			if ep.WireGuardSecretKey == "" || isPlaceholder(ep.WireGuardSecretKey) {
				return fmt.Errorf("wireguard secret key is required for endpoint: %s", ep.Name)
			}
			if ep.WireGuardPublicKey == "" || isPlaceholder(ep.WireGuardPublicKey) {
				return fmt.Errorf("wireguard peer public key is required for endpoint: %s", ep.Name)
			}
		}
		seen[ep.Name] = struct{}{}
	}
	if p.PreferredID == "" {
		return errors.New("preferred endpoint is required")
	}
	if _, ok := seen[p.PreferredID]; !ok {
		return fmt.Errorf("preferred endpoint not found: %s", p.PreferredID)
	}
	for _, endpointID := range p.Fallback.EndpointIDs {
		if _, ok := seen[endpointID]; !ok {
			return fmt.Errorf("fallback endpoint not found: %s", endpointID)
		}
	}
	if p.ReconnectPolicy.MaxRetries <= 0 {
		return errors.New("reconnect maxRetries must be > 0")
	}
	if p.ReconnectPolicy.BaseBackoff <= 0 || p.ReconnectPolicy.MaxBackoff <= 0 {
		return errors.New("reconnect backoff must be > 0")
	}
	if p.ReconnectPolicy.BaseBackoff > p.ReconnectPolicy.MaxBackoff {
		return errors.New("baseBackoff cannot exceed maxBackoff")
	}
	for _, cidr := range append(append([]string{}, p.DirectCIDRs...), p.ProxyCIDRs...) {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			return fmt.Errorf("invalid CIDR in routing lists: %s", cidr)
		}
	}
	if p.TrafficLimitMB < 0 {
		return errors.New("trafficLimitMb cannot be negative")
	}
	if p.TrafficLimitGB < 0 {
		return errors.New("trafficLimitGb cannot be negative")
	}
	if p.TrafficUsedBytes < 0 {
		return errors.New("trafficUsedBytes cannot be negative")
	}
	if p.TUN.Enabled {
		if p.TUN.Interface == "" {
			return errors.New("tun.interface is required when TUN is enabled")
		}
		if p.TUN.MTU <= 0 {
			return errors.New("tun.mtu must be > 0 when TUN is enabled")
		}
	}
	return nil
}

func isPlaceholder(v string) bool {
	normalized := strings.ToLower(strings.TrimSpace(v))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "replace-me") || strings.Contains(normalized, "example.com")
}
