package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

func scanPlatformAccount(row pgx.Row) (*domain.PlatformAccount, error) {
	var account domain.PlatformAccount
	var lastMessage *string
	var lastAuthenticatedAt *time.Time

	if err := row.Scan(
		&account.ID,
		&account.DeviceID,
		&account.Platform,
		&account.AccountName,
		&account.Status,
		&lastMessage,
		&lastAuthenticatedAt,
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		return nil, err
	}

	account.LastMessage = lastMessage
	account.LastAuthenticatedAt = lastAuthenticatedAt
	return &account, nil
}

func scanPlatformAccountWithLoad(row pgx.Row) (*domain.PlatformAccount, error) {
	var account domain.PlatformAccount
	var lastMessage *string
	var lastAuthenticatedAt *time.Time

	if err := row.Scan(
		&account.ID,
		&account.DeviceID,
		&account.Platform,
		&account.AccountName,
		&account.Status,
		&lastMessage,
		&lastAuthenticatedAt,
		&account.CreatedAt,
		&account.UpdatedAt,
		&account.Load.TaskCount,
		&account.Load.PendingTaskCount,
		&account.Load.RunningTaskCount,
		&account.Load.NeedsVerifyTaskCount,
		&account.Load.FailedTaskCount,
		&account.Load.ActiveLoginSessionCount,
		&account.Load.VerificationLoginSessionCount,
	); err != nil {
		return nil, err
	}

	account.LastMessage = lastMessage
	account.LastAuthenticatedAt = lastAuthenticatedAt
	return &account, nil
}

const platformAccountSelectColumns = `
	pa.id, pa.device_id, pa.platform, pa.account_name, pa.status, pa.last_message,
	pa.last_authenticated_at, pa.created_at, pa.updated_at
`

const platformAccountLoadColumns = `
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.device_id = pa.device_id AND pt.platform = pa.platform AND pt.account_name = pa.account_name), 0)::BIGINT AS task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.device_id = pa.device_id AND pt.platform = pa.platform AND pt.account_name = pa.account_name AND pt.status = 'pending'), 0)::BIGINT AS pending_task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.device_id = pa.device_id AND pt.platform = pa.platform AND pt.account_name = pa.account_name AND pt.status = 'running'), 0)::BIGINT AS running_task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.device_id = pa.device_id AND pt.platform = pa.platform AND pt.account_name = pa.account_name AND pt.status = 'needs_verify'), 0)::BIGINT AS needs_verify_task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.device_id = pa.device_id AND pt.platform = pa.platform AND pt.account_name = pa.account_name AND pt.status = 'failed'), 0)::BIGINT AS failed_task_count,
	COALESCE((SELECT COUNT(*) FROM login_sessions ls WHERE ls.device_id = pa.device_id AND ls.platform = pa.platform AND ls.account_name = pa.account_name AND ls.status IN ('pending', 'running', 'verification_required')), 0)::BIGINT AS active_login_session_count,
	COALESCE((SELECT COUNT(*) FROM login_sessions ls WHERE ls.device_id = pa.device_id AND ls.platform = pa.platform AND ls.account_name = pa.account_name AND ls.status = 'verification_required'), 0)::BIGINT AS verification_login_session_count
`

func platformAccountQueryWithLoad(whereClause string) string {
	return fmt.Sprintf(`
		SELECT %s, %s
		FROM platform_accounts pa
		INNER JOIN devices d ON d.id = pa.device_id
		%s
	`, platformAccountSelectColumns, platformAccountLoadColumns, whereClause)
}

