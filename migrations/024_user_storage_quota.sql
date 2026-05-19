ALTER TABLE users
    ADD COLUMN IF NOT EXISTS storage_quota_bytes BIGINT CHECK (storage_quota_bytes IS NULL OR storage_quota_bytes >= 0);
