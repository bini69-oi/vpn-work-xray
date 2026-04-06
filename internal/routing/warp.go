package routing

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// WarpMode selects how the WARP outbound is built.
type WarpMode string

const (
	WarpModeWireGuard WarpMode = "wireguard"
	WarpModeSocks     WarpMode = "socks"
)

// ParseWarpMode reads WARP_MODE (wireguard|socks). Empty defaults to wireguard.
func ParseWarpMode() WarpMode {
	m := strings.ToLower(strings.TrimSpace(os.Getenv("WARP_MODE")))
	switch m {
	case "", "wireguard":
		return WarpModeWireGuard
	case "socks":
		return WarpModeSocks
	default:
		return WarpModeWireGuard
	}
}

// BuildWarpOutbound returns the Xray outbound map for tag "warp".
func BuildWarpOutbound() (map[string]any, error) {
	switch ParseWarpMode() {
	case WarpModeSocks:
		return buildWarpSocksOutbound()
	default:
		return buildWarpWireGuardOutbound()
	}
}

func buildWarpSocksOutbound() (map[string]any, error) {
	addr := strings.TrimSpace(os.Getenv("WARP_SOCKS_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:40000"
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("parse WARP_SOCKS_ADDR: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return nil, fmt.Errorf("invalid WARP_SOCKS_ADDR port")
	}
	return map[string]any{
		"tag":      "warp",
		"protocol": "socks",
		"settings": map[string]any{
			"servers": []map[string]any{
				{"address": host, "port": port},
			},
		},
	}, nil
}

func buildWarpWireGuardOutbound() (map[string]any, error) {
	sec, peer, err := loadWarpKeys()
	if err != nil {
		return nil, err
	}
	v4 := firstNonEmpty(os.Getenv("VPN_PRODUCT_WARP_LOCAL_V4"), "172.16.0.2/32")
	v6 := firstNonEmpty(os.Getenv("VPN_PRODUCT_WARP_LOCAL_V6"), "fd01:db8:85a3::1/128")
	endpoint := firstNonEmpty(os.Getenv("VPN_PRODUCT_WARP_ENDPOINT"), "engage.cloudflareclient.com:2408")
	mtu := 1280
	if raw := strings.TrimSpace(os.Getenv("VPN_PRODUCT_WARP_MTU")); raw != "" {
		if n, convErr := strconv.Atoi(raw); convErr == nil && n > 0 {
			mtu = n
		}
	}
	return map[string]any{
		"tag":      "warp",
		"protocol": "wireguard",
		"settings": map[string]any{
			"secretKey": sec,
			"address":   []string{v4, v6},
			"peers": []map[string]any{
				{
					"publicKey":  peer,
					"allowedIPs": []string{"0.0.0.0/0", "::/0"},
					"endpoint":   endpoint,
				},
			},
			"reserved": []int{0, 0, 0},
			"mtu":      mtu,
		},
	}, nil
}

func loadWarpKeys() (secretKey string, peerPub string, err error) {
	// Inline env (for tests / containers).
	if s := strings.TrimSpace(os.Getenv("WARP_PRIVATE_KEY")); s != "" {
		if p := strings.TrimSpace(os.Getenv("WARP_PEER_PUBLIC_KEY")); p != "" {
			return s, p, nil
		}
		return "", "", fmt.Errorf("WARP_PEER_PUBLIC_KEY is required with WARP_PRIVATE_KEY")
	}
	path := WarpEnvPath()
	// #nosec G304 -- path is operator-controlled WARP secrets file.
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read warp env %s: %w", path, err)
	}
	kv := parseDotEnv(string(data))
	sec := strings.TrimSpace(kv["WARP_PRIVATE_KEY"])
	peer := strings.TrimSpace(kv["WARP_PEER_PUBLIC_KEY"])
	if sec == "" || peer == "" {
		return "", "", fmt.Errorf("WARP_PRIVATE_KEY and WARP_PEER_PUBLIC_KEY must be set in %s or environment", path)
	}
	return sec, peer, nil
}

func parseDotEnv(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		v = strings.Trim(v, `"'`)
		out[k] = v
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return b
}

// WarpStatus describes whether WARP outbound is likely usable (best-effort).
type WarpStatus struct {
	Mode       string `json:"mode"`
	EnvFile    string `json:"envFile"`
	KeysLoaded bool   `json:"keysLoaded"`
	SocksAddr  string `json:"socksAddr,omitempty"`
	Message    string `json:"message,omitempty"`
}

// DescribeWarpStatus is used by GET /api/v1/routing/warp/status.
func DescribeWarpStatus() WarpStatus {
	mode := string(ParseWarpMode())
	st := WarpStatus{Mode: mode, EnvFile: WarpEnvPath()}
	if ParseWarpMode() == WarpModeSocks {
		addr := strings.TrimSpace(os.Getenv("WARP_SOCKS_ADDR"))
		if addr == "" {
			addr = "127.0.0.1:40000"
		}
		st.SocksAddr = addr
		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			st.Message = "invalid WARP_SOCKS_ADDR"
			return st
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			st.Message = "invalid WARP_SOCKS_ADDR port"
			return st
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), dialTimeoutDur())
		if err != nil {
			st.Message = fmt.Sprintf("socks not reachable: %v", err)
			return st
		}
		_ = conn.Close()
		st.KeysLoaded = true
		st.Message = "socks reachable"
		return st
	}
	if strings.TrimSpace(os.Getenv("WARP_PRIVATE_KEY")) != "" && strings.TrimSpace(os.Getenv("WARP_PEER_PUBLIC_KEY")) != "" {
		st.KeysLoaded = true
		st.Message = "keys from environment"
		return st
	}
	if _, err := os.Stat(WarpEnvPath()); err == nil {
		if sec, peer, err := loadWarpKeys(); err == nil && sec != "" && peer != "" {
			st.KeysLoaded = true
			st.Message = "keys from warp.env"
			return st
		}
	}
	st.Message = "wireguard keys not configured"
	return st
}

func dialTimeoutDur() time.Duration {
	t := strings.TrimSpace(os.Getenv("VPN_PRODUCT_WARP_DIAL_TIMEOUT"))
	if t == "" {
		return 2 * time.Second
	}
	d, err := time.ParseDuration(t)
	if err != nil {
		return 2 * time.Second
	}
	return d
}
