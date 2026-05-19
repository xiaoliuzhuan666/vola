package backups

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepo struct {
	db *pgxpool.Pool
}

func NewPostgresRepo(db *pgxpool.Pool) *PostgresRepo {
	if db == nil {
		return nil
	}
	return &PostgresRepo{db: db}
}

func (r *PostgresRepo) ListBackupTargets(ctx context.Context, userID uuid.UUID) ([]Target, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, kind, name, enabled, config_json, secret_configured,
		        auto_backup_enabled, auto_backup_interval_hours, last_auto_backup_at,
		        retention_keep_last, retention_keep_days,
		        last_backup_at, last_backup_object, last_backup_error, created_at, updated_at
		   FROM backup_targets
		  WHERE user_id = $1
		  ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []Target{}
	for rows.Next() {
		target, err := scanPostgresTarget(rows)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (r *PostgresRepo) GetBackupTarget(ctx context.Context, userID, targetID uuid.UUID) (*Target, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, user_id, kind, name, enabled, config_json, secret_configured,
		        auto_backup_enabled, auto_backup_interval_hours, last_auto_backup_at,
		        retention_keep_last, retention_keep_days,
		        last_backup_at, last_backup_object, last_backup_error, created_at, updated_at
		   FROM backup_targets
		  WHERE id = $1 AND user_id = $2
		  LIMIT 1`,
		targetID,
		userID,
	)
	target, err := scanPostgresTarget(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &target, nil
}

func (r *PostgresRepo) ListAutoBackupTargets(ctx context.Context) ([]Target, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, kind, name, enabled, config_json, secret_configured,
		        auto_backup_enabled, auto_backup_interval_hours, last_auto_backup_at,
		        retention_keep_last, retention_keep_days,
		        last_backup_at, last_backup_object, last_backup_error, created_at, updated_at
		   FROM backup_targets
		  WHERE enabled = true AND auto_backup_enabled = true
		  ORDER BY COALESCE(last_backup_at, created_at) ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []Target{}
	for rows.Next() {
		target, err := scanPostgresTarget(rows)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (r *PostgresRepo) UpsertBackupTarget(ctx context.Context, target Target) error {
	now := time.Now().UTC()
	if target.CreatedAt.IsZero() {
		target.CreatedAt = now
	}
	target.UpdatedAt = now
	configJSON, err := json.Marshal(ConfigFromTarget(target))
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO backup_targets (
			id, user_id, kind, name, enabled, config_json, secret_configured,
			auto_backup_enabled, auto_backup_interval_hours, last_auto_backup_at,
			retention_keep_last, retention_keep_days,
			last_backup_at, last_backup_object, last_backup_error, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (id) DO UPDATE SET
			kind = EXCLUDED.kind,
			name = EXCLUDED.name,
			enabled = EXCLUDED.enabled,
			config_json = EXCLUDED.config_json,
			secret_configured = EXCLUDED.secret_configured,
			auto_backup_enabled = EXCLUDED.auto_backup_enabled,
			auto_backup_interval_hours = EXCLUDED.auto_backup_interval_hours,
			last_auto_backup_at = EXCLUDED.last_auto_backup_at,
			retention_keep_last = EXCLUDED.retention_keep_last,
			retention_keep_days = EXCLUDED.retention_keep_days,
			last_backup_at = EXCLUDED.last_backup_at,
			last_backup_object = EXCLUDED.last_backup_object,
			last_backup_error = EXCLUDED.last_backup_error,
			updated_at = EXCLUDED.updated_at`,
		target.ID,
		target.UserID,
		target.Kind,
		target.Name,
		target.Enabled,
		configJSON,
		target.SecretConfigured,
		target.AutoBackupEnabled,
		target.AutoBackupIntervalHours,
		target.LastAutoBackupAt,
		target.RetentionKeepLast,
		target.RetentionKeepDays,
		target.LastBackupAt,
		target.LastBackupObject,
		target.LastBackupError,
		target.CreatedAt,
		target.UpdatedAt,
	)
	return err
}

func (r *PostgresRepo) UpdateBackupTargetResult(ctx context.Context, userID, targetID uuid.UUID, backupAt *time.Time, objectName, lastError string, auto bool) error {
	_, err := r.db.Exec(ctx,
		`UPDATE backup_targets
		    SET last_backup_at = CASE WHEN $1::timestamptz IS NOT NULL THEN $1::timestamptz ELSE last_backup_at END,
		        last_auto_backup_at = CASE WHEN $2::boolean AND $1::timestamptz IS NOT NULL THEN $1::timestamptz ELSE last_auto_backup_at END,
		        last_backup_object = CASE WHEN $1::timestamptz IS NOT NULL THEN $3 ELSE last_backup_object END,
		        last_backup_error = $4,
		        updated_at = $5
		  WHERE id = $6 AND user_id = $7`,
		backupAt,
		auto,
		objectName,
		lastError,
		time.Now().UTC(),
		targetID,
		userID,
	)
	return err
}

func (r *PostgresRepo) InsertBackupRun(ctx context.Context, run Run) error {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO backup_runs (
			id, user_id, target_id, target_name, target_kind, trigger, status,
			object_name, location, size_bytes, started_at, completed_at, duration_ms,
			error, remote_deleted_at, remote_delete_error, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		run.ID, run.UserID, run.TargetID, run.TargetName, run.TargetKind, run.Trigger, run.Status,
		run.ObjectName, run.Location, run.SizeBytes, run.StartedAt, run.CompletedAt, run.DurationMs,
		run.Error, run.RemoteDeletedAt, run.RemoteDeleteError, run.CreatedAt,
	)
	return err
}

func (r *PostgresRepo) ListBackupRuns(ctx context.Context, userID uuid.UUID, targetID *uuid.UUID, limit int) ([]Run, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args := []any{userID, limit}
	filter := ""
	if targetID != nil && *targetID != uuid.Nil {
		args = append(args, *targetID)
		filter = " AND target_id = $3"
	}
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT id, user_id, target_id, target_name, target_kind, trigger, status,
		        object_name, location, size_bytes, started_at, completed_at, duration_ms,
		        error, remote_deleted_at, remote_delete_error, created_at
		   FROM backup_runs
		  WHERE user_id = $1%s
		  ORDER BY started_at DESC
		  LIMIT $2`, filter),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []Run{}
	for rows.Next() {
		run, err := scanPostgresRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *PostgresRepo) ListPrunableBackupRuns(ctx context.Context, userID, targetID uuid.UUID, keepLast, keepDays int, now time.Time) ([]Run, error) {
	runs, err := r.ListBackupRuns(ctx, userID, &targetID, 500)
	if err != nil {
		return nil, err
	}
	cutoff := time.Time{}
	if keepDays > 0 {
		cutoff = now.UTC().AddDate(0, 0, -keepDays)
	}
	successIndex := 0
	prunable := []Run{}
	for _, run := range runs {
		if run.Status != RunStatusSuccess || run.ObjectName == "" || run.RemoteDeletedAt != nil {
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

func (r *PostgresRepo) UpdateBackupRunRemoteDelete(ctx context.Context, userID, runID uuid.UUID, deletedAt *time.Time, deleteError string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE backup_runs
		    SET remote_deleted_at = $1,
		        remote_delete_error = $2
		  WHERE id = $3 AND user_id = $4`,
		deletedAt,
		deleteError,
		runID,
		userID,
	)
	return err
}

