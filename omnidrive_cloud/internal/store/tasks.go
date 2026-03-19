package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
	var skillRevision *string
	var leaseOwnerDeviceID *string
	var leaseToken *string
	var leaseExpiresAt *time.Time
	var cancelRequestedAt *time.Time
	var runAt *time.Time
	var finishedAt *time.Time

	if err := row.Scan(
		&task.ID,
		&task.DeviceID,
		&accountID,
		&skillID,
		&skillRevision,
		&task.Platform,
		&task.AccountName,
		&task.Title,
		&contentText,
		&mediaPayload,
		&task.Status,
		&message,
		&verificationPayload,
		&leaseOwnerDeviceID,
		&leaseToken,
		&leaseExpiresAt,
		&task.AttemptCount,
		&cancelRequestedAt,
		&runAt,
		&finishedAt,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	task.AccountID = accountID
	task.SkillID = skillID
	task.SkillRevision = skillRevision
	task.ContentText = contentText
	task.MediaPayload = bytesOrNil(mediaPayload)
	task.Message = message
	task.VerificationPayload = bytesOrNil(verificationPayload)
	task.LeaseOwnerDeviceID = leaseOwnerDeviceID
	task.LeaseToken = leaseToken
	task.LeaseExpiresAt = leaseExpiresAt
	task.CancelRequestedAt = cancelRequestedAt
	task.RunAt = runAt
	task.FinishedAt = finishedAt
	return &task, nil
}

