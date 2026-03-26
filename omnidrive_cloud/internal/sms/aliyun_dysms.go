package sms

import (
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	"github.com/alibabacloud-go/tea/dara"
)

func newAliyunDysmsClient(cfg RegistrationConfig) (*dysmsapi.Client, error) {
	config := &openapi.Config{
		AccessKeyId:     dara.String(strings.TrimSpace(cfg.AccessKeyID)),
		AccessKeySecret: dara.String(strings.TrimSpace(cfg.AccessKeySecret)),
		Endpoint:        dara.String(strings.TrimSpace(cfg.Endpoint)),
	}
	return dysmsapi.NewClient(config)
}

func sendAliyunDysmsRegistrationCode(cfg RegistrationConfig, phone string, outID string, verificationCode string) (*SendCodeResult, error) {
	client, err := newAliyunDysmsClient(cfg)
	if err != nil {
		return nil, err
	}

	request := &dysmsapi.SendSmsRequest{
		PhoneNumbers:  dara.String(strings.TrimSpace(phone)),
		SignName:      dara.String(strings.TrimSpace(cfg.SignName)),
		TemplateCode:  dara.String(strings.TrimSpace(cfg.TemplateCode)),
		TemplateParam: dara.String(BuildTemplateParam(cfg.TemplateParam, verificationCode)),
		OutId:         dara.String(strings.TrimSpace(outID)),
	}

	response, err := client.SendSms(request)
	if err != nil {
		return nil, wrapAliyunError(ProviderAliyunDysmsapi, err)
	}
	if response == nil || response.Body == nil {
		return nil, &ProviderError{
			Provider: ProviderAliyunDysmsapi,
			Message:  "empty sms provider response",
		}
	}
	body := response.Body
	if !strings.EqualFold(stringValue(body.Code), "OK") {
		return nil, &ProviderError{
			Provider:   ProviderAliyunDysmsapi,
			Code:       stringValue(body.Code),
			Message:    stringValue(body.Message),
			StatusCode: 200,
			RequestID:  stringValue(body.RequestId),
		}
	}

	return &SendCodeResult{
		RequestID: stringValue(body.RequestId),
		BizID:     stringValue(body.BizId),
		Provider:  ProviderAliyunDysmsapi,
		Code:      stringValue(body.Code),
		Message:   stringValue(body.Message),
	}, nil
}

func sendAliyunDysmsTemplateSMS(cfg RegistrationConfig, phone string, outID string, signName string, templateCode string, templateParam string) (*SendCodeResult, error) {
	client, err := newAliyunDysmsClient(cfg)
	if err != nil {
		return nil, err
	}

	request := &dysmsapi.SendSmsRequest{
		PhoneNumbers:  dara.String(strings.TrimSpace(phone)),
		SignName:      dara.String(strings.TrimSpace(signName)),
		TemplateCode:  dara.String(strings.TrimSpace(templateCode)),
		TemplateParam: dara.String(strings.TrimSpace(templateParam)),
		OutId:         dara.String(strings.TrimSpace(outID)),
	}

	response, err := client.SendSms(request)
	if err != nil {
		return nil, wrapAliyunError(ProviderAliyunDysmsapi, err)
	}
	if response == nil || response.Body == nil {
		return nil, &ProviderError{
			Provider: ProviderAliyunDysmsapi,
			Message:  "empty sms provider response",
		}
	}
	body := response.Body
	if !strings.EqualFold(stringValue(body.Code), "OK") {
		return nil, &ProviderError{
			Provider:   ProviderAliyunDysmsapi,
			Code:       stringValue(body.Code),
			Message:    stringValue(body.Message),
			StatusCode: 200,
			RequestID:  stringValue(body.RequestId),
		}
	}

	return &SendCodeResult{
		RequestID: stringValue(body.RequestId),
		BizID:     stringValue(body.BizId),
		Provider:  ProviderAliyunDysmsapi,
		Code:      stringValue(body.Code),
		Message:   stringValue(body.Message),
	}, nil
}
