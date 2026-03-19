package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
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

type SkillHandler struct {
	app *appstate.App
}

type createSkillRequest struct {
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	OutputType        string      `json:"outputType"`
	ModelName         string      `json:"modelName"`
	PromptTemplate    *string     `json:"promptTemplate"`
	ReferencePayload  interface{} `json:"referencePayload"`
	DeviceID          *string     `json:"deviceId"`
	ExecutionTime     *string     `json:"executionTime"`
	RepeatDaily       *bool       `json:"repeatDaily"`
	StoryboardEnabled *bool       `json:"storyboardEnabled"`
	IsEnabled         *bool       `json:"isEnabled"`
}

type updateSkillRequest struct {
	Name              *string     `json:"name"`
	Description       *string     `json:"description"`
	OutputType        *string     `json:"outputType"`
	ModelName         *string     `json:"modelName"`
	PromptTemplate    *string     `json:"promptTemplate"`
	ReferencePayload  interface{} `json:"referencePayload"`
	DeviceID          *string     `json:"deviceId"`
	ExecutionTime     *string     `json:"executionTime"`
	RepeatDaily       *bool       `json:"repeatDaily"`
	StoryboardEnabled *bool       `json:"storyboardEnabled"`
	IsEnabled         *bool       `json:"isEnabled"`
}

type createSkillAssetRequest struct {
	AssetType  string  `json:"assetType"`
	FileName   string  `json:"fileName"`
	MimeType   *string `json:"mimeType"`
	StorageKey *string `json:"storageKey"`
	PublicURL  *string `json:"publicUrl"`
	SizeBytes  *int64  `json:"sizeBytes"`
}

func NewSkillHandler(app *appstate.App) *SkillHandler {
	return &SkillHandler{app: app}
}

func parseSkillExecutionTime(raw string, now time.Time) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}

	localNow := now.In(time.Local)
	for _, layout := range []string{"15:04:05", "15:04"} {
		parsed, err := time.Parse(layout, value)
		if err != nil {
			continue
		}
		next := time.Date(
			localNow.Year(),
			localNow.Month(),
			localNow.Day(),
			parsed.Hour(),
			parsed.Minute(),
			parsed.Second(),
			0,
			localNow.Location(),
		)
		if !next.After(localNow) {
			next = next.Add(24 * time.Hour)
		}
		nextUTC := next.UTC()
		return &nextUTC, nil
	}

	return nil, fmt.Errorf("executionTime must be RFC3339 or HH:MM[:SS]")
}

func (h *SkillHandler) List(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	items, err := h.app.Store.ListSkillsByOwner(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skills")
		return
	}
	if deviceID := strings.TrimSpace(r.URL.Query().Get("deviceId")); deviceID != "" {
		filtered := make([]domain.ProductSkill, 0, len(items))
		for _, item := range items {
			if item.DeviceID != nil && strings.TrimSpace(*item.DeviceID) == deviceID {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *SkillHandler) Detail(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
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

	render.JSON(w, http.StatusOK, skill)
}

func (h *SkillHandler) Workspace(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
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

	assets, err := h.app.Store.ListSkillAssets(r.Context(), skillID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill assets")
		return
	}

	recentTasks, err := h.app.Store.ListPublishTasksBySkill(r.Context(), user.ID, skillID, 8)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill tasks")
		return
	}
	recentAIJobs, err := h.app.Store.ListAIJobsBySkill(r.Context(), user.ID, skillID, 8)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill AI jobs")
		return
	}
	deviceSyncs, err := h.app.Store.ListSkillSyncStatesBySkill(r.Context(), user.ID, skillID, 12)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill sync states")
		return
	}
	deviceSyncs, err = decorateSkillSyncStatesWithCurrentRevision(r.Context(), h.app, user.ID, deviceSyncs)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to decorate skill sync states")
		return
	}

	render.JSON(w, http.StatusOK, domain.ProductSkillWorkspace{
		Skill:        *skill,
		Assets:       assets,
		RecentTasks:  recentTasks,
		RecentAIJobs: recentAIJobs,
		DeviceSyncs:  deviceSyncs,
	})
}

