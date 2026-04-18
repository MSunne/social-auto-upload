package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	aiclient "omnidrive_cloud/internal/ai"
	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

var aiModelSlugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

type AdminAIHandler struct {
	app *appstate.App
}

// 创建管理端AIHandler相关实例，组装运行所需依赖并返回给上层流程复用。
func NewAdminAIHandler(app *appstate.App) *AdminAIHandler {
	return &AdminAIHandler{app: app}
}

type adminCreateAIModelRequest struct {
	ID                        string          `json:"id"`
	Vendor                    string          `json:"vendor"`
	ModelName                 string          `json:"modelName"`
	ModelAlias                string          `json:"modelAlias"`
	Category                  string          `json:"category"`
	BillingMode               string          `json:"billingMode"`
	ChatProtocol              string          `json:"chatProtocol"`
	ModelType                 string          `json:"modelType"`
	BaseURL                   *string         `json:"baseUrl"`
	APIKey                    *string         `json:"apiKey"`
	RawRate                   *float64        `json:"rawRate"`
	BillingAmount             *float64        `json:"billingAmount"`
	ChatInputRawRate          *float64        `json:"chatInputRawRate"`
	ChatOutputRawRate         *float64        `json:"chatOutputRawRate"`
	ChatInputBillingAmount    *float64        `json:"chatInputBillingAmount"`
	ChatOutputBillingAmount   *float64        `json:"chatOutputBillingAmount"`
	Description               *string         `json:"description"`
	PricingPayload            json.RawMessage `json:"pricingPayload"`
	ImageReferenceLimit       *int            `json:"imageReferenceLimit"`
	ImageSupportedSizes       []string        `json:"imageSupportedSizes"`
	VideoReferenceLimit       *int            `json:"videoReferenceLimit"`
	VideoSupportedResolutions []string        `json:"videoSupportedResolutions"`
	VideoSupportedDurations   []string        `json:"videoSupportedDurations"`
	SupportedFileTypes        []string        `json:"supportedFileTypes"`
	IsEnabled                 bool            `json:"isEnabled"`
}

type adminUpdateAIModelRequest struct {
	Vendor                    *string          `json:"vendor"`
	ModelName                 *string          `json:"modelName"`
	ModelAlias                *string          `json:"modelAlias"`
	Category                  *string          `json:"category"`
	BillingMode               *string          `json:"billingMode"`
	ChatProtocol              *string          `json:"chatProtocol"`
	ModelType                 *string          `json:"modelType"`
	BaseURL                   *string          `json:"baseUrl"`
	APIKey                    *string          `json:"apiKey"`
	RawRate                   *float64         `json:"rawRate"`
	BillingAmount             *float64         `json:"billingAmount"`
	ChatInputRawRate          *float64         `json:"chatInputRawRate"`
	ChatOutputRawRate         *float64         `json:"chatOutputRawRate"`
	ChatInputBillingAmount    *float64         `json:"chatInputBillingAmount"`
	ChatOutputBillingAmount   *float64         `json:"chatOutputBillingAmount"`
	Description               *string          `json:"description"`
	PricingPayload            *json.RawMessage `json:"pricingPayload"`
	ImageReferenceLimit       *int             `json:"imageReferenceLimit"`
	ImageSupportedSizes       *[]string        `json:"imageSupportedSizes"`
	VideoReferenceLimit       *int             `json:"videoReferenceLimit"`
	VideoSupportedResolutions *[]string        `json:"videoSupportedResolutions"`
	VideoSupportedDurations   *[]string        `json:"videoSupportedDurations"`
	SupportedFileTypes        *[]string        `json:"supportedFileTypes"`
	IsEnabled                 *bool            `json:"isEnabled"`
}

// 规范化AI模型Category，统一管理端AI模型链路的输入格式和后续处理行为。
func normalizeAIModelCategory(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "text":
		return "chat"
	case "audio":
		return "music"
	default:
		return strings.ToLower(strings.TrimSpace(category))
	}
}

// 处理校验AI模型Category相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func validateAIModelCategory(category string) bool {
	switch normalizeAIModelCategory(category) {
	case "image", "video", "chat", "music":
		return true
	default:
		return false
	}
}

