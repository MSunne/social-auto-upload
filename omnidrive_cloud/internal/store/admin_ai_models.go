package store

import (
	"context"
	"fmt"
	"strings"

	"omnidrive_cloud/internal/domain"
)

type AdminAIModelListFilter struct {
	Query  string
	Status string
	AdminPageFilter
}

func (s *Store) ListAdminAIModels(ctx context.Context, filter AdminAIModelListFilter) ([]domain.AdminAIModelSummary, int64, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(vendor ILIKE $%d OR model_name ILIKE $%d OR category ILIKE $%d)", argIndex, argIndex, argIndex))
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
		FROM ai_models
		%s
	`, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, vendor, model_name, category, is_enabled
		FROM ai_models
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []domain.AdminAIModelSummary
	for rows.Next() {
		var item domain.AdminAIModelSummary
		if err := rows.Scan(
			&item.ID,
			&item.Vendor,
			&item.ModelName,
			&item.Category,
			&item.IsEnabled,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}

type CreateAIModelInput struct {
	ID             string
	Vendor         string
	ModelName      string
	Category       string
	Description    *string
	PricingPayload []byte
	IsEnabled      bool
}

func (s *Store) CreateAIModel(ctx context.Context, input CreateAIModelInput) (*domain.AIModel, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO ai_models (
			id, vendor, model_name, category, description, pricing_payload, is_enabled, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, CLOCK_TIMESTAMP(), CLOCK_TIMESTAMP()
		)
		RETURNING id, vendor, model_name, category, description, pricing_payload, is_enabled, created_at, updated_at
	`,
		input.ID,
		input.Vendor,
		input.ModelName,
		input.Category,
		input.Description,
		input.PricingPayload,
		input.IsEnabled,
	)

	return scanAIModel(row)
}

type UpdateAIModelInput struct {
	Vendor         *string
	ModelName      *string
	Category       *string
	Description    *string
	PricingPayload []byte
	IsEnabled      *bool
}

func (s *Store) UpdateAIModel(ctx context.Context, id string, input UpdateAIModelInput) (*domain.AIModel, error) {
	setParts := []string{"updated_at = CLOCK_TIMESTAMP()"}
	args := []any{id}
	argIndex := 2

	if input.Vendor != nil {
		setParts = append(setParts, fmt.Sprintf("vendor = $%d", argIndex))
		args = append(args, *input.Vendor)
		argIndex++
	}
	if input.ModelName != nil {
		setParts = append(setParts, fmt.Sprintf("model_name = $%d", argIndex))
		args = append(args, *input.ModelName)
		argIndex++
	}
	if input.Category != nil {
		setParts = append(setParts, fmt.Sprintf("category = $%d", argIndex))
		args = append(args, *input.Category)
		argIndex++
	}
	if input.Description != nil {
		setParts = append(setParts, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, *input.Description)
		argIndex++
	}
	if input.PricingPayload != nil {
		setParts = append(setParts, fmt.Sprintf("pricing_payload = $%d", argIndex))
		args = append(args, input.PricingPayload)
		argIndex++
	}
	if input.IsEnabled != nil {
		setParts = append(setParts, fmt.Sprintf("is_enabled = $%d", argIndex))
		args = append(args, *input.IsEnabled)
		argIndex++
	}

	if len(setParts) == 1 {
		// Nothing to update, return the existing model
		row := s.pool.QueryRow(ctx, `
			SELECT id, vendor, model_name, category, description, pricing_payload, is_enabled, created_at, updated_at
			FROM ai_models
			WHERE id = $1
		`, id)
		return scanAIModel(row)
	}

	setClause := strings.Join(setParts, ", ")

	row := s.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE ai_models
		SET %s
		WHERE id = $1
		RETURNING id, vendor, model_name, category, description, pricing_payload, is_enabled, created_at, updated_at
	`, setClause), args...)

	return scanAIModel(row)
}
