package secondary

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresUserRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresUserRepo(pool *pgxpool.Pool) *PostgresUserRepo {
	return &PostgresUserRepo{pool: pool}
}

func (r *PostgresUserRepo) CreateUser(username, hash, role string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// We default org_id to null or fetch 'default'
	// For V1 MVP, just insert basics
	query := `INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3)`
	_, err := r.pool.Exec(ctx, query, username, hash, role)
	return err
}

func (r *PostgresUserRepo) GrantAllCapabilities(userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `INSERT INTO user_capabilities (user_id, capability_key)
		SELECT $1, key FROM capabilities
		ON CONFLICT DO NOTHING`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

// User struct shared for now
type UserRecord struct {
	ID                 string
	Username           string
	Hash               string
	Role               string
	Email              *string
	DisplayName        *string
	Active             bool
	MustRotatePassword bool
	Metadata           map[string]interface{}
}

func decodeMetadata(raw []byte) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var meta map[string]interface{}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil
	}
	return meta
}

func encodeMetadata(metadata map[string]interface{}) ([]byte, error) {
	if metadata == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(metadata)
}

type UserSessionRecord struct {
	ID          string
	UserID      string
	RefreshHash string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	LastUsedAt  *time.Time
	IPAddress   *string
	UserAgent   *string
}

