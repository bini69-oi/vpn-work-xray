//go:build linux

package platform

import (
	"context"
	"os/exec"
)

type LinuxController struct{}
type commandRunner func(name string, args ...string) error

var runCommand commandRunner = func(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func New() Controller {
	return LinuxController{}
}

func (LinuxController) EnableSystemProxy(_ context.Context, _ ProxySettings) error { return nil }
func (LinuxController) DisableSystemProxy(_ context.Context) error                 { return nil }
func (LinuxController) SetupTUN(_ context.Context, iface string) error {
	if iface == "" {
		return nil
	}
	commands := [][]string{
		{"ip", "link", "set", "dev", iface, "up"},
		{"ip", "route", "replace", "default", "dev", iface},
	}
	for _, cmd := range commands {
		if err := runCommand(cmd[0], cmd[1:]...); err != nil {
			return err
		}
	}
	return nil
}
func (LinuxController) TeardownTUN(_ context.Context, iface string) error {
	if iface == "" {
		return nil
	}
	commands := [][]string{
		{"ip", "route", "del", "default", "dev", iface},
		{"ip", "link", "set", "dev", iface, "down"},
	}
	for _, cmd := range commands {
		_ = runCommand(cmd[0], cmd[1:]...)
	}
	return nil
}
func (LinuxController) SupportsTUN() bool { return true }
