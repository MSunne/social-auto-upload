package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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

type TaskHandler struct {
	app *appstate.App
}

type createTaskRequest struct {
	DeviceID     string                   `json:"deviceId"`
	AccountID    *string                  `json:"accountId"`
	SkillID      *string                  `json:"skillId"`
	Platform     string                   `json:"platform"`
	AccountName  string                   `json:"accountName"`
	Title        string                   `json:"title"`
	ContentText  *string                  `json:"contentText"`
	MediaPayload interface{}              `json:"mediaPayload"`
	MaterialRefs []taskMaterialRefRequest `json:"materialRefs"`
	RunAt        *string                  `json:"runAt"`
}

type updateTaskRequest struct {
	Title        *string                   `json:"title"`
	ContentText  *string                   `json:"contentText"`
	MediaPayload interface{}               `json:"mediaPayload"`
	MaterialRefs *[]taskMaterialRefRequest `json:"materialRefs"`
	Status       *string                   `json:"status"`
	Message      *string                   `json:"message"`
	RunAt        *string                   `json:"runAt"`
}

type taskMaterialRefRequest struct {
	Root string  `json:"root"`
	Path string  `json:"path"`
	Role *string `json:"role"`
}

func NewTaskHandler(app *appstate.App) *TaskHandler {
	return &TaskHandler{app: app}
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
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
	items, err := h.app.Store.ListPublishTasksByOwner(r.Context(), user.ID, store.ListPublishTasksFilter{
		DeviceID:    strings.TrimSpace(r.URL.Query().Get("deviceId")),
		Status:      strings.TrimSpace(r.URL.Query().Get("status")),
		Platform:    strings.TrimSpace(r.URL.Query().Get("platform")),
		AccountName: strings.TrimSpace(r.URL.Query().Get("accountName")),
		Limit:       limit,
	})
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
	if !device.IsEnabled {
		render.Error(w, http.StatusConflict, "Device is disabled")
		return
	}

	if payload.AccountID != nil && strings.TrimSpace(*payload.AccountID) != "" {
		accountID := strings.TrimSpace(*payload.AccountID)
		account, accountErr := h.app.Store.GetOwnedAccountByID(r.Context(), accountID, user.ID)
		if accountErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to validate account")
			return
		}
		if account == nil {
			render.Error(w, http.StatusNotFound, "Account not found")
			return
		}
		if account.DeviceID != payload.DeviceID {
			render.Error(w, http.StatusConflict, "Account does not belong to the selected device")
			return
		}
		if account.Platform != payload.Platform || account.AccountName != payload.AccountName {
			render.Error(w, http.StatusConflict, "Account platform or name does not match the selected task target")
			return
		}
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

	var runAt *time.Time
	if payload.RunAt != nil && strings.TrimSpace(*payload.RunAt) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*payload.RunAt))
		if parseErr != nil {
			render.Error(w, http.StatusBadRequest, "runAt must be RFC3339")
			return
		}
		runAt = &parsed
	}

	taskID := uuid.NewString()
	materialRefs, err := h.prepareTaskMaterialRefs(r.Context(), user.ID, taskID, payload.DeviceID, payload.MaterialRefs)
	if err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := h.app.Store.CreatePublishTask(r.Context(), store.CreatePublishTaskInput{
		ID:           taskID,
		DeviceID:     payload.DeviceID,
		AccountID:    payload.AccountID,
		SkillID:      payload.SkillID,
		Platform:     payload.Platform,
		AccountName:  payload.AccountName,
		Title:        payload.Title,
		ContentText:  payload.ContentText,
		MediaPayload: mediaPayload,
		Status:       "pending",
		RunAt:        runAt,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	if _, err := h.app.Store.ReplacePublishTaskMaterialRefs(r.Context(), task.ID, user.ID, materialRefs); err != nil {
		_, _ = h.app.Store.DeletePublishTask(r.Context(), task.ID, user.ID)
		render.Error(w, http.StatusInternalServerError, "Failed to save task materials")
		return
	}
	_, _ = h.app.Store.CreatePublishTaskEvent(r.Context(), store.CreatePublishTaskEventInput{
		ID:        uuid.NewString(),
		TaskID:    task.ID,
		EventType: "created",
		Source:    "cloud",
		Status:    task.Status,
		Message:   auditStringPtr("任务已由云端创建"),
		Payload: mustJSONBytes(map[string]any{
			"deviceId":         task.DeviceID,
			"accountId":        task.AccountID,
			"skillId":          task.SkillID,
			"platform":         task.Platform,
			"accountName":      task.AccountName,
			"materialRefCount": len(materialRefs),
			"title":            task.Title,
			"runAt":            task.RunAt,
		}),
	})
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "publish_task",
		ResourceID:   &task.ID,
		Action:       "create",
		Title:        "创建发布任务",
		Source:       task.Platform,
		Status:       task.Status,
		Message:      auditStringPtr("发布任务已创建"),
		Payload: mustJSONBytes(map[string]any{
			"deviceId":         task.DeviceID,
			"accountName":      task.AccountName,
			"materialRefCount": len(materialRefs),
			"title":            task.Title,
		}),
	})

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

