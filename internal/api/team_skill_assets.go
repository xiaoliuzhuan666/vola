package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	pathpkg "path"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	teamSkillPublicationsVersion  = "vola.team-skill-publications/v1"
	teamSkillPublicationsPath     = "/settings/team-skill-publications.json"
	teamSkillSubscriptionsVersion = "vola.team-skill-subscriptions/v1"
	teamSkillSubscriptionsPath    = "/settings/team-skill-subscriptions.json"
	teamSkillReviewHistoryVersion = "vola.team-skill-review-history/v1"
	teamSkillReviewHistoryPath    = "/settings/team-skill-review-history.json"
	teamSkillNotificationsVersion = "vola.team-skill-update-notifications/v1"
	teamSkillNotificationsPath    = "/settings/team-skill-update-notifications.json"
)

type teamSkillPublication struct {
	SkillPath             string `json:"skill_path"`
	Status                string `json:"status"`
	Visibility            string `json:"visibility"`
	Note                  string `json:"note,omitempty"`
	ReviewStatus          string `json:"review_status,omitempty"`
	ReviewNote            string `json:"review_note,omitempty"`
	ReviewRequestedAt     string `json:"review_requested_at,omitempty"`
	ReviewRequestedBy     string `json:"review_requested_by,omitempty"`
	ReviewRequestedByRole string `json:"review_requested_by_role,omitempty"`
	ReviewedAt            string `json:"reviewed_at,omitempty"`
	ReviewedBy            string `json:"reviewed_by,omitempty"`
	ReviewedByRole        string `json:"reviewed_by_role,omitempty"`
	UpdatedAt             string `json:"updated_at,omitempty"`
	PublishedAt           string `json:"published_at,omitempty"`
	ArchivedAt            string `json:"archived_at,omitempty"`
	Implicit              bool   `json:"implicit,omitempty"`
}

type teamSkillPublicationsDocument struct {
	Version      string                 `json:"version"`
	UpdatedAt    string                 `json:"updated_at,omitempty"`
	Publications []teamSkillPublication `json:"publications"`
}

type teamSkillPublicationsResponse struct {
	Version      string                 `json:"version"`
	Team         *models.Team           `json:"team,omitempty"`
	StoragePath  string                 `json:"storage_path"`
	UpdatedAt    string                 `json:"updated_at,omitempty"`
	Publications []teamSkillPublication `json:"publications"`
}

type teamSkillPublicationSaveRequest struct {
	SkillPath  string `json:"skill_path"`
	Status     string `json:"status"`
	Visibility string `json:"visibility"`
	Note       string `json:"note,omitempty"`
}

type teamSkillReviewEvent struct {
	ID         string `json:"id"`
	AssetType  string `json:"asset_type"`
	SkillPath  string `json:"skill_path,omitempty"`
	AgentSlug  string `json:"agent_slug,omitempty"`
	Action     string `json:"action"`
	Status     string `json:"status,omitempty"`
	Visibility string `json:"visibility,omitempty"`
	Note       string `json:"note,omitempty"`
	ActorID    string `json:"actor_id,omitempty"`
	ActorRole  string `json:"actor_role,omitempty"`
	CreatedAt  string `json:"created_at"`
}

type teamSkillReviewHistoryDocument struct {
	Version   string                 `json:"version"`
	UpdatedAt string                 `json:"updated_at,omitempty"`
	Events    []teamSkillReviewEvent `json:"events"`
}

type teamSkillReviewHistoryResponse struct {
	Version     string                 `json:"version"`
	Team        *models.Team           `json:"team,omitempty"`
	StoragePath string                 `json:"storage_path"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
	Events      []teamSkillReviewEvent `json:"events"`
}

type teamSkillReviewRequest struct {
	AssetType string `json:"asset_type,omitempty"`
	SkillPath string `json:"skill_path,omitempty"`
	AgentSlug string `json:"agent_slug,omitempty"`
	Note      string `json:"note,omitempty"`
}

type teamSkillReviewResolveRequest struct {
	AssetType string `json:"asset_type,omitempty"`
	SkillPath string `json:"skill_path,omitempty"`
	AgentSlug string `json:"agent_slug,omitempty"`
	Decision  string `json:"decision"`
	Note      string `json:"note,omitempty"`
}

type teamSkillSubscription struct {
	TeamID            string `json:"team_id"`
	TeamSlug          string `json:"team_slug,omitempty"`
	TeamName          string `json:"team_name,omitempty"`
	SourcePath        string `json:"source_path"`
	TargetPath        string `json:"target_path"`
	SourceFingerprint string `json:"source_fingerprint,omitempty"`
	Files             int    `json:"files"`
	Bytes             int64  `json:"bytes"`
	InstalledAt       string `json:"installed_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
	CheckedAt         string `json:"checked_at,omitempty"`
}

type teamSkillSubscriptionsDocument struct {
	Version       string                  `json:"version"`
	UpdatedAt     string                  `json:"updated_at,omitempty"`
	Subscriptions []teamSkillSubscription `json:"subscriptions"`
}

type teamSkillSubscriptionStatus struct {
	teamSkillSubscription
	SourceCurrentFingerprint string `json:"source_current_fingerprint,omitempty"`
	UpdateAvailable          bool   `json:"update_available"`
	SourceMissing            bool   `json:"source_missing"`
	Error                    string `json:"error,omitempty"`
}

type teamSkillSubscriptionsResponse struct {
	Version       string                        `json:"version"`
	StoragePath   string                        `json:"storage_path"`
	UpdatedAt     string                        `json:"updated_at,omitempty"`
	CheckedAt     string                        `json:"checked_at,omitempty"`
	Subscriptions []teamSkillSubscriptionStatus `json:"subscriptions"`
}

type teamSkillSubscriptionMemberReport struct {
	UserID                   string `json:"user_id"`
	UserSlug                 string `json:"user_slug,omitempty"`
	DisplayName              string `json:"display_name,omitempty"`
	Role                     string `json:"role,omitempty"`
	Status                   string `json:"status"`
	TargetPath               string `json:"target_path,omitempty"`
	SourceFingerprint        string `json:"source_fingerprint,omitempty"`
	SourceCurrentFingerprint string `json:"source_current_fingerprint,omitempty"`
	Files                    int    `json:"files,omitempty"`
	Bytes                    int64  `json:"bytes,omitempty"`
	InstalledAt              string `json:"installed_at,omitempty"`
	UpdatedAt                string `json:"updated_at,omitempty"`
	CheckedAt                string `json:"checked_at,omitempty"`
	UpdateAvailable          bool   `json:"update_available"`
	SourceMissing            bool   `json:"source_missing"`
	Error                    string `json:"error,omitempty"`
}

type teamSkillSubscriptionSkillReport struct {
	SkillPath            string                              `json:"skill_path"`
	Status               string                              `json:"status"`
	Visibility           string                              `json:"visibility"`
	SourceFingerprint    string                              `json:"source_fingerprint,omitempty"`
	SourceMissing        bool                                `json:"source_missing"`
	InstalledCount       int                                 `json:"installed_count"`
	UpdateAvailableCount int                                 `json:"update_available_count"`
	SourceMissingCount   int                                 `json:"source_missing_count"`
	NotInstalledCount    int                                 `json:"not_installed_count"`
	Members              []teamSkillSubscriptionMemberReport `json:"members"`
}

type teamSkillSubscriptionReportResponse struct {
	Version     string                             `json:"version"`
	Team        *models.Team                       `json:"team,omitempty"`
	GeneratedAt string                             `json:"generated_at"`
	Skills      []teamSkillSubscriptionSkillReport `json:"skills"`
}

type teamSkillUpdateNotification struct {
	ID                   string `json:"id"`
	Kind                 string `json:"kind"`
	TeamID               string `json:"team_id,omitempty"`
	TeamSlug             string `json:"team_slug,omitempty"`
	SkillPath            string `json:"skill_path,omitempty"`
	UserID               string `json:"user_id,omitempty"`
	UserSlug             string `json:"user_slug,omitempty"`
	DisplayName          string `json:"display_name,omitempty"`
	MemberRole           string `json:"member_role,omitempty"`
	Status               string `json:"status"`
	Message              string `json:"message"`
	SourceFingerprint    string `json:"source_fingerprint,omitempty"`
	InstalledFingerprint string `json:"installed_fingerprint,omitempty"`
	CreatedAt            string `json:"created_at"`
}

type teamSkillUpdateNotificationsDocument struct {
	Version       string                        `json:"version"`
	UpdatedAt     string                        `json:"updated_at,omitempty"`
	LastCheckedAt string                        `json:"last_checked_at,omitempty"`
	Notifications []teamSkillUpdateNotification `json:"notifications"`
}

