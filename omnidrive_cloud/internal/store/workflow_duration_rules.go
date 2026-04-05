package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

func scanWorkflowDurationRule(scan scanFn) (*domain.WorkflowDurationRule, error) {
	var item domain.WorkflowDurationRule
	var specialPriceCredits *int64
	var description *string
	if err := scan(
		&item.ID,
		&item.WorkflowCode,
		&item.OutputType,
		&item.DurationSeconds,
		&item.SegmentSeconds,
		&specialPriceCredits,
		&item.IsEnabled,
		&item.SortOrder,
		&description,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.SpecialPriceCredits = specialPriceCredits
	item.Description = trimOptionalString(description)
	return &item, nil
}

func (s *Store) ListWorkflowDurationRules(ctx context.Context) ([]domain.WorkflowDurationRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, workflow_code, output_type, duration_seconds, segment_seconds, special_price_credits,
		       is_enabled, sort_order, description, created_at, updated_at
		FROM workflow_duration_rules
		ORDER BY workflow_code ASC, output_type ASC, sort_order ASC, duration_seconds ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.WorkflowDurationRule, 0)
	for rows.Next() {
		item, scanErr := scanWorkflowDurationRule(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) ListEnabledWorkflowDurationRules(ctx context.Context, workflowCode string, outputType string) ([]domain.WorkflowDurationRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, workflow_code, output_type, duration_seconds, segment_seconds, special_price_credits,
		       is_enabled, sort_order, description, created_at, updated_at
		FROM workflow_duration_rules
		WHERE workflow_code = $1
		  AND output_type = $2
		  AND is_enabled = TRUE
		ORDER BY sort_order ASC, duration_seconds ASC
	`, strings.TrimSpace(workflowCode), strings.TrimSpace(outputType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.WorkflowDurationRule, 0)
	for rows.Next() {
		item, scanErr := scanWorkflowDurationRule(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) GetWorkflowDurationRuleByID(ctx context.Context, ruleID string) (*domain.WorkflowDurationRule, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, workflow_code, output_type, duration_seconds, segment_seconds, special_price_credits,
		       is_enabled, sort_order, description, created_at, updated_at
		FROM workflow_duration_rules
		WHERE id = $1
	`, strings.TrimSpace(ruleID))
	item, err := scanWorkflowDurationRule(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) FindEnabledWorkflowDurationRule(ctx context.Context, workflowCode string, outputType string, durationSeconds int) (*domain.WorkflowDurationRule, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, workflow_code, output_type, duration_seconds, segment_seconds, special_price_credits,
		       is_enabled, sort_order, description, created_at, updated_at
		FROM workflow_duration_rules
		WHERE workflow_code = $1
		  AND output_type = $2
		  AND duration_seconds = $3
		  AND is_enabled = TRUE
		LIMIT 1
	`, strings.TrimSpace(workflowCode), strings.TrimSpace(outputType), durationSeconds)
	item, err := scanWorkflowDurationRule(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) CreateWorkflowDurationRule(ctx context.Context, input CreateWorkflowDurationRuleInput) (*domain.WorkflowDurationRule, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO workflow_duration_rules (
			id, workflow_code, output_type, duration_seconds, segment_seconds, special_price_credits,
			is_enabled, sort_order, description
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, workflow_code, output_type, duration_seconds, segment_seconds, special_price_credits,
		          is_enabled, sort_order, description, created_at, updated_at
	`, strings.TrimSpace(input.ID), strings.TrimSpace(input.WorkflowCode), strings.TrimSpace(input.OutputType),
		input.DurationSeconds, input.SegmentSeconds, input.SpecialPriceCredits, input.IsEnabled, input.SortOrder, trimOptionalString(input.Description))
	return scanWorkflowDurationRule(row.Scan)
}

func (s *Store) UpdateWorkflowDurationRule(ctx context.Context, ruleID string, input UpdateWorkflowDurationRuleInput) (*domain.WorkflowDurationRule, error) {
	var specialPriceCredits any
	if input.SpecialPriceCreditsTouched {
		specialPriceCredits = input.SpecialPriceCredits
	}
	var description any
	if input.DescriptionTouched {
		description = trimOptionalString(input.Description)
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE workflow_duration_rules
		SET workflow_code = COALESCE($2, workflow_code),
		    output_type = COALESCE($3, output_type),
		    duration_seconds = COALESCE($4, duration_seconds),
		    segment_seconds = COALESCE($5, segment_seconds),
		    special_price_credits = CASE
		        WHEN $6 = TRUE THEN $7
		        ELSE special_price_credits
		    END,
		    is_enabled = COALESCE($8, is_enabled),
		    sort_order = COALESCE($9, sort_order),
		    description = CASE
		        WHEN $10 = TRUE THEN $11
		        ELSE description
		    END,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, workflow_code, output_type, duration_seconds, segment_seconds, special_price_credits,
		          is_enabled, sort_order, description, created_at, updated_at
	`, strings.TrimSpace(ruleID), trimmedStringPointer(input.WorkflowCode), trimmedStringPointer(input.OutputType),
		input.DurationSeconds, input.SegmentSeconds, input.SpecialPriceCreditsTouched, specialPriceCredits,
		input.IsEnabled, input.SortOrder, input.DescriptionTouched, description)
	item, err := scanWorkflowDurationRule(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}
