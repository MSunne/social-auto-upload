package handlers

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/sms"
	"omnidrive_cloud/internal/store"
)

type AuthHandler struct {
	app *appstate.App
}

type registerRequest struct {
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	CountryCode string `json:"countryCode"`
	Name        string `json:"name"`
	Password    string `json:"password"`
	SMSCode     string `json:"smsCode"`
	PartnerCode string `json:"partnerCode"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type phoneLoginRequest struct {
	Phone       string `json:"phone"`
	CountryCode string `json:"countryCode"`
	Password    string `json:"password"`
}

type passwordLoginRequest struct {
	Account  string `json:"account"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type sendRegisterSMSCodeRequest struct {
	Phone       string `json:"phone"`
	CountryCode string `json:"countryCode"`
}

func NewAuthHandler(app *appstate.App) *AuthHandler {
	return &AuthHandler{app: app}
}

func normalizeUserPhone(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	var digits strings.Builder
	digits.Grow(len(trimmed))
	for _, char := range trimmed {
		if char >= '0' && char <= '9' {
			digits.WriteRune(char)
		}
	}

	normalized := digits.String()
	if strings.HasPrefix(normalized, "86") && len(normalized) == 13 {
		normalized = normalized[2:]
	}
	if len(normalized) == 11 && strings.HasPrefix(normalized, "1") {
		return normalized
	}
	return ""
}

func normalizeUserEmail(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	if _, err := mail.ParseAddress(trimmed); err != nil {
		return ""
	}
	return trimmed
}

func normalizeCountryDialCode(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "86"
	}

	var digits strings.Builder
	digits.Grow(len(trimmed))
	for _, char := range trimmed {
		if char >= '0' && char <= '9' {
			digits.WriteRune(char)
		}
	}
	return digits.String()
}

func resolveAuthIdentifiers(phoneInput string, emailInput string) (string, string, string) {
	rawPhone := strings.TrimSpace(phoneInput)
	rawEmail := strings.TrimSpace(emailInput)

	if rawPhone != "" {
		phone := normalizeUserPhone(rawPhone)
		if phone == "" {
			return "", "", "Invalid phone"
		}
		if rawEmail != "" {
			if candidate := normalizeUserPhone(rawEmail); candidate == phone {
				return phone, "", ""
			}
			email := normalizeUserEmail(rawEmail)
			if email == "" {
				return "", "", "Invalid email"
			}
			return phone, email, ""
		}
		return phone, "", ""
	}

	if rawEmail == "" {
		return "", "", "Phone or email is required"
	}
	if phone := normalizeUserPhone(rawEmail); phone != "" {
		return phone, "", ""
	}
	email := normalizeUserEmail(rawEmail)
	if email == "" {
		return "", "", "Invalid phone or email"
	}
	return "", email, ""
}

func normalizeRegisterSMSConfig(settings effectiveAdminSystemSettings) sms.RegistrationConfig {
	return sms.RegistrationConfig{
		Provider:        strings.TrimSpace(settings.SMSRegistration.Provider),
		Endpoint:        strings.TrimSpace(settings.SMSRegistration.Endpoint),
		AccessKeyID:     strings.TrimSpace(settings.SMSRegistration.AccessKeyID),
		AccessKeySecret: strings.TrimSpace(settings.SMSRegistration.AccessKeySecret),
		SignName:        strings.TrimSpace(settings.SMSRegistration.SignName),
		TemplateCode:    strings.TrimSpace(settings.SMSRegistration.TemplateCode),
		TemplateParam:   strings.TrimSpace(settings.SMSRegistration.TemplateParam),
		SchemeName:      strings.TrimSpace(settings.SMSRegistration.SchemeName),
		DefaultCountry:  strings.TrimSpace(settings.SMSRegistration.DefaultCountryCode),
		ValidMinutes:    settings.SMSRegistration.ValidMinutes,
		CooldownSeconds: settings.SMSRegistration.CooldownSeconds,
		DailyLimit:      settings.SMSRegistration.DailyLimit,
		CodeLength:      settings.SMSRegistration.CodeLength,
	}
}