func (h *SkillHandler) Impact(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
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

	limit := 0
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsed < 0 {
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
	issueCodeFilter := strings.TrimSpace(r.URL.Query().Get("issueCode"))
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

	tasks, err := h.app.Store.ListPublishTasksBySkill(r.Context(), user.ID, skillID, 0)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load impacted tasks")
		return
	}

	items := make([]domain.PublishTaskDiagnosticItem, 0, len(tasks))
	for _, task := range tasks {
		if statusFilter != "" && task.Status != statusFilter {
			continue
		}
		item, diagErr := buildPublishTaskDiagnosticItem(r.Context(), h.app, user.ID, &task)
		if diagErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to build skill impact diagnostics")
			return
		}
		isReady := publishTaskReadinessAllowsExecution(item.Readiness)
		if readinessFilter == "ready" && !isReady {
			continue
		}
		if readinessFilter == "blocked" && isReady {
			continue
		}
		if issueCodeFilter != "" {
			matched := false
			for _, issueCode := range item.Readiness.IssueCodes {
				if issueCode == issueCodeFilter {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		items = append(items, item)
		if limit > 0 && len(items) >= limit {
			break
		}
	}

	render.JSON(w, http.StatusOK, domain.ProductSkillImpactWorkspace{
		Skill:      *skill,
		Items:      items,
		Summary:    summarizePublishTaskDiagnosticItems(items),
		ServerTime: time.Now().UTC(),
	})
}

func (h *SkillHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	var payload createSkillRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Description = strings.TrimSpace(payload.Description)
	payload.OutputType = strings.TrimSpace(payload.OutputType)
	payload.ModelName = strings.TrimSpace(payload.ModelName)
	if payload.Name == "" || payload.Description == "" || payload.OutputType == "" || payload.ModelName == "" {
		render.Error(w, http.StatusBadRequest, "name, description, outputType, and modelName are required")
		return
	}
	var deviceID *string
	if payload.DeviceID != nil && strings.TrimSpace(*payload.DeviceID) != "" {
		trimmed := strings.TrimSpace(*payload.DeviceID)
		device, err := h.app.Store.GetOwnedDevice(r.Context(), trimmed, user.ID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to validate device")
			return
		}
		if device == nil {
			render.Error(w, http.StatusNotFound, "Device not found")
			return
		}
		deviceID = &trimmed
	}
	var executionTime *time.Time
	var nextRunAt *time.Time
	if payload.ExecutionTime != nil && strings.TrimSpace(*payload.ExecutionTime) != "" {
		parsed, err := parseSkillExecutionTime(strings.TrimSpace(*payload.ExecutionTime), time.Now())
		if err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		executionTime = parsed
		nextRunAt = parsed
	}
	repeatDaily := payload.RepeatDaily != nil && *payload.RepeatDaily
	storyboardEnabled := true
	if payload.StoryboardEnabled != nil {
		storyboardEnabled = *payload.StoryboardEnabled
	}

	var referenceBytes []byte
	var err error
	if payload.ReferencePayload != nil {
		referenceBytes, err = json.Marshal(payload.ReferencePayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "referencePayload must be valid json")
			return
		}
	}

	isEnabled := true
	if payload.IsEnabled != nil {
		isEnabled = *payload.IsEnabled
	}

	skill, err := h.app.Store.CreateSkill(r.Context(), store.CreateSkillInput{
		ID:                uuid.NewString(),
		OwnerUserID:       user.ID,
		DeviceID:          deviceID,
		Name:              payload.Name,
		Description:       payload.Description,
		OutputType:        payload.OutputType,
		ModelName:         payload.ModelName,
		PromptTemplate:    payload.PromptTemplate,
		ReferencePayload:  referenceBytes,
		ExecutionTime:     executionTime,
		RepeatDaily:       repeatDaily,
		StoryboardEnabled: storyboardEnabled,
		NextRunAt:         nextRunAt,
		IsEnabled:         isEnabled,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create skill")
		return
	}
	skill, err = h.app.Store.GetOwnedSkillByID(r.Context(), skill.ID, user.ID)
	if err != nil || skill == nil {
		render.Error(w, http.StatusInternalServerError, "Failed to reload created skill")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "skill",
		ResourceID:   &skill.ID,
		Action:       "create",
		Title:        "创建产品技能",
		Source:       skill.OutputType,
		Status:       "success",
		Message:      auditStringPtr("产品技能已创建"),
		Payload: mustJSONBytes(map[string]any{
			"name":      skill.Name,
			"modelName": skill.ModelName,
			"deviceId":  skill.DeviceID,
		}),
	})

	render.JSON(w, http.StatusCreated, skill)
}

func (h *SkillHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
	if skillID == "" {
		render.Error(w, http.StatusBadRequest, "skillId is required")
		return
	}

	var payload updateSkillRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var referenceBytes []byte
	var err error
	referenceTouched := payload.ReferencePayload != nil
	if referenceTouched {
		referenceBytes, err = json.Marshal(payload.ReferencePayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "referencePayload must be valid json")
			return
		}
	}
	var deviceID *string
	deviceTouched := payload.DeviceID != nil
	if deviceTouched {
		trimmed := strings.TrimSpace(*payload.DeviceID)
		if trimmed != "" {
			device, err := h.app.Store.GetOwnedDevice(r.Context(), trimmed, user.ID)
			if err != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to validate device")
				return
			}
			if device == nil {
				render.Error(w, http.StatusNotFound, "Device not found")
				return
			}
			deviceID = &trimmed
		}
	}
	var executionTime *time.Time
	executionTouched := payload.ExecutionTime != nil
	if executionTouched && strings.TrimSpace(*payload.ExecutionTime) != "" {
		parsed, err := parseSkillExecutionTime(strings.TrimSpace(*payload.ExecutionTime), time.Now())
		if err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		executionTime = parsed
	}
	var nextRunAt *time.Time
	if executionTouched {
		nextRunAt = executionTime
	}

	skill, err := h.app.Store.UpdateSkill(r.Context(), skillID, user.ID, store.UpdateSkillInput{
		Name:              payload.Name,
		Description:       payload.Description,
		OutputType:        payload.OutputType,
		ModelName:         payload.ModelName,
		PromptTemplate:    payload.PromptTemplate,
		ReferencePayload:  referenceBytes,
		ReferenceTouched:  referenceTouched,
		DeviceID:          deviceID,
		DeviceTouched:     deviceTouched,
		ExecutionTime:     executionTime,
		ExecutionTouched:  executionTouched,
		RepeatDaily:       payload.RepeatDaily,
		StoryboardEnabled: payload.StoryboardEnabled,
		NextRunAt:         nextRunAt,
		NextRunTouched:    executionTouched,
		IsEnabled:         payload.IsEnabled,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update skill")
		return
	}
	if skill == nil {
		render.Error(w, http.StatusNotFound, "Skill not found")
		return
	}
	skill, err = h.app.Store.GetOwnedSkillByID(r.Context(), skill.ID, user.ID)
	if err != nil || skill == nil {
		render.Error(w, http.StatusInternalServerError, "Failed to reload updated skill")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "skill",
		ResourceID:   &skill.ID,
		Action:       "update",
		Title:        "更新产品技能",
		Source:       skill.OutputType,
		Status:       "success",
		Message:      auditStringPtr("产品技能已更新"),
		Payload: mustJSONBytes(map[string]any{
			"name":              payload.Name,
			"modelName":         payload.ModelName,
			"storyboardEnabled": payload.StoryboardEnabled,
			"isEnabled":         payload.IsEnabled,
		}),
	})

	render.JSON(w, http.StatusOK, skill)
}