type teamSkillUpdateNotificationsResponse struct {
	Version       string                               `json:"version"`
	Team          *models.Team                         `json:"team,omitempty"`
	StoragePath   string                               `json:"storage_path"`
	UpdatedAt     string                               `json:"updated_at,omitempty"`
	LastCheckedAt string                               `json:"last_checked_at,omitempty"`
	Notifications []teamSkillUpdateNotification        `json:"notifications"`
	Report        *teamSkillSubscriptionReportResponse `json:"report,omitempty"`
}

func (s *Server) handleTeamSkillPublicationsList(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	doc, err := s.readTeamSkillPublications(r.Context(), team.HubUserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	skills, err := s.listSkills(r.Context(), team.HubUserID, trustLevelFromCtx(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	publications := s.teamSkillPublicationsForSkills(doc, skills, team)
	respondOK(w, teamSkillPublicationsResponse{
		Version:      teamSkillPublicationsVersion,
		Team:         team,
		StoragePath:  teamSkillPublicationsPath,
		UpdatedAt:    doc.UpdatedAt,
		Publications: publications,
	})
}

func (s *Server) handleTeamSkillPublicationSave(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req teamSkillPublicationSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	publication := normalizeTeamSkillPublication(teamSkillPublication{
		SkillPath:  req.SkillPath,
		Status:     req.Status,
		Visibility: req.Visibility,
		Note:       strings.TrimSpace(req.Note),
	})
	if publication.SkillPath == "" || publication.SkillPath == "/skills" {
		respondValidationError(w, "skill_path", "skill_path must point to one skill under /skills")
		return
	}
	if !team.CanWrite {
		respondForbidden(w, "this team role cannot update skill publication state")
		return
	}
	if teamSkillPublicationNeedsAdmin(publication) && !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can publish or archive team skills")
		return
	}
	doc, err := s.readTeamSkillPublications(r.Context(), team.HubUserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	doc = upsertTeamSkillPublication(doc, publication, now)
	if err := s.writeTeamSkillPublications(s.requestSourceContext(r, "team-skill-publication"), team.HubUserID, doc); err != nil {
		respondInternalError(w, err)
		return
	}
	if err := s.appendTeamSkillReviewEvent(s.requestSourceContext(r, "team-skill-publication"), team.HubUserID, teamSkillReviewEvent{
		ID:         uuid.NewString(),
		AssetType:  "skill",
		SkillPath:  publication.SkillPath,
		Action:     teamSkillPublicationAction(publication),
		Status:     publication.Status,
		Visibility: publication.Visibility,
		Note:       publication.Note,
		ActorID:    userID.String(),
		ActorRole:  team.Role,
		CreatedAt:  now,
	}); err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, teamSkillPublicationsResponse{
		Version:      teamSkillPublicationsVersion,
		Team:         team,
		StoragePath:  teamSkillPublicationsPath,
		UpdatedAt:    doc.UpdatedAt,
		Publications: doc.Publications,
	})
}

func (s *Server) handleTeamSkillSubscriptionsList(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	doc, err := s.readTeamSkillSubscriptions(r.Context(), userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	teamFilter := strings.TrimSpace(r.URL.Query().Get("team_id"))
	if teamFilter == "" {
		teamFilter = strings.TrimSpace(r.URL.Query().Get("team"))
	}
	checkedAt := time.Now().UTC().Format(time.RFC3339)
	statuses := make([]teamSkillSubscriptionStatus, 0, len(doc.Subscriptions))
	for _, item := range doc.Subscriptions {
		if teamFilter != "" && teamFilter != item.TeamID && teamFilter != item.TeamSlug {
			continue
		}
		status := teamSkillSubscriptionStatus{teamSkillSubscription: item}
		status.CheckedAt = checkedAt
		if s.TeamService != nil {
			team, err := s.resolveTeamForSubscription(r.Context(), userID, item)
			if err != nil {
				status.Error = err.Error()
				status.SourceMissing = true
			} else {
				files, err := s.collectLocalSkillFiles(r.Context(), team.HubUserID, item.SourcePath)
				if err != nil {
					status.SourceMissing = true
					if !errors.Is(err, services.ErrEntryNotFound) {
						status.Error = err.Error()
					}
				} else {
					status.SourceCurrentFingerprint = localSkillFilesFingerprint(files)
					status.UpdateAvailable = item.SourceFingerprint != "" && status.SourceCurrentFingerprint != "" && item.SourceFingerprint != status.SourceCurrentFingerprint
				}
			}
		}
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].TeamSlug == statuses[j].TeamSlug {
			return statuses[i].SourcePath < statuses[j].SourcePath
		}
		return statuses[i].TeamSlug < statuses[j].TeamSlug
	})
	respondOK(w, teamSkillSubscriptionsResponse{
		Version:       teamSkillSubscriptionsVersion,
		StoragePath:   teamSkillSubscriptionsPath,
		UpdatedAt:     doc.UpdatedAt,
		CheckedAt:     checkedAt,
		Subscriptions: statuses,
	})
}

func (s *Server) handleTeamSkillReviewHistoryList(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	doc, err := s.readTeamSkillReviewHistory(r.Context(), team.HubUserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	assetType := normalizeTeamReviewAssetType(r.URL.Query().Get("asset_type"))
	skillPath := normalizeAssignedSkillPath(r.URL.Query().Get("skill_path"))
	agentSlug := normalizeSlugInput(r.URL.Query().Get("agent_slug"))
	events := filterTeamSkillReviewEvents(doc.Events, assetType, skillPath, agentSlug)
	respondOK(w, teamSkillReviewHistoryResponse{
		Version:     teamSkillReviewHistoryVersion,
		Team:        team,
		StoragePath: teamSkillReviewHistoryPath,
		UpdatedAt:   doc.UpdatedAt,
		Events:      events,
	})
}

func (s *Server) handleTeamSkillReviewRequestCreate(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanWrite {
		respondForbidden(w, "this team role cannot request reviews")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req teamSkillReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	assetType := normalizeTeamReviewAssetType(req.AssetType)
	now := time.Now().UTC().Format(time.RFC3339)
	event := teamSkillReviewEvent{
		ID:        uuid.NewString(),
		AssetType: assetType,
		Action:    "request_review",
		Note:      strings.TrimSpace(req.Note),
		ActorID:   userID.String(),
		ActorRole: team.Role,
		CreatedAt: now,
	}
	switch assetType {
	case "skill":
		publication := normalizeTeamSkillPublication(teamSkillPublication{
			SkillPath:             req.SkillPath,
			Status:                "draft",
			Visibility:            "private",
			Note:                  strings.TrimSpace(req.Note),
			ReviewStatus:          "requested",
			ReviewNote:            strings.TrimSpace(req.Note),
			ReviewRequestedAt:     now,
			ReviewRequestedBy:     userID.String(),
			ReviewRequestedByRole: team.Role,
		})
		if publication.SkillPath == "" || publication.SkillPath == "/skills" {
			respondValidationError(w, "skill_path", "skill_path must point to one skill under /skills")
			return
		}
		doc, err := s.readTeamSkillPublications(r.Context(), team.HubUserID)
		if err != nil {
			respondInternalError(w, err)
			return
		}
		doc = upsertTeamSkillPublication(doc, publication, now)
		if err := s.writeTeamSkillPublications(s.requestSourceContext(r, "team-skill-review"), team.HubUserID, doc); err != nil {
			respondInternalError(w, err)
			return
		}
		event.SkillPath = publication.SkillPath
		event.Status = publication.Status
		event.Visibility = publication.Visibility
	case "agent":
		slug := normalizeSlugInput(req.AgentSlug)
		if slug == "" {
			respondValidationError(w, "agent_slug", "agent_slug is required")
			return
		}
		asset, err := s.readTeamAgentAsset(r.Context(), team.HubUserID, slug)
		if err != nil {
			respondNotFound(w, "team agent")
			return
		}
		asset.ReviewStatus = "requested"
		asset.ReviewNote = strings.TrimSpace(req.Note)
		asset.ReviewRequestedAt = now
		asset.ReviewRequestedBy = userID.String()
		asset.ReviewRequestedByRole = team.Role
		asset.UpdatedAt = now
		if err := s.writeTeamAgentAsset(s.requestSourceContext(r, "team-agent-review"), team.HubUserID, asset); err != nil {
			respondInternalError(w, err)
			return
		}
		event.AgentSlug = asset.Slug
		event.Status = asset.Status
		event.Visibility = asset.Visibility
	default:
		respondValidationError(w, "asset_type", "asset_type must be skill or agent")
		return
	}
	if err := s.appendTeamSkillReviewEvent(s.requestSourceContext(r, "team-skill-review"), team.HubUserID, event); err != nil {
		respondInternalError(w, err)
		return
	}
	history, err := s.readTeamSkillReviewHistory(r.Context(), team.HubUserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, teamSkillReviewHistoryResponse{
		Version:     teamSkillReviewHistoryVersion,
		Team:        team,
		StoragePath: teamSkillReviewHistoryPath,
		UpdatedAt:   history.UpdatedAt,
		Events:      history.Events,
	})
}

func (s *Server) handleTeamSkillReviewResolve(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can resolve reviews")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req teamSkillReviewResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	decision := strings.TrimSpace(strings.ToLower(req.Decision))
	if decision != "approved" && decision != "changes_requested" {
		respondValidationError(w, "decision", "decision must be approved or changes_requested")
		return
	}
	assetType := normalizeTeamReviewAssetType(req.AssetType)
	now := time.Now().UTC().Format(time.RFC3339)
	event := teamSkillReviewEvent{
		ID:        uuid.NewString(),
		AssetType: assetType,
		Action:    decision,
		Note:      strings.TrimSpace(req.Note),
		ActorID:   userID.String(),
		ActorRole: team.Role,
		CreatedAt: now,
	}
	nextStatus := "draft"
	nextVisibility := "private"
	if decision == "approved" {
		nextStatus = "published"
		nextVisibility = "team"
	}
	switch assetType {
	case "skill":
		skillPath := normalizeAssignedSkillPath(req.SkillPath)
		if skillPath == "" || skillPath == "/skills" {
			respondValidationError(w, "skill_path", "skill_path must point to one skill under /skills")
			return
		}
		doc, err := s.readTeamSkillPublications(r.Context(), team.HubUserID)
		if err != nil {
			respondInternalError(w, err)
			return
		}
		publication := teamSkillPublication{
			SkillPath:      skillPath,
			Status:         nextStatus,
			Visibility:     nextVisibility,
			Note:           strings.TrimSpace(req.Note),
			ReviewStatus:   decision,
			ReviewNote:     strings.TrimSpace(req.Note),
			ReviewedAt:     now,
			ReviewedBy:     userID.String(),
			ReviewedByRole: team.Role,
		}
		if current, ok := teamSkillPublicationLookup(doc)[skillPath]; ok {
			publication.ReviewRequestedAt = current.ReviewRequestedAt
			publication.ReviewRequestedBy = current.ReviewRequestedBy
			publication.ReviewRequestedByRole = current.ReviewRequestedByRole
			if publication.Note == "" {
				publication.Note = current.Note
			}
		}
		doc = upsertTeamSkillPublication(doc, publication, now)
		if err := s.writeTeamSkillPublications(s.requestSourceContext(r, "team-skill-review"), team.HubUserID, doc); err != nil {
			respondInternalError(w, err)
			return
		}
		event.SkillPath = skillPath
		event.Status = nextStatus
		event.Visibility = nextVisibility
	case "agent":
		slug := normalizeSlugInput(req.AgentSlug)
		if slug == "" {
			respondValidationError(w, "agent_slug", "agent_slug is required")
			return
		}
		asset, err := s.readTeamAgentAsset(r.Context(), team.HubUserID, slug)
		if err != nil {
			respondNotFound(w, "team agent")
			return
		}
		asset.Status = nextStatus
		asset.Visibility = nextVisibility
		asset.ReviewStatus = decision
		asset.ReviewNote = strings.TrimSpace(req.Note)
		asset.ReviewedAt = now
		asset.ReviewedBy = userID.String()
		asset.ReviewedByRole = team.Role
		asset.UpdatedAt = now
		if asset.Status == "published" && asset.PublishedAt == "" {
			asset.PublishedAt = now
		}
		if err := s.writeTeamAgentAsset(s.requestSourceContext(r, "team-agent-review"), team.HubUserID, asset); err != nil {
			respondInternalError(w, err)
			return
		}
		event.AgentSlug = asset.Slug
		event.Status = asset.Status
		event.Visibility = asset.Visibility
	default:
		respondValidationError(w, "asset_type", "asset_type must be skill or agent")
		return
	}
	if err := s.appendTeamSkillReviewEvent(s.requestSourceContext(r, "team-skill-review"), team.HubUserID, event); err != nil {
		respondInternalError(w, err)
		return
	}
	history, err := s.readTeamSkillReviewHistory(r.Context(), team.HubUserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, teamSkillReviewHistoryResponse{
		Version:     teamSkillReviewHistoryVersion,
		Team:        team,
		StoragePath: teamSkillReviewHistoryPath,
		UpdatedAt:   history.UpdatedAt,
		Events:      history.Events,
	})
}

func (s *Server) handleTeamSkillSubscriptionReport(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can view member subscription reports")
		return
	}
	report, err := s.buildTeamSkillSubscriptionReport(r.Context(), team, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, report)
}

func (s *Server) handleTeamSkillSubscriptionsCheck(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can run team update checks")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	report, err := s.buildTeamSkillSubscriptionReport(r.Context(), team, now)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	notifications := teamSkillNotificationsFromReport(team, report, now)
	doc := teamSkillUpdateNotificationsDocument{
		Version:       teamSkillNotificationsVersion,
		UpdatedAt:     now,
		LastCheckedAt: now,
		Notifications: notifications,
	}
	if err := s.writeTeamSkillUpdateNotifications(s.requestSourceContext(r, "team-skill-update-check"), team.HubUserID, doc); err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, teamSkillUpdateNotificationsResponse{
		Version:       teamSkillNotificationsVersion,
		Team:          team,
		StoragePath:   teamSkillNotificationsPath,
		UpdatedAt:     doc.UpdatedAt,
		LastCheckedAt: doc.LastCheckedAt,
		Notifications: doc.Notifications,
		Report:        &report,
	})
}

func (s *Server) handleTeamSkillUpdateNotificationsList(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can view team update notifications")
		return
	}
	doc, err := s.readTeamSkillUpdateNotifications(r.Context(), team.HubUserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, teamSkillUpdateNotificationsResponse{
		Version:       teamSkillNotificationsVersion,
		Team:          team,
		StoragePath:   teamSkillNotificationsPath,
		UpdatedAt:     doc.UpdatedAt,
		LastCheckedAt: doc.LastCheckedAt,
		Notifications: doc.Notifications,
	})
}

