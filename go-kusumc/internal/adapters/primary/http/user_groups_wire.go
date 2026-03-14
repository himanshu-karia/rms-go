package http

import (
	"strings"
	"time"

	"ingestion-go/internal/adapters/secondary"
)

type wireUserGroup struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    *string                `json:"description,omitempty"`
	Scope          map[string]string      `json:"scope"`
	DefaultRoleIDs []string               `json:"default_role_ids"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type wireUserGroupMember struct {
	GroupID  string  `json:"group_id"`
	UserID   string  `json:"user_id"`
	Username string  `json:"username"`
	Role     string  `json:"role"`
	Email    *string `json:"email,omitempty"`
	AddedAt  string  `json:"added_at"`
	AddedBy  *string `json:"added_by,omitempty"`
}

func normalizeUserGroupScopeOut(scope map[string]string) map[string]string {
	if scope == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(scope))
	for rawKey, rawValue := range scope {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		value := strings.TrimSpace(rawValue)
		if value == "" {
			continue
		}

		switch key {
		case "stateId", "state_id":
			out["state_id"] = value
		case "authorityId", "authority_id":
			out["authority_id"] = value
		case "projectId", "project_id":
			out["project_id"] = value
		default:
			out[key] = value
		}
	}
	return out
}

// Internal scope representation currently uses camelCase keys (persisted in DB JSON).
// Accept snake_case on wire and normalize to internal keys for storage/filters.
func normalizeUserGroupScopeIn(scope map[string]string) map[string]string {
	if scope == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(scope))
	for rawKey, rawValue := range scope {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		value := strings.TrimSpace(rawValue)
		if value == "" {
			continue
		}

		switch key {
		case "state_id", "stateId":
			out["stateId"] = value
		case "authority_id", "authorityId":
			out["authorityId"] = value
		case "project_id", "projectId":
			out["projectId"] = value
		default:
			out[key] = value
		}
	}
	return out
}

func toWireUserGroup(group secondary.UserGroupRecord) wireUserGroup {
	return wireUserGroup{
		ID:             group.ID,
		Name:           group.Name,
		Description:    group.Description,
		Scope:          normalizeUserGroupScopeOut(group.Scope),
		DefaultRoleIDs: group.DefaultRoleIDs,
		Metadata:       group.Metadata,
		CreatedAt:      group.CreatedAt,
		UpdatedAt:      group.UpdatedAt,
	}
}

func toWireUserGroupMember(member secondary.UserGroupMemberRecord) wireUserGroupMember {
	addedAt := ""
	if !member.AddedAt.IsZero() {
		addedAt = member.AddedAt.UTC().Format(time.RFC3339)
	}

	return wireUserGroupMember{
		GroupID:  member.GroupID,
		UserID:   member.UserID,
		Username: member.Username,
		Role:     member.Role,
		Email:    member.Email,
		AddedAt:  addedAt,
		AddedBy:  member.AddedBy,
	}
}
