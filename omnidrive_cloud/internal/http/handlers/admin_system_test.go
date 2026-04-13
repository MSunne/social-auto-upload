package handlers

import (
	"testing"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/config"
)

func TestDefaultAdminSystemSettingsUsesGeminiPromptOptimizeModel(t *testing.T) {
	settings := defaultAdminSystemSettings(config.Config{
		DefaultChatModel:  "claude-opus-4-6-thinking",
		DefaultImageModel: "gemini-3-pro-image-preview",
		DefaultVideoModel: "veo-3.1-fast-fl",
	})

	if got := settings.PromptOptimizeModel; got != "gemini-3.1-pro-preview" {
		t.Fatalf("defaultAdminSystemSettings().PromptOptimizeModel = %q, want %q", got, "gemini-3.1-pro-preview")
	}
}

func TestBuildAdminSystemConfigPayloadIncludesPromptOptimizeModel(t *testing.T) {
	cfg := config.Config{
		AdminEmail:        "admin@example.com",
		DefaultChatModel:  "gemini-3.1-pro-preview",
		DefaultImageModel: "gemini-3-pro-image-preview",
		DefaultVideoModel: "veo-3.1-fast-fl",
	}
	settings := defaultAdminSystemSettings(cfg)
	settings.PromptOptimizeModel = "gemini-3.1-pro-preview"

	payload := buildAdminSystemConfigPayload(&appstate.App{Config: cfg}, settings)
	if payload.PromptOptimizeModel != settings.PromptOptimizeModel {
		t.Fatalf("buildAdminSystemConfigPayload().PromptOptimizeModel = %q, want %q", payload.PromptOptimizeModel, settings.PromptOptimizeModel)
	}
}
