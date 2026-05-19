CREATE TABLE IF NOT EXISTS file_blobs (
    entry_id UUID PRIMARY KEY REFERENCES file_tree(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data BYTEA,
    size_bytes BIGINT NOT NULL,
    sha256 VARCHAR(128) NOT NULL,
    storage_backend VARCHAR(32) NOT NULL DEFAULT 'db',
    object_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT file_blobs_storage_location_check CHECK (
        (storage_backend = 'db' AND data IS NOT NULL)
        OR
        (storage_backend <> 'db' AND object_key IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_file_blobs_user_id
    ON file_blobs(user_id);
CREATE INDEX IF NOT EXISTS idx_file_blobs_storage_backend
    ON file_blobs(storage_backend);
CREATE INDEX IF NOT EXISTS idx_file_blobs_object_key
    ON file_blobs(object_key)
    WHERE object_key IS NOT NULL;
