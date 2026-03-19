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

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type adminSystemConfigPatchRequest struct {
	AIWorkerEnabled           *bool                           `json:"aiWorkerEnabled"`
	PaymentChannels           []string                        `json:"paymentChannels"`
	BillingManualSupport      *adminManualSupportPatchRequest `json:"billingManualSupport"`
	DefaultChatModel          *string                         `json:"defaultChatModel"`
	DefaultImageModel         *string                         `json:"defaultImageModel"`
	DefaultVideoModel         *string                         `json:"defaultVideoModel"`
	StoryboardPrompt          *string                         `json:"storyboardPrompt"`
	StoryboardModel           *string                         `json:"storyboardModel"`
	StoryboardReferences      []map[string]any                `json:"storyboardReferences"`
	ImageStoryboardPrompt     *string                         `json:"imageStoryboardPrompt"`
	ImageStoryboardModel      *string                         `json:"imageStoryboardModel"`
	ImageStoryboardReferences []map[string]any                `json:"imageStoryboardReferences"`
}

type adminManualSupportPatchRequest struct {
	Name      *string `json:"name"`
	Contact   *string `json:"contact"`
	QRCodeURL *string `json:"qrCodeUrl"`
	Note      *string `json:"note"`
}

type effectiveAdminSystemSettings struct {
	AIWorkerEnabled           bool
	PaymentChannels           []string
	BillingManualSupport      domain.AdminManualSupportConfig
	DefaultChatModel          string
	DefaultImageModel         string
	DefaultVideoModel         string
	StoryboardPrompt          string
	StoryboardModel           string
	StoryboardReferences      json.RawMessage
	ImageStoryboardPrompt     string
	ImageStoryboardModel      string
	ImageStoryboardReferences json.RawMessage
	UpdatedAt                 *time.Time
}

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
		DefaultChatModel:          strings.TrimSpace(cfg.DefaultChatModel),
		DefaultImageModel:         strings.TrimSpace(cfg.DefaultImageModel),
		DefaultVideoModel:         strings.TrimSpace(cfg.DefaultVideoModel),
		StoryboardPrompt:          "",
		StoryboardModel:           strings.TrimSpace(cfg.DefaultChatModel),
		StoryboardReferences:      []byte("[]"),
		ImageStoryboardPrompt:     "",
		ImageStoryboardModel:      strings.TrimSpace(cfg.DefaultChatModel),
		ImageStoryboardReferences: []byte("[]"),
	}
}

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

func (s effectiveAdminSystemSettings) paymentChannelEnabled(channel string) bool {
	for _, item := range s.PaymentChannels {
		if item == channel {
			return true
		}
	}
	return false
}

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
	settings.DefaultChatModel = strings.TrimSpace(record.DefaultChatModel)
	settings.DefaultImageModel = strings.TrimSpace(record.DefaultImageModel)
	settings.DefaultVideoModel = strings.TrimSpace(record.DefaultVideoModel)
	settings.StoryboardPrompt = strings.TrimSpace(record.StoryboardPrompt)
	settings.StoryboardModel = strings.TrimSpace(record.StoryboardModel)
	settings.StoryboardReferences = append([]byte(nil), record.StoryboardReferences...)
	settings.ImageStoryboardPrompt = strings.TrimSpace(record.ImageStoryboardPrompt)
	settings.ImageStoryboardModel = strings.TrimSpace(record.ImageStoryboardModel)
	settings.ImageStoryboardReferences = append([]byte(nil), record.ImageStoryboardReferences...)
	settings.UpdatedAt = adminTimePtr(record.UpdatedAt)
	return settings, nil
}

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

	return domain.AdminSystemConfig{
		AuthMode:                  "database_rbac",
		AdminEmail:                app.Config.AdminEmail,
		S3Configured:              app.Config.S3Bucket != "" && app.Config.S3Endpoint != "" && app.Config.S3AccessKey != "" && app.Config.S3SecretKey != "",
		S3Endpoint:                app.Config.S3Endpoint,
		S3Bucket:                  app.Config.S3Bucket,
		AIWorkerEnabled:           settings.AIWorkerEnabled,
		PaymentChannels:           append([]string(nil), settings.PaymentChannels...),
		BillingManualSupport:      settings.BillingManualSupport,
		DefaultChatModel:          settings.DefaultChatModel,
		DefaultImageModel:         settings.DefaultImageModel,
		DefaultVideoModel:         settings.DefaultVideoModel,
		StoryboardPrompt:          settings.StoryboardPrompt,
		StoryboardModel:           settings.StoryboardModel,
		StoryboardReferences:      settings.StoryboardReferences,
		ImageStoryboardPrompt:     settings.ImageStoryboardPrompt,
		ImageStoryboardModel:      settings.ImageStoryboardModel,
		ImageStoryboardReferences: settings.ImageStoryboardReferences,
		Notes:                     notes,
		UpdatedAt:                 settings.UpdatedAt,
	}
}

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

