package handlers

import (
	"net/http"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/http/render"
)

type HealthHandler struct {
	app *appstate.App
}

func NewHealthHandler(app *appstate.App) *HealthHandler {
	return &HealthHandler{app: app}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Store.Ping(r.Context()); err != nil {
		render.Error(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	render.JSON(w, http.StatusOK, map[string]any{
		"name":        "OmniDrive API",
		"environment": h.app.Config.Environment,
		"status":      "ok",
	})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, _ *http.Request) {
	render.JSON(w, http.StatusOK, map[string]any{
		"status": "ready",
		"modules": []string{
			"auth",
			"devices",
			"accounts",
			"skills",
			"tasks",
			"agent",
		},
	})
}
