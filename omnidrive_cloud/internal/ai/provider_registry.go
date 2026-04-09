package ai

import (
	"fmt"
	"net/url"
	"strings"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/domain"
)

// 创建供应方相关实例，组装运行所需依赖并返回给上层流程复用。
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

// 解析供应方模型，根据当前配置和上下文确定最终使用结果。
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

// 处理供应方来源相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 规范化供应方Vendor，统一供应方注册表链路的输入格式和后续处理行为。
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

// 推断供应方BaseURL，在输入缺省时补齐供应方注册表链路需要的派生值。
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

// 处理string值Ptr相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func stringValuePtr(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
