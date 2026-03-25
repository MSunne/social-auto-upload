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

func normalizeTemplateParam(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultTemplateParam
	}
	return strings.ReplaceAll(trimmed, "{{code}}", "##code##")
}

func newAliyunDypnsClient(cfg RegistrationConfig) (*dypnsapi.Client, error) {
	config := &openapi.Config{
		AccessKeyId:     dara.String(strings.TrimSpace(cfg.AccessKeyID)),
		AccessKeySecret: dara.String(strings.TrimSpace(cfg.AccessKeySecret)),
		Endpoint:        dara.String(strings.TrimSpace(cfg.Endpoint)),
	}
	return dypnsapi.NewClient(config)
}

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

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func int32Value(value *int32) int {
	if value == nil {
		return 0
	}
	return int(*value)
}
