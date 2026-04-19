package api

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	perrors "github.com/xtls/xray-core/internal/errors"
	"github.com/xtls/xray-core/internal/integration/xui"
	profilepkg "github.com/xtls/xray-core/internal/profile"
)

func (s *Server) handleAdminUser(w http.ResponseWriter, r *http.Request) {
	if s.subs == nil {
		writeError(w, http.StatusNotImplemented, perrors.New("VPN_SUBS_001", "subscriptions are not configured"))
		return
	}
	tail := trimKnownPrefix(r.URL.Path, "/admin/user/", "/v1/user/")
	tail = strings.Trim(tail, "/")
	parts := strings.Split(tail, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusNotFound, perrors.New("VPN_API_MINIAPP_404", "user endpoint not found"))
		return
	}
	userID := strings.TrimSpace(parts[0])
	action := strings.ToLower(strings.TrimSpace(parts[1]))
	if !strings.HasPrefix(userID, "tg_") {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_MINIAPP_001", "userId must start with tg_"))
		return
	}
	switch action {
	case "status":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
			return
		}
		s.handleMiniappUserStatus(w, r, userID)
		return
	case "profile":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
			return
		}
		s.handleMiniappUserProfile(w, r, userID)
		return
	default:
		writeError(w, http.StatusNotFound, perrors.New("VPN_API_MINIAPP_404", "user endpoint not found"))
	}
}

func (s *Server) miniappPublicSubscriptionURL(token string) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("VPN_PRODUCT_PUBLIC_BASE_URL")), "/")
	if base == "" || strings.TrimSpace(token) == "" {
		return ""
	}
	return base + "/public/subscriptions/" + strings.TrimSpace(token)
}

func (s *Server) handleMiniappUserProfile(w http.ResponseWriter, r *http.Request, userID string) {
	ctx := r.Context()
	sub, err := s.subs.GetLastByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, profilepkg.ErrNotFound) {
			writeError(w, http.StatusNotFound, perrors.New("VPN_SUBS_404", "subscription not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var expiresISO string
	if sub.ExpiresAt != nil && !sub.ExpiresAt.IsZero() {
		expiresISO = sub.ExpiresAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"userId":         userID,
		"subscriptionId": sub.ID,
		"status":         sub.Status,
		"revoked":        sub.Revoked,
		"expiresAt":      expiresISO,
		"tokenHint":      sub.TokenHint,
		"profileIds":     sub.ProfileIDs,
	})
}

func (s *Server) handleMiniappUserStatus(w http.ResponseWriter, r *http.Request, userID string) {
	ctx := r.Context()
	sub, err := s.subs.GetActiveByUser(ctx, userID)
	if err != nil {
		if !errors.Is(err, profilepkg.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		sub, err = s.subs.GetLastByUser(ctx, userID)
		if err != nil {
			if errors.Is(err, profilepkg.ErrNotFound) {
				writeJSON(w, http.StatusOK, map[string]any{
					"active":     false,
					"plan":       "Стандарт",
					"daysLeft":   0,
					"expiresAt":  "",
					"usedBytes":  0,
					"totalBytes": 1024 * 1024 * 1024 * 1024,
					"subUrl":     "",
					"url":        "",
					"happLink":   "",
				})
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	now := time.Now().UTC()
	active := !sub.Revoked && strings.EqualFold(strings.TrimSpace(sub.Status), "active")
	if sub.ExpiresAt != nil && !sub.ExpiresAt.IsZero() && !sub.ExpiresAt.UTC().After(now) {
		active = false
	}

	var expiresISO string
	var expiresUnix int64
	if sub.ExpiresAt != nil && !sub.ExpiresAt.IsZero() {
		expiresISO = sub.ExpiresAt.UTC().Format(time.RFC3339)
		expiresUnix = sub.ExpiresAt.UTC().Unix()
	}

	used := int64(0)
	total := int64(1024) * 1024 * 1024 * 1024
	xuiDBPath, xuiInboundPort := s.resolveXUIConfig()
	if usage, uErr := xui.GetClientUsage(ctx, xuiDBPath, xuiInboundPort, userID); uErr == nil {
		used = maxInt64(usage.UpBytes+usage.DownBytes, 0)
		if usage.Total > 0 {
			total = usage.Total
		}
		if expiresUnix == 0 && usage.ExpiryMS > 0 {
			expiresUnix = usage.ExpiryMS / 1000
			expiresISO = time.Unix(expiresUnix, 0).UTC().Format(time.RFC3339)
		}
	}

	daysLeft := int64(0)
	if expiresUnix > 0 {
		daysLeft = daysLeftFromUnix(expiresUnix)
	}

	token, tokErr := s.subs.RevealSubscriptionToken(ctx, sub.ID)
	subURL := ""
	if tokErr == nil && strings.TrimSpace(token) != "" {
		subURL = s.miniappPublicSubscriptionURL(token)
	}

	happLink := ""
	if s.delivery != nil && len(sub.ProfileIDs) > 0 {
		if p, perr := s.profiles.Get(ctx, sub.ProfileIDs[0]); perr == nil {
			if links, lerr := s.delivery.GenerateHappImportLinks(p); lerr == nil && len(links) > 0 {
				happLink = links[0]
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"active":     active,
		"plan":       "Стандарт",
		"daysLeft":   daysLeft,
		"expiresAt":  expiresISO,
		"usedBytes":  used,
		"totalBytes": total,
		"subUrl":     subURL,
		"url":        subURL,
		"happLink":   happLink,
	})
}
