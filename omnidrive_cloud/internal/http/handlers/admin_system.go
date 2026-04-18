package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"omnidrive_cloud/internal/ai"
	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type adminSystemConfigPatchRequest struct {
	AIWorkerEnabled                  *bool                             `json:"aiWorkerEnabled"`
	PaymentChannels                  []string                          `json:"paymentChannels"`
	BillingManualSupport             *adminManualSupportPatchRequest   `json:"billingManualSupport"`
	SMSRegistration                  *adminSMSRegistrationPatchRequest `json:"smsRegistration"`
	DigitalHumanCreditsPerSecond     *float64                          `json:"digitalHumanCreditsPerSecond"`
	MixVideoCreditsPerSecond         *float64                          `json:"mixVideoCreditsPerSecond"`
	DigitalHumanShoppingDefaultModel *string                           `json:"digitalHumanShoppingDefaultModel"`
	DigitalHumanSpeechDefaultModel   *string                           `json:"digitalHumanSpeechDefaultModel"`
	DefaultChatModel                 *string                           `json:"defaultChatModel"`
	PromptOptimizeModel              *string                           `json:"promptOptimizeModel"`
	DefaultImageModel                *string                           `json:"defaultImageModel"`
	DefaultVideoModel                *string                           `json:"defaultVideoModel"`
	VideoCoverPrompt                 *string                           `json:"videoCoverPrompt"`
	StoryboardPrompt                 *string                           `json:"storyboardPrompt"`
	StoryboardModel                  *string                           `json:"storyboardModel"`
	StoryboardReferences             []map[string]any                  `json:"storyboardReferences"`
	ImageStoryboardPrompt            *string                           `json:"imageStoryboardPrompt"`
	ImageStoryboardModel             *string                           `json:"imageStoryboardModel"`
	ImageStoryboardReferences        []map[string]any                  `json:"imageStoryboardReferences"`
}

type adminManualSupportPatchRequest struct {
	Name      *string `json:"name"`
	Contact   *string `json:"contact"`
	QRCodeURL *string `json:"qrCodeUrl"`
	Note      *string `json:"note"`
}

type adminSMSRegistrationPatchRequest struct {
	Enabled            *bool   `json:"enabled"`
	Provider           *string `json:"provider"`
	Endpoint           *string `json:"endpoint"`
	AccessKeyID        *string `json:"accessKeyId"`
	AccessKeySecret    *string `json:"accessKeySecret"`
	SignName           *string `json:"signName"`
	TemplateCode       *string `json:"templateCode"`
	TemplateParam      *string `json:"templateParam"`
	SchemeName         *string `json:"schemeName"`
	DefaultCountryCode *string `json:"defaultCountryCode"`
	ValidMinutes       *int    `json:"validMinutes"`
	CooldownSeconds    *int    `json:"cooldownSeconds"`
	DailyLimit         *int    `json:"dailyLimit"`
	CodeLength         *int    `json:"codeLength"`
}

type effectiveAdminSystemSettings struct {
	AIWorkerEnabled                    bool
	PaymentChannels                    []string
	BillingManualSupport               domain.AdminManualSupportConfig
	SMSRegistration                    domain.AdminSMSRegistrationConfig
	DigitalHumanCreditsPerSecondMillis int64
	MixVideoCreditsPerSecondMillis     int64
	DigitalHumanShoppingDefaultModel   string
	DigitalHumanSpeechDefaultModel     string
	DefaultChatModel                   string
	PromptOptimizeModel                string
	DefaultImageModel                  string
	DefaultVideoModel                  string
	VideoCoverPrompt                   string
	StoryboardPrompt                   string
	StoryboardModel                    string
	StoryboardReferences               json.RawMessage
	ImageStoryboardPrompt              string
	ImageStoryboardModel               string
	ImageStoryboardReferences          json.RawMessage
	UpdatedAt                          *time.Time
}

// 处理默认管理端系统Settings相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func defaultAdminSystemSettings(cfg config.Config) effectiveAdminSystemSettings {
	return effectiveAdminSystemSettings{
		AIWorkerEnabled: cfg.AIWorkerEnabled,
		PaymentChannels: []string{"alipay", "wechatpay", "manual_cs"},
		BillingManualSupport: domain.AdminManualSupportConfig{
			Name:      strings.TrimSpace(cfg.BillingManualSupportName),
			Contact:   strings.TrimSpace(cfg.BillingManualSupportContact),
			QRCodeURL: strings.TrimSpace(cfg.BillingManualSupportQRCodeURL),
			Note:      strings.TrimSpace(cfg.BillingManualSupportNote),
		},
		SMSRegistration: domain.AdminSMSRegistrationConfig{
			Enabled:            false,
			Provider:           "aliyun_dypnsapi",
			Endpoint:           "dypnsapi.aliyuncs.com",
			TemplateParam:      `{"code":"##code##"}`,
			DefaultCountryCode: "86",
			ValidMinutes:       10,
			CooldownSeconds:    60,
			DailyLimit:         10,
			CodeLength:         6,
		},
		DigitalHumanCreditsPerSecondMillis: 0,
		MixVideoCreditsPerSecondMillis:     300,
		DigitalHumanShoppingDefaultModel:   "",
		DigitalHumanSpeechDefaultModel:     "",
		DefaultChatModel:                   strings.TrimSpace(cfg.DefaultChatModel),
		PromptOptimizeModel:                "gemini-3.1-pro-preview",
		DefaultImageModel:                  strings.TrimSpace(cfg.DefaultImageModel),
		DefaultVideoModel:                  strings.TrimSpace(cfg.DefaultVideoModel),
		VideoCoverPrompt:                   strings.TrimSpace(ai.DefaultSkillVideoCoverPromptTemplate),
		StoryboardPrompt:                   strings.TrimSpace(ai.DefaultVideoStoryboardSystemPrompt),
		StoryboardModel:                    strings.TrimSpace(ai.DefaultVideoStoryboardModelName),
		StoryboardReferences:               []byte("[]"),
		ImageStoryboardPrompt:              "",
		ImageStoryboardModel:               strings.TrimSpace(cfg.DefaultChatModel),
		ImageStoryboardReferences:          []byte("[]"),
	}
}

