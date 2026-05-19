package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/agi-bar/neudrive/internal/models"
	"github.com/agi-bar/neudrive/internal/services"
	"github.com/agi-bar/neudrive/internal/skillsarchive"
	"github.com/google/uuid"
)

const skillConversionVersion = "neudrive.skill-conversion/v1"

type skillConversionRequest struct {
	SourcePath     string `json:"source_path"`
	SourcePlatform string `json:"source_platform,omitempty"`
	TargetPlatform string `json:"target_platform"`
	TargetPath     string `json:"target_path,omitempty"`
	Overwrite      bool   `json:"overwrite,omitempty"`
	TeamID         string `json:"team_id,omitempty"`
}

type skillConversionResponse struct {
	Version        string                      `json:"version"`
	Scope          string                      `json:"scope"`
	Team           *models.Team                `json:"team,omitempty"`
	Applied        bool                        `json:"applied"`
	ConvertedAt    string                      `json:"converted_at"`
	SourcePath     string                      `json:"source_path"`
	TargetPath     string                      `json:"target_path"`
	SourcePlatform string                      `json:"source_platform"`
	TargetPlatform string                      `json:"target_platform"`
	Summary        skillConversionSummary      `json:"summary"`
	Files          []skillConversionFileChange `json:"files"`
	AutoItems      []skillConversionReportItem `json:"auto_items,omitempty"`
	ManualItems    []skillConversionReportItem `json:"manual_items,omitempty"`
	Unsupported    []skillConversionReportItem `json:"unsupported,omitempty"`
	Warnings       []skillConversionReportItem `json:"warnings,omitempty"`
}

type skillConversionSummary struct {
	Converted int `json:"converted"`
	Copied    int `json:"copied"`
	Generated int `json:"generated"`
	Conflicts int `json:"conflicts"`
	Auto      int `json:"auto"`
	Manual    int `json:"manual"`
	Warnings  int `json:"warnings"`
}

type skillConversionFileChange struct {
	Action     string `json:"action"`
	SourcePath string `json:"source_path,omitempty"`
	TargetPath string `json:"target_path"`
	RelPath    string `json:"rel_path"`
	Reason     string `json:"reason,omitempty"`
	SizeBytes  int64  `json:"size_bytes,omitempty"`
}

type skillConversionReportItem struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

type skillConversionEntry struct {
	RelPath     string
	SourcePath  string
	Data        []byte
	ContentType string
	Generated   bool
}

func (s *Server) handleSkillConversionPreview(w http.ResponseWriter, r *http.Request) {
	_, ok := s.checkSkillConversionAccess(w, r)
	if !ok {
		return
	}
	req, ok := decodeSkillConversionRequest(w, r)
	if !ok {
		return
	}
	target, ok := s.resolveScopedHubTarget(w, r, req.TeamID, false)
	if !ok {
		return
	}
	resp, err := s.buildSkillConversion(r.Context(), target, req, false)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, resp)
}

func (s *Server) handleSkillConversionApply(w http.ResponseWriter, r *http.Request) {
	_, ok := s.checkSkillConversionAccess(w, r)
	if !ok {
		return
	}
	req, ok := decodeSkillConversionRequest(w, r)
	if !ok {
		return
	}
	target, ok := s.resolveScopedHubTarget(w, r, req.TeamID, true)
	if !ok {
		return
	}
	resp, err := s.buildSkillConversion(r.Context(), target, req, true)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	if target.Scope == "personal" {
		respondOKWithLocalGitSync(w, resp, s.syncLocalGitMirror(r.Context(), target.UserID))
		return
	}
	respondOK(w, resp)
}

func (s *Server) checkSkillConversionAccess(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	if !s.agentCheckAuth(w, r, models.TrustLevelWork, models.ScopeWriteSkills) {
		return uuid.Nil, false
	}
	if s.FileTreeService == nil {
		respondNotConfigured(w, "file tree service")
		return uuid.Nil, false
	}
	userID, _ := userIDFromCtx(r.Context())
	return userID, true
}

func decodeSkillConversionRequest(w http.ResponseWriter, r *http.Request) (skillConversionRequest, bool) {
	var req skillConversionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return skillConversionRequest{}, false
	}
	return req, true
}