// 规范化AI模型计费Mode，统一管理端AI模型链路的输入格式和后续处理行为。
func normalizeAIModelBillingMode(category string, billingMode string) string {
	switch strings.ToLower(strings.TrimSpace(billingMode)) {
	case "per_call", "per_second", "per_token":
		return strings.ToLower(strings.TrimSpace(billingMode))
	default:
		if normalizeAIModelCategory(category) == "chat" {
			return "per_token"
		}
		return "per_call"
	}
}

// 处理校验AI模型计费Mode相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func validateAIModelBillingMode(billingMode string) bool {
	switch strings.ToLower(strings.TrimSpace(billingMode)) {
	case "per_call", "per_second", "per_token":
		return true
	default:
		return false
	}
}

func normalizeAIModelChatProtocol(category string, value string, baseURL *string) string {
	if normalizeAIModelCategory(category) != "chat" {
		return aiclient.ChatProtocolAuto
	}
	normalized := aiclient.ResolveConfiguredChatProtocol(valueOrEmpty(baseURL), value)
	if normalized == "" {
		return ""
	}
	return normalized
}

// 规范化可选管理端Text，统一管理端AI模型链路的输入格式和后续处理行为。
func normalizeOptionalAdminText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// 规范化管理端String列表，统一管理端AI模型链路的输入格式和后续处理行为。
func normalizeAdminStringList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

