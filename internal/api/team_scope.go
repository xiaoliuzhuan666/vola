package api

import (
	"net/http"
	"strings"

	"github.com/agi-bar/neudrive/internal/models"
	"github.com/google/uuid"
)

type scopedHubTarget struct {
	Scope  string       `json:"scope"`
	UserID uuid.UUID    `json:"-"`
	Team   *models.Team `json:"team,omitempty"`
}

func (s *Server) resolveScopedHubTarget(w http.ResponseWriter, r *http.Request, explicitTeam string, requireWrite bool) (scopedHubTarget, bool) {
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return scopedHubTarget{}, false
	}

	teamIdentifier := strings.TrimSpace(explicitTeam)
	if teamIdentifier == "" {
		teamIdentifier = strings.TrimSpace(r.URL.Query().Get("team_id"))
	}
	if teamIdentifier == "" {
		teamIdentifier = strings.TrimSpace(r.URL.Query().Get("team"))
	}
	if teamIdentifier == "" {
		return scopedHubTarget{Scope: "personal", UserID: userID}, true
	}

	if s.TeamService == nil {
		respondNotConfigured(w, "team service")
		return scopedHubTarget{}, false
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return scopedHubTarget{}, false
	}

	var (
		team *models.Team
		err  error
	)
	if teamID, parseErr := uuid.Parse(teamIdentifier); parseErr == nil {
		team, err = s.TeamService.GetForUser(r.Context(), userID, teamID)
	} else {
		team, err = s.TeamService.GetBySlugForUser(r.Context(), userID, teamIdentifier)
	}
	if err != nil {
		respondNotFound(w, "team")
		return scopedHubTarget{}, false
	}
	if requireWrite && !team.CanWrite {
		respondForbidden(w, "this team role cannot write files")
		return scopedHubTarget{}, false
	}

	return scopedHubTarget{Scope: "team", UserID: team.HubUserID, Team: team}, true
}