func (s *Store) ListPublishTasksByOwner(ctx context.Context, ownerUserID string, filter ListPublishTasksFilter) ([]domain.PublishTask, error) {
	query := `
		SELECT pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		       pt.title, pt.content_text, pt.media_payload, pt.status, pt.message,
		       pt.verification_payload, pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at,
		       pt.attempt_count, pt.cancel_requested_at, pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
		FROM publish_tasks pt
		INNER JOIN devices d ON d.id = pt.device_id
		WHERE d.owner_user_id = $1
	`
	args := []any{ownerUserID}
	argIndex := 2

	if deviceID := strings.TrimSpace(filter.DeviceID); deviceID != "" {
		query += fmt.Sprintf(" AND pt.device_id = $%d", argIndex)
		args = append(args, deviceID)
		argIndex++
	}
	if accountID := strings.TrimSpace(filter.AccountID); accountID != "" {
		query += fmt.Sprintf(" AND pt.account_id = $%d", argIndex)
		args = append(args, accountID)
		argIndex++
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query += fmt.Sprintf(" AND pt.status = $%d", argIndex)
		args = append(args, status)
		argIndex++
	}
	if platform := strings.TrimSpace(filter.Platform); platform != "" {
		query += fmt.Sprintf(" AND pt.platform = $%d", argIndex)
		args = append(args, platform)
		argIndex++
	}
	if accountName := strings.TrimSpace(filter.AccountName); accountName != "" {
		query += fmt.Sprintf(" AND pt.account_name ILIKE $%d", argIndex)
		args = append(args, "%"+accountName+"%")
		argIndex++
	}

	query += " ORDER BY pt.updated_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
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

func (s *Store) GetPublishTaskByOwner(ctx context.Context, taskID string, ownerUserID string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		       pt.title, pt.content_text, pt.media_payload, pt.status, pt.message,
		       pt.verification_payload, pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at,
		       pt.attempt_count, pt.cancel_requested_at, pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
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

func (s *Store) GetPublishTaskByID(ctx context.Context, taskID string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, device_id, account_id, skill_id, skill_revision, platform, account_name,
		       title, content_text, media_payload, status, message,
		       verification_payload, lease_owner_device_id, lease_token, lease_expires_at,
		       attempt_count, cancel_requested_at, run_at, finished_at, created_at, updated_at
		FROM publish_tasks
		WHERE id = $1
	`, taskID)

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
			id, device_id, account_id, skill_id, skill_revision, platform, account_name, title,
			content_text, media_payload, status, message, run_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, device_id, account_id, skill_id, skill_revision, platform, account_name,
		          title, content_text, media_payload, status, message, verification_payload,
		          lease_owner_device_id, lease_token, lease_expires_at, attempt_count, cancel_requested_at,
		          run_at, finished_at, created_at, updated_at
	`, input.ID, input.DeviceID, input.AccountID, input.SkillID, input.SkillRevision, input.Platform, input.AccountName,
		input.Title, input.ContentText, input.MediaPayload, input.Status, input.Message, input.RunAt)

	return scanPublishTask(row)
}

func (s *Store) ListPendingPublishTasksByDevice(ctx context.Context, deviceID string) ([]domain.PublishTask, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, device_id, account_id, skill_id, skill_revision, platform, account_name,
		       title, content_text, media_payload, status, message, verification_payload,
		       lease_owner_device_id, lease_token, lease_expires_at, attempt_count, cancel_requested_at,
		       run_at, finished_at, created_at, updated_at
		FROM publish_tasks
	WHERE device_id = $1
	  AND (
	      (status = 'pending' AND (run_at IS NULL OR run_at <= NOW()) AND (lease_expires_at IS NULL OR lease_expires_at < NOW()))
	      OR (status IN ('running', 'cancel_requested') AND lease_owner_device_id = $1 AND lease_expires_at >= NOW())
	  )
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
			id, device_id, account_id, skill_id, skill_revision, platform, account_name, title,
			content_text, media_payload, status, message, verification_payload,
			run_at, finished_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE
		SET device_id = EXCLUDED.device_id,
		    account_id = EXCLUDED.account_id,
		    skill_id = EXCLUDED.skill_id,
		    skill_revision = COALESCE(EXCLUDED.skill_revision, publish_tasks.skill_revision),
		    platform = EXCLUDED.platform,
		    account_name = EXCLUDED.account_name,
		    title = EXCLUDED.title,
		    content_text = EXCLUDED.content_text,
		    media_payload = EXCLUDED.media_payload,
		    status = EXCLUDED.status,
		    message = EXCLUDED.message,
		    verification_payload = EXCLUDED.verification_payload,
		    lease_owner_device_id = CASE
		        WHEN EXCLUDED.status IN ('running', 'cancel_requested') THEN publish_tasks.lease_owner_device_id
		        ELSE NULL
		    END,
		    lease_token = CASE
		        WHEN EXCLUDED.status IN ('running', 'cancel_requested') THEN publish_tasks.lease_token
		        ELSE NULL
		    END,
		    lease_expires_at = CASE
		        WHEN EXCLUDED.status IN ('running', 'cancel_requested') THEN publish_tasks.lease_expires_at
		        ELSE NULL
		    END,
		    cancel_requested_at = CASE
		        WHEN EXCLUDED.status = 'cancel_requested' THEN COALESCE(publish_tasks.cancel_requested_at, NOW())
		        ELSE NULL
		    END,
		    run_at = EXCLUDED.run_at,
		    finished_at = EXCLUDED.finished_at,
		    updated_at = NOW()
		RETURNING id, device_id, account_id, skill_id, skill_revision, platform, account_name,
		          title, content_text, media_payload, status, message, verification_payload,
		          lease_owner_device_id, lease_token, lease_expires_at, attempt_count, cancel_requested_at,
		          run_at, finished_at, created_at, updated_at
	`, input.ID, input.DeviceID, input.AccountID, input.SkillID, input.SkillRevision, input.Platform, input.AccountName,
		input.Title, input.ContentText, input.MediaPayload, input.Status, input.Message,
		input.VerificationPayload, input.RunAt, input.FinishedAt)

	return scanPublishTask(row)
}

func (s *Store) UpdatePublishTask(ctx context.Context, taskID string, ownerUserID string, input UpdatePublishTaskInput) (*domain.PublishTask, error) {
	var mediaPayload any
	if input.MediaTouched {
		mediaPayload = input.MediaPayload
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE publish_tasks pt
		SET title = COALESCE($3, pt.title),
		    content_text = COALESCE($4, pt.content_text),
		    media_payload = COALESCE($5, pt.media_payload),
		    status = COALESCE($6, pt.status),
		    message = COALESCE($7, pt.message),
		    run_at = COALESCE($8, pt.run_at),
		    updated_at = NOW()
		FROM devices d
		WHERE pt.device_id = d.id
		  AND pt.id = $1
		  AND d.owner_user_id = $2
		RETURNING pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		          pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.verification_payload,
		          pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at, pt.attempt_count, pt.cancel_requested_at,
		          pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
	`, taskID, ownerUserID, input.Title, input.ContentText, mediaPayload, input.Status, input.Message, input.RunAt)

	task, err := scanPublishTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) RefreshPublishTaskSkillRevision(ctx context.Context, taskID string, ownerUserID string, skillRevision *string, message *string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE publish_tasks pt
		SET skill_revision = $3,
		    message = COALESCE($4, pt.message),
		    updated_at = NOW()
		FROM devices d
		WHERE pt.device_id = d.id
		  AND pt.id = $1
		  AND d.owner_user_id = $2
		RETURNING pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		          pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.verification_payload,
		          pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at, pt.attempt_count, pt.cancel_requested_at,
		          pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
	`, taskID, ownerUserID, skillRevision, message)

	task, err := scanPublishTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) ClaimPublishTaskLease(ctx context.Context, taskID string, deviceID string, leaseToken string, leaseExpiresAt time.Time) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE publish_tasks
		SET status = 'running',
		    lease_owner_device_id = $2,
		    lease_token = $3,
		    lease_expires_at = $4,
		    attempt_count = attempt_count + 1,
		    cancel_requested_at = NULL,
		    updated_at = NOW()
	WHERE id = $1
	  AND device_id = $2
	  AND status = 'pending'
	  AND (run_at IS NULL OR run_at <= NOW())
	  AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
		RETURNING id, device_id, account_id, skill_id, skill_revision, platform, account_name,
		          title, content_text, media_payload, status, message, verification_payload,
		          lease_owner_device_id, lease_token, lease_expires_at, attempt_count, cancel_requested_at,
		          run_at, finished_at, created_at, updated_at
	`, taskID, deviceID, leaseToken, leaseExpiresAt)

	task, err := scanPublishTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) RenewPublishTaskLease(ctx context.Context, taskID string, deviceID string, leaseToken string, leaseExpiresAt time.Time) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE publish_tasks
		SET lease_expires_at = $4,
		    updated_at = NOW()
		WHERE id = $1
		  AND device_id = $2
		  AND lease_owner_device_id = $2
		  AND lease_token = $3
		  AND status IN ('running', 'cancel_requested')
		RETURNING id, device_id, account_id, skill_id, skill_revision, platform, account_name,
		          title, content_text, media_payload, status, message, verification_payload,
		          lease_owner_device_id, lease_token, lease_expires_at, attempt_count, cancel_requested_at,
		          run_at, finished_at, created_at, updated_at
	`, taskID, deviceID, leaseToken, leaseExpiresAt)

	task, err := scanPublishTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) ReleasePublishTaskLeaseByAgent(ctx context.Context, taskID string, deviceID string, leaseToken string, message *string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE publish_tasks
		SET status = CASE
		        WHEN status = 'cancel_requested' THEN 'cancelled'
		        ELSE 'pending'
		    END,
		    message = COALESCE($4, CASE
		        WHEN status = 'cancel_requested' THEN '本地设备已确认取消任务'
		        ELSE '本地设备已释放任务租约并重新排队'
		    END),
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    cancel_requested_at = CASE
		        WHEN status = 'cancel_requested' THEN cancel_requested_at
		        ELSE NULL
		    END,
		    finished_at = CASE
		        WHEN status = 'cancel_requested' THEN NOW()
		        ELSE NULL
		    END,
		    updated_at = NOW()
		WHERE id = $1
		  AND device_id = $2
		  AND lease_owner_device_id = $2
		  AND lease_token = $3
		  AND status IN ('running', 'cancel_requested')
		RETURNING id, device_id, account_id, skill_id, skill_revision, platform, account_name,
		          title, content_text, media_payload, status, message, verification_payload,
		          lease_owner_device_id, lease_token, lease_expires_at, attempt_count, cancel_requested_at,
		          run_at, finished_at, created_at, updated_at
	`, taskID, deviceID, leaseToken, message)

	task, err := scanPublishTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) RequestCancelPublishTask(ctx context.Context, taskID string, ownerUserID string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE publish_tasks pt
		SET status = CASE
		        WHEN pt.status = 'pending' THEN 'cancelled'
		        WHEN pt.status IN ('running', 'needs_verify') THEN 'cancel_requested'
		        ELSE pt.status
		    END,
		    message = CASE
		        WHEN pt.status = 'pending' THEN '任务已取消'
		        WHEN pt.status IN ('running', 'needs_verify') THEN '已请求取消，等待本地设备确认'
		        ELSE pt.message
		    END,
		    cancel_requested_at = CASE
		        WHEN pt.status IN ('running', 'needs_verify') THEN NOW()
		        ELSE pt.cancel_requested_at
		    END,
		    finished_at = CASE
		        WHEN pt.status = 'pending' THEN NOW()
		        ELSE pt.finished_at
		    END,
		    lease_owner_device_id = CASE
		        WHEN pt.status = 'pending' THEN NULL
		        ELSE pt.lease_owner_device_id
		    END,
		    lease_token = CASE
		        WHEN pt.status = 'pending' THEN NULL
		        ELSE pt.lease_token
		    END,
		    lease_expires_at = CASE
		        WHEN pt.status = 'pending' THEN NULL
		        ELSE pt.lease_expires_at
		    END,
		    updated_at = NOW()
		FROM devices d
		WHERE pt.device_id = d.id
		  AND pt.id = $1
		  AND d.owner_user_id = $2
		RETURNING pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		          pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.verification_payload,
		          pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at, pt.attempt_count, pt.cancel_requested_at,
		          pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
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

func (s *Store) RetryPublishTask(ctx context.Context, taskID string, ownerUserID string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE publish_tasks pt
		SET status = 'pending',
		    message = '等待重新执行',
		    verification_payload = NULL,
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    cancel_requested_at = NULL,
		    finished_at = NULL,
		    updated_at = NOW()
		FROM devices d
		WHERE pt.device_id = d.id
		  AND pt.id = $1
		  AND d.owner_user_id = $2
		RETURNING pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		          pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.verification_payload,
		          pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at, pt.attempt_count, pt.cancel_requested_at,
		          pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
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

func (s *Store) ForceReleasePublishTaskLease(ctx context.Context, taskID string, ownerUserID string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE publish_tasks pt
		SET status = CASE
		        WHEN pt.status = 'cancel_requested' THEN 'cancelled'
		        ELSE 'pending'
		    END,
		    message = CASE
		        WHEN pt.status = 'cancel_requested' THEN '任务租约已由云端手动释放并标记为取消'
		        ELSE '任务租约已由云端手动释放并重新排队'
		    END,
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    cancel_requested_at = CASE
		        WHEN pt.status = 'cancel_requested' THEN pt.cancel_requested_at
		        ELSE NULL
		    END,
		    finished_at = CASE
		        WHEN pt.status = 'cancel_requested' THEN NOW()
		        ELSE NULL
		    END,
		    updated_at = NOW()
		FROM devices d
		WHERE pt.device_id = d.id
		  AND pt.id = $1
		  AND d.owner_user_id = $2
		  AND pt.status IN ('running', 'cancel_requested')
		  AND pt.lease_owner_device_id IS NOT NULL
		  AND pt.lease_token IS NOT NULL
		RETURNING pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		          pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.verification_payload,
		          pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at, pt.attempt_count, pt.cancel_requested_at,
		          pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
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

func (s *Store) ResumePublishTaskFromVerification(ctx context.Context, taskID string, ownerUserID string, message *string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE publish_tasks pt
		SET status = 'pending',
		    message = COALESCE($3, '等待继续执行'),
		    verification_payload = NULL,
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    cancel_requested_at = NULL,
		    finished_at = NULL,
		    updated_at = NOW()
		FROM devices d
		WHERE pt.device_id = d.id
		  AND pt.id = $1
		  AND d.owner_user_id = $2
		  AND pt.status = 'needs_verify'
		RETURNING pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		          pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.verification_payload,
		          pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at, pt.attempt_count, pt.cancel_requested_at,
		          pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
	`, taskID, ownerUserID, message)

	task, err := scanPublishTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) ResolvePublishTaskManually(ctx context.Context, taskID string, ownerUserID string, status string, message *string) (*domain.PublishTask, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE publish_tasks pt
		SET status = $3,
		    message = COALESCE($4, CASE
		        WHEN $3 IN ('success', 'completed') THEN '任务已由人工处理完成'
		        WHEN $3 = 'failed' THEN '任务已人工标记为失败'
		        WHEN $3 = 'cancelled' THEN '任务已人工取消'
		        ELSE pt.message
		    END),
		    verification_payload = NULL,
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    cancel_requested_at = CASE
		        WHEN $3 = 'cancelled' THEN COALESCE(pt.cancel_requested_at, NOW())
		        ELSE NULL
		    END,
		    finished_at = NOW(),
		    updated_at = NOW()
		FROM devices d
		WHERE pt.device_id = d.id
		  AND pt.id = $1
		  AND d.owner_user_id = $2
		  AND pt.status = 'needs_verify'
		RETURNING pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		          pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.verification_payload,
		          pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at, pt.attempt_count, pt.cancel_requested_at,
		          pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
	`, taskID, ownerUserID, status, message)

	task, err := scanPublishTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (s *Store) RecoverExpiredPublishTaskLeases(ctx context.Context, deviceID string) ([]domain.PublishTask, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE publish_tasks
		SET status = CASE
		        WHEN status = 'cancel_requested' THEN 'cancelled'
		        ELSE 'pending'
		    END,
		    message = CASE
		        WHEN status = 'cancel_requested' THEN '取消租约已过期，系统已标记为取消'
		        ELSE '执行租约已过期，任务已重新排队'
		    END,
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    cancel_requested_at = CASE
		        WHEN status = 'cancel_requested' THEN cancel_requested_at
		        ELSE NULL
		    END,
		    finished_at = CASE
		        WHEN status = 'cancel_requested' THEN NOW()
		        ELSE finished_at
		    END,
		    updated_at = NOW()
		WHERE device_id = $1
		  AND status IN ('running', 'cancel_requested')
		  AND lease_expires_at IS NOT NULL
		  AND lease_expires_at < NOW()
		RETURNING id, device_id, account_id, skill_id, skill_revision, platform, account_name,
		          title, content_text, media_payload, status, message, verification_payload,
		          lease_owner_device_id, lease_token, lease_expires_at, attempt_count, cancel_requested_at,
		          run_at, finished_at, created_at, updated_at
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

func scanPublishTaskEvent(row pgx.Row) (*domain.PublishTaskEvent, error) {
	var event domain.PublishTaskEvent
	var message *string
	var payload []byte

	if err := row.Scan(
		&event.ID,
		&event.TaskID,
		&event.EventType,
		&event.Source,
		&event.Status,
		&message,
		&payload,
		&event.CreatedAt,
	); err != nil {
		return nil, err
	}

	event.Message = message
	event.Payload = bytesOrNil(payload)
	return &event, nil
}

func scanPublishTaskArtifact(row pgx.Row) (*domain.PublishTaskArtifact, error) {
	var item domain.PublishTaskArtifact
	var title *string
	var fileName *string
	var mimeType *string
	var storageKey *string
	var publicURL *string
	var sizeBytes *int64
	var textContent *string
	var payload []byte

	if err := row.Scan(
		&item.ID,
		&item.TaskID,
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
	item.Payload = bytesOrNil(payload)
	return &item, nil
}

func scanPublishTaskMaterialRef(row pgx.Row) (*domain.PublishTaskMaterialRef, error) {
	var item domain.PublishTaskMaterialRef
	var absolutePath *string
	var sizeBytes *int64
	var modifiedAt *string
	var extension *string
	var mimeType *string
	var previewText *string

	if err := row.Scan(
		&item.ID,
		&item.TaskID,
		&item.DeviceID,
		&item.RootName,
		&item.RelativePath,
		&item.Role,
		&item.Name,
		&item.Kind,
		&absolutePath,
		&sizeBytes,
		&modifiedAt,
		&extension,
		&mimeType,
		&item.IsText,
		&previewText,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}

	item.AbsolutePath = absolutePath
	item.SizeBytes = sizeBytes
	item.ModifiedAt = modifiedAt
	item.Extension = extension
	item.MimeType = mimeType
	item.PreviewText = previewText
	return &item, nil
}

func scanPublishTaskRuntimeState(row pgx.Row) (*domain.PublishTaskRuntimeState, error) {
	var item domain.PublishTaskRuntimeState
	var executionPayload []byte
	var lastAgentSyncAt *time.Time

	if err := row.Scan(
		&item.TaskID,
		&executionPayload,
		&lastAgentSyncAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}

	item.ExecutionPayload = bytesOrNil(executionPayload)
	item.LastAgentSyncAt = lastAgentSyncAt
	return &item, nil
}

func (s *Store) CreatePublishTaskEvent(ctx context.Context, input CreatePublishTaskEventInput) (*domain.PublishTaskEvent, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO publish_task_events (
			id, task_id, event_type, source, status, message, payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, task_id, event_type, source, status, message, payload, created_at
	`, input.ID, input.TaskID, input.EventType, input.Source, input.Status, input.Message, input.Payload)

	return scanPublishTaskEvent(row)
}

func (s *Store) ListPublishTaskEventsByOwner(ctx context.Context, taskID string, ownerUserID string) ([]domain.PublishTaskEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT pte.id, pte.task_id, pte.event_type, pte.source, pte.status, pte.message, pte.payload, pte.created_at
		FROM publish_task_events pte
		INNER JOIN publish_tasks pt ON pt.id = pte.task_id
		INNER JOIN devices d ON d.id = pt.device_id
		WHERE pte.task_id = $1 AND d.owner_user_id = $2
		ORDER BY pte.created_at ASC
	`, taskID, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.PublishTaskEvent, 0)
	for rows.Next() {
		event, scanErr := scanPublishTaskEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *event)
	}
	return items, rows.Err()
}

func (s *Store) UpsertPublishTaskArtifacts(ctx context.Context, items []UpsertPublishTaskArtifactInput) ([]domain.PublishTaskArtifact, error) {
	if len(items) == 0 {
		return []domain.PublishTaskArtifact{}, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	result := make([]domain.PublishTaskArtifact, 0, len(items))
	for _, item := range items {
		row := tx.QueryRow(ctx, `
			INSERT INTO publish_task_artifacts (
				id, task_id, artifact_key, artifact_type, source, title, file_name, mime_type,
				storage_key, public_url, size_bytes, text_content, payload
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (task_id, artifact_key) DO UPDATE
			SET artifact_type = EXCLUDED.artifact_type,
			    source = EXCLUDED.source,
			    title = EXCLUDED.title,
			    file_name = EXCLUDED.file_name,
			    mime_type = EXCLUDED.mime_type,
			    storage_key = EXCLUDED.storage_key,
			    public_url = EXCLUDED.public_url,
			    size_bytes = EXCLUDED.size_bytes,
			    text_content = EXCLUDED.text_content,
			    payload = EXCLUDED.payload,
			    updated_at = NOW()
			RETURNING id, task_id, artifact_key, artifact_type, source, title, file_name, mime_type,
			          storage_key, public_url, size_bytes, text_content, payload, created_at, updated_at
		`, uuid.NewString(), item.TaskID, item.ArtifactKey, item.ArtifactType, item.Source,
			item.Title, item.FileName, item.MimeType, item.StorageKey, item.PublicURL,
			item.SizeBytes, item.TextContent, item.Payload)

		artifact, scanErr := scanPublishTaskArtifact(row)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *artifact)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) ListPublishTaskArtifactsByOwner(ctx context.Context, taskID string, ownerUserID string) ([]domain.PublishTaskArtifact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT pta.id, pta.task_id, pta.artifact_key, pta.artifact_type, pta.source, pta.title,
		       pta.file_name, pta.mime_type, pta.storage_key, pta.public_url, pta.size_bytes,
		       pta.text_content, pta.payload, pta.created_at, pta.updated_at
		FROM publish_task_artifacts pta
		INNER JOIN publish_tasks pt ON pt.id = pta.task_id
		INNER JOIN devices d ON d.id = pt.device_id
		WHERE pta.task_id = $1 AND d.owner_user_id = $2
		ORDER BY pta.updated_at DESC, pta.created_at DESC
	`, taskID, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.PublishTaskArtifact, 0)
	for rows.Next() {
		artifact, scanErr := scanPublishTaskArtifact(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *artifact)
	}
	return items, rows.Err()
}

func (s *Store) ListPublishTaskArtifactsByTaskID(ctx context.Context, taskID string) ([]domain.PublishTaskArtifact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, task_id, artifact_key, artifact_type, source, title,
		       file_name, mime_type, storage_key, public_url, size_bytes,
		       text_content, payload, created_at, updated_at
		FROM publish_task_artifacts
		WHERE task_id = $1
		ORDER BY updated_at DESC, created_at DESC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.PublishTaskArtifact, 0)
	for rows.Next() {
		artifact, scanErr := scanPublishTaskArtifact(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *artifact)
	}
	return items, rows.Err()
}

func (s *Store) ReplacePublishTaskMaterialRefs(ctx context.Context, taskID string, ownerUserID string, items []ReplacePublishTaskMaterialRefInput) ([]domain.PublishTaskMaterialRef, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		DELETE FROM publish_task_material_refs refs
		USING publish_tasks pt, devices d
		WHERE refs.task_id = pt.id
		  AND pt.device_id = d.id
		  AND refs.task_id = $1
		  AND d.owner_user_id = $2
	`, taskID, ownerUserID); err != nil {
		return nil, err
	}

	results := make([]domain.PublishTaskMaterialRef, 0, len(items))
	for _, item := range items {
		row := tx.QueryRow(ctx, `
			INSERT INTO publish_task_material_refs (
				id, task_id, device_id, root_name, relative_path, role, name, kind,
				absolute_path, size_bytes, modified_at, extension, mime_type, is_text, preview_text
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			RETURNING id, task_id, device_id, root_name, relative_path, role, name, kind,
			          absolute_path, size_bytes, modified_at, extension, mime_type, is_text, preview_text,
			          created_at, updated_at
		`, uuid.NewString(), taskID, item.DeviceID, item.RootName, item.RelativePath, item.Role, item.Name,
			item.Kind, item.AbsolutePath, item.SizeBytes, item.ModifiedAt, item.Extension, item.MimeType, item.IsText, item.PreviewText)

		ref, scanErr := scanPublishTaskMaterialRef(row)
		if scanErr != nil {
			return nil, scanErr
		}
		results = append(results, *ref)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Store) ListPublishTaskMaterialRefsByOwner(ctx context.Context, taskID string, ownerUserID string) ([]domain.PublishTaskMaterialRef, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT refs.id, refs.task_id, refs.device_id, refs.root_name, refs.relative_path, refs.role,
		       refs.name, refs.kind, refs.absolute_path, refs.size_bytes, refs.modified_at, refs.extension,
		       refs.mime_type, refs.is_text, refs.preview_text, refs.created_at, refs.updated_at
		FROM publish_task_material_refs refs
		INNER JOIN publish_tasks pt ON pt.id = refs.task_id
		INNER JOIN devices d ON d.id = pt.device_id
		WHERE refs.task_id = $1 AND d.owner_user_id = $2
		ORDER BY refs.created_at ASC
	`, taskID, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.PublishTaskMaterialRef, 0)
	for rows.Next() {
		ref, scanErr := scanPublishTaskMaterialRef(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *ref)
	}
	return items, rows.Err()
}

func (s *Store) ListPublishTaskMaterialRefsByTaskID(ctx context.Context, taskID string) ([]domain.PublishTaskMaterialRef, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT refs.id, refs.task_id, refs.device_id, refs.root_name, refs.relative_path, refs.role,
		       refs.name, refs.kind, refs.absolute_path, refs.size_bytes, refs.modified_at, refs.extension,
		       refs.mime_type, refs.is_text, refs.preview_text, refs.created_at, refs.updated_at
		FROM publish_task_material_refs refs
		WHERE refs.task_id = $1
		ORDER BY refs.created_at ASC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.PublishTaskMaterialRef, 0)
	for rows.Next() {
		ref, scanErr := scanPublishTaskMaterialRef(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *ref)
	}
	return items, rows.Err()
}

func (s *Store) ListPublishTasksByMaterialRef(ctx context.Context, ownerUserID string, deviceID string, rootName string, relativePath string, subtree bool, limit int) ([]domain.PublishTask, error) {
	normalizedPath := normalizeMaterialPath(relativePath)
	query := `
		SELECT * FROM (
			SELECT DISTINCT ON (pt.id)
			       pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
			       pt.title, pt.content_text, pt.media_payload, pt.status, pt.message,
			       pt.verification_payload, pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at,
			       pt.attempt_count, pt.cancel_requested_at, pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
			FROM publish_tasks pt
			INNER JOIN devices d ON d.id = pt.device_id
			INNER JOIN publish_task_material_refs refs ON refs.task_id = pt.id
			WHERE d.owner_user_id = $1
			  AND refs.device_id = $2
			  AND refs.root_name = $3
	`
	args := []any{ownerUserID, deviceID, rootName}
	argIndex := 4
	if subtree {
		query += fmt.Sprintf(" AND (refs.relative_path = $%d OR refs.relative_path LIKE $%d)", argIndex, argIndex+1)
		args = append(args, normalizedPath, normalizedPath+"/%")
		argIndex += 2
	} else {
		query += fmt.Sprintf(" AND refs.relative_path = $%d", argIndex)
		args = append(args, normalizedPath)
		argIndex++
	}
	query += `
			ORDER BY pt.id, pt.updated_at DESC
		) task_refs
		ORDER BY updated_at DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
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

func (s *Store) CountPublishTaskMaterialHealth(ctx context.Context, taskID string) (int64, int64, int64, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::BIGINT AS total_count,
			COALESCE(SUM(
				CASE
					WHEN entries.relative_path IS NOT NULL AND entries.is_available = TRUE THEN 1
					ELSE 0
				END
			), 0)::BIGINT AS available_count,
			COALESCE(SUM(
				CASE
					WHEN entries.relative_path IS NOT NULL
					 AND entries.is_available = TRUE
					 AND (
					 	refs.name IS DISTINCT FROM entries.name
					 	OR refs.kind IS DISTINCT FROM entries.kind
					 	OR refs.size_bytes IS DISTINCT FROM entries.size_bytes
					 	OR refs.modified_at IS DISTINCT FROM entries.modified_at
					 	OR refs.extension IS DISTINCT FROM entries.extension
					 	OR refs.mime_type IS DISTINCT FROM entries.mime_type
					 ) THEN 1
					ELSE 0
				END
			), 0)::BIGINT AS drifted_count
		FROM publish_task_material_refs refs
		LEFT JOIN device_material_entries entries
		  ON entries.device_id = refs.device_id
		 AND entries.root_name = refs.root_name
		 AND entries.relative_path = refs.relative_path
		WHERE refs.task_id = $1
	`, taskID)

	var totalCount int64
	var availableCount int64
	var driftedCount int64
	if err := row.Scan(&totalCount, &availableCount, &driftedCount); err != nil {
		return 0, 0, 0, err
	}
	return totalCount, availableCount, driftedCount, nil
}

func (s *Store) CountPublishTaskAvailableMaterials(ctx context.Context, taskID string) (int64, int64, error) {
	totalCount, availableCount, _, err := s.CountPublishTaskMaterialHealth(ctx, taskID)
	if err != nil {
		return 0, 0, err
	}
	return totalCount, availableCount, nil
}

func (s *Store) GetPublishTaskRuntimeStateByTaskID(ctx context.Context, taskID string) (*domain.PublishTaskRuntimeState, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT task_id, execution_payload, last_agent_sync_at, created_at, updated_at
		FROM publish_task_runtime_states
		WHERE task_id = $1
	`, taskID)

	item, err := scanPublishTaskRuntimeState(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) UpsertPublishTaskRuntimeState(ctx context.Context, input UpsertPublishTaskRuntimeStateInput) (*domain.PublishTaskRuntimeState, error) {
	var executionPayload any
	if input.ExecutionTouched {
		executionPayload = input.ExecutionPayload
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO publish_task_runtime_states (
			task_id, execution_payload, last_agent_sync_at
		)
		VALUES ($1, $2, $3)
		ON CONFLICT (task_id) DO UPDATE
		SET execution_payload = CASE
		        WHEN $4::boolean THEN $2::jsonb
		        ELSE publish_task_runtime_states.execution_payload
		    END,
		    last_agent_sync_at = COALESCE($3, publish_task_runtime_states.last_agent_sync_at),
		    updated_at = NOW()
		RETURNING task_id, execution_payload, last_agent_sync_at, created_at, updated_at
	`, input.TaskID, executionPayload, input.LastAgentSyncAt, input.ExecutionTouched)

	return scanPublishTaskRuntimeState(row)
}

func (s *Store) DeletePublishTaskRuntimeState(ctx context.Context, taskID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM publish_task_runtime_states
		WHERE task_id = $1
	`, taskID)
	return err
}

func (s *Store) DeletePublishTaskArtifactsByOwner(ctx context.Context, taskID string, ownerUserID string) (int64, error) {
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM publish_task_artifacts pta
		USING publish_tasks pt, devices d
		WHERE pta.task_id = pt.id
		  AND pt.device_id = d.id
		  AND pta.task_id = $1
		  AND d.owner_user_id = $2
	`, taskID, ownerUserID)
	if err != nil {
		return 0, err
	}
	return commandTag.RowsAffected(), nil
}

func (s *Store) DeletePublishTask(ctx context.Context, taskID string, ownerUserID string) (bool, error) {
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM publish_tasks pt
		USING devices d
		WHERE pt.device_id = d.id
		  AND pt.id = $1
		  AND d.owner_user_id = $2
	`, taskID, ownerUserID)
	if err != nil {
		return false, err
	}
	return commandTag.RowsAffected() > 0, nil
}