// 规范化管理端Supported文件Types，统一管理端AI模型链路的输入格式和后续处理行为。
func normalizeAdminSupportedFileTypes(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		if !strings.Contains(trimmed, "/") && !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + strings.TrimPrefix(trimmed, ".")
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

// 规范化管理端更新可选Text，统一管理端AI模型链路的输入格式和后续处理行为。
func normalizeAdminUpdateOptionalText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

// 规范化创建AI模型载荷，统一管理端AI模型链路的输入格式和后续处理行为。
func normalizeCreateAIModelPayload(payload adminCreateAIModelRequest) (store.CreateAIModelInput, error) {
	category := normalizeAIModelCategory(firstNonEmptyAdminValue(payload.ModelType, payload.Category))
	if !validateAIModelCategory(category) {
		return store.CreateAIModelInput{}, errInvalidAIModelCategory
	}
	rawBillingMode := strings.TrimSpace(payload.BillingMode)
	if rawBillingMode != "" && !validateAIModelBillingMode(rawBillingMode) {
		return store.CreateAIModelInput{}, errInvalidAIModelBillingMode
	}
	billingMode := normalizeAIModelBillingMode(category, rawBillingMode)
	chatProtocol := normalizeAIModelChatProtocol(category, payload.ChatProtocol, payload.BaseURL)
	if chatProtocol == "" {
		return store.CreateAIModelInput{}, errInvalidAIModelChatProtocol
	}

	vendor := strings.TrimSpace(payload.Vendor)
	modelName := strings.TrimSpace(payload.ModelName)
	modelAlias := strings.TrimSpace(payload.ModelAlias)
	baseURL := normalizeOptionalAdminText(payload.BaseURL)
	if vendor == "" || modelName == "" || baseURL == nil {
		return store.CreateAIModelInput{}, renderableError("vendor, modelName, and baseUrl are required")
	}
	if modelAlias == "" {
		return store.CreateAIModelInput{}, renderableError("modelAlias is required")
	}

	input := store.CreateAIModelInput{
		ID:                 strings.TrimSpace(payload.ID),
		Vendor:             vendor,
		ModelName:          modelName,
		ModelAlias:         modelAlias,
		Category:           category,
		BillingMode:        billingMode,
		ChatProtocol:       chatProtocol,
		BaseURL:            baseURL,
		APIKey:             normalizeOptionalAdminText(payload.APIKey),
		RawRate:            payload.RawRate,
		BillingAmount:      payload.BillingAmount,
		Description:        normalizeOptionalAdminText(payload.Description),
		IsEnabled:          payload.IsEnabled,
		PricingPayload:     nil,
		SupportedFileTypes: mustJSONBytes(normalizeAdminSupportedFileTypes(payload.SupportedFileTypes)),
	}
	if payload.PricingPayload != nil {
		input.PricingPayload = []byte(payload.PricingPayload)
	}

	switch category {
	case "image":
		input.ImageReferenceLimit = payload.ImageReferenceLimit
		input.ImageSupportedSizes = mustJSONBytes(normalizeAdminStringList(payload.ImageSupportedSizes))
		input.VideoSupportedResolutions = mustJSONBytes([]string{})
		input.VideoSupportedDurations = mustJSONBytes([]string{})
	case "video":
		input.VideoReferenceLimit = payload.VideoReferenceLimit
		input.ImageSupportedSizes = mustJSONBytes([]string{})
		input.VideoSupportedResolutions = mustJSONBytes(normalizeAdminStringList(payload.VideoSupportedResolutions))
		input.VideoSupportedDurations = mustJSONBytes(normalizeAdminStringList(payload.VideoSupportedDurations))
	default:
		input.ImageSupportedSizes = mustJSONBytes([]string{})
		input.VideoSupportedResolutions = mustJSONBytes([]string{})
		input.VideoSupportedDurations = mustJSONBytes([]string{})
	}

	if input.ID == "" {
		input.ID = buildAIModelID(input.Vendor, input.ModelName)
	}

	return input, nil
}

// 规范化更新AI模型载荷，统一管理端AI模型链路的输入格式和后续处理行为。
func normalizeUpdateAIModelPayload(payload adminUpdateAIModelRequest, currentCategory string) (store.UpdateAIModelInput, error) {
	if payload.ModelAlias != nil && strings.TrimSpace(valueOrEmpty(payload.ModelAlias)) == "" {
		return store.UpdateAIModelInput{}, renderableError("modelAlias cannot be empty")
	}
	input := store.UpdateAIModelInput{
		Vendor:         trimmedStringPtr(strings.TrimSpace(valueOrEmpty(payload.Vendor))),
		ModelName:      trimmedStringPtr(strings.TrimSpace(valueOrEmpty(payload.ModelName))),
		ModelAlias:     trimmedStringPtr(strings.TrimSpace(valueOrEmpty(payload.ModelAlias))),
		BaseURL:        normalizeOptionalAdminText(payload.BaseURL),
		APIKey:         normalizeAdminUpdateOptionalText(payload.APIKey),
		RawRate:        payload.RawRate,
		BillingAmount:  payload.BillingAmount,
		Description:    normalizeOptionalAdminText(payload.Description),
		IsEnabled:      payload.IsEnabled,
		PricingPayload: nil,
	}
	if payload.PricingPayload != nil && *payload.PricingPayload != nil {
		input.PricingPayload = []byte(*payload.PricingPayload)
	}

	resolvedCategory := ""
	if payload.Category != nil || payload.ModelType != nil {
		resolvedCategory = normalizeAIModelCategory(firstNonEmptyAdminValue(valueOrEmpty(payload.ModelType), valueOrEmpty(payload.Category)))
		if !validateAIModelCategory(resolvedCategory) {
			return store.UpdateAIModelInput{}, errInvalidAIModelCategory
		}
		input.Category = &resolvedCategory
	}
	if payload.BillingMode != nil {
		rawBillingMode := strings.TrimSpace(valueOrEmpty(payload.BillingMode))
		if rawBillingMode != "" && !validateAIModelBillingMode(rawBillingMode) {
			return store.UpdateAIModelInput{}, errInvalidAIModelBillingMode
		}
		resolvedBillingMode := normalizeAIModelBillingMode(firstNonEmptyAdminValue(resolvedCategory, valueOrEmpty(payload.Category)), rawBillingMode)
		input.BillingMode = &resolvedBillingMode
	}
	resolvedCategoryForProtocol := firstNonEmptyAdminValue(resolvedCategory, normalizeAIModelCategory(currentCategory))
	shouldUpdateChatProtocol := payload.ChatProtocol != nil
	if normalizeAIModelCategory(resolvedCategoryForProtocol) == "chat" && payload.BaseURL != nil {
		shouldUpdateChatProtocol = true
	}
	if shouldUpdateChatProtocol {
		resolvedChatProtocol := normalizeAIModelChatProtocol(resolvedCategoryForProtocol, valueOrEmpty(payload.ChatProtocol), payload.BaseURL)
		if resolvedChatProtocol == "" {
			return store.UpdateAIModelInput{}, errInvalidAIModelChatProtocol
		}
		input.ChatProtocol = &resolvedChatProtocol
	}

	if payload.ImageReferenceLimit != nil {
		input.ImageReferenceLimit = payload.ImageReferenceLimit
	}
	if payload.ImageSupportedSizes != nil {
		values := normalizeAdminStringList(*payload.ImageSupportedSizes)
		input.ImageSupportedSizes = &values
	}
	if payload.VideoReferenceLimit != nil {
		input.VideoReferenceLimit = payload.VideoReferenceLimit
	}
	if payload.VideoSupportedResolutions != nil {
		values := normalizeAdminStringList(*payload.VideoSupportedResolutions)
		input.VideoSupportedResolutions = &values
	}
	if payload.VideoSupportedDurations != nil {
		values := normalizeAdminStringList(*payload.VideoSupportedDurations)
		input.VideoSupportedDurations = &values
	}
	if payload.SupportedFileTypes != nil {
		values := normalizeAdminSupportedFileTypes(*payload.SupportedFileTypes)
		input.SupportedFileTypes = &values
	}

	switch resolvedCategory {
	case "image":
		empty := []string{}
		input.VideoSupportedResolutions = &empty
		input.VideoSupportedDurations = &empty
	case "video":
		empty := []string{}
		input.ImageSupportedSizes = &empty
	default:
		if resolvedCategory != "" {
			empty := []string{}
			zero := 0
			input.ImageReferenceLimit = &zero
			input.VideoReferenceLimit = &zero
			input.ImageSupportedSizes = &empty
			input.VideoSupportedResolutions = &empty
			input.VideoSupportedDurations = &empty
		}
	}

	return input, nil
}

// 构建AI模型ID，为管理端AI模型生成后续步骤所需的派生参数或载荷。
func buildAIModelID(vendor string, modelName string) string {
	seed := strings.ToLower(strings.TrimSpace(vendor + "-" + modelName))
	slug := strings.Trim(aiModelSlugSanitizer.ReplaceAllString(seed, "-"), "-")
	if slug != "" {
		return slug
	}
	return uuid.NewString()
}

// 处理管理端AI列表模型接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminAIHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, err := h.app.Store.ListAdminAIModels(r.Context(), store.AdminAIModelListFilter{
		Query:    strings.TrimSpace(r.URL.Query().Get("query")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Category: normalizeAIModelCategory(firstNonEmptyAdminValue(r.URL.Query().Get("modelType"), r.URL.Query().Get("category"))),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI models")
		return
	}

	renderAdminList(w, page, total, items, nil, map[string]any{
		"query":           strings.TrimSpace(r.URL.Query().Get("query")),
		"status":          strings.TrimSpace(r.URL.Query().Get("status")),
		"category":        normalizeAIModelCategory(firstNonEmptyAdminValue(r.URL.Query().Get("modelType"), r.URL.Query().Get("category"))),
		"statusOptions":   []string{"active", "inactive"},
		"categoryOptions": []string{"image", "video", "chat", "music"},
	})
}

