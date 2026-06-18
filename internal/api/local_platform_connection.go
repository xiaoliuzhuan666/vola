package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/agi-bar/vola/internal/platforms"
	"github.com/agi-bar/vola/internal/runtimecfg"
)

var refreshLocalPlatformConnection = platforms.RefreshConnection

type localPlatformConnectionRequest struct {
	Platform string `json:"platform"`
}

type localPlatformConnectionResponse struct {
	Platform   string                     `json:"platform"`
	Name       string                     `json:"name"`
	Refreshed  bool                       `json:"refreshed"`
	Connection runtimecfg.LocalConnection `json:"connection,omitempty"`
}

func (s *Server) handleLocalPlatformConnectionRefresh(w http.ResponseWriter, r *http.Request) {
	_, ok := s.checkLocalSkillSyncAccess(w, r)
	if !ok {
		return
	}
	if !s.ensureLocalPlatformMode(w) {
		return
	}

	var req localPlatformConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		respondValidationError(w, "platform", "platform is required")
		return
	}
	adapter, err := platforms.Resolve(platform)
	if err != nil {
		respondValidationError(w, "platform", "unknown platform")
		return
	}
	if adapter.ID() != "codex" && adapter.ID() != "claude-code" {
		respondValidationError(w, "platform", "only codex and claude-code support local connection refresh")
		return
	}
	platform = adapter.ID()

	configPath, cfg, err := runtimecfg.LoadConfig("")
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if err := runtimecfg.EnsureLocalDefaults(cfg); err != nil {
		respondInternalError(w, err)
		return
	}

	executable, err := os.Executable()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	_, state, err := runtimecfg.LoadState("")
	if err != nil {
		respondInternalError(w, err)
		return
	}
	daemonURL := ""
	if state != nil {
		daemonURL = strings.TrimSpace(state.APIBase)
	}
	if daemonURL == "" {
		daemonURL = strings.TrimSpace(cfg.Local.PublicBaseURL)
	}
	if daemonURL == "" {
		respondInternalError(w, fmt.Errorf("local daemon URL not found"))
		return
	}
	updated, err := refreshLocalPlatformConnection(r.Context(), cfg, platform, executable, daemonURL)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	if err := runtimecfg.SaveConfig(configPath, cfg); err != nil {
		respondInternalError(w, err)
		return
	}

	respondOK(w, localPlatformConnectionResponse{
		Platform:   platform,
		Name:       adapter.DisplayName(),
		Refreshed:  true,
		Connection: updated,
	})
}
