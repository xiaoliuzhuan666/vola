package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/google/uuid"
)

const (
	skillAssignmentsVersion = "vola.skill-assignments/v1"
	skillAssignmentsPath    = "/settings/agent-skill-assignments.json"
)

type skillAgentTarget struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Platform        string   `json:"platform"`
	InstallPathHint string   `json:"install_path_hint,omitempty"`
	SupportsApply   bool     `json:"supports_apply"`
	SupportStatus   string   `json:"support_status"`
	ApplyMode       string   `json:"apply_mode,omitempty"`
	ExportSupported bool     `json:"export_supported"`
	AutoApplyReason string   `json:"auto_apply_reason,omitempty"`
	DocsPath        string   `json:"docs_path,omitempty"`
	DirectoryRules  []string `json:"directory_rules,omitempty"`
}

type skillAgentAssignment struct {
	AgentID    string   `json:"agent_id"`
	SkillPaths []string `json:"skill_paths"`
}

type skillAssignmentsDocument struct {
	Version     string                 `json:"version"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
	Assignments []skillAgentAssignment `json:"assignments"`
}

type skillAssignmentsResponse struct {
	Version     string                 `json:"version"`
	Scope       string                 `json:"scope"`
	Team        *models.Team           `json:"team,omitempty"`
	StoragePath string                 `json:"storage_path"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
	Agents      []skillAgentTarget     `json:"agents"`
	Assignments []skillAgentAssignment `json:"assignments"`
}

type saveSkillAssignmentsRequest struct {
	Assignments []skillAgentAssignment `json:"assignments"`
	TeamID      string                 `json:"team_id,omitempty"`
}

var skillAgentTargets = []skillAgentTarget{
	{
		ID: "claude-code", Name: "Claude Code", Platform: "claude-code", InstallPathHint: "~/.claude/skills",
		SupportsApply: true, SupportStatus: "managed_apply", ApplyMode: "managed-directory", ExportSupported: true,
		DocsPath: "docs/agent-skill-targets.zh-CN.md",
		DirectoryRules: []string{
			"Vola writes assigned skills into ~/.claude/skills by default.",
			"Only directories with .vola-managed.json are updated or cleaned.",
		},
	},
	{
		ID: "codex", Name: "Codex", Platform: "codex", InstallPathHint: "~/.agents/skills",
		SupportsApply: true, SupportStatus: "managed_apply", ApplyMode: "managed-directory", ExportSupported: true,
		DocsPath: "docs/agent-skill-targets.zh-CN.md",
		DirectoryRules: []string{
			"Vola writes assigned skills into ~/.agents/skills by default.",
			"Only directories with .vola-managed.json are updated or cleaned.",
		},
	},
	{
		ID: "cursor", Name: "Cursor", Platform: "cursor", InstallPathHint: ".cursor/rules",
		SupportsApply: false, SupportStatus: "export_only", ApplyMode: "export-only", ExportSupported: true,
		AutoApplyReason: "Cursor rules are project-specific, so Vola keeps assignments and export packages but does not edit Cursor configuration automatically.",
		DocsPath:        "docs/agent-skill-targets.zh-CN.md",
		DirectoryRules: []string{
			"Cursor assignments are preserved in Hub and can be exported as a zip package.",
			"Review the exported SKILL.md and assets, then manually adapt them into the Cursor rules you already use.",
			"Vola does not edit .cursor/rules, ~/.cursor, or Cursor project configuration automatically.",
		},
	},
	{
		ID: "gemini-cli", Name: "Gemini CLI", Platform: "gemini-cli", InstallPathHint: "Export package; manual GEMINI.md integration",
		SupportsApply: false, SupportStatus: "export_only", ApplyMode: "export-only", ExportSupported: true,
		AutoApplyReason: "Gemini CLI guidance is commonly file-based, but there is no Vola-managed Skill directory contract to write safely.",
		DocsPath:        "docs/agent-skill-targets.zh-CN.md",
		DirectoryRules: []string{
			"Gemini CLI assignments are preserved in Hub and can be exported as a zip package.",
			"Review the exported SKILL.md and assets, then manually reference or adapt them in the project or user guidance files you already use.",
			"Vola does not edit GEMINI.md or Gemini CLI configuration automatically.",
		},
	},
}