// 处理分镜模型Touched相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func storyboardModelTouched(raw map[string]json.RawMessage) bool {
	return nestedFieldTouched(raw, "storyboardModel")
}

// 处理校验分镜包模型相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func validateStoryboardPackageModel(ctx context.Context, app *appstate.App, modelName string) error {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" || app == nil || app.Store == nil {
		return nil
	}
	model, err := app.Store.GetAIModelByName(ctx, modelName)
	if err != nil {
		return err
	}
	if model == nil || !ai.SupportsStoryboardPackageModel(model) {
		return renderableError("storyboardModel must reference an enabled chat model that supports image inputs")
	}
	return nil
}

// 处理校验提示词优化模型相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func validatePromptOptimizeModel(ctx context.Context, app *appstate.App, modelName string) error {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" || app == nil || app.Store == nil {
		return nil
	}
	model, err := app.Store.GetAIModelByName(ctx, modelName)
	if err != nil {
		return err
	}
	if model == nil || !ai.SupportsStoryboardPackageModel(model) {
		return renderableError("promptOptimizeModel must reference an enabled chat model that supports image inputs")
	}
	return nil
}

// 规范化管理端支付Channels，统一管理端系统链路的输入格式和后续处理行为。
func normalizeAdminPaymentChannels(channels []string) ([]string, error) {
	if len(channels) == 0 {
		return nil, errors.New("paymentChannels must contain at least one channel")
	}

	items := make([]string, 0, len(channels))
	seen := make(map[string]struct{}, len(channels))
	for _, raw := range channels {
		value := strings.TrimSpace(strings.ToLower(raw))
		switch value {
		case "manual", "manual_cs", "customer_service", "customer-service":
			value = "manual_cs"
		case "wechat", "wechatpay", "wechat_pay":
			value = "wechatpay"
		case "alipay":
		default:
			return nil, errors.New("unsupported payment channel: " + raw)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	if len(items) == 0 {
		return nil, errors.New("paymentChannels must contain at least one channel")
	}
	return items, nil
}

// 处理支付Channel启用相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s effectiveAdminSystemSettings) paymentChannelEnabled(channel string) bool {
	for _, item := range s.PaymentChannels {
		if item == channel {
			return true
		}
	}
	return false
}

// 处理filter启用支付Channels相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func filterEnabledPaymentChannels(packageChannels []string, enabledChannels []string) []string {
	if len(packageChannels) == 0 || len(enabledChannels) == 0 {
		return []string{}
	}

	enabled := make(map[string]struct{}, len(enabledChannels))
	for _, item := range enabledChannels {
		enabled[item] = struct{}{}
	}

	filtered := make([]string, 0, len(packageChannels))
	seen := make(map[string]struct{}, len(packageChannels))
	for _, raw := range packageChannels {
		channel := normalizeBillingChannel(raw)
		if _, exists := enabled[channel]; !exists {
			continue
		}
		if _, exists := seen[channel]; exists {
			continue
		}
		seen[channel] = struct{}{}
		filtered = append(filtered, channel)
	}
	return filtered
}

// 加载生效管理端系统Settings，供管理端系统继续处理当前业务状态。
func loadEffectiveAdminSystemSettings(ctx context.Context, app *appstate.App) (effectiveAdminSystemSettings, error) {
	settings := defaultAdminSystemSettings(app.Config)
	record, err := app.Store.GetAdminSystemSettings(ctx)
	if err != nil {
		return settings, err
	}
	if record == nil {
		return settings, nil
	}

	settings.AIWorkerEnabled = record.AIWorkerEnabled
	settings.PaymentChannels = append([]string(nil), record.PaymentChannels...)
	settings.BillingManualSupport = record.BillingManualSupport
	settings.SMSRegistration = record.SMSRegistration
	settings.DigitalHumanCreditsPerSecondMillis = record.DigitalHumanCreditsPerSecondMillis
	if settings.DigitalHumanCreditsPerSecondMillis <= 0 && record.DigitalHumanCreditsPerSecond > 0 {
		settings.DigitalHumanCreditsPerSecondMillis = record.DigitalHumanCreditsPerSecond * store.DigitalHumanCreditMillisScale
	}
	settings.MixVideoCreditsPerSecondMillis = record.MixVideoCreditsPerSecondMillis
	if settings.MixVideoCreditsPerSecondMillis <= 0 && record.MixVideoCreditsPerSecond > 0 {
		settings.MixVideoCreditsPerSecondMillis = record.MixVideoCreditsPerSecond * store.CreditMillisScale
	}
	if value := strings.TrimSpace(record.DigitalHumanShoppingDefaultModel); value != "" {
		settings.DigitalHumanShoppingDefaultModel = value
	}
	if value := strings.TrimSpace(record.DigitalHumanSpeechDefaultModel); value != "" {
		settings.DigitalHumanSpeechDefaultModel = value
	}
	settings.DefaultChatModel = strings.TrimSpace(record.DefaultChatModel)
	settings.PromptOptimizeModel = strings.TrimSpace(record.PromptOptimizeModel)
	settings.DefaultImageModel = strings.TrimSpace(record.DefaultImageModel)
	settings.DefaultVideoModel = strings.TrimSpace(record.DefaultVideoModel)
	if value := strings.TrimSpace(record.VideoCoverPrompt); value != "" {
		settings.VideoCoverPrompt = value
	}
	settings.StoryboardPrompt = strings.TrimSpace(record.StoryboardPrompt)
	settings.StoryboardModel = strings.TrimSpace(record.StoryboardModel)
	settings.StoryboardReferences = append([]byte(nil), record.StoryboardReferences...)
	settings.ImageStoryboardPrompt = strings.TrimSpace(record.ImageStoryboardPrompt)
	settings.ImageStoryboardModel = strings.TrimSpace(record.ImageStoryboardModel)
	settings.ImageStoryboardReferences = append([]byte(nil), record.ImageStoryboardReferences...)
	settings.UpdatedAt = adminTimePtr(record.UpdatedAt)
	return settings, nil
}