func (s *Store) ListAccountsByOwner(ctx context.Context, ownerUserID string, deviceID string) ([]domain.PlatformAccount, error) {
	query := platformAccountQueryWithLoad(`WHERE d.owner_user_id = $1`)
	args := []any{ownerUserID}
	if deviceID != "" {
		query += ` AND pa.device_id = $2`
		args = append(args, deviceID)
	}
	query += ` ORDER BY pa.updated_at DESC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.PlatformAccount, 0)
	for rows.Next() {
		account, scanErr := scanPlatformAccountWithLoad(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *account)
	}
	return items, rows.Err()
}

func (s *Store) GetOwnedAccountByID(ctx context.Context, accountID string, ownerUserID string) (*domain.PlatformAccount, error) {
	row := s.pool.QueryRow(ctx, platformAccountQueryWithLoad(`
		WHERE pa.id = $1 AND d.owner_user_id = $2
	`), accountID, ownerUserID)

	account, err := scanPlatformAccountWithLoad(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return account, nil
}

func (s *Store) GetAccountByDeviceTarget(ctx context.Context, deviceID string, platform string, accountName string) (*domain.PlatformAccount, error) {
	row := s.pool.QueryRow(ctx, platformAccountQueryWithLoad(`
		WHERE pa.device_id = $1 AND pa.platform = $2 AND pa.account_name = $3
	`), deviceID, platform, accountName)

	account, err := scanPlatformAccountWithLoad(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return account, nil
}

func (s *Store) ListPublishTasksByAccountTarget(ctx context.Context, ownerUserID string, deviceID string, platform string, accountName string, limit int) ([]domain.PublishTask, error) {
	query := `
		SELECT pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		       pt.title, pt.content_text, pt.media_payload, pt.status, pt.message,
		       pt.verification_payload, pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at,
		       pt.attempt_count, pt.cancel_requested_at, pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
		FROM publish_tasks pt
		INNER JOIN devices d ON d.id = pt.device_id
		WHERE d.owner_user_id = $1
		  AND pt.device_id = $2
		  AND pt.platform = $3
		  AND pt.account_name = $4
		ORDER BY pt.updated_at DESC
	`
	args := []any{ownerUserID, deviceID, platform, accountName}
	if limit > 0 {
		query += ` LIMIT $5`
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

func (s *Store) DeleteOwnedAccount(ctx context.Context, accountID string, ownerUserID string) (bool, error) {
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM platform_accounts pa
		USING devices d
		WHERE pa.device_id = d.id
		  AND pa.id = $1
		  AND d.owner_user_id = $2
	`, accountID, ownerUserID)
	if err != nil {
		return false, err
	}
	return commandTag.RowsAffected() > 0, nil
}

func (s *Store) GetAccountUsageSummary(ctx context.Context, accountID string, ownerUserID string) (int64, int64, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE((
				SELECT COUNT(*)
				FROM publish_tasks pt
				INNER JOIN platform_accounts pa ON pa.id = $1
				INNER JOIN devices d ON d.id = pa.device_id
				WHERE pa.id = $1
				  AND d.owner_user_id = $2
				  AND pt.device_id = pa.device_id
				  AND pt.platform = pa.platform
				  AND pt.account_name = pa.account_name
			), 0)::BIGINT,
			COALESCE((
				SELECT COUNT(*)
				FROM login_sessions ls
				INNER JOIN platform_accounts pa ON pa.id = $1
				INNER JOIN devices d ON d.id = pa.device_id
				WHERE pa.id = $1
				  AND d.owner_user_id = $2
				  AND ls.device_id = pa.device_id
				  AND ls.platform = pa.platform
				  AND ls.account_name = pa.account_name
				  AND ls.status IN ('pending', 'running', 'verification_required')
			), 0)::BIGINT
	`, accountID, ownerUserID)

	var taskCount int64
	var activeLoginSessionCount int64
	if err := row.Scan(&taskCount, &activeLoginSessionCount); err != nil {
		return 0, 0, err
	}
	return taskCount, activeLoginSessionCount, nil
}

func scanLoginSession(row pgx.Row) (*domain.LoginSession, error) {
	var session domain.LoginSession
	var qrData *string
	var verificationPayload []byte
	var message *string

	if err := row.Scan(
		&session.ID,
		&session.DeviceID,
		&session.UserID,
		&session.Platform,
		&session.AccountName,
		&session.Status,
		&qrData,
		&verificationPayload,
		&message,
		&session.CreatedAt,
		&session.UpdatedAt,
	); err != nil {
		return nil, err
	}

	session.QRData = qrData
	session.VerificationPayload = bytesOrNil(verificationPayload)
	session.Message = message
	return &session, nil
}

func (s *Store) CreateLoginSession(ctx context.Context, input CreateLoginSessionInput) (*domain.LoginSession, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO login_sessions (id, device_id, user_id, platform, account_name, status, message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, device_id, user_id, platform, account_name, status, qr_data,
		          verification_payload, message, created_at, updated_at
	`, input.ID, input.DeviceID, input.UserID, input.Platform, input.AccountName, input.Status, input.Message)

	return scanLoginSession(row)
}

