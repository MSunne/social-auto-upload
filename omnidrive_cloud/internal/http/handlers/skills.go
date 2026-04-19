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
	"omnidrive_cloud/internal/workflow"
)

type SkillHandler struct {
	app *appstate.App
}

type createSkillRequest struct {
	Name                     string      `json:"name"`
	Description              string      `json:"description"`
	OutputType               string      `json:"outputType"`
	ModelName                string      `json:"modelName"`
	FixedDurationSeconds     *int        `json:"fixedDurationSeconds"`
	PromptTemplate           *string     `json:"promptTemplate"`
	StoryboardPromptTemplate *string     `json:"storyboardPromptTemplate"`
	PublishPromptTemplate    *string     `json:"publishPromptTemplate"`
	PublishIntroEnabled      *bool       `json:"publishIntroEnabled"`
	CoverPromptTemplate      *string     `json:"coverPromptTemplate"`
	Topics                   []string    `json:"topics"`
	ReferencePayload         interface{} `json:"referencePayload"`
	DeviceID                 *string     `json:"deviceId"`
	ExecutionTime            *string     `json:"executionTime"`
	RepeatDaily              *bool       `json:"repeatDaily"`
	StoryboardEnabled        *bool       `json:"storyboardEnabled"`
	IsEnabled                *bool       `json:"isEnabled"`
}

type updateSkillRequest struct {
	Name                     *string     `json:"name"`
	Description              *string     `json:"description"`
	OutputType               *string     `json:"outputType"`
	ModelName                *string     `json:"modelName"`
	FixedDurationSeconds     *int        `json:"fixedDurationSeconds"`
	PromptTemplate           *string     `json:"promptTemplate"`
	StoryboardPromptTemplate *string     `json:"storyboardPromptTemplate"`
	PublishPromptTemplate    *string     `json:"publishPromptTemplate"`
	PublishIntroEnabled      *bool       `json:"publishIntroEnabled"`
	CoverPromptTemplate      *string     `json:"coverPromptTemplate"`
	Topics                   []string    `json:"topics"`
	ReferencePayload         interface{} `json:"referencePayload"`
	DeviceID                 *string     `json:"deviceId"`
	ExecutionTime            *string     `json:"executionTime"`
	RepeatDaily              *bool       `json:"repeatDaily"`
	StoryboardEnabled        *bool       `json:"storyboardEnabled"`
	IsEnabled                *bool       `json:"isEnabled"`
}

type createSkillAssetRequest struct {
	AssetType  string  `json:"assetType"`
	FileName   string  `json:"fileName"`
	MimeType   *string `json:"mimeType"`
	StorageKey *string `json:"storageKey"`
	PublicURL  *string `json:"publicUrl"`
	SizeBytes  *int64  `json:"sizeBytes"`
}

type skillEditorDefaultsResponse struct {
	CoverPromptTemplateDefault  string                      `json:"coverPromptTemplateDefault"`
	MixVideoScriptRewritePrompt string                      `json:"mixVideoScriptRewritePrompt"`
	MixVideoPublishIntroPrompt  string                      `json:"mixVideoPublishIntroPrompt"`
	VideoTextDurationOptions    []skillEditorDurationOption `json:"videoTextDurationOptions"`
}

type skillEditorDurationOption struct {
	RuleID              string `json:"ruleId"`
	DurationSeconds     int    `json:"durationSeconds"`
	SegmentSeconds      int    `json:"segmentSeconds"`
	SpecialPriceCredits *int64 `json:"specialPriceCredits,omitempty"`
	Label               string `json:"label"`
}

type skillFixedDurationConfig struct {
	DurationSeconds int
	SegmentSeconds  int
	Rule            *domain.WorkflowDurationRule
}

// 创建技能Handler相关实例，组装运行所需依赖并返回给上层流程复用。
func NewSkillHandler(app *appstate.App) *SkillHandler {
	return &SkillHandler{app: app}
}

// 处理清洗技能调度相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func sanitizeSkillSchedule(skill *domain.ProductSkill) {
	if skill == nil {
		return
	}
	skill.ExecutionTime = nil
	skill.RepeatDaily = false
	skill.NextRunAt = nil
}

const videoTextWorkflowCode = "video_text"

// 判断是否属于视频Text输出Type，供当前链路选择后续处理策略。
func isVideoTextOutputType(outputType string) bool {
	switch workflow.NormalizeSkillOutputType(outputType) {
	case "video", "video_text", "视文模式":
		return true
	default:
		return false
	}
}

// 根据技能计算工作流输出Type，供技能链路复用关键派生结果。
func workflowOutputTypeForSkill(outputType string) string {
	return workflow.NormalizeSkillOutputType(outputType)
}

// 规范化Fixed时长Seconds，统一技能链路的输入格式和后续处理行为。
func normalizeFixedDurationSeconds(value *int) *int {
	if value == nil || *value <= 0 {
		return nil
	}
	return value
}

