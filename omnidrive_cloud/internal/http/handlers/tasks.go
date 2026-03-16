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

type resumeTaskRequest struct {
	Message *string `json:"message"`
}

type manualResolveTaskRequest struct {
	Status       string      `json:"status"`
	Message      *string     `json:"message"`
	TextEvidence *string     `json:"textEvidence"`
	Payload      interface{} `json:"payload"`
}

type batchRepairTasksRequest struct {
	TaskIDs     []string `json:"taskIds"`
	Operations  []string `json:"operations"`
	DeviceID    string   `json:"deviceId"`
	Status      string   `json:"status"`
	Platform    string   `json:"platform"`
	AccountName string   `json:"accountName"`
	SkillID     string   `json:"skillId"`
	Readiness   string   `json:"readiness"`
	Dimension   string   `json:"dimension"`
	IssueCode   string   `json:"issueCode"`
	Limit       int      `json:"limit"`
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

func (h *TaskHandler) Diagnostics(w http.ResponseWriter, r *http.Request) {
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
	readinessFilter := strings.TrimSpace(r.URL.Query().Get("readiness"))
	if readinessFilter != "" && readinessFilter != "ready" && readinessFilter != "blocked" {
		render.Error(w, http.StatusBadRequest, "readiness must be ready or blocked")
		return
	}
	dimensionFilter := strings.TrimSpace(r.URL.Query().Get("dimension"))
	if dimensionFilter != "" && dimensionFilter != "device" && dimensionFilter != "account" && dimensionFilter != "skill" && dimensionFilter != "materials" {
		render.Error(w, http.StatusBadRequest, "dimension must be device, account, skill, or materials")
		return
	}
	issueCodeFilter := strings.TrimSpace(r.URL.Query().Get("issueCode"))

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

	results := make([]domain.PublishTaskDiagnosticItem, 0, len(items))
	for _, task := range items {
		diagnostic, diagErr := buildPublishTaskDiagnosticItem(r.Context(), h.app, user.ID, &task)
		if diagErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to build task diagnostics")
			return
		}
		isReady := publishTaskReadinessAllowsExecution(diagnostic.Readiness)
		if readinessFilter == "ready" && !isReady {
			continue
		}
		if readinessFilter == "blocked" && isReady {
			continue
		}
		if dimensionFilter != "" {
			matched := false
			for _, dimension := range diagnostic.BlockingDimensions {
				if dimension == dimensionFilter {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if issueCodeFilter != "" {
			matched := false
			for _, issueCode := range diagnostic.Readiness.IssueCodes {
				if issueCode == issueCodeFilter {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		results = append(results, diagnostic)
	}
	summary := summarizePublishTaskDiagnosticItems(results)

	render.JSON(w, http.StatusOK, map[string]any{
		"items":      results,
		"summary":    summary,
		"serverTime": time.Now().UTC(),
	})
}

func (h *TaskHandler) BulkRepair(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	var payload batchRepairTasksRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	operations := normalizeBatchRepairOperations(payload.Operations)
	if len(operations) == 0 {
		render.Error(w, http.StatusBadRequest, "operations must include refresh_materials or refresh_skill")
		return
	}
	payload.Readiness = strings.TrimSpace(payload.Readiness)
	if payload.Readiness != "" && payload.Readiness != "ready" && payload.Readiness != "blocked" {
		render.Error(w, http.StatusBadRequest, "readiness must be ready or blocked")
		return
	}
	payload.Dimension = strings.TrimSpace(payload.Dimension)
	if payload.Dimension != "" && payload.Dimension != "device" && payload.Dimension != "account" && payload.Dimension != "skill" && payload.Dimension != "materials" {
		render.Error(w, http.StatusBadRequest, "dimension must be device, account, skill, or materials")
		return
	}
	if payload.Limit < 0 {
		render.Error(w, http.StatusBadRequest, "limit must be a positive integer")
		return
	}

	tasks, err := h.selectTasksForBatchRepair(r.Context(), user.ID, payload)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to select tasks for bulk repair")
		return
	}

	items := make([]domain.PublishTaskBulkRepairItem, 0, len(tasks))
	for _, task := range tasks {
		item, itemErr := h.bulkRepairTask(r.Context(), user.ID, &task, operations)
		if itemErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to execute bulk repair")
			return
		}
		items = append(items, item)
	}

	render.JSON(w, http.StatusOK, domain.PublishTaskBulkRepairResult{
		Items:      items,
		Summary:    summarizeBulkRepairItems(items),
		ServerTime: time.Now().UTC(),
	})
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

	var skillRevision *string
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
		revision, revisionErr := h.app.Store.GetSkillRevision(r.Context(), skill.ID, user.ID)
		if revisionErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to snapshot skill revision")
			return
		}
		skillRevision = normalizeTrimmedStringPtr(revision)
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
		ID:            taskID,
		DeviceID:      payload.DeviceID,
		AccountID:     payload.AccountID,
		SkillID:       payload.SkillID,
		SkillRevision: skillRevision,
		Platform:      payload.Platform,
		AccountName:   payload.AccountName,
		Title:         payload.Title,
		ContentText:   payload.ContentText,
		MediaPayload:  mediaPayload,
		Status:        "pending",
		RunAt:         runAt,
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
			"skillRevision":    task.SkillRevision,
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

	device, account, skill, err := loadPublishTaskContextForOwner(r.Context(), h.app, user.ID, task)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task context")
		return
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
	runtimeState, err := h.app.Store.GetPublishTaskRuntimeStateByTaskID(r.Context(), task.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task runtime state")
		return
	}
	readiness := buildPublishTaskReadiness(r.Context(), h.app, task, device, account, skill)

	render.JSON(w, http.StatusOK, domain.PublishTaskWorkspace{
		Task:      *task,
		Device:    device,
		Account:   account,
		Skill:     skill,
		Events:    events,
		Artifacts: artifacts,
		Materials: materials,
		Actions:   computePublishTaskActions(task, len(materials)),
		Readiness: readiness,
		Runtime:   runtimeState,
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

func (h *TaskHandler) RefreshMaterials(w http.ResponseWriter, r *http.Request) {
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
	result, outcome, message, err := h.executeTaskMaterialRefresh(r.Context(), user.ID, task, true)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to refresh task materials")
		return
	}
	if outcome != "success" {
		if result != nil {
			render.JSON(w, http.StatusConflict, result)
			return
		}
		render.Error(w, http.StatusConflict, message)
		return
	}
	render.JSON(w, http.StatusOK, result)
}

func (h *TaskHandler) RefreshSkill(w http.ResponseWriter, r *http.Request) {
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
	result, outcome, message, err := h.executeTaskSkillRefresh(r.Context(), user.ID, task, true)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to refresh task skill revision")
		return
	}
	if outcome != "success" {
		render.Error(w, http.StatusConflict, message)
		return
	}
	render.JSON(w, http.StatusOK, result)
}

func normalizeBatchRepairOperations(input []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(input))
	for _, raw := range input {
		normalized := strings.TrimSpace(strings.ToLower(raw))
		switch normalized {
		case "refresh_materials", "refresh_skill":
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			items = append(items, normalized)
		}
	}
	return items
}

func (h *TaskHandler) selectTasksForBatchRepair(ctx context.Context, ownerUserID string, payload batchRepairTasksRequest) ([]domain.PublishTask, error) {
	selected := make([]domain.PublishTask, 0)
	seen := map[string]struct{}{}

	if len(payload.TaskIDs) > 0 {
		for _, rawID := range payload.TaskIDs {
			taskID := strings.TrimSpace(rawID)
			if taskID == "" {
				continue
			}
			if _, exists := seen[taskID]; exists {
				continue
			}
			task, err := h.app.Store.GetPublishTaskByOwner(ctx, taskID, ownerUserID)
			if err != nil {
				return nil, err
			}
			if task == nil {
				continue
			}
			selected = append(selected, *task)
			seen[taskID] = struct{}{}
			if payload.Limit > 0 && len(selected) >= payload.Limit {
				break
			}
		}
	} else {
		items, err := h.app.Store.ListPublishTasksByOwner(ctx, ownerUserID, store.ListPublishTasksFilter{
			DeviceID:    strings.TrimSpace(payload.DeviceID),
			Status:      strings.TrimSpace(payload.Status),
			Platform:    strings.TrimSpace(payload.Platform),
			AccountName: strings.TrimSpace(payload.AccountName),
			Limit:       payload.Limit,
		})
		if err != nil {
			return nil, err
		}
		selected = append(selected, items...)
	}

	if strings.TrimSpace(payload.SkillID) == "" && payload.Readiness == "" && payload.Dimension == "" && strings.TrimSpace(payload.IssueCode) == "" {
		return selected, nil
	}

	filtered := make([]domain.PublishTask, 0, len(selected))
	for _, task := range selected {
		if strings.TrimSpace(payload.SkillID) != "" {
			if task.SkillID == nil || strings.TrimSpace(*task.SkillID) != strings.TrimSpace(payload.SkillID) {
				continue
			}
		}
		diagnostic, err := buildPublishTaskDiagnosticItem(ctx, h.app, ownerUserID, &task)
		if err != nil {
			return nil, err
		}
		isReady := publishTaskReadinessAllowsExecution(diagnostic.Readiness)
		if payload.Readiness == "ready" && !isReady {
			continue
		}
		if payload.Readiness == "blocked" && isReady {
			continue
		}
		if payload.Dimension != "" {
			matched := false
			for _, dimension := range diagnostic.BlockingDimensions {
				if dimension == payload.Dimension {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if strings.TrimSpace(payload.IssueCode) != "" {
			matched := false
			for _, issueCode := range diagnostic.Readiness.IssueCodes {
				if issueCode == strings.TrimSpace(payload.IssueCode) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, task)
		if payload.Limit > 0 && len(filtered) >= payload.Limit {
			break
		}
	}
	return filtered, nil
}

func (h *TaskHandler) bulkRepairTask(ctx context.Context, ownerUserID string, task *domain.PublishTask, operations []string) (domain.PublishTaskBulkRepairItem, error) {
	device, account, skill, err := loadPublishTaskContextForOwner(ctx, h.app, ownerUserID, task)
	if err != nil {
		return domain.PublishTaskBulkRepairItem{}, err
	}
	readinessBefore := buildPublishTaskReadiness(ctx, h.app, task, device, account, skill)

	item := domain.PublishTaskBulkRepairItem{
		Task:              *task,
		Status:            "skipped",
		AppliedOperations: []string{},
		ReadinessBefore:   readinessBefore,
		ReadinessAfter:    readinessBefore,
	}

	for _, operation := range operations {
		switch operation {
		case "refresh_materials":
			result, outcome, message, opErr := h.executeTaskMaterialRefresh(ctx, ownerUserID, task, true)
			if opErr != nil {
				item.Status = "failed"
				item.Message = auditStringPtr(opErr.Error())
				return item, nil
			}
			item.MaterialRefresh = result
			if outcome == "success" {
				item.AppliedOperations = append(item.AppliedOperations, operation)
				item.Task = result.Task
				item.ReadinessAfter = result.Readiness
			} else if item.Message == nil && strings.TrimSpace(message) != "" {
				item.Message = auditStringPtr(message)
			}
		case "refresh_skill":
			result, outcome, message, opErr := h.executeTaskSkillRefresh(ctx, ownerUserID, task, true)
			if opErr != nil {
				item.Status = "failed"
				item.Message = auditStringPtr(opErr.Error())
				return item, nil
			}
			item.SkillRefresh = result
			if outcome == "success" {
				item.AppliedOperations = append(item.AppliedOperations, operation)
				item.Task = result.Task
				item.ReadinessAfter = result.Readiness
				task = &result.Task
			} else if item.Message == nil && strings.TrimSpace(message) != "" {
				item.Message = auditStringPtr(message)
			}
		}
	}

	if len(item.AppliedOperations) > 0 {
		item.Status = "success"
		if item.Message == nil {
			item.Message = auditStringPtr("批量修复已完成")
		}
	} else if item.Message == nil {
		item.Message = auditStringPtr("没有可应用的修复操作")
	}

	return item, nil
}

func (h *TaskHandler) executeTaskMaterialRefresh(ctx context.Context, ownerUserID string, task *domain.PublishTask, recordEvents bool) (*domain.PublishTaskMaterialRefreshResult, string, string, error) {
	if task == nil {
		return nil, "skipped", "任务不存在", nil
	}
	if task.Status == "running" || task.Status == "cancel_requested" {
		return nil, "skipped", "任务仍在执行或等待取消确认，暂时不能刷新素材快照", nil
	}

	existingRefs, err := h.app.Store.ListPublishTaskMaterialRefsByOwner(ctx, task.ID, ownerUserID)
	if err != nil {
		return nil, "failed", "", err
	}
	if len(existingRefs) == 0 {
		return nil, "skipped", "任务没有素材快照可刷新", nil
	}

	device, account, skill, err := loadPublishTaskContextForOwner(ctx, h.app, ownerUserID, task)
	if err != nil {
		return nil, "failed", "", err
	}

	refreshedInputs := make([]store.ReplacePublishTaskMaterialRefInput, 0, len(existingRefs))
	issues := make([]domain.PublishTaskMaterialRefreshIssue, 0)
	var changedCount int64
	for _, ref := range existingRefs {
		entry, entryErr := h.app.Store.GetMaterialEntryByOwner(ctx, ownerUserID, task.DeviceID, ref.RootName, ref.RelativePath)
		if entryErr != nil {
			return nil, "failed", "", entryErr
		}
		if entry == nil || !entry.IsAvailable {
			refCopy := ref
			issues = append(issues, domain.PublishTaskMaterialRefreshIssue{
				Code:         "material_missing",
				Message:      "素材已缺失或尚未同步，无法刷新任务快照",
				RootName:     ref.RootName,
				RelativePath: ref.RelativePath,
				Role:         ref.Role,
				PreviousRef:  &refCopy,
				CurrentEntry: entry,
			})
			continue
		}
		if !publishTaskMaterialRefMatchesEntry(ref, entry) {
			changedCount++
		}
		refreshedInputs = append(refreshedInputs, buildPublishTaskMaterialRefInput(task.ID, task.DeviceID, ref.Role, entry))
	}

	if len(issues) > 0 {
		readiness := buildPublishTaskReadiness(ctx, h.app, task, device, account, skill)
		return &domain.PublishTaskMaterialRefreshResult{
			Task:           *task,
			Materials:      existingRefs,
			Readiness:      readiness,
			RefreshedCount: 0,
			ChangedCount:   changedCount,
			MissingCount:   int64(len(issues)),
			Issues:         issues,
		}, "skipped", "部分素材已缺失，无法刷新任务素材快照", nil
	}

	refreshedRefs, err := h.app.Store.ReplacePublishTaskMaterialRefs(ctx, task.ID, ownerUserID, refreshedInputs)
	if err != nil {
		return nil, "failed", "", err
	}
	readiness := buildPublishTaskReadiness(ctx, h.app, task, device, account, skill)

	if recordEvents {
		_, _ = h.app.Store.CreatePublishTaskEvent(ctx, store.CreatePublishTaskEventInput{
			ID:        uuid.NewString(),
			TaskID:    task.ID,
			EventType: "materials_refreshed",
			Source:    "cloud",
			Status:    task.Status,
			Message:   auditStringPtr("任务素材快照已刷新"),
			Payload: mustJSONBytes(map[string]any{
				"refreshedCount": len(refreshedRefs),
				"changedCount":   changedCount,
			}),
		})
		recordAuditEvent(h.app, ctx, store.CreateAuditEventInput{
			OwnerUserID:  ownerUserID,
			ResourceType: "publish_task",
			ResourceID:   &task.ID,
			Action:       "refresh_materials",
			Title:        "刷新任务素材快照",
			Source:       task.Platform,
			Status:       task.Status,
			Message:      auditStringPtr("任务素材快照已刷新"),
			Payload: mustJSONBytes(map[string]any{
				"refreshedCount": len(refreshedRefs),
				"changedCount":   changedCount,
			}),
		})
	}

	return &domain.PublishTaskMaterialRefreshResult{
		Task:           *task,
		Materials:      refreshedRefs,
		Readiness:      readiness,
		RefreshedCount: int64(len(refreshedRefs)),
		ChangedCount:   changedCount,
		MissingCount:   0,
		Issues:         issues,
	}, "success", "任务素材快照已刷新", nil
}

func (h *TaskHandler) executeTaskSkillRefresh(ctx context.Context, ownerUserID string, task *domain.PublishTask, recordEvents bool) (*domain.PublishTaskSkillRefreshResult, string, string, error) {
	if task == nil {
		return nil, "skipped", "任务不存在", nil
	}
	if task.Status == "running" || task.Status == "cancel_requested" {
		return nil, "skipped", "任务仍在执行或等待取消确认，暂时不能刷新技能版本快照", nil
	}
	if task.SkillID == nil || strings.TrimSpace(*task.SkillID) == "" {
		return nil, "skipped", "任务未绑定云端技能", nil
	}

	skill, err := h.app.Store.GetOwnedSkillByID(ctx, strings.TrimSpace(*task.SkillID), ownerUserID)
	if err != nil {
		return nil, "failed", "", err
	}
	if skill == nil {
		return nil, "skipped", "任务绑定的技能已不存在", nil
	}

	revision, err := h.app.Store.GetSkillRevision(ctx, skill.ID, ownerUserID)
	if err != nil {
		return nil, "failed", "", err
	}
	currentRevision := normalizeTrimmedStringPtr(revision)
	previousRevision := task.SkillRevision

	refreshedTask, err := h.app.Store.RefreshPublishTaskSkillRevision(ctx, task.ID, ownerUserID, currentRevision, auditStringPtr("任务技能版本快照已刷新"))
	if err != nil {
		return nil, "failed", "", err
	}
	if refreshedTask == nil {
		return nil, "skipped", "任务不存在", nil
	}

	device, account, skill, err := loadPublishTaskContextForOwner(ctx, h.app, ownerUserID, refreshedTask)
	if err != nil {
		return nil, "failed", "", err
	}
	readiness := buildPublishTaskReadiness(ctx, h.app, refreshedTask, device, account, skill)
	revisionChanged := trimmedStringValue(previousRevision) != trimmedStringValue(currentRevision)

	if recordEvents {
		_, _ = h.app.Store.CreatePublishTaskEvent(ctx, store.CreatePublishTaskEventInput{
			ID:        uuid.NewString(),
			TaskID:    refreshedTask.ID,
			EventType: "skill_refreshed",
			Source:    "cloud",
			Status:    refreshedTask.Status,
			Message:   auditStringPtr("任务技能版本快照已刷新"),
			Payload: mustJSONBytes(map[string]any{
				"previousRevision": previousRevision,
				"currentRevision":  currentRevision,
				"revisionChanged":  revisionChanged,
			}),
		})
		recordAuditEvent(h.app, ctx, store.CreateAuditEventInput{
			OwnerUserID:  ownerUserID,
			ResourceType: "publish_task",
			ResourceID:   &refreshedTask.ID,
			Action:       "refresh_skill",
			Title:        "刷新任务技能版本快照",
			Source:       refreshedTask.Platform,
			Status:       refreshedTask.Status,
			Message:      auditStringPtr("任务技能版本快照已刷新"),
			Payload: mustJSONBytes(map[string]any{
				"previousRevision": previousRevision,
				"currentRevision":  currentRevision,
				"revisionChanged":  revisionChanged,
			}),
		})
	}

	return &domain.PublishTaskSkillRefreshResult{
		Task:             *refreshedTask,
		Skill:            skill,
		Readiness:        readiness,
		PreviousRevision: previousRevision,
		CurrentRevision:  currentRevision,
		RevisionChanged:  revisionChanged,
	}, "success", "任务技能版本快照已刷新", nil
}

func summarizeBulkRepairItems(items []domain.PublishTaskBulkRepairItem) domain.PublishTaskBulkRepairSummary {
	summary := domain.PublishTaskBulkRepairSummary{
		ByStatus:    map[string]int64{},
		ByOperation: map[string]int64{},
	}
	for _, item := range items {
		summary.SelectedCount++
		summary.ProcessedCount++
		summary.ByStatus[item.Status]++
		switch item.Status {
		case "success":
			summary.SuccessCount++
		case "failed":
			summary.FailedCount++
		default:
			summary.SkippedCount++
		}
		for _, operation := range item.AppliedOperations {
			summary.ByOperation[operation]++
		}
	}
	return summary
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
	if err := h.app.Store.DeletePublishTaskRuntimeState(r.Context(), task.ID); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to clear previous task runtime state")
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
	if err := h.app.Store.DeletePublishTaskRuntimeState(r.Context(), task.ID); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to clear task runtime state")
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

func (h *TaskHandler) Resume(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	var payload resumeTaskRequest
	if err := render.DecodeJSON(r, &payload); err != nil && err.Error() != "EOF" {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.Message = normalizeTrimmedString(payload.Message)

	task, err := h.app.Store.ResumePublishTaskFromVerification(r.Context(), taskID, user.ID, payload.Message)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to resume task")
		return
	}
	if task == nil {
		render.Error(w, http.StatusConflict, "Task is not resumable from needs_verify")
		return
	}
	if err := h.app.Store.DeletePublishTaskRuntimeState(r.Context(), task.ID); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to clear task runtime state")
		return
	}

	message := payload.Message
	if message == nil {
		message = auditStringPtr("人工验证后任务已恢复为待执行")
	}
	_, _ = h.app.Store.CreatePublishTaskEvent(r.Context(), store.CreatePublishTaskEventInput{
		ID:        uuid.NewString(),
		TaskID:    task.ID,
		EventType: "resumed",
		Source:    "cloud",
		Status:    task.Status,
		Message:   message,
	})
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "publish_task",
		ResourceID:   &task.ID,
		Action:       "resume",
		Title:        "恢复发布任务",
		Source:       task.Platform,
		Status:       task.Status,
		Message:      message,
	})

	render.JSON(w, http.StatusOK, task)
}

func (h *TaskHandler) ManualResolve(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	var payload manualResolveTaskRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.Status = strings.TrimSpace(payload.Status)
	payload.Message = normalizeTrimmedString(payload.Message)
	payload.TextEvidence = normalizeTrimmedString(payload.TextEvidence)
	if payload.Status != "success" && payload.Status != "completed" && payload.Status != "failed" && payload.Status != "cancelled" {
		render.Error(w, http.StatusBadRequest, "status must be one of success, completed, failed, cancelled")
		return
	}

	task, err := h.app.Store.ResolvePublishTaskManually(r.Context(), taskID, user.ID, payload.Status, payload.Message)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to resolve task manually")
		return
	}
	if task == nil {
		render.Error(w, http.StatusConflict, "Task is not manually resolvable")
		return
	}

	var artifactCount int
	if payload.TextEvidence != nil || payload.Payload != nil {
		var artifactPayload []byte
		if payload.Payload != nil {
			artifactPayload = mustJSONBytes(payload.Payload)
		}
		title := "人工处理记录"
		if _, err := h.app.Store.UpsertPublishTaskArtifacts(r.Context(), []store.UpsertPublishTaskArtifactInput{{
			TaskID:       task.ID,
			ArtifactKey:  "manual-resolution",
			ArtifactType: "manual_note",
			Source:       "cloud",
			Title:        &title,
			TextContent:  payload.TextEvidence,
			Payload:      artifactPayload,
		}}); err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to persist manual resolution evidence")
			return
		}
		artifactCount = 1
	}

	message := payload.Message
	if message == nil {
		switch payload.Status {
		case "success", "completed":
			message = auditStringPtr("任务已人工处理完成")
		case "failed":
			message = auditStringPtr("任务已人工标记为失败")
		case "cancelled":
			message = auditStringPtr("任务已人工取消")
		}
	}
	_, _ = h.app.Store.CreatePublishTaskEvent(r.Context(), store.CreatePublishTaskEventInput{
		ID:        uuid.NewString(),
		TaskID:    task.ID,
		EventType: "manual_resolved",
		Source:    "cloud",
		Status:    task.Status,
		Message:   message,
		Payload: mustJSONBytes(map[string]any{
			"artifactCount": artifactCount,
			"status":        payload.Status,
		}),
	})
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "publish_task",
		ResourceID:   &task.ID,
		Action:       "manual_resolve",
		Title:        "人工处理发布任务",
		Source:       task.Platform,
		Status:       task.Status,
		Message:      message,
		Payload: mustJSONBytes(map[string]any{
			"artifactCount": artifactCount,
			"status":        payload.Status,
		}),
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

		items = append(items, buildPublishTaskMaterialRefInput(taskID, deviceID, role, entry))
	}

	return items, nil
}

func buildPublishTaskMaterialRefInput(taskID string, deviceID string, role string, entry *domain.MaterialEntry) store.ReplacePublishTaskMaterialRefInput {
	return store.ReplacePublishTaskMaterialRefInput{
		TaskID:       taskID,
		DeviceID:     deviceID,
		RootName:     entry.RootName,
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
	}
}

func publishTaskMaterialRefMatchesEntry(ref domain.PublishTaskMaterialRef, entry *domain.MaterialEntry) bool {
	if entry == nil {
		return false
	}
	return ref.RootName == entry.RootName &&
		ref.RelativePath == entry.RelativePath &&
		ref.Name == entry.Name &&
		ref.Kind == entry.Kind &&
		trimmedStringValue(ref.AbsolutePath) == trimmedStringValue(entry.AbsolutePath) &&
		trimmedInt64Value(ref.SizeBytes) == trimmedInt64Value(entry.SizeBytes) &&
		trimmedStringValue(ref.ModifiedAt) == trimmedStringValue(entry.ModifiedAt) &&
		trimmedStringValue(ref.Extension) == trimmedStringValue(entry.Extension) &&
		trimmedStringValue(ref.MimeType) == trimmedStringValue(entry.MimeType) &&
		ref.IsText == entry.IsText &&
		trimmedStringValue(ref.PreviewText) == trimmedStringValue(entry.PreviewText)
}

func trimmedStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func trimmedInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
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

func computePublishTaskActions(task *domain.PublishTask, materialCount int) domain.PublishTaskActionState {
	if task == nil {
		return domain.PublishTaskActionState{}
	}

	canRefreshSkill := task.SkillID != nil && strings.TrimSpace(*task.SkillID) != ""
	canRefreshMaterials := materialCount > 0

	switch task.Status {
	case "pending":
		return domain.PublishTaskActionState{CanEdit: true, CanCancel: true, CanRetry: false, CanDelete: true, CanForceRelease: false, CanResume: false, CanResolveManual: false, CanRefreshMaterials: canRefreshMaterials, CanRefreshSkill: canRefreshSkill}
	case "running", "cancel_requested":
		return domain.PublishTaskActionState{CanEdit: false, CanCancel: task.Status == "running", CanRetry: false, CanDelete: false, CanForceRelease: true, CanResume: false, CanResolveManual: false, CanRefreshMaterials: false, CanRefreshSkill: false}
	case "needs_verify":
		return domain.PublishTaskActionState{CanEdit: true, CanCancel: true, CanRetry: true, CanDelete: true, CanForceRelease: false, CanResume: true, CanResolveManual: true, CanRefreshMaterials: canRefreshMaterials, CanRefreshSkill: canRefreshSkill}
	case "failed", "cancelled", "success", "completed":
		return domain.PublishTaskActionState{CanEdit: true, CanCancel: false, CanRetry: true, CanDelete: true, CanForceRelease: false, CanResume: false, CanResolveManual: false, CanRefreshMaterials: canRefreshMaterials, CanRefreshSkill: canRefreshSkill}
	default:
		return domain.PublishTaskActionState{CanEdit: true, CanCancel: false, CanRetry: false, CanDelete: true, CanForceRelease: false, CanResume: false, CanResolveManual: false, CanRefreshMaterials: canRefreshMaterials, CanRefreshSkill: canRefreshSkill}
	}
}