// 处理管理端AI详情模型接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminAIHandler) DetailModel(w http.ResponseWriter, r *http.Request) {
	modelID := strings.TrimSpace(chi.URLParam(r, "modelId"))
	if modelID == "" {
		render.Error(w, http.StatusBadRequest, "modelId is required")
		return
	}

	record, err := h.app.Store.GetAIModelByID(r.Context(), modelID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI model")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "AI model not found")
		return
	}

	render.JSON(w, http.StatusOK, record)
}

var errInvalidAIModelCategory = renderableError("category must be one of: image, video, chat, music")
var errInvalidAIModelBillingMode = renderableError("billingMode must be one of: per_call, per_second, per_token")
var errInvalidAIModelChatProtocol = renderableError("chatProtocol must be one of: auto, openai_chat_completions, anthropic_messages")

type renderableError string

// 返回renderable错误的可读错误消息，供日志记录和错误透传统一使用。
func (e renderableError) Error() string { return string(e) }

// 处理管理端AI创建模型接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminAIHandler) CreateModel(w http.ResponseWriter, r *http.Request) {
	var payload adminCreateAIModelRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	input, err := normalizeCreateAIModelPayload(payload)
	if err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	record, err := h.app.Store.CreateAIModel(r.Context(), input)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create AI model")
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "ai_model",
		ResourceID:   stringPtr(record.ID),
		Action:       "create",
		Title:        "新增AI模型配置",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("已成功添加大模型 " + record.ModelName),
		Payload: mustJSONBytes(map[string]any{
			"id":        record.ID,
			"vendor":    record.Vendor,
			"modelName": record.ModelName,
			"category":  record.Category,
			"isEnabled": record.IsEnabled,
		}),
	})

	render.JSON(w, http.StatusCreated, record)
}

