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

func scanDevice(row pgx.Row) (*domain.Device, error) {
	var device domain.Device
	var localIP *string
	var publicIP *string
	var model *string
	var chatModel *string
	var imageModel *string
	var videoModel *string
	var notes *string
	var agentKey *string
	var runtimePayload []byte
	var ownerUserID *string
	var lastSeenAt *time.Time

	if err := row.Scan(
		&device.ID,
		&ownerUserID,
		&device.DeviceCode,
		&agentKey,
		&device.Name,
		&localIP,
		&publicIP,
		&model,
		&chatModel,
		&imageModel,
		&videoModel,
		&device.IsEnabled,
		&runtimePayload,
		&lastSeenAt,
		&notes,
		&device.CreatedAt,
		&device.UpdatedAt,
	); err != nil {
		return nil, err
	}

	device.OwnerUserID = ownerUserID
	if agentKey != nil {
		device.AgentKey = *agentKey
	}
	device.LocalIP = localIP
	device.PublicIP = publicIP
	device.DefaultReasoningModel = model
	device.DefaultChatModel = chatModel
	device.DefaultImageModel = imageModel
	device.DefaultVideoModel = videoModel
	device.RuntimePayload = bytesOrNil(runtimePayload)
	device.LastSeenAt = lastSeenAt
	device.Notes = notes
	device.Status = computeDeviceStatus(lastSeenAt)
	return &device, nil
}

func scanDeviceWithLoad(row pgx.Row) (*domain.Device, error) {
	var device domain.Device
	var localIP *string
	var publicIP *string
	var model *string
	var chatModel *string
	var imageModel *string
	var videoModel *string
	var notes *string
	var agentKey *string
	var runtimePayload []byte
	var ownerUserID *string
	var lastSeenAt *time.Time

	if err := row.Scan(
		&device.ID,
		&ownerUserID,
		&device.DeviceCode,
		&agentKey,
		&device.Name,
		&localIP,
		&publicIP,
		&model,
		&chatModel,
		&imageModel,
		&videoModel,
		&device.IsEnabled,
		&runtimePayload,
		&lastSeenAt,
		&notes,
		&device.CreatedAt,
		&device.UpdatedAt,
		&device.Load.AccountCount,
		&device.Load.ActiveAccountCount,
		&device.Load.MaterialRootCount,
		&device.Load.MaterialEntryCount,
		&device.Load.PendingTaskCount,
		&device.Load.RunningTaskCount,
		&device.Load.NeedsVerifyTaskCount,
		&device.Load.CancelRequestedTaskCount,
		&device.Load.FailedTaskCount,
		&device.Load.ActiveLoginSessionCount,
		&device.Load.VerificationLoginSessionCount,
		&device.Load.LeasedTaskCount,
		&device.Load.LeasedAIJobCount,
	); err != nil {
		return nil, err
	}

	device.OwnerUserID = ownerUserID
	if agentKey != nil {
		device.AgentKey = *agentKey
	}
	device.LocalIP = localIP
	device.PublicIP = publicIP
	device.DefaultReasoningModel = model
	device.DefaultChatModel = chatModel
	device.DefaultImageModel = imageModel
	device.DefaultVideoModel = videoModel
	device.RuntimePayload = bytesOrNil(runtimePayload)
	device.LastSeenAt = lastSeenAt
	device.Notes = notes
	device.Status = computeDeviceStatus(lastSeenAt)

	return &device, nil
}

const deviceSelectColumns = `
	id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
	default_reasoning_model, default_chat_model, default_image_model, default_video_model,
	is_enabled, runtime_payload, last_seen_at, notes,
	created_at, updated_at
`

const deviceSelectColumnsQualified = `
	devices.id, devices.owner_user_id, devices.device_code, devices.agent_key, devices.name, devices.local_ip, devices.public_ip,
	devices.default_reasoning_model, devices.default_chat_model, devices.default_image_model, devices.default_video_model,
	devices.is_enabled, devices.runtime_payload, devices.last_seen_at, devices.notes,
	devices.created_at, devices.updated_at
`

const deviceLoadColumns = `
	COALESCE((SELECT COUNT(*) FROM platform_accounts pa WHERE pa.device_id = devices.id), 0)::BIGINT AS account_count,
	COALESCE((SELECT COUNT(*) FROM platform_accounts pa WHERE pa.device_id = devices.id AND pa.status = 'active'), 0)::BIGINT AS active_account_count,
	COALESCE((SELECT COUNT(*) FROM device_material_roots mr WHERE mr.device_id = devices.id AND mr.is_available = TRUE), 0)::BIGINT AS material_root_count,
	COALESCE((SELECT COUNT(*) FROM device_material_entries me WHERE me.device_id = devices.id AND me.is_available = TRUE), 0)::BIGINT AS material_entry_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.device_id = devices.id AND pt.status = 'pending'), 0)::BIGINT AS pending_task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.device_id = devices.id AND pt.status = 'running'), 0)::BIGINT AS running_task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.device_id = devices.id AND pt.status = 'needs_verify'), 0)::BIGINT AS needs_verify_task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.device_id = devices.id AND pt.status = 'cancel_requested'), 0)::BIGINT AS cancel_requested_task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.device_id = devices.id AND pt.status = 'failed'), 0)::BIGINT AS failed_task_count,
	COALESCE((SELECT COUNT(*) FROM login_sessions ls WHERE ls.device_id = devices.id AND ls.status IN ('pending', 'running', 'verification_required')), 0)::BIGINT AS active_login_session_count,
	COALESCE((SELECT COUNT(*) FROM login_sessions ls WHERE ls.device_id = devices.id AND ls.status = 'verification_required'), 0)::BIGINT AS verification_login_session_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.lease_owner_device_id = devices.id AND pt.status IN ('running', 'cancel_requested') AND pt.lease_token IS NOT NULL), 0)::BIGINT AS leased_task_count,
	COALESCE((SELECT COUNT(*) FROM ai_jobs aj WHERE aj.lease_owner_device_id = devices.id AND aj.status = 'running' AND aj.lease_token IS NOT NULL), 0)::BIGINT AS leased_ai_job_count
`

