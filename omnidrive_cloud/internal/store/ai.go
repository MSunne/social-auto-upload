package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

const aiJobSelectColumns = `
	id, owner_user_id, device_id, skill_id, source, local_task_id, job_type, model_name, prompt, status,
	input_payload, output_payload, message, cost_credits, lease_owner_device_id, lease_token, lease_expires_at,
	delivery_status, delivery_message, local_publish_task_id, run_at, created_at, updated_at, delivered_at, finished_at
`

const executableAIJobSourcesSQL = "'omnidrive_cloud', 'omnibull_local', 'openclaw_skill', 'openclaw_main_chat'"

type aiModelPricingPayload struct {
	ChatInputRawRate        *float64 `json:"chatInputRawRate,omitempty"`
	ChatOutputRawRate       *float64 `json:"chatOutputRawRate,omitempty"`
	ChatInputBillingAmount  *float64 `json:"chatInputBillingAmount,omitempty"`
	ChatOutputBillingAmount *float64 `json:"chatOutputBillingAmount,omitempty"`
}

func normalizeAIModelBillingMode(category string, billingMode string) string {
	switch strings.ToLower(strings.TrimSpace(billingMode)) {
	case "per_call", "per_second", "per_token":
		return strings.ToLower(strings.TrimSpace(billingMode))
	default:
		if strings.TrimSpace(category) == "chat" {
			return "per_token"
		}
		return "per_call"
	}
}

func scanAIModel(row pgx.Row) (*domain.AIModel, error) {
	var model domain.AIModel
	var baseURL *string
	var apiKey *string
	var rawRate *float64
	var billingAmount *float64
	var description *string
	var pricingPayload []byte
	var imageReferenceLimit *int
	var imageSupportedSizes []byte
	var videoReferenceLimit *int
	var videoSupportedResolutions []byte
	var videoSupportedDurations []byte

	if err := row.Scan(
		&model.ID,
		&model.Vendor,
		&model.ModelName,
		&model.Category,
		&model.BillingMode,
		&baseURL,
		&apiKey,
		&rawRate,
		&billingAmount,
		&description,
		&pricingPayload,
		&imageReferenceLimit,
		&imageSupportedSizes,
		&videoReferenceLimit,
		&videoSupportedResolutions,
		&videoSupportedDurations,
		&model.IsEnabled,
		&model.CreatedAt,
		&model.UpdatedAt,
	); err != nil {
		return nil, err
	}

	model.BillingMode = normalizeAIModelBillingMode(model.Category, model.BillingMode)
	model.BaseURL = normalizeOptionalString(baseURL)
	model.APIKey = normalizeOptionalString(apiKey)
	model.RawRate = rawRate
	model.BillingAmount = billingAmount
	model.Description = description
	model.PricingPayload = bytesOrNil(pricingPayload)
	applyAIModelPricingPayload(&model)
	model.ImageReferenceLimit = imageReferenceLimit
	model.ImageSupportedSizes = decodeStringList(imageSupportedSizes)
	model.VideoReferenceLimit = videoReferenceLimit
	model.VideoSupportedResolutions = decodeStringList(videoSupportedResolutions)
	model.VideoSupportedDurations = decodeStringList(videoSupportedDurations)
	return &model, nil
}

func applyAIModelPricingPayload(model *domain.AIModel) {
	if model == nil {
		return
	}

	var payload aiModelPricingPayload
	if len(model.PricingPayload) > 0 {
		if err := json.Unmarshal(model.PricingPayload, &payload); err == nil {
			model.ChatInputRawRate = payload.ChatInputRawRate
			model.ChatOutputRawRate = payload.ChatOutputRawRate
			model.ChatInputBillingAmount = payload.ChatInputBillingAmount
			model.ChatOutputBillingAmount = payload.ChatOutputBillingAmount
		}
	}

	if strings.TrimSpace(model.Category) != "chat" {
		return
	}

	if model.ChatInputRawRate == nil && model.RawRate != nil {
		model.ChatInputRawRate = model.RawRate
	}
	if model.ChatOutputRawRate == nil && model.RawRate != nil {
		model.ChatOutputRawRate = model.RawRate
	}
	if model.ChatInputBillingAmount == nil && model.BillingAmount != nil {
		model.ChatInputBillingAmount = model.BillingAmount
	}
	if model.ChatOutputBillingAmount == nil && model.BillingAmount != nil {
		model.ChatOutputBillingAmount = model.BillingAmount
	}
}

