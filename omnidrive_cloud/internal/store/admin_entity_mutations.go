package store

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

type UpdateAdminUserTargetInput struct {
	Name         *string
	IsActive     *bool
	Notes        *string
	NotesTouched bool
}

type UpdateAdminDeviceTargetInput struct {
	Name                  *string
	DefaultReasoningModel *string
	IsEnabled             *bool
}

type UpdateAdminMediaAccountTargetInput struct {
	Notes        *string
	NotesTouched bool
}

type UpdateAdminPublishTaskTargetInput struct {
	Notes                  *string
	NotesTouched           bool
	ExceptionReason        *string
	ExceptionReasonTouched bool
	RiskTags               []string
	RiskTagsTouched        bool
}

type UpdateAdminAIJobTargetInput struct {
	Notes                  *string
	NotesTouched           bool
	ExceptionReason        *string
	ExceptionReasonTouched bool
	RiskTags               []string
	RiskTagsTouched        bool
}

func trimOptionalStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func (s *Store) UpdateAdminUserTarget(ctx context.Context, userID string, input UpdateAdminUserTargetInput) (*domain.AdminUserRow, error) {
	var nameValue any
	if input.Name != nil {
		nameValue = strings.TrimSpace(*input.Name)
	}
	notesValue := ""
	if input.Notes != nil {
		notesValue = strings.TrimSpace(*input.Notes)
	}

	commandTag, err := s.pool.Exec(ctx, `
		UPDATE users
		SET
			name = COALESCE($2, name),
			is_active = COALESCE($3, is_active),
			notes = CASE
				WHEN $4 THEN NULLIF($5, '')
				ELSE notes
			END,
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(userID), nameValue, input.IsActive, input.NotesTouched, notesValue)
	if err != nil {
		return nil, err
	}
	if commandTag.RowsAffected() == 0 {
		return nil, nil
	}
	return s.GetAdminUserByID(ctx, userID)
}

func (s *Store) UpdateAdminDeviceTarget(ctx context.Context, deviceID string, input UpdateAdminDeviceTargetInput) (*domain.AdminDeviceRow, error) {
	var nameValue any
	if input.Name != nil {
		nameValue = strings.TrimSpace(*input.Name)
	}
	var modelValue any
	if input.DefaultReasoningModel != nil {
		modelValue = strings.TrimSpace(*input.DefaultReasoningModel)
	}

	commandTag, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET
			name = COALESCE($2, name),
			default_reasoning_model = COALESCE($3, default_reasoning_model),
			is_enabled = COALESCE($4, is_enabled),
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(deviceID), nameValue, modelValue, input.IsEnabled)
	if err != nil {
		return nil, err
	}
	if commandTag.RowsAffected() == 0 {
		return nil, nil
	}
	return s.GetAdminDeviceByID(ctx, deviceID)
}

func (s *Store) UpdateAdminMediaAccountTarget(ctx context.Context, accountID string, input UpdateAdminMediaAccountTargetInput) (*domain.AdminMediaAccountRow, error) {
	notesValue := ""
	if input.Notes != nil {
		notesValue = strings.TrimSpace(*input.Notes)
	}

	commandTag, err := s.pool.Exec(ctx, `
		UPDATE platform_accounts
		SET
			notes = CASE
				WHEN $2 THEN NULLIF($3, '')
				ELSE notes
			END,
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(accountID), input.NotesTouched, notesValue)
	if err != nil {
		return nil, err
	}
	if commandTag.RowsAffected() == 0 {
		return nil, nil
	}
	return s.GetAdminAccountByID(ctx, accountID)
}

