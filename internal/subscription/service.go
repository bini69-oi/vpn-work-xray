package subscription

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xtls/xray-core/internal/delivery"
	"github.com/xtls/xray-core/internal/domain"
	profilepkg "github.com/xtls/xray-core/internal/profile"
)

type Repository interface {
	CreateSubscription(ctx context.Context, item domain.Subscription) (domain.Subscription, error)
	GetSubscription(ctx context.Context, id string) (domain.Subscription, error)
	GetActiveSubscriptionByUser(ctx context.Context, userID string) (domain.Subscription, error)
	GetLastSubscriptionByUser(ctx context.Context, userID string) (domain.Subscription, error)
	GetSubscriptionByToken(ctx context.Context, token string) (domain.Subscription, error)
	RevokeSubscription(ctx context.Context, id string) error
	RevokeActiveSubscriptionsByUser(ctx context.Context, userID string) (int64, error)
	SetSubscriptionExpiration(ctx context.Context, id string, expiresAt *time.Time) error
	ReactivateSubscription(ctx context.Context, id string, expiresAt *time.Time) error
	RotateSubscriptionToken(ctx context.Context, id, token string) (domain.Subscription, error)
	TouchSubscriptionAccess(ctx context.Context, id string) error
	UpdateSubscriptionProfiles(ctx context.Context, id string, profileIDs []string) error
	CleanupExpired(ctx context.Context, retentionDays int) (int64, error)
}

type ProfileProvider interface {
	Get(ctx context.Context, id string) (domain.Profile, error)
}

type IssueRepository interface {
	CreateSubscriptionIssue(ctx context.Context, item domain.SubscriptionIssue) error
	ListSubscriptionIssues(ctx context.Context, userID string, limit int) ([]domain.SubscriptionIssue, error)
}

type Service struct {
	repo     Repository
	issues   IssueRepository
	profiles ProfileProvider
	delivery *delivery.Service
}

func NewService(repo Repository, profiles ProfileProvider, links *delivery.Service) *Service {
	if links == nil {
		links = delivery.NewService()
	}
	var issues IssueRepository
	if repoWithIssues, ok := repo.(IssueRepository); ok {
		issues = repoWithIssues
	}
	return &Service{repo: repo, issues: issues, profiles: profiles, delivery: links}
}

func (s *Service) Create(ctx context.Context, item domain.Subscription) (domain.Subscription, error) {
	if len(item.ProfileIDs) == 0 {
		return domain.Subscription{}, errors.New("profileIds are required")
	}
	if item.ID == "" {
		item.ID = randomID("sub")
	}
	token, err := randomToken(32)
	if err != nil {
		return domain.Subscription{}, err
	}
	item.Token = token
	if item.Name == "" {
		item.Name = "happ-subscription"
	}
	now := time.Now().UTC()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	item.Status = "active"
	return s.repo.CreateSubscription(ctx, item)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Subscription, error) {
	return s.repo.GetSubscription(ctx, id)
}

func (s *Service) Revoke(ctx context.Context, id string) error {
	return s.repo.RevokeSubscription(ctx, id)
}

func (s *Service) Rotate(ctx context.Context, id string) (domain.Subscription, error) {
	token, err := randomToken(32)
	if err != nil {
		return domain.Subscription{}, err
	}
	item, err := s.repo.RotateSubscriptionToken(ctx, id, token)
	if err != nil {
		return domain.Subscription{}, err
	}
	item.Token = token
	return item, nil
}

func (s *Service) BuildContentByToken(ctx context.Context, token string) (string, domain.Subscription, error) {
	sub, err := s.repo.GetSubscriptionByToken(ctx, token)
	if err != nil {
		return "", domain.Subscription{}, err
	}
	status := strings.ToLower(strings.TrimSpace(sub.Status))
	if status != "" && status != "active" {
		return "", domain.Subscription{}, errors.New("subscription inactive")
	}
	if sub.Revoked {
		sub.Status = "revoked"
		return "", domain.Subscription{}, errors.New("subscription revoked")
	}
	if sub.ExpiresAt != nil && time.Now().UTC().After(*sub.ExpiresAt) {
		sub.Status = "expired"
		return "", domain.Subscription{}, errors.New("subscription expired")
	}
	lines := make([]string, 0, len(sub.ProfileIDs)*2)
	for _, profileID := range sub.ProfileIDs {
		profile, err := s.profiles.Get(ctx, profileID)
		if err != nil {
			continue
		}
		if !profile.Enabled {
			continue
		}
		if err := profilepkg.ValidateProfile(profile); err != nil {
			continue
		}
		if profile.Blocked {
			continue
		}
		if profile.SubscriptionExpiresAt != nil && time.Now().UTC().After(*profile.SubscriptionExpiresAt) {
			continue
		}
		links, err := s.delivery.GenerateHappImportLinks(profile)
		if err != nil {
			continue
		}
		lines = append(lines, links...)
	}
	if len(lines) == 0 {
		return "", domain.Subscription{}, errors.New("no valid profiles available for subscription")
	}
	_ = s.repo.TouchSubscriptionAccess(ctx, sub.ID)
	sub.Status = "active"
	return strings.Join(lines, "\n"), sub, nil
}

