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
	device_id,
	skill_id,
	account_id,
	platform,
	account_name,
	run_at,
	schedule_payload,
	local_publish_task_id,
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
	var deviceID *string
	var skillID *string
	var accountID *string
	var platform *string
	var accountName *string
	var runAt *time.Time
	var schedulePayload []byte
	var localPublishTaskID *string
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
		&deviceID,
		&skillID,
		&accountID,
		&platform,
		&accountName,
		&runAt,
		&schedulePayload,
		&localPublishTaskID,
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
	task.DeviceID = normalizeOptionalString(deviceID)
	task.SkillID = normalizeOptionalString(skillID)
	task.AccountID = normalizeOptionalString(accountID)
	task.Platform = normalizeOptionalString(platform)
	task.AccountName = normalizeOptionalString(accountName)
	task.RunAt = runAt
	task.SchedulePayload = bytesOrNil(schedulePayload)
	task.LocalPublishTaskID = normalizeOptionalString(localPublishTaskID)
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
			device_id,
			skill_id,
			account_id,
			platform,
			account_name,
			run_at,
			schedule_payload,
			local_publish_task_id,
			estimated_duration_seconds,
			estimated_credits,
			estimated_credits_millis,
			billing_status,
			billing_payload,
			progress,
			request_payload
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15, $16, $17, $18, $19, $20::jsonb, $21::jsonb, $22::jsonb)
		RETURNING `+mixVideoTaskSelectColumns+`
	`, input.ID, input.OwnerUserID, input.Source, input.Status, input.SourceAssets, input.RefAudioAsset, input.ScriptText, input.DeviceID, input.SkillID, input.AccountID, input.Platform, input.AccountName, timePtrValue(input.RunAt), nullableJSON(input.SchedulePayload), input.LocalPublishTaskID, input.EstimatedDurationSeconds, estimatedCreditsLegacy, input.EstimatedCreditsMillis, input.BillingStatus, nullableJSON(input.BillingPayload), nullableJSON(input.Progress), nullableJSON(input.RequestPayload))

	return scanMixVideoTask(row)
}

func (s *Store) DeleteMixVideoTask(ctx context.Context, taskID string, ownerUserID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM mix_video_tasks
		WHERE id = $1
		  AND owner_user_id = $2
		  AND status IN ('queued', 'scheduled')
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
		    (status = 'scheduled' AND run_at IS NOT NULL AND run_at <= NOW())
		    OR status IN ('queued', 'running')
		    OR (status = 'completed' AND billing_status = 'settlement_pending')
		)
		  AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
		ORDER BY CASE WHEN status = 'scheduled' THEN 0 WHEN status = 'queued' THEN 0 WHEN status = 'running' THEN 1 ELSE 2 END, created_at ASC, id ASC
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
		      ((status = 'scheduled' AND run_at IS NOT NULL AND run_at <= NOW()) OR status IN ('queued', 'running'))
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

func (s *Store) LinkMixVideoTaskToPublishTask(ctx context.Context, input LinkMixVideoTaskPublishTaskInput) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE mix_video_tasks
		SET local_publish_task_id = $2,
		    updated_at = NOW()
		WHERE id = $1
		  AND owner_user_id = $3
	`, strings.TrimSpace(input.TaskID), strings.TrimSpace(input.LocalPublishTaskID), strings.TrimSpace(input.OwnerUserID))
	return err
}

