package domain

import "time"

type Protocol string

const (
	ProtocolVLESS    Protocol = "vless"
	ProtocolHysteria Protocol = "hysteria2"
	ProtocolWG       Protocol = "wireguard"
)

type RouteMode string

const (
	RouteModeSplit RouteMode = "split"
	RouteModeFull  RouteMode = "full"
)

type ConnectionState string

const (
	StateIdle         ConnectionState = "idle"
	StateConnecting   ConnectionState = "connecting"
	StateConnected    ConnectionState = "connected"
	StateDegraded     ConnectionState = "degraded"
	StateReconnecting ConnectionState = "reconnecting"
	StateFailed       ConnectionState = "failed"
)

type SecurityLevel string

const (
	SecurityLevelStandard SecurityLevel = "standard_secure"
	SecurityLevelStrict   SecurityLevel = "strict_secure"
)

type SecurityPolicy struct {
	Level                     SecurityLevel `json:"level"`
	EnableKillSwitch          bool          `json:"enableKillSwitch"`
	DisableProtocolDowngrade  bool          `json:"disableProtocolDowngrade"`
	RestrictDiagnostics       bool          `json:"restrictDiagnostics"`
}

type Endpoint struct {
	Name               string   `json:"name"`
	Address            string   `json:"address"`
	Port               int      `json:"port"`
	Protocol           Protocol `json:"protocol"`
	ServerTag          string   `json:"serverTag"`
	UUID               string   `json:"uuid,omitempty"`
	Flow               string   `json:"flow,omitempty"`
	ServerName         string   `json:"serverName,omitempty"`
	Fingerprint        string   `json:"fingerprint,omitempty"`
	RealityPublicKey   string   `json:"realityPublicKey,omitempty"`
	RealityShortID     string   `json:"realityShortId,omitempty"`
	RealityShortIDs    []string `json:"realityShortIds,omitempty"`
	RealitySpiderX     string   `json:"realitySpiderX,omitempty"`
	RealityDest        string   `json:"realityDest,omitempty"`
	RealityServerNames []string `json:"realityServerNames,omitempty"`
	ALPN               []string `json:"alpn,omitempty"`
	HysteriaPassword   string   `json:"hysteriaPassword,omitempty"`
	HysteriaUpMbps     int      `json:"hysteriaUpMbps,omitempty"`
	HysteriaDownMbps   int      `json:"hysteriaDownMbps,omitempty"`
	HysteriaObfs       string   `json:"hysteriaObfs,omitempty"`
	HysteriaObfsPass   string   `json:"hysteriaObfsPass,omitempty"`
	WireGuardSecretKey string   `json:"wireGuardSecretKey,omitempty"`
	WireGuardPublicKey string   `json:"wireGuardPublicKey,omitempty"`
	WireGuardLocalIP   string   `json:"wireGuardLocalIp,omitempty"`
}

type FallbackChain struct {
	EndpointIDs []string `json:"endpointIds"`
}

type DNSOptions struct {
	Primary   []string `json:"primary"`
	Fallback  []string `json:"fallback"`
	UseDoH    bool     `json:"useDoH"`
	UseDoQ    bool     `json:"useDoQ"`
	QueryIPv6 bool     `json:"queryIPv6"`
}

type ReconnectPolicy struct {
	MaxRetries       int           `json:"maxRetries"`
	BaseBackoff      time.Duration `json:"baseBackoff"`
	MaxBackoff       time.Duration `json:"maxBackoff"`
	FailureWindow    time.Duration `json:"failureWindow"`
	DegradedFailures int           `json:"degradedFailures"`
}

type TUNOptions struct {
	Enabled     bool   `json:"enabled"`
	Interface   string `json:"interface"`
	MTU         int    `json:"mtu"`
	IPv4Address string `json:"ipv4Address"`
}

type Profile struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	Description           string          `json:"description"`
	Enabled               bool            `json:"enabled"`
	RouteMode             RouteMode       `json:"routeMode"`
	Endpoints             []Endpoint      `json:"endpoints"`
	PreferredID           string          `json:"preferredId"`
	Fallback              FallbackChain   `json:"fallback"`
	DNS                   DNSOptions      `json:"dns"`
	DirectDomains         []string        `json:"directDomains,omitempty"`
	ProxyDomains          []string        `json:"proxyDomains,omitempty"`
	DirectCIDRs           []string        `json:"directCidrs,omitempty"`
	ProxyCIDRs            []string        `json:"proxyCidrs,omitempty"`
	DirectCountries       []string        `json:"directCountries,omitempty"`
	ProxyCountries        []string        `json:"proxyCountries,omitempty"`
	InvertRouting         bool            `json:"invertRouting,omitempty"`
	TrafficLimitMB        int64           `json:"trafficLimitMb,omitempty"`
	TrafficUsedUp         int64           `json:"trafficUsedUp,omitempty"`
	TrafficUsedDown       int64           `json:"trafficUsedDown,omitempty"`
	SubscriptionExpiresAt *time.Time      `json:"subscriptionExpiresAt,omitempty"`
	TrafficLimitGB        int64           `json:"trafficLimitGb,omitempty"`
	TrafficUsedBytes      int64           `json:"trafficUsedBytes,omitempty"`
	Blocked               bool            `json:"blocked,omitempty"`
	TUN                   TUNOptions      `json:"tun,omitempty"`
	Security              SecurityPolicy   `json:"security,omitempty"`
	ReconnectPolicy       ReconnectPolicy `json:"reconnectPolicy"`
	CreatedAt             time.Time       `json:"createdAt"`
	UpdatedAt             time.Time       `json:"updatedAt"`
}

type RuntimeStatus struct {
	State          ConnectionState `json:"state"`
	ActiveProfile  string          `json:"activeProfile"`
	ActiveEndpoint string          `json:"activeEndpoint"`
	LastError      string          `json:"lastError,omitempty"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type PanelUser struct {
	ID         string    `json:"id"`
	Panel      string    `json:"panel"`
	ExternalID string    `json:"externalId"`
	Username   string    `json:"username"`
	ProfileID  string    `json:"profileId"`
	Status     string    `json:"status"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
