package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	v1 "github.com/xtls/xray-core/product/api/v1"
	"github.com/xtls/xray-core/product/delivery"
	"github.com/xtls/xray-core/product/diagnostics"
	"github.com/xtls/xray-core/product/domain"
	"github.com/xtls/xray-core/product/domain/account"
	perrors "github.com/xtls/xray-core/product/errors"
	"github.com/xtls/xray-core/product/integration/xui"
	"github.com/xtls/xray-core/product/logging"
	"github.com/xtls/xray-core/product/telemetry"
)

type ConnectionService interface {
	Connect(ctx context.Context, profileID string) error
	Disconnect(ctx context.Context) error
	Status(ctx context.Context) domain.RuntimeStatus
}

type ProfileService interface {
	List(ctx context.Context) ([]domain.Profile, error)
	Get(ctx context.Context, id string) (domain.Profile, error)
	Save(ctx context.Context, p domain.Profile) (domain.Profile, error)
	Delete(ctx context.Context, id string) error
	SetTrafficLimit(ctx context.Context, profileID string, limitMB int64) error
	AddTrafficUsage(ctx context.Context, profileID string, uploadBytes, downloadBytes int64) error
	SetBlocked(ctx context.Context, profileID string, blocked bool) error
	UpsertPanelUser(ctx context.Context, user domain.PanelUser) error
	ListPanelUsers(ctx context.Context, panel string) ([]domain.PanelUser, error)
}

type SubscriptionService interface {
	Create(ctx context.Context, item domain.Subscription) (domain.Subscription, error)
	Get(ctx context.Context, id string) (domain.Subscription, error)
	Revoke(ctx context.Context, id string) error
	Rotate(ctx context.Context, id string) (domain.Subscription, error)
	BuildContentByToken(ctx context.Context, token string) (string, domain.Subscription, error)
	IssueLink30Days(ctx context.Context, userID string, profileIDs []string, name string, source string) (domain.Subscription, error)
	ListIssues(ctx context.Context, userID string, limit int) ([]domain.SubscriptionIssue, error)
	AssignProfiles(ctx context.Context, subscriptionID string, profileIDs []string) error
	GetActiveByUser(ctx context.Context, userID string) (domain.Subscription, error)
	ExtendActiveByUser(ctx context.Context, userID string, days int) (domain.Subscription, error)
	BlockActiveByUser(ctx context.Context, userID string) (domain.Subscription, error)
}

type Server struct {
	connection  ConnectionService
	profiles    ProfileService
	diagnostics *diagnostics.Service
	delivery    *delivery.Service
	subs        SubscriptionService
	apiToken    string
	adminToken  string
	adminAllowlist []netip.Prefix
	xuiDBPath   string
	xuiInboundPort int
	log         *logging.Logger
	counter     uint64
	trafficMu   sync.Mutex
	rateMu      sync.Mutex
	rateByIP    map[string]rateEntry
	publicRateByKey map[string]rateEntry
	lastTraffic map[string]struct {
		up   int64
		down int64
	}
}

type rateEntry struct {
	WindowStart time.Time
	Count       int
}

func NewServer(connection ConnectionService, profiles ProfileService, diagnosticsService *diagnostics.Service, apiToken string, logger *logging.Logger, deliveryService *delivery.Service, subs SubscriptionService) *Server {
	if deliveryService == nil {
		deliveryService = delivery.NewService()
	}
	return &Server{
		connection:  connection,
		profiles:    profiles,
		diagnostics: diagnosticsService,
		delivery:    deliveryService,
		subs:        subs,
		apiToken:    apiToken,
		xuiDBPath:   "/etc/x-ui/x-ui.db",
		xuiInboundPort: 8443,
		log:         logger.WithModule("api"),
		rateByIP:    map[string]rateEntry{},
		publicRateByKey: map[string]rateEntry{},
		lastTraffic: map[string]struct {
			up   int64
			down int64
		}{},
	}
}

func (s *Server) WithAdminToken(token string) *Server {
	s.adminToken = strings.TrimSpace(token)
	return s
}

func (s *Server) WithAdminAllowlist(raw string) *Server {
	s.adminAllowlist = parseAllowlist(raw)
	return s
}

func (s *Server) With3XUI(dbPath string, inboundPort int) *Server {
	if strings.TrimSpace(dbPath) != "" {
		s.xuiDBPath = strings.TrimSpace(dbPath)
	}
	if inboundPort > 0 {
		s.xuiInboundPort = inboundPort
	}
	return s
}

