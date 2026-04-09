package ai

import (
	"strings"

	"omnidrive_cloud/internal/domain"
)

const (
	DefaultVideoStoryboardSystemPrompt = "你是内容创作分镜与脚本优化助手。请结合用户目标、参考图片和参考文本，输出适合继续交给视频模型执行的精炼脚本，同时生成和脚本一致的首帧封面。输出中需要保留主体、场景、镜头、风格、文案和节奏等关键信息。"
	DefaultVideoStoryboardModelName    = "gemini-3.1-pro"
	DefaultVideoCoverModelName         = "gemini-3-pro-image-preview"
)

// 处理Supports分镜包模型相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func SupportsStoryboardPackageModel(model *domain.AIModel) bool {
	if model == nil || !model.IsEnabled {
		return false
	}
	if strings.TrimSpace(strings.ToLower(model.Category)) != "chat" {
		return false
	}
	if supportsImageInputTypes(model.SupportedFileTypes) {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(model.ModelName)), "gemini")
}

// 处理supports图片输入Types相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func supportsImageInputTypes(fileTypes []string) bool {
	for _, item := range fileTypes {
		normalized := strings.ToLower(strings.TrimSpace(item))
		switch {
		case normalized == "image/*":
			return true
		case strings.HasPrefix(normalized, "image/"):
			return true
		case normalized == ".png", normalized == ".jpg", normalized == ".jpeg", normalized == ".webp", normalized == ".gif":
			return true
		}
	}
	return false
}
