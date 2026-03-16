package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AdminAIHandler struct {
	app *appstate.App
}

func NewAdminAIHandler(app *appstate.App) *AdminAIHandler {
	return &AdminAIHandler{app: app}
}

type adminCreateAIModelRequest struct {
	ID             string          `json:"id"`
	Vendor         string          `json:"vendor"`
	ModelName      string          `json:"modelName"`
	Category       string          `json:"category"`
	Description    *string         `json:"description"`
	PricingPayload json.RawMessage `json:"pricingPayload"`
	IsEnabled      bool            `json:"isEnabled"`
}

type adminUpdateAIModelRequest struct {
	Vendor         *string          `json:"vendor"`
	ModelName      *string          `json:"modelName"`
	Category       *string          `json:"category"`
	Description    *string          `json:"description"`
	PricingPayload *json.RawMessage `json:"pricingPayload"`
	IsEnabled      *bool            `json:"isEnabled"`
}

func (h *AdminAIHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, err := h.app.Store.ListAdminAIModels(r.Context(), store.AdminAIModelListFilter{
		Query:  strings.TrimSpace(r.URL.Query().Get("query")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
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
		"query":         strings.TrimSpace(r.URL.Query().Get("query")),
		"status":        strings.TrimSpace(r.URL.Query().Get("status")),
		"statusOptions": []string{"active", "inactive"},
	})
}

func (h *AdminAIHandler) CreateModel(w http.ResponseWriter, r *http.Request) {
	var payload adminCreateAIModelRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())

	var pricingBytes []byte
	if payload.PricingPayload != nil {
		pricingBytes = []byte(payload.PricingPayload)
	}

	record, err := h.app.Store.CreateAIModel(r.Context(), store.CreateAIModelInput{
		ID:             strings.TrimSpace(payload.ID),
		Vendor:         strings.TrimSpace(payload.Vendor),
		ModelName:      strings.TrimSpace(payload.ModelName),
		Category:       strings.TrimSpace(payload.Category),
		Description:    trimmedStringPtr(valueOrEmpty(payload.Description)),
		PricingPayload: pricingBytes,
		IsEnabled:      payload.IsEnabled,
	})
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

	var pricingBytes []byte
	if payload.PricingPayload != nil && *payload.PricingPayload != nil {
		pricingBytes = []byte(*payload.PricingPayload)
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	record, err := h.app.Store.UpdateAIModel(r.Context(), modelID, store.UpdateAIModelInput{
		Vendor:         payload.Vendor,
		ModelName:      payload.ModelName,
		Category:       payload.Category,
		Description:    payload.Description,
		PricingPayload: pricingBytes,
		IsEnabled:      payload.IsEnabled,
	})
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