func (s *Server) Handler() http.Handler {
	root := http.NewServeMux()
	admin := http.NewServeMux()
	// New production-oriented admin contract.
	admin.HandleFunc("/admin/profiles", s.handleProfiles)
	admin.HandleFunc("/admin/profiles/upsert", s.handleProfilesUpsert)
	admin.HandleFunc("/admin/profiles/delete", s.handleProfilesDelete)
	admin.HandleFunc("/admin/profiles/", s.handleProfileScoped)
	admin.HandleFunc("/admin/subscriptions", s.handleSubscriptions)
	admin.HandleFunc("/admin/subscriptions/", s.handleSubscriptionScoped)
	admin.HandleFunc("/admin/delivery/links", s.handleDeliveryLinks)
	admin.HandleFunc("/admin/health", s.handleHealth)
	admin.HandleFunc("/admin/readiness", s.handleReadiness)
	admin.Handle("/admin/metrics", telemetry.Handler())
	// Keep runtime/legacy control endpoints for backward compatibility.
	admin.HandleFunc("/v1/profiles", s.handleProfiles)
	admin.HandleFunc("/v1/profiles/upsert", s.handleProfilesUpsert)
	admin.HandleFunc("/v1/profiles/delete", s.handleProfilesDelete)
	admin.HandleFunc("/v1/connect", s.handleConnect)
	admin.HandleFunc("/v1/disconnect", s.handleDisconnect)
	admin.HandleFunc("/v1/status", s.handleStatus)
	admin.HandleFunc("/v1/account", s.handleAccount)
	admin.HandleFunc("/v1/diagnostics/snapshot", s.handleSnapshot)
	admin.HandleFunc("/v1/quota/set", s.handleQuotaSet)
	admin.HandleFunc("/v1/quota/add", s.handleQuotaAdd)
	admin.HandleFunc("/v1/quota/block", s.handleQuotaBlock)
	admin.HandleFunc("/v1/stats/profiles", s.handleProfileStats)
	admin.HandleFunc("/v1/integration/3xui/users/upsert", s.handlePanelUserUpsert)
	admin.HandleFunc("/v1/integration/3xui/users", s.handlePanelUsersList)
	admin.HandleFunc("/v1/health", s.handleHealth)
	admin.HandleFunc("/v1/readiness", s.handleReadiness)
	admin.HandleFunc("/v1/delivery/links", s.handleDeliveryLinks)
	admin.HandleFunc("/v1/profiles/", s.handleProfileScoped)
	admin.HandleFunc("/v1/subscriptions", s.handleSubscriptions)
	admin.HandleFunc("/v1/subscriptions/", s.handleSubscriptionScoped)
	admin.HandleFunc("/v1/issue/link", s.handleIssueLink)
	admin.HandleFunc("/v1/issue/history", s.handleIssueHistory)
	admin.HandleFunc("/v1/issue/apply-to-3xui", s.handleIssueApplyTo3XUI)
	admin.HandleFunc("/v1/subscriptions/lifecycle", s.handleSubscriptionLifecycle)
	admin.HandleFunc("/admin/issue/link", s.handleIssueLink)
	admin.HandleFunc("/admin/issue/history", s.handleIssueHistory)
	admin.HandleFunc("/admin/issue/apply-to-3xui", s.handleIssueApplyTo3XUI)
	admin.HandleFunc("/admin/subscriptions/lifecycle", s.handleSubscriptionLifecycle)
	admin.Handle("/v1/metrics", telemetry.Handler())

	root.HandleFunc("/public/subscriptions/", s.handlePublicSubscription)
	root.HandleFunc("/s/", s.handlePublicSubscription) // backward-compatible minimal public route
	root.Handle("/admin/", s.withAuthScope("admin", admin))
	root.Handle("/v1/", s.withAuthScope("v1", s.withDeprecated(admin)))
	return s.withObservability(s.withRateLimit(root))
}

func (s *Server) handleProfileScoped(w http.ResponseWriter, r *http.Request) {
	trimmed := trimKnownPrefix(r.URL.Path, "/v1/profiles/", "/admin/profiles/")
	if trimmed == "" {
		writeError(w, http.StatusNotFound, perrors.New("VPN_API_PROFILE_404", "profile endpoint not found"))
		return
	}
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 || parts[1] != "link" {
		writeError(w, http.StatusNotFound, perrors.New("VPN_API_PROFILE_404", "profile endpoint not found"))
		return
	}
	profileID := parts[0]
	profile, err := s.profiles.Get(r.Context(), profileID)
	if err != nil {
		writeError(w, http.StatusNotFound, perrors.New("VPN_API_PROFILE_404", "profile not found"))
		return
	}
	endpoint := strings.TrimSpace(r.URL.Query().Get("endpoint"))
	if endpoint == "" {
		endpoint = profile.PreferredID
	}
	if !profile.Enabled || profile.Blocked {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_PROFILE_005", "profile is not deliverable"))
		return
	}
	ep, ok := selectProfileEndpoint(profile, endpoint)
	if !ok || (ep.Protocol != domain.ProtocolVLESS && ep.Protocol != domain.ProtocolHysteria) {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_DELIVERY_002", "endpoint is not in supported happ flow"))
		return
	}
	link, err := s.delivery.GenerateLink(profile, endpoint)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profileId": profileID, "endpoint": endpoint, "link": link})
}

func (s *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	if s.subs == nil {
		writeError(w, http.StatusNotImplemented, perrors.New("VPN_SUBS_001", "subscriptions are not configured"))
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	var req v1.CreateSubscriptionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_008", "invalid json body"))
		return
	}
	var expiresAt *time.Time
	if strings.TrimSpace(req.ExpiresAt) != "" {
		ts, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, perrors.New("VPN_SUBS_002", "invalid expiresAt format"))
			return
		}
		expiresAt = &ts
	}
	item, err := s.subs.Create(r.Context(), domain.Subscription{
		Name:       strings.TrimSpace(req.Name),
		UserID:     strings.TrimSpace(req.UserID),
		ProfileIDs: req.ProfileIDs,
		ExpiresAt:  expiresAt,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, v1.SubscriptionResponse{
		Subscription: item,
		URL:          safeSubscriptionURLFromRequest(r, item.Token),
	})
}

