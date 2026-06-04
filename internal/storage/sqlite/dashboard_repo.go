package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agi-bar/vola/internal/hubpath"
	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	"github.com/agi-bar/vola/internal/systemskills"
	"github.com/google/uuid"
)

type DashboardRepo struct {
	Store *Store
}

func NewDashboardRepo(store *Store) services.DashboardRepo {
	return &DashboardRepo{Store: store}
}

func (r *DashboardRepo) GetStats(ctx context.Context, userID uuid.UUID) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{
		WeeklyActivity: []models.DashboardActivity{},
		Pending:        []models.DashboardPending{},
	}
	db := r.Store.DB()
	if db == nil {
		return nil, fmt.Errorf("sqlite.DashboardRepo.GetStats: database not configured")
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM scoped_tokens
		  WHERE user_id = ? AND revoked_at IS NULL AND expires_at > ? AND name LIKE 'local platform %'`,
		userID.String(),
		now,
	).Scan(&stats.TotalConnections); err != nil {
		return nil, fmt.Errorf("sqlite.DashboardRepo.GetStats: connections count: %w", err)
	}

	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM file_tree WHERE user_id = ? AND is_directory = 0 AND deleted_at IS NULL`,
		userID.String(),
	).Scan(&stats.TotalFiles); err != nil {
		return nil, fmt.Errorf("sqlite.DashboardRepo.GetStats: files count: %w", err)
	}

	memoryPat := hubpath.NormalizeStorage("/memory/") + "%"
	profilePat := hubpath.NormalizeStorage("/memory/profile/") + "%"
	conversationPat := hubpath.NormalizeStorage("/conversations/") + "%"
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM file_tree
		  WHERE user_id = ? AND is_directory = 0 AND deleted_at IS NULL
		    AND path LIKE ?
		    AND path NOT LIKE ?`,
		userID.String(),
		memoryPat,
		profilePat,
	).Scan(&stats.TotalMemory); err != nil {
		return nil, fmt.Errorf("sqlite.DashboardRepo.GetStats: memory count: %w", err)
	}

	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM file_tree
		  WHERE user_id = ? AND is_directory = 0 AND deleted_at IS NULL
		    AND path LIKE ?`,
		userID.String(),
		profilePat,
	).Scan(&stats.TotalProfile); err != nil {
		return nil, fmt.Errorf("sqlite.DashboardRepo.GetStats: profile count: %w", err)
	}

	skillStoragePat := hubpath.NormalizeStorage("/skills/") + "%"
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM file_tree
		  WHERE user_id = ? AND is_directory = 0 AND deleted_at IS NULL
		    AND path LIKE ?
		    AND path LIKE '%/SKILL.md'`,
		userID.String(),
		skillStoragePat,
	).Scan(&stats.TotalSkills); err != nil {
		return nil, fmt.Errorf("sqlite.DashboardRepo.GetStats: skills count: %w", err)
	}
	stats.TotalSkills += len(systemskills.SkillSummaries())

	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM file_tree
		  WHERE user_id = ? AND is_directory = 1 AND deleted_at IS NULL
		    AND path LIKE ?
		    AND kind = ?`,
		userID.String(),
		conversationPat,
		services.EntryKindConversationBundle,
	).Scan(&stats.TotalConversations); err != nil {
		return nil, fmt.Errorf("sqlite.DashboardRepo.GetStats: conversations count: %w", err)
	}

	projects, err := NewProjectRepo(r.Store).ListProjects(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("sqlite.DashboardRepo.GetStats: projects count: %w", err)
	}
	stats.TotalProjects = len(projects)

	// Weekly activity for SQLite: count activity logs per day for the last 7 days.
	// Since sqlite time function is flexible, we group by day formatted YYYY-MM-DD.
	weeklyRows, err := db.QueryContext(ctx,
		`SELECT strftime('%Y-%m-%d', created_at) AS day, COUNT(*)
		 FROM activity_log
		 WHERE user_id = ? AND created_at >= datetime('now', '-7 days')
		 GROUP BY day ORDER BY day ASC`,
		userID.String(),
	)
	if err == nil {
		defer weeklyRows.Close()
		for weeklyRows.Next() {
			var activity models.DashboardActivity
			if err := weeklyRows.Scan(&activity.Platform, &activity.Count); err == nil {
				stats.WeeklyActivity = append(stats.WeeklyActivity, activity)
			}
		}
	}

	return stats, nil
}

func (r *DashboardRepo) LogActivity(ctx context.Context, id, userID uuid.UUID, connectionID *uuid.UUID, action, path string, metadata map[string]interface{}, createdAt time.Time) error {
	db := r.Store.DB()
	if db == nil {
		return fmt.Errorf("sqlite.DashboardRepo.LogActivity: database not configured")
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	var connIDStr *string
	if connectionID != nil {
		s := connectionID.String()
		connIDStr = &s
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO activity_log (id, user_id, connection_id, action, path, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.String(), userID.String(), connIDStr, action, path, string(metadataBytes), createdAt.Format(time.RFC3339Nano))
	return err
}

func (r *DashboardRepo) GetActivities(ctx context.Context, userID uuid.UUID, limit int) ([]models.ActivityLog, error) {
	db := r.Store.DB()
	if db == nil {
		return nil, fmt.Errorf("sqlite.DashboardRepo.GetActivities: database not configured")
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, user_id, connection_id, action, path, metadata, created_at
		 FROM activity_log
		 WHERE user_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		userID.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activities := make([]models.ActivityLog, 0)
	for rows.Next() {
		var idStr, uidStr string
		var connIDStr *string
		var action, path, metadataStr string
		var createdAtStr string

		err := rows.Scan(&idStr, &uidStr, &connIDStr, &action, &path, &metadataStr, &createdAtStr)
		if err != nil {
			return nil, err
		}

		id, _ := uuid.Parse(idStr)
		uid, _ := uuid.Parse(uidStr)
		createdAt, _ := time.Parse(time.RFC3339, createdAtStr)
		if createdAt.IsZero() {
			createdAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
		}
		if createdAt.IsZero() {
			createdAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
		}

		var connID uuid.UUID
		if connIDStr != nil {
			connID, _ = uuid.Parse(*connIDStr)
		}

		metadata := make(map[string]interface{})
		if len(metadataStr) > 0 {
			_ = json.Unmarshal([]byte(metadataStr), &metadata)
		}

		activities = append(activities, models.ActivityLog{
			ID:           id,
			UserID:       uid,
			ConnectionID: connID,
			Action:       action,
			Path:         path,
			Metadata:     metadata,
			CreatedAt:    createdAt,
		})
	}
	return activities, nil
}
