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
		whereParts = append(whereParts, fmt.Sprintf("(name ILIKE $%d OR output_type ILIKE $%d OR model_name ILIKE $%d OR COALESCE((SELECT am.model_alias FROM ai_models am WHERE am.model_name = product_skills.model_name LIMIT 1), '') ILIKE $%d)", argIndex, argIndex, argIndex, argIndex))
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
		SELECT id, name, description, output_type, model_name,
		       COALESCE((SELECT am.model_alias FROM ai_models am WHERE am.model_name = product_skills.model_name LIMIT 1), product_skills.model_name) AS model_alias,
		       prompt_template, storyboard_prompt_template, publish_prompt_template, publish_intro_enabled, cover_prompt_template, topics, storyboard_enabled, is_enabled
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
		var topicsPayload []byte
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Description,
			&item.OutputType,
			&item.ModelName,
			&item.ModelAlias,
			&item.PromptTemplate,
			&item.StoryboardPromptTemplate,
			&item.PublishPromptTemplate,
			&item.PublishIntroEnabled,
			&item.CoverPromptTemplate,
			&topicsPayload,
			&item.StoryboardEnabled,
			&item.IsEnabled,
		); err != nil {
			return nil, 0, err
		}
		item.Topics = normalizeSkillTopicsFromJSON(topicsPayload)
		items = append(items, item)
	}

	return items, total, rows.Err()
}

type UpdateProductSkillAdminInput struct {
	Description              *string
	PromptTemplate           *string
	StoryboardPromptTemplate *string
	PublishPromptTemplate    *string
	PublishIntroEnabled      *bool
	Topics                   []string
	TopicsTouched            bool
	StoryboardEnabled        *bool
	IsEnabled                *bool
}

func (s *Store) UpdateProductSkillAdmin(ctx context.Context, id string, input UpdateProductSkillAdminInput) (*domain.ProductSkill, error) {
	setParts := []string{"updated_at = CLOCK_TIMESTAMP()"}
	args := []any{id}
	argIndex := 2

	if input.Description != nil {
		setParts = append(setParts, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.Description))
		argIndex++
	}
	if input.PromptTemplate != nil {
		setParts = append(setParts, fmt.Sprintf("prompt_template = $%d", argIndex))
		args = append(args, trimmedStringOrNil(input.PromptTemplate))
		argIndex++
	}
	if input.StoryboardPromptTemplate != nil {
		setParts = append(setParts, fmt.Sprintf("storyboard_prompt_template = $%d", argIndex))
		args = append(args, trimmedStringOrNil(input.StoryboardPromptTemplate))
		argIndex++
	}
	if input.PublishPromptTemplate != nil {
		setParts = append(setParts, fmt.Sprintf("publish_prompt_template = $%d", argIndex))
		args = append(args, trimmedStringOrNil(input.PublishPromptTemplate))
		argIndex++
	}
	if input.PublishIntroEnabled != nil {
		setParts = append(setParts, fmt.Sprintf("publish_intro_enabled = $%d", argIndex))
		args = append(args, *input.PublishIntroEnabled)
		argIndex++
	}
	if input.TopicsTouched {
		setParts = append(setParts, fmt.Sprintf("topics = $%d", argIndex))
		args = append(args, marshalSkillTopics(input.Topics))
		argIndex++
	}
	if input.StoryboardEnabled != nil {
		setParts = append(setParts, fmt.Sprintf("storyboard_enabled = $%d", argIndex))
		args = append(args, *input.StoryboardEnabled)
		argIndex++
	}
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

func trimmedStringOrNil(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
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
