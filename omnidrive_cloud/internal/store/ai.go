package store

import (
	"context"
	"errors"
	"strconv"
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

func scanAIJob(row pgx.Row) (*domain.AIJob, error) {
	var job domain.AIJob
	var prompt *string
	var inputPayload []byte
	var outputPayload []byte
	var message *string
	var finishedAt *time.Time

	if err := row.Scan(
		&job.ID,
		&job.OwnerUserID,
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

	job.Prompt = prompt
	job.InputPayload = bytesOrNil(inputPayload)
	job.OutputPayload = bytesOrNil(outputPayload)
	job.Message = message
	job.FinishedAt = finishedAt
	return &job, nil
}

func (s *Store) ListAIJobsByOwner(ctx context.Context, ownerUserID string, jobType string, status string) ([]domain.AIJob, error) {
	query := `
		SELECT id, owner_user_id, job_type, model_name, prompt, status, input_payload,
		       output_payload, message, cost_credits, created_at, updated_at, finished_at
		FROM ai_jobs
		WHERE owner_user_id = $1
	`
	args := []any{ownerUserID}
	argIndex := 2
	if strings.TrimSpace(jobType) != "" {
		query += ` AND job_type = $` + strconv.Itoa(argIndex)
		args = append(args, jobType)
		argIndex++
	}
	if strings.TrimSpace(status) != "" {
		query += ` AND status = $` + strconv.Itoa(argIndex)
		args = append(args, status)
	}
	query += ` ORDER BY updated_at DESC`

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
			id, owner_user_id, job_type, model_name, prompt, status, input_payload, message
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, owner_user_id, job_type, model_name, prompt, status, input_payload,
		          output_payload, message, cost_credits, created_at, updated_at, finished_at
	`, input.ID, input.OwnerUserID, input.JobType, input.ModelName, input.Prompt, input.Status, input.InputPayload, input.Message)

	return scanAIJob(row)
}

func (s *Store) GetAIJobByOwner(ctx context.Context, jobID string, ownerUserID string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, owner_user_id, job_type, model_name, prompt, status, input_payload,
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
