package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

func (s *Store) CreateUser(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO users (id, email, name, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, name, is_active, created_at, updated_at
	`, input.ID, strings.ToLower(input.Email), input.Name, input.PasswordHash)

	var user domain.User
	if err := row.Scan(&user.ID, &user.Email, &user.Name, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*UserWithPassword, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, is_active, created_at, updated_at
		FROM users
		WHERE email = $1
	`, strings.ToLower(email))

	var result UserWithPassword
	if err := row.Scan(
		&result.User.ID,
		&result.User.Email,
		&result.User.Name,
		&result.PasswordHash,
		&result.User.IsActive,
		&result.User.CreatedAt,
		&result.User.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, name, is_active, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id)

	var user domain.User
	if err := row.Scan(&user.ID, &user.Email, &user.Name, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
