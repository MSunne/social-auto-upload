package ai

import (
	"testing"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/domain"
)

func TestResolveModelRuntimeConfigFallsBackToAPIYIConfig(t *testing.T) {
	model := &domain.AIModel{
		Vendor:    "apiyi",
		ModelName: "gemini-3.1-pro-preview",
	}

	baseURL, apiKey := ResolveModelRuntimeConfig(
		config.Config{
			APIYIBaseURL: "https://api.apiyi.com",
			APIYIApiKey:  "sk-test",
		},
		model,
	)

	if baseURL != "https://api.apiyi.com" {
		t.Fatalf("expected APIYI base URL fallback, got %q", baseURL)
	}
	if apiKey != "sk-test" {
		t.Fatalf("expected APIYI api key fallback, got %q", apiKey)
	}
}

func TestResolveModelRuntimeConfigPrefersModelOverrides(t *testing.T) {
	baseURLValue := "https://override.example/v1/chat/completions"
	apiKeyValue := "sk-override"
	model := &domain.AIModel{
		Vendor:    "apiyi",
		ModelName: "gemini-3.1-pro-preview",
		BaseURL:   &baseURLValue,
		APIKey:    &apiKeyValue,
	}

	baseURL, apiKey := ResolveModelRuntimeConfig(
		config.Config{
			APIYIBaseURL: "https://api.apiyi.com",
			APIYIApiKey:  "sk-test",
		},
		model,
	)

	if baseURL != baseURLValue {
		t.Fatalf("expected explicit model base URL to win, got %q", baseURL)
	}
	if apiKey != apiKeyValue {
		t.Fatalf("expected explicit model api key to win, got %q", apiKey)
	}
}
