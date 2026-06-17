package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/agi-bar/vola/internal/models"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListProjectMaterials(w http.ResponseWriter, r *http.Request) {
	if s.ProjectService == nil {
		respondNotConfigured(w, "project service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	materials, err := s.ProjectService.ListMaterials(r.Context(), userID, chi.URLParam(r, "name"))
	if err != nil {
		respondNotFound(w, "project materials")
		return
	}
	respondOK(w, map[string]interface{}{"materials": materials})
}

func (s *Server) handleSaveProjectMaterial(w http.ResponseWriter, r *http.Request) {
	if s.ProjectService == nil {
		respondNotConfigured(w, "project service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req models.ProjectMaterialInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	material, err := s.ProjectService.SaveMaterial(s.requestSourceContext(r, "manual"), userID, chi.URLParam(r, "name"), req)
	if err != nil {
		respondValidationError(w, "material", err.Error())
		return
	}
	respondCreatedWithLocalGitSync(w, material, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleCopyProjectMaterial(w http.ResponseWriter, r *http.Request) {
	if s.ProjectService == nil {
		respondNotConfigured(w, "project service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req models.ProjectMaterialCopyInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	material, err := s.ProjectService.CopyMaterial(s.requestSourceContext(r, "manual"), userID, chi.URLParam(r, "name"), req)
	if err != nil {
		respondValidationError(w, "material", err.Error())
		return
	}
	respondCreatedWithLocalGitSync(w, material, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleListProjectContextPacks(w http.ResponseWriter, r *http.Request) {
	if s.ProjectService == nil {
		respondNotConfigured(w, "project service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	packs, err := s.ProjectService.ListContextPacks(r.Context(), userID, chi.URLParam(r, "name"))
	if err != nil {
		respondNotFound(w, "project context packs")
		return
	}
	respondOK(w, map[string]interface{}{"context_packs": packs})
}

func (s *Server) handleBuildProjectContextPack(w http.ResponseWriter, r *http.Request) {
	if s.ProjectService == nil {
		respondNotConfigured(w, "project service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req models.ProjectContextPackInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	pack, err := s.ProjectService.BuildContextPack(s.requestSourceContext(r, "manual"), userID, chi.URLParam(r, "name"), req)
	if err != nil {
		respondValidationError(w, "context_pack", err.Error())
		return
	}
	respondCreatedWithLocalGitSync(w, pack, s.syncLocalGitMirror(r.Context(), userID))
}

func (s *Server) handleReadProjectContextPack(w http.ResponseWriter, r *http.Request) {
	if s.ProjectService == nil {
		respondNotConfigured(w, "project service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	pack, err := s.ProjectService.ReadContextPack(r.Context(), userID, chi.URLParam(r, "name"), chi.URLParam(r, "pack"))
	if err != nil {
		respondNotFound(w, "project context pack")
		return
	}
	respondOK(w, map[string]interface{}{"context_pack": pack})
}

func (s *Server) handleBuildProjectRepositoryExport(w http.ResponseWriter, r *http.Request) {
	if s.ProjectService == nil {
		respondNotConfigured(w, "project service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req models.ProjectRepositoryExportInput
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
			return
		}
	}
	export, err := s.ProjectService.BuildRepositoryExport(r.Context(), userID, chi.URLParam(r, "name"), req)
	if err != nil {
		respondValidationError(w, "repository_export", err.Error())
		return
	}
	respondOK(w, map[string]interface{}{"repository_export": export})
}

func (s *Server) handleApplyProjectRepositoryExport(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalMode() {
		respondError(w, http.StatusNotImplemented, ErrCodeUnsupported, "repository export apply is only available in local mode")
		return
	}
	if s.ProjectService == nil {
		respondNotConfigured(w, "project service")
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	var req models.ProjectRepositoryExportApplyInput
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
			return
		}
	}
	export, err := s.ProjectService.BuildRepositoryExport(r.Context(), userID, chi.URLParam(r, "name"), models.ProjectRepositoryExportInput{
		RepositoryDir: req.RepositoryDir,
		MaterialPaths: req.MaterialPaths,
		PackPaths:     req.PackPaths,
		IncludeIndex:  req.IncludeIndex,
	})
	if err != nil {
		respondValidationError(w, "repository_export", err.Error())
		return
	}
	result, err := writeProjectRepositoryExport(req, export)
	if err != nil {
		respondValidationError(w, "repository_export_apply", err.Error())
		return
	}
	respondOK(w, map[string]interface{}{"repository_export_apply": result})
}

func writeProjectRepositoryExport(req models.ProjectRepositoryExportApplyInput, export *models.ProjectRepositoryExport) (*models.ProjectRepositoryExportApplyResult, error) {
	if export == nil {
		return nil, errors.New("repository export is required")
	}
	root, err := resolveRepositoryRoot(req.RepositoryRoot)
	if err != nil {
		return nil, err
	}
	overwrite := true
	if req.Overwrite != nil {
		overwrite = *req.Overwrite
	}
	result := &models.ProjectRepositoryExportApplyResult{
		Project:        export.Project,
		RepositoryRoot: root,
		RepositoryDir:  export.RepositoryDir,
		Files:          make([]models.ProjectRepositoryExportApplyFile, 0, len(export.Files)),
		GeneratedAt:    export.GeneratedAt,
	}
	for _, file := range export.Files {
		item := models.ProjectRepositoryExportApplyFile{
			Path:   file.Path,
			Source: file.Source,
		}
		target, err := safeRepositoryTarget(root, file.Path)
		if err != nil {
			item.Status = "error"
			item.Message = err.Error()
			result.Files = append(result.Files, item)
			continue
		}
		item.TargetPath = target
		if info, err := os.Lstat(target); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				item.Status = "error"
				item.Message = "target path is a symlink"
				result.Files = append(result.Files, item)
				continue
			}
			if !overwrite {
				item.Status = "skipped"
				item.Message = "target exists"
				result.Files = append(result.Files, item)
				continue
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			item.Status = "error"
			item.Message = err.Error()
			result.Files = append(result.Files, item)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			item.Status = "error"
			item.Message = err.Error()
			result.Files = append(result.Files, item)
			continue
		}
		if err := verifyTargetDirWithinRoot(root, filepath.Dir(target)); err != nil {
			item.Status = "error"
			item.Message = err.Error()
			result.Files = append(result.Files, item)
			continue
		}
		data := []byte(file.Content)
		if err := os.WriteFile(target, data, 0o644); err != nil {
			item.Status = "error"
			item.Message = err.Error()
			result.Files = append(result.Files, item)
			continue
		}
		item.Status = "written"
		item.BytesWritten = len(data)
		result.Files = append(result.Files, item)
	}
	return result, nil
}

func resolveRepositoryRoot(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("repository_root is required")
	}
	if strings.HasPrefix(trimmed, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		trimmed = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(trimmed, "~"), string(filepath.Separator)))
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("repository_root must be an existing directory")
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func safeRepositoryTarget(root, rel string) (string, error) {
	clean := strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" {
		return "", errors.New("repository file path is required")
	}
	clean = filepath.Clean(filepath.FromSlash(clean))
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." || filepath.IsAbs(clean) {
		return "", errors.New("repository file path must stay inside the repository")
	}
	target := filepath.Join(root, clean)
	if err := verifyTargetDirWithinRoot(root, filepath.Dir(target)); err != nil {
		return "", err
	}
	return target, nil
}

func verifyTargetDirWithinRoot(root, dir string) error {
	root = filepath.Clean(root)
	dir = filepath.Clean(dir)
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("repository file path must stay inside the repository")
	}
	if rel == "." {
		return nil
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("repository file path uses a symlink directory")
		}
	}
	return nil
}
