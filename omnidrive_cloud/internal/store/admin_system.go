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
	DefaultChatModel              string
	DefaultImageModel             string
	DefaultVideoModel             string
	StoryboardPrompt              string
	StoryboardModel               string
	StoryboardReferences          []byte
	ImageStoryboardPrompt         string
	ImageStoryboardModel          string
	ImageStoryboardReferences     []byte
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
		&item.DefaultChatModel,
		&item.DefaultImageModel,
		&item.DefaultVideoModel,
		&item.StoryboardPrompt,
		&item.StoryboardModel,
		&item.StoryboardReferences,
		&item.ImageStoryboardPrompt,
		&item.ImageStoryboardModel,
		&item.ImageStoryboardReferences,
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
			default_chat_model,
			default_image_model,
			default_video_model,
			storyboard_prompt_template,
			storyboard_model,
			storyboard_reference_payload,
			image_storyboard_prompt_template,
			image_storyboard_model,
			image_storyboard_reference_payload,
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
	storyboardReferences := input.StoryboardReferences
	if len(storyboardReferences) == 0 {
		storyboardReferences = []byte("[]")
	}
	imageStoryboardReferences := input.ImageStoryboardReferences
	if len(imageStoryboardReferences) == 0 {
		imageStoryboardReferences = []byte("[]")
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO admin_system_configs (
			id,
			ai_worker_enabled,
			payment_channels,
			billing_manual_support_name,
			billing_manual_support_contact,
			billing_manual_support_qr_code_url,
			billing_manual_support_note,
			default_chat_model,
			default_image_model,
			default_video_model,
			storyboard_prompt_template,
			storyboard_model,
			storyboard_reference_payload,
			image_storyboard_prompt_template,
			image_storyboard_model,
			image_storyboard_reference_payload
		)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14, $15, $16::jsonb)
		ON CONFLICT (id) DO UPDATE
		SET
			ai_worker_enabled = EXCLUDED.ai_worker_enabled,
			payment_channels = EXCLUDED.payment_channels,
			billing_manual_support_name = EXCLUDED.billing_manual_support_name,
			billing_manual_support_contact = EXCLUDED.billing_manual_support_contact,
			billing_manual_support_qr_code_url = EXCLUDED.billing_manual_support_qr_code_url,
			billing_manual_support_note = EXCLUDED.billing_manual_support_note,
			default_chat_model = EXCLUDED.default_chat_model,
			default_image_model = EXCLUDED.default_image_model,
			default_video_model = EXCLUDED.default_video_model,
			storyboard_prompt_template = EXCLUDED.storyboard_prompt_template,
			storyboard_model = EXCLUDED.storyboard_model,
			storyboard_reference_payload = EXCLUDED.storyboard_reference_payload,
			image_storyboard_prompt_template = EXCLUDED.image_storyboard_prompt_template,
			image_storyboard_model = EXCLUDED.image_storyboard_model,
			image_storyboard_reference_payload = EXCLUDED.image_storyboard_reference_payload,
			updated_at = NOW()
		RETURNING
			id,
			ai_worker_enabled,
			payment_channels,
			billing_manual_support_name,
			billing_manual_support_contact,
			billing_manual_support_qr_code_url,
			billing_manual_support_note,
			default_chat_model,
			default_image_model,
			default_video_model,
			storyboard_prompt_template,
			storyboard_model,
			storyboard_reference_payload,
			image_storyboard_prompt_template,
			image_storyboard_model,
			image_storyboard_reference_payload,
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
		input.DefaultChatModel,
		input.DefaultImageModel,
		input.DefaultVideoModel,
		input.StoryboardPrompt,
		input.StoryboardModel,
		storyboardReferences,
		input.ImageStoryboardPrompt,
		input.ImageStoryboardModel,
		imageStoryboardReferences,
	)

	return scanAdminSystemSettings(row.Scan)
}
