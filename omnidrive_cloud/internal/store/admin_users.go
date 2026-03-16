package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

type AdminUserWithPassword struct {
	Admin        domain.AdminIdentity
	PasswordHash string
}

type AdminIdentityListFilter struct {
	Query  string
	Status string
	RoleID string
	AdminPageFilter
}

type CreateAdminUserInput struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	IsActive     bool
	AuthMode     string
	RoleIDs      []string
}

type UpdateAdminUserInput struct {
	Email          *string
	Name           *string
	PasswordHash   *string
	IsActive       *bool
	RoleIDs        []string
	RoleIDsTouched bool
}

type CreateAdminSessionInput struct {
	ID          string
	AdminUserID string
	ExpiresAt   time.Time
	IPAddress   *string
	UserAgent   *string
}

func (s *Store) CountAdminUsers(ctx context.Context) (int64, error) {
	var count int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*)::BIGINT FROM admin_users`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) CountActiveAdminUsers(ctx context.Context) (int64, error) {
	var count int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*)::BIGINT FROM admin_users WHERE is_active = TRUE`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) CountActiveAdminsByRole(ctx context.Context, roleID string) (int64, error) {
	var count int64
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)::BIGINT
		FROM admin_users au
		INNER JOIN admin_user_roles aur ON aur.admin_user_id = au.id
		WHERE aur.role_id = $1 AND au.is_active = TRUE
	`, roleID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) GetAdminUserByEmail(ctx context.Context, email string) (*AdminUserWithPassword, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			au.id,
			au.email,
			au.name,
			au.password_hash,
			au.is_active,
			au.auth_mode,
			au.last_login_at,
			au.created_at,
			au.updated_at,
			COALESCE(ARRAY_AGG(DISTINCT ar.id) FILTER (WHERE ar.id IS NOT NULL), '{}'),
			COALESCE(ARRAY_AGG(DISTINCT ar.name) FILTER (WHERE ar.name IS NOT NULL), '{}'),
			COALESCE(ARRAY_AGG(DISTINCT arp.permission_code) FILTER (WHERE arp.permission_code IS NOT NULL), '{}')
		FROM admin_users au
		LEFT JOIN admin_user_roles aur ON aur.admin_user_id = au.id
		LEFT JOIN admin_roles ar ON ar.id = aur.role_id
		LEFT JOIN admin_role_permissions arp ON arp.role_id = ar.id
		WHERE au.email = $1
		GROUP BY au.id
	`, strings.ToLower(strings.TrimSpace(email)))

	var result AdminUserWithPassword
	var roleIDs []string
	var roles []string
	var permissions []string
	if err := row.Scan(
		&result.Admin.ID,
		&result.Admin.Email,
		&result.Admin.Name,
		&result.PasswordHash,
		&result.Admin.IsActive,
		&result.Admin.AuthMode,
		&result.Admin.LastLoginAt,
		&result.Admin.CreatedAt,
		&result.Admin.UpdatedAt,
		&roleIDs,
		&roles,
		&permissions,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	result.Admin.RoleIDs = normalizeAdminStringList(roleIDs)
	result.Admin.Roles = normalizeAdminStringList(roles)
	result.Admin.Role = adminPrimaryRole(result.Admin.Roles)
	result.Admin.Permissions = normalizeAdminStringList(permissions)
	return &result, nil
}

func (s *Store) GetAdminIdentityByID(ctx context.Context, adminID string) (*domain.AdminIdentity, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			au.id,
			au.email,
			au.name,
			au.is_active,
			au.auth_mode,
			au.last_login_at,
			au.created_at,
			au.updated_at,
			COALESCE(ARRAY_AGG(DISTINCT ar.id) FILTER (WHERE ar.id IS NOT NULL), '{}'),
			COALESCE(ARRAY_AGG(DISTINCT ar.name) FILTER (WHERE ar.name IS NOT NULL), '{}'),
			COALESCE(ARRAY_AGG(DISTINCT arp.permission_code) FILTER (WHERE arp.permission_code IS NOT NULL), '{}')
		FROM admin_users au
		LEFT JOIN admin_user_roles aur ON aur.admin_user_id = au.id
		LEFT JOIN admin_roles ar ON ar.id = aur.role_id
		LEFT JOIN admin_role_permissions arp ON arp.role_id = ar.id
		WHERE au.id = $1
		GROUP BY au.id
	`, adminID)

	item, err := scanAdminIdentity(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) GetAdminIdentityBySessionID(ctx context.Context, sessionID string) (*domain.AdminIdentity, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			au.id,
			au.email,
			au.name,
			au.is_active,
			au.auth_mode,
			au.last_login_at,
			au.created_at,
			au.updated_at,
			COALESCE(ARRAY_AGG(DISTINCT ar.id) FILTER (WHERE ar.id IS NOT NULL), '{}'),
			COALESCE(ARRAY_AGG(DISTINCT ar.name) FILTER (WHERE ar.name IS NOT NULL), '{}'),
			COALESCE(ARRAY_AGG(DISTINCT arp.permission_code) FILTER (WHERE arp.permission_code IS NOT NULL), '{}')
		FROM admin_sessions s
		INNER JOIN admin_users au ON au.id = s.admin_user_id
		LEFT JOIN admin_user_roles aur ON aur.admin_user_id = au.id
		LEFT JOIN admin_roles ar ON ar.id = aur.role_id
		LEFT JOIN admin_role_permissions arp ON arp.role_id = ar.id
		WHERE s.id = $1
		  AND s.status = 'active'
		  AND s.revoked_at IS NULL
		  AND s.expires_at > NOW()
		  AND au.is_active = TRUE
		GROUP BY au.id
	`, sessionID)

	item, err := scanAdminIdentity(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.SessionID = sessionID
	return item, nil
}