func nestedFieldTouched(raw map[string]json.RawMessage, key string) bool {
	_, exists := raw[key]
	return exists
}

func normalizePatchedString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func adminTimePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

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

func hasStoryboardManagedPrefix(storageKey string) bool {
	return strings.HasPrefix(strings.TrimSpace(storageKey), "system-config/storyboard/")
}

func stringValueFromAny(value any) string {
	text, _ := value.(string)
	return text
}

func (h *AdminAuthHandler) buildAdminSystemConfig(ctx context.Context) (domain.AdminSystemConfig, error) {
	settings, err := loadEffectiveAdminSystemSettings(ctx, h.app)
	if err != nil {
		return domain.AdminSystemConfig{}, err
	}
	return buildAdminSystemConfigPayload(h.app, settings), nil
}

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

	if nestedFieldTouched(raw, "defaultChatModel") {
		settings.DefaultChatModel = normalizePatchedString(payload.DefaultChatModel)
	}
	if nestedFieldTouched(raw, "defaultImageModel") {
		settings.DefaultImageModel = normalizePatchedString(payload.DefaultImageModel)
	}
	if nestedFieldTouched(raw, "defaultVideoModel") {
		settings.DefaultVideoModel = normalizePatchedString(payload.DefaultVideoModel)
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
	if strings.TrimSpace(settings.DefaultChatModel) == "" {
		render.Error(w, http.StatusBadRequest, "defaultChatModel is required")
		return
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
		settings.StoryboardModel = settings.DefaultChatModel
	}
	if strings.TrimSpace(settings.ImageStoryboardModel) == "" {
		settings.ImageStoryboardModel = settings.DefaultChatModel
	}

	record, err := h.app.Store.UpsertAdminSystemSettings(r.Context(), store.UpsertAdminSystemSettingsInput{
		AIWorkerEnabled:               settings.AIWorkerEnabled,
		PaymentChannels:               settings.PaymentChannels,
		BillingManualSupportName:      settings.BillingManualSupport.Name,
		BillingManualSupportContact:   settings.BillingManualSupport.Contact,
		BillingManualSupportQRCodeURL: settings.BillingManualSupport.QRCodeURL,
		BillingManualSupportNote:      settings.BillingManualSupport.Note,
		DefaultChatModel:              settings.DefaultChatModel,
		DefaultImageModel:             settings.DefaultImageModel,
		DefaultVideoModel:             settings.DefaultVideoModel,
		StoryboardPrompt:              settings.StoryboardPrompt,
		StoryboardModel:               settings.StoryboardModel,
		StoryboardReferences:          settings.StoryboardReferences,
		ImageStoryboardPrompt:         settings.ImageStoryboardPrompt,
		ImageStoryboardModel:          settings.ImageStoryboardModel,
		ImageStoryboardReferences:     settings.ImageStoryboardReferences,
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
			"aiWorkerEnabled":           settings.AIWorkerEnabled,
			"paymentChannels":           settings.PaymentChannels,
			"billingManualSupport":      settings.BillingManualSupport,
			"defaultChatModel":          settings.DefaultChatModel,
			"defaultImageModel":         settings.DefaultImageModel,
			"defaultVideoModel":         settings.DefaultVideoModel,
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
