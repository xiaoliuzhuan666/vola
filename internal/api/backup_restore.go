package api

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/agi-bar/vola/internal/hubpath"
	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/google/uuid"
)

const (
	restoreModeSkip      = "skip"
	restoreModeOverwrite = "overwrite"
	maxRestoreEntryBytes = 50 << 20
	maxRestoreTotalBytes = 200 << 20
)

type backupRestorePreviewResponse struct {
	Recognized bool                    `json:"recognized"`
	FileName   string                  `json:"file_name,omitempty"`
	SizeBytes  int64                   `json:"size_bytes"`
	TotalFiles int                     `json:"total_files"`
	TotalBytes int64                   `json:"total_bytes"`
	Categories []backupRestoreCategory `json:"categories"`
	Warnings   []string                `json:"warnings,omitempty"`
}

type backupRestoreApplyResponse struct {
	Recognized  bool                        `json:"recognized"`
	Mode        string                      `json:"mode"`
	Applied     int                         `json:"applied"`
	Skipped     int                         `json:"skipped"`
	Overwritten int                         `json:"overwritten"`
	Errors      []string                    `json:"errors,omitempty"`
	Warnings    []string                    `json:"warnings,omitempty"`
	Entries     []backupRestoreAppliedEntry `json:"entries"`
}

type backupRestoreCategory struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

type backupRestoreAppliedEntry struct {
	Path     string `json:"path"`
	ZipPath  string `json:"zip_path"`
	Category string `json:"category"`
	Action   string `json:"action"`
	Bytes    int64  `json:"bytes"`
	Error    string `json:"error,omitempty"`
}

type backupRestoreEntry struct {
	ZipPath    string
	CleanPath  string
	TargetPath string
	CategoryID string
	Label      string
	Bytes      int64
	Data       []byte
}

