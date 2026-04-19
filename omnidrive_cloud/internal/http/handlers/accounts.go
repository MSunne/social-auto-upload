package handlers

import (
	"context"
	"fmt"
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
	SkillID       string                              `json:"skillId"`
	PublishAt     *string                             `json:"publishAt"`
	ScheduleSlots []createAccountSkillRunScheduleSlot `json:"scheduleSlots"`
}

type createAccountSkillRunScheduleSlot struct {
	ScheduleKey           *string `json:"scheduleKey"`
	TimeOfDay             string  `json:"timeOfDay"`
	RepeatDaily           bool    `json:"repeatDaily"`
	GenerationLeadMinutes int     `json:"generationLeadMinutes"`
}

const (
	reusablePendingLoginSessionWindow      = 2 * time.Minute
	reusableVerificationLoginSessionWindow = 20 * time.Minute
)

// 判断是否属于Login取消动作，供当前链路选择后续处理策略。
func isLoginCancelAction(actionType string) bool {
	switch strings.TrimSpace(actionType) {
	case "cancel_session", "cancel_login":
		return true
	default:
		return false
	}
}

// 创建账号Handler相关实例，组装运行所需依赖并返回给上层流程复用。
func NewAccountHandler(app *appstate.App) *AccountHandler {
	return &AccountHandler{app: app}
}

// 处理findReusable登录会话相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func findReusableLoginSession(ctx context.Context, store *store.Store, ownerUserID string, deviceID string, platform string, accountName string) (*domain.LoginSession, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	deviceID = strings.TrimSpace(deviceID)
	platform = strings.TrimSpace(platform)
	accountName = strings.TrimSpace(accountName)
	if ownerUserID == "" || deviceID == "" || platform == "" || accountName == "" {
		return nil, nil
	}

	sessions, err := store.ListLoginSessionsByAccountTarget(ctx, ownerUserID, deviceID, platform, accountName, 6)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for i := range sessions {
		session := &sessions[i]
		if isReusableLoginSession(session, now) {
			return session, nil
		}
	}
	return nil, nil
}

// 判断是否属于Reusable登录会话，供当前链路选择后续处理策略。
func isReusableLoginSession(session *domain.LoginSession, now time.Time) bool {
	if session == nil {
		return false
	}
	status := strings.TrimSpace(session.Status)
	age := now.Sub(session.UpdatedAt)
	switch status {
	case "pending", "running":
		return age <= reusablePendingLoginSessionWindow
	case "verification_required":
		return age <= reusableVerificationLoginSessionWindow
	default:
		return false
	}
}

// 处理账号列表接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理账号详情接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理账号工作区接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

	render.JSON(w, http.StatusOK, domain.PlatformAccountWorkspace{
		Account:     *account,
		RecentTasks: recentTasks,
	})
}

