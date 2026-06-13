package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	pathpkg "path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/agi-bar/vola/internal/backups"
	"github.com/agi-bar/vola/internal/localgitsync"
	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/google/uuid"
)

// JobConfig controls whether a job is enabled and how often it runs.
type JobConfig struct {
	Enabled  bool
	Interval time.Duration
}

// SchedulerConfig holds the configuration for all background jobs.
type SchedulerConfig struct {
	CleanExpiredScratch        JobConfig
	CleanExpiredTokens         JobConfig
	CleanExpiredSync           JobConfig
	ArchiveExpiredMessages     JobConfig
	GenerateDailySkillLearning JobConfig
	RunQueuedGitMirrors        JobConfig
	RunExternalBackups         JobConfig
	CheckTeamSkillUpdates      JobConfig
}

// DefaultSchedulerConfig returns the default configuration for all jobs.
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		CleanExpiredScratch: JobConfig{
			Enabled:  true,
			Interval: 6 * time.Hour,
		},
		CleanExpiredTokens: JobConfig{
			Enabled:  true,
			Interval: 1 * time.Hour,
		},
		CleanExpiredSync: JobConfig{
			Enabled:  true,
			Interval: 1 * time.Hour,
		},
		ArchiveExpiredMessages: JobConfig{
			Enabled:  true,
			Interval: 1 * time.Hour,
		},
		GenerateDailySkillLearning: JobConfig{
			Enabled:  true,
			Interval: 24 * time.Hour,
		},
		RunQueuedGitMirrors: JobConfig{
			Enabled:  true,
			Interval: 15 * time.Second,
		},
		RunExternalBackups: JobConfig{
			Enabled:  true,
			Interval: 1 * time.Hour,
		},
		CheckTeamSkillUpdates: JobConfig{
			Enabled:  true,
			Interval: 1 * time.Hour,
		},
	}
}

// Scheduler manages periodic background jobs.
type Scheduler struct {
	memory        *services.MemoryService
	token         *services.TokenService
	user          *services.UserService
	inbox         *services.InboxService
	sync          *services.SyncService
	skillLearning *services.SkillLearningService
	gitMirror     *localgitsync.Service
	backup        *backups.Service
	fileTree      *services.FileTreeService
	team          *services.TeamService
	logger        *slog.Logger
	config        SchedulerConfig
	stop          chan struct{}
	wg            sync.WaitGroup
}

func NewScheduler(memory *services.MemoryService, token *services.TokenService, userSvc *services.UserService, inbox *services.InboxService, syncSvc *services.SyncService, skillLearningSvc *services.SkillLearningService, gitMirrorSvc *localgitsync.Service, backupSvc *backups.Service, fileTree *services.FileTreeService, teamSvc *services.TeamService, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		memory:        memory,
		token:         token,
		user:          userSvc,
		inbox:         inbox,
		sync:          syncSvc,
		skillLearning: skillLearningSvc,
		gitMirror:     gitMirrorSvc,
		backup:        backupSvc,
		fileTree:      fileTree,
		team:          teamSvc,
		logger:        logger,
		config:        DefaultSchedulerConfig(),
		stop:          make(chan struct{}),
	}
}

func NewSchedulerWithConfig(memory *services.MemoryService, token *services.TokenService, userSvc *services.UserService, inbox *services.InboxService, syncSvc *services.SyncService, skillLearningSvc *services.SkillLearningService, gitMirrorSvc *localgitsync.Service, backupSvc *backups.Service, fileTree *services.FileTreeService, teamSvc *services.TeamService, logger *slog.Logger, config SchedulerConfig) *Scheduler {
	return &Scheduler{
		memory:        memory,
		token:         token,
		user:          userSvc,
		inbox:         inbox,
		sync:          syncSvc,
		skillLearning: skillLearningSvc,
		gitMirror:     gitMirrorSvc,
		backup:        backupSvc,
		fileTree:      fileTree,
		team:          teamSvc,
		logger:        logger,
		config:        config,
		stop:          make(chan struct{}),
	}
}

