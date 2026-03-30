package connection

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/product/configgen"
	"github.com/xtls/xray-core/product/domain"
	"github.com/xtls/xray-core/product/logging"
	"github.com/xtls/xray-core/product/reconnect"
)

type fakeProfiles struct {
	p         domain.Profile
	accessErr error
}

func (f fakeProfiles) Get(_ context.Context, id string) (domain.Profile, error) {
	if id != f.p.ID {
		return domain.Profile{}, errors.New("not found")
	}
	return f.p, nil
}

func (f fakeProfiles) CheckAccess(_ context.Context, _ string) error {
	return f.accessErr
}

type fakeRuntime struct {
	applyErrs []error
	stopErr   error
	calls     int
}

func (r *fakeRuntime) ApplyConfig(_ context.Context, _ string) error {
	r.calls++
	if len(r.applyErrs) == 0 {
		return nil
	}
	err := r.applyErrs[0]
	r.applyErrs = r.applyErrs[1:]
	return err
}

func (r *fakeRuntime) Stop(_ context.Context) error { return r.stopErr }

func TestManagerConnectSuccess(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	profile := domain.Profile{
		ID:        "p1",
		Name:      "demo",
		Enabled:   true,
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"},
		},
		PreferredID: "main",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries:  2,
			BaseBackoff: 10 * time.Millisecond,
			MaxBackoff:  20 * time.Millisecond,
		},
	}
	m := NewManager(
		fakeProfiles{p: profile},
		configgen.NewGenerator(t.TempDir()),
		&fakeRuntime{},
		reconnect.NewEngine(1),
		logger,
		logger,
	)

	require.NoError(t, m.Connect(context.Background(), profile.ID))
	status := m.Status(context.Background())
	assert.Equal(t, domain.StateConnected, status.State)
}

func TestManagerConnectFallbackOnFailure(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	profile := domain.Profile{
		ID:        "p-fallback",
		Name:      "demo",
		Enabled:   true,
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"},
			{Name: "backup", Address: "2.2.2.2", Port: 8443, Protocol: domain.ProtocolHysteria, ServerTag: "proxy", HysteriaPassword: "pass"},
		},
		PreferredID: "main",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 2, BaseBackoff: 1 * time.Millisecond, MaxBackoff: 2 * time.Millisecond,
		},
	}
	runtime := &fakeRuntime{applyErrs: []error{errors.New("main failed"), nil}}
	m := NewManager(
		fakeProfiles{p: profile},
		configgen.NewGenerator(t.TempDir()),
		runtime,
		reconnect.NewEngine(2),
		logger,
		logger,
	)
	require.NoError(t, m.Connect(context.Background(), profile.ID))
	assert.Equal(t, 2, runtime.calls)
	assert.Equal(t, "backup", m.Status(context.Background()).ActiveEndpoint)
}

func TestManagerDisconnectReturnsRuntimeError(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	profile := domain.Profile{
		ID: "p1", Name: "demo", Enabled: true, RouteMode: domain.RouteModeSplit,
		Endpoints:   []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"}},
		PreferredID: "main",
	}
	runtime := &fakeRuntime{stopErr: errors.New("stop failed")}
	m := NewManager(fakeProfiles{p: profile}, configgen.NewGenerator(t.TempDir()), runtime, reconnect.NewEngine(1), logger, logger)
	err = m.Disconnect(context.Background())
	require.Error(t, err)
}

func TestManagerConnectDeniedByAccessCheck(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	profile := domain.Profile{
		ID: "p1", Name: "demo", Enabled: true, RouteMode: domain.RouteModeSplit,
		Endpoints:   []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"}},
		PreferredID: "main",
	}
	m := NewManager(fakeProfiles{p: profile, accessErr: errors.New("subscription expired")}, configgen.NewGenerator(t.TempDir()), &fakeRuntime{}, reconnect.NewEngine(1), logger, logger)
	err = m.Connect(context.Background(), profile.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscription expired")
}

func TestManagerConnectBusyPortFailure(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	profile := domain.Profile{
		ID: "p1", Name: "demo", Enabled: true, RouteMode: domain.RouteModeSplit,
		Endpoints:   []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"}},
		PreferredID: "main",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond,
		},
	}
	runtime := &fakeRuntime{applyErrs: []error{errors.New("listen tcp 127.0.0.1:10808: bind: address already in use")}}
	m := NewManager(fakeProfiles{p: profile}, configgen.NewGenerator(t.TempDir()), runtime, reconnect.NewEngine(1), logger, logger)
	err = m.Connect(context.Background(), profile.ID)
	require.Error(t, err)
	assert.Equal(t, domain.StateFailed, m.Status(context.Background()).State)
}

