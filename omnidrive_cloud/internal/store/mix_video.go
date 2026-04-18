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

const mixVideoTaskSelectColumns = `
	id,
	owner_user_id,
	source,
	status,
	remote_task_id,
	source_assets,
	ref_audio_asset,
	result_asset,
	script_text,
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

func decodeMixVideoAsset(raw []byte) (*domain.MixVideoAsset, error) {
	if len(raw) == 0 || strings.EqualFold(strings.TrimSpace(string(raw)), "null") {
		return nil, nil
	}
	var asset domain.MixVideoAsset
	if err := json.Unmarshal(raw, &asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

func decodeMixVideoAssets(raw []byte) ([]domain.MixVideoAsset, error) {
	if len(raw) == 0 || strings.EqualFold(strings.TrimSpace(string(raw)), "null") {
		return []domain.MixVideoAsset{}, nil
	}
	var assets []domain.MixVideoAsset
	if err := json.Unmarshal(raw, &assets); err != nil {
		return nil, err
	}
	if assets == nil {
		return []domain.MixVideoAsset{}, nil
	}
	return assets, nil
}

func decodeMixVideoProgress(raw []byte) (*domain.MixVideoProgress, error) {
	if len(raw) == 0 || strings.EqualFold(strings.TrimSpace(string(raw)), "null") {
		return nil, nil
	}
	var progress domain.MixVideoProgress
	if err := json.Unmarshal(raw, &progress); err != nil {
		return nil, err
	}
	return &progress, nil
}

func scanMixVideoTask(row pgx.Row) (*domain.MixVideoTask, error) {
	var task domain.MixVideoTask
	var remoteTaskID *string
	var sourceAssets []byte
	var refAudioAsset []byte
	var resultAsset []byte
	var estimatedCreditsLegacy int64
	var estimatedCreditsMillis int64
	var actualDurationSeconds *int
	var finalCreditsLegacy *int64
	var finalCreditsMillis *int64
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
		&task.Source,
		&task.Status,
		&remoteTaskID,
		&sourceAssets,
		&refAudioAsset,
		&resultAsset,
		&task.ScriptText,
		&task.EstimatedDurationSeconds,
		&estimatedCreditsLegacy,
		&estimatedCreditsMillis,
		&actualDurationSeconds,
		&finalCreditsLegacy,
		&finalCreditsMillis,
		&task.BillingStatus,
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

	assets, err := decodeMixVideoAssets(sourceAssets)
	if err != nil {
		return nil, err
	}
	refAudio, err := decodeMixVideoAsset(refAudioAsset)
	if err != nil {
		return nil, err
	}
	if refAudio == nil {
		return nil, fmt.Errorf("mix video task missing ref audio asset")
	}
	result, err := decodeMixVideoAsset(resultAsset)
	if err != nil {
		return nil, err
	}
	progressValue, err := decodeMixVideoProgress(progress)
	if err != nil {
		return nil, err
	}

	task.RemoteTaskID = normalizeOptionalString(remoteTaskID)
	task.SourceAssets = assets
	task.RefAudioAsset = *refAudio
	task.ResultAsset = result
	if estimatedCreditsMillis <= 0 && estimatedCreditsLegacy > 0 {
		estimatedCreditsMillis = estimatedCreditsLegacy * CreditMillisScale
	}
	task.EstimatedCreditsMillis = estimatedCreditsMillis
	task.EstimatedCredits = CreditsFromMillis(estimatedCreditsMillis)
	task.ActualDurationSeconds = actualDurationSeconds
	task.FinalCreditsMillis = finalCreditsMillis
	if task.FinalCreditsMillis == nil && finalCreditsLegacy != nil {
		fallbackMillis := *finalCreditsLegacy * CreditMillisScale
		task.FinalCreditsMillis = &fallbackMillis
	}
	task.FinalCredits = CreditsPtrFromMillis(task.FinalCreditsMillis)
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

func (s *Store) CreateMixVideoTask(ctx context.Context, input CreateMixVideoTaskInput) (*domain.MixVideoTask, error) {
	estimatedCreditsLegacy := RoundMillisToWholeCredits(input.EstimatedCreditsMillis)
	row := s.pool.QueryRow(ctx, `
		INSERT INTO mix_video_tasks (
			id,
			owner_user_id,
			source,
			status,
			source_assets,
			ref_audio_asset,
			script_text,
			estimated_duration_seconds,
			estimated_credits,
			estimated_credits_millis,
			billing_status,
			billing_payload,
			progress,
			request_payload
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8, $9, $10, $11, $12::jsonb, $13::jsonb, $14::jsonb)
		RETURNING `+mixVideoTaskSelectColumns+`
	`, input.ID, input.OwnerUserID, input.Source, input.Status, input.SourceAssets, input.RefAudioAsset, input.ScriptText, input.EstimatedDurationSeconds, estimatedCreditsLegacy, input.EstimatedCreditsMillis, input.BillingStatus, nullableJSON(input.BillingPayload), nullableJSON(input.Progress), nullableJSON(input.RequestPayload))

	return scanMixVideoTask(row)
}

func (s *Store) DeleteMixVideoTask(ctx context.Context, taskID string, ownerUserID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM mix_video_tasks
		WHERE id = $1
		  AND owner_user_id = $2
		  AND status = 'queued'
		  AND remote_task_id IS NULL
	`, strings.TrimSpace(taskID), strings.TrimSpace(ownerUserID))
	return err
}