func (s *Server) handleSubscriptionScoped(w http.ResponseWriter, r *http.Request) {
	if s.subs == nil {
		writeError(w, http.StatusNotImplemented, perrors.New("VPN_SUBS_001", "subscriptions are not configured"))
		return
	}
	trimmed := trimKnownPrefix(r.URL.Path, "/v1/subscriptions/", "/admin/subscriptions/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, perrors.New("VPN_SUBS_404", "subscription not found"))
		return
	}
	id := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		item, err := s.subs.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, perrors.New("VPN_SUBS_404", "subscription not found"))
			return
		}
		writeJSON(w, http.StatusOK, v1.SubscriptionResponse{Subscription: item, URL: safeSubscriptionURLFromRequest(r, item.Token)})
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := s.subs.Revoke(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
		return
	}
	if len(parts) == 2 && parts[1] == "revoke" && (r.Method == http.MethodPost || r.Method == http.MethodDelete) {
		if err := s.subs.Revoke(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
		return
	}
	if len(parts) == 2 && parts[1] == "rotate" && r.Method == http.MethodPost {
		item, err := s.subs.Rotate(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, v1.SubscriptionResponse{Subscription: item, URL: safeSubscriptionURLFromRequest(r, item.Token)})
		return
	}
	if len(parts) == 2 && parts[1] == "profiles" && r.Method == http.MethodGet {
		item, err := s.subs.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, perrors.New("VPN_SUBS_404", "subscription not found"))
			return
		}
		items := make([]domain.Profile, 0, len(item.ProfileIDs))
		for _, pid := range item.ProfileIDs {
			p, err := s.profiles.Get(r.Context(), pid)
			if err != nil {
				continue
			}
			items = append(items, p)
		}
		writeJSON(w, http.StatusOK, map[string]any{"subscriptionId": id, "profiles": items})
		return
	}
	writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
}

func (s *Server) handlePublicSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	if s.subs == nil {
		writeError(w, http.StatusNotFound, perrors.New("VPN_SUBS_404", "subscription not found"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	token := publicTokenFromPath(r.URL.Path)
	if token == "" {
		writeError(w, http.StatusNotFound, perrors.New("VPN_SUBS_404", "subscription not found"))
		return
	}
	content, _, err := s.subs.BuildContentByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, perrors.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, perrors.ErrUnauthorized)
			return
		}
		writeError(w, http.StatusNotFound, perrors.New("VPN_SUBS_404", "subscription not found"))
		return
	}
	title, expiresAtUnix, expiresAtISO, daysLeft := s.subscriptionMeta(r.Context(), token)
	if userInfo := s.subscriptionUserInfoHeader(r.Context(), token); userInfo != "" {
		w.Header().Set("Subscription-Userinfo", userInfo)
		// Some clients read this legacy alias.
		w.Header().Set("X-Subscription-Userinfo", userInfo)
	}
	if title != "" {
		w.Header().Set("Profile-Title", title)
		w.Header().Set("X-Profile-Title", title)
		w.Header().Set("Content-Disposition", "inline; filename=\""+title+"\"")
	}
	if expiresAtUnix > 0 {
		w.Header().Set("X-Subscription-Expire", strconv.FormatInt(expiresAtUnix, 10))
	}
	if expiresAtISO != "" {
		w.Header().Set("X-Subscription-Expires-At", expiresAtISO)
	}
	if notice := subscriptionNotice(expiresAtUnix); notice != "" {
		w.Header().Set("Profile-Notice", notice)
		w.Header().Set("X-Profile-Notice", notice)
		w.Header().Set("X-Subscription-Message", notice)
	}
	content = withFooterNotice(content, daysLeft)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, content+"\n")
}

func (s *Server) subscriptionUserInfoHeader(ctx context.Context, token string) string {
	if strings.TrimSpace(token) == "" || s.subs == nil {
		return ""
	}
	_, resolved, err := s.subs.BuildContentByToken(ctx, token)
	if err != nil {
		return ""
	}
	used := int64(0)
	total := int64(1024) * 1024 * 1024 * 1024 // default 1 TB
	expireUnix := int64(0)
	userScoped := false
	if resolved.ExpiresAt != nil && !resolved.ExpiresAt.IsZero() {
		expireUnix = resolved.ExpiresAt.UTC().Unix()
	}
	if strings.TrimSpace(resolved.UserID) != "" {
		userScoped = true
		xuiDBPath := s.xuiDBPath
		if env := strings.TrimSpace(os.Getenv("VPN_PRODUCT_3XUI_DB_PATH")); env != "" {
			xuiDBPath = env
		}
		xuiInboundPort := s.xuiInboundPort
		if raw := strings.TrimSpace(os.Getenv("VPN_PRODUCT_3XUI_INBOUND_PORT")); raw != "" {
			if n, convErr := strconv.Atoi(raw); convErr == nil && n > 0 {
				xuiInboundPort = n
			}
		}
		if usage, uErr := xui.GetClientUsage(ctx, xuiDBPath, xuiInboundPort, strings.TrimSpace(resolved.UserID)); uErr == nil {
			used = maxInt64(usage.UpBytes+usage.DownBytes, 0)
			if usage.Total > 0 {
				total = usage.Total
			}
			if expireUnix == 0 && usage.ExpiryMS > 0 {
				expireUnix = usage.ExpiryMS / 1000
			}
		}
	}
	// Never mix in shared profile counters for user-scoped subscriptions.
	// If x-ui usage cannot be read, keep safe defaults instead of showing global inbound usage.
	if !userScoped && used == 0 {
		// fallback for legacy profiles without user-scoped x-ui stats
	for _, pid := range resolved.ProfileIDs {
		p, perr := s.profiles.Get(ctx, pid)
		if perr != nil {
			continue
		}
		if p.TrafficUsedBytes > 0 {
			used += p.TrafficUsedBytes
		} else {
			used += p.TrafficUsedUp + p.TrafficUsedDown
		}
		if p.TrafficLimitGB > 0 {
			total = p.TrafficLimitGB * 1024 * 1024 * 1024
		} else if p.TrafficLimitMB > 0 {
			total = p.TrafficLimitMB * 1024 * 1024
		}
		break
	}
	}
	parts := []string{
		"upload=0",
		"download=" + strconv.FormatInt(maxInt64(used, 0), 10),
		"total=" + strconv.FormatInt(maxInt64(total, 0), 10),
	}
	if expireUnix > 0 {
		parts = append(parts, "expire="+strconv.FormatInt(expireUnix, 10))
	}
	return strings.Join(parts, "; ")
}

