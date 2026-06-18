package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/agi-bar/vola/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) handleAdminUsersList(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAccountAccess(w, r) {
		return
	}
	if s.UserService == nil {
		respondNotConfigured(w, "user service")
		return
	}
	accounts, err := s.UserService.ListAccounts(r.Context(), s.defaultUserStorageQuotaBytes())
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]interface{}{"users": accounts})
}

func (s *Server) handleAdminUsersCreate(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAccountAccess(w, r) {
		return
	}
	if s.AuthHandler == nil || s.AuthHandler.AuthService == nil {
		respondNotConfigured(w, "auth service")
		return
	}
	if s.UserService == nil {
		respondNotConfigured(w, "user service")
		return
	}

	var req models.AdminCreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	if req.StorageQuotaBytes != nil && *req.StorageQuotaBytes < 0 {
		respondValidationError(w, "storage_quota_bytes", "storage_quota_bytes must be >= 0")
		return
	}
	user, err := s.AuthHandler.AuthService.CreateUser(r.Context(), models.RegisterRequest{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Slug:        req.Slug,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}

	var account *models.AdminUserAccount
	if req.StorageQuotaBytes != nil {
		account, err = s.UserService.UpdateStorageQuota(r.Context(), user.ID, req.StorageQuotaBytes, s.defaultUserStorageQuotaBytes())
	} else {
		account, err = s.UserService.GetAccount(r.Context(), user.ID, s.defaultUserStorageQuotaBytes())
	}
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondCreated(w, map[string]interface{}{"user": account})
}

func (s *Server) handleAdminUserQuotaUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAccountAccess(w, r) {
		return
	}
	if s.UserService == nil {
		respondNotConfigured(w, "user service")
		return
	}

	userID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		respondValidationError(w, "id", "id must be a UUID")
		return
	}
	quotaBytes, err := parseQuotaUpdateBody(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	account, err := s.UserService.UpdateStorageQuota(r.Context(), userID, quotaBytes, s.defaultUserStorageQuotaBytes())
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]interface{}{"user": account})
}

func (s *Server) requireAdminAccountAccess(w http.ResponseWriter, r *http.Request) bool {
	return s.requireInstanceAdminAccess(w, r)
}

func (s *Server) requireInstanceAdminAccess(w http.ResponseWriter, r *http.Request) bool {
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return false
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return false
	}
	if s.isInstanceAdminUser(userID) {
		return true
	}
	respondForbidden(w, "instance admin APIs require an instance administrator user")
	return false
}

func (s *Server) isInstanceAdminUser(userID uuid.UUID) bool {
	if userID == uuid.Nil {
		return false
	}
	if s != nil && s.LocalOwnerID != uuid.Nil && userID == s.LocalOwnerID {
		return true
	}
	if s != nil && s.Config != nil {
		for _, id := range s.Config.InstanceAdminUserIDs {
			if id == userID {
				return true
			}
		}
	}
	return false
}

func (s *Server) defaultUserStorageQuotaBytes() int64 {
	if s != nil && s.Config != nil && s.Config.UserStorageQuotaBytes > 0 {
		return s.Config.UserStorageQuotaBytes
	}
	return 100 * 1024 * 1024
}

func parseQuotaUpdateBody(r *http.Request) (*int64, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return nil, err
	}
	value, ok := raw["storage_quota_bytes"]
	if !ok {
		return nil, errMissingQuotaField()
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil, nil
	}
	var quota int64
	if err := json.Unmarshal(value, &quota); err != nil {
		return nil, err
	}
	if quota < 0 {
		return nil, errNegativeQuota()
	}
	return &quota, nil
}

type adminQuotaError string

func (e adminQuotaError) Error() string {
	return string(e)
}

func errMissingQuotaField() error {
	return adminQuotaError("storage_quota_bytes is required; use null to inherit the default")
}

func errNegativeQuota() error {
	return adminQuotaError("storage_quota_bytes must be >= 0")
}
