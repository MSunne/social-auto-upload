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
	AIWorkerEnabled                   bool
	PaymentChannels                   []string
	BillingManualSupportName          string
	BillingManualSupportContact       string
	BillingManualSupportQRCodeURL     string
	BillingManualSupportNote          string
	SMSRegistrationEnabled            bool
	SMSRegistrationProvider           string
	SMSRegistrationEndpoint           string
	SMSRegistrationAccessKeyID        string
	SMSRegistrationAccessKeySecret    string
	SMSRegistrationSignName           string
	SMSRegistrationTemplateCode       string
	SMSRegistrationTemplateParam      string
	SMSRegistrationSchemeName         string
	SMSRegistrationDefaultCountryCode string
	SMSRegistrationValidMinutes       int
	SMSRegistrationCooldownSeconds    int
	SMSRegistrationDailyLimit         int
	SMSRegistrationCodeLength         int
	DefaultChatModel                  string
	DefaultImageModel                 string
	DefaultVideoModel                 string
	VideoCoverPrompt                  string
	StoryboardPrompt                  string
	StoryboardModel                   string
	StoryboardReferences              []byte
	ImageStoryboardPrompt             string
	ImageStoryboardModel              string
	ImageStoryboardReferences         []byte
}

// 处理扫描管理端系统Settings相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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
		&item.SMSRegistration.Enabled,
		&item.SMSRegistration.Provider,
		&item.SMSRegistration.Endpoint,
		&item.SMSRegistration.AccessKeyID,
		&item.SMSRegistration.AccessKeySecret,
		&item.SMSRegistration.SignName,
		&item.SMSRegistration.TemplateCode,
		&item.SMSRegistration.TemplateParam,
		&item.SMSRegistration.SchemeName,
		&item.SMSRegistration.DefaultCountryCode,
		&item.SMSRegistration.ValidMinutes,
		&item.SMSRegistration.CooldownSeconds,
		&item.SMSRegistration.DailyLimit,
		&item.SMSRegistration.CodeLength,
		&item.DefaultChatModel,
		&item.DefaultImageModel,
		&item.DefaultVideoModel,
		&item.VideoCoverPrompt,
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

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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
			sms_registration_enabled,
			sms_registration_provider,
			sms_registration_endpoint,
			sms_registration_access_key_id,
			sms_registration_access_key_secret,
			sms_registration_sign_name,
			sms_registration_template_code,
			sms_registration_template_param,
			sms_registration_scheme_name,
			sms_registration_default_country_code,
			sms_registration_valid_minutes,
			sms_registration_cooldown_seconds,
			sms_registration_daily_limit,
			sms_registration_code_length,
			default_chat_model,
			default_image_model,
			default_video_model,
			video_cover_prompt_template,
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

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
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
			sms_registration_enabled,
			sms_registration_provider,
			sms_registration_endpoint,
			sms_registration_access_key_id,
			sms_registration_access_key_secret,
			sms_registration_sign_name,
			sms_registration_template_code,
			sms_registration_template_param,
			sms_registration_scheme_name,
			sms_registration_default_country_code,
			sms_registration_valid_minutes,
			sms_registration_cooldown_seconds,
			sms_registration_daily_limit,
			sms_registration_code_length,
			default_chat_model,
			default_image_model,
			default_video_model,
			video_cover_prompt_template,
			storyboard_prompt_template,
			storyboard_model,
			storyboard_reference_payload,
			image_storyboard_prompt_template,
			image_storyboard_model,
			image_storyboard_reference_payload
		)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28::jsonb, $29, $30, $31::jsonb)
		ON CONFLICT (id) DO UPDATE
		SET
			ai_worker_enabled = EXCLUDED.ai_worker_enabled,
			payment_channels = EXCLUDED.payment_channels,
			billing_manual_support_name = EXCLUDED.billing_manual_support_name,
			billing_manual_support_contact = EXCLUDED.billing_manual_support_contact,
			billing_manual_support_qr_code_url = EXCLUDED.billing_manual_support_qr_code_url,
			billing_manual_support_note = EXCLUDED.billing_manual_support_note,
			sms_registration_enabled = EXCLUDED.sms_registration_enabled,
			sms_registration_provider = EXCLUDED.sms_registration_provider,
			sms_registration_endpoint = EXCLUDED.sms_registration_endpoint,
			sms_registration_access_key_id = EXCLUDED.sms_registration_access_key_id,
			sms_registration_access_key_secret = EXCLUDED.sms_registration_access_key_secret,
			sms_registration_sign_name = EXCLUDED.sms_registration_sign_name,
			sms_registration_template_code = EXCLUDED.sms_registration_template_code,
			sms_registration_template_param = EXCLUDED.sms_registration_template_param,
			sms_registration_scheme_name = EXCLUDED.sms_registration_scheme_name,
			sms_registration_default_country_code = EXCLUDED.sms_registration_default_country_code,
			sms_registration_valid_minutes = EXCLUDED.sms_registration_valid_minutes,
			sms_registration_cooldown_seconds = EXCLUDED.sms_registration_cooldown_seconds,
			sms_registration_daily_limit = EXCLUDED.sms_registration_daily_limit,
			sms_registration_code_length = EXCLUDED.sms_registration_code_length,
			default_chat_model = EXCLUDED.default_chat_model,
			default_image_model = EXCLUDED.default_image_model,
			default_video_model = EXCLUDED.default_video_model,
			video_cover_prompt_template = EXCLUDED.video_cover_prompt_template,
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
			sms_registration_enabled,
			sms_registration_provider,
			sms_registration_endpoint,
			sms_registration_access_key_id,
			sms_registration_access_key_secret,
			sms_registration_sign_name,
			sms_registration_template_code,
			sms_registration_template_param,
			sms_registration_scheme_name,
			sms_registration_default_country_code,
			sms_registration_valid_minutes,
			sms_registration_cooldown_seconds,
			sms_registration_daily_limit,
			sms_registration_code_length,
			default_chat_model,
			default_image_model,
			default_video_model,
			video_cover_prompt_template,
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
		input.SMSRegistrationEnabled,
		input.SMSRegistrationProvider,
		input.SMSRegistrationEndpoint,
		input.SMSRegistrationAccessKeyID,
		input.SMSRegistrationAccessKeySecret,
		input.SMSRegistrationSignName,
		input.SMSRegistrationTemplateCode,
		input.SMSRegistrationTemplateParam,
		input.SMSRegistrationSchemeName,
		input.SMSRegistrationDefaultCountryCode,
		input.SMSRegistrationValidMinutes,
		input.SMSRegistrationCooldownSeconds,
		input.SMSRegistrationDailyLimit,
		input.SMSRegistrationCodeLength,
		input.DefaultChatModel,
		input.DefaultImageModel,
		input.DefaultVideoModel,
		input.VideoCoverPrompt,
		input.StoryboardPrompt,
		input.StoryboardModel,
		storyboardReferences,
		input.ImageStoryboardPrompt,
		input.ImageStoryboardModel,
		imageStoryboardReferences,
	)

	return scanAdminSystemSettings(row.Scan)
}
