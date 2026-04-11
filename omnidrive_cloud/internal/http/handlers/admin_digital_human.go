package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

func (h *AdminConsoleHandler) ListDigitalHumanModels(w http.ResponseWriter, r *http.Request) {
	renderDigitalHumanModels(w, r, h.app)
}

func (h *AdminConsoleHandler) ListDigitalHumanTasks(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, summary, err := h.app.Store.ListAdminDigitalHumanTasks(r.Context(), store.AdminDigitalHumanTaskListFilter{
		Query:  strings.TrimSpace(r.URL.Query().Get("query")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Mode:   strings.TrimSpace(r.URL.Query().Get("mode")),
		UserID: strings.TrimSpace(r.URL.Query().Get("userId")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin digital human tasks")
		return
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":  strings.TrimSpace(r.URL.Query().Get("query")),
		"status": strings.TrimSpace(r.URL.Query().Get("status")),
		"mode":   strings.TrimSpace(r.URL.Query().Get("mode")),
		"userId": strings.TrimSpace(r.URL.Query().Get("userId")),
	})
}

func (h *AdminConsoleHandler) DetailDigitalHumanTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	record, err := h.app.Store.GetAdminDigitalHumanTaskByID(r.Context(), taskID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin digital human task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Digital human task not found")
		return
	}

	render.JSON(w, http.StatusOK, record)
}
