package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

type AdminAIModelListFilter struct {
	Query    string
	Status   string
	Category string
	AdminPageFilter
}

type AIModelUsageSummary struct {
	SkillCount               int64
	AIJobCount               int64
	SystemConfigDefaultCount int64
	DeviceDefaultCount       int64
}

func (s *Store) ListAdminAIModels(ctx context.Context, filter AdminAIModelListFilter) ([]domain.AIModel, int64, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(vendor ILIKE $%[1]d OR model_name ILIKE $%[1]d OR category ILIKE $%[1]d OR COALESCE(base_url, '') ILIKE $%[1]d)", argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}

	if category := strings.TrimSpace(filter.Category); category != "" {
		whereParts = append(whereParts, fmt.Sprintf("category = $%d", argIndex))
		args = append(args, category)
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
		SELECT
			id, vendor, model_name, category, billing_mode, base_url, api_key, raw_rate, billing_amount,
			description, pricing_payload,
			image_reference_limit, image_supported_sizes,
			video_reference_limit, video_supported_resolutions, video_supported_durations,
			is_enabled, created_at, updated_at
		FROM ai_models
		%s
		ORDER BY updated_at DESC, created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []domain.AIModel
	for rows.Next() {
		item, scanErr := scanAIModel(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, *item)
	}

	return items, total, rows.Err()
}

func (s *Store) GetAIModelByID(ctx context.Context, id string) (*domain.AIModel, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			id, vendor, model_name, category, billing_mode, base_url, api_key, raw_rate, billing_amount,
			description, pricing_payload,
			image_reference_limit, image_supported_sizes,
			video_reference_limit, video_supported_resolutions, video_supported_durations,
			is_enabled, created_at, updated_at
		FROM ai_models
		WHERE id = $1
	`, strings.TrimSpace(id))

	model, err := scanAIModel(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return model, nil
}

type CreateAIModelInput struct {
	ID                        string
	Vendor                    string
	ModelName                 string
	Category                  string
	BillingMode               string
	BaseURL                   *string
	APIKey                    *string
	RawRate                   *float64
	BillingAmount             *float64
	Description               *string
	PricingPayload            []byte
	ImageReferenceLimit       *int
	ImageSupportedSizes       []byte
	VideoReferenceLimit       *int
	VideoSupportedResolutions []byte
	VideoSupportedDurations   []byte
	IsEnabled                 bool
}

func (s *Store) CreateAIModel(ctx context.Context, input CreateAIModelInput) (*domain.AIModel, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO ai_models (
			id, vendor, model_name, category, billing_mode, base_url, api_key, raw_rate, billing_amount,
			description, pricing_payload,
			image_reference_limit, image_supported_sizes,
			video_reference_limit, video_supported_resolutions, video_supported_durations,
			is_enabled, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11,
			$12, $13,
			$14, $15, $16,
			$17, CLOCK_TIMESTAMP(), CLOCK_TIMESTAMP()
		)
		RETURNING
			id, vendor, model_name, category, billing_mode, base_url, api_key, raw_rate, billing_amount,
			description, pricing_payload,
			image_reference_limit, image_supported_sizes,
			video_reference_limit, video_supported_resolutions, video_supported_durations,
			is_enabled, created_at, updated_at
	`,
		input.ID,
		input.Vendor,
		input.ModelName,
		input.Category,
		input.BillingMode,
		input.BaseURL,
		input.APIKey,
		input.RawRate,
		input.BillingAmount,
		input.Description,
		input.PricingPayload,
		input.ImageReferenceLimit,
		input.ImageSupportedSizes,
		input.VideoReferenceLimit,
		input.VideoSupportedResolutions,
		input.VideoSupportedDurations,
		input.IsEnabled,
	)

	return scanAIModel(row)
}

