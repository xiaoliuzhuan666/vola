package backups

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/google/uuid"
)

const backupSecretScopePrefix = "backup-target-"
const defaultAutoBackupIntervalHours = 24
const maxRetentionKeepLast = 365
const maxRetentionKeepDays = 3650

type Service struct {
	repo      Repo
	exportSvc *services.ExportService
	vaultSvc  *services.VaultService
	client    *http.Client
}

func NewService(repo Repo, exportSvc *services.ExportService, vaultSvc *services.VaultService) *Service {
	if repo == nil || exportSvc == nil {
		return nil
	}
	return &Service{
		repo:      repo,
		exportSvc: exportSvc,
		vaultSvc:  vaultSvc,
		client:    http.DefaultClient,
	}
}

func (s *Service) ListTargets(ctx context.Context, userID uuid.UUID) ([]Target, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("backup targets service not configured")
	}
	targets, err := s.repo.ListBackupTargets(ctx, userID)
	if err != nil {
		return nil, err
	}
	if targets == nil {
		targets = []Target{}
	}
	return targets, nil
}

func (s *Service) SaveTarget(ctx context.Context, userID uuid.UUID, req SaveTargetRequest) (*Target, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("backup targets service not configured")
	}
	targetID, existing, err := s.targetForSave(ctx, userID, req.ID)
	if err != nil {
		return nil, err
	}
	target := Target{
		ID:                      targetID,
		UserID:                  userID,
		Kind:                    normalizeKind(req.Kind),
		Name:                    strings.TrimSpace(req.Name),
		Enabled:                 req.Enabled,
		WebDAVURL:               strings.TrimSpace(req.WebDAVURL),
		WebDAVUsername:          strings.TrimSpace(req.WebDAVUsername),
		S3Endpoint:              strings.TrimSpace(req.S3Endpoint),
		S3Bucket:                strings.TrimSpace(req.S3Bucket),
		S3Region:                strings.TrimSpace(req.S3Region),
		S3Prefix:                cleanPrefix(req.S3Prefix),
		S3AccessKeyID:           strings.TrimSpace(req.S3AccessKeyID),
		S3PathStyle:             req.S3PathStyle,
		SecretConfigured:        existing != nil && existing.SecretConfigured,
		AutoBackupEnabled:       req.AutoBackupEnabled,
		AutoBackupIntervalHours: normalizeAutoBackupInterval(req.AutoBackupIntervalHours),
		RetentionKeepLast:       normalizeRetentionKeepLast(req.RetentionKeepLast),
		RetentionKeepDays:       normalizeRetentionKeepDays(req.RetentionKeepDays),
	}
	if target.Name == "" {
		target.Name = defaultNameForKind(target.Kind)
	}
	if existing != nil {
		target.LastAutoBackupAt = existing.LastAutoBackupAt
		target.LastBackupAt = existing.LastBackupAt
		target.LastBackupObject = existing.LastBackupObject
		target.LastBackupError = existing.LastBackupError
		target.CreatedAt = existing.CreatedAt
	}
	if err := validateTarget(target); err != nil {
		return nil, err
	}
	secret := targetSecret{
		WebDAVPassword:    req.WebDAVPassword,
		S3SecretAccessKey: req.S3SecretAccessKey,
	}
	if hasSecretValue(secret) {
		if err := s.writeSecret(ctx, userID, targetID, secret); err != nil {
			return nil, err
		}
		target.SecretConfigured = true
	}
	if target.Kind == KindS3 && !target.SecretConfigured {
		return nil, fmt.Errorf("S3-compatible backup requires a secret access key")
	}
	if err := s.repo.UpsertBackupTarget(ctx, target); err != nil {
		return nil, err
	}
	saved, err := s.repo.GetBackupTarget(ctx, userID, targetID)
	if err != nil {
		return nil, err
	}
	if saved == nil {
		return &target, nil
	}
	return saved, nil
}

func (s *Service) RunTarget(ctx context.Context, userID, targetID uuid.UUID) (*RunResult, error) {
	return s.RunTargetWithTrigger(ctx, userID, targetID, RunTriggerManual)
}

