package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/agi-bar/neudrive/internal/models"
	"github.com/agi-bar/neudrive/internal/services"
	"github.com/google/uuid"
)

type TeamRepo struct {
	Store *Store
}

func NewTeamRepo(store *Store) services.TeamRepo {
	return &TeamRepo{Store: store}
}

func (r *TeamRepo) CreateTeam(ctx context.Context, creatorUserID uuid.UUID, req models.CreateTeamRequest) (*models.Team, error) {
	tx, err := r.Store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.CreateTeam: begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	teamID := uuid.New()
	hubUserID := uuid.New()
	var quotaArg interface{}
	if req.StorageQuotaBytes != nil {
		quotaArg = *req.StorageQuotaBytes
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO users (id, slug, display_name, account_type, email, avatar_url, bio, timezone, language, storage_quota_bytes, created_at, updated_at)
		 VALUES (?, ?, ?, 'team_hub', '', '', '', 'UTC', 'zh-CN', ?, ?, ?)`,
		hubUserID.String(), "team-"+teamID.String(), req.Name, quotaArg, timeText(now), timeText(now))
	if err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.CreateTeam: insert hub user: %w", err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO teams (id, slug, name, description, hub_user_id, created_by_user_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		teamID.String(), req.Slug, req.Name, req.Description, hubUserID.String(), creatorUserID.String(), timeText(now), timeText(now))
	if err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.CreateTeam: insert team: %w", err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO team_members (team_id, user_id, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		teamID.String(), creatorUserID.String(), models.TeamRoleOwner, timeText(now), timeText(now))
	if err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.CreateTeam: insert owner: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.CreateTeam: commit: %w", err)
	}
	return r.GetTeamForUser(ctx, creatorUserID, teamID)
}

func (r *TeamRepo) ListTeamsForUser(ctx context.Context, userID uuid.UUID) ([]models.Team, error) {
	rows, err := r.Store.DB().QueryContext(ctx, sqliteTeamSelectSQL(`
		WHERE tm.user_id = ?
		GROUP BY t.id, t.slug, t.name, t.description, t.hub_user_id, t.created_by_user_id, tm.role, uh.storage_quota_bytes, t.created_at, t.updated_at
		ORDER BY t.created_at DESC`), userID.String())
	if err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.ListTeamsForUser: %w", err)
	}
	defer rows.Close()
	teams := []models.Team{}
	for rows.Next() {
		team, scanErr := scanSQLiteTeam(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("sqlite.TeamRepo.ListTeamsForUser: scan: %w", scanErr)
		}
		teams = append(teams, *team)
	}
	return teams, rows.Err()
}

func (r *TeamRepo) GetTeamForUser(ctx context.Context, userID uuid.UUID, teamID uuid.UUID) (*models.Team, error) {
	row := r.Store.DB().QueryRowContext(ctx, sqliteTeamSelectSQL(`
		WHERE t.id = ? AND tm.user_id = ?
		GROUP BY t.id, t.slug, t.name, t.description, t.hub_user_id, t.created_by_user_id, tm.role, uh.storage_quota_bytes, t.created_at, t.updated_at`),
		teamID.String(), userID.String())
	return scanSQLiteTeam(row)
}

func (r *TeamRepo) GetTeamBySlugForUser(ctx context.Context, userID uuid.UUID, slug string) (*models.Team, error) {
	row := r.Store.DB().QueryRowContext(ctx, sqliteTeamSelectSQL(`
		WHERE t.slug = ? AND tm.user_id = ?
		GROUP BY t.id, t.slug, t.name, t.description, t.hub_user_id, t.created_by_user_id, tm.role, uh.storage_quota_bytes, t.created_at, t.updated_at`),
		strings.TrimSpace(strings.ToLower(slug)), userID.String())
	return scanSQLiteTeam(row)
}

func (r *TeamRepo) UpdateTeam(ctx context.Context, userID, teamID uuid.UUID, req models.UpdateTeamRequest) (*models.Team, error) {
	team, err := r.GetTeamForUser(ctx, userID, teamID)
	if err != nil {
		return nil, err
	}
	if !models.TeamRoleCanManageMembers(team.Role) {
		return nil, fmt.Errorf("sqlite.TeamRepo.UpdateTeam: user cannot manage team")
	}
	tx, err := r.Store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.UpdateTeam: begin tx: %w", err)
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx,
		`UPDATE teams SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
		req.Name, req.Description, timeText(now), teamID.String())
	if err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.UpdateTeam: update team: %w", err)
	}
	var quotaArg interface{}
	if req.StorageQuotaBytes != nil {
		quotaArg = *req.StorageQuotaBytes
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE users SET storage_quota_bytes = ?, updated_at = ? WHERE id = ?`,
		quotaArg, timeText(now), team.HubUserID.String())
	if err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.UpdateTeam: update hub user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.UpdateTeam: commit: %w", err)
	}
	return r.GetTeamForUser(ctx, userID, teamID)
}

func (r *TeamRepo) ListMembers(ctx context.Context, teamID uuid.UUID) ([]models.TeamMember, error) {
	rows, err := r.Store.DB().QueryContext(ctx,
		`SELECT tm.team_id, tm.user_id, u.slug, u.display_name, u.email, tm.role, tm.created_at, tm.updated_at
		   FROM team_members tm
		   JOIN users u ON u.id = tm.user_id
		  WHERE tm.team_id = ?
		  ORDER BY CASE tm.role WHEN 'owner' THEN 1 WHEN 'admin' THEN 2 WHEN 'member' THEN 3 ELSE 4 END, u.slug`,
		teamID.String())
	if err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.ListMembers: %w", err)
	}
	defer rows.Close()
	members := []models.TeamMember{}
	for rows.Next() {
		member, scanErr := scanSQLiteTeamMember(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("sqlite.TeamRepo.ListMembers: scan: %w", scanErr)
		}
		members = append(members, *member)
	}
	return members, rows.Err()
}

func (r *TeamRepo) AddMember(ctx context.Context, teamID, userID uuid.UUID, role string) (*models.TeamMember, error) {
	now := time.Now().UTC()
	_, err := r.Store.DB().ExecContext(ctx,
		`INSERT INTO team_members (team_id, user_id, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(team_id, user_id) DO UPDATE SET role = excluded.role, updated_at = excluded.updated_at`,
		teamID.String(), userID.String(), role, timeText(now), timeText(now))
	if err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.AddMember: %w", err)
	}
	return r.getMember(ctx, teamID, userID)
}

func (r *TeamRepo) UpdateMemberRole(ctx context.Context, teamID, userID uuid.UUID, role string) (*models.TeamMember, error) {
	if role != models.TeamRoleOwner {
		if err := r.ensureOwnerWouldRemain(ctx, teamID, userID); err != nil {
			return nil, err
		}
	}
	result, err := r.Store.DB().ExecContext(ctx,
		`UPDATE team_members SET role = ?, updated_at = ? WHERE team_id = ? AND user_id = ?`,
		role, timeText(time.Now().UTC()), teamID.String(), userID.String())
	if err != nil {
		return nil, fmt.Errorf("sqlite.TeamRepo.UpdateMemberRole: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, fmt.Errorf("sqlite.TeamRepo.UpdateMemberRole: member not found")
	}
	return r.getMember(ctx, teamID, userID)
}

func (r *TeamRepo) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	if err := r.ensureOwnerWouldRemain(ctx, teamID, userID); err != nil {
		return err
	}
	result, err := r.Store.DB().ExecContext(ctx,
		`DELETE FROM team_members WHERE team_id = ? AND user_id = ?`,
		teamID.String(), userID.String())
	if err != nil {
		return fmt.Errorf("sqlite.TeamRepo.RemoveMember: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("sqlite.TeamRepo.RemoveMember: member not found")
	}
	return nil
}

func (r *TeamRepo) getMember(ctx context.Context, teamID, userID uuid.UUID) (*models.TeamMember, error) {
	row := r.Store.DB().QueryRowContext(ctx,
		`SELECT tm.team_id, tm.user_id, u.slug, u.display_name, u.email, tm.role, tm.created_at, tm.updated_at
		   FROM team_members tm
		   JOIN users u ON u.id = tm.user_id
		  WHERE tm.team_id = ? AND tm.user_id = ?`,
		teamID.String(), userID.String())
	return scanSQLiteTeamMember(row)
}

func (r *TeamRepo) ensureOwnerWouldRemain(ctx context.Context, teamID, userID uuid.UUID) error {
	var role string
	err := r.Store.DB().QueryRowContext(ctx,
		`SELECT role FROM team_members WHERE team_id = ? AND user_id = ?`,
		teamID.String(), userID.String()).Scan(&role)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("sqlite.TeamRepo.ensureOwnerWouldRemain: %w", err)
	}
	if role != models.TeamRoleOwner {
		return nil
	}
	var ownerCount int
	if err := r.Store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM team_members WHERE team_id = ? AND role = 'owner'`,
		teamID.String()).Scan(&ownerCount); err != nil {
		return fmt.Errorf("sqlite.TeamRepo.ensureOwnerWouldRemain: count owners: %w", err)
	}
	if ownerCount <= 1 {
		return fmt.Errorf("sqlite.TeamRepo.ensureOwnerWouldRemain: team must keep at least one owner")
	}
	return nil
}

