package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

const digitalHumanTaskSelectColumns = `
	id,
	owner_user_id,
	ai_job_id,
	mode,
	source,
	status,
	model_name,
	remote_task_id,
	character_asset,
	goods_asset,
	ref_audio_asset,
	result_asset,
	goods_title,
	goods_text,
	estimated_duration_seconds,
	estimated_credits,
	estimated_credits_millis,
	actual_duration_seconds,
	final_credits,
	final_credits_millis,
	billing_status,
	billing_payload,
	progress,
	request_payload,
	remote_response_payload,
	error_message,
	started_at,
	completed_at,
	created_at,
	updated_at,
	lease_token,
	lease_expires_at,
	working_dir
`

func decodeDigitalHumanAsset(raw []byte) (*domain.DigitalHumanAsset, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if strings.EqualFold(strings.TrimSpace(string(raw)), "null") {
		return nil, nil
	}
	var asset domain.DigitalHumanAsset
	if err := json.Unmarshal(raw, &asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

func decodeDigitalHumanProgress(raw []byte) (*domain.DigitalHumanProgress, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if strings.EqualFold(strings.TrimSpace(string(raw)), "null") {
		return nil, nil
	}
	var progress domain.DigitalHumanProgress
	if err := json.Unmarshal(raw, &progress); err != nil {
		return nil, err
	}
	return &progress, nil
}

func scanDigitalHumanTask(row pgx.Row) (*domain.DigitalHumanTask, error) {
	var task domain.DigitalHumanTask
	var aiJobID *string
	var remoteTaskID *string
	var characterAsset []byte
	var goodsAsset []byte
	var refAudioAsset []byte
	var resultAsset []byte
	var goodsTitle *string
	var estimatedCreditsLegacy int64
	var estimatedCreditsMillis int64
	var actualDurationSeconds *int
	var finalCreditsLegacy *int64
	var finalCreditsMillis *int64
	var billingStatus string
	var billingPayload []byte
	var progress []byte
	var requestPayload []byte
	var remoteResponsePayload []byte
	var errorMessage *string
	var startedAt *time.Time
	var completedAt *time.Time
	var leaseToken *string
	var leaseExpiresAt *time.Time
	var workingDir *string

	if err := row.Scan(
		&task.ID,
		&task.OwnerUserID,
		&aiJobID,
		&task.Mode,
		&task.Source,
		&task.Status,
		&task.ModelName,
		&remoteTaskID,
		&characterAsset,
		&goodsAsset,
		&refAudioAsset,
		&resultAsset,
		&goodsTitle,
		&task.GoodsText,
		&task.EstimatedDurationSeconds,
		&estimatedCreditsLegacy,
		&estimatedCreditsMillis,
		&actualDurationSeconds,
		&finalCreditsLegacy,
		&finalCreditsMillis,
		&billingStatus,
		&billingPayload,
		&progress,
		&requestPayload,
		&remoteResponsePayload,
		&errorMessage,
		&startedAt,
		&completedAt,
		&task.CreatedAt,
		&task.UpdatedAt,
		&leaseToken,
		&leaseExpiresAt,
		&workingDir,
	); err != nil {
		return nil, err
	}

	character, err := decodeDigitalHumanAsset(characterAsset)
	if err != nil {
		return nil, err
	}
	if character == nil {
		return nil, fmt.Errorf("digital human task missing character asset")
	}
	refAudio, err := decodeDigitalHumanAsset(refAudioAsset)
	if err != nil {
		return nil, err
	}
	if refAudio == nil {
		return nil, fmt.Errorf("digital human task missing ref audio asset")
	}
	goods, err := decodeDigitalHumanAsset(goodsAsset)
	if err != nil {
		return nil, err
	}
	result, err := decodeDigitalHumanAsset(resultAsset)
	if err != nil {
		return nil, err
	}
	progressValue, err := decodeDigitalHumanProgress(progress)
	if err != nil {
		return nil, err
	}

	task.AIJobID = normalizeOptionalString(aiJobID)
	task.RemoteTaskID = normalizeOptionalString(remoteTaskID)
	task.CharacterAsset = *character
	task.GoodsAsset = goods
	task.RefAudioAsset = *refAudio
	task.ResultAsset = result
	task.GoodsTitle = normalizeOptionalString(goodsTitle)
	if estimatedCreditsMillis <= 0 && estimatedCreditsLegacy > 0 {
		estimatedCreditsMillis = estimatedCreditsLegacy * DigitalHumanCreditMillisScale
	}
	task.EstimatedCreditsMillis = estimatedCreditsMillis
	task.EstimatedCredits = DigitalHumanCreditsFromMillis(estimatedCreditsMillis)
	task.ActualDurationSeconds = actualDurationSeconds
	task.FinalCreditsMillis = finalCreditsMillis
	if task.FinalCreditsMillis == nil && finalCreditsLegacy != nil {
		fallbackMillis := *finalCreditsLegacy * DigitalHumanCreditMillisScale
		task.FinalCreditsMillis = &fallbackMillis
	}
	task.FinalCredits = DigitalHumanCreditsPtrFromMillis(task.FinalCreditsMillis)
	task.BillingStatus = billingStatus
	task.BillingPayload = bytesOrNil(billingPayload)
	task.Progress = progressValue
	task.RequestPayload = bytesOrNil(requestPayload)
	task.RemoteResponsePayload = bytesOrNil(remoteResponsePayload)
	task.ErrorMessage = normalizeOptionalString(errorMessage)
	task.StartedAt = startedAt
	task.CompletedAt = completedAt
	task.LeaseToken = normalizeOptionalString(leaseToken)
	task.LeaseExpiresAt = leaseExpiresAt
	task.WorkingDir = normalizeOptionalString(workingDir)
	return &task, nil
}

func (s *Store) CreateDigitalHumanTask(ctx context.Context, input CreateDigitalHumanTaskInput) (*domain.DigitalHumanTask, error) {
	estimatedCreditsLegacy := DigitalHumanRoundMillisToWholeCredits(input.EstimatedCreditsMillis)
	row := s.pool.QueryRow(ctx, `
		INSERT INTO digital_human_tasks (
			id,
			owner_user_id,
			ai_job_id,
			mode,
			source,
			status,
			model_name,
			character_asset,
			goods_asset,
			ref_audio_asset,
			goods_title,
			goods_text,
			estimated_duration_seconds,
			estimated_credits,
			estimated_credits_millis,
			billing_status,
			billing_payload,
			progress,
			request_payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9::jsonb, $10, $11, $12, $13, $14, $15, $16::jsonb, $17::jsonb, $18::jsonb)
		RETURNING `+digitalHumanTaskSelectColumns+`
	`, input.ID, input.OwnerUserID, input.AIJobID, input.Mode, input.Source, input.Status, strings.TrimSpace(input.ModelName), input.CharacterAsset, nullableJSON(input.GoodsAsset), input.RefAudioAsset, input.GoodsTitle, input.GoodsText, input.EstimatedDurationSeconds, estimatedCreditsLegacy, input.EstimatedCreditsMillis, input.BillingStatus, nullableJSON(input.BillingPayload), nullableJSON(input.Progress), nullableJSON(input.RequestPayload))

	return scanDigitalHumanTask(row)
}

func (s *Store) ListDigitalHumanTasksByOwner(ctx context.Context, ownerUserID string, filter ListDigitalHumanTasksFilter) ([]domain.DigitalHumanTask, error) {
	query := `
		SELECT ` + digitalHumanTaskSelectColumns + `
		FROM digital_human_tasks
		WHERE owner_user_id = $1
	`
	args := []any{ownerUserID}
	argIndex := 2
	if trimmed := strings.TrimSpace(filter.Mode); trimmed != "" {
		query += fmt.Sprintf(" AND mode = $%d", argIndex)
		args = append(args, trimmed)
		argIndex++
	}
	if trimmed := strings.TrimSpace(filter.AIJobID); trimmed != "" {
		query += fmt.Sprintf(" AND ai_job_id = $%d", argIndex)
		args = append(args, trimmed)
		argIndex++
	}
	if trimmed := strings.TrimSpace(filter.Status); trimmed != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, trimmed)
		argIndex++
	}
	query += ` ORDER BY updated_at DESC, created_at DESC`
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.DigitalHumanTask, 0)
	for rows.Next() {
		task, scanErr := scanDigitalHumanTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *task)
	}
	return items, rows.Err()
}

