package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/runtimecfg"
)

const localCloudProfileName = "official"
const localCloudDefaultAPIBase = "https://driver.sunningfun.cn"
const localCloudDefaultHTTPTimeout = 60 * time.Second

var localCloudImportHTTPTimeout = 15 * time.Second
var localCloudImportUsagePollTimeout = 15 * time.Second
var localCloudImportUsagePollInterval = time.Second

type localCloudStatusResponse struct {
	Connected bool                   `json:"connected"`
	APIBase   string                 `json:"api_base,omitempty"`
	Profile   string                 `json:"profile,omitempty"`
	AuthMode  string                 `json:"auth_mode,omitempty"`
	Account   map[string]interface{} `json:"account,omitempty"`
	Quota     *localCloudQuota       `json:"quota,omitempty"`
	UpdatedAt string                 `json:"updated_at,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

type localCloudQuota struct {
	StorageQuotaBytes          *int64 `json:"storage_quota_bytes,omitempty"`
	EffectiveStorageQuotaBytes int64  `json:"effective_storage_quota_bytes"`
	StorageUsedBytes           int64  `json:"storage_used_bytes"`
}

type localCloudPushResponse struct {
	Status       localCloudStatusResponse   `json:"status"`
	BundleStats  models.BundleStats         `json:"bundle_stats"`
	ImportResult *models.BundleImportResult `json:"import_result,omitempty"`
	Confirmed    bool                       `json:"confirmed"`
	Warning      string                     `json:"warning,omitempty"`
}

type localCloudClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func (s *Server) handleLocalCloudStatus(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "local cloud account is only available in desktop local mode")
		return
	}
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	status, err := s.localCloudStatus(r.Context(), true)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, status)
}

func (s *Server) handleLocalCloudLogin(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "local cloud account is only available in desktop local mode")
		return
	}
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	apiBase := s.localCloudConfiguredAPIBase()
	authResp, err := newLocalCloudClient(apiBase, "").login(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadGateway, ErrCodeBadRequest, err.Error())
		return
	}
	if err := saveLocalCloudProfile(apiBase, authResp, "desktop-password"); err != nil {
		respondInternalError(w, err)
		return
	}
	status, err := s.localCloudStatus(r.Context(), true)
	if err != nil {
		respondError(w, http.StatusBadGateway, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, status)
}

func (s *Server) handleLocalCloudRegister(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "local cloud account is only available in desktop local mode")
		return
	}
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	apiBase := s.localCloudConfiguredAPIBase()
	authResp, err := newLocalCloudClient(apiBase, "").register(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadGateway, ErrCodeBadRequest, err.Error())
		return
	}
	if err := saveLocalCloudProfile(apiBase, authResp, "desktop-password"); err != nil {
		respondInternalError(w, err)
		return
	}
	status, err := s.localCloudStatus(r.Context(), true)
	if err != nil {
		respondError(w, http.StatusBadGateway, ErrCodeBadRequest, err.Error())
		return
	}
	respondCreated(w, status)
}

func (s *Server) handleLocalCloudDisconnect(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "local cloud account is only available in desktop local mode")
		return
	}
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	configPath, cfg, err := runtimecfg.LoadConfig("")
	if err != nil {
		respondInternalError(w, err)
		return
	}
	delete(cfg.Profiles, localCloudProfileName)
	if runtimecfg.TargetProfileName(cfg.CurrentTarget) == localCloudProfileName {
		cfg.CurrentTarget = runtimecfg.TargetLocal
		cfg.CurrentProfile = ""
	}
	if err := runtimecfg.SaveConfig(configPath, cfg); err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, localCloudStatusResponse{Connected: false, Profile: localCloudProfileName})
}

func (s *Server) handleLocalCloudPush(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "local cloud account is only available in desktop local mode")
		return
	}
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	if s.SyncService == nil {
		respondNotConfigured(w, "sync service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	status, profile, err := s.localCloudStatusWithProfile(r.Context(), true)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	if !status.Connected {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "official cloud account is not connected")
		return
	}
	bundle, err := s.SyncService.ExportBundleJSON(r.Context(), userID, models.BundleFilters{})
	if err != nil {
		respondInternalError(w, err)
		return
	}
	startUsed := storageUsedFromStatus(status)
	imported, err := newLocalCloudClient(status.APIBase, profile.Token).importBundle(r.Context(), *bundle)
	if err != nil {
		if isLocalCloudTimeout(err) {
			nextStatus, usageChanged, statusErr := s.waitForLocalCloudUsageChange(r.Context(), startUsed)
			if statusErr == nil {
				nextStatus.Error = "官方云端已收到资料并产生用量变化，但导入确认响应超时。请稍后刷新确认最终导入明细。"
				if !usageChanged {
					nextStatus.Error = "上传请求已发送到官方云端，但导入确认响应超时。本机已刷新当前云端额度；如果资料刚刚上传过，用量可能不会再次增加。"
				}
				respondOK(w, localCloudPushResponse{
					Status:      nextStatus,
					BundleStats: bundle.Stats,
					Confirmed:   false,
					Warning:     nextStatus.Error,
				})
				return
			}
		}
		respondError(w, http.StatusBadGateway, ErrCodeBadRequest, err.Error())
		return
	}
	status, err = s.localCloudStatus(r.Context(), true)
	if err != nil {
		respondError(w, http.StatusBadGateway, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, localCloudPushResponse{
		Status:       status,
		BundleStats:  bundle.Stats,
		ImportResult: imported,
		Confirmed:    true,
	})
}

func storageUsedFromStatus(status localCloudStatusResponse) int64 {
	if status.Quota == nil {
		return 0
	}
	return status.Quota.StorageUsedBytes
}

func isLocalCloudTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "client.timeout")
}

func (s *Server) waitForLocalCloudUsageChange(ctx context.Context, startUsed int64) (localCloudStatusResponse, bool, error) {
	deadline := time.Now().Add(localCloudImportUsagePollTimeout)
	for {
		status, err := s.localCloudStatus(ctx, true)
		if err != nil {
			return status, false, err
		}
		if storageUsedFromStatus(status) > startUsed {
			return status, true, nil
		}
		if !time.Now().Before(deadline) {
			return status, false, nil
		}
		timer := time.NewTimer(localCloudImportUsagePollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return status, false, ctx.Err()
		case <-timer.C:
		}
	}
}

func (s *Server) localCloudStatus(ctx context.Context, fetchRemote bool) (localCloudStatusResponse, error) {
	status, _, err := s.localCloudStatusWithProfile(ctx, fetchRemote)
	return status, err
}

func (s *Server) localCloudStatusWithProfile(ctx context.Context, fetchRemote bool) (localCloudStatusResponse, runtimecfg.SyncProfile, error) {
	configPath, cfg, err := runtimecfg.LoadConfig("")
	if err != nil {
		return localCloudStatusResponse{}, runtimecfg.SyncProfile{}, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]runtimecfg.SyncProfile{}
	}
	profile, ok := cfg.Profiles[localCloudProfileName]
	apiBase := s.localCloudAPIBase(profile.APIBase)
	profile.APIBase = apiBase
	status := localCloudStatusResponse{
		Connected: ok && strings.TrimSpace(profile.Token) != "",
		APIBase:   apiBase,
		Profile:   localCloudProfileName,
		AuthMode:  strings.TrimSpace(profile.AuthMode),
		UpdatedAt: strings.TrimSpace(profile.UpdatedAt),
	}
	if !status.Connected || !fetchRemote {
		return status, profile, nil
	}
	profile, err = refreshLocalCloudPasswordSession(ctx, configPath, cfg, localCloudProfileName, profile)
	if err != nil {
		status.Connected = false
		status.Error = err.Error()
		return status, profile, nil
	}
	status.AuthMode = strings.TrimSpace(profile.AuthMode)
	status.UpdatedAt = strings.TrimSpace(profile.UpdatedAt)
	account, err := newLocalCloudClient(apiBase, profile.Token).me(ctx)
	if err != nil {
		status.Error = err.Error()
		return status, profile, nil
	}
	status.Account = account
	status.Quota = quotaFromAccount(account)
	return status, profile, nil
}

func (s *Server) localCloudAPIBase(saved string) string {
	if trimmed := strings.TrimRight(strings.TrimSpace(saved), "/"); trimmed != "" && !isLegacyOfficialCloudAPIBase(trimmed) {
		return trimmed
	}
	return localCloudDefaultAPIBase
}

func isLegacyOfficialCloudAPIBase(apiBase string) bool {
	switch strings.ToLower(strings.TrimRight(strings.TrimSpace(apiBase), "/")) {
	case "https://vola.ai", "https://www.vola.ai":
		return true
	default:
		return false
	}
}

func (s *Server) localCloudConfiguredAPIBase() string {
	_, cfg, err := runtimecfg.LoadConfig("")
	if err != nil || cfg == nil {
		return s.localCloudAPIBase("")
	}
	if profile, ok := cfg.Profiles[localCloudProfileName]; ok {
		return s.localCloudAPIBase(profile.APIBase)
	}
	return s.localCloudAPIBase("")
}

func saveLocalCloudProfile(apiBase string, authResp *models.AuthResponse, source string) error {
	if authResp == nil {
		return fmt.Errorf("empty auth response")
	}
	configPath, cfg, err := runtimecfg.LoadConfig("")
	if err != nil {
		return err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]runtimecfg.SyncProfile{}
	}
	expiresAt := time.Now().UTC().Add(time.Duration(authResp.ExpiresIn) * time.Second).Format(time.RFC3339)
	sourceValue := strings.TrimSpace(source)
	if authResp.User.Slug != "" {
		sourceValue = sourceValue + ":" + authResp.User.Slug
	}
	cfg.Profiles[localCloudProfileName] = runtimecfg.SyncProfile{
		APIBase:      strings.TrimRight(strings.TrimSpace(apiBase), "/"),
		Token:        strings.TrimSpace(authResp.AccessToken),
		RefreshToken: strings.TrimSpace(authResp.RefreshToken),
		ExpiresAt:    expiresAt,
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		Source:       sourceValue,
		AuthMode:     "password_session",
	}
	return runtimecfg.SaveConfig(configPath, cfg)
}

func refreshLocalCloudPasswordSession(ctx context.Context, configPath string, cfg *runtimecfg.CLIConfig, profileName string, profile runtimecfg.SyncProfile) (runtimecfg.SyncProfile, error) {
	if strings.TrimSpace(profile.AuthMode) != "password_session" {
		return profile, nil
	}
	if strings.TrimSpace(profile.Token) != "" && !profileTokenExpired(profile.ExpiresAt) {
		return profile, nil
	}
	if strings.TrimSpace(profile.RefreshToken) == "" {
		return profile, fmt.Errorf("official cloud session expired; please sign in again")
	}
	authResp, err := newLocalCloudClient(profile.APIBase, "").refresh(ctx, profile.RefreshToken)
	if err != nil {
		return profile, err
	}
	profile.Token = strings.TrimSpace(authResp.AccessToken)
	profile.RefreshToken = strings.TrimSpace(authResp.RefreshToken)
	profile.ExpiresAt = time.Now().UTC().Add(time.Duration(authResp.ExpiresIn) * time.Second).Format(time.RFC3339)
	profile.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	profile.AuthMode = "password_session"
	cfg.Profiles[profileName] = profile
	if err := runtimecfg.SaveConfig(configPath, cfg); err != nil {
		return profile, err
	}
	return profile, nil
}

func profileTokenExpired(expiresAt string) bool {
	expiresAt = strings.TrimSpace(expiresAt)
	if expiresAt == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return false
	}
	return !parsed.After(time.Now().UTC().Add(30 * time.Second))
}

func quotaFromAccount(account map[string]interface{}) *localCloudQuota {
	if account == nil {
		return nil
	}
	quotaValue, hasQuota := numberFromMap(account, "effective_storage_quota_bytes")
	usedValue, hasUsed := numberFromMap(account, "storage_used_bytes")
	rawQuota, _ := nullableNumberFromMap(account, "storage_quota_bytes")
	if !hasQuota && !hasUsed {
		return nil
	}
	return &localCloudQuota{
		StorageQuotaBytes:          rawQuota,
		EffectiveStorageQuotaBytes: quotaValue,
		StorageUsedBytes:           usedValue,
	}
}

func numberFromMap(values map[string]interface{}, key string) (int64, bool) {
	raw, ok := values[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	default:
		return 0, false
	}
}

func nullableNumberFromMap(values map[string]interface{}, key string) (*int64, bool) {
	value, ok := numberFromMap(values, key)
	if !ok {
		return nil, false
	}
	return &value, true
}

func newLocalCloudClient(baseURL, token string) *localCloudClient {
	return &localCloudClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		http: &http.Client{
			Timeout: localCloudDefaultHTTPTimeout,
		},
	}
}

func (c *localCloudClient) register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error) {
	var out models.AuthResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/auth/register", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *localCloudClient) login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	var out models.AuthResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/auth/login", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *localCloudClient) refresh(ctx context.Context, refreshToken string) (*models.AuthResponse, error) {
	var out models.AuthResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/auth/refresh", models.RefreshRequest{RefreshToken: refreshToken}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *localCloudClient) me(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := c.doJSON(ctx, http.MethodGet, "/api/auth/me", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *localCloudClient) importBundle(ctx context.Context, bundle models.Bundle) (*models.BundleImportResult, error) {
	var out models.BundleImportResult
	if err := c.withTimeout(localCloudImportHTTPTimeout).doJSON(ctx, http.MethodPost, "/agent/import/bundle", bundle, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *localCloudClient) withTimeout(timeout time.Duration) *localCloudClient {
	next := *c
	next.http = &http.Client{Timeout: timeout}
	return &next
}

func (c *localCloudClient) doJSON(ctx context.Context, method string, path string, payload interface{}, out interface{}) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeLocalCloudError(resp.StatusCode, respBody)
	}
	return decodeLocalCloudSuccess(respBody, out)
}

func decodeLocalCloudSuccess(body []byte, out interface{}) error {
	if out == nil || len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Data != nil {
		return json.Unmarshal(envelope.Data, out)
	}
	return json.Unmarshal(body, out)
}

func decodeLocalCloudError(status int, body []byte) error {
	var payload struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if strings.TrimSpace(payload.Message) != "" {
			return fmt.Errorf("official cloud returned %d: %s", status, payload.Message)
		}
		if payload.Error != nil {
			var nested struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(payload.Error, &nested); err == nil && strings.TrimSpace(nested.Message) != "" {
				return fmt.Errorf("official cloud returned %d: %s", status, nested.Message)
			}
			var text string
			if err := json.Unmarshal(payload.Error, &text); err == nil && strings.TrimSpace(text) != "" {
				return fmt.Errorf("official cloud returned %d: %s", status, text)
			}
		}
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		text = http.StatusText(status)
	}
	return fmt.Errorf("official cloud returned %d: %s", status, text)
}
