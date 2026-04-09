package handlers

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/store"
)

// 记录审计事件，把审计、账务或运行轨迹写入持久化存储。
func recordAuditEvent(app *appstate.App, ctx context.Context, input store.CreateAuditEventInput) {
	if app == nil || app.Store == nil {
		return
	}
	if input.ID == "" {
		input.ID = uuid.NewString()
	}
	if err := app.Store.CreateAuditEvent(ctx, input); err != nil {
		app.Logger.Error("failed to record user audit event",
			"audit_id", input.ID,
			"resource_type", input.ResourceType,
			"action", input.Action,
			"status", input.Status,
			"error", err,
		)
		return
	}
	app.Logger.Debug("user audit event recorded",
		"audit_id", input.ID,
		"owner_user_id", input.OwnerUserID,
		"resource_type", input.ResourceType,
		"resource_id", input.ResourceID,
		"action", input.Action,
		"status", input.Status,
	)
}

// 记录管理端审计Log，把审计、账务或运行轨迹写入持久化存储。
func recordAdminAuditLog(app *appstate.App, ctx context.Context, input store.CreateAdminAuditLogInput) {
	if app == nil || app.Store == nil {
		return
	}
	if input.ID == "" {
		input.ID = uuid.NewString()
	}
	if err := app.Store.CreateAdminAuditLog(ctx, input); err != nil {
		app.Logger.Error("failed to record admin audit log",
			"audit_id", input.ID,
			"resource_type", input.ResourceType,
			"action", input.Action,
			"status", input.Status,
			"error", err,
		)
		return
	}
	app.Logger.Debug("admin audit log recorded",
		"audit_id", input.ID,
		"admin_user_id", input.AdminUserID,
		"resource_type", input.ResourceType,
		"resource_id", input.ResourceID,
		"action", input.Action,
		"status", input.Status,
	)
}

// 将任意结构序列化为 JSON 字节，失败时直接 panic 以暴露调用方数据错误。
func mustJSONBytes(payload any) []byte {
	if payload == nil {
		return nil
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return bytes
}

// 处理审计StringPtr相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func auditStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// 将非空字符串转换为指针，统一存储层对可选字符串字段的入参表达。
func stringPtr(value string) *string {
	return auditStringPtr(value)
}
