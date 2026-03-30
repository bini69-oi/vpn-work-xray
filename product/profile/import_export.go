package profile

import (
	"context"
	"encoding/json"
	"io"

	"github.com/xtls/xray-core/product/domain"
)

func (s *Service) ImportJSON(ctx context.Context, r io.Reader) (domain.Profile, error) {
	var p domain.Profile
	if err := json.NewDecoder(r).Decode(&p); err != nil {
		return domain.Profile{}, err
	}
	return s.Save(ctx, p)
}

func (s *Service) ExportJSON(ctx context.Context, profileID string, w io.Writer) error {
	p, err := s.Get(ctx, profileID)
	if err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(p)
}