func (s *Store) UpdateAdminPublishTaskTarget(ctx context.Context, taskID string, input UpdateAdminPublishTaskTargetInput) (*domain.AdminPublishTaskRow, error) {
	notesValue := ""
	if input.Notes != nil {
		notesValue = strings.TrimSpace(*input.Notes)
	}
	exceptionReasonValue := ""
	if input.ExceptionReason != nil {
		exceptionReasonValue = strings.TrimSpace(*input.ExceptionReason)
	}
	riskTags := normalizeTextValues(input.RiskTags)
	if input.RiskTagsTouched && riskTags == nil {
		riskTags = []string{}
	}
	riskTagsPayload, err := json.Marshal(riskTags)
	if err != nil {
		return nil, err
	}

	commandTag, err := s.pool.Exec(ctx, `
		UPDATE publish_tasks
		SET
			notes = CASE
				WHEN $2 THEN NULLIF($3, '')
				ELSE notes
			END,
			exception_reason = CASE
				WHEN $4 THEN NULLIF($5, '')
				ELSE exception_reason
			END,
			risk_tags = CASE
				WHEN $6 THEN COALESCE($7::jsonb, '[]'::jsonb)
				ELSE risk_tags
			END,
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(taskID), input.NotesTouched, notesValue, input.ExceptionReasonTouched, exceptionReasonValue, input.RiskTagsTouched, riskTagsPayload)
	if err != nil {
		return nil, err
	}
	if commandTag.RowsAffected() == 0 {
		return nil, nil
	}
	return s.GetAdminTaskByID(ctx, taskID)
}

func (s *Store) UpdateAdminAIJobTarget(ctx context.Context, jobID string, input UpdateAdminAIJobTargetInput) (*domain.AdminAIJobRow, error) {
	notesValue := ""
	if input.Notes != nil {
		notesValue = strings.TrimSpace(*input.Notes)
	}
	exceptionReasonValue := ""
	if input.ExceptionReason != nil {
		exceptionReasonValue = strings.TrimSpace(*input.ExceptionReason)
	}
	riskTags := normalizeTextValues(input.RiskTags)
	if input.RiskTagsTouched && riskTags == nil {
		riskTags = []string{}
	}
	riskTagsPayload, err := json.Marshal(riskTags)
	if err != nil {
		return nil, err
	}

	commandTag, err := s.pool.Exec(ctx, `
		UPDATE ai_jobs
		SET
			notes = CASE
				WHEN $2 THEN NULLIF($3, '')
				ELSE notes
			END,
			exception_reason = CASE
				WHEN $4 THEN NULLIF($5, '')
				ELSE exception_reason
			END,
			risk_tags = CASE
				WHEN $6 THEN COALESCE($7::jsonb, '[]'::jsonb)
				ELSE risk_tags
			END,
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(jobID), input.NotesTouched, notesValue, input.ExceptionReasonTouched, exceptionReasonValue, input.RiskTagsTouched, riskTagsPayload)
	if err != nil {
		return nil, err
	}
	if commandTag.RowsAffected() == 0 {
		return nil, nil
	}
	return s.GetAdminAIJobByID(ctx, jobID)
}

func (s *Store) ForceReleasePublishTaskLeasesByDevice(ctx context.Context, deviceID string, message *string) ([]domain.PublishTask, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	rows, err := tx.Query(ctx, `
		UPDATE publish_tasks pt
		SET status = CASE
		        WHEN pt.status = 'cancel_requested' THEN 'cancelled'
		        ELSE 'pending'
		    END,
		    message = COALESCE(
		        $2,
		        CASE
		            WHEN pt.status = 'cancel_requested' THEN '任务租约已由运营后台按设备释放并标记为取消'
		            ELSE '任务租约已由运营后台按设备释放并重新排队'
		        END
		    ),
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
		WHERE pt.lease_owner_device_id = $1
		  AND pt.status IN ('running', 'cancel_requested')
		  AND pt.lease_token IS NOT NULL
		RETURNING pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		          pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.verification_payload,
		          pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at, pt.attempt_count, pt.cancel_requested_at,
		          pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
	`, strings.TrimSpace(deviceID), message)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]domain.PublishTask, 0)
	taskIDs := make([]string, 0)
	for rows.Next() {
		task, scanErr := scanPublishTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		tasks = append(tasks, *task)
		taskIDs = append(taskIDs, task.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(taskIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			DELETE FROM publish_task_runtime_states
			WHERE task_id = ANY($1)
		`, taskIDs); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Store) ForceReleaseAIJobLeasesByDevice(ctx context.Context, deviceID string, message *string) ([]domain.AIJob, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE ai_jobs
		SET status = 'queued',
		    message = COALESCE($2, 'AI 任务租约已由运营后台按设备释放并重新排队'),
		    lease_owner_device_id = NULL,
		    lease_token = NULL,
		    lease_expires_at = NULL,
		    finished_at = NULL,
		    updated_at = NOW()
		WHERE lease_owner_device_id = $1
		  AND status = 'running'
		  AND lease_token IS NOT NULL
		RETURNING `+aiJobSelectColumns+`
	`, strings.TrimSpace(deviceID), message)
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
