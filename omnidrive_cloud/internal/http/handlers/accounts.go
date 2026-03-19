package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
	"omnidrive_cloud/internal/workflow"
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

type createAccountSkillRunRequest struct {
	SkillID   string  `json:"skillId"`
	PublishAt *string `json:"publishAt"`
}

func isLoginCancelAction(actionType string) bool {
	switch strings.TrimSpace(actionType) {
	case "cancel_session", "cancel_login":
		return true
	default:
		return false
	}
}

func NewAccountHandler(app *appstate.App) *AccountHandler {
	return &AccountHandler{app: app}
}

func findReusableLoginSession(ctx context.Context, store *store.Store, ownerUserID string, deviceID string, platform string, accountName string) (*domain.LoginSession, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	deviceID = strings.TrimSpace(deviceID)
	platform = strings.TrimSpace(platform)
	accountName = strings.TrimSpace(accountName)
	if ownerUserID == "" || deviceID == "" || platform == "" || accountName == "" {
		return nil, nil
	}

	sessions, err := store.ListLoginSessionsByAccountTarget(ctx, ownerUserID, deviceID, platform, accountName, 1)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	session := &sessions[0]
	status := strings.TrimSpace(session.Status)
	age := time.Since(session.UpdatedAt)
	switch status {
	case "pending", "running":
		if age <= 2*time.Minute {
			return session, nil
		}
	}
	return nil, nil
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

func (h *AccountHandler) Workspace(w http.ResponseWriter, r *http.Request) {
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

	recentTasks, err := h.app.Store.ListPublishTasksByAccountTarget(r.Context(), user.ID, account.DeviceID, account.Platform, account.AccountName, 8)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load account tasks")
		return
	}

	activeLoginSessions, err := h.app.Store.ListLoginSessionsByAccountTarget(r.Context(), user.ID, account.DeviceID, account.Platform, account.AccountName, 6)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load account login sessions")
		return
	}

	render.JSON(w, http.StatusOK, domain.PlatformAccountWorkspace{
		Account:             *account,
		RecentTasks:         recentTasks,
		ActiveLoginSessions: activeLoginSessions,
	})
}

func (h *AccountHandler) CreateSkillRun(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}

	settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI configuration")
		return
	}
	if !settings.AIWorkerEnabled {
		render.Error(w, http.StatusServiceUnavailable, "AI worker is currently disabled by admin configuration")
		return
	}

	var payload createAccountSkillRunRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
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

	skillID := strings.TrimSpace(payload.SkillID)
	if skillID == "" {
		render.Error(w, http.StatusBadRequest, "skillId is required")
		return
	}
	skill, err := h.app.Store.GetOwnedSkillByID(r.Context(), skillID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill")
		return
	}
	if skill == nil {
		render.Error(w, http.StatusNotFound, "Skill not found")
		return
	}
	if !skill.IsEnabled {
		render.Error(w, http.StatusConflict, "Skill is disabled")
		return
	}
	if skill.DeviceID == nil || strings.TrimSpace(*skill.DeviceID) != account.DeviceID {
		render.Error(w, http.StatusConflict, "Skill does not belong to the selected OmniBull device")
		return
	}

	jobType, ok := workflow.MapSkillOutputTypeToJobType(skill.OutputType)
	if !ok {
		render.Error(w, http.StatusConflict, "Skill outputType is not supported")
		return
	}
	model, err := h.app.Store.GetAIModelByName(r.Context(), strings.TrimSpace(skill.ModelName))
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to validate AI model")
		return
	}
	if model == nil || !model.IsEnabled {
		render.Error(w, http.StatusConflict, "Skill model is disabled or missing")
		return
	}
	if model.Category != jobType {
		render.Error(w, http.StatusConflict, "Skill model category does not match skill output type")
		return
	}

	if payload.PublishAt == nil || strings.TrimSpace(*payload.PublishAt) == "" {
		render.Error(w, http.StatusBadRequest, "publishAt is required")
		return
	}
	publishAt, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*payload.PublishAt))
	if parseErr != nil {
		render.Error(w, http.StatusBadRequest, "publishAt must be RFC3339")
		return
	}
	publishAt = publishAt.UTC()
	generateAt := workflow.ScheduledSkillGenerationTime(publishAt)

	inputPayload, err := workflow.BuildSkillAIJobPayload(
		r.Context(),
		h.app,
		*skill,
		generateAt,
		publishAt,
		jobType,
		[]workflow.PublishTarget{{
			AccountID:   &account.ID,
			Platform:    account.Platform,
			AccountName: account.AccountName,
		}},
	)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to prepare account skill run payload")
		return
	}

	status := "scheduled"
	message := "等待定时生成"
	if !generateAt.After(time.Now().UTC()) {
		status = "queued"
		message = "等待云端生成"
	}

	prompt := workflow.BuildSkillJobPrompt(*skill)
	job, err := h.app.Store.CreateAIJob(r.Context(), store.CreateAIJobInput{
		ID:           uuid.NewString(),
		OwnerUserID:  user.ID,
		DeviceID:     &account.DeviceID,
		SkillID:      &skill.ID,
		Source:       "account_skill_binding",
		LocalTaskID:  nil,
		JobType:      jobType,
		ModelName:    strings.TrimSpace(skill.ModelName),
		Prompt:       &prompt,
		InputPayload: inputPayload,
		Status:       status,
		Message:      &message,
		RunAt:        &generateAt,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create account skill run")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "ai_job",
		ResourceID:   &job.ID,
		Action:       "create",
		Title:        "账号绑定技能任务",
		Source:       account.Platform,
		Status:       job.Status,
		Message:      auditStringPtr("已为账号创建专属技能生成任务"),
		Payload: mustJSONBytes(map[string]any{
			"accountId":   account.ID,
			"accountName": account.AccountName,
			"deviceId":    account.DeviceID,
			"skillId":     skill.ID,
			"publishAt":   publishAt,
			"generateAt":  generateAt,
			"jobType":     jobType,
			"source":      job.Source,
		}),
	})

	render.JSON(w, http.StatusCreated, job)
}

func (h *AccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}

	taskCount, activeLoginSessionCount, err := h.app.Store.GetAccountUsageSummary(r.Context(), accountID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect account usage")
		return
	}
	if taskCount > 0 || activeLoginSessionCount > 0 {
		render.JSON(w, http.StatusConflict, map[string]any{
			"error": "Account is still referenced by tasks or active login sessions",
			"usage": map[string]any{
				"publishTaskCount":        taskCount,
				"activeLoginSessionCount": activeLoginSessionCount,
			},
		})
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

	existingSession, err := findReusableLoginSession(r.Context(), h.app.Store, user.ID, account.DeviceID, account.Platform, account.AccountName)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect active validation session")
		return
	}
	if existingSession != nil {
		render.JSON(w, http.StatusOK, existingSession)
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

	existingSession, err := findReusableLoginSession(r.Context(), h.app.Store, user.ID, payload.DeviceID, payload.Platform, payload.AccountName)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect active login session")
		return
	}
	if existingSession != nil {
		render.JSON(w, http.StatusOK, existingSession)
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

	if isLoginCancelAction(payload.ActionType) {
		cancelMessage := "登录会话已取消，等待本地 SAU 停止当前登录流程"
		if _, err := h.app.Store.UpdateLoginSessionEvent(r.Context(), sessionID, store.LoginEventInput{
			Status:  "cancelled",
			Message: &cancelMessage,
		}); err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to cancel login session")
			return
		}
	}

	render.JSON(w, http.StatusCreated, action)
}
