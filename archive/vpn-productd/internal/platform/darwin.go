//go:build darwin

package platform

import "context"

type DarwinController struct{}

func New() Controller {
	return DarwinController{}
}

func (DarwinController) EnableSystemProxy(_ context.Context, _ ProxySettings) error { return nil }
func (DarwinController) DisableSystemProxy(_ context.Context) error                 { return nil }
func (DarwinController) SetupTUN(_ context.Context, _ string) error                 { return nil }
func (DarwinController) TeardownTUN(_ context.Context, _ string) error              { return nil }
func (DarwinController) SupportsTUN() bool                                          { return true }