func (s *Store) GetDigitalHumanTaskByAIJobID(ctx context.Context, aiJobID string) (*domain.DigitalHumanTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+digitalHumanTaskSelectColumns+`
		FROM digital_human_tasks
		WHERE ai_job_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, strings.TrimSpace(aiJobID))

	task, err := scanDigitalHumanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) GetDigitalHumanTaskByOwner(ctx context.Context, taskID string, ownerUserID string) (*domain.DigitalHumanTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+digitalHumanTaskSelectColumns+`
		FROM digital_human_tasks
		WHERE id = $1 AND owner_user_id = $2
	`, taskID, ownerUserID)

	task, err := scanDigitalHumanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) GetDigitalHumanTaskByID(ctx context.Context, taskID string) (*domain.DigitalHumanTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+digitalHumanTaskSelectColumns+`
		FROM digital_human_tasks
		WHERE id = $1
	`, taskID)

	task, err := scanDigitalHumanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) ListExecutableDigitalHumanTasks(ctx context.Context, limit int) ([]domain.DigitalHumanTask, error) {
	query := `
		SELECT ` + digitalHumanTaskSelectColumns + `
		FROM digital_human_tasks
		WHERE (
		    status IN ('queued', 'running')
		    OR (status = 'completed' AND billing_status = 'settlement_pending')
		)
		  AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
		ORDER BY CASE WHEN status = 'queued' THEN 0 WHEN status = 'running' THEN 1 ELSE 2 END, created_at ASC, id ASC
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

	items := make([]domain.DigitalHumanTask, 0)
	for rows.Next() {
		task, scanErr := scanDigitalHumanTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *task)
	}
	return items, rows.Err()
}

