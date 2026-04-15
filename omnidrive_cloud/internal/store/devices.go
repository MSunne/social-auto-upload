package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

// 处理扫描设备相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func scanDevice(row pgx.Row) (*domain.Device, error) {
	var device domain.Device
	var localIP *string
	var publicIP *string
	var model *string
	var chatModel *string
	var imageModel *string
	var videoModel *string
	var platformCapabilities []byte
	var platformCapabilitiesRevision *string
	var notes *string
	var agentKey *string
	var runtimePayload []byte
	var ownerUserID *string
	var lastSeenAt *time.Time
	var supersededByDeviceID *string
	var supersededAt *time.Time

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
		&platformCapabilities,
		&platformCapabilitiesRevision,
		&device.IsEnabled,
		&runtimePayload,
		&lastSeenAt,
		&notes,
		&supersededByDeviceID,
		&supersededAt,
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
	if len(platformCapabilities) > 0 {
		_ = json.Unmarshal(platformCapabilities, &device.PlatformCapabilities)
	}
	device.PlatformCapabilitiesRevision = platformCapabilitiesRevision
	device.RuntimePayload = bytesOrNil(runtimePayload)
	device.LastSeenAt = lastSeenAt
	device.Notes = notes
	device.SupersededByDeviceID = supersededByDeviceID
	device.SupersededAt = supersededAt
	device.IdentityState = computeDeviceIdentityState(supersededByDeviceID, supersededAt)
	device.Status = computeDeviceStatus(lastSeenAt, runtimePayload)
	device.BridgeStatus = computeDeviceBridgeStatus(lastSeenAt, runtimePayload)
	return &device, nil
}

// 处理扫描设备加载相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func scanDeviceWithLoad(row pgx.Row) (*domain.Device, error) {
	var device domain.Device
	var localIP *string
	var publicIP *string
	var model *string
	var chatModel *string
	var imageModel *string
	var videoModel *string
	var platformCapabilities []byte
	var platformCapabilitiesRevision *string
	var notes *string
	var agentKey *string
	var runtimePayload []byte
	var ownerUserID *string
	var lastSeenAt *time.Time
	var supersededByDeviceID *string
	var supersededAt *time.Time

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
		&platformCapabilities,
		&platformCapabilitiesRevision,
		&device.IsEnabled,
		&runtimePayload,
		&lastSeenAt,
		&notes,
		&supersededByDeviceID,
		&supersededAt,
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
	if len(platformCapabilities) > 0 {
		_ = json.Unmarshal(platformCapabilities, &device.PlatformCapabilities)
	}
	device.PlatformCapabilitiesRevision = platformCapabilitiesRevision
	device.RuntimePayload = bytesOrNil(runtimePayload)
	device.LastSeenAt = lastSeenAt
	device.Notes = notes
	device.SupersededByDeviceID = supersededByDeviceID
	device.SupersededAt = supersededAt
	device.IdentityState = computeDeviceIdentityState(supersededByDeviceID, supersededAt)
	device.Status = computeDeviceStatus(lastSeenAt, runtimePayload)
	device.BridgeStatus = computeDeviceBridgeStatus(lastSeenAt, runtimePayload)

	return &device, nil
}

const deviceSelectColumns = `
	id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
	default_reasoning_model, default_chat_model, default_image_model, default_video_model,
	platform_capabilities, platform_capabilities_revision,
	is_enabled, runtime_payload, last_seen_at, notes, superseded_by_device_id, superseded_at,
	created_at, updated_at
`

