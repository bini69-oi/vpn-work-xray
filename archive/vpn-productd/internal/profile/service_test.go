package profile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/internal/domain"
)

type repoStub struct {
	profiles       []domain.Profile
	state          map[string]string
	upserted       int
	checkAccessErr error
}

func (r *repoStub) UpsertProfile(_ context.Context, profile domain.Profile) error {
	r.upserted++
	for idx := range r.profiles {
		if r.profiles[idx].ID == profile.ID {
			r.profiles[idx] = profile
			return nil
		}
	}
	r.profiles = append(r.profiles, profile)
	return nil
}
func (r *repoStub) GetProfile(_ context.Context, id string) (domain.Profile, error) {
	for _, p := range r.profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return domain.Profile{}, ErrNotFound
}
func (r *repoStub) ListProfiles(_ context.Context) ([]domain.Profile, error) { return r.profiles, nil }
func (r *repoStub) DeleteProfile(context.Context, string) error              { return nil }
func (r *repoStub) SetActiveProfile(context.Context, string) error           { return nil }
func (r *repoStub) GetActiveProfile(context.Context) (string, error)         { return "", nil }
func (r *repoStub) SetTrafficLimit(context.Context, string, int64) error     { return nil }
func (r *repoStub) AddTrafficUsage(context.Context, string, int64, int64) error {
	return nil
}
func (r *repoStub) SetBlocked(context.Context, string, bool) error { return nil }
func (r *repoStub) CheckAccess(context.Context, string) error      { return r.checkAccessErr }
func (r *repoStub) UpsertPanelUser(context.Context, domain.PanelUser) error {
	return nil
}
func (r *repoStub) ListPanelUsers(context.Context, string) ([]domain.PanelUser, error) {
	return nil, nil
}
func (r *repoStub) GetRuntimeState(_ context.Context, key string) (string, error) {
	return r.state[key], nil
}
func (r *repoStub) SetRuntimeState(_ context.Context, key, value string) error {
	r.state[key] = value
	return nil
}

func TestRotateRealityShortIDs(t *testing.T) {
	repo := &repoStub{
		state: map[string]string{},
		profiles: []domain.Profile{
			{ID: "p1", Name: "x", RouteMode: domain.RouteModeSplit, Endpoints: []domain.Endpoint{{Name: "v", Address: "1.1.1.1", Port: 443, ServerTag: "proxy", Protocol: domain.ProtocolVLESS, UUID: "11111111-2222-3333-4444-555555555555"}}, PreferredID: "v", ReconnectPolicy: domain.ReconnectPolicy{MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second}},
		},
	}
	svc := NewService(repo)
	require.NoError(t, svc.RotateRealityShortIDs(context.Background(), 0))
	require.Equal(t, 1, repo.upserted)
	got, err := repo.GetProfile(context.Background(), "p1")
	require.NoError(t, err)
	require.NotEmpty(t, got.Endpoints[0].RealityShortID)
	require.Len(t, got.Endpoints[0].RealityShortIDs, 1)
}

func TestCheckAccessPassThrough(t *testing.T) {
	repo := &repoStub{state: map[string]string{}, checkAccessErr: errors.New("blocked")}
	svc := NewService(repo)
	err := svc.CheckAccess(context.Background(), "p1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}
