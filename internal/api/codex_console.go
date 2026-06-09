package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/agi-bar/vola/internal/hubpath"
	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/platforms"
	"github.com/agi-bar/vola/internal/services"
	"github.com/agi-bar/vola/internal/skillsarchive"
	sqlitestorage "github.com/agi-bar/vola/internal/storage/sqlite"
	"github.com/google/uuid"
)

type codexConsoleResponse struct {
	Platform          string                                `json:"platform"`
	UpdatedAt         string                                `json:"updated_at"`
	Overview          codexConsoleOverview                  `json:"overview"`
	Threads           []codexConsoleThread                  `json:"threads"`
	Goals             []codexConsoleGoal                    `json:"goals"`
	Automations       []codexConsoleAutomation              `json:"automations"`
	Runs              []codexConsoleRun                     `json:"runs"`
	Artifacts         []codexConsoleArtifact                `json:"artifacts"`
	ArtifactRegistry  codexConsoleArtifactRegistrySummary   `json:"artifact_registry"`
	Hooks             []codexConsoleHookRisk                `json:"hooks"`
	MemoryCandidates  []codexConsoleMemoryCandidate         `json:"memory_candidates"`
	Handovers         []codexConsoleHandoverSummary         `json:"handovers"`
	SkillCandidates   []codexConsoleSkillCandidate          `json:"skill_candidates"`
	SensitiveFindings []sqlitestorage.AgentSensitiveFinding `json:"sensitive_findings,omitempty"`
	VaultCandidates   []sqlitestorage.AgentVaultCandidate   `json:"vault_candidates,omitempty"`
	Notes             []string                              `json:"notes,omitempty"`
}

type codexConsoleOverview struct {
	Threads              int                     `json:"threads"`
	Goals                int                     `json:"goals"`
	Automations          int                     `json:"automations"`
	Runs                 int                     `json:"runs"`
	Artifacts            int                     `json:"artifacts"`
	Hooks                int                     `json:"hooks"`
	MemoryCandidates     int                     `json:"memory_candidates"`
	Handovers            int                     `json:"handovers"`
	SkillCandidates      int                     `json:"skill_candidates"`
	MemoryReviewRequired int                     `json:"memory_review_required"`
	MemoryAccepted       int                     `json:"memory_accepted"`
	MemoryIgnored        int                     `json:"memory_ignored"`
	MemoryDeferred       int                     `json:"memory_deferred"`
	MemorySynced         int                     `json:"memory_synced"`
	Projects             int                     `json:"projects"`
	Skills               int                     `json:"skills"`
	Tools                int                     `json:"tools"`
	SensitiveFindings    int                     `json:"sensitive_findings"`
	VaultCandidates      int                     `json:"vault_candidates"`
	LastActivity         string                  `json:"last_activity,omitempty"`
	Workspaces           []codexConsoleWorkspace `json:"workspaces,omitempty"`
}

type codexConsoleWorkspace struct {
	Name         string `json:"name"`
	Threads      int    `json:"threads"`
	LastActivity string `json:"last_activity,omitempty"`
}

type codexConsoleThread struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Summary         string `json:"summary,omitempty"`
	Project         string `json:"project,omitempty"`
	StartedAt       string `json:"started_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
	Archived        bool   `json:"archived"`
	SourcePath      string `json:"source_path,omitempty"`
	MessageCount    int    `json:"message_count"`
	UserTurns       int    `json:"user_turns"`
	AssistantTurns  int    `json:"assistant_turns"`
	ToolCalls       int    `json:"tool_calls"`
	ToolResults     int    `json:"tool_results"`
	ThinkingEvents  int    `json:"thinking_events"`
	AttachmentCount int    `json:"attachment_count"`
	ArtifactCount   int    `json:"artifact_count"`
}

type codexConsoleGoal struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	ThreadID    string `json:"thread_id,omitempty"`
	ThreadTitle string `json:"thread_title,omitempty"`
	Project     string `json:"project,omitempty"`
	SourcePath  string `json:"source_path,omitempty"`
	ObservedAt  string `json:"observed_at,omitempty"`
	Description string `json:"description,omitempty"`
}

type codexConsoleAutomation struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind,omitempty"`
	Status     string `json:"status,omitempty"`
	Schedule   string `json:"schedule,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
	SourcePath string `json:"source_path,omitempty"`
}

type codexConsoleRun struct {
	ID              string                 `json:"id"`
	ThreadID        string                 `json:"thread_id"`
	ThreadTitle     string                 `json:"thread_title"`
	Project         string                 `json:"project,omitempty"`
	StartedAt       string                 `json:"started_at,omitempty"`
	UpdatedAt       string                 `json:"updated_at,omitempty"`
	SourcePath      string                 `json:"source_path,omitempty"`
	ToolCalls       int                    `json:"tool_calls"`
	ToolResults     int                    `json:"tool_results"`
	BrowserActions  int                    `json:"browser_actions"`
	ComputerActions int                    `json:"computer_actions"`
	Approvals       int                    `json:"approvals"`
	Errors          int                    `json:"errors"`
	Artifacts       int                    `json:"artifacts"`
	Events          []codexConsoleRunEvent `json:"events,omitempty"`
}

type codexConsoleRunEvent struct {
	At     string `json:"at,omitempty"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
}

type codexConsoleArtifact struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	Role             string `json:"role,omitempty"`
	ThreadID         string `json:"thread_id,omitempty"`
	ThreadTitle      string `json:"thread_title,omitempty"`
	Project          string `json:"project,omitempty"`
	SourcePath       string `json:"source_path,omitempty"`
	Detail           string `json:"detail,omitempty"`
	HandoffNote      string `json:"handoff_note,omitempty"`
	AgentInstruction string `json:"agent_instruction,omitempty"`
}

type codexConsoleArtifactRegistrySummary struct {
	Status           string                               `json:"status,omitempty"`
	Path             string                               `json:"path,omitempty"`
	SavedAt          string                               `json:"saved_at,omitempty"`
	ArtifactCount    int                                  `json:"artifact_count,omitempty"`
	ProjectCount     int                                  `json:"project_count,omitempty"`
	ProjectSummaries []codexConsoleArtifactProjectSummary `json:"project_summaries,omitempty"`
}

type codexConsoleHookRisk struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	Bundle         string   `json:"bundle,omitempty"`
	Status         string   `json:"status"`
	RiskLevel      string   `json:"risk_level,omitempty"`
	Shebang        string   `json:"shebang,omitempty"`
	EnvVars        []string `json:"env_vars,omitempty"`
	RiskSignals    []string `json:"risk_signals,omitempty"`
	WritePathHints []string `json:"write_path_hints,omitempty"`
	SourcePath     string   `json:"source_path,omitempty"`
	Detail         string   `json:"detail,omitempty"`
}

type codexConsoleMemoryCandidate struct {
	ID           string                          `json:"id"`
	Title        string                          `json:"title"`
	Kind         string                          `json:"kind"`
	Content      string                          `json:"content"`
	SourcePath   string                          `json:"source_path,omitempty"`
	Confidence   float64                         `json:"confidence,omitempty"`
	ReviewStatus string                          `json:"review_status"`
	ReviewNote   string                          `json:"review_note,omitempty"`
	ReviewedAt   string                          `json:"reviewed_at,omitempty"`
	MemoryPath   string                          `json:"memory_path,omitempty"`
	Conflict     *codexConsoleMemoryConflictHint `json:"conflict,omitempty"`
}

type codexConsoleHandoverSummary struct {
	ID                   string                     `json:"id"`
	Project              string                     `json:"project"`
	Summary              string                     `json:"summary"`
	LatestActivity       string                     `json:"latest_activity,omitempty"`
	ThreadCount          int                        `json:"thread_count"`
	RunCount             int                        `json:"run_count"`
	ArtifactCount        int                        `json:"artifact_count"`
	MemoryCandidateCount int                        `json:"memory_candidate_count"`
	RecentThreads        []codexConsoleHandoverItem `json:"recent_threads,omitempty"`
	RecentRuns           []codexConsoleHandoverItem `json:"recent_runs,omitempty"`
	RecentArtifacts      []codexConsoleHandoverItem `json:"recent_artifacts,omitempty"`
	MemoryCandidates     []codexConsoleHandoverItem `json:"memory_candidates,omitempty"`
	Status               string                     `json:"status,omitempty"`
	Path                 string                     `json:"path,omitempty"`
	SavedAt              string                     `json:"saved_at,omitempty"`
	Version              int64                      `json:"version,omitempty"`
	SavedContent         string                     `json:"saved_content,omitempty"`
}

type codexConsoleHandoverItem struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Kind       string `json:"kind,omitempty"`
	Detail     string `json:"detail,omitempty"`
	At         string `json:"at,omitempty"`
	SourcePath string `json:"source_path,omitempty"`
}

type codexConsoleSkillCandidate struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Title           string   `json:"title"`
	Project         string   `json:"project,omitempty"`
	ThreadID        string   `json:"thread_id,omitempty"`
	ThreadTitle     string   `json:"thread_title,omitempty"`
	UpdatedAt       string   `json:"updated_at,omitempty"`
	SourcePath      string   `json:"source_path,omitempty"`
	Confidence      float64  `json:"confidence,omitempty"`
	ToolCalls       int      `json:"tool_calls"`
	ArtifactCount   int      `json:"artifact_count"`
	Signals         []string `json:"signals,omitempty"`
	Rationale       string   `json:"rationale,omitempty"`
	Draft           string   `json:"draft"`
	Status          string   `json:"status,omitempty"`
	StatusNote      string   `json:"status_note,omitempty"`
	StatusUpdatedAt string   `json:"status_updated_at,omitempty"`
	SkillPath       string   `json:"skill_path,omitempty"`
	SavedAt         string   `json:"saved_at,omitempty"`
	Edited          bool     `json:"edited,omitempty"`
	MetadataPath    string   `json:"metadata_path,omitempty"`
	ManifestPath    string   `json:"manifest_path,omitempty"`
}

type codexConsoleMemoryConflictHint struct {
	Status            string `json:"status"`
	Target            string `json:"target"`
	Category          string `json:"category"`
	Path              string `json:"path"`
	ExistingSource    string `json:"existing_source,omitempty"`
	ExistingUpdatedAt string `json:"existing_updated_at,omitempty"`
	ExistingContent   string `json:"existing_content,omitempty"`
	CandidateContent  string `json:"candidate_content,omitempty"`
	Message           string `json:"message"`
}

type codexConsoleMemorySyncRequest struct {
	IDs              []string          `json:"ids,omitempty"`
	All              bool              `json:"all,omitempty"`
	Target           string            `json:"target,omitempty"`
	Project          string            `json:"project,omitempty"`
	ContentOverrides map[string]string `json:"content_overrides,omitempty"`
}

type codexConsoleMemorySyncResponse struct {
	Target    string                         `json:"target"`
	Project   string                         `json:"project,omitempty"`
	Requested int                            `json:"requested"`
	Synced    int                            `json:"synced"`
	Skipped   int                            `json:"skipped"`
	Failed    int                            `json:"failed"`
	Items     []codexConsoleMemorySyncResult `json:"items"`
	Paths     []string                       `json:"paths,omitempty"`
}

type codexConsoleMemorySyncResult struct {
	ID       string `json:"id"`
	Title    string `json:"title,omitempty"`
	Category string `json:"category,omitempty"`
	Path     string `json:"path,omitempty"`
	Target   string `json:"target,omitempty"`
	Project  string `json:"project,omitempty"`
	Edited   bool   `json:"edited,omitempty"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}

type codexConsoleMemoryReviewRequest struct {
	IDs    []string `json:"ids,omitempty"`
	Status string   `json:"status"`
	Note   string   `json:"note,omitempty"`
}

type codexConsoleMemoryConflictResolveRequest struct {
	ID            string `json:"id"`
	Resolution    string `json:"resolution"`
	MergedContent string `json:"merged_content,omitempty"`
}

type codexConsoleSkillCandidateSaveRequest struct {
	ID            string `json:"id"`
	Overwrite     bool   `json:"overwrite,omitempty"`
	DraftOverride string `json:"draft_override,omitempty"`
}

type codexConsoleSkillCandidateAssignPreviewRequest struct {
	ID          string            `json:"id"`
	AgentIDs    []string          `json:"agent_ids,omitempty"`
	TargetRoots map[string]string `json:"target_roots,omitempty"`
}

type codexConsoleSkillCandidateStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

type codexConsoleArtifactRegistrySaveRequest struct {
	Overwrite bool `json:"overwrite,omitempty"`
}

type codexConsoleArtifactRegistrySaveResponse struct {
	Status        string `json:"status"`
	Path          string `json:"path"`
	SavedAt       string `json:"saved_at,omitempty"`
	ArtifactCount int    `json:"artifact_count"`
	ProjectCount  int    `json:"project_count"`
	Message       string `json:"message,omitempty"`
}

type codexConsoleHandoverSaveRequest struct {
	ID              string `json:"id"`
	Overwrite       bool   `json:"overwrite,omitempty"`
	ContentOverride string `json:"content_override,omitempty"`
}

type codexConsoleHandoverSaveResponse struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	Status  string `json:"status"`
	Path    string `json:"path"`
	SavedAt string `json:"saved_at,omitempty"`
	Version int64  `json:"version,omitempty"`
	Edited  bool   `json:"edited,omitempty"`
	Message string `json:"message,omitempty"`
}

type codexConsoleSkillCandidateSaveResponse struct {
	ID           string                       `json:"id"`
	Name         string                       `json:"name"`
	Status       string                       `json:"status"`
	SkillPath    string                       `json:"skill_path"`
	Path         string                       `json:"path"`
	MetadataPath string                       `json:"metadata_path"`
	ManifestPath string                       `json:"manifest_path"`
	SavedAt      string                       `json:"saved_at,omitempty"`
	Edited       bool                         `json:"edited,omitempty"`
	Files        []string                     `json:"files,omitempty"`
	Manifest     *skillsarchive.SkillManifest `json:"manifest,omitempty"`
	Message      string                       `json:"message,omitempty"`
}

type codexConsoleSkillCandidateAssignPreviewResponse struct {
	ID          string                  `json:"id"`
	Status      string                  `json:"status"`
	SkillPath   string                  `json:"skill_path"`
	AgentIDs    []string                `json:"agent_ids"`
	Assignments []skillAgentAssignment  `json:"assignments,omitempty"`
	SyncPreview *localSkillSyncResponse `json:"sync_preview,omitempty"`
	Message     string                  `json:"message,omitempty"`
}

type codexConsoleSkillCandidateStatusResponse struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	SkillPath       string `json:"skill_path"`
	MetadataPath    string `json:"metadata_path"`
	ManifestPath    string `json:"manifest_path"`
	StatusUpdatedAt string `json:"status_updated_at,omitempty"`
	Message         string `json:"message,omitempty"`
}

type codexConsoleMemoryConflictResolveResponse struct {
	ID            string `json:"id"`
	Title         string `json:"title,omitempty"`
	Category      string `json:"category"`
	Resolution    string `json:"resolution"`
	Status        string `json:"status"`
	Path          string `json:"path,omitempty"`
	ExistingPath  string `json:"existing_path,omitempty"`
	CandidatePath string `json:"candidate_path,omitempty"`
	ReviewPath    string `json:"review_path,omitempty"`
	Message       string `json:"message,omitempty"`
}

type codexConsoleMemoryReviewResponse struct {
	Path    string                           `json:"path"`
	Status  string                           `json:"status"`
	Updated int                              `json:"updated"`
	Failed  int                              `json:"failed"`
	Items   []codexConsoleMemoryReviewResult `json:"items"`
}

type codexConsoleMemoryReviewResult struct {
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type codexConsoleMemoryReviewState struct {
	Version   int                                     `json:"version"`
	UpdatedAt string                                  `json:"updated_at"`
	Items     map[string]codexConsoleMemoryReviewItem `json:"items"`
}

type codexConsoleMemoryReviewItem struct {
	ID         string `json:"id"`
	Title      string `json:"title,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Status     string `json:"status"`
	Note       string `json:"note,omitempty"`
	SourcePath string `json:"source_path,omitempty"`
	MemoryPath string `json:"memory_path,omitempty"`
	ReviewedAt string `json:"reviewed_at,omitempty"`
	SyncedAt   string `json:"synced_at,omitempty"`
}

