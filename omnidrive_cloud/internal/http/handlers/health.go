package handlers

import (
	"net/http"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/http/render"
)

type HealthHandler struct {
	app *appstate.App
}

// 创建HealthHandler相关实例，组装运行所需依赖并返回给上层流程复用。
func NewHealthHandler(app *appstate.App) *HealthHandler {
	return &HealthHandler{app: app}
}

// 处理HealthHealth接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理Health就绪接口，解析请求参数并调用应用状态或存储层完成业务动作。
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
