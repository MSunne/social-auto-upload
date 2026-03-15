package store

import (
	"context"
	"errors"
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
	device.RuntimePayload = bytesOrNil(runtimePayload)
	device.LastSeenAt = lastSeenAt
	device.Notes = notes
	device.Status = computeDeviceStatus(lastSeenAt)
	return &device, nil
}

func (s *Store) ListDevicesByOwner(ctx context.Context, ownerUserID string) ([]domain.Device, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
		       default_reasoning_model, is_enabled, runtime_payload, last_seen_at, notes,
		       created_at, updated_at
		FROM devices
		WHERE owner_user_id = $1
		ORDER BY updated_at DESC
	`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Device, 0)
	for rows.Next() {
		device, scanErr := scanDevice(rows)
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
		       default_reasoning_model, is_enabled, runtime_payload, last_seen_at, notes,
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
	row := s.pool.QueryRow(ctx, `
		SELECT id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
		       default_reasoning_model, is_enabled, runtime_payload, last_seen_at, notes,
		       created_at, updated_at
		FROM devices
		WHERE id = $1 AND owner_user_id = $2
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

func (s *Store) GetDeviceByCode(ctx context.Context, deviceCode string) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
		       default_reasoning_model, is_enabled, runtime_payload, last_seen_at, notes,
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
		          default_reasoning_model, is_enabled, runtime_payload, last_seen_at, notes,
		          created_at, updated_at
	`, deviceCode, ownerUserID)

	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return device, nil
}

func (s *Store) UpdateDevice(ctx context.Context, deviceID string, ownerUserID string, input UpdateDeviceInput) (*domain.Device, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE devices
		SET name = COALESCE($3, name),
		    default_reasoning_model = COALESCE($4, default_reasoning_model),
		    is_enabled = COALESCE($5, is_enabled),
		    updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2
		RETURNING id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
		          default_reasoning_model, is_enabled, runtime_payload, last_seen_at, notes,
		          created_at, updated_at
	`, deviceID, ownerUserID, input.Name, input.DefaultReasoningModel, input.IsEnabled)

	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return device, nil
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
		          default_reasoning_model, is_enabled, runtime_payload, last_seen_at, notes,
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
