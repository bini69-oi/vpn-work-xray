//go:build windows

package platform

import "context"

type WindowsController struct{}

func New() Controller {
	return WindowsController{}
}

func (WindowsController) EnableSystemProxy(_ context.Context, _ ProxySettings) error { return nil }
func (WindowsController) DisableSystemProxy(_ context.Context) error                 { return nil }
func (WindowsController) SetupTUN(_ context.Context, _ string) error                 { return nil }
func (WindowsController) TeardownTUN(_ context.Context, _ string) error              { return nil }
func (WindowsController) SupportsTUN() bool                                          { return true }