// 解析技能Fixed时长配置，根据当前配置和上下文确定最终使用结果。
func resolveSkillFixedDurationConfig(ctx context.Context, app *appstate.App, modelName string, fixedDurationSeconds *int) (*skillFixedDurationConfig, error) {
	normalized := normalizeFixedDurationSeconds(fixedDurationSeconds)
	if normalized == nil {
		return nil, fmt.Errorf("视文模式必须配置 fixedDurationSeconds")
	}
	model, err := app.Store.GetAIModelByName(ctx, strings.TrimSpace(modelName))
	if err != nil {
		return nil, err
	}
	if model == nil || strings.TrimSpace(model.Category) != "video" {
		return nil, fmt.Errorf("当前所选模型不是可用的视频模型")
	}
	segmentSeconds := workflow.ResolveSkillVideoSegmentSeconds(model)
	if segmentSeconds <= 0 {
		return nil, fmt.Errorf("当前模型未配置可用的视频基础时长")
	}
	if *normalized < segmentSeconds || *normalized%segmentSeconds != 0 {
		return nil, fmt.Errorf("当前模型基础时长为 %d 秒，仅支持 %d 的倍数时长", segmentSeconds, segmentSeconds)
	}
	rule, err := app.Store.FindEnabledWorkflowDurationRule(ctx, videoTextWorkflowCode, "视文模式", *normalized)
	if err != nil {
		return nil, err
	}
	return &skillFixedDurationConfig{
		DurationSeconds: *normalized,
		SegmentSeconds:  segmentSeconds,
		Rule:            rule,
	}, nil
}

// 处理校验技能Fixed时长相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func validateSkillFixedDuration(ctx context.Context, app *appstate.App, outputType string, modelName string, fixedDurationSeconds *int) (*skillFixedDurationConfig, error) {
	normalized := normalizeFixedDurationSeconds(fixedDurationSeconds)
	if !isVideoTextOutputType(outputType) {
		if normalized != nil {
			return nil, fmt.Errorf("fixedDurationSeconds only supports 视文模式")
		}
		return nil, nil
	}
	return resolveSkillFixedDurationConfig(ctx, app, modelName, normalized)
}

func validateMixVideoSkillConfig(ctx context.Context, app *appstate.App, outputType string, modelName string, promptTemplate *string, referencePayload []byte) error {
	if !workflow.IsMixVideoSkillOutput(outputType) {
		return nil
	}
	if strings.TrimSpace(normalizePatchedString(promptTemplate)) == "" {
		return fmt.Errorf("promptTemplate is required for 混剪")
	}
	model, err := app.Store.GetAIModelByName(ctx, strings.TrimSpace(modelName))
	if err != nil {
		return err
	}
	if model == nil || !model.IsEnabled || strings.TrimSpace(model.Category) != "chat" {
		return fmt.Errorf("mix video modelName must reference an enabled chat model")
	}
	config := workflow.ResolveMixVideoSkillConfig(domain.ProductSkill{ReferencePayload: referencePayload})
	if strings.TrimSpace(config.PublishTemplate) == "" {
		return fmt.Errorf("referencePayload.mixVideo.publishTemplate is required for 混剪")
	}
	return nil
}

// 解析技能Execution时间，为技能提供结构化输入。
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

// 规范化技能主题，统一技能链路的输入格式和后续处理行为。
func normalizeSkillTopics(topics []string) []string {
	if len(topics) == 0 {
		return []string{}
	}
	normalized := make([]string, 0, len(topics))
	seen := make(map[string]struct{}, len(topics))
	for _, item := range topics {
		topic := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(item), "#"))
		if topic == "" {
			continue
		}
		key := strings.ToLower(topic)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, topic)
	}
	return normalized
}

func firstNonNilString(primary *string, fallback *string) *string {
	if primary != nil {
		return primary
	}
	return fallback
}

func resolveUpdatedSkillReferencePayload(existing []byte, next []byte, touched bool) []byte {
	if touched {
		return next
	}
	return existing
}

// 处理技能列表接口，解析请求参数并调用应用状态或存储层完成业务动作。
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
				item.ExecutionTime = nil
				item.RepeatDaily = false
				item.NextRunAt = nil
				filtered = append(filtered, item)
			}
		}
		items = filtered
	} else {
		for i := range items {
			items[i].ExecutionTime = nil
			items[i].RepeatDaily = false
			items[i].NextRunAt = nil
		}
	}
	render.JSON(w, http.StatusOK, items)
}