// 构建管理端系统配置载荷，为管理端系统生成后续步骤所需的派生参数或载荷。
func buildAdminSystemConfigPayload(app *appstate.App, settings effectiveAdminSystemSettings) domain.AdminSystemConfig {
	notes := []string{
		"管理端已切换为数据库管理员 + 角色权限模型。",
		"服务首次启动时会用环境变量注入首个超级管理员，后续以数据库管理员表为准。",
		"当前客服充值、分销佣金、结算批次与提现审核主链都已接入数据库模型。",
	}
	if settings.UpdatedAt != nil {
		notes = append(notes, "运营后台更新的系统配置会持久化到数据库，并覆盖默认展示配置。")
	}
	if !settings.AIWorkerEnabled {
		notes = append(notes, "AI Worker 当前已关闭，新的 AI 任务创建会被阻止，直到后台重新启用。")
	}
	if settings.SMSRegistration.Enabled {
		notes = append(notes, "短信注册已启用，注册页将要求先完成手机验证码校验。")
	} else {
		notes = append(notes, "短信注册当前关闭，请先在系统配置中启用并填写阿里云短信参数。")
	}

	return domain.AdminSystemConfig{
		AuthMode:                         "database_rbac",
		AdminEmail:                       app.Config.AdminEmail,
		S3Configured:                     app.Config.S3Bucket != "" && app.Config.S3Endpoint != "" && app.Config.S3AccessKey != "" && app.Config.S3SecretKey != "",
		S3Endpoint:                       app.Config.S3Endpoint,
		S3Bucket:                         app.Config.S3Bucket,
		AIWorkerEnabled:                  settings.AIWorkerEnabled,
		PaymentChannels:                  append([]string(nil), settings.PaymentChannels...),
		BillingManualSupport:             settings.BillingManualSupport,
		SMSRegistration:                  settings.SMSRegistration,
		DigitalHumanCreditsPerSecond:     store.DigitalHumanCreditsFromMillis(settings.DigitalHumanCreditsPerSecondMillis),
		MixVideoCreditsPerSecond:         store.CreditsFromMillis(settings.MixVideoCreditsPerSecondMillis),
		DigitalHumanShoppingDefaultModel: settings.DigitalHumanShoppingDefaultModel,
		DigitalHumanSpeechDefaultModel:   settings.DigitalHumanSpeechDefaultModel,
		DefaultChatModel:                 settings.DefaultChatModel,
		PromptOptimizeModel:              settings.PromptOptimizeModel,
		DefaultImageModel:                settings.DefaultImageModel,
		DefaultVideoModel:                settings.DefaultVideoModel,
		VideoCoverPrompt:                 settings.VideoCoverPrompt,
		StoryboardPrompt:                 settings.StoryboardPrompt,
		StoryboardModel:                  settings.StoryboardModel,
		StoryboardReferences:             settings.StoryboardReferences,
		ImageStoryboardPrompt:            settings.ImageStoryboardPrompt,
		ImageStoryboardModel:             settings.ImageStoryboardModel,
		ImageStoryboardReferences:        settings.ImageStoryboardReferences,
		Notes:                            notes,
		UpdatedAt:                        settings.UpdatedAt,
	}
}

// 处理解码管理端系统配置Patch请求相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func decodeAdminSystemConfigPatchRequest(r *http.Request, destination any) (map[string]json.RawMessage, error) {
	if r.Body == nil {
		return nil, errors.New("empty request body")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, errors.New("empty request body")
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return nil, err
	}

	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// 处理nestedFieldTouched相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func nestedFieldTouched(raw map[string]json.RawMessage, key string) bool {
	_, exists := raw[key]
	return exists
}

// 规范化PatchedString，统一管理端系统链路的输入格式和后续处理行为。
func normalizePatchedString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// 规范化PatchedInt，统一管理端系统链路的输入格式和后续处理行为。
func normalizePatchedInt(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

// 规范化短信供应方，统一管理端系统链路的输入格式和后续处理行为。
func normalizeSMSProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "aliyun", "aliyun_dypnsapi", "aliyun-dypnsapi":
		return "aliyun_dypnsapi"
	case "aliyun_dysmsapi", "aliyun-dysmsapi", "aliyun_sms", "aliyun-sms":
		return "aliyun_dysmsapi"
	default:
		return ""
	}
}

