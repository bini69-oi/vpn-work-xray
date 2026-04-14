package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/xtls/xray-core/internal/configgen"
	perrors "github.com/xtls/xray-core/internal/errors"
	vpnrouting "github.com/xtls/xray-core/internal/routing"
)

func (s *Server) handleRoutingRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		doc, err := vpnrouting.LoadRoutingConfigFile(vpnrouting.RoutingRulesPath())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if doc == nil {
			writeJSON(w, http.StatusOK, map[string]any{"routing": nil, "path": vpnrouting.RoutingRulesPath()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"routing": doc, "path": vpnrouting.RoutingRulesPath()})
	case http.MethodPut:
		var doc vpnrouting.RoutingConfigFile
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&doc); err != nil {
			writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_008", "invalid json body"))
			return
		}
		if len(doc.Rules) == 0 {
			writeError(w, http.StatusBadRequest, perrors.New("VPN_ROUTING_001", "rules must not be empty"))
			return
		}
		if err := vpnrouting.SaveRoutingConfigFile(vpnrouting.RoutingRulesPath(), &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": vpnrouting.RoutingRulesPath()})
	default:
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
	}
}

func (s *Server) handleRoutingReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	st := s.connection.Status(r.Context())
	if strings.TrimSpace(st.ActiveProfile) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"reloaded": false, "message": "no active profile"})
		return
	}
	if err := s.connection.Connect(r.Context(), st.ActiveProfile); err != nil {
		if rb, ok := s.connection.(interface{ RollbackLastGood(ctx context.Context) error }); ok {
			_ = rb.RollbackLastGood(r.Context())
		}
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reloaded": true, "profileId": st.ActiveProfile})
}

func (s *Server) handleRoutingGeodataStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	st, err := vpnrouting.GeodataStatus(vpnrouting.GeodataDir())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dir": vpnrouting.GeodataDir(), "files": st})
}

func (s *Server) handleRoutingGeodataUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	ctx := r.Context()
	dir := vpnrouting.GeodataDir()
	if err := configgen.ForceRefreshGeoAssets(ctx, dir); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	if base := strings.TrimSpace(os.Getenv("VPN_PRODUCT_DATA_DIR")); base != "" {
		assetsDir := filepath.Join(base, "assets")
		if err := configgen.ForceRefreshGeoAssets(ctx, assetsDir); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": dir, "assetsMirrorError": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": dir})
}

func (s *Server) handleRoutingWarpStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	st := vpnrouting.DescribeWarpStatus()
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleRoutingWarpSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	script := strings.TrimSpace(os.Getenv("VPN_PRODUCT_SETUP_WARP_SCRIPT"))
	if script == "" {
		script = "/opt/vpn-product/src/deploy/scripts/setup_warp.sh"
	}
	if _, err := os.Stat(script); err != nil {
		writeError(w, http.StatusNotImplemented, perrors.New("VPN_WARP_001", "setup script not found at "+script))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	// #nosec G204 -- script path is operator-controlled (VPN_PRODUCT_SETUP_WARP_SCRIPT).
	cmd := exec.CommandContext(ctx, script)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeError(w, http.StatusBadGateway, perrors.Wrap("VPN_WARP_002", string(out), err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": strings.TrimSpace(string(out))})
}

func (s *Server) handleRoutingWarpDomains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, perrors.New("VPN_API_METHOD_001", "method not allowed"))
		return
	}
	var doc vpnrouting.WarpDomainsFile
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&doc); err != nil {
		writeError(w, http.StatusBadRequest, perrors.New("VPN_API_BODY_008", "invalid json body"))
		return
	}
	path := vpnrouting.WarpDomainsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := vpnrouting.SaveWarpDomainsFile(path, &doc); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": path})
}
