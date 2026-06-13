package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	pathpkg "path"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/agi-bar/vola/internal/skillsarchive"
	"github.com/google/uuid"
)

type skillDiffRequest struct {
	TeamID     string `json:"team_id"`
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
}

type skillDiffFileItem struct {
	RelPath       string `json:"rel_path"`
	Status        string `json:"status"` // "added", "modified", "deleted", "unchanged"
	SourceContent string `json:"source_content,omitempty"`
	TargetContent string `json:"target_content,omitempty"`
}

type skillDiffResponse struct {
	Version string              `json:"version"`
	Files   []skillDiffFileItem `json:"files"`
}

type skillRollbackRequest struct {
	TargetPath     string `json:"target_path"`
	BackupFilePath string `json:"backup_file_path"`
}

type skillRollbackResponse struct {
	Version  string `json:"version"`
	Success  bool   `json:"success"`
	Restored int    `json:"restored"`
}

func (s *Server) handleSkillSubscriptionsDiff(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelWork, models.ScopeReadSkills) {
		return
	}
	userID, _ := userIDFromCtx(r.Context())
	var req skillDiffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	teamID := strings.TrimSpace(req.TeamID)
	sourcePath := normalizeAssignedSkillPath(req.SourcePath)
	targetPath := normalizeAssignedSkillPath(req.TargetPath)
	if teamID == "" || sourcePath == "" || targetPath == "" {
		respondValidationError(w, "fields", "team_id, source_path and target_path are required")
		return
	}

	teamUUID, err := uuid.Parse(teamID)
	if err != nil {
		respondValidationError(w, "team_id", "invalid team_id")
		return
	}
	team, err := s.TeamService.GetForUser(r.Context(), userID, teamUUID)
	if err != nil {
		respondNotFound(w, "team")
		return
	}

	teamFiles, err := s.collectLocalSkillFiles(r.Context(), team.HubUserID, sourcePath)
	if err != nil {
		respondError(w, http.StatusNotFound, ErrCodeNotFound, "team skill files not found")
		return
	}

	personalFiles, err := s.collectLocalSkillFiles(r.Context(), userID, targetPath)
	if err != nil && !errors.Is(err, services.ErrEntryNotFound) {
		respondInternalError(w, err)
		return
	}

	teamFilesMap := make(map[string]localSkillFile)
	for _, f := range teamFiles {
		teamFilesMap[f.RelPath] = f
	}

	personalFilesMap := make(map[string]localSkillFile)
	for _, f := range personalFiles {
		personalFilesMap[f.RelPath] = f
	}

	var diffFiles []skillDiffFileItem

	for relPath, tf := range teamFilesMap {
		pf, exists := personalFilesMap[relPath]
		if !exists {
			diffFiles = append(diffFiles, skillDiffFileItem{
				RelPath:       relPath,
				Status:        "added",
				SourceContent: string(tf.Data),
			})
		} else {
			if !bytes.Equal(tf.Data, pf.Data) {
				diffFiles = append(diffFiles, skillDiffFileItem{
					RelPath:       relPath,
					Status:        "modified",
					SourceContent: string(tf.Data),
					TargetContent: string(pf.Data),
				})
			} else {
				diffFiles = append(diffFiles, skillDiffFileItem{
					RelPath: relPath,
					Status:  "unchanged",
				})
			}
		}
	}

	for relPath, pf := range personalFilesMap {
		_, exists := teamFilesMap[relPath]
		if !exists {
			diffFiles = append(diffFiles, skillDiffFileItem{
				RelPath:       relPath,
				Status:        "deleted",
				TargetContent: string(pf.Data),
			})
		}
	}

	respondOK(w, skillDiffResponse{
		Version: "vola.skill-diff/v1",
		Files:   diffFiles,
	})
}

