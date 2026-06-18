package api

import (
	"context"
	"errors"
	"time"

	"github.com/agi-bar/vola/internal/models"
	sqlitestorage "github.com/agi-bar/vola/internal/storage/sqlite"
	"github.com/google/uuid"
)

var errLocalPlatformImportBusy = errors.New("Vola is still saving local data. Please wait a few seconds and try again.")

func withLocalPlatformSQLiteRetry[T any](ctx context.Context, op func() (T, error)) (T, error) {
	var zero T
	delay := 90 * time.Millisecond
	for attempt := 0; attempt < 8; attempt++ {
		value, err := op()
		if err == nil {
			return value, nil
		}
		if !sqlitestorage.IsBusyLockedError(err) {
			return zero, err
		}
		if attempt == 7 {
			return zero, errLocalPlatformImportBusy
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
		if delay < 1200*time.Millisecond {
			delay *= 2
		}
	}
	return zero, errLocalPlatformImportBusy
}

func withLocalPlatformSQLiteRetryNoValue(ctx context.Context, op func() error) error {
	_, err := withLocalPlatformSQLiteRetry(ctx, func() (struct{}, error) {
		return struct{}{}, op()
	})
	return err
}

func (s *Server) withLocalPlatformImportLock(op func() error) error {
	s.localPlatformImportMu.Lock()
	defer s.localPlatformImportMu.Unlock()
	return op()
}

func (s *Server) retryLocalPlatformWriteEntry(ctx context.Context, userID uuid.UUID, path, content, contentType string, opts models.FileTreeWriteOptions) (*models.FileTreeEntry, error) {
	return withLocalPlatformSQLiteRetry(ctx, func() (*models.FileTreeEntry, error) {
		return s.FileTreeService.WriteEntry(ctx, userID, path, content, contentType, opts)
	})
}

func (s *Server) retryLocalPlatformWriteBinaryEntry(ctx context.Context, userID uuid.UUID, path string, data []byte, contentType string, opts models.FileTreeWriteOptions) (*models.FileTreeEntry, error) {
	return withLocalPlatformSQLiteRetry(ctx, func() (*models.FileTreeEntry, error) {
		return s.FileTreeService.WriteBinaryEntry(ctx, userID, path, data, contentType, opts)
	})
}

func (s *Server) retryLocalPlatformEnsureDirectoryWithMetadata(ctx context.Context, userID uuid.UUID, path string, metadata map[string]interface{}, minTrustLevel int) error {
	return withLocalPlatformSQLiteRetryNoValue(ctx, func() error {
		_, err := s.FileTreeService.EnsureDirectoryWithMetadata(ctx, userID, path, metadata, minTrustLevel)
		return err
	})
}

func (s *Server) retryLocalPlatformImportScratch(ctx context.Context, userID uuid.UUID, content, source, title string, createdAt time.Time, expiresAt *time.Time) (*models.FileTreeEntry, error) {
	return withLocalPlatformSQLiteRetry(ctx, func() (*models.FileTreeEntry, error) {
		return s.MemoryService.ImportScratch(ctx, userID, content, source, title, createdAt, expiresAt)
	})
}

func (s *Server) retryLocalPlatformUpsertProfile(ctx context.Context, userID uuid.UUID, category, content, source string) error {
	return withLocalPlatformSQLiteRetryNoValue(ctx, func() error {
		return s.MemoryService.UpsertProfile(ctx, userID, category, content, source)
	})
}

func (s *Server) retryLocalPlatformCreateProject(ctx context.Context, userID uuid.UUID, name string) (*models.Project, error) {
	return withLocalPlatformSQLiteRetry(ctx, func() (*models.Project, error) {
		return s.ProjectService.Create(ctx, userID, name)
	})
}

func (s *Server) retryLocalPlatformUpdateProjectContext(ctx context.Context, userID uuid.UUID, name, contextBody string) error {
	return withLocalPlatformSQLiteRetryNoValue(ctx, func() error {
		return s.ProjectService.UpdateContext(ctx, userID, name, contextBody)
	})
}
