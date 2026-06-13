package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	pathpkg "path"
	"sort"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const teamMcpAssetVersion = "vola.team-mcp/v1"

type teamMcpAsset struct {
	Version               string            `json:"version"`
	Slug                  string            `json:"slug"`
	Name                  string            `json:"name"`
	Description           string            `json:"description,omitempty"`
	Transport             string            `json:"transport"` // "stdio" or "http"
	Command               string            `json:"command,omitempty"`
	Args                  []string          `json:"args,omitempty"`
	Env                   map[string]string `json:"env,omitempty"`
	URL                   string            `json:"url,omitempty"`
	Headers               map[string]string `json:"headers,omitempty"`
	Status                string            `json:"status"`
	Visibility            string            `json:"visibility"`
	ReviewStatus          string            `json:"review_status,omitempty"`
	ReviewNote            string            `json:"review_note,omitempty"`
	ReviewRequestedAt     string            `json:"review_requested_at,omitempty"`
	ReviewRequestedBy     string            `json:"review_requested_by,omitempty"`
	ReviewRequestedByRole string            `json:"review_requested_by_role,omitempty"`
	ReviewedAt            string            `json:"reviewed_at,omitempty"`
	ReviewedBy            string            `json:"reviewed_by,omitempty"`
	ReviewedByRole        string            `json:"reviewed_by_role,omitempty"`
	SourceTeamID          string            `json:"source_team_id,omitempty"`
	SourceTeamSlug        string            `json:"source_team_slug,omitempty"`
	CreatedAt             string            `json:"created_at,omitempty"`
	UpdatedAt             string            `json:"updated_at,omitempty"`
	PublishedAt           string            `json:"published_at,omitempty"`
	ArchivedAt            string            `json:"archived_at,omitempty"`
	Path                  string            `json:"path,omitempty"`
}

type teamMcpsResponse struct {
	Version string         `json:"version"`
	Team    *models.Team   `json:"team,omitempty"`
	Mcps    []teamMcpAsset `json:"mcps"`
}

