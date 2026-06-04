package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/agi-bar/vola/internal/backups"
	"github.com/google/uuid"
)

func (s *Store) ListBackupTargets(ctx context.Context, userID uuid.UUID) ([]backups.Target, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, kind, name, enabled, config_json, secret_configured,
		        auto_backup_enabled, auto_backup_interval_hours, COALESCE(last_auto_backup_at, ''),
		        retention_keep_last, retention_keep_days,
		        COALESCE(last_backup_at, ''), COALESCE(last_backup_object, ''), COALESCE(last_backup_error, ''),
		        created_at, updated_at
		   FROM backup_targets
		  WHERE user_id = ?
		  ORDER BY created_at ASC`,
		userID.String(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []backups.Target{}
	for rows.Next() {
		target, err := scanBackupTarget(rows)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (s *Store) GetBackupTarget(ctx context.Context, userID, targetID uuid.UUID) (*backups.Target, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, kind, name, enabled, config_json, secret_configured,
		        auto_backup_enabled, auto_backup_interval_hours, COALESCE(last_auto_backup_at, ''),
		        retention_keep_last, retention_keep_days,
		        COALESCE(last_backup_at, ''), COALESCE(last_backup_object, ''), COALESCE(last_backup_error, ''),
		        created_at, updated_at
		   FROM backup_targets
		  WHERE id = ? AND user_id = ?
		  LIMIT 1`,
		targetID.String(),
		userID.String(),
	)
	target, err := scanBackupTarget(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &target, nil
}