func (s *Server) handleBackupRestorePreview(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	fileName, payload, err := readBackupRestorePayload(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	preview, err := buildBackupRestorePreview(fileName, payload)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOK(w, preview)
}

func (s *Server) handleBackupRestoreApply(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
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
	fileName, payload, err := readBackupRestorePayload(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	mode := normalizeRestoreMode(restoreModeFromRequest(r))
	ctx := s.requestSourceContext(r, "restore")
	result, err := s.applyBackupRestore(ctx, userID, fileName, payload, mode)
	if err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	respondOKWithLocalGitSync(w, result, s.syncLocalGitMirror(r.Context(), userID))
}

func readBackupRestorePayload(r *http.Request) (string, []byte, error) {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		file, header, err := r.FormFile("file")
		if err != nil {
			return "", nil, fmt.Errorf("backup zip file is required")
		}
		defer file.Close()
		payload, err := io.ReadAll(file)
		if err != nil {
			return "", nil, err
		}
		fileName := ""
		if header != nil {
			fileName = header.Filename
		}
		return fileName, payload, nil
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return "", nil, err
	}
	fileName := strings.TrimSpace(r.URL.Query().Get("filename"))
	if fileName != "" {
		fileName = filepath.Base(fileName)
	}
	return fileName, payload, nil
}

func buildBackupRestorePreview(fileName string, payload []byte) (backupRestorePreviewResponse, error) {
	preview := backupRestorePreviewResponse{
		FileName:   strings.TrimSpace(fileName),
		SizeBytes:  int64(len(payload)),
		Categories: []backupRestoreCategory{},
	}
	entries, warnings, recognized, err := inspectBackupRestoreEntries(payload, false)
	if err != nil {
		return preview, err
	}
	preview.Recognized = recognized
	preview.Warnings = append(preview.Warnings, warnings...)

	categories := map[string]*backupRestoreCategory{}
	unknownFiles := 0
	for _, entry := range entries {
		categoryID, label := entry.CategoryID, entry.Label
		if categoryID == "unknown" {
			unknownFiles++
		}
		category := categories[categoryID]
		if category == nil {
			category = &backupRestoreCategory{ID: categoryID, Label: label}
			categories[categoryID] = category
		}
		category.Files++
		category.Bytes += entry.Bytes
		preview.TotalFiles++
		preview.TotalBytes += entry.Bytes
	}

	keys := make([]string, 0, len(categories))
	for key := range categories {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		preview.Categories = append(preview.Categories, *categories[key])
	}

	if preview.TotalFiles == 0 {
		preview.Warnings = append(preview.Warnings, "ZIP 内没有可恢复文件。")
	}
	if !preview.Recognized {
		preview.Warnings = append(preview.Warnings, "这不像 Vola 导出的备份包，建议先人工确认来源。")
	}
	if categories["vault"] != nil {
		preview.Warnings = append(preview.Warnings, "备份包包含 Vault 范围；正式恢复时需要再次确认密钥和权限。")
	}
	if unknownFiles > 0 {
		preview.Warnings = append(preview.Warnings, fmt.Sprintf("有 %d 个文件不属于当前已识别的数据分类。", unknownFiles))
	}
	return preview, nil
}

func (s *Server) applyBackupRestore(ctx context.Context, userID uuid.UUID, fileName string, payload []byte, mode string) (backupRestoreApplyResponse, error) {
	result := backupRestoreApplyResponse{
		Mode:    normalizeRestoreMode(mode),
		Entries: []backupRestoreAppliedEntry{},
	}
	entries, warnings, recognized, err := inspectBackupRestoreEntries(payload, true)
	if err != nil {
		return result, err
	}
	result.Recognized = recognized
	result.Warnings = append(result.Warnings, warnings...)
	if !recognized {
		return result, fmt.Errorf("backup zip file is not recognized as a Vola export")
	}
	for _, warning := range warnings {
		if strings.Contains(warning, "不安全路径") {
			return result, fmt.Errorf("backup zip contains unsafe paths")
		}
	}
	vaultWarningAdded := false
	for _, entry := range entries {
		applied := backupRestoreAppliedEntry{
			Path:     entry.TargetPath,
			ZipPath:  entry.ZipPath,
			Category: entry.CategoryID,
			Bytes:    entry.Bytes,
		}
		if entry.TargetPath == "" || entry.CategoryID == "unknown" {
			applied.Action = "skipped"
			result.Skipped++
			result.Entries = append(result.Entries, applied)
			continue
		}
		if entry.CategoryID == "vault" && !vaultWarningAdded {
			result.Warnings = append(result.Warnings, "Vault 恢复只写回导出包里的范围清单；secret 原值需要从数据库备份或密钥系统恢复。")
			vaultWarningAdded = true
		}
		exists := false
		if _, err := s.FileTreeService.Read(ctx, userID, entry.TargetPath, models.TrustLevelFull); err == nil {
			exists = true
		} else if !errors.Is(err, services.ErrEntryNotFound) {
			applied.Action = "error"
			applied.Error = err.Error()
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", entry.TargetPath, err))
			result.Entries = append(result.Entries, applied)
			continue
		}
		if exists && result.Mode == restoreModeSkip {
			applied.Action = "skipped"
			result.Skipped++
			result.Entries = append(result.Entries, applied)
			continue
		}
		metadata := services.WithSourceMetadata(nil, "restore")
		metadata["restore_zip"] = strings.TrimSpace(fileName)
		metadata["restore_zip_path"] = entry.ZipPath
		metadata["restore_category"] = entry.CategoryID
		opts := models.FileTreeWriteOptions{
			Kind:          backupRestoreKind(entry.CategoryID, entry.TargetPath),
			MinTrustLevel: backupRestoreTrustLevel(entry.CategoryID),
			Metadata:      metadata,
		}
		contentType := backupRestoreContentType(entry.TargetPath, entry.Data)
		if backupRestoreShouldWriteText(contentType, entry.Data) {
			_, err = s.FileTreeService.WriteEntry(ctx, userID, entry.TargetPath, string(entry.Data), contentType, opts)
		} else {
			_, err = s.FileTreeService.WriteBinaryEntry(ctx, userID, entry.TargetPath, entry.Data, contentType, opts)
		}
		if err != nil {
			applied.Action = "error"
			applied.Error = err.Error()
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", entry.TargetPath, err))
			result.Entries = append(result.Entries, applied)
			continue
		}
		if exists {
			applied.Action = "overwritten"
			result.Overwritten++
		} else {
			applied.Action = "created"
		}
		result.Applied++
		result.Entries = append(result.Entries, applied)
	}
	return result, nil
}

func normalizeBackupZipPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.TrimPrefix(value, "/")
	parts := strings.Split(value, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, "/")
}