func (s *Store) ListMixVideoTasksByOwner(ctx context.Context, ownerUserID string, filter ListMixVideoTasksFilter) ([]domain.MixVideoTask, error) {
	query := `
		SELECT ` + mixVideoTaskSelectColumns + `
		FROM mix_video_tasks
		WHERE owner_user_id = $1
	`
	args := []any{ownerUserID}
	argIndex := 2
	if trimmed := strings.TrimSpace(filter.Status); trimmed != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, trimmed)
		argIndex++
	}
	if filter.BeforeUpdatedAt != nil {
		if beforeID := strings.TrimSpace(filter.BeforeID); beforeID != "" {
			query += fmt.Sprintf(" AND (updated_at, id) < ($%d, $%d)", argIndex, argIndex+1)
			args = append(args, *filter.BeforeUpdatedAt, beforeID)
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

	items := make([]domain.MixVideoTask, 0)
	for rows.Next() {
		task, scanErr := scanMixVideoTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *task)
	}
	return items, rows.Err()
}

func (s *Store) GetMixVideoTaskByOwner(ctx context.Context, taskID string, ownerUserID string) (*domain.MixVideoTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+mixVideoTaskSelectColumns+`
		FROM mix_video_tasks
		WHERE id = $1 AND owner_user_id = $2
	`, strings.TrimSpace(taskID), strings.TrimSpace(ownerUserID))

	task, err := scanMixVideoTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) GetMixVideoTaskByID(ctx context.Context, taskID string) (*domain.MixVideoTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+mixVideoTaskSelectColumns+`
		FROM mix_video_tasks
		WHERE id = $1
	`, strings.TrimSpace(taskID))

	task, err := scanMixVideoTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) ListExecutableMixVideoTasks(ctx context.Context, limit int) ([]domain.MixVideoTask, error) {
	query := `
		SELECT ` + mixVideoTaskSelectColumns + `
		FROM mix_video_tasks
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

	items := make([]domain.MixVideoTask, 0)
	for rows.Next() {
		task, scanErr := scanMixVideoTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *task)
	}
	return items, rows.Err()
}

func (s *Store) ClaimMixVideoTaskLease(ctx context.Context, taskID string, leaseToken string, leaseExpiresAt time.Time) (*domain.MixVideoTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE mix_video_tasks
		SET lease_token = $2,
		    lease_expires_at = $3,
		    updated_at = NOW()
		WHERE id = $1
		  AND (
		      status IN ('queued', 'running')
		      OR (status = 'completed' AND billing_status = 'settlement_pending')
		  )
		  AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
		RETURNING `+mixVideoTaskSelectColumns+`
	`, strings.TrimSpace(taskID), strings.TrimSpace(leaseToken), leaseExpiresAt.UTC())

	task, err := scanMixVideoTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) RenewMixVideoTaskLease(ctx context.Context, taskID string, leaseToken string, leaseExpiresAt time.Time) (*domain.MixVideoTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE mix_video_tasks
		SET lease_expires_at = $3,
		    updated_at = NOW()
		WHERE id = $1
		  AND lease_token = $2
		RETURNING `+mixVideoTaskSelectColumns+`
	`, strings.TrimSpace(taskID), strings.TrimSpace(leaseToken), leaseExpiresAt.UTC())

	task, err := scanMixVideoTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) SyncMixVideoTaskExecution(ctx context.Context, taskID string, leaseToken string, input UpdateMixVideoTaskExecutionInput) (*domain.MixVideoTask, error) {
	var remoteTaskID any
	if input.RemoteTaskTouched {
		remoteTaskID = input.RemoteTaskID
	}
	var requestPayload any
	if input.RequestPayloadTouched {
		requestPayload = nullableJSON(input.RequestPayload)
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
	var billingStatus any
	if input.BillingStatusTouched {
		billingStatus = input.BillingStatus
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
	finalCreditsLegacy := int64(0)
	if input.FinalCreditsTouched && input.FinalCreditsMillis != nil {
		finalCreditsLegacy = RoundMillisToWholeCredits(*input.FinalCreditsMillis)
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE mix_video_tasks
		SET status = COALESCE($3::text, status),
		    remote_task_id = CASE WHEN $4 = TRUE THEN $5 ELSE remote_task_id END,
		    request_payload = CASE WHEN $6 = TRUE THEN $7::jsonb ELSE request_payload END,
		    result_asset = CASE WHEN $8 = TRUE THEN $9::jsonb ELSE result_asset END,
		    actual_duration_seconds = CASE WHEN $10 = TRUE THEN $11::int ELSE actual_duration_seconds END,
		    final_credits = CASE WHEN $12 = TRUE THEN $13::bigint ELSE final_credits END,
		    final_credits_millis = CASE WHEN $12 = TRUE THEN $14::bigint ELSE final_credits_millis END,
		    billing_status = CASE WHEN $15 = TRUE THEN $16::text ELSE billing_status END,
		    billing_payload = CASE WHEN $17 = TRUE THEN $18::jsonb ELSE billing_payload END,
		    progress = CASE WHEN $19 = TRUE THEN $20::jsonb ELSE progress END,
		    remote_response_payload = CASE WHEN $21 = TRUE THEN $22::jsonb ELSE remote_response_payload END,
		    error_message = COALESCE($23::text, error_message),
		    started_at = CASE WHEN $24 = TRUE THEN $25::timestamptz ELSE started_at END,
		    completed_at = CASE WHEN $26 = TRUE THEN $27::timestamptz ELSE completed_at END,
		    working_dir = CASE WHEN $28 = TRUE THEN $29::text ELSE working_dir END,
		    lease_token = CASE WHEN COALESCE($3::text, status) = 'running' THEN lease_token ELSE NULL END,
		    lease_expires_at = CASE WHEN COALESCE($3::text, status) = 'running' THEN lease_expires_at ELSE NULL END,
		    updated_at = NOW()
		WHERE id = $1
		  AND lease_token = $2
		RETURNING `+mixVideoTaskSelectColumns+`
	`, strings.TrimSpace(taskID), strings.TrimSpace(leaseToken), input.Status, input.RemoteTaskTouched, remoteTaskID, input.RequestPayloadTouched, requestPayload, input.ResultAssetTouched, resultAsset, input.ActualDurationTouched, actualDurationSeconds, input.FinalCreditsTouched, finalCreditsLegacy, finalCreditsMillis, input.BillingStatusTouched, billingStatus, input.BillingPayloadTouched, billingPayload, input.ProgressTouched, progress, input.RemotePayloadTouched, remotePayload, input.ErrorMessage, input.StartedTouched, startedAt, input.CompletedTouched, completedAt, input.WorkingDirTouched, workingDir)

	task, err := scanMixVideoTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}
