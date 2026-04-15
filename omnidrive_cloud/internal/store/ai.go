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

// 处理AI作业QualifiedColumn相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func aiJobQualifiedColumn(alias string, column string) string {
	trimmedAlias := strings.TrimSpace(alias)
	if trimmedAlias == "" {
		return column
	}
	return trimmedAlias + "." + column
}

func aiJobAccountIDExpression(alias string) string {
	qualified := aiJobQualifiedColumn(alias, "input_payload")
	return fmt.Sprintf(
		"COALESCE(NULLIF(TRIM(%s->>'accountId'), ''), NULLIF(TRIM(%s->'publishPayload'->'targets'->0->>'accountId'), ''))",
		qualified,
		qualified,
	)
}

const (
	aiJobPayloadModeFull          = "full"
	aiJobPayloadModeSummary       = "summary"
	aiJobPayloadModeHistoryDetail = "history_detail"
	aiJobArtifactModePreview      = "preview"
	aiJobArtifactModeChat         = "chat"
	aiJobSummaryPromptLimit       = 160
)

// 处理AI作业列表载荷Mode相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func aiJobListPayloadMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return aiJobPayloadModeSummary
	case aiJobPayloadModeFull:
		return aiJobPayloadModeFull
	case aiJobPayloadModeSummary:
		return aiJobPayloadModeSummary
	default:
		return aiJobPayloadModeSummary
	}
}

func aiJobDetailPayloadMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", aiJobPayloadModeFull:
		return aiJobPayloadModeFull
	case aiJobPayloadModeSummary:
		return aiJobPayloadModeSummary
	case aiJobPayloadModeHistoryDetail:
		return aiJobPayloadModeHistoryDetail
	default:
		return aiJobPayloadModeFull
	}
}

func aiJobArtifactMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", aiJobPayloadModeFull:
		return aiJobPayloadModeFull
	case aiJobArtifactModePreview:
		return aiJobArtifactModePreview
	case aiJobArtifactModeChat:
		return aiJobArtifactModeChat
	default:
		return aiJobPayloadModeFull
	}
}

// 处理AI作业输入载荷SelectColumn相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func aiJobInputPayloadSelectColumn(alias string, payloadMode string) string {
	qualified := aiJobQualifiedColumn(alias, "input_payload")
	jobType := aiJobQualifiedColumn(alias, "job_type")
	switch strings.ToLower(strings.TrimSpace(payloadMode)) {
	case aiJobPayloadModeHistoryDetail:
		return fmt.Sprintf(
			`CASE
				WHEN %s = 'chat' THEN jsonb_strip_nulls(jsonb_build_object(
					'messages', %s->'messages',
					'attachments', %s->'attachments',
					'skillName', %s->'skillName',
					'conversationId', %s->'conversationId'
				))
				WHEN %s IN ('video', 'image') THEN jsonb_strip_nulls(jsonb_build_object(
					'prompt', %s->'prompt',
					'skillName', %s->'skillName'
				))
				ELSE jsonb_strip_nulls(jsonb_build_object(
					'skillName', %s->'skillName',
					'conversationId', %s->'conversationId'
				))
			END AS input_payload`,
			jobType,
			qualified,
			qualified,
			qualified,
			qualified,
			jobType,
			qualified,
			qualified,
			qualified,
			qualified,
		)
	}
	if aiJobListPayloadMode(payloadMode) != aiJobPayloadModeSummary {
		return qualified
	}
	return fmt.Sprintf(
		`jsonb_strip_nulls(jsonb_build_object('skillName', %s->'skillName', 'conversationId', %s->'conversationId')) AS input_payload`,
		qualified,
		qualified,
	)
}

// 处理AI作业提示词SelectColumn相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func aiJobPromptSelectColumn(alias string, payloadMode string) string {
	qualified := aiJobQualifiedColumn(alias, "prompt")
	if aiJobListPayloadMode(payloadMode) != aiJobPayloadModeSummary {
		return qualified
	}
	return fmt.Sprintf(
		`CASE WHEN %s IS NULL THEN NULL ELSE LEFT(%s, %d) END AS prompt`,
		qualified,
		qualified,
		aiJobSummaryPromptLimit,
	)
}