func inspectBackupRestoreEntries(payload []byte, includeData bool) ([]backupRestoreEntry, []string, bool, error) {
	if len(payload) == 0 {
		return nil, nil, false, fmt.Errorf("backup zip file is empty")
	}
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, nil, false, fmt.Errorf("backup zip file cannot be read")
	}
	entries := []backupRestoreEntry{}
	warnings := []string{}
	unsafeFiles := 0
	hasExportPrefix := false
	hasRecognizedCategory := false
	for _, zipEntry := range reader.File {
		if zipEntry.FileInfo().IsDir() {
			continue
		}
		if zipEntry.UncompressedSize64 > maxRestoreEntryBytes {
			return nil, nil, false, fmt.Errorf("backup zip entry %s is too large", zipEntry.Name)
		}
		var totalBytes uint64
		for _, entry := range entries {
			totalBytes += uint64(entry.Bytes)
		}
		if totalBytes+zipEntry.UncompressedSize64 > maxRestoreTotalBytes {
			return nil, nil, false, fmt.Errorf("backup zip uncompressed size is too large")
		}
		cleanPath, hadExportPrefix, safe := normalizeBackupZipPathStrict(zipEntry.Name)
		if !safe {
			unsafeFiles++
			continue
		}
		if cleanPath == "" {
			continue
		}
		if hadExportPrefix {
			hasExportPrefix = true
		}
		categoryID, label := backupRestoreCategoryForPath(cleanPath)
		if categoryID != "unknown" {
			hasRecognizedCategory = true
		}
		entry := backupRestoreEntry{
			ZipPath:    zipEntry.Name,
			CleanPath:  cleanPath,
			TargetPath: backupRestoreTargetPath(cleanPath),
			CategoryID: categoryID,
			Label:      label,
			Bytes:      int64(zipEntry.UncompressedSize64),
		}
		if includeData {
			file, err := zipEntry.Open()
			if err != nil {
				return nil, nil, false, err
			}
			entry.Data, err = io.ReadAll(file)
			_ = file.Close()
			if err != nil {
				return nil, nil, false, err
			}
		}
		entries = append(entries, entry)
	}
	if unsafeFiles > 0 {
		warnings = append(warnings, fmt.Sprintf("有 %d 个 ZIP 条目包含不安全路径，已忽略。", unsafeFiles))
	}
	return entries, warnings, len(entries) > 0 && (hasExportPrefix || hasRecognizedCategory), nil
}

func normalizeBackupZipPathStrict(value string) (string, bool, bool) {
	normalized := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if normalized == "" || strings.HasPrefix(normalized, "/") {
		return "", false, false
	}
	parts := strings.Split(normalized, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "." || part == ".." {
			return "", false, false
		}
		cleaned = append(cleaned, part)
	}
	if len(cleaned) == 0 {
		return "", false, true
	}
	hadExportPrefix := cleaned[0] == "export"
	if hadExportPrefix {
		cleaned = cleaned[1:]
	}
	if len(cleaned) == 0 {
		return "", hadExportPrefix, true
	}
	return strings.Join(cleaned, "/"), hadExportPrefix, true
}