func (h *SkillHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
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

	taskCount, accountCount, aiJobCount, err := h.app.Store.GetSkillUsageSummary(r.Context(), skillID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect skill usage")
		return
	}
	if taskCount > 0 || aiJobCount > 0 {
		render.JSON(w, http.StatusConflict, map[string]any{
			"error": "Skill is still referenced by publish tasks or AI jobs",
			"usage": map[string]any{
				"publishTaskCount":     taskCount,
				"distinctAccountCount": accountCount,
				"aiJobCount":           aiJobCount,
			},
		})
		return
	}

	assets, err := h.app.Store.ListSkillAssets(r.Context(), skillID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect skill assets")
		return
	}

	deleted, err := h.app.Store.DeleteSkill(r.Context(), skillID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to delete skill")
		return
	}
	if !deleted {
		render.Error(w, http.StatusNotFound, "Skill not found")
		return
	}
	cleanupSkillAssetFiles(h.app, r.Context(), assets)
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "skill",
		ResourceID:   &skillID,
		Action:       "delete",
		Title:        "删除产品技能",
		Source:       skill.OutputType,
		Status:       "success",
		Message:      auditStringPtr("产品技能已删除"),
		Payload: mustJSONBytes(map[string]any{
			"name":       skill.Name,
			"outputType": skill.OutputType,
		}),
	})

	render.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *SkillHandler) ListAssets(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
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

	items, err := h.app.Store.ListSkillAssets(r.Context(), skillID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill assets")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *SkillHandler) CreateAsset(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
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

	var payload createSkillAssetRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.AssetType = strings.TrimSpace(payload.AssetType)
	payload.FileName = sanitizeUploadFilename(payload.FileName)
	if payload.AssetType == "" || payload.FileName == "" {
		render.Error(w, http.StatusBadRequest, "assetType and fileName are required")
		return
	}

	managedRef, transferred, err := normalizeManagedObjectRef(r.Context(), h.app, fmt.Sprintf("skills/%s/%s", user.ID, skillID), managedObjectRef{
		FileName:   payload.FileName,
		MimeType:   payload.MimeType,
		StorageKey: payload.StorageKey,
		PublicURL:  payload.PublicURL,
		SizeBytes:  payload.SizeBytes,
	})
	if err != nil {
		render.Error(w, http.StatusBadGateway, "Failed to mirror remote asset into storage")
		return
	}
	payload.MimeType = managedRef.MimeType
	payload.StorageKey = managedRef.StorageKey
	payload.PublicURL = managedRef.PublicURL
	payload.SizeBytes = managedRef.SizeBytes

	asset, err := h.app.Store.CreateSkillAsset(r.Context(), store.CreateSkillAssetInput{
		ID:          uuid.NewString(),
		SkillID:     skillID,
		OwnerUserID: user.ID,
		AssetType:   payload.AssetType,
		FileName:    payload.FileName,
		MimeType:    payload.MimeType,
		StorageKey:  payload.StorageKey,
		PublicURL:   payload.PublicURL,
		SizeBytes:   payload.SizeBytes,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create skill asset")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "skill_asset",
		ResourceID:   &asset.ID,
		Action:       "create",
		Title:        "添加技能资产",
		Source:       payload.AssetType,
		Status:       "success",
		Message:      auditStringPtr(skillAssetAuditMessage(transferred)),
		Payload: mustJSONBytes(map[string]any{
			"skillId":  skillID,
			"fileName": asset.FileName,
		}),
	})

	render.JSON(w, http.StatusCreated, asset)
}

func cleanupSkillAssetFiles(app *appstate.App, ctx context.Context, assets []domain.ProductSkillAsset) {
	if app == nil || app.Storage == nil {
		return
	}
	seen := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		if asset.StorageKey == nil || strings.TrimSpace(*asset.StorageKey) == "" {
			continue
		}
		key := strings.TrimSpace(*asset.StorageKey)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		_ = app.Storage.DeleteObject(ctx, key)
	}
}

