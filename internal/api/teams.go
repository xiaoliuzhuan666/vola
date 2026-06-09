package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/agi-bar/vola/internal/hubpath"
	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) handleTeamsList(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	if s.TeamService == nil {
		respondNotConfigured(w, "team service")
		return
	}
	teams, err := s.TeamService.ListForUser(r.Context(), userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]any{"teams": teams})
}

func (s *Server) handleTeamsCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	if s.TeamService == nil {
		respondNotConfigured(w, "team service")
		return
	}
	var req models.CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	team, err := s.TeamService.Create(r.Context(), userID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondCreated(w, map[string]any{"team": team})
}

func (s *Server) handleTeamsGet(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	respondOK(w, map[string]any{"team": team})
}

func (s *Server) handleTeamsUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can update this team")
		return
	}
	var req models.UpdateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	updated, err := s.TeamService.Update(r.Context(), userID, team.ID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, map[string]any{"team": updated})
}

func (s *Server) handleTeamMembersList(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	members, err := s.TeamService.ListMembers(r.Context(), team.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]any{"members": members})
}

func (s *Server) handleTeamMembersAdd(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can manage members")
		return
	}
	if s.UserService == nil {
		respondNotConfigured(w, "user service")
		return
	}
	var req models.AddTeamMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	user, err := s.UserService.GetBySlug(r.Context(), strings.TrimSpace(req.UserSlug))
	if err != nil {
		respondNotFound(w, "user")
		return
	}
	member, err := s.TeamService.AddMember(r.Context(), team.ID, user.ID, req.Role)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondCreated(w, map[string]any{"member": member})
}

func (s *Server) handleTeamMemberUpdate(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can manage members")
		return
	}
	memberUserID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "user_id")))
	if err != nil {
		respondValidationError(w, "user_id", "user_id must be a UUID")
		return
	}
	var req models.UpdateTeamMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	member, err := s.TeamService.UpdateMemberRole(r.Context(), team.ID, memberUserID, req.Role)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, map[string]any{"member": member})
}

func (s *Server) handleTeamMemberRemove(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanManageMembers {
		respondForbidden(w, "only team owners and admins can manage members")
		return
	}
	memberUserID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "user_id")))
	if err != nil {
		respondValidationError(w, "user_id", "user_id must be a UUID")
		return
	}
	if err := s.TeamService.RemoveMember(r.Context(), team.ID, memberUserID); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, map[string]string{"status": "removed"})
}