func (s *Server) readTeamSkillPublications(ctx context.Context, teamHubUserID uuid.UUID) (teamSkillPublicationsDocument, error) {
	entry, err := s.FileTreeService.Read(ctx, teamHubUserID, teamSkillPublicationsPath, models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			return teamSkillPublicationsDocument{Version: teamSkillPublicationsVersion, Publications: []teamSkillPublication{}}, nil
		}
		return teamSkillPublicationsDocument{}, err
	}
	var doc teamSkillPublicationsDocument
	if err := json.Unmarshal([]byte(entry.Content), &doc); err != nil {
		return teamSkillPublicationsDocument{}, err
	}
	doc.Version = teamSkillPublicationsVersion
	doc.Publications = normalizeTeamSkillPublications(doc.Publications)
	return doc, nil
}

func (s *Server) writeTeamSkillPublications(ctx context.Context, teamHubUserID uuid.UUID, doc teamSkillPublicationsDocument) error {
	doc.Version = teamSkillPublicationsVersion
	doc.Publications = normalizeTeamSkillPublications(doc.Publications)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	_, err = s.FileTreeService.WriteEntry(ctx, teamHubUserID, teamSkillPublicationsPath, string(append(data, '\n')), "application/json", models.FileTreeWriteOptions{
		Kind: "team_skill_publications",
		Metadata: map[string]interface{}{
			"source":       "team-skill-publication",
			"capture_mode": "team-skill-publication",
		},
		MinTrustLevel: models.TrustLevelWork,
	})
	return err
}

func (s *Server) readTeamSkillReviewHistory(ctx context.Context, teamHubUserID uuid.UUID) (teamSkillReviewHistoryDocument, error) {
	entry, err := s.FileTreeService.Read(ctx, teamHubUserID, teamSkillReviewHistoryPath, models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			return teamSkillReviewHistoryDocument{Version: teamSkillReviewHistoryVersion, Events: []teamSkillReviewEvent{}}, nil
		}
		return teamSkillReviewHistoryDocument{}, err
	}
	var doc teamSkillReviewHistoryDocument
	if err := json.Unmarshal([]byte(entry.Content), &doc); err != nil {
		return teamSkillReviewHistoryDocument{}, err
	}
	doc.Version = teamSkillReviewHistoryVersion
	doc.Events = normalizeTeamSkillReviewEvents(doc.Events)
	return doc, nil
}

func (s *Server) writeTeamSkillReviewHistory(ctx context.Context, teamHubUserID uuid.UUID, doc teamSkillReviewHistoryDocument) error {
	doc.Version = teamSkillReviewHistoryVersion
	doc.Events = normalizeTeamSkillReviewEvents(doc.Events)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	_, err = s.FileTreeService.WriteEntry(ctx, teamHubUserID, teamSkillReviewHistoryPath, string(append(data, '\n')), "application/json", models.FileTreeWriteOptions{
		Kind: "team_skill_review_history",
		Metadata: map[string]interface{}{
			"source":       "team-skill-review",
			"capture_mode": "team-skill-review",
		},
		MinTrustLevel: models.TrustLevelWork,
	})
	return err
}