const deviceSelectColumnsQualified = `
	devices.id, devices.owner_user_id, devices.device_code, devices.agent_key, devices.name, devices.local_ip, devices.public_ip,
	devices.default_reasoning_model, devices.default_chat_model, devices.default_image_model, devices.default_video_model,
	devices.platform_capabilities, devices.platform_capabilities_revision,
	devices.is_enabled, devices.runtime_payload, devices.last_seen_at, devices.notes, devices.superseded_by_device_id, devices.superseded_at,
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

// 处理设备Query加载相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func deviceQueryWithLoad(whereClause string) string {
	return fmt.Sprintf(`
		SELECT %s, %s
		FROM devices
		%s
	`, deviceSelectColumns, deviceLoadColumns, whereClause)
}

// 执行设备相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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

// 执行设备相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetDeviceByID(ctx context.Context, deviceID string) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, "SELECT "+deviceSelectColumns+" FROM devices WHERE id = $1", deviceID)
	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return device, nil
}

// 执行设备相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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

// 执行设备相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetDeviceByCode(ctx context.Context, deviceCode string) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+deviceSelectColumns+`
		FROM devices
		WHERE device_code = $1
		ORDER BY
			CASE WHEN superseded_by_device_id IS NULL THEN 0 ELSE 1 END,
			updated_at DESC
		LIMIT 1
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

// 执行设备相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetDeviceByIdentity(ctx context.Context, deviceCode string, agentKey string) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+deviceSelectColumns+`
		FROM devices
		WHERE device_code = $1 AND agent_key = $2
		ORDER BY updated_at DESC
		LIMIT 1
	`, deviceCode, agentKey)
	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return device, nil
}

// 执行设备相关的租约与并发控制操作，确保调度和执行状态保持一致。
func (s *Store) ClaimDevice(ctx context.Context, deviceCode string, ownerUserID string) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, `
		WITH target AS (
			SELECT id
			FROM devices
			WHERE device_code = $1
			ORDER BY
				CASE WHEN superseded_by_device_id IS NULL THEN 0 ELSE 1 END,
				updated_at DESC
			LIMIT 1
		)
		UPDATE devices
		SET owner_user_id = $2,
		    is_enabled = TRUE,
		    updated_at = NOW()
		WHERE id = (SELECT id FROM target)
		  AND (owner_user_id IS NULL OR owner_user_id = $2)
		RETURNING `+deviceSelectColumns+`
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

// 根据设备身份更新心跳字段，避免不同写入路径重复维护同一组列。
func updateHeartbeatDeviceRecord(ctx context.Context, tx pgx.Tx, deviceID string, input HeartbeatInput, runtimePayload []byte, now time.Time) (*domain.Device, error) {
	row := tx.QueryRow(ctx, `
		UPDATE devices
		SET device_code = $2,
		    name = $3,
		    local_ip = COALESCE($4, devices.local_ip),
		    public_ip = COALESCE($5, devices.public_ip),
		    runtime_payload = $6,
		    last_seen_at = $7,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING `+deviceSelectColumns+`
	`, deviceID, input.DeviceCode, input.DeviceName, input.LocalIP, input.PublicIP, runtimePayload, now)
	return scanDevice(row)
}