func deviceQueryWithLoad(whereClause string) string {
	return fmt.Sprintf(`
		SELECT %s, %s
		FROM devices
		%s
	`, deviceSelectColumns, deviceLoadColumns, whereClause)
}

func (s *Store) ListDevicesByOwner(ctx context.Context, ownerUserID string) ([]domain.Device, error) {
	rows, err := s.pool.Query(ctx, deviceQueryWithLoad(`
		WHERE owner_user_id = $1
		ORDER BY updated_at DESC
	`), ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Device, 0)
	for rows.Next() {
		device, scanErr := scanDeviceWithLoad(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *device)
	}
	return items, rows.Err()
}

func (s *Store) GetDeviceByID(ctx context.Context, deviceID string) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
		       default_reasoning_model, default_chat_model, default_image_model, default_video_model,
		       is_enabled, runtime_payload, last_seen_at, notes,
		       created_at, updated_at
		FROM devices
		WHERE id = $1
	`, deviceID)
	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return device, nil
}

func (s *Store) GetOwnedDevice(ctx context.Context, deviceID string, ownerUserID string) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, deviceQueryWithLoad(`
		WHERE id = $1 AND owner_user_id = $2
	`), deviceID, ownerUserID)
	device, err := scanDeviceWithLoad(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return device, nil
}

func (s *Store) GetDeviceByCode(ctx context.Context, deviceCode string) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
		       default_reasoning_model, default_chat_model, default_image_model, default_video_model,
		       is_enabled, runtime_payload, last_seen_at, notes,
		       created_at, updated_at
		FROM devices
		WHERE device_code = $1
	`, deviceCode)
	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return device, nil
}

func (s *Store) ClaimDevice(ctx context.Context, deviceCode string, ownerUserID string) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE devices
		SET owner_user_id = $2,
		    is_enabled = TRUE,
		    updated_at = NOW()
		WHERE device_code = $1
		  AND (owner_user_id IS NULL OR owner_user_id = $2)
		RETURNING id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
		          default_reasoning_model, default_chat_model, default_image_model, default_video_model,
		          is_enabled, runtime_payload, last_seen_at, notes,
		          created_at, updated_at
	`, deviceCode, ownerUserID)

	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return s.GetOwnedDevice(ctx, device.ID, ownerUserID)
}

func (s *Store) UpdateDevice(ctx context.Context, deviceID string, ownerUserID string, input UpdateDeviceInput) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE devices
		SET name = COALESCE($3, name),
		    default_reasoning_model = COALESCE($4, default_reasoning_model),
		    default_chat_model = COALESCE($5, default_chat_model),
		    default_image_model = COALESCE($6, default_image_model),
		    default_video_model = COALESCE($7, default_video_model),
		    is_enabled = COALESCE($8, is_enabled),
		    updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2
		RETURNING id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
		          default_reasoning_model, default_chat_model, default_image_model, default_video_model,
		          is_enabled, runtime_payload, last_seen_at, notes,
		          created_at, updated_at
	`, deviceID, ownerUserID, input.Name, input.DefaultReasoningModel, input.DefaultChatModel, input.DefaultImageModel, input.DefaultVideoModel, input.IsEnabled)

	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return s.GetOwnedDevice(ctx, device.ID, ownerUserID)
}

func (s *Store) UpsertHeartbeatDevice(ctx context.Context, input HeartbeatInput) (*domain.Device, error) {
	now := time.Now().UTC()
	runtimePayload := input.RuntimePayload
	if len(runtimePayload) == 0 {
		runtimePayload = nil
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO devices (
			id, device_code, agent_key, name, local_ip, public_ip, runtime_payload,
			is_enabled, last_seen_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, FALSE, $8)
		ON CONFLICT (device_code) DO UPDATE
		SET agent_key = CASE
				WHEN devices.agent_key IS NULL OR devices.agent_key = EXCLUDED.agent_key
				THEN EXCLUDED.agent_key
				ELSE devices.agent_key
			END,
		    name = EXCLUDED.name,
		    local_ip = EXCLUDED.local_ip,
		    public_ip = EXCLUDED.public_ip,
		    runtime_payload = EXCLUDED.runtime_payload,
		    last_seen_at = EXCLUDED.last_seen_at,
		    updated_at = NOW()
		RETURNING id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
		          default_reasoning_model, default_chat_model, default_image_model, default_video_model,
		          is_enabled, runtime_payload, last_seen_at, notes,
		          created_at, updated_at
	`,
		uuid.NewString(),
		input.DeviceCode,
		input.AgentKey,
		input.DeviceName,
		input.LocalIP,
		input.PublicIP,
		runtimePayload,
		now,
	)

	device, err := scanDevice(row)
	if err != nil {
		return nil, err
	}
	return device, nil
}

func (s *Store) UnbindDevice(ctx context.Context, deviceID string, ownerUserID string) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE devices
		SET owner_user_id = NULL,
		    is_enabled = FALSE,
		    default_reasoning_model = NULL,
		    default_chat_model = NULL,
		    default_image_model = NULL,
		    default_video_model = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2
		RETURNING id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
		          default_reasoning_model, default_chat_model, default_image_model, default_video_model,
		          is_enabled, runtime_payload, last_seen_at, notes, created_at, updated_at
	`, deviceID, ownerUserID)

	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return device, nil
}