type teamMcpSaveRequest struct {
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Transport   string            `json:"transport"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Status      string            `json:"status,omitempty"`
	Visibility  string            `json:"visibility,omitempty"`
}

func (s *Server) handleTeamMcpList(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	mcps, err := s.listTeamMcps(r.Context(), team)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, teamMcpsResponse{Version: teamMcpAssetVersion, Team: team, Mcps: mcps})
}

func (s *Server) handleTeamMcpSave(w http.ResponseWriter, r *http.Request) {
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
		respondForbidden(w, "this team role cannot write team MCP configurations")
		return
	}
	var req teamMcpSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	asset := s.normalizeTeamMcpAsset(teamMcpAsset{
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		Transport:   req.Transport,
		Command:     req.Command,
		Args:        req.Args,
		Env:         req.Env,
		URL:         req.URL,
		Headers:     req.Headers,
		Status:      req.Status,
		Visibility:  req.Visibility,
	})
	if asset.Slug == "" || asset.Name == "" {
		respondValidationError(w, "slug", "mcp slug and name are required")
		return
	}
	if (asset.Status == "published" || asset.Visibility == "team") && !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can publish or share team MCP configurations")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	existing, _ := s.readTeamMcpAsset(r.Context(), team.HubUserID, asset.Slug)
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

	if err := s.writeTeamMcpAsset(s.requestSourceContext(r, "team-mcp"), team.HubUserID, asset); err != nil {
		respondInternalError(w, err)
		return
	}
	GlobalBroker.Publish(team.ID.String(), "mcp_update", `{"slug": "`+asset.Slug+`"}`)

	action := "save_mcp_draft"
	if asset.Status == "published" {
		action = "publish_mcp"
	} else if asset.Status == "archived" {
		action = "archive_mcp"
	}

	if err := s.appendTeamSkillReviewEvent(s.requestSourceContext(r, "team-mcp"), team.HubUserID, teamSkillReviewEvent{
		ID:         uuid.NewString(),
		AssetType:  "mcp",
		Action:     action,
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

	mcps, err := s.listTeamMcps(r.Context(), team)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, teamMcpsResponse{Version: teamMcpAssetVersion, Team: team, Mcps: mcps})
}

func (s *Server) handleTeamMcpReviewRequest(w http.ResponseWriter, r *http.Request) {
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
		respondForbidden(w, "this team role cannot request review")
		return
	}
	slug := normalizeSlugInput(chi.URLParam(r, "mcp"))
	if slug == "" {
		respondValidationError(w, "mcp", "mcp slug is required")
		return
	}
	asset, err := s.readTeamMcpAsset(r.Context(), team.HubUserID, slug)
	if err != nil {
		respondNotFound(w, "mcp configuration")
		return
	}
	if asset.Status == "published" || asset.Status == "archived" {
		respondValidationError(w, "mcp", "only draft configurations can be submitted for review")
		return
	}
	var req teamSkillReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	asset.ReviewStatus = "requested"
	asset.ReviewNote = req.Note
	asset.ReviewRequestedAt = now
	asset.ReviewRequestedBy = userID.String()
	asset.ReviewRequestedByRole = team.Role
	asset.UpdatedAt = now

	if err := s.writeTeamMcpAsset(s.requestSourceContext(r, "team-mcp-review"), team.HubUserID, asset); err != nil {
		respondInternalError(w, err)
		return
	}

	if err := s.appendTeamSkillReviewEvent(s.requestSourceContext(r, "team-mcp-review"), team.HubUserID, teamSkillReviewEvent{
		ID:         uuid.NewString(),
		AssetType:  "mcp",
		Action:     "submit_review",
		Status:     asset.Status,
		Visibility: asset.Visibility,
		Note:       req.Note,
		ActorID:    userID.String(),
		ActorRole:  team.Role,
		CreatedAt:  now,
	}); err != nil {
		respondInternalError(w, err)
		return
	}

	mcps, err := s.listTeamMcps(r.Context(), team)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, teamMcpsResponse{Version: teamMcpAssetVersion, Team: team, Mcps: mcps})
}

func (s *Server) handleTeamMcpReviewResolve(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	if !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can resolve review requests")
		return
	}
	slug := normalizeSlugInput(chi.URLParam(r, "mcp"))
	if slug == "" {
		respondValidationError(w, "mcp", "mcp slug is required")
		return
	}
	asset, err := s.readTeamMcpAsset(r.Context(), team.HubUserID, slug)
	if err != nil {
		respondNotFound(w, "mcp configuration")
		return
	}
	var req teamSkillReviewResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	decision := strings.TrimSpace(strings.ToLower(req.Decision))
	if decision != "approve" && decision != "reject" {
		respondValidationError(w, "decision", "decision must be 'approve' or 'reject'")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	action := "review_reject"
	if decision == "approve" {
		action = "review_approve"
		asset.ReviewStatus = "approved"
		asset.Status = "published"
		asset.Visibility = "team"
		asset.PublishedAt = now
	} else {
		asset.ReviewStatus = "rejected"
	}
	asset.ReviewNote = req.Note
	asset.ReviewedAt = now
	asset.ReviewedBy = userID.String()
	asset.ReviewedByRole = team.Role
	asset.UpdatedAt = now

	if err := s.writeTeamMcpAsset(s.requestSourceContext(r, "team-mcp-review"), team.HubUserID, asset); err != nil {
		respondInternalError(w, err)
		return
	}

	if err := s.appendTeamSkillReviewEvent(s.requestSourceContext(r, "team-mcp-review"), team.HubUserID, teamSkillReviewEvent{
		ID:         uuid.NewString(),
		AssetType:  "mcp",
		Action:     action,
		Status:     asset.Status,
		Visibility: asset.Visibility,
		Note:       req.Note,
		ActorID:    userID.String(),
		ActorRole:  team.Role,
		CreatedAt:  now,
	}); err != nil {
		respondInternalError(w, err)
		return
	}

	mcps, err := s.listTeamMcps(r.Context(), team)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, teamMcpsResponse{Version: teamMcpAssetVersion, Team: team, Mcps: mcps})
}

func (s *Server) listTeamMcps(ctx context.Context, team *models.Team) ([]teamMcpAsset, error) {
	snapshot, err := s.FileTreeService.Snapshot(ctx, team.HubUserID, "/team/mcps", models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			return []teamMcpAsset{}, nil
		}
		return nil, err
	}
	mcps := make([]teamMcpAsset, 0)
	for _, entry := range snapshot.Entries {
		if entry.IsDirectory || entry.DeletedAt != nil || pathpkg.Base(entry.Path) != "mcp.vola.json" {
			continue
		}
		var asset teamMcpAsset
		if err := json.Unmarshal([]byte(entry.Content), &asset); err != nil {
			continue
		}
		asset = s.normalizeTeamMcpAsset(asset)
		asset.Path = entry.Path
		if !team.CanManageMembers && !teamMcpVisible(asset) {
			continue
		}
		mcps = append(mcps, asset)
	}
	sort.Slice(mcps, func(i, j int) bool { return mcps[i].Slug < mcps[j].Slug })
	return mcps, nil
}

func (s *Server) readTeamMcpAsset(ctx context.Context, teamHubUserID uuid.UUID, slug string) (teamMcpAsset, error) {
	entry, err := s.FileTreeService.Read(ctx, teamHubUserID, teamMcpAssetPath(slug), models.TrustLevelFull)
	if err != nil {
		return teamMcpAsset{}, err
	}
	var asset teamMcpAsset
	if err := json.Unmarshal([]byte(entry.Content), &asset); err != nil {
		return teamMcpAsset{}, err
	}
	asset = s.normalizeTeamMcpAsset(asset)
	asset.Path = entry.Path
	return asset, nil
}

func (s *Server) writeTeamMcpAsset(ctx context.Context, teamHubUserID uuid.UUID, asset teamMcpAsset) error {
	asset = s.normalizeTeamMcpAsset(asset)
	data, err := json.MarshalIndent(asset, "", "  ")
	if err != nil {
		return err
	}
	if _, err := s.FileTreeService.WriteEntry(ctx, teamHubUserID, teamMcpAssetPath(asset.Slug), string(append(data, '\n')), "application/json", models.FileTreeWriteOptions{
		Kind: "team_mcp",
		Metadata: map[string]interface{}{
			"source":       "team-mcp",
			"capture_mode": "team-mcp",
			"mcp_slug":     asset.Slug,
		},
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		return err
	}
	_, err = s.FileTreeService.WriteEntry(ctx, teamHubUserID, teamMcpReadmePath(asset.Slug), renderTeamMcpReadme(asset), "text/markdown", models.FileTreeWriteOptions{
		Kind: "team_mcp_readme",
		Metadata: map[string]interface{}{
			"source":       "team-mcp",
			"capture_mode": "team-mcp",
			"mcp_slug":     asset.Slug,
		},
		MinTrustLevel: models.TrustLevelWork,
	})
	return err
}

func (s *Server) normalizeTeamMcpAsset(asset teamMcpAsset) teamMcpAsset {
	asset.Version = teamMcpAssetVersion
	asset.Slug = normalizeSlugInput(asset.Slug)
	if asset.Slug == "" {
		asset.Slug = normalizeSlugInput(asset.Name)
	}
	asset.Name = strings.TrimSpace(asset.Name)
	asset.Description = strings.TrimSpace(asset.Description)
	asset.Transport = strings.TrimSpace(strings.ToLower(asset.Transport))
	if asset.Transport == "" {
		asset.Transport = "stdio"
	}
	asset.Command = strings.TrimSpace(asset.Command)
	asset.URL = strings.TrimSpace(asset.URL)
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
	return asset
}

func teamMcpVisible(asset teamMcpAsset) bool {
	return asset.Status == "published" && asset.Visibility == "team"
}

func teamMcpAssetPath(slug string) string {
	return "/team/mcps/" + normalizeSlugInput(slug) + "/mcp.vola.json"
}

func teamMcpReadmePath(slug string) string {
	return "/team/mcps/" + normalizeSlugInput(slug) + "/README.md"
}

func renderTeamMcpReadme(asset teamMcpAsset) string {
	var b strings.Builder
	b.WriteString("# MCP: " + firstNonEmpty(asset.Name, asset.Slug) + "\n\n")
	if asset.Description != "" {
		b.WriteString(asset.Description + "\n\n")
	}
	b.WriteString("## Connection Parameters\n\n")
	b.WriteString("- Transport: " + asset.Transport + "\n")
	if asset.Transport == "stdio" {
		b.WriteString("- Command: `" + asset.Command + "`\n")
		if len(asset.Args) > 0 {
			b.WriteString("- Arguments: ")
			for _, arg := range asset.Args {
				b.WriteString("`" + arg + "` ")
			}
			b.WriteString("\n")
		}
		if len(asset.Env) > 0 {
			b.WriteString("- Environment:\n")
			for k, v := range asset.Env {
				b.WriteString("  - `" + k + "=" + v + "`\n")
			}
		}
	} else if asset.Transport == "http" {
		b.WriteString("- URL: " + asset.URL + "\n")
		if len(asset.Headers) > 0 {
			b.WriteString("- Custom Headers:\n")
			for k, v := range asset.Headers {
				b.WriteString("  - `" + k + ": " + v + "`\n")
			}
		}
	}
	b.WriteString("\n## Status\n\n")
	b.WriteString("- Status: " + asset.Status + "\n")
	b.WriteString("- Visibility: " + asset.Visibility + "\n")
	return b.String()
}
