package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/agi-bar/neudrive/internal/localgitsync"
	"github.com/agi-bar/neudrive/internal/models"
	"github.com/google/uuid"
)

const (
	opsStatusOK       = "ok"
	opsStatusWarning  = "warning"
	opsStatusCritical = "critical"
)

type opsStatusResponse struct {
	Status      string             `json:"status"`
	GeneratedAt string             `json:"generated_at"`
	Storage     string             `json:"storage,omitempty"`
	LocalMode   bool               `json:"local_mode"`
	PublicURL   string             `json:"public_url,omitempty"`
	GitMirror   opsGitMirrorStatus `json:"git_mirror"`
	Backup      opsBackupStatus    `json:"backup"`
	Checks      []opsCheck         `json:"checks"`
	Docs        []opsDocRef        `json:"docs"`
}

type opsCheck struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Action  string `json:"action,omitempty"`
}

type opsDocRef struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

type opsGitMirrorStatus struct {
	ServiceConfigured  bool   `json:"service_configured"`
	ExecutionMode      string `json:"execution_mode,omitempty"`
	HostedRoot         string `json:"hosted_root,omitempty"`
	HostedRootSet      bool   `json:"hosted_root_set"`
	RemoteURL          string `json:"remote_url,omitempty"`
	AutoPushEnabled    bool   `json:"auto_push_enabled"`
	SyncState          string `json:"sync_state,omitempty"`
	LastSyncedAt       string `json:"last_synced_at,omitempty"`
	LastPushAt         string `json:"last_push_at,omitempty"`
	LastError          string `json:"last_error,omitempty"`
	LastPushError      string `json:"last_push_error,omitempty"`
	RemoteConflict     bool   `json:"remote_conflict,omitempty"`
	GitHubAppConnected bool   `json:"github_app_connected"`
	GitHubTokenSet     bool   `json:"github_token_set"`
}

type opsBackupStatus struct {
	ServiceConfigured      bool                   `json:"service_configured"`
	TargetsConfigured      int                    `json:"targets_configured"`
	EnabledTargets         int                    `json:"enabled_targets"`
	TargetsWithSecrets     int                    `json:"targets_with_secrets"`
	TargetsWithLastBackup  int                    `json:"targets_with_last_backup"`
	HistoryCount           int                    `json:"history_count"`
	LastSuccessfulBackupAt string                 `json:"last_successful_backup_at,omitempty"`
	LastBackupObject       string                 `json:"last_backup_object,omitempty"`
	LastRunStatus          string                 `json:"last_run_status,omitempty"`
	LastRunAt              string                 `json:"last_run_at,omitempty"`
	LastError              string                 `json:"last_error,omitempty"`
	Targets                []opsBackupTargetState `json:"targets"`
	RecentRuns             []opsBackupRunState    `json:"recent_runs"`
}

type opsBackupTargetState struct {
	ID                      uuid.UUID `json:"id"`
	Kind                    string    `json:"kind"`
	Name                    string    `json:"name"`
	Enabled                 bool      `json:"enabled"`
	SecretConfigured        bool      `json:"secret_configured"`
	AutoBackupEnabled       bool      `json:"auto_backup_enabled"`
	AutoBackupIntervalHours int       `json:"auto_backup_interval_hours"`
	RetentionKeepLast       int       `json:"retention_keep_last"`
	RetentionKeepDays       int       `json:"retention_keep_days"`
	LastAutoBackupAt        string    `json:"last_auto_backup_at,omitempty"`
	LastBackupAt            string    `json:"last_backup_at,omitempty"`
	LastBackupObject        string    `json:"last_backup_object,omitempty"`
	LastBackupError         string    `json:"last_backup_error,omitempty"`
}

type opsBackupRunState struct {
	ID              uuid.UUID `json:"id"`
	TargetID        uuid.UUID `json:"target_id"`
	TargetName      string    `json:"target_name"`
	TargetKind      string    `json:"target_kind"`
	Trigger         string    `json:"trigger"`
	Status          string    `json:"status"`
	ObjectName      string    `json:"object_name,omitempty"`
	SizeBytes       int64     `json:"size_bytes"`
	StartedAt       string    `json:"started_at"`
	CompletedAt     string    `json:"completed_at,omitempty"`
	DurationMs      int64     `json:"duration_ms"`
	Error           string    `json:"error,omitempty"`
	RemoteDeletedAt string    `json:"remote_deleted_at,omitempty"`
}

