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
	_ = app.Store.CreateAuditEvent(ctx, input)
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
