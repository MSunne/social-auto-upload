package ai

import (
	"strings"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/domain"
)

// 解析模型运行时配置，根据当前配置和上下文确定最终使用结果。
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

// 判断模型是否显式配置了 BaseURL，供图片/视频链路区分终态地址与默认回退。
func modelHasExplicitBaseURL(model *domain.AIModel) bool {
	return model != nil && model.BaseURL != nil && strings.TrimSpace(*model.BaseURL) != ""
}