func (s *Server) appendTeamSkillReviewEvent(ctx context.Context, teamHubUserID uuid.UUID, event teamSkillReviewEvent) error {
	event = normalizeTeamSkillReviewEvent(event)
	if event.AssetType == "" || event.Action == "" {
		return nil
	}
	doc, err := s.readTeamSkillReviewHistory(ctx, teamHubUserID)
	if err != nil {
		return err
	}
	doc.Events = append(doc.Events, event)
	doc.Events = normalizeTeamSkillReviewEvents(doc.Events)
	if len(doc.Events) > 300 {
		doc.Events = doc.Events[:300]
	}
	doc.UpdatedAt = event.CreatedAt
	return s.writeTeamSkillReviewHistory(ctx, teamHubUserID, doc)
}

func normalizeTeamSkillReviewEvents(items []teamSkillReviewEvent) []teamSkillReviewEvent {
	out := make([]teamSkillReviewEvent, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = normalizeTeamSkillReviewEvent(item)
		if item.AssetType == "" || item.Action == "" {
			continue
		}
		key := item.ID
		if key == "" {
			key = item.AssetType + "\x00" + item.SkillPath + "\x00" + item.AgentSlug + "\x00" + item.Action + "\x00" + item.CreatedAt
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func normalizeTeamSkillReviewEvent(item teamSkillReviewEvent) teamSkillReviewEvent {
	item.ID = strings.TrimSpace(item.ID)
	item.AssetType = normalizeTeamReviewAssetType(item.AssetType)
	item.SkillPath = normalizeAssignedSkillPath(item.SkillPath)
	item.AgentSlug = normalizeSlugInput(item.AgentSlug)
	item.Action = strings.TrimSpace(strings.ToLower(item.Action))
	item.Status = strings.TrimSpace(strings.ToLower(item.Status))
	item.Visibility = strings.TrimSpace(strings.ToLower(item.Visibility))
	item.Note = strings.TrimSpace(item.Note)
	item.ActorID = strings.TrimSpace(item.ActorID)
	item.ActorRole = strings.TrimSpace(strings.ToLower(item.ActorRole))
	item.CreatedAt = strings.TrimSpace(item.CreatedAt)
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.CreatedAt == "" {
		item.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if item.AssetType == "skill" && (item.SkillPath == "" || item.SkillPath == "/skills") {
		item.AssetType = ""
	}
	if item.AssetType == "agent" && item.AgentSlug == "" {
		item.AssetType = ""
	}
	return item
}

func normalizeTeamReviewAssetType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "skill"
	}
	switch value {
	case "skill", "agent":
		return value
	default:
		return ""
	}
}

func filterTeamSkillReviewEvents(items []teamSkillReviewEvent, assetType, skillPath, agentSlug string) []teamSkillReviewEvent {
	out := make([]teamSkillReviewEvent, 0, len(items))
	for _, item := range normalizeTeamSkillReviewEvents(items) {
		if assetType != "" && item.AssetType != assetType {
			continue
		}
		if skillPath != "" && item.SkillPath != skillPath {
			continue
		}
		if agentSlug != "" && item.AgentSlug != agentSlug {
			continue
		}
		out = append(out, item)
	}
	return out
}

func normalizeTeamSkillPublications(items []teamSkillPublication) []teamSkillPublication {
	byPath := map[string]teamSkillPublication{}
	for _, item := range items {
		normalized := normalizeTeamSkillPublication(item)
		if normalized.SkillPath == "" || normalized.SkillPath == "/skills" {
			continue
		}
		byPath[normalized.SkillPath] = normalized
	}
	out := make([]teamSkillPublication, 0, len(byPath))
	for _, item := range byPath {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SkillPath < out[j].SkillPath })
	return out
}

func normalizeTeamSkillPublication(item teamSkillPublication) teamSkillPublication {
	item.SkillPath = normalizeAssignedSkillPath(item.SkillPath)
	item.Status = strings.TrimSpace(strings.ToLower(item.Status))
	if item.Status == "" {
		item.Status = "draft"
	}
	switch item.Status {
	case "draft", "published", "archived":
	default:
		item.Status = "draft"
	}
	item.Visibility = strings.TrimSpace(strings.ToLower(item.Visibility))
	if item.Visibility == "" {
		item.Visibility = "private"
	}
	switch item.Visibility {
	case "private", "team":
	default:
		item.Visibility = "private"
	}
	item.Note = strings.TrimSpace(item.Note)
	item.ReviewStatus = normalizeTeamReviewStatus(item.ReviewStatus)
	item.ReviewNote = strings.TrimSpace(item.ReviewNote)
	item.ReviewRequestedBy = strings.TrimSpace(item.ReviewRequestedBy)
	item.ReviewRequestedByRole = strings.TrimSpace(strings.ToLower(item.ReviewRequestedByRole))
	item.ReviewedBy = strings.TrimSpace(item.ReviewedBy)
	item.ReviewedByRole = strings.TrimSpace(strings.ToLower(item.ReviewedByRole))
	return item
}

func normalizeTeamReviewStatus(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "requested", "approved", "changes_requested":
		return value
	default:
		return ""
	}
}

func upsertTeamSkillPublication(doc teamSkillPublicationsDocument, item teamSkillPublication, now string) teamSkillPublicationsDocument {
	doc.Version = teamSkillPublicationsVersion
	doc.UpdatedAt = now
	item = normalizeTeamSkillPublication(item)
	item.UpdatedAt = now
	if item.Status == "published" && item.PublishedAt == "" {
		item.PublishedAt = now
	}
	if item.Status == "archived" && item.ArchivedAt == "" {
		item.ArchivedAt = now
	}
	next := make([]teamSkillPublication, 0, len(doc.Publications)+1)
	replaced := false
	for _, current := range normalizeTeamSkillPublications(doc.Publications) {
		if current.SkillPath == item.SkillPath {
			if item.PublishedAt == "" {
				item.PublishedAt = current.PublishedAt
			}
			if item.ArchivedAt == "" {
				item.ArchivedAt = current.ArchivedAt
			}
			if item.ReviewStatus == "" {
				item.ReviewStatus = current.ReviewStatus
			}
			if item.ReviewNote == "" {
				item.ReviewNote = current.ReviewNote
			}
			if item.ReviewRequestedAt == "" {
				item.ReviewRequestedAt = current.ReviewRequestedAt
			}
			if item.ReviewRequestedBy == "" {
				item.ReviewRequestedBy = current.ReviewRequestedBy
			}
			if item.ReviewRequestedByRole == "" {
				item.ReviewRequestedByRole = current.ReviewRequestedByRole
			}
			if item.ReviewedAt == "" {
				item.ReviewedAt = current.ReviewedAt
			}
			if item.ReviewedBy == "" {
				item.ReviewedBy = current.ReviewedBy
			}
			if item.ReviewedByRole == "" {
				item.ReviewedByRole = current.ReviewedByRole
			}
			next = append(next, item)
			replaced = true
			continue
		}
		next = append(next, current)
	}
	if !replaced {
		next = append(next, item)
	}
	doc.Publications = normalizeTeamSkillPublications(next)
	return doc
}

func teamSkillPublicationNeedsAdmin(item teamSkillPublication) bool {
	return item.Status == "published" || item.Status == "archived" || item.Visibility == "team"
}

func teamSkillPublicationAction(item teamSkillPublication) string {
	item = normalizeTeamSkillPublication(item)
	switch item.Status {
	case "published":
		return "publish"
	case "archived":
		return "archive"
	default:
		return "move_to_draft"
	}
}

func teamSkillPublicationLookup(doc teamSkillPublicationsDocument) map[string]teamSkillPublication {
	out := map[string]teamSkillPublication{}
	for _, item := range normalizeTeamSkillPublications(doc.Publications) {
		out[item.SkillPath] = item
	}
	return out
}

func (s *Server) teamSkillPublicationsForSkills(doc teamSkillPublicationsDocument, skills []models.SkillSummary, team *models.Team) []teamSkillPublication {
	lookup := teamSkillPublicationLookup(doc)
	out := make([]teamSkillPublication, 0, len(skills))
	for _, skill := range skills {
		path := normalizeAssignedSkillPath(firstNonEmpty(skill.BundlePath, skill.Path))
		if path == "" || path == "/skills" {
			continue
		}
		item, ok := lookup[path]
		if !ok {
			item = teamSkillPublication{
				SkillPath:  path,
				Status:     "published",
				Visibility: "team",
				Implicit:   true,
			}
		}
		if team != nil && !team.CanManageMembers && !teamSkillPublicationVisible(item) {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SkillPath < out[j].SkillPath })
	return out
}

