package http

import (
	"strings"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/services"
)

type wireUserRoleSummary struct {
	BindingID       string                 `json:"binding_id"`
	RoleKey         string                 `json:"role_key"`
	RoleType        string                 `json:"role_type"`
	RoleDescription string                 `json:"role_description"`
	Scope           map[string]interface{} `json:"scope"`
}

type wireUserSummary struct {
	ID                 string                 `json:"id"`
	Username           string                 `json:"username"`
	DisplayName        string                 `json:"display_name"`
	Status             string                 `json:"status"`
	MustRotatePassword bool                   `json:"must_rotate_password"`
	Metadata           map[string]interface{} `json:"metadata"`
	Roles              []wireUserRoleSummary  `json:"roles"`
}

type wireListUsersResult struct {
	Users      []wireUserSummary `json:"users"`
	NextCursor string            `json:"next_cursor"`
}

func toWireUserRoleSummary(role services.UserRoleSummary) wireUserRoleSummary {
	return wireUserRoleSummary{
		BindingID:       role.BindingID,
		RoleKey:         role.RoleKey,
		RoleType:        role.RoleType,
		RoleDescription: role.RoleDescription,
		Scope:           role.Scope,
	}
}

func toWireUserSummary(user services.UserSummary) wireUserSummary {
	wiredRoles := make([]wireUserRoleSummary, 0, len(user.Roles))
	for _, r := range user.Roles {
		wiredRoles = append(wiredRoles, toWireUserRoleSummary(r))
	}

	return wireUserSummary{
		ID:                 user.ID,
		Username:           user.Username,
		DisplayName:        user.DisplayName,
		Status:             user.Status,
		MustRotatePassword: user.MustRotatePassword,
		Metadata:           user.Metadata,
		Roles:              wiredRoles,
	}
}

func toWireListUsersResult(result *services.ListUsersResult) wireListUsersResult {
	if result == nil {
		return wireListUsersResult{Users: []wireUserSummary{}, NextCursor: ""}
	}
	wiredUsers := make([]wireUserSummary, 0, len(result.Users))
	for _, u := range result.Users {
		wiredUsers = append(wiredUsers, toWireUserSummary(u))
	}
	return wireListUsersResult{Users: wiredUsers, NextCursor: result.NextCursor}
}

func toWireUserSummaryFromRecord(user secondary.UserRecord) wireUserSummary {
	displayName := user.Username
	if user.DisplayName != nil && strings.TrimSpace(*user.DisplayName) != "" {
		displayName = *user.DisplayName
	}
	status := "disabled"
	if user.Active {
		status = "active"
	}

	return wireUserSummary{
		ID:                 user.ID,
		Username:           user.Username,
		DisplayName:        displayName,
		Status:             status,
		MustRotatePassword: user.MustRotatePassword,
		Metadata:           user.Metadata,
		Roles:              []wireUserRoleSummary{},
	}
}
