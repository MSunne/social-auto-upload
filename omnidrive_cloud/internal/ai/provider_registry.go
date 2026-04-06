package ai

import (
	"fmt"
	"net/url"
	"strings"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/domain"
)

func newProviders(cfg config.Config) (map[string]Provider, error) {
	apiyiProvider, err := NewAPIYIProvider(cfg)
	if err != nil {
		return nil, err
	}
	lconaiProvider, err := NewLConAIProvider(cfg)
	if err != nil {
		return nil, err
	}

	return map[string]Provider{
		"apiyi":  apiyiProvider,
		"lconai": lconaiProvider,
	}, nil
}

func resolveProviderForModel(providers map[string]Provider, model *domain.AIModel) (Provider, error) {
	if model == nil {
		return nil, fmt.Errorf("ai model is required")
	}
	vendor := providerSource(model)
	provider, ok := providers[vendor]
	if !ok || provider == nil {
		return nil, fmt.Errorf("provider is not configured for vendor: %s", vendor)
	}
	return provider, nil
}

func providerSource(model *domain.AIModel) string {
	if model == nil {
		return "unknown"
	}
	vendor := normalizeProviderVendor(model.Vendor)
	switch vendor {
	case "apiyi", "lconai":
		return vendor
	case "":
		if inferred := inferProviderFromBaseURL(stringValuePtr(model.BaseURL)); inferred != "" {
			return inferred
		}
		return "unknown"
	default:
		if inferred := inferProviderFromBaseURL(stringValuePtr(model.BaseURL)); inferred != "" {
			return inferred
		}
		return vendor
	}
}

func normalizeProviderVendor(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "lconai", "lcon", "龙坤", "万象龙坤":
		return "lconai"
	case "apiyi", "api易", "万象引擎", "阿里":
		return "apiyi"
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}

func inferProviderFromBaseURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		lower := strings.ToLower(trimmed)
		switch {
		case strings.Contains(lower, "api.apiyi.com"):
			return "apiyi"
		case strings.Contains(lower, "n.lconai.com"):
			return "lconai"
		default:
			return ""
		}
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "api.apiyi.com":
		return "apiyi"
	case "n.lconai.com":
		return "lconai"
	default:
		return ""
	}
}

func stringValuePtr(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
