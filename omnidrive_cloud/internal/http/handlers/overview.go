package handlers

import (
	"net/http"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
)

type OverviewHandler struct {
	app *appstate.App
}

func NewOverviewHandler(app *appstate.App) *OverviewHandler {
	return &OverviewHandler{app: app}
}

func (h *OverviewHandler) Summary(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	summary, err := h.app.Store.GetOverviewSummary(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load overview summary")
		return
	}
	render.JSON(w, http.StatusOK, summary)
}

func (h *OverviewHandler) History(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	items, err := h.app.Store.ListHistoryByOwner(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load history")
		return
	}
	render.JSON(w, http.StatusOK, items)
}
