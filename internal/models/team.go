package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	TeamRoleOwner  = "owner"
	TeamRoleAdmin  = "admin"
	TeamRoleMember = "member"
	TeamRoleViewer = "viewer"
)

type Team struct {
	ID                uuid.UUID `json:"id" db:"id"`
	Slug              string    `json:"slug" db:"slug"`
	Name              string    `json:"name" db:"name"`
	Description       string    `json:"description,omitempty" db:"description"`
	HubUserID         uuid.UUID `json:"hub_user_id,omitempty" db:"hub_user_id"`
	CreatedByUserID   uuid.UUID `json:"created_by_user_id,omitempty" db:"created_by_user_id"`
	Role              string    `json:"role,omitempty" db:"role"`
	CanManageMembers  bool      `json:"can_manage_members"`
	CanWrite          bool      `json:"can_write"`
	StorageUsedBytes  int64     `json:"storage_used_bytes"`
	StorageQuotaBytes *int64    `json:"storage_quota_bytes,omitempty"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type TeamMember struct {
	TeamID      uuid.UUID `json:"team_id" db:"team_id"`
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	UserSlug    string    `json:"user_slug" db:"user_slug"`
	DisplayName string    `json:"display_name" db:"display_name"`
	Email       string    `json:"email,omitempty" db:"email"`
	Role        string    `json:"role" db:"role"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type CreateTeamRequest struct {
	Slug              string `json:"slug"`
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	StorageQuotaBytes *int64 `json:"storage_quota_bytes,omitempty"`
}

type UpdateTeamRequest struct {
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	StorageQuotaBytes *int64 `json:"storage_quota_bytes,omitempty"`
}

type AddTeamMemberRequest struct {
	UserSlug string `json:"user_slug"`
	Role     string `json:"role"`
}

type UpdateTeamMemberRequest struct {
	Role string `json:"role"`
}

func IsValidTeamRole(role string) bool {
	switch role {
	case TeamRoleOwner, TeamRoleAdmin, TeamRoleMember, TeamRoleViewer:
		return true
	default:
		return false
	}
}

func TeamRoleCanManageMembers(role string) bool {
	return role == TeamRoleOwner || role == TeamRoleAdmin
}

func TeamRoleCanWrite(role string) bool {
	return role == TeamRoleOwner || role == TeamRoleAdmin || role == TeamRoleMember
}
