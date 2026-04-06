package routing

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// WarpDomainsFile matches /etc/vpn-product/warp_domains.json.
type WarpDomainsFile struct {
	Domains     []string  `json:"domains"`
	GeositeTags []string  `json:"geosite_tags"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DefaultWarpDomains returns built-in lists when no file is present.
func DefaultWarpDomains() WarpDomainsFile {
	return WarpDomainsFile{
		Domains: []string{
			"instagram.com", "facebook.com", "twitter.com", "x.com",
			"linkedin.com", "medium.com", "quora.com", "discord.com", "discord.gg",
			"spotify.com", "soundcloud.com", "twitch.tv", "notion.so", "archive.org",
			"patreon.com", "openai.com", "chatgpt.com", "claude.ai",
		},
		GeositeTags: []string{"openai", "google-ai"},
		UpdatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// LoadWarpDomainsOrDefault reads warp_domains.json or returns defaults.
func LoadWarpDomainsOrDefault(path string) WarpDomainsFile {
	// #nosec G304 -- path is operator-controlled warp domain list.
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return DefaultWarpDomains()
	}
	var w WarpDomainsFile
	if err := json.Unmarshal(data, &w); err != nil {
		return DefaultWarpDomains()
	}
	if len(w.Domains) == 0 && len(w.GeositeTags) == 0 {
		return DefaultWarpDomains()
	}
	return w
}

// SaveWarpDomainsFile writes warp_domains.json atomically.
func SaveWarpDomainsFile(path string, w *WarpDomainsFile) error {
	if w == nil {
		return fmt.Errorf("warp domains is nil")
	}
	w.UpdatedAt = time.Now().UTC()
	raw, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal warp domains: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write warp domains: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename warp domains: %w", err)
	}
	return nil
}
