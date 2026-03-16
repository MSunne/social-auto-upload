package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

const adminSystemConfigID = "global"

type UpsertAdminSystemSettingsInput struct {
	AIWorkerEnabled               bool
	PaymentChannels               []string
	BillingManualSupportName      string
	BillingManualSupportContact   string
	BillingManualSupportQRCodeURL string
	BillingManualSupportNote      string
}

func scanAdminSystemSettings(scan scanFn) (*domain.AdminSystemSettingsRecord, error) {
	var item domain.AdminSystemSettingsRecord
	var paymentChannelsPayload []byte

	if err := scan(
		&item.ID,
		&item.AIWorkerEnabled,
		&paymentChannelsPayload,
		&item.BillingManualSupport.Name,
		&item.BillingManualSupport.Contact,
		&item.BillingManualSupport.QRCodeURL,
		&item.BillingManualSupport.Note,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if len(paymentChannelsPayload) > 0 {
		if err := json.Unmarshal(paymentChannelsPayload, &item.PaymentChannels); err != nil {
			return nil, err
		}
	}
	if len(item.PaymentChannels) == 0 {
		item.PaymentChannels = []string{"alipay", "wechatpay", "manual_cs"}
	}

	return &item, nil
}

func (s *Store) GetAdminSystemSettings(ctx context.Context) (*domain.AdminSystemSettingsRecord, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			id,
			ai_worker_enabled,
			payment_channels,
			billing_manual_support_name,
			billing_manual_support_contact,
			billing_manual_support_qr_code_url,
			billing_manual_support_note,
			created_at,
			updated_at
		FROM admin_system_configs
		WHERE id = $1
	`, adminSystemConfigID)

	item, err := scanAdminSystemSettings(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) UpsertAdminSystemSettings(ctx context.Context, input UpsertAdminSystemSettingsInput) (*domain.AdminSystemSettingsRecord, error) {
	paymentChannelsPayload, err := json.Marshal(input.PaymentChannels)
	if err != nil {
		return nil, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO admin_system_configs (
			id,
			ai_worker_enabled,
			payment_channels,
			billing_manual_support_name,
			billing_manual_support_contact,
			billing_manual_support_qr_code_url,
			billing_manual_support_note
		)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE
		SET
			ai_worker_enabled = EXCLUDED.ai_worker_enabled,
			payment_channels = EXCLUDED.payment_channels,
			billing_manual_support_name = EXCLUDED.billing_manual_support_name,
			billing_manual_support_contact = EXCLUDED.billing_manual_support_contact,
			billing_manual_support_qr_code_url = EXCLUDED.billing_manual_support_qr_code_url,
			billing_manual_support_note = EXCLUDED.billing_manual_support_note,
			updated_at = NOW()
		RETURNING
			id,
			ai_worker_enabled,
			payment_channels,
			billing_manual_support_name,
			billing_manual_support_contact,
			billing_manual_support_qr_code_url,
			billing_manual_support_note,
			created_at,
			updated_at
	`,
		adminSystemConfigID,
		input.AIWorkerEnabled,
		paymentChannelsPayload,
		input.BillingManualSupportName,
		input.BillingManualSupportContact,
		input.BillingManualSupportQRCodeURL,
		input.BillingManualSupportNote,
	)

	return scanAdminSystemSettings(row.Scan)
}
