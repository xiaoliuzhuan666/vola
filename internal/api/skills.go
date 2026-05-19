package api

import "net/http"

func (s *Server) handleSkillsList(w http.ResponseWriter, r *http.Request) {
	if s.FileTreeService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "file tree service not configured")
		return
	}

	target, ok := s.resolveScopedHubTarget(w, r, "", false)
	if !ok {
		return
	}

	skills, err := s.listSkills(r.Context(), target.UserID, trustLevelFromCtx(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondOK(w, map[string]interface{}{
		"scope":  target.Scope,
		"team":   target.Team,
		"skills": skills,
	})
}
