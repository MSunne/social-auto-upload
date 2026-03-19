package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

type AdminSkillListFilter struct {
	Query  string
	Status string // "all", "active", "inactive"
	AdminPageFilter
}

func (s *Store) ListAdminSkills(ctx context.Context, filter AdminSkillListFilter) ([]domain.AdminSkillSummary, int64, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(name ILIKE $%d OR output_type ILIKE $%d OR model_name ILIKE $%d)", argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}

	switch strings.TrimSpace(filter.Status) {
	case "active":
		whereParts = append(whereParts, fmt.Sprintf("is_enabled = $%d", argIndex))
		args = append(args, true)
		argIndex++
	case "inactive":
		whereParts = append(whereParts, fmt.Sprintf("is_enabled = $%d", argIndex))
		args = append(args, false)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM product_skills
		%s
	`, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, output_type, model_name, is_enabled
		FROM product_skills
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []domain.AdminSkillSummary
	for rows.Next() {
		var item domain.AdminSkillSummary
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.OutputType,
			&item.ModelName,
			&item.IsEnabled,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}

type UpdateProductSkillAdminInput struct {
	IsEnabled *bool
}

func (s *Store) UpdateProductSkillAdmin(ctx context.Context, id string, input UpdateProductSkillAdminInput) (*domain.ProductSkill, error) {
	setParts := []string{"updated_at = CLOCK_TIMESTAMP()"}
	args := []any{id}
	argIndex := 2

	if input.IsEnabled != nil {
		setParts = append(setParts, fmt.Sprintf("is_enabled = $%d", argIndex))
		args = append(args, *input.IsEnabled)
		argIndex++
	}

	if len(setParts) == 1 {
		// Nothing to update, return the existing skill
		return s.GetProductSkillByID(ctx, id)
	}

	setClause := strings.Join(setParts, ", ")

	row := s.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE product_skills
		SET %s
		WHERE id = $1
		RETURNING %s
	`, setClause, skillSelectColumns), args...)

	return scanSkill(row)
}

func (s *Store) GetProductSkillByID(ctx context.Context, id string) (*domain.ProductSkill, error) {
	row := s.pool.QueryRow(ctx, skillQueryWithLoad(`
		WHERE id = $1
	`), id)

	skill, err := scanSkillWithLoad(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return skill, nil
}