func (s *Store) ClaimDigitalHumanTaskLease(ctx context.Context, taskID string, leaseToken string, leaseExpiresAt time.Time) (*domain.DigitalHumanTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE digital_human_tasks
		SET lease_token = $2,
		    lease_expires_at = $3,
		    updated_at = NOW()
		WHERE id = $1
		  AND (
		      status IN ('queued', 'running')
		      OR (status = 'completed' AND billing_status = 'settlement_pending')
		  )
		  AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
		RETURNING `+digitalHumanTaskSelectColumns+`
	`, taskID, leaseToken, leaseExpiresAt.UTC())

	task, err := scanDigitalHumanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) RenewDigitalHumanTaskLease(ctx context.Context, taskID string, leaseToken string, leaseExpiresAt time.Time) (*domain.DigitalHumanTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE digital_human_tasks
		SET lease_expires_at = $3,
		    updated_at = NOW()
		WHERE id = $1
		  AND lease_token = $2
		RETURNING `+digitalHumanTaskSelectColumns+`
	`, taskID, leaseToken, leaseExpiresAt.UTC())

	task, err := scanDigitalHumanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) SyncDigitalHumanTaskExecution(ctx context.Context, taskID string, leaseToken string, input UpdateDigitalHumanTaskExecutionInput) (*domain.DigitalHumanTask, error) {
	var remoteTaskID any
	if input.RemoteTaskTouched {
		remoteTaskID = input.RemoteTaskID
	}
	var resultAsset any
	if input.ResultAssetTouched {
		resultAsset = nullableJSON(input.ResultAsset)
	}
	var actualDurationSeconds any
	if input.ActualDurationTouched {
		actualDurationSeconds = input.ActualDurationSeconds
	}
	var finalCreditsMillis any
	if input.FinalCreditsTouched {
		finalCreditsMillis = input.FinalCreditsMillis
	}
	var billingPayload any
	if input.BillingPayloadTouched {
		billingPayload = nullableJSON(input.BillingPayload)
	}
	var progress any
	if input.ProgressTouched {
		progress = nullableJSON(input.Progress)
	}
	var remotePayload any
	if input.RemotePayloadTouched {
		remotePayload = nullableJSON(input.RemoteResponsePayload)
	}
	var startedAt any
	if input.StartedTouched {
		startedAt = input.StartedAt
	}
	var completedAt any
	if input.CompletedTouched {
		completedAt = input.CompletedAt
	}
	var workingDir any
	if input.WorkingDirTouched {
		workingDir = input.WorkingDir
	}
	var billingStatus any
	if input.BillingStatusTouched {
		billingStatus = input.BillingStatus
	}
	finalCreditsLegacy := int64(0)
	if input.FinalCreditsTouched && input.FinalCreditsMillis != nil {
		finalCreditsLegacy = DigitalHumanRoundMillisToWholeCredits(*input.FinalCreditsMillis)
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE digital_human_tasks
		SET status = COALESCE($3::text, status),
		    remote_task_id = CASE
		        WHEN $4 = TRUE THEN $5
		        ELSE remote_task_id
		    END,
		    result_asset = CASE
		        WHEN $6 = TRUE THEN $7::jsonb
		        ELSE result_asset
		    END,
		    actual_duration_seconds = CASE
		        WHEN $8 = TRUE THEN $9::int
		        ELSE actual_duration_seconds
		    END,
		    final_credits = CASE
		        WHEN $10 = TRUE THEN $11::bigint
		        ELSE final_credits
		    END,
		    final_credits_millis = CASE
		        WHEN $10 = TRUE THEN $12::bigint
		        ELSE final_credits_millis
		    END,
		    billing_status = CASE
		        WHEN $13 = TRUE THEN $14::text
		        ELSE billing_status
		    END,
		    billing_payload = CASE
		        WHEN $15 = TRUE THEN $16::jsonb
		        ELSE billing_payload
		    END,
		    progress = CASE
		        WHEN $17 = TRUE THEN $18::jsonb
		        ELSE progress
		    END,
		    remote_response_payload = CASE
		        WHEN $19 = TRUE THEN $20::jsonb
		        ELSE remote_response_payload
		    END,
		    error_message = COALESCE($21::text, error_message),
		    started_at = CASE
		        WHEN $22 = TRUE THEN $23::timestamptz
		        ELSE started_at
		    END,
		    completed_at = CASE
		        WHEN $24 = TRUE THEN $25::timestamptz
		        ELSE completed_at
		    END,
		    working_dir = CASE
		        WHEN $26 = TRUE THEN $27::text
		        ELSE working_dir
		    END,
		    lease_token = CASE
		        WHEN COALESCE($3::text, status) = 'running' THEN lease_token
		        ELSE NULL
		    END,
		    lease_expires_at = CASE
		        WHEN COALESCE($3::text, status) = 'running' THEN lease_expires_at
		        ELSE NULL
		    END,
		    updated_at = NOW()
		WHERE id = $1
		  AND lease_token = $2
		RETURNING `+digitalHumanTaskSelectColumns+`
	`, taskID, leaseToken, input.Status, input.RemoteTaskTouched, remoteTaskID, input.ResultAssetTouched, resultAsset, input.ActualDurationTouched, actualDurationSeconds, input.FinalCreditsTouched, finalCreditsLegacy, finalCreditsMillis, input.BillingStatusTouched, billingStatus, input.BillingPayloadTouched, billingPayload, input.ProgressTouched, progress, input.RemotePayloadTouched, remotePayload, input.ErrorMessage, input.StartedTouched, startedAt, input.CompletedTouched, completedAt, input.WorkingDirTouched, workingDir)

	task, err := scanDigitalHumanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func nullableJSON(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return value
}
