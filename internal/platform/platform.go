package platform

import "context"

type ProxySettings struct {
	Host string
	Port int
}

type Controller interface {
	EnableSystemProxy(ctx context.Context, cfg ProxySettings) error
	DisableSystemProxy(ctx context.Context) error
	SetupTUN(ctx context.Context, iface string) error
	TeardownTUN(ctx context.Context, iface string) error
	SupportsTUN() bool
}