func (s *Server) buildSkillConversion(ctx context.Context, target scopedHubTarget, req skillConversionRequest, apply bool) (*skillConversionResponse, error) {
	sourcePath := normalizeAssignedSkillPath(req.SourcePath)
	if sourcePath == "" {
		return nil, fmt.Errorf("source_path is required")
	}
	targetPlatform, err := normalizeSkillConversionPlatform(req.TargetPlatform)
	if err != nil {
		return nil, err
	}
	sourceEntries, err := s.collectSkillConversionEntries(ctx, target.UserID, sourcePath)
	if err != nil {
		return nil, err
	}
	if len(sourceEntries) == 0 {
		return nil, fmt.Errorf("%s has no skill files", sourcePath)
	}

	sourceManifest, err := s.readOrBuildSkillConversionManifest(ctx, target.UserID, sourcePath, sourceEntries, req.SourcePlatform)
	if err != nil {
		return nil, err
	}
	sourcePlatform := strings.TrimSpace(req.SourcePlatform)
	if sourcePlatform == "" && sourceManifest != nil {
		sourcePlatform = sourceManifest.SourcePlatform
	}
	if sourcePlatform == "" {
		sourcePlatform = inferSkillConversionSourcePlatform(sourceManifest, sourceEntries, targetPlatform)
	}
	sourcePlatform, err = normalizeSkillConversionPlatform(sourcePlatform)
	if err != nil {
		return nil, err
	}
	if sourcePlatform == targetPlatform {
		return nil, fmt.Errorf("source_platform and target_platform are the same")
	}

	targetPath := normalizeSkillConversionTargetPath(req.TargetPath, sourcePath, targetPlatform)
	targetName := strings.TrimPrefix(targetPath, "/skills/")
	if targetName == "" || targetName == targetPath {
		return nil, fmt.Errorf("target_path must be under /skills")
	}

	converted := buildConvertedSkillEntries(sourceEntries, sourceManifest, sourcePlatform, targetPlatform)
	manifestEntries, err := buildConvertedSkillManifestEntries(converted, targetName, sourcePath, sourcePlatform, targetPlatform)
	if err != nil {
		return nil, err
	}
	converted = append(converted, manifestEntries...)
	sort.Slice(converted, func(i, j int) bool { return converted[i].RelPath < converted[j].RelPath })

	resp := &skillConversionResponse{
		Version:        skillConversionVersion,
		Scope:          target.Scope,
		Team:           target.Team,
		Applied:        apply,
		ConvertedAt:    time.Now().UTC().Format(time.RFC3339),
		SourcePath:     sourcePath,
		TargetPath:     targetPath,
		SourcePlatform: sourcePlatform,
		TargetPlatform: targetPlatform,
		Files:          []skillConversionFileChange{},
	}
	resp.AutoItems = buildSkillConversionAutoItems(sourceManifest, sourceEntries, sourcePlatform, targetPlatform)
	resp.ManualItems, resp.Unsupported, resp.Warnings = buildSkillConversionReport(sourceManifest, sourceEntries, sourcePlatform, targetPlatform)

	for _, entry := range converted {
		targetFilePath := path.Join(targetPath, entry.RelPath)
		action := "copy"
		if entry.Generated {
			action = "generate"
		} else if entry.RelPath == "SKILL.md" {
			action = "convert"
		}
		change := skillConversionFileChange{
			Action:     action,
			SourcePath: entry.SourcePath,
			TargetPath: targetFilePath,
			RelPath:    entry.RelPath,
			SizeBytes:  int64(len(entry.Data)),
		}
		current, err := s.FileTreeService.Read(ctx, target.UserID, targetFilePath, models.TrustLevelFull)
		if err == nil && !req.Overwrite {
			change.Action = "conflict"
			change.Reason = "target file already exists"
			if !isBinaryMetadata(current.Metadata) && current.Content == string(entry.Data) {
				change.Reason = "target file already exists with identical content"
			}
			resp.addConversionFile(change)
			continue
		}
		if err != nil && !errors.Is(err, services.ErrEntryNotFound) {
			return nil, err
		}
		resp.addConversionFile(change)
	}

	resp.Summary.Auto = len(resp.AutoItems)
	resp.Summary.Manual = len(resp.ManualItems) + len(resp.Unsupported)
	resp.Summary.Warnings = len(resp.Warnings)
	if resp.Summary.Conflicts > 0 && apply && !req.Overwrite {
		return resp, nil
	}
	if apply {
		for _, entry := range converted {
			targetFilePath := path.Join(targetPath, entry.RelPath)
			if !req.Overwrite {
				if _, err := s.FileTreeService.Read(ctx, target.UserID, targetFilePath, models.TrustLevelFull); err == nil {
					continue
				} else if err != nil && !errors.Is(err, services.ErrEntryNotFound) {
					return nil, err
				}
			}
			metadata := map[string]interface{}{
				"source_platform":     sourcePlatform,
				"target_platform":     targetPlatform,
				"source_skill_path":   sourcePath,
				"conversion_version":  skillConversionVersion,
				"conversion_applied":  true,
				"conversion_rel_path": entry.RelPath,
			}
			contentType := skillsarchive.DetectContentType(entry.RelPath, entry.Data)
			if skillsarchive.LooksBinary(entry.RelPath, entry.Data) {
				if _, err := s.FileTreeService.WriteBinaryEntry(ctx, target.UserID, targetFilePath, entry.Data, contentType, models.FileTreeWriteOptions{
					Kind:          "skill_asset",
					Metadata:      metadata,
					MinTrustLevel: models.TrustLevelWork,
				}); err != nil {
					return nil, err
				}
				continue
			}
			if _, err := s.FileTreeService.WriteEntry(ctx, target.UserID, targetFilePath, string(entry.Data), contentType, models.FileTreeWriteOptions{
				Kind:          "skill_file",
				Metadata:      metadata,
				MinTrustLevel: models.TrustLevelWork,
			}); err != nil {
				return nil, err
			}
		}
	}
	return resp, nil
}