func (h *TaskHandler) Workspace(w http.ResponseWriter, r *http.Request) {
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

	device, err := h.app.Store.GetOwnedDevice(r.Context(), task.DeviceID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task device")
		return
	}

	var account *domain.PlatformAccount
	if task.AccountID != nil && strings.TrimSpace(*task.AccountID) != "" {
		account, err = h.app.Store.GetOwnedAccountByID(r.Context(), strings.TrimSpace(*task.AccountID), user.ID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load task account")
			return
		}
	}

	var skill *domain.ProductSkill
	if task.SkillID != nil && strings.TrimSpace(*task.SkillID) != "" {
		skill, err = h.app.Store.GetOwnedSkillByID(r.Context(), strings.TrimSpace(*task.SkillID), user.ID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load task skill")
			return
		}
	}

	events, err := h.app.Store.ListPublishTaskEventsByOwner(r.Context(), task.ID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task events")
		return
	}
	artifacts, err := h.app.Store.ListPublishTaskArtifactsByOwner(r.Context(), task.ID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task artifacts")
		return
	}
	materials, err := h.app.Store.ListPublishTaskMaterialRefsByOwner(r.Context(), task.ID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task materials")
		return
	}

	render.JSON(w, http.StatusOK, domain.PublishTaskWorkspace{
		Task:      *task,
		Device:    device,
		Account:   account,
		Skill:     skill,
		Events:    events,
		Artifacts: artifacts,
		Materials: materials,
		Actions:   computePublishTaskActions(task),
	})
}

func (h *TaskHandler) Events(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	items, err := h.app.Store.ListPublishTaskEventsByOwner(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task events")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *TaskHandler) Artifacts(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	items, err := h.app.Store.ListPublishTaskArtifactsByOwner(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task artifacts")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *TaskHandler) Materials(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	items, err := h.app.Store.ListPublishTaskMaterialRefsByOwner(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task materials")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	existing, err := h.app.Store.GetPublishTaskByOwner(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task")
		return
	}
	if existing == nil {
		render.Error(w, http.StatusNotFound, "Task not found")
		return
	}

	var payload updateTaskRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var mediaPayload []byte
	mediaTouched := payload.MediaPayload != nil
	if mediaTouched {
		mediaPayload, err = json.Marshal(payload.MediaPayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "mediaPayload must be valid json")
			return
		}
	}

	var runAt *time.Time
	if payload.RunAt != nil && strings.TrimSpace(*payload.RunAt) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*payload.RunAt))
		if parseErr != nil {
			render.Error(w, http.StatusBadRequest, "runAt must be RFC3339")
			return
		}
		runAt = &parsed
	}

	materialRefCount := -1
	var materialRefs []store.ReplacePublishTaskMaterialRefInput
	if payload.MaterialRefs != nil {
		materialRefs, err = h.prepareTaskMaterialRefs(r.Context(), user.ID, existing.ID, existing.DeviceID, *payload.MaterialRefs)
		if err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		materialRefCount = len(materialRefs)
	}

	task, err := h.app.Store.UpdatePublishTask(r.Context(), taskID, user.ID, store.UpdatePublishTaskInput{
		Title:        payload.Title,
		ContentText:  payload.ContentText,
		MediaPayload: mediaPayload,
		MediaTouched: mediaTouched,
		Status:       payload.Status,
		Message:      payload.Message,
		RunAt:        runAt,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update task")
		return
	}
	if payload.MaterialRefs != nil {
		if _, replaceErr := h.app.Store.ReplacePublishTaskMaterialRefs(r.Context(), task.ID, user.ID, materialRefs); replaceErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to update task materials")
			return
		}
	}
	_, _ = h.app.Store.CreatePublishTaskEvent(r.Context(), store.CreatePublishTaskEventInput{
		ID:        uuid.NewString(),
		TaskID:    task.ID,
		EventType: "updated",
		Source:    "cloud",
		Status:    task.Status,
		Message:   task.Message,
		Payload: mustJSONBytes(map[string]any{
			"title":            payload.Title,
			"contentText":      payload.ContentText,
			"status":           payload.Status,
			"message":          payload.Message,
			"runAt":            payload.RunAt,
			"mediaTouched":     mediaTouched,
			"materialsTouched": payload.MaterialRefs != nil,
			"materialRefCount": materialRefCount,
		}),
	})
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "publish_task",
		ResourceID:   &task.ID,
		Action:       "update",
		Title:        "更新发布任务",
		Source:       task.Platform,
		Status:       task.Status,
		Message:      task.Message,
		Payload: mustJSONBytes(map[string]any{
			"title":            payload.Title,
			"status":           payload.Status,
			"message":          payload.Message,
			"runAt":            payload.RunAt,
			"materialsTouched": payload.MaterialRefs != nil,
			"materialRefCount": materialRefCount,
		}),
	})

	render.JSON(w, http.StatusOK, task)
}