// 处理技能EditorDefaults接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *SkillHandler) EditorDefaults(w http.ResponseWriter, r *http.Request) {
	settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill defaults")
		return
	}
	durationRules, err := h.app.Store.ListEnabledWorkflowDurationRules(r.Context(), videoTextWorkflowCode, "视文模式")
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load workflow duration rules")
		return
	}
	durationOptions := make([]skillEditorDurationOption, 0, len(durationRules))
	for _, item := range durationRules {
		durationOptions = append(durationOptions, skillEditorDurationOption{
			RuleID:              item.ID,
			DurationSeconds:     item.DurationSeconds,
			SegmentSeconds:      item.SegmentSeconds,
			SpecialPriceCredits: item.SpecialPriceCredits,
			Label:               fmt.Sprintf("%ds", item.DurationSeconds),
		})
	}
	render.JSON(w, http.StatusOK, skillEditorDefaultsResponse{
		CoverPromptTemplateDefault:  strings.TrimSpace(settings.VideoCoverPrompt),
		MixVideoScriptRewritePrompt: strings.TrimSpace(settings.MixVideoScriptRewritePrompt),
		MixVideoPublishIntroPrompt:  strings.TrimSpace(settings.MixVideoPublishIntroPrompt),
		VideoTextDurationOptions:    durationOptions,
	})
}

// 处理技能详情接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

	sanitizeSkillSchedule(skill)
	render.JSON(w, http.StatusOK, skill)
}

// 处理技能工作区接口，解析请求参数并调用应用状态或存储层完成业务动作。
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
	sanitizeSkillSchedule(skill)

	render.JSON(w, http.StatusOK, domain.ProductSkillWorkspace{
		Skill:        *skill,
		Assets:       assets,
		RecentTasks:  recentTasks,
		RecentAIJobs: recentAIJobs,
		DeviceSyncs:  deviceSyncs,
	})
}

// 处理技能Impact接口，解析请求参数并调用应用状态或存储层完成业务动作。
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
	sanitizeSkillSchedule(skill)

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

// 处理技能创建接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *SkillHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	var payload createSkillRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Description = strings.TrimSpace(payload.Description)
	payload.OutputType = workflow.NormalizeSkillOutputType(payload.OutputType)
	payload.ModelName = strings.TrimSpace(payload.ModelName)
	payload.Topics = normalizeSkillTopics(payload.Topics)
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
		if !device.IsEnabled {
			render.Error(w, http.StatusConflict, "Device is disabled")
			return
		}
		deviceID = &trimmed
	}
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
	if _, err := validateSkillFixedDuration(r.Context(), h.app, payload.OutputType, payload.ModelName, payload.FixedDurationSeconds); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateMixVideoSkillConfig(r.Context(), h.app, payload.OutputType, payload.ModelName, payload.PromptTemplate, referenceBytes); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	isEnabled := true
	if payload.IsEnabled != nil {
		isEnabled = *payload.IsEnabled
	}
	publishIntroEnabled := true
	if payload.PublishIntroEnabled != nil {
		publishIntroEnabled = *payload.PublishIntroEnabled
	}

	storyboardPromptTemplate := payload.StoryboardPromptTemplate
	if storyboardPromptTemplate == nil && !workflow.IsMixVideoSkillOutput(payload.OutputType) {
		storyboardPromptTemplate = stringPtr(workflow.DefaultSkillStoryboardPromptTemplate(payload.OutputType))
	}
	skill, err := h.app.Store.CreateSkill(r.Context(), store.CreateSkillInput{
		ID:                       uuid.NewString(),
		OwnerUserID:              user.ID,
		DeviceID:                 deviceID,
		Name:                     payload.Name,
		Description:              payload.Description,
		OutputType:               payload.OutputType,
		ModelName:                payload.ModelName,
		FixedDurationSeconds:     normalizeFixedDurationSeconds(payload.FixedDurationSeconds),
		PromptTemplate:           payload.PromptTemplate,
		StoryboardPromptTemplate: storyboardPromptTemplate,
		PublishPromptTemplate:    payload.PublishPromptTemplate,
		PublishIntroEnabled:      publishIntroEnabled,
		CoverPromptTemplate:      stringPtr(normalizePatchedString(payload.CoverPromptTemplate)),
		Topics:                   payload.Topics,
		ReferencePayload:         referenceBytes,
		ExecutionTime:            nil,
		RepeatDaily:              false,
		StoryboardEnabled:        storyboardEnabled,
		NextRunAt:                nil,
		IsEnabled:                isEnabled,
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
			"name":                          skill.Name,
			"modelName":                     skill.ModelName,
			"fixedDurationSeconds":          skill.FixedDurationSeconds,
			"deviceId":                      skill.DeviceID,
			"topics":                        skill.Topics,
			"publishIntroEnabled":           skill.PublishIntroEnabled,
			"coverPromptTemplateConfigured": skill.CoverPromptTemplate != nil,
		}),
	})

	sanitizeSkillSchedule(skill)
	render.JSON(w, http.StatusCreated, skill)
}