func decodeStringList(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return values
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func scanAIJob(row pgx.Row) (*domain.AIJob, error) {
	var job domain.AIJob
	var deviceID *string
	var skillID *string
	var localTaskID *string
	var prompt *string
	var inputPayload []byte
	var outputPayload []byte
	var message *string
	var leaseOwnerDeviceID *string
	var leaseToken *string
	var leaseExpiresAt *time.Time
	var deliveryMessage *string
	var localPublishTaskID *string
	var runAt *time.Time
	var deliveredAt *time.Time
	var finishedAt *time.Time

	if err := row.Scan(
		&job.ID,
		&job.OwnerUserID,
		&deviceID,
		&skillID,
		&job.Source,
		&localTaskID,
		&job.JobType,
		&job.ModelName,
		&prompt,
		&job.Status,
		&inputPayload,
		&outputPayload,
		&message,
		&job.CostCredits,
		&leaseOwnerDeviceID,
		&leaseToken,
		&leaseExpiresAt,
		&job.DeliveryStatus,
		&deliveryMessage,
		&localPublishTaskID,
		&runAt,
		&job.CreatedAt,
		&job.UpdatedAt,
		&deliveredAt,
		&finishedAt,
	); err != nil {
		return nil, err
	}

	job.DeviceID = deviceID
	job.SkillID = skillID
	job.LocalTaskID = localTaskID
	job.Prompt = prompt
	job.InputPayload = bytesOrNil(inputPayload)
	job.OutputPayload = bytesOrNil(outputPayload)
	job.Message = message
	job.LeaseOwnerDeviceID = leaseOwnerDeviceID
	job.LeaseToken = leaseToken
	job.LeaseExpiresAt = leaseExpiresAt
	job.DeliveryMessage = deliveryMessage
	job.LocalPublishTaskID = localPublishTaskID
	job.RunAt = runAt
	job.DeliveredAt = deliveredAt
	job.FinishedAt = finishedAt
	return &job, nil
}

func scanAIJobArtifact(row pgx.Row) (*domain.AIJobArtifact, error) {
	var item domain.AIJobArtifact
	var title *string
	var fileName *string
	var mimeType *string
	var storageKey *string
	var publicURL *string
	var sizeBytes *int64
	var textContent *string
	var deviceID *string
	var rootName *string
	var relativePath *string
	var absolutePath *string
	var payload []byte

	if err := row.Scan(
		&item.ID,
		&item.JobID,
		&item.ArtifactKey,
		&item.ArtifactType,
		&item.Source,
		&title,
		&fileName,
		&mimeType,
		&storageKey,
		&publicURL,
		&sizeBytes,
		&textContent,
		&deviceID,
		&rootName,
		&relativePath,
		&absolutePath,
		&payload,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}

	item.Title = title
	item.FileName = fileName
	item.MimeType = mimeType
	item.StorageKey = storageKey
	item.PublicURL = publicURL
	item.SizeBytes = sizeBytes
	item.TextContent = textContent
	item.DeviceID = deviceID
	item.RootName = rootName
	item.RelativePath = relativePath
	item.AbsolutePath = absolutePath
	item.Payload = bytesOrNil(payload)
	return &item, nil
}

func (s *Store) ListAIModels(ctx context.Context, category string) ([]domain.AIModel, error) {
	query := `
		SELECT
			id, vendor, model_name, category, billing_mode, base_url, api_key, raw_rate, billing_amount,
			description, pricing_payload,
			image_reference_limit, image_supported_sizes,
			video_reference_limit, video_supported_resolutions, video_supported_durations,
			is_enabled, created_at, updated_at
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
		SELECT
			id, vendor, model_name, category, billing_mode, base_url, api_key, raw_rate, billing_amount,
			description, pricing_payload,
			image_reference_limit, image_supported_sizes,
			video_reference_limit, video_supported_resolutions, video_supported_durations,
			is_enabled, created_at, updated_at
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

func (s *Store) ListAIJobsByOwner(ctx context.Context, ownerUserID string, filter ListAIJobsFilter) ([]domain.AIJob, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM ai_jobs
		WHERE owner_user_id = $1
	`, aiJobSelectColumns)
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
	if strings.TrimSpace(filter.DeviceID) != "" {
		query += fmt.Sprintf(" AND device_id = $%d", argIndex)
		args = append(args, filter.DeviceID)
		argIndex++
	}
	if strings.TrimSpace(filter.AccountID) != "" {
		query += fmt.Sprintf(" AND COALESCE(input_payload->>'accountId', '') = $%d", argIndex)
		args = append(args, filter.AccountID)
		argIndex++
	}
	if strings.TrimSpace(filter.Source) != "" {
		query += fmt.Sprintf(" AND source = $%d", argIndex)
		args = append(args, filter.Source)
		argIndex++
	}
	if strings.TrimSpace(filter.ExcludeSource) != "" {
		query += fmt.Sprintf(" AND source <> $%d", argIndex)
		args = append(args, filter.ExcludeSource)
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
			id, owner_user_id, device_id, skill_id, source, local_task_id, job_type, model_name, prompt, status, input_payload, message, run_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING `+aiJobSelectColumns+`
	`, input.ID, input.OwnerUserID, input.DeviceID, input.SkillID, input.Source, input.LocalTaskID, input.JobType, input.ModelName, input.Prompt, input.Status, input.InputPayload, input.Message, input.RunAt)

	return scanAIJob(row)
}

func (s *Store) GetAIJobByOwner(ctx context.Context, jobID string, ownerUserID string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+aiJobSelectColumns+`
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

func (s *Store) GetAIJobByID(ctx context.Context, jobID string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+aiJobSelectColumns+`
		FROM ai_jobs
		WHERE id = $1
	`, jobID)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) GetAIJobByLocalTask(ctx context.Context, ownerUserID string, deviceID string, localTaskID string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+aiJobSelectColumns+`
		FROM ai_jobs
		WHERE owner_user_id = $1
		  AND device_id = $2
		  AND local_task_id = $3
		ORDER BY updated_at DESC
		LIMIT 1
	`, ownerUserID, deviceID, localTaskID)

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
	var deviceID any
	if input.DeviceTouched {
		deviceID = input.DeviceID
	}
	var skillID any
	if input.SkillTouched {
		skillID = input.SkillID
	}
	var localTaskID any
	if input.LocalTaskTouched {
		localTaskID = input.LocalTaskID
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
	var localPublishTaskID any
	if input.LocalPublishTaskTouched {
		localPublishTaskID = input.LocalPublishTaskID
	}
	var runAt any
	if input.RunAtTouched {
		runAt = input.RunAt
	}
	var deliveredAt any
	if input.DeliveredTouched {
		deliveredAt = input.DeliveredAt
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET device_id = CASE
		        WHEN $3 = TRUE THEN $4
		        ELSE device_id
		    END,
		    skill_id = CASE
		        WHEN $5 = TRUE THEN $6
		        ELSE skill_id
		    END,
		    local_task_id = CASE
		        WHEN $7 = TRUE THEN $8
		        ELSE local_task_id
		    END,
		    prompt = COALESCE($9::text, prompt),
		    status = COALESCE($10::text, status),
		    input_payload = CASE
		        WHEN $15 = TRUE THEN $11::jsonb
		        ELSE input_payload
		    END,
		    output_payload = CASE
		        WHEN $16 = TRUE THEN $12::jsonb
		        ELSE output_payload
		    END,
		    message = COALESCE($13::text, message),
		    cost_credits = COALESCE($14::BIGINT, cost_credits),
		    delivery_status = COALESCE($17::text, delivery_status),
		    delivery_message = COALESCE($18::text, delivery_message),
		    local_publish_task_id = CASE
		        WHEN $19 = TRUE THEN $20
		        ELSE local_publish_task_id
		    END,
		    run_at = CASE
		        WHEN $21 = TRUE THEN $22::timestamptz
		        ELSE run_at
		    END,
		    delivered_at = CASE
		        WHEN $25 = TRUE THEN $26::timestamptz
		        ELSE delivered_at
		    END,
		    finished_at = CASE
		        WHEN $23 = TRUE THEN $24::timestamptz
		        ELSE finished_at
		    END,
		    updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2
		RETURNING `+aiJobSelectColumns+`
	`, jobID, ownerUserID, input.DeviceTouched, deviceID, input.SkillTouched, skillID, input.LocalTaskTouched, localTaskID, input.Prompt, input.Status, inputPayload, outputPayload, input.Message, input.CostCredits, input.InputTouched, input.OutputTouched, input.DeliveryStatus, input.DeliveryMessage, input.LocalPublishTaskTouched, localPublishTaskID, input.RunAtTouched, runAt, input.FinishedTouched, finishedAt, input.DeliveredTouched, deliveredAt)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) UpdateAIJobDeliveryByDevice(ctx context.Context, jobID string, deviceID string, status string, message *string, localPublishTaskID *string, deliveredAt *time.Time) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET delivery_status = COALESCE($3::text, delivery_status),
		    delivery_message = COALESCE($4::text, delivery_message),
		    local_publish_task_id = COALESCE($5::text, local_publish_task_id),
		    delivered_at = COALESCE($6::timestamptz, delivered_at),
		    updated_at = NOW()
		WHERE id = $1
		  AND device_id = $2
		RETURNING `+aiJobSelectColumns+`
	`, jobID, deviceID, status, message, localPublishTaskID, deliveredAt)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) CancelAIJob(ctx context.Context, jobID string, ownerUserID string, message *string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET status = 'cancelled',
		    message = COALESCE($3, 'AI 任务已取消'),
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    finished_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2 AND status IN ('queued', 'running')
		RETURNING `+aiJobSelectColumns+`
	`, jobID, ownerUserID, message)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) RetryAIJob(ctx context.Context, jobID string, ownerUserID string, message *string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET status = 'queued',
		    message = COALESCE($3, 'AI 任务已重新排队'),
		    output_payload = NULL,
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    finished_at = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2 AND status IN ('failed', 'cancelled', 'success', 'completed')
		RETURNING `+aiJobSelectColumns+`
	`, jobID, ownerUserID, message)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) ForceReleaseAIJobLeaseByOwner(ctx context.Context, jobID string, ownerUserID string, message *string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET status = 'queued',
		    message = COALESCE($3, 'AI 任务租约已由云端手动释放并重新排队'),
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    finished_at = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2 AND status = 'running'
		RETURNING `+aiJobSelectColumns+`
	`, jobID, ownerUserID, message)

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
		SELECT ` + aiJobSelectColumns + `
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

func (s *Store) ListPendingAIJobsByDevice(ctx context.Context, deviceID string) ([]domain.AIJob, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+aiJobSelectColumns+`
		FROM ai_jobs
		WHERE device_id = $1
		  AND (
		      (status = 'queued' AND (lease_expires_at IS NULL OR lease_expires_at < NOW()))
		      OR (status = 'running' AND lease_owner_device_id = $1 AND lease_expires_at >= NOW())
		  )
		ORDER BY created_at ASC
	`, deviceID)
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

func (s *Store) ListAgentAIJobsByDevice(ctx context.Context, deviceID string, source string, limit int) ([]domain.AIJob, error) {
	query := `
		SELECT ` + aiJobSelectColumns + `
		FROM ai_jobs
		WHERE device_id = $1
	`
	args := []any{deviceID}
	argIndex := 2
	if strings.TrimSpace(source) != "" {
		query += fmt.Sprintf(" AND source = $%d", argIndex)
		args = append(args, source)
		argIndex++
	}
	query += ` ORDER BY updated_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
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

func (s *Store) ListExecutableAIJobs(ctx context.Context, limit int) ([]domain.AIJob, error) {
	query := `
		SELECT ` + aiJobSelectColumns + `
		FROM ai_jobs
		WHERE status = 'queued'
		  AND source IN (` + executableAIJobSourcesSQL + `)
		  AND (run_at IS NULL OR run_at <= NOW())
		  AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
		ORDER BY created_at ASC
	`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT $1`
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

func (s *Store) PromoteDueScheduledAIJobs(ctx context.Context, limit int) ([]domain.AIJob, error) {
	query := `
		UPDATE ai_jobs
		SET status = 'queued',
		    message = CASE
		        WHEN COALESCE(message, '') = '' THEN '已到执行时间，等待云端生成'
		        ELSE message
		    END,
		    updated_at = NOW()
		WHERE id IN (
		    SELECT id
		    FROM ai_jobs
		    WHERE status = 'scheduled'
		      AND (run_at IS NULL OR run_at <= NOW())
		    ORDER BY run_at ASC NULLS FIRST, created_at ASC
	`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT $1`
		args = append(args, limit)
	}
	query += `
		)
		RETURNING ` + aiJobSelectColumns

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

func (s *Store) FindScheduledOrActiveAIJobBySkillRun(ctx context.Context, skillID string, runAt time.Time) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+aiJobSelectColumns+`
		FROM ai_jobs
		WHERE skill_id = $1
		  AND run_at = $2
		  AND status IN ('scheduled', 'queued', 'running')
		ORDER BY created_at DESC
		LIMIT 1
	`, skillID, runAt.UTC())

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) RecoverExpiredAIJobLeases(ctx context.Context, deviceID string) ([]domain.AIJob, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE ai_jobs
		SET status = 'queued',
		    message = 'AI 任务租约超时，已重新排队',
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    finished_at = NULL,
		    updated_at = NOW()
		WHERE device_id = $1
		  AND status = 'running'
		  AND lease_owner_device_id IS NOT NULL
		  AND lease_token IS NOT NULL
		  AND lease_expires_at < NOW()
		RETURNING `+aiJobSelectColumns+`
	`, deviceID)
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

func (s *Store) RecoverExpiredExecutableAIJobLeases(ctx context.Context) ([]domain.AIJob, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE ai_jobs
		SET status = 'queued',
		    message = 'AI 任务租约超时，已重新排队',
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    finished_at = NULL,
		    updated_at = NOW()
		WHERE source IN (`+executableAIJobSourcesSQL+`)
		  AND status = 'running'
		  AND lease_token IS NOT NULL
		  AND lease_expires_at IS NOT NULL
		  AND lease_expires_at < NOW()
		RETURNING `+aiJobSelectColumns+`
	`)
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

