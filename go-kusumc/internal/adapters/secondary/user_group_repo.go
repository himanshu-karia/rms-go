package secondary

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type UserGroupRecord struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    *string                `json:"description,omitempty"`
	Scope          map[string]string      `json:"scope"`
	DefaultRoleIDs []string               `json:"defaultRoleIds"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type UserGroupMemberRecord struct {
	UserID   string    `json:"userId"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	AddedAt  time.Time `json:"addedAt"`
	AddedBy  *string   `json:"addedBy,omitempty"`
	Email    *string   `json:"email,omitempty"`
	GroupID  string    `json:"groupId"`
}

func (r *PostgresRepo) ListUserGroups(scopeFilters map[string]string) ([]UserGroupRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, name, description, scope, default_role_ids, metadata, created_at, updated_at
		FROM user_groups
		WHERE deleted_at IS NULL`

	args := []interface{}{}
	argIdx := 1
	for key, value := range scopeFilters {
		if value == "" {
			continue
		}
		query += fmt.Sprintf(" AND scope->>'%s' = $%d", key, argIdx)
		args = append(args, value)
		argIdx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []UserGroupRecord
	for rows.Next() {
		var rec UserGroupRecord
		var desc *string
		var scopeBytes []byte
		var roleBytes []byte
		var metadataBytes []byte
		if err := rows.Scan(&rec.ID, &rec.Name, &desc, &scopeBytes, &roleBytes, &metadataBytes, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		rec.Description = desc
		if len(scopeBytes) > 0 {
			_ = json.Unmarshal(scopeBytes, &rec.Scope)
		}
		if len(roleBytes) > 0 {
			_ = json.Unmarshal(roleBytes, &rec.DefaultRoleIDs)
		}
		if len(metadataBytes) > 0 {
			_ = json.Unmarshal(metadataBytes, &rec.Metadata)
		}
		groups = append(groups, rec)
	}
	return groups, nil
}

func (r *PostgresRepo) GetUserGroupByID(groupID string) (*UserGroupRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := r.Pool.QueryRow(ctx, `
		SELECT id, name, description, scope, default_role_ids, metadata, created_at, updated_at
		FROM user_groups
		WHERE id = $1 AND deleted_at IS NULL
	`, groupID)

	var rec UserGroupRecord
	var desc *string
	var scopeBytes []byte
	var roleBytes []byte
	var metadataBytes []byte
	if err := row.Scan(&rec.ID, &rec.Name, &desc, &scopeBytes, &roleBytes, &metadataBytes, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rec.Description = desc
	if len(scopeBytes) > 0 {
		_ = json.Unmarshal(scopeBytes, &rec.Scope)
	}
	if len(roleBytes) > 0 {
		_ = json.Unmarshal(roleBytes, &rec.DefaultRoleIDs)
	}
	if len(metadataBytes) > 0 {
		_ = json.Unmarshal(metadataBytes, &rec.Metadata)
	}
	return &rec, nil
}

func (r *PostgresRepo) CreateUserGroup(name string, description *string, scope map[string]string, defaultRoleIDs []string, metadata map[string]interface{}) (*UserGroupRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scopeBytes, err := json.Marshal(scope)
	if err != nil {
		return nil, err
	}
	rolesBytes, err := json.Marshal(defaultRoleIDs)
	if err != nil {
		return nil, err
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	var rec UserGroupRecord
	var descOut *string
	var scopeOut, rolesOut, metadataOut []byte
	row := r.Pool.QueryRow(ctx, `
		INSERT INTO user_groups (name, description, scope, default_role_ids, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, description, scope, default_role_ids, metadata, created_at, updated_at
	`, name, description, scopeBytes, rolesBytes, metadataBytes)
	if err := row.Scan(&rec.ID, &rec.Name, &descOut, &scopeOut, &rolesOut, &metadataOut, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return nil, err
	}
	if len(scopeOut) > 0 {
		_ = json.Unmarshal(scopeOut, &rec.Scope)
	}
	if len(rolesOut) > 0 {
		_ = json.Unmarshal(rolesOut, &rec.DefaultRoleIDs)
	}
	if len(metadataOut) > 0 {
		_ = json.Unmarshal(metadataOut, &rec.Metadata)
	}
	rec.Description = descOut
	return &rec, nil
}

func (r *PostgresRepo) UpdateUserGroup(id string, name *string, description *string, defaultRoleIDs *[]string, metadata *map[string]interface{}) (*UserGroupRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *name)
		argIdx++
	}
	if description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *description)
		argIdx++
	}
	if defaultRoleIDs != nil {
		rolesBytes, err := json.Marshal(defaultRoleIDs)
		if err != nil {
			return nil, err
		}
		setClauses = append(setClauses, fmt.Sprintf("default_role_ids = $%d", argIdx))
		args = append(args, rolesBytes)
		argIdx++
	}
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return nil, err
		}
		setClauses = append(setClauses, fmt.Sprintf("metadata = $%d", argIdx))
		args = append(args, metadataBytes)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	args = append(args, id)
	query := fmt.Sprintf(`
		UPDATE user_groups
		SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, name, description, scope, default_role_ids, metadata, created_at, updated_at
	`, joinClauses(setClauses), argIdx)

	var rec UserGroupRecord
	var descOut *string
	var scopeOut, rolesOut, metadataOut []byte
	row := r.Pool.QueryRow(ctx, query, args...)
	if err := row.Scan(&rec.ID, &rec.Name, &descOut, &scopeOut, &rolesOut, &metadataOut, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return nil, err
	}
	if len(scopeOut) > 0 {
		_ = json.Unmarshal(scopeOut, &rec.Scope)
	}
	if len(rolesOut) > 0 {
		_ = json.Unmarshal(rolesOut, &rec.DefaultRoleIDs)
	}
	if len(metadataOut) > 0 {
		_ = json.Unmarshal(metadataOut, &rec.Metadata)
	}
	rec.Description = descOut
	return &rec, nil
}

func (r *PostgresRepo) SoftDeleteUserGroup(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.Pool.Exec(ctx, `
		UPDATE user_groups
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

func (r *PostgresRepo) ListUserGroupMembers(groupID string) ([]UserGroupMemberRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx, `
		SELECT m.group_id, u.id, u.username, u.role, u.email, m.added_at, m.added_by
		FROM user_group_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.group_id = $1 AND u.deleted_at IS NULL
		ORDER BY m.added_at DESC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []UserGroupMemberRecord
	for rows.Next() {
		var rec UserGroupMemberRecord
		if err := rows.Scan(&rec.GroupID, &rec.UserID, &rec.Username, &rec.Role, &rec.Email, &rec.AddedAt, &rec.AddedBy); err != nil {
			return nil, err
		}
		members = append(members, rec)
	}
	return members, nil
}

func (r *PostgresRepo) AddUserToGroup(groupID, userID string, addedBy *string) (*UserGroupMemberRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.Pool.Exec(ctx, `
		INSERT INTO user_group_members (group_id, user_id, added_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_id, user_id) DO NOTHING
	`, groupID, userID, addedBy)
	if err != nil {
		return nil, err
	}

	var rec UserGroupMemberRecord
	row := r.Pool.QueryRow(ctx, `
		SELECT m.group_id, u.id, u.username, u.role, u.email, m.added_at, m.added_by
		FROM user_group_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.group_id = $1 AND m.user_id = $2
	`, groupID, userID)
	if err := row.Scan(&rec.GroupID, &rec.UserID, &rec.Username, &rec.Role, &rec.Email, &rec.AddedAt, &rec.AddedBy); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *PostgresRepo) RemoveUserFromGroup(groupID, userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.Pool.Exec(ctx, `
		DELETE FROM user_group_members
		WHERE group_id = $1 AND user_id = $2
	`, groupID, userID)
	return err
}

func joinClauses(clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}
	out := clauses[0]
	for i := 1; i < len(clauses); i++ {
		out += ", " + clauses[i]
	}
	return out
}
