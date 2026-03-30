package sources

import (
	"context"

	"github.com/xtls/xray-core/product/domain"
)

// Source is a future extension point for remote/local profile updates.
type Source interface {
	Name() string
	FetchProfiles(ctx context.Context) ([]domain.Profile, error)
}