func (s *Server) handleOpsStatus(w http.ResponseWriter, r *http.Request) {
	if !s.agentCheckAuth(w, r, models.TrustLevelFull, models.ScopeAdmin) {
		return
	}
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}
	status := s.buildOpsStatus(r.Context(), userID)
	respondOK(w, status)
}

func (s *Server) buildOpsStatus(ctx context.Context, userID uuid.UUID) opsStatusResponse {
	checks := []opsCheck{{
		ID:      "server",
		Status:  opsStatusOK,
		Message: "HTTP service is responding.",
	}}
	gitMirror, gitChecks := s.buildOpsGitMirrorStatus(ctx, userID)
	backup, backupChecks := s.buildOpsBackupStatus(ctx, userID)
	checks = append(checks, gitChecks...)
	checks = append(checks, backupChecks...)
	checks = append(checks, buildRemoteBackupArtifactCheck(gitMirror, backup))

	publicURL := ""
	if s != nil && s.Config != nil {
		publicURL = s.Config.PublicBaseURL
	}
	return opsStatusResponse{
		Status:      worstOpsStatus(checks),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Storage:     s.Storage,
		LocalMode:   s.isLocalMode(),
		PublicURL:   publicURL,
		GitMirror:   gitMirror,
		Backup:      backup,
		Checks:      checks,
		Docs: []opsDocRef{
			{Title: "Deployment reliability", Path: "docs/deployment-reliability.zh-CN.md"},
			{Title: "Account storage and mobile sync", Path: "docs/account-storage-mobile-sync.zh-CN.md"},
			{Title: "Production deploy", Path: "deploy/prod/README.md"},
			{Title: "GitHub backup", Path: "docs/github-backup.zh-CN.md"},
		},
	}
}

func (s *Server) buildOpsGitMirrorStatus(ctx context.Context, userID uuid.UUID) (opsGitMirrorStatus, []opsCheck) {
	status := opsGitMirrorStatus{
		ServiceConfigured: s != nil && s.LocalGitSync != nil,
		ExecutionMode:     localgitsync.ExecutionModeHosted,
	}
	if s == nil {
		return status, []opsCheck{{
			ID:      "git_mirror_service",
			Status:  opsStatusWarning,
			Message: "Git mirror service is not configured.",
		}}
	}
	if s.isLocalMode() {
		status.ExecutionMode = localgitsync.ExecutionModeLocal
	}
	if s.Config != nil {
		status.HostedRoot = strings.TrimSpace(s.Config.GitMirrorHostedRoot)
		status.HostedRootSet = status.HostedRoot != ""
	}
	if s.LocalGitSync == nil {
		return status, []opsCheck{{
			ID:      "git_mirror_service",
			Status:  opsStatusWarning,
			Message: "Git mirror service is not configured.",
		}}
	}

	checks := []opsCheck{}
	settings, err := s.LocalGitSync.GetMirrorSettings(ctx, userID)
	if err != nil {
		checks = append(checks, opsCheck{
			ID:      "git_mirror_settings",
			Status:  opsStatusCritical,
			Message: "Git mirror settings cannot be loaded.",
			Action:  err.Error(),
		})
	} else if settings != nil {
		status.ExecutionMode = settings.ExecutionMode
		status.RemoteURL = settings.RemoteURL
		status.AutoPushEnabled = settings.AutoPushEnabled
		status.SyncState = settings.SyncState
		status.LastSyncedAt = settings.LastSyncedAt
		status.LastPushAt = settings.LastPushAt
		status.LastError = settings.LastError
		status.LastPushError = settings.LastPushError
		status.RemoteConflict = settings.RemoteConflict
		status.GitHubAppConnected = settings.GitHubAppUserConnected
		status.GitHubTokenSet = settings.GitHubTokenConfigured
	}

	if status.ExecutionMode == localgitsync.ExecutionModeHosted && !status.HostedRootSet {
		checkStatus := opsStatusWarning
		if status.RemoteURL != "" || status.AutoPushEnabled {
			checkStatus = opsStatusCritical
		}
		checks = append(checks, opsCheck{
			ID:      "git_mirror_hosted_root",
			Status:  checkStatus,
			Message: "Hosted Git mirror root is not configured.",
			Action:  "Set GIT_MIRROR_HOSTED_ROOT and mount a persistent volume.",
		})
	} else {
		checks = append(checks, opsCheck{
			ID:      "git_mirror_hosted_root",
			Status:  opsStatusOK,
			Message: "Git mirror storage path is configured for the current execution mode.",
		})
	}

	switch {
	case status.RemoteConflict:
		checks = append(checks, opsCheck{
			ID:      "git_mirror_remote",
			Status:  opsStatusCritical,
			Message: "Remote Git repository has changes outside neuDrive.",
			Action:  "Review the remote repository before forcing overwrite.",
		})
	case status.LastError != "" || status.LastPushError != "" || status.SyncState == localgitsync.SyncStateError:
		checks = append(checks, opsCheck{
			ID:      "git_mirror_remote",
			Status:  opsStatusCritical,
			Message: "Git mirror has a recent sync or push error.",
			Action:  firstOpsNonEmpty(status.LastPushError, status.LastError),
		})
	case status.RemoteURL == "":
		checks = append(checks, opsCheck{
			ID:      "git_mirror_remote",
			Status:  opsStatusWarning,
			Message: "GitHub backup repository is not configured.",
		})
	case status.LastPushAt == "" && status.LastSyncedAt == "":
		checks = append(checks, opsCheck{
			ID:      "git_mirror_remote",
			Status:  opsStatusWarning,
			Message: "GitHub backup repository is configured, but no successful sync is recorded yet.",
		})
	default:
		checks = append(checks, opsCheck{
			ID:      "git_mirror_remote",
			Status:  opsStatusOK,
			Message: "GitHub backup has a recorded sync.",
		})
	}

	return status, checks
}

