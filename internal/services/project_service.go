package services

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/hubpath"
	"github.com/agi-bar/vola/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectService struct {
	db       *pgxpool.Pool
	repo     ProjectRepo
	role     *RoleService
	fileTree *FileTreeService
}

func NewProjectService(db *pgxpool.Pool, role *RoleService, fileTree *FileTreeService) *ProjectService {
	return &ProjectService{db: db, role: role, fileTree: fileTree}
}

func NewProjectServiceWithRepo(repo ProjectRepo, role *RoleService, fileTree *FileTreeService) *ProjectService {
	return &ProjectService{repo: repo, role: role, fileTree: fileTree}
}

func (s *ProjectService) List(ctx context.Context, userID uuid.UUID) ([]models.Project, error) {
	if s.repo != nil {
		return s.repo.ListProjects(ctx, userID)
	}
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, name, status, context_md, metadata, created_at, updated_at
		 FROM projects WHERE user_id = $1 AND status = 'active'
		 ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("project.List: %w", err)
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Status, &p.ContextMD, &p.Metadata, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("project.List: scan: %w", err)
		}
		if s.fileTree != nil {
			if entry, err := s.fileTree.Read(ctx, userID, hubpath.ProjectContextPath(p.Name), models.TrustLevelFull); err == nil {
				p.ContextMD = entry.Content
				if entry.UpdatedAt.After(p.UpdatedAt) {
					p.UpdatedAt = entry.UpdatedAt
				}
				p.Metadata = mergeMetadata(p.Metadata, entry.Metadata)
			}
		}
		p.Description = firstNonEmpty(metadataString(p.Metadata, "description"), firstMarkdownParagraph(p.ContextMD))
		p.PrimaryPath = hubpath.ProjectContextPath(p.Name)
		p.LogPath = hubpath.ProjectLogPath(p.Name)
		p.Capabilities = projectCapabilities()
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *ProjectService) Get(ctx context.Context, userID uuid.UUID, name string) (*models.Project, error) {
	if s.repo != nil {
		return s.repo.GetProject(ctx, userID, name)
	}
	var p models.Project
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, name, status, context_md, metadata, created_at, updated_at
		 FROM projects WHERE user_id = $1 AND name = $2`, userID, name).
		Scan(&p.ID, &p.UserID, &p.Name, &p.Status, &p.ContextMD, &p.Metadata, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("project.Get: %w", err)
	}
	if s.fileTree != nil {
		if entry, err := s.fileTree.Read(ctx, userID, hubpath.ProjectContextPath(name), models.TrustLevelFull); err == nil {
			p.ContextMD = entry.Content
			if entry.UpdatedAt.After(p.UpdatedAt) {
				p.UpdatedAt = entry.UpdatedAt
			}
			p.Metadata = mergeMetadata(p.Metadata, entry.Metadata)
		}
	}
	p.Description = firstNonEmpty(metadataString(p.Metadata, "description"), firstMarkdownParagraph(p.ContextMD))
	p.PrimaryPath = hubpath.ProjectContextPath(p.Name)
	p.LogPath = hubpath.ProjectLogPath(p.Name)
	p.Capabilities = projectCapabilities()
	return &p, nil
}

// Create creates a new project and a corresponding worker role scoped to it.
func (s *ProjectService) Create(ctx context.Context, userID uuid.UUID, name string) (*models.Project, error) {
	if err := validateSlug(name, 128); err != nil {
		return nil, fmt.Errorf("project.Create: invalid name: %w", err)
	}
	if s.repo != nil {
		return s.repo.CreateProject(ctx, userID, name)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("project.Create: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	id := uuid.New()
	now := time.Now().UTC()

	_, err = tx.Exec(ctx,
		`INSERT INTO projects (id, user_id, name, status, context_md, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, 'active', '', '{}', $4, $4)`,
		id, userID, name, now)
	if err != nil {
		return nil, fmt.Errorf("project.Create: insert: %w", err)
	}

	roleName := "worker-" + name
	projectPath := hubpath.ProjectDir(name)
	_, err = tx.Exec(ctx,
		`INSERT INTO roles (id, user_id, name, role_type, config, allowed_paths, allowed_vault_scopes, lifecycle, created_at)
		 VALUES ($1, $2, $3, 'worker', '{}', $4, '{}', 'project', $5)`,
		uuid.New(), userID, roleName, []string{projectPath}, now)
	if err != nil {
		return nil, fmt.Errorf("project.Create: create worker role: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("project.Create: commit: %w", err)
	}

	source := SourceOrDefault(ctx, "manual")
	if s.fileTree != nil {
		if _, err := s.fileTree.EnsureDirectoryWithMetadata(ctx, userID, projectPath, projectBundleDirectoryMetadata(name, "", "active", source), models.TrustLevelWork); err != nil {
			return nil, err
		}
		if _, err := s.fileTree.WriteEntry(ctx, userID, hubpath.ProjectContextPath(name), "", "text/markdown", models.FileTreeWriteOptions{
			Kind:          "project_context",
			MinTrustLevel: models.TrustLevelWork,
			Metadata: map[string]interface{}{
				"project": name,
				"status":  "active",
				"source":  source,
			},
		}); err != nil {
			return nil, err
		}
		if _, err := s.fileTree.WriteEntry(ctx, userID, hubpath.ProjectLogPath(name), "", "application/x-ndjson", models.FileTreeWriteOptions{
			Kind:          "project_log",
			MinTrustLevel: models.TrustLevelWork,
			Metadata: map[string]interface{}{
				"project": name,
				"source":  source,
			},
		}); err != nil {
			return nil, err
		}
	}

	p := &models.Project{
		ID:           id,
		UserID:       userID,
		Name:         name,
		Status:       "active",
		Description:  "",
		PrimaryPath:  hubpath.ProjectContextPath(name),
		LogPath:      hubpath.ProjectLogPath(name),
		Capabilities: projectCapabilities(),
		ContextMD:    "",
		Metadata:     projectBundleDirectoryMetadata(name, "", "active", source),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return p, nil
}

func (s *ProjectService) Archive(ctx context.Context, userID uuid.UUID, name string) error {
	if s.repo != nil {
		return s.repo.ArchiveProject(ctx, userID, name)
	}
	_, err := s.db.Exec(ctx,
		`UPDATE projects SET status = 'archived', updated_at = $1 WHERE user_id = $2 AND name = $3`,
		time.Now().UTC(), userID, name)
	if err != nil {
		return fmt.Errorf("project.Archive: %w", err)
	}
	if s.fileTree != nil {
		project, _ := s.Get(ctx, userID, name)
		contextMD := ""
		source := ""
		if project != nil {
			contextMD = project.ContextMD
			source = EntrySourceFromMetadata(project.Metadata)
		}
		_, _ = s.fileTree.EnsureDirectoryWithMetadata(ctx, userID, hubpath.ProjectDir(name), projectBundleDirectoryMetadata(name, contextMD, "archived", source), models.TrustLevelWork)
	}
	return nil
}

func (s *ProjectService) UpdateContext(ctx context.Context, userID uuid.UUID, name, contextMD string) error {
	if s.repo != nil {
		return s.repo.UpdateProjectContext(ctx, userID, name, contextMD)
	}
	if s.fileTree != nil {
		source := ""
		if project, err := s.Get(ctx, userID, name); err == nil {
			source = EntrySourceFromMetadata(project.Metadata)
		}
		if _, err := s.fileTree.WriteEntry(ctx, userID, hubpath.ProjectContextPath(name), contextMD, "text/markdown", models.FileTreeWriteOptions{
			Kind:          "project_context",
			MinTrustLevel: models.TrustLevelWork,
			Metadata: map[string]interface{}{
				"project":     name,
				"description": firstMarkdownParagraph(contextMD),
			},
		}); err != nil {
			return fmt.Errorf("project.UpdateContext: write canonical entry: %w", err)
		}
		if _, err := s.fileTree.EnsureDirectoryWithMetadata(ctx, userID, hubpath.ProjectDir(name), projectBundleDirectoryMetadata(name, contextMD, "", source), models.TrustLevelWork); err != nil {
			return fmt.Errorf("project.UpdateContext: ensure bundle dir: %w", err)
		}
	}
	_, err := s.db.Exec(ctx,
		`UPDATE projects SET context_md = $1, updated_at = $2 WHERE user_id = $3 AND name = $4`,
		contextMD, time.Now().UTC(), userID, name)
	if err != nil {
		return fmt.Errorf("project.UpdateContext: %w", err)
	}
	return nil
}

func (s *ProjectService) AppendLog(ctx context.Context, projectID uuid.UUID, log models.ProjectLog) error {
	projectName, userID, err := s.projectIdentity(ctx, projectID)
	if err != nil {
		return err
	}
	if s.repo != nil {
		return s.repo.AppendProjectLog(ctx, userID, projectName, log)
	}

	now := time.Now().UTC()
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	log.ProjectID = projectID
	log.CreatedAt = now
	if strings.TrimSpace(log.Source) == "" {
		log.Source = SourceOrDefault(ctx, "vola")
	}

	if s.fileTree != nil {
		path := hubpath.ProjectLogPath(projectName)
		current := ""
		if existing, err := s.fileTree.Read(ctx, userID, path, models.TrustLevelFull); err == nil {
			current = strings.TrimRight(existing.Content, "\n")
		}
		line, err := json.Marshal(log)
		if err != nil {
			return fmt.Errorf("project.AppendLog: marshal: %w", err)
		}
		nextContent := string(line)
		if current != "" {
			nextContent = current + "\n" + nextContent
		}
		if _, err := s.fileTree.WriteEntry(ctx, userID, path, nextContent, "application/x-ndjson", models.FileTreeWriteOptions{
			Kind:          "project_log",
			MinTrustLevel: models.TrustLevelWork,
			Metadata: map[string]interface{}{
				"project": projectName,
			},
		}); err != nil {
			return fmt.Errorf("project.AppendLog: write canonical entry: %w", err)
		}
		project, _ := s.Get(ctx, userID, projectName)
		contextMD := ""
		status := "active"
		source := log.Source
		if project != nil {
			contextMD = project.ContextMD
			if project.Status != "" {
				status = project.Status
			}
			if source == "" {
				source = EntrySourceFromMetadata(project.Metadata)
			}
		}
		if _, err := s.fileTree.EnsureDirectoryWithMetadata(ctx, userID, hubpath.ProjectDir(projectName), projectBundleDirectoryMetadata(projectName, contextMD, status, source), models.TrustLevelWork); err != nil {
			return fmt.Errorf("project.AppendLog: ensure bundle dir: %w", err)
		}
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO project_logs (id, project_id, source, role, action, summary, artifacts, tags, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		log.ID, log.ProjectID, log.Source, log.Role, log.Action, log.Summary, log.Artifacts, log.Tags, log.CreatedAt)
	if err != nil {
		return fmt.Errorf("project.AppendLog: %w", err)
	}

	_, err = s.db.Exec(ctx,
		`UPDATE projects SET updated_at = $1 WHERE id = $2`,
		now, projectID)
	if err != nil {
		return fmt.Errorf("project.AppendLog: update project timestamp: %w", err)
	}
	return nil
}

func (s *ProjectService) GetLogs(ctx context.Context, projectID uuid.UUID, limit int) ([]models.ProjectLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if s.repo != nil {
		projectName, userID, err := s.projectIdentity(ctx, projectID)
		if err != nil {
			return nil, err
		}
		return s.repo.GetProjectLogs(ctx, userID, projectName, limit)
	}

	if s.fileTree != nil {
		projectName, userID, err := s.projectIdentity(ctx, projectID)
		if err == nil {
			if entry, readErr := s.fileTree.Read(ctx, userID, hubpath.ProjectLogPath(projectName), models.TrustLevelFull); readErr == nil {
				logs := parseProjectLogs(entry.Content)
				if len(logs) > limit {
					logs = logs[len(logs)-limit:]
				}
				reverseProjectLogs(logs)
				return logs, nil
			}
		}
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, project_id, source, role, action, summary, artifacts, tags, created_at
		 FROM project_logs WHERE project_id = $1
		 ORDER BY created_at DESC LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("project.GetLogs: %w", err)
	}
	defer rows.Close()

	var logs []models.ProjectLog
	for rows.Next() {
		var l models.ProjectLog
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Source, &l.Role, &l.Action, &l.Summary, &l.Artifacts, &l.Tags, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("project.GetLogs: scan: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (s *ProjectService) SaveMaterial(ctx context.Context, userID uuid.UUID, projectName string, input models.ProjectMaterialInput) (*models.ProjectMaterial, error) {
	if s.fileTree == nil {
		return nil, fmt.Errorf("project.SaveMaterial: file tree service not configured")
	}
	if _, err := s.Get(ctx, userID, projectName); err != nil {
		return nil, fmt.Errorf("project.SaveMaterial: project not found: %w", err)
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = firstMarkdownHeading(input.Content)
	}
	if title == "" {
		title = "Project material"
	}
	if strings.TrimSpace(input.Content) == "" {
		return nil, fmt.Errorf("project.SaveMaterial: content is required")
	}
	slug := firstNonEmpty(input.Slug, title)
	entryPath := hubpath.ProjectMaterialPath(projectName, slug)
	now := time.Now().UTC()
	metadata := map[string]interface{}{
		"project":     projectName,
		"title":       title,
		"source":      SourceOrDefault(ctx, "manual"),
		"imported_at": now.Format(time.RFC3339),
	}
	if value := strings.TrimSpace(input.SourceURL); value != "" {
		metadata["source_url"] = value
	}
	if value := hubpath.NormalizePublic(input.SourcePath); value != "/" {
		metadata["source_path"] = value
	}
	if value := strings.TrimSpace(input.SourceType); value != "" {
		metadata["source_type"] = value
	}
	if value := strings.TrimSpace(input.Description); value != "" {
		metadata["description"] = value
	}
	if value := strings.TrimSpace(input.SourceUpdatedAt); value != "" {
		metadata["source_updated_at"] = value
	}
	if value := normalizeRepositoryPath(input.RepositoryPath); value != "" {
		metadata["repository_path"] = value
	}
	if tags := cleanStringSlice(input.Tags); len(tags) > 0 {
		metadata["tags"] = tags
	}
	entry, err := s.fileTree.WriteEntry(ctx, userID, entryPath, input.Content, "text/markdown", models.FileTreeWriteOptions{
		Kind:          "project_material",
		MinTrustLevel: models.TrustLevelWork,
		Metadata:      metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("project.SaveMaterial: write material: %w", err)
	}
	return projectMaterialFromEntry(projectName, *entry), nil
}

func (s *ProjectService) CopyMaterial(ctx context.Context, userID uuid.UUID, projectName string, input models.ProjectMaterialCopyInput) (*models.ProjectMaterial, error) {
	if s.fileTree == nil {
		return nil, fmt.Errorf("project.CopyMaterial: file tree service not configured")
	}
	sourcePath := strings.TrimSpace(input.SourcePath)
	if sourcePath == "" {
		return nil, fmt.Errorf("project.CopyMaterial: source_path is required")
	}
	entry, err := s.fileTree.Read(ctx, userID, sourcePath, models.TrustLevelFull)
	if err != nil {
		return nil, fmt.Errorf("project.CopyMaterial: read source: %w", err)
	}
	if entry.IsDirectory {
		return nil, fmt.Errorf("project.CopyMaterial: source_path must be a Markdown file")
	}
	contentType := strings.ToLower(strings.TrimSpace(entry.ContentType))
	publicSourcePath := hubpath.StorageToPublic(entry.Path)
	if !strings.HasSuffix(strings.ToLower(publicSourcePath), ".md") && !strings.Contains(contentType, "markdown") {
		return nil, fmt.Errorf("project.CopyMaterial: source_path must be a Markdown file")
	}
	title := firstNonEmpty(input.Title, metadataString(entry.Metadata, "title"), firstMarkdownHeading(entry.Content), strings.TrimSuffix(path.Base(publicSourcePath), ".md"))
	return s.SaveMaterial(ctx, userID, projectName, models.ProjectMaterialInput{
		Title:           title,
		Slug:            firstNonEmpty(input.Slug, strings.TrimSuffix(path.Base(publicSourcePath), ".md")),
		Content:         entry.Content,
		SourcePath:      publicSourcePath,
		SourceURL:       input.SourceURL,
		SourceType:      "vola-file",
		Description:     input.Description,
		Tags:            input.Tags,
		SourceUpdatedAt: firstNonEmpty(input.SourceUpdatedAt, entry.UpdatedAt.UTC().Format(time.RFC3339)),
		RepositoryPath:  input.RepositoryPath,
	})
}

func (s *ProjectService) ListMaterials(ctx context.Context, userID uuid.UUID, projectName string) ([]models.ProjectMaterial, error) {
	if s.fileTree == nil {
		return nil, fmt.Errorf("project.ListMaterials: file tree service not configured")
	}
	if _, err := s.Get(ctx, userID, projectName); err != nil {
		return nil, fmt.Errorf("project.ListMaterials: project not found: %w", err)
	}
	entries, err := s.fileTree.List(ctx, userID, hubpath.ProjectMaterialsDir(projectName), models.TrustLevelFull)
	if err != nil {
		if err == ErrEntryNotFound {
			return []models.ProjectMaterial{}, nil
		}
		return nil, fmt.Errorf("project.ListMaterials: %w", err)
	}
	materials := make([]models.ProjectMaterial, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDirectory || !strings.HasSuffix(hubpath.NormalizePublic(entry.Path), ".md") {
			continue
		}
		materials = append(materials, *projectMaterialFromEntry(projectName, entry))
	}
	return materials, nil
}

func (s *ProjectService) BuildContextPack(ctx context.Context, userID uuid.UUID, projectName string, input models.ProjectContextPackInput) (*models.ProjectContextPack, error) {
	if s.fileTree == nil {
		return nil, fmt.Errorf("project.BuildContextPack: file tree service not configured")
	}
	project, err := s.Get(ctx, userID, projectName)
	if err != nil {
		return nil, fmt.Errorf("project.BuildContextPack: project not found: %w", err)
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "AI context pack"
	}
	slug := firstNonEmpty(input.Slug, title)
	includeContext := boolDefault(input.IncludeContext, true)
	includeRecentLogs := boolDefault(input.IncludeRecentLogs, true)
	limit := input.RecentLogLimit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	materials, err := s.contextPackMaterials(ctx, userID, projectName, input.MaterialPaths)
	if err != nil {
		return nil, err
	}
	logs := []models.ProjectLog{}
	if includeRecentLogs {
		if items, logErr := s.GetLogs(ctx, project.ID, limit); logErr == nil {
			logs = items
		}
	}

	content := renderProjectContextPack(project, title, strings.TrimSpace(input.Purpose), includeContext, logs, materials)
	entryPath := hubpath.ProjectContextPackPath(projectName, slug)
	repositoryPath := contextPackRepositoryPath(input.RepositoryDir, input.RepositoryFilename, path.Base(entryPath))
	metadata := map[string]interface{}{
		"project":                 projectName,
		"title":                   title,
		"purpose":                 strings.TrimSpace(input.Purpose),
		"source":                  SourceOrDefault(ctx, "manual"),
		"material_paths":          materialPaths(materials),
		"include_context":         includeContext,
		"include_recent_logs":     includeRecentLogs,
		"recent_log_limit":        limit,
		"repository_path":         repositoryPath,
		"generated_at":            time.Now().UTC().Format(time.RFC3339),
		"repository_export_ready": true,
	}
	entry, err := s.fileTree.WriteEntry(ctx, userID, entryPath, content, "text/markdown", models.FileTreeWriteOptions{
		Kind:          "project_context_pack",
		MinTrustLevel: models.TrustLevelWork,
		Metadata:      metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("project.BuildContextPack: write pack: %w", err)
	}
	return projectContextPackFromEntry(projectName, *entry), nil
}

func (s *ProjectService) ListContextPacks(ctx context.Context, userID uuid.UUID, projectName string) ([]models.ProjectContextPack, error) {
	if s.fileTree == nil {
		return nil, fmt.Errorf("project.ListContextPacks: file tree service not configured")
	}
	if _, err := s.Get(ctx, userID, projectName); err != nil {
		return nil, fmt.Errorf("project.ListContextPacks: project not found: %w", err)
	}
	entries, err := s.fileTree.List(ctx, userID, hubpath.ProjectContextPacksDir(projectName), models.TrustLevelFull)
	if err != nil {
		if err == ErrEntryNotFound {
			return []models.ProjectContextPack{}, nil
		}
		return nil, fmt.Errorf("project.ListContextPacks: %w", err)
	}
	packs := make([]models.ProjectContextPack, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDirectory || !strings.HasSuffix(hubpath.NormalizePublic(entry.Path), ".md") {
			continue
		}
		packs = append(packs, *projectContextPackFromEntry(projectName, entry))
	}
	return packs, nil
}

func (s *ProjectService) ReadContextPack(ctx context.Context, userID uuid.UUID, projectName, pack string) (*models.ProjectContextPack, error) {
	if s.fileTree == nil {
		return nil, fmt.Errorf("project.ReadContextPack: file tree service not configured")
	}
	if _, err := s.Get(ctx, userID, projectName); err != nil {
		return nil, fmt.Errorf("project.ReadContextPack: project not found: %w", err)
	}
	entryPath := pack
	if !strings.HasPrefix(strings.TrimSpace(pack), "/") {
		entryPath = hubpath.ProjectContextPackPath(projectName, strings.TrimSuffix(pack, ".md"))
	}
	entry, err := s.fileTree.Read(ctx, userID, entryPath, models.TrustLevelFull)
	if err != nil {
		return nil, fmt.Errorf("project.ReadContextPack: %w", err)
	}
	return projectContextPackFromEntry(projectName, *entry), nil
}

func (s *ProjectService) BuildRepositoryExport(ctx context.Context, userID uuid.UUID, projectName string, input models.ProjectRepositoryExportInput) (*models.ProjectRepositoryExport, error) {
	if s.fileTree == nil {
		return nil, fmt.Errorf("project.BuildRepositoryExport: file tree service not configured")
	}
	if _, err := s.Get(ctx, userID, projectName); err != nil {
		return nil, fmt.Errorf("project.BuildRepositoryExport: project not found: %w", err)
	}
	repoDir := normalizeRepositoryDir(input.RepositoryDir)
	if repoDir == "" {
		repoDir = "docs/ai-context"
	}
	includeIndex := boolDefault(input.IncludeIndex, true)
	materials, err := s.contextPackMaterials(ctx, userID, projectName, input.MaterialPaths)
	if err != nil {
		return nil, err
	}
	packs, err := s.repositoryExportPacks(ctx, userID, projectName, input.PackPaths)
	if err != nil {
		return nil, err
	}
	files := make([]models.ProjectRepositoryExportFile, 0, len(materials)+len(packs)+1)
	if includeIndex {
		files = append(files, models.ProjectRepositoryExportFile{
			Path:    path.Join(repoDir, "README.md"),
			Content: renderRepositoryExportIndex(projectName, repoDir, materials, packs),
			Source:  "vola",
		})
	}
	for _, material := range materials {
		target := material.RepositoryPath
		if target == "" {
			target = path.Join(repoDir, "materials", path.Base(material.Path))
		}
		files = append(files, models.ProjectRepositoryExportFile{
			Path:    target,
			Content: material.Content,
			Source:  material.Path,
		})
	}
	for _, pack := range packs {
		target := pack.RepositoryPath
		if target == "" {
			target = path.Join(repoDir, "context-packs", path.Base(pack.Path))
		}
		files = append(files, models.ProjectRepositoryExportFile{
			Path:    target,
			Content: pack.Content,
			Source:  pack.Path,
		})
	}
	return &models.ProjectRepositoryExport{
		Project:       projectName,
		RepositoryDir: repoDir,
		Files:         files,
		GeneratedAt:   time.Now().UTC(),
	}, nil
}

func (s *ProjectService) projectIdentity(ctx context.Context, projectID uuid.UUID) (string, uuid.UUID, error) {
	if s.repo != nil {
		return s.repo.GetProjectIdentity(ctx, projectID)
	}
	var name string
	var userID uuid.UUID
	err := s.db.QueryRow(ctx,
		`SELECT name, user_id FROM projects WHERE id = $1`,
		projectID,
	).Scan(&name, &userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", uuid.Nil, fmt.Errorf("project.projectIdentity: project not found")
		}
		return "", uuid.Nil, fmt.Errorf("project.projectIdentity: %w", err)
	}
	return name, userID, nil
}

func parseProjectLogs(content string) []models.ProjectLog {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	logs := make([]models.ProjectLog, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var log models.ProjectLog
		if err := json.Unmarshal([]byte(line), &log); err == nil {
			logs = append(logs, log)
		}
	}
	return logs
}

func reverseProjectLogs(logs []models.ProjectLog) {
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}
}

func projectCapabilities() []string {
	return []string{"context", "logs", "materials", "context-packs", "repository-export"}
}

func projectMaterialFromEntry(projectName string, entry models.FileTreeEntry) *models.ProjectMaterial {
	publicPath := hubpath.StorageToPublic(entry.Path)
	title := firstNonEmpty(metadataString(entry.Metadata, "title"), firstMarkdownHeading(entry.Content), strings.TrimSuffix(path.Base(publicPath), ".md"))
	return &models.ProjectMaterial{
		Project:         firstNonEmpty(metadataString(entry.Metadata, "project"), projectName),
		Title:           title,
		Slug:            strings.TrimSuffix(path.Base(publicPath), ".md"),
		Path:            publicPath,
		Content:         entry.Content,
		SourcePath:      metadataString(entry.Metadata, "source_path"),
		SourceURL:       metadataString(entry.Metadata, "source_url"),
		SourceType:      metadataString(entry.Metadata, "source_type"),
		Description:     firstNonEmpty(metadataString(entry.Metadata, "description"), firstMarkdownParagraph(entry.Content)),
		Tags:            cleanStringSlice(toStringSlice(entry.Metadata["tags"])),
		SourceUpdatedAt: metadataString(entry.Metadata, "source_updated_at"),
		RepositoryPath:  metadataString(entry.Metadata, "repository_path"),
		Metadata:        entry.Metadata,
		CreatedAt:       entry.CreatedAt,
		UpdatedAt:       entry.UpdatedAt,
	}
}

func projectContextPackFromEntry(projectName string, entry models.FileTreeEntry) *models.ProjectContextPack {
	publicPath := hubpath.StorageToPublic(entry.Path)
	title := firstNonEmpty(metadataString(entry.Metadata, "title"), firstMarkdownHeading(entry.Content), strings.TrimSuffix(path.Base(publicPath), ".md"))
	return &models.ProjectContextPack{
		Project:        firstNonEmpty(metadataString(entry.Metadata, "project"), projectName),
		Title:          title,
		Slug:           strings.TrimSuffix(path.Base(publicPath), ".md"),
		Path:           publicPath,
		Content:        entry.Content,
		Purpose:        metadataString(entry.Metadata, "purpose"),
		MaterialPaths:  cleanStringSlice(toStringSlice(entry.Metadata["material_paths"])),
		RepositoryPath: metadataString(entry.Metadata, "repository_path"),
		Metadata:       entry.Metadata,
		CreatedAt:      entry.CreatedAt,
		UpdatedAt:      entry.UpdatedAt,
	}
}

func (s *ProjectService) contextPackMaterials(ctx context.Context, userID uuid.UUID, projectName string, requested []string) ([]models.ProjectMaterial, error) {
	if len(cleanStringSlice(requested)) == 0 {
		return s.ListMaterials(ctx, userID, projectName)
	}
	materials := make([]models.ProjectMaterial, 0, len(requested))
	for _, rawPath := range cleanStringSlice(requested) {
		entryPath := rawPath
		if !strings.HasPrefix(entryPath, "/") {
			entryPath = hubpath.ProjectMaterialPath(projectName, strings.TrimSuffix(entryPath, ".md"))
		}
		entry, err := s.fileTree.Read(ctx, userID, entryPath, models.TrustLevelFull)
		if err != nil {
			return nil, fmt.Errorf("project.contextPackMaterials: read %s: %w", rawPath, err)
		}
		materials = append(materials, *projectMaterialFromEntry(projectName, *entry))
	}
	return materials, nil
}

func (s *ProjectService) repositoryExportPacks(ctx context.Context, userID uuid.UUID, projectName string, requested []string) ([]models.ProjectContextPack, error) {
	if len(cleanStringSlice(requested)) == 0 {
		return s.ListContextPacks(ctx, userID, projectName)
	}
	packs := make([]models.ProjectContextPack, 0, len(requested))
	for _, rawPath := range cleanStringSlice(requested) {
		pack, err := s.ReadContextPack(ctx, userID, projectName, rawPath)
		if err != nil {
			return nil, err
		}
		packs = append(packs, *pack)
	}
	return packs, nil
}

func materialPaths(materials []models.ProjectMaterial) []string {
	out := make([]string, 0, len(materials))
	for _, material := range materials {
		if strings.TrimSpace(material.Path) != "" {
			out = append(out, material.Path)
		}
	}
	sort.Strings(out)
	return out
}

func renderProjectContextPack(project *models.Project, title, purpose string, includeContext bool, logs []models.ProjectLog, materials []models.ProjectMaterial) string {
	var b strings.Builder
	b.WriteString("# " + title + "\n\n")
	b.WriteString("- Project: `" + project.Name + "`\n")
	b.WriteString("- Generated at: " + time.Now().UTC().Format(time.RFC3339) + "\n")
	if purpose != "" {
		b.WriteString("- Purpose: " + purpose + "\n")
	}
	if len(materials) > 0 {
		b.WriteString("- Source materials:\n")
		for _, material := range materials {
			b.WriteString("  - `" + material.Path + "`")
			if material.SourceURL != "" {
				b.WriteString(" (" + material.SourceURL + ")")
			}
			b.WriteString("\n")
		}
	}
	if includeContext {
		b.WriteString("\n## Project Context\n\n")
		if strings.TrimSpace(project.ContextMD) == "" {
			b.WriteString("_No project context has been written yet._\n")
		} else {
			b.WriteString(strings.TrimSpace(project.ContextMD) + "\n")
		}
	}
	if len(logs) > 0 {
		b.WriteString("\n## Recent Logs\n\n")
		for _, log := range logs {
			when := ""
			if !log.CreatedAt.IsZero() {
				when = " · " + log.CreatedAt.UTC().Format(time.RFC3339)
			}
			action := firstNonEmpty(log.Action, "log")
			b.WriteString("- " + action + when + ": " + strings.TrimSpace(log.Summary) + "\n")
		}
	}
	if len(materials) > 0 {
		b.WriteString("\n## Materials\n")
		for _, material := range materials {
			b.WriteString("\n### " + material.Title + "\n\n")
			b.WriteString("- Vola path: `" + material.Path + "`\n")
			if material.SourceURL != "" {
				b.WriteString("- Source URL: " + material.SourceURL + "\n")
			}
			if material.SourceUpdatedAt != "" {
				b.WriteString("- Source updated at: " + material.SourceUpdatedAt + "\n")
			}
			if material.RepositoryPath != "" {
				b.WriteString("- Repository path: `" + material.RepositoryPath + "`\n")
			}
			b.WriteString("\n")
			b.WriteString(strings.TrimSpace(material.Content) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func renderRepositoryExportIndex(projectName, repoDir string, materials []models.ProjectMaterial, packs []models.ProjectContextPack) string {
	var b strings.Builder
	b.WriteString("# AI Context: " + projectName + "\n\n")
	b.WriteString("This directory contains Vola-managed project context for AI-assisted collaboration.\n\n")
	b.WriteString("- Source project: `" + projectName + "`\n")
	b.WriteString("- Default directory: `" + repoDir + "`\n")
	b.WriteString("- Generated at: " + time.Now().UTC().Format(time.RFC3339) + "\n\n")
	if len(packs) > 0 {
		b.WriteString("## Context Packs\n\n")
		for _, pack := range packs {
			target := firstNonEmpty(pack.RepositoryPath, path.Join(repoDir, "context-packs", path.Base(pack.Path)))
			b.WriteString("- [" + pack.Title + "](" + strings.TrimPrefix(target, repoDir+"/") + ")")
			if pack.Purpose != "" {
				b.WriteString(" - " + pack.Purpose)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if len(materials) > 0 {
		b.WriteString("## Materials\n\n")
		for _, material := range materials {
			target := firstNonEmpty(material.RepositoryPath, path.Join(repoDir, "materials", path.Base(material.Path)))
			b.WriteString("- [" + material.Title + "](" + strings.TrimPrefix(target, repoDir+"/") + ")")
			if material.SourceURL != "" {
				b.WriteString(" - " + material.SourceURL)
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func cleanStringSlice(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func normalizeRepositoryPath(raw string) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	cleaned = path.Clean(cleaned)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return ""
	}
	return cleaned
}

func normalizeRepositoryDir(raw string) string {
	cleaned := normalizeRepositoryPath(raw)
	if cleaned == "" {
		return ""
	}
	return strings.TrimSuffix(cleaned, "/")
}

func contextPackRepositoryPath(repoDir, filename, fallback string) string {
	dir := normalizeRepositoryDir(repoDir)
	if dir == "" {
		dir = "docs/ai-context/context-packs"
	}
	name := strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/"))
	if name == "" {
		name = fallback
	}
	name = path.Base(name)
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}
	return path.Join(dir, name)
}

func projectBundleDirectoryMetadata(name, contextMD, status, source string) map[string]interface{} {
	summary := models.BundleSummary{
		Kind:         BundleKindProject,
		Name:         name,
		Path:         hubpath.ProjectDir(name),
		Source:       strings.TrimSpace(source),
		Description:  firstMarkdownParagraph(contextMD),
		Status:       firstNonEmpty(status, "active"),
		PrimaryPath:  hubpath.ProjectContextPath(name),
		LogPath:      hubpath.ProjectLogPath(name),
		Capabilities: projectCapabilities(),
	}
	metadata := BundleMetadata(summary)
	metadata["project"] = name
	return metadata
}
