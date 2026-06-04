package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
)

const skillLearningSummaryVersion = "vola.skill-learning-summary/v1"

type skillLearningSummaryResponse struct {
	Version           string                         `json:"version"`
	Scope             string                         `json:"scope"`
	Team              *models.Team                   `json:"team,omitempty"`
	Stats             services.SkillLearningStats    `json:"stats"`
	Items             []services.SkillLearningItem   `json:"items"`
	Actions           []services.SkillLearningAction `json:"actions"`
	LatestRun         *services.LearningRun          `json:"latest_run,omitempty"`
	CandidateProposal *services.GrowthProposal       `json:"candidate_proposal,omitempty"`
}

type skillLearningRunResponse struct {
	Version string                `json:"version"`
	Scope   string                `json:"scope"`
	Team    *models.Team          `json:"team,omitempty"`
	Run     services.LearningRun  `json:"run"`
	Summary SkillLearningResponse `json:"summary"`
}

type SkillLearningResponse struct {
	Stats   services.SkillLearningStats    `json:"stats"`
	Items   []services.SkillLearningItem   `json:"items"`
	Actions []services.SkillLearningAction `json:"actions"`
}

type skillLearningNotesResponse struct {
	Version string                       `json:"version"`
	Scope   string                       `json:"scope"`
	Team    *models.Team                 `json:"team,omitempty"`
	Notes   []services.SkillLearningNote `json:"notes"`
}

func (s *Server) handleSkillsLearningSummary(w http.ResponseWriter, r *http.Request) {
	if s.FileTreeService == nil || s.SkillLearningService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "skill learning service not configured")
		return
	}

	target, ok := s.resolveScopedHubTarget(w, r, "", false)
	if !ok {
		return
	}

	summary, err := s.SkillLearningService.LoadSummary(r.Context(), target.UserID, trustLevelFromCtx(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	latestRun, err := s.SkillLearningService.LoadLatestLearningRun(r.Context(), target.UserID, trustLevelFromCtx(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondOK(w, skillLearningSummaryResponse{
		Version:   skillLearningSummaryVersion,
		Scope:     target.Scope,
		Team:      target.Team,
		Stats:     summary.Stats,
		Items:     summary.Items,
		Actions:   summary.Actions,
		LatestRun: latestRun,
	})
}

func (s *Server) handleSkillsLearningRecommend(w http.ResponseWriter, r *http.Request) {
	if s.FileTreeService == nil || s.SkillLearningService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "skill learning service not configured")
		return
	}

	target, ok := s.resolveScopedHubTarget(w, r, "", false)
	if !ok {
		return
	}

	query := r.URL.Query().Get("q")
	summary, err := s.SkillLearningService.Recommend(r.Context(), target.UserID, trustLevelFromCtx(r.Context()), query)
	if err == nil {
		notes, notesErr := s.SkillLearningService.ListRecentNotes(r.Context(), target.UserID, trustLevelFromCtx(r.Context()), 14)
		if notesErr == nil {
			summary, err = s.SkillLearningService.RecommendWithNotes(r.Context(), target.UserID, trustLevelFromCtx(r.Context()), query, notes)
		}
	}
	if err != nil {
		respondInternalError(w, err)
		return
	}
	var candidateProposal *services.GrowthProposal
	if shouldCreateCandidateSkillProposal(query, summary) && s.GrowthProposalService != nil {
		proposal, proposalErr := s.GrowthProposalService.CreateNewSkillProposal(r.Context(), target.UserID, trustLevelFromCtx(r.Context()), query)
		if proposalErr == nil {
			candidateProposal = &proposal
		}
	}

	respondOK(w, skillLearningSummaryResponse{
		Version:           skillLearningSummaryVersion,
		Scope:             target.Scope,
		Team:              target.Team,
		Stats:             summary.Stats,
		Items:             summary.Items,
		Actions:           summary.Actions,
		CandidateProposal: candidateProposal,
	})
}

func (s *Server) handleSkillsLearningNotes(w http.ResponseWriter, r *http.Request) {
	if s.FileTreeService == nil || s.SkillLearningService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "skill learning service not configured")
		return
	}

	target, ok := s.resolveScopedHubTarget(w, r, "", false)
	if !ok {
		return
	}

	days := 14
	if value := r.URL.Query().Get("days"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			days = parsed
		}
	}

	notes, err := s.SkillLearningService.ListRecentNotes(r.Context(), target.UserID, trustLevelFromCtx(r.Context()), days)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondOK(w, skillLearningNotesResponse{
		Version: skillLearningSummaryVersion,
		Scope:   target.Scope,
		Team:    target.Team,
		Notes:   notes,
	})
}

func shouldCreateCandidateSkillProposal(query string, summary services.SkillLearningSummary) bool {
	if strings.TrimSpace(query) == "" {
		return false
	}
	if len(summary.Items) == 0 {
		return true
	}
	hasContentMatch := false
	for _, reason := range summary.Items[0].MatchReasons {
		if strings.Contains(reason, "名称") || strings.Contains(reason, "路径") || strings.Contains(reason, "说明") || strings.Contains(reason, "标签") {
			hasContentMatch = true
			break
		}
	}
	if !hasContentMatch {
		return true
	}
	top := summary.Items[0].MatchScore
	if top == 0 {
		top = summary.Items[0].Score
	}
	return top < 35
}

func (s *Server) handleSkillsLearningRunCreate(w http.ResponseWriter, r *http.Request) {
	if s.FileTreeService == nil || s.SkillLearningService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "skill learning service not configured")
		return
	}

	target, ok := s.resolveScopedHubTarget(w, r, "", false)
	if !ok {
		return
	}

	_, summary, run, err := s.SkillLearningService.WriteDailyLearningRun(r.Context(), target.UserID, trustLevelFromCtx(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondOK(w, skillLearningRunResponse{
		Version: skillLearningSummaryVersion,
		Scope:   target.Scope,
		Team:    target.Team,
		Run:     run,
		Summary: SkillLearningResponse{
			Stats:   summary.Stats,
			Items:   summary.Items,
			Actions: summary.Actions,
		},
	})
}