// 处理AI作业输出载荷SelectColumn相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func aiJobOutputPayloadSelectColumn(alias string, payloadMode string) string {
	qualified := aiJobQualifiedColumn(alias, "output_payload")
	jobType := aiJobQualifiedColumn(alias, "job_type")
	switch strings.ToLower(strings.TrimSpace(payloadMode)) {
	case aiJobPayloadModeHistoryDetail:
		return fmt.Sprintf(
			`CASE
				WHEN %s = 'chat' THEN jsonb_strip_nulls(jsonb_build_object(
					'text', %s->'text',
					'attachments', %s->'attachments',
					'stage', %s->'stage',
					'completionState', %s->'completionState',
					'warningCodes', %s->'warningCodes',
					'warningMessage', %s->'warningMessage',
					'protocolFamily', %s->'protocolFamily',
					'streamDiagnostics', %s->'streamDiagnostics'
				))
				WHEN %s = 'video' THEN jsonb_strip_nulls(jsonb_build_object(
					'stage', %s->'stage',
					'artifacts', %s->'artifacts',
					'video', CASE
						WHEN NULLIF(TRIM(COALESCE(%s->'video'->>'contentUrl', '')), '') IS NOT NULL THEN jsonb_build_object(
							'contentUrl', %s->'video'->'contentUrl'
						)
						ELSE NULL
					END,
					'storyboard', CASE
						WHEN NULLIF(TRIM(COALESCE(%s->'storyboard'->>'optimizedPrompt', '')), '') IS NOT NULL THEN jsonb_build_object(
							'optimizedPrompt', %s->'storyboard'->'optimizedPrompt'
						)
						ELSE NULL
					END
				))
				WHEN %s = 'image' THEN jsonb_strip_nulls(jsonb_build_object(
					'stage', %s->'stage',
					'artifacts', %s->'artifacts',
					'storyboard', CASE
						WHEN NULLIF(TRIM(COALESCE(%s->'storyboard'->>'optimizedPrompt', '')), '') IS NOT NULL THEN jsonb_build_object(
							'optimizedPrompt', %s->'storyboard'->'optimizedPrompt'
						)
						ELSE NULL
					END
				))
				ELSE jsonb_strip_nulls(jsonb_build_object(
					'stage', %s->'stage'
				))
			END AS output_payload`,
			jobType,
			qualified,
			qualified,
			qualified,
			qualified,
			qualified,
			qualified,
			qualified,
			qualified,
			jobType,
			qualified,
			qualified,
			qualified,
			qualified,
			qualified,
			qualified,
			jobType,
			qualified,
			qualified,
			qualified,
			qualified,
			qualified,
		)
	}
	if aiJobListPayloadMode(payloadMode) != aiJobPayloadModeSummary {
		return qualified
	}
	return fmt.Sprintf(
		`jsonb_strip_nulls(jsonb_build_object('stage', %s->'stage')) AS output_payload`,
		qualified,
	)
}

// 根据计算AI作业SelectColumns，供AI作业链路复用关键派生结果。
func aiJobSelectColumnsFor(alias string, payloadMode string) string {
	qualifiedModelName := aiJobQualifiedColumn(alias, "model_name")
	columns := []string{
		aiJobQualifiedColumn(alias, "id"),
		aiJobQualifiedColumn(alias, "owner_user_id"),
		aiJobQualifiedColumn(alias, "device_id"),
		aiJobQualifiedColumn(alias, "skill_id"),
		aiJobQualifiedColumn(alias, "deleted_by_admin_user_id"),
		aiJobQualifiedColumn(alias, "source"),
		aiJobQualifiedColumn(alias, "local_task_id"),
		aiJobQualifiedColumn(alias, "job_type"),
		qualifiedModelName,
		fmt.Sprintf(
			"COALESCE((SELECT am.model_alias FROM ai_models am WHERE am.model_name = %s LIMIT 1), %s) AS model_alias",
			qualifiedModelName,
			qualifiedModelName,
		),
		aiJobPromptSelectColumn(alias, payloadMode),
		aiJobQualifiedColumn(alias, "status"),
		aiJobInputPayloadSelectColumn(alias, payloadMode),
		aiJobOutputPayloadSelectColumn(alias, payloadMode),
		aiJobQualifiedColumn(alias, "message"),
		aiJobQualifiedColumn(alias, "cost_credits"),
		aiJobQualifiedColumn(alias, "lease_owner_device_id"),
		aiJobQualifiedColumn(alias, "lease_token"),
		aiJobQualifiedColumn(alias, "lease_expires_at"),
		aiJobQualifiedColumn(alias, "delivery_status"),
		aiJobQualifiedColumn(alias, "delivery_message"),
		aiJobQualifiedColumn(alias, "local_publish_task_id"),
		aiJobQualifiedColumn(alias, "run_at"),
		aiJobQualifiedColumn(alias, "created_at"),
		aiJobQualifiedColumn(alias, "updated_at"),
		aiJobQualifiedColumn(alias, "delivered_at"),
		aiJobQualifiedColumn(alias, "finished_at"),
		aiJobQualifiedColumn(alias, "deleted_at"),
	}
	return "\n\t" + strings.Join(columns, ",\n\t") + "\n"
}

var aiJobSelectColumns = aiJobSelectColumnsFor("ai_jobs", aiJobPayloadModeFull)

func aiJobArtifactQualifiedColumn(alias string, column string) string {
	trimmedAlias := strings.TrimSpace(alias)
	if trimmedAlias == "" {
		return column
	}
	return trimmedAlias + "." + column
}