// 迁移设备激活配置到新身份，并重置为可重新认领状态。
func transferDeviceActivationConfig(ctx context.Context, tx pgx.Tx, fromDeviceID string, toDeviceID string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE device_activation_configs
		SET device_id = $2,
		    status = 'ready',
		    activated_by_user_id = NULL,
		    activated_at = NULL,
		    updated_at = NOW()
		WHERE device_id = $1
	`, fromDeviceID, toDeviceID); err != nil {
		return err
	}
	return nil
}

// 执行设备相关的数据库写入，维护持久化状态与后续业务流转。
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
		RETURNING `+deviceSelectColumns+`
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

// 执行设备相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) UpsertHeartbeatDevice(ctx context.Context, input HeartbeatInput) (*domain.Device, error) {
	now := time.Now().UTC()
	runtimePayload := input.RuntimePayload
	if len(runtimePayload) == 0 {
		runtimePayload = nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	existingByIdentity, err := scanDevice(tx.QueryRow(ctx, `
		SELECT `+deviceSelectColumns+`
		FROM devices
		WHERE device_code = $1 AND agent_key = $2
		ORDER BY updated_at DESC
		LIMIT 1
		FOR UPDATE
	`, input.DeviceCode, input.AgentKey))
	if err == nil {
		device, updateErr := updateHeartbeatDeviceRecord(ctx, tx, existingByIdentity.ID, input, runtimePayload, now)
		if updateErr != nil {
			return nil, updateErr
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return device, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if input.DeviceFingerprint != "" {
		deviceByFingerprint, fingerprintErr := scanDevice(tx.QueryRow(ctx, `
			SELECT `+deviceSelectColumns+`
			FROM devices
			WHERE agent_key = $1
			  AND COALESCE(runtime_payload->>'deviceFingerprint', '') = $2
			ORDER BY updated_at DESC
			LIMIT 1
			FOR UPDATE
		`, input.AgentKey, input.DeviceFingerprint))
		if fingerprintErr == nil {
			device, updateErr := updateHeartbeatDeviceRecord(ctx, tx, deviceByFingerprint.ID, input, runtimePayload, now)
			if updateErr != nil {
				return nil, updateErr
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return device, nil
		}
		if !errors.Is(fingerprintErr, pgx.ErrNoRows) {
			return nil, fingerprintErr
		}
	}

	var predecessor *domain.Device
	predecessor, err = scanDevice(tx.QueryRow(ctx, `
		SELECT `+deviceSelectColumns+`
		FROM devices
		WHERE device_code = $1
		ORDER BY
			CASE WHEN superseded_by_device_id IS NULL THEN 0 ELSE 1 END,
			updated_at DESC
		LIMIT 1
		FOR UPDATE
	`, input.DeviceCode))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		predecessor = nil
	}

	created, err := scanDevice(tx.QueryRow(ctx, `
		INSERT INTO devices (
			id, device_code, agent_key, name, local_ip, public_ip, runtime_payload,
			is_enabled, last_seen_at, superseded_by_device_id, superseded_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, FALSE, $8, NULL, NULL)
		RETURNING `+deviceSelectColumns+`
	`,
		uuid.NewString(),
		input.DeviceCode,
		input.AgentKey,
		input.DeviceName,
		input.LocalIP,
		input.PublicIP,
		runtimePayload,
		now,
	))
	if err != nil {
		return nil, err
	}

	if predecessor != nil && predecessor.ID != created.ID {
		if _, err := tx.Exec(ctx, `
			UPDATE devices
			SET superseded_by_device_id = $2,
			    superseded_at = $3,
			    is_enabled = FALSE,
			    updated_at = NOW()
			WHERE id = $1
		`, predecessor.ID, created.ID, now); err != nil {
			return nil, err
		}
		if err := transferDeviceActivationConfig(ctx, tx, predecessor.ID, created.ID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetDeviceByIdentity(ctx, input.DeviceCode, input.AgentKey)
}

// 处理Unbind设备相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) UnbindDevice(ctx context.Context, deviceID string, ownerUserID string) (*domain.Device, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		UPDATE devices
		SET owner_user_id = NULL,
		    is_enabled = FALSE,
		    default_reasoning_model = NULL,
		    default_chat_model = NULL,
		    default_image_model = NULL,
		    default_video_model = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2
		RETURNING `+deviceSelectColumns+`
	`, deviceID, ownerUserID)

	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE device_activation_configs
		SET status = 'ready',
		    activated_by_user_id = NULL,
		    activated_at = NULL,
		    updated_at = NOW()
		WHERE device_id = $1
	`, device.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return device, nil
}

// 处理管理端Unbind设备相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) AdminUnbindDevice(ctx context.Context, deviceID string) (*domain.Device, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		UPDATE devices
		SET owner_user_id = NULL,
		    is_enabled = FALSE,
		    default_reasoning_model = NULL,
		    default_chat_model = NULL,
		    default_image_model = NULL,
		    default_video_model = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND owner_user_id IS NOT NULL
		RETURNING `+deviceSelectColumns+`
	`, deviceID)

	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE device_activation_configs
		SET status = 'ready',
		    activated_by_user_id = NULL,
		    activated_at = NULL,
		    updated_at = NOW()
		WHERE device_id = $1
	`, device.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return device, nil
}