func (s *Server) subscriptionMeta(ctx context.Context, token string) (title string, expiresAtUnix int64, expiresAtISO string, daysLeft int64) {
	if strings.TrimSpace(token) == "" || s.subs == nil {
		return "", 0, "", 0
	}
	_, resolved, err := s.subs.BuildContentByToken(ctx, token)
	if err != nil {
		return "", 0, "", 0
	}
	title = "VPN"
	for _, pid := range resolved.ProfileIDs {
		p, perr := s.profiles.Get(ctx, pid)
		if perr != nil {
			continue
		}
		if strings.TrimSpace(p.Name) != "" {
			title = strings.TrimSpace(p.Name)
		}
		break
	}
	if resolved.ExpiresAt != nil && !resolved.ExpiresAt.IsZero() {
		expiresAtUnix = resolved.ExpiresAt.UTC().Unix()
		expiresAtISO = resolved.ExpiresAt.UTC().Format(time.RFC3339)
		daysLeft = daysLeftFromUnix(expiresAtUnix)
	}
	return title, expiresAtUnix, expiresAtISO, daysLeft
}

func maxInt64(v int64, min int64) int64 {
	if v < min {
		return min
	}
	return v
}

func subscriptionNotice(expiresAtUnix int64) string {
	if expiresAtUnix <= 0 {
		return ""
	}
	daysLeft := daysLeftFromUnix(expiresAtUnix)
	return "✅ " + strconv.FormatInt(daysLeft, 10) + " дней осталось подписки ✅"
}

func daysLeftFromUnix(expiresAtUnix int64) int64 {
	now := time.Now().UTC().Unix()
	secondsLeft := expiresAtUnix - now
	if secondsLeft <= 0 {
		return 0
	}
	return (secondsLeft + 86399) / 86400
}

func withFooterNotice(content string, daysLeft int64) string {
	if strings.TrimSpace(content) == "" {
		return content
	}
	if daysLeft <= 0 {
		return content
	}
	footer := strings.Join([]string{
		"",
		"# ⚠ " + strconv.FormatInt(daysLeft, 10) + " - ОСТАЛОСЬ ДНЕЙ ПОДПИСКИ ⚠",
	}, "\n")
	return content + footer
}

