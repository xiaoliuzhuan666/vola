package services

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var teamSlugRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type TeamService struct {
	db   *pgxpool.Pool
	repo TeamRepo
}

func NewTeamService(db *pgxpool.Pool) *TeamService {
	return &TeamService{db: db}
}

func NewTeamServiceWithRepo(repo TeamRepo) *TeamService {
	return &TeamService{repo: repo}
}

func (s *TeamService) Create(ctx context.Context, creatorUserID uuid.UUID, req models.CreateTeamRequest) (*models.Team, error) {
	normalizedReq, err := normalizeCreateTeamRequest(req)
	if err != nil {
		return nil, err
	}
	if s.repo != nil {
		return s.repo.CreateTeam(ctx, creatorUserID, normalizedReq)
	}
	teamID := uuid.New()
	hubUserID := uuid.New()
	now := time.Now().UTC()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("team.Create: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO users (id, slug, display_name, email, avatar_url, bio, timezone, language, account_type, storage_quota_bytes, created_at, updated_at)
		 VALUES ($1, $2, $3, '', '', '', 'UTC', 'zh-CN', 'team_hub', $4, $5, $5)`,
		hubUserID, "team-"+teamID.String(), normalizedReq.Name, normalizedReq.StorageQuotaBytes, now)
	if err != nil {
		return nil, fmt.Errorf("team.Create: insert hub user: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO teams (id, slug, name, description, hub_user_id, created_by_user_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`,
		teamID, normalizedReq.Slug, normalizedReq.Name, normalizedReq.Description, hubUserID, creatorUserID, now)
	if err != nil {
		return nil, fmt.Errorf("team.Create: insert team: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id, role, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $4)`,
		teamID, creatorUserID, models.TeamRoleOwner, now)
	if err != nil {
		return nil, fmt.Errorf("team.Create: insert owner: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("team.Create: commit: %w", err)
	}
	return s.GetForUser(ctx, creatorUserID, teamID)
}

func (s *TeamService) ListForUser(ctx context.Context, userID uuid.UUID) ([]models.Team, error) {
	if s.repo != nil {
		return s.repo.ListTeamsForUser(ctx, userID)
	}
	rows, err := s.db.Query(ctx, teamSelectSQL(`
		WHERE tm.user_id = $1
		GROUP BY t.id, t.slug, t.name, t.description, t.hub_user_id, t.created_by_user_id, tm.role, uh.storage_quota_bytes, t.created_at, t.updated_at
		ORDER BY t.created_at DESC`), userID)
	if err != nil {
		return nil, fmt.Errorf("team.ListForUser: %w", err)
	}
	defer rows.Close()
	teams := []models.Team{}
	for rows.Next() {
		team, scanErr := scanTeam(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("team.ListForUser: scan: %w", scanErr)
		}
		teams = append(teams, *team)
	}
	return teams, rows.Err()
}

func (s *TeamService) GetForUser(ctx context.Context, userID, teamID uuid.UUID) (*models.Team, error) {
	if s.repo != nil {
		return s.repo.GetTeamForUser(ctx, userID, teamID)
	}
	row := s.db.QueryRow(ctx, teamSelectSQL(`
		WHERE t.id = $1 AND tm.user_id = $2
		GROUP BY t.id, t.slug, t.name, t.description, t.hub_user_id, t.created_by_user_id, tm.role, uh.storage_quota_bytes, t.created_at, t.updated_at`),
		teamID, userID)
	return scanTeam(row)
}

func (s *TeamService) GetBySlugForUser(ctx context.Context, userID uuid.UUID, slug string) (*models.Team, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if s.repo != nil {
		return s.repo.GetTeamBySlugForUser(ctx, userID, slug)
	}
	row := s.db.QueryRow(ctx, teamSelectSQL(`
		WHERE t.slug = $1 AND tm.user_id = $2
		GROUP BY t.id, t.slug, t.name, t.description, t.hub_user_id, t.created_by_user_id, tm.role, uh.storage_quota_bytes, t.created_at, t.updated_at`),
		slug, userID)
	return scanTeam(row)
}

func (s *TeamService) Update(ctx context.Context, userID, teamID uuid.UUID, req models.UpdateTeamRequest) (*models.Team, error) {
	normalizedReq, err := normalizeUpdateTeamRequest(req)
	if err != nil {
		return nil, err
	}
	if s.repo != nil {
		return s.repo.UpdateTeam(ctx, userID, teamID, normalizedReq)
	}
	team, err := s.GetForUser(ctx, userID, teamID)
	if err != nil {
		return nil, err
	}
	if !models.TeamRoleCanManageMembers(team.Role) {
		return nil, fmt.Errorf("team.Update: user cannot manage team")
	}
	now := time.Now().UTC()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("team.Update: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx,
		`UPDATE teams SET name = $1, description = $2, updated_at = $3 WHERE id = $4`,
		normalizedReq.Name, normalizedReq.Description, now, teamID)
	if err != nil {
		return nil, fmt.Errorf("team.Update: update team: %w", err)
	}
	if normalizedReq.StorageQuotaBytes != nil {
		_, err = tx.Exec(ctx,
			`UPDATE users SET storage_quota_bytes = $1, updated_at = $2 WHERE id = $3`,
			*normalizedReq.StorageQuotaBytes, now, team.HubUserID)
	} else {
		_, err = tx.Exec(ctx,
			`UPDATE users SET storage_quota_bytes = NULL, updated_at = $1 WHERE id = $2`,
			now, team.HubUserID)
	}
	if err != nil {
		return nil, fmt.Errorf("team.Update: update hub user: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("team.Update: commit: %w", err)
	}
	return s.GetForUser(ctx, userID, teamID)
}

func (s *TeamService) ListMembers(ctx context.Context, teamID uuid.UUID) ([]models.TeamMember, error) {
	if s.repo != nil {
		return s.repo.ListMembers(ctx, teamID)
	}
	rows, err := s.db.Query(ctx,
		`SELECT tm.team_id, tm.user_id, u.slug, COALESCE(u.display_name, ''), COALESCE(u.email, ''), tm.role, tm.created_at, tm.updated_at
		   FROM team_members tm
		   JOIN users u ON u.id = tm.user_id
		  WHERE tm.team_id = $1
		  ORDER BY CASE tm.role WHEN 'owner' THEN 1 WHEN 'admin' THEN 2 WHEN 'member' THEN 3 ELSE 4 END, u.slug`,
		teamID)
	if err != nil {
		return nil, fmt.Errorf("team.ListMembers: %w", err)
	}
	defer rows.Close()
	members := []models.TeamMember{}
	for rows.Next() {
		member, scanErr := scanTeamMember(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("team.ListMembers: scan: %w", scanErr)
		}
		members = append(members, *member)
	}
	return members, rows.Err()
}

func (s *TeamService) AddMember(ctx context.Context, teamID, userID uuid.UUID, role string) (*models.TeamMember, error) {
	role = normalizeTeamRole(role)
	if !models.IsValidTeamRole(role) || role == models.TeamRoleOwner {
		return nil, fmt.Errorf("team.AddMember: role must be admin, member, or viewer")
	}
	if s.repo != nil {
		return s.repo.AddMember(ctx, teamID, userID, role)
	}
	now := time.Now().UTC()
	_, err := s.db.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id, role, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $4)
		 ON CONFLICT (team_id, user_id) DO UPDATE
		   SET role = EXCLUDED.role, updated_at = EXCLUDED.updated_at`,
		teamID, userID, role, now)
	if err != nil {
		return nil, fmt.Errorf("team.AddMember: %w", err)
	}
	return s.getMember(ctx, teamID, userID)
}

func (s *TeamService) UpdateMemberRole(ctx context.Context, teamID, userID uuid.UUID, role string) (*models.TeamMember, error) {
	role = normalizeTeamRole(role)
	if !models.IsValidTeamRole(role) {
		return nil, fmt.Errorf("team.UpdateMemberRole: invalid role")
	}
	if s.repo != nil {
		return s.repo.UpdateMemberRole(ctx, teamID, userID, role)
	}
	if role != models.TeamRoleOwner {
		if err := s.ensureOwnerWouldRemain(ctx, teamID, userID); err != nil {
			return nil, err
		}
	}
	tag, err := s.db.Exec(ctx,
		`UPDATE team_members SET role = $1, updated_at = $2 WHERE team_id = $3 AND user_id = $4`,
		role, time.Now().UTC(), teamID, userID)
	if err != nil {
		return nil, fmt.Errorf("team.UpdateMemberRole: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("team.UpdateMemberRole: member not found")
	}
	return s.getMember(ctx, teamID, userID)
}

func (s *TeamService) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	if s.repo != nil {
		return s.repo.RemoveMember(ctx, teamID, userID)
	}
	if err := s.ensureOwnerWouldRemain(ctx, teamID, userID); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID)
	if err != nil {
		return fmt.Errorf("team.RemoveMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("team.RemoveMember: member not found")
	}
	return nil
}

func (s *TeamService) getMember(ctx context.Context, teamID, userID uuid.UUID) (*models.TeamMember, error) {
	row := s.db.QueryRow(ctx,
		`SELECT tm.team_id, tm.user_id, u.slug, COALESCE(u.display_name, ''), COALESCE(u.email, ''), tm.role, tm.created_at, tm.updated_at
		   FROM team_members tm
		   JOIN users u ON u.id = tm.user_id
		  WHERE tm.team_id = $1 AND tm.user_id = $2`,
		teamID, userID)
	return scanTeamMember(row)
}

func (s *TeamService) ensureOwnerWouldRemain(ctx context.Context, teamID, userID uuid.UUID) error {
	var role string
	err := s.db.QueryRow(ctx, `SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID).Scan(&role)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("team.ensureOwnerWouldRemain: %w", err)
	}
	if role != models.TeamRoleOwner {
		return nil
	}
	var ownerCount int
	err = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM team_members WHERE team_id = $1 AND role = 'owner'`, teamID).Scan(&ownerCount)
	if err != nil {
		return fmt.Errorf("team.ensureOwnerWouldRemain: count owners: %w", err)
	}
	if ownerCount <= 1 {
		return fmt.Errorf("team.ensureOwnerWouldRemain: team must keep at least one owner")
	}
	return nil
}

func normalizeCreateTeamRequest(req models.CreateTeamRequest) (models.CreateTeamRequest, error) {
	req.Slug = strings.TrimSpace(strings.ToLower(req.Slug))
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" {
		req.Name = req.Slug
	}
	if err := validateTeamSlug(req.Slug); err != nil {
		return req, err
	}
	if req.StorageQuotaBytes != nil && *req.StorageQuotaBytes < 0 {
		return req, fmt.Errorf("storage_quota_bytes must be >= 0")
	}
	return req, nil
}

func normalizeUpdateTeamRequest(req models.UpdateTeamRequest) (models.UpdateTeamRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" {
		return req, fmt.Errorf("team name is required")
	}
	if req.StorageQuotaBytes != nil && *req.StorageQuotaBytes < 0 {
		return req, fmt.Errorf("storage_quota_bytes must be >= 0")
	}
	return req, nil
}

func validateTeamSlug(slug string) error {
	if !teamSlugRegexp.MatchString(slug) {
		return fmt.Errorf("team slug must use lowercase letters, numbers, and hyphens, up to 64 characters")
	}
	return nil
}

func normalizeTeamRole(role string) string {
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		return models.TeamRoleMember
	}
	return role
}

func teamSelectSQL(suffix string) string {
	return `SELECT t.id,
	              t.slug,
	              t.name,
	              COALESCE(t.description, ''),
	              t.hub_user_id,
	              t.created_by_user_id,
	              tm.role,
	              uh.storage_quota_bytes,
	              COALESCE(SUM(
		              CASE
			              WHEN ft.is_directory THEN 0
			              WHEN fb.entry_id IS NOT NULL THEN fb.size_bytes
			              ELSE OCTET_LENGTH(COALESCE(ft.content, ''))
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

type teamScanner interface {
	Scan(dest ...interface{}) error
}

func scanTeam(row teamScanner) (*models.Team, error) {
	var (
		team  models.Team
		quota sql.NullInt64
	)
	if err := row.Scan(
		&team.ID,
		&team.Slug,
		&team.Name,
		&team.Description,
		&team.HubUserID,
		&team.CreatedByUserID,
		&team.Role,
		&quota,
		&team.StorageUsedBytes,
		&team.CreatedAt,
		&team.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if quota.Valid {
		value := quota.Int64
		team.StorageQuotaBytes = &value
	}
	team.CanManageMembers = models.TeamRoleCanManageMembers(team.Role)
	team.CanWrite = models.TeamRoleCanWrite(team.Role)
	return &team, nil
}

type teamMemberScanner interface {
	Scan(dest ...interface{}) error
}

func scanTeamMember(row teamMemberScanner) (*models.TeamMember, error) {
	var member models.TeamMember
	if err := row.Scan(
		&member.TeamID,
		&member.UserID,
		&member.UserSlug,
		&member.DisplayName,
		&member.Email,
		&member.Role,
		&member.CreatedAt,
		&member.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &member, nil
}
