package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

type UpsertAdminPermissionInput struct {
	Code        string
	Name        string
	Description string
	Category    string
}

type UpsertAdminRoleInput struct {
	ID              string
	Name            string
	Description     *string
	IsSystem        bool
	PermissionCodes []string
}

type CreateAdminRoleInput struct {
	ID              string
	Name            string
	Description     *string
	PermissionCodes []string
}

// 确保管理端RBACCatalog已满足执行前提，必要时补齐缺失状态或配置。
func (s *Store) EnsureAdminRBACCatalog(ctx context.Context, permissions []UpsertAdminPermissionInput, roles []UpsertAdminRoleInput) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, permission := range permissions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO admin_permissions (code, name, description, category, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
			ON CONFLICT (code) DO UPDATE
			SET name = EXCLUDED.name,
			    description = EXCLUDED.description,
			    category = EXCLUDED.category,
			    updated_at = NOW()
		`, permission.Code, permission.Name, permission.Description, permission.Category); err != nil {
			return err
		}
	}

	for _, role := range roles {
		if _, err := tx.Exec(ctx, `
			INSERT INTO admin_roles (id, name, description, is_system, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
			ON CONFLICT (id) DO UPDATE
			SET name = EXCLUDED.name,
			    description = EXCLUDED.description,
			    is_system = EXCLUDED.is_system,
			    updated_at = NOW()
		`, role.ID, role.Name, role.Description, role.IsSystem); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `DELETE FROM admin_role_permissions WHERE role_id = $1`, role.ID); err != nil {
			return err
		}
		for _, code := range normalizeAdminStringList(role.PermissionCodes) {
			if _, err := tx.Exec(ctx, `
				INSERT INTO admin_role_permissions (role_id, permission_code, created_at)
				VALUES ($1, $2, NOW())
				ON CONFLICT (role_id, permission_code) DO NOTHING
			`, role.ID, code); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListAdminPermissions(ctx context.Context) ([]domain.AdminPermission, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT code, name, description, category
		FROM admin_permissions
		ORDER BY category ASC, code ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AdminPermission, 0)
	for rows.Next() {
		var item domain.AdminPermission
		if err := rows.Scan(&item.Code, &item.Name, &item.Description, &item.Category); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListAdminRoles(ctx context.Context) ([]domain.AdminRole, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			ar.id,
			ar.name,
			ar.description,
			ar.is_system,
			ar.created_at,
			ar.updated_at,
			COUNT(DISTINCT aur.admin_user_id)::BIGINT,
			COALESCE(ARRAY_AGG(DISTINCT arp.permission_code) FILTER (WHERE arp.permission_code IS NOT NULL), '{}')
		FROM admin_roles ar
		LEFT JOIN admin_user_roles aur ON aur.role_id = ar.id
		LEFT JOIN admin_role_permissions arp ON arp.role_id = ar.id
		GROUP BY ar.id
		ORDER BY ar.is_system DESC, ar.name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AdminRole, 0)
	for rows.Next() {
		var item domain.AdminRole
		var permissions []string
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Description,
			&item.IsSystem,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.AdminCount,
			&permissions,
		); err != nil {
			return nil, err
		}
		item.Permissions = normalizeAdminStringList(permissions)
		item.PermissionCount = len(item.Permissions)
		items = append(items, item)
	}
	return items, rows.Err()
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) CreateAdminRole(ctx context.Context, input CreateAdminRoleInput) (*domain.AdminRole, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := validateAdminPermissionCodes(ctx, tx, input.PermissionCodes); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_roles (id, name, description, is_system, created_at, updated_at)
		VALUES ($1, $2, $3, FALSE, NOW(), NOW())
	`, input.ID, input.Name, input.Description); err != nil {
		return nil, err
	}

	for _, code := range normalizeAdminStringList(input.PermissionCodes) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO admin_role_permissions (role_id, permission_code, created_at)
			VALUES ($1, $2, NOW())
		`, input.ID, code); err != nil {
			return nil, err
		}
	}

	item, err := getAdminRoleByIDTx(ctx, tx, input.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return item, nil
}

// 处理校验管理端权限Codes相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func validateAdminPermissionCodes(ctx context.Context, tx pgx.Tx, permissionCodes []string) error {
	codes := normalizeAdminStringList(permissionCodes)
	if len(codes) == 0 {
		return fmt.Errorf("at least one permission is required")
	}

	var count int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM admin_permissions
		WHERE code = ANY($1)
	`, codes).Scan(&count); err != nil {
		return err
	}
	if count != len(codes) {
		return fmt.Errorf("one or more permissions are invalid")
	}
	return nil
}

// 获取管理端角色ID事务，为当前链路返回后续处理所需的数据内容。
func getAdminRoleByIDTx(ctx context.Context, tx pgx.Tx, roleID string) (*domain.AdminRole, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			ar.id,
			ar.name,
			ar.description,
			ar.is_system,
			ar.created_at,
			ar.updated_at,
			COUNT(DISTINCT aur.admin_user_id)::BIGINT,
			COALESCE(ARRAY_AGG(DISTINCT arp.permission_code) FILTER (WHERE arp.permission_code IS NOT NULL), '{}')
		FROM admin_roles ar
		LEFT JOIN admin_user_roles aur ON aur.role_id = ar.id
		LEFT JOIN admin_role_permissions arp ON arp.role_id = ar.id
		WHERE ar.id = $1
		GROUP BY ar.id
	`, roleID)

	var item domain.AdminRole
	var permissions []string
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Description,
		&item.IsSystem,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.AdminCount,
		&permissions,
	); err != nil {
		return nil, err
	}
	item.Permissions = normalizeAdminStringList(permissions)
	item.PermissionCount = len(item.Permissions)
	return &item, nil
}
