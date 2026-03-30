package v1

import "github.com/xtls/xray-core/product/domain"

type ConnectRequest struct {
	ProfileID string `json:"profileId"`
}

type ProfileResponse struct {
	Profile domain.Profile `json:"profile"`
}

type ProfilesResponse struct {
	Profiles []domain.Profile `json:"profiles"`
}

type StatusResponse struct {
	Status domain.RuntimeStatus `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

type CreateSubscriptionRequest struct {
	Name       string   `json:"name"`
	UserID     string   `json:"userId"`
	ProfileIDs []string `json:"profileIds"`
	ExpiresAt  string   `json:"expiresAt,omitempty"`
}

type SubscriptionResponse struct {
	Subscription domain.Subscription `json:"subscription"`
	URL          string              `json:"url,omitempty"`
}

type IssueLinkRequest struct {
	UserID     string   `json:"userId"`
	ProfileIDs []string `json:"profileIds,omitempty"`
	Name       string   `json:"name,omitempty"`
	Source     string   `json:"source,omitempty"`
}

type IssueLinkResponse struct {
	Subscription domain.Subscription `json:"subscription"`
	URL          string              `json:"url"`
	Days         int                 `json:"days"`
	AppliedTo3XUI bool               `json:"appliedTo3xui"`
	ProfileID    string              `json:"profileId,omitempty"`
	ApplyError   string              `json:"applyError,omitempty"`
}

type IssueHistoryResponse struct {
	Items []domain.SubscriptionIssue `json:"items"`
}

type ApplyTo3XUIRequest struct {
	UserID         string `json:"userId"`
	SubscriptionID string `json:"subscriptionId"`
	BaseProfileID  string `json:"baseProfileId,omitempty"`
}

type ApplyTo3XUIResponse struct {
	OK             bool   `json:"ok"`
	SubscriptionID string `json:"subscriptionId"`
	ProfileID      string `json:"profileId"`
}

type LifecycleRequest struct {
	UserID string `json:"userId"`
	Action string `json:"action"` // renew|block
	Days   int    `json:"days,omitempty"`
}

type LifecycleResponse struct {
	OK             bool   `json:"ok"`
	Action         string `json:"action"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
	ProfileID      string `json:"profileId,omitempty"`
	ExpiresAt      string `json:"expiresAt,omitempty"`
}
