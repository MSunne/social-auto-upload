package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AIHandler struct {
	app *appstate.App
}

type createAIJobRequest struct {
	SkillID      *string     `json:"skillId"`
	JobType      string      `json:"jobType"`
	ModelName    string      `json:"modelName"`
	Prompt       *string     `json:"prompt"`
	InputPayload interface{} `json:"inputPayload"`
}

type updateAIJobRequest struct {
	SkillID       *string     `json:"skillId"`
	Prompt        *string     `json:"prompt"`
	Status        *string     `json:"status"`
	InputPayload  interface{} `json:"inputPayload"`
	OutputPayload interface{} `json:"outputPayload"`
	Message       *string     `json:"message"`
	CostCredits   *int64      `json:"costCredits"`
	FinishedAt    *string     `json:"finishedAt"`
}

func NewAIHandler(app *appstate.App) *AIHandler {
	return &AIHandler{app: app}
}

func (h *AIHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	items, err := h.app.Store.ListAIModels(r.Context(), category)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI models")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AIHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	limit := 0
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			render.Error(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = parsed
	}
	items, err := h.app.Store.ListAIJobsByOwner(r.Context(), user.ID, store.ListAIJobsFilter{
		JobType: strings.TrimSpace(r.URL.Query().Get("jobType")),
		Status:  strings.TrimSpace(r.URL.Query().Get("status")),
		SkillID: strings.TrimSpace(r.URL.Query().Get("skillId")),
		Limit:   limit,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI jobs")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AIHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	var payload createAIJobRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.JobType = strings.TrimSpace(payload.JobType)
	payload.ModelName = strings.TrimSpace(payload.ModelName)
	if payload.JobType == "" || payload.ModelName == "" {
		render.Error(w, http.StatusBadRequest, "jobType and modelName are required")
		return
	}
	model, err := h.app.Store.GetAIModelByName(r.Context(), payload.ModelName)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to validate AI model")
		return
	}
	if model == nil || !model.IsEnabled {
		render.Error(w, http.StatusNotFound, "AI model not found")
		return
	}

	var skillID *string
	if payload.SkillID != nil && strings.TrimSpace(*payload.SkillID) != "" {
		trimmed := strings.TrimSpace(*payload.SkillID)
		skill, skillErr := h.app.Store.GetOwnedSkillByID(r.Context(), trimmed, user.ID)
		if skillErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to validate skill")
			return
		}
		if skill == nil {
			render.Error(w, http.StatusNotFound, "Skill not found")
			return
		}
		if skill.OutputType != payload.JobType {
			render.Error(w, http.StatusConflict, "Skill outputType does not match AI job type")
			return
		}
		skillID = &trimmed
	}

	var inputPayload []byte
	if payload.InputPayload != nil {
		inputPayload, err = json.Marshal(payload.InputPayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "inputPayload must be valid json")
			return
		}
	}

	message := "任务已创建，等待后续执行器接管"
	job, err := h.app.Store.CreateAIJob(r.Context(), store.CreateAIJobInput{
		ID:           uuid.NewString(),
		OwnerUserID:  user.ID,
		SkillID:      skillID,
		JobType:      payload.JobType,
		ModelName:    payload.ModelName,
		Prompt:       payload.Prompt,
		InputPayload: inputPayload,
		Status:       "queued",
		Message:      &message,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create AI job")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "ai_job",
		ResourceID:   &job.ID,
		Action:       "create",
		Title:        "创建 AI 任务",
		Source:       job.ModelName,
		Status:       job.Status,
		Message:      job.Message,
		Payload: mustJSONBytes(map[string]any{
			"jobType":   job.JobType,
			"modelName": job.ModelName,
			"prompt":    job.Prompt,
		}),
	})
	render.JSON(w, http.StatusCreated, job)
}

func (h *AIHandler) DetailJob(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	job, err := h.app.Store.GetAIJobByOwner(r.Context(), jobID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI job")
		return
	}
	if job == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}
	render.JSON(w, http.StatusOK, job)
}

func (h *AIHandler) WorkspaceJob(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	job, err := h.app.Store.GetAIJobByOwner(r.Context(), jobID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI job")
		return
	}
	if job == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}

	model, err := h.app.Store.GetAIModelByName(r.Context(), job.ModelName)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI model")
		return
	}
	var skill *domain.ProductSkill
	if job.SkillID != nil && strings.TrimSpace(*job.SkillID) != "" {
		skill, err = h.app.Store.GetOwnedSkillByID(r.Context(), strings.TrimSpace(*job.SkillID), user.ID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load AI skill")
			return
		}
	}

	render.JSON(w, http.StatusOK, domain.AIJobWorkspace{
		Job:     *job,
		Model:   model,
		Skill:   skill,
		Actions: computeAIJobActions(job),
	})
}