func aiJobArtifactSelectColumnsFor(alias string, mode string) string {
	textNullColumn := "NULL::text"
	jsonNullColumn := "NULL::jsonb"
	columns := []string{
		aiJobArtifactQualifiedColumn(alias, "id"),
		aiJobArtifactQualifiedColumn(alias, "job_id"),
		aiJobArtifactQualifiedColumn(alias, "artifact_key"),
		aiJobArtifactQualifiedColumn(alias, "artifact_type"),
		aiJobArtifactQualifiedColumn(alias, "source"),
		aiJobArtifactQualifiedColumn(alias, "title"),
		aiJobArtifactQualifiedColumn(alias, "file_name"),
		aiJobArtifactQualifiedColumn(alias, "mime_type"),
		aiJobArtifactQualifiedColumn(alias, "storage_key"),
		aiJobArtifactQualifiedColumn(alias, "public_url"),
		aiJobArtifactQualifiedColumn(alias, "size_bytes"),
		aiJobArtifactQualifiedColumn(alias, "text_content"),
		aiJobArtifactQualifiedColumn(alias, "device_id"),
		aiJobArtifactQualifiedColumn(alias, "root_name"),
		aiJobArtifactQualifiedColumn(alias, "relative_path"),
		aiJobArtifactQualifiedColumn(alias, "absolute_path"),
		aiJobArtifactQualifiedColumn(alias, "payload"),
		aiJobArtifactQualifiedColumn(alias, "created_at"),
		aiJobArtifactQualifiedColumn(alias, "updated_at"),
	}

	switch aiJobArtifactMode(mode) {
	case aiJobArtifactModePreview:
		columns = []string{
			aiJobArtifactQualifiedColumn(alias, "id"),
			aiJobArtifactQualifiedColumn(alias, "job_id"),
			aiJobArtifactQualifiedColumn(alias, "artifact_key"),
			aiJobArtifactQualifiedColumn(alias, "artifact_type"),
			aiJobArtifactQualifiedColumn(alias, "source"),
			textNullColumn + " AS title",
			aiJobArtifactQualifiedColumn(alias, "file_name"),
			aiJobArtifactQualifiedColumn(alias, "mime_type"),
			textNullColumn + " AS storage_key",
			aiJobArtifactQualifiedColumn(alias, "public_url"),
			aiJobArtifactQualifiedColumn(alias, "size_bytes"),
			textNullColumn + " AS text_content",
			textNullColumn + " AS device_id",
			textNullColumn + " AS root_name",
			textNullColumn + " AS relative_path",
			textNullColumn + " AS absolute_path",
			jsonNullColumn + " AS payload",
			aiJobArtifactQualifiedColumn(alias, "created_at"),
			aiJobArtifactQualifiedColumn(alias, "updated_at"),
		}
	case aiJobArtifactModeChat:
		columns = []string{
			aiJobArtifactQualifiedColumn(alias, "id"),
			aiJobArtifactQualifiedColumn(alias, "job_id"),
			aiJobArtifactQualifiedColumn(alias, "artifact_key"),
			aiJobArtifactQualifiedColumn(alias, "artifact_type"),
			aiJobArtifactQualifiedColumn(alias, "source"),
			aiJobArtifactQualifiedColumn(alias, "title"),
			aiJobArtifactQualifiedColumn(alias, "file_name"),
			aiJobArtifactQualifiedColumn(alias, "mime_type"),
			textNullColumn + " AS storage_key",
			aiJobArtifactQualifiedColumn(alias, "public_url"),
			aiJobArtifactQualifiedColumn(alias, "size_bytes"),
			aiJobArtifactQualifiedColumn(alias, "text_content"),
			textNullColumn + " AS device_id",
			textNullColumn + " AS root_name",
			textNullColumn + " AS relative_path",
			textNullColumn + " AS absolute_path",
			jsonNullColumn + " AS payload",
			aiJobArtifactQualifiedColumn(alias, "created_at"),
			aiJobArtifactQualifiedColumn(alias, "updated_at"),
		}
	}

	return "\n\t" + strings.Join(columns, ",\n\t") + "\n"
}

const aiModelSelectColumns = `
	id, vendor, model_name, model_alias, category, billing_mode, chat_protocol, base_url, api_key, raw_rate, billing_amount,
	description, pricing_payload,
	image_reference_limit, image_supported_sizes,
	video_reference_limit, video_supported_resolutions, video_supported_durations,
	supported_file_types,
	is_enabled, created_at, updated_at
`

const executableAIJobSourcesSQL = "'omnidrive_cloud', 'omnibull_local', 'account_skill_binding', 'openclaw_skill', 'openclaw_main_chat'"

type aiModelPricingPayload struct {
	ChatInputRawRate        *float64 `json:"chatInputRawRate,omitempty"`
	ChatOutputRawRate       *float64 `json:"chatOutputRawRate,omitempty"`
	ChatInputBillingAmount  *float64 `json:"chatInputBillingAmount,omitempty"`
	ChatOutputBillingAmount *float64 `json:"chatOutputBillingAmount,omitempty"`
}

// 规范化AI模型计费Mode，统一AI作业链路的输入格式和后续处理行为。
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

// 处理扫描AI模型相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func scanAIModel(row pgx.Row) (*domain.AIModel, error) {
	var model domain.AIModel
	var chatProtocol string
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
	var supportedFileTypes []byte

	if err := row.Scan(
		&model.ID,
		&model.Vendor,
		&model.ModelName,
		&model.ModelAlias,
		&model.Category,
		&model.BillingMode,
		&chatProtocol,
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
		&supportedFileTypes,
		&model.IsEnabled,
		&model.CreatedAt,
		&model.UpdatedAt,
	); err != nil {
		return nil, err
	}

	model.BillingMode = normalizeAIModelBillingMode(model.Category, model.BillingMode)
	model.ChatProtocol = normalizeAIModelChatProtocol(chatProtocol)
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
	model.SupportedFileTypes = decodeStringList(supportedFileTypes)
	return &model, nil
}

func normalizeAIModelChatProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto":
		return "auto"
	case "openai_chat_completions":
		return "openai_chat_completions"
	case "anthropic_messages":
		return "anthropic_messages"
	default:
		return "auto"
	}
}

// 应用AI模型定价载荷，把外部输入转换为当前链路的最终状态变更。
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

// 处理解码String列表相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 规范化可选String，统一AI作业链路的输入格式和后续处理行为。
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

// 处理扫描AI作业相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func scanAIJob(row pgx.Row) (*domain.AIJob, error) {
	var job domain.AIJob
	var deviceID *string
	var skillID *string
	var deletedByAdminUserID *string
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
	var deletedAt *time.Time

	if err := row.Scan(
		&job.ID,
		&job.OwnerUserID,
		&deviceID,
		&skillID,
		&deletedByAdminUserID,
		&job.Source,
		&localTaskID,
		&job.JobType,
		&job.ModelName,
		&job.ModelAlias,
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
		&deletedAt,
	); err != nil {
		return nil, err
	}

	job.DeviceID = deviceID
	job.SkillID = skillID
	job.DeletedByAdminUserID = deletedByAdminUserID
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
	job.DeletedAt = deletedAt
	return &job, nil
}