func (s *Server) handleIssueLink(w http.ResponseWriter, r *http.Request) {
	if s.subs == nil {
		writeError(w, http.StatusNotImplemented, perrors.New("VPN_SUBS_001", "subscriptions are not configured"))
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	var req v1.IssueLinkRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_009", "invalid json body"))
		return
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_SUBS_010", "userId is required"))
		return
	}
	profileIDs := req.ProfileIDs
	if len(profileIDs) == 0 {
		profileIDs = []string{"xui-test-vpn"}
	}
	item, err := s.subs.IssueLink30Days(r.Context(), userID, profileIDs, strings.TrimSpace(req.Name), strings.TrimSpace(req.Source))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp := v1.IssueLinkResponse{
		Subscription: item,
		URL:          safeSubscriptionURLFromRequest(r, item.Token),
		Days:         30,
	}
	profileID, applyErr := s.applySubscriptionTo3XUI(r.Context(), userID, item.ID, "")
	if applyErr == nil {
		resp.AppliedTo3XUI = true
		resp.ProfileID = profileID
	} else {
		resp.AppliedTo3XUI = false
		resp.ApplyError = applyErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleIssueHistory(w http.ResponseWriter, r *http.Request) {
	if s.subs == nil {
		writeError(w, http.StatusNotImplemented, perrors.New("VPN_SUBS_001", "subscriptions are not configured"))
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_SUBS_011", "userId is required"))
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	items, err := s.subs.ListIssues(r.Context(), userID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, v1.IssueHistoryResponse{Items: items})
}

func (s *Server) handleIssueApplyTo3XUI(w http.ResponseWriter, r *http.Request) {
	if s.subs == nil {
		writeError(w, http.StatusNotImplemented, perrors.New("VPN_SUBS_001", "subscriptions are not configured"))
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	var req v1.ApplyTo3XUIRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_010", "invalid json body"))
		return
	}
	userID := strings.TrimSpace(req.UserID)
	subscriptionID := strings.TrimSpace(req.SubscriptionID)
	if userID == "" || subscriptionID == "" {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_SUBS_012", "userId and subscriptionId are required"))
		return
	}
	userProfileID, err := s.applySubscriptionTo3XUI(r.Context(), userID, subscriptionID, req.BaseProfileID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, v1.ApplyTo3XUIResponse{
		OK:             true,
		SubscriptionID: subscriptionID,
		ProfileID:      userProfileID,
	})
}

func (s *Server) applySubscriptionTo3XUI(ctx context.Context, userID string, subscriptionID string, baseProfileID string) (string, error) {
	sub, err := s.subs.Get(ctx, strings.TrimSpace(subscriptionID))
	if err != nil {
		return "", perrors.New("VPN_SUBS_404", "subscription not found")
	}
	if strings.TrimSpace(sub.UserID) != strings.TrimSpace(userID) {
		return "", perrors.New("VPN_SUBS_013", "subscription does not belong to user")
	}
	baseID := strings.TrimSpace(baseProfileID)
	if baseID == "" {
		baseID = "xui-test-vpn"
	}
	baseProfile, err := s.profiles.Get(ctx, baseID)
	if err != nil {
		return "", perrors.New("VPN_SUBS_014", "base profile not found")
	}
	if len(baseProfile.Endpoints) == 0 {
		return "", perrors.New("VPN_SUBS_015", "base profile has no endpoints")
	}
	userProfileID := "user-" + sanitizeID(userID)
	userUUID := ""
	if existing, getErr := s.profiles.Get(ctx, userProfileID); getErr == nil {
		if len(existing.Endpoints) > 0 {
			userUUID = strings.TrimSpace(existing.Endpoints[0].UUID)
		}
	}
	if userUUID == "" {
		userUUID = uuid.NewString()
	}
	userProfile := baseProfile
	userProfile.ID = userProfileID
	userProfile.Name = "VPN"
	userProfile.Description = "Personal profile for " + userID
	userProfile.TrafficLimitGB = 1024
	if sub.ExpiresAt != nil {
		userProfile.SubscriptionExpiresAt = sub.ExpiresAt
	}
	userProfile.Endpoints[0].UUID = userUUID
	userProfile.Endpoints[0].Name = "primary"
	userProfile.PreferredID = "primary"
	userProfile.Fallback.EndpointIDs = []string{"primary"}
	if _, err := s.profiles.Save(ctx, userProfile); err != nil {
		return "", err
	}
	if err := s.subs.AssignProfiles(ctx, strings.TrimSpace(subscriptionID), []string{userProfileID}); err != nil {
		return "", err
	}
	xuiDBPath := s.xuiDBPath
	if env := strings.TrimSpace(os.Getenv("VPN_PRODUCT_3XUI_DB_PATH")); env != "" {
		xuiDBPath = env
	}
	xuiInboundPort := s.xuiInboundPort
	if raw := strings.TrimSpace(os.Getenv("VPN_PRODUCT_3XUI_INBOUND_PORT")); raw != "" {
		if n, convErr := strconv.Atoi(raw); convErr == nil && n > 0 {
			xuiInboundPort = n
		}
	}
	if err := xui.UpsertClient(ctx, xui.ClientSpec{
		DBPath:      xuiDBPath,
		InboundPort: xuiInboundPort,
		Email:       strings.TrimSpace(userID),
		UUID:        userUUID,
		Flow:        userProfile.Endpoints[0].Flow,
		TotalBytes:  1024 * 1024 * 1024 * 1024,
		ExpiresAt:   sub.ExpiresAt,
	}); err != nil {
		return "", perrors.Wrap("VPN_3XUI_001", "apply to 3x-ui failed: "+err.Error(), err)
	}
	_ = s.profiles.UpsertPanelUser(ctx, domain.PanelUser{
		ID:         "3x-" + sanitizeID(userID),
		Panel:      "3x-ui",
		ExternalID: strings.TrimSpace(userID),
		Username:   strings.TrimSpace(userID),
		ProfileID:  userProfileID,
		Status:     "active",
		UpdatedAt:  time.Now().UTC(),
	})
	return userProfileID, nil
}

func (s *Server) handleSubscriptionLifecycle(w http.ResponseWriter, r *http.Request) {
	if s.subs == nil {
		writeError(w, http.StatusNotImplemented, perrors.New("VPN_SUBS_001", "subscriptions are not configured"))
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	var req v1.LifecycleRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_011", "invalid json body"))
		return
	}
	userID := strings.TrimSpace(req.UserID)
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if userID == "" || action == "" {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_SUBS_016", "userId and action are required"))
		return
	}
	xuiDBPath := s.xuiDBPath
	if env := strings.TrimSpace(os.Getenv("VPN_PRODUCT_3XUI_DB_PATH")); env != "" {
		xuiDBPath = env
	}
	xuiInboundPort := s.xuiInboundPort
	if raw := strings.TrimSpace(os.Getenv("VPN_PRODUCT_3XUI_INBOUND_PORT")); raw != "" {
		if n, convErr := strconv.Atoi(raw); convErr == nil && n > 0 {
			xuiInboundPort = n
		}
	}
	switch action {
	case "renew":
		item, err := s.subs.ExtendActiveByUser(r.Context(), userID, req.Days)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		expires := item.ExpiresAt
		if err := xui.UpdateClientLifecycle(r.Context(), xui.ClientLifecycleSpec{
			DBPath:      xuiDBPath,
			InboundPort: xuiInboundPort,
			Email:       userID,
			Enable:      true,
			TotalBytes:  1024 * 1024 * 1024 * 1024,
			ExpiresAt:   expires,
		}); err != nil {
			writeError(w, http.StatusBadRequest, perrors.Wrap("VPN_3XUI_002", "renew in 3x-ui failed: "+err.Error(), err))
			return
		}
		if len(item.ProfileIDs) > 0 {
			p, err := s.profiles.Get(r.Context(), item.ProfileIDs[0])
			if err == nil {
				p.TrafficLimitGB = 1024
				p.Blocked = false
				p.SubscriptionExpiresAt = expires
				_, _ = s.profiles.Save(r.Context(), p)
			}
		}
		resp := v1.LifecycleResponse{OK: true, Action: "renew", SubscriptionID: item.ID}
		if item.ExpiresAt != nil {
			resp.ExpiresAt = item.ExpiresAt.UTC().Format(time.RFC3339)
		}
		writeJSON(w, http.StatusOK, resp)
		return
	case "block":
		item, err := s.subs.BlockActiveByUser(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := xui.UpdateClientLifecycle(r.Context(), xui.ClientLifecycleSpec{
			DBPath:      xuiDBPath,
			InboundPort: xuiInboundPort,
			Email:       userID,
			Enable:      false,
			TotalBytes:  1024 * 1024 * 1024 * 1024,
			ExpiresAt:   item.ExpiresAt,
		}); err != nil {
			writeError(w, http.StatusBadRequest, perrors.Wrap("VPN_3XUI_003", "block in 3x-ui failed: "+err.Error(), err))
			return
		}
		if len(item.ProfileIDs) > 0 {
			p, err := s.profiles.Get(r.Context(), item.ProfileIDs[0])
			if err == nil {
				p.Blocked = true
				_, _ = s.profiles.Save(r.Context(), p)
			}
		}
		writeJSON(w, http.StatusOK, v1.LifecycleResponse{OK: true, Action: "block", SubscriptionID: item.ID})
		return
	default:
		writeError(w, http.StatusBadRequest, perrors.New("VPN_SUBS_017", "unsupported action"))
		return
	}
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	items, err := s.profiles.List(r.Context())
	if err != nil {
		s.log.WithRequestID(requestIDFromContext(r.Context())).Errorf("profiles list failed err=%v code=%s", err, perrors.CodeOf(err))
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, v1.ProfilesResponse{Profiles: items})
}

func (s *Server) handleProfilesUpsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_001", "cannot read body"))
		return
	}
	var p domain.Profile
	if err := json.Unmarshal(raw, &p); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_PROFILE_001", "invalid profile json"))
		return
	}
	saved, err := s.profiles.Save(r.Context(), p)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, v1.ProfileResponse{Profile: saved})
}