// 处理账号创建技能运行接口，解析请求参数并调用应用状态或存储层完成业务动作。
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
	device, err := h.app.Store.GetOwnedDevice(r.Context(), account.DeviceID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to validate device")
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

	slots := make([]workflow.AccountSkillScheduleConfig, 0, len(payload.ScheduleSlots))
	if len(payload.ScheduleSlots) > 0 {
		for _, slot := range payload.ScheduleSlots {
			normalizedTimeOfDay, normalizeErr := workflow.NormalizeAccountSkillTimeOfDay(slot.TimeOfDay)
			if normalizeErr != nil {
				render.Error(w, http.StatusBadRequest, normalizeErr.Error())
				return
			}
			scheduleConfig := workflow.AccountSkillScheduleConfig{
				ScheduleKey:           strings.TrimSpace(stringValue(slot.ScheduleKey)),
				TimeOfDay:             normalizedTimeOfDay,
				RepeatDaily:           slot.RepeatDaily,
				Timezone:              time.Now().Location().String(),
				GenerationLeadMinutes: workflow.NormalizeAccountSkillGenerationLeadMinutes(slot.GenerationLeadMinutes),
			}
			if scheduleConfig.ScheduleKey == "" {
				scheduleConfig.ScheduleKey = workflow.BuildDefaultAccountSkillScheduleKey(skill.ID, account.ID, scheduleConfig)
			}
			slots = append(slots, scheduleConfig)
		}
	}

	if workflow.IsMixVideoSkillOutput(skill.OutputType) {
		h.createMixVideoSkillRuns(w, r, user.ID, *account, *skill, slots, payload.PublishAt, settings)
		return
	}

	createdJobs := make([]domain.AIJob, 0, len(slots)+1)
	createdCount := 0
	if len(slots) == 0 {
		if payload.PublishAt == nil || strings.TrimSpace(*payload.PublishAt) == "" {
			render.Error(w, http.StatusBadRequest, "scheduleSlots is required")
			return
		}
		publishAt, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*payload.PublishAt))
		if parseErr != nil {
			render.Error(w, http.StatusBadRequest, "publishAt must be RFC3339")
			return
		}
		prepared, prepareErr := workflow.PrepareAccountSkillRun(r.Context(), h.app, *skill, *account, publishAt, nil)
		if prepareErr != nil {
			render.Error(w, http.StatusConflict, prepareErr.Error())
			return
		}
		jobID := uuid.NewString()
		if prepared.Status == "queued" {
			billingPreview, previewErr := previewAIJobBilling(r.Context(), h.app, &domain.AIJob{
				ID:           jobID,
				OwnerUserID:  user.ID,
				DeviceID:     &account.DeviceID,
				SkillID:      &skill.ID,
				Source:       "account_skill_binding",
				JobType:      prepared.JobType,
				ModelName:    prepared.ModelName,
				Prompt:       stringPtr(prepared.Prompt),
				InputPayload: prepared.InputPayload,
			})
			if previewErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to validate billing availability")
				return
			}
			if usageBillingShouldBlock(billingPreview) {
				renderUsageBillingBlocked(w, billingPreview)
				return
			}
		}
		var job *domain.AIJob
		lockKey := buildAccountSkillRunLockKey(user.ID, account.ID, skill.ID, prepared.GenerateAt, "")
		if err := h.app.Store.WithAdvisoryLock(r.Context(), lockKey, func() error {
			existing, err := h.app.Store.FindActiveAccountSkillJobByRun(r.Context(), user.ID, skill.ID, account.DeviceID, account.ID, prepared.GenerateAt)
			if err != nil {
				return err
			}
			if existing != nil {
				job = existing
				return nil
			}

			created, createErr := h.app.Store.CreateAIJob(r.Context(), store.CreateAIJobInput{
				ID:           jobID,
				OwnerUserID:  user.ID,
				DeviceID:     &account.DeviceID,
				SkillID:      &skill.ID,
				Source:       "account_skill_binding",
				LocalTaskID:  nil,
				JobType:      prepared.JobType,
				ModelName:    prepared.ModelName,
				Prompt:       stringPtr(prepared.Prompt),
				InputPayload: prepared.InputPayload,
				Status:       prepared.Status,
				Message:      stringPtr(prepared.Message),
				RunAt:        &prepared.GenerateAt,
			})
			if createErr != nil {
				return createErr
			}
			job = created
			createdCount++
			return nil
		}); err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to create account skill run")
			return
		}
		if createdCount == 1 {
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
					"publishAt":   prepared.PublishAt,
					"generateAt":  prepared.GenerateAt,
					"jobType":     prepared.JobType,
					"source":      job.Source,
				}),
			})
		}
		createdJobs = append(createdJobs, *job)
		statusCode := http.StatusOK
		if createdCount == 1 {
			statusCode = http.StatusCreated
		}
		render.JSON(w, statusCode, createdJobs)
		return
	}

	type pendingAccountSkillJob struct {
		input        store.CreateAIJobInput
		auditPayload []byte
		scheduleKey  string
		generateAt   time.Time
	}
	pendingJobs := make([]pendingAccountSkillJob, 0, len(slots))

	for _, slot := range slots {
		publishAt, publishErr := workflow.NextAccountSkillPublishAt(slot.TimeOfDay, slot.Timezone, time.Now())
		if publishErr != nil {
			render.Error(w, http.StatusBadRequest, publishErr.Error())
			return
		}
		prepared, prepareErr := workflow.PrepareAccountSkillRun(r.Context(), h.app, *skill, *account, publishAt, &slot)
		if prepareErr != nil {
			render.Error(w, http.StatusConflict, prepareErr.Error())
			return
		}
		jobID := uuid.NewString()
		if prepared.Status == "queued" {
			billingPreview, previewErr := previewAIJobBilling(r.Context(), h.app, &domain.AIJob{
				ID:           jobID,
				OwnerUserID:  user.ID,
				DeviceID:     &account.DeviceID,
				SkillID:      &skill.ID,
				Source:       "account_skill_binding",
				JobType:      prepared.JobType,
				ModelName:    prepared.ModelName,
				Prompt:       stringPtr(prepared.Prompt),
				InputPayload: prepared.InputPayload,
			})
			if previewErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to validate billing availability")
				return
			}
			if usageBillingShouldBlock(billingPreview) {
				renderUsageBillingBlocked(w, billingPreview)
				return
			}
		}
		pendingJobs = append(pendingJobs, pendingAccountSkillJob{
			input: store.CreateAIJobInput{
				ID:           jobID,
				OwnerUserID:  user.ID,
				DeviceID:     &account.DeviceID,
				SkillID:      &skill.ID,
				Source:       "account_skill_binding",
				LocalTaskID:  nil,
				JobType:      prepared.JobType,
				ModelName:    prepared.ModelName,
				Prompt:       stringPtr(prepared.Prompt),
				InputPayload: prepared.InputPayload,
				Status:       prepared.Status,
				Message:      stringPtr(prepared.Message),
				RunAt:        &prepared.GenerateAt,
			},
			scheduleKey: slot.ScheduleKey,
			generateAt:  prepared.GenerateAt,
			auditPayload: mustJSONBytes(map[string]any{
				"accountId":             account.ID,
				"accountName":           account.AccountName,
				"deviceId":              account.DeviceID,
				"skillId":               skill.ID,
				"publishAt":             prepared.PublishAt,
				"generateAt":            prepared.GenerateAt,
				"jobType":               prepared.JobType,
				"source":                "account_skill_binding",
				"timeOfDay":             slot.TimeOfDay,
				"repeatDaily":           slot.RepeatDaily,
				"scheduleKey":           slot.ScheduleKey,
				"scheduleZone":          slot.Timezone,
				"generationLeadMinutes": slot.GenerationLeadMinutes,
			}),
		})
	}

	for _, pending := range pendingJobs {
		var job *domain.AIJob
		lockKey := buildAccountSkillRunLockKey(user.ID, account.ID, skill.ID, pending.generateAt, pending.scheduleKey)
		jobCreated := false
		if err := h.app.Store.WithAdvisoryLock(r.Context(), lockKey, func() error {
			var (
				existing *domain.AIJob
				err      error
			)
			if strings.TrimSpace(pending.scheduleKey) != "" {
				existing, err = h.app.Store.FindAccountSkillJobByScheduleSlot(r.Context(), user.ID, account.ID, pending.scheduleKey, pending.generateAt)
			} else {
				existing, err = h.app.Store.FindActiveAccountSkillJobByRun(r.Context(), user.ID, skill.ID, account.DeviceID, account.ID, pending.generateAt)
			}
			if err != nil {
				return err
			}
			if existing != nil {
				job = existing
				return nil
			}

			created, createErr := h.app.Store.CreateAIJob(r.Context(), pending.input)
			if createErr != nil {
				return createErr
			}
			job = created
			jobCreated = true
			return nil
		}); err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to create account skill run")
			return
		}
		if jobCreated {
			createdCount++
			recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
				OwnerUserID:  user.ID,
				ResourceType: "ai_job",
				ResourceID:   &job.ID,
				Action:       "create",
				Title:        "账号绑定技能任务",
				Source:       account.Platform,
				Status:       job.Status,
				Message:      auditStringPtr("已为账号创建专属技能生成任务"),
				Payload:      pending.auditPayload,
			})
		}
		createdJobs = append(createdJobs, *job)
	}

	statusCode := http.StatusOK
	if createdCount == len(createdJobs) {
		statusCode = http.StatusCreated
	}
	render.JSON(w, statusCode, createdJobs)
}

