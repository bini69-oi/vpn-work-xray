package connection

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/xtls/xray-core/core"
	// Register all Xray components for embedded runtime startup.
	_ "github.com/xtls/xray-core/main/distro/all"
	_ "github.com/xtls/xray-core/main/json"
	"github.com/xtls/xray-core/internal/platform"
)

type RuntimeController interface {
	ApplyConfig(ctx context.Context, configPath string) error
	Stop(ctx context.Context) error
}

type XrayRuntime struct {
	instance       runtimeInstance
	platform       platform.Controller
	activeTUNIface string
	startInstance  func(format string, config []byte) (runtimeInstance, error)
}

func NewXrayRuntime() *XrayRuntime {
	return NewXrayRuntimeWith(platform.New())
}

func NewXrayRuntimeWith(controller platform.Controller) *XrayRuntime {
	if controller == nil {
		controller = platform.New()
	}
	return &XrayRuntime{
		platform:      controller,
		startInstance: defaultStartInstance,
	}
}

func (r *XrayRuntime) ApplyConfig(ctx context.Context, configPath string) error {
	// #nosec G304 -- configPath points to internally generated runtime config.
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	newTUNIface := detectTUNInterface(raw)
	next, err := r.startInstance("json", raw)
	if err != nil {
		return err
	}

	prevInstance := r.instance
	prevTUNIface := r.activeTUNIface

	if newTUNIface != "" && newTUNIface != prevTUNIface {
		if err := r.platform.SetupTUN(ctx, newTUNIface); err != nil {
			_ = r.platform.TeardownTUN(ctx, newTUNIface)
			_ = next.Close()
			return err
		}
	}

	if prevInstance != nil {
		_ = prevInstance.Close()
	}
	if prevTUNIface != "" && prevTUNIface != newTUNIface {
		_ = r.platform.TeardownTUN(ctx, prevTUNIface)
	}

	r.activeTUNIface = newTUNIface
	r.instance = next
	return nil
}

type runtimeInstance interface {
	Close() error
}

func defaultStartInstance(format string, config []byte) (runtimeInstance, error) {
	return core.StartInstance(format, config)
}

func (r *XrayRuntime) Stop(ctx context.Context) error {
	var errs []error
	if r.instance != nil {
		if err := r.instance.Close(); err != nil {
			errs = append(errs, err)
		}
		r.instance = nil
	}
	if r.activeTUNIface != "" {
		if err := r.platform.TeardownTUN(ctx, r.activeTUNIface); err != nil {
			errs = append(errs, err)
		}
		r.activeTUNIface = ""
	}
	return errors.Join(errs...)
}

func detectTUNInterface(raw []byte) string {
	var cfg struct {
		Inbounds []struct {
			Protocol string `json:"protocol"`
			Settings struct {
				Name string `json:"name"`
			} `json:"settings"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ""
	}
	for _, inbound := range cfg.Inbounds {
		if inbound.Protocol != "tun" {
			continue
		}
		if inbound.Settings.Name != "" {
			return inbound.Settings.Name
		}
		return "utun0"
	}
	return ""
}
