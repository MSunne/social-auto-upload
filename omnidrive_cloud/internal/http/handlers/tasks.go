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

type TaskHandler struct {
	app *appstate.App
}

type createTaskRequest struct {
	DeviceID     string      `json:"deviceId"`
	AccountID    *string     `json:"accountId"`
	SkillID      *string     `json:"skillId"`
	Platform     string      `json:"platform"`
	AccountName  string      `json:"accountName"`
	Title        string      `json:"title"`
	ContentText  *string     `json:"contentText"`
	MediaPayload interface{} `json:"mediaPayload"`
	RunAt        *string     `json:"runAt"`
}

func NewTaskHandler(app *appstate.App) *TaskHandler {
	return &TaskHandler{app: app}
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	items, err := h.app.Store.ListPublishTasksByOwner(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load tasks")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	var payload createTaskRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.DeviceID = strings.TrimSpace(payload.DeviceID)
	payload.Platform = strings.TrimSpace(payload.Platform)
	payload.AccountName = strings.TrimSpace(payload.AccountName)
	payload.Title = strings.TrimSpace(payload.Title)
	if payload.DeviceID == "" || payload.Platform == "" || payload.AccountName == "" || payload.Title == "" {
		render.Error(w, http.StatusBadRequest, "deviceId, platform, accountName, and title are required")
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

	if payload.SkillID != nil && strings.TrimSpace(*payload.SkillID) != "" {
		skill, skillErr := h.app.Store.GetOwnedSkillByID(r.Context(), strings.TrimSpace(*payload.SkillID), user.ID)
		if skillErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to validate skill")
			return
		}
		if skill == nil {
			render.Error(w, http.StatusNotFound, "Skill not found")
			return
		}
	}

	var mediaPayload []byte
	if payload.MediaPayload != nil {
		mediaPayload, err = json.Marshal(payload.MediaPayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "mediaPayload must be valid json")
			return
		}
	}

	task, err := h.app.Store.CreatePublishTask(r.Context(), store.CreatePublishTaskInput{
		ID:           uuid.NewString(),
		DeviceID:     payload.DeviceID,
		AccountID:    payload.AccountID,
		SkillID:      payload.SkillID,
		Platform:     payload.Platform,
		AccountName:  payload.AccountName,
		Title:        payload.Title,
		ContentText:  payload.ContentText,
		MediaPayload: mediaPayload,
		Status:       "pending",
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	render.JSON(w, http.StatusCreated, task)
}

func (h *TaskHandler) Detail(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	task, err := h.app.Store.GetPublishTaskByOwner(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task")
		return
	}
	if task == nil {
		render.Error(w, http.StatusNotFound, "Task not found")
		return
	}

	render.JSON(w, http.StatusOK, task)
}