func (s *Store) GetLoginSessionByID(ctx context.Context, sessionID string) (*domain.LoginSession, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, device_id, user_id, platform, account_name, status, qr_data,
		       verification_payload, message, created_at, updated_at
		FROM login_sessions
		WHERE id = $1
	`, sessionID)

	session, err := scanLoginSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return session, nil
}

func (s *Store) GetOwnedLoginSession(ctx context.Context, sessionID string, ownerUserID string) (*domain.LoginSession, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT ls.id, ls.device_id, ls.user_id, ls.platform, ls.account_name, ls.status, ls.qr_data,
		       ls.verification_payload, ls.message, ls.created_at, ls.updated_at
		FROM login_sessions ls
		INNER JOIN devices d ON d.id = ls.device_id
		WHERE ls.id = $1 AND d.owner_user_id = $2
	`, sessionID, ownerUserID)

	session, err := scanLoginSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return session, nil
}

func (s *Store) ListActiveLoginSessionsByOwner(ctx context.Context, ownerUserID string, deviceID string, limit int) ([]domain.LoginSession, error) {
	query := `
		SELECT ls.id, ls.device_id, ls.user_id, ls.platform, ls.account_name, ls.status, ls.qr_data,
		       ls.verification_payload, ls.message, ls.created_at, ls.updated_at
		FROM login_sessions ls
		INNER JOIN devices d ON d.id = ls.device_id
		WHERE d.owner_user_id = $1
		  AND ls.status IN ('pending', 'running', 'verification_required')
	`
	args := []any{ownerUserID}
	if deviceID != "" {
		query += ` AND ls.device_id = $2`
		args = append(args, deviceID)
	}
	query += ` ORDER BY ls.updated_at DESC`
	if limit > 0 {
		query += ` LIMIT $3`
		if deviceID == "" {
			query = `
				SELECT ls.id, ls.device_id, ls.user_id, ls.platform, ls.account_name, ls.status, ls.qr_data,
				       ls.verification_payload, ls.message, ls.created_at, ls.updated_at
				FROM login_sessions ls
				INNER JOIN devices d ON d.id = ls.device_id
				WHERE d.owner_user_id = $1
				  AND ls.status IN ('pending', 'running', 'verification_required')
				ORDER BY ls.updated_at DESC
				LIMIT $2
			`
			args = []any{ownerUserID, limit}
		} else {
			args = append(args, limit)
		}
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.LoginSession, 0)
	for rows.Next() {
		session, scanErr := scanLoginSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *session)
	}
	return items, rows.Err()
}

func (s *Store) ListLoginSessionsByAccountTarget(ctx context.Context, ownerUserID string, deviceID string, platform string, accountName string, limit int) ([]domain.LoginSession, error) {
	query := `
		SELECT ls.id, ls.device_id, ls.user_id, ls.platform, ls.account_name, ls.status, ls.qr_data,
		       ls.verification_payload, ls.message, ls.created_at, ls.updated_at
		FROM login_sessions ls
		INNER JOIN devices d ON d.id = ls.device_id
		WHERE d.owner_user_id = $1
		  AND ls.device_id = $2
		  AND ls.platform = $3
		  AND ls.account_name = $4
		  AND ls.status IN ('pending', 'running', 'verification_required')
		ORDER BY ls.updated_at DESC
	`
	args := []any{ownerUserID, deviceID, platform, accountName}
	if limit > 0 {
		query += ` LIMIT $5`
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.LoginSession, 0)
	for rows.Next() {
		session, scanErr := scanLoginSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *session)
	}
	return items, rows.Err()
}