func (h *AccountHandler) createMixVideoSkillRuns(
	w http.ResponseWriter,
	r *http.Request,
	ownerUserID string,
	account domain.PlatformAccount,
	skill domain.ProductSkill,
	slots []workflow.AccountSkillScheduleConfig,
	publishAtRaw *string,
	settings effectiveAdminSystemSettings,
) {
	createdTasks := make([]domain.MixVideoTask, 0, len(slots)+1)
	createdCount := 0
	runSettings := workflow.MixVideoRunSettings{
		CreditsPerSecondMillis: settings.MixVideoCreditsPerSecondMillis,
		ScriptRewritePrompt:    settings.MixVideoScriptRewritePrompt,
		PublishIntroPrompt:     settings.MixVideoPublishIntroPrompt,
	}

	createTask := func(prepared *workflow.PreparedAccountMixVideoRun, scheduleKey string) (*domain.MixVideoTask, bool, error) {
		taskID := uuid.NewString()
		lockKey := buildAccountSkillRunLockKey(ownerUserID, account.ID, skill.ID, prepared.GenerateAt, scheduleKey)
		var (
			task    *domain.MixVideoTask
			created bool
		)
		err := h.app.Store.WithAdvisoryLock(r.Context(), lockKey, func() error {
			var (
				existing *domain.MixVideoTask
				err      error
			)
			if strings.TrimSpace(scheduleKey) != "" {
				existing, err = h.app.Store.FindAccountSkillMixVideoTaskByScheduleSlot(r.Context(), ownerUserID, account.ID, scheduleKey, prepared.GenerateAt)
			} else {
				existing, err = h.app.Store.FindActiveAccountSkillMixVideoTaskByRun(r.Context(), ownerUserID, skill.ID, account.DeviceID, account.ID, prepared.GenerateAt)
			}
			if err != nil {
				return err
			}
			if existing != nil {
				task = existing
				return nil
			}

			createdTask, err := h.app.Store.CreateMixVideoTask(r.Context(), store.CreateMixVideoTaskInput{
				ID:                       taskID,
				OwnerUserID:              ownerUserID,
				Source:                   "account_skill_binding",
				Status:                   prepared.Status,
				SourceAssets:             mustJSONBytes(prepared.SourceAssets),
				RefAudioAsset:            mustJSONBytes(prepared.RefAudioAsset),
				ScriptText:               prepared.ScriptText,
				DeviceID:                 &account.DeviceID,
				SkillID:                  &skill.ID,
				AccountID:                &account.ID,
				Platform:                 &account.Platform,
				AccountName:              &account.AccountName,
				RunAt:                    &prepared.GenerateAt,
				SchedulePayload:          prepared.SchedulePayload,
				LocalPublishTaskID:       nil,
				EstimatedDurationSeconds: prepared.EstimatedDurationSeconds,
				EstimatedCreditsMillis:   prepared.EstimatedCreditsMillis,
				BillingStatus:            "pending",
				BillingPayload: mustJSONBytes(map[string]any{
					"creditsPerSecond":       prepared.BillingPreview.CreditsPerSecond,
					"creditsPerSecondMillis": settings.MixVideoCreditsPerSecondMillis,
					"estimateCharsPerSecond": 4,
					"billingMessage":         "混剪任务待预扣积分",
				}),
				Progress: mustJSONBytes(domain.MixVideoProgress{
					Message: prepared.Message,
				}),
				RequestPayload: prepared.RequestPayload,
			})
			if err != nil {
				return err
			}
			precharged, err := h.app.Store.PrechargeMixVideoTask(r.Context(), createdTask.ID)
			if err != nil {
				_ = h.app.Store.DeleteMixVideoTask(r.Context(), createdTask.ID, ownerUserID)
				return err
			}
			task = precharged
			created = true
			return nil
		})
		return task, created, err
	}

	if len(slots) == 0 {
		if publishAtRaw == nil || strings.TrimSpace(*publishAtRaw) == "" {
			render.Error(w, http.StatusBadRequest, "scheduleSlots is required")
			return
		}
		publishAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*publishAtRaw))
		if err != nil {
			render.Error(w, http.StatusBadRequest, "publishAt must be RFC3339")
			return
		}
		prepared, err := workflow.PrepareAccountMixVideoRun(r.Context(), h.app, runSettings, skill, account, publishAt, nil)
		if err != nil {
			render.Error(w, http.StatusConflict, err.Error())
			return
		}
		if !prepared.BillingPreview.CanAfford {
			render.Error(w, http.StatusConflict, fmt.Sprintf("当前积分不足，预计需要 %s 积分，还差 %s 积分", formatMixVideoCredits(prepared.BillingPreview.EstimatedCredits), formatMixVideoCredits(prepared.BillingPreview.ShortfallCredits)))
			return
		}
		task, created, err := createTask(prepared, "")
		if err != nil {
			if err == store.ErrMixVideoBillingInsufficientBalance {
				render.Error(w, http.StatusConflict, fmt.Sprintf("当前积分不足，预计需要 %s 积分，还差 %s 积分", formatMixVideoCredits(prepared.BillingPreview.EstimatedCredits), formatMixVideoCredits(prepared.BillingPreview.ShortfallCredits)))
				return
			}
			render.Error(w, http.StatusInternalServerError, "Failed to create account skill run")
			return
		}
		if created {
			createdCount++
			recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
				OwnerUserID:  ownerUserID,
				ResourceType: "mix_video_task",
				ResourceID:   &task.ID,
				Action:       "create",
				Title:        "账号绑定混剪任务",
				Source:       account.Platform,
				Status:       task.Status,
				Message:      auditStringPtr("已为账号创建混剪任务"),
				Payload: mustJSONBytes(map[string]any{
					"accountId":   account.ID,
					"accountName": account.AccountName,
					"deviceId":    account.DeviceID,
					"skillId":     skill.ID,
					"publishAt":   prepared.PublishAt,
					"generateAt":  prepared.GenerateAt,
					"source":      task.Source,
				}),
			})
		}
		createdTasks = append(createdTasks, *task)
		statusCode := http.StatusOK
		if created {
			statusCode = http.StatusCreated
		}
		render.JSON(w, statusCode, createdTasks)
		return
	}

	for _, slot := range slots {
		publishAt, err := workflow.NextAccountSkillPublishAt(slot.TimeOfDay, slot.Timezone, time.Now())
		if err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		prepared, err := workflow.PrepareAccountMixVideoRun(r.Context(), h.app, runSettings, skill, account, publishAt, &slot)
		if err != nil {
			render.Error(w, http.StatusConflict, err.Error())
			return
		}
		if !prepared.BillingPreview.CanAfford {
			render.Error(w, http.StatusConflict, fmt.Sprintf("当前积分不足，预计需要 %s 积分，还差 %s 积分", formatMixVideoCredits(prepared.BillingPreview.EstimatedCredits), formatMixVideoCredits(prepared.BillingPreview.ShortfallCredits)))
			return
		}
		task, created, err := createTask(prepared, slot.ScheduleKey)
		if err != nil {
			if err == store.ErrMixVideoBillingInsufficientBalance {
				render.Error(w, http.StatusConflict, fmt.Sprintf("当前积分不足，预计需要 %s 积分，还差 %s 积分", formatMixVideoCredits(prepared.BillingPreview.EstimatedCredits), formatMixVideoCredits(prepared.BillingPreview.ShortfallCredits)))
				return
			}
			render.Error(w, http.StatusInternalServerError, "Failed to create account skill run")
			return
		}
		if created {
			createdCount++
			recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
				OwnerUserID:  ownerUserID,
				ResourceType: "mix_video_task",
				ResourceID:   &task.ID,
				Action:       "create",
				Title:        "账号绑定混剪任务",
				Source:       account.Platform,
				Status:       task.Status,
				Message:      auditStringPtr("已为账号创建混剪任务"),
				Payload: mustJSONBytes(map[string]any{
					"accountId":             account.ID,
					"accountName":           account.AccountName,
					"deviceId":              account.DeviceID,
					"skillId":               skill.ID,
					"publishAt":             prepared.PublishAt,
					"generateAt":            prepared.GenerateAt,
					"timeOfDay":             slot.TimeOfDay,
					"repeatDaily":           slot.RepeatDaily,
					"scheduleKey":           slot.ScheduleKey,
					"scheduleZone":          slot.Timezone,
					"generationLeadMinutes": slot.GenerationLeadMinutes,
				}),
			})
		}
		createdTasks = append(createdTasks, *task)
	}

	statusCode := http.StatusOK
	if createdCount == len(createdTasks) {
		statusCode = http.StatusCreated
	}
	render.JSON(w, statusCode, createdTasks)
}