type RoleBindingRecord struct {
	ID        string
	UserID    string
	RoleKey   string
	RoleType  string
	Scope     map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ListUsersFilters struct {
	Status      string
	Query       string
	RoleKey     string
	RoleType    string
	StateID     string
	AuthorityID string
	ProjectID   string
	GroupID     string
	Cursor      string
	Limit       int
}

func (r *PostgresUserRepo) GetUserByUsername(username string) (*UserRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT id, username, email, display_name, password_hash, role, active, must_rotate_password, metadata
		FROM users WHERE username = $1 AND deleted_at IS NULL`

	var u UserRecord
	var metadata []byte
	err := r.pool.QueryRow(ctx, query, username).Scan(
		&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.Hash, &u.Role, &u.Active, &u.MustRotatePassword, &metadata,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	u.Metadata = decodeMetadata(metadata)
	return &u, nil
}

func (r *PostgresUserRepo) GetAllUsers() ([]UserRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT id, username, role, active, display_name FROM users WHERE deleted_at IS NULL`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserRecord
	for rows.Next() {
		var u UserRecord
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.Active, &u.DisplayName); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *PostgresUserRepo) ListUsers(filters ListUsersFilters) ([]UserRecord, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	limit := filters.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	query := `SELECT DISTINCT u.id, u.username, u.role, u.active, u.display_name, u.must_rotate_password, u.metadata
		FROM users u`
	joins := []string{}
	where := []string{"u.deleted_at IS NULL"}
	args := []interface{}{}
	argIdx := 1

	needsBindings := filters.RoleKey != "" || filters.RoleType != "" || filters.StateID != "" || filters.AuthorityID != "" || filters.ProjectID != ""
	if needsBindings {
		joins = append(joins, "JOIN user_role_bindings b ON b.user_id = u.id")
	}
	if filters.GroupID != "" {
		joins = append(joins, "JOIN user_group_members m ON m.user_id = u.id")
		where = append(where, fmt.Sprintf("m.group_id = $%d", argIdx))
		args = append(args, filters.GroupID)
		argIdx++
	}

	if filters.Status != "" {
		switch filters.Status {
		case "active":
			where = append(where, "u.active = true")
		case "disabled":
			where = append(where, "u.active = false")
		}
	}
	if filters.Query != "" {
		where = append(where, fmt.Sprintf("(u.username ILIKE $%d OR COALESCE(u.display_name,'') ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+filters.Query+"%")
		argIdx++
	}
	if filters.RoleKey != "" {
		where = append(where, fmt.Sprintf("b.role_key = $%d", argIdx))
		args = append(args, filters.RoleKey)
		argIdx++
	}
	if filters.RoleType != "" {
		where = append(where, fmt.Sprintf("b.role_type = $%d", argIdx))
		args = append(args, filters.RoleType)
		argIdx++
	}
	if filters.StateID != "" {
		where = append(where, fmt.Sprintf("(b.scope->>'stateId' = $%d OR b.scope = '{}'::jsonb)", argIdx))
		args = append(args, filters.StateID)
		argIdx++
	}
	if filters.AuthorityID != "" {
		where = append(where, fmt.Sprintf("(b.scope->>'authorityId' = $%d OR b.scope = '{}'::jsonb)", argIdx))
		args = append(args, filters.AuthorityID)
		argIdx++
	}
	if filters.ProjectID != "" {
		where = append(where, fmt.Sprintf("(b.scope->>'projectId' = $%d OR b.scope = '{}'::jsonb)", argIdx))
		args = append(args, filters.ProjectID)
		argIdx++
	}
	if filters.Cursor != "" {
		where = append(where, fmt.Sprintf("u.id::text < $%d", argIdx))
		args = append(args, filters.Cursor)
		argIdx++
	}

	if len(joins) > 0 {
		query += " " + strings.Join(joins, " ")
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY u.id DESC LIMIT %d", limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var users []UserRecord
	for rows.Next() {
		var u UserRecord
		var meta []byte
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.Active, &u.DisplayName, &u.MustRotatePassword, &meta); err != nil {
			return nil, "", err
		}
		u.Metadata = decodeMetadata(meta)
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	nextCursor := ""
	if len(users) > limit {
		nextCursor = users[limit].ID
		users = users[:limit]
	}
	return users, nextCursor, nil
}

func (r *PostgresUserRepo) GetUserByID(id string) (*UserRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT id, username, email, display_name, password_hash, role, active, must_rotate_password, metadata
		FROM users WHERE id = $1 AND deleted_at IS NULL`
	var u UserRecord
	var metadata []byte
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.Hash, &u.Role, &u.Active, &u.MustRotatePassword, &metadata,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	u.Metadata = decodeMetadata(metadata)
	return &u, nil
}

func (r *PostgresUserRepo) CreateUserWithProfile(username string, email, displayName *string, hash, role string, active, mustRotate bool, metadata map[string]interface{}) (*UserRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload, err := encodeMetadata(metadata)
	if err != nil {
		return nil, err
	}

	query := `INSERT INTO users (username, email, display_name, password_hash, role, active, must_rotate_password, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, username, email, display_name, password_hash, role, active, must_rotate_password, metadata`
	var u UserRecord
	var meta []byte
	err = r.pool.QueryRow(ctx, query, username, email, displayName, hash, role, active, mustRotate, payload).Scan(
		&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.Hash, &u.Role, &u.Active, &u.MustRotatePassword, &meta,
	)
	if err != nil {
		return nil, err
	}
	u.Metadata = decodeMetadata(meta)
	return &u, nil
}

func (r *PostgresUserRepo) UpdateUserProfile(userID string, displayName *string, active *bool, mustRotate *bool, metadata map[string]interface{}) (*UserRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if displayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", argIdx))
		args = append(args, *displayName)
		argIdx++
	}
	if active != nil {
		setClauses = append(setClauses, fmt.Sprintf("active = $%d", argIdx))
		args = append(args, *active)
		argIdx++
	}
	if mustRotate != nil {
		setClauses = append(setClauses, fmt.Sprintf("must_rotate_password = $%d", argIdx))
		args = append(args, *mustRotate)
		argIdx++
	}
	if metadata != nil {
		payload, err := encodeMetadata(metadata)
		if err != nil {
			return nil, err
		}
		setClauses = append(setClauses, fmt.Sprintf("metadata = $%d", argIdx))
		args = append(args, payload)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	query := fmt.Sprintf(`UPDATE users SET %s, updated_at = NOW() WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, username, email, display_name, password_hash, role, active, must_rotate_password, metadata`, strings.Join(setClauses, ", "), argIdx)
	args = append(args, userID)

	var u UserRecord
	var meta []byte
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.Hash, &u.Role, &u.Active, &u.MustRotatePassword, &meta,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	u.Metadata = decodeMetadata(meta)
	return &u, nil
}

func (r *PostgresUserRepo) UpdatePassword(userID, newHash string, mustRotate bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `UPDATE users SET password_hash = $1, must_rotate_password = $2, updated_at = NOW() WHERE id = $3 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, query, newHash, mustRotate, userID)
	return err
}

func (r *PostgresUserRepo) CreateSession(userID, refreshHash string, expiresAt time.Time, ipAddress, userAgent *string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `INSERT INTO user_sessions (user_id, refresh_token_hash, expires_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var id string
	err := r.pool.QueryRow(ctx, query, userID, refreshHash, expiresAt, ipAddress, userAgent).Scan(&id)
	return id, err
}

func (r *PostgresUserRepo) GetSessionByRefreshHash(refreshHash string) (*UserSessionRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT id, user_id, refresh_token_hash, created_at, expires_at, revoked_at, last_used_at, ip_address, user_agent
		FROM user_sessions WHERE refresh_token_hash = $1`
	var s UserSessionRecord
	var revokedAt, lastUsedAt *time.Time
	var ipAddress, userAgent *string
	if err := r.pool.QueryRow(ctx, query, refreshHash).Scan(
		&s.ID, &s.UserID, &s.RefreshHash, &s.CreatedAt, &s.ExpiresAt, &revokedAt, &lastUsedAt, &ipAddress, &userAgent,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	s.RevokedAt = revokedAt
	s.LastUsedAt = lastUsedAt
	s.IPAddress = ipAddress
	s.UserAgent = userAgent
	return &s, nil
}

func (r *PostgresUserRepo) GetSessionByID(sessionID string) (*UserSessionRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT id, user_id, refresh_token_hash, created_at, expires_at, revoked_at, last_used_at, ip_address, user_agent
		FROM user_sessions WHERE id = $1`
	var s UserSessionRecord
	var revokedAt, lastUsedAt *time.Time
	var ipAddress, userAgent *string
	if err := r.pool.QueryRow(ctx, query, sessionID).Scan(
		&s.ID, &s.UserID, &s.RefreshHash, &s.CreatedAt, &s.ExpiresAt, &revokedAt, &lastUsedAt, &ipAddress, &userAgent,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	s.RevokedAt = revokedAt
	s.LastUsedAt = lastUsedAt
	s.IPAddress = ipAddress
	s.UserAgent = userAgent
	return &s, nil
}

func (r *PostgresUserRepo) ListSessionsByUserID(userID string, limit int) ([]UserSessionRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	query := `SELECT id, user_id, refresh_token_hash, created_at, expires_at, revoked_at, last_used_at, ip_address, user_agent
		FROM user_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]UserSessionRecord, 0, limit)
	for rows.Next() {
		var rec UserSessionRecord
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.RefreshHash, &rec.CreatedAt, &rec.ExpiresAt, &rec.RevokedAt, &rec.LastUsedAt, &rec.IPAddress, &rec.UserAgent); err != nil {
			return nil, err
		}
		items = append(items, rec)
	}
	return items, rows.Err()
}

func (r *PostgresUserRepo) RotateSessionRefresh(sessionID, newRefreshHash string, newExpiry time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `UPDATE user_sessions SET refresh_token_hash = $1, expires_at = $2, last_used_at = NOW() WHERE id = $3 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, query, newRefreshHash, newExpiry, sessionID)
	return err
}

func (r *PostgresUserRepo) RevokeSession(sessionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `UPDATE user_sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, query, sessionID)
	return err
}

func (r *PostgresUserRepo) RevokeSessionsByUserID(userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `UPDATE user_sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

func (r *PostgresUserRepo) UpdateSelfProfile(userID string, email, displayName, phone *string) (*UserRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	current, err := r.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, nil
	}

	mergedMeta := current.Metadata
	if mergedMeta == nil {
		mergedMeta = map[string]interface{}{}
	}
	if phone != nil {
		value := strings.TrimSpace(*phone)
		if value == "" {
			delete(mergedMeta, "phone")
		} else {
			mergedMeta["phone"] = value
		}
	}
	payload, err := encodeMetadata(mergedMeta)
	if err != nil {
		return nil, err
	}

	query := `UPDATE users
		SET email = COALESCE($1, email),
			display_name = COALESCE($2, display_name),
			metadata = $3,
			updated_at = NOW()
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING id, username, email, display_name, password_hash, role, active, must_rotate_password, metadata`

	var updated UserRecord
	var rawMeta []byte
	if err := r.pool.QueryRow(ctx, query, email, displayName, payload, userID).Scan(
		&updated.ID, &updated.Username, &updated.Email, &updated.DisplayName, &updated.Hash, &updated.Role, &updated.Active, &updated.MustRotatePassword, &rawMeta,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	updated.Metadata = decodeMetadata(rawMeta)
	return &updated, nil
}

func (r *PostgresUserRepo) ListCapabilitiesByUserID(userID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT capability_key FROM user_capabilities WHERE user_id = $1`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var caps []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		caps = append(caps, key)
	}
	return caps, nil
}

