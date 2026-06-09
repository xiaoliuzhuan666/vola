package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	pathpkg "path"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/agi-bar/vola/internal/skillsarchive"
)

const skillCopyVersion = "vola.skill-copy/v1"

type skillCopyToPersonalRequest struct {
	TeamID     string `json:"team_id"`
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path,omitempty"`
	Overwrite  bool   `json:"overwrite,omitempty"`
}

type skillCopyToPersonalResponse struct {
	Version    string       `json:"version"`
	Applied    bool         `json:"applied"`
	CopiedAt   string       `json:"copied_at"`
	Team       *models.Team `json:"team,omitempty"`
	SourcePath string       `json:"source_path"`
	TargetPath string       `json:"target_path"`
	Files      int          `json:"files"`
	Bytes      int64        `json:"bytes"`
	Overwrite  bool         `json:"overwrite"`
}

func (s *Server) handleSkillCopyToPersonal(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelWork, models.ScopeWriteSkills) {
		return
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return
	}

	userID, _ := userIDFromCtx(r.Context())
	var req skillCopyToPersonalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	teamID := strings.TrimSpace(req.TeamID)
	if teamID == "" {
		respondValidationError(w, "team_id", "team_id is required")
		return
	}
	sourcePath := normalizeAssignedSkillPath(req.SourcePath)
	if sourcePath == "" || sourcePath == "/skills" {
		respondValidationError(w, "source_path", "source_path must point to one skill under /skills")
		return
	}
	targetPath := normalizeAssignedSkillPath(req.TargetPath)
	if targetPath == "" {
		targetPath = sourcePath
	}
	if targetPath == "/skills" {
		respondValidationError(w, "target_path", "target_path must point to one skill under /skills")
		return
	}

	source, ok := s.resolveScopedHubTarget(w, r, teamID, false)
	if !ok {
		return
	}
	if source.Scope != "team" || source.Team == nil {
		respondValidationError(w, "team_id", "team_id must reference a team")
		return
	}
	visible, err := s.teamSkillPathReadableByRole(r.Context(), source.Team, sourcePath)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !visible {
		respondError(w, http.StatusNotFound, ErrCodeNotFound, fmt.Sprintf("%s not found", sourcePath))
		return
	}

	files, err := s.collectLocalSkillFiles(r.Context(), source.UserID, sourcePath)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			respondError(w, http.StatusNotFound, ErrCodeNotFound, fmt.Sprintf("%s not found", sourcePath))
			return
		}
		respondInternalError(w, err)
		return
	}
	if len(files) == 0 {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, fmt.Sprintf("%s has no skill files", sourcePath))
		return
	}

	if !req.Overwrite {
		conflicts := make([]string, 0)
		for _, file := range files {
			targetFilePath := pathpkg.Join(targetPath, file.RelPath)
			if _, readErr := s.FileTreeService.Read(r.Context(), userID, targetFilePath, models.TrustLevelFull); readErr == nil {
				conflicts = append(conflicts, targetFilePath)
				continue
			} else if !errors.Is(readErr, services.ErrEntryNotFound) {
				respondInternalError(w, readErr)
				return
			}
		}
		if len(conflicts) > 0 {
			respondError(w, http.StatusConflict, ErrCodeConflict, "target files already exist: "+strings.Join(conflicts, ", "))
			return
		}
	}

	copiedAt := time.Now().UTC().Format(time.RFC3339)
	writeCtx := s.requestSourceContext(r, "team-copy")
	var bytesCopied int64
	for _, file := range files {
		targetFilePath := pathpkg.Join(targetPath, file.RelPath)
		contentType := strings.TrimSpace(file.ContentType)
		if contentType == "" {
			contentType = skillsarchive.DetectContentType(file.RelPath, file.Data)
		}
		metadata := map[string]interface{}{
			"source":           "team-copy",
			"source_team_id":   source.Team.ID.String(),
			"source_team_slug": source.Team.Slug,
			"source_skill":     sourcePath,
			"source_path":      file.HubPath,
			"copied_at":        copiedAt,
		}
		bytesCopied += int64(len(file.Data))

		if skillsarchive.LooksBinary(file.RelPath, file.Data) {
			_, err = s.FileTreeService.WriteBinaryEntry(writeCtx, userID, targetFilePath, file.Data, contentType, models.FileTreeWriteOptions{
				Kind:          "skill_asset",
				Metadata:      metadata,
				MinTrustLevel: models.TrustLevelWork,
			})
		} else {
			_, err = s.FileTreeService.WriteEntry(writeCtx, userID, targetFilePath, string(file.Data), contentType, models.FileTreeWriteOptions{
				Kind:          "skill_file",
				Metadata:      metadata,
				MinTrustLevel: models.TrustLevelWork,
			})
		}
		if err != nil {
			if errors.Is(err, services.ErrReadOnlyPath) {
				respondForbidden(w, err.Error())
				return
			}
			respondInternalError(w, err)
			return
		}
	}
	if err := s.upsertTeamSkillSubscription(writeCtx, userID, source.Team, sourcePath, targetPath, files, copiedAt); err != nil {
		respondInternalError(w, err)
		return
	}

	respondOKWithLocalGitSync(w, skillCopyToPersonalResponse{
		Version:    skillCopyVersion,
		Applied:    true,
		CopiedAt:   copiedAt,
		Team:       source.Team,
		SourcePath: sourcePath,
		TargetPath: targetPath,
		Files:      len(files),
		Bytes:      bytesCopied,
		Overwrite:  req.Overwrite,
	}, s.syncLocalGitMirror(r.Context(), userID))
}