// 构建账号技能运行Lock键，为账号生成后续步骤所需的派生参数或载荷。
func buildAccountSkillRunLockKey(ownerUserID string, accountID string, skillID string, generateAt time.Time, scheduleKey string) string {
	if strings.TrimSpace(scheduleKey) != "" {
		return "account-skill-schedule:" + strings.TrimSpace(ownerUserID) + ":" + strings.TrimSpace(scheduleKey)
	}
	return "account-skill-run:" + strings.TrimSpace(ownerUserID) + ":" + strings.TrimSpace(accountID) + ":" + strings.TrimSpace(skillID) + ":" + generateAt.UTC().Format(time.RFC3339)
}

// 处理账号删除技能运行接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AccountHandler) DeleteSkillRun(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
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

	mixTask, err := h.app.Store.GetMixVideoTaskByOwner(r.Context(), jobID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load account skill run")
		return
	}
	if mixTask != nil && strings.TrimSpace(mixTask.Source) == "account_skill_binding" {
		if mixTask.AccountID == nil || strings.TrimSpace(*mixTask.AccountID) != accountID {
			render.Error(w, http.StatusNotFound, "Account skill run not found")
			return
		}
		if strings.TrimSpace(stringValue(mixTask.LocalPublishTaskID)) != "" {
			render.Error(w, http.StatusConflict, "任务已进入发布链路，不能直接删除")
			return
		}
		if mixTask.Status == "running" {
			render.Error(w, http.StatusConflict, "任务正在执行中，请稍后再试")
			return
		}
		scheduleConfig, _ := workflow.ParseAccountSkillScheduleConfig(mixTask.RequestPayload)
		scheduleKey := ""
		repeating := false
		if scheduleConfig != nil {
			scheduleKey = scheduleConfig.ScheduleKey
			repeating = scheduleConfig.RepeatDaily
		}
		deleted, err := h.app.Store.DeleteAccountSkillMixVideoRunPlanByOwner(r.Context(), jobID, user.ID, scheduleKey, repeating)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to delete account skill run")
			return
		}
		if !deleted {
			render.Error(w, http.StatusConflict, "任务当前状态不支持删除")
			return
		}
		recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
			OwnerUserID:  user.ID,
			ResourceType: "mix_video_task",
			ResourceID:   &jobID,
			Action:       "delete",
			Title:        "删除账号混剪计划",
			Source:       account.Platform,
			Status:       "success",
			Message:      auditStringPtr("账号混剪计划已删除，重复计划已停止续排"),
			Payload: mustJSONBytes(map[string]any{
				"accountId":          account.ID,
				"accountName":        account.AccountName,
				"deviceId":           account.DeviceID,
				"mixVideoTaskId":     jobID,
				"scheduleKey":        scheduleKey,
				"repeatDaily":        repeating,
				"publishAt":          mixTask.RunAt,
				"localPublishTaskId": mixTask.LocalPublishTaskID,
				"taskStatus":         mixTask.Status,
			}),
		})
		render.JSON(w, http.StatusOK, map[string]any{"deleted": true})
		return
	}

	job, err := h.app.Store.GetAIJobByOwner(r.Context(), jobID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load account skill run")
		return
	}
	if job == nil || strings.TrimSpace(job.Source) != "account_skill_binding" {
		render.Error(w, http.StatusNotFound, "Account skill run not found")
		return
	}
	if workflow.ExtractAccountSkillTargetAccountID(job.InputPayload) != accountID {
		render.Error(w, http.StatusNotFound, "Account skill run not found")
		return
	}
	if strings.TrimSpace(stringValue(job.LocalPublishTaskID)) != "" {
		render.Error(w, http.StatusConflict, "任务已进入发布链路，不能直接删除")
		return
	}
	if job.Status == "running" {
		render.Error(w, http.StatusConflict, "任务正在执行中，请稍后再试")
		return
	}

	linkedTasks, err := h.app.Store.ListPublishTasksByAIJobOwner(r.Context(), jobID, user.ID, 1)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect linked publish tasks")
		return
	}
	if len(linkedTasks) > 0 {
		render.Error(w, http.StatusConflict, "任务已进入发布链路，不能直接删除")
		return
	}

	artifacts, err := h.app.Store.ListAIJobArtifactsByOwner(r.Context(), jobID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load account skill artifacts")
		return
	}

	scheduleConfig, _ := workflow.ParseAccountSkillScheduleConfig(job.InputPayload)
	scheduleKey := ""
	repeating := false
	if scheduleConfig != nil {
		scheduleKey = scheduleConfig.ScheduleKey
		repeating = scheduleConfig.RepeatDaily
	}

	deleted, err := h.app.Store.DeleteAccountSkillRunPlanByOwner(r.Context(), jobID, user.ID, scheduleKey, repeating)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to delete account skill run")
		return
	}
	if !deleted {
		render.Error(w, http.StatusConflict, "任务当前状态不支持删除")
		return
	}

	cleanupAIArtifactFiles(h.app, r.Context(), artifacts)

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "ai_job",
		ResourceID:   &jobID,
		Action:       "delete",
		Title:        "删除账号技能计划",
		Source:       account.Platform,
		Status:       "success",
		Message:      auditStringPtr("账号技能计划已删除，重复计划已停止续排"),
		Payload: mustJSONBytes(map[string]any{
			"accountId":      account.ID,
			"accountName":    account.AccountName,
			"deviceId":       account.DeviceID,
			"jobId":          jobID,
			"scheduleKey":    scheduleKey,
			"repeatDaily":    repeating,
			"publishAt":      job.RunAt,
			"localTaskId":    job.LocalTaskID,
			"jobStatus":      job.Status,
			"deliveryStatus": job.DeliveryStatus,
		}),
	})

	render.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// 处理账号删除接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}

	taskCount, _, err := h.app.Store.GetAccountUsageSummary(r.Context(), accountID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect account usage")
		return
	}
	if taskCount > 0 {
		render.JSON(w, http.StatusConflict, map[string]any{
			"error": "账号仍存在计划中任务，请先处理后再解绑",
			"usage": map[string]any{
				"publishTaskCount": taskCount,
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
		Message:      auditStringPtr("云端账号镜像及历史记录已删除，并阻止设备自动补回"),
	})
	render.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}
