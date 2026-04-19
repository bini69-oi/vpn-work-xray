package account

import "time"

type PlanStatus string

const (
	PlanStatusUnknown PlanStatus = "unknown"
	PlanStatusActive  PlanStatus = "active"
	PlanStatusExpired PlanStatus = "expired"
)

// SubscriptionInfo is a forward-compatible placeholder for billing/auth integration.
type SubscriptionInfo struct {
	UserID       string     `json:"userId"`
	PlanID       string     `json:"planId"`
	Status       PlanStatus `json:"status"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
	LastSyncAt   *time.Time `json:"lastSyncAt,omitempty"`
	FeatureFlags []string   `json:"featureFlags,omitempty"`
}
