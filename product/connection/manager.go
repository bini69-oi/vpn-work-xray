package connection

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/xtls/xray-core/product/configgen"
	"github.com/xtls/xray-core/product/domain"
	perrors "github.com/xtls/xray-core/product/errors"
	"github.com/xtls/xray-core/product/logging"
	"github.com/xtls/xray-core/product/reconnect"
	"github.com/xtls/xray-core/product/telemetry"
)

type ProfileReader interface {
	Get(ctx context.Context, id string) (domain.Profile, error)
	CheckAccess(ctx context.Context, profileID string) error
}

type Manager struct {
	mu        sync.RWMutex
	profiles  ProfileReader
	gen       *configgen.Generator
	runtime   RuntimeController
	reconnect *reconnect.Engine
	log       *logging.Logger
	configLog *logging.Logger
	status    domain.RuntimeStatus
}

func NewManager(profiles ProfileReader, gen *configgen.Generator, runtime RuntimeController, reconnectEngine *reconnect.Engine, logger *logging.Logger, configLogger *logging.Logger) *Manager {
	return &Manager{
		profiles:  profiles,
		gen:       gen,
		runtime:   runtime,
		reconnect: reconnectEngine,
		log:       logger,
		configLog: configLogger,
		status: domain.RuntimeStatus{
			State:     domain.StateIdle,
			UpdatedAt: time.Now().UTC(),
		},
	}
}

func (m *Manager) Status(_ context.Context) domain.RuntimeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) setStatus(state domain.ConnectionState, profileID, endpoint, lastErr string) {
	m.status = domain.RuntimeStatus{
		State:          state,
		ActiveProfile:  profileID,
		ActiveEndpoint: endpoint,
		LastError:      lastErr,
		UpdatedAt:      time.Now().UTC(),
	}
}

func (m *Manager) Connect(ctx context.Context, profileID string) error {
	profile, err := m.profiles.Get(ctx, profileID)
	if err != nil {
		return err
	}
	if err := m.profiles.CheckAccess(ctx, profileID); err != nil {
		return err
	}
	if profile.Blocked {
		return errors.New("profile is blocked by quota policy")
	}
	if profile.TrafficLimitMB > 0 {
		limitBytes := profile.TrafficLimitMB * 1024 * 1024
		if profile.TrafficUsedUp+profile.TrafficUsedDown >= limitBytes {
			return errors.New("profile traffic quota exceeded")
		}
	}
	endpoints := endpointOrder(profile)
	if err := validateConnectSecurity(profile, endpoints); err != nil {
		return err
	}

	m.mu.Lock()
	m.setStatus(domain.StateConnecting, profile.ID, "", "")
	m.mu.Unlock()

	var lastErr error
	for i, endpoint := range endpoints {
		artifact, err := m.gen.Generate(profile, endpoint.Name)
		if err != nil {
			lastErr = err
			m.configLog.Errorf("config generation failed profile=%s endpoint=%s err=%v code=%s", profile.ID, endpoint.Name, err, perrors.ErrConfig.Code)
			continue
		}
		for _, warning := range artifact.Warnings {
			m.log.Warnf("config warning profile=%s endpoint=%s warning=%s", profile.ID, endpoint.Name, warning)
		}
		if err := m.runtime.ApplyConfig(ctx, artifact.Path); err == nil {
			m.mu.Lock()
			m.setStatus(domain.StateConnected, profile.ID, endpoint.Name, "")
			m.mu.Unlock()
			telemetry.Default().XrayStatus.Set(1)
			telemetry.Default().ActiveSessions.Set(1)
			m.log.Infof("connected profile=%s endpoint=%s", profile.ID, endpoint.Name)
			return nil
		} else {
			lastErr = err
			telemetry.Default().XrayStatus.Set(0)
			telemetry.Default().ActiveSessions.Set(0)
			m.log.Warnf("xray start failed profile=%s endpoint=%s err=%v code=%s", profile.ID, endpoint.Name, err, perrors.ErrCoreStart.Code)
		}
		if i == len(endpoints)-1 {
			break
		}
		decision := m.reconnect.Next(profile.ReconnectPolicy, i)
		if !decision.Retry {
			break
		}
		m.mu.Lock()
		m.setStatus(domain.StateReconnecting, profile.ID, endpoint.Name, decision.Delay.String())
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(decision.Delay):
		}
	}

	m.mu.Lock()
	m.setStatus(domain.StateFailed, profile.ID, "", errToString(lastErr))
	m.mu.Unlock()
	return lastErr
}

func (m *Manager) Disconnect(ctx context.Context) error {
	if err := m.runtime.Stop(ctx); err != nil {
		return err
	}
	telemetry.Default().XrayStatus.Set(0)
	telemetry.Default().ActiveSessions.Set(0)
	m.mu.Lock()
	m.setStatus(domain.StateIdle, "", "", "")
	m.mu.Unlock()
	m.log.Infof("disconnected")
	return nil
}

func endpointOrder(profile domain.Profile) []domain.Endpoint {
	lookup := make(map[string]domain.Endpoint, len(profile.Endpoints))
	for _, ep := range profile.Endpoints {
		lookup[ep.Name] = ep
	}
	ordered := make([]domain.Endpoint, 0, len(profile.Endpoints))
	if preferred, ok := lookup[profile.PreferredID]; ok {
		ordered = append(ordered, preferred)
	}
	for _, name := range profile.Fallback.EndpointIDs {
		ep, ok := lookup[name]
		if !ok || containsEndpoint(ordered, ep.Name) {
			continue
		}
		ordered = append(ordered, ep)
	}
	for _, ep := range profile.Endpoints {
		if containsEndpoint(ordered, ep.Name) {
			continue
		}
		ordered = append(ordered, ep)
	}
	if profile.Security.DisableProtocolDowngrade && len(ordered) > 0 {
		strongest := protocolStrength(ordered[0].Protocol)
		filtered := ordered[:0]
		for _, ep := range ordered {
			if protocolStrength(ep.Protocol) < strongest {
				continue
			}
			filtered = append(filtered, ep)
		}
		ordered = filtered
	}
	return ordered
}

func containsEndpoint(endpoints []domain.Endpoint, name string) bool {
	for _, ep := range endpoints {
		if ep.Name == name {
			return true
		}
	}
	return false
}

func protocolStrength(p domain.Protocol) int {
	switch p {
	case domain.ProtocolVLESS:
		return 3
	case domain.ProtocolHysteria:
		return 2
	case domain.ProtocolWG:
		return 1
	default:
		return 0
	}
}

func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func validateConnectSecurity(profile domain.Profile, endpoints []domain.Endpoint) error {
	if len(endpoints) == 0 {
		return errors.New("profile has no endpoints")
	}
	if profile.Security.Level == domain.SecurityLevelStrict && len(endpoints) == 1 && len(profile.Endpoints) > 1 {
		return fmt.Errorf("strict policy blocked insecure fallback chain")
	}
	return nil
}
