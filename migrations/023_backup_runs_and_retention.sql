ALTER TABLE backup_targets
    ADD COLUMN IF NOT EXISTS retention_keep_last INTEGER NOT NULL DEFAULT 0;

ALTER TABLE backup_targets
    ADD COLUMN IF NOT EXISTS retention_keep_days INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS backup_runs (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_id UUID NOT NULL REFERENCES backup_targets(id) ON DELETE CASCADE,
    target_name TEXT NOT NULL DEFAULT '',
    target_kind TEXT NOT NULL DEFAULT '',
    trigger TEXT NOT NULL DEFAULT 'manual',
    status TEXT NOT NULL,
    object_name TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    remote_deleted_at TIMESTAMPTZ,
    remote_delete_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS backup_runs_user_started_idx
    ON backup_runs(user_id, started_at DESC);

CREATE INDEX IF NOT EXISTS backup_runs_target_started_idx
    ON backup_runs(target_id, started_at DESC);