type UpdateAIModelInput struct {
	Vendor                    *string
	ModelName                 *string
	Category                  *string
	BillingMode               *string
	BaseURL                   *string
	APIKey                    *string
	RawRate                   *float64
	BillingAmount             *float64
	Description               *string
	PricingPayload            []byte
	ImageReferenceLimit       *int
	ImageSupportedSizes       *[]string
	VideoReferenceLimit       *int
	VideoSupportedResolutions *[]string
	VideoSupportedDurations   *[]string
	IsEnabled                 *bool
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
	if input.BillingMode != nil {
		setParts = append(setParts, fmt.Sprintf("billing_mode = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.BillingMode))
		argIndex++
	}
	if input.BaseURL != nil {
		setParts = append(setParts, fmt.Sprintf("base_url = $%d", argIndex))
		args = append(args, *input.BaseURL)
		argIndex++
	}
	if input.APIKey != nil {
		setParts = append(setParts, fmt.Sprintf("api_key = NULLIF($%d, '')", argIndex))
		args = append(args, strings.TrimSpace(*input.APIKey))
		argIndex++
	}
	if input.RawRate != nil {
		setParts = append(setParts, fmt.Sprintf("raw_rate = $%d", argIndex))
		args = append(args, *input.RawRate)
		argIndex++
	}
	if input.BillingAmount != nil {
		setParts = append(setParts, fmt.Sprintf("billing_amount = $%d", argIndex))
		args = append(args, *input.BillingAmount)
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
	if input.ImageReferenceLimit != nil {
		setParts = append(setParts, fmt.Sprintf("image_reference_limit = $%d", argIndex))
		args = append(args, *input.ImageReferenceLimit)
		argIndex++
	}
	if input.ImageSupportedSizes != nil {
		setParts = append(setParts, fmt.Sprintf("image_supported_sizes = $%d", argIndex))
		args = append(args, mustJSONBytes(*input.ImageSupportedSizes))
		argIndex++
	}
	if input.VideoReferenceLimit != nil {
		setParts = append(setParts, fmt.Sprintf("video_reference_limit = $%d", argIndex))
		args = append(args, *input.VideoReferenceLimit)
		argIndex++
	}
	if input.VideoSupportedResolutions != nil {
		setParts = append(setParts, fmt.Sprintf("video_supported_resolutions = $%d", argIndex))
		args = append(args, mustJSONBytes(*input.VideoSupportedResolutions))
		argIndex++
	}
	if input.VideoSupportedDurations != nil {
		setParts = append(setParts, fmt.Sprintf("video_supported_durations = $%d", argIndex))
		args = append(args, mustJSONBytes(*input.VideoSupportedDurations))
		argIndex++
	}

	if len(setParts) == 1 {
		// Nothing to update, return the existing model
		row := s.pool.QueryRow(ctx, `
			SELECT
				id, vendor, model_name, category, billing_mode, base_url, api_key, raw_rate, billing_amount,
				description, pricing_payload,
				image_reference_limit, image_supported_sizes,
				video_reference_limit, video_supported_resolutions, video_supported_durations,
				is_enabled, created_at, updated_at
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
		RETURNING
			id, vendor, model_name, category, billing_mode, base_url, api_key, raw_rate, billing_amount,
			description, pricing_payload,
			image_reference_limit, image_supported_sizes,
			video_reference_limit, video_supported_resolutions, video_supported_durations,
			is_enabled, created_at, updated_at
	`, setClause), args...)

	return scanAIModel(row)
}

func (s *Store) GetAIModelUsageSummary(ctx context.Context, id string) (AIModelUsageSummary, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE((SELECT COUNT(*) FROM product_skills WHERE model_name = am.model_name), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM ai_jobs WHERE model_name = am.model_name), 0)::BIGINT,
			COALESCE((
				SELECT COUNT(*)
				FROM admin_system_configs
				WHERE default_chat_model = am.model_name
				   OR default_image_model = am.model_name
				   OR default_video_model = am.model_name
			), 0)::BIGINT,
			COALESCE((
				SELECT COUNT(*)
				FROM devices
				WHERE default_chat_model = am.model_name
				   OR default_image_model = am.model_name
				   OR default_video_model = am.model_name
			), 0)::BIGINT
		FROM ai_models am
		WHERE am.id = $1
	`, strings.TrimSpace(id))

	var summary AIModelUsageSummary
	if err := row.Scan(
		&summary.SkillCount,
		&summary.AIJobCount,
		&summary.SystemConfigDefaultCount,
		&summary.DeviceDefaultCount,
	); err != nil {
		return AIModelUsageSummary{}, err
	}
	return summary, nil
}

func (s *Store) DeleteAIModel(ctx context.Context, id string) (bool, error) {
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM ai_models
		WHERE id = $1
	`, strings.TrimSpace(id))
	if err != nil {
		return false, err
	}
	return commandTag.RowsAffected() > 0, nil
}
