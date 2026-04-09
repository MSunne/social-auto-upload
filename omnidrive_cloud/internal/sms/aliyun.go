package sms

import (
	"errors"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	dypnsapi "github.com/alibabacloud-go/dypnsapi-20170525/v3/client"
	"github.com/alibabacloud-go/tea/dara"
	teasdk "github.com/alibabacloud-go/tea/tea"
)

const DefaultTemplateParam = `{"code":"##code##"}`

type RegistrationConfig struct {
	Provider        string
	Endpoint        string
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	TemplateCode    string
	TemplateParam   string
	SchemeName      string
	DefaultCountry  string
	ValidMinutes    int
	CooldownSeconds int
	DailyLimit      int
	CodeLength      int
}

type SendCodeResult struct {
	RequestID string
	BizID     string
	Provider  string
	Code      string
	Message   string
}

type VerifyCodeResult struct {
	RequestID    string
	Provider     string
	Code         string
	Message      string
	VerifyResult string
}

type ProviderError struct {
	Provider   string
	Code       string
	Message    string
	StatusCode int
	RequestID  string
}

// 返回供应方错误的可读错误消息，供日志记录和错误透传统一使用。
func (e *ProviderError) Error() string {
	if strings.TrimSpace(e.Code) == "" && strings.TrimSpace(e.Message) == "" {
		return "sms provider request failed"
	}
	if strings.TrimSpace(e.Code) == "" {
		return strings.TrimSpace(e.Message)
	}
	if strings.TrimSpace(e.Message) == "" {
		return strings.TrimSpace(e.Code)
	}
	return strings.TrimSpace(e.Code) + ": " + strings.TrimSpace(e.Message)
}

// 规范化TemplateParam，统一阿里云短信链路的输入格式和后续处理行为。
func normalizeTemplateParam(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultTemplateParam
	}
	return strings.ReplaceAll(trimmed, "{{code}}", "##code##")
}

// 创建AliyunDypns客户端相关实例，组装运行所需依赖并返回给上层流程复用。
func newAliyunDypnsClient(cfg RegistrationConfig) (*dypnsapi.Client, error) {
	config := &openapi.Config{
		AccessKeyId:     dara.String(strings.TrimSpace(cfg.AccessKeyID)),
		AccessKeySecret: dara.String(strings.TrimSpace(cfg.AccessKeySecret)),
		Endpoint:        dara.String(strings.TrimSpace(cfg.Endpoint)),
	}
	return dypnsapi.NewClient(config)
}

// 发送AliyunDypnsRegistration编码，把当前业务消息下发到外部通道。
func sendAliyunDypnsRegistrationCode(cfg RegistrationConfig, phone string, countryCode string, outID string) (*SendCodeResult, error) {
	client, err := newAliyunDypnsClient(cfg)
	if err != nil {
		return nil, err
	}

	request := &dypnsapi.SendSmsVerifyCodeRequest{
		PhoneNumber:      dara.String(strings.TrimSpace(phone)),
		CountryCode:      dara.String(strings.TrimSpace(countryCode)),
		SignName:         dara.String(strings.TrimSpace(cfg.SignName)),
		TemplateCode:     dara.String(strings.TrimSpace(cfg.TemplateCode)),
		TemplateParam:    dara.String(normalizeTemplateParam(cfg.TemplateParam)),
		ValidTime:        dara.Int64(int64(cfg.ValidMinutes * 60)),
		Interval:         dara.Int64(int64(cfg.CooldownSeconds)),
		CodeLength:       dara.Int64(int64(cfg.CodeLength)),
		CodeType:         dara.Int64(1),
		DuplicatePolicy:  dara.Int64(1),
		ReturnVerifyCode: dara.Bool(false),
		OutId:            dara.String(strings.TrimSpace(outID)),
	}
	if strings.TrimSpace(cfg.SchemeName) != "" {
		request.SchemeName = dara.String(strings.TrimSpace(cfg.SchemeName))
	}

	response, err := client.SendSmsVerifyCode(request)
	if err != nil {
		return nil, wrapAliyunError("aliyun_dypnsapi", err)
	}
	if response == nil || response.Body == nil {
		return nil, &ProviderError{
			Provider: "aliyun_dypnsapi",
			Message:  "empty sms provider response",
		}
	}
	body := response.Body
	if !boolValue(body.Success) || !strings.EqualFold(stringValue(body.Code), "OK") {
		return nil, &ProviderError{
			Provider:   "aliyun_dypnsapi",
			Code:       stringValue(body.Code),
			Message:    stringValue(body.Message),
			StatusCode: int32Value(response.StatusCode),
			RequestID:  stringValue(body.RequestId),
		}
	}

	result := &SendCodeResult{
		RequestID: stringValue(body.RequestId),
		Provider:  "aliyun_dypnsapi",
		Code:      stringValue(body.Code),
		Message:   stringValue(body.Message),
	}
	if body.Model != nil {
		result.BizID = stringValue(body.Model.BizId)
		if result.RequestID == "" {
			result.RequestID = stringValue(body.Model.RequestId)
		}
	}
	return result, nil
}

// 处理verifyAliyunDypnsRegistration编码相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func verifyAliyunDypnsRegistrationCode(cfg RegistrationConfig, phone string, countryCode string, code string, outID string) (*VerifyCodeResult, error) {
	client, err := newAliyunDypnsClient(cfg)
	if err != nil {
		return nil, err
	}

	request := &dypnsapi.CheckSmsVerifyCodeRequest{
		PhoneNumber: dara.String(strings.TrimSpace(phone)),
		CountryCode: dara.String(strings.TrimSpace(countryCode)),
		VerifyCode:  dara.String(strings.TrimSpace(code)),
		OutId:       dara.String(strings.TrimSpace(outID)),
	}
	if strings.TrimSpace(cfg.SchemeName) != "" {
		request.SchemeName = dara.String(strings.TrimSpace(cfg.SchemeName))
	}

	response, err := client.CheckSmsVerifyCode(request)
	if err != nil {
		return nil, wrapAliyunError("aliyun_dypnsapi", err)
	}
	if response == nil || response.Body == nil {
		return nil, &ProviderError{
			Provider: "aliyun_dypnsapi",
			Message:  "empty sms provider response",
		}
	}
	body := response.Body
	if !boolValue(body.Success) || !strings.EqualFold(stringValue(body.Code), "OK") {
		return nil, &ProviderError{
			Provider:   "aliyun_dypnsapi",
			Code:       stringValue(body.Code),
			Message:    stringValue(body.Message),
			StatusCode: 200,
		}
	}

	result := &VerifyCodeResult{
		Provider: "aliyun_dypnsapi",
		Code:     stringValue(body.Code),
		Message:  stringValue(body.Message),
	}
	if body.Model != nil {
		result.RequestID = stringValue(body.Model.OutId)
		result.VerifyResult = stringValue(body.Model.VerifyResult)
	}
	return result, nil
}

// 处理wrapAliyun错误相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func wrapAliyunError(provider string, err error) error {
	var sdkErr *teasdk.SDKError
	if !errors.As(err, &sdkErr) {
		return err
	}
	statusCode := 0
	if sdkErr.StatusCode != nil {
		statusCode = *sdkErr.StatusCode
	}
	return &ProviderError{
		Provider:   provider,
		Code:       stringValue(sdkErr.Code),
		Message:    stringValue(sdkErr.Message),
		StatusCode: statusCode,
	}
}

// 处理string值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// 处理bool值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func boolValue(value *bool) bool {
	return value != nil && *value
}

// 处理int32值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func int32Value(value *int32) int {
	if value == nil {
		return 0
	}
	return int(*value)
}