func (h *TaskHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	task, err := h.app.Store.RequestCancelPublishTask(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to cancel task")
		return
	}
	if task == nil {
		render.Error(w, http.StatusNotFound, "Task not found")
		return
	}

	_, _ = h.app.Store.CreatePublishTaskEvent(r.Context(), store.CreatePublishTaskEventInput{
		ID:        uuid.NewString(),
		TaskID:    task.ID,
		EventType: "cancel_requested",
		Source:    "cloud",
		Status:    task.Status,
		Message:   task.Message,
	})
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "publish_task",
		ResourceID:   &task.ID,
		Action:       "cancel",
		Title:        "请求取消发布任务",
		Source:       task.Platform,
		Status:       task.Status,
		Message:      task.Message,
	})

	render.JSON(w, http.StatusOK, task)
}

func (h *TaskHandler) Retry(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	existing, err := h.app.Store.GetPublishTaskByOwner(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect task")
		return
	}
	if existing == nil {
		render.Error(w, http.StatusNotFound, "Task not found")
		return
	}
	if existing.Status == "running" || existing.Status == "cancel_requested" {
		render.Error(w, http.StatusConflict, "Task is still executing or waiting for cancel confirmation")
		return
	}

	artifactsBeforeRetry, err := h.app.Store.ListPublishTaskArtifactsByOwner(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect task artifacts")
		return
	}

	task, err := h.app.Store.RetryPublishTask(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to retry task")
		return
	}
	if task == nil {
		render.Error(w, http.StatusNotFound, "Task not found")
		return
	}

	cleanupArtifactFiles(h.app, r.Context(), artifactsBeforeRetry)
	clearedArtifactCount, clearErr := h.app.Store.DeletePublishTaskArtifactsByOwner(r.Context(), task.ID, user.ID)
	if clearErr != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to clear previous task artifacts")
		return
	}

	_, _ = h.app.Store.CreatePublishTaskEvent(r.Context(), store.CreatePublishTaskEventInput{
		ID:        uuid.NewString(),
		TaskID:    task.ID,
		EventType: "retried",
		Source:    "cloud",
		Status:    task.Status,
		Message:   task.Message,
		Payload: mustJSONBytes(map[string]any{
			"clearedArtifactCount": clearedArtifactCount,
		}),
	})
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "publish_task",
		ResourceID:   &task.ID,
		Action:       "retry",
		Title:        "重试发布任务",
		Source:       task.Platform,
		Status:       task.Status,
		Message:      task.Message,
		Payload: mustJSONBytes(map[string]any{
			"clearedArtifactCount": clearedArtifactCount,
		}),
	})

	render.JSON(w, http.StatusOK, task)
}

