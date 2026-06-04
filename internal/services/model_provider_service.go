package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/models"
	"github.com/google/uuid"
)

const (
	ModelProvidersVersion = "vola.model-providers/v1"
	ModelProvidersPath    = "/settings/model-providers.json"
)

type ModelProviderService struct {
	fileTree   *FileTreeService
	vault      *VaultService
	httpClient *http.Client
}

type ModelProviderDocument struct {
	Version                   string          `json:"version"`
	UpdatedAt                 string          `json:"updated_at,omitempty"`
	DefaultSummaryProviderID  string          `json:"default_summary_provider_id,omitempty"`
	DefaultProposalProviderID string          `json:"default_proposal_provider_id,omitempty"`
	Providers                 []ModelProvider `json:"providers"`
}

type ModelProvider struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	BaseURL        string                 `json:"base_url,omitempty"`
	APIKeyRef      string                 `json:"api_key_ref,omitempty"`
	Models         ModelProviderModels    `json:"models"`
	Enabled        bool                   `json:"enabled"`
	LastVerifiedAt string                 `json:"last_verified_at,omitempty"`
	LastError      string                 `json:"last_error,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type ModelProviderModels struct {
	Summary  string `json:"summary,omitempty"`
	Proposal string `json:"proposal,omitempty"`
	JSON     string `json:"json,omitempty"`
}

type SaveModelProvidersRequest struct {
	DefaultSummaryProviderID  string                     `json:"default_summary_provider_id,omitempty"`
	DefaultProposalProviderID string                     `json:"default_proposal_provider_id,omitempty"`
	Providers                 []ModelProviderSaveRequest `json:"providers"`
}

type ModelProviderSaveRequest struct {
	ID        string              `json:"id"`
	Type      string              `json:"type"`
	Name      string              `json:"name"`
	BaseURL   string              `json:"base_url,omitempty"`
	APIKey    string              `json:"api_key,omitempty"`
	APIKeyRef string              `json:"api_key_ref,omitempty"`
	Models    ModelProviderModels `json:"models"`
	Enabled   bool                `json:"enabled"`
}

type ModelProviderTestRequest struct {
	ProviderID string                    `json:"provider_id,omitempty"`
	Provider   *ModelProviderSaveRequest `json:"provider,omitempty"`
}

type ModelProviderTestResult struct {
	OK         bool   `json:"ok"`
	ProviderID string `json:"provider_id,omitempty"`
	Type       string `json:"type,omitempty"`
	Message    string `json:"message"`
	TestedAt   string `json:"tested_at"`
}

type GenerateRequest struct {
	ProviderID string
	Model      string
	Prompt     string
}

func NewModelProviderService(fileTree *FileTreeService, vault *VaultService) *ModelProviderService {
	return &ModelProviderService{
		fileTree: fileTree,
		vault:    vault,
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (s *ModelProviderService) Load(ctx context.Context, userID uuid.UUID, trustLevel int) (ModelProviderDocument, error) {
	if s == nil || s.fileTree == nil {
		return ModelProviderDocument{}, fmt.Errorf("model provider service not configured")
	}
	entry, err := s.fileTree.Read(ctx, userID, ModelProvidersPath, trustLevel)
	if err != nil {
		if errors.Is(err, ErrEntryNotFound) {
			return ModelProviderDocument{Version: ModelProvidersVersion, Providers: []ModelProvider{}}, nil
		}
		return ModelProviderDocument{}, fmt.Errorf("model providers.Load: %w", err)
	}
	var doc ModelProviderDocument
	if err := json.Unmarshal([]byte(entry.Content), &doc); err != nil {
		return ModelProviderDocument{}, fmt.Errorf("model providers.Load: decode: %w", err)
	}
	doc.Version = ModelProvidersVersion
	doc.Providers = normalizeModelProviders(doc.Providers)
	return doc, nil
}

func (s *ModelProviderService) Save(ctx context.Context, userID uuid.UUID, req SaveModelProvidersRequest) (ModelProviderDocument, error) {
	if s == nil || s.fileTree == nil {
		return ModelProviderDocument{}, fmt.Errorf("model provider service not configured")
	}
	providers, err := s.normalizeSaveProviders(ctx, userID, req.Providers)
	if err != nil {
		return ModelProviderDocument{}, err
	}
	doc := ModelProviderDocument{
		Version:                   ModelProvidersVersion,
		UpdatedAt:                 time.Now().UTC().Format(time.RFC3339),
		DefaultSummaryProviderID:  strings.TrimSpace(req.DefaultSummaryProviderID),
		DefaultProposalProviderID: strings.TrimSpace(req.DefaultProposalProviderID),
		Providers:                 providers,
	}
	if doc.DefaultSummaryProviderID == "" && len(doc.Providers) > 0 {
		doc.DefaultSummaryProviderID = doc.Providers[0].ID
	}
	if doc.DefaultProposalProviderID == "" && len(doc.Providers) > 0 {
		doc.DefaultProposalProviderID = doc.Providers[0].ID
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return ModelProviderDocument{}, err
	}
	data = append(data, '\n')
	if _, err := s.fileTree.WriteEntry(ctx, userID, ModelProvidersPath, string(data), "application/json", models.FileTreeWriteOptions{
		Kind: "model_provider_config",
		Metadata: map[string]interface{}{
			"source":       "manual",
			"capture_mode": "model-provider-config",
		},
		MinTrustLevel: models.TrustLevelFull,
	}); err != nil {
		return ModelProviderDocument{}, fmt.Errorf("model providers.Save: write config: %w", err)
	}
	return doc, nil
}

func (s *ModelProviderService) Test(ctx context.Context, userID uuid.UUID, trustLevel int, req ModelProviderTestRequest) (ModelProviderTestResult, error) {
	provider, apiKey, err := s.resolveTestProvider(ctx, userID, trustLevel, req)
	if err != nil {
		return ModelProviderTestResult{}, err
	}
	testedAt := time.Now().UTC().Format(time.RFC3339)
	err = s.testProvider(ctx, provider, apiKey)
	if err != nil {
		return ModelProviderTestResult{
			OK:         false,
			ProviderID: provider.ID,
			Type:       provider.Type,
			Message:    err.Error(),
			TestedAt:   testedAt,
		}, nil
	}
	return ModelProviderTestResult{
		OK:         true,
		ProviderID: provider.ID,
		Type:       provider.Type,
		Message:    "connection ok",
		TestedAt:   testedAt,
	}, nil
}

func (s *ModelProviderService) GenerateText(ctx context.Context, userID uuid.UUID, trustLevel int, req GenerateRequest) (string, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return "", fmt.Errorf("prompt is required")
	}
	provider, apiKey, err := s.resolveStoredProvider(ctx, userID, trustLevel, req.ProviderID)
	if err != nil {
		return "", err
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = firstNonEmpty(provider.Models.Summary, provider.Models.JSON, provider.Models.Proposal)
	}
	if provider.Type == "ollama" {
		return s.generateOllamaText(ctx, provider, model, req.Prompt)
	}
	if provider.Type == "anthropic" {
		return s.generateAnthropicText(ctx, provider, apiKey, model, req.Prompt)
	}
	if provider.Type == "gemini" {
		return s.generateGeminiText(ctx, provider, apiKey, model, req.Prompt)
	}
	return s.generateOpenAICompatibleText(ctx, provider, apiKey, model, req.Prompt)
}

func (s *ModelProviderService) GenerateJSON(ctx context.Context, userID uuid.UUID, trustLevel int, req GenerateRequest, out interface{}) error {
	text, err := s.GenerateText(ctx, userID, trustLevel, req)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(text), out); err != nil {
		return fmt.Errorf("decode model JSON: %w", err)
	}
	return nil
}

func (s *ModelProviderService) normalizeSaveProviders(ctx context.Context, userID uuid.UUID, items []ModelProviderSaveRequest) ([]ModelProvider, error) {
	seen := map[string]struct{}{}
	out := make([]ModelProvider, 0, len(items))
	for _, item := range items {
		provider, err := modelProviderFromSaveRequest(item)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[provider.ID]; ok {
			return nil, fmt.Errorf("duplicate provider id %q", provider.ID)
		}
		seen[provider.ID] = struct{}{}
		apiKey := strings.TrimSpace(item.APIKey)
		if apiKey != "" {
			if s.vault == nil {
				return nil, fmt.Errorf("vault service not configured")
			}
			scope := modelProviderVaultScope(provider.ID)
			if err := s.vault.Write(ctx, userID, scope, apiKey, "Model provider API key for "+provider.Name, models.TrustLevelFull); err != nil {
				return nil, err
			}
			provider.APIKeyRef = "vault://" + scope
		}
		if provider.APIKeyRef == "" && modelProviderNeedsAPIKey(provider.Type) {
			provider.APIKeyRef = strings.TrimSpace(item.APIKeyRef)
		}
		out = append(out, provider)
	}
	return normalizeModelProviders(out), nil
}

func (s *ModelProviderService) resolveTestProvider(ctx context.Context, userID uuid.UUID, trustLevel int, req ModelProviderTestRequest) (ModelProvider, string, error) {
	if req.Provider != nil {
		provider, err := modelProviderFromSaveRequest(*req.Provider)
		if err != nil {
			return ModelProvider{}, "", err
		}
		apiKey := strings.TrimSpace(req.Provider.APIKey)
		if apiKey == "" && strings.TrimSpace(provider.APIKeyRef) != "" {
			apiKey, _ = s.readAPIKey(ctx, userID, provider.APIKeyRef, trustLevel)
		}
		return provider, apiKey, nil
	}
	return s.resolveStoredProvider(ctx, userID, trustLevel, req.ProviderID)
}

func (s *ModelProviderService) resolveStoredProvider(ctx context.Context, userID uuid.UUID, trustLevel int, providerID string) (ModelProvider, string, error) {
	doc, err := s.Load(ctx, userID, trustLevel)
	if err != nil {
		return ModelProvider{}, "", err
	}
	cleanID := strings.TrimSpace(providerID)
	if cleanID == "" {
		cleanID = firstNonEmpty(doc.DefaultSummaryProviderID, doc.DefaultProposalProviderID)
	}
	for _, provider := range doc.Providers {
		if provider.ID != cleanID {
			continue
		}
		apiKey, err := s.readAPIKey(ctx, userID, provider.APIKeyRef, trustLevel)
		if err != nil && modelProviderNeedsAPIKey(provider.Type) {
			return ModelProvider{}, "", err
		}
		return provider, apiKey, nil
	}
	return ModelProvider{}, "", fmt.Errorf("model provider %q not found", cleanID)
}

func (s *ModelProviderService) readAPIKey(ctx context.Context, userID uuid.UUID, ref string, trustLevel int) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", fmt.Errorf("api key is not configured")
	}
	if s.vault == nil {
		return "", fmt.Errorf("vault service not configured")
	}
	scope, ok := strings.CutPrefix(strings.TrimSpace(ref), "vault://")
	if !ok {
		return "", fmt.Errorf("unsupported api_key_ref")
	}
	return s.vault.Read(ctx, userID, scope, trustLevel)
}

func (s *ModelProviderService) testProvider(ctx context.Context, provider ModelProvider, apiKey string) error {
	switch provider.Type {
	case "ollama":
		return s.testOllama(ctx, provider)
	case "openai-compatible", "openai":
		return s.testOpenAICompatible(ctx, provider, apiKey)
	case "anthropic":
		_, err := s.generateAnthropicText(ctx, provider, apiKey, firstNonEmpty(provider.Models.Summary, provider.Models.JSON, provider.Models.Proposal), "Reply with OK.")
		return err
	case "gemini":
		_, err := s.generateGeminiText(ctx, provider, apiKey, firstNonEmpty(provider.Models.Summary, provider.Models.JSON, provider.Models.Proposal), "Reply with OK.")
		return err
	default:
		return fmt.Errorf("unsupported provider type %q", provider.Type)
	}
}

func (s *ModelProviderService) testOllama(ctx context.Context, provider ModelProvider) error {
	baseURL, err := normalizeProviderBaseURL(firstNonEmpty(provider.BaseURL, "http://localhost:11434"))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *ModelProviderService) testOpenAICompatible(ctx context.Context, provider ModelProvider, apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("api key is required")
	}
	baseURL, err := normalizeProviderBaseURL(provider.BaseURL)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("openai-compatible connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			return fmt.Errorf("openai-compatible returned status %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("openai-compatible returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *ModelProviderService) generateOpenAICompatibleText(ctx context.Context, provider ModelProvider, apiKey, model, prompt string) (string, error) {
	if provider.Type != "openai-compatible" && provider.Type != "openai" {
		return "", fmt.Errorf("GenerateText currently supports openai-compatible providers only")
	}
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("api key is required")
	}
	if strings.TrimSpace(model) == "" {
		return "", fmt.Errorf("model is required")
	}
	baseURL, err := normalizeProviderBaseURL(provider.BaseURL)
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("model provider returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("model provider returned empty content")
	}
	return parsed.Choices[0].Message.Content, nil
}

func (s *ModelProviderService) generateOllamaText(ctx context.Context, provider ModelProvider, model, prompt string) (string, error) {
	if strings.TrimSpace(model) == "" {
		return "", fmt.Errorf("model is required")
	}
	baseURL, err := normalizeProviderBaseURL(firstNonEmpty(provider.BaseURL, "http://localhost:11434"))
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/generate", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var parsed struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if strings.TrimSpace(parsed.Response) == "" {
		return "", fmt.Errorf("ollama returned empty response")
	}
	return parsed.Response, nil
}

func (s *ModelProviderService) generateAnthropicText(ctx context.Context, provider ModelProvider, apiKey, model, prompt string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("api key is required")
	}
	if strings.TrimSpace(model) == "" {
		return "", fmt.Errorf("model is required")
	}
	baseURL, err := normalizeProviderBaseURL(firstNonEmpty(provider.BaseURL, "https://api.anthropic.com"))
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{
		"model":      model,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	for _, part := range parsed.Content {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			return part.Text, nil
		}
	}
	return "", fmt.Errorf("anthropic returned empty text content")
}

func (s *ModelProviderService) generateGeminiText(ctx context.Context, provider ModelProvider, apiKey, model, prompt string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("api key is required")
	}
	model = strings.TrimPrefix(strings.TrimSpace(model), "models/")
	if model == "" {
		return "", fmt.Errorf("model is required")
	}
	baseURL, err := normalizeProviderBaseURL(firstNonEmpty(provider.BaseURL, "https://generativelanguage.googleapis.com/v1beta"))
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]string{{"text": prompt}}},
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/models/" + url.PathEscape(model) + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("gemini returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	for _, candidate := range parsed.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				return part.Text, nil
			}
		}
	}
	return "", fmt.Errorf("gemini returned empty text content")
}

func modelProviderFromSaveRequest(req ModelProviderSaveRequest) (ModelProvider, error) {
	id := normalizeModelProviderID(req.ID)
	if id == "" {
		return ModelProvider{}, fmt.Errorf("provider id is required")
	}
	providerType := normalizeModelProviderType(req.Type)
	if providerType == "" {
		return ModelProvider{}, fmt.Errorf("provider type is required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = id
	}
	baseURL := strings.TrimSpace(req.BaseURL)
	if providerType == "openai" && baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if providerType == "ollama" && baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if providerType == "anthropic" && baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	if providerType == "gemini" && baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	if baseURL != "" {
		if _, err := normalizeProviderBaseURL(baseURL); err != nil {
			return ModelProvider{}, err
		}
	}
	return ModelProvider{
		ID:        id,
		Type:      providerType,
		Name:      name,
		BaseURL:   baseURL,
		APIKeyRef: strings.TrimSpace(req.APIKeyRef),
		Models:    req.Models,
		Enabled:   req.Enabled,
	}, nil
}

func normalizeModelProviders(items []ModelProvider) []ModelProvider {
	out := make([]ModelProvider, 0, len(items))
	for _, item := range items {
		item.ID = normalizeModelProviderID(item.ID)
		item.Type = normalizeModelProviderType(item.Type)
		item.Name = strings.TrimSpace(item.Name)
		item.BaseURL = strings.TrimSpace(item.BaseURL)
		item.APIKeyRef = strings.TrimSpace(item.APIKeyRef)
		if item.ID == "" || item.Type == "" {
			continue
		}
		if item.Name == "" {
			item.Name = item.ID
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func normalizeModelProviderID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	valid := regexp.MustCompile(`[^a-z0-9-]+`)
	value = valid.ReplaceAllString(value, "")
	value = strings.Trim(value, "-")
	if len(value) > 64 {
		value = value[:64]
		value = strings.Trim(value, "-")
	}
	return value
}

func normalizeModelProviderType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "openai", "openai-compatible", "ollama", "anthropic", "gemini":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func modelProviderVaultScope(providerID string) string {
	return "model." + normalizeModelProviderID(providerID)
}

func modelProviderNeedsAPIKey(providerType string) bool {
	return providerType != "ollama"
}

func normalizeProviderBaseURL(value string) (string, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return "", fmt.Errorf("base_url is required")
	}
	parsed, err := url.Parse(clean)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid base_url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("base_url must use http or https")
	}
	return strings.TrimRight(clean, "/"), nil
}