func (r *PostgresUserRepo) SoftDeleteUser(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.pool.Exec(ctx, "UPDATE users SET deleted_at = NOW(), active = false WHERE id = $1 AND deleted_at IS NULL", id)
	return err
}

func (r *PostgresUserRepo) ListRoleBindingsByUserID(userID string) ([]RoleBindingRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.pool.Query(ctx, `SELECT id, user_id, role_key, role_type, scope, created_at, updated_at
		FROM user_role_bindings WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []RoleBindingRecord
	for rows.Next() {
		var rec RoleBindingRecord
		var scopeRaw []byte
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.RoleKey, &rec.RoleType, &scopeRaw, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		rec.Scope = decodeMetadata(scopeRaw)
		items = append(items, rec)
	}
	return items, rows.Err()
}

func (r *PostgresUserRepo) UpsertRoleBinding(userID, roleKey, roleType string, scope map[string]interface{}) (*RoleBindingRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if scope == nil {
		scope = map[string]interface{}{}
	}
	payload, err := encodeMetadata(scope)
	if err != nil {
		return nil, err
	}

	query := `INSERT INTO user_role_bindings (user_id, role_key, role_type, scope)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, role_key, role_type, scope)
		DO UPDATE SET updated_at = NOW()
		RETURNING id, user_id, role_key, role_type, scope, created_at, updated_at`
	var rec RoleBindingRecord
	var scopeRaw []byte
	if err := r.pool.QueryRow(ctx, query, userID, roleKey, roleType, payload).Scan(
		&rec.ID, &rec.UserID, &rec.RoleKey, &rec.RoleType, &scopeRaw, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if len(scopeRaw) > 0 {
		rec.Scope = decodeMetadata(scopeRaw)
	}
	return &rec, nil
}

func (r *PostgresUserRepo) DeleteRoleBinding(userID, bindingID string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd, err := r.pool.Exec(ctx, "DELETE FROM user_role_bindings WHERE id = $1 AND user_id = $2", bindingID, userID)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

func (r *PostgresUserRepo) DeleteRoleBindingByScope(userID, roleKey, roleType string, scope map[string]interface{}) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if scope == nil {
		scope = map[string]interface{}{}
	}
	payload, err := encodeMetadata(scope)
	if err != nil {
		return false, err
	}
	cmd, err := r.pool.Exec(ctx, "DELETE FROM user_role_bindings WHERE user_id = $1 AND role_key = $2 AND role_type = $3 AND scope = $4", userID, roleKey, roleType, payload)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}
