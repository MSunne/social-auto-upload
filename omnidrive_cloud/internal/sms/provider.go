package sms

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	ProviderAliyunDypnsapi = "aliyun_dypnsapi"
	ProviderAliyunDysmsapi = "aliyun_dysmsapi"
)

// 规范化供应方，统一供应方链路的输入格式和后续处理行为。
func NormalizeProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "aliyun", ProviderAliyunDypnsapi, "aliyun-dypnsapi":
		return ProviderAliyunDypnsapi
	case ProviderAliyunDysmsapi, "aliyun-dysmsapi", "aliyun_sms", "aliyun-sms":
		return ProviderAliyunDysmsapi
	default:
		return ""
	}
}

// 解析供应方，根据当前配置和上下文确定最终使用结果。
func ResolveProvider(provider string, templateCode string) string {
	normalized := NormalizeProvider(provider)
	if normalized == "" {
		normalized = ProviderAliyunDypnsapi
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(templateCode)), "SMS_") {
		return ProviderAliyunDysmsapi
	}
	return normalized
}

// 处理Uses本地编码验证码相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func UsesLocalCodeVerification(provider string) bool {
	return ResolveProvider(provider, "") == ProviderAliyunDysmsapi
}

// 构建TemplateParam，为供应方生成后续步骤所需的派生参数或载荷。
func BuildTemplateParam(template string, verificationCode string) string {
	normalized := normalizeTemplateParam(template)
	if strings.TrimSpace(verificationCode) == "" {
		return normalized
	}
	if !strings.Contains(normalized, "##code##") {
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(normalized), &payload); err == nil {
			if _, exists := payload["code"]; exists {
				payload["code"] = strings.TrimSpace(verificationCode)
				if encoded, marshalErr := json.Marshal(payload); marshalErr == nil {
					return string(encoded)
				}
			}
		}
	}
	return strings.ReplaceAll(normalized, "##code##", strings.TrimSpace(verificationCode))
}

// 处理Generate验证码编码相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func GenerateVerificationCode(length int) (string, error) {
	if length < 4 || length > 8 {
		length = 6
	}
	max := byte(10)
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	for index, value := range buffer {
		buffer[index] = '0' + (value % max)
	}
	return string(buffer), nil
}

// 发送Registration编码，把当前业务消息下发到外部通道。
func SendRegistrationCode(cfg RegistrationConfig, phone string, countryCode string, outID string, verificationCode string) (*SendCodeResult, error) {
	switch ResolveProvider(cfg.Provider, cfg.TemplateCode) {
	case ProviderAliyunDysmsapi:
		return sendAliyunDysmsRegistrationCode(cfg, phone, outID, verificationCode)
	case ProviderAliyunDypnsapi:
		return sendAliyunDypnsRegistrationCode(cfg, phone, countryCode, outID)
	default:
		return nil, &ProviderError{
			Provider: providerOrUnknown(cfg.Provider),
			Message:  fmt.Sprintf("unsupported sms provider: %s", strings.TrimSpace(cfg.Provider)),
		}
	}
}

// 发送Template短信，把当前业务消息下发到外部通道。
func SendTemplateSMS(cfg RegistrationConfig, phone string, outID string, signName string, templateCode string, templateParam string) (*SendCodeResult, error) {
	normalizedProvider := ResolveProvider(cfg.Provider, templateCode)
	switch normalizedProvider {
	case ProviderAliyunDysmsapi:
		return sendAliyunDysmsTemplateSMS(cfg, phone, outID, signName, templateCode, templateParam)
	default:
		return nil, &ProviderError{
			Provider: providerOrUnknown(normalizedProvider),
			Message:  fmt.Sprintf("unsupported sms provider for template sms: %s", strings.TrimSpace(normalizedProvider)),
		}
	}
}

// 处理VerifyRegistration编码相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func VerifyRegistrationCode(cfg RegistrationConfig, phone string, countryCode string, code string, outID string) (*VerifyCodeResult, error) {
	switch ResolveProvider(cfg.Provider, cfg.TemplateCode) {
	case ProviderAliyunDypnsapi:
		return verifyAliyunDypnsRegistrationCode(cfg, phone, countryCode, code, outID)
	case ProviderAliyunDysmsapi:
		return nil, &ProviderError{
			Provider: ProviderAliyunDysmsapi,
			Message:  "verification should be handled locally for aliyun_dysmsapi",
		}
	default:
		return nil, &ProviderError{
			Provider: providerOrUnknown(cfg.Provider),
			Message:  fmt.Sprintf("unsupported sms provider: %s", strings.TrimSpace(cfg.Provider)),
		}
	}
}

// 处理供应方Unknown相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func providerOrUnknown(provider string) string {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}