// Start begins running all enabled periodic jobs in the background.
func (s *Scheduler) Start(ctx context.Context) {
	s.logger.Info("starting background job scheduler")

	if s.config.CleanExpiredScratch.Enabled && s.memory != nil {
		s.startJob(ctx, "CleanExpiredScratch", s.config.CleanExpiredScratch.Interval, s.cleanExpiredScratch)
	}
	if s.config.CleanExpiredTokens.Enabled && s.token != nil {
		s.startJob(ctx, "CleanExpiredTokens", s.config.CleanExpiredTokens.Interval, s.cleanExpiredTokens)
	}
	if s.config.CleanExpiredSync.Enabled && s.sync != nil {
		s.startJob(ctx, "CleanExpiredSyncSessions", s.config.CleanExpiredSync.Interval, s.cleanExpiredSyncSessions)
	}
	if s.config.ArchiveExpiredMessages.Enabled && s.inbox != nil {
		s.startJob(ctx, "ArchiveExpiredMessages", s.config.ArchiveExpiredMessages.Interval, s.archiveExpiredMessages)
	}
	if s.config.GenerateDailySkillLearning.Enabled && s.skillLearning != nil {
		s.startJob(ctx, "GenerateDailySkillLearning", s.config.GenerateDailySkillLearning.Interval, s.generateDailySkillLearning)
	}
	if s.config.RunQueuedGitMirrors.Enabled && s.gitMirror != nil {
		s.startJob(ctx, "RunQueuedGitMirrors", s.config.RunQueuedGitMirrors.Interval, s.runQueuedGitMirrors)
	}
	if s.config.RunExternalBackups.Enabled && s.backup != nil {
		s.startJob(ctx, "RunExternalBackups", s.config.RunExternalBackups.Interval, s.runExternalBackups)
	}
	if s.config.CheckTeamSkillUpdates.Enabled && s.user != nil && s.fileTree != nil && s.team != nil {
		s.startJob(ctx, "CheckTeamSkillUpdates", s.config.CheckTeamSkillUpdates.Interval, s.checkTeamSkillUpdates)
	}

	s.logger.Info("background job scheduler started")
}

// Stop gracefully stops all jobs and waits for them to finish.
func (s *Scheduler) Stop() {
	s.logger.Info("stopping background job scheduler")
	close(s.stop)
	s.wg.Wait()
	s.logger.Info("background job scheduler stopped")
}

// startJob launches a goroutine that runs the given function at the specified interval.
// It runs the job once immediately on startup, then on each tick.
func (s *Scheduler) startJob(ctx context.Context, name string, interval time.Duration, fn func(context.Context)) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("job registered", "job", name, "interval", interval.String())

		// Run once immediately at startup.
		fn(ctx)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fn(ctx)
			case <-s.stop:
				s.logger.Info("job stopping", "job", name)
				return
			case <-ctx.Done():
				s.logger.Info("job stopping (context cancelled)", "job", name)
				return
			}
		}
	}()
}

func (s *Scheduler) cleanExpiredScratch(ctx context.Context) {
	name := "CleanExpiredScratch"
	start := time.Now()
	s.logger.Info("job started", "job", name)

	count, err := s.memory.CleanExpiredScratch(ctx)
	duration := time.Since(start)

	if err != nil {
		s.logger.Error("job failed", "job", name, "error", err, "duration", duration.String())
		return
	}

	s.logger.Info("job completed", "job", name, "affected", count, "duration", duration.String())
}

func (s *Scheduler) cleanExpiredTokens(ctx context.Context) {
	name := "CleanExpiredTokens"
	start := time.Now()
	s.logger.Info("job started", "job", name)

	count, err := s.token.DeactivateExpiredTokens(ctx)
	duration := time.Since(start)

	if err != nil {
		s.logger.Error("job failed", "job", name, "error", err, "duration", duration.String())
		return
	}

	s.logger.Info("job completed", "job", name, "affected", count, "duration", duration.String())
}

func (s *Scheduler) cleanExpiredSyncSessions(ctx context.Context) {
	name := "CleanExpiredSyncSessions"
	start := time.Now()
	s.logger.Info("job started", "job", name)

	if s.sync == nil {
		s.logger.Info("job skipped", "job", name, "reason", "sync service not configured")
		return
	}

	result, err := s.sync.CleanExpiredSessions(ctx)
	duration := time.Since(start)

	if err != nil {
		s.logger.Error("job failed", "job", name, "error", err, "duration", duration.String())
		return
	}

	s.logger.Info(
		"job completed",
		"job", name,
		"expired_sessions", result.ExpiredSessions,
		"deleted_parts", result.DeletedParts,
		"deleted_bytes", result.DeletedBytes,
		"duration", duration.String(),
	)
}