func (s *Server) collectSkillConversionEntries(ctx context.Context, userID uuid.UUID, sourcePath string) ([]skillConversionEntry, error) {
	files, err := s.collectLocalSkillFiles(ctx, userID, sourcePath)
	if err != nil {
		return nil, err
	}
	out := make([]skillConversionEntry, 0, len(files))
	for _, file := range files {
		if file.RelPath == "" {
			continue
		}
		out = append(out, skillConversionEntry{
			RelPath:     file.RelPath,
			SourcePath:  file.HubPath,
			Data:        append([]byte{}, file.Data...),
			ContentType: file.ContentType,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out, nil
}

func (s *Server) readOrBuildSkillConversionManifest(ctx context.Context, userID uuid.UUID, sourcePath string, entries []skillConversionEntry, sourcePlatform string) (*skillsarchive.SkillManifest, error) {
	manifestPath := path.Join(sourcePath, skillsarchive.ManifestFile)
	entry, err := s.FileTreeService.Read(ctx, userID, manifestPath, models.TrustLevelFull)
	if err == nil {
		var manifest skillsarchive.SkillManifest
		if err := json.Unmarshal([]byte(entry.Content), &manifest); err == nil && manifest.Version != "" {
			return &manifest, nil
		}
	}
	if err != nil && !errors.Is(err, services.ErrEntryNotFound) {
		return nil, err
	}
	skillName := strings.TrimPrefix(sourcePath, "/skills/")
	sourceArchiveEntries := make([]skillsarchive.Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.RelPath == skillsarchive.ManifestFile {
			continue
		}
		sourceArchiveEntries = append(sourceArchiveEntries, skillsarchive.Entry{
			SkillName: skillName,
			RelPath:   entry.RelPath,
			Data:      entry.Data,
		})
	}
	manifests := skillsarchive.BuildManifests(sourceArchiveEntries, sourcePlatform, "conversion-preview")
	if len(manifests) == 0 {
		return nil, nil
	}
	return &manifests[0], nil
}

func buildConvertedSkillEntries(entries []skillConversionEntry, manifest *skillsarchive.SkillManifest, sourcePlatform, targetPlatform string) []skillConversionEntry {
	out := make([]skillConversionEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.RelPath == skillsarchive.ManifestFile {
			continue
		}
		next := entry
		next.Data = append([]byte{}, entry.Data...)
		if entry.RelPath == "SKILL.md" && !skillsarchive.LooksBinary(entry.RelPath, entry.Data) {
			next.Data = []byte(adaptSkillMarkdownForConversion(string(entry.Data), manifest, sourcePlatform, targetPlatform))
		}
		out = append(out, next)
	}
	return out
}

func buildConvertedSkillManifestEntries(entries []skillConversionEntry, targetName, sourcePath, sourcePlatform, targetPlatform string) ([]skillConversionEntry, error) {
	archiveEntries := make([]skillsarchive.Entry, 0, len(entries))
	for _, entry := range entries {
		archiveEntries = append(archiveEntries, skillsarchive.Entry{
			SkillName: targetName,
			RelPath:   entry.RelPath,
			Data:      entry.Data,
		})
	}
	manifests := skillsarchive.BuildManifests(archiveEntries, targetPlatform, "converted-from-"+strings.TrimPrefix(sourcePath, "/skills/"))
	if len(manifests) == 0 {
		return nil, nil
	}
	manifests[0].Warnings = append(manifests[0].Warnings, skillsarchive.SkillManifestWarning{
		Code:     "converted_skill",
		Severity: "info",
		Message:  fmt.Sprintf("Converted from %s to %s by neuDrive.", sourcePlatform, targetPlatform),
	})
	withManifest, err := skillsarchive.AppendManifestEntries(nil, manifests)
	if err != nil {
		return nil, err
	}
	out := make([]skillConversionEntry, 0, len(withManifest))
	for _, entry := range withManifest {
		if entry.RelPath != skillsarchive.ManifestFile {
			continue
		}
		out = append(out, skillConversionEntry{
			RelPath:   entry.RelPath,
			Data:      entry.Data,
			Generated: true,
		})
	}
	return out, nil
}

func buildSkillConversionReport(manifest *skillsarchive.SkillManifest, entries []skillConversionEntry, sourcePlatform, targetPlatform string) ([]skillConversionReportItem, []skillConversionReportItem, []skillConversionReportItem) {
	manual := []skillConversionReportItem{}
	unsupported := []skillConversionReportItem{}
	warnings := []skillConversionReportItem{}
	if manifest != nil {
		for _, ref := range manifest.ExternalReferences {
			if !ref.Included {
				manual = append(manual, skillConversionReportItem{
					Code:     "external_reference_missing",
					Severity: "warning",
					Path:     ref.SourceFile,
					Message:  ref.Path + " is referenced outside the skill folder and needs manual upload before the converted skill can use it.",
				})
			}
		}
		if len(manifest.EnvVars) > 0 {
			manual = append(manual, skillConversionReportItem{
				Code:     "env_vars_required",
				Severity: "warning",
				Message:  "Environment variables needed by this skill: " + strings.Join(manifest.EnvVars, ", "),
			})
		}
		if manifest.Summary.Scripts > 0 {
			manual = append(manual, skillConversionReportItem{
				Code:     "script_runtime_check",
				Severity: "warning",
				Message:  "This skill includes scripts. Verify interpreter paths, executable bits, and runtime permissions in " + targetPlatform + ".",
			})
		}
		if manifest.Summary.DependencyFiles > 0 {
			manual = append(manual, skillConversionReportItem{
				Code:     "dependencies_required",
				Severity: "warning",
				Message:  "This skill includes dependency files. Install or prepare the matching Python/Node environment for " + targetPlatform + ".",
			})
		}
		for _, item := range manifest.Warnings {
			warnings = append(warnings, skillConversionReportItem{
				Code:     item.Code,
				Severity: item.Severity,
				Path:     item.Path,
				Message:  item.Message,
			})
		}
	}
	for _, entry := range entries {
		lower := strings.ToLower(entry.RelPath)
		switch {
		case strings.Contains(lower, ".codex-plugin/") || strings.HasSuffix(lower, "plugin.json"):
			unsupported = append(unsupported, skillConversionReportItem{
				Code:     "plugin_config",
				Severity: "warning",
				Path:     entry.RelPath,
				Message:  "Plugin metadata is copied as reference material, but plugin installation is not converted automatically.",
			})
		case strings.Contains(lower, "mcp") && (strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".toml") || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")):
			manual = append(manual, skillConversionReportItem{
				Code:     "mcp_config",
				Severity: "warning",
				Path:     entry.RelPath,
				Message:  "MCP configuration is platform-specific. Recreate server registration and secrets in " + targetPlatform + ".",
			})
		case strings.Contains(lower, "hook"):
			manual = append(manual, skillConversionReportItem{
				Code:     "hook_config",
				Severity: "warning",
				Path:     entry.RelPath,
				Message:  "Hook behavior is platform-specific. Review this file before enabling the converted skill.",
			})
		}
	}
	if sourcePlatform == "claude-code" && targetPlatform == "codex" {
		warnings = append(warnings, skillConversionReportItem{
			Code:     "claude_to_codex",
			Severity: "info",
			Message:  "SKILL.md and included assets are copied into Codex-compatible layout. Claude-only plugins and external tools require review.",
		})
	}
	if sourcePlatform == "codex" && targetPlatform == "claude-code" {
		warnings = append(warnings, skillConversionReportItem{
			Code:     "codex_to_claude",
			Severity: "info",
			Message:  "SKILL.md and included assets are copied into Claude-compatible layout. Codex plugins, MCP config, and hooks require review.",
		})
	}
	return dedupeConversionItems(manual), dedupeConversionItems(unsupported), dedupeConversionItems(warnings)
}

func buildSkillConversionAutoItems(manifest *skillsarchive.SkillManifest, entries []skillConversionEntry, sourcePlatform, targetPlatform string) []skillConversionReportItem {
	items := []skillConversionReportItem{
		{
			Code:     "skill_markdown_converted",
			Severity: "info",
			Path:     "SKILL.md",
			Message:  "SKILL.md is copied with neuDrive conversion notes for " + targetPlatform + ".",
		},
		{
			Code:     "file_tree_preserved",
			Severity: "info",
			Message:  "Skill files are copied into the target skill folder without dropping scripts, dependency files, assets, or included external reference files.",
		},
	}
	if manifest != nil {
		if manifest.Summary.Scripts > 0 {
			items = append(items, skillConversionReportItem{
				Code:     "script_files_copied",
				Severity: "info",
				Message:  "Script files are copied into the converted skill. Runtime permissions still need user review.",
			})
		}
		if manifest.Summary.DependencyFiles > 0 {
			items = append(items, skillConversionReportItem{
				Code:     "dependency_files_copied",
				Severity: "info",
				Message:  "Dependency files such as requirements.txt, pyproject.toml, package.json, and lock files are copied.",
			})
		}
		if manifest.Summary.Resources > 0 || manifest.Summary.BinaryFiles > 0 {
			items = append(items, skillConversionReportItem{
				Code:     "assets_copied",
				Severity: "info",
				Message:  "Asset and resource files are copied into the converted skill folder.",
			})
		}
		for _, ref := range manifest.ExternalReferences {
			if ref.Included {
				items = append(items, skillConversionReportItem{
					Code:     "external_reference_included",
					Severity: "info",
					Path:     ref.SourceFile,
					Message:  ref.Path + " is included in the converted skill package.",
				})
			}
		}
	}
	if sourcePlatform == "claude-code" && targetPlatform == "codex" {
		items = append(items, skillConversionReportItem{
			Code:     "claude_external_paths_rewritten",
			Severity: "info",
			Message:  "Claude external tool references in SKILL.md are rewritten to packaged external/ paths when the referenced files were included.",
		})
	}
	if len(entries) == 0 {
		return dedupeConversionItems(items)
	}
	return dedupeConversionItems(items)
}

func adaptSkillMarkdownForConversion(content string, manifest *skillsarchive.SkillManifest, sourcePlatform, targetPlatform string) string {
	trimmed := strings.TrimSpace(content)
	body := removeExistingConversionBlock(trimmed)
	if sourcePlatform == "claude-code" && targetPlatform == "codex" {
		for _, ref := range skillsarchive.ExtractClaudeExternalReferences(body) {
			if rel, ok := skillsarchive.ExternalAssetPathForClaudeReference(ref); ok {
				body = strings.ReplaceAll(body, ref, rel)
			}
		}
	}
	noteLines := []string{
		"<!-- neudrive-conversion:start -->",
		"## neuDrive Conversion Notes",
		"",
		"- Source platform: " + sourcePlatform,
		"- Target platform: " + targetPlatform,
	}
	if manifest != nil {
		if manifest.Summary.Scripts > 0 {
			noteLines = append(noteLines, "- Scripts copied: verify runtime and permissions before relying on them.")
		}
		if manifest.Summary.DependencyFiles > 0 {
			noteLines = append(noteLines, "- Dependencies copied: prepare the matching runtime environment.")
		}
		if len(manifest.EnvVars) > 0 {
			noteLines = append(noteLines, "- Environment variables: "+strings.Join(manifest.EnvVars, ", "))
		}
		if len(manifest.ExternalReferences) > 0 {
			noteLines = append(noteLines, "- External references: check `manifest.neudrive.json` for included and missing files.")
		}
	}
	noteLines = append(noteLines, "<!-- neudrive-conversion:end -->")
	return strings.TrimSpace(body) + "\n\n" + strings.Join(noteLines, "\n") + "\n"
}

func removeExistingConversionBlock(content string) string {
	const start = "<!-- neudrive-conversion:start -->"
	const end = "<!-- neudrive-conversion:end -->"
	startIndex := strings.Index(content, start)
	endIndex := strings.Index(content, end)
	if startIndex >= 0 && endIndex > startIndex {
		endIndex += len(end)
		return strings.TrimSpace(content[:startIndex] + content[endIndex:])
	}
	return content
}

func normalizeSkillConversionPlatform(value string) (string, error) {
	clean := strings.ToLower(strings.TrimSpace(value))
	switch clean {
	case "claude", "claude-code":
		return "claude-code", nil
	case "codex", "codex-cli":
		return "codex", nil
	default:
		return "", fmt.Errorf("unsupported platform %q", value)
	}
}

func inferSkillConversionSourcePlatform(manifest *skillsarchive.SkillManifest, entries []skillConversionEntry, targetPlatform string) string {
	if manifest != nil {
		for _, platform := range manifest.SupportedPlatforms {
			if normalized, err := normalizeSkillConversionPlatform(platform); err == nil && normalized != targetPlatform {
				return normalized
			}
		}
	}
	for _, entry := range entries {
		content := string(entry.Data)
		if strings.Contains(content, ".claude/") || strings.Contains(strings.ToLower(content), "claude") {
			return "claude-code"
		}
		if strings.Contains(strings.ToLower(content), "codex") || strings.Contains(strings.ToLower(entry.RelPath), ".codex") {
			return "codex"
		}
	}
	if targetPlatform == "codex" {
		return "claude-code"
	}
	return "codex"
}

func normalizeSkillConversionTargetPath(targetPath, sourcePath, targetPlatform string) string {
	normalized := normalizeAssignedSkillPath(targetPath)
	if normalized != "" {
		return normalized
	}
	sourceName := strings.TrimPrefix(sourcePath, "/skills/")
	sourceName = strings.TrimSuffix(sourceName, "/")
	suffix := "codex"
	if targetPlatform == "claude-code" {
		suffix = "claude"
	}
	return path.Clean("/skills/" + strings.Trim(sourceName, "/") + "-" + suffix)
}

func (r *skillConversionResponse) addConversionFile(change skillConversionFileChange) {
	r.Files = append(r.Files, change)
	switch change.Action {
	case "convert":
		r.Summary.Converted++
	case "copy":
		r.Summary.Copied++
	case "generate":
		r.Summary.Generated++
	case "conflict":
		r.Summary.Conflicts++
	}
}

func dedupeConversionItems(items []skillConversionReportItem) []skillConversionReportItem {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := []skillConversionReportItem{}
	for _, item := range items {
		key := item.Code + "\x00" + item.Path + "\x00" + item.Message
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity < out[j].Severity
		}
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Path < out[j].Path
	})
	return out
}
