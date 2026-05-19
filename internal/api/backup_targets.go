package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/agi-bar/neudrive/internal/backups"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) handleBackupTargetsList(w http.ResponseWriter, r *http.Request) {
	if s.BackupService == nil {
		respondNotConfigured(w, "backup targets service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	targets, err := s.BackupService.ListTargets(r.Context(), userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, targets)
}

func (s *Server) handleBackupTargetsSave(w http.ResponseWriter, r *http.Request) {
	if s.BackupService == nil {
		respondNotConfigured(w, "backup targets service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req backups.SaveTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	target, err := s.BackupService.SaveTarget(r.Context(), userID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, target)
}

func (s *Server) handleBackupTargetRun(w http.ResponseWriter, r *http.Request) {
	if s.BackupService == nil {
		respondNotConfigured(w, "backup targets service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid backup target id")
		return
	}
	result, err := s.BackupService.RunTarget(r.Context(), userID, targetID)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, result)
}

func (s *Server) handleBackupRunsList(w http.ResponseWriter, r *http.Request) {
	if s.BackupService == nil {
		respondNotConfigured(w, "backup targets service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var targetID *uuid.UUID
	if rawTargetID := strings.TrimSpace(r.URL.Query().Get("target_id")); rawTargetID != "" {
		parsed, err := uuid.Parse(rawTargetID)
		if err != nil {
			respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid backup target id")
			return
		}
		targetID = &parsed
	}
	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid limit")
			return
		}
		limit = parsed
	}
	runs, err := s.BackupService.ListRuns(r.Context(), userID, targetID, limit)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, runs)
}