func (s *Service) IssueLink30Days(ctx context.Context, userID string, profileIDs []string, name string, source string) (domain.Subscription, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.Subscription{}, errors.New("userId is required")
	}
	item, isNew, err := s.RenewOrCreate(ctx, userID, 30, profileIDs, name, source)
	if err != nil {
		return domain.Subscription{}, err
	}
	// Only record an issuance event when a new token is created.
	if isNew && s.issues != nil {
		now := time.Now().UTC()
		expires := now.Add(30 * 24 * time.Hour)
		if item.ExpiresAt != nil {
			expires = item.ExpiresAt.UTC()
		}
		_ = s.issues.CreateSubscriptionIssue(ctx, domain.SubscriptionIssue{
			ID:             randomID("issue"),
			UserID:         userID,
			SubscriptionID: item.ID,
			TokenHint:      item.TokenHint,
			Source:         strings.TrimSpace(source),
			IssuedAt:       now,
			ExpiresAt:      expires,
		})
	}
	return item, nil
}

func (s *Service) RenewOrCreate(ctx context.Context, userID string, days int, profileIDs []string, name string, _ string) (sub domain.Subscription, isNew bool, err error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.Subscription{}, false, errors.New("userId is required")
	}
	if days <= 0 {
		days = 30
	}
	last, err := s.repo.GetLastSubscriptionByUser(ctx, userID)
	switch {
	case err == nil:
		base := time.Now().UTC()
		if last.ExpiresAt != nil && last.ExpiresAt.After(base) {
			base = last.ExpiresAt.UTC()
		}
		expires := base.Add(time.Duration(days) * 24 * time.Hour)
		if err := s.repo.ReactivateSubscription(ctx, last.ID, &expires); err != nil {
			return domain.Subscription{}, false, err
		}
		reactivated, err := s.repo.GetSubscription(ctx, last.ID)
		if err != nil {
			return domain.Subscription{}, false, err
		}
		reactivated.Status = "active"
		return reactivated, false, nil
	case errors.Is(err, profilepkg.ErrNotFound):
		// fallthrough
	default:
		return domain.Subscription{}, false, err
	}

	now := time.Now().UTC()
	expires := now.Add(time.Duration(days) * 24 * time.Hour)
	created, err := s.Create(ctx, domain.Subscription{
		Name:       strings.TrimSpace(name),
		UserID:     userID,
		ProfileIDs: profileIDs,
		ExpiresAt:  &expires,
	})
	if err != nil {
		return domain.Subscription{}, false, err
	}
	return created, true, nil
}

func (s *Service) ListIssues(ctx context.Context, userID string, limit int) ([]domain.SubscriptionIssue, error) {
	if s.issues == nil {
		return []domain.SubscriptionIssue{}, nil
	}
	return s.issues.ListSubscriptionIssues(ctx, userID, limit)
}

func (s *Service) AssignProfiles(ctx context.Context, subscriptionID string, profileIDs []string) error {
	if strings.TrimSpace(subscriptionID) == "" {
		return errors.New("subscriptionId is required")
	}
	if len(profileIDs) == 0 {
		return errors.New("profileIds are required")
	}
	return s.repo.UpdateSubscriptionProfiles(ctx, subscriptionID, profileIDs)
}

func (s *Service) GetActiveByUser(ctx context.Context, userID string) (domain.Subscription, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.Subscription{}, errors.New("userId is required")
	}
	return s.repo.GetActiveSubscriptionByUser(ctx, userID)
}

func (s *Service) ExtendActiveByUser(ctx context.Context, userID string, days int) (domain.Subscription, error) {
	if days <= 0 {
		days = 30
	}
	item, err := s.GetActiveByUser(ctx, userID)
	if err != nil {
		return domain.Subscription{}, err
	}
	base := time.Now().UTC()
	if item.ExpiresAt != nil && item.ExpiresAt.After(base) {
		base = item.ExpiresAt.UTC()
	}
	expires := base.Add(time.Duration(days) * 24 * time.Hour)
	if err := s.repo.SetSubscriptionExpiration(ctx, item.ID, &expires); err != nil {
		return domain.Subscription{}, err
	}
	return s.repo.GetSubscription(ctx, item.ID)
}

func (s *Service) BlockActiveByUser(ctx context.Context, userID string) (domain.Subscription, error) {
	item, err := s.GetActiveByUser(ctx, userID)
	if err != nil {
		return domain.Subscription{}, err
	}
	if err := s.repo.RevokeSubscription(ctx, item.ID); err != nil {
		return domain.Subscription{}, err
	}
	return s.repo.GetSubscription(ctx, item.ID)
}

func (s *Service) CleanupExpired(ctx context.Context, retentionDays int) (int64, error) {
	return s.repo.CleanupExpired(ctx, retentionDays)
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func randomID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}