// 处理管理端AI更新模型接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminAIHandler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	modelID := strings.TrimSpace(chi.URLParam(r, "modelId"))
	if modelID == "" {
		render.Error(w, http.StatusBadRequest, "modelId is required")
		return
	}

	var payload adminUpdateAIModelRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	currentModel, err := h.app.Store.GetAIModelByID(r.Context(), modelID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect AI model")
		return
	}
	if currentModel == nil {
		render.Error(w, http.StatusNotFound, "AI model not found")
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	input, err := normalizeUpdateAIModelPayload(payload, currentModel.Category)
	if err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	record, err := h.app.Store.UpdateAIModel(r.Context(), modelID, input)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update AI model")
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "ai_model",
		ResourceID:   stringPtr(record.ID),
		Action:       "update",
		Title:        "更新AI模型配置",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("已更新大模型 " + record.ModelName),
		Payload: mustJSONBytes(map[string]any{
			"id":        record.ID,
			"isEnabled": record.IsEnabled,
			"category":  record.Category,
		}),
	})

	render.JSON(w, http.StatusOK, record)
}

// 处理管理端AI删除模型接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminAIHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	modelID := strings.TrimSpace(chi.URLParam(r, "modelId"))
	if modelID == "" {
		render.Error(w, http.StatusBadRequest, "modelId is required")
		return
	}

	record, err := h.app.Store.GetAIModelByID(r.Context(), modelID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect AI model")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "AI model not found")
		return
	}

	usage, err := h.app.Store.GetAIModelUsageSummary(r.Context(), modelID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect AI model usage")
		return
	}
	if usage.SkillCount > 0 || usage.AIJobCount > 0 || usage.SystemConfigDefaultCount > 0 || usage.DeviceDefaultCount > 0 {
		render.JSON(w, http.StatusConflict, map[string]any{
			"error": "AI model is still referenced by skills, AI jobs, or default settings",
			"usage": map[string]any{
				"skillCount":               usage.SkillCount,
				"aiJobCount":               usage.AIJobCount,
				"systemConfigDefaultCount": usage.SystemConfigDefaultCount,
				"deviceDefaultCount":       usage.DeviceDefaultCount,
			},
		})
		return
	}

	deleted, err := h.app.Store.DeleteAIModel(r.Context(), modelID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to delete AI model")
		return
	}
	if !deleted {
		render.Error(w, http.StatusNotFound, "AI model not found")
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "ai_model",
		ResourceID:   stringPtr(record.ID),
		Action:       "delete",
		Title:        "删除AI模型配置",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("已删除大模型 " + record.ModelName),
		Payload: mustJSONBytes(map[string]any{
			"id":        record.ID,
			"vendor":    record.Vendor,
			"modelName": record.ModelName,
			"category":  record.Category,
		}),
	})

	render.JSON(w, http.StatusOK, map[string]any{
		"deleted": true,
		"id":      record.ID,
	})
}