// 规范化短信Country编码，统一管理端系统链路的输入格式和后续处理行为。
func normalizeSMSCountryCode(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var digits strings.Builder
	digits.Grow(len(trimmed))
	for _, char := range trimmed {
		if char >= '0' && char <= '9' {
			digits.WriteRune(char)
		}
	}
	return digits.String()
}

// 根据审计计算maskSecret，供管理端系统链路复用关键派生结果。
func maskSecretForAudit(value string) string {
	trimmed := strings.TrimSpace(value)
	switch {
	case trimmed == "":
		return ""
	case len(trimmed) <= 4:
		return "****"
	default:
		return trimmed[:2] + "****" + trimmed[len(trimmed)-2:]
	}
}

// 处理管理端时间Ptr相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func adminTimePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

// 清理Removed分镜参考，释放当前链路不再需要的临时资源或旧数据。
func cleanupRemovedStoryboardReferences(ctx context.Context, app *appstate.App, previous json.RawMessage, next json.RawMessage) {
	if app == nil || app.Storage == nil {
		return
	}

	previousKeys := extractStoryboardStorageKeys(previous)
	if len(previousKeys) == 0 {
		return
	}
	nextKeys := extractStoryboardStorageKeys(next)
	for key := range previousKeys {
		if _, exists := nextKeys[key]; exists {
			continue
		}
		if !hasStoryboardManagedPrefix(key) {
			continue
		}
		_ = app.Storage.DeleteObject(ctx, key)
	}
}

// 合并分镜参考Payloads，统一多来源数据后返回稳定结果。
func mergeStoryboardReferencePayloads(payloads ...json.RawMessage) json.RawMessage {
	items := make([]map[string]any, 0)
	for _, payload := range payloads {
		if len(payload) == 0 {
			continue
		}
		var decoded []map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			continue
		}
		items = append(items, decoded...)
	}
	if len(items) == 0 {
		return json.RawMessage("[]")
	}
	merged, err := json.Marshal(items)
	if err != nil {
		return json.RawMessage("[]")
	}
	return merged
}

// 提取分镜存储键s，供管理端系统后续关联和分支判断复用。
func extractStoryboardStorageKeys(payload json.RawMessage) map[string]struct{} {
	results := make(map[string]struct{})
	if len(payload) == 0 {
		return results
	}

	var items []map[string]any
	if err := json.Unmarshal(payload, &items); err != nil {
		return results
	}
	for _, item := range items {
		key := strings.TrimSpace(stringValueFromAny(item["storageKey"]))
		if key == "" {
			continue
		}
		results[key] = struct{}{}
	}
	return results
}

// 判断是否存在分镜ManagedPrefix，供当前链路选择后续处理策略。
func hasStoryboardManagedPrefix(storageKey string) bool {
	return strings.HasPrefix(strings.TrimSpace(storageKey), "system-config/storyboard/")
}

// 处理string值Any相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func stringValueFromAny(value any) string {
	text, _ := value.(string)
	return text
}

// 处理管理端认证构建管理端系统配置接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminAuthHandler) buildAdminSystemConfig(ctx context.Context) (domain.AdminSystemConfig, error) {
	settings, err := loadEffectiveAdminSystemSettings(ctx, h.app)
	if err != nil {
		return domain.AdminSystemConfig{}, err
	}
	return buildAdminSystemConfigPayload(h.app, settings), nil
}