func (s *Service) RunTargetWithTrigger(ctx context.Context, userID, targetID uuid.UUID, trigger string) (*RunResult, error) {
	if s == nil || s.repo == nil || s.exportSvc == nil {
		return nil, fmt.Errorf("backup targets service not configured")
	}
	target, err := s.repo.GetBackupTarget(ctx, userID, targetID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("backup target not found")
	}
	if !target.Enabled {
		return nil, fmt.Errorf("backup target is disabled")
	}
	if err := validateTarget(*target); err != nil {
		return nil, err
	}
	trigger = normalizeRunTrigger(trigger)
	startedAt := time.Now().UTC()
	run := Run{
		ID:         uuid.New(),
		UserID:     userID,
		TargetID:   target.ID,
		TargetName: target.Name,
		TargetKind: target.Kind,
		Trigger:    trigger,
		Status:     RunStatusFailed,
		StartedAt:  startedAt,
		CreatedAt:  startedAt,
	}
	finishFailure := func(objectName string, runErr error) error {
		completedAt := time.Now().UTC()
		run.ObjectName = objectName
		run.CompletedAt = &completedAt
		run.DurationMs = completedAt.Sub(startedAt).Milliseconds()
		run.Error = runErr.Error()
		_ = s.repo.InsertBackupRun(ctx, run)
		_ = s.repo.UpdateBackupTargetResult(ctx, userID, target.ID, nil, objectName, runErr.Error(), trigger == RunTriggerAuto)
		return runErr
	}
	secret, err := s.readSecret(ctx, userID, target.ID)
	if err != nil {
		return nil, finishFailure(target.LastBackupObject, err)
	}

	var buf bytes.Buffer
	if err := s.exportSvc.ExportToZip(ctx, userID, &buf); err != nil {
		return nil, finishFailure(target.LastBackupObject, err)
	}
	objectName := targetObjectName(*target, time.Now().UTC())
	location, err := s.upload(ctx, *target, secret, objectName, buf.Bytes())
	if err != nil {
		return nil, finishFailure(objectName, err)
	}
	completedAt := time.Now().UTC()
	if err := s.repo.UpdateBackupTargetResult(ctx, userID, target.ID, &completedAt, objectName, "", trigger == RunTriggerAuto); err != nil {
		return nil, err
	}
	run.Status = RunStatusSuccess
	run.ObjectName = objectName
	run.Location = location
	run.SizeBytes = int64(buf.Len())
	run.CompletedAt = &completedAt
	run.DurationMs = completedAt.Sub(startedAt).Milliseconds()
	if err := s.repo.InsertBackupRun(ctx, run); err != nil {
		return nil, err
	}
	if err := s.applyRetention(ctx, *target, secret, objectName, completedAt); err != nil {
		run.RemoteDeleteError = err.Error()
	}
	target.LastBackupAt = &completedAt
	if trigger == RunTriggerAuto {
		target.LastAutoBackupAt = &completedAt
	}
	target.LastBackupObject = objectName
	target.LastBackupError = ""
	return &RunResult{
		Target:      *target,
		Run:         run,
		ObjectName:  objectName,
		Location:    location,
		SizeBytes:   int64(buf.Len()),
		CompletedAt: completedAt.Format(time.RFC3339),
		Message:     "External backup uploaded.",
	}, nil
}

func (s *Service) RunDueTargets(ctx context.Context, now time.Time, limit int) (AutomationResult, error) {
	result := AutomationResult{}
	if s == nil || s.repo == nil {
		return result, fmt.Errorf("backup targets service not configured")
	}
	if limit <= 0 {
		limit = 20
	}
	targets, err := s.repo.ListAutoBackupTargets(ctx)
	if err != nil {
		return result, err
	}
	result.Checked = len(targets)
	now = now.UTC()
	for _, target := range targets {
		if !target.Enabled || !target.AutoBackupEnabled {
			result.Skipped++
			continue
		}
		if target.LastBackupAt != nil && target.LastBackupAt.UTC().Add(autoBackupInterval(target)).After(now) {
			result.Skipped++
			continue
		}
		if result.Due >= limit {
			result.Skipped++
			continue
		}
		result.Due++
		if _, err := s.RunTargetWithTrigger(ctx, target.UserID, target.ID, RunTriggerAuto); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", target.Name, err))
			continue
		}
		result.Succeeded++
	}
	return result, nil
}

func (s *Service) targetForSave(ctx context.Context, userID uuid.UUID, rawID string) (uuid.UUID, *Target, error) {
	if strings.TrimSpace(rawID) == "" {
		return uuid.New(), nil, nil
	}
	targetID, err := uuid.Parse(strings.TrimSpace(rawID))
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("invalid backup target id")
	}
	existing, err := s.repo.GetBackupTarget(ctx, userID, targetID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	if existing == nil {
		return uuid.Nil, nil, fmt.Errorf("backup target not found")
	}
	return targetID, existing, nil
}

func (s *Service) upload(ctx context.Context, target Target, secret targetSecret, objectName string, data []byte) (string, error) {
	switch target.Kind {
	case KindWebDAV:
		return s.uploadWebDAV(ctx, target, secret, objectName, data)
	case KindS3:
		return s.uploadS3(ctx, target, secret, objectName, data)
	default:
		return "", fmt.Errorf("unsupported backup target kind %q", target.Kind)
	}
}

func (s *Service) deleteRemoteObject(ctx context.Context, target Target, secret targetSecret, objectName string) error {
	if !isNeuDriveBackupObject(objectName) {
		return fmt.Errorf("refusing to delete non-Vola backup object %q", objectName)
	}
	switch target.Kind {
	case KindWebDAV:
		return s.deleteWebDAV(ctx, target, secret, objectName)
	case KindS3:
		return s.deleteS3(ctx, target, secret, objectName)
	default:
		return fmt.Errorf("unsupported backup target kind %q", target.Kind)
	}
}

func (s *Service) writeSecret(ctx context.Context, userID, targetID uuid.UUID, secret targetSecret) error {
	if s.vaultSvc == nil {
		return fmt.Errorf("vault service not configured for backup credentials")
	}
	payload, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	return s.vaultSvc.Write(ctx, userID, secretScope(targetID), string(payload), "External backup target credentials", models.TrustLevelFull)
}