func (s *Server) buildOpsBackupStatus(ctx context.Context, userID uuid.UUID) (opsBackupStatus, []opsCheck) {
	status := opsBackupStatus{
		ServiceConfigured: s != nil && s.BackupService != nil,
		Targets:           []opsBackupTargetState{},
		RecentRuns:        []opsBackupRunState{},
	}
	if s == nil || s.BackupService == nil {
		return status, []opsCheck{{
			ID:      "external_backup_service",
			Status:  opsStatusWarning,
			Message: "External backup target service is not configured.",
		}}
	}

	targets, err := s.BackupService.ListTargets(ctx, userID)
	if err != nil {
		return status, []opsCheck{{
			ID:      "external_backup_targets",
			Status:  opsStatusCritical,
			Message: "External backup targets cannot be loaded.",
			Action:  err.Error(),
		}}
	}
	status.TargetsConfigured = len(targets)
	for _, target := range targets {
		targetState := opsBackupTargetState{
			ID:                      target.ID,
			Kind:                    target.Kind,
			Name:                    target.Name,
			Enabled:                 target.Enabled,
			SecretConfigured:        target.SecretConfigured,
			AutoBackupEnabled:       target.AutoBackupEnabled,
			AutoBackupIntervalHours: target.AutoBackupIntervalHours,
			RetentionKeepLast:       target.RetentionKeepLast,
			RetentionKeepDays:       target.RetentionKeepDays,
			LastBackupObject:        target.LastBackupObject,
			LastBackupError:         target.LastBackupError,
		}
		if target.LastAutoBackupAt != nil {
			targetState.LastAutoBackupAt = target.LastAutoBackupAt.UTC().Format(time.RFC3339)
		}
		if target.LastBackupAt != nil {
			targetState.LastBackupAt = target.LastBackupAt.UTC().Format(time.RFC3339)
			status.TargetsWithLastBackup++
			if status.LastSuccessfulBackupAt == "" || target.LastBackupAt.UTC().Format(time.RFC3339) > status.LastSuccessfulBackupAt {
				status.LastSuccessfulBackupAt = target.LastBackupAt.UTC().Format(time.RFC3339)
				status.LastBackupObject = target.LastBackupObject
			}
		}
		if target.Enabled {
			status.EnabledTargets++
		}
		if target.SecretConfigured {
			status.TargetsWithSecrets++
		}
		if target.LastBackupError != "" {
			status.LastError = target.LastBackupError
		}
		status.Targets = append(status.Targets, targetState)
	}

	if runs, err := s.BackupService.ListRuns(ctx, userID, nil, 8); err == nil {
		status.HistoryCount = len(runs)
		for idx, run := range runs {
			runState := opsBackupRunState{
				ID:         run.ID,
				TargetID:   run.TargetID,
				TargetName: run.TargetName,
				TargetKind: run.TargetKind,
				Trigger:    run.Trigger,
				Status:     run.Status,
				ObjectName: run.ObjectName,
				SizeBytes:  run.SizeBytes,
				StartedAt:  run.StartedAt.UTC().Format(time.RFC3339),
				DurationMs: run.DurationMs,
				Error:      run.Error,
			}
			if run.CompletedAt != nil {
				runState.CompletedAt = run.CompletedAt.UTC().Format(time.RFC3339)
			}
			if run.RemoteDeletedAt != nil {
				runState.RemoteDeletedAt = run.RemoteDeletedAt.UTC().Format(time.RFC3339)
			}
			if idx == 0 {
				status.LastRunStatus = run.Status
				status.LastRunAt = runState.StartedAt
				if run.Error != "" {
					status.LastError = run.Error
				}
			}
			status.RecentRuns = append(status.RecentRuns, runState)
		}
	}

	checks := []opsCheck{}
	switch {
	case status.TargetsConfigured == 0:
		checks = append(checks, opsCheck{
			ID:      "external_backup_targets",
			Status:  opsStatusWarning,
			Message: "No WebDAV or S3-compatible backup target is configured.",
		})
	case status.EnabledTargets == 0:
		checks = append(checks, opsCheck{
			ID:      "external_backup_targets",
			Status:  opsStatusWarning,
			Message: "External backup targets exist, but none is enabled.",
		})
	case status.LastError != "":
		checks = append(checks, opsCheck{
			ID:      "external_backup_targets",
			Status:  opsStatusCritical,
			Message: "At least one external backup target has a recent upload error.",
			Action:  status.LastError,
		})
	case status.TargetsWithLastBackup == 0:
		checks = append(checks, opsCheck{
			ID:      "external_backup_targets",
			Status:  opsStatusWarning,
			Message: "External backup targets are configured, but no successful upload is recorded yet.",
		})
	default:
		checks = append(checks, opsCheck{
			ID:      "external_backup_targets",
			Status:  opsStatusOK,
			Message: "External backup target has a recorded upload.",
		})
	}
	return status, checks
}