func (h *AIHandler) UpdateJob(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	existing, err := h.app.Store.GetAIJobByOwner(r.Context(), jobID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI job")
		return
	}
	if existing == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}

	var payload updateAIJobRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var inputPayload []byte
	inputTouched := payload.InputPayload != nil
	if inputTouched {
		inputPayload, err = json.Marshal(payload.InputPayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "inputPayload must be valid json")
			return
		}
	}

	var outputPayload []byte
	outputTouched := payload.OutputPayload != nil
	if outputTouched {
		outputPayload, err = json.Marshal(payload.OutputPayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "outputPayload must be valid json")
			return
		}
	}

	if payload.Status != nil && !isAllowedAIJobTransition(existing.Status, strings.TrimSpace(*payload.Status)) {
		render.Error(w, http.StatusConflict, "AI job status transition is not allowed")
		return
	}

	var skillID *string
	skillTouched := payload.SkillID != nil
	if payload.SkillID != nil {
		trimmed := strings.TrimSpace(*payload.SkillID)
		if trimmed == "" {
			skillID = nil
		} else {
			skill, skillErr := h.app.Store.GetOwnedSkillByID(r.Context(), trimmed, user.ID)
			if skillErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to validate skill")
				return
			}
			if skill == nil {
				render.Error(w, http.StatusNotFound, "Skill not found")
				return
			}
			if skill.OutputType != existing.JobType {
				render.Error(w, http.StatusConflict, "Skill outputType does not match AI job type")
				return
			}
			skillID = &trimmed
		}
	}

	var finishedAt *time.Time
	if payload.FinishedAt != nil && strings.TrimSpace(*payload.FinishedAt) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*payload.FinishedAt))
		if parseErr != nil {
			render.Error(w, http.StatusBadRequest, "finishedAt must be RFC3339")
			return
		}
		finishedAt = &parsed
	}

	job, err := h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
		SkillID:         skillID,
		SkillTouched:    skillTouched,
		Prompt:          payload.Prompt,
		Status:          normalizeAIStatus(payload.Status),
		InputPayload:    inputPayload,
		InputTouched:    inputTouched,
		OutputPayload:   outputPayload,
		OutputTouched:   outputTouched,
		Message:         payload.Message,
		CostCredits:     payload.CostCredits,
		FinishedAt:      finishedAt,
		FinishedTouched: payload.FinishedAt != nil,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update AI job")
		return
	}
	if job == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "ai_job",
		ResourceID:   &job.ID,
		Action:       "update",
		Title:        "更新 AI 任务",
		Source:       job.ModelName,
		Status:       job.Status,
		Message:      job.Message,
		Payload: mustJSONBytes(map[string]any{
			"skillId":     payload.SkillID,
			"status":      payload.Status,
			"costCredits": payload.CostCredits,
			"hasOutput":   outputTouched,
			"hasInput":    inputTouched,
			"finishedAt":  payload.FinishedAt,
		}),
	})

	render.JSON(w, http.StatusOK, job)
}

func (h *AIHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	existing, err := h.app.Store.GetAIJobByOwner(r.Context(), jobID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI job")
		return
	}
	if existing == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}
	if !computeAIJobActions(existing).CanCancel {
		render.Error(w, http.StatusConflict, "AI job cannot be cancelled")
		return
	}

	now := time.Now().UTC()
	message := "AI 任务已取消"
	status := "cancelled"
	job, err := h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
		Status:          &status,
		Message:         &message,
		FinishedAt:      &now,
		FinishedTouched: true,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to cancel AI job")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "ai_job",
		ResourceID:   &job.ID,
		Action:       "cancel",
		Title:        "取消 AI 任务",
		Source:       job.ModelName,
		Status:       job.Status,
		Message:      job.Message,
	})

	render.JSON(w, http.StatusOK, job)
}

func (h *AIHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	existing, err := h.app.Store.GetAIJobByOwner(r.Context(), jobID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI job")
		return
	}
	if existing == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}
	if !computeAIJobActions(existing).CanRetry {
		render.Error(w, http.StatusConflict, "AI job cannot be retried")
		return
	}

	status := "queued"
	message := "AI 任务已重新排队"
	job, err := h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
		Status:          &status,
		Message:         &message,
		OutputPayload:   []byte("null"),
		OutputTouched:   true,
		FinishedAt:      nil,
		FinishedTouched: true,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to retry AI job")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "ai_job",
		ResourceID:   &job.ID,
		Action:       "retry",
		Title:        "重试 AI 任务",
		Source:       job.ModelName,
		Status:       job.Status,
		Message:      job.Message,
	})

	render.JSON(w, http.StatusOK, job)
}

func normalizeAIStatus(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func isAllowedAIJobTransition(current string, next string) bool {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if current == "" || next == "" {
		return false
	}
	if current == next {
		return true
	}

	switch current {
	case "queued":
		return next == "running" || next == "cancelled" || next == "failed"
	case "running":
		return next == "success" || next == "completed" || next == "failed" || next == "cancelled"
	case "failed", "cancelled", "success", "completed":
		return false
	default:
		return true
	}
}

func computeAIJobActions(job *domain.AIJob) domain.AIJobActionState {
	if job == nil {
		return domain.AIJobActionState{}
	}

	switch job.Status {
	case "queued":
		return domain.AIJobActionState{CanEdit: true, CanCancel: true, CanRetry: false}
	case "running":
		return domain.AIJobActionState{CanEdit: false, CanCancel: true, CanRetry: false}
	case "failed", "cancelled", "success", "completed":
		return domain.AIJobActionState{CanEdit: true, CanCancel: false, CanRetry: true}
	default:
		return domain.AIJobActionState{CanEdit: true, CanCancel: false, CanRetry: false}
	}
}