func (s *Scheduler) archiveExpiredMessages(ctx context.Context) {
	name := "ArchiveExpiredMessages"
	start := time.Now()
	s.logger.Info("job started", "job", name)

	count, err := s.inbox.ArchiveExpiredMessages(ctx)
	duration := time.Since(start)

	if err != nil {
		s.logger.Error("job failed", "job", name, "error", err, "duration", duration.String())
		return
	}

	s.logger.Info("job completed", "job", name, "affected", count, "duration", duration.String())
}

func (s *Scheduler) generateDailySkillLearning(ctx context.Context) {
	name := "GenerateDailySkillLearning"
	start := time.Now()
	s.logger.Info("job started", "job", name)

	if s.skillLearning == nil {
		s.logger.Info("job skipped", "job", name, "reason", "skill learning service not configured")
		return
	}
	if s.user == nil {
		s.logger.Info("job skipped", "job", name, "reason", "user service not configured")
		return
	}
	accounts, err := s.user.ListAccounts(ctx, 0)
	if err != nil {
		s.logger.Error("job failed", "job", name, "error", err, "duration", time.Since(start).String())
		return
	}
	var count int64
	for _, account := range accounts {
		if _, _, err := s.skillLearning.WriteDailyNote(ctx, account.ID, 4); err == nil {
			count++
		}
	}
	duration := time.Since(start)
	s.logger.Info("job completed", "job", name, "affected", count, "duration", duration.String())
}

func (s *Scheduler) runQueuedGitMirrors(ctx context.Context) {
	name := "RunQueuedGitMirrors"
	start := time.Now()
	s.logger.Info("job started", "job", name)

	if s.gitMirror == nil {
		s.logger.Info("job skipped", "job", name, "reason", "git mirror service not configured")
		return
	}
	if err := s.gitMirror.RunQueuedGitMirrorSyncs(ctx, 20); err != nil {
		s.logger.Error("job failed", "job", name, "error", err, "duration", time.Since(start).String())
		return
	}
	s.logger.Info("job completed", "job", name, "duration", time.Since(start).String())
}

func (s *Scheduler) runExternalBackups(ctx context.Context) {
	name := "RunExternalBackups"
	start := time.Now()
	s.logger.Info("job started", "job", name)

	if s.backup == nil {
		s.logger.Info("job skipped", "job", name, "reason", "backup service not configured")
		return
	}
	result, err := s.backup.RunDueTargets(ctx, time.Now().UTC(), 20)
	duration := time.Since(start)
	if err != nil {
		s.logger.Error("job failed", "job", name, "error", err, "duration", duration.String())
		return
	}
	if result.Failed > 0 {
		s.logger.Warn(
			"job completed with backup errors",
			"job", name,
			"checked", result.Checked,
			"due", result.Due,
			"succeeded", result.Succeeded,
			"failed", result.Failed,
			"skipped", result.Skipped,
			"duration", duration.String(),
		)
		return
	}
	s.logger.Info(
		"job completed",
		"job", name,
		"checked", result.Checked,
		"due", result.Due,
		"succeeded", result.Succeeded,
		"skipped", result.Skipped,
		"duration", duration.String(),
	)
}

