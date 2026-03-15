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

type AIHandler struct {
	app *appstate.App
}

type createAIJobRequest struct {
	JobType      string      `json:"jobType"`
	ModelName    string      `json:"modelName"`
	Prompt       *string     `json:"prompt"`
	InputPayload interface{} `json:"inputPayload"`
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
	jobType := strings.TrimSpace(r.URL.Query().Get("jobType"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	items, err := h.app.Store.ListAIJobsByOwner(r.Context(), user.ID, jobType, status)
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

	var inputPayload []byte
	var err error
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