func (s *Server) handleSkillSubscriptionsRollback(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelWork, models.ScopeWriteSkills) {
		return
	}
	userID, _ := userIDFromCtx(r.Context())
	var req skillRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	targetPath := normalizeAssignedSkillPath(req.TargetPath)
	backupPath := strings.TrimSpace(req.BackupFilePath)
	if targetPath == "" || backupPath == "" {
		respondValidationError(w, "fields", "target_path and backup_file_path are required")
		return
	}

	skillName := pathpkg.Base(targetPath)
	expectedPrefix := fmt.Sprintf("/settings/team-skill-backups/%s/", skillName)
	if !strings.HasPrefix(backupPath, expectedPrefix) || !strings.HasSuffix(backupPath, "-backup.zip") {
		respondForbidden(w, "invalid backup path")
		return
	}

	zipBytes, _, err := s.FileTreeService.ReadBinary(r.Context(), userID, backupPath, models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			respondNotFound(w, "backup file")
			return
		}
		respondInternalError(w, err)
		return
	}
	if len(zipBytes) == 0 {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "backup file is empty")
		return
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid zip format")
		return
	}

	currentFiles, _ := s.collectLocalSkillFiles(r.Context(), userID, targetPath)
	for _, f := range currentFiles {
		_ = s.FileTreeService.Delete(r.Context(), userID, pathpkg.Join(targetPath, f.RelPath))
	}

	var restoredCount int
	var restoredFiles []localSkillFile
	writeCtx := s.requestSourceContext(r, "team-rollback")

	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}
		
		// Prevent Zip Slip vulnerability
		cleanedName := pathpkg.Clean(file.Name)
		if strings.HasPrefix(cleanedName, "..") || pathpkg.IsAbs(cleanedName) || strings.Contains(cleanedName, "../") {
			continue
		}
		targetFilePath := pathpkg.Join(targetPath, cleanedName)
		expectedPrefix := strings.TrimSuffix(targetPath, "/") + "/"
		if !strings.HasPrefix(targetFilePath, expectedPrefix) {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			continue
		}

		contentType := skillsarchive.DetectContentType(cleanedName, data)

		metadata := map[string]interface{}{
			"source":      "team-rollback",
			"restored_at": time.Now().UTC().Format(time.RFC3339),
			"backup_path": backupPath,
		}

		if skillsarchive.LooksBinary(cleanedName, data) {
			_, err = s.FileTreeService.WriteBinaryEntry(writeCtx, userID, targetFilePath, data, contentType, models.FileTreeWriteOptions{
				Kind:          "skill_asset",
				Metadata:      metadata,
				MinTrustLevel: models.TrustLevelWork,
			})
		} else {
			_, err = s.FileTreeService.WriteEntry(writeCtx, userID, targetFilePath, string(data), contentType, models.FileTreeWriteOptions{
				Kind:          "skill_file",
				Metadata:      metadata,
				MinTrustLevel: models.TrustLevelWork,
			})
		}

		if err == nil {
			restoredCount++
			restoredFiles = append(restoredFiles, localSkillFile{
				RelPath: cleanedName,
				Data:    data,
			})
		}
	}

	if restoredCount > 0 {
		subDoc, err := s.readTeamSkillSubscriptions(r.Context(), userID)
		if err == nil {
			fingerprint := localSkillFilesFingerprint(restoredFiles)
			updated := false
			for i, sub := range subDoc.Subscriptions {
				if sub.TargetPath == targetPath {
					subDoc.Subscriptions[i].SourceFingerprint = fingerprint
					subDoc.Subscriptions[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
					updated = true
					break
				}
			}
			if updated {
				_ = s.writeTeamSkillSubscriptions(writeCtx, userID, subDoc)
			}
		}
	}

	respondOKWithLocalGitSync(w, skillRollbackResponse{
		Version:  "vola.skill-rollback/v1",
		Success:  true,
		Restored: restoredCount,
	}, s.syncLocalGitMirror(r.Context(), userID))
}