func backupRestoreTargetPath(cleanPath string) string {
	if cleanPath == "" || cleanPath == "metadata.json" {
		return ""
	}
	switch {
	case strings.HasPrefix(cleanPath, "skills/"):
		return hubpath.NormalizeStorage("/" + cleanPath)
	case strings.HasPrefix(cleanPath, "memory/profile/"):
		return hubpath.NormalizeStorage("/" + cleanPath)
	case strings.HasPrefix(cleanPath, "memory/scratch/"):
		return hubpath.NormalizeStorage("/" + cleanPath)
	case strings.HasPrefix(cleanPath, "memory/projects/"):
		return hubpath.NormalizeStorage("/projects/" + strings.TrimPrefix(cleanPath, "memory/projects/"))
	case strings.HasPrefix(cleanPath, "projects/"):
		return hubpath.NormalizeStorage("/" + cleanPath)
	case strings.HasPrefix(cleanPath, "roles/"):
		return hubpath.NormalizeStorage("/" + cleanPath)
	case strings.HasPrefix(cleanPath, "inbox/"):
		return hubpath.NormalizeStorage("/" + cleanPath)
	case strings.HasPrefix(cleanPath, "identity/"):
		return hubpath.NormalizeStorage("/" + cleanPath)
	case strings.HasPrefix(cleanPath, "vault/"):
		return hubpath.NormalizeStorage("/" + cleanPath)
	default:
		return ""
	}
}

func restoreModeFromRequest(r *http.Request) string {
	if value := strings.TrimSpace(r.URL.Query().Get("mode")); value != "" {
		return value
	}
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		_ = r.ParseMultipartForm(50 << 20)
		return strings.TrimSpace(r.FormValue("mode"))
	}
	return ""
}

func normalizeRestoreMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case restoreModeOverwrite:
		return restoreModeOverwrite
	default:
		return restoreModeSkip
	}
}

func backupRestoreContentType(targetPath string, data []byte) string {
	if ext := path.Ext(targetPath); ext != "" {
		if detected := mime.TypeByExtension(ext); detected != "" {
			return detected
		}
	}
	if utf8.Valid(data) {
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

func backupRestoreShouldWriteText(contentType string, data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "yaml") ||
		strings.Contains(contentType, "javascript")
}

func backupRestoreKind(categoryID string, targetPath string) string {
	switch categoryID {
	case "skills":
		return "skill"
	case "memory_profile":
		return "memory_profile"
	case "projects":
		if strings.HasSuffix(targetPath, "/context.md") {
			return "project_context"
		}
		return "project"
	case "roles":
		return "role"
	case "inbox":
		return "inbox"
	case "identity":
		return "identity"
	case "vault":
		return "vault"
	default:
		return ""
	}
}

func backupRestoreTrustLevel(categoryID string) int {
	switch categoryID {
	case "skills":
		return models.TrustLevelGuest
	case "projects":
		return models.TrustLevelWork
	default:
		return models.TrustLevelFull
	}
}

func backupRestoreCategoryForPath(pathValue string) (string, string) {
	switch {
	case pathValue == "identity/profile.json" || strings.HasPrefix(pathValue, "identity/"):
		return "identity", "Identity"
	case strings.HasPrefix(pathValue, "vault/"):
		return "vault", "Vault"
	case strings.HasPrefix(pathValue, "skills/"):
		return "skills", "Skills"
	case strings.HasPrefix(pathValue, "memory/profile/"):
		return "memory_profile", "Memory profile"
	case strings.HasPrefix(pathValue, "memory/projects/") || strings.HasPrefix(pathValue, "projects/"):
		return "projects", "Projects"
	case strings.HasPrefix(pathValue, "memory/scratch/"):
		return "scratch", "Scratch"
	case strings.HasPrefix(pathValue, "roles/"):
		return "roles", "Roles"
	case strings.HasPrefix(pathValue, "inbox/"):
		return "inbox", "Inbox"
	default:
		return "unknown", "Other files"
	}
}