func (s *Server) handleSkillAssignmentsGet(w http.ResponseWriter, r *http.Request) {
	if s.FileTreeService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "file tree service not configured")
		return
	}
	target, ok := s.resolveScopedHubTarget(w, r, "", false)
	if !ok {
		return
	}

	doc, err := s.readSkillAssignments(r, target.UserID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, skillAssignmentsResponse{
		Version:     skillAssignmentsVersion,
		Scope:       target.Scope,
		Team:        target.Team,
		StoragePath: skillAssignmentsPath,
		UpdatedAt:   doc.UpdatedAt,
		Agents:      append([]skillAgentTarget{}, skillAgentTargets...),
		Assignments: doc.Assignments,
	})
}

func (s *Server) handleSkillAssignmentsSave(w http.ResponseWriter, r *http.Request) {
	if s.FileTreeService == nil {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "file tree service not configured")
		return
	}
	var req saveSkillAssignmentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	target, ok := s.resolveScopedHubTarget(w, r, req.TeamID, true)
	if !ok {
		return
	}

	doc := skillAssignmentsDocument{
		Version:     skillAssignmentsVersion,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Assignments: normalizeSkillAssignments(req.Assignments),
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		respondInternalError(w, err)
		return
	}
	data = append(data, '\n')

	if _, err := s.FileTreeService.WriteEntry(r.Context(), target.UserID, skillAssignmentsPath, string(data), "application/json", models.FileTreeWriteOptions{
		Kind: "skill_assignment",
		Metadata: map[string]interface{}{
			"source":       "manual",
			"capture_mode": "skill-assignment",
			"scope":        target.Scope,
		},
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		respondInternalError(w, err)
		return
	}

	resp := skillAssignmentsResponse{
		Version:     skillAssignmentsVersion,
		Scope:       target.Scope,
		Team:        target.Team,
		StoragePath: skillAssignmentsPath,
		UpdatedAt:   doc.UpdatedAt,
		Agents:      append([]skillAgentTarget{}, skillAgentTargets...),
		Assignments: doc.Assignments,
	}
	if target.Scope == "personal" {
		respondOKWithLocalGitSync(w, resp, s.syncLocalGitMirror(r.Context(), target.UserID))
		return
	}
	respondOK(w, resp)
}

func (s *Server) readSkillAssignments(r *http.Request, userID uuid.UUID) (skillAssignmentsDocument, error) {
	return s.readSkillAssignmentsFromContext(r.Context(), userID)
}

func (s *Server) readSkillAssignmentsFromContext(ctx context.Context, userID uuid.UUID) (skillAssignmentsDocument, error) {
	entry, err := s.FileTreeService.Read(ctx, userID, skillAssignmentsPath, models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			return skillAssignmentsDocument{Version: skillAssignmentsVersion, Assignments: []skillAgentAssignment{}}, nil
		}
		return skillAssignmentsDocument{}, err
	}
	var doc skillAssignmentsDocument
	if err := json.Unmarshal([]byte(entry.Content), &doc); err != nil {
		return skillAssignmentsDocument{}, err
	}
	doc.Version = skillAssignmentsVersion
	doc.Assignments = normalizeSkillAssignments(doc.Assignments)
	return doc, nil
}

func normalizeSkillAssignments(items []skillAgentAssignment) []skillAgentAssignment {
	knownAgents := map[string]struct{}{}
	for _, target := range skillAgentTargets {
		knownAgents[target.ID] = struct{}{}
	}
	grouped := map[string]map[string]struct{}{}
	for _, item := range items {
		agentID := strings.TrimSpace(item.AgentID)
		if _, ok := knownAgents[agentID]; !ok {
			continue
		}
		if grouped[agentID] == nil {
			grouped[agentID] = map[string]struct{}{}
		}
		for _, skillPath := range item.SkillPaths {
			if normalized := normalizeAssignedSkillPath(skillPath); normalized != "" {
				grouped[agentID][normalized] = struct{}{}
			}
		}
	}

	out := make([]skillAgentAssignment, 0, len(skillAgentTargets))
	for _, target := range skillAgentTargets {
		paths := sortedStringSet(grouped[target.ID])
		out = append(out, skillAgentAssignment{
			AgentID:    target.ID,
			SkillPaths: paths,
		})
	}
	return out
}

func normalizeAssignedSkillPath(value string) string {
	clean := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if clean == "" {
		return ""
	}
	if strings.HasSuffix(clean, "/SKILL.md") {
		clean = path.Dir(clean)
	}
	clean = strings.TrimSuffix(clean, "/")
	if clean == "" || clean == "." {
		return ""
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	if !strings.HasPrefix(clean, "/skills/") {
		clean = "/skills/" + strings.TrimPrefix(clean, "/")
	}
	clean = path.Clean(clean)
	if clean != "/skills" && !strings.HasPrefix(clean, "/skills/") {
		return ""
	}
	return clean
}

func sortedStringSet(values map[string]struct{}) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