// 处理管理端认证更新系统配置接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminAuthHandler) UpdateSystemConfig(w http.ResponseWriter, r *http.Request) {
	admin := httpcontext.CurrentAdmin(r.Context())
	if admin == nil {
		render.Error(w, http.StatusUnauthorized, "Admin not found")
		return
	}

	var payload adminSystemConfigPatchRequest
	raw, err := decodeAdminSystemConfigPatchRequest(r, &payload)
	if err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load system config")
		return
	}
	previousStoryboardReferences := append(json.RawMessage(nil), settings.StoryboardReferences...)
	previousImageStoryboardReferences := append(json.RawMessage(nil), settings.ImageStoryboardReferences...)

	if nestedFieldTouched(raw, "aiWorkerEnabled") {
		if payload.AIWorkerEnabled == nil {
			render.Error(w, http.StatusBadRequest, "aiWorkerEnabled must be a boolean")
			return
		}
		settings.AIWorkerEnabled = *payload.AIWorkerEnabled
	}

	if nestedFieldTouched(raw, "paymentChannels") {
		if payload.PaymentChannels == nil {
			render.Error(w, http.StatusBadRequest, "paymentChannels must be an array")
			return
		}
		normalizedChannels, normalizeErr := normalizeAdminPaymentChannels(payload.PaymentChannels)
		if normalizeErr != nil {
			render.Error(w, http.StatusBadRequest, normalizeErr.Error())
			return
		}
		settings.PaymentChannels = normalizedChannels
	}

	if nestedFieldTouched(raw, "billingManualSupport") {
		if payload.BillingManualSupport == nil {
			render.Error(w, http.StatusBadRequest, "billingManualSupport must be an object")
			return
		}

		supportRaw := map[string]json.RawMessage{}
		if err := json.Unmarshal(raw["billingManualSupport"], &supportRaw); err != nil {
			render.Error(w, http.StatusBadRequest, "billingManualSupport must be an object")
			return
		}

		if nestedFieldTouched(supportRaw, "name") {
			settings.BillingManualSupport.Name = normalizePatchedString(payload.BillingManualSupport.Name)
		}
		if nestedFieldTouched(supportRaw, "contact") {
			settings.BillingManualSupport.Contact = normalizePatchedString(payload.BillingManualSupport.Contact)
		}
		if nestedFieldTouched(supportRaw, "qrCodeUrl") {
			settings.BillingManualSupport.QRCodeURL = normalizePatchedString(payload.BillingManualSupport.QRCodeURL)
		}
		if nestedFieldTouched(supportRaw, "note") {
			settings.BillingManualSupport.Note = normalizePatchedString(payload.BillingManualSupport.Note)
		}
	}

	if nestedFieldTouched(raw, "smsRegistration") {
		if payload.SMSRegistration == nil {
			render.Error(w, http.StatusBadRequest, "smsRegistration must be an object")
			return
		}

		smsRaw := map[string]json.RawMessage{}
		if err := json.Unmarshal(raw["smsRegistration"], &smsRaw); err != nil {
			render.Error(w, http.StatusBadRequest, "smsRegistration must be an object")
			return
		}

		if nestedFieldTouched(smsRaw, "enabled") {
			if payload.SMSRegistration.Enabled == nil {
				render.Error(w, http.StatusBadRequest, "smsRegistration.enabled must be a boolean")
				return
			}
			settings.SMSRegistration.Enabled = *payload.SMSRegistration.Enabled
		}
		if nestedFieldTouched(smsRaw, "provider") {
			provider := normalizeSMSProvider(normalizePatchedString(payload.SMSRegistration.Provider))
			if provider == "" {
				render.Error(w, http.StatusBadRequest, "smsRegistration.provider only supports aliyun_dypnsapi or aliyun_dysmsapi")
				return
			}
			settings.SMSRegistration.Provider = provider
		}
		if nestedFieldTouched(smsRaw, "endpoint") {
			settings.SMSRegistration.Endpoint = normalizePatchedString(payload.SMSRegistration.Endpoint)
		}
		if nestedFieldTouched(smsRaw, "accessKeyId") {
			settings.SMSRegistration.AccessKeyID = normalizePatchedString(payload.SMSRegistration.AccessKeyID)
		}
		if nestedFieldTouched(smsRaw, "accessKeySecret") {
			settings.SMSRegistration.AccessKeySecret = normalizePatchedString(payload.SMSRegistration.AccessKeySecret)
		}
		if nestedFieldTouched(smsRaw, "signName") {
			settings.SMSRegistration.SignName = normalizePatchedString(payload.SMSRegistration.SignName)
		}
		if nestedFieldTouched(smsRaw, "templateCode") {
			settings.SMSRegistration.TemplateCode = normalizePatchedString(payload.SMSRegistration.TemplateCode)
		}
		if nestedFieldTouched(smsRaw, "templateParam") {
			settings.SMSRegistration.TemplateParam = normalizePatchedString(payload.SMSRegistration.TemplateParam)
		}
		if nestedFieldTouched(smsRaw, "schemeName") {
			settings.SMSRegistration.SchemeName = normalizePatchedString(payload.SMSRegistration.SchemeName)
		}
		if nestedFieldTouched(smsRaw, "defaultCountryCode") {
			settings.SMSRegistration.DefaultCountryCode = normalizeSMSCountryCode(normalizePatchedString(payload.SMSRegistration.DefaultCountryCode))
		}
		if nestedFieldTouched(smsRaw, "validMinutes") {
			settings.SMSRegistration.ValidMinutes = normalizePatchedInt(payload.SMSRegistration.ValidMinutes, settings.SMSRegistration.ValidMinutes)
		}
		if nestedFieldTouched(smsRaw, "cooldownSeconds") {
			settings.SMSRegistration.CooldownSeconds = normalizePatchedInt(payload.SMSRegistration.CooldownSeconds, settings.SMSRegistration.CooldownSeconds)
		}
		if nestedFieldTouched(smsRaw, "dailyLimit") {
			settings.SMSRegistration.DailyLimit = normalizePatchedInt(payload.SMSRegistration.DailyLimit, settings.SMSRegistration.DailyLimit)
		}
		if nestedFieldTouched(smsRaw, "codeLength") {
			settings.SMSRegistration.CodeLength = normalizePatchedInt(payload.SMSRegistration.CodeLength, settings.SMSRegistration.CodeLength)
		}
	}
	if nestedFieldTouched(raw, "digitalHumanCreditsPerSecond") {
		if payload.DigitalHumanCreditsPerSecond == nil {
			render.Error(w, http.StatusBadRequest, "digitalHumanCreditsPerSecond must be a number")
			return
		}
		millis, err := store.DigitalHumanCreditsToMillis(*payload.DigitalHumanCreditsPerSecond)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "digitalHumanCreditsPerSecond "+err.Error())
			return
		}
		settings.DigitalHumanCreditsPerSecondMillis = millis
	}
	if nestedFieldTouched(raw, "mixVideoCreditsPerSecond") {
		if payload.MixVideoCreditsPerSecond == nil {
			render.Error(w, http.StatusBadRequest, "mixVideoCreditsPerSecond must be a number")
			return
		}
		millis, err := store.CreditsToMillis(*payload.MixVideoCreditsPerSecond)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "mixVideoCreditsPerSecond "+err.Error())
			return
		}
		settings.MixVideoCreditsPerSecondMillis = millis
	}
	if nestedFieldTouched(raw, "digitalHumanShoppingDefaultModel") {
		settings.DigitalHumanShoppingDefaultModel = normalizePatchedString(payload.DigitalHumanShoppingDefaultModel)
	}
	if nestedFieldTouched(raw, "digitalHumanSpeechDefaultModel") {
		settings.DigitalHumanSpeechDefaultModel = normalizePatchedString(payload.DigitalHumanSpeechDefaultModel)
	}

	if nestedFieldTouched(raw, "defaultChatModel") {
		settings.DefaultChatModel = normalizePatchedString(payload.DefaultChatModel)
	}
	if nestedFieldTouched(raw, "promptOptimizeModel") {
		settings.PromptOptimizeModel = normalizePatchedString(payload.PromptOptimizeModel)
	}
	if nestedFieldTouched(raw, "defaultImageModel") {
		settings.DefaultImageModel = normalizePatchedString(payload.DefaultImageModel)
	}
	if nestedFieldTouched(raw, "defaultVideoModel") {
		settings.DefaultVideoModel = normalizePatchedString(payload.DefaultVideoModel)
	}
	if nestedFieldTouched(raw, "videoCoverPrompt") {
		settings.VideoCoverPrompt = normalizePatchedString(payload.VideoCoverPrompt)
		if settings.VideoCoverPrompt == "" {
			settings.VideoCoverPrompt = strings.TrimSpace(ai.DefaultSkillVideoCoverPromptTemplate)
		}
	}
	if nestedFieldTouched(raw, "storyboardPrompt") {
		settings.StoryboardPrompt = normalizePatchedString(payload.StoryboardPrompt)
	}
	if nestedFieldTouched(raw, "storyboardModel") {
		settings.StoryboardModel = normalizePatchedString(payload.StoryboardModel)
	}
	if nestedFieldTouched(raw, "storyboardReferences") {
		if payload.StoryboardReferences == nil {
			render.Error(w, http.StatusBadRequest, "storyboardReferences must be an array")
			return
		}
		referencePayload, marshalErr := json.Marshal(payload.StoryboardReferences)
		if marshalErr != nil {
			render.Error(w, http.StatusBadRequest, "storyboardReferences must be valid json")
			return
		}
		settings.StoryboardReferences = referencePayload
	}
	if nestedFieldTouched(raw, "imageStoryboardPrompt") {
		settings.ImageStoryboardPrompt = normalizePatchedString(payload.ImageStoryboardPrompt)
	}
	if nestedFieldTouched(raw, "imageStoryboardModel") {
		settings.ImageStoryboardModel = normalizePatchedString(payload.ImageStoryboardModel)
	}
	if nestedFieldTouched(raw, "imageStoryboardReferences") {
		if payload.ImageStoryboardReferences == nil {
			render.Error(w, http.StatusBadRequest, "imageStoryboardReferences must be an array")
			return
		}
		referencePayload, marshalErr := json.Marshal(payload.ImageStoryboardReferences)
		if marshalErr != nil {
			render.Error(w, http.StatusBadRequest, "imageStoryboardReferences must be valid json")
			return
		}
		settings.ImageStoryboardReferences = referencePayload
	}

	if strings.TrimSpace(settings.BillingManualSupport.Name) == "" {
		render.Error(w, http.StatusBadRequest, "billingManualSupport.name is required")
		return
	}
	if len(settings.PaymentChannels) == 0 {
		render.Error(w, http.StatusBadRequest, "paymentChannels must contain at least one channel")
		return
	}
	if nestedFieldTouched(raw, "digitalHumanShoppingDefaultModel") && strings.TrimSpace(settings.DigitalHumanShoppingDefaultModel) != "" {
		if err := validateDigitalHumanModelName(r.Context(), h.app, settings.DigitalHumanShoppingDefaultModel); err != nil {
			render.Error(w, http.StatusBadRequest, "digitalHumanShoppingDefaultModel "+err.Error())
			return
		}
	}
	if nestedFieldTouched(raw, "digitalHumanSpeechDefaultModel") && strings.TrimSpace(settings.DigitalHumanSpeechDefaultModel) != "" {
		if err := validateDigitalHumanModelName(r.Context(), h.app, settings.DigitalHumanSpeechDefaultModel); err != nil {
			render.Error(w, http.StatusBadRequest, "digitalHumanSpeechDefaultModel "+err.Error())
			return
		}
	}
	if strings.TrimSpace(settings.DefaultChatModel) == "" {
		render.Error(w, http.StatusBadRequest, "defaultChatModel is required")
		return
	}
	if nestedFieldTouched(raw, "promptOptimizeModel") {
		if err := validatePromptOptimizeModel(r.Context(), h.app, settings.PromptOptimizeModel); err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if strings.TrimSpace(settings.DefaultImageModel) == "" {
		render.Error(w, http.StatusBadRequest, "defaultImageModel is required")
		return
	}
	if strings.TrimSpace(settings.DefaultVideoModel) == "" {
		render.Error(w, http.StatusBadRequest, "defaultVideoModel is required")
		return
	}
	if strings.TrimSpace(settings.StoryboardModel) == "" {
		settings.StoryboardModel = strings.TrimSpace(ai.DefaultVideoStoryboardModelName)
	}
	if strings.TrimSpace(settings.ImageStoryboardModel) == "" {
		settings.ImageStoryboardModel = settings.DefaultChatModel
	}
	if storyboardModelTouched(raw) {
		if err := validateStoryboardPackageModel(r.Context(), h.app, settings.StoryboardModel); err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if strings.TrimSpace(settings.SMSRegistration.Provider) == "" {
		settings.SMSRegistration.Provider = "aliyun_dypnsapi"
	}
	if strings.TrimSpace(settings.SMSRegistration.Endpoint) == "" {
		switch settings.SMSRegistration.Provider {
		case "aliyun_dysmsapi":
			settings.SMSRegistration.Endpoint = "dysmsapi.aliyuncs.com"
		default:
			settings.SMSRegistration.Endpoint = "dypnsapi.aliyuncs.com"
		}
	}
	if strings.TrimSpace(settings.SMSRegistration.TemplateParam) == "" {
		settings.SMSRegistration.TemplateParam = `{"code":"##code##"}`
	}
	if strings.TrimSpace(settings.SMSRegistration.DefaultCountryCode) == "" {
		settings.SMSRegistration.DefaultCountryCode = "86"
	}
	if settings.SMSRegistration.ValidMinutes <= 0 {
		settings.SMSRegistration.ValidMinutes = 10
	}
	if settings.SMSRegistration.CooldownSeconds <= 0 {
		settings.SMSRegistration.CooldownSeconds = 60
	}
	if settings.SMSRegistration.DailyLimit <= 0 {
		settings.SMSRegistration.DailyLimit = 10
	}
	if settings.SMSRegistration.CodeLength < 4 || settings.SMSRegistration.CodeLength > 8 {
		render.Error(w, http.StatusBadRequest, "smsRegistration.codeLength must be between 4 and 8")
		return
	}
	if settings.SMSRegistration.Enabled {
		if strings.TrimSpace(settings.SMSRegistration.AccessKeyID) == "" {
			render.Error(w, http.StatusBadRequest, "smsRegistration.accessKeyId is required when sms registration is enabled")
			return
		}
		if strings.TrimSpace(settings.SMSRegistration.AccessKeySecret) == "" {
			render.Error(w, http.StatusBadRequest, "smsRegistration.accessKeySecret is required when sms registration is enabled")
			return
		}
		if strings.TrimSpace(settings.SMSRegistration.SignName) == "" {
			render.Error(w, http.StatusBadRequest, "smsRegistration.signName is required when sms registration is enabled")
			return
		}
		if strings.TrimSpace(settings.SMSRegistration.TemplateCode) == "" {
			render.Error(w, http.StatusBadRequest, "smsRegistration.templateCode is required when sms registration is enabled")
			return
		}
		if !strings.Contains(settings.SMSRegistration.TemplateParam, "##code##") {
			render.Error(w, http.StatusBadRequest, "smsRegistration.templateParam must contain ##code## as the verification placeholder")
			return
		}
	}

	record, err := h.app.Store.UpsertAdminSystemSettings(r.Context(), store.UpsertAdminSystemSettingsInput{
		AIWorkerEnabled:                    settings.AIWorkerEnabled,
		PaymentChannels:                    settings.PaymentChannels,
		BillingManualSupportName:           settings.BillingManualSupport.Name,
		BillingManualSupportContact:        settings.BillingManualSupport.Contact,
		BillingManualSupportQRCodeURL:      settings.BillingManualSupport.QRCodeURL,
		BillingManualSupportNote:           settings.BillingManualSupport.Note,
		SMSRegistrationEnabled:             settings.SMSRegistration.Enabled,
		SMSRegistrationProvider:            settings.SMSRegistration.Provider,
		SMSRegistrationEndpoint:            settings.SMSRegistration.Endpoint,
		SMSRegistrationAccessKeyID:         settings.SMSRegistration.AccessKeyID,
		SMSRegistrationAccessKeySecret:     settings.SMSRegistration.AccessKeySecret,
		SMSRegistrationSignName:            settings.SMSRegistration.SignName,
		SMSRegistrationTemplateCode:        settings.SMSRegistration.TemplateCode,
		SMSRegistrationTemplateParam:       settings.SMSRegistration.TemplateParam,
		SMSRegistrationSchemeName:          settings.SMSRegistration.SchemeName,
		SMSRegistrationDefaultCountryCode:  settings.SMSRegistration.DefaultCountryCode,
		SMSRegistrationValidMinutes:        settings.SMSRegistration.ValidMinutes,
		SMSRegistrationCooldownSeconds:     settings.SMSRegistration.CooldownSeconds,
		SMSRegistrationDailyLimit:          settings.SMSRegistration.DailyLimit,
		SMSRegistrationCodeLength:          settings.SMSRegistration.CodeLength,
		DigitalHumanCreditsPerSecond:       store.DigitalHumanRoundMillisToWholeCredits(settings.DigitalHumanCreditsPerSecondMillis),
		DigitalHumanCreditsPerSecondMillis: settings.DigitalHumanCreditsPerSecondMillis,
		MixVideoCreditsPerSecond:           store.RoundMillisToWholeCredits(settings.MixVideoCreditsPerSecondMillis),
		MixVideoCreditsPerSecondMillis:     settings.MixVideoCreditsPerSecondMillis,
		DigitalHumanShoppingDefaultModel:   settings.DigitalHumanShoppingDefaultModel,
		DigitalHumanSpeechDefaultModel:     settings.DigitalHumanSpeechDefaultModel,
		DefaultChatModel:                   settings.DefaultChatModel,
		PromptOptimizeModel:                settings.PromptOptimizeModel,
		DefaultImageModel:                  settings.DefaultImageModel,
		DefaultVideoModel:                  settings.DefaultVideoModel,
		VideoCoverPrompt:                   settings.VideoCoverPrompt,
		StoryboardPrompt:                   settings.StoryboardPrompt,
		StoryboardModel:                    settings.StoryboardModel,
		StoryboardReferences:               settings.StoryboardReferences,
		ImageStoryboardPrompt:              settings.ImageStoryboardPrompt,
		ImageStoryboardModel:               settings.ImageStoryboardModel,
		ImageStoryboardReferences:          settings.ImageStoryboardReferences,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update system config")
		return
	}

	settings.UpdatedAt = adminTimePtr(record.UpdatedAt)

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  auditStringPtr(admin.ID),
		AdminEmail:   auditStringPtr(admin.Email),
		AdminName:    auditStringPtr(admin.Name),
		ResourceType: "system_config",
		ResourceID:   auditStringPtr(record.ID),
		Action:       "update",
		Title:        "更新系统配置",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("系统配置已更新"),
		Payload: mustJSONBytes(map[string]any{
			"aiWorkerEnabled":                  settings.AIWorkerEnabled,
			"paymentChannels":                  settings.PaymentChannels,
			"billingManualSupport":             settings.BillingManualSupport,
			"digitalHumanCreditsPerSecond":     store.DigitalHumanCreditsFromMillis(settings.DigitalHumanCreditsPerSecondMillis),
			"digitalHumanShoppingDefaultModel": settings.DigitalHumanShoppingDefaultModel,
			"digitalHumanSpeechDefaultModel":   settings.DigitalHumanSpeechDefaultModel,
			"smsRegistration": map[string]any{
				"enabled":            settings.SMSRegistration.Enabled,
				"provider":           settings.SMSRegistration.Provider,
				"endpoint":           settings.SMSRegistration.Endpoint,
				"accessKeyId":        maskSecretForAudit(settings.SMSRegistration.AccessKeyID),
				"accessKeySecret":    maskSecretForAudit(settings.SMSRegistration.AccessKeySecret),
				"signName":           settings.SMSRegistration.SignName,
				"templateCode":       settings.SMSRegistration.TemplateCode,
				"templateParam":      settings.SMSRegistration.TemplateParam,
				"schemeName":         settings.SMSRegistration.SchemeName,
				"defaultCountryCode": settings.SMSRegistration.DefaultCountryCode,
				"validMinutes":       settings.SMSRegistration.ValidMinutes,
				"cooldownSeconds":    settings.SMSRegistration.CooldownSeconds,
				"dailyLimit":         settings.SMSRegistration.DailyLimit,
				"codeLength":         settings.SMSRegistration.CodeLength,
			},
			"defaultChatModel":          settings.DefaultChatModel,
			"promptOptimizeModel":       settings.PromptOptimizeModel,
			"defaultImageModel":         settings.DefaultImageModel,
			"defaultVideoModel":         settings.DefaultVideoModel,
			"videoCoverPrompt":          settings.VideoCoverPrompt,
			"storyboardPrompt":          settings.StoryboardPrompt,
			"storyboardModel":           settings.StoryboardModel,
			"storyboardReferences":      json.RawMessage(settings.StoryboardReferences),
			"imageStoryboardPrompt":     settings.ImageStoryboardPrompt,
			"imageStoryboardModel":      settings.ImageStoryboardModel,
			"imageStoryboardReferences": json.RawMessage(settings.ImageStoryboardReferences),
			"updatedAt":                 record.UpdatedAt,
		}),
	})

	if nestedFieldTouched(raw, "storyboardReferences") || nestedFieldTouched(raw, "imageStoryboardReferences") {
		cleanupRemovedStoryboardReferences(
			r.Context(),
			h.app,
			mergeStoryboardReferencePayloads(previousStoryboardReferences, previousImageStoryboardReferences),
			mergeStoryboardReferencePayloads(settings.StoryboardReferences, settings.ImageStoryboardReferences),
		)
	}

	render.JSON(w, http.StatusOK, buildAdminSystemConfigPayload(h.app, settings))
}

