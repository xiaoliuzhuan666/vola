package backups

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	KindWebDAV = "webdav"
	KindS3     = "s3"
)

type Target struct {
	ID                      uuid.UUID  `json:"id"`
	UserID                  uuid.UUID  `json:"user_id,omitempty"`
	Kind                    string     `json:"kind"`
	Name                    string     `json:"name"`
	Enabled                 bool       `json:"enabled"`
	WebDAVURL               string     `json:"webdav_url,omitempty"`
	WebDAVUsername          string     `json:"webdav_username,omitempty"`
	S3Endpoint              string     `json:"s3_endpoint,omitempty"`
	S3Bucket                string     `json:"s3_bucket,omitempty"`
	S3Region                string     `json:"s3_region,omitempty"`
	S3Prefix                string     `json:"s3_prefix,omitempty"`
	S3AccessKeyID           string     `json:"s3_access_key_id,omitempty"`
	S3PathStyle             bool       `json:"s3_path_style"`
	SecretConfigured        bool       `json:"secret_configured"`
	AutoBackupEnabled       bool       `json:"auto_backup_enabled"`
	AutoBackupIntervalHours int        `json:"auto_backup_interval_hours"`
	RetentionKeepLast       int        `json:"retention_keep_last"`
	RetentionKeepDays       int        `json:"retention_keep_days"`
	LastAutoBackupAt        *time.Time `json:"last_auto_backup_at,omitempty"`
	LastBackupAt            *time.Time `json:"last_backup_at,omitempty"`
	LastBackupObject        string     `json:"last_backup_object,omitempty"`
	LastBackupError         string     `json:"last_backup_error,omitempty"`
	CreatedAt               time.Time  `json:"created_at,omitempty"`
	UpdatedAt               time.Time  `json:"updated_at,omitempty"`
}

type Config struct {
	WebDAVURL      string `json:"webdav_url,omitempty"`
	WebDAVUsername string `json:"webdav_username,omitempty"`
	S3Endpoint     string `json:"s3_endpoint,omitempty"`
	S3Bucket       string `json:"s3_bucket,omitempty"`
	S3Region       string `json:"s3_region,omitempty"`
	S3Prefix       string `json:"s3_prefix,omitempty"`
	S3AccessKeyID  string `json:"s3_access_key_id,omitempty"`
	S3PathStyle    bool   `json:"s3_path_style"`
}

func ConfigFromTarget(target Target) Config {
	return Config{
		WebDAVURL:      target.WebDAVURL,
		WebDAVUsername: target.WebDAVUsername,
		S3Endpoint:     target.S3Endpoint,
		S3Bucket:       target.S3Bucket,
		S3Region:       target.S3Region,
		S3Prefix:       target.S3Prefix,
		S3AccessKeyID:  target.S3AccessKeyID,
		S3PathStyle:    target.S3PathStyle,
	}
}

func ApplyConfig(target *Target, cfg Config) {
	if target == nil {
		return
	}
	target.WebDAVURL = cfg.WebDAVURL
	target.WebDAVUsername = cfg.WebDAVUsername
	target.S3Endpoint = cfg.S3Endpoint
	target.S3Bucket = cfg.S3Bucket
	target.S3Region = cfg.S3Region
	target.S3Prefix = cfg.S3Prefix
	target.S3AccessKeyID = cfg.S3AccessKeyID
	target.S3PathStyle = cfg.S3PathStyle
}

type targetSecret struct {
	WebDAVPassword    string `json:"webdav_password,omitempty"`
	S3SecretAccessKey string `json:"s3_secret_access_key,omitempty"`
}

type SaveTargetRequest struct {
	ID                      string `json:"id,omitempty"`
	Kind                    string `json:"kind"`
	Name                    string `json:"name"`
	Enabled                 bool   `json:"enabled"`
	WebDAVURL               string `json:"webdav_url,omitempty"`
	WebDAVUsername          string `json:"webdav_username,omitempty"`
	WebDAVPassword          string `json:"webdav_password,omitempty"`
	S3Endpoint              string `json:"s3_endpoint,omitempty"`
	S3Bucket                string `json:"s3_bucket,omitempty"`
	S3Region                string `json:"s3_region,omitempty"`
	S3Prefix                string `json:"s3_prefix,omitempty"`
	S3AccessKeyID           string `json:"s3_access_key_id,omitempty"`
	S3SecretAccessKey       string `json:"s3_secret_access_key,omitempty"`
	S3PathStyle             bool   `json:"s3_path_style"`
	AutoBackupEnabled       bool   `json:"auto_backup_enabled"`
	AutoBackupIntervalHours int    `json:"auto_backup_interval_hours"`
	RetentionKeepLast       int    `json:"retention_keep_last"`
	RetentionKeepDays       int    `json:"retention_keep_days"`
}

type RunResult struct {
	Target      Target `json:"target"`
	Run         Run    `json:"run"`
	ObjectName  string `json:"object_name"`
	Location    string `json:"location"`
	SizeBytes   int64  `json:"size_bytes"`
	CompletedAt string `json:"completed_at"`
	Message     string `json:"message,omitempty"`
}

type AutomationResult struct {
	Checked   int      `json:"checked"`
	Due       int      `json:"due"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Skipped   int      `json:"skipped"`
	Errors    []string `json:"errors,omitempty"`
}

const (
	RunTriggerManual = "manual"
	RunTriggerAuto   = "auto"

	RunStatusSuccess = "success"
	RunStatusFailed  = "failed"
)

type Run struct {
	ID                uuid.UUID  `json:"id"`
	UserID            uuid.UUID  `json:"user_id,omitempty"`
	TargetID          uuid.UUID  `json:"target_id"`
	TargetName        string     `json:"target_name"`
	TargetKind        string     `json:"target_kind"`
	Trigger           string     `json:"trigger"`
	Status            string     `json:"status"`
	ObjectName        string     `json:"object_name,omitempty"`
	Location          string     `json:"location,omitempty"`
	SizeBytes         int64      `json:"size_bytes"`
	StartedAt         time.Time  `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	DurationMs        int64      `json:"duration_ms"`
	Error             string     `json:"error,omitempty"`
	RemoteDeletedAt   *time.Time `json:"remote_deleted_at,omitempty"`
	RemoteDeleteError string     `json:"remote_delete_error,omitempty"`
	CreatedAt         time.Time  `json:"created_at,omitempty"`
}

type Repo interface {
	ListBackupTargets(ctx context.Context, userID uuid.UUID) ([]Target, error)
	ListAutoBackupTargets(ctx context.Context) ([]Target, error)
	GetBackupTarget(ctx context.Context, userID, targetID uuid.UUID) (*Target, error)
	UpsertBackupTarget(ctx context.Context, target Target) error
	UpdateBackupTargetResult(ctx context.Context, userID, targetID uuid.UUID, backupAt *time.Time, objectName, lastError string, auto bool) error
	InsertBackupRun(ctx context.Context, run Run) error
	ListBackupRuns(ctx context.Context, userID uuid.UUID, targetID *uuid.UUID, limit int) ([]Run, error)
	ListPrunableBackupRuns(ctx context.Context, userID, targetID uuid.UUID, keepLast, keepDays int, now time.Time) ([]Run, error)
	UpdateBackupRunRemoteDelete(ctx context.Context, userID, runID uuid.UUID, deletedAt *time.Time, deleteError string) error
}
