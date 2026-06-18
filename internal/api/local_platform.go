package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/hubpath"
	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/agi-bar/vola/internal/skillsarchive"
	sqlitestorage "github.com/agi-bar/vola/internal/storage/sqlite"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type localPlatformImportRequest struct {
	Platform     string                            `json:"platform"`
	Sources      []sqlitestorage.Source            `json:"sources,omitempty"`
	AgentPayload *sqlitestorage.AgentExportPayload `json:"agent_payload,omitempty"`
}

type localPlatformImportResponse struct {
	Files *sqlitestorage.ImportResult      `json:"files,omitempty"`
	Agent *sqlitestorage.AgentImportResult `json:"agent,omitempty"`
}

type localPlatformExportRequest struct {
	Platform   string `json:"platform"`
	OutputRoot string `json:"output_root"`
}

type localPlatformTokenRequest struct {
	Platform   string `json:"platform"`
	TrustLevel int    `json:"trust_level,omitempty"`
}

type localPlatformSkillsArchiveRequest struct {
	Platform    string `json:"platform"`
	ArchivePath string `json:"archive_path"`
}

const localPlatformProfileContentLimitBytes = 64 * 1024
const localPlatformProfileSummaryBudgetBytes = localPlatformProfileContentLimitBytes - 4096
const localPlatformScratchContentLimitBytes = 64 * 1024

func (s *Server) ensureLocalPlatformMode(w http.ResponseWriter) bool {
	if s.isLocalMode() {
		return true
	}
	respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "platform local filesystem operations are only available in local mode")
	return false
}

func (s *Server) handleAgentCreateLocalPlatformToken(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	if !s.ensureLocalPlatformMode(w) {
		return
	}
	if s.TokenService == nil {
		respondNotConfigured(w, "token service")
		return
	}

	var req localPlatformTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	scopes := make([]string, 0, len(models.AllScopes)-1)
	for _, scope := range models.AllScopes {
		if scope == models.ScopeAdmin {
			continue
		}
		scopes = append(scopes, scope)
	}
	trustLevel := req.TrustLevel
	if trustLevel <= 0 {
		trustLevel = models.TrustLevelWork
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = "platform"
	}
	userID, _ := userIDFromCtx(r.Context())
	tokenResp, err := s.TokenService.CreateToken(r.Context(), userID, models.CreateTokenRequest{
		Name:          "local platform " + platform,
		Scopes:        scopes,
		MaxTrustLevel: trustLevel,
		ExpiresInDays: 365,
	})
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondCreated(w, tokenResp)
}

func (s *Server) handleAgentRevokeLocalPlatformToken(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	if !s.ensureLocalPlatformMode(w) {
		return
	}
	if s.TokenService == nil {
		respondNotConfigured(w, "token service")
		return
	}

	tokenID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		respondValidationError(w, "id", "token id must be a valid UUID")
		return
	}
	userID, _ := userIDFromCtx(r.Context())
	if err := s.TokenService.RevokeToken(r.Context(), userID, tokenID); err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]string{"status": "revoked", "id": tokenID.String()})
}

func (s *Server) handleAgentImportLocalPlatformData(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	if !s.ensureLocalPlatformMode(w) {
		return
	}
	if s.FileTreeService == nil || s.MemoryService == nil || s.ProjectService == nil {
		respondError(w, http.StatusInternalServerError, ErrCodeInternal, "platform import services not configured")
		return
	}

	var req localPlatformImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	if req.Sources == nil && req.AgentPayload == nil {
		respondValidationError(w, "sources", "at least one import source or agent payload is required")
		return
	}

	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		respondValidationError(w, "platform", "platform is required")
		return
	}

	userID, _ := userIDFromCtx(r.Context())
	resp := &localPlatformImportResponse{}
	var err error
	err = s.withLocalPlatformImportLock(func() error {
		if req.AgentPayload != nil {
			resp.Agent, err = s.importLocalPlatformAgentPayload(r.Context(), userID, platform, *req.AgentPayload)
			if err != nil {
				return err
			}
		}
		if req.Sources != nil {
			resp.Files, err = s.importLocalPlatformSources(r.Context(), userID, platform, req.Sources)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errLocalPlatformImportBusy) {
			respondError(w, http.StatusConflict, ErrCodeConflict, err.Error())
			return
		}
		respondInternalError(w, err)
		return
	}

	respondOKWithLocalGitSync(w, resp, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleAgentExportLocalPlatformData(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	if !s.ensureLocalPlatformMode(w) {
		return
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return
	}

	var req localPlatformExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		respondValidationError(w, "platform", "platform is required")
		return
	}
	outputRoot := strings.TrimSpace(req.OutputRoot)
	if outputRoot == "" {
		respondValidationError(w, "output_root", "output_root is required")
		return
	}

	userID, _ := userIDFromCtx(r.Context())
	result, err := s.exportLocalPlatformSnapshot(r.Context(), userID, platform, outputRoot)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, result)
}

