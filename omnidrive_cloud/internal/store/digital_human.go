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
	mode,
	source,
	status,
	remote_task_id,
	character_asset,
	goods_asset,
	ref_audio_asset,
	result_asset,
	goods_title,
	goods_text,
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
	var remoteTaskID *string
	var characterAsset []byte
	var goodsAsset []byte
	var refAudioAsset []byte
	var resultAsset []byte
	var goodsTitle *string
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
		&task.Mode,
		&task.Source,
		&task.Status,
		&remoteTaskID,
		&characterAsset,
		&goodsAsset,
		&refAudioAsset,
		&resultAsset,
		&goodsTitle,
		&task.GoodsText,
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

	task.RemoteTaskID = normalizeOptionalString(remoteTaskID)
	task.CharacterAsset = *character
	task.GoodsAsset = goods
	task.RefAudioAsset = *refAudio
	task.ResultAsset = result
	task.GoodsTitle = normalizeOptionalString(goodsTitle)
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
	row := s.pool.QueryRow(ctx, `
		INSERT INTO digital_human_tasks (
			id,
			owner_user_id,
			mode,
			source,
			status,
			character_asset,
			goods_asset,
			ref_audio_asset,
			goods_title,
			goods_text,
			progress,
			request_payload
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9, $10, $11::jsonb, $12::jsonb)
		RETURNING `+digitalHumanTaskSelectColumns+`
	`, input.ID, input.OwnerUserID, input.Mode, input.Source, input.Status, input.CharacterAsset, nullableJSON(input.GoodsAsset), input.RefAudioAsset, input.GoodsTitle, input.GoodsText, nullableJSON(input.Progress), nullableJSON(input.RequestPayload))

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
		WHERE status IN ('queued', 'running')
		  AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
		ORDER BY CASE WHEN status = 'queued' THEN 0 ELSE 1 END, created_at ASC, id ASC
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
		  AND status IN ('queued', 'running')
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
		    progress = CASE
		        WHEN $8 = TRUE THEN $9::jsonb
		        ELSE progress
		    END,
		    remote_response_payload = CASE
		        WHEN $10 = TRUE THEN $11::jsonb
		        ELSE remote_response_payload
		    END,
		    error_message = COALESCE($12::text, error_message),
		    started_at = CASE
		        WHEN $13 = TRUE THEN $14::timestamptz
		        ELSE started_at
		    END,
		    completed_at = CASE
		        WHEN $15 = TRUE THEN $16::timestamptz
		        ELSE completed_at
		    END,
		    working_dir = CASE
		        WHEN $17 = TRUE THEN $18::text
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
	`, taskID, leaseToken, input.Status, input.RemoteTaskTouched, remoteTaskID, input.ResultAssetTouched, resultAsset, input.ProgressTouched, progress, input.RemotePayloadTouched, remotePayload, input.ErrorMessage, input.StartedTouched, startedAt, input.CompletedTouched, completedAt, input.WorkingDirTouched, workingDir)

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
