package handlers

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/store"
)

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

func auditStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringPtr(value string) *string {
	return auditStringPtr(value)
}