func (s *Store) ListAdminIdentities(ctx context.Context, filter AdminIdentityListFilter) ([]domain.AdminIdentity, int64, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(au.email ILIKE $%d OR au.name ILIKE $%d OR au.id ILIKE $%d)", argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}

	switch strings.TrimSpace(filter.Status) {
	case "active":
		whereParts = append(whereParts, fmt.Sprintf("au.is_active = $%d", argIndex))
		args = append(args, true)
		argIndex++
	case "inactive":
		whereParts = append(whereParts, fmt.Sprintf("au.is_active = $%d", argIndex))
		args = append(args, false)
		argIndex++
	}

	if roleID := strings.TrimSpace(filter.RoleID); roleID != "" {
		whereParts = append(whereParts, fmt.Sprintf(`
			EXISTS (
				SELECT 1
				FROM admin_user_roles aur_filter
				WHERE aur_filter.admin_user_id = au.id
				  AND aur_filter.role_id = $%d
			)
		`, argIndex))
		args = append(args, roleID)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM admin_users au
		%s
	`, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			au.id,
			au.email,
			au.name,
			au.is_active,
			au.auth_mode,
			au.last_login_at,
			au.created_at,
			au.updated_at,
			COALESCE(ARRAY_AGG(DISTINCT ar.id) FILTER (WHERE ar.id IS NOT NULL), '{}'),
			COALESCE(ARRAY_AGG(DISTINCT ar.name) FILTER (WHERE ar.name IS NOT NULL), '{}'),
			COALESCE(ARRAY_AGG(DISTINCT arp.permission_code) FILTER (WHERE arp.permission_code IS NOT NULL), '{}')
		FROM admin_users au
		LEFT JOIN admin_user_roles aur ON aur.admin_user_id = au.id
		LEFT JOIN admin_roles ar ON ar.id = aur.role_id
		LEFT JOIN admin_role_permissions arp ON arp.role_id = ar.id
		%s
		GROUP BY au.id
		ORDER BY au.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.AdminIdentity, 0)
	for rows.Next() {
		item, err := scanAdminIdentity(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (s *Store) CreateAdminUser(ctx context.Context, input CreateAdminUserInput) (*domain.AdminIdentity, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	roleIDs := normalizeAdminStringList(input.RoleIDs)
	if err := validateAdminRoleIDs(ctx, tx, roleIDs); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_users (id, email, name, password_hash, is_active, auth_mode, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`, input.ID, strings.ToLower(strings.TrimSpace(input.Email)), input.Name, input.PasswordHash, input.IsActive, input.AuthMode); err != nil {
		return nil, err
	}

	for _, roleID := range roleIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO admin_user_roles (admin_user_id, role_id, created_at)
			VALUES ($1, $2, NOW())
		`, input.ID, roleID); err != nil {
			return nil, err
		}
	}

	item, err := getAdminIdentityByIDTx(ctx, tx, input.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Store) UpdateAdminUser(ctx context.Context, adminID string, input UpdateAdminUserInput) (*domain.AdminIdentity, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	sets := make([]string, 0, 4)
	args := make([]any, 0, 5)
	argIndex := 1

	if input.Email != nil {
		sets = append(sets, fmt.Sprintf("email = $%d", argIndex))
		args = append(args, strings.ToLower(strings.TrimSpace(*input.Email)))
		argIndex++
	}
	if input.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.Name))
		argIndex++
	}
	if input.PasswordHash != nil {
		sets = append(sets, fmt.Sprintf("password_hash = $%d", argIndex))
		args = append(args, *input.PasswordHash)
		argIndex++
	}
	if input.IsActive != nil {
		sets = append(sets, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *input.IsActive)
		argIndex++
	}

	if len(sets) > 0 {
		sets = append(sets, "updated_at = NOW()")
		args = append(args, adminID)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE admin_users
			SET %s
			WHERE id = $%d
		`, strings.Join(sets, ", "), argIndex), args...); err != nil {
			return nil, err
		}
	}

	if input.RoleIDsTouched {
		roleIDs := normalizeAdminStringList(input.RoleIDs)
		if err := validateAdminRoleIDs(ctx, tx, roleIDs); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM admin_user_roles WHERE admin_user_id = $1`, adminID); err != nil {
			return nil, err
		}
		for _, roleID := range roleIDs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO admin_user_roles (admin_user_id, role_id, created_at)
				VALUES ($1, $2, NOW())
			`, adminID, roleID); err != nil {
				return nil, err
			}
		}
	}

	if input.PasswordHash != nil || (input.IsActive != nil && !*input.IsActive) {
		if _, err := tx.Exec(ctx, `
			UPDATE admin_sessions
			SET status = 'revoked',
			    revoked_at = NOW(),
			    updated_at = NOW()
			WHERE admin_user_id = $1
			  AND status = 'active'
			  AND revoked_at IS NULL
		`, adminID); err != nil {
			return nil, err
		}
	}

	item, err := getAdminIdentityByIDTx(ctx, tx, adminID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Store) CreateAdminSession(ctx context.Context, input CreateAdminSessionInput) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_sessions (id, admin_user_id, status, expires_at, ip_address, user_agent, last_seen_at, created_at, updated_at)
		VALUES ($1, $2, 'active', $3, $4, $5, NOW(), NOW(), NOW())
	`, input.ID, input.AdminUserID, input.ExpiresAt.UTC(), input.IPAddress, input.UserAgent); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE admin_users
		SET last_login_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`, input.AdminUserID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Store) RevokeAdminSession(ctx context.Context, sessionID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE admin_sessions
		SET status = 'revoked',
		    revoked_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'active'
		  AND revoked_at IS NULL
	`, sessionID)
	return err
}

func (s *Store) RevokeAdminSessionsByUserID(ctx context.Context, adminUserID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE admin_sessions
		SET status = 'revoked',
		    revoked_at = NOW(),
		    updated_at = NOW()
		WHERE admin_user_id = $1
		  AND status = 'active'
		  AND revoked_at IS NULL
	`, adminUserID)
	return err
}