// 处理扫描AI作业产物相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListAIModels(ctx context.Context, category string) ([]domain.AIModel, error) {
	query := `
		SELECT ` + aiModelSelectColumns + `
		FROM ai_models
		WHERE is_enabled = TRUE
	`
	args := []any{}
	if strings.TrimSpace(category) != "" {
		query += ` AND category = $1`
		args = append(args, category)
	}
	query += ` ORDER BY category ASC, COALESCE(NULLIF(TRIM(model_alias), ''), model_name) ASC`

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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetAIModelByName(ctx context.Context, modelName string) (*domain.AIModel, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+aiModelSelectColumns+`
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetAIModelByIDOrName(ctx context.Context, value string) (*domain.AIModel, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+aiModelSelectColumns+`
		FROM ai_models
		WHERE id = $1 OR model_name = $1
	`, strings.TrimSpace(value))

	model, err := scanAIModel(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return model, nil
}

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListAIJobsByOwner(ctx context.Context, ownerUserID string, filter ListAIJobsFilter) ([]domain.AIJob, error) {
	payloadMode := aiJobListPayloadMode(filter.PayloadMode)
	query := fmt.Sprintf(`
		SELECT %s
		FROM ai_jobs
		WHERE owner_user_id = $1
		  AND deleted_at IS NULL
	`, aiJobSelectColumnsFor("ai_jobs", payloadMode))
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
		query += fmt.Sprintf(" AND %s = $%d", aiJobAccountIDExpression(""), argIndex)
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
	if filter.BeforeUpdatedAt != nil {
		if strings.TrimSpace(filter.BeforeID) != "" {
			query += fmt.Sprintf(" AND (updated_at, id) < ($%d, $%d)", argIndex, argIndex+1)
			args = append(args, *filter.BeforeUpdatedAt, filter.BeforeID)
			argIndex += 2
		} else {
			query += fmt.Sprintf(" AND updated_at < $%d", argIndex)
			args = append(args, *filter.BeforeUpdatedAt)
			argIndex++
		}
	}
	query += ` ORDER BY updated_at DESC, id DESC`
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

// 执行AI作业相关的数据库写入，维护持久化状态与后续业务流转。
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetAIJobByOwner(ctx context.Context, jobID string, ownerUserID string) (*domain.AIJob, error) {
	return s.GetAIJobByOwnerWithPayloadMode(ctx, jobID, ownerUserID, aiJobPayloadModeFull)
}

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetAIJobByOwnerWithPayloadMode(ctx context.Context, jobID string, ownerUserID string, payloadMode string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+aiJobSelectColumnsFor("ai_jobs", aiJobDetailPayloadMode(payloadMode))+`
		FROM ai_jobs
		WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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

// 执行AI作业相关的数据库写入，维护持久化状态与后续业务流转。
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

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
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
	if job != nil && strings.EqualFold(strings.TrimSpace(job.Status), "failed") {
		if err := s.returnUsageCreditsForFailedSourceTx(ctx, tx, "ai_job", job.ID, valueOrEmpty(job.Message)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return job, nil
}

// 执行AI作业相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) UpdateAIJobDeliveryByDevice(ctx context.Context, jobID string, deviceID string, status string, message *string, localPublishTaskID *string, deliveredAt *time.Time) (*domain.AIJob, bool, error) {
	currentRow := s.pool.QueryRow(ctx, `
		SELECT `+aiJobSelectColumns+`
		FROM ai_jobs
		WHERE id = $1
		  AND device_id = $2
	`, jobID, deviceID)

	current, err := scanAIJob(currentRow)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if !aiJobDeliveryChanged(current, status, message, localPublishTaskID) {
		return current, false, nil
	}

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
			return nil, false, nil
		}
		return nil, false, err
	}
	return job, true, nil
}

// 处理AI作业投递Changed相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func aiJobDeliveryChanged(current *domain.AIJob, status string, message *string, localPublishTaskID *string) bool {
	if current == nil {
		return true
	}

	nextStatus := strings.TrimSpace(status)
	if nextStatus == "" {
		nextStatus = strings.TrimSpace(current.DeliveryStatus)
	}
	if nextStatus != strings.TrimSpace(current.DeliveryStatus) {
		return true
	}

	if !sameOptionalDeliveryValue(current.DeliveryMessage, message) {
		return true
	}
	if !sameOptionalDeliveryValue(current.LocalPublishTaskID, localPublishTaskID) {
		return true
	}
	return false
}

// 处理same可选投递值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func sameOptionalDeliveryValue(current *string, incoming *string) bool {
	if incoming == nil {
		return true
	}
	return strings.TrimSpace(valueOrEmpty(current)) == strings.TrimSpace(valueOrEmpty(incoming))
}

// 取消AI作业，推进状态机并释放当前占用资源。
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
		WHERE id = $1 AND owner_user_id = $2 AND status IN ('queued', 'running', 'waiting_recharge')
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

