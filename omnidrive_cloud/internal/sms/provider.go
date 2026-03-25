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

func UsesLocalCodeVerification(provider string) bool {
	return ResolveProvider(provider, "") == ProviderAliyunDysmsapi
}

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

func providerOrUnknown(provider string) string {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}
