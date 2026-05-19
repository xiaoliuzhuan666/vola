package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agi-bar/neudrive/internal/models"
	"github.com/agi-bar/neudrive/internal/services"
	"github.com/google/uuid"
)

const (
	localSkillSyncVersion     = "neudrive.local-skill-sync/v1"
	localSkillManagedFileName = ".neudrive-managed.json"
)

type localSkillSyncRequest struct {
	AgentIDs    []string          `json:"agent_ids,omitempty"`
	TargetRoots map[string]string `json:"target_roots,omitempty"`
	TeamID      string            `json:"team_id,omitempty"`
}

type localSkillSyncExportRequest struct {
	AgentID string `json:"agent_id"`
	TeamID  string `json:"team_id,omitempty"`
}

type localSkillSyncResponse struct {
	Version   string                    `json:"version"`
	Scope     string                    `json:"scope"`
	Team      *models.Team              `json:"team,omitempty"`
	Mode      string                    `json:"mode"`
	Applied   bool                      `json:"applied"`
	Cleanup   bool                      `json:"cleanup"`
	UpdatedAt string                    `json:"updated_at"`
	Agents    []localSkillSyncAgentPlan `json:"agents"`
}

type localSkillSyncAgentPlan struct {
	AgentID            string                   `json:"agent_id"`
	Name               string                   `json:"name"`
	TargetRoot         string                   `json:"target_root,omitempty"`
	Supported          bool                     `json:"supported"`
	SupportStatus      string                   `json:"support_status"`
	ApplyMode          string                   `json:"apply_mode,omitempty"`
	ExportSupported    bool                     `json:"export_supported"`
	ExportAvailable    bool                     `json:"export_available"`
	ExportFileName     string                   `json:"export_file_name,omitempty"`
	AutoApplyReason    string                   `json:"auto_apply_reason,omitempty"`
	DocsPath           string                   `json:"docs_path,omitempty"`
	DirectoryRules     []string                 `json:"directory_rules,omitempty"`
	DetectedRoots      []localSkillDetectedRoot `json:"detected_roots,omitempty"`
	Message            string                   `json:"message,omitempty"`
	AssignedSkillPaths []string                 `json:"assigned_skill_paths"`
	Summary            localSkillSyncSummary    `json:"summary"`
	Changes            []localSkillSyncChange   `json:"changes"`
	Errors             []string                 `json:"errors,omitempty"`
}

type localSkillDetectedRoot struct {
	Path     string `json:"path"`
	Role     string `json:"role,omitempty"`
	Exists   bool   `json:"exists"`
	Writable bool   `json:"writable"`
	IsDir    bool   `json:"is_dir"`
	Message  string `json:"message,omitempty"`
}

type localSkillSyncSummary struct {
	Add       int `json:"add"`
	Update    int `json:"update"`
	Unchanged int `json:"unchanged"`
	Missing   int `json:"missing"`
	Conflict  int `json:"conflict"`
	Removable int `json:"removable"`
	Export    int `json:"export"`
	Written   int `json:"written"`
	Deleted   int `json:"deleted"`
}

type localSkillSyncChange struct {
	Action     string `json:"action"`
	SkillPath  string `json:"skill_path,omitempty"`
	RelPath    string `json:"rel_path,omitempty"`
	TargetPath string `json:"target_path,omitempty"`
	Reason     string `json:"reason,omitempty"`
	SizeBytes  int64  `json:"size_bytes,omitempty"`
}