// 处理技能更新接口，解析请求参数并调用应用状态或存储层完成业务动作。
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
	payload.Topics = normalizeSkillTopics(payload.Topics)
	referenceTouched := payload.ReferencePayload != nil
	if referenceTouched {
		referenceBytes, err = json.Marshal(payload.ReferencePayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "referencePayload must be valid json")
			return
		}
	}
	existing, err := h.app.Store.GetOwnedSkillByID(r.Context(), skillID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill")
		return
	}
	if existing == nil {
		render.Error(w, http.StatusNotFound, "Skill not found")
		return
	}
	nextOutputType := existing.OutputType
	if payload.OutputType != nil && strings.TrimSpace(*payload.OutputType) != "" {
		normalizedOutputType := workflow.NormalizeSkillOutputType(*payload.OutputType)
		nextOutputType = normalizedOutputType
		payload.OutputType = &normalizedOutputType
	}
	nextModelName := existing.ModelName
	if payload.ModelName != nil && strings.TrimSpace(*payload.ModelName) != "" {
		nextModelName = strings.TrimSpace(*payload.ModelName)
	}
	nextFixedDurationSeconds := existing.FixedDurationSeconds
	if payload.FixedDurationSeconds != nil {
		nextFixedDurationSeconds = normalizeFixedDurationSeconds(payload.FixedDurationSeconds)
	}
	if !isVideoTextOutputType(nextOutputType) {
		nextFixedDurationSeconds = nil
	}
	if _, err := validateSkillFixedDuration(r.Context(), h.app, nextOutputType, nextModelName, nextFixedDurationSeconds); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateMixVideoSkillConfig(r.Context(), h.app, nextOutputType, nextModelName, firstNonNilString(payload.PromptTemplate, existing.PromptTemplate), resolveUpdatedSkillReferencePayload(existing.ReferencePayload, referenceBytes, referenceTouched)); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
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
			if !device.IsEnabled {
				render.Error(w, http.StatusConflict, "Device is disabled")
				return
			}
			deviceID = &trimmed
		}
	}
	repeatDaily := false

	skill, err := h.app.Store.UpdateSkill(r.Context(), skillID, user.ID, store.UpdateSkillInput{
		Name:                     payload.Name,
		Description:              payload.Description,
		OutputType:               payload.OutputType,
		ModelName:                payload.ModelName,
		FixedDurationSeconds:     nextFixedDurationSeconds,
		FixedDurationTouched:     payload.FixedDurationSeconds != nil || (payload.OutputType != nil && !isVideoTextOutputType(nextOutputType)),
		PromptTemplate:           payload.PromptTemplate,
		StoryboardPromptTemplate: payload.StoryboardPromptTemplate,
		PublishPromptTemplate:    payload.PublishPromptTemplate,
		PublishIntroEnabled:      payload.PublishIntroEnabled,
		CoverPromptTemplate:      stringPtr(normalizePatchedString(payload.CoverPromptTemplate)),
		Topics:                   payload.Topics,
		TopicsTouched:            payload.Topics != nil,
		ReferencePayload:         referenceBytes,
		ReferenceTouched:         referenceTouched,
		DeviceID:                 deviceID,
		DeviceTouched:            deviceTouched,
		ExecutionTime:            nil,
		ExecutionTouched:         true,
		RepeatDaily:              &repeatDaily,
		StoryboardEnabled:        payload.StoryboardEnabled,
		NextRunAt:                nil,
		NextRunTouched:           true,
		IsEnabled:                payload.IsEnabled,
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
			"name":                 payload.Name,
			"modelName":            payload.ModelName,
			"fixedDurationSeconds": nextFixedDurationSeconds,
			"topics":               payload.Topics,
			"publishIntroEnabled":  payload.PublishIntroEnabled,
			"coverPromptTemplate":  stringPtr(normalizePatchedString(payload.CoverPromptTemplate)),
			"storyboardEnabled":    payload.StoryboardEnabled,
			"isEnabled":            payload.IsEnabled,
		}),
	})

	sanitizeSkillSchedule(skill)
	render.JSON(w, http.StatusOK, skill)
}

// 处理技能删除接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理技能列表Assets接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理技能创建资源接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 清理技能资源文件，释放当前链路不再需要的临时资源或旧数据。
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

// 处理技能上传资源接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理技能删除资源接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理清洗上传Filename相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func sanitizeUploadFilename(fileName string) string {
	base := strings.TrimSpace(filepath.Base(fileName))
	if base == "" || base == "." || base == "/" {
		return "file.bin"
	}
	return strings.ReplaceAll(base, " ", "_")
}

// 处理技能资源审计消息相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func skillAssetAuditMessage(transferred bool) string {
	if transferred {
		return "技能资产已从远程地址转存到对象存储"
	}
	return "技能资产元数据已创建"
}