func (s *Scheduler) checkTeamSkillUpdates(ctx context.Context) {
	name := "CheckTeamSkillUpdates"
	start := time.Now()
	s.logger.Info("job started", "job", name)

	accounts, err := s.user.ListAccounts(ctx, 0)
	if err != nil {
		s.logger.Error("job failed to list accounts", "job", name, "error", err)
		return
	}

	var count int
	for _, acc := range accounts {
		userID := acc.ID
		subEntry, err := s.fileTree.Read(ctx, userID, "/settings/team-skill-subscriptions.json", models.TrustLevelFull)
		if err != nil {
			continue
		}
		var subDoc struct {
			Version       string `json:"version"`
			Subscriptions []struct {
				TeamID            string `json:"team_id"`
				TeamSlug          string `json:"team_slug,omitempty"`
				SourcePath        string `json:"source_path"`
				TargetPath        string `json:"target_path"`
				SourceFingerprint string `json:"source_fingerprint,omitempty"`
			} `json:"subscriptions"`
		}
		if err := json.Unmarshal([]byte(subEntry.Content), &subDoc); err != nil {
			continue
		}

		var updatedNotifications []map[string]interface{}

		for _, sub := range subDoc.Subscriptions {
			var team *models.Team
			if teamID, err := uuid.Parse(sub.TeamID); err == nil {
				team, _ = s.team.GetForUser(ctx, userID, teamID)
			}
			if team == nil && sub.TeamSlug != "" {
				team, _ = s.team.GetBySlugForUser(ctx, userID, sub.TeamSlug)
			}
			if team == nil {
				continue
			}

			currentFingerprint, err := s.computeTeamSkillFingerprint(ctx, team.HubUserID, sub.SourcePath)
			if err != nil {
				continue
			}

			if sub.SourceFingerprint != "" && currentFingerprint != "" && sub.SourceFingerprint != currentFingerprint {
				count++
				updatedNotifications = append(updatedNotifications, map[string]interface{}{
					"team_id":     team.ID.String(),
					"team_slug":   team.Slug,
					"skill_path":  sub.SourcePath,
					"target_path": sub.TargetPath,
					"status":      "update_available",
					"updated_at":  time.Now().UTC().Format(time.RFC3339),
				})

				threadID := "update-skill-" + team.ID.String() + "-" + sub.SourcePath
				existingMsgs, _ := s.inbox.GetMessages(ctx, userID, "user", "incoming")
				alreadySent := false
				for _, msg := range existingMsgs {
					if msg.ThreadID == threadID {
						alreadySent = true
						break
					}
				}
				if !alreadySent {
					_, _ = s.inbox.Send(ctx, userID, models.InboxMessage{
						ID:             uuid.New(),
						FromAddress:    "system",
						ToAddress:      userID.String(),
						ThreadID:       threadID,
						Priority:       "normal",
						ActionRequired: true,
						Domain:         "skills",
						Subject:        "团队共享技能有新版本: " + pathpkg.Base(sub.SourcePath),
						Body:           fmt.Sprintf("您的团队 %s 发布了 %s 的最新修改版本，请前往团队资料库进行一键更新。", team.Name, pathpkg.Base(sub.SourcePath)),
						Status:         "incoming",
						CreatedAt:      time.Now(),
					})
				}
			}
		}

		if len(updatedNotifications) > 0 {
			notifDoc := map[string]interface{}{
				"version":       "vola.personal-update-notifications/v1",
				"notifications": updatedNotifications,
				"updated_at":    time.Now().UTC().Format(time.RFC3339),
			}
			notifData, _ := json.MarshalIndent(notifDoc, "", "  ")
			_, _ = s.fileTree.WriteEntry(ctx, userID, "/settings/personal-update-notifications.json", string(append(notifData, '\n')), "application/json", models.FileTreeWriteOptions{
				Kind:          "personal_update_notifications",
				MinTrustLevel: models.TrustLevelFull,
			})
		} else {
			_ = s.fileTree.Delete(ctx, userID, "/settings/personal-update-notifications.json")
		}
	}

	duration := time.Since(start)
	s.logger.Info("job completed", "job", name, "detected_updates", count, "duration", duration.String())
}

func (s *Scheduler) computeTeamSkillFingerprint(ctx context.Context, teamHubUserID uuid.UUID, skillPath string) (string, error) {
	snapshot, err := s.fileTree.Snapshot(ctx, teamHubUserID, skillPath, models.TrustLevelFull)
	if err != nil {
		return "", err
	}
	type skillFile struct {
		relPath string
		data    []byte
	}
	var files []skillFile
	for _, entry := range snapshot.Entries {
		if entry.IsDirectory || entry.DeletedAt != nil {
			continue
		}
		rel := strings.TrimPrefix(entry.Path, skillPath)
		rel = strings.TrimPrefix(rel, "/")
		files = append(files, skillFile{
			relPath: rel,
			data:    []byte(entry.Content),
		})
	}
	if len(files) == 0 {
		return "", nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].relPath < files[j].relPath })
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.relPath))
		h.Write([]byte{0})
		h.Write(f.data)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