func (s *Service) readSecret(ctx context.Context, userID, targetID uuid.UUID) (targetSecret, error) {
	var secret targetSecret
	if s.vaultSvc == nil {
		return secret, nil
	}
	raw, err := s.vaultSvc.Read(ctx, userID, secretScope(targetID), models.TrustLevelFull)
	if err != nil {
		return secret, nil
	}
	_ = json.Unmarshal([]byte(raw), &secret)
	return secret, nil
}

func secretScope(targetID uuid.UUID) string {
	return backupSecretScopePrefix + targetID.String()
}

func normalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindWebDAV:
		return KindWebDAV
	case KindS3, "s3-compatible", "s3_compatible":
		return KindS3
	default:
		return ""
	}
}

func defaultNameForKind(kind string) string {
	switch kind {
	case KindWebDAV:
		return "WebDAV backup"
	case KindS3:
		return "S3-compatible backup"
	default:
		return "External backup"
	}
}

func validateTarget(target Target) error {
	switch target.Kind {
	case KindWebDAV:
		if err := validateHTTPURL(target.WebDAVURL, "WebDAV URL"); err != nil {
			return err
		}
	case KindS3:
		if err := validateHTTPURL(target.S3Endpoint, "S3 endpoint"); err != nil {
			return err
		}
		if strings.TrimSpace(target.S3Bucket) == "" {
			return fmt.Errorf("S3 bucket is required")
		}
		if strings.TrimSpace(target.S3AccessKeyID) == "" {
			return fmt.Errorf("S3 access key ID is required")
		}
	default:
		return fmt.Errorf("backup target kind must be webdav or s3")
	}
	return nil
}

func validateHTTPURL(value, label string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid URL", label)
	}
	switch parsed.Scheme {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("%s must start with http:// or https://", label)
	}
}

func hasSecretValue(secret targetSecret) bool {
	return strings.TrimSpace(secret.WebDAVPassword) != "" || strings.TrimSpace(secret.S3SecretAccessKey) != ""
}

func cleanPrefix(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	parts := strings.Split(value, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, "/")
}

func normalizeAutoBackupInterval(hours int) int {
	if hours <= 0 {
		return defaultAutoBackupIntervalHours
	}
	if hours > 24*30 {
		return 24 * 30
	}
	return hours
}

func normalizeRetentionKeepLast(value int) int {
	if value < 0 {
		return 0
	}
	if value > maxRetentionKeepLast {
		return maxRetentionKeepLast
	}
	return value
}

func normalizeRetentionKeepDays(value int) int {
	if value < 0 {
		return 0
	}
	if value > maxRetentionKeepDays {
		return maxRetentionKeepDays
	}
	return value
}

func normalizeRunTrigger(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case RunTriggerAuto:
		return RunTriggerAuto
	default:
		return RunTriggerManual
	}
}

func autoBackupInterval(target Target) time.Duration {
	return time.Duration(normalizeAutoBackupInterval(target.AutoBackupIntervalHours)) * time.Hour
}

func targetObjectName(target Target, now time.Time) string {
	filename := "vola-export-" + now.UTC().Format("20060102-150405Z") + ".zip"
	if target.Kind == KindS3 && strings.TrimSpace(target.S3Prefix) != "" {
		return path.Join(target.S3Prefix, filename)
	}
	return filename
}

func (s *Service) ListRuns(ctx context.Context, userID uuid.UUID, targetID *uuid.UUID, limit int) ([]Run, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("backup targets service not configured")
	}
	return s.repo.ListBackupRuns(ctx, userID, targetID, limit)
}

func (s *Service) applyRetention(ctx context.Context, target Target, secret targetSecret, currentObject string, now time.Time) error {
	keepLast := normalizeRetentionKeepLast(target.RetentionKeepLast)
	keepDays := normalizeRetentionKeepDays(target.RetentionKeepDays)
	if keepLast == 0 && keepDays == 0 {
		return nil
	}
	runs, err := s.repo.ListPrunableBackupRuns(ctx, target.UserID, target.ID, keepLast, keepDays, now)
	if err != nil {
		return err
	}
	var errs []string
	for _, run := range runs {
		if run.ObjectName == "" || run.ObjectName == currentObject || !isNeuDriveBackupObject(run.ObjectName) {
			continue
		}
		deletedAt := time.Now().UTC()
		if err := s.deleteRemoteObject(ctx, target, secret, run.ObjectName); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", run.ObjectName, err))
			_ = s.repo.UpdateBackupRunRemoteDelete(ctx, target.UserID, run.ID, nil, err.Error())
			continue
		}
		_ = s.repo.UpdateBackupRunRemoteDelete(ctx, target.UserID, run.ID, &deletedAt, "")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func isNeuDriveBackupObject(objectName string) bool {
	base := path.Base(strings.TrimSpace(objectName))
	return strings.HasPrefix(base, "vola-export-") && strings.HasSuffix(base, ".zip")
}
