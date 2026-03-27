package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

var (
	ErrDeviceNotFound                      = errors.New("device not found")
	ErrDeviceAlreadyClaimed                = errors.New("device already claimed")
	ErrDeviceActivationNotConfigured       = errors.New("device activation code not configured")
	ErrDeviceActivationCodeInvalid         = errors.New("device activation code invalid")
	ErrDeviceActivationDisabled            = errors.New("device activation disabled")
	ErrDeviceActivationCodeRequired        = errors.New("device activation code is required")
	ErrDeviceActivationCodeConflict        = errors.New("device activation code already exists")
	ErrDeviceActivationResetRequiresUnbind = errors.New("device activation reset requires unbind")
)

type UpdateDeviceActivationConfigInput struct {
	OrderNo           *string
	OrderNoTouched    bool
	ActivationCode    *string
	ActivationTouched bool
	Status            *string
	Notes             *string
	NotesTouched      bool
}

func normalizeComparableCode(value string) string {
	trimmed := strings.TrimSpace(strings.ToUpper(value))
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, char := range trimmed {
		if unicode.IsDigit(char) || unicode.IsLetter(char) {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func normalizeDeviceActivationCode(value string) string {
	return normalizeComparableCode(value)
}

func hashStandaloneActivationCode(activationCode string) string {
	sum := sha256.Sum256([]byte(normalizeDeviceActivationCode(activationCode)))
	return hex.EncodeToString(sum[:])
}

func hashLegacyDeviceActivationCode(deviceCode string, activationCode string) string {
	sum := sha256.Sum256([]byte(normalizeComparableCode(deviceCode) + ":" + normalizeDeviceActivationCode(activationCode)))
	return hex.EncodeToString(sum[:])
}

func buildDeviceActivationHint(normalizedCode string) *string {
	if normalizedCode == "" {
		return nil
	}
	if len(normalizedCode) <= 4 {
		return stringPtr(normalizedCode)
	}
	return stringPtr("尾号 " + normalizedCode[len(normalizedCode)-4:])
}

func scanDeviceActivationConfig(scan scanFn) (*domain.DeviceActivationConfig, error) {
	var item domain.DeviceActivationConfig
	var orderNo *string
	var activationCodeHint *string
	var activatedByUserID *string
	var activatedAt *time.Time
	var notes *string
	if err := scan(
		&item.ID,
		&item.DeviceID,
		&orderNo,
		&activationCodeHint,
		&item.Status,
		&activatedByUserID,
		&activatedAt,
		&notes,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.OrderNo = orderNo
	item.ActivationCodeHint = activationCodeHint
	item.ActivatedByUserID = activatedByUserID
	item.ActivatedAt = activatedAt
	item.Notes = notes
	return &item, nil
}

func (s *Store) ClaimDeviceWithActivation(ctx context.Context, activationCode string, ownerUserID string) (*domain.Device, error) {
	normalizedActivationCode := normalizeDeviceActivationCode(activationCode)
	if normalizedActivationCode == "" {
		return nil, ErrDeviceActivationCodeInvalid
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var deviceID string
	var storedDeviceCode string
	var existingOwner *string
	var activationHash *string
	var activationStatus *string

	if err := tx.QueryRow(ctx, `
		SELECT d.id, d.device_code, d.owner_user_id, dac.activation_code_hash, dac.status
		FROM device_activation_configs dac
		INNER JOIN devices d ON d.id = dac.device_id
		WHERE dac.activation_code_hash = $1
		   OR dac.activation_code_hash = ENCODE(
				DIGEST(
					REGEXP_REPLACE(UPPER(d.device_code), '[^A-Z0-9]', '', 'g') || ':' || $2,
					'sha256'
				),
				'hex'
			)
	`, hashStandaloneActivationCode(normalizedActivationCode), normalizedActivationCode).Scan(
		&deviceID,
		&storedDeviceCode,
		&existingOwner,
		&activationHash,
		&activationStatus,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDeviceActivationCodeInvalid
		}
		return nil, err
	}

	if existingOwner != nil && strings.TrimSpace(*existingOwner) != "" && strings.TrimSpace(*existingOwner) != strings.TrimSpace(ownerUserID) {
		return nil, ErrDeviceAlreadyClaimed
	}
	if activationHash == nil || strings.TrimSpace(*activationHash) == "" {
		return nil, ErrDeviceActivationNotConfigured
	}
	if activationStatus != nil && strings.EqualFold(strings.TrimSpace(*activationStatus), "disabled") {
		return nil, ErrDeviceActivationDisabled
	}
	standaloneHash := hashStandaloneActivationCode(normalizedActivationCode)
	legacyHash := hashLegacyDeviceActivationCode(storedDeviceCode, normalizedActivationCode)
	if !strings.EqualFold(strings.TrimSpace(*activationHash), standaloneHash) && !strings.EqualFold(strings.TrimSpace(*activationHash), legacyHash) {
		return nil, ErrDeviceActivationCodeInvalid
	}

	row := tx.QueryRow(ctx, `
		UPDATE devices
		SET owner_user_id = $2,
		    is_enabled = TRUE,
		    updated_at = NOW()
		WHERE id = $1
		  AND (owner_user_id IS NULL OR owner_user_id = $2)
		RETURNING id, owner_user_id, device_code, agent_key, name, local_ip, public_ip,
		          default_reasoning_model, default_chat_model, default_image_model, default_video_model,
		          is_enabled, runtime_payload, last_seen_at, notes,
		          created_at, updated_at
	`, deviceID, ownerUserID)

	device, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDeviceAlreadyClaimed
		}
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE device_activation_configs
		SET status = 'activated',
		    activated_by_user_id = $2,
		    activated_at = NOW(),
		    updated_at = NOW()
		WHERE device_id = $1
	`, device.ID, ownerUserID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetOwnedDevice(ctx, device.ID, ownerUserID)
}

func (s *Store) UpdateAdminDeviceActivationConfig(ctx context.Context, deviceID string, input UpdateDeviceActivationConfigInput) (*domain.AdminDeviceRow, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var existingOwner *string
	var existingConfigID *string
	var existingActivationHash *string
	var existingActivationHint *string
	if err := tx.QueryRow(ctx, `
		SELECT d.owner_user_id, dac.id, dac.activation_code_hash, dac.activation_code_hint
		FROM devices d
		LEFT JOIN device_activation_configs dac ON dac.device_id = d.id
		WHERE d.id = $1
	`, deviceID).Scan(&existingOwner, &existingConfigID, &existingActivationHash, &existingActivationHint); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	orderNoValue := ""
	if input.OrderNo != nil {
		orderNoValue = strings.TrimSpace(*input.OrderNo)
	}
	notesValue := ""
	if input.Notes != nil {
		notesValue = strings.TrimSpace(*input.Notes)
	}
	var statusValue any
	if input.Status != nil {
		statusValue = strings.TrimSpace(*input.Status)
	}

	var activationHash any
	var activationHint any
	if input.ActivationTouched {
		if input.ActivationCode == nil || normalizeDeviceActivationCode(*input.ActivationCode) == "" {
			return nil, ErrDeviceActivationCodeRequired
		}
		if existingOwner != nil && strings.TrimSpace(*existingOwner) != "" {
			return nil, ErrDeviceActivationResetRequiresUnbind
		}
		normalizedCode := normalizeDeviceActivationCode(*input.ActivationCode)
		standaloneHash := hashStandaloneActivationCode(normalizedCode)
		var duplicateDeviceID string
		if err := tx.QueryRow(ctx, `
			SELECT device_id
			FROM device_activation_configs
			WHERE activation_code_hash = $1
			  AND device_id <> $2
			LIMIT 1
		`, standaloneHash, deviceID).Scan(&duplicateDeviceID); err == nil {
			return nil, ErrDeviceActivationCodeConflict
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		activationHash = standaloneHash
		if hint := buildDeviceActivationHint(normalizedCode); hint != nil {
			activationHint = *hint
		}
	}
	if statusValue != nil {
		if strings.EqualFold(statusValue.(string), "ready") && existingOwner != nil && strings.TrimSpace(*existingOwner) != "" {
			return nil, ErrDeviceActivationResetRequiresUnbind
		}
	}
	if existingConfigID == nil && !input.ActivationTouched {
		return nil, ErrDeviceActivationCodeRequired
	}
	if !input.ActivationTouched {
		if existingActivationHash != nil {
			activationHash = *existingActivationHash
		}
		if existingActivationHint != nil {
			activationHint = *existingActivationHint
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO device_activation_configs (
			id, device_id, order_no, activation_code_hash, activation_code_hint, status, notes
		)
		VALUES (
			$1, $2, NULLIF($3, ''), $4, $5, COALESCE(NULLIF($6, ''), 'ready'), NULLIF($7, '')
		)
		ON CONFLICT (device_id) DO UPDATE
		SET
			order_no = CASE
				WHEN $8 THEN NULLIF($3, '')
				ELSE device_activation_configs.order_no
			END,
			activation_code_hash = CASE
				WHEN $9 THEN $4
				ELSE device_activation_configs.activation_code_hash
			END,
			activation_code_hint = CASE
				WHEN $9 THEN $5
				ELSE device_activation_configs.activation_code_hint
			END,
			status = COALESCE(NULLIF($6, ''), device_activation_configs.status),
			activated_by_user_id = CASE
				WHEN COALESCE(NULLIF($6, ''), '') = 'ready' THEN NULL
				WHEN $9 THEN NULL
				ELSE device_activation_configs.activated_by_user_id
			END,
			activated_at = CASE
				WHEN COALESCE(NULLIF($6, ''), '') = 'ready' THEN NULL
				WHEN $9 THEN NULL
				ELSE device_activation_configs.activated_at
			END,
			notes = CASE
				WHEN $10 THEN NULLIF($7, '')
				ELSE device_activation_configs.notes
			END,
			updated_at = NOW()
	`, uuid.NewString(), deviceID, orderNoValue, activationHash, activationHint, statusValue, notesValue, input.OrderNoTouched, input.ActivationTouched, input.NotesTouched); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAdminDeviceByID(ctx, deviceID)
}
