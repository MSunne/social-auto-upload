package handlers

import (
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
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
	if _, err := mail.ParseAddress(email); err != nil {
		render.Error(w, http.StatusBadRequest, "Invalid admin email")
		return
	}

	adminWithPassword, err := h.app.Store.GetAdminUserByEmail(r.Context(), email)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to query admin user")
		return
	}
	if adminWithPassword == nil || !adminWithPassword.Admin.IsActive {
		render.Error(w, http.StatusUnauthorized, "Invalid admin credentials")
		return
	}
	if err := h.app.AdminTokens.VerifyPassword(payload.Password, adminWithPassword.PasswordHash); err != nil {
		render.Error(w, http.StatusUnauthorized, "Invalid admin credentials")
		return
	}

	sessionID := uuid.NewString()
	expiresAt := time.Now().UTC().Add(time.Duration(h.app.Config.AdminAccessTokenExpireMinutes) * time.Minute)
	if err := h.app.Store.CreateAdminSession(r.Context(), store.CreateAdminSessionInput{
		ID:          sessionID,
		AdminUserID: adminWithPassword.Admin.ID,
		IPAddress:   headerStringPtr(r.Header.Get("X-Forwarded-For")),
		UserAgent:   headerStringPtr(r.UserAgent()),
		ExpiresAt:   expiresAt,
	}); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create admin session")
		return
	}

	token, err := h.app.AdminTokens.IssueToken(appstate.BuildAdminSessionSubject(sessionID))
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to issue admin token")
		return
	}

	admin, err := h.app.Store.GetAdminIdentityBySessionID(r.Context(), sessionID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin identity")
		return
	}
	if admin == nil {
		render.Error(w, http.StatusUnauthorized, "Admin not found")
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "admin_session",
		ResourceID:   stringPtr(sessionID),
		Action:       "login",
		Title:        "管理员登录",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("管理员登录成功"),
		Payload: mustJSONBytes(map[string]any{
			"authMode":  admin.AuthMode,
			"roleIds":   admin.RoleIDs,
			"sessionId": sessionID,
		}),
	})

	render.JSON(w, http.StatusOK, map[string]any{
		"accessToken": token,
		"tokenType":   "bearer",
		"admin":       admin,
	})
}

func (h *AdminAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	admin := httpcontext.CurrentAdmin(r.Context())
	if admin == nil || strings.TrimSpace(admin.SessionID) == "" {
		render.Error(w, http.StatusUnauthorized, "Admin session not found")
		return
	}

	if err := h.app.Store.RevokeAdminSession(r.Context(), admin.SessionID); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to revoke admin session")
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "admin_session",
		ResourceID:   stringPtr(admin.SessionID),
		Action:       "logout",
		Title:        "管理员登出",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("管理员已登出"),
	})

	render.JSON(w, http.StatusOK, map[string]any{
		"success": true,
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

func (h *AdminAuthHandler) SystemConfig(w http.ResponseWriter, r *http.Request) {
	configPayload, err := h.buildAdminSystemConfig(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load system config")
		return
	}
	render.JSON(w, http.StatusOK, configPayload)
}

func headerStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