func teamSkillPublicationVisible(item teamSkillPublication) bool {
	item = normalizeTeamSkillPublication(item)
	return item.Status == "published" && item.Visibility == "team"
}

func filterTeamSkillSummariesForVisibility(skills []models.SkillSummary, doc teamSkillPublicationsDocument, team *models.Team) []models.SkillSummary {
	if team == nil || team.CanManageMembers {
		return skills
	}
	lookup := teamSkillPublicationLookup(doc)
	out := make([]models.SkillSummary, 0, len(skills))
	for _, skill := range skills {
		skillPath := normalizeAssignedSkillPath(firstNonEmpty(skill.BundlePath, skill.Path))
		if skillPath == "" || skillPath == "/skills" {
			out = append(out, skill)
			continue
		}
		item, ok := lookup[skillPath]
		if !ok || teamSkillPublicationVisible(item) {
			out = append(out, skill)
		}
	}
	return out
}

func filterTeamFileEntriesForVisibility(entries []models.FileTreeEntry, doc teamSkillPublicationsDocument, team *models.Team) []models.FileTreeEntry {
	if team == nil || team.CanManageMembers {
		return entries
	}
	lookup := teamSkillPublicationLookup(doc)
	out := make([]models.FileTreeEntry, 0, len(entries))
	for _, entry := range entries {
		bundlePath, ok := teamSkillBundlePathForTreePath(entry.Path)
		if !ok {
			out = append(out, entry)
			continue
		}
		item, exists := lookup[bundlePath]
		if !exists || teamSkillPublicationVisible(item) {
			out = append(out, entry)
		}
	}
	return out
}

func filterTeamFileNodesForVisibility(nodes []*FileNode, doc teamSkillPublicationsDocument, team *models.Team) []*FileNode {
	if team == nil || team.CanManageMembers || len(nodes) == 0 {
		return nodes
	}
	lookup := teamSkillPublicationLookup(doc)
	out := make([]*FileNode, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		bundlePath, ok := teamSkillBundlePathForTreePath(node.Path)
		if !ok {
			out = append(out, node)
			continue
		}
		item, exists := lookup[bundlePath]
		if !exists || teamSkillPublicationVisible(item) {
			out = append(out, node)
		}
	}
	return out
}

func teamSkillBundlePathForTreePath(value string) (string, bool) {
	clean := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if clean == "" {
		return "", false
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	clean = pathpkg.Clean(clean)
	if clean == "/skills" {
		return "", false
	}
	if !strings.HasPrefix(clean, "/skills/") {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(clean, "/skills/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", false
	}
	return "/skills/" + parts[0], true
}

func teamSkillTreePathVisible(rawPath string, doc teamSkillPublicationsDocument, team *models.Team) bool {
	if team == nil || team.CanManageMembers {
		return true
	}
	bundlePath, ok := teamSkillBundlePathForTreePath(rawPath)
	if !ok {
		return true
	}
	item, exists := teamSkillPublicationLookup(doc)[bundlePath]
	return !exists || teamSkillPublicationVisible(item)
}

func (s *Server) teamSkillPathReadableByRole(ctx context.Context, team *models.Team, rawPath string) (bool, error) {
	if team == nil || team.CanManageMembers {
		return true, nil
	}
	doc, err := s.readTeamSkillPublications(ctx, team.HubUserID)
	if err != nil {
		return false, err
	}
	return teamSkillTreePathVisible(rawPath, doc, team), nil
}

func (s *Server) readTeamSkillSubscriptions(ctx context.Context, userID uuid.UUID) (teamSkillSubscriptionsDocument, error) {
	entry, err := s.FileTreeService.Read(ctx, userID, teamSkillSubscriptionsPath, models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			return teamSkillSubscriptionsDocument{Version: teamSkillSubscriptionsVersion, Subscriptions: []teamSkillSubscription{}}, nil
		}
		return teamSkillSubscriptionsDocument{}, err
	}
	var doc teamSkillSubscriptionsDocument
	if err := json.Unmarshal([]byte(entry.Content), &doc); err != nil {
		return teamSkillSubscriptionsDocument{}, err
	}
	doc.Version = teamSkillSubscriptionsVersion
	doc.Subscriptions = normalizeTeamSkillSubscriptions(doc.Subscriptions)
	return doc, nil
}

func (s *Server) writeTeamSkillSubscriptions(ctx context.Context, userID uuid.UUID, doc teamSkillSubscriptionsDocument) error {
	doc.Version = teamSkillSubscriptionsVersion
	doc.Subscriptions = normalizeTeamSkillSubscriptions(doc.Subscriptions)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	_, err = s.FileTreeService.WriteEntry(ctx, userID, teamSkillSubscriptionsPath, string(append(data, '\n')), "application/json", models.FileTreeWriteOptions{
		Kind: "team_skill_subscriptions",
		Metadata: map[string]interface{}{
			"source":       "team-skill-subscription",
			"capture_mode": "team-skill-subscription",
		},
		MinTrustLevel: models.TrustLevelWork,
	})
	return err
}

func (s *Server) upsertTeamSkillSubscription(ctx context.Context, userID uuid.UUID, team *models.Team, sourcePath, targetPath string, files []localSkillFile, copiedAt string) error {
	if team == nil {
		return nil
	}
	doc, err := s.readTeamSkillSubscriptions(ctx, userID)
	if err != nil {
		return err
	}
	sourcePath = normalizeAssignedSkillPath(sourcePath)
	targetPath = normalizeAssignedSkillPath(targetPath)
	now := copiedAt
	if now == "" {
		now = time.Now().UTC().Format(time.RFC3339)
	}
	var bytesCopied int64
	for _, file := range files {
		bytesCopied += int64(len(file.Data))
	}
	item := teamSkillSubscription{
		TeamID:            team.ID.String(),
		TeamSlug:          team.Slug,
		TeamName:          team.Name,
		SourcePath:        sourcePath,
		TargetPath:        targetPath,
		SourceFingerprint: localSkillFilesFingerprint(files),
		Files:             len(files),
		Bytes:             bytesCopied,
		UpdatedAt:         now,
		CheckedAt:         now,
	}
	doc.UpdatedAt = now
	doc.Subscriptions = upsertTeamSkillSubscriptionItem(doc.Subscriptions, item, now)
	return s.writeTeamSkillSubscriptions(ctx, userID, doc)
}

func normalizeTeamSkillSubscriptions(items []teamSkillSubscription) []teamSkillSubscription {
	byKey := map[string]teamSkillSubscription{}
	for _, item := range items {
		item.TeamID = strings.TrimSpace(item.TeamID)
		item.TeamSlug = strings.TrimSpace(item.TeamSlug)
		item.TeamName = strings.TrimSpace(item.TeamName)
		item.SourcePath = normalizeAssignedSkillPath(item.SourcePath)
		item.TargetPath = normalizeAssignedSkillPath(item.TargetPath)
		if item.TeamID == "" || item.SourcePath == "" || item.SourcePath == "/skills" {
			continue
		}
		byKey[item.TeamID+"\x00"+item.SourcePath] = item
	}
	out := make([]teamSkillSubscription, 0, len(byKey))
	for _, item := range byKey {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TeamSlug == out[j].TeamSlug {
			return out[i].SourcePath < out[j].SourcePath
		}
		return out[i].TeamSlug < out[j].TeamSlug
	})
	return out
}

func upsertTeamSkillSubscriptionItem(items []teamSkillSubscription, item teamSkillSubscription, now string) []teamSkillSubscription {
	item = normalizeTeamSkillSubscriptions([]teamSkillSubscription{item})[0]
	next := make([]teamSkillSubscription, 0, len(items)+1)
	replaced := false
	for _, current := range normalizeTeamSkillSubscriptions(items) {
		if current.TeamID == item.TeamID && current.SourcePath == item.SourcePath {
			if current.InstalledAt != "" {
				item.InstalledAt = current.InstalledAt
			}
			if item.InstalledAt == "" {
				item.InstalledAt = now
			}
			next = append(next, item)
			replaced = true
			continue
		}
		next = append(next, current)
	}
	if !replaced {
		item.InstalledAt = now
		next = append(next, item)
	}
	return normalizeTeamSkillSubscriptions(next)
}