func TestManagerConcurrentConnect100Users(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	profile := domain.Profile{
		ID: "p1", Name: "demo", Enabled: true, RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"},
			{Name: "backup", Address: "2.2.2.2", Port: 8443, Protocol: domain.ProtocolHysteria, ServerTag: "proxy", HysteriaPassword: "x"},
		},
		PreferredID: "main",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond,
		},
	}
	m := NewManager(fakeProfiles{p: profile}, configgen.NewGenerator(t.TempDir()), &fakeRuntime{}, reconnect.NewEngine(1), logger, logger)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Connect(context.Background(), profile.ID)
		}()
	}
	wg.Wait()
	state := m.Status(context.Background()).State
	assert.True(t, state == domain.StateConnected || state == domain.StateFailed || state == domain.StateReconnecting || state == domain.StateConnecting)
}

func TestManagerConnectValidationBranches(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)

	t.Run("profile not found", func(t *testing.T) {
		m := NewManager(fakeProfiles{p: domain.Profile{ID: "real"}}, configgen.NewGenerator(t.TempDir()), &fakeRuntime{}, reconnect.NewEngine(1), logger, logger)
		err := m.Connect(context.Background(), "missing")
		require.Error(t, err)
	})

	t.Run("blocked profile", func(t *testing.T) {
		profile := domain.Profile{ID: "p1", Name: "demo", Blocked: true}
		m := NewManager(fakeProfiles{p: profile}, configgen.NewGenerator(t.TempDir()), &fakeRuntime{}, reconnect.NewEngine(1), logger, logger)
		err := m.Connect(context.Background(), "p1")
		require.Error(t, err)
	})

	t.Run("quota exceeded", func(t *testing.T) {
		profile := domain.Profile{
			ID: "p1", Name: "demo", TrafficLimitMB: 1, TrafficUsedUp: 800000, TrafficUsedDown: 500000,
			Endpoints:   []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"}},
			PreferredID: "main",
		}
		m := NewManager(fakeProfiles{p: profile}, configgen.NewGenerator(t.TempDir()), &fakeRuntime{}, reconnect.NewEngine(1), logger, logger)
		err := m.Connect(context.Background(), "p1")
		require.Error(t, err)
	})

	t.Run("no endpoints", func(t *testing.T) {
		profile := domain.Profile{ID: "p1", Name: "demo"}
		m := NewManager(fakeProfiles{p: profile}, configgen.NewGenerator(t.TempDir()), &fakeRuntime{}, reconnect.NewEngine(1), logger, logger)
		err := m.Connect(context.Background(), "p1")
		require.Error(t, err)
	})
}

func TestManagerReconnectContextCancel(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	profile := domain.Profile{
		ID: "p1", Name: "demo", Enabled: true, RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"},
			{Name: "backup", Address: "2.2.2.2", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000002", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "b1c2d3e4"},
		},
		PreferredID: "main",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 2, BaseBackoff: 20 * time.Millisecond, MaxBackoff: 40 * time.Millisecond,
		},
	}
	runtime := &fakeRuntime{applyErrs: []error{errors.New("boom"), nil}}
	m := NewManager(fakeProfiles{p: profile}, configgen.NewGenerator(t.TempDir()), runtime, reconnect.NewEngine(1), logger, logger)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = m.Connect(ctx, profile.ID)
	require.Error(t, err)
}

func TestEndpointOrderAndErrToString(t *testing.T) {
	ordered := endpointOrder(domain.Profile{
		PreferredID: "b",
		Endpoints: []domain.Endpoint{
			{Name: "a"}, {Name: "b"}, {Name: "c"},
		},
	})
	require.Len(t, ordered, 3)
	assert.Equal(t, "b", ordered[0].Name)
	assert.Equal(t, "", errToString(nil))
	assert.Equal(t, "x", errToString(errors.New("x")))
}

func TestManagerDisconnectSuccess(t *testing.T) {
	logger, err := logging.New("")
	require.NoError(t, err)
	profile := domain.Profile{
		ID: "p1", Name: "demo", Enabled: true, RouteMode: domain.RouteModeSplit,
		Endpoints:   []domain.Endpoint{{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001", ServerName: "sni.example.com", RealityPublicKey: "pub", RealityShortID: "a1b2c3d4"}},
		PreferredID: "main",
	}
	m := NewManager(fakeProfiles{p: profile}, configgen.NewGenerator(t.TempDir()), &fakeRuntime{}, reconnect.NewEngine(1), logger, logger)
	require.NoError(t, m.Disconnect(context.Background()))
	assert.Equal(t, domain.StateIdle, m.Status(context.Background()).State)
}