type codexConsoleSavedSkillCandidateMetadata struct {
	Version         string   `json:"version"`
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Status          string   `json:"status"`
	SkillPath       string   `json:"skill_path"`
	Path            string   `json:"path"`
	Project         string   `json:"project,omitempty"`
	ThreadID        string   `json:"thread_id,omitempty"`
	ThreadTitle     string   `json:"thread_title,omitempty"`
	UpdatedAt       string   `json:"updated_at,omitempty"`
	SourcePath      string   `json:"source_path,omitempty"`
	Confidence      float64  `json:"confidence,omitempty"`
	ToolCalls       int      `json:"tool_calls,omitempty"`
	ArtifactCount   int      `json:"artifact_count,omitempty"`
	Signals         []string `json:"signals,omitempty"`
	Rationale       string   `json:"rationale,omitempty"`
	SavedAt         string   `json:"saved_at"`
	Edited          bool     `json:"edited,omitempty"`
	StatusNote      string   `json:"status_note,omitempty"`
	StatusUpdatedAt string   `json:"status_updated_at,omitempty"`
}

type codexConsoleSavedArtifactRegistry struct {
	Version          string                               `json:"version"`
	Source           string                               `json:"source"`
	SourcePlatform   string                               `json:"source_platform"`
	SavedAt          string                               `json:"saved_at"`
	ArtifactCount    int                                  `json:"artifact_count"`
	ProjectCount     int                                  `json:"project_count"`
	Projects         []string                             `json:"projects,omitempty"`
	ProjectSummaries []codexConsoleArtifactProjectSummary `json:"project_summaries,omitempty"`
	Artifacts        []codexConsoleArtifact               `json:"artifacts"`
}

type codexConsoleArtifactProjectSummary struct {
	Project          string                          `json:"project"`
	ArtifactCount    int                             `json:"artifact_count"`
	Roles            []codexConsoleArtifactRoleCount `json:"roles,omitempty"`
	PrimaryArtifacts []codexConsoleArtifactHandoff   `json:"primary_artifacts,omitempty"`
}

type codexConsoleArtifactRoleCount struct {
	Role  string `json:"role"`
	Count int    `json:"count"`
}

type codexConsoleArtifactHandoff struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Role             string `json:"role,omitempty"`
	HandoffNote      string `json:"handoff_note,omitempty"`
	AgentInstruction string `json:"agent_instruction,omitempty"`
}

var codexArtifactPathPattern = regexp.MustCompile(`(?m)(?:^|[\s"'(:]|\\r\\n|\\n|\\r|\\t)((?:/[^\\\s"')]+|[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+)\.(?:html|md|markdown|png|jpe?g|webp|pdf|pptx?|docx?|xlsx?|csv|json|txt))`)
var codexHookEnvPattern = regexp.MustCompile(`\b[A-Z][A-Z0-9_]{2,}\b`)
var codexHookWriteCommandPattern = regexp.MustCompile(`(?:^|[;&|]\s*)(?:rm|mv|cp|mkdir|touch)\b`)

const codexConsoleMemoryProfileMaxBytes = 64 * 1024
const codexConsoleMemoryReviewPath = "/platforms/codex/console/memory-review.json"
const codexConsoleSkillCandidateVersion = "vola.codex-skill-candidate/v1"
const codexConsoleSkillCandidateMetadataFile = "candidate.vola.json"
const codexConsoleHandoverVersion = "vola.codex-handover/v1"
const codexConsoleArtifactRegistryVersion = "vola.codex-artifact-registry/v1"
const codexConsoleArtifactRegistryPath = "/platforms/codex/console/artifacts.json"

func (s *Server) handleLocalCodexConsole(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "Codex Console is only available in local mode")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}

	cfg, err := loadRuntimeCLIConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	payload, err := platforms.PrepareAgentImportPayload(r.Context(), cfg, "codex")
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	resp := buildCodexConsoleResponse(payload)
	reviewState, err := s.loadCodexConsoleMemoryReviewState(r.Context(), userID)
	if err != nil {
		resp.Notes = append(resp.Notes, "Failed to load Codex Console memory review state: "+err.Error())
	} else {
		applyCodexConsoleMemoryReviewState(&resp, reviewState)
	}
	if err := s.applyCodexConsoleMemoryConflictHints(r.Context(), userID, &resp, payload.MemoryItems); err != nil {
		resp.Notes = append(resp.Notes, "Failed to preview Codex Console memory conflicts: "+err.Error())
	}
	if err := s.applyCodexConsoleArtifactRegistryState(r.Context(), userID, &resp); err != nil {
		resp.Notes = append(resp.Notes, "Failed to load Codex Console artifact registry state: "+err.Error())
	}
	if err := s.applyCodexConsoleHandoverState(r.Context(), userID, &resp); err != nil {
		resp.Notes = append(resp.Notes, "Failed to load Codex Console handover state: "+err.Error())
	}
	if err := s.applyCodexConsoleSkillCandidateState(r.Context(), userID, &resp); err != nil {
		resp.Notes = append(resp.Notes, "Failed to load Codex Console skill candidate state: "+err.Error())
	}
	resp.Overview = buildCodexConsoleOverview(resp, payload)
	respondOK(w, resp)
}

func (s *Server) handleLocalCodexConsoleMemorySync(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "Codex Console is only available in local mode")
		return
	}
	if s.MemoryService == nil {
		respondNotConfigured(w, "memory service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}

	var req codexConsoleMemorySyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	req.Target = codexConsoleMemorySyncTarget(req.Target)
	req.Project = strings.TrimSpace(req.Project)
	if req.Target == "project" {
		if s.ProjectService == nil {
			respondNotConfigured(w, "project service")
			return
		}
		if req.Project == "" {
			respondValidationError(w, "project", "project is required when target is project")
			return
		}
	} else if req.Target != "profile" {
		respondValidationError(w, "target", "target must be profile or project")
		return
	}
	if !req.All && len(req.IDs) == 0 {
		respondValidationError(w, "ids", "at least one memory candidate id is required")
		return
	}

	cfg, err := loadRuntimeCLIConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	payload, err := platforms.PrepareAgentImportPayload(r.Context(), cfg, "codex")
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	ctx := s.requestSourceContext(r, "codex-console")
	resp := s.syncCodexConsoleMemoryCandidates(ctx, userID, payload.MemoryItems, req)
	if resp.Synced > 0 {
		if err := s.markCodexConsoleMemorySynced(ctx, userID, payload.MemoryItems, resp.Items); err != nil {
			resp.Items = append(resp.Items, codexConsoleMemorySyncResult{
				Status:  "failed",
				Message: "synced memory but failed to update review state: " + err.Error(),
			})
			resp.Failed++
		}
	}
	respondOKWithLocalGitSync(w, resp, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleLocalCodexConsoleMemoryReview(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "Codex Console is only available in local mode")
		return
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}

	var req codexConsoleMemoryReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	req.Status = strings.TrimSpace(strings.ToLower(req.Status))
	if !isCodexConsoleMemoryReviewStatus(req.Status) {
		respondValidationError(w, "status", "status must be accepted, ignored, deferred, or review_required")
		return
	}
	if len(req.IDs) == 0 {
		respondValidationError(w, "ids", "at least one memory candidate id is required")
		return
	}

	cfg, err := loadRuntimeCLIConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	payload, err := platforms.PrepareAgentImportPayload(r.Context(), cfg, "codex")
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	ctx := s.requestSourceContext(r, "codex-console")
	resp, err := s.updateCodexConsoleMemoryReview(ctx, userID, payload.MemoryItems, req)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKWithLocalGitSync(w, resp, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleLocalCodexConsoleMemoryConflictResolve(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "Codex Console is only available in local mode")
		return
	}
	if s.MemoryService == nil {
		respondNotConfigured(w, "memory service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}

	var req codexConsoleMemoryConflictResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Resolution = normalizeCodexConsoleMemoryConflictResolution(req.Resolution)
	req.MergedContent = cleanCodexConsoleText(req.MergedContent)
	if req.ID == "" {
		respondValidationError(w, "id", "memory candidate id is required")
		return
	}
	if !isCodexConsoleMemoryConflictResolution(req.Resolution) {
		respondValidationError(w, "resolution", "resolution must be keep_existing, use_candidate, keep_both, or merge")
		return
	}

	cfg, err := loadRuntimeCLIConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	payload, err := platforms.PrepareAgentImportPayload(r.Context(), cfg, "codex")
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	item, candidate, ok := codexConsoleMemoryItemByID(payload.MemoryItems, req.ID)
	if !ok {
		respondError(w, http.StatusNotFound, ErrCodeNotFound, "memory candidate not found")
		return
	}
	category := codexConsoleMemoryProfileCategory(candidate)
	existing, ok, err := s.codexConsoleMemoryProfile(r.Context(), userID, category)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !ok {
		respondValidationError(w, "id", "profile memory conflict not found")
		return
	}
	candidateContent := strings.TrimSpace(renderCodexConsoleMemoryItem(item))
	if candidateContent == "" {
		respondValidationError(w, "id", "memory candidate is empty")
		return
	}
	if strings.TrimSpace(existing.Content) == candidateContent || strings.TrimSpace(existing.Source) == "agent:codex" {
		respondValidationError(w, "id", "memory candidate no longer conflicts with profile memory")
		return
	}

	ctx := s.requestSourceContext(r, "codex-console")
	resp, err := s.resolveCodexConsoleMemoryProfileConflict(ctx, userID, item, candidate, existing, req.Resolution, req.MergedContent)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKWithLocalGitSync(w, resp, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleLocalCodexConsoleSkillCandidateSave(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "Codex Console is only available in local mode")
		return
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}

	var req codexConsoleSkillCandidateSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		respondValidationError(w, "id", "skill candidate id is required")
		return
	}

	cfg, err := loadRuntimeCLIConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	payload, err := platforms.PrepareAgentImportPayload(r.Context(), cfg, "codex")
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	resp := buildCodexConsoleResponse(payload)
	candidate, ok := codexConsoleSkillCandidateByID(resp.SkillCandidates, req.ID)
	if !ok {
		respondError(w, http.StatusNotFound, ErrCodeNotFound, "skill candidate not found")
		return
	}
	saveResp, err := s.saveCodexConsoleSkillCandidate(s.requestSourceContext(r, "codex-console"), userID, candidate, req.Overwrite, req.DraftOverride)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKWithLocalGitSync(w, saveResp, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleLocalCodexConsoleSkillCandidateAssignPreview(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "Codex Console is only available in local mode")
		return
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}

	var req codexConsoleSkillCandidateAssignPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		respondValidationError(w, "id", "skill candidate id is required")
		return
	}
	agentIDs, err := normalizeCodexConsoleSkillCandidateAssignAgentIDs(req.AgentIDs)
	if err != nil {
		respondValidationError(w, "agent_ids", err.Error())
		return
	}

	cfg, err := loadRuntimeCLIConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	payload, err := platforms.PrepareAgentImportPayload(r.Context(), cfg, "codex")
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	console := buildCodexConsoleResponse(payload)
	if err := s.applyCodexConsoleSkillCandidateState(r.Context(), userID, &console); err != nil {
		respondInternalError(w, err)
		return
	}
	candidate, ok := codexConsoleSkillCandidateByID(console.SkillCandidates, req.ID)
	if !ok {
		respondError(w, http.StatusNotFound, ErrCodeNotFound, "skill candidate not found")
		return
	}
	assignResp, err := s.assignCodexConsoleSkillCandidateAndPreview(r.Context(), userID, candidate, agentIDs, req.TargetRoots)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKWithLocalGitSync(w, assignResp, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleLocalCodexConsoleSkillCandidateStatus(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "Codex Console is only available in local mode")
		return
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}

	var req codexConsoleSkillCandidateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		respondValidationError(w, "id", "skill candidate id is required")
		return
	}
	status, err := normalizeCodexConsoleSkillCandidateStatus(req.Status)
	if err != nil {
		respondValidationError(w, "status", err.Error())
		return
	}

	cfg, err := loadRuntimeCLIConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	payload, err := platforms.PrepareAgentImportPayload(r.Context(), cfg, "codex")
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	console := buildCodexConsoleResponse(payload)
	candidate, ok := codexConsoleSkillCandidateByID(console.SkillCandidates, req.ID)
	if !ok {
		respondError(w, http.StatusNotFound, ErrCodeNotFound, "skill candidate not found")
		return
	}
	resp, err := s.updateCodexConsoleSkillCandidateStatus(s.requestSourceContext(r, "codex-console"), userID, candidate, status, req.Note)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKWithLocalGitSync(w, resp, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleLocalCodexConsoleArtifactsSave(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "Codex Console is only available in local mode")
		return
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}

	var req codexConsoleArtifactRegistrySaveRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				req = codexConsoleArtifactRegistrySaveRequest{}
			} else {
				respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
				return
			}
		}
	}

	cfg, err := loadRuntimeCLIConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	payload, err := platforms.PrepareAgentImportPayload(r.Context(), cfg, "codex")
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	resp := buildCodexConsoleResponse(payload)
	saveResp, err := s.saveCodexConsoleArtifactRegistry(s.requestSourceContext(r, "codex-console"), userID, resp.Artifacts, req.Overwrite)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKWithLocalGitSync(w, saveResp, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleLocalCodexConsoleHandoverSave(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "Codex Console is only available in local mode")
		return
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}

	var req codexConsoleHandoverSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		respondValidationError(w, "id", "handover id is required")
		return
	}

	cfg, err := loadRuntimeCLIConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	payload, err := platforms.PrepareAgentImportPayload(r.Context(), cfg, "codex")
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	resp := buildCodexConsoleResponse(payload)
	handover, ok := codexConsoleHandoverByID(resp.Handovers, req.ID)
	if !ok {
		respondError(w, http.StatusNotFound, ErrCodeNotFound, "handover not found")
		return
	}
	saveResp, err := s.saveCodexConsoleHandover(s.requestSourceContext(r, "codex-console"), userID, handover, req.Overwrite, req.ContentOverride)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKWithLocalGitSync(w, saveResp, s.syncLocalGitMirror(r.Context(), userID))
}