func (s *Store) ClaimAIJobLease(ctx context.Context, jobID string, deviceID string, leaseToken string, leaseExpiresAt time.Time) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET status = 'running',
		    lease_owner_device_id = $2,
		    lease_token = $3,
		    lease_expires_at = $4,
		    updated_at = NOW()
		WHERE id = $1
		  AND device_id = $2
		  AND status = 'queued'
		  AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
		RETURNING `+aiJobSelectColumns+`
	`, jobID, deviceID, leaseToken, leaseExpiresAt)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) ClaimCloudAIJobLease(ctx context.Context, jobID string, leaseToken string, leaseExpiresAt time.Time) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET status = 'running',
		    lease_owner_device_id = NULL,
		    lease_token = $2,
		    lease_expires_at = $3,
		    updated_at = NOW()
		WHERE id = $1
		  AND source IN (`+executableAIJobSourcesSQL+`)
		  AND status = 'queued'
		  AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
		RETURNING `+aiJobSelectColumns+`
	`, jobID, leaseToken, leaseExpiresAt)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) RenewAIJobLease(ctx context.Context, jobID string, deviceID string, leaseToken string, leaseExpiresAt time.Time) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET lease_expires_at = $4,
		    updated_at = NOW()
		WHERE id = $1
		  AND device_id = $2
		  AND lease_owner_device_id = $2
		  AND lease_token = $3
		  AND status = 'running'
		RETURNING `+aiJobSelectColumns+`
	`, jobID, deviceID, leaseToken, leaseExpiresAt)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) RenewCloudAIJobLease(ctx context.Context, jobID string, leaseToken string, leaseExpiresAt time.Time) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET lease_expires_at = $3,
		    updated_at = NOW()
		WHERE id = $1
		  AND lease_owner_device_id IS NULL
		  AND lease_token = $2
		  AND status = 'running'
		RETURNING `+aiJobSelectColumns+`
	`, jobID, leaseToken, leaseExpiresAt)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) ReleaseAIJobLeaseByAgent(ctx context.Context, jobID string, deviceID string, leaseToken string, message *string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET status = 'queued',
		    message = COALESCE($4, '本地设备已释放 AI 任务租约并重新排队'),
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    finished_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND device_id = $2
		  AND lease_owner_device_id = $2
		  AND lease_token = $3
		  AND status = 'running'
		RETURNING `+aiJobSelectColumns+`
	`, jobID, deviceID, leaseToken, message)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) SyncCloudAIJobExecution(ctx context.Context, jobID string, leaseToken string, input UpdateAIJobInput) (*domain.AIJob, error) {
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
		SET status = COALESCE($3::text, status),
		    output_payload = CASE
		        WHEN $7 = TRUE THEN $4::jsonb
		        ELSE output_payload
		    END,
		    message = COALESCE($5::text, message),
		    cost_credits = COALESCE($6::BIGINT, cost_credits),
		    lease_owner_device_id = CASE
		        WHEN COALESCE($3::text, status) = 'running' THEN lease_owner_device_id
		        ELSE NULL
		    END,
		    lease_token = CASE
		        WHEN COALESCE($3::text, status) = 'running' THEN lease_token
		        ELSE NULL
		    END,
		    lease_expires_at = CASE
		        WHEN COALESCE($3::text, status) = 'running' THEN lease_expires_at
		        ELSE NULL
		    END,
		    finished_at = CASE
		        WHEN $8 = TRUE THEN $9::timestamptz
		        WHEN COALESCE($3::text, status) IN ('success', 'completed', 'failed', 'cancelled') THEN NOW()
		        ELSE finished_at
		    END,
		    updated_at = NOW()
		WHERE id = $1
		  AND source IN (`+executableAIJobSourcesSQL+`)
		  AND lease_owner_device_id IS NULL
		  AND lease_token = $2
		  AND status = 'running'
		RETURNING `+aiJobSelectColumns+`
	`, jobID, leaseToken, input.Status, outputPayload, input.Message, input.CostCredits, input.OutputTouched, input.FinishedTouched, finishedAt)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) SyncAIJobExecution(ctx context.Context, jobID string, deviceID string, leaseToken string, input UpdateAIJobInput) (*domain.AIJob, error) {
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
		SET status = COALESCE($4::text, status),
		    output_payload = CASE
		        WHEN $8 = TRUE THEN $5::jsonb
		        ELSE output_payload
		    END,
		    message = COALESCE($6::text, message),
		    cost_credits = COALESCE($7::BIGINT, cost_credits),
		    lease_owner_device_id = CASE
		        WHEN COALESCE($4::text, status) = 'running' THEN lease_owner_device_id
		        ELSE NULL
		    END,
		    lease_token = CASE
		        WHEN COALESCE($4::text, status) = 'running' THEN lease_token
		        ELSE NULL
		    END,
		    lease_expires_at = CASE
		        WHEN COALESCE($4::text, status) = 'running' THEN lease_expires_at
		        ELSE NULL
		    END,
		    finished_at = CASE
		        WHEN $9 = TRUE THEN $10::timestamptz
		        WHEN COALESCE($4::text, status) IN ('success', 'completed', 'failed', 'cancelled') THEN NOW()
		        ELSE finished_at
		    END,
		    updated_at = NOW()
		WHERE id = $1
		  AND device_id = $2
		  AND lease_owner_device_id = $2
		  AND lease_token = $3
		  AND status = 'running'
		RETURNING `+aiJobSelectColumns+`
	`, jobID, deviceID, leaseToken, input.Status, outputPayload, input.Message, input.CostCredits, input.OutputTouched, input.FinishedTouched, finishedAt)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) UpsertAIJobArtifacts(ctx context.Context, items []UpsertAIJobArtifactInput) ([]domain.AIJobArtifact, error) {
	if len(items) == 0 {
		return []domain.AIJobArtifact{}, nil
	}

	result := make([]domain.AIJobArtifact, 0, len(items))
	for _, item := range items {
		row := s.pool.QueryRow(ctx, `
			INSERT INTO ai_job_artifacts (
				id, job_id, artifact_key, artifact_type, source, title, file_name, mime_type, storage_key,
				public_url, size_bytes, text_content, device_id, root_name, relative_path, absolute_path, payload
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			ON CONFLICT (job_id, artifact_key) DO UPDATE
			SET artifact_type = EXCLUDED.artifact_type,
			    source = EXCLUDED.source,
			    title = EXCLUDED.title,
			    file_name = EXCLUDED.file_name,
			    mime_type = EXCLUDED.mime_type,
			    storage_key = EXCLUDED.storage_key,
			    public_url = EXCLUDED.public_url,
			    size_bytes = EXCLUDED.size_bytes,
			    text_content = EXCLUDED.text_content,
			    device_id = EXCLUDED.device_id,
			    root_name = EXCLUDED.root_name,
			    relative_path = EXCLUDED.relative_path,
			    absolute_path = EXCLUDED.absolute_path,
			    payload = EXCLUDED.payload,
			    updated_at = NOW()
			RETURNING id, job_id, artifact_key, artifact_type, source, title, file_name, mime_type, storage_key,
			          public_url, size_bytes, text_content, device_id, root_name, relative_path, absolute_path, payload,
			          created_at, updated_at
		`, uuid.NewString(), item.JobID, item.ArtifactKey, item.ArtifactType, item.Source, item.Title, item.FileName, item.MimeType, item.StorageKey,
			item.PublicURL, item.SizeBytes, item.TextContent, item.DeviceID, item.RootName, item.RelativePath, item.AbsolutePath, item.Payload)

		artifact, err := scanAIJobArtifact(row)
		if err != nil {
			return nil, err
		}
		result = append(result, *artifact)
	}
	return result, nil
}

func (s *Store) ListAIJobArtifactsByOwner(ctx context.Context, jobID string, ownerUserID string) ([]domain.AIJobArtifact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.job_id, a.artifact_key, a.artifact_type, a.source, a.title, a.file_name, a.mime_type,
		       a.storage_key, a.public_url, a.size_bytes, a.text_content, a.device_id, a.root_name, a.relative_path,
		       a.absolute_path, a.payload, a.created_at, a.updated_at
		FROM ai_job_artifacts a
		INNER JOIN ai_jobs j ON j.id = a.job_id
		WHERE a.job_id = $1 AND j.owner_user_id = $2
		ORDER BY a.created_at ASC
	`, jobID, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AIJobArtifact, 0)
	for rows.Next() {
		artifact, scanErr := scanAIJobArtifact(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *artifact)
	}
	return items, rows.Err()
}

