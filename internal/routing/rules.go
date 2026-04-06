package routing

import (
	"encoding/json"
	"fmt"
	"os"
)

// RoutingConfigFile is persisted at RoutingRulesPath when operators override routing via API.
type RoutingConfigFile struct {
	DomainStrategy string           `json:"domainStrategy"`
	DomainMatcher  string           `json:"domainMatcher"`
	Rules          []map[string]any `json:"rules"`
}

// LoadRoutingConfigFile reads routing JSON from path. Empty or missing file returns nil, nil.
func LoadRoutingConfigFile(path string) (*RoutingConfigFile, error) {
	// #nosec G304 -- path is operator-controlled routing config location.
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read routing config: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	var doc RoutingConfigFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse routing config: %w", err)
	}
	if len(doc.Rules) == 0 {
		return nil, nil
	}
	return &doc, nil
}

// SaveRoutingConfigFile writes routing JSON atomically.
func SaveRoutingConfigFile(path string, doc *RoutingConfigFile) error {
	if doc == nil {
		return fmt.Errorf("routing config is nil")
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal routing config: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write routing config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename routing config: %w", err)
	}
	return nil
}
