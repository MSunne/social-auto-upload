package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

func scanPublishTask(row pgx.Row) (*domain.PublishTask, error) {
	var task domain.PublishTask
	var contentText *string
	var mediaPayload []byte
	var message *string
	var verificationPayload []byte
	var accountID *string
	var skillID *string
	var runAt *time.Time
	var finishedAt *time.Time

	if err := row.Scan(
		&task.ID,
		&task.DeviceID,
		&accountID,
		&skillID,
		&task.Platform,
		&task.AccountName,
		&task.Title,
		&contentText,
		&mediaPayload,
		&task.Status,
		&message,
		&verificationPayload,
		&runAt,
		&finishedAt,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	task.AccountID = accountID
	task.SkillID = skillID
	task.ContentText = contentText
	task.MediaPayload = bytesOrNil(mediaPayload)
	task.Message = message
	task.VerificationPayload = bytesOrNil(verificationPayload)
	task.RunAt = runAt
	task.FinishedAt = finishedAt
	return &task, nil
}

func (s *Store) ListPublishTasksByOwner(ctx context.Context, ownerUserID string) ([]domain.PublishTask, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.platform, pt.account_name,
		       pt.title, pt.content_text, pt.media_payload, pt.status, pt.message,
		       pt.verification_payload, pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
		FROM publish_tasks pt
		INNER JOIN devices d ON d.id = pt.device_id
		WHERE d.owner_user_id = $1
		ORDER BY pt.updated_at DESC
	`, ownerUserID)
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

func (s *Store) GetPublishTaskByOwner(ctx context.Context, taskID string, ownerUserID string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.platform, pt.account_name,
		       pt.title, pt.content_text, pt.media_payload, pt.status, pt.message,
		       pt.verification_payload, pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
		FROM publish_tasks pt
		INNER JOIN devices d ON d.id = pt.device_id
		WHERE pt.id = $1 AND d.owner_user_id = $2
	`, taskID, ownerUserID)

	task, err := scanPublishTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) CreatePublishTask(ctx context.Context, input CreatePublishTaskInput) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO publish_tasks (
			id, device_id, account_id, skill_id, platform, account_name, title,
			content_text, media_payload, status, message, run_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, device_id, account_id, skill_id, platform, account_name,
		          title, content_text, media_payload, status, message, verification_payload,
		          run_at, finished_at, created_at, updated_at
	`, input.ID, input.DeviceID, input.AccountID, input.SkillID, input.Platform, input.AccountName,
		input.Title, input.ContentText, input.MediaPayload, input.Status, input.Message, input.RunAt)

	return scanPublishTask(row)
}

func (s *Store) ListPendingPublishTasksByDevice(ctx context.Context, deviceID string) ([]domain.PublishTask, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, device_id, account_id, skill_id, platform, account_name,
		       title, content_text, media_payload, status, message, verification_payload,
		       run_at, finished_at, created_at, updated_at
		FROM publish_tasks
		WHERE device_id = $1 AND status IN ('pending', 'running', 'needs_verify')
		ORDER BY created_at ASC
	`, deviceID)
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

func (s *Store) SyncPublishTask(ctx context.Context, input SyncPublishTaskInput) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO publish_tasks (
			id, device_id, account_id, skill_id, platform, account_name, title,
			content_text, media_payload, status, message, verification_payload,
			run_at, finished_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (id) DO UPDATE
		SET device_id = EXCLUDED.device_id,
		    account_id = EXCLUDED.account_id,
		    skill_id = EXCLUDED.skill_id,
		    platform = EXCLUDED.platform,
		    account_name = EXCLUDED.account_name,
		    title = EXCLUDED.title,
		    content_text = EXCLUDED.content_text,
		    media_payload = EXCLUDED.media_payload,
		    status = EXCLUDED.status,
		    message = EXCLUDED.message,
		    verification_payload = EXCLUDED.verification_payload,
		    run_at = EXCLUDED.run_at,
		    finished_at = EXCLUDED.finished_at,
		    updated_at = NOW()
		RETURNING id, device_id, account_id, skill_id, platform, account_name,
		          title, content_text, media_payload, status, message, verification_payload,
		          run_at, finished_at, created_at, updated_at
	`, input.ID, input.DeviceID, input.AccountID, input.SkillID, input.Platform, input.AccountName,
		input.Title, input.ContentText, input.MediaPayload, input.Status, input.Message,
		input.VerificationPayload, input.RunAt, input.FinishedAt)

	return scanPublishTask(row)
}