func (s *Store) ListAIJobArtifactsByJobID(ctx context.Context, jobID string) ([]domain.AIJobArtifact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, job_id, artifact_key, artifact_type, source, title, file_name, mime_type, storage_key,
		       public_url, size_bytes, text_content, device_id, root_name, relative_path, absolute_path, payload,
		       created_at, updated_at
		FROM ai_job_artifacts
		WHERE job_id = $1
		ORDER BY created_at ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AIJobArtifact, 0)
	for rows.Next() {
		artifact, scanErr := scanAIJobArtifact(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *artifact)
	}
	return items, rows.Err()
}

func (s *Store) DeleteAIJobArtifactsByOwner(ctx context.Context, jobID string, ownerUserID string) (int64, error) {
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM ai_job_artifacts a
		USING ai_jobs j
		WHERE a.job_id = j.id
		  AND a.job_id = $1
		  AND j.owner_user_id = $2
	`, jobID, ownerUserID)
	if err != nil {
		return 0, err
	}
	return commandTag.RowsAffected(), nil
}

func (s *Store) LinkAIJobToPublishTask(ctx context.Context, input LinkAIJobPublishTaskInput) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ai_job_publish_links (job_id, task_id, owner_user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (job_id, task_id) DO NOTHING
	`, input.JobID, input.TaskID, input.OwnerUserID)
	return err
}

func (s *Store) ListPublishTasksByAIJobOwner(ctx context.Context, jobID string, ownerUserID string, limit int) ([]domain.PublishTask, error) {
	query := `
		SELECT pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		       pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.verification_payload,
		       pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at, pt.attempt_count, pt.cancel_requested_at,
		       pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
		FROM ai_job_publish_links l
		INNER JOIN publish_tasks pt ON pt.id = l.task_id
		WHERE l.job_id = $1 AND l.owner_user_id = $2
		ORDER BY l.created_at DESC
	`
	args := []any{jobID, ownerUserID}
	if limit > 0 {
		query += ` LIMIT $3`
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.PublishTask, 0)
	for rows.Next() {
		task, scanErr := scanPublishTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *task)
	}
	return items, rows.Err()
}