func buildCodexConsoleResponse(payload sqlitestorage.AgentExportPayload) codexConsoleResponse {
	now := time.Now().UTC().Format(time.RFC3339)
	resp := codexConsoleResponse{
		Platform:          "codex",
		UpdatedAt:         now,
		SensitiveFindings: payload.SensitiveFindings,
		VaultCandidates:   payload.VaultCandidates,
		Notes:             payload.Notes,
	}
	if payload.Codex != nil {
		artifactSeen := map[string]bool{}
		threadIDs := map[string]struct{}{}
		for index, convo := range payload.Codex.Conversations {
			thread := buildCodexConsoleThread(index, convo)
			thread.ID = uniqueCodexConsoleThreadID(thread.ID, thread.SourcePath, index, threadIDs)
			threadArtifacts := collectCodexConsoleArtifacts(convo, thread, artifactSeen)
			thread.ArtifactCount = len(threadArtifacts)
			resp.Threads = append(resp.Threads, thread)
			resp.Artifacts = append(resp.Artifacts, threadArtifacts...)
			if goal := buildCodexConsoleGoal(thread, convo); goal.ID != "" {
				resp.Goals = append(resp.Goals, goal)
			}
			if run := buildCodexConsoleRun(thread, convo, len(threadArtifacts)); run.ID != "" {
				resp.Runs = append(resp.Runs, run)
			}
		}
		resp.Hooks = collectCodexConsoleHooks(payload.Codex.Bundles)
	}
	for _, record := range payload.Automations {
		resp.Automations = append(resp.Automations, buildCodexConsoleAutomation(record))
	}
	for _, item := range payload.MemoryItems {
		resp.MemoryCandidates = append(resp.MemoryCandidates, buildCodexConsoleMemoryCandidate(item))
	}
	sortCodexConsoleResponse(&resp)
	resp.Handovers = buildCodexConsoleHandovers(resp)
	resp.SkillCandidates = buildCodexConsoleSkillCandidates(resp)
	resp.Overview = buildCodexConsoleOverview(resp, payload)
	return resp
}

func (s *Server) syncCodexConsoleMemoryCandidates(ctx context.Context, userID uuid.UUID, items []sqlitestorage.AgentMemoryItem, req codexConsoleMemorySyncRequest) codexConsoleMemorySyncResponse {
	resp := codexConsoleMemorySyncResponse{
		Target:  codexConsoleMemorySyncResponseTarget(req),
		Project: strings.TrimSpace(req.Project),
	}
	requested := map[string]bool{}
	if req.All {
		resp.Requested = len(items)
	} else {
		resp.Requested = len(req.IDs)
		for _, id := range req.IDs {
			id = strings.TrimSpace(id)
			if id != "" {
				requested[id] = false
			}
		}
	}

	for _, item := range items {
		candidate := buildCodexConsoleMemoryCandidate(item)
		if candidate.ID == "" {
			continue
		}
		if !req.All {
			if _, ok := requested[candidate.ID]; !ok {
				continue
			}
			requested[candidate.ID] = true
		}
		result := s.syncCodexConsoleMemoryCandidate(ctx, userID, item, candidate, req)
		resp.Items = append(resp.Items, result)
		switch result.Status {
		case "synced":
			resp.Synced++
			if result.Path != "" {
				resp.Paths = append(resp.Paths, result.Path)
			}
		case "skipped":
			resp.Skipped++
		default:
			resp.Failed++
		}
	}

	for id, found := range requested {
		if found {
			continue
		}
		resp.Failed++
		resp.Items = append(resp.Items, codexConsoleMemorySyncResult{
			ID:      id,
			Status:  "failed",
			Message: "memory candidate not found",
		})
	}
	sort.Strings(resp.Paths)
	return resp
}