func (s *Store) ListPendingLoginTasksByDevice(ctx context.Context, deviceID string) ([]domain.LoginSession, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, device_id, user_id, platform, account_name, status, qr_data,
		       verification_payload, message, created_at, updated_at
		FROM login_sessions
		WHERE device_id = $1 AND status IN ('pending', 'running', 'verification_required')
		ORDER BY created_at ASC
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.LoginSession, 0)
	for rows.Next() {
		session, scanErr := scanLoginSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *session)
	}
	return items, rows.Err()
}

func (s *Store) UpdateLoginSessionEvent(ctx context.Context, sessionID string, input LoginEventInput) (*domain.LoginSession, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE login_sessions
		SET status = $2,
		    message = $3,
		    qr_data = $4,
		    verification_payload = $5,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, device_id, user_id, platform, account_name, status, qr_data,
		          verification_payload, message, created_at, updated_at
	`, sessionID, input.Status, input.Message, input.QRData, input.VerificationPayload)

	session, err := scanLoginSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return session, nil
}

func (s *Store) UpsertPlatformAccountFromLogin(ctx context.Context, session *domain.LoginSession) error {
	if session.Status != "success" && session.Status != "active" {
		return nil
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO platform_accounts (
			id, device_id, platform, account_name, status, last_message, last_authenticated_at
		)
		VALUES ($1, $2, $3, $4, 'active', $5, NOW())
		ON CONFLICT (device_id, platform, account_name) DO UPDATE
		SET status = 'active',
		    last_message = EXCLUDED.last_message,
		    last_authenticated_at = NOW(),
		    updated_at = NOW()
	`, uuid.NewString(), session.DeviceID, session.Platform, session.AccountName, session.Message)
	return err
}

func (s *Store) UpsertPlatformAccount(ctx context.Context, deviceID string, platform string, accountName string, status string, lastMessage *string, lastAuthenticatedAt *time.Time) (*domain.PlatformAccount, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO platform_accounts (
			id, device_id, platform, account_name, status, last_message, last_authenticated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (device_id, platform, account_name) DO UPDATE
		SET status = EXCLUDED.status,
		    last_message = EXCLUDED.last_message,
		    last_authenticated_at = COALESCE(EXCLUDED.last_authenticated_at, platform_accounts.last_authenticated_at),
		    updated_at = NOW()
		RETURNING id, device_id, platform, account_name, status, last_message,
		          last_authenticated_at, created_at, updated_at
	`, uuid.NewString(), deviceID, platform, accountName, status, lastMessage, lastAuthenticatedAt)

	return scanPlatformAccount(row)
}

func scanLoginAction(row pgx.Row) (*domain.LoginSessionAction, error) {
	var action domain.LoginSessionAction
	var payload []byte
	var consumedAt *time.Time

	if err := row.Scan(
		&action.ID,
		&action.SessionID,
		&action.ActionType,
		&payload,
		&action.Status,
		&action.CreatedAt,
		&consumedAt,
	); err != nil {
		return nil, err
	}

	action.Payload = bytesOrNil(payload)
	action.ConsumedAt = consumedAt
	return &action, nil
}

func (s *Store) CreateLoginAction(ctx context.Context, input CreateLoginActionInput) (*domain.LoginSessionAction, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO login_session_actions (id, session_id, action_type, payload, status)
		VALUES ($1, $2, $3, $4, 'pending')
		RETURNING id, session_id, action_type, payload, status, created_at, consumed_at
	`, input.ID, input.SessionID, input.ActionType, input.Payload)

	return scanLoginAction(row)
}

func (s *Store) ConsumePendingLoginActions(ctx context.Context, sessionID string) ([]domain.LoginSessionAction, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE login_session_actions
		SET status = 'consumed',
		    consumed_at = NOW()
		WHERE id IN (
			SELECT id
			FROM login_session_actions
			WHERE session_id = $1 AND status = 'pending'
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, session_id, action_type, payload, status, created_at, consumed_at
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.LoginSessionAction, 0)
	for rows.Next() {
		action, scanErr := scanLoginAction(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *action)
	}
	return items, rows.Err()
}
