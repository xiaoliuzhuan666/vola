package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	pathpkg "path"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/hubpath"
	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/objectstore"
	"github.com/agi-bar/vola/internal/systemskills"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *FileTreeService) WriteBinaryEntry(
	ctx context.Context,
	userID uuid.UUID,
	path string,
	data []byte,
	contentType string,
	opts models.FileTreeWriteOptions,
) (*models.FileTreeEntry, error) {
	if s.repo != nil {
		return s.repo.WriteBinaryEntry(ctx, userID, path, data, contentType, opts)
	}
	storagePath := hubpath.NormalizeStorage(path)
	if systemskills.IsProtectedPath(storagePath) {
		return nil, ErrReadOnlyPath
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if s.db == nil {
		return nil, fmt.Errorf("filetree.WriteBinaryEntry: database not configured")
	}

	parentDirs := make([]string, 0, 4)
	for dir := pathpkg.Dir(strings.TrimSuffix(storagePath, "/")); dir != "." && dir != "/" && dir != ""; dir = pathpkg.Dir(dir) {
		parentDirs = append(parentDirs, dir)
	}
	for i := len(parentDirs) - 1; i >= 0; i-- {
		if err := s.EnsureDirectory(ctx, userID, parentDirs[i]); err != nil {
			return nil, fmt.Errorf("filetree.WriteBinaryEntry: ensure parent dir %q: %w", parentDirs[i], err)
		}
	}

	now := time.Now().UTC()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("filetree.WriteBinaryEntry: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := s.lockEntry(ctx, tx, userID, storagePath)
	if err != nil && !errors.Is(err, ErrEntryNotFound) {
		return nil, err
	}

	metadata := mergeMetadata(nil, opts.Metadata)
	minTrust := opts.MinTrustLevel
	if minTrust <= 0 {
		minTrust = models.TrustLevelGuest
	}
	kind := strings.TrimSpace(opts.Kind)
	if kind == "" {
		kind = classifyEntryKind(storagePath, false)
	}

	if current != nil {
		if opts.ExpectedVersion != nil && current.Version != *opts.ExpectedVersion {
			return nil, ErrOptimisticLockConflict
		}
		if opts.ExpectedChecksum != "" && current.Checksum != opts.ExpectedChecksum {
			return nil, ErrOptimisticLockConflict
		}
		if err := s.enforceStorageQuotaTx(ctx, tx, userID, current, int64(len(data))); err != nil {
			return nil, err
		}
		metadata = mergeMetadata(current.Metadata, opts.Metadata)
		metadata = WithSourceContextMetadata(metadata, ctx)
		blobInfo, err := s.storeBlob(ctx, current.ID, userID, data, contentType, "")
		if err != nil {
			return nil, err
		}
		metadata = mergeMetadata(metadata, blobInfo.Metadata())
		checksum := entryChecksum(hubpath.NormalizePublic(storagePath), "", contentType, metadata)

		var updated models.FileTreeEntry
		err = tx.QueryRow(ctx,
			fmt.Sprintf(`UPDATE file_tree
			 SET kind = $3,
			     is_directory = false,
			     content = '',
			     content_type = $4,
			     metadata = $5,
			     checksum = $6,
			     version = version + 1,
			     min_trust_level = $7,
			     deleted_at = NULL,
			     updated_at = $8
			 WHERE user_id = $1 AND path = $2
			 RETURNING %s`, fileTreeSelectColumns),
			userID, current.Path, kind, contentType, metadata, checksum, minTrust, now,
		).Scan(
			&updated.ID,
			&updated.UserID,
			&updated.Path,
			&updated.Kind,
			&updated.IsDirectory,
			&updated.Content,
			&updated.ContentType,
			&updated.Metadata,
			&updated.Checksum,
			&updated.Version,
			&updated.MinTrustLevel,
			&updated.CreatedAt,
			&updated.UpdatedAt,
			&updated.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("filetree.WriteBinaryEntry: update: %w", err)
		}
		if err := s.upsertBlobTxWithInfo(ctx, tx, updated.ID, userID, data, blobInfo, now); err != nil {
			return nil, err
		}
		if err := s.insertEntryVersion(ctx, tx, &updated, "update"); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("filetree.WriteBinaryEntry: commit update: %w", err)
		}
		return &updated, nil
	}

	if err := s.enforceStorageQuotaTx(ctx, tx, userID, nil, int64(len(data))); err != nil {
		return nil, err
	}
	entryID := uuid.New()
	metadata = WithSourceContextMetadata(metadata, ctx)
	blobInfo, err := s.storeBlob(ctx, entryID, userID, data, contentType, "")
	if err != nil {
		return nil, err
	}
	metadata = mergeMetadata(metadata, blobInfo.Metadata())
	checksum := entryChecksum(hubpath.NormalizePublic(storagePath), "", contentType, metadata)

	entry := &models.FileTreeEntry{
		ID:            entryID,
		UserID:        userID,
		Path:          storagePath,
		Kind:          kind,
		IsDirectory:   false,
		Content:       "",
		ContentType:   contentType,
		Metadata:      metadata,
		Checksum:      checksum,
		Version:       1,
		MinTrustLevel: minTrust,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO file_tree (
			id, user_id, path, kind, is_directory, content, content_type, metadata,
			checksum, version, min_trust_level, created_at, updated_at
		) VALUES ($1, $2, $3, $4, false, '', $5, $6, $7, 1, $8, $9, $9)`,
		entry.ID, entry.UserID, entry.Path, entry.Kind, entry.ContentType,
		entry.Metadata, entry.Checksum, entry.MinTrustLevel, entry.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("filetree.WriteBinaryEntry: insert: %w", err)
	}
	if err := s.upsertBlobTxWithInfo(ctx, tx, entry.ID, userID, data, blobInfo, now); err != nil {
		return nil, err
	}
	if err := s.insertEntryVersion(ctx, tx, entry, "create"); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("filetree.WriteBinaryEntry: commit insert: %w", err)
	}
	return entry, nil
}

func (s *FileTreeService) ReadBinary(ctx context.Context, userID uuid.UUID, path string, trustLevel int) ([]byte, *models.FileTreeEntry, error) {
	if s.repo != nil {
		return s.repo.ReadBinary(ctx, userID, path, trustLevel)
	}
	entry, err := s.Read(ctx, userID, path, trustLevel)
	if err != nil {
		return nil, nil, err
	}
	data, ok, err := s.ReadBlobByEntryID(ctx, entry.ID)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, entry, fmt.Errorf("filetree.ReadBinary: blob not found for %s", hubpath.NormalizePublic(path))
	}
	return data, entry, nil
}

func (s *FileTreeService) ReadBlobByEntryID(ctx context.Context, entryID uuid.UUID) ([]byte, bool, error) {
	if reader, ok := s.repo.(interface {
		ReadBlobByEntryID(context.Context, uuid.UUID) ([]byte, bool, error)
	}); ok {
		return reader.ReadBlobByEntryID(ctx, entryID)
	}
	if s.db == nil {
		return nil, false, fmt.Errorf("filetree.ReadBlobByEntryID: database not configured")
	}
	var data []byte
	var storageBackend string
	var objectKey sql.NullString
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(data, ''::bytea), COALESCE(storage_backend, 'db'), object_key
		   FROM file_blobs
		  WHERE entry_id = $1`,
		entryID,
	).Scan(&data, &storageBackend, &objectKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("filetree.ReadBlobByEntryID: %w", err)
	}
	if storageBackend == objectstore.BackendCOS {
		if s.blobStore == nil || !s.blobStore.Enabled() {
			return nil, true, fmt.Errorf("filetree.ReadBlobByEntryID: blob is stored in COS but object storage is not configured")
		}
		if !objectKey.Valid || strings.TrimSpace(objectKey.String) == "" {
			return nil, true, fmt.Errorf("filetree.ReadBlobByEntryID: COS object key missing")
		}
		data, err := s.blobStore.Get(ctx, objectKey.String)
		if err != nil {
			if errors.Is(err, objectstore.ErrObjectNotFound) {
				return nil, false, nil
			}
			return nil, true, fmt.Errorf("filetree.ReadBlobByEntryID: COS get: %w", err)
		}
		return data, true, nil
	}
	return data, true, nil
}

func (s *FileTreeService) upsertBlobTx(ctx context.Context, tx pgx.Tx, entryID, userID uuid.UUID, data []byte, now time.Time) error {
	info, err := s.storeBlob(ctx, entryID, userID, data, "", "")
	if err != nil {
		return err
	}
	return s.upsertBlobTxWithInfo(ctx, tx, entryID, userID, data, info, now)
}

func (s *FileTreeService) upsertBlobTxWithSHA(ctx context.Context, tx pgx.Tx, entryID, userID uuid.UUID, data []byte, sha256Hex string, now time.Time) error {
	info, err := s.storeBlob(ctx, entryID, userID, data, "", sha256Hex)
	if err != nil {
		return err
	}
	return s.upsertBlobTxWithInfo(ctx, tx, entryID, userID, data, info, now)
}

func (s *FileTreeService) upsertBlobTxWithInfo(ctx context.Context, tx pgx.Tx, entryID, userID uuid.UUID, data []byte, info blobStorageInfo, now time.Time) error {
	var blobData interface{} = data
	var objectKey interface{}
	if info.Backend != objectstore.BackendDB {
		blobData = nil
		objectKey = info.ObjectKey
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO file_blobs (entry_id, user_id, data, size_bytes, sha256, storage_backend, object_key, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		 ON CONFLICT (entry_id) DO UPDATE SET
		   user_id = EXCLUDED.user_id,
		   data = EXCLUDED.data,
		   size_bytes = EXCLUDED.size_bytes,
		   sha256 = EXCLUDED.sha256,
		   storage_backend = EXCLUDED.storage_backend,
		   object_key = EXCLUDED.object_key,
		   updated_at = EXCLUDED.updated_at`,
		entryID, userID, blobData, info.SizeBytes, info.SHA256, info.Backend, objectKey, now,
	)
	if err != nil {
		return fmt.Errorf("filetree.upsertBlobTx: %w", err)
	}
	return nil
}

func (s *FileTreeService) deleteBlobTx(ctx context.Context, tx pgx.Tx, entryID uuid.UUID) error {
	if s.blobStore != nil && s.blobStore.Enabled() {
		var storageBackend string
		var objectKey sql.NullString
		err := tx.QueryRow(ctx,
			`SELECT COALESCE(storage_backend, 'db'), object_key FROM file_blobs WHERE entry_id = $1`,
			entryID,
		).Scan(&storageBackend, &objectKey)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("filetree.deleteBlobTx: read blob metadata: %w", err)
		}
		if err == nil && storageBackend == objectstore.BackendCOS && objectKey.Valid && strings.TrimSpace(objectKey.String) != "" {
			if deleteErr := s.blobStore.Delete(ctx, objectKey.String); deleteErr != nil {
				return fmt.Errorf("filetree.deleteBlobTx: COS delete: %w", deleteErr)
			}
		}
	}
	_, err := tx.Exec(ctx, `DELETE FROM file_blobs WHERE entry_id = $1`, entryID)
	if err != nil {
		return fmt.Errorf("filetree.deleteBlobTx: %w", err)
	}
	return nil
}

type blobStorageInfo struct {
	Backend   string
	ObjectKey string
	SizeBytes int
	SHA256    string
}

func (b blobStorageInfo) Metadata() map[string]interface{} {
	metadata := map[string]interface{}{
		"binary":       true,
		"blob_storage": b.Backend,
		"size_bytes":   b.SizeBytes,
		"sha256":       b.SHA256,
	}
	if b.ObjectKey != "" {
		metadata["blob_object_key"] = b.ObjectKey
	}
	return metadata
}

func (s *FileTreeService) storeBlob(ctx context.Context, entryID, userID uuid.UUID, data []byte, contentType, sha256Hex string) (blobStorageInfo, error) {
	if sha256Hex == "" {
		_, sha256Hex = binaryMetadataWithHash(data)
	}
	info := blobStorageInfo{
		Backend:   objectstore.BackendDB,
		SizeBytes: len(data),
		SHA256:    sha256Hex,
	}
	if s.blobStore == nil || !s.blobStore.Enabled() {
		return info, nil
	}
	objectKey := s.blobStore.Key("users", userID.String(), "file-blobs", entryID.String(), sha256Hex)
	if err := s.blobStore.Put(ctx, objectKey, data, contentType); err != nil {
		return blobStorageInfo{}, fmt.Errorf("filetree.storeBlob: COS put: %w", err)
	}
	info.Backend = s.blobStore.Backend()
	info.ObjectKey = objectKey
	return info, nil
}

func binaryMetadata(data []byte) map[string]interface{} {
	metadata, _ := binaryMetadataWithHash(data)
	return metadata
}

func binaryMetadataWithHash(data []byte) (map[string]interface{}, string) {
	hash := sha256.Sum256(data)
	shaHex := hex.EncodeToString(hash[:])
	return map[string]interface{}{
		"binary":       true,
		"blob_storage": "db",
		"size_bytes":   len(data),
		"sha256":       shaHex,
	}, shaHex
}