func buildRemoteBackupArtifactCheck(git opsGitMirrorStatus, backup opsBackupStatus) opsCheck {
	gitHasBackup := git.RemoteURL != "" && (git.LastPushAt != "" || git.LastSyncedAt != "") && git.LastError == "" && git.LastPushError == "" && !git.RemoteConflict
	externalHasBackup := backup.TargetsWithLastBackup > 0 && backup.LastError == ""
	if gitHasBackup || externalHasBackup {
		return opsCheck{
			ID:      "remote_backup_artifact",
			Status:  opsStatusOK,
			Message: "At least one remote backup artifact is recorded.",
		}
	}
	if git.RemoteURL != "" || backup.EnabledTargets > 0 {
		return opsCheck{
			ID:      "remote_backup_artifact",
			Status:  opsStatusWarning,
			Message: "Backup destination exists, but no successful remote backup artifact is recorded yet.",
		}
	}
	return opsCheck{
		ID:      "remote_backup_artifact",
		Status:  opsStatusWarning,
		Message: "No remote backup destination has been set up.",
	}
}

func worstOpsStatus(checks []opsCheck) string {
	status := opsStatusOK
	for _, check := range checks {
		switch check.Status {
		case opsStatusCritical:
			return opsStatusCritical
		case opsStatusWarning:
			status = opsStatusWarning
		}
	}
	return status
}

func firstOpsNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