func (s *Store) ListAutoBackupTargets(ctx context.Context) ([]backups.Target, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, kind, name, enabled, config_json, secret_configured,
		        auto_backup_enabled, auto_backup_interval_hours, COALESCE(last_auto_backup_at, ''),
		        retention_keep_last, retention_keep_days,
		        COALESCE(last_backup_at, ''), COALESCE(last_backup_object, ''), COALESCE(last_backup_error, ''),
		        created_at, updated_at
		   FROM backup_targets
		  WHERE enabled = 1 AND auto_backup_enabled = 1
		  ORDER BY COALESCE(last_backup_at, created_at) ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []backups.Target{}
	for rows.Next() {
		target, err := scanBackupTarget(rows)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (s *Store) UpsertBackupTarget(ctx context.Context, target backups.Target) error {
	now := time.Now().UTC()
	if target.CreatedAt.IsZero() {
		target.CreatedAt = now
	}
	target.UpdatedAt = now
	configJSON, err := json.Marshal(backups.ConfigFromTarget(target))
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO backup_targets (
			id, user_id, kind, name, enabled, config_json, secret_configured,
			auto_backup_enabled, auto_backup_interval_hours, last_auto_backup_at,
			retention_keep_last, retention_keep_days,
			last_backup_at, last_backup_object, last_backup_error, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			kind = excluded.kind,
			name = excluded.name,
			enabled = excluded.enabled,
			config_json = excluded.config_json,
			secret_configured = excluded.secret_configured,
			auto_backup_enabled = excluded.auto_backup_enabled,
			auto_backup_interval_hours = excluded.auto_backup_interval_hours,
			last_auto_backup_at = excluded.last_auto_backup_at,
			retention_keep_last = excluded.retention_keep_last,
			retention_keep_days = excluded.retention_keep_days,
			last_backup_at = excluded.last_backup_at,
			last_backup_object = excluded.last_backup_object,
			last_backup_error = excluded.last_backup_error,
			updated_at = excluded.updated_at`,
		target.ID.String(),
		target.UserID.String(),
		target.Kind,
		target.Name,
		boolToSQLite(target.Enabled),
		string(configJSON),
		boolToSQLite(target.SecretConfigured),
		boolToSQLite(target.AutoBackupEnabled),
		target.AutoBackupIntervalHours,
		localGitMirrorNullableTimeText(target.LastAutoBackupAt),
		target.RetentionKeepLast,
		target.RetentionKeepDays,
		localGitMirrorNullableTimeText(target.LastBackupAt),
		target.LastBackupObject,
		target.LastBackupError,
		timeText(target.CreatedAt),
		timeText(target.UpdatedAt),
	)
	return err
}

func (s *Store) UpdateBackupTargetResult(ctx context.Context, userID, targetID uuid.UUID, backupAt *time.Time, objectName, lastError string, auto bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE backup_targets
		    SET last_backup_at = CASE WHEN ? IS NOT NULL THEN ? ELSE last_backup_at END,
		        last_auto_backup_at = CASE WHEN ? = 1 AND ? IS NOT NULL THEN ? ELSE last_auto_backup_at END,
		        last_backup_object = CASE WHEN ? IS NOT NULL THEN ? ELSE last_backup_object END,
		        last_backup_error = ?,
		        updated_at = ?
		  WHERE id = ? AND user_id = ?`,
		localGitMirrorNullableTimeText(backupAt),
		localGitMirrorNullableTimeText(backupAt),
		boolToSQLite(auto),
		localGitMirrorNullableTimeText(backupAt),
		localGitMirrorNullableTimeText(backupAt),
		localGitMirrorNullableTimeText(backupAt),
		objectName,
		lastError,
		timeText(time.Now().UTC()),
		targetID.String(),
		userID.String(),
	)
	return err
}

func (s *Store) InsertBackupRun(ctx context.Context, run backups.Run) error {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO backup_runs (
			id, user_id, target_id, target_name, target_kind, trigger, status,
			object_name, location, size_bytes, started_at, completed_at, duration_ms,
			error, remote_deleted_at, remote_delete_error, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID.String(),
		run.UserID.String(),
		run.TargetID.String(),
		run.TargetName,
		run.TargetKind,
		run.Trigger,
		run.Status,
		run.ObjectName,
		run.Location,
		run.SizeBytes,
		timeText(run.StartedAt),
		localGitMirrorNullableTimeText(run.CompletedAt),
		run.DurationMs,
		run.Error,
		localGitMirrorNullableTimeText(run.RemoteDeletedAt),
		run.RemoteDeleteError,
		timeText(run.CreatedAt),
	)
	return err
}

func (s *Store) ListBackupRuns(ctx context.Context, userID uuid.UUID, targetID *uuid.UUID, limit int) ([]backups.Run, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args := []any{userID.String()}
	filter := ""
	if targetID != nil && *targetID != uuid.Nil {
		args = append(args, targetID.String())
		filter = " AND target_id = ?"
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, target_id, target_name, target_kind, trigger, status,
		        COALESCE(object_name, ''), COALESCE(location, ''), size_bytes,
		        started_at, COALESCE(completed_at, ''), duration_ms, COALESCE(error, ''),
		        COALESCE(remote_deleted_at, ''), COALESCE(remote_delete_error, ''), created_at
		   FROM backup_runs
		  WHERE user_id = ?`+filter+`
		  ORDER BY started_at DESC
		  LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []backups.Run{}
	for rows.Next() {
		run, err := scanBackupRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) ListPrunableBackupRuns(ctx context.Context, userID, targetID uuid.UUID, keepLast, keepDays int, now time.Time) ([]backups.Run, error) {
	runs, err := s.ListBackupRuns(ctx, userID, &targetID, 500)
	if err != nil {
		return nil, err
	}
	cutoff := time.Time{}
	if keepDays > 0 {
		cutoff = now.UTC().AddDate(0, 0, -keepDays)
	}
	successIndex := 0
	prunable := []backups.Run{}
	for _, run := range runs {
		if run.Status != backups.RunStatusSuccess || run.ObjectName == "" || run.RemoteDeletedAt != nil {
			continue
		}
		successIndex++
		if keepLast > 0 && successIndex <= keepLast {
			continue
		}
		if !cutoff.IsZero() && run.CompletedAt != nil && run.CompletedAt.After(cutoff) {
			continue
		}
		prunable = append(prunable, run)
	}
	return prunable, nil
}

func (s *Store) UpdateBackupRunRemoteDelete(ctx context.Context, userID, runID uuid.UUID, deletedAt *time.Time, deleteError string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE backup_runs
		    SET remote_deleted_at = ?,
		        remote_delete_error = ?
		  WHERE id = ? AND user_id = ?`,
		localGitMirrorNullableTimeText(deletedAt),
		deleteError,
		runID.String(),
		userID.String(),
	)
	return err
}

type backupTargetScanner interface {
	Scan(dest ...any) error
}

func scanBackupTarget(row backupTargetScanner) (backups.Target, error) {
	var (
		rawID             string
		rawUserID         string
		configJSON        string
		lastBackupAt      string
		lastAutoBackupAt  string
		createdAt         string
		updatedAt         string
		target            backups.Target
		secretConfigured  bool
		enabled           bool
		autoBackupEnabled bool
	)
	if err := row.Scan(
		&rawID,
		&rawUserID,
		&target.Kind,
		&target.Name,
		&enabled,
		&configJSON,
		&secretConfigured,
		&autoBackupEnabled,
		&target.AutoBackupIntervalHours,
		&lastAutoBackupAt,
		&target.RetentionKeepLast,
		&target.RetentionKeepDays,
		&lastBackupAt,
		&target.LastBackupObject,
		&target.LastBackupError,
		&createdAt,
		&updatedAt,
	); err != nil {
		return target, err
	}
	targetID, err := uuid.Parse(rawID)
	if err != nil {
		return target, err
	}
	userID, err := uuid.Parse(rawUserID)
	if err != nil {
		return target, err
	}
	var cfg backups.Config
	_ = json.Unmarshal([]byte(configJSON), &cfg)
	target.ID = targetID
	target.UserID = userID
	target.Enabled = enabled
	target.SecretConfigured = secretConfigured
	target.AutoBackupEnabled = autoBackupEnabled
	target.LastAutoBackupAt = nullableTime(lastAutoBackupAt)
	target.LastBackupAt = nullableTime(lastBackupAt)
	target.CreatedAt = mustParseTime(createdAt)
	target.UpdatedAt = mustParseTime(updatedAt)
	backups.ApplyConfig(&target, cfg)
	return target, nil
}

type backupRunScanner interface {
	Scan(dest ...any) error
}

func scanBackupRun(row backupRunScanner) (backups.Run, error) {
	var (
		rawID           string
		rawUserID       string
		rawTargetID     string
		startedAt       string
		completedAt     string
		remoteDeletedAt string
		createdAt       string
		run             backups.Run
	)
	if err := row.Scan(
		&rawID,
		&rawUserID,
		&rawTargetID,
		&run.TargetName,
		&run.TargetKind,
		&run.Trigger,
		&run.Status,
		&run.ObjectName,
		&run.Location,
		&run.SizeBytes,
		&startedAt,
		&completedAt,
		&run.DurationMs,
		&run.Error,
		&remoteDeletedAt,
		&run.RemoteDeleteError,
		&createdAt,
	); err != nil {
		return run, err
	}
	var err error
	run.ID, err = uuid.Parse(rawID)
	if err != nil {
		return run, err
	}
	run.UserID, err = uuid.Parse(rawUserID)
	if err != nil {
		return run, err
	}
	run.TargetID, err = uuid.Parse(rawTargetID)
	if err != nil {
		return run, err
	}
	run.StartedAt = mustParseTime(startedAt)
	run.CompletedAt = nullableTime(completedAt)
	run.RemoteDeletedAt = nullableTime(remoteDeletedAt)
	run.CreatedAt = mustParseTime(createdAt)
	return run, nil
}
