package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/http/render"
)

type adminPageQuery struct {
	Page     int
	PageSize int
}

type adminListResponse[T any] struct {
	Items      []T                    `json:"items"`
	Pagination domain.AdminPagination `json:"pagination"`
	Summary    any                    `json:"summary,omitempty"`
	Filters    any                    `json:"filters,omitempty"`
}

// 解析管理端PageQuery，为管理端公共提供结构化输入。
func parseAdminPageQuery(r *http.Request) adminPageQuery {
	page := 1
	pageSize := 20

	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			page = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("pageSize")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			pageSize = parsed
		}
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return adminPageQuery{
		Page:     page,
		PageSize: pageSize,
	}
}

// 构建管理端Pagination，为管理端公共生成后续步骤所需的派生参数或载荷。
func buildAdminPagination(page int, pageSize int, total int64) domain.AdminPagination {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return domain.AdminPagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}

func renderAdminList[T any](w http.ResponseWriter, page adminPageQuery, total int64, items []T, summary any, filters any) {
	render.JSON(w, http.StatusOK, adminListResponse[T]{
		Items:      items,
		Pagination: buildAdminPagination(page.Page, page.PageSize, total),
		Summary:    summary,
		Filters:    filters,
	})
}

// 规范化管理端账号，统一管理端公共链路的输入格式和后续处理行为。
func normalizeAdminAccount(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// 处理首个Non空值管理端值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstNonEmptyAdminValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
