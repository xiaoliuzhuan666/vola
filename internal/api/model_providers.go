package api

import (
	"encoding/json"
	"net/http"

	"github.com/agi-bar/vola/internal/services"
)

type modelProvidersResponse struct {
	Version                   string                           `json:"version"`
	StoragePath               string                           `json:"storage_path"`
	UpdatedAt                 string                           `json:"updated_at,omitempty"`
	DefaultSummaryProviderID  string                           `json:"default_summary_provider_id,omitempty"`
	DefaultProposalProviderID string                           `json:"default_proposal_provider_id,omitempty"`
	Providers                 []services.ModelProvider         `json:"providers"`
	SupportedTypes            []modelProviderSupportedTypeInfo `json:"supported_types"`
}

type modelProviderSupportedTypeInfo struct {
	Type              string `json:"type"`
	Name              string `json:"name"`
	RequiresAPIKey    bool   `json:"requires_api_key"`
	DefaultBaseURL    string `json:"default_base_url,omitempty"`
	LiveTestSupported bool   `json:"live_test_supported"`
}

func (s *Server) handleModelProvidersGet(w http.ResponseWriter, r *http.Request) {
	if s.ModelProviderService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "model provider service not configured")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	doc, err := s.ModelProviderService.Load(r.Context(), userID, trustLevelFromCtx(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, modelProvidersResponseFromDocument(doc))
}

func (s *Server) handleModelProvidersSave(w http.ResponseWriter, r *http.Request) {
	if s.ModelProviderService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "model provider service not configured")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req services.SaveModelProvidersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	doc, err := s.ModelProviderService.Save(r.Context(), userID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOKWithLocalGitSync(w, modelProvidersResponseFromDocument(doc), s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleModelProvidersTest(w http.ResponseWriter, r *http.Request) {
	if s.ModelProviderService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "model provider service not configured")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req services.ModelProviderTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	result, err := s.ModelProviderService.Test(r.Context(), userID, trustLevelFromCtx(r.Context()), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, result)
}

func modelProvidersResponseFromDocument(doc services.ModelProviderDocument) modelProvidersResponse {
	return modelProvidersResponse{
		Version:                   services.ModelProvidersVersion,
		StoragePath:               services.ModelProvidersPath,
		UpdatedAt:                 doc.UpdatedAt,
		DefaultSummaryProviderID:  doc.DefaultSummaryProviderID,
		DefaultProposalProviderID: doc.DefaultProposalProviderID,
		Providers:                 doc.Providers,
		SupportedTypes: []modelProviderSupportedTypeInfo{
			{Type: "openai-compatible", Name: "OpenAI-compatible", RequiresAPIKey: true, LiveTestSupported: true},
			{Type: "openai", Name: "OpenAI", RequiresAPIKey: true, DefaultBaseURL: "https://api.openai.com/v1", LiveTestSupported: true},
			{Type: "ollama", Name: "Ollama", RequiresAPIKey: false, DefaultBaseURL: "http://localhost:11434", LiveTestSupported: true},
			{Type: "anthropic", Name: "Anthropic", RequiresAPIKey: true, DefaultBaseURL: "https://api.anthropic.com", LiveTestSupported: true},
			{Type: "gemini", Name: "Gemini", RequiresAPIKey: true, DefaultBaseURL: "https://generativelanguage.googleapis.com/v1beta", LiveTestSupported: true},
		},
	}
}
