package handlers

import (
	"net/http"
	"strconv"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type OverviewHandler struct {
	app *appstate.App
}

// 创建概览Handler相关实例，组装运行所需依赖并返回给上层流程复用。
func NewOverviewHandler(app *appstate.App) *OverviewHandler {
	return &OverviewHandler{app: app}
}

// 处理概览汇总接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *OverviewHandler) Summary(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	summary, err := h.app.Store.GetOverviewSummary(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load overview summary")
		return
	}
	render.JSON(w, http.StatusOK, summary)
}

// 处理概览历史接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *OverviewHandler) History(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	limit := 0
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			render.Error(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = parsed
	}

	items, err := h.app.Store.ListHistoryByOwner(r.Context(), user.ID, store.ListHistoryFilter{
		Kind:   strings.TrimSpace(r.URL.Query().Get("kind")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:  limit,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load history")
		return
	}
	render.JSON(w, http.StatusOK, items)
}
