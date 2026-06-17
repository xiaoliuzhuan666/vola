package models

import (
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID           uuid.UUID              `json:"id" db:"id"`
	UserID       uuid.UUID              `json:"user_id" db:"user_id"`
	Name         string                 `json:"name" db:"name"`
	Status       string                 `json:"status" db:"status"` // active, archived
	Description  string                 `json:"description,omitempty"`
	PrimaryPath  string                 `json:"primary_path,omitempty"`
	LogPath      string                 `json:"log_path,omitempty"`
	Capabilities []string               `json:"capabilities,omitempty"`
	ContextMD    string                 `json:"context_md" db:"context_md"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at" db:"updated_at"`
}

type ProjectLog struct {
	ID        uuid.UUID `json:"id" db:"id"`
	ProjectID uuid.UUID `json:"project_id" db:"project_id"`
	Source    string    `json:"source" db:"source"`
	Role      string    `json:"role" db:"role"`
	Action    string    `json:"action" db:"action"`
	Summary   string    `json:"summary" db:"summary"`
	Artifacts []string  `json:"artifacts" db:"artifacts"`
	Tags      []string  `json:"tags" db:"tags"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type ProjectMaterialInput struct {
	Title           string   `json:"title"`
	Slug            string   `json:"slug,omitempty"`
	Content         string   `json:"content"`
	SourcePath      string   `json:"source_path,omitempty"`
	SourceURL       string   `json:"source_url,omitempty"`
	SourceType      string   `json:"source_type,omitempty"`
	Description     string   `json:"description,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	SourceUpdatedAt string   `json:"source_updated_at,omitempty"`
	RepositoryPath  string   `json:"repository_path,omitempty"`
}

type ProjectMaterial struct {
	Project         string                 `json:"project"`
	Title           string                 `json:"title"`
	Slug            string                 `json:"slug"`
	Path            string                 `json:"path"`
	Content         string                 `json:"content,omitempty"`
	SourcePath      string                 `json:"source_path,omitempty"`
	SourceURL       string                 `json:"source_url,omitempty"`
	SourceType      string                 `json:"source_type,omitempty"`
	Description     string                 `json:"description,omitempty"`
	Tags            []string               `json:"tags,omitempty"`
	SourceUpdatedAt string                 `json:"source_updated_at,omitempty"`
	RepositoryPath  string                 `json:"repository_path,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type ProjectMaterialCopyInput struct {
	SourcePath      string   `json:"source_path"`
	Title           string   `json:"title,omitempty"`
	Slug            string   `json:"slug,omitempty"`
	SourceURL       string   `json:"source_url,omitempty"`
	Description     string   `json:"description,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	RepositoryPath  string   `json:"repository_path,omitempty"`
	SourceUpdatedAt string   `json:"source_updated_at,omitempty"`
}

type ProjectContextPackInput struct {
	Title              string   `json:"title"`
	Slug               string   `json:"slug,omitempty"`
	Purpose            string   `json:"purpose,omitempty"`
	MaterialPaths      []string `json:"material_paths,omitempty"`
	IncludeContext     *bool    `json:"include_context,omitempty"`
	IncludeRecentLogs  *bool    `json:"include_recent_logs,omitempty"`
	RecentLogLimit     int      `json:"recent_log_limit,omitempty"`
	RepositoryDir      string   `json:"repository_dir,omitempty"`
	RepositoryFilename string   `json:"repository_filename,omitempty"`
}

type ProjectContextPack struct {
	Project        string                 `json:"project"`
	Title          string                 `json:"title"`
	Slug           string                 `json:"slug"`
	Path           string                 `json:"path"`
	Content        string                 `json:"content,omitempty"`
	Purpose        string                 `json:"purpose,omitempty"`
	MaterialPaths  []string               `json:"material_paths,omitempty"`
	RepositoryPath string                 `json:"repository_path,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type ProjectRepositoryExportInput struct {
	RepositoryDir string   `json:"repository_dir,omitempty"`
	MaterialPaths []string `json:"material_paths,omitempty"`
	PackPaths     []string `json:"pack_paths,omitempty"`
	IncludeIndex  *bool    `json:"include_index,omitempty"`
}

type ProjectRepositoryExportApplyInput struct {
	RepositoryRoot string   `json:"repository_root"`
	RepositoryDir  string   `json:"repository_dir,omitempty"`
	MaterialPaths  []string `json:"material_paths,omitempty"`
	PackPaths      []string `json:"pack_paths,omitempty"`
	IncludeIndex   *bool    `json:"include_index,omitempty"`
	Overwrite      *bool    `json:"overwrite,omitempty"`
}

type ProjectRepositoryExportFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Source  string `json:"source,omitempty"`
}

type ProjectRepositoryExportApplyFile struct {
	Path         string `json:"path"`
	TargetPath   string `json:"target_path"`
	Source       string `json:"source,omitempty"`
	Status       string `json:"status"`
	BytesWritten int    `json:"bytes_written,omitempty"`
	Message      string `json:"message,omitempty"`
}

type ProjectRepositoryExport struct {
	Project       string                        `json:"project"`
	RepositoryDir string                        `json:"repository_dir"`
	Files         []ProjectRepositoryExportFile `json:"files"`
	GeneratedAt   time.Time                     `json:"generated_at"`
}

type ProjectRepositoryExportApplyResult struct {
	Project        string                             `json:"project"`
	RepositoryRoot string                             `json:"repository_root"`
	RepositoryDir  string                             `json:"repository_dir"`
	Files          []ProjectRepositoryExportApplyFile `json:"files"`
	GeneratedAt    time.Time                          `json:"generated_at"`
}