func localSkillFilesFingerprint(files []localSkillFile) string {
	if len(files) == 0 {
		return ""
	}
	sorted := append([]localSkillFile{}, files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].RelPath < sorted[j].RelPath })
	hash := sha256.New()
	for _, file := range sorted {
		hash.Write([]byte(file.RelPath))
		hash.Write([]byte{0})
		hash.Write(file.Data)
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (s *Server) resolveTeamForSubscription(ctx context.Context, userID uuid.UUID, item teamSkillSubscription) (*models.Team, error) {
	if s.TeamService == nil {
		return nil, errors.New("team service not configured")
	}
	if teamID, err := uuid.Parse(strings.TrimSpace(item.TeamID)); err == nil {
		return s.TeamService.GetForUser(ctx, userID, teamID)
	}
	if item.TeamSlug != "" {
		return s.TeamService.GetBySlugForUser(ctx, userID, item.TeamSlug)
	}
	return nil, errors.New("team is not available")
}

func (s *Server) buildTeamSkillSubscriptionReport(ctx context.Context, team *models.Team, checkedAt string) (teamSkillSubscriptionReportResponse, error) {
	if team == nil {
		return teamSkillSubscriptionReportResponse{}, errors.New("team is required")
	}
	if checkedAt == "" {
		checkedAt = time.Now().UTC().Format(time.RFC3339)
	}
	members, err := s.TeamService.ListMembers(ctx, team.ID)
	if err != nil {
		return teamSkillSubscriptionReportResponse{}, err
	}
	memberSubscriptions := make(map[string]map[string]teamSkillSubscription, len(members))
	for _, member := range members {
		doc, err := s.readTeamSkillSubscriptions(ctx, member.UserID)
		if err != nil {
			return teamSkillSubscriptionReportResponse{}, err
		}
		byPath := map[string]teamSkillSubscription{}
		for _, item := range doc.Subscriptions {
			if item.TeamID == team.ID.String() {
				byPath[item.SourcePath] = item
			}
		}
		memberSubscriptions[member.UserID.String()] = byPath
	}
	skills, err := s.listSkills(ctx, team.HubUserID, models.TrustLevelFull)
	if err != nil {
		return teamSkillSubscriptionReportResponse{}, err
	}
	publications, err := s.readTeamSkillPublications(ctx, team.HubUserID)
	if err != nil {
		return teamSkillSubscriptionReportResponse{}, err
	}
	publicationLookup := teamSkillPublicationLookup(publications)
	reports := make([]teamSkillSubscriptionSkillReport, 0, len(skills))
	for _, skill := range skills {
		skillPath := normalizeAssignedSkillPath(firstNonEmpty(skill.BundlePath, skill.Path))
		if skillPath == "" || skillPath == "/skills" {
			continue
		}
		publication, ok := publicationLookup[skillPath]
		if !ok {
			publication = teamSkillPublication{SkillPath: skillPath, Status: "published", Visibility: "team", Implicit: true}
		}
		files, err := s.collectLocalSkillFiles(ctx, team.HubUserID, skillPath)
		sourceMissing := false
		sourceFingerprint := ""
		if err != nil {
			if !errors.Is(err, services.ErrEntryNotFound) {
				return teamSkillSubscriptionReportResponse{}, err
			}
			sourceMissing = true
		} else {
			sourceFingerprint = localSkillFilesFingerprint(files)
		}
		report := teamSkillSubscriptionSkillReport{
			SkillPath:         skillPath,
			Status:            publication.Status,
			Visibility:        publication.Visibility,
			SourceFingerprint: sourceFingerprint,
			SourceMissing:     sourceMissing,
			Members:           make([]teamSkillSubscriptionMemberReport, 0, len(members)),
		}
		for _, member := range members {
			memberReport := teamSkillSubscriptionMemberReport{
				UserID:                   member.UserID.String(),
				UserSlug:                 member.UserSlug,
				DisplayName:              member.DisplayName,
				Role:                     member.Role,
				Status:                   "not_installed",
				SourceCurrentFingerprint: sourceFingerprint,
				CheckedAt:                checkedAt,
				SourceMissing:            sourceMissing,
			}
			if sub, ok := memberSubscriptions[member.UserID.String()][skillPath]; ok {
				memberReport.TargetPath = sub.TargetPath
				memberReport.SourceFingerprint = sub.SourceFingerprint
				memberReport.Files = sub.Files
				memberReport.Bytes = sub.Bytes
				memberReport.InstalledAt = sub.InstalledAt
				memberReport.UpdatedAt = sub.UpdatedAt
				memberReport.Status = "installed"
				if sourceMissing {
					memberReport.Status = "source_missing"
					memberReport.SourceMissing = true
					report.SourceMissingCount++
				} else if sub.SourceFingerprint != "" && sourceFingerprint != "" && sub.SourceFingerprint != sourceFingerprint {
					memberReport.Status = "update_available"
					memberReport.UpdateAvailable = true
					report.UpdateAvailableCount++
				} else {
					report.InstalledCount++
				}
			} else {
				report.NotInstalledCount++
			}
			report.Members = append(report.Members, memberReport)
		}
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].SkillPath < reports[j].SkillPath })
	return teamSkillSubscriptionReportResponse{
		Version:     teamSkillSubscriptionsVersion,
		Team:        team,
		GeneratedAt: checkedAt,
		Skills:      reports,
	}, nil
}

func teamSkillNotificationsFromReport(team *models.Team, report teamSkillSubscriptionReportResponse, now string) []teamSkillUpdateNotification {
	notifications := make([]teamSkillUpdateNotification, 0)
	for _, skill := range report.Skills {
		for _, member := range skill.Members {
			if member.Status != "update_available" && member.Status != "source_missing" {
				continue
			}
			kind := "team_skill_update_available"
			message := "Team skill has a newer version for this member."
			if member.Status == "source_missing" {
				kind = "team_skill_source_missing"
				message = "Installed team skill source is missing from the team library."
			}
			item := teamSkillUpdateNotification{
				ID:                   uuid.NewString(),
				Kind:                 kind,
				TeamID:               team.ID.String(),
				TeamSlug:             team.Slug,
				SkillPath:            skill.SkillPath,
				UserID:               member.UserID,
				UserSlug:             member.UserSlug,
				DisplayName:          member.DisplayName,
				MemberRole:           member.Role,
				Status:               member.Status,
				Message:              message,
				SourceFingerprint:    member.SourceCurrentFingerprint,
				InstalledFingerprint: member.SourceFingerprint,
				CreatedAt:            now,
			}
			notifications = append(notifications, item)
		}
	}
	sort.Slice(notifications, func(i, j int) bool {
		if notifications[i].SkillPath == notifications[j].SkillPath {
			return notifications[i].UserSlug < notifications[j].UserSlug
		}
		return notifications[i].SkillPath < notifications[j].SkillPath
	})
	return notifications
}

func (s *Server) readTeamSkillUpdateNotifications(ctx context.Context, teamHubUserID uuid.UUID) (teamSkillUpdateNotificationsDocument, error) {
	entry, err := s.FileTreeService.Read(ctx, teamHubUserID, teamSkillNotificationsPath, models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			return teamSkillUpdateNotificationsDocument{Version: teamSkillNotificationsVersion, Notifications: []teamSkillUpdateNotification{}}, nil
		}
		return teamSkillUpdateNotificationsDocument{}, err
	}
	var doc teamSkillUpdateNotificationsDocument
	if err := json.Unmarshal([]byte(entry.Content), &doc); err != nil {
		return teamSkillUpdateNotificationsDocument{}, err
	}
	doc.Version = teamSkillNotificationsVersion
	doc.Notifications = normalizeTeamSkillUpdateNotifications(doc.Notifications)
	return doc, nil
}

func (s *Server) writeTeamSkillUpdateNotifications(ctx context.Context, teamHubUserID uuid.UUID, doc teamSkillUpdateNotificationsDocument) error {
	doc.Version = teamSkillNotificationsVersion
	doc.Notifications = normalizeTeamSkillUpdateNotifications(doc.Notifications)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	_, err = s.FileTreeService.WriteEntry(ctx, teamHubUserID, teamSkillNotificationsPath, string(append(data, '\n')), "application/json", models.FileTreeWriteOptions{
		Kind: "team_skill_update_notifications",
		Metadata: map[string]interface{}{
			"source":       "team-skill-update-check",
			"capture_mode": "team-skill-update-check",
		},
		MinTrustLevel: models.TrustLevelWork,
	})
	return err
}