func (h *TaskHandler) ForceRelease(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	task, err := h.app.Store.ForceReleasePublishTaskLease(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to force release task lease")
		return
	}
	if task == nil {
		render.Error(w, http.StatusConflict, "Task is not in a releasable leased state")
		return
	}

	_, _ = h.app.Store.CreatePublishTaskEvent(r.Context(), store.CreatePublishTaskEventInput{
		ID:        uuid.NewString(),
		TaskID:    task.ID,
		EventType: "force_released",
		Source:    "cloud",
		Status:    task.Status,
		Message:   task.Message,
	})
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "publish_task",
		ResourceID:   &task.ID,
		Action:       "force_release",
		Title:        "手动释放任务租约",
		Source:       task.Platform,
		Status:       task.Status,
		Message:      task.Message,
	})

	render.JSON(w, http.StatusOK, task)
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	artifacts, err := h.app.Store.ListPublishTaskArtifactsByOwner(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect task artifacts")
		return
	}

	deleted, err := h.app.Store.DeletePublishTask(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to delete task")
		return
	}
	if !deleted {
		render.Error(w, http.StatusNotFound, "Task not found")
		return
	}
	cleanupArtifactFiles(h.app, r.Context(), artifacts)
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "publish_task",
		ResourceID:   &taskID,
		Action:       "delete",
		Title:        "删除发布任务",
		Source:       "tasks",
		Status:       "success",
		Message:      auditStringPtr("发布任务已删除"),
	})

	render.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *TaskHandler) prepareTaskMaterialRefs(ctx context.Context, ownerUserID string, taskID string, deviceID string, refs []taskMaterialRefRequest) ([]store.ReplacePublishTaskMaterialRefInput, error) {
	items := make([]store.ReplacePublishTaskMaterialRefInput, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))

	for _, ref := range refs {
		rootName := strings.TrimSpace(ref.Root)
		relativePath := strings.TrimSpace(ref.Path)
		if rootName == "" || relativePath == "" {
			return nil, fmt.Errorf("materialRefs root and path are required")
		}

		entry, err := h.app.Store.GetMaterialEntryByOwner(ctx, ownerUserID, deviceID, rootName, relativePath)
		if err != nil {
			return nil, err
		}
		if entry == nil || !entry.IsAvailable {
			return nil, fmt.Errorf("materialRefs contains a file that is not mirrored on the selected device")
		}

		role := "media"
		if ref.Role != nil && strings.TrimSpace(*ref.Role) != "" {
			role = strings.TrimSpace(*ref.Role)
		}
		dedupeKey := role + "::" + rootName + "::" + entry.RelativePath
		if _, exists := seen[dedupeKey]; exists {
			continue
		}
		seen[dedupeKey] = struct{}{}

		items = append(items, store.ReplacePublishTaskMaterialRefInput{
			TaskID:       taskID,
			DeviceID:     deviceID,
			RootName:     rootName,
			RelativePath: entry.RelativePath,
			Role:         role,
			Name:         entry.Name,
			Kind:         entry.Kind,
			AbsolutePath: entry.AbsolutePath,
			SizeBytes:    entry.SizeBytes,
			ModifiedAt:   entry.ModifiedAt,
			Extension:    entry.Extension,
			MimeType:     entry.MimeType,
			IsText:       entry.IsText,
			PreviewText:  entry.PreviewText,
		})
	}

	return items, nil
}

func cleanupArtifactFiles(app *appstate.App, ctx context.Context, artifacts []domain.PublishTaskArtifact) {
	if app == nil || app.Storage == nil {
		return
	}
	seen := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.StorageKey == nil || strings.TrimSpace(*artifact.StorageKey) == "" {
			continue
		}
		key := strings.TrimSpace(*artifact.StorageKey)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		_ = app.Storage.DeleteObject(ctx, key)
	}
}

func cleanupReplacedArtifactFiles(app *appstate.App, ctx context.Context, previous []domain.PublishTaskArtifact, current []domain.PublishTaskArtifact) {
	if app == nil || app.Storage == nil {
		return
	}

	previousByKey := make(map[string]string, len(previous))
	for _, artifact := range previous {
		if artifact.StorageKey == nil || strings.TrimSpace(*artifact.StorageKey) == "" {
			continue
		}
		previousByKey[artifact.ArtifactKey] = strings.TrimSpace(*artifact.StorageKey)
	}

	seen := make(map[string]struct{}, len(current))
	for _, artifact := range current {
		if artifact.StorageKey == nil || strings.TrimSpace(*artifact.StorageKey) == "" {
			continue
		}
		newKey := strings.TrimSpace(*artifact.StorageKey)
		oldKey, exists := previousByKey[artifact.ArtifactKey]
		if !exists || oldKey == "" || oldKey == newKey {
			continue
		}
		if _, done := seen[oldKey]; done {
			continue
		}
		seen[oldKey] = struct{}{}
		_ = app.Storage.DeleteObject(ctx, oldKey)
	}
}

func computePublishTaskActions(task *domain.PublishTask) domain.PublishTaskActionState {
	if task == nil {
		return domain.PublishTaskActionState{}
	}

	switch task.Status {
	case "pending":
		return domain.PublishTaskActionState{CanEdit: true, CanCancel: true, CanRetry: false, CanDelete: true, CanForceRelease: false}
	case "running", "cancel_requested":
		return domain.PublishTaskActionState{CanEdit: false, CanCancel: task.Status == "running", CanRetry: false, CanDelete: false, CanForceRelease: true}
	case "needs_verify":
		return domain.PublishTaskActionState{CanEdit: true, CanCancel: true, CanRetry: true, CanDelete: true, CanForceRelease: false}
	case "failed", "cancelled", "success", "completed":
		return domain.PublishTaskActionState{CanEdit: true, CanCancel: false, CanRetry: true, CanDelete: true, CanForceRelease: false}
	default:
		return domain.PublishTaskActionState{CanEdit: true, CanCancel: false, CanRetry: false, CanDelete: true, CanForceRelease: false}
	}
}