func (s *Server) handleProfilesDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	var payload struct {
		ProfileID string `json:"profileId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_002", "invalid json body"))
		return
	}
	if payload.ProfileID == "" {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_PROFILE_002", "profileId is required"))
		return
	}
	if err := s.profiles.Delete(r.Context(), payload.ProfileID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	var req v1.ConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_003", "invalid json body"))
		return
	}
	if req.ProfileID == "" {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_PROFILE_003", "profileId is required"))
		return
	}
	if err := s.connection.Connect(r.Context(), req.ProfileID); err != nil {
		s.log.WithRequestID(requestIDFromContext(r.Context())).Errorf("connect failed profile=%s err=%v code=%s", req.ProfileID, err, perrors.CodeOf(err))
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, v1.StatusResponse{Status: s.connection.Status(r.Context())})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	if err := s.connection.Disconnect(r.Context()); err != nil {
		s.log.WithRequestID(requestIDFromContext(r.Context())).Errorf("disconnect failed err=%v code=%s", err, perrors.CodeOf(err))
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, v1.StatusResponse{Status: s.connection.Status(r.Context())})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	writeJSON(w, http.StatusOK, v1.StatusResponse{Status: s.connection.Status(r.Context())})
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	snapshot, err := s.diagnostics.Snapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	info := account.SubscriptionInfo{
		Status: account.PlanStatusUnknown,
	}
	profileID := strings.TrimSpace(r.URL.Query().Get("profileId"))
	if profileID != "" {
		items, err := s.profiles.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		found := false
		for _, p := range items {
			if p.ID != profileID {
				continue
			}
			found = true
			if p.Blocked {
				info.Status = account.PlanStatusExpired
			} else {
				info.Status = account.PlanStatusActive
			}
			break
		}
		if !found {
			writeError(w, http.StatusNotFound, perrors.New("VPN_ACCOUNT_404", "profile not found for account status"))
			return
		}
	}
	writeJSON(w, http.StatusOK, v1.AccountResponse{
		Account: info,
	})
}

func (s *Server) handleQuotaSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	var payload struct {
		ProfileID string `json:"profileId"`
		LimitMB   int64  `json:"limitMb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_004", "invalid json body"))
		return
	}
	if payload.ProfileID == "" || payload.LimitMB < 0 {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_QUOTA_001", "profileId and non-negative limitMb are required"))
		return
	}
	if err := s.profiles.SetTrafficLimit(r.Context(), payload.ProfileID, payload.LimitMB); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleQuotaAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	var payload struct {
		ProfileID     string `json:"profileId"`
		UploadBytes   int64  `json:"uploadBytes"`
		DownloadBytes int64  `json:"downloadBytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_005", "invalid json body"))
		return
	}
	if payload.ProfileID == "" || payload.UploadBytes < 0 || payload.DownloadBytes < 0 {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_QUOTA_002", "invalid quota usage payload"))
		return
	}
	if err := s.profiles.AddTrafficUsage(r.Context(), payload.ProfileID, payload.UploadBytes, payload.DownloadBytes); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	telemetry.Default().TrafficBytes.WithLabelValues("tx").Add(float64(payload.UploadBytes))
	telemetry.Default().TrafficBytes.WithLabelValues("rx").Add(float64(payload.DownloadBytes))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleQuotaBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	var payload struct {
		ProfileID string `json:"profileId"`
		Blocked   bool   `json:"blocked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_006", "invalid json body"))
		return
	}
	if payload.ProfileID == "" {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_PROFILE_004", "profileId is required"))
		return
	}
	if err := s.profiles.SetBlocked(r.Context(), payload.ProfileID, payload.Blocked); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleProfileStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	items, err := s.profiles.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	type profileStat struct {
		ProfileID      string `json:"profileId"`
		Name           string `json:"name"`
		UploadBytes    int64  `json:"uploadBytes"`
		DownloadBytes  int64  `json:"downloadBytes"`
		TotalBytes     int64  `json:"totalBytes"`
		TrafficLimitMB int64  `json:"trafficLimitMb"`
		Blocked        bool   `json:"blocked"`
	}
	out := make([]profileStat, 0, len(items))
	for _, p := range items {
		out = append(out, profileStat{
			ProfileID:      p.ID,
			Name:           p.Name,
			UploadBytes:    p.TrafficUsedUp,
			DownloadBytes:  p.TrafficUsedDown,
			TotalBytes:     p.TrafficUsedUp + p.TrafficUsedDown,
			TrafficLimitMB: p.TrafficLimitMB,
			Blocked:        p.Blocked,
		})
	}
	s.refreshTrafficMetrics(items)
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handlePanelUserUpsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	var payload domain.PanelUser
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_007", "invalid json body"))
		return
	}
	if payload.ID == "" || payload.ProfileID == "" {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_PANEL_001", "id and profileId are required"))
		return
	}
	if payload.Panel == "" {
		payload.Panel = "3x-ui"
	}
	if err := s.profiles.UpsertPanelUser(r.Context(), payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePanelUsersList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	panel := strings.TrimSpace(r.URL.Query().Get("panel"))
	if panel == "" {
		panel = "3x-ui"
	}
	items, err := s.profiles.ListPanelUsers(r.Context(), panel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	report := s.diagnostics.SelfCheck(r.Context())
	if report.Healthy() {
		writeJSON(w, http.StatusOK, report)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, report)
}

func (s *Server) handleDeliveryLinks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	profileID := strings.TrimSpace(r.URL.Query().Get("profileId"))
	if profileID == "" {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_DELIVERY_001", "profileId is required"))
		return
	}
	items, err := s.profiles.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, item := range items {
		if item.ID != profileID {
			continue
		}
		links, err := s.delivery.GenerateHappImportLinks(item)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"profileId": profileID, "links": links, "supportedProtocols": []string{string(domain.ProtocolVLESS), string(domain.ProtocolHysteria)}})
		return
	}
	writeError(w, http.StatusNotFound, perrors.New("VPN_DELIVERY_404", "profile not found"))
}