func normalizeTeamSkillUpdateNotifications(items []teamSkillUpdateNotification) []teamSkillUpdateNotification {
	out := make([]teamSkillUpdateNotification, 0, len(items))
	for _, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		item.Kind = strings.TrimSpace(strings.ToLower(item.Kind))
		item.TeamID = strings.TrimSpace(item.TeamID)
		item.TeamSlug = strings.TrimSpace(item.TeamSlug)
		item.SkillPath = normalizeAssignedSkillPath(item.SkillPath)
		item.UserID = strings.TrimSpace(item.UserID)
		item.UserSlug = strings.TrimSpace(item.UserSlug)
		item.DisplayName = strings.TrimSpace(item.DisplayName)
		item.MemberRole = strings.TrimSpace(strings.ToLower(item.MemberRole))
		item.Status = strings.TrimSpace(strings.ToLower(item.Status))
		item.Message = strings.TrimSpace(item.Message)
		item.SourceFingerprint = strings.TrimSpace(item.SourceFingerprint)
		item.InstalledFingerprint = strings.TrimSpace(item.InstalledFingerprint)
		item.CreatedAt = strings.TrimSpace(item.CreatedAt)
		if item.ID == "" {
			item.ID = uuid.NewString()
		}
		if item.CreatedAt == "" {
			item.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if item.SkillPath == "" || item.UserID == "" || item.Status == "" {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

type teamAgentAsset struct {
	Version               string   `json:"version"`
	Slug                  string   `json:"slug"`
	Name                  string   `json:"name"`
	Description           string   `json:"description,omitempty"`
	Instructions          string   `json:"instructions,omitempty"`
	Status                string   `json:"status"`
	Visibility            string   `json:"visibility"`
	DefaultSkillPaths     []string `json:"default_skill_paths,omitempty"`
	TargetAgents          []string `json:"target_agents,omitempty"`
	Model                 string   `json:"model,omitempty"`
	Permissions           []string `json:"permissions,omitempty"`
	ApprovalRequired      []string `json:"approval_required,omitempty"`
	Maintainer            string   `json:"maintainer,omitempty"`
	ReviewStatus          string   `json:"review_status,omitempty"`
	ReviewNote            string   `json:"review_note,omitempty"`
	ReviewRequestedAt     string   `json:"review_requested_at,omitempty"`
	ReviewRequestedBy     string   `json:"review_requested_by,omitempty"`
	ReviewRequestedByRole string   `json:"review_requested_by_role,omitempty"`
	ReviewedAt            string   `json:"reviewed_at,omitempty"`
	ReviewedBy            string   `json:"reviewed_by,omitempty"`
	ReviewedByRole        string   `json:"reviewed_by_role,omitempty"`
	SourceTeamID          string   `json:"source_team_id,omitempty"`
	SourceTeamSlug        string   `json:"source_team_slug,omitempty"`
	InstalledFromTeamID   string   `json:"installed_from_team_id,omitempty"`
	CreatedAt             string   `json:"created_at,omitempty"`
	UpdatedAt             string   `json:"updated_at,omitempty"`
	PublishedAt           string   `json:"published_at,omitempty"`
	ArchivedAt            string   `json:"archived_at,omitempty"`
	Path                  string   `json:"path,omitempty"`
	ReadmePath            string   `json:"readme_path,omitempty"`
}

type teamAgentsResponse struct {
	Version string           `json:"version"`
	Team    *models.Team     `json:"team,omitempty"`
	Agents  []teamAgentAsset `json:"agents"`
}

type teamAgentSaveRequest struct {
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Description       string   `json:"description,omitempty"`
	Instructions      string   `json:"instructions,omitempty"`
	Status            string   `json:"status,omitempty"`
	Visibility        string   `json:"visibility,omitempty"`
	DefaultSkillPaths []string `json:"default_skill_paths,omitempty"`
	TargetAgents      []string `json:"target_agents,omitempty"`
	Model             string   `json:"model,omitempty"`
	Permissions       []string `json:"permissions,omitempty"`
	ApprovalRequired  []string `json:"approval_required,omitempty"`
	Maintainer        string   `json:"maintainer,omitempty"`
}

const teamAgentAssetVersion = "vola.team-agent/v1"

func (s *Server) handleTeamAgentsList(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	agents, err := s.listTeamAgents(r.Context(), team)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, teamAgentsResponse{Version: teamAgentAssetVersion, Team: team, Agents: agents})
}

func (s *Server) handleTeamAgentSave(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	if !team.CanWrite {
		respondForbidden(w, "this team role cannot write agent recipes")
		return
	}
	var req teamAgentSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	asset := normalizeTeamAgentAsset(teamAgentAsset{
		Slug:              req.Slug,
		Name:              req.Name,
		Description:       req.Description,
		Instructions:      req.Instructions,
		Status:            req.Status,
		Visibility:        req.Visibility,
		DefaultSkillPaths: req.DefaultSkillPaths,
		TargetAgents:      req.TargetAgents,
		Model:             req.Model,
		Permissions:       req.Permissions,
		ApprovalRequired:  req.ApprovalRequired,
		Maintainer:        req.Maintainer,
	})
	if asset.Slug == "" || asset.Name == "" {
		respondValidationError(w, "slug", "agent slug and name are required")
		return
	}
	if teamAgentNeedsAdmin(asset) && !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can publish or archive team agents")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	existing, _ := s.readTeamAgentAsset(r.Context(), team.HubUserID, asset.Slug)
	if existing.CreatedAt != "" {
		asset.CreatedAt = existing.CreatedAt
	}
	if asset.CreatedAt == "" {
		asset.CreatedAt = now
	}
	if asset.PublishedAt == "" && existing.PublishedAt != "" {
		asset.PublishedAt = existing.PublishedAt
	}
	if asset.ArchivedAt == "" && existing.ArchivedAt != "" {
		asset.ArchivedAt = existing.ArchivedAt
	}
	if asset.ReviewStatus == "" {
		asset.ReviewStatus = existing.ReviewStatus
	}
	if asset.ReviewNote == "" {
		asset.ReviewNote = existing.ReviewNote
	}
	if asset.ReviewRequestedAt == "" {
		asset.ReviewRequestedAt = existing.ReviewRequestedAt
	}
	if asset.ReviewRequestedBy == "" {
		asset.ReviewRequestedBy = existing.ReviewRequestedBy
	}
	if asset.ReviewRequestedByRole == "" {
		asset.ReviewRequestedByRole = existing.ReviewRequestedByRole
	}
	if asset.ReviewedAt == "" {
		asset.ReviewedAt = existing.ReviewedAt
	}
	if asset.ReviewedBy == "" {
		asset.ReviewedBy = existing.ReviewedBy
	}
	if asset.ReviewedByRole == "" {
		asset.ReviewedByRole = existing.ReviewedByRole
	}
	if asset.Status == "published" && asset.PublishedAt == "" {
		asset.PublishedAt = now
	}
	if asset.Status == "archived" && asset.ArchivedAt == "" {
		asset.ArchivedAt = now
	}
	asset.UpdatedAt = now
	asset.SourceTeamID = team.ID.String()
	asset.SourceTeamSlug = team.Slug
	if err := s.writeTeamAgentAsset(s.requestSourceContext(r, "team-agent"), team.HubUserID, asset); err != nil {
		respondInternalError(w, err)
		return
	}
	if err := s.appendTeamSkillReviewEvent(s.requestSourceContext(r, "team-agent"), team.HubUserID, teamSkillReviewEvent{
		ID:         uuid.NewString(),
		AssetType:  "agent",
		AgentSlug:  asset.Slug,
		Action:     teamAgentAction(asset),
		Status:     asset.Status,
		Visibility: asset.Visibility,
		Note:       asset.Description,
		ActorID:    userID.String(),
		ActorRole:  team.Role,
		CreatedAt:  now,
	}); err != nil {
		respondInternalError(w, err)
		return
	}
	agents, err := s.listTeamAgents(r.Context(), team)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, teamAgentsResponse{Version: teamAgentAssetVersion, Team: team, Agents: agents})
}

func (s *Server) handleTeamAgentInstall(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	slug := normalizeSlugInput(chi.URLParam(r, "agent"))
	if slug == "" {
		respondValidationError(w, "agent", "agent is required")
		return
	}
	asset, err := s.readTeamAgentAsset(r.Context(), team.HubUserID, slug)
	if err != nil {
		respondNotFound(w, "team agent")
		return
	}
	if !team.CanManageMembers && !teamAgentVisible(asset) {
		respondNotFound(w, "team agent")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	asset.InstalledFromTeamID = team.ID.String()
	asset.SourceTeamID = team.ID.String()
	asset.SourceTeamSlug = team.Slug
	asset.UpdatedAt = now
	asset.Path = personalAgentAssetPath(asset.Slug)
	asset.ReadmePath = personalAgentReadmePath(asset.Slug)
	if err := s.writePersonalAgentAsset(s.requestSourceContext(r, "team-agent-install"), userID, asset); err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKWithLocalGitSync(w, map[string]interface{}{"agent": asset}, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) listTeamAgents(ctx context.Context, team *models.Team) ([]teamAgentAsset, error) {
	snapshot, err := s.FileTreeService.Snapshot(ctx, team.HubUserID, "/team/agents", models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			return []teamAgentAsset{}, nil
		}
		return nil, err
	}
	agents := make([]teamAgentAsset, 0)
	for _, entry := range snapshot.Entries {
		if entry.IsDirectory || entry.DeletedAt != nil || pathpkg.Base(entry.Path) != "agent.vola.json" {
			continue
		}
		var asset teamAgentAsset
		if err := json.Unmarshal([]byte(entry.Content), &asset); err != nil {
			continue
		}
		asset = normalizeTeamAgentAsset(asset)
		asset.Path = entry.Path
		asset.ReadmePath = pathpkg.Join(pathpkg.Dir(entry.Path), "README.md")
		if !team.CanManageMembers && !teamAgentVisible(asset) {
			continue
		}
		agents = append(agents, asset)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Slug < agents[j].Slug })
	return agents, nil
}

func (s *Server) readTeamAgentAsset(ctx context.Context, teamHubUserID uuid.UUID, slug string) (teamAgentAsset, error) {
	entry, err := s.FileTreeService.Read(ctx, teamHubUserID, teamAgentAssetPath(slug), models.TrustLevelFull)
	if err != nil {
		return teamAgentAsset{}, err
	}
	var asset teamAgentAsset
	if err := json.Unmarshal([]byte(entry.Content), &asset); err != nil {
		return teamAgentAsset{}, err
	}
	asset = normalizeTeamAgentAsset(asset)
	asset.Path = entry.Path
	asset.ReadmePath = teamAgentReadmePath(asset.Slug)
	return asset, nil
}

func (s *Server) writeTeamAgentAsset(ctx context.Context, teamHubUserID uuid.UUID, asset teamAgentAsset) error {
	asset = normalizeTeamAgentAsset(asset)
	data, err := json.MarshalIndent(asset, "", "  ")
	if err != nil {
		return err
	}
	if _, err := s.FileTreeService.WriteEntry(ctx, teamHubUserID, teamAgentAssetPath(asset.Slug), string(append(data, '\n')), "application/json", models.FileTreeWriteOptions{
		Kind: "team_agent",
		Metadata: map[string]interface{}{
			"source":       "team-agent",
			"capture_mode": "team-agent",
			"agent_slug":   asset.Slug,
		},
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		return err
	}
	_, err = s.FileTreeService.WriteEntry(ctx, teamHubUserID, teamAgentReadmePath(asset.Slug), renderTeamAgentReadme(asset), "text/markdown", models.FileTreeWriteOptions{
		Kind: "team_agent_readme",
		Metadata: map[string]interface{}{
			"source":       "team-agent",
			"capture_mode": "team-agent",
			"agent_slug":   asset.Slug,
		},
		MinTrustLevel: models.TrustLevelWork,
	})
	return err
}

func (s *Server) writePersonalAgentAsset(ctx context.Context, userID uuid.UUID, asset teamAgentAsset) error {
	asset = normalizeTeamAgentAsset(asset)
	data, err := json.MarshalIndent(asset, "", "  ")
	if err != nil {
		return err
	}
	if _, err := s.FileTreeService.WriteEntry(ctx, userID, personalAgentAssetPath(asset.Slug), string(append(data, '\n')), "application/json", models.FileTreeWriteOptions{
		Kind: "agent_recipe",
		Metadata: map[string]interface{}{
			"source":       "team-agent-install",
			"capture_mode": "agent-recipe",
			"agent_slug":   asset.Slug,
		},
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		return err
	}
	_, err = s.FileTreeService.WriteEntry(ctx, userID, personalAgentReadmePath(asset.Slug), renderTeamAgentReadme(asset), "text/markdown", models.FileTreeWriteOptions{
		Kind: "agent_recipe_readme",
		Metadata: map[string]interface{}{
			"source":       "team-agent-install",
			"capture_mode": "agent-recipe",
			"agent_slug":   asset.Slug,
		},
		MinTrustLevel: models.TrustLevelWork,
	})
	return err
}

func normalizeTeamAgentAsset(asset teamAgentAsset) teamAgentAsset {
	asset.Version = teamAgentAssetVersion
	asset.Slug = normalizeSlugInput(asset.Slug)
	if asset.Slug == "" {
		asset.Slug = normalizeSlugInput(asset.Name)
	}
	asset.Name = strings.TrimSpace(asset.Name)
	asset.Description = strings.TrimSpace(asset.Description)
	asset.Instructions = strings.TrimSpace(asset.Instructions)
	asset.Model = strings.TrimSpace(asset.Model)
	asset.Maintainer = strings.TrimSpace(asset.Maintainer)
	asset.ReviewStatus = normalizeTeamReviewStatus(asset.ReviewStatus)
	asset.ReviewNote = strings.TrimSpace(asset.ReviewNote)
	asset.ReviewRequestedBy = strings.TrimSpace(asset.ReviewRequestedBy)
	asset.ReviewRequestedByRole = strings.TrimSpace(strings.ToLower(asset.ReviewRequestedByRole))
	asset.ReviewedBy = strings.TrimSpace(asset.ReviewedBy)
	asset.ReviewedByRole = strings.TrimSpace(strings.ToLower(asset.ReviewedByRole))
	asset.Status = strings.TrimSpace(strings.ToLower(asset.Status))
	if asset.Status == "" {
		asset.Status = "draft"
	}
	switch asset.Status {
	case "draft", "published", "archived":
	default:
		asset.Status = "draft"
	}
	asset.Visibility = strings.TrimSpace(strings.ToLower(asset.Visibility))
	if asset.Visibility == "" {
		asset.Visibility = "private"
	}
	switch asset.Visibility {
	case "private", "team":
	default:
		asset.Visibility = "private"
	}
	asset.DefaultSkillPaths = normalizeSkillPathList(asset.DefaultSkillPaths)
	asset.TargetAgents = normalizeStringList(asset.TargetAgents)
	asset.Permissions = normalizeStringList(asset.Permissions)
	asset.ApprovalRequired = normalizeStringList(asset.ApprovalRequired)
	return asset
}

func teamAgentNeedsAdmin(asset teamAgentAsset) bool {
	return asset.Status == "published" || asset.Status == "archived" || asset.Visibility == "team"
}

func teamAgentAction(asset teamAgentAsset) string {
	asset = normalizeTeamAgentAsset(asset)
	switch asset.Status {
	case "published":
		return "publish_agent"
	case "archived":
		return "archive_agent"
	default:
		return "save_agent_draft"
	}
}

func teamAgentVisible(asset teamAgentAsset) bool {
	asset = normalizeTeamAgentAsset(asset)
	return asset.Status == "published" && asset.Visibility == "team"
}

func normalizeSkillPathList(items []string) []string {
	seen := map[string]struct{}{}
	for _, item := range items {
		normalized := normalizeAssignedSkillPath(item)
		if normalized == "" || normalized == "/skills" {
			continue
		}
		seen[normalized] = struct{}{}
	}
	return sortedStringSet(seen)
}

func normalizeStringList(items []string) []string {
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		seen[item] = struct{}{}
	}
	return sortedStringSet(seen)
}

func teamAgentAssetPath(slug string) string {
	return "/team/agents/" + normalizeSlugInput(slug) + "/agent.vola.json"
}

func teamAgentReadmePath(slug string) string {
	return "/team/agents/" + normalizeSlugInput(slug) + "/README.md"
}

func personalAgentAssetPath(slug string) string {
	return "/agents/" + normalizeSlugInput(slug) + "/agent.vola.json"
}

func personalAgentReadmePath(slug string) string {
	return "/agents/" + normalizeSlugInput(slug) + "/README.md"
}

func normalizeSlugInput(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		isAllowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAllowed {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if r == '-' || r == '_' || unicode.IsSpace(r) {
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 64 {
		out = strings.TrimRight(out[:64], "-")
	}
	return out
}

func renderTeamAgentReadme(asset teamAgentAsset) string {
	var b strings.Builder
	b.WriteString("# " + firstNonEmpty(asset.Name, asset.Slug) + "\n\n")
	if asset.Description != "" {
		b.WriteString(asset.Description + "\n\n")
	}
	b.WriteString("## Instructions\n\n")
	if asset.Instructions != "" {
		b.WriteString(asset.Instructions + "\n\n")
	} else {
		b.WriteString("Describe how this team agent should behave.\n\n")
	}
	if len(asset.DefaultSkillPaths) > 0 {
		b.WriteString("## Default Skills\n\n")
		for _, skillPath := range asset.DefaultSkillPaths {
			b.WriteString("- " + skillPath + "\n")
		}
		b.WriteString("\n")
	}
	if len(asset.TargetAgents) > 0 {
		b.WriteString("## Target Agent Runtimes\n\n")
		for _, target := range asset.TargetAgents {
			b.WriteString("- " + target + "\n")
		}
		b.WriteString("\n")
	}
	if asset.Model != "" {
		b.WriteString("## Model\n\n" + asset.Model + "\n\n")
	}
	if len(asset.Permissions) > 0 || len(asset.ApprovalRequired) > 0 {
		b.WriteString("## Permissions\n\n")
		for _, permission := range asset.Permissions {
			b.WriteString("- " + permission + "\n")
		}
		for _, approval := range asset.ApprovalRequired {
			b.WriteString("- Requires approval: " + approval + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Status\n\n")
	b.WriteString("- Status: " + asset.Status + "\n")
	b.WriteString("- Visibility: " + asset.Visibility + "\n")
	if asset.Maintainer != "" {
		b.WriteString("- Maintainer: " + asset.Maintainer + "\n")
	}
	return b.String()
}
