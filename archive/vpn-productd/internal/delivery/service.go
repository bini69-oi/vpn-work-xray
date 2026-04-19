package delivery

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/xtls/xray-core/internal/domain"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) GenerateLinks(profile domain.Profile) (map[string]string, error) {
	links := make(map[string]string, len(profile.Endpoints))
	for _, ep := range profile.Endpoints {
		link, err := s.GenerateLink(profile, ep.Name)
		if err != nil {
			return nil, err
		}
		links[ep.Name] = link
	}
	return links, nil
}

func (s *Service) GenerateHappImportLinks(profile domain.Profile) ([]string, error) {
	links := make([]string, 0, len(profile.Endpoints))
	for _, ep := range profile.Endpoints {
		if ep.Protocol != domain.ProtocolVLESS && ep.Protocol != domain.ProtocolHysteria {
			continue
		}
		link, err := s.GenerateLink(profile, ep.Name)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("no supported happ endpoints")
	}
	return links, nil
}

func (s *Service) GenerateLink(profile domain.Profile, endpointName string) (string, error) {
	ep, ok := selectEndpoint(profile, endpointName)
	if !ok {
		return "", fmt.Errorf("endpoint not found: %s", endpointName)
	}
	switch ep.Protocol {
	case domain.ProtocolVLESS:
		if ep.UUID == "" || ep.RealityPublicKey == "" || firstNonEmpty(ep.RealityShortID, firstFromSlice(ep.RealityShortIDs)) == "" || firstNonEmpty(ep.ServerName, firstFromSlice(ep.RealityServerNames)) == "" {
			return "", fmt.Errorf("incomplete vless reality endpoint: %s", ep.Name)
		}
		return vlessLink(profile, ep), nil
	case domain.ProtocolHysteria:
		if ep.HysteriaPassword == "" {
			return "", fmt.Errorf("incomplete hysteria2 endpoint: %s", ep.Name)
		}
		return h2Link(profile, ep), nil
	case domain.ProtocolWG:
		return wireguardLink(profile, ep), nil
	default:
		return "", fmt.Errorf("unsupported protocol: %s", ep.Protocol)
	}
}

func vlessLink(profile domain.Profile, ep domain.Endpoint) string {
	params := url.Values{}
	params.Set("type", "tcp")
	params.Set("security", "reality")
	params.Set("pbk", ep.RealityPublicKey)
	params.Set("sid", firstNonEmpty(ep.RealityShortID, firstFromSlice(ep.RealityShortIDs)))
	params.Set("sni", firstNonEmpty(ep.ServerName, firstFromSlice(ep.RealityServerNames)))
	params.Set("fp", firstNonEmpty(ep.Fingerprint, "chrome"))
	if ep.RealitySpiderX != "" {
		params.Set("spx", ep.RealitySpiderX)
	}
	if ep.Flow != "" {
		params.Set("flow", ep.Flow)
	}
	if len(ep.ALPN) > 0 {
		params.Set("alpn", strings.Join(ep.ALPN, ","))
	}
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", ep.UUID, ep.Address, ep.Port, params.Encode(), url.QueryEscape(profile.Name))
}

func h2Link(profile domain.Profile, ep domain.Endpoint) string {
	params := url.Values{}
	params.Set("server", ep.Address)
	params.Set("port", strconv.Itoa(ep.Port))
	params.Set("password", ep.HysteriaPassword)
	if ep.HysteriaObfs != "" {
		params.Set("obfs", ep.HysteriaObfs)
	}
	if ep.HysteriaObfsPass != "" {
		params.Set("obfs-password", ep.HysteriaObfsPass)
	}
	if ep.ServerName != "" {
		params.Set("sni", ep.ServerName)
	}
	return "h2://" + base64.RawURLEncoding.EncodeToString([]byte(params.Encode())) + "#" + url.QueryEscape(profile.Name)
}

func wireguardLink(profile domain.Profile, ep domain.Endpoint) string {
	params := url.Values{}
	params.Set("endpoint", fmt.Sprintf("%s:%d", ep.Address, ep.Port))
	params.Set("address", "10.0.0.2/32")
	params.Set("publicKey", "replace-me")
	params.Set("mtu", "1420")
	return "wireguard://" + base64.RawURLEncoding.EncodeToString([]byte(params.Encode())) + "#" + url.QueryEscape(profile.Name)
}

func selectEndpoint(profile domain.Profile, endpointName string) (domain.Endpoint, bool) {
	for _, ep := range profile.Endpoints {
		if ep.Name == endpointName {
			return ep, true
		}
	}
	return domain.Endpoint{}, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstFromSlice(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