func (h *SkillHandler) UploadAsset(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
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

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		render.Error(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	assetType := strings.TrimSpace(r.FormValue("assetType"))
	if assetType == "" {
		render.Error(w, http.StatusBadRequest, "assetType is required")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		render.Error(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 32<<20))
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to read file")
		return
	}

	fileName := sanitizeUploadFilename(header.Filename)
	contentType := header.Header.Get("Content-Type")
	object, err := h.app.Storage.SaveBytes(
		r.Context(),
		fmt.Sprintf("skills/%s/%s/%s-%s", user.ID, skillID, uuid.NewString(), fileName),
		contentType,
		data,
	)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to store file")
		return
	}

	asset, err := h.app.Store.CreateSkillAsset(r.Context(), store.CreateSkillAssetInput{
		ID:          uuid.NewString(),
		SkillID:     skillID,
		OwnerUserID: user.ID,
		AssetType:   assetType,
		FileName:    fileName,
		MimeType:    &object.ContentType,
		StorageKey:  &object.StorageKey,
		PublicURL:   &object.PublicURL,
		SizeBytes:   &object.SizeBytes,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create asset metadata")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "skill_asset",
		ResourceID:   &asset.ID,
		Action:       "upload",
		Title:        "上传技能资产",
		Source:       assetType,
		Status:       "success",
		Message:      auditStringPtr("技能资产文件已上传"),
		Payload: mustJSONBytes(map[string]any{
			"skillId":   skillID,
			"fileName":  asset.FileName,
			"publicUrl": asset.PublicURL,
		}),
	})

	render.JSON(w, http.StatusCreated, asset)
}

func (h *SkillHandler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
	assetID := strings.TrimSpace(chi.URLParam(r, "assetId"))
	if skillID == "" || assetID == "" {
		render.Error(w, http.StatusBadRequest, "skillId and assetId are required")
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

	asset, err := h.app.Store.DeleteSkillAsset(r.Context(), skillID, assetID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to delete skill asset")
		return
	}
	if asset == nil {
		render.Error(w, http.StatusNotFound, "Skill asset not found")
		return
	}

	cleanupSkillAssetFiles(h.app, r.Context(), []domain.ProductSkillAsset{*asset})
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "skill_asset",
		ResourceID:   &asset.ID,
		Action:       "delete",
		Title:        "删除技能资产",
		Source:       asset.AssetType,
		Status:       "success",
		Message:      auditStringPtr("技能资产已删除"),
		Payload: mustJSONBytes(map[string]any{
			"skillId":   skillID,
			"fileName":  asset.FileName,
			"publicUrl": asset.PublicURL,
		}),
	})

	render.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func sanitizeUploadFilename(fileName string) string {
	base := strings.TrimSpace(filepath.Base(fileName))
	if base == "" || base == "." || base == "/" {
		return "file.bin"
	}
	return strings.ReplaceAll(base, " ", "_")
}

func skillAssetAuditMessage(transferred bool) string {
	if transferred {
		return "技能资产已从远程地址转存到对象存储"
	}
	return "技能资产元数据已创建"
}