func validateAdminRoleIDs(ctx context.Context, tx pgx.Tx, roleIDs []string) error {
	items := normalizeAdminStringList(roleIDs)
	if len(items) == 0 {
		return fmt.Errorf("at least one role is required")
	}

	var count int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM admin_roles
		WHERE id = ANY($1)
	`, items).Scan(&count); err != nil {
		return err
	}
	if count != len(items) {
		return fmt.Errorf("one or more roles are invalid")
	}
	return nil
}

func scanAdminIdentity(scan func(dest ...any) error) (*domain.AdminIdentity, error) {
	var item domain.AdminIdentity
	var roleIDs []string
	var roles []string
	var permissions []string
	if err := scan(
		&item.ID,
		&item.Email,
		&item.Name,
		&item.IsActive,
		&item.AuthMode,
		&item.LastLoginAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&roleIDs,
		&roles,
		&permissions,
	); err != nil {
		return nil, err
	}
	item.RoleIDs = normalizeAdminStringList(roleIDs)
	item.Roles = normalizeAdminStringList(roles)
	item.Role = adminPrimaryRole(item.Roles)
	item.Permissions = normalizeAdminStringList(permissions)
	return &item, nil
}

func getAdminIdentityByIDTx(ctx context.Context, tx pgx.Tx, adminID string) (*domain.AdminIdentity, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			au.id,
			au.email,
			au.name,
			au.is_active,
			au.auth_mode,
			au.last_login_at,
			au.created_at,
			au.updated_at,
			COALESCE(ARRAY_AGG(DISTINCT ar.id) FILTER (WHERE ar.id IS NOT NULL), '{}'),
			COALESCE(ARRAY_AGG(DISTINCT ar.name) FILTER (WHERE ar.name IS NOT NULL), '{}'),
			COALESCE(ARRAY_AGG(DISTINCT arp.permission_code) FILTER (WHERE arp.permission_code IS NOT NULL), '{}')
		FROM admin_users au
		LEFT JOIN admin_user_roles aur ON aur.admin_user_id = au.id
		LEFT JOIN admin_roles ar ON ar.id = aur.role_id
		LEFT JOIN admin_role_permissions arp ON arp.role_id = ar.id
		WHERE au.id = $1
		GROUP BY au.id
	`, adminID)

	item, err := scanAdminIdentity(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}