func ensureRegisterSMSReady(settings effectiveAdminSystemSettings) (sms.RegistrationConfig, string) {
	config := normalizeRegisterSMSConfig(settings)
	if !settings.SMSRegistration.Enabled {
		return config, "短信注册暂未开启，请联系管理员"
	}
	if strings.TrimSpace(config.AccessKeyID) == "" || strings.TrimSpace(config.AccessKeySecret) == "" {
		return config, "短信配置尚未完成，请联系管理员"
	}
	if strings.TrimSpace(config.SignName) == "" || strings.TrimSpace(config.TemplateCode) == "" {
		return config, "短信模板配置尚未完成，请联系管理员"
	}
	if strings.TrimSpace(config.Provider) == "" {
		config.Provider = "aliyun_dypnsapi"
	}
	if strings.TrimSpace(config.Endpoint) == "" {
		config.Endpoint = "dypnsapi.aliyuncs.com"
	}
	if strings.TrimSpace(config.TemplateParam) == "" {
		config.TemplateParam = sms.DefaultTemplateParam
	}
	if strings.TrimSpace(config.DefaultCountry) == "" {
		config.DefaultCountry = "86"
	}
	if config.ValidMinutes <= 0 {
		config.ValidMinutes = 10
	}
	if config.CooldownSeconds <= 0 {
		config.CooldownSeconds = 60
	}
	if config.DailyLimit <= 0 {
		config.DailyLimit = 10
	}
	if config.CodeLength < 4 || config.CodeLength > 8 {
		config.CodeLength = 6
	}
	return config, ""
}

func mapSMSProviderError(err error) (int, string, map[string]any) {
	var providerErr *sms.ProviderError
	if !errors.As(err, &providerErr) {
		return http.StatusBadGateway, "短信发送失败，请稍后重试", nil
	}

	fields := map[string]any{}
	if code := strings.TrimSpace(providerErr.Code); code != "" {
		fields["code"] = code
	}
	if requestID := strings.TrimSpace(providerErr.RequestID); requestID != "" {
		fields["requestId"] = requestID
	}

	switch strings.TrimSpace(providerErr.Code) {
	case "MOBILE_NUMBER_ILLEGAL":
		return http.StatusBadRequest, "手机号格式不正确，请检查手机号和国家区号", fields
	case "BUSINESS_LIMIT_CONTROL":
		return http.StatusTooManyRequests, "该手机号今日验证码发送次数已达上限，请明天再试", fields
	case "FREQUENCY_FAIL":
		return http.StatusTooManyRequests, "验证码发送过于频繁，请稍后再试", fields
	case "INVALID_PARAMETERS":
		return http.StatusBadRequest, "短信模板或签名配置无效，请联系管理员检查短信配置", fields
	case "FUNCTION_NOT_OPENED":
		return http.StatusServiceUnavailable, "短信发送服务尚未开通，请联系管理员完成阿里云配置", fields
	default:
		if message := strings.TrimSpace(providerErr.Message); message != "" {
			return http.StatusBadGateway, "短信发送失败：" + message, fields
		}
		return http.StatusBadGateway, "短信发送失败，请稍后重试", fields
	}
}

func (h *AuthHandler) respondLoginSuccess(w http.ResponseWriter, userWithPassword *store.UserWithPassword) {
	token, err := h.app.Tokens.IssueToken(userWithPassword.User.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to issue token")
		return
	}
	h.app.Logger.Info("user login succeeded", "user_id", userWithPassword.User.ID, "email", userWithPassword.User.Email, "phone", userWithPassword.User.Phone)

	render.JSON(w, http.StatusOK, map[string]any{
		"accessToken": token,
		"tokenType":   "bearer",
		"user":        userWithPassword.User,
	})
}