func (s *Server) markCodexConsoleMemorySynced(ctx context.Context, userID uuid.UUID, items []sqlitestorage.AgentMemoryItem, results []codexConsoleMemorySyncResult) error {
	if s.FileTreeService == nil {
		return nil
	}
	state, err := s.loadCodexConsoleMemoryReviewState(ctx, userID)
	if err != nil {
		return err
	}
	candidates := codexConsoleMemoryCandidateByID(items)
	now := time.Now().UTC().Format(time.RFC3339)
	changed := false
	for _, result := range results {
		if result.Status != "synced" || strings.TrimSpace(result.ID) == "" {
			continue
		}
		candidate, ok := candidates[result.ID]
		if !ok {
			continue
		}
		state.Items[result.ID] = codexConsoleMemoryReviewItem{
			ID:         result.ID,
			Title:      candidate.Title,
			Kind:       candidate.Kind,
			Status:     "synced",
			SourcePath: candidate.SourcePath,
			MemoryPath: result.Path,
			ReviewedAt: now,
			SyncedAt:   now,
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return s.saveCodexConsoleMemoryReviewState(ctx, userID, state)
}

func (s *Server) updateCodexConsoleMemoryReview(ctx context.Context, userID uuid.UUID, items []sqlitestorage.AgentMemoryItem, req codexConsoleMemoryReviewRequest) (codexConsoleMemoryReviewResponse, error) {
	resp := codexConsoleMemoryReviewResponse{
		Path:   codexConsoleMemoryReviewPath,
		Status: req.Status,
	}
	state, err := s.loadCodexConsoleMemoryReviewState(ctx, userID)
	if err != nil {
		return resp, err
	}
	candidates := codexConsoleMemoryCandidateByID(items)
	now := time.Now().UTC().Format(time.RFC3339)
	for _, rawID := range req.IDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		candidate, ok := candidates[id]
		if !ok {
			resp.Failed++
			resp.Items = append(resp.Items, codexConsoleMemoryReviewResult{
				ID:      id,
				Status:  "failed",
				Message: "memory candidate not found",
			})
			continue
		}
		if req.Status == "review_required" {
			delete(state.Items, id)
		} else {
			current := state.Items[id]
			current.ID = id
			current.Title = candidate.Title
			current.Kind = candidate.Kind
			current.Status = req.Status
			current.Note = truncateCodexConsoleText(req.Note, 500)
			current.SourcePath = candidate.SourcePath
			current.ReviewedAt = now
			if req.Status != "synced" {
				current.SyncedAt = ""
			}
			state.Items[id] = current
		}
		resp.Updated++
		resp.Items = append(resp.Items, codexConsoleMemoryReviewResult{
			ID:     id,
			Title:  candidate.Title,
			Status: req.Status,
		})
	}
	if resp.Updated > 0 {
		if err := s.saveCodexConsoleMemoryReviewState(ctx, userID, state); err != nil {
			return resp, err
		}
	}
	return resp, nil
}

func (s *Server) resolveCodexConsoleMemoryProfileConflict(ctx context.Context, userID uuid.UUID, item sqlitestorage.AgentMemoryItem, candidate codexConsoleMemoryCandidate, existing models.MemoryProfile, resolution string, mergedContent string) (codexConsoleMemoryConflictResolveResponse, error) {
	category := codexConsoleMemoryProfileCategory(candidate)
	existingPath := hubpath.ProfilePath(category)
	candidateContent := strings.TrimSpace(renderCodexConsoleMemoryItem(item))
	resp := codexConsoleMemoryConflictResolveResponse{
		ID:           candidate.ID,
		Title:        candidate.Title,
		Category:     category,
		Resolution:   resolution,
		ExistingPath: existingPath,
		ReviewPath:   codexConsoleMemoryReviewPath,
	}
	if len([]byte(candidateContent)) > codexConsoleMemoryProfileMaxBytes {
		resp.Status = "failed"
		resp.Message = "memory candidate is too large for profile memory"
		return resp, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	reviewStatus := "synced"
	targetPath := existingPath
	message := "Conflict resolved by using the Codex memory candidate."

	switch resolution {
	case "keep_existing":
		reviewStatus = "ignored"
		message = "Conflict resolved by keeping existing profile memory."
	case "use_candidate":
		if err := s.MemoryService.UpsertProfile(ctx, userID, category, candidateContent, "agent:codex"); err != nil {
			return resp, err
		}
	case "keep_both":
		targetCategory, err := s.uniqueCodexConsoleMemoryProfileCategory(ctx, userID, category)
		if err != nil {
			return resp, err
		}
		targetPath = hubpath.ProfilePath(targetCategory)
		resp.CandidatePath = targetPath
		if err := s.MemoryService.UpsertProfile(ctx, userID, targetCategory, candidateContent, "agent:codex"); err != nil {
			return resp, err
		}
		message = "Conflict resolved by keeping both profile memories."
	case "merge":
		merged := renderCodexConsoleMergedMemoryConflict(existing.Content, candidateContent, candidate, now)
		if strings.TrimSpace(mergedContent) != "" {
			merged = strings.TrimSpace(mergedContent)
		}
		if len([]byte(merged)) > codexConsoleMemoryProfileMaxBytes {
			resp.Status = "failed"
			resp.Message = "merged memory is too large for profile memory"
			return resp, nil
		}
		if err := s.MemoryService.UpsertProfile(ctx, userID, category, merged, "agent:codex"); err != nil {
			return resp, err
		}
		message = "Conflict resolved by merging existing profile memory with the Codex candidate."
	}

	state, err := s.loadCodexConsoleMemoryReviewState(ctx, userID)
	if err != nil {
		return resp, err
	}
	current := state.Items[candidate.ID]
	current.ID = candidate.ID
	current.Title = candidate.Title
	current.Kind = candidate.Kind
	current.Status = reviewStatus
	current.Note = message
	current.SourcePath = candidate.SourcePath
	current.MemoryPath = targetPath
	current.ReviewedAt = now
	if reviewStatus == "synced" {
		current.SyncedAt = now
	} else {
		current.SyncedAt = ""
	}
	state.Items[candidate.ID] = current
	if err := s.saveCodexConsoleMemoryReviewState(ctx, userID, state); err != nil {
		return resp, err
	}

	resp.Status = reviewStatus
	resp.Path = targetPath
	resp.Message = message
	return resp, nil
}

func (s *Server) codexConsoleMemoryProfile(ctx context.Context, userID uuid.UUID, category string) (models.MemoryProfile, bool, error) {
	profiles, err := s.MemoryService.GetProfile(ctx, userID)
	if err != nil {
		return models.MemoryProfile{}, false, err
	}
	for _, profile := range profiles {
		if strings.TrimSpace(profile.Category) == category {
			return profile, true, nil
		}
	}
	return models.MemoryProfile{}, false, nil
}

func (s *Server) uniqueCodexConsoleMemoryProfileCategory(ctx context.Context, userID uuid.UUID, baseCategory string) (string, error) {
	profiles, err := s.MemoryService.GetProfile(ctx, userID)
	if err != nil {
		return "", err
	}
	used := map[string]bool{}
	for _, profile := range profiles {
		used[strings.TrimSpace(profile.Category)] = true
	}
	stem := strings.Trim(strings.TrimSpace(baseCategory), "-")
	if stem == "" {
		stem = "codex-memory"
	}
	for index := 0; index < 100; index++ {
		suffix := "-codex"
		if index > 0 {
			suffix = fmt.Sprintf("-codex-%d", index+1)
		}
		limit := 128 - len(suffix)
		nextStem := stem
		if len(nextStem) > limit {
			nextStem = strings.Trim(nextStem[:limit], "-")
		}
		candidate := nextStem + suffix
		if !used[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not allocate a separate profile memory category for %s", baseCategory)
}

func renderCodexConsoleMergedMemoryConflict(existingContent, candidateContent string, candidate codexConsoleMemoryCandidate, resolvedAt string) string {
	title := strings.TrimSpace(candidate.Title)
	if title == "" {
		title = "Codex memory"
	}
	lines := []string{
		"# Merged Codex memory: " + title,
		"",
		"## Existing profile memory",
		"",
		strings.TrimSpace(existingContent),
		"",
		"## Codex candidate",
		"",
		strings.TrimSpace(candidateContent),
		"",
		"- Conflict resolution: merged by Codex Console",
		"- Memory candidate: " + candidate.ID,
		"- Resolved at: " + resolvedAt,
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func codexConsoleSkillCandidateByID(candidates []codexConsoleSkillCandidate, id string) (codexConsoleSkillCandidate, bool) {
	id = strings.TrimSpace(id)
	for _, candidate := range candidates {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return codexConsoleSkillCandidate{}, false
}

func normalizeCodexConsoleSkillCandidateAssignAgentIDs(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{"codex"}, nil
	}
	requested := map[string]struct{}{}
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if localSkillAgentByID(id) == nil {
			return nil, fmt.Errorf("unknown agent_id %q", id)
		}
		requested[id] = struct{}{}
	}
	if len(requested) == 0 {
		return nil, fmt.Errorf("agent_ids must include at least one supported agent")
	}
	out := []string{}
	for _, target := range skillAgentTargets {
		if _, ok := requested[target.ID]; ok {
			out = append(out, target.ID)
		}
	}
	return out, nil
}

func normalizeCodexConsoleSkillCandidateStatus(value string) (string, error) {
	status := strings.ToLower(strings.TrimSpace(value))
	switch status {
	case "draft", "ready", "archived":
		return status, nil
	default:
		return "", fmt.Errorf("status must be draft, ready, or archived")
	}
}

func (s *Server) assignCodexConsoleSkillCandidateAndPreview(ctx context.Context, userID uuid.UUID, candidate codexConsoleSkillCandidate, agentIDs []string, targetRoots map[string]string) (codexConsoleSkillCandidateAssignPreviewResponse, error) {
	skillPath := normalizeAssignedSkillPath(candidate.SkillPath)
	resp := codexConsoleSkillCandidateAssignPreviewResponse{
		ID:        candidate.ID,
		Status:    "not_saved",
		SkillPath: skillPath,
		AgentIDs:  append([]string{}, agentIDs...),
		Message:   "save the skill candidate before assigning it to an Agent",
	}
	if skillPath == "" || skillPath == "/skills" {
		return resp, nil
	}
	if _, err := s.FileTreeService.Read(ctx, userID, path.Join(skillPath, "SKILL.md"), models.TrustLevelFull); err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			return resp, nil
		}
		return resp, err
	}

	doc, err := s.readSkillAssignmentsFromContext(ctx, userID)
	if err != nil {
		return resp, err
	}
	byAgent := map[string]map[string]struct{}{}
	for _, item := range normalizeSkillAssignments(doc.Assignments) {
		if byAgent[item.AgentID] == nil {
			byAgent[item.AgentID] = map[string]struct{}{}
		}
		for _, itemSkillPath := range item.SkillPaths {
			if normalized := normalizeAssignedSkillPath(itemSkillPath); normalized != "" {
				byAgent[item.AgentID][normalized] = struct{}{}
			}
		}
	}
	for _, agentID := range agentIDs {
		if byAgent[agentID] == nil {
			byAgent[agentID] = map[string]struct{}{}
		}
		byAgent[agentID][skillPath] = struct{}{}
	}

	assignments := make([]skillAgentAssignment, 0, len(skillAgentTargets))
	for _, target := range skillAgentTargets {
		assignments = append(assignments, skillAgentAssignment{
			AgentID:    target.ID,
			SkillPaths: sortedStringSet(byAgent[target.ID]),
		})
	}
	assignments = normalizeSkillAssignments(assignments)
	nextDoc := skillAssignmentsDocument{
		Version:     skillAssignmentsVersion,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Assignments: assignments,
	}
	data, err := json.MarshalIndent(nextDoc, "", "  ")
	if err != nil {
		return resp, err
	}
	data = append(data, '\n')
	if _, err := s.FileTreeService.WriteEntry(ctx, userID, skillAssignmentsPath, string(data), "application/json", models.FileTreeWriteOptions{
		Kind: "skill_assignment",
		Metadata: map[string]interface{}{
			"source":       "codex-console",
			"capture_mode": "skill-candidate-assignment",
			"candidate_id": candidate.ID,
			"skill_path":   skillPath,
			"agent_ids":    agentIDs,
		},
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		return resp, err
	}

	preview, err := s.buildLocalSkillSyncResponse(ctx, scopedHubTarget{Scope: "personal", UserID: userID}, localSkillSyncRequest{
		AgentIDs:    agentIDs,
		TargetRoots: targetRoots,
	}, false, false)
	if err != nil {
		return resp, err
	}
	resp.Status = "assigned"
	resp.Assignments = assignments
	resp.SyncPreview = preview
	resp.Message = "skill candidate assigned in Vola; preview generated without writing local Agent directories"
	return resp, nil
}

func (s *Server) updateCodexConsoleSkillCandidateStatus(ctx context.Context, userID uuid.UUID, candidate codexConsoleSkillCandidate, status string, note string) (codexConsoleSkillCandidateStatusResponse, error) {
	target := codexConsoleSkillCandidateTarget(candidate)
	resp := codexConsoleSkillCandidateStatusResponse{
		ID:           candidate.ID,
		Status:       "not_saved",
		SkillPath:    target.SkillPath,
		MetadataPath: target.MetadataPath,
		ManifestPath: target.ManifestPath,
		Message:      "save the skill candidate before changing review status",
	}
	entry, err := s.FileTreeService.Read(ctx, userID, target.MetadataPath, models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			return resp, nil
		}
		return resp, err
	}
	var meta codexConsoleSavedSkillCandidateMetadata
	if err := json.Unmarshal([]byte(entry.Content), &meta); err != nil {
		return resp, err
	}
	if meta.ID != candidate.ID {
		resp.Status = "failed"
		resp.Message = "saved candidate metadata does not match this candidate"
		return resp, nil
	}
	draftEntry, err := s.FileTreeService.Read(ctx, userID, target.MarkdownPath, models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			resp.Status = "failed"
			resp.Message = "saved SKILL.md is missing"
			return resp, nil
		}
		return resp, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	meta.Status = status
	meta.StatusNote = strings.TrimSpace(note)
	meta.StatusUpdatedAt = now
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return resp, err
	}
	metaData = append(metaData, '\n')

	manifestEntries := []skillsarchive.Entry{
		{SkillName: target.ManifestSkillName, RelPath: "SKILL.md", Data: []byte(draftEntry.Content)},
		{SkillName: target.ManifestSkillName, RelPath: codexConsoleSkillCandidateMetadataFile, Data: metaData},
	}
	manifests := skillsarchive.BuildManifests(manifestEntries, "codex", "codex-console-skill-candidate-"+candidate.ID)
	if len(manifests) == 0 {
		resp.Status = "failed"
		resp.Message = "failed to rebuild skill manifest"
		return resp, nil
	}
	manifest := manifests[0]
	manifest.Warnings = append(manifest.Warnings, skillsarchive.SkillManifestWarning{
		Code:     "codex_console_candidate",
		Severity: "info",
		Message:  "Generated from Codex Console; review status is " + status + ".",
	})
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return resp, err
	}
	manifestData = append(manifestData, '\n')

	fileMetadata := map[string]interface{}{
		"source":           "codex-console",
		"source_platform":  "codex",
		"candidate_id":     candidate.ID,
		"candidate_status": status,
		"thread_id":        candidate.ThreadID,
		"project":          candidate.Project,
		"edited":           meta.Edited,
	}
	if _, err := s.FileTreeService.WriteEntry(ctx, userID, target.MetadataPath, string(metaData), "application/json", models.FileTreeWriteOptions{
		Kind:          "skill_file",
		Metadata:      fileMetadata,
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		return resp, err
	}
	if _, err := s.FileTreeService.WriteEntry(ctx, userID, target.ManifestPath, string(manifestData), "application/json", models.FileTreeWriteOptions{
		Kind:          "skill_file",
		Metadata:      fileMetadata,
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		return resp, err
	}

	resp.Status = status
	resp.StatusUpdatedAt = now
	resp.Message = "skill candidate review status updated"
	return resp, nil
}

func (s *Server) saveCodexConsoleSkillCandidate(ctx context.Context, userID uuid.UUID, candidate codexConsoleSkillCandidate, overwrite bool, draftOverride string) (codexConsoleSkillCandidateSaveResponse, error) {
	target := codexConsoleSkillCandidateTarget(candidate)
	resp := codexConsoleSkillCandidateSaveResponse{
		ID:           candidate.ID,
		Name:         candidate.Name,
		Status:       "draft",
		SkillPath:    target.SkillPath,
		Path:         target.MarkdownPath,
		MetadataPath: target.MetadataPath,
		ManifestPath: target.ManifestPath,
		Files:        []string{target.MarkdownPath, target.MetadataPath, target.ManifestPath},
	}
	if strings.TrimSpace(candidate.ID) == "" {
		resp.Status = "failed"
		resp.Message = "skill candidate id is required"
		return resp, nil
	}
	if strings.TrimSpace(candidate.Draft) == "" {
		resp.Status = "failed"
		resp.Message = "skill candidate draft is empty"
		return resp, nil
	}
	draft := strings.TrimSpace(candidate.Draft)
	edited := false
	if strings.TrimSpace(draftOverride) != "" {
		draft = strings.TrimSpace(draftOverride)
		edited = draft != strings.TrimSpace(candidate.Draft)
	}

	if !overwrite {
		if existing, err := s.FileTreeService.Read(ctx, userID, target.MetadataPath, models.TrustLevelFull); err == nil {
			var meta codexConsoleSavedSkillCandidateMetadata
			if json.Unmarshal([]byte(existing.Content), &meta) == nil && meta.ID == candidate.ID {
				resp.Status = "saved"
				resp.SavedAt = meta.SavedAt
				resp.Message = "skill candidate already saved"
				return resp, nil
			}
			resp.Status = "exists"
			resp.Message = "target candidate metadata already exists; review it before overwriting"
			return resp, nil
		} else if err != nil && !errors.Is(err, services.ErrEntryNotFound) {
			return resp, err
		}
		if _, err := s.FileTreeService.Read(ctx, userID, target.MarkdownPath, models.TrustLevelFull); err == nil {
			resp.Status = "exists"
			resp.Message = "target SKILL.md already exists; review it before overwriting"
			return resp, nil
		} else if err != nil && !errors.Is(err, services.ErrEntryNotFound) {
			return resp, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	meta := codexConsoleSavedSkillCandidateMetadata{
		Version:       codexConsoleSkillCandidateVersion,
		ID:            candidate.ID,
		Name:          candidate.Name,
		Status:        "draft",
		SkillPath:     target.SkillPath,
		Path:          target.MarkdownPath,
		Project:       candidate.Project,
		ThreadID:      candidate.ThreadID,
		ThreadTitle:   candidate.ThreadTitle,
		UpdatedAt:     candidate.UpdatedAt,
		SourcePath:    candidate.SourcePath,
		Confidence:    candidate.Confidence,
		ToolCalls:     candidate.ToolCalls,
		ArtifactCount: candidate.ArtifactCount,
		Signals:       append([]string{}, candidate.Signals...),
		Rationale:     candidate.Rationale,
		SavedAt:       now,
		Edited:        edited,
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return resp, err
	}
	metaData = append(metaData, '\n')

	draft = draft + "\n"
	manifestEntries := []skillsarchive.Entry{
		{SkillName: target.ManifestSkillName, RelPath: "SKILL.md", Data: []byte(draft)},
		{SkillName: target.ManifestSkillName, RelPath: codexConsoleSkillCandidateMetadataFile, Data: metaData},
	}
	manifests := skillsarchive.BuildManifests(manifestEntries, "codex", "codex-console-skill-candidate-"+candidate.ID)
	if len(manifests) == 0 {
		resp.Status = "failed"
		resp.Message = "failed to build skill manifest"
		return resp, nil
	}
	manifest := manifests[0]
	manifest.Warnings = append(manifest.Warnings, skillsarchive.SkillManifestWarning{
		Code:     "codex_console_candidate",
		Severity: "info",
		Message:  "Generated from Codex Console as a draft; review before assigning to an Agent.",
	})
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return resp, err
	}
	manifestData = append(manifestData, '\n')

	metadata := map[string]interface{}{
		"source":           "codex-console",
		"source_platform":  "codex",
		"candidate_id":     candidate.ID,
		"candidate_status": "draft",
		"thread_id":        candidate.ThreadID,
		"project":          candidate.Project,
		"edited":           edited,
	}
	if _, err := s.FileTreeService.WriteEntry(ctx, userID, target.MarkdownPath, draft, "text/markdown", models.FileTreeWriteOptions{
		Kind:          "skill_file",
		Metadata:      metadata,
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		return resp, err
	}
	if _, err := s.FileTreeService.WriteEntry(ctx, userID, target.MetadataPath, string(metaData), "application/json", models.FileTreeWriteOptions{
		Kind:          "skill_file",
		Metadata:      metadata,
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		return resp, err
	}
	if _, err := s.FileTreeService.WriteEntry(ctx, userID, target.ManifestPath, string(manifestData), "application/json", models.FileTreeWriteOptions{
		Kind:          "skill_file",
		Metadata:      metadata,
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		return resp, err
	}

	resp.Status = "saved"
	resp.SavedAt = now
	resp.Edited = edited
	resp.Manifest = &manifest
	resp.Message = "skill candidate saved as a Vola draft"
	return resp, nil
}

type codexConsoleSkillCandidateTargetPaths struct {
	Slug              string
	ManifestSkillName string
	SkillPath         string
	MarkdownPath      string
	MetadataPath      string
	ManifestPath      string
}

func codexConsoleSkillCandidateTarget(candidate codexConsoleSkillCandidate) codexConsoleSkillCandidateTargetPaths {
	base := normalizeCodexConsoleID(firstNonEmptyConsoleString(candidate.Name, candidate.ThreadTitle, candidate.Title, "codex-skill"))
	if base == "" || base == "item" {
		base = "codex-skill"
	}
	if len(base) > 56 {
		base = strings.Trim(base[:56], "-")
	}
	suffix := codexConsoleIDHash(candidate.ID)
	slug := strings.Trim(base+"-"+suffix, "-")
	skillPath := path.Join("/skills/_candidates", slug)
	return codexConsoleSkillCandidateTargetPaths{
		Slug:              slug,
		ManifestSkillName: slug,
		SkillPath:         skillPath,
		MarkdownPath:      path.Join(skillPath, "SKILL.md"),
		MetadataPath:      path.Join(skillPath, codexConsoleSkillCandidateMetadataFile),
		ManifestPath:      path.Join(skillPath, skillsarchive.ManifestFile),
	}
}

func (s *Server) applyCodexConsoleSkillCandidateState(ctx context.Context, userID uuid.UUID, resp *codexConsoleResponse) error {
	if s.FileTreeService == nil || len(resp.SkillCandidates) == 0 {
		return nil
	}
	for index := range resp.SkillCandidates {
		candidate := &resp.SkillCandidates[index]
		target := codexConsoleSkillCandidateTarget(*candidate)
		entry, err := s.FileTreeService.Read(ctx, userID, target.MetadataPath, models.TrustLevelFull)
		if err != nil {
			if errors.Is(err, services.ErrEntryNotFound) {
				continue
			}
			return err
		}
		var meta codexConsoleSavedSkillCandidateMetadata
		if err := json.Unmarshal([]byte(entry.Content), &meta); err != nil || meta.ID != candidate.ID {
			continue
		}
		candidate.Status = firstNonEmptyConsoleString(meta.Status, "draft")
		candidate.StatusNote = meta.StatusNote
		candidate.StatusUpdatedAt = meta.StatusUpdatedAt
		candidate.SkillPath = firstNonEmptyConsoleString(meta.SkillPath, target.SkillPath)
		candidate.SavedAt = meta.SavedAt
		candidate.Edited = meta.Edited
		candidate.MetadataPath = target.MetadataPath
		candidate.ManifestPath = target.ManifestPath
		if savedDraft, err := s.FileTreeService.Read(ctx, userID, target.MarkdownPath, models.TrustLevelFull); err == nil {
			candidate.Draft = savedDraft.Content
		} else if err != nil && !errors.Is(err, services.ErrEntryNotFound) {
			return err
		}
	}
	return nil
}

func (s *Server) saveCodexConsoleArtifactRegistry(ctx context.Context, userID uuid.UUID, artifacts []codexConsoleArtifact, overwrite bool) (codexConsoleArtifactRegistrySaveResponse, error) {
	resp := codexConsoleArtifactRegistrySaveResponse{
		Status:        "saved",
		Path:          codexConsoleArtifactRegistryPath,
		ArtifactCount: len(artifacts),
		ProjectCount:  len(codexConsoleArtifactProjects(artifacts)),
	}
	if len(artifacts) == 0 {
		resp.Status = "failed"
		resp.Message = "artifact registry is empty"
		return resp, nil
	}
	if !overwrite {
		if entry, err := s.FileTreeService.Read(ctx, userID, codexConsoleArtifactRegistryPath, models.TrustLevelFull); err == nil {
			var registry codexConsoleSavedArtifactRegistry
			if json.Unmarshal([]byte(entry.Content), &registry) == nil && registry.Version == codexConsoleArtifactRegistryVersion {
				resp.Status = "saved"
				resp.SavedAt = registry.SavedAt
				resp.ArtifactCount = registry.ArtifactCount
				resp.ProjectCount = registry.ProjectCount
				resp.Message = "artifact registry already saved"
				return resp, nil
			}
			resp.Status = "exists"
			resp.Message = "artifact registry already exists; review it before overwriting"
			return resp, nil
		} else if err != nil && !errors.Is(err, services.ErrEntryNotFound) {
			return resp, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	registry := codexConsoleSavedArtifactRegistry{
		Version:          codexConsoleArtifactRegistryVersion,
		Source:           "codex-console",
		SourcePlatform:   "codex",
		SavedAt:          now,
		ArtifactCount:    len(artifacts),
		ProjectCount:     len(codexConsoleArtifactProjects(artifacts)),
		Projects:         codexConsoleArtifactProjects(artifacts),
		ProjectSummaries: codexConsoleArtifactProjectSummaries(artifacts),
		Artifacts:        append([]codexConsoleArtifact{}, artifacts...),
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return resp, err
	}
	data = append(data, '\n')
	metadata := map[string]interface{}{
		"source":          "codex-console",
		"source_platform": "codex",
		"version":         codexConsoleArtifactRegistryVersion,
		"artifact_count":  len(artifacts),
		"project_count":   registry.ProjectCount,
	}
	if _, err := s.FileTreeService.WriteEntry(ctx, userID, codexConsoleArtifactRegistryPath, string(data), "application/json", models.FileTreeWriteOptions{
		Kind:          "artifact_registry",
		Metadata:      metadata,
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		return resp, err
	}
	resp.SavedAt = now
	resp.Message = "artifact registry saved as a Vola console file"
	return resp, nil
}

func (s *Server) applyCodexConsoleArtifactRegistryState(ctx context.Context, userID uuid.UUID, resp *codexConsoleResponse) error {
	resp.ArtifactRegistry = codexConsoleArtifactRegistrySummary{
		Status:        "not_saved",
		Path:          codexConsoleArtifactRegistryPath,
		ArtifactCount: len(resp.Artifacts),
		ProjectCount:  len(codexConsoleArtifactProjects(resp.Artifacts)),
	}
	if s.FileTreeService == nil {
		return nil
	}
	entry, err := s.FileTreeService.Read(ctx, userID, codexConsoleArtifactRegistryPath, models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			return nil
		}
		return err
	}
	var registry codexConsoleSavedArtifactRegistry
	if err := json.Unmarshal([]byte(entry.Content), &registry); err != nil {
		resp.ArtifactRegistry.Status = "invalid"
		resp.ArtifactRegistry.SavedAt = entry.UpdatedAt.UTC().Format(time.RFC3339)
		return nil
	}
	if registry.Version != codexConsoleArtifactRegistryVersion {
		resp.ArtifactRegistry.Status = "unsupported"
		resp.ArtifactRegistry.SavedAt = entry.UpdatedAt.UTC().Format(time.RFC3339)
		return nil
	}
	resp.ArtifactRegistry.Status = "saved"
	resp.ArtifactRegistry.SavedAt = firstNonEmptyConsoleString(registry.SavedAt, entry.UpdatedAt.UTC().Format(time.RFC3339))
	resp.ArtifactRegistry.ArtifactCount = registry.ArtifactCount
	resp.ArtifactRegistry.ProjectCount = registry.ProjectCount
	resp.ArtifactRegistry.ProjectSummaries = registry.ProjectSummaries
	return nil
}

func codexConsoleArtifactProjects(artifacts []codexConsoleArtifact) []string {
	seen := map[string]bool{}
	for _, artifact := range artifacts {
		project := strings.TrimSpace(artifact.Project)
		if project == "" {
			project = "unassigned"
		}
		seen[project] = true
	}
	projects := make([]string, 0, len(seen))
	for project := range seen {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	return projects
}

func codexConsoleArtifactProjectSummaries(artifacts []codexConsoleArtifact) []codexConsoleArtifactProjectSummary {
	byProject := map[string][]codexConsoleArtifact{}
	for _, artifact := range artifacts {
		project := strings.TrimSpace(artifact.Project)
		if project == "" {
			project = "unassigned"
		}
		byProject[project] = append(byProject[project], artifact)
	}
	projects := make([]string, 0, len(byProject))
	for project := range byProject {
		projects = append(projects, project)
	}
	sort.Strings(projects)

	summaries := make([]codexConsoleArtifactProjectSummary, 0, len(projects))
	for _, project := range projects {
		items := byProject[project]
		roleCounts := map[string]int{}
		for _, item := range items {
			role := strings.TrimSpace(item.Role)
			if role == "" {
				role = "file-reference"
			}
			roleCounts[role]++
		}
		roles := make([]codexConsoleArtifactRoleCount, 0, len(roleCounts))
		for role, count := range roleCounts {
			roles = append(roles, codexConsoleArtifactRoleCount{Role: role, Count: count})
		}
		sort.Slice(roles, func(i, j int) bool {
			if roles[i].Count == roles[j].Count {
				return roles[i].Role < roles[j].Role
			}
			return roles[i].Count > roles[j].Count
		})

		summaries = append(summaries, codexConsoleArtifactProjectSummary{
			Project:          project,
			ArtifactCount:    len(items),
			Roles:            roles,
			PrimaryArtifacts: codexConsoleArtifactPrimaryHandoffs(items),
		})
	}
	return summaries
}

func codexConsoleArtifactPrimaryHandoffs(artifacts []codexConsoleArtifact) []codexConsoleArtifactHandoff {
	items := append([]codexConsoleArtifact{}, artifacts...)
	sort.SliceStable(items, func(i, j int) bool {
		leftRank := codexConsoleArtifactRoleRank(items[i].Role)
		rightRank := codexConsoleArtifactRoleRank(items[j].Role)
		if leftRank == rightRank {
			return items[i].Name < items[j].Name
		}
		return leftRank < rightRank
	})
	limit := min(len(items), 5)
	handoffs := make([]codexConsoleArtifactHandoff, 0, limit)
	for _, item := range items[:limit] {
		handoffs = append(handoffs, codexConsoleArtifactHandoff{
			ID:               item.ID,
			Name:             item.Name,
			Role:             item.Role,
			HandoffNote:      item.HandoffNote,
			AgentInstruction: item.AgentInstruction,
		})
	}
	return handoffs
}

func codexConsoleArtifactRoleRank(role string) int {
	switch strings.TrimSpace(role) {
	case "handoff-document":
		return 0
	case "preview-output":
		return 1
	case "visual-evidence":
		return 2
	case "structured-data":
		return 3
	case "run-evidence":
		return 4
	case "attachment":
		return 5
	default:
		return 6
	}
}

func codexConsoleHandoverByID(handovers []codexConsoleHandoverSummary, id string) (codexConsoleHandoverSummary, bool) {
	id = strings.TrimSpace(id)
	for _, handover := range handovers {
		if handover.ID == id {
			return handover, true
		}
	}
	return codexConsoleHandoverSummary{}, false
}

func (s *Server) saveCodexConsoleHandover(ctx context.Context, userID uuid.UUID, handover codexConsoleHandoverSummary, _ bool, contentOverride string) (codexConsoleHandoverSaveResponse, error) {
	targetPath := codexConsoleHandoverPath(handover.Project)
	resp := codexConsoleHandoverSaveResponse{
		ID:      handover.ID,
		Project: handover.Project,
		Status:  "saved",
		Path:    targetPath,
	}
	if strings.TrimSpace(handover.ID) == "" {
		resp.Status = "failed"
		resp.Message = "handover id is required"
		return resp, nil
	}
	if strings.TrimSpace(handover.Project) == "" {
		resp.Status = "failed"
		resp.Message = "handover project is required"
		return resp, nil
	}

	existing := ""
	if entry, err := s.FileTreeService.Read(ctx, userID, targetPath, models.TrustLevelFull); err == nil {
		existing = entry.Content
	} else if err != nil && !errors.Is(err, services.ErrEntryNotFound) {
		return resp, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	content := renderCodexConsoleHandoverMarkdown(handover, now, existing)
	if strings.TrimSpace(contentOverride) != "" {
		content = normalizeCodexConsoleHandoverOverride(handover, contentOverride)
		resp.Edited = true
	}
	if strings.TrimSpace(content) == "" {
		resp.Status = "failed"
		resp.Message = "handover content is empty"
		return resp, nil
	}
	metadata := map[string]interface{}{
		"source":          "codex-console",
		"source_platform": "codex",
		"version":         codexConsoleHandoverVersion,
		"handover_id":     handover.ID,
		"project":         handover.Project,
		"latest_activity": handover.LatestActivity,
		"edited":          resp.Edited,
	}
	entry, err := s.FileTreeService.WriteEntry(ctx, userID, targetPath, content, "text/markdown", models.FileTreeWriteOptions{
		Kind:          "project_handover",
		Metadata:      metadata,
		MinTrustLevel: models.TrustLevelWork,
	})
	if err != nil {
		return resp, err
	}
	resp.SavedAt = now
	if entry != nil {
		resp.Version = entry.Version
	}
	if resp.Edited {
		resp.Message = "edited handover saved as a Vola project file"
	} else {
		resp.Message = "handover saved as a Vola project file"
	}
	return resp, nil
}

func (s *Server) applyCodexConsoleHandoverState(ctx context.Context, userID uuid.UUID, resp *codexConsoleResponse) error {
	if s.FileTreeService == nil || len(resp.Handovers) == 0 {
		return nil
	}
	for index := range resp.Handovers {
		handover := &resp.Handovers[index]
		targetPath := codexConsoleHandoverPath(handover.Project)
		entry, err := s.FileTreeService.Read(ctx, userID, targetPath, models.TrustLevelFull)
		if err != nil {
			if errors.Is(err, services.ErrEntryNotFound) {
				continue
			}
			return err
		}
		if !strings.Contains(entry.Content, codexConsoleHandoverMarker(handover.ID)) {
			continue
		}
		handover.Status = "saved"
		handover.Path = targetPath
		handover.SavedAt = entry.UpdatedAt.UTC().Format(time.RFC3339)
		handover.Version = entry.Version
		handover.SavedContent = entry.Content
	}
	return nil
}

func codexConsoleHandoverPath(project string) string {
	return path.Join(hubpath.ProjectDir(project), "handover.md")
}

func codexConsoleHandoverMarker(id string) string {
	return fmt.Sprintf("<!-- codex-console-handover:%s -->", normalizeCodexConsoleID(id))
}

func normalizeCodexConsoleHandoverOverride(handover codexConsoleHandoverSummary, content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	marker := codexConsoleHandoverMarker(handover.ID)
	if strings.Contains(content, marker) {
		return content + "\n"
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "#") {
		out := []string{lines[0], "", marker}
		if len(lines) > 1 {
			out = append(out, lines[1:]...)
		}
		return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
	}
	return strings.TrimSpace(strings.Join([]string{
		"# Codex handover: " + codexConsoleMarkdownLine(handover.Project, "unassigned"),
		"",
		marker,
		"",
		content,
	}, "\n")) + "\n"
}

func renderCodexConsoleHandoverMarkdown(handover codexConsoleHandoverSummary, savedAt string, existing string) string {
	manualNotes := codexConsoleHandoverManualNotes(existing)
	lines := []string{
		"# Codex handover: " + codexConsoleMarkdownLine(handover.Project, "unassigned"),
		"",
		codexConsoleHandoverMarker(handover.ID),
		"",
		"Generated by Vola Codex Console at " + savedAt + ".",
		"",
		"## Project status",
		"",
		"- Summary: " + codexConsoleMarkdownLine(handover.Summary, "No summary."),
		"- Latest activity: " + codexConsoleMarkdownLine(handover.LatestActivity, "unknown"),
		fmt.Sprintf("- Threads: %d", handover.ThreadCount),
		fmt.Sprintf("- Runs: %d", handover.RunCount),
		fmt.Sprintf("- Artifacts: %d", handover.ArtifactCount),
		fmt.Sprintf("- Memory candidates: %d", handover.MemoryCandidateCount),
		"",
	}
	appendCodexConsoleHandoverGroup(&lines, "Recent threads", handover.RecentThreads, "No recent threads.")
	appendCodexConsoleHandoverGroup(&lines, "Recent runs", handover.RecentRuns, "No recent runs.")
	appendCodexConsoleHandoverGroup(&lines, "Recent artifacts", handover.RecentArtifacts, "No recent artifacts.")
	appendCodexConsoleHandoverGroup(&lines, "Memory candidates", handover.MemoryCandidates, "No memory candidates.")
	lines = append(lines,
		"## Review notes",
		"",
		"- Candidate memories should be reviewed before syncing to long-term memory.",
		"- Skill candidates should be reviewed before assignment or local agent sync.",
		"- Hook files should remain disabled until their commands and write paths are reviewed.",
		"",
		"## Manual notes",
		"",
	)
	if strings.TrimSpace(manualNotes) != "" {
		lines = append(lines, strings.TrimSpace(manualNotes))
	} else {
		lines = append(lines, "No manual notes yet.")
	}
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func appendCodexConsoleHandoverGroup(lines *[]string, title string, items []codexConsoleHandoverItem, empty string) {
	*lines = append(*lines, "## "+title, "")
	if len(items) == 0 {
		*lines = append(*lines, "- "+empty, "")
		return
	}
	for _, item := range items {
		head := codexConsoleMarkdownLine(item.Title, item.ID)
		parts := []string{}
		if strings.TrimSpace(item.Kind) != "" {
			parts = append(parts, item.Kind)
		}
		if strings.TrimSpace(item.At) != "" {
			parts = append(parts, item.At)
		}
		if len(parts) > 0 {
			head += " (" + strings.Join(parts, ", ") + ")"
		}
		*lines = append(*lines, "- "+head)
		if detail := codexConsoleMarkdownLine(item.Detail, ""); detail != "" {
			*lines = append(*lines, "  - Detail: "+detail)
		}
		if source := codexConsoleMarkdownLine(item.SourcePath, ""); source != "" {
			*lines = append(*lines, "  - Source: `"+strings.ReplaceAll(source, "`", "'")+"`")
		}
	}
	*lines = append(*lines, "")
}

func codexConsoleHandoverManualNotes(existing string) string {
	existing = strings.TrimSpace(existing)
	if existing == "" {
		return ""
	}
	marker := "\n## Manual notes"
	index := strings.Index(existing, marker)
	if index < 0 {
		if strings.HasPrefix(existing, "## Manual notes") {
			index = 0
		} else {
			return ""
		}
	}
	section := strings.TrimLeft(existing[index:], "\n")
	lines := strings.Split(section, "\n")
	if len(lines) <= 1 {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines[1:], "\n"))
}

func codexConsoleMarkdownLine(value string, fallback string) string {
	value = truncateCodexConsoleText(value, 500)
	value = strings.Join(strings.Fields(strings.ReplaceAll(value, "\n", " ")), " ")
	if value == "" {
		return fallback
	}
	return value
}

func codexConsoleMemoryItemByID(items []sqlitestorage.AgentMemoryItem, id string) (sqlitestorage.AgentMemoryItem, codexConsoleMemoryCandidate, bool) {
	id = strings.TrimSpace(id)
	for _, item := range items {
		candidate := buildCodexConsoleMemoryCandidate(item)
		if candidate.ID == id {
			return item, candidate, true
		}
	}
	return sqlitestorage.AgentMemoryItem{}, codexConsoleMemoryCandidate{}, false
}

func normalizeCodexConsoleMemoryConflictResolution(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "keep_existing", "keep-existing", "keep", "keep_a":
		return "keep_existing"
	case "use_candidate", "use-candidate", "replace", "replace_existing", "keep_b":
		return "use_candidate"
	case "keep_both", "keep-both", "both":
		return "keep_both"
	case "merge", "merged":
		return "merge"
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}

func isCodexConsoleMemoryConflictResolution(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "keep_existing", "use_candidate", "keep_both", "merge":
		return true
	default:
		return false
	}
}

func (s *Server) syncCodexConsoleMemoryCandidate(ctx context.Context, userID uuid.UUID, item sqlitestorage.AgentMemoryItem, candidate codexConsoleMemoryCandidate, req codexConsoleMemorySyncRequest) codexConsoleMemorySyncResult {
	item, edited := codexConsoleMemorySyncItem(item, candidate.ID, req)
	if codexConsoleMemorySyncTarget(req.Target) == "project" {
		return s.syncCodexConsoleMemoryCandidateToProject(ctx, userID, item, candidate, req.Project, edited)
	}
	return s.syncCodexConsoleMemoryCandidateToProfile(ctx, userID, item, candidate, edited)
}

func (s *Server) syncCodexConsoleMemoryCandidateToProfile(ctx context.Context, userID uuid.UUID, item sqlitestorage.AgentMemoryItem, candidate codexConsoleMemoryCandidate, edited bool) codexConsoleMemorySyncResult {
	category := codexConsoleMemoryProfileCategory(candidate)
	path := hubpath.ProfilePath(category)
	result := codexConsoleMemorySyncResult{
		ID:       candidate.ID,
		Title:    candidate.Title,
		Category: category,
		Path:     path,
		Target:   "profile",
		Edited:   edited,
	}
	content := renderCodexConsoleMemoryItem(item)
	if strings.TrimSpace(cleanCodexConsoleText(item.Content)) == "" || strings.TrimSpace(content) == "" {
		result.Status = "skipped"
		result.Message = "memory candidate is empty"
		return result
	}
	if len([]byte(content)) > codexConsoleMemoryProfileMaxBytes {
		result.Status = "skipped"
		result.Message = "memory candidate is too large for profile memory"
		return result
	}
	if existing, ok, err := s.codexConsoleMemoryProfile(ctx, userID, category); err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	} else if ok && strings.TrimSpace(existing.Content) != strings.TrimSpace(content) && strings.TrimSpace(existing.Source) != "agent:codex" {
		result.Status = "skipped"
		result.Message = "profile memory conflict requires review"
		return result
	}
	if err := s.MemoryService.UpsertProfile(ctx, userID, category, content, "agent:codex"); err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}
	result.Status = "synced"
	return result
}

func (s *Server) syncCodexConsoleMemoryCandidateToProject(ctx context.Context, userID uuid.UUID, item sqlitestorage.AgentMemoryItem, candidate codexConsoleMemoryCandidate, projectName string, edited bool) codexConsoleMemorySyncResult {
	projectName = strings.TrimSpace(projectName)
	path := hubpath.ProjectContextPath(projectName)
	result := codexConsoleMemorySyncResult{
		ID:      candidate.ID,
		Title:   candidate.Title,
		Path:    path,
		Target:  "project",
		Project: projectName,
		Edited:  edited,
	}
	if projectName == "" {
		result.Status = "failed"
		result.Message = "project is required"
		return result
	}
	content := renderCodexConsoleProjectMemorySection(item, candidate)
	if strings.TrimSpace(cleanCodexConsoleText(item.Content)) == "" || strings.TrimSpace(content) == "" {
		result.Status = "skipped"
		result.Message = "memory candidate is empty"
		return result
	}
	if len([]byte(content)) > codexConsoleMemoryProfileMaxBytes {
		result.Status = "skipped"
		result.Message = "memory candidate is too large for project context"
		return result
	}
	project, err := s.ProjectService.Get(ctx, userID, projectName)
	if err != nil {
		project, err = s.ProjectService.Create(ctx, userID, projectName)
		if err != nil {
			result.Status = "failed"
			result.Message = err.Error()
			return result
		}
	}
	current := ""
	if project != nil {
		current = strings.TrimSpace(project.ContextMD)
	}
	marker := codexConsoleMemoryProjectMarker(candidate.ID)
	if strings.Contains(current, marker) {
		result.Status = "skipped"
		result.Message = "memory candidate already exists in project context"
		return result
	}
	next := strings.TrimSpace(current)
	if next != "" {
		next += "\n\n"
	}
	next += content
	if err := s.ProjectService.UpdateContext(ctx, userID, projectName, next); err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}
	result.Status = "synced"
	return result
}

func (s *Server) loadCodexConsoleMemoryReviewState(ctx context.Context, userID uuid.UUID) (codexConsoleMemoryReviewState, error) {
	state := codexConsoleMemoryReviewState{
		Version: 1,
		Items:   map[string]codexConsoleMemoryReviewItem{},
	}
	if s.FileTreeService == nil {
		return state, nil
	}
	entry, err := s.FileTreeService.Read(ctx, userID, codexConsoleMemoryReviewPath, models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, services.ErrEntryNotFound) {
			return state, nil
		}
		return state, err
	}
	if strings.TrimSpace(entry.Content) == "" {
		return state, nil
	}
	if err := json.Unmarshal([]byte(entry.Content), &state); err != nil {
		return state, fmt.Errorf("decode Codex Console memory review state: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Items == nil {
		state.Items = map[string]codexConsoleMemoryReviewItem{}
	}
	return state, nil
}

func (s *Server) saveCodexConsoleMemoryReviewState(ctx context.Context, userID uuid.UUID, state codexConsoleMemoryReviewState) error {
	if s.FileTreeService == nil {
		return nil
	}
	state.Version = 1
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if state.Items == nil {
		state.Items = map[string]codexConsoleMemoryReviewItem{}
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Codex Console memory review state: %w", err)
	}
	_, err = s.FileTreeService.WriteEntry(ctx, userID, codexConsoleMemoryReviewPath, string(body)+"\n", "application/json", models.FileTreeWriteOptions{
		Kind:          "codex_console_memory_review",
		MinTrustLevel: models.TrustLevelFull,
		Metadata: map[string]interface{}{
			"platform": "codex",
			"source":   "codex-console",
		},
	})
	if err != nil {
		return fmt.Errorf("write Codex Console memory review state: %w", err)
	}
	return nil
}

func applyCodexConsoleMemoryReviewState(resp *codexConsoleResponse, state codexConsoleMemoryReviewState) {
	for index := range resp.MemoryCandidates {
		candidate := &resp.MemoryCandidates[index]
		if candidate.ReviewStatus == "" {
			candidate.ReviewStatus = "review_required"
		}
		review, ok := state.Items[candidate.ID]
		if !ok {
			continue
		}
		if isCodexConsoleMemoryReviewStatus(review.Status) || review.Status == "synced" {
			candidate.ReviewStatus = review.Status
		}
		candidate.ReviewNote = review.Note
		candidate.ReviewedAt = review.ReviewedAt
		candidate.MemoryPath = review.MemoryPath
	}
}

func (s *Server) applyCodexConsoleMemoryConflictHints(ctx context.Context, userID uuid.UUID, resp *codexConsoleResponse, items []sqlitestorage.AgentMemoryItem) error {
	if s.MemoryService == nil {
		return nil
	}
	profiles, err := s.MemoryService.GetProfile(ctx, userID)
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		return nil
	}
	profileByCategory := map[string]models.MemoryProfile{}
	for _, profile := range profiles {
		profileByCategory[strings.TrimSpace(profile.Category)] = profile
	}
	itemByID := map[string]sqlitestorage.AgentMemoryItem{}
	for _, item := range items {
		candidate := buildCodexConsoleMemoryCandidate(item)
		if candidate.ID != "" {
			itemByID[candidate.ID] = item
		}
	}
	for index := range resp.MemoryCandidates {
		candidate := &resp.MemoryCandidates[index]
		switch strings.TrimSpace(strings.ToLower(candidate.ReviewStatus)) {
		case "ignored", "synced":
			continue
		}
		item, ok := itemByID[candidate.ID]
		if !ok {
			continue
		}
		category := codexConsoleMemoryProfileCategory(*candidate)
		existing, ok := profileByCategory[category]
		if !ok {
			continue
		}
		nextContent := strings.TrimSpace(renderCodexConsoleMemoryItem(item))
		if nextContent == "" || strings.TrimSpace(existing.Content) == nextContent {
			continue
		}
		if strings.TrimSpace(existing.Source) == "agent:codex" {
			continue
		}
		candidate.Conflict = &codexConsoleMemoryConflictHint{
			Status:            "possible",
			Target:            "profile",
			Category:          category,
			Path:              hubpath.ProfilePath(category),
			ExistingSource:    strings.TrimSpace(existing.Source),
			ExistingUpdatedAt: existing.UpdatedAt.UTC().Format(time.RFC3339),
			ExistingContent:   truncateCodexConsoleText(existing.Content, 2000),
			CandidateContent:  truncateCodexConsoleText(nextContent, 2000),
			Message:           "Profile memory already has different content for this category.",
		}
	}
	return nil
}

func codexConsoleMemoryCandidateByID(items []sqlitestorage.AgentMemoryItem) map[string]codexConsoleMemoryCandidate {
	candidates := map[string]codexConsoleMemoryCandidate{}
	for _, item := range items {
		candidate := buildCodexConsoleMemoryCandidate(item)
		if strings.TrimSpace(candidate.ID) != "" {
			candidates[candidate.ID] = candidate
		}
	}
	return candidates
}

func isCodexConsoleMemoryReviewStatus(status string) bool {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "review_required", "accepted", "ignored", "deferred":
		return true
	default:
		return false
	}
}

func codexConsoleMemoryProfileCategory(candidate codexConsoleMemoryCandidate) string {
	title := strings.TrimSpace(candidate.Title)
	lower := strings.ToLower(title)
	if strings.HasPrefix(lower, "chronicle/") {
		return "codex-chronicle-" + normalizeCodexConsoleID(strings.TrimSpace(title[len("chronicle/"):]))
	}
	return "codex-" + normalizeCodexConsoleID(title)
}

func codexConsoleMemorySyncTarget(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "profile", "/memory/profile":
		return "profile"
	case "project", "project_context", "project-context":
		return "project"
	default:
		return value
	}
}

func codexConsoleMemorySyncResponseTarget(req codexConsoleMemorySyncRequest) string {
	if codexConsoleMemorySyncTarget(req.Target) == "project" {
		return hubpath.ProjectContextPath(req.Project)
	}
	return "/memory/profile"
}

func codexConsoleMemorySyncItem(item sqlitestorage.AgentMemoryItem, id string, req codexConsoleMemorySyncRequest) (sqlitestorage.AgentMemoryItem, bool) {
	if len(req.ContentOverrides) == 0 {
		return item, false
	}
	for key, value := range req.ContentOverrides {
		if strings.TrimSpace(key) == id {
			next := item
			next.Content = cleanCodexConsoleText(value)
			return next, strings.TrimSpace(next.Content) != strings.TrimSpace(item.Content)
		}
	}
	return item, false
}

func renderCodexConsoleProjectMemorySection(item sqlitestorage.AgentMemoryItem, candidate codexConsoleMemoryCandidate) string {
	title := cleanCodexConsoleText(candidate.Title)
	if title == "" {
		title = "memory"
	}
	content := cleanCodexConsoleText(item.Content)
	lines := []string{
		codexConsoleMemoryProjectMarker(candidate.ID),
		"## Codex memory: " + title,
		"",
		content,
		"",
		"- Source: Codex Console",
		"- Memory candidate: " + candidate.ID,
		"- Kind: " + candidate.Kind,
		fmt.Sprintf("- Exactness: %s", fallbackAgentExactness(item.Exactness)),
	}
	if len(item.SourcePaths) > 0 {
		lines = append(lines, "- Source paths:")
		for _, source := range item.SourcePaths {
			if strings.TrimSpace(source) != "" {
				lines = append(lines, "  - "+strings.TrimSpace(source))
			}
		}
	}
	if item.Confidence > 0 {
		lines = append(lines, fmt.Sprintf("- Confidence: %.2f", item.Confidence))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderCodexConsoleMemoryItem(item sqlitestorage.AgentMemoryItem) string {
	clean := item
	clean.Title = cleanCodexConsoleText(clean.Title)
	clean.Content = cleanCodexConsoleText(clean.Content)
	if len(clean.SourcePaths) > 0 {
		paths := make([]string, 0, len(clean.SourcePaths))
		for _, source := range clean.SourcePaths {
			source = strings.TrimSpace(source)
			if source != "" {
				paths = append(paths, source)
			}
		}
		clean.SourcePaths = paths
	}
	return renderAgentMemoryItem(clean)
}

func codexConsoleMemoryProjectMarker(id string) string {
	return fmt.Sprintf("<!-- codex-console-memory:%s -->", normalizeCodexConsoleID(id))
}

func buildCodexConsoleThread(index int, convo sqlitestorage.ClaudeConversation) codexConsoleThread {
	threadID := strings.TrimSpace(convo.SessionID)
	if threadID == "" {
		threadID = fmt.Sprintf("thread-%03d", index+1)
	}
	title := cleanCodexConsoleText(convo.Name)
	if title == "" {
		title = "Codex thread"
	}
	thread := codexConsoleThread{
		ID:           threadID,
		Title:        title,
		Summary:      truncateCodexConsoleText(convo.Summary, 260),
		Project:      strings.TrimSpace(convo.ProjectName),
		StartedAt:    strings.TrimSpace(convo.StartedAt),
		UpdatedAt:    latestCodexConversationTime(convo),
		SourcePath:   firstString(convo.SourcePaths),
		MessageCount: len(convo.Messages),
		Archived:     strings.Contains(firstString(convo.SourcePaths), "/archived_sessions/"),
	}
	if thread.UpdatedAt == "" {
		thread.UpdatedAt = thread.StartedAt
	}
	for _, message := range convo.Messages {
		switch strings.ToLower(strings.TrimSpace(message.Role)) {
		case "user":
			thread.UserTurns++
		case "assistant":
			thread.AssistantTurns++
		}
		for _, part := range message.Parts {
			switch strings.TrimSpace(part.Type) {
			case "tool_call":
				thread.ToolCalls++
			case "tool_result":
				thread.ToolResults++
			case "thinking":
				thread.ThinkingEvents++
			case "attachment":
				thread.AttachmentCount++
			}
		}
	}
	return thread
}

func uniqueCodexConsoleThreadID(id, sourcePath string, index int, seen map[string]struct{}) string {
	id = strings.TrimSpace(id)
	if id == "" {
		id = fmt.Sprintf("thread-%03d", index+1)
	}
	if _, ok := seen[id]; !ok {
		seen[id] = struct{}{}
		return id
	}
	seed := firstNonEmptyConsoleString(sourcePath, fmt.Sprintf("%s-%d", id, index))
	suffix := codexConsoleIDHash(seed)
	candidate := id + "-" + suffix
	for counter := 2; ; counter++ {
		if _, ok := seen[candidate]; !ok {
			seen[candidate] = struct{}{}
			return candidate
		}
		candidate = fmt.Sprintf("%s-%s-%d", id, suffix, counter)
	}
}

func codexConsoleIDHash(value string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return fmt.Sprintf("%08x", h.Sum32())
}

func buildCodexConsoleAutomation(record sqlitestorage.AgentRecord) codexConsoleAutomation {
	meta := record.Metadata
	automation := codexConsoleAutomation{
		ID:         firstNonEmptyInterface(meta["id"], record.Name),
		Name:       firstNonEmptyInterface(meta["name"], record.Name),
		Kind:       firstNonEmptyInterface(meta["kind"]),
		Status:     firstNonEmptyInterface(meta["status"]),
		Schedule:   firstNonEmptyInterface(meta["schedule"]),
		Prompt:     truncateCodexConsoleText(firstNonEmptyInterface(meta["prompt"]), 360),
		SourcePath: firstString(record.SourcePaths),
	}
	if automation.ID == "" {
		automation.ID = strings.TrimSpace(record.Name)
	}
	if automation.Name == "" {
		automation.Name = automation.ID
	}
	return automation
}

func buildCodexConsoleRun(thread codexConsoleThread, convo sqlitestorage.ClaudeConversation, artifacts int) codexConsoleRun {
	run := codexConsoleRun{
		ID:          "run-" + thread.ID,
		ThreadID:    thread.ID,
		ThreadTitle: thread.Title,
		Project:     thread.Project,
		StartedAt:   thread.StartedAt,
		UpdatedAt:   thread.UpdatedAt,
		SourcePath:  thread.SourcePath,
		Artifacts:   artifacts,
	}
	for _, message := range convo.Messages {
		for _, part := range message.Parts {
			if part.Type != "tool_call" && part.Type != "tool_result" {
				continue
			}
			name := strings.TrimSpace(part.Name)
			detail := strings.TrimSpace(firstNonEmptyConsoleString(part.ArgsText, part.Text, message.Content))
			lower := strings.ToLower(name + " " + detail + " " + message.Kind)
			if part.Type == "tool_call" {
				run.ToolCalls++
			}
			if part.Type == "tool_result" {
				run.ToolResults++
			}
			if strings.Contains(lower, "browser") || strings.Contains(lower, "web_search") || strings.Contains(lower, "playwright") {
				run.BrowserActions++
			}
			if strings.Contains(lower, "computer") || strings.Contains(lower, "screenshot") || strings.Contains(lower, "press_key") || strings.Contains(lower, "type_text") {
				run.ComputerActions++
			}
			if strings.Contains(lower, "approval") || strings.Contains(lower, "approve") {
				run.Approvals++
			}
			if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "exception") {
				run.Errors++
			}
			if len(run.Events) < 8 {
				title := name
				if title == "" {
					title = part.Type
				}
				run.Events = append(run.Events, codexConsoleRunEvent{
					At:     strings.TrimSpace(message.Timestamp),
					Type:   part.Type,
					Title:  title,
					Detail: truncateCodexConsoleText(detail, 180),
				})
			}
		}
	}
	if run.ToolCalls == 0 && run.ToolResults == 0 && artifacts == 0 {
		return codexConsoleRun{}
	}
	return run
}

func buildCodexConsoleGoal(thread codexConsoleThread, convo sqlitestorage.ClaudeConversation) codexConsoleGoal {
	for _, message := range convo.Messages {
		if strings.ToLower(strings.TrimSpace(message.Role)) != "user" {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if !looksLikeCodexGoal(content) {
			continue
		}
		title := firstNonEmptyConsoleLine(content, "Codex goal")
		return codexConsoleGoal{
			ID:          "goal-" + thread.ID,
			Title:       truncateCodexConsoleText(title, 140),
			Status:      "observed",
			ThreadID:    thread.ID,
			ThreadTitle: thread.Title,
			Project:     thread.Project,
			SourcePath:  thread.SourcePath,
			ObservedAt:  strings.TrimSpace(message.Timestamp),
			Description: truncateCodexConsoleText(content, 420),
		}
	}
	return codexConsoleGoal{}
}

func buildCodexConsoleMemoryCandidate(item sqlitestorage.AgentMemoryItem) codexConsoleMemoryCandidate {
	title := cleanCodexConsoleText(item.Title)
	if title == "" {
		title = "memory"
	}
	kind := "codex-memory"
	if strings.HasPrefix(strings.ToLower(title), "chronicle/") {
		kind = "chronicle"
	}
	return codexConsoleMemoryCandidate{
		ID:           "memory-" + normalizeCodexConsoleID(title),
		Title:        title,
		Kind:         kind,
		Content:      truncateCodexConsoleText(item.Content, 520),
		SourcePath:   firstString(item.SourcePaths),
		Confidence:   item.Confidence,
		ReviewStatus: "review_required",
	}
}

func collectCodexConsoleArtifacts(convo sqlitestorage.ClaudeConversation, thread codexConsoleThread, seen map[string]bool) []codexConsoleArtifact {
	artifacts := []codexConsoleArtifact{}
	for _, message := range convo.Messages {
		for _, part := range message.Parts {
			if part.Type == "attachment" {
				name := strings.TrimSpace(part.FileName)
				if name == "" {
					name = "attachment"
				}
				key := thread.ID + ":attachment:" + name
				if seen[key] {
					continue
				}
				seen[key] = true
				artifact := codexConsoleArtifact{
					ID:          codexConsoleStableID("artifact", key),
					Name:        name,
					Kind:        firstNonEmptyConsoleString(part.MimeType, "attachment"),
					ThreadID:    thread.ID,
					ThreadTitle: thread.Title,
					Project:     thread.Project,
					SourcePath:  thread.SourcePath,
				}
				artifacts = append(artifacts, enrichCodexConsoleArtifact(artifact))
			}
			if part.Type != "tool_result" {
				continue
			}
			text := firstNonEmptyConsoleString(part.Text, message.Content)
			for _, match := range codexArtifactPathPattern.FindAllStringSubmatch(text, 6) {
				if len(match) < 2 {
					continue
				}
				name := strings.TrimSpace(match[1])
				if name == "" {
					continue
				}
				key := thread.ID + ":path:" + name
				if seen[key] {
					continue
				}
				seen[key] = true
				artifact := codexConsoleArtifact{
					ID:          codexConsoleStableID("artifact", key),
					Name:        name,
					Kind:        "file-reference",
					ThreadID:    thread.ID,
					ThreadTitle: thread.Title,
					Project:     thread.Project,
					SourcePath:  thread.SourcePath,
					Detail:      "Referenced by a Codex tool result.",
				}
				artifacts = append(artifacts, enrichCodexConsoleArtifact(artifact))
			}
		}
	}
	return artifacts
}

func enrichCodexConsoleArtifact(artifact codexConsoleArtifact) codexConsoleArtifact {
	role, note := codexConsoleArtifactRoleAndNote(artifact)
	artifact.Role = role
	artifact.HandoffNote = note
	artifact.AgentInstruction = codexConsoleArtifactAgentInstruction(artifact, role, note)
	return artifact
}

func codexConsoleArtifactRoleAndNote(artifact codexConsoleArtifact) (string, string) {
	name := strings.ToLower(strings.TrimSpace(artifact.Name))
	kind := strings.ToLower(strings.TrimSpace(artifact.Kind))
	ext := strings.ToLower(path.Ext(name))
	switch {
	case strings.HasPrefix(kind, "image/") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" || ext == ".gif":
		return "visual-evidence", "Use this as visual evidence or a screenshot when explaining what was verified."
	case ext == ".html" || ext == ".htm":
		return "preview-output", "Use this as a rendered preview, report, or local page output before continuing the work."
	case ext == ".md" || ext == ".markdown" || ext == ".pdf" || ext == ".doc" || ext == ".docx" || ext == ".ppt" || ext == ".pptx":
		return "handoff-document", "Read this document before continuing; it likely contains a plan, report, or final deliverable context."
	case ext == ".json" || ext == ".csv" || ext == ".xlsx" || ext == ".xls":
		return "structured-data", "Use this as structured input or exported data; inspect schema and provenance before changing it."
	case ext == ".log" || ext == ".txt":
		return "run-evidence", "Use this as execution evidence, notes, or logs when checking what happened."
	case strings.Contains(kind, "attachment"):
		return "attachment", "Treat this as a user-provided or generated attachment; inspect it before assuming it is final output."
	default:
		return "file-reference", "Use this referenced file as project context; confirm whether it is final output or intermediate work."
	}
}

func codexConsoleArtifactAgentInstruction(artifact codexConsoleArtifact, role string, note string) string {
	name := strings.TrimSpace(artifact.Name)
	if name == "" {
		name = "this artifact"
	}
	lines := []string{
		"Review `" + strings.ReplaceAll(name, "`", "'") + "` as " + role + ".",
		note,
	}
	if strings.TrimSpace(artifact.Project) != "" {
		lines = append(lines, "Project: "+strings.TrimSpace(artifact.Project)+".")
	}
	if strings.TrimSpace(artifact.ThreadTitle) != "" {
		lines = append(lines, "Source Codex thread: "+strings.TrimSpace(artifact.ThreadTitle)+".")
	}
	return strings.Join(lines, "\n")
}

func collectCodexConsoleHooks(bundles []sqlitestorage.ClaudeBundle) []codexConsoleHookRisk {
	hooks := []codexConsoleHookRisk{}
	seen := map[string]bool{}
	for _, bundle := range bundles {
		bundleName := strings.TrimSpace(bundle.Name)
		if bundleName == "" {
			bundleName = "skill"
		}
		for _, file := range bundle.Files {
			relPath := strings.TrimSpace(firstNonEmptyConsoleString(file.Path, file.SourcePath))
			if relPath == "" || !isCodexConsoleHookPath(relPath) {
				continue
			}
			key := bundleName + ":" + relPath
			if seen[key] {
				continue
			}
			seen[key] = true
			analysis := analyzeCodexConsoleHookContent(file.Content)
			hooks = append(hooks, codexConsoleHookRisk{
				ID:             codexConsoleStableID("hook", key),
				Name:           relPath,
				Kind:           codexConsoleHookKind(relPath),
				Bundle:         bundleName,
				Status:         "manual_required",
				RiskLevel:      analysis.RiskLevel,
				Shebang:        analysis.Shebang,
				EnvVars:        analysis.EnvVars,
				RiskSignals:    analysis.RiskSignals,
				WritePathHints: analysis.WritePathHints,
				SourcePath:     firstNonEmptyConsoleString(file.SourcePath, firstString(bundle.SourcePaths)),
				Detail:         truncateCodexConsoleText(firstNonEmptyConsoleLine(file.Content, "Hook file requires review before enabling."), 220),
			})
		}
	}
	sort.Slice(hooks, func(i, j int) bool {
		if hooks[i].Bundle == hooks[j].Bundle {
			return hooks[i].Name < hooks[j].Name
		}
		return hooks[i].Bundle < hooks[j].Bundle
	})
	return hooks
}

type codexConsoleHookAnalysis struct {
	RiskLevel      string
	Shebang        string
	EnvVars        []string
	RiskSignals    []string
	WritePathHints []string
}

func isCodexConsoleHookPath(pathValue string) bool {
	clean := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(pathValue), "\\", "/"))
	clean = strings.Trim(clean, "/")
	return strings.HasPrefix(clean, "hooks/") || strings.Contains(clean, "/hooks/")
}

func analyzeCodexConsoleHookContent(content string) codexConsoleHookAnalysis {
	analysis := codexConsoleHookAnalysis{RiskLevel: "low"}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return analysis
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "#!") {
		analysis.Shebang = truncateCodexConsoleText(strings.TrimSpace(lines[0]), 120)
	}
	lower := strings.ToLower(trimmed)
	signals := []struct {
		needle string
		label  string
		level  string
	}{
		{"rm -rf", "destructive delete", "high"},
		{"sudo ", "privileged command", "high"},
		{"| sh", "remote shell pipe", "high"},
		{"| bash", "remote shell pipe", "high"},
		{"osascript", "macOS automation", "high"},
		{"curl ", "network fetch", "medium"},
		{"wget ", "network fetch", "medium"},
		{"chmod ", "permission change", "medium"},
		{"chown ", "ownership change", "medium"},
		{"ssh ", "remote shell", "medium"},
		{"scp ", "remote copy", "medium"},
		{"git push", "git write", "medium"},
		{"docker ", "container command", "medium"},
		{"kubectl ", "cluster command", "medium"},
	}
	for _, signal := range signals {
		if strings.Contains(lower, signal.needle) {
			analysis.RiskSignals = appendUniqueLimited(analysis.RiskSignals, signal.label, 8)
			if signal.level == "high" {
				analysis.RiskLevel = "high"
			} else if analysis.RiskLevel == "low" {
				analysis.RiskLevel = "medium"
			}
		}
	}
	for _, match := range codexHookEnvPattern.FindAllString(trimmed, -1) {
		if isCommonCodexHookEnvWord(match) {
			continue
		}
		analysis.EnvVars = appendUniqueLimited(analysis.EnvVars, match, 10)
	}
	for _, pathHint := range collectCodexConsoleHookWritePathHints(trimmed) {
		analysis.WritePathHints = appendUniqueLimited(analysis.WritePathHints, truncateCodexConsoleText(pathHint, 120), 8)
	}
	if len(analysis.WritePathHints) > 0 && analysis.RiskLevel == "low" {
		analysis.RiskLevel = "medium"
	}
	return analysis
}

func collectCodexConsoleHookWritePathHints(content string) []string {
	var hints []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lower := strings.ToLower(trimmed)
		hasWriteOperation := codexHookWriteCommandPattern.MatchString(lower) ||
			strings.Contains(lower, "tee ") ||
			strings.Contains(trimmed, ">")
		if !hasWriteOperation {
			continue
		}
		for _, field := range splitCodexConsoleHookFields(trimmed) {
			pathHint := normalizeCodexConsoleHookPathField(field)
			if pathHint == "" {
				continue
			}
			hints = appendUniqueLimited(hints, pathHint, 8)
		}
	}
	return hints
}

func splitCodexConsoleHookFields(value string) []string {
	var fields []string
	var current strings.Builder
	var quote rune
	flush := func() {
		field := strings.TrimSpace(current.String())
		if field != "" {
			fields = append(fields, field)
		}
		current.Reset()
	}
	for _, r := range value {
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t', '\r', '\n', ';', ',', ')':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return fields
}

func normalizeCodexConsoleHookPathField(value string) string {
	clean := strings.TrimSpace(value)
	clean = strings.Trim(clean, `"'`)
	if index := strings.LastIndex(clean, ">"); index >= 0 {
		clean = clean[index+1:]
	}
	clean = strings.Trim(clean, `"'`)
	if strings.HasPrefix(clean, "//") {
		return ""
	}
	if strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "~/") || clean == "~" {
		return clean
	}
	return ""
}

func isCommonCodexHookEnvWord(value string) bool {
	switch strings.TrimSpace(value) {
	case "PATH", "HOME", "SHELL", "USER", "PWD", "TMPDIR", "JSON", "HTTP", "HTTPS", "URL":
		return true
	default:
		return false
	}
}

func codexConsoleHookKind(pathValue string) string {
	lower := strings.ToLower(strings.TrimSpace(pathValue))
	switch {
	case strings.HasSuffix(lower, ".sh"):
		return "shell"
	case strings.HasSuffix(lower, ".js"), strings.HasSuffix(lower, ".mjs"), strings.HasSuffix(lower, ".ts"):
		return "node"
	case strings.HasSuffix(lower, ".py"):
		return "python"
	case strings.HasSuffix(lower, ".json"), strings.HasSuffix(lower, ".toml"), strings.HasSuffix(lower, ".yaml"), strings.HasSuffix(lower, ".yml"):
		return "config"
	default:
		return "hook"
	}
}

func buildCodexConsoleOverview(resp codexConsoleResponse, payload sqlitestorage.AgentExportPayload) codexConsoleOverview {
	overview := codexConsoleOverview{
		Threads:           len(resp.Threads),
		Goals:             len(resp.Goals),
		Automations:       len(resp.Automations),
		Runs:              len(resp.Runs),
		Artifacts:         len(resp.Artifacts),
		Hooks:             len(resp.Hooks),
		MemoryCandidates:  len(resp.MemoryCandidates),
		Handovers:         len(resp.Handovers),
		SkillCandidates:   len(resp.SkillCandidates),
		Projects:          len(payload.Projects),
		Tools:             len(payload.Tools),
		SensitiveFindings: len(resp.SensitiveFindings),
		VaultCandidates:   len(resp.VaultCandidates),
	}
	if payload.Codex != nil {
		overview.Skills = len(payload.Codex.Bundles)
	}
	workspaces := map[string]*codexConsoleWorkspace{}
	for _, thread := range resp.Threads {
		if thread.UpdatedAt > overview.LastActivity {
			overview.LastActivity = thread.UpdatedAt
		}
		name := strings.TrimSpace(thread.Project)
		if name == "" {
			name = "unassigned"
		}
		item := workspaces[name]
		if item == nil {
			item = &codexConsoleWorkspace{Name: name}
			workspaces[name] = item
		}
		item.Threads++
		if thread.UpdatedAt > item.LastActivity {
			item.LastActivity = thread.UpdatedAt
		}
	}
	for _, candidate := range resp.MemoryCandidates {
		switch strings.TrimSpace(strings.ToLower(candidate.ReviewStatus)) {
		case "accepted":
			overview.MemoryAccepted++
		case "ignored":
			overview.MemoryIgnored++
		case "deferred":
			overview.MemoryDeferred++
		case "synced":
			overview.MemorySynced++
		default:
			overview.MemoryReviewRequired++
		}
	}
	for _, item := range workspaces {
		overview.Workspaces = append(overview.Workspaces, *item)
	}
	sort.Slice(overview.Workspaces, func(i, j int) bool {
		if overview.Workspaces[i].LastActivity == overview.Workspaces[j].LastActivity {
			return overview.Workspaces[i].Name < overview.Workspaces[j].Name
		}
		return overview.Workspaces[i].LastActivity > overview.Workspaces[j].LastActivity
	})
	return overview
}

func buildCodexConsoleHandovers(resp codexConsoleResponse) []codexConsoleHandoverSummary {
	byProject := map[string]*codexConsoleHandoverSummary{}
	projectNames := map[string]bool{}
	ensure := func(project string) *codexConsoleHandoverSummary {
		project = strings.TrimSpace(project)
		if project == "" {
			project = "unassigned"
		}
		item := byProject[project]
		if item == nil {
			item = &codexConsoleHandoverSummary{
				ID:      codexConsoleStableID("handover", project),
				Project: project,
			}
			byProject[project] = item
		}
		return item
	}
	for _, thread := range resp.Threads {
		projectNames[firstNonEmptyConsoleString(thread.Project, "unassigned")] = true
		handover := ensure(thread.Project)
		handover.ThreadCount++
		if thread.UpdatedAt > handover.LatestActivity {
			handover.LatestActivity = thread.UpdatedAt
		}
		if len(handover.RecentThreads) < 5 {
			handover.RecentThreads = append(handover.RecentThreads, codexConsoleHandoverItem{
				ID:         thread.ID,
				Title:      thread.Title,
				Kind:       "thread",
				Detail:     thread.Summary,
				At:         thread.UpdatedAt,
				SourcePath: thread.SourcePath,
			})
		}
	}
	for _, run := range resp.Runs {
		handover := ensure(run.Project)
		handover.RunCount++
		if run.UpdatedAt > handover.LatestActivity {
			handover.LatestActivity = run.UpdatedAt
		}
		if len(handover.RecentRuns) < 5 {
			handover.RecentRuns = append(handover.RecentRuns, codexConsoleHandoverItem{
				ID:         run.ID,
				Title:      run.ThreadTitle,
				Kind:       "run",
				Detail:     fmt.Sprintf("%d tool calls, %d tool results, %d errors", run.ToolCalls, run.ToolResults, run.Errors),
				At:         run.UpdatedAt,
				SourcePath: run.SourcePath,
			})
		}
	}
	for _, artifact := range resp.Artifacts {
		handover := ensure(artifact.Project)
		handover.ArtifactCount++
		if len(handover.RecentArtifacts) < 5 {
			handover.RecentArtifacts = append(handover.RecentArtifacts, codexConsoleHandoverItem{
				ID:         artifact.ID,
				Title:      artifact.Name,
				Kind:       artifact.Kind,
				Detail:     artifact.ThreadTitle,
				SourcePath: artifact.SourcePath,
			})
		}
	}
	for _, candidate := range resp.MemoryCandidates {
		project := inferCodexConsoleMemoryProject(candidate, projectNames)
		handover := ensure(project)
		handover.MemoryCandidateCount++
		if len(handover.MemoryCandidates) < 5 {
			handover.MemoryCandidates = append(handover.MemoryCandidates, codexConsoleHandoverItem{
				ID:         candidate.ID,
				Title:      candidate.Title,
				Kind:       candidate.Kind,
				Detail:     firstNonEmptyConsoleString(candidate.ReviewStatus, "review_required"),
				SourcePath: candidate.SourcePath,
			})
		}
	}
	handovers := make([]codexConsoleHandoverSummary, 0, len(byProject))
	for _, handover := range byProject {
		handover.Summary = renderCodexConsoleHandoverSummary(*handover)
		handovers = append(handovers, *handover)
	}
	sort.Slice(handovers, func(i, j int) bool {
		if handovers[i].LatestActivity == handovers[j].LatestActivity {
			return handovers[i].Project < handovers[j].Project
		}
		return handovers[i].LatestActivity > handovers[j].LatestActivity
	})
	return handovers
}

func inferCodexConsoleMemoryProject(candidate codexConsoleMemoryCandidate, projectNames map[string]bool) string {
	source := "/" + strings.Trim(strings.ReplaceAll(candidate.SourcePath, "\\", "/"), "/") + "/"
	for project := range projectNames {
		if project == "" || project == "unassigned" {
			continue
		}
		if strings.Contains(source, "/"+project+"/") {
			return project
		}
	}
	return "unassigned"
}

func renderCodexConsoleHandoverSummary(handover codexConsoleHandoverSummary) string {
	parts := []string{
		fmt.Sprintf("%s has %d threads, %d runs, %d artifacts, and %d memory candidates.",
			handover.Project,
			handover.ThreadCount,
			handover.RunCount,
			handover.ArtifactCount,
			handover.MemoryCandidateCount,
		),
	}
	if handover.LatestActivity != "" {
		parts = append(parts, "Latest activity: "+handover.LatestActivity+".")
	}
	if len(handover.RecentThreads) > 0 {
		parts = append(parts, "Recent thread: "+handover.RecentThreads[0].Title+".")
	}
	return strings.Join(parts, " ")
}

func buildCodexConsoleSkillCandidates(resp codexConsoleResponse) []codexConsoleSkillCandidate {
	threadByID := map[string]codexConsoleThread{}
	for _, thread := range resp.Threads {
		threadByID[thread.ID] = thread
	}
	candidates := []codexConsoleSkillCandidate{}
	seen := map[string]bool{}
	for _, run := range resp.Runs {
		if run.Errors > 0 || run.ToolCalls == 0 {
			continue
		}
		thread := threadByID[run.ThreadID]
		project := firstNonEmptyConsoleString(run.Project, thread.Project)
		if strings.TrimSpace(project) == "" {
			project = "unassigned"
		}
		title := firstNonEmptyConsoleString(thread.Title, run.ThreadTitle, "Codex workflow")
		name := normalizeCodexConsoleID(project + "-" + title)
		id := codexConsoleStableID("skill-candidate", project+":"+run.ThreadID+":"+title)
		if seen[id] {
			continue
		}
		seen[id] = true
		signals := []string{"completed without recorded errors"}
		if run.ToolCalls > 0 {
			signals = append(signals, fmt.Sprintf("%d tool calls", run.ToolCalls))
		}
		if run.Artifacts > 0 {
			signals = append(signals, fmt.Sprintf("%d artifacts", run.Artifacts))
		}
		confidence := 0.55
		if run.ToolCalls >= 3 {
			confidence += 0.15
		}
		if run.Artifacts > 0 {
			confidence += 0.10
		}
		if strings.TrimSpace(project) != "unassigned" {
			confidence += 0.10
		}
		if confidence > 0.90 {
			confidence = 0.90
		}
		candidate := codexConsoleSkillCandidate{
			ID:            id,
			Name:          name,
			Title:         "Candidate skill: " + title,
			Project:       project,
			ThreadID:      run.ThreadID,
			ThreadTitle:   title,
			UpdatedAt:     firstNonEmptyConsoleString(run.UpdatedAt, thread.UpdatedAt),
			SourcePath:    firstNonEmptyConsoleString(run.SourcePath, thread.SourcePath),
			Confidence:    confidence,
			ToolCalls:     run.ToolCalls,
			ArtifactCount: run.Artifacts,
			Signals:       signals,
			Rationale:     renderCodexConsoleSkillCandidateRationale(project, title, run),
		}
		candidate.Draft = renderCodexConsoleSkillCandidateDraft(candidate)
		candidates = append(candidates, candidate)
		if len(candidates) >= 80 {
			break
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].UpdatedAt == candidates[j].UpdatedAt {
			return candidates[i].Name < candidates[j].Name
		}
		return candidates[i].UpdatedAt > candidates[j].UpdatedAt
	})
	return candidates
}

