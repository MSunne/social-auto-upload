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

func scanAIModel(row pgx.Row) (*domain.AIModel, error) {
	var model domain.AIModel
	var description *string
	var pricingPayload []byte

	if err := row.Scan(
		&model.ID,
		&model.Vendor,
		&model.ModelName,
		&model.Category,
		&description,
		&pricingPayload,
		&model.IsEnabled,
		&model.CreatedAt,
		&model.UpdatedAt,
	); err != nil {
		return nil, err
	}

	model.Description = description
	model.PricingPayload = bytesOrNil(pricingPayload)
	return &model, nil
}

func (s *Store) ListAIModels(ctx context.Context, category string) ([]domain.AIModel, error) {
	query := `
		SELECT id, vendor, model_name, category, description, pricing_payload, is_enabled, created_at, updated_at
		FROM ai_models
		WHERE is_enabled = TRUE
	`
	args := []any{}
	if strings.TrimSpace(category) != "" {
		query += ` AND category = $1`
		args = append(args, category)
	}
	query += ` ORDER BY category ASC, model_name ASC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AIModel, 0)
	for rows.Next() {
		model, scanErr := scanAIModel(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *model)
	}
	return items, rows.Err()
}

func (s *Store) GetAIModelByName(ctx context.Context, modelName string) (*domain.AIModel, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, vendor, model_name, category, description, pricing_payload, is_enabled, created_at, updated_at
		FROM ai_models
		WHERE model_name = $1
	`, modelName)

	model, err := scanAIModel(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return model, nil
}

func scanAIJob(row pgx.Row) (*domain.AIJob, error) {
	var job domain.AIJob
	var skillID *string
	var prompt *string
	var inputPayload []byte
	var outputPayload []byte
	var message *string
	var finishedAt *time.Time

	if err := row.Scan(
		&job.ID,
		&job.OwnerUserID,
		&skillID,
		&job.JobType,
		&job.ModelName,
		&prompt,
		&job.Status,
		&inputPayload,
		&outputPayload,
		&message,
		&job.CostCredits,
		&job.CreatedAt,
		&job.UpdatedAt,
		&finishedAt,
	); err != nil {
		return nil, err
	}

	job.SkillID = skillID
	job.Prompt = prompt
	job.InputPayload = bytesOrNil(inputPayload)
	job.OutputPayload = bytesOrNil(outputPayload)
	job.Message = message
	job.FinishedAt = finishedAt
	return &job, nil
}

func (s *Store) ListAIJobsByOwner(ctx context.Context, ownerUserID string, filter ListAIJobsFilter) ([]domain.AIJob, error) {
	query := `
		SELECT id, owner_user_id, skill_id, job_type, model_name, prompt, status, input_payload,
		       output_payload, message, cost_credits, created_at, updated_at, finished_at
		FROM ai_jobs
		WHERE owner_user_id = $1
	`
	args := []any{ownerUserID}
	argIndex := 2
	if strings.TrimSpace(filter.JobType) != "" {
		query += fmt.Sprintf(" AND job_type = $%d", argIndex)
		args = append(args, filter.JobType)
		argIndex++
	}
	if strings.TrimSpace(filter.Status) != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, filter.Status)
		argIndex++
	}
	if strings.TrimSpace(filter.SkillID) != "" {
		query += fmt.Sprintf(" AND skill_id = $%d", argIndex)
		args = append(args, filter.SkillID)
		argIndex++
	}
	query += ` ORDER BY updated_at DESC`
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AIJob, 0)
	for rows.Next() {
		job, scanErr := scanAIJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *job)
	}
	return items, rows.Err()
}

func (s *Store) CreateAIJob(ctx context.Context, input CreateAIJobInput) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO ai_jobs (
			id, owner_user_id, skill_id, job_type, model_name, prompt, status, input_payload, message
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, owner_user_id, skill_id, job_type, model_name, prompt, status, input_payload,
		          output_payload, message, cost_credits, created_at, updated_at, finished_at
	`, input.ID, input.OwnerUserID, input.SkillID, input.JobType, input.ModelName, input.Prompt, input.Status, input.InputPayload, input.Message)

	return scanAIJob(row)
}

func (s *Store) GetAIJobByOwner(ctx context.Context, jobID string, ownerUserID string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, owner_user_id, skill_id, job_type, model_name, prompt, status, input_payload,
		       output_payload, message, cost_credits, created_at, updated_at, finished_at
		FROM ai_jobs
		WHERE id = $1 AND owner_user_id = $2
	`, jobID, ownerUserID)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) UpdateAIJob(ctx context.Context, jobID string, ownerUserID string, input UpdateAIJobInput) (*domain.AIJob, error) {
	var skillID any
	if input.SkillTouched {
		skillID = input.SkillID
	}
	var inputPayload any
	if input.InputTouched {
		inputPayload = input.InputPayload
	}
	var outputPayload any
	if input.OutputTouched {
		outputPayload = input.OutputPayload
	}
	var finishedAt any
	if input.FinishedTouched {
		finishedAt = input.FinishedAt
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET skill_id = CASE
		        WHEN $3 = TRUE THEN $4
		        ELSE skill_id
		    END,
		    prompt = COALESCE($5::text, prompt),
		    status = COALESCE($6::text, status),
		    input_payload = CASE
		        WHEN $11 = TRUE THEN $7::jsonb
		        ELSE input_payload
		    END,
		    output_payload = CASE
		        WHEN $12 = TRUE THEN $8::jsonb
		        ELSE output_payload
		    END,
		    message = COALESCE($9::text, message),
		    cost_credits = COALESCE($10::BIGINT, cost_credits),
		    finished_at = CASE
		        WHEN $13 = TRUE THEN $14::timestamptz
		        ELSE finished_at
		    END,
		    updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2
		RETURNING id, owner_user_id, skill_id, job_type, model_name, prompt, status, input_payload,
		          output_payload, message, cost_credits, created_at, updated_at, finished_at
	`, jobID, ownerUserID, input.SkillTouched, skillID, input.Prompt, input.Status, inputPayload, outputPayload, input.Message, input.CostCredits, input.InputTouched, input.OutputTouched, input.FinishedTouched, finishedAt)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) ListAIJobsBySkill(ctx context.Context, ownerUserID string, skillID string, limit int) ([]domain.AIJob, error) {
	query := `
		SELECT id, owner_user_id, skill_id, job_type, model_name, prompt, status, input_payload,
		       output_payload, message, cost_credits, created_at, updated_at, finished_at
		FROM ai_jobs
		WHERE owner_user_id = $1 AND skill_id = $2
		ORDER BY updated_at DESC
	`
	args := []any{ownerUserID, skillID}
	if limit > 0 {
		query += ` LIMIT $3`
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AIJob, 0)
	for rows.Next() {
		job, scanErr := scanAIJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *job)
	}
	return items, rows.Err()
}
