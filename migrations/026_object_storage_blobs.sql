ALTER TABLE file_blobs
    ALTER COLUMN data DROP NOT NULL;

ALTER TABLE file_blobs
    ADD COLUMN IF NOT EXISTS storage_backend VARCHAR(32) NOT NULL DEFAULT 'db';

ALTER TABLE file_blobs
    ADD COLUMN IF NOT EXISTS object_key TEXT;

CREATE INDEX IF NOT EXISTS idx_file_blobs_storage_backend
    ON file_blobs(storage_backend);

CREATE INDEX IF NOT EXISTS idx_file_blobs_object_key
    ON file_blobs(object_key)
    WHERE object_key IS NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conname = 'file_blobs_storage_location_check'
    ) THEN
        ALTER TABLE file_blobs
            ADD CONSTRAINT file_blobs_storage_location_check
            CHECK (
                (storage_backend = 'db' AND data IS NOT NULL)
                OR
                (storage_backend <> 'db' AND object_key IS NOT NULL)
            );
    END IF;
END $$;
