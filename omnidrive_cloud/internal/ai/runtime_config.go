package ai

import (
	"strings"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/domain"
)

func ResolveModelRuntimeConfig(cfg config.Config, model *domain.AIModel) (string, string) {
	if model == nil {
		return "", ""
	}

	baseURL := ""
	if model.BaseURL != nil {
		baseURL = strings.TrimSpace(*model.BaseURL)
	}

	apiKey := ""
	if model.APIKey != nil {
		apiKey = strings.TrimSpace(*model.APIKey)
	}

	switch strings.ToLower(strings.TrimSpace(model.Vendor)) {
	case "apiyi":
		if baseURL == "" {
			baseURL = strings.TrimSpace(cfg.APIYIBaseURL)
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(cfg.APIYIApiKey)
		}
	}

	return baseURL, apiKey
}
