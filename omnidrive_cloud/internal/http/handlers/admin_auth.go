package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
)

type AdminAuthHandler struct {
	app *appstate.App
}

type adminLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewAdminAuthHandler(app *appstate.App) *AdminAuthHandler {
	return &AdminAuthHandler{app: app}
}

func (h *AdminAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var payload adminLoginRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	email := strings.TrimSpace(strings.ToLower(payload.Email))
	password := payload.Password
	configuredEmail := strings.TrimSpace(strings.ToLower(h.app.Config.AdminEmail))
	configuredPassword := h.app.Config.AdminPassword

	if subtle.ConstantTimeCompare([]byte(email), []byte(configuredEmail)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(configuredPassword)) != 1 {
		render.Error(w, http.StatusUnauthorized, "Invalid admin credentials")
		return
	}

	token, err := h.app.AdminTokens.IssueToken(appstate.BootstrapAdminID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to issue admin token")
		return
	}

	render.JSON(w, http.StatusOK, map[string]any{
		"accessToken": token,
		"tokenType":   "bearer",
		"admin":       h.app.BootstrapAdminIdentity(),
	})
}

func (h *AdminAuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	admin := httpcontext.CurrentAdmin(r.Context())
	if admin == nil {
		render.Error(w, http.StatusUnauthorized, "Admin not found")
		return
	}
	render.JSON(w, http.StatusOK, admin)
}

func (h *AdminAuthHandler) ListAdmins(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	admin := h.app.BootstrapAdminIdentity()
	renderAdminList(w, page, 1, []domain.AdminIdentity{*admin}, map[string]any{
		"authMode":  "bootstrap_env",
		"roleCount": 1,
	}, map[string]any{
		"authMode": "bootstrap_env",
	})
}

func (h *AdminAuthHandler) SystemConfig(w http.ResponseWriter, r *http.Request) {
	configPayload := domain.AdminSystemConfig{
		AuthMode:        "bootstrap_env",
		AdminEmail:      h.app.Config.AdminEmail,
		S3Configured:    h.app.Config.S3Bucket != "" && h.app.Config.S3Endpoint != "" && h.app.Config.S3AccessKey != "" && h.app.Config.S3SecretKey != "",
		S3Endpoint:      h.app.Config.S3Endpoint,
		S3Bucket:        h.app.Config.S3Bucket,
		AIWorkerEnabled: h.app.Config.AIWorkerEnabled,
		PaymentChannels: []string{"alipay", "wechatpay", "manual_cs"},
		Notes: []string{
			"当前管理端鉴权为 bootstrap env 模式，后续会升级为真正的 admin_users / roles / permissions 模型。",
			"当前客服充值审核主链已经可联调，分销和提现模块仍处于 schema_pending 阶段。",
		},
	}
	render.JSON(w, http.StatusOK, configPayload)
}
