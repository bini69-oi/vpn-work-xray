package profile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/xtls/xray-core/product/domain"
)

var ErrNotFound = errors.New("profile not found")

type Repository interface {
	UpsertProfile(ctx context.Context, profile domain.Profile) error
	GetProfile(ctx context.Context, id string) (domain.Profile, error)
	ListProfiles(ctx context.Context) ([]domain.Profile, error)
	DeleteProfile(ctx context.Context, id string) error
	SetActiveProfile(ctx context.Context, id string) error
	GetActiveProfile(ctx context.Context) (string, error)
	SetTrafficLimit(ctx context.Context, profileID string, limitMB int64) error
	AddTrafficUsage(ctx context.Context, profileID string, uploadBytes, downloadBytes int64) error
	SetBlocked(ctx context.Context, profileID string, blocked bool) error
	CheckAccess(ctx context.Context, profileID string) error
	UpsertPanelUser(ctx context.Context, user domain.PanelUser) error
	ListPanelUsers(ctx context.Context, panel string) ([]domain.PanelUser, error)
	GetRuntimeState(ctx context.Context, key string) (string, error)
	SetRuntimeState(ctx context.Context, key, value string) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(ctx context.Context, p domain.Profile) (domain.Profile, error) {
	if err := ValidateProfile(p); err != nil {
		return domain.Profile{}, err
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	if err := s.repo.UpsertProfile(ctx, p); err != nil {
		return domain.Profile{}, err
	}
	return p, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Profile, error) {
	return s.repo.GetProfile(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]domain.Profile, error) {
	return s.repo.ListProfiles(ctx)
}

func (s *Service) Count(ctx context.Context) (int, error) {
	items, err := s.repo.ListProfiles(ctx)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.DeleteProfile(ctx, id)
}

func (s *Service) SetActive(ctx context.Context, id string) error {
	p, err := s.repo.GetProfile(ctx, id)
	if err != nil {
		return err
	}
	if !p.Enabled {
		return errors.New("profile is disabled")
	}
	return s.repo.SetActiveProfile(ctx, id)
}

func (s *Service) Active(ctx context.Context) (domain.Profile, error) {
	id, err := s.repo.GetActiveProfile(ctx)
	if err != nil {
		return domain.Profile{}, err
	}
	if id == "" {
		return domain.Profile{}, ErrNotFound
	}
	return s.repo.GetProfile(ctx, id)
}

func (s *Service) SetTrafficLimit(ctx context.Context, profileID string, limitMB int64) error {
	return s.repo.SetTrafficLimit(ctx, profileID, limitMB)
}

func (s *Service) AddTrafficUsage(ctx context.Context, profileID string, uploadBytes, downloadBytes int64) error {
	return s.repo.AddTrafficUsage(ctx, profileID, uploadBytes, downloadBytes)
}

func (s *Service) SetBlocked(ctx context.Context, profileID string, blocked bool) error {
	return s.repo.SetBlocked(ctx, profileID, blocked)
}

func (s *Service) CheckAccess(ctx context.Context, profileID string) error {
	return s.repo.CheckAccess(ctx, profileID)
}

func (s *Service) UpsertPanelUser(ctx context.Context, user domain.PanelUser) error {
	return s.repo.UpsertPanelUser(ctx, user)
}

func (s *Service) ListPanelUsers(ctx context.Context, panel string) ([]domain.PanelUser, error) {
	return s.repo.ListPanelUsers(ctx, panel)
}

func (s *Service) RotateRealityShortIDs(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 7 * 24 * time.Hour
	}
	lastRaw, err := s.repo.GetRuntimeState(ctx, "shortid_rotation_at")
	if err == nil && lastRaw != "" {
		if ts, parseErr := time.Parse(time.RFC3339Nano, lastRaw); parseErr == nil && time.Since(ts) < interval {
			return nil
		}
	}
	items, err := s.repo.ListProfiles(ctx)
	if err != nil {
		return err
	}
	for _, p := range items {
		changed := false
		for idx := range p.Endpoints {
			if p.Endpoints[idx].Protocol != domain.ProtocolVLESS {
				continue
			}
			sid, sidErr := randomShortID()
			if sidErr != nil {
				return sidErr
			}
			p.Endpoints[idx].RealityShortID = sid
			p.Endpoints[idx].RealityShortIDs = []string{sid}
			changed = true
		}
		if !changed {
			continue
		}
		p.UpdatedAt = time.Now().UTC()
		if err := s.repo.UpsertProfile(ctx, p); err != nil {
			return err
		}
	}
	return s.repo.SetRuntimeState(ctx, "shortid_rotation_at", time.Now().UTC().Format(time.RFC3339Nano))
}

func randomShortID() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