func (s *Server) handleAgentImportLocalSkillsArchive(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	if !s.ensureLocalPlatformMode(w) {
		return
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return
	}

	var req localPlatformSkillsArchiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	archivePath := strings.TrimSpace(req.ArchivePath)
	if archivePath == "" {
		respondValidationError(w, "archive_path", "archive_path is required")
		return
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = "claude-web"
	}

	userID, _ := userIDFromCtx(r.Context())
	result, err := s.importLocalSkillsArchive(r.Context(), userID, platform, archivePath)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKWithLocalGitSync(w, result, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) importLocalPlatformSources(ctx context.Context, userID uuid.UUID, platform string, sources []sqlitestorage.Source) (*sqlitestorage.ImportResult, error) {
	result := &sqlitestorage.ImportResult{Platform: platform}
	for _, source := range sources {
		if strings.TrimSpace(source.Path) == "" {
			continue
		}
		info, err := os.Stat(source.Path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if info.IsDir() {
			err = filepath.WalkDir(source.Path, func(pathValue string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() {
					if pathValue != source.Path && isManagedNeuDriveDir(pathValue) {
						return filepath.SkipDir
					}
					return nil
				}
				rel, err := filepath.Rel(source.Path, pathValue)
				if err != nil {
					return err
				}
				hubPath := filepath.ToSlash(filepath.Join("/platforms", platform, source.Domain, source.Label, rel))
				bytesWritten, err := s.writeLocalPlatformFile(ctx, userID, hubPath, pathValue, map[string]interface{}{
					"platform":      platform,
					"domain":        source.Domain,
					"source_label":  source.Label,
					"original_path": pathValue,
				})
				if err != nil {
					return err
				}
				result.Files++
				result.Bytes += bytesWritten
				result.Paths = append(result.Paths, hubPath)
				return nil
			})
			if err != nil {
				return nil, err
			}
			continue
		}

		hubPath := filepath.ToSlash(filepath.Join("/platforms", platform, source.Domain, source.Label))
		bytesWritten, err := s.writeLocalPlatformFile(ctx, userID, hubPath, source.Path, map[string]interface{}{
			"platform":        platform,
			"source_platform": platform,
			"domain":          source.Domain,
			"source_label":    source.Label,
			"original_path":   source.Path,
		})
		if err != nil {
			return nil, err
		}
		result.Files++
		result.Bytes += bytesWritten
		result.Paths = append(result.Paths, hubPath)
	}
	sort.Strings(result.Paths)
	return result, nil
}

func (s *Server) writeLocalPlatformFile(ctx context.Context, userID uuid.UUID, hubPath, srcPath string, metadata map[string]interface{}) (int64, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return 0, err
	}
	contentType := skillsarchive.DetectContentType(srcPath, data)
	if skillsarchive.LooksBinary(srcPath, data) {
		_, err = s.retryLocalPlatformWriteBinaryEntry(ctx, userID, hubPath, data, contentType, models.FileTreeWriteOptions{
			Metadata:      metadata,
			MinTrustLevel: models.TrustLevelWork,
		})
		return int64(len(data)), err
	}
	_, err = s.retryLocalPlatformWriteEntry(ctx, userID, hubPath, string(data), contentType, models.FileTreeWriteOptions{
		Metadata:      metadata,
		MinTrustLevel: models.TrustLevelWork,
	})
	return int64(len(data)), err
}

func (s *Server) exportLocalPlatformSnapshot(ctx context.Context, userID uuid.UUID, platform, outputRoot string) (*sqlitestorage.ExportResult, error) {
	result := &sqlitestorage.ExportResult{Platform: platform, OutputRoot: outputRoot}
	root := filepath.ToSlash(filepath.Join("/platforms", platform))
	snapshot, err := s.FileTreeService.Snapshot(ctx, userID, root, models.TrustLevelFull)
	if err != nil {
		return nil, err
	}
	for _, entry := range snapshot.Entries {
		if entry.IsDirectory {
			continue
		}
		rel := strings.TrimPrefix(entry.Path, root)
		rel = strings.TrimPrefix(rel, "/")
		target := filepath.Join(outputRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if isBinaryMetadata(entry.Metadata) {
			data, _, err := s.FileTreeService.ReadBinary(ctx, userID, entry.Path, models.TrustLevelFull)
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(target, data, 0o644); err != nil {
				return nil, err
			}
			result.Files++
			result.Bytes += int64(len(data))
			result.Paths = append(result.Paths, target)
			continue
		}
		if err := os.WriteFile(target, []byte(entry.Content), 0o644); err != nil {
			return nil, err
		}
		result.Files++
		result.Bytes += int64(len(entry.Content))
		result.Paths = append(result.Paths, target)
	}
	sort.Strings(result.Paths)
	return result, nil
}

func (s *Server) importLocalPlatformAgentPayload(ctx context.Context, userID uuid.UUID, platform string, payload sqlitestorage.AgentExportPayload) (*sqlitestorage.AgentImportResult, error) {
	result := &sqlitestorage.AgentImportResult{Platform: platform}
	source := "agent:" + platform

	if content := renderAgentProfileRules(platform, payload.ProfileRules); strings.TrimSpace(content) != "" {
		if err := s.importAgentProfileRules(ctx, userID, platform, source, content, payload.ProfileRules, result); err != nil {
			return nil, err
		}
	}

	for _, item := range payload.MemoryItems {
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		now := time.Now().UTC()
		expiresAt := now.AddDate(1, 0, 0)
		entry, err := s.importAgentMemoryItem(ctx, userID, platform, source, item, now, &expiresAt, result)
		if err != nil {
			return nil, err
		}
		result.MemoryItems++
		result.Imported++
		if entry != nil {
			result.Paths = append(result.Paths, entry.Path)
		} else {
			result.Paths = append(result.Paths, importedScratchPath(source, item.Title, now))
		}
	}

	for _, project := range payload.Projects {
		name := strings.TrimSpace(project.Name)
		if name == "" || strings.TrimSpace(project.Context) == "" {
			continue
		}
		if _, err := s.ProjectService.Get(ctx, userID, name); err != nil {
			if _, err := s.retryLocalPlatformCreateProject(ctx, userID, name); err != nil {
				return nil, err
			}
		}
		contextBody := renderAgentProjectContext(project)
		if err := s.retryLocalPlatformUpdateProjectContext(ctx, userID, name, contextBody); err != nil {
			return nil, err
		}
		if _, err := s.retryLocalPlatformWriteEntry(ctx, userID, hubpath.ProjectContextPath(name), contextBody, "text/markdown", models.FileTreeWriteOptions{
			Kind:          "project_context",
			MinTrustLevel: models.TrustLevelCollaborate,
			Metadata: map[string]interface{}{
				"source_platform": platform,
				"capture_mode":    "agent",
				"exactness":       project.Exactness,
				"source_paths":    project.SourcePaths,
			},
		}); err != nil {
			return nil, err
		}
		result.Projects++
		result.Imported++
		result.Paths = append(result.Paths, hubpath.ProjectContextPath(name))
	}

	if payload.Claude != nil {
		if err := s.importClaudeLocalInventory(ctx, userID, platform, *payload.Claude, result); err != nil {
			return nil, err
		}
	}
	if payload.Codex != nil {
		if err := s.importCodexLocalInventory(ctx, userID, platform, *payload.Codex, result); err != nil {
			return nil, err
		}
	}

	if err := s.importOptionalAgentArtifact(ctx, userID, platform, "automations.json", payload.Automations, result, len(payload.Automations), false, nil); err != nil {
		return nil, err
	}
	if err := s.importOptionalAgentArtifact(ctx, userID, platform, "tools.json", payload.Tools, result, len(payload.Tools), false, nil); err != nil {
		return nil, err
	}
	if err := s.importOptionalAgentArtifact(ctx, userID, platform, "connections.json", payload.Connections, result, len(payload.Connections), false, nil); err != nil {
		return nil, err
	}
	if err := s.importOptionalAgentArtifact(ctx, userID, platform, "archives.json", payload.Archives, result, len(payload.Archives), false, nil); err != nil {
		return nil, err
	}
	if err := s.importOptionalAgentArtifact(ctx, userID, platform, "unsupported.json", payload.Unsupported, result, len(payload.Unsupported), true, nil); err != nil {
		return nil, err
	}
	if err := s.importOptionalAgentArtifact(ctx, userID, platform, "sensitive-findings.json", payload.SensitiveFindings, result, len(payload.SensitiveFindings), false, func() {
		result.SensitiveFindings += len(payload.SensitiveFindings)
	}); err != nil {
		return nil, err
	}
	if err := s.importOptionalAgentArtifact(ctx, userID, platform, "vault-candidates.json", payload.VaultCandidates, result, len(payload.VaultCandidates), false, func() {
		result.VaultCandidates += len(payload.VaultCandidates)
	}); err != nil {
		return nil, err
	}
	if content := renderAgentNotes(payload.Notes); strings.TrimSpace(content) != "" {
		target := filepath.ToSlash(filepath.Join("/platforms", platform, "agent", "notes.md"))
		if _, err := s.retryLocalPlatformWriteEntry(ctx, userID, target, content, "text/markdown", models.FileTreeWriteOptions{
			Kind:          "file",
			MinTrustLevel: models.TrustLevelWork,
			Metadata: map[string]interface{}{
				"source_platform": platform,
				"capture_mode":    "agent",
				"exactness":       "reference",
			},
		}); err != nil {
			if errors.Is(err, services.ErrStorageQuotaExceeded) {
				result.Blocked++
				sort.Strings(result.Paths)
				return result, nil
			}
			return nil, err
		}
		result.Artifacts++
		result.Archived++
		result.Paths = append(result.Paths, target)
	}

	sort.Strings(result.Paths)
	return result, nil
}

func (s *Server) importOptionalAgentArtifact(ctx context.Context, userID uuid.UUID, platform, filename string, payload any, result *sqlitestorage.AgentImportResult, archivedCount int, countAsBlocked bool, onWritten func()) error {
	written, err := s.writeLocalAgentArtifact(ctx, userID, platform, filename, payload)
	if err != nil {
		if errors.Is(err, services.ErrStorageQuotaExceeded) {
			result.Blocked++
			return nil
		}
		return err
	}
	if written == "" {
		return nil
	}
	result.Artifacts++
	result.Archived += archivedCount
	if countAsBlocked {
		result.Blocked += archivedCount
	}
	if onWritten != nil {
		onWritten()
	}
	result.Paths = append(result.Paths, written)
	return nil
}

func (s *Server) importAgentMemoryItem(ctx context.Context, userID uuid.UUID, platform, source string, item sqlitestorage.AgentMemoryItem, createdAt time.Time, expiresAt *time.Time, result *sqlitestorage.AgentImportResult) (*models.FileTreeEntry, error) {
	content := renderAgentMemoryItem(item)
	title := strings.TrimSpace(item.Title)
	if len(content) <= localPlatformScratchContentLimitBytes {
		return s.retryLocalPlatformImportScratch(ctx, userID, content, source, title, createdAt, expiresAt)
	}

	archivePath := agentMemoryArchivePath(platform, title, createdAt)
	if _, err := s.retryLocalPlatformWriteEntry(ctx, userID, archivePath, content+"\n", "text/markdown", models.FileTreeWriteOptions{
		Kind:          "file",
		MinTrustLevel: models.TrustLevelWork,
		Metadata: map[string]interface{}{
			"source_platform": platform,
			"capture_mode":    "agent",
			"exactness":       "reference",
			"import_kind":     "agent_memory_archive",
			"original_bytes":  len(content),
			"memory_title":    title,
		},
	}); err != nil {
		return nil, err
	}
	result.Artifacts++
	result.Archived++
	result.Paths = append(result.Paths, archivePath)

	summary := renderArchivedAgentMemorySummary(platform, archivePath, len(content), item)
	return s.retryLocalPlatformImportScratch(ctx, userID, summary, source, title, createdAt, expiresAt)
}

func (s *Server) importAgentProfileRules(ctx context.Context, userID uuid.UUID, platform, source, content string, rules []sqlitestorage.AgentProfileRule, result *sqlitestorage.AgentImportResult) error {
	category := platform + "-agent"
	profilePath := hubpath.ProfilePath(category)
	if len(content) <= localPlatformProfileContentLimitBytes {
		if err := s.retryLocalPlatformUpsertProfile(ctx, userID, category, content, source); err != nil {
			return err
		}
		result.ProfileCategories++
		result.Imported++
		result.Paths = append(result.Paths, profilePath)
		return nil
	}

	archivePath := filepath.ToSlash(filepath.Join("/platforms", platform, "agent", "profile-rules.md"))
	if _, err := s.retryLocalPlatformWriteEntry(ctx, userID, archivePath, content+"\n", "text/markdown", models.FileTreeWriteOptions{
		Kind:          "file",
		MinTrustLevel: models.TrustLevelWork,
		Metadata: map[string]interface{}{
			"source_platform":  platform,
			"capture_mode":     "agent",
			"exactness":        "reference",
			"import_kind":      "agent_profile_rules_archive",
			"profile_category": category,
			"original_bytes":   len(content),
		},
	}); err != nil {
		return err
	}

	summary := renderArchivedAgentProfileRulesSummary(platform, archivePath, len(content), rules)
	if err := s.retryLocalPlatformUpsertProfile(ctx, userID, category, summary, source); err != nil {
		return err
	}

	result.ProfileCategories++
	result.Artifacts++
	result.Imported++
	result.Archived++
	result.Paths = append(result.Paths, profilePath, archivePath)
	return nil
}

func renderArchivedAgentProfileRulesSummary(platform, archivePath string, originalBytes int, rules []sqlitestorage.AgentProfileRule) string {
	lines := []string{
		fmt.Sprintf("# %s agent-derived profile rules", platform),
		"",
		"The imported profile rules were larger than a single profile memory entry, so Vola preserved the exact content as a platform archive.",
		"",
		fmt.Sprintf("- Full archive: `%s`", archivePath),
		fmt.Sprintf("- Original size: %d bytes", originalBytes),
	}
	if len(rules) > 0 {
		lines = append(lines, "- Imported rule groups:")
		omitted := 0
		for index, rule := range rules {
			if index >= 12 {
				omitted = len(rules) - index
				break
			}
			title := strings.TrimSpace(rule.Title)
			if title == "" {
				title = "Rule"
			}
			source := ""
			if len(rule.SourcePaths) > 0 {
				source = " — " + strings.Join(rule.SourcePaths, ", ")
			}
			line := "  - " + truncateRunes(title+source, 512)
			next := append(lines, line)
			if len(strings.Join(next, "\n")) > localPlatformProfileSummaryBudgetBytes {
				omitted = len(rules) - index
				break
			}
			lines = next
		}
		if omitted > 0 {
			lines = append(lines, fmt.Sprintf("  - ...and %d more", omitted))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func agentMemoryArchivePath(platform, title string, createdAt time.Time) string {
	rawTitle := strings.TrimSpace(title)
	if rawTitle == "" {
		rawTitle = "memory"
	}
	key := fmt.Sprintf("vola/agent-memory-archive/%s/%s/%s", platform, rawTitle, createdAt.UTC().Format(time.RFC3339Nano))
	id := uuid.NewSHA1(uuid.NameSpaceURL, []byte(key)).String()[:12]
	slug := truncateRunes(rawTitle, 80)
	return filepath.ToSlash(filepath.Join("/platforms", platform, "agent", "memory", createdAt.UTC().Format("2006-01-02")+"-"+slug+"-"+id+".md"))
}

func renderArchivedAgentMemorySummary(platform, archivePath string, originalBytes int, item sqlitestorage.AgentMemoryItem) string {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = "Imported memory"
	}
	lines := []string{
		"# " + title,
		"",
		"The imported memory item was larger than a single scratch memory entry, so Vola preserved the exact content as a platform archive.",
		"",
		fmt.Sprintf("- Source platform: %s", platform),
		fmt.Sprintf("- Full archive: `%s`", archivePath),
		fmt.Sprintf("- Original size: %d bytes", originalBytes),
		fmt.Sprintf("- Exactness: %s", fallbackAgentExactness(item.Exactness)),
	}
	if len(item.SourcePaths) > 0 {
		lines = append(lines, "- Source paths:")
		for index, sourcePath := range item.SourcePaths {
			if index >= 8 {
				lines = append(lines, fmt.Sprintf("  - ...and %d more", len(item.SourcePaths)-index))
				break
			}
			lines = append(lines, "  - "+truncateRunes(sourcePath, 512))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (s *Server) writeLocalAgentArtifact(ctx context.Context, userID uuid.UUID, platform, filename string, payload any) (string, error) {
	if isEmptyAgentPayload(payload) {
		return "", nil
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	target := filepath.ToSlash(filepath.Join("/platforms", platform, "agent", filename))
	if _, err := s.retryLocalPlatformWriteEntry(ctx, userID, target, string(data)+"\n", "application/json", models.FileTreeWriteOptions{
		Kind:          "file",
		MinTrustLevel: models.TrustLevelWork,
		Metadata: map[string]interface{}{
			"source_platform": platform,
			"capture_mode":    "agent",
			"exactness":       "reference",
		},
	}); err != nil {
		return "", err
	}
	return target, nil
}

func (s *Server) importLocalSkillsArchive(ctx context.Context, userID uuid.UUID, platform, archivePath string) (*sqlitestorage.ImportResult, error) {
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open skills archive: %w", err)
	}
	files, err := skillsarchive.ParseZipBytes(data, filepath.Base(archivePath))
	if err != nil {
		return nil, err
	}
	manifests := skillsarchive.BuildManifests(files, platform, filepath.Base(archivePath))
	files, err = skillsarchive.AppendManifestEntries(files, manifests)
	if err != nil {
		return nil, fmt.Errorf("build skill manifests: %w", err)
	}

	result := &sqlitestorage.ImportResult{Platform: platform}
	for _, file := range files {
		hubPath := filepath.ToSlash(path.Join("/skills", file.SkillName, file.RelPath))
		metadata := map[string]interface{}{
			"source_platform": platform,
			"source_archive":  filepath.Base(archivePath),
			"capture_mode":    "archive",
		}
		contentType := skillsarchive.DetectContentType(file.RelPath, file.Data)
		if skillsarchive.LooksBinary(file.RelPath, file.Data) {
			if _, err := s.retryLocalPlatformWriteBinaryEntry(ctx, userID, hubPath, file.Data, contentType, models.FileTreeWriteOptions{
				Kind:          "skill_asset",
				Metadata:      metadata,
				MinTrustLevel: models.TrustLevelWork,
			}); err != nil {
				return nil, fmt.Errorf("write %s: %w", hubPath, err)
			}
		} else {
			if _, err := s.retryLocalPlatformWriteEntry(ctx, userID, hubPath, string(file.Data), contentType, models.FileTreeWriteOptions{
				Kind:          "skill_file",
				Metadata:      metadata,
				MinTrustLevel: models.TrustLevelWork,
			}); err != nil {
				return nil, fmt.Errorf("write %s: %w", hubPath, err)
			}
		}
		if !file.Generated {
			result.Files++
			result.Bytes += int64(len(file.Data))
			result.Paths = append(result.Paths, hubPath)
		}
	}
	sort.Strings(result.Paths)
	return result, nil
}

func isManagedNeuDriveDir(pathValue string) bool {
	_, err := os.Stat(filepath.Join(pathValue, ".vola-managed.json"))
	return err == nil
}

func isBinaryMetadata(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	value, ok := metadata["binary"]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true"
	default:
		return false
	}
}

func renderAgentProfileRules(platform string, rules []sqlitestorage.AgentProfileRule) string {
	if len(rules) == 0 {
		return ""
	}
	lines := []string{
		fmt.Sprintf("# %s agent-derived profile rules", platform),
		"",
	}
	for _, rule := range rules {
		content := strings.TrimSpace(rule.Content)
		if content == "" {
			continue
		}
		title := strings.TrimSpace(rule.Title)
		if title == "" {
			title = "Rule"
		}
		lines = append(lines, "## "+title, "")
		lines = append(lines, content, "")
		lines = append(lines, fmt.Sprintf("- Exactness: %s", fallbackAgentExactness(rule.Exactness)))
		if len(rule.SourcePaths) > 0 {
			lines = append(lines, "- Source paths:")
			for _, source := range rule.SourcePaths {
				lines = append(lines, "  - "+source)
			}
		}
		if rule.Confidence > 0 {
			lines = append(lines, fmt.Sprintf("- Confidence: %.2f", rule.Confidence))
		}
		lines = append(lines, "")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderAgentMemoryItem(item sqlitestorage.AgentMemoryItem) string {
	lines := []string{}
	if title := strings.TrimSpace(item.Title); title != "" {
		lines = append(lines, "# "+title, "")
	}
	lines = append(lines, strings.TrimSpace(item.Content), "")
	lines = append(lines, fmt.Sprintf("- Exactness: %s", fallbackAgentExactness(item.Exactness)))
	if len(item.SourcePaths) > 0 {
		lines = append(lines, "- Source paths:")
		for _, source := range item.SourcePaths {
			lines = append(lines, "  - "+source)
		}
	}
	if item.Confidence > 0 {
		lines = append(lines, fmt.Sprintf("- Confidence: %.2f", item.Confidence))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderAgentProjectContext(project sqlitestorage.AgentProjectRecord) string {
	lines := []string{strings.TrimSpace(project.Context), ""}
	lines = append(lines, fmt.Sprintf("- Exactness: %s", fallbackAgentExactness(project.Exactness)))
	if len(project.SourcePaths) > 0 {
		lines = append(lines, "- Source paths:")
		for _, source := range project.SourcePaths {
			lines = append(lines, "  - "+source)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderAgentNotes(notes []string) string {
	if len(notes) == 0 {
		return ""
	}
	lines := []string{"# Notes", ""}
	for _, note := range notes {
		note = strings.TrimSpace(note)
		if note == "" {
			continue
		}
		lines = append(lines, "- "+note)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func fallbackAgentExactness(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "exact", "derived", "reference":
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return "derived"
	}
}

func isEmptyAgentPayload(payload any) bool {
	switch typed := payload.(type) {
	case []sqlitestorage.AgentRecord:
		return len(typed) == 0
	case []sqlitestorage.AgentSensitiveFinding:
		return len(typed) == 0
	case []sqlitestorage.AgentVaultCandidate:
		return len(typed) == 0
	default:
		return payload == nil
	}
}

func importedScratchPath(source, title string, createdAt time.Time) string {
	key := fmt.Sprintf("vola/imported-scratch/%s/%s/%s", source, title, createdAt.UTC().Format(time.RFC3339Nano))
	legacyID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(key))
	slugBase := title
	if strings.TrimSpace(slugBase) == "" {
		slugBase = source
	}
	slug := fmt.Sprintf("%s-%s", slugBase, legacyID.String()[:8])
	return hubpath.ScratchPath(createdAt.UTC(), slug)
}
