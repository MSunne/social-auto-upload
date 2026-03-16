package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AdminSkillHandler struct {
	app *appstate.App
}

func NewAdminSkillHandler(app *appstate.App) *AdminSkillHandler {
	return &AdminSkillHandler{app: app}
}

type adminUpdateSkillRequest struct {
	IsEnabled *bool `json:"isEnabled"`
}

func (h *AdminSkillHandler) ListSkills(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, err := h.app.Store.ListAdminSkills(r.Context(), store.AdminSkillListFilter{
		Query:  strings.TrimSpace(r.URL.Query().Get("query")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skills")
		return
	}

	renderAdminList(w, page, total, items, nil, map[string]any{
		"query":         strings.TrimSpace(r.URL.Query().Get("query")),
		"status":        strings.TrimSpace(r.URL.Query().Get("status")),
		"statusOptions": []string{"active", "inactive"},
	})
}

func (h *AdminSkillHandler) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
	if skillID == "" {
		render.Error(w, http.StatusBadRequest, "skillId is required")
		return
	}

	var payload adminUpdateSkillRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	record, err := h.app.Store.UpdateProductSkillAdmin(r.Context(), skillID, store.UpdateProductSkillAdminInput{
		IsEnabled: payload.IsEnabled,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update skill")
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "product_skill",
		ResourceID:   stringPtr(record.ID),
		Action:       "update",
		Title:        "更新公共技能状态",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("已更新技能 " + record.Name),
		Payload: mustJSONBytes(map[string]any{
			"id":        record.ID,
			"isEnabled": record.IsEnabled,
		}),
	})

	render.JSON(w, http.StatusOK, record)
}