func (s *Server) handleTeamSkillsList(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	skills, err := s.listSkills(r.Context(), team.HubUserID, trustLevelFromCtx(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	doc, err := s.readTeamSkillPublications(r.Context(), team.HubUserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	skills = filterTeamSkillSummariesForVisibility(skills, doc, team)
	respondOK(w, map[string]any{"skills": skills})
}

func (s *Server) handleTeamTreeRead(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	path := chi.URLParam(r, "*")
	doc, err := s.readTeamSkillPublications(r.Context(), team.HubUserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !teamSkillTreePathVisible(path, doc, team) {
		respondNotFound(w, "file")
		return
	}
	node, err := s.readOrListTreePath(r.Context(), team.HubUserID, trustLevelFromCtx(r.Context()), path)
	if err != nil {
		respondNotFound(w, "file")
		return
	}
	if node != nil {
		node.Children = filterTeamFileNodesForVisibility(node.Children, doc, team)
	}
	respondOK(w, node)
}

func (s *Server) handleTeamTreeSnapshot(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	snapshot, err := s.FileTreeService.Snapshot(r.Context(), team.HubUserID, path, trustLevelFromCtx(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	doc, err := s.readTeamSkillPublications(r.Context(), team.HubUserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	nodes := make([]*FileNode, 0, len(snapshot.Entries))
	for _, entry := range filterTeamFileEntriesForVisibility(filterVisibleEntries(snapshot.Entries), doc, team) {
		nodes = append(nodes, fileTreeEntryToNode(s.renderSystemSkillEntry(r.Context(), team.HubUserID, trustLevelFromCtx(r.Context()), &entry)))
	}
	respondOK(w, SnapshotResponse{
		Path:         snapshot.Path,
		Cursor:       snapshot.Cursor,
		RootChecksum: snapshot.RootChecksum,
		Entries:      nodes,
	})
}

func (s *Server) handleTeamTreeWrite(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanWrite {
		respondForbidden(w, "this team role cannot write files")
		return
	}
	path := chi.URLParam(r, "*")
	if path == "" {
		respondValidationError(w, "path", "path is required")
		return
	}
	var req WriteFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	ctx := s.requestSourceContext(r, "team")
	ctx, req.Metadata = applyExplicitSourceHints(ctx, req.Metadata, req.Source, req.SourcePlatform)
	req.Metadata = services.WithSourceMetadata(req.Metadata, services.SourceOrDefault(ctx, "team"))
	req.Metadata["team_id"] = team.ID.String()
	req.Metadata["team_slug"] = team.Slug

	if req.IsDir {
		entry, err := s.FileTreeService.EnsureDirectoryWithMetadata(ctx, team.HubUserID, path, req.Metadata, req.MinTrustLevel)
		if err != nil {
			respondInternalError(w, err)
			return
		}
		respondOK(w, fileTreeEntryToNode(s.renderSystemSkillEntry(r.Context(), team.HubUserID, trustLevelFromCtx(r.Context()), entry)))
		return
	}

	mimeType := req.MimeType
	if mimeType == "" {
		mimeType = "text/plain"
	}
	minTrustLevel := req.MinTrustLevel
	if minTrustLevel <= 0 {
		minTrustLevel = models.TrustLevelCollaborate
	}
	entry, err := s.FileTreeService.WriteEntry(ctx, team.HubUserID, path, req.Content, mimeType, models.FileTreeWriteOptions{
		Metadata:         req.Metadata,
		MinTrustLevel:    minTrustLevel,
		ExpectedVersion:  req.ExpectedVersion,
		ExpectedChecksum: req.ExpectedChecksum,
	})
	if err != nil {
		if errors.Is(err, services.ErrOptimisticLockConflict) {
			respondError(w, http.StatusConflict, ErrCodeConflict, err.Error())
			return
		}
		respondInternalError(w, err)
		return
	}
	respondOK(w, fileTreeEntryToNode(s.renderSystemSkillEntry(r.Context(), team.HubUserID, trustLevelFromCtx(r.Context()), entry)))
}

func (s *Server) handleTeamTreeDelete(w http.ResponseWriter, r *http.Request) {
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanWrite {
		respondForbidden(w, "this team role cannot delete files")
		return
	}
	path := chi.URLParam(r, "*")
	if path == "" {
		respondValidationError(w, "path", "path is required")
		return
	}
	if err := s.FileTreeService.Delete(r.Context(), team.HubUserID, path); err != nil {
		respondNotFound(w, "file")
		return
	}
	respondOK(w, map[string]string{"status": "deleted", "path": hubpath.NormalizePublic(path)})
}

func (s *Server) handleAgentTeamsList(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelCollaborate, models.ScopeReadTree) {
		return
	}
	userID, _ := userIDFromCtx(r.Context())
	if s.TeamService == nil {
		respondNotConfigured(w, "team service")
		return
	}
	teams, err := s.TeamService.ListForUser(r.Context(), userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]any{"teams": teams})
}

func (s *Server) handleAgentTeamSkillsList(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelCollaborate, models.ScopeReadSkills) {
		return
	}
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	skills, err := s.listSkills(r.Context(), team.HubUserID, trustLevelFromCtx(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	doc, err := s.readTeamSkillPublications(r.Context(), team.HubUserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	skills = filterTeamSkillSummariesForVisibility(skills, doc, team)
	respondOK(w, map[string]any{"team": team, "skills": skills})
}

func (s *Server) handleAgentTeamTreeRead(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelCollaborate, models.ScopeReadTree) {
		return
	}
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	path := chi.URLParam(r, "*")
	doc, err := s.readTeamSkillPublications(r.Context(), team.HubUserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !teamSkillTreePathVisible(path, doc, team) {
		respondNotFound(w, "file")
		return
	}
	node, err := s.readOrListTreePath(r.Context(), team.HubUserID, trustLevelFromCtx(r.Context()), path)
	if err != nil {
		respondNotFound(w, "file")
		return
	}
	if node != nil {
		node.Children = filterTeamFileNodesForVisibility(node.Children, doc, team)
	}
	respondOK(w, map[string]any{"team": team, "node": node})
}

func (s *Server) handleAgentTeamTreeWrite(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelWork, models.ScopeWriteTree) {
		return
	}
	team, ok := s.currentTeam(w, r)
	if !ok {
		return
	}
	if !team.CanWrite {
		respondForbidden(w, "this team role cannot write files")
		return
	}
	s.handleTeamTreeWrite(w, r)
}

func (s *Server) currentTeam(w http.ResponseWriter, r *http.Request) (*models.Team, bool) {
	if s.TeamService == nil {
		respondNotConfigured(w, "team service")
		return nil, false
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return nil, false
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return nil, false
	}
	identifier := strings.TrimSpace(chi.URLParam(r, "team"))
	if identifier == "" {
		respondValidationError(w, "team", "team is required")
		return nil, false
	}
	var (
		team *models.Team
		err  error
	)
	if teamID, parseErr := uuid.Parse(identifier); parseErr == nil {
		team, err = s.TeamService.GetForUser(r.Context(), userID, teamID)
	} else {
		team, err = s.TeamService.GetBySlugForUser(r.Context(), userID, identifier)
	}
	if err != nil {
		respondNotFound(w, "team")
		return nil, false
	}
	return team, true
}
