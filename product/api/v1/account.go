package v1

import "github.com/xtls/xray-core/product/domain/account"

// AccountResponse reserves stable API shape for future auth/subscription integration.
type AccountResponse struct {
	Account account.SubscriptionInfo `json:"account"`
}