func (h *AuthHandler) verifyLoginPassword(w http.ResponseWriter, userWithPassword *store.UserWithPassword, password string) bool {
	if userWithPassword == nil {
		render.Error(w, http.StatusUnauthorized, "Invalid credentials")
		return false
	}
	if err := h.app.Tokens.VerifyPassword(password, userWithPassword.PasswordHash); err != nil {
		render.Error(w, http.StatusUnauthorized, "Invalid credentials")
		return false
	}
	return true
}

func (h *AuthHandler) SendRegisterSMSCode(w http.ResponseWriter, r *http.Request) {
	var payload sendRegisterSMSCodeRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	phone := normalizeUserPhone(payload.Phone)
	if phone == "" {
		render.Error(w, http.StatusBadRequest, "请输入正确的手机号")
		return
	}

	settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load system config")
		return
	}
	smsConfig, configErr := ensureRegisterSMSReady(settings)
	if configErr != "" {
		render.Error(w, http.StatusServiceUnavailable, configErr)
		return
	}

	countryCode := normalizeCountryDialCode(payload.CountryCode)
	if strings.TrimSpace(payload.CountryCode) == "" {
		countryCode = normalizeCountryDialCode(smsConfig.DefaultCountry)
	}
	if countryCode == "" {
		render.Error(w, http.StatusBadRequest, "请输入正确的国家区号")
		return
	}

	existingUser, err := h.app.Store.GetUserByPhone(r.Context(), phone)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to query user")
		return
	}
	if existingUser != nil {
		render.Error(w, http.StatusConflict, "该手机号已注册，请直接登录")
		return
	}

	now := time.Now().UTC()
	latest, err := h.app.Store.GetLatestPhoneVerification(r.Context(), phone, countryCode, store.PhoneVerificationSceneRegister)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load verification status")
		return
	}
	if latest != nil {
		if latest.ExpiresAt.Before(now) && (latest.Status == "sent" || latest.Status == "verified") {
			_ = h.app.Store.MarkPhoneVerificationExpired(r.Context(), latest.ID)
		} else {
			elapsed := time.Since(latest.CreatedAt.UTC())
			if elapsed < time.Duration(smsConfig.CooldownSeconds)*time.Second {
				retryAfter := smsConfig.CooldownSeconds - int(elapsed/time.Second)
				if retryAfter < 1 {
					retryAfter = 1
				}
				render.ErrorWithFields(w, http.StatusTooManyRequests, "验证码发送过于频繁，请稍后再试", map[string]any{
					"code":              "FREQUENCY_FAIL",
					"retryAfterSeconds": retryAfter,
				})
				return
			}
		}
	}

	nowLocal := time.Now().In(time.Local)
	todayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, nowLocal.Location())
	countToday, err := h.app.Store.CountPhoneVerificationsSince(r.Context(), phone, countryCode, store.PhoneVerificationSceneRegister, todayStart)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to count verification requests")
		return
	}
	if countToday >= int64(smsConfig.DailyLimit) {
		render.ErrorWithFields(w, http.StatusTooManyRequests, "该手机号今日验证码发送次数已达上限，请明天再试", map[string]any{
			"code": "BUSINESS_LIMIT_CONTROL",
		})
		return
	}

	verificationID := uuid.NewString()
	record, err := h.app.Store.CreatePhoneVerification(r.Context(), store.CreatePhoneVerificationInput{
		ID:           verificationID,
		Scene:        store.PhoneVerificationSceneRegister,
		Phone:        phone,
		CountryCode:  countryCode,
		Provider:     smsConfig.Provider,
		TemplateCode: smsConfig.TemplateCode,
		ExpiresAt:    now.Add(time.Duration(smsConfig.ValidMinutes) * time.Minute),
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create verification session")
		return
	}

	sendResult, err := sms.SendRegistrationCode(smsConfig, phone, countryCode, verificationID)
	if err != nil {
		statusCode, message, fields := mapSMSProviderError(err)
		var providerErr *sms.ProviderError
		if errors.As(err, &providerErr) {
			_ = h.app.Store.MarkPhoneVerificationFailed(r.Context(), record.ID, providerErr.Code, providerErr.Message)
		} else {
			_ = h.app.Store.MarkPhoneVerificationFailed(r.Context(), record.ID, "", err.Error())
		}
		render.ErrorWithFields(w, statusCode, message, fields)
		return
	}

	if err := h.app.Store.MarkPhoneVerificationSent(r.Context(), record.ID, sendResult.RequestID, sendResult.BizID, sendResult.Code, sendResult.Message); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to store verification status")
		return
	}

	render.JSON(w, http.StatusCreated, map[string]any{
		"message":            "验证码已发送，请注意查收短信",
		"cooldownSeconds":    smsConfig.CooldownSeconds,
		"validMinutes":       smsConfig.ValidMinutes,
		"codeLength":         smsConfig.CodeLength,
		"defaultCountryCode": countryCode,
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var payload registerRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	phone, email, identifierErr := resolveAuthIdentifiers(payload.Phone, payload.Email)
	countryCode := normalizeCountryDialCode(payload.CountryCode)
	if identifierErr != "" {
		render.Error(w, http.StatusBadRequest, identifierErr)
		return
	}
	if phone != "" && countryCode == "" {
		render.Error(w, http.StatusBadRequest, "请输入正确的国家区号")
		return
	}
	if len(payload.Name) == 0 {
		render.Error(w, http.StatusBadRequest, "Name is required")
		return
	}
	if len(payload.Password) < 6 {
		render.Error(w, http.StatusBadRequest, "Password must be at least 6 characters")
		return
	}

	if phone != "" {
		existing, err := h.app.Store.GetUserByPhone(r.Context(), phone)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to query user")
			return
		}
		if existing != nil {
			render.Error(w, http.StatusConflict, "Phone already exists")
			return
		}
	}

	if email != "" {
		existing, err := h.app.Store.GetUserByEmail(r.Context(), email)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to query user")
			return
		}
		if existing != nil {
			render.Error(w, http.StatusConflict, "Email already exists")
			return
		}
	}

	if phone != "" {
		settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load system config")
			return
		}
		smsConfig, configErr := ensureRegisterSMSReady(settings)
		if configErr != "" {
			render.Error(w, http.StatusServiceUnavailable, configErr)
			return
		}
		if strings.TrimSpace(payload.CountryCode) == "" {
			countryCode = normalizeCountryDialCode(smsConfig.DefaultCountry)
		}
		if strings.TrimSpace(payload.SMSCode) == "" {
			render.Error(w, http.StatusBadRequest, "请输入短信验证码")
			return
		}

		verification, err := h.app.Store.GetLatestPhoneVerification(r.Context(), phone, countryCode, store.PhoneVerificationSceneRegister)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load verification status")
			return
		}
		if verification == nil {
			render.Error(w, http.StatusBadRequest, "请先获取短信验证码")
			return
		}
		if verification.Status == "consumed" {
			render.Error(w, http.StatusBadRequest, "该验证码已使用，请重新获取")
			return
		}
		if verification.ExpiresAt.Before(time.Now().UTC()) {
			_ = h.app.Store.MarkPhoneVerificationExpired(r.Context(), verification.ID)
			render.Error(w, http.StatusBadRequest, "验证码已过期，请重新获取")
			return
		}

		verifyResult, err := sms.VerifyRegistrationCode(smsConfig, phone, countryCode, payload.SMSCode, verification.ID)
		if err != nil {
			_ = h.app.Store.IncrementPhoneVerificationAttempt(r.Context(), verification.ID)
			statusCode, message, fields := mapSMSProviderError(err)
			render.ErrorWithFields(w, statusCode, message, fields)
			return
		}
		if !strings.EqualFold(strings.TrimSpace(verifyResult.VerifyResult), "PASS") {
			_ = h.app.Store.IncrementPhoneVerificationAttempt(r.Context(), verification.ID)
			render.Error(w, http.StatusBadRequest, "验证码不正确，请重新输入")
			return
		}
		if err := h.app.Store.MarkPhoneVerificationVerified(r.Context(), verification.ID, verifyResult.Code, verifyResult.Message); err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to update verification status")
			return
		}
	}

	passwordHash, err := h.app.Tokens.HashPassword(payload.Password)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	user, err := h.app.Store.CreateUserRegistration(r.Context(), store.CreateUserRegistrationInput{
		CreateUserInput: store.CreateUserInput{
			ID:           uuid.NewString(),
			Email:        email,
			Phone:        phone,
			Name:         payload.Name,
			PasswordHash: passwordHash,
		},
		PartnerCode: payload.PartnerCode,
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrPartnerCodeInvalid):
			render.Error(w, http.StatusBadRequest, "Invalid partner code")
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to create user")
		}
		return
	}

	if phone != "" {
		if verification, verificationErr := h.app.Store.GetLatestPhoneVerification(r.Context(), phone, countryCode, store.PhoneVerificationSceneRegister); verificationErr == nil && verification != nil {
			_ = h.app.Store.MarkPhoneVerificationConsumed(r.Context(), verification.ID)
		}
	}

	h.app.Logger.Info("user registered", "user_id", user.ID, "email", user.Email, "phone", user.Phone)
	render.JSON(w, http.StatusCreated, user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var payload loginRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	phone, email, identifierErr := resolveAuthIdentifiers(payload.Phone, payload.Email)
	if identifierErr != "" {
		render.Error(w, http.StatusBadRequest, identifierErr)
		return
	}

	var userWithPassword *store.UserWithPassword
	var err error
	if phone != "" {
		userWithPassword, err = h.app.Store.GetUserByPhone(r.Context(), phone)
	} else {
		userWithPassword, err = h.app.Store.GetUserByEmail(r.Context(), email)
	}
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to query user")
		return
	}
	if userWithPassword == nil {
		render.Error(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	if !h.verifyLoginPassword(w, userWithPassword, payload.Password) {
		return
	}
	h.respondLoginSuccess(w, userWithPassword)
}

func (h *AuthHandler) LoginWithPhone(w http.ResponseWriter, r *http.Request) {
	var payload phoneLoginRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	phone := normalizeUserPhone(payload.Phone)
	if phone == "" {
		render.Error(w, http.StatusBadRequest, "请输入正确的手机号")
		return
	}
	if strings.TrimSpace(payload.Password) == "" {
		render.Error(w, http.StatusBadRequest, "Password is required")
		return
	}

	userWithPassword, err := h.app.Store.GetUserByPhone(r.Context(), phone)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to query user")
		return
	}
	if !h.verifyLoginPassword(w, userWithPassword, payload.Password) {
		return
	}
	h.respondLoginSuccess(w, userWithPassword)
}

func (h *AuthHandler) LoginWithPassword(w http.ResponseWriter, r *http.Request) {
	var payload passwordLoginRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	account := strings.TrimSpace(payload.Account)
	if account == "" {
		account = strings.TrimSpace(payload.Email)
	}
	if account == "" {
		render.Error(w, http.StatusBadRequest, "Account is required")
		return
	}
	if strings.TrimSpace(payload.Password) == "" {
		render.Error(w, http.StatusBadRequest, "Password is required")
		return
	}

	email := normalizeUserEmail(account)
	if email == "" {
		render.Error(w, http.StatusBadRequest, "请输入正确的账户邮箱")
		return
	}

	userWithPassword, err := h.app.Store.GetUserByEmail(r.Context(), email)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to query user")
		return
	}
	if !h.verifyLoginPassword(w, userWithPassword, payload.Password) {
		return
	}
	h.respondLoginSuccess(w, userWithPassword)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	if user == nil {
		render.Error(w, http.StatusUnauthorized, "User not found")
		return
	}
	render.JSON(w, http.StatusOK, user)
}