// 处理管理端认证上传分镜资源接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminAuthHandler) UploadStoryboardAsset(w http.ResponseWriter, r *http.Request) {
	admin := httpcontext.CurrentAdmin(r.Context())
	if admin == nil {
		render.Error(w, http.StatusUnauthorized, "Admin not found")
		return
	}
	if h.app == nil || h.app.Storage == nil {
		render.Error(w, http.StatusServiceUnavailable, "Storage service is not available")
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		render.Error(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		render.Error(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 32<<20))
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to read file")
		return
	}

	fileName := sanitizeUploadFilename(header.Filename)
	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	object, err := h.app.Storage.SaveBytes(
		r.Context(),
		fmt.Sprintf("system-config/storyboard/%s-%s", uuid.NewString(), fileName),
		contentType,
		data,
	)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to store storyboard reference")
		return
	}

	kind := "text"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(object.ContentType)), "image/") {
		kind = "image"
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  auditStringPtr(admin.ID),
		AdminEmail:   auditStringPtr(admin.Email),
		AdminName:    auditStringPtr(admin.Name),
		ResourceType: "system_config",
		Action:       "upload_storyboard_asset",
		Title:        "上传分镜参考文件",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("分镜参考文件已上传"),
		Payload: mustJSONBytes(map[string]any{
			"fileName":   fileName,
			"mimeType":   object.ContentType,
			"storageKey": object.StorageKey,
			"publicUrl":  object.PublicURL,
			"sizeBytes":  object.SizeBytes,
			"kind":       kind,
		}),
	})

	render.JSON(w, http.StatusCreated, map[string]any{
		"fileName":   fileName,
		"mimeType":   object.ContentType,
		"storageKey": object.StorageKey,
		"publicUrl":  object.PublicURL,
		"sizeBytes":  object.SizeBytes,
		"kind":       kind,
	})
}