func (s *Store) DeleteAccountSkillMixVideoRunPlanByOwner(ctx context.Context, taskID string, ownerUserID string, scheduleKey string, repeating bool) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	if repeating && strings.TrimSpace(scheduleKey) != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE mix_video_tasks
			SET schedule_payload = jsonb_set(
			        jsonb_set(COALESCE(schedule_payload, '{}'::jsonb), '{repeatDaily}', 'false'::jsonb, true),
			        '{scheduleKey}',
			        to_jsonb(''::text),
			        true
			    ),
			    request_payload = jsonb_set(
			        jsonb_set(COALESCE(request_payload, '{}'::jsonb), '{scheduleConfig,repeatDaily}', 'false'::jsonb, true),
			        '{scheduleConfig,scheduleKey}',
			        to_jsonb(''::text),
			        true
			    ),
			    updated_at = NOW()
			WHERE owner_user_id = $1
			  AND source = 'account_skill_binding'
			  AND COALESCE(schedule_payload->>'scheduleKey', '') = $2
		`, ownerUserID, strings.TrimSpace(scheduleKey)); err != nil {
			return false, err
		}
	}

	commandTag, err := tx.Exec(ctx, `
		DELETE FROM mix_video_tasks
		WHERE id = $1
		  AND owner_user_id = $2
		  AND source = 'account_skill_binding'
		  AND local_publish_task_id IS NULL
		  AND status IN ('scheduled', 'queued', 'failed', 'cancelled')
	`, taskID, ownerUserID)
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

func (s *Store) ListRecurringAccountSkillTemplateMixVideoTasks(ctx context.Context, limit int) ([]domain.MixVideoTask, error) {
	query := `
		SELECT ` + mixVideoTaskSelectColumns + `
		FROM (
			SELECT DISTINCT ON (COALESCE(schedule_payload->>'scheduleKey', ''))
				` + mixVideoTaskSelectColumns + `
			FROM mix_video_tasks
			WHERE source = 'account_skill_binding'
			  AND COALESCE(schedule_payload->>'scheduleKey', '') <> ''
			  AND COALESCE(schedule_payload->>'repeatDaily', 'false') = 'true'
			ORDER BY COALESCE(schedule_payload->>'scheduleKey', ''), updated_at DESC, created_at DESC
		) recurring_tasks
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

func (s *Store) FindAccountSkillMixVideoTaskByScheduleSlot(ctx context.Context, ownerUserID string, accountID string, scheduleKey string, runAt time.Time) (*domain.MixVideoTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+mixVideoTaskSelectColumns+`
		FROM mix_video_tasks
		WHERE owner_user_id = $1
		  AND source = 'account_skill_binding'
		  AND COALESCE(schedule_payload->>'scheduleKey', '') = $2
		  AND COALESCE(account_id, '') = $3
		  AND run_at = $4
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, ownerUserID, strings.TrimSpace(scheduleKey), strings.TrimSpace(accountID), runAt.UTC())

	task, err := scanMixVideoTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) FindActiveAccountSkillMixVideoTaskByRun(ctx context.Context, ownerUserID string, skillID string, deviceID string, accountID string, runAt time.Time) (*domain.MixVideoTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+mixVideoTaskSelectColumns+`
		FROM mix_video_tasks
		WHERE owner_user_id = $1
		  AND source = 'account_skill_binding'
		  AND skill_id = $2
		  AND device_id = $3
		  AND account_id = $4
		  AND run_at = $5
		  AND status IN ('scheduled', 'queued', 'running')
		ORDER BY created_at DESC
		LIMIT 1
	`, ownerUserID, skillID, deviceID, strings.TrimSpace(accountID), runAt.UTC())

	task, err := scanMixVideoTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) FindReusablePublishTaskByMixVideoTarget(ctx context.Context, mixVideoTaskID string, ownerUserID string, deviceID string, platform string, accountName string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		       pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.verification_payload,
		       pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at, pt.attempt_count, pt.cancel_requested_at,
		       pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
		FROM publish_tasks pt
		WHERE pt.device_id = $2
		  AND pt.platform = $3
		  AND pt.account_name = $4
		  AND pt.status IN ('pending', 'scheduled', 'running', 'cancel_requested', 'needs_verify', 'success', 'completed')
		  AND (
		      pt.id = COALESCE((SELECT local_publish_task_id FROM mix_video_tasks WHERE id = $1 AND owner_user_id = $5), '')
		      OR COALESCE(pt.media_payload->>'mixVideoTaskId', '') = $1
		  )
		ORDER BY pt.created_at DESC, pt.id DESC
		LIMIT 1
	`, mixVideoTaskID, deviceID, platform, accountName, ownerUserID)

	task, err := scanPublishTask(row)
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
