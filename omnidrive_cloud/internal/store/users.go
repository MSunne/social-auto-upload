package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

const userSelectColumns = `
	id,
	COALESCE(email, phone, '') AS email,
	COALESCE(phone, '') AS phone,
	name,
	is_active,
	created_at,
	updated_at
`

const updateDevelopmentSeedUserSQL = `
	UPDATE users
	SET
		email = CASE WHEN $2::text IS NULL THEN email ELSE NULLIF($2::text, '') END,
		phone = CASE WHEN $3::text IS NULL THEN phone ELSE NULLIF($3::text, '') END,
		name = CASE WHEN $4::text IS NULL OR NULLIF($4::text, '') IS NULL THEN name ELSE $4::text END,
		password_hash = COALESCE($5::text, password_hash),
		is_active = COALESCE($6::boolean, is_active),
		updated_at = NOW()
	WHERE id = $1
	RETURNING ` + userSelectColumns

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) CreateUser(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	var email any
	if trimmed := strings.ToLower(strings.TrimSpace(input.Email)); trimmed != "" {
		email = trimmed
	}
	var phone any
	if trimmed := strings.TrimSpace(input.Phone); trimmed != "" {
		phone = trimmed
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+userSelectColumns+`
	`, input.ID, email, phone, input.Name, input.PasswordHash)

	var user domain.User
	if err := row.Scan(&user.ID, &user.Email, &user.Phone, &user.Name, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return nil, err
	}
	return &user, nil
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*UserWithPassword, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, nil
	}

	row := s.pool.QueryRow(ctx, `
		SELECT `+userSelectColumns+`, password_hash
		FROM users
		WHERE email = $1
	`, email)

	var result UserWithPassword
	if err := row.Scan(
		&result.User.ID,
		&result.User.Email,
		&result.User.Phone,
		&result.User.Name,
		&result.User.IsActive,
		&result.User.CreatedAt,
		&result.User.UpdatedAt,
		&result.PasswordHash,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetUserByPhone(ctx context.Context, phone string) (*UserWithPassword, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil, nil
	}

	row := s.pool.QueryRow(ctx, `
		SELECT `+userSelectColumns+`, password_hash
		FROM users
		WHERE phone = $1
	`, phone)

	var result UserWithPassword
	if err := row.Scan(
		&result.User.ID,
		&result.User.Email,
		&result.User.Phone,
		&result.User.Name,
		&result.User.IsActive,
		&result.User.CreatedAt,
		&result.User.UpdatedAt,
		&result.PasswordHash,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+userSelectColumns+`
		FROM users
		WHERE id = $1
	`, id)

	var user domain.User
	if err := row.Scan(&user.ID, &user.Email, &user.Phone, &user.Name, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// 执行存储层相关的数据库写入，确保开发环境演示账号与预置口径保持一致。
func (s *Store) UpdateDevelopmentSeedUser(ctx context.Context, id string, input UpdateDevelopmentSeedUserInput) (*domain.User, error) {
	var email any
	if input.Email != nil {
		email = strings.ToLower(strings.TrimSpace(*input.Email))
	}
	var phone any
	if input.Phone != nil {
		phone = strings.TrimSpace(*input.Phone)
	}
	var name any
	if input.Name != nil {
		name = strings.TrimSpace(*input.Name)
	}

	row := s.pool.QueryRow(ctx, `
		`+updateDevelopmentSeedUserSQL, id, email, phone, name, input.PasswordHash, input.IsActive)

	var user domain.User
	if err := row.Scan(&user.ID, &user.Email, &user.Phone, &user.Name, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