func renderCodexConsoleSkillCandidateRationale(project, title string, run codexConsoleRun) string {
	return fmt.Sprintf("This Codex thread finished without recorded errors and used %d tool calls in project %s. Review it before turning the draft into a managed Vola skill.", run.ToolCalls, project)
}

func renderCodexConsoleSkillCandidateDraft(candidate codexConsoleSkillCandidate) string {
	lines := []string{
		"---",
		"name: " + candidate.Name,
		"description: Candidate Skill generated from a successful Codex thread.",
		"when_to_use: Use this candidate after reviewing the source workflow and verifying it still applies.",
		"---",
		"",
		"# " + candidate.Name,
		"",
		"Use this skill when working on tasks similar to this Codex thread:",
		"",
		"- Project: " + firstNonEmptyConsoleString(candidate.Project, "unassigned"),
		"- Thread: " + candidate.ThreadTitle,
		"- Source path: " + firstNonEmptyConsoleString(candidate.SourcePath, "unknown"),
		"- Confidence: " + fmt.Sprintf("%.2f", candidate.Confidence),
		"",
		"## Workflow",
		"",
		"- Review the current project context and relevant files before editing.",
		"- Reuse the successful steps from the source Codex thread only after checking they still apply.",
		"- Keep changes scoped to the project and document any artifacts produced.",
		"- Verify the result with the project's tests, build, or manual checks.",
		"",
		"## Review notes",
		"",
		"- This is a candidate generated from Codex Console data.",
		"- Confirm commands, dependencies, credentials, and file paths before installing it as a real skill.",
	}
	if len(candidate.Signals) > 0 {
		lines = append(lines, "", "## Signals", "")
		for _, signal := range candidate.Signals {
			lines = append(lines, "- "+signal)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func sortCodexConsoleResponse(resp *codexConsoleResponse) {
	sort.Slice(resp.Threads, func(i, j int) bool {
		if resp.Threads[i].UpdatedAt == resp.Threads[j].UpdatedAt {
			return resp.Threads[i].Title < resp.Threads[j].Title
		}
		return resp.Threads[i].UpdatedAt > resp.Threads[j].UpdatedAt
	})
	sort.Slice(resp.Automations, func(i, j int) bool { return resp.Automations[i].Name < resp.Automations[j].Name })
	sort.Slice(resp.Runs, func(i, j int) bool {
		if resp.Runs[i].UpdatedAt == resp.Runs[j].UpdatedAt {
			return resp.Runs[i].ThreadTitle < resp.Runs[j].ThreadTitle
		}
		return resp.Runs[i].UpdatedAt > resp.Runs[j].UpdatedAt
	})
	sort.Slice(resp.Artifacts, func(i, j int) bool { return resp.Artifacts[i].Name < resp.Artifacts[j].Name })
	sort.Slice(resp.Hooks, func(i, j int) bool {
		if resp.Hooks[i].Bundle == resp.Hooks[j].Bundle {
			return resp.Hooks[i].Name < resp.Hooks[j].Name
		}
		return resp.Hooks[i].Bundle < resp.Hooks[j].Bundle
	})
	sort.Slice(resp.MemoryCandidates, func(i, j int) bool { return resp.MemoryCandidates[i].Title < resp.MemoryCandidates[j].Title })
}

func latestCodexConversationTime(convo sqlitestorage.ClaudeConversation) string {
	latest := strings.TrimSpace(convo.StartedAt)
	for _, message := range convo.Messages {
		if strings.TrimSpace(message.Timestamp) > latest {
			latest = strings.TrimSpace(message.Timestamp)
		}
	}
	return latest
}

func looksLikeCodexGoal(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "/goal") ||
		strings.Contains(content, "目标：") ||
		strings.Contains(content, "目标:") ||
		strings.Contains(lower, "goal:") ||
		strings.Contains(lower, "objective:")
}

func normalizeCodexConsoleID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	normalized := strings.Trim(out.String(), "-")
	if normalized == "" {
		return "item"
	}
	if len(normalized) > 72 {
		return normalized[:72]
	}
	return normalized
}

func codexConsoleStableID(prefix, value string) string {
	normalized := normalizeCodexConsoleID(value)
	if normalized == "" || normalized == "item" {
		return prefix + "-" + codexConsoleIDHash(value)
	}
	return prefix + "-" + normalized + "-" + codexConsoleIDHash(value)
}

func truncateCodexConsoleText(value string, limit int) string {
	value = cleanCodexConsoleText(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	if limit <= 1 {
		return string(runes[:limit])
	}
	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}

func cleanCodexConsoleText(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
	if value == "" {
		return ""
	}
	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if len(cleaned) > 0 && !blank {
				cleaned = append(cleaned, "")
				blank = true
			}
			continue
		}
		if strings.ContainsRune(line, unicode.ReplacementChar) {
			continue
		}
		line = strings.Map(func(r rune) rune {
			if r == '\t' {
				return ' '
			}
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, line)
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		cleaned = append(cleaned, line)
		blank = false
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmptyConsoleString(values ...string) string {
	for _, value := range values {
		value = cleanCodexConsoleText(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyConsoleLine(value, fallback string) string {
	for _, line := range strings.Split(value, "\n") {
		line = cleanCodexConsoleText(line)
		if line != "" {
			return strings.TrimSpace(line)
		}
	}
	return fallback
}

func appendUniqueLimited(values []string, next string, limit int) []string {
	next = strings.TrimSpace(next)
	if next == "" || (limit > 0 && len(values) >= limit) {
		return values
	}
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}

func firstNonEmptyInterface(values ...interface{}) string {
	for _, value := range values {
		text := cleanCodexConsoleText(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}