func sqliteTeamSelectSQL(suffix string) string {
	return `SELECT t.id,
	              t.slug,
	              t.name,
	              t.description,
	              t.hub_user_id,
	              t.created_by_user_id,
	              tm.role,
	              uh.storage_quota_bytes,
	              COALESCE(SUM(
		              CASE
			              WHEN ft.is_directory = 1 THEN 0
			              WHEN fb.entry_id IS NOT NULL THEN fb.size_bytes
			              ELSE length(CAST(COALESCE(ft.content, '') AS BLOB))
		              END
	              ), 0) AS used_bytes,
	              t.created_at,
	              t.updated_at
	         FROM teams t
	         JOIN users uh ON uh.id = t.hub_user_id
	         JOIN team_members tm ON tm.team_id = t.id
	         LEFT JOIN file_tree ft ON ft.user_id = t.hub_user_id AND ft.deleted_at IS NULL
	         LEFT JOIN file_blobs fb ON fb.entry_id = ft.id
	` + suffix
}

type sqliteTeamScanner interface {
	Scan(dest ...interface{}) error
}

func scanSQLiteTeam(row sqliteTeamScanner) (*models.Team, error) {
	var (
		team         models.Team
		teamID       string
		hubUserID    string
		createdByRaw sql.NullString
		quota        sql.NullInt64
		createdAt    string
		updatedAt    string
		storageUsed  int64
	)
	if err := row.Scan(&teamID, &team.Slug, &team.Name, &team.Description, &hubUserID, &createdByRaw, &team.Role, &quota, &storageUsed, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	parsedTeamID, err := uuid.Parse(teamID)
	if err != nil {
		return nil, err
	}
	parsedHubUserID, err := uuid.Parse(hubUserID)
	if err != nil {
		return nil, err
	}
	team.ID = parsedTeamID
	team.HubUserID = parsedHubUserID
	if createdByRaw.Valid && createdByRaw.String != "" {
		if createdBy, parseErr := uuid.Parse(createdByRaw.String); parseErr == nil {
			team.CreatedByUserID = createdBy
		}
	}
	if quota.Valid {
		value := quota.Int64
		team.StorageQuotaBytes = &value
	}
	team.StorageUsedBytes = storageUsed
	team.CreatedAt = mustParseTime(createdAt)
	team.UpdatedAt = mustParseTime(updatedAt)
	team.CanManageMembers = models.TeamRoleCanManageMembers(team.Role)
	team.CanWrite = models.TeamRoleCanWrite(team.Role)
	return &team, nil
}

func scanSQLiteTeamMember(row sqliteTeamScanner) (*models.TeamMember, error) {
	var (
		member    models.TeamMember
		teamID    string
		userID    string
		createdAt string
		updatedAt string
	)
	if err := row.Scan(&teamID, &userID, &member.UserSlug, &member.DisplayName, &member.Email, &member.Role, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	parsedTeamID, err := uuid.Parse(teamID)
	if err != nil {
		return nil, err
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	member.TeamID = parsedTeamID
	member.UserID = parsedUserID
	member.CreatedAt = mustParseTime(createdAt)
	member.UpdatedAt = mustParseTime(updatedAt)
	return &member, nil
}
