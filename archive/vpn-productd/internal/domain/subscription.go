package domain

import "time"

type Subscription struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	UserID     string     `json:"userId"`
	Token      string     `json:"-"`
	TokenHint  string     `json:"tokenHint,omitempty"`
	ProfileIDs []string   `json:"profileIds"`
	Status     string     `json:"status"`
	Revoked    bool       `json:"revoked"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
	RotatedAt  *time.Time `json:"rotatedAt,omitempty"`
	RotationCount int     `json:"rotationCount,omitempty"`
	LastAccessAt *time.Time `json:"lastAccessAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type SubscriptionIssue struct {
	ID             string    `json:"id"`
	UserID         string    `json:"userId"`
	SubscriptionID string    `json:"subscriptionId"`
	TokenHint      string    `json:"tokenHint,omitempty"`
	Source         string    `json:"source,omitempty"`
	IssuedAt       time.Time `json:"issuedAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

