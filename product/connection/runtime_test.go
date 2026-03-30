package connection

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/product/platform"
)

func TestXrayRuntimeApplyConfig_MinimalConfig(t *testing.T) {
	t.Parallel()

	cfg := `{
  "log": {"loglevel": "warning"},
  "inbounds": [
    {
      "tag": "socks-in",
      "listen": "127.0.0.1",
      "port": 19080,
      "protocol": "socks",
      "settings": {"udp": true}
    }
  ],
  "outbounds": [
    {"tag": "proxy", "protocol": "freedom"}
  ]
}`
	path := filepath.Join(t.TempDir(), "runtime.json")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtime := NewXrayRuntime()
	require.NoError(t, runtime.ApplyConfig(context.Background(), path))
	require.NoError(t, runtime.Stop(context.Background()))
}

func TestDetectTUNInterface(t *testing.T) {
	raw := []byte(`{"inbounds":[{"protocol":"socks","settings":{"udp":true}},{"protocol":"tun","settings":{"name":"tun9"}}]}`)
	iface := detectTUNInterface(raw)
	assert.Equal(t, "tun9", iface)
}

func TestDetectTUNInterfaceDefaultsToUTUN0(t *testing.T) {
	raw := []byte(`{"inbounds":[{"protocol":"tun","settings":{}}]}`)
	assert.Equal(t, "utun0", detectTUNInterface(raw))
}

func TestXrayRuntimeApplyConfigSetupTUNFailureRollsBack(t *testing.T) {
	cfg := `{"inbounds":[{"protocol":"tun","settings":{"name":"tun42"}}],"outbounds":[{"protocol":"freedom","tag":"proxy"}]}`
	path := filepath.Join(t.TempDir(), "runtime.json")
	require.NoError(t, os.WriteFile(path, []byte(cfg), 0o600))

	fakeController := &fakePlatformController{setupErr: errors.New("setup failed")}
	runtime := NewXrayRuntimeWith(fakeController)
	runtime.startInstance = func(_ string, _ []byte) (runtimeInstance, error) {
		return &fakeRuntimeInstance{}, nil
	}

	err := runtime.ApplyConfig(context.Background(), path)
	require.Error(t, err)
	assert.Equal(t, []string{"tun42"}, fakeController.setupCalls)
	assert.Equal(t, []string{"tun42"}, fakeController.teardownCalls)
	assert.Empty(t, runtime.activeTUNIface)
}

func TestXrayRuntimeStopAlwaysTeardown(t *testing.T) {
	fakeController := &fakePlatformController{teardownErr: errors.New("teardown failed")}
	runtime := NewXrayRuntimeWith(fakeController)
	runtime.instance = &fakeRuntimeInstance{closeErr: errors.New("close failed")}
	runtime.activeTUNIface = "tun77"

	err := runtime.Stop(context.Background())
	require.Error(t, err)
	assert.Equal(t, []string{"tun77"}, fakeController.teardownCalls)
	assert.Nil(t, runtime.instance)
	assert.Empty(t, runtime.activeTUNIface)
}

type fakeRuntimeInstance struct {
	closeErr error
}

func (f *fakeRuntimeInstance) Close() error { return f.closeErr }

type fakePlatformController struct {
	setupCalls    []string
	teardownCalls []string
	setupErr      error
	teardownErr   error
}

var _ platform.Controller = (*fakePlatformController)(nil)

func (f *fakePlatformController) EnableSystemProxy(context.Context, platform.ProxySettings) error {
	return nil
}

func (f *fakePlatformController) DisableSystemProxy(context.Context) error { return nil }

func (f *fakePlatformController) SetupTUN(_ context.Context, iface string) error {
	f.setupCalls = append(f.setupCalls, iface)
	return f.setupErr
}

func (f *fakePlatformController) TeardownTUN(_ context.Context, iface string) error {
	f.teardownCalls = append(f.teardownCalls, iface)
	return f.teardownErr
}

func (f *fakePlatformController) SupportsTUN() bool { return true }

func TestRuntimeStopWithoutInstance(t *testing.T) {
	runtime := NewXrayRuntimeWith(&fakePlatformController{})
	require.NoError(t, runtime.Stop(context.Background()))
}

func TestNewXrayRuntimeWithNilController(t *testing.T) {
	runtime := NewXrayRuntimeWith(nil)
	require.NotNil(t, runtime)
}

func TestApplyConfigReadFailure(t *testing.T) {
	runtime := NewXrayRuntimeWith(&fakePlatformController{})
	err := runtime.ApplyConfig(context.Background(), "/path/does/not/exist.json")
	require.Error(t, err)
}
