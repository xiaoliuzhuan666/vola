CREATE TABLE IF NOT EXISTS backup_targets (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    secret_configured BOOLEAN NOT NULL DEFAULT false,
    last_backup_at TIMESTAMPTZ,
    last_backup_object TEXT NOT NULL DEFAULT '',
    last_backup_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS backup_targets_user_id_idx ON backup_targets(user_id);
