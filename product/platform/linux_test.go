//go:build linux

package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinuxControllerSetupTUNRunsCommandsInOrder(t *testing.T) {
	orig := runCommand
	defer func() { runCommand = orig }()

	calls := make([][]string, 0, 2)
	runCommand = func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}

	err := LinuxController{}.SetupTUN(context.Background(), "tun10")
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"ip", "link", "set", "dev", "tun10", "up"},
		{"ip", "route", "replace", "default", "dev", "tun10"},
	}, calls)
}

func TestLinuxControllerSetupTUNReturnsError(t *testing.T) {
	orig := runCommand
	defer func() { runCommand = orig }()

	runCommand = func(_ string, args ...string) error {
		return errors.New("boom")
	}

	err := LinuxController{}.SetupTUN(context.Background(), "tun10")
	require.Error(t, err)
}

func TestLinuxControllerTeardownTUNIgnoresCommandErrors(t *testing.T) {
	orig := runCommand
	defer func() { runCommand = orig }()

	calls := make([][]string, 0, 2)
	runCommand = func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return errors.New("boom")
	}

	err := LinuxController{}.TeardownTUN(context.Background(), "tun10")
	require.NoError(t, err)
	assert.Len(t, calls, 2)
}