type localSkillManagedMarker struct {
	Version   string `json:"version"`
	ManagedBy string `json:"managed_by"`
	AgentID   string `json:"agent_id"`
	SkillPath string `json:"skill_path"`
	Scope     string `json:"scope,omitempty"`
	TeamID    string `json:"team_id,omitempty"`
	TeamSlug  string `json:"team_slug,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

type localSkillFile struct {
	SkillPath   string
	HubPath     string
	RelPath     string
	Data        []byte
	ContentType string
}

type localSkillWriteOperation struct {
	Change localSkillSyncChange
	Data   []byte
}

type localSkillDeleteOperation struct {
	Change localSkillSyncChange
}

type localSkillScope struct {
	Scope    string
	TeamID   string
	TeamSlug string
	TeamName string
}

func (s *Server) handleLocalSkillSyncPreview(w http.ResponseWriter, r *http.Request) {
	_, ok := s.checkLocalSkillSyncAccess(w, r)
	if !ok {
		return
	}
	req, ok := decodeLocalSkillSyncRequest(w, r)
	if !ok {
		return
	}
	target, ok := s.resolveScopedHubTarget(w, r, req.TeamID, false)
	if !ok {
		return
	}
	resp, err := s.buildLocalSkillSyncResponse(r.Context(), target, req, false, false)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, resp)
}

func (s *Server) handleLocalSkillSyncApply(w http.ResponseWriter, r *http.Request) {
	_, ok := s.checkLocalSkillSyncAccess(w, r)
	if !ok {
		return
	}
	req, ok := decodeLocalSkillSyncRequest(w, r)
	if !ok {
		return
	}
	target, ok := s.resolveScopedHubTarget(w, r, req.TeamID, false)
	if !ok {
		return
	}
	resp, err := s.buildLocalSkillSyncResponse(r.Context(), target, req, true, false)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, resp)
}

func (s *Server) handleLocalSkillSyncCleanup(w http.ResponseWriter, r *http.Request) {
	_, ok := s.checkLocalSkillSyncAccess(w, r)
	if !ok {
		return
	}
	req, ok := decodeLocalSkillSyncRequest(w, r)
	if !ok {
		return
	}
	target, ok := s.resolveScopedHubTarget(w, r, req.TeamID, false)
	if !ok {
		return
	}
	resp, err := s.buildLocalSkillSyncResponse(r.Context(), target, req, true, true)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, resp)
}

func (s *Server) handleLocalSkillSyncExport(w http.ResponseWriter, r *http.Request) {
	_, ok := s.checkLocalSkillSyncAccess(w, r)
	if !ok {
		return
	}
	var req localSkillSyncExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	agent := localSkillAgentByID(req.AgentID)
	if agent == nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "unknown agent_id")
		return
	}
	if !agent.ExportSupported {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "agent does not support export package")
		return
	}
	target, ok := s.resolveScopedHubTarget(w, r, req.TeamID, false)
	if !ok {
		return
	}
	scope := localSkillScopeFromTarget(target)
	doc, err := s.readSkillAssignmentsFromContext(r.Context(), target.UserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	assigned := localSkillAssignmentsByAgent(doc.Assignments)[agent.ID]
	filename := localSkillExportFileName(*agent, scope)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := s.writeLocalSkillExportPackage(r.Context(), zw, target.UserID, *agent, scope, assigned); err != nil {
		_ = zw.Close()
		respondInternalError(w, err)
		return
	}
	if err := zw.Close(); err != nil {
		respondInternalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) checkLocalSkillSyncAccess(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return uuid.Nil, false
	}
	if !s.ensureLocalPlatformMode(w) {
		return uuid.Nil, false
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return uuid.Nil, false
	}
	userID, _ := userIDFromCtx(r.Context())
	return userID, true
}

func decodeLocalSkillSyncRequest(w http.ResponseWriter, r *http.Request) (localSkillSyncRequest, bool) {
	var req localSkillSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return localSkillSyncRequest{}, false
	}
	return req, true
}

func (s *Server) buildLocalSkillSyncResponse(ctx context.Context, target scopedHubTarget, req localSkillSyncRequest, apply bool, cleanupOnly bool) (*localSkillSyncResponse, error) {
	scope := localSkillScopeFromTarget(target)
	doc, err := s.readSkillAssignmentsFromContext(ctx, target.UserID)
	if err != nil {
		return nil, err
	}
	assigned := localSkillAssignmentsByAgent(doc.Assignments)
	agents := localSkillSelectedAgents(req.AgentIDs)
	resp := &localSkillSyncResponse{
		Version:   localSkillSyncVersion,
		Scope:     target.Scope,
		Team:      target.Team,
		Mode:      "preview",
		Applied:   apply,
		Cleanup:   cleanupOnly,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Agents:    make([]localSkillSyncAgentPlan, 0, len(agents)),
	}
	if apply && cleanupOnly {
		resp.Mode = "cleanup"
	} else if apply {
		resp.Mode = "apply"
	}

	for _, agent := range agents {
		plan, writes, deletes := s.buildLocalSkillSyncAgentPlan(ctx, target.UserID, scope, agent, assigned[agent.ID], req.TargetRoots, cleanupOnly)
		if apply {
			if cleanupOnly {
				s.applyLocalSkillDeletes(&plan, deletes)
			} else {
				s.applyLocalSkillWrites(&plan, scope, writes)
			}
		}
		resp.Agents = append(resp.Agents, plan)
	}
	return resp, nil
}

func (s *Server) buildLocalSkillSyncAgentPlan(
	ctx context.Context,
	userID uuid.UUID,
	scope localSkillScope,
	agent skillAgentTarget,
	assignedSkillPaths []string,
	targetRoots map[string]string,
	cleanupOnly bool,
) (localSkillSyncAgentPlan, []localSkillWriteOperation, []localSkillDeleteOperation) {
	plan := localSkillSyncAgentPlan{
		AgentID:            agent.ID,
		Name:               agent.Name,
		Supported:          agent.SupportsApply,
		SupportStatus:      agent.SupportStatus,
		ApplyMode:          agent.ApplyMode,
		ExportSupported:    agent.ExportSupported,
		AutoApplyReason:    agent.AutoApplyReason,
		DocsPath:           agent.DocsPath,
		DirectoryRules:     append([]string{}, agent.DirectoryRules...),
		DetectedRoots:      detectLocalSkillAgentRoots(agent.ID, targetRoots),
		AssignedSkillPaths: append([]string{}, assignedSkillPaths...),
		Changes:            []localSkillSyncChange{},
	}
	if !agent.SupportsApply {
		plan.Message = "This agent can be assigned and exported, but neuDrive will not write its local configuration automatically."
		if !cleanupOnly {
			s.addLocalSkillExportChanges(ctx, userID, scope, &plan, assignedSkillPaths)
		}
		return plan, nil, nil
	}

	root, err := localSkillTargetRoot(agent.ID, targetRoots)
	if err != nil {
		plan.Errors = append(plan.Errors, err.Error())
		return plan, nil, nil
	}
	plan.TargetRoot = root
	plan.DetectedRoots = []localSkillDetectedRoot{probeLocalSkillPath(root, "managed skill root")}

	assignedSet := map[string]struct{}{}
	for _, skillPath := range assignedSkillPaths {
		assignedSet[skillPath] = struct{}{}
	}

	writes := []localSkillWriteOperation{}
	markerDirs := map[string]string{}
	if !cleanupOnly {
		for _, skillPath := range assignedSkillPaths {
			files, err := s.collectLocalSkillFiles(ctx, userID, skillPath)
			if err != nil {
				if errors.Is(err, services.ErrEntryNotFound) {
					plan.Errors = append(plan.Errors, fmt.Sprintf("%s is not available in Hub", skillPath))
				} else {
					plan.Errors = append(plan.Errors, err.Error())
				}
				continue
			}
			sourceRelPaths := map[string]struct{}{}
			skillDir, err := localSkillDestinationDir(root, skillPath, scope)
			if err != nil {
				plan.Errors = append(plan.Errors, err.Error())
				continue
			}
			markerDirs[skillPath] = skillDir
			skillDirInfo, skillDirErr := os.Stat(skillDir)
			if skillDirErr != nil && !errors.Is(skillDirErr, os.ErrNotExist) {
				plan.Errors = append(plan.Errors, skillDirErr.Error())
				continue
			}
			if skillDirErr == nil && !skillDirInfo.IsDir() {
				plan.addChange(localSkillSyncChange{
					Action:     "conflict",
					SkillPath:  skillPath,
					TargetPath: skillDir,
					Reason:     "target skill path exists and is not a directory",
				})
				continue
			}
			marker, hasMarker, markerErr := readLocalSkillManagedMarker(skillDir)
			if markerErr != nil {
				plan.Errors = append(plan.Errors, markerErr.Error())
			}
			managedBySameSkill := hasMarker && localSkillMarkerMatches(marker, agent.ID, skillPath, scope)
			if skillDirErr == nil && !managedBySameSkill {
				plan.addChange(localSkillSyncChange{
					Action:     "conflict",
					SkillPath:  skillPath,
					TargetPath: skillDir,
					Reason:     "target skill directory already exists and is not managed by neuDrive for this assignment",
				})
				continue
			}

			for _, file := range files {
				sourceRelPaths[file.RelPath] = struct{}{}
				targetPath, err := localSkillDestinationPath(root, skillPath, file.RelPath, scope)
				if err != nil {
					plan.Errors = append(plan.Errors, err.Error())
					continue
				}
				change := localSkillSyncChange{
					SkillPath:  skillPath,
					RelPath:    file.RelPath,
					TargetPath: targetPath,
					SizeBytes:  int64(len(file.Data)),
				}
				existing, err := os.ReadFile(targetPath)
				if errors.Is(err, os.ErrNotExist) {
					change.Action = "add"
					plan.addChange(change)
					writes = append(writes, localSkillWriteOperation{Change: change, Data: file.Data})
					continue
				}
				if err != nil {
					change.Action = "conflict"
					change.Reason = err.Error()
					plan.addChange(change)
					continue
				}
				if bytes.Equal(existing, file.Data) {
					change.Action = "unchanged"
					plan.addChange(change)
					continue
				}
				if managedBySameSkill {
					change.Action = "update"
					plan.addChange(change)
					writes = append(writes, localSkillWriteOperation{Change: change, Data: file.Data})
					continue
				}
				change.Action = "conflict"
				change.Reason = "target file differs and is not managed by neuDrive for this skill"
				plan.addChange(change)
			}
			addMissingLocalSkillFiles(&plan, root, agent.ID, skillPath, scope, sourceRelPaths)
		}
	}

	for skillPath, skillDir := range markerDirs {
		if _, ok := assignedSet[skillPath]; ok {
			markerChange := localSkillSyncChange{
				Action:     "marker",
				SkillPath:  skillPath,
				TargetPath: filepath.Join(skillDir, localSkillManagedFileName),
			}
			writes = append(writes, localSkillWriteOperation{Change: markerChange})
		}
	}

	deletes := collectLocalSkillDeletes(&plan, root, agent.ID, scope, assignedSet)
	return plan, writes, deletes
}

func (s *Server) addLocalSkillExportChanges(ctx context.Context, userID uuid.UUID, scope localSkillScope, plan *localSkillSyncAgentPlan, assignedSkillPaths []string) {
	if plan == nil || !plan.ExportSupported {
		return
	}
	plan.ExportFileName = localSkillExportFileName(skillAgentTarget{
		ID:       plan.AgentID,
		Name:     plan.Name,
		Platform: plan.AgentID,
	}, scope)
	for _, skillPath := range assignedSkillPaths {
		files, err := s.collectLocalSkillFiles(ctx, userID, skillPath)
		if err != nil {
			if errors.Is(err, services.ErrEntryNotFound) {
				plan.Errors = append(plan.Errors, fmt.Sprintf("%s is not available in Hub", skillPath))
			} else {
				plan.Errors = append(plan.Errors, err.Error())
			}
			continue
		}
		var total int64
		for _, file := range files {
			total += int64(len(file.Data))
		}
		plan.ExportAvailable = true
		plan.addChange(localSkillSyncChange{
			Action:     "export",
			SkillPath:  normalizeAssignedSkillPath(skillPath),
			TargetPath: plan.ExportFileName,
			Reason:     "export package available; review and install manually for this agent",
			SizeBytes:  total,
		})
	}
}

func (s *Server) collectLocalSkillFiles(ctx context.Context, userID uuid.UUID, skillPath string) ([]localSkillFile, error) {
	normalized := normalizeAssignedSkillPath(skillPath)
	if normalized == "" {
		return nil, fmt.Errorf("invalid skill path %q", skillPath)
	}
	snapshot, err := s.FileTreeService.Snapshot(ctx, userID, normalized, models.TrustLevelFull)
	if err != nil {
		return nil, err
	}
	files := make([]localSkillFile, 0, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		if entry.IsDirectory || entry.DeletedAt != nil {
			continue
		}
		prefix := strings.TrimSuffix(normalized, "/") + "/"
		if !strings.HasPrefix(entry.Path, prefix) {
			continue
		}
		relPath := strings.TrimPrefix(entry.Path, prefix)
		if relPath == "" {
			continue
		}
		data := []byte(entry.Content)
		if isBinaryMetadata(entry.Metadata) {
			binaryData, _, err := s.FileTreeService.ReadBinary(ctx, userID, entry.Path, models.TrustLevelFull)
			if err != nil {
				return nil, err
			}
			data = binaryData
		}
		files = append(files, localSkillFile{
			SkillPath:   normalized,
			HubPath:     entry.Path,
			RelPath:     filepath.ToSlash(relPath),
			Data:        data,
			ContentType: entry.ContentType,
		})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].SkillPath != files[j].SkillPath {
			return files[i].SkillPath < files[j].SkillPath
		}
		return files[i].RelPath < files[j].RelPath
	})
	return files, nil
}

func (s *Server) applyLocalSkillWrites(plan *localSkillSyncAgentPlan, scope localSkillScope, writes []localSkillWriteOperation) {
	if !plan.Supported || plan.TargetRoot == "" {
		return
	}
	if plan.Summary.Conflict > 0 {
		plan.Errors = append(plan.Errors, "conflicts found; no files were written for this agent")
		return
	}
	for _, op := range writes {
		if op.Change.Action == "marker" {
			if err := writeLocalSkillManagedMarker(filepath.Dir(op.Change.TargetPath), plan.AgentID, op.Change.SkillPath, scope); err != nil {
				plan.Errors = append(plan.Errors, err.Error())
			}
			continue
		}
		if op.Change.Action != "add" && op.Change.Action != "update" {
			continue
		}
		if !localPathInside(plan.TargetRoot, op.Change.TargetPath) {
			plan.Errors = append(plan.Errors, fmt.Sprintf("%s is outside target root", op.Change.TargetPath))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(op.Change.TargetPath), 0o755); err != nil {
			plan.Errors = append(plan.Errors, err.Error())
			continue
		}
		if err := os.WriteFile(op.Change.TargetPath, op.Data, 0o644); err != nil {
			plan.Errors = append(plan.Errors, err.Error())
			continue
		}
		plan.Summary.Written++
	}
}

func (s *Server) applyLocalSkillDeletes(plan *localSkillSyncAgentPlan, deletes []localSkillDeleteOperation) {
	if !plan.Supported || plan.TargetRoot == "" {
		return
	}
	for _, op := range deletes {
		if op.Change.Action != "delete" {
			continue
		}
		if !localPathInside(plan.TargetRoot, op.Change.TargetPath) {
			plan.Errors = append(plan.Errors, fmt.Sprintf("%s is outside target root", op.Change.TargetPath))
			continue
		}
		if err := os.RemoveAll(op.Change.TargetPath); err != nil {
			plan.Errors = append(plan.Errors, err.Error())
			continue
		}
		plan.Summary.Deleted++
	}
}

func addMissingLocalSkillFiles(plan *localSkillSyncAgentPlan, root, agentID, skillPath string, scope localSkillScope, sourceRelPaths map[string]struct{}) {
	skillDir, err := localSkillDestinationDir(root, skillPath, scope)
	if err != nil {
		plan.Errors = append(plan.Errors, err.Error())
		return
	}
	marker, hasMarker, err := readLocalSkillManagedMarker(skillDir)
	if err != nil {
		plan.Errors = append(plan.Errors, err.Error())
		return
	}
	if !hasMarker || !localSkillMarkerMatches(marker, agentID, skillPath, scope) {
		return
	}
	err = filepath.WalkDir(skillDir, func(pathValue string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(skillDir, pathValue)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == localSkillManagedFileName {
			return nil
		}
		if _, ok := sourceRelPaths[rel]; ok {
			return nil
		}
		plan.addChange(localSkillSyncChange{
			Action:     "missing",
			SkillPath:  skillPath,
			RelPath:    rel,
			TargetPath: pathValue,
			Reason:     "local file is not present in Hub skill",
		})
		return nil
	})
	if err != nil {
		plan.Errors = append(plan.Errors, err.Error())
	}
}

func collectLocalSkillDeletes(plan *localSkillSyncAgentPlan, root, agentID string, scope localSkillScope, assignedSet map[string]struct{}) []localSkillDeleteOperation {
	deletes := []localSkillDeleteOperation{}
	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return deletes
	}
	if err != nil {
		plan.Errors = append(plan.Errors, err.Error())
		return deletes
	}
	if !info.IsDir() {
		plan.Errors = append(plan.Errors, root+" is not a directory")
		return deletes
	}
	err = filepath.WalkDir(root, func(pathValue string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() || pathValue == root {
			return nil
		}
		marker, hasMarker, err := readLocalSkillManagedMarker(pathValue)
		if err != nil {
			return err
		}
		if !hasMarker {
			return nil
		}
		if localSkillMarkerMatchesScope(marker, agentID, scope) {
			normalizedSkillPath := normalizeAssignedSkillPath(marker.SkillPath)
			if _, ok := assignedSet[normalizedSkillPath]; !ok {
				change := localSkillSyncChange{
					Action:     "delete",
					SkillPath:  normalizedSkillPath,
					TargetPath: pathValue,
					Reason:     "managed skill is no longer assigned",
				}
				plan.addChange(change)
				deletes = append(deletes, localSkillDeleteOperation{Change: change})
			}
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		plan.Errors = append(plan.Errors, err.Error())
	}
	return deletes
}

func (p *localSkillSyncAgentPlan) addChange(change localSkillSyncChange) {
	p.Changes = append(p.Changes, change)
	switch change.Action {
	case "add":
		p.Summary.Add++
	case "update":
		p.Summary.Update++
	case "unchanged":
		p.Summary.Unchanged++
	case "missing":
		p.Summary.Missing++
	case "conflict":
		p.Summary.Conflict++
	case "delete":
		p.Summary.Removable++
	case "export":
		p.Summary.Export++
	}
}

func localSkillAssignmentsByAgent(assignments []skillAgentAssignment) map[string][]string {
	out := map[string][]string{}
	for _, item := range normalizeSkillAssignments(assignments) {
		out[item.AgentID] = append([]string{}, item.SkillPaths...)
	}
	return out
}

func localSkillSelectedAgents(agentIDs []string) []skillAgentTarget {
	if len(agentIDs) == 0 {
		return append([]skillAgentTarget{}, skillAgentTargets...)
	}
	selected := map[string]struct{}{}
	for _, id := range agentIDs {
		selected[strings.TrimSpace(id)] = struct{}{}
	}
	out := []skillAgentTarget{}
	for _, target := range skillAgentTargets {
		if _, ok := selected[target.ID]; ok {
			out = append(out, target)
		}
	}
	return out
}

func localSkillAgentByID(agentID string) *skillAgentTarget {
	clean := strings.TrimSpace(agentID)
	for _, target := range skillAgentTargets {
		if target.ID == clean {
			next := target
			return &next
		}
	}
	return nil
}

func localSkillTargetRoot(agentID string, targetRoots map[string]string) (string, error) {
	value := ""
	if targetRoots != nil {
		value = strings.TrimSpace(targetRoots[agentID])
	}
	if value == "" {
		switch agentID {
		case "claude-code":
			value = "~/.claude/skills"
		case "codex":
			value = "~/.codex/skills"
		default:
			return "", fmt.Errorf("no local skill root configured for %s", agentID)
		}
	}
	expanded, err := expandLocalSkillHome(value)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(expanded) {
		expanded, err = filepath.Abs(expanded)
		if err != nil {
			return "", err
		}
	}
	return filepath.Clean(expanded), nil
}

func detectLocalSkillAgentRoots(agentID string, targetRoots map[string]string) []localSkillDetectedRoot {
	if target, err := localSkillTargetRoot(agentID, targetRoots); err == nil {
		return []localSkillDetectedRoot{probeLocalSkillPath(target, "managed skill root")}
	}
	candidates := []struct {
		path string
		role string
	}{}
	switch agentID {
	case "cursor":
		candidates = append(candidates,
			struct {
				path string
				role string
			}{path: "~/.cursor", role: "user config root"},
			struct {
				path string
				role string
			}{path: "~/.cursor/rules", role: "user rules candidate"},
			struct {
				path string
				role string
			}{path: ".cursor/rules", role: "workspace rules candidate"},
		)
	case "gemini-cli":
		candidates = append(candidates,
			struct {
				path string
				role string
			}{path: "~/.gemini", role: "user config root"},
			struct {
				path string
				role string
			}{path: "GEMINI.md", role: "workspace guidance candidate"},
		)
	}
	out := make([]localSkillDetectedRoot, 0, len(candidates))
	for _, candidate := range candidates {
		expanded, err := expandLocalSkillHome(candidate.path)
		if err != nil {
			out = append(out, localSkillDetectedRoot{Path: candidate.path, Role: candidate.role, Message: err.Error()})
			continue
		}
		if !filepath.IsAbs(expanded) {
			if abs, err := filepath.Abs(expanded); err == nil {
				expanded = abs
			}
		}
		out = append(out, probeLocalSkillPath(filepath.Clean(expanded), candidate.role))
	}
	return out
}

func probeLocalSkillPath(pathValue, role string) localSkillDetectedRoot {
	probe := localSkillDetectedRoot{Path: pathValue, Role: role}
	info, err := os.Stat(pathValue)
	if errors.Is(err, os.ErrNotExist) {
		probe.Message = "not found"
		return probe
	}
	if err != nil {
		probe.Message = err.Error()
		return probe
	}
	probe.Exists = true
	probe.IsDir = info.IsDir()
	probe.Writable = info.Mode().Perm()&0o200 != 0
	return probe
}

func expandLocalSkillHome(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("target root is required")
	}
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if value == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(value, "~/")), nil
	}
	return value, nil
}

func localSkillDestinationDir(root, skillPath string, scope localSkillScope) (string, error) {
	normalized := normalizeAssignedSkillPath(skillPath)
	if normalized == "" || normalized == "/skills" {
		return "", fmt.Errorf("invalid skill path %q", skillPath)
	}
	rel := strings.TrimPrefix(normalized, "/skills/")
	if rel == "" || rel == normalized {
		return "", fmt.Errorf("invalid skill path %q", skillPath)
	}
	if scope.Scope == "team" {
		rel = localSkillTeamDestinationName(scope, rel)
	}
	target := filepath.Join(root, filepath.FromSlash(rel))
	if !localPathInside(root, target) {
		return "", fmt.Errorf("%s resolves outside target root", skillPath)
	}
	return target, nil
}

func localSkillDestinationPath(root, skillPath, relPath string, scope localSkillScope) (string, error) {
	skillDir, err := localSkillDestinationDir(root, skillPath, scope)
	if err != nil {
		return "", err
	}
	cleanRel := filepath.Clean(filepath.FromSlash(strings.TrimSpace(relPath)))
	if cleanRel == "." || strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
		return "", fmt.Errorf("invalid local skill relative path %q", relPath)
	}
	target := filepath.Join(skillDir, cleanRel)
	if !localPathInside(root, target) {
		return "", fmt.Errorf("%s resolves outside target root", relPath)
	}
	return target, nil
}

func localPathInside(root, target string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

func readLocalSkillManagedMarker(skillDir string) (localSkillManagedMarker, bool, error) {
	data, err := os.ReadFile(filepath.Join(skillDir, localSkillManagedFileName))
	if errors.Is(err, os.ErrNotExist) {
		return localSkillManagedMarker{}, false, nil
	}
	if err != nil {
		return localSkillManagedMarker{}, false, err
	}
	var marker localSkillManagedMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return localSkillManagedMarker{}, false, err
	}
	return marker, true, nil
}

func writeLocalSkillManagedMarker(skillDir, agentID, skillPath string, scope localSkillScope) error {
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}
	marker := localSkillManagedMarker{
		Version:   localSkillSyncVersion,
		ManagedBy: "neudrive",
		AgentID:   agentID,
		SkillPath: normalizeAssignedSkillPath(skillPath),
		Scope:     localSkillScopeName(scope),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if marker.Scope == "team" {
		marker.TeamID = scope.TeamID
		marker.TeamSlug = scope.TeamSlug
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(skillDir, localSkillManagedFileName), append(data, '\n'), 0o644)
}

func localSkillMarkerMatches(marker localSkillManagedMarker, agentID, skillPath string, scope localSkillScope) bool {
	return marker.ManagedBy == "neudrive" &&
		marker.AgentID == agentID &&
		localSkillMarkerMatchesScope(marker, agentID, scope) &&
		normalizeAssignedSkillPath(marker.SkillPath) == normalizeAssignedSkillPath(skillPath)
}

func localSkillMarkerMatchesScope(marker localSkillManagedMarker, agentID string, scope localSkillScope) bool {
	if marker.ManagedBy != "neudrive" || marker.AgentID != agentID {
		return false
	}
	markerScope := strings.TrimSpace(marker.Scope)
	if markerScope == "" {
		markerScope = "personal"
	}
	expectedScope := localSkillScopeName(scope)
	if markerScope != expectedScope {
		return false
	}
	if expectedScope == "team" {
		return strings.TrimSpace(marker.TeamID) != "" && marker.TeamID == scope.TeamID
	}
	return true
}

func localSkillExportFileName(agent skillAgentTarget, scope localSkillScope) string {
	name := strings.TrimSpace(agent.ID)
	if name == "" {
		name = "agent"
	}
	name = strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(strings.ToLower(name))
	if localSkillScopeName(scope) == "team" {
		teamSlug := localSkillSafeName(scope.TeamSlug)
		if teamSlug == "" {
			teamSlug = "team"
		}
		return "neudrive-skills-" + teamSlug + "-" + name + ".zip"
	}
	return "neudrive-skills-" + name + ".zip"
}

func (s *Server) writeLocalSkillExportPackage(ctx context.Context, zw *zip.Writer, userID uuid.UUID, agent skillAgentTarget, scope localSkillScope, assignedSkillPaths []string) error {
	if zw == nil {
		return fmt.Errorf("zip writer is required")
	}
	readme := localSkillExportReadme(agent, scope, assignedSkillPaths)
	if err := writeLocalSkillExportFile(zw, "README.md", []byte(readme)); err != nil {
		return err
	}
	for _, skillPath := range assignedSkillPaths {
		files, err := s.collectLocalSkillFiles(ctx, userID, skillPath)
		if err != nil {
			return err
		}
		skillName := strings.TrimPrefix(normalizeAssignedSkillPath(skillPath), "/skills/")
		for _, file := range files {
			zipPath := pathpkg.Join("skills", skillName, filepath.ToSlash(file.RelPath))
			if err := writeLocalSkillExportFile(zw, zipPath, file.Data); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeLocalSkillExportFile(zw *zip.Writer, zipPath string, data []byte) error {
	clean := pathpkg.Clean(strings.TrimPrefix(strings.ReplaceAll(zipPath, "\\", "/"), "/"))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || clean == ".." {
		return fmt.Errorf("invalid export path %q", zipPath)
	}
	header := &zip.FileHeader{
		Name:   clean,
		Method: zip.Deflate,
	}
	header.Modified = time.Now().UTC()
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func localSkillExportReadme(agent skillAgentTarget, scope localSkillScope, assignedSkillPaths []string) string {
	lines := []string{
		"# neuDrive Skill Export",
		"",
		"Agent: " + agent.Name,
		"Scope: " + localSkillScopeLabel(scope),
		"Support status: " + firstLocalSkillNonEmpty(agent.SupportStatus, "export_only"),
		"",
		"## What is included",
		"",
		"- Full skill folders under `skills/`, including `SKILL.md`, scripts, dependency files, assets, external Claude tools/plugins, and `manifest.neudrive.json` when present.",
		"- This package does not install MCP servers, plugins, hooks, or secrets.",
		"",
		"## Directory rules",
		"",
	}
	if len(agent.DirectoryRules) == 0 {
		lines = append(lines, "- Review the target agent documentation before copying files.")
	} else {
		for _, rule := range agent.DirectoryRules {
			lines = append(lines, "- "+rule)
		}
	}
	if strings.TrimSpace(agent.AutoApplyReason) != "" {
		lines = append(lines, "", "## Why neuDrive did not write this automatically", "", agent.AutoApplyReason)
	}
	lines = append(lines, "", "## Assigned skills", "")
	if len(assignedSkillPaths) == 0 {
		lines = append(lines, "- No skills assigned.")
	} else {
		for _, skillPath := range assignedSkillPaths {
			lines = append(lines, "- "+normalizeAssignedSkillPath(skillPath))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func localSkillScopeFromTarget(target scopedHubTarget) localSkillScope {
	scope := localSkillScope{
		Scope: "personal",
	}
	if target.Scope == "team" && target.Team != nil {
		scope.Scope = "team"
		scope.TeamID = target.Team.ID.String()
		scope.TeamSlug = target.Team.Slug
		scope.TeamName = target.Team.Name
	}
	return scope
}

func localSkillScopeName(scope localSkillScope) string {
	if strings.TrimSpace(scope.Scope) == "team" {
		return "team"
	}
	return "personal"
}

func localSkillScopeLabel(scope localSkillScope) string {
	if localSkillScopeName(scope) != "team" {
		return "personal"
	}
	name := strings.TrimSpace(scope.TeamName)
	slug := strings.TrimSpace(scope.TeamSlug)
	if name != "" && slug != "" {
		return "team " + name + " (" + slug + ")"
	}
	if slug != "" {
		return "team " + slug
	}
	return "team " + strings.TrimSpace(scope.TeamID)
}

func localSkillTeamDestinationName(scope localSkillScope, rel string) string {
	teamSlug := localSkillSafeName(scope.TeamSlug)
	if teamSlug == "" {
		teamSlug = "team"
	}
	rel = strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/")
	rel = strings.ReplaceAll(rel, "/", "--")
	return teamSlug + "--" + localSkillSafeName(rel)
}

func localSkillSafeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("/", "-", "\\", "-", " ", "-", "_", "-").Replace(value)
	var b strings.Builder
	previousDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		if !ok {
			r = '-'
		}
		if r == '-' {
			if previousDash {
				continue
			}
			previousDash = true
		} else {
			previousDash = false
		}
		b.WriteRune(r)
	}
	return strings.Trim(b.String(), "-")
}

func firstLocalSkillNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
