package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AccountHandler struct {
	app *appstate.App
}

type createRemoteLoginRequest struct {
	DeviceID    string `json:"deviceId"`
	Platform    string `json:"platform"`
	AccountName string `json:"accountName"`
}

type createLoginActionRequest struct {
	ActionType string      `json:"actionType"`
	Payload    interface{} `json:"payload"`
}

func NewAccountHandler(app *appstate.App) *AccountHandler {
	return &AccountHandler{app: app}
}

func (h *AccountHandler) List(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	deviceID := strings.TrimSpace(r.URL.Query().Get("deviceId"))

	items, err := h.app.Store.ListAccountsByOwner(r.Context(), user.ID, deviceID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load accounts")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AccountHandler) Detail(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}

	account, err := h.app.Store.GetOwnedAccountByID(r.Context(), accountID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load account")
		return
	}
	if account == nil {
		render.Error(w, http.StatusNotFound, "Account not found")
		return
	}
	render.JSON(w, http.StatusOK, account)
}

func (h *AccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}

	deleted, err := h.app.Store.DeleteOwnedAccount(r.Context(), accountID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to delete account")
		return
	}
	if !deleted {
		render.Error(w, http.StatusNotFound, "Account not found")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "account",
		ResourceID:   &accountID,
		Action:       "delete",
		Title:        "删除平台账号镜像",
		Source:       "accounts",
		Status:       "success",
		Message:      auditStringPtr("云端账号镜像已删除"),
	})
	render.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *AccountHandler) Validate(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}

	account, err := h.app.Store.GetOwnedAccountByID(r.Context(), accountID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load account")
		return
	}
	if account == nil {
		render.Error(w, http.StatusNotFound, "Account not found")
		return
	}

	message := "等待本地 OmniBull 重新验证账号"
	session, err := h.app.Store.CreateLoginSession(r.Context(), store.CreateLoginSessionInput{
		ID:          uuid.NewString(),
		DeviceID:    account.DeviceID,
		UserID:      user.ID,
		Platform:    account.Platform,
		AccountName: account.AccountName,
		Status:      "pending",
		Message:     &message,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create validation session")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "login_session",
		ResourceID:   &session.ID,
		Action:       "validate",
		Title:        "发起账号重新验证",
		Source:       account.Platform,
		Status:       session.Status,
		Message:      session.Message,
		Payload: mustJSONBytes(map[string]any{
			"deviceId":    account.DeviceID,
			"accountId":   account.ID,
			"accountName": account.AccountName,
		}),
	})

	render.JSON(w, http.StatusCreated, session)
}

func (h *AccountHandler) CreateRemoteLogin(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	var payload createRemoteLoginRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.DeviceID = strings.TrimSpace(payload.DeviceID)
	payload.Platform = strings.TrimSpace(payload.Platform)
	payload.AccountName = strings.TrimSpace(payload.AccountName)
	if payload.DeviceID == "" || payload.Platform == "" || payload.AccountName == "" {
		render.Error(w, http.StatusBadRequest, "deviceId, platform, and accountName are required")
		return
	}

	device, err := h.app.Store.GetOwnedDevice(r.Context(), payload.DeviceID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device")
		return
	}
	if device == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}
	if !device.IsEnabled {
		render.Error(w, http.StatusConflict, "Device is disabled")
		return
	}

	message := "等待本地 OmniBull 拉起登录流程"
	session, err := h.app.Store.CreateLoginSession(r.Context(), store.CreateLoginSessionInput{
		ID:          uuid.NewString(),
		DeviceID:    payload.DeviceID,
		UserID:      user.ID,
		Platform:    payload.Platform,
		AccountName: payload.AccountName,
		Status:      "pending",
		Message:     &message,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create remote login session")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "login_session",
		ResourceID:   &session.ID,
		Action:       "create",
		Title:        "发起远程登录",
		Source:       payload.Platform,
		Status:       session.Status,
		Message:      session.Message,
		Payload: mustJSONBytes(map[string]any{
			"deviceId":    payload.DeviceID,
			"accountName": payload.AccountName,
		}),
	})

	render.JSON(w, http.StatusCreated, session)
}

func (h *AccountHandler) GetLoginSession(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	if sessionID == "" {
		render.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	session, err := h.app.Store.GetOwnedLoginSession(r.Context(), sessionID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load login session")
		return
	}
	if session == nil {
		render.Error(w, http.StatusNotFound, "Login session not found")
		return
	}
	render.JSON(w, http.StatusOK, session)
}

func (h *AccountHandler) CreateLoginAction(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	if sessionID == "" {
		render.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	session, err := h.app.Store.GetOwnedLoginSession(r.Context(), sessionID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load login session")
		return
	}
	if session == nil {
		render.Error(w, http.StatusNotFound, "Login session not found")
		return
	}

	var payload createLoginActionRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.ActionType = strings.TrimSpace(payload.ActionType)
	if payload.ActionType == "" {
		render.Error(w, http.StatusBadRequest, "actionType is required")
		return
	}

	var payloadBytes []byte
	if payload.Payload != nil {
		payloadBytes, err = json.Marshal(payload.Payload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "payload must be valid json")
			return
		}
	}

	action, err := h.app.Store.CreateLoginAction(r.Context(), store.CreateLoginActionInput{
		ID:         uuid.NewString(),
		SessionID:  sessionID,
		ActionType: payload.ActionType,
		Payload:    payloadBytes,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create login action")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "login_session_action",
		ResourceID:   &action.ID,
		Action:       payload.ActionType,
		Title:        "提交登录会话动作",
		Source:       session.Platform,
		Status:       action.Status,
		Message:      auditStringPtr("用户在云端提交了验证动作"),
		Payload:      payloadBytes,
	})

	render.JSON(w, http.StatusCreated, action)
}