func (s *Server) withAuthScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.apiToken == "" {
			writeError(w, http.StatusInternalServerError, perrors.New("VPN_CONFIG_AUTH_001", "api token is not configured"))
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		expected := s.apiToken
		if scope == "admin" && strings.TrimSpace(s.adminToken) != "" {
			expected = strings.TrimSpace(s.adminToken)
		}
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			rid := requestIDFromContext(r.Context())
			s.log.WithRequestID(rid).Warnf("authorization failed remote=%s path=%s", requestRemoteAddr(r), r.URL.Path)
			writeError(w, http.StatusUnauthorized, perrors.ErrUnauthorized)
			return
		}
		if scope == "admin" && len(s.adminAllowlist) > 0 {
			clientIP := requestClientIP(r)
			if !ipAllowed(clientIP, s.adminAllowlist) {
				writeError(w, http.StatusForbidden, perrors.New("VPN_ADMIN_IP_001", "admin ip is not allowed"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func parseAllowlist(raw string) []netip.Prefix {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			if p, err := netip.ParsePrefix(item); err == nil {
				out = append(out, p)
			}
			continue
		}
		ip, err := netip.ParseAddr(item)
		if err != nil {
			continue
		}
		if ip.Is4() {
			out = append(out, netip.PrefixFrom(ip, 32))
		} else {
			out = append(out, netip.PrefixFrom(ip, 128))
		}
	}
	return out
}

func requestClientIP(r *http.Request) string {
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		first := strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
		if first != "" {
			return first
		}
	}
	return requestRemoteAddr(r)
}

func ipAllowed(ipRaw string, allowlist []netip.Prefix) bool {
	ip, err := netip.ParseAddr(strings.TrimSpace(ipRaw))
	if err != nil {
		return false
	}
	for _, prefix := range allowlist {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func (s *Server) withObservability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := s.nextRequestID()
		ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
		started := time.Now()
		rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(ctx))

		status := s.connection.Status(ctx).State
		if status == domain.StateConnected || status == domain.StateConnecting || status == domain.StateReconnecting {
			telemetry.Default().XrayStatus.Set(1)
		} else {
			telemetry.Default().XrayStatus.Set(0)
		}
		if status == domain.StateConnected {
			telemetry.Default().ActiveSessions.Set(1)
		} else {
			telemetry.Default().ActiveSessions.Set(0)
		}
		safePath := sanitizePathForObservability(r.URL.Path)
		telemetry.ObserveAPILatency(r.Method, safePath, rec.statusCode, started)
		s.log.WithRequestID(requestID).Infof("request method=%s path=%s status=%d remote=%s latency_ms=%d", r.Method, safePath, rec.statusCode, requestRemoteAddr(r), time.Since(started).Milliseconds())
	})
}

