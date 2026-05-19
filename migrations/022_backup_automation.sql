ALTER TABLE backup_targets
    ADD COLUMN IF NOT EXISTS auto_backup_enabled BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE backup_targets
    ADD COLUMN IF NOT EXISTS auto_backup_interval_hours INTEGER NOT NULL DEFAULT 24;

ALTER TABLE backup_targets
    ADD COLUMN IF NOT EXISTS last_auto_backup_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS backup_targets_auto_backup_idx
    ON backup_targets(enabled, auto_backup_enabled, last_backup_at);