type postgresTargetScanner interface {
	Scan(dest ...any) error
}

func scanPostgresTarget(row postgresTargetScanner) (Target, error) {
	var (
		target     Target
		configJSON []byte
	)
	if err := row.Scan(
		&target.ID,
		&target.UserID,
		&target.Kind,
		&target.Name,
		&target.Enabled,
		&configJSON,
		&target.SecretConfigured,
		&target.AutoBackupEnabled,
		&target.AutoBackupIntervalHours,
		&target.LastAutoBackupAt,
		&target.RetentionKeepLast,
		&target.RetentionKeepDays,
		&target.LastBackupAt,
		&target.LastBackupObject,
		&target.LastBackupError,
		&target.CreatedAt,
		&target.UpdatedAt,
	); err != nil {
		return target, err
	}
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return target, fmt.Errorf("backup target config: %w", err)
	}
	ApplyConfig(&target, cfg)
	return target, nil
}

type postgresRunScanner interface {
	Scan(dest ...any) error
}

func scanPostgresRun(row postgresRunScanner) (Run, error) {
	var run Run
	err := row.Scan(
		&run.ID,
		&run.UserID,
		&run.TargetID,
		&run.TargetName,
		&run.TargetKind,
		&run.Trigger,
		&run.Status,
		&run.ObjectName,
		&run.Location,
		&run.SizeBytes,
		&run.StartedAt,
		&run.CompletedAt,
		&run.DurationMs,
		&run.Error,
		&run.RemoteDeletedAt,
		&run.RemoteDeleteError,
		&run.CreatedAt,
	)
	return run, err
}