func (s *Server) withRateLimit(next http.Handler) http.Handler {
	const (
		window = time.Minute
		limit  = 120
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := requestRemoteAddr(r)
		now := time.Now().UTC()
		s.rateMu.Lock()
		entry := s.rateByIP[ip]
		if entry.WindowStart.IsZero() || now.Sub(entry.WindowStart) >= window {
			entry = rateEntry{WindowStart: now, Count: 0}
		}
		entry.Count++
		s.rateByIP[ip] = entry
		s.rateMu.Unlock()
		if entry.Count > limit {
			writeError(w, http.StatusTooManyRequests, perrors.New("VPN_API_RATE_001", "rate limit exceeded"))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/public/subscriptions/") || strings.HasPrefix(r.URL.Path, "/s/") {
			const publicLimit = 30
			token := publicTokenFromPath(r.URL.Path)
			key := ip + "|" + token
			s.rateMu.Lock()
			pubEntry := s.publicRateByKey[key]
			if pubEntry.WindowStart.IsZero() || now.Sub(pubEntry.WindowStart) >= window {
				pubEntry = rateEntry{WindowStart: now, Count: 0}
			}
			pubEntry.Count++
			s.publicRateByKey[key] = pubEntry
			s.rateMu.Unlock()
			if pubEntry.Count > publicLimit {
				writeError(w, http.StatusTooManyRequests, perrors.New("VPN_API_RATE_002", "rate limit exceeded"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type requestIDKey struct{}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (s *Server) nextRequestID() string {
	n := atomic.AddUint64(&s.counter, 1)
	return "req-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10) + "-" + strconv.FormatUint(n, 10)
}

func requestIDFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(requestIDKey{}).(string); ok {
		return value
	}
	return "system"
}

func requestRemoteAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) refreshTrafficMetrics(items []domain.Profile) {
	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()
	for _, p := range items {
		prev := s.lastTraffic[p.ID]
		if p.TrafficUsedUp > prev.up {
			telemetry.Default().TrafficBytes.WithLabelValues("tx").Add(float64(p.TrafficUsedUp - prev.up))
		}
		if p.TrafficUsedDown > prev.down {
			telemetry.Default().TrafficBytes.WithLabelValues("rx").Add(float64(p.TrafficUsedDown - prev.down))
		}
		s.lastTraffic[p.ID] = struct {
			up   int64
			down int64
		}{up: p.TrafficUsedUp, down: p.TrafficUsedDown}
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, err error) {
	msg := "request failed"
	code := perrors.CodeOf(err)
	if status < 500 {
		msg = perrors.MessageOf(err)
	}
	writeJSON(w, status, v1.ErrorResponse{
		Error: msg,
		Code:  code,
	})
}

func subscriptionURLFromRequest(r *http.Request, token string) string {
	scheme := "http"
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.ToLower(strings.Split(forwardedProto, ",")[0])
	}
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/public/subscriptions/" + token
}

func subscriptionURLFromBase(baseURL string, token string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" || strings.TrimSpace(token) == "" {
		return ""
	}
	return trimmed + "/public/subscriptions/" + token
}

func isLoopbackHost(hostport string) bool {
	host := strings.TrimSpace(hostport)
	if host == "" {
		return false
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func safeSubscriptionURLFromRequest(r *http.Request, token string) string {
	if strings.TrimSpace(token) == "" {
		return ""
	}
	if publicBaseURL := strings.TrimSpace(os.Getenv("VPN_PRODUCT_PUBLIC_BASE_URL")); publicBaseURL != "" {
		return subscriptionURLFromBase(publicBaseURL, token)
	}
	if isLoopbackHost(r.Host) {
		return ""
	}
	return subscriptionURLFromRequest(r, token)
}

func sanitizePathForObservability(path string) string {
	if strings.HasPrefix(path, "/public/subscriptions/") {
		return "/public/subscriptions/:token"
	}
	if strings.HasPrefix(path, "/s/") {
		return "/s/:token"
	}
	return path
}

func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	snapshot, err := s.diagnostics.Snapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, perrors.New("VPN_READY_001", "not ready"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ready": true,
		"runtimeState": snapshot.Runtime.State,
		"profileCount": snapshot.ProfileCount,
		"checkedAt": snapshot.GeneratedAt,
	})
}

func (s *Server) withDeprecated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Sunset", "2027-01-01T00:00:00Z")
		w.Header().Set("Link", "</admin>; rel=\"successor-version\"")
		next.ServeHTTP(w, r)
	})
}

func publicTokenFromPath(path string) string {
	if strings.HasPrefix(path, "/public/subscriptions/") {
		return strings.TrimSpace(strings.TrimPrefix(path, "/public/subscriptions/"))
	}
	if strings.HasPrefix(path, "/s/") {
		return strings.TrimSpace(strings.TrimPrefix(path, "/s/"))
	}
	return ""
}

func trimKnownPrefix(path string, prefixes ...string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return strings.TrimPrefix(path, prefix)
		}
	}
	return path
}

func sanitizeID(v string) string {
	trimmed := strings.TrimSpace(strings.ToLower(v))
	if trimmed == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	if len(out) > 48 {
		return out[:48]
	}
	return out
}

func selectProfileEndpoint(profile domain.Profile, endpointName string) (domain.Endpoint, bool) {
	for _, ep := range profile.Endpoints {
		if ep.Name == endpointName {
			return ep, true
		}
	}
	return domain.Endpoint{}, false
}