// 重试AI作业，重置必要状态后重新放回执行链路。
func (s *Store) RetryAIJob(ctx context.Context, jobID string, ownerUserID string, message *string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET status = 'queued',
		    message = COALESCE($3, 'AI 任务已重新排队'),
		    output_payload = NULL,
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    delivery_status = 'pending',
		    delivery_message = NULL,
		    local_publish_task_id = NULL,
		    delivered_at = NULL,
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

// 执行AI作业相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) DeleteAccountSkillRunPlanByOwner(ctx context.Context, jobID string, ownerUserID string, scheduleKey string, repeating bool) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	if repeating && strings.TrimSpace(scheduleKey) != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE ai_jobs
			SET input_payload = jsonb_set(
			        jsonb_set(COALESCE(input_payload, '{}'::jsonb), '{scheduleConfig,repeatDaily}', 'false'::jsonb, true),
			        '{scheduleConfig,scheduleKey}',
			        to_jsonb(''::text),
			        true
			    ),
			    updated_at = NOW()
			WHERE owner_user_id = $1
			  AND source = 'account_skill_binding'
			  AND COALESCE(input_payload->'scheduleConfig'->>'scheduleKey', '') = $2
		`, ownerUserID, strings.TrimSpace(scheduleKey)); err != nil {
			return false, err
		}
	}

	commandTag, err := tx.Exec(ctx, `
		DELETE FROM ai_jobs
		WHERE id = $1
		  AND owner_user_id = $2
		  AND source = 'account_skill_binding'
		  AND local_publish_task_id IS NULL
		  AND status IN ('scheduled', 'queued', 'waiting_recharge', 'failed', 'cancelled')
	`, jobID, ownerUserID)
	if err != nil {
		return false, err
	}
	if commandTag.RowsAffected() == 0 {
		return false, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// 根据ce释放AI作业租约归属方计算，供AI作业链路复用关键派生结果。
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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

// 判断是否存在活跃AI作业s技能来源，供当前链路选择后续处理策略。
func (s *Store) HasActiveAIJobsBySkillAndSource(ctx context.Context, ownerUserID string, skillID string, source string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM ai_jobs
			WHERE owner_user_id = $1
			  AND skill_id = $2
			  AND source = $3
			  AND status IN ('scheduled', 'queued', 'pending', 'running', 'waiting_recharge')
		)
	`, ownerUserID, skillID, source).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListRecurringAccountSkillTemplateJobs(ctx context.Context, limit int) ([]domain.AIJob, error) {
	query := `
		SELECT ` + aiJobSelectColumnsFor("recurring_jobs", aiJobPayloadModeFull) + `
		FROM (
			SELECT DISTINCT ON (COALESCE(input_payload->'scheduleConfig'->>'scheduleKey', ''))
				` + aiJobSelectColumns + `
			FROM ai_jobs
			WHERE source = 'account_skill_binding'
			  AND COALESCE(input_payload->'scheduleConfig'->>'scheduleKey', '') <> ''
			  AND COALESCE(input_payload->'scheduleConfig'->>'repeatDaily', 'false') = 'true'
			ORDER BY COALESCE(input_payload->'scheduleConfig'->>'scheduleKey', ''), updated_at DESC, created_at DESC
		) recurring_jobs
		ORDER BY updated_at DESC, created_at DESC
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) FindActiveAccountSkillJobByScheduleKey(ctx context.Context, ownerUserID string, scheduleKey string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+aiJobSelectColumns+`
		FROM ai_jobs
		WHERE owner_user_id = $1
		  AND source = 'account_skill_binding'
		  AND COALESCE(input_payload->'scheduleConfig'->>'scheduleKey', '') = $2
		  AND status IN ('scheduled', 'queued', 'running', 'waiting_recharge')
		ORDER BY run_at ASC NULLS FIRST, created_at DESC
		LIMIT 1
	`, ownerUserID, strings.TrimSpace(scheduleKey))

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回指定账号计划时段的最新任务状态。
func (s *Store) FindAccountSkillJobByScheduleSlot(ctx context.Context, ownerUserID string, accountID string, scheduleKey string, runAt time.Time) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+aiJobSelectColumns+`
		FROM ai_jobs
		WHERE owner_user_id = $1
		  AND source = 'account_skill_binding'
		  AND COALESCE(input_payload->'scheduleConfig'->>'scheduleKey', '') = $2
		  AND `+aiJobAccountIDExpression("")+` = $3
		  AND run_at = $4
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, ownerUserID, strings.TrimSpace(scheduleKey), strings.TrimSpace(accountID), runAt.UTC())

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) FindActiveAccountSkillJobByRun(ctx context.Context, ownerUserID string, skillID string, deviceID string, accountID string, runAt time.Time) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+aiJobSelectColumns+`
		FROM ai_jobs
		WHERE owner_user_id = $1
		  AND source = 'account_skill_binding'
		  AND skill_id = $2
		  AND device_id = $3
		  AND run_at = $4
		  AND status IN ('scheduled', 'queued', 'running', 'waiting_recharge')
		  AND `+aiJobAccountIDExpression("")+` = $5
		ORDER BY created_at DESC
		LIMIT 1
	`, ownerUserID, skillID, deviceID, runAt.UTC(), strings.TrimSpace(accountID))

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListPendingAIJobsByDevice(ctx context.Context, deviceID string) ([]domain.AIJob, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+aiJobSelectColumns+`
		FROM ai_jobs
		WHERE device_id = $1
		  AND (
		      (status IN ('queued', 'waiting_recharge') AND (lease_expires_at IS NULL OR lease_expires_at < NOW()))
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListAgentAIJobsByDevice(ctx context.Context, deviceID string, sources []string, limit int) ([]domain.AIJob, error) {
	query := `
		SELECT ` + aiJobSelectColumns + `
		FROM ai_jobs
		WHERE device_id = $1
	`
	args := []any{deviceID}
	argIndex := 2
	normalizedSources := make([]string, 0, len(sources))
	for _, source := range sources {
		trimmed := strings.TrimSpace(source)
		if trimmed == "" {
			continue
		}
		normalizedSources = append(normalizedSources, trimmed)
	}
	if len(normalizedSources) > 0 {
		query += fmt.Sprintf(" AND source = ANY($%d)", argIndex)
		args = append(args, normalizedSources)
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

// 执行Agent AI作业增量队列查询，依赖上下文和连接池返回设备仍需处理的变更项。
func (s *Store) ListAgentAIJobsDeltaByDevice(ctx context.Context, deviceID string, sources []string, limit int, updatedAfter *time.Time, afterID string) ([]domain.AIJob, error) {
	query := `
		SELECT ` + aiJobSelectColumns + `
		FROM ai_jobs
		WHERE device_id = $1
		  AND (
		      status IN ('scheduled', 'queued', 'pending', 'waiting_recharge', 'running')
		      OR (
		          status IN ('success', 'completed', 'failed', 'cancelled')
		          AND COALESCE(NULLIF(TRIM(delivery_status), ''), 'pending') = 'pending'
		      )
		  )
	`
	args := []any{deviceID}
	argIndex := 2

	normalizedSources := make([]string, 0, len(sources))
	for _, source := range sources {
		trimmed := strings.TrimSpace(source)
		if trimmed == "" {
			continue
		}
		normalizedSources = append(normalizedSources, trimmed)
	}
	if len(normalizedSources) > 0 {
		query += fmt.Sprintf(" AND source = ANY($%d)", argIndex)
		args = append(args, normalizedSources)
		argIndex++
	}
	if updatedAfter != nil {
		if strings.TrimSpace(afterID) != "" {
			query += fmt.Sprintf(" AND (updated_at, id) > ($%d, $%d)", argIndex, argIndex+1)
			args = append(args, *updatedAfter, strings.TrimSpace(afterID))
			argIndex += 2
		} else {
			query += fmt.Sprintf(" AND updated_at > $%d", argIndex)
			args = append(args, *updatedAfter)
			argIndex++
		}
	}
	query += ` ORDER BY updated_at ASC, id ASC`
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListExecutableAIJobs(ctx context.Context, limit int) ([]domain.AIJob, error) {
	query := `
		SELECT ` + aiJobSelectColumnsFor("target", aiJobPayloadModeFull) + `
		FROM ai_jobs AS target
		WHERE target.status IN ('queued', 'waiting_recharge')
		  AND target.source IN (` + executableAIJobSourcesSQL + `)
		  AND (target.run_at IS NULL OR target.run_at <= NOW())
		  AND (target.lease_expires_at IS NULL OR target.lease_expires_at < NOW())
		  AND NOT EXISTS (
		      SELECT 1
		      FROM ai_jobs AS prior
		      WHERE prior.owner_user_id = target.owner_user_id
		        AND prior.source IN (` + executableAIJobSourcesSQL + `)
		        AND prior.id <> target.id
		        AND prior.status IN ('scheduled', 'queued', 'running', 'waiting_recharge')
		        AND (
		            COALESCE(prior.run_at, prior.created_at) < COALESCE(target.run_at, target.created_at)
		            OR (
		                COALESCE(prior.run_at, prior.created_at) = COALESCE(target.run_at, target.created_at)
		                AND (
		                    prior.created_at < target.created_at
		                    OR (prior.created_at = target.created_at AND prior.id < target.id)
		                )
		            )
		        )
		  )
		ORDER BY COALESCE(target.run_at, target.created_at) ASC, target.created_at ASC, target.id ASC
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListPendingExecutableAIJobsBefore(ctx context.Context, endExclusive time.Time, limit int) ([]domain.AIJob, error) {
	query := `
		SELECT ` + aiJobSelectColumns + `
		FROM ai_jobs
		WHERE source IN (` + executableAIJobSourcesSQL + `)
		  AND status IN ('scheduled', 'queued', 'waiting_recharge')
		  AND COALESCE(run_at, NOW()) < $1
		ORDER BY owner_user_id ASC, COALESCE(run_at, created_at) ASC, created_at ASC, id ASC
	`
	args := []any{endExclusive.UTC()}
	if limit > 0 {
		query += ` LIMIT $2`
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

// 处理PromoteDueScheduledAI作业s相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) FindScheduledOrActiveAIJobBySkillRun(ctx context.Context, skillID string, runAt time.Time) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+aiJobSelectColumns+`
		FROM ai_jobs
		WHERE skill_id = $1
		  AND run_at = $2
		  AND status IN ('scheduled', 'queued', 'running', 'waiting_recharge')
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

// 恢复ExpiredAI作业Leases，把中断或遗留状态重新接回当前执行链路。
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

// 恢复ExpiredExecutableAI作业Leases，把中断或遗留状态重新接回当前执行链路。
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

// 恢复中断ExecutableAI作业s，把中断或遗留状态重新接回当前执行链路。
func (s *Store) RecoverInterruptedExecutableAIJobs(ctx context.Context) ([]domain.AIJob, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE ai_jobs
		SET status = 'queued',
		    message = CASE
		        WHEN job_type = 'video' AND COALESCE(output_payload->'video'->>'id', '') <> '' THEN 'AI worker 重启后已恢复视频任务，继续回查云端结果'
		        ELSE 'AI worker 重启后已恢复任务，重新进入执行队列'
		    END,
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    finished_at = NULL,
		    updated_at = NOW()
		WHERE source IN (`+executableAIJobSourcesSQL+`)
		  AND status = 'running'
		  AND lease_owner_device_id IS NULL
		  AND lease_token IS NOT NULL
		  AND lease_expires_at IS NULL
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

// 标记StaleQueuedExecutableAI作业s失败，记录原因并推进后续回滚或人工处理逻辑。
func (s *Store) FailStaleQueuedExecutableAIJobs(ctx context.Context, queuedBefore time.Time, limit int) ([]domain.AIJob, error) {
	query := `
		WITH candidates AS (
			SELECT target.id
			FROM ai_jobs AS target
			WHERE target.source IN (` + executableAIJobSourcesSQL + `)
			  AND target.status = 'queued'
			  AND target.lease_owner_device_id IS NULL
			  AND target.lease_token IS NULL
			  AND (target.lease_expires_at IS NULL OR target.lease_expires_at < NOW())
			  AND (target.run_at IS NULL OR target.run_at <= NOW())
			  AND GREATEST(
			      COALESCE(target.updated_at, target.created_at),
			      COALESCE(target.run_at, target.created_at)
			  ) < $1
			  AND NOT EXISTS (
			      SELECT 1
			      FROM ai_jobs AS prior
			      WHERE prior.owner_user_id = target.owner_user_id
			        AND prior.source IN (` + executableAIJobSourcesSQL + `)
			        AND prior.id <> target.id
			        AND prior.status IN ('scheduled', 'queued', 'running', 'waiting_recharge')
			        AND (
			            COALESCE(prior.run_at, prior.created_at) < COALESCE(target.run_at, target.created_at)
			            OR (
			                COALESCE(prior.run_at, prior.created_at) = COALESCE(target.run_at, target.created_at)
			                AND (
			                    prior.created_at < target.created_at
			                    OR (prior.created_at = target.created_at AND prior.id < target.id)
			                )
			            )
			        )
			  )
			ORDER BY COALESCE(target.run_at, target.created_at) ASC, target.created_at ASC, target.id ASC
	`
	args := []any{queuedBefore.UTC()}
	if limit > 0 {
		query += ` LIMIT $2`
		args = append(args, limit)
	}
	query += `
		)
		UPDATE ai_jobs AS target
		SET status = 'failed',
		    message = 'AI 任务排队超时，已自动终止，请重新发起',
		    finished_at = NOW(),
		    updated_at = NOW()
		FROM candidates
		WHERE target.id = candidates.id
		RETURNING ` + aiJobSelectColumnsFor("target", aiJobPayloadModeFull) + `
	`

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

// 执行AI作业相关的租约与并发控制操作，确保调度和执行状态保持一致。
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

// 执行AI作业相关的租约与并发控制操作，确保调度和执行状态保持一致。
func (s *Store) ClaimCloudAIJobLease(ctx context.Context, jobID string, leaseToken string, leaseExpiresAt time.Time) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs AS target
		SET status = 'running',
		    lease_owner_device_id = NULL,
		    lease_token = $2,
		    lease_expires_at = $3,
		    updated_at = NOW()
		WHERE target.id = $1
		  AND target.source IN (`+executableAIJobSourcesSQL+`)
		  AND target.status IN ('queued', 'waiting_recharge')
		  AND (target.lease_expires_at IS NULL OR target.lease_expires_at < NOW())
		  AND NOT EXISTS (
		      SELECT 1
		      FROM ai_jobs AS prior
		      WHERE prior.owner_user_id = target.owner_user_id
		        AND prior.source IN (`+executableAIJobSourcesSQL+`)
		        AND prior.id <> target.id
		        AND prior.status IN ('scheduled', 'queued', 'running', 'waiting_recharge')
		        AND (
		            COALESCE(prior.run_at, prior.created_at) < COALESCE(target.run_at, target.created_at)
		            OR (
		                COALESCE(prior.run_at, prior.created_at) = COALESCE(target.run_at, target.created_at)
		                AND (
		                    prior.created_at < target.created_at
		                    OR (prior.created_at = target.created_at AND prior.id < target.id)
		                )
		            )
	          )
		  )
		RETURNING `+aiJobSelectColumnsFor("target", aiJobPayloadModeFull)+`
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

// 重新入队CloudAI作业Backoff，让当前对象在后续轮询中再次获得执行机会。
func (s *Store) RequeueCloudAIJobWithBackoff(ctx context.Context, jobID string, leaseToken string, retryAt time.Time, message *string, outputPayload []byte) (*domain.AIJob, error) {
	var payload any
	if len(outputPayload) > 0 {
		payload = outputPayload
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET status = 'queued',
		    output_payload = CASE
		        WHEN $4::jsonb IS NULL THEN output_payload
		        ELSE $4::jsonb
		    END,
		    message = COALESCE($5::text, message),
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = $3,
		    finished_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND source IN (`+executableAIJobSourcesSQL+`)
		  AND lease_owner_device_id IS NULL
		  AND lease_token = $2
		  AND status = 'running'
		RETURNING `+aiJobSelectColumns+`
	`, jobID, leaseToken, retryAt.UTC(), payload, message)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

// 执行AI作业相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) MarkCloudAIJobWaitingRecharge(ctx context.Context, jobID string, leaseToken string, retryAt time.Time, message *string, outputPayload []byte) (*domain.AIJob, error) {
	var payload any
	if len(outputPayload) > 0 {
		payload = outputPayload
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET status = 'waiting_recharge',
		    output_payload = CASE
		        WHEN $4::jsonb IS NULL THEN output_payload
		        ELSE $4::jsonb
		    END,
		    message = COALESCE($5::text, message),
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = $3,
		    finished_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND source IN (`+executableAIJobSourcesSQL+`)
		  AND lease_owner_device_id IS NULL
		  AND lease_token = $2
		  AND status = 'running'
		RETURNING `+aiJobSelectColumns+`
	`, jobID, leaseToken, retryAt.UTC(), payload, message)

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

// 执行AI作业相关的租约与并发控制操作，确保调度和执行状态保持一致。
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

// 执行AI作业相关的租约与并发控制操作，确保调度和执行状态保持一致。
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

// 执行AI作业相关的租约与并发控制操作，确保调度和执行状态保持一致。
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

// 同步CloudAI作业Execution，把当前上报或计算结果落到持久化状态中。
func (s *Store) SyncCloudAIJobExecution(ctx context.Context, jobID string, leaseToken string, input UpdateAIJobInput) (*domain.AIJob, error) {
	var outputPayload any
	if input.OutputTouched {
		outputPayload = input.OutputPayload
	}
	var finishedAt any
	if input.FinishedTouched {
		finishedAt = input.FinishedAt
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
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
	if job != nil && strings.EqualFold(strings.TrimSpace(job.Status), "failed") {
		if err := s.returnUsageCreditsForFailedSourceTx(ctx, tx, "ai_job", job.ID, valueOrEmpty(job.Message)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return job, nil
}

// 同步AI作业Execution，把当前上报或计算结果落到持久化状态中。
func (s *Store) SyncAIJobExecution(ctx context.Context, jobID string, deviceID string, leaseToken string, input UpdateAIJobInput) (*domain.AIJob, error) {
	var outputPayload any
	if input.OutputTouched {
		outputPayload = input.OutputPayload
	}
	var finishedAt any
	if input.FinishedTouched {
		finishedAt = input.FinishedAt
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
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
	if job != nil && strings.EqualFold(strings.TrimSpace(job.Status), "failed") {
		if err := s.returnUsageCreditsForFailedSourceTx(ctx, tx, "ai_job", job.ID, valueOrEmpty(job.Message)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return job, nil
}

// 执行AI作业相关的数据库写入，维护持久化状态与后续业务流转。
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListAIJobArtifactsByOwner(ctx context.Context, jobID string, ownerUserID string) ([]domain.AIJobArtifact, error) {
	return s.ListAIJobArtifactsByOwnerWithMode(ctx, jobID, ownerUserID, aiJobPayloadModeFull)
}

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListAIJobArtifactsByOwnerWithMode(ctx context.Context, jobID string, ownerUserID string, mode string) ([]domain.AIJobArtifact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+aiJobArtifactSelectColumnsFor("a", aiJobArtifactMode(mode))+`
		FROM ai_job_artifacts a
		INNER JOIN ai_jobs j ON j.id = a.job_id
		WHERE a.job_id = $1 AND j.owner_user_id = $2 AND j.deleted_at IS NULL
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

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListAIJobArtifactsByJobID(ctx context.Context, jobID string) ([]domain.AIJobArtifact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+aiJobArtifactSelectColumnsFor("", aiJobPayloadModeFull)+`
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

// 执行AI作业相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) DeleteAIJobPublishLinksByOwner(ctx context.Context, jobID string, ownerUserID string) (int64, error) {
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM ai_job_publish_links
		WHERE job_id = $1
		  AND owner_user_id = $2
	`, jobID, ownerUserID)
	if err != nil {
		return 0, err
	}
	return commandTag.RowsAffected(), nil
}

// 执行AI作业相关的数据库写入，维护软删除状态并让用户侧立即隐藏该记录。
func (s *Store) SoftDeleteAIJobByOwner(ctx context.Context, jobID string, ownerUserID string, adminUserID string) (*domain.AIJob, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE ai_jobs
		SET deleted_at = NOW(),
		    deleted_by_admin_user_id = $3,
		    updated_at = NOW()
		WHERE id = $1
		  AND owner_user_id = $2
		  AND deleted_at IS NULL
		  AND (
		      status IN ('failed', 'cancelled', 'success', 'completed')
		      OR (status = 'running' AND updated_at <= NOW() - INTERVAL '15 minutes')
		  )
		RETURNING `+aiJobSelectColumns+`
	`, strings.TrimSpace(jobID), strings.TrimSpace(ownerUserID), strings.TrimSpace(adminUserID))

	job, err := scanAIJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

// 执行AI作业相关的数据库写入，维护持久化状态与后续业务流转。
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

// 处理LinkAI作业发布任务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) LinkAIJobToPublishTask(ctx context.Context, input LinkAIJobPublishTaskInput) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ai_job_publish_links (job_id, task_id, owner_user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (job_id, task_id) DO NOTHING
	`, input.JobID, input.TaskID, input.OwnerUserID)
	return err
}

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) FindReusablePublishTaskByAIJobTarget(ctx context.Context, jobID string, ownerUserID string, deviceID string, platform string, accountName string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		       pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.verification_payload,
		       pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at, pt.attempt_count, pt.cancel_requested_at,
		       pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
		FROM publish_tasks pt
		LEFT JOIN ai_job_publish_links l
		  ON l.task_id = pt.id
		 AND l.owner_user_id = $2
		WHERE pt.device_id = $3
		  AND pt.platform = $4
		  AND pt.account_name = $5
		  AND pt.status IN ('pending', 'scheduled', 'running', 'cancel_requested', 'needs_verify', 'success', 'completed')
		  AND (
		      l.job_id = $1
		      OR (
		          COALESCE(pt.media_payload->>'aiJobId', '') = $1
		          AND EXISTS (
		              SELECT 1
		              FROM publish_task_material_refs refs
		              WHERE refs.task_id = pt.id
		          )
		      )
		  )
		ORDER BY pt.created_at DESC, pt.id DESC
		LIMIT 1
	`, jobID, ownerUserID, deviceID, platform, accountName)

	task, err := scanPublishTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

// 执行AI作业相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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
