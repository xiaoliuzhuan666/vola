package api

import (
	"errors"
	"net/http"

	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/go-chi/chi/v5"
)

const growthProposalsResponseVersion = "vola.growth-proposals/v1"

type growthProposalsResponse struct {
	Version   string                    `json:"version"`
	Scope     string                    `json:"scope"`
	Team      *models.Team              `json:"team,omitempty"`
	Proposals []services.GrowthProposal `json:"proposals"`
}

type growthProposalResponse struct {
	Version  string                  `json:"version"`
	Scope    string                  `json:"scope"`
	Team     *models.Team            `json:"team,omitempty"`
	Proposal services.GrowthProposal `json:"proposal"`
}

func (s *Server) handleGrowthProposalsList(w http.ResponseWriter, r *http.Request) {
	if s.GrowthProposalService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "growth proposal service not configured")
		return
	}
	target, ok := s.resolveScopedHubTarget(w, r, "", false)
	if !ok {
		return
	}
	proposals, err := s.GrowthProposalService.List(r.Context(), target.UserID, trustLevelFromCtx(r.Context()), r.URL.Query().Get("status"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, growthProposalsResponse{
		Version:   growthProposalsResponseVersion,
		Scope:     target.Scope,
		Team:      target.Team,
		Proposals: proposals,
	})
}

func (s *Server) handleGrowthProposalAccept(w http.ResponseWriter, r *http.Request) {
	s.handleGrowthProposalAction(w, r, "accept")
}

func (s *Server) handleGrowthProposalDismiss(w http.ResponseWriter, r *http.Request) {
	s.handleGrowthProposalAction(w, r, "dismiss")
}

func (s *Server) handleGrowthProposalApply(w http.ResponseWriter, r *http.Request) {
	s.handleGrowthProposalAction(w, r, "apply")
}

func (s *Server) handleGrowthProposalAction(w http.ResponseWriter, r *http.Request, action string) {
	if s.GrowthProposalService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "growth proposal service not configured")
		return
	}
	target, ok := s.resolveScopedHubTarget(w, r, "", false)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var (
		proposal services.GrowthProposal
		err      error
	)
	switch action {
	case "accept":
		proposal, err = s.GrowthProposalService.Accept(r.Context(), target.UserID, trustLevelFromCtx(r.Context()), id)
	case "dismiss":
		proposal, err = s.GrowthProposalService.Dismiss(r.Context(), target.UserID, trustLevelFromCtx(r.Context()), id)
	case "apply":
		proposal, err = s.GrowthProposalService.Apply(r.Context(), target.UserID, trustLevelFromCtx(r.Context()), id)
	default:
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "unsupported proposal action")
		return
	}
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			respondError(w, http.StatusNotFound, ErrCodeNotFound, "growth proposal not found")
			return
		}
		respondInternalError(w, err)
		return
	}
	respondOK(w, growthProposalResponse{
		Version:  growthProposalsResponseVersion,
		Scope:    target.Scope,
		Team:     target.Team,
		Proposal: proposal,
	})
}
