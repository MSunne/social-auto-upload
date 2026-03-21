package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	aiclient "omnidrive_cloud/internal/ai"
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
	DeviceID     *string     `json:"deviceId"`
	SkillID      *string     `json:"skillId"`
	Source       *string     `json:"source"`
	LocalTaskID  *string     `json:"localTaskId"`
	JobType      string      `json:"jobType"`
	ModelName    string      `json:"modelName"`
	Prompt       *string     `json:"prompt"`
	InputPayload interface{} `json:"inputPayload"`
}

type updateAIJobRequest struct {
	DeviceID      *string     `json:"deviceId"`
	SkillID       *string     `json:"skillId"`
	Prompt        *string     `json:"prompt"`
	Status        *string     `json:"status"`
	InputPayload  interface{} `json:"inputPayload"`
	OutputPayload interface{} `json:"outputPayload"`
	Message       *string     `json:"message"`
	CostCredits   *int64      `json:"costCredits"`
	RunAt         *string     `json:"runAt"`
	FinishedAt    *string     `json:"finishedAt"`
}

type createPublishTaskFromAIJobRequest struct {
	DeviceID     *string  `json:"deviceId"`
	AccountID    *string  `json:"accountId"`
	Platform     string   `json:"platform"`
	AccountName  string   `json:"accountName"`
	Title        *string  `json:"title"`
	ContentText  *string  `json:"contentText"`
	ArtifactKeys []string `json:"artifactKeys"`
	RunAt        *string  `json:"runAt"`
}

type uploadAIArtifactURLRequest struct {
	ArtifactType string  `json:"artifactType"`
	ArtifactKey  string  `json:"artifactKey"`
	Source       string  `json:"source"`
	Title        *string `json:"title"`
	PublicURL    string  `json:"publicUrl"`
	FileName     string  `json:"fileName"`
	MimeType     *string `json:"mimeType"`
	DeviceID     *string `json:"deviceId"`
	RootName     *string `json:"rootName"`
	RelativePath *string `json:"relativePath"`
	AbsolutePath *string `json:"absolutePath"`
}

type chatStreamResponse struct {
	JobID        string         `json:"jobId,omitempty"`
	ModelName    string         `json:"modelName,omitempty"`
	Delta        string         `json:"delta,omitempty"`
	Text         string         `json:"text,omitempty"`
	Role         string         `json:"role,omitempty"`
	Usage        map[string]any `json:"usage,omitempty"`
	FinishReason string         `json:"finishReason,omitempty"`
	Progressed   bool           `json:"progressed,omitempty"`
	Done         bool           `json:"done,omitempty"`
	Error        string         `json:"error,omitempty"`
}

type chatAttachmentDraft struct {
	FileName    string  `json:"fileName"`
	MimeType    string  `json:"mimeType"`
	DataURL     string  `json:"dataUrl"`
	Base64Data  string  `json:"base64Data"`
	TextContent *string `json:"textContent"`
	SizeBytes   *int64  `json:"sizeBytes"`
}

type persistedChatAttachment struct {
	ArtifactKey  string  `json:"artifactKey"`
	FileName     string  `json:"fileName"`
	MimeType     string  `json:"mimeType"`
	PublicURL    string  `json:"publicUrl"`
	StorageKey   string  `json:"storageKey,omitempty"`
	SizeBytes    *int64  `json:"sizeBytes,omitempty"`
	Kind         string  `json:"kind"`
	TextContent  *string `json:"textContent,omitempty"`
	MessageIndex int     `json:"messageIndex"`
}

func NewAIHandler(app *appstate.App) *AIHandler {
	return &AIHandler{app: app}
}

func sanitizeAIModelForPublic(model *domain.AIModel) *domain.AIModel {
	if model == nil {
		return nil
	}
	sanitized := *model
	sanitized.APIKey = nil
	return &sanitized
}

func sanitizeAIModelListForPublic(items []domain.AIModel) []domain.AIModel {
	if len(items) == 0 {
		return items
	}
	sanitized := make([]domain.AIModel, 0, len(items))
	for _, item := range items {
		copyItem := item
		copyItem.APIKey = nil
		sanitized = append(sanitized, copyItem)
	}
	return sanitized
}

func (h *AIHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	category := normalizeAIModelCategory(firstNonEmptyAdminValue(
		r.URL.Query().Get("modelType"),
		r.URL.Query().Get("type"),
		r.URL.Query().Get("category"),
	))
	items, err := h.app.Store.ListAIModels(r.Context(), category)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI models")
		return
	}
	render.JSON(w, http.StatusOK, sanitizeAIModelListForPublic(items))
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
		JobType:       strings.TrimSpace(r.URL.Query().Get("jobType")),
		Status:        strings.TrimSpace(r.URL.Query().Get("status")),
		SkillID:       strings.TrimSpace(r.URL.Query().Get("skillId")),
		DeviceID:      strings.TrimSpace(r.URL.Query().Get("deviceId")),
		AccountID:     strings.TrimSpace(r.URL.Query().Get("accountId")),
		Source:        strings.TrimSpace(r.URL.Query().Get("source")),
		ExcludeSource: strings.TrimSpace(r.URL.Query().Get("excludeSource")),
		Limit:         limit,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI jobs")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AIHandler) StreamChat(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI configuration")
		return
	}
	if !settings.AIWorkerEnabled {
		render.Error(w, http.StatusServiceUnavailable, "AI worker is currently disabled by admin configuration")
		return
	}

	var payload createAIJobRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.ModelName = strings.TrimSpace(payload.ModelName)
	if payload.ModelName == "" {
		render.Error(w, http.StatusBadRequest, "modelName is required")
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
	if strings.TrimSpace(model.Category) != "chat" {
		render.Error(w, http.StatusConflict, "AI model category does not match chat")
		return
	}

	var inputPayload []byte
	if payload.InputPayload != nil {
		inputPayload, err = json.Marshal(payload.InputPayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "inputPayload must be valid json")
			return
		}
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		render.Error(w, http.StatusInternalServerError, "Streaming is not supported by this server")
		return
	}

	jobID := uuid.NewString()
	source := "omnidrive_chat"
	initialMessage := "聊天生成中"

	createdJob, err := h.app.Store.CreateAIJob(r.Context(), store.CreateAIJobInput{
		ID:           jobID,
		OwnerUserID:  user.ID,
		Source:       source,
		JobType:      "chat",
		ModelName:    payload.ModelName,
		Prompt:       payload.Prompt,
		InputPayload: stripChatAttachmentDrafts(inputPayload),
		Status:       "running",
		Message:      &initialMessage,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create chat history")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "ai_job",
		ResourceID:   &createdJob.ID,
		Action:       "create",
		Title:        "创建流式聊天",
		Source:       createdJob.ModelName,
		Status:       createdJob.Status,
		Message:      createdJob.Message,
		Payload: mustJSONBytes(map[string]any{
			"jobType":   createdJob.JobType,
			"modelName": createdJob.ModelName,
			"source":    createdJob.Source,
		}),
	})

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if err := writeSSEEvent(w, flusher, "meta", chatStreamResponse{
		JobID:     jobID,
		ModelName: payload.ModelName,
		Role:      "assistant",
	}); err != nil {
		return
	}

	sanitizedInputPayload, artifactInputs, attachmentRefs, err := h.prepareStreamChatPayload(r.Context(), user.ID, jobID, payload.Prompt, inputPayload)
	if err != nil {
		failedStatus := "failed"
		failedAt := time.Now().UTC()
		_, _ = h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
			Status:          &failedStatus,
			Message:         stringPtr(err.Error()),
			FinishedAt:      &failedAt,
			FinishedTouched: true,
		})
		_ = writeSSEEvent(w, flusher, "error", chatStreamResponse{
			JobID:     jobID,
			ModelName: payload.ModelName,
			Error:     err.Error(),
		})
		return
	}

	if len(artifactInputs) > 0 {
		if _, err := h.app.Store.UpsertAIJobArtifacts(r.Context(), artifactInputs); err != nil {
			failedStatus := "failed"
			failedAt := time.Now().UTC()
			_, _ = h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
				Status:          &failedStatus,
				Message:         stringPtr("Failed to persist chat attachments"),
				FinishedAt:      &failedAt,
				FinishedTouched: true,
			})
			_ = writeSSEEvent(w, flusher, "error", chatStreamResponse{
				JobID:     jobID,
				ModelName: payload.ModelName,
				Error:     "Failed to persist chat attachments",
			})
			return
		}
	}

	updatedJob, err := h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
		InputPayload: sanitizedInputPayload,
		InputTouched: true,
	})
	if err != nil || updatedJob == nil {
		failedStatus := "failed"
		failedAt := time.Now().UTC()
		_, _ = h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
			Status:          &failedStatus,
			Message:         stringPtr("Failed to save chat payload"),
			FinishedAt:      &failedAt,
			FinishedTouched: true,
		})
		_ = writeSSEEvent(w, flusher, "error", chatStreamResponse{
			JobID:     jobID,
			ModelName: payload.ModelName,
			Error:     "Failed to save chat payload",
		})
		return
	}

	req, err := aiclient.BuildChatRequest(updatedJob)
	if err != nil {
		failedStatus := "failed"
		failedAt := time.Now().UTC()
		_, _ = h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
			Status:          &failedStatus,
			Message:         stringPtr(err.Error()),
			FinishedAt:      &failedAt,
			FinishedTouched: true,
		})
		_ = writeSSEEvent(w, flusher, "error", chatStreamResponse{
			JobID:     jobID,
			ModelName: payload.ModelName,
			Error:     err.Error(),
		})
		return
	}

	if model.BaseURL != nil {
		req.BaseURL = strings.TrimSpace(*model.BaseURL)
	}
	if model.APIKey != nil {
		req.APIKey = strings.TrimSpace(*model.APIKey)
	}

	provider, err := aiclient.NewAPIYIProvider(h.app.Config)
	if err != nil {
		failedStatus := "failed"
		failedAt := time.Now().UTC()
		_, _ = h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
			Status:          &failedStatus,
			Message:         stringPtr("Failed to initialize AI provider"),
			FinishedAt:      &failedAt,
			FinishedTouched: true,
		})
		_ = writeSSEEvent(w, flusher, "error", chatStreamResponse{
			JobID:     jobID,
			ModelName: payload.ModelName,
			Error:     "Failed to initialize AI provider",
		})
		return
	}

	result, err := provider.GenerateChatStream(r.Context(), req, func(chunk aiclient.ChatStreamChunk) error {
		if chunk.Done {
			return writeSSEEvent(w, flusher, "done", chatStreamResponse{
				JobID:        jobID,
				ModelName:    payload.ModelName,
				Text:         chunk.Text,
				Role:         chunk.Role,
				Usage:        chunk.Usage,
				FinishReason: chunk.FinishReason,
				Done:         true,
			})
		}
		if chunk.Progressed {
			return writeSSEEvent(w, flusher, "progress", chatStreamResponse{
				JobID:      jobID,
				ModelName:  payload.ModelName,
				Role:       chunk.Role,
				Progressed: true,
			})
		}
		if chunk.Delta == "" {
			return nil
		}
		return writeSSEEvent(w, flusher, "delta", chatStreamResponse{
			JobID:        jobID,
			ModelName:    payload.ModelName,
			Delta:        chunk.Delta,
			Text:         chunk.Text,
			Role:         chunk.Role,
			Usage:        chunk.Usage,
			FinishReason: chunk.FinishReason,
		})
	})
	if err != nil {
		failedStatus := "failed"
		failedAt := time.Now().UTC()
		outputPayload := mustJSONBytes(map[string]any{
			"error": err.Error(),
		})
		_, _ = h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
			Status:          &failedStatus,
			OutputPayload:   outputPayload,
			OutputTouched:   true,
			Message:         stringPtr(err.Error()),
			FinishedAt:      &failedAt,
			FinishedTouched: true,
		})
		_ = writeSSEEvent(w, flusher, "error", chatStreamResponse{
			JobID:     jobID,
			ModelName: payload.ModelName,
			Error:     err.Error(),
		})
		return
	}

	if result == nil {
		result = &aiclient.ChatResult{}
	}

	responsePayload := mustJSONBytes(map[string]any{
		"text":         result.Text,
		"role":         result.Role,
		"usage":        result.Usage,
		"finishReason": result.FinishReason,
		"attachments":  attachmentRefs,
	})
	successStatus := "success"
	successMessage := "聊天已完成"
	finishedAt := time.Now().UTC()
	_, _ = h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
		Status:          &successStatus,
		OutputPayload:   responsePayload,
		OutputTouched:   true,
		Message:         &successMessage,
		FinishedAt:      &finishedAt,
		FinishedTouched: true,
	})
	if strings.TrimSpace(result.Text) != "" {
		_, _ = h.app.Store.UpsertAIJobArtifacts(r.Context(), []store.UpsertAIJobArtifactInput{{
			JobID:        jobID,
			ArtifactKey:  "assistant-response",
			ArtifactType: "chat_response",
			Source:       payload.ModelName,
			Title:        stringPtr("助手回复"),
			TextContent:  stringPtr(result.Text),
			Payload: mustJSONBytes(map[string]any{
				"usage":        result.Usage,
				"finishReason": result.FinishReason,
			}),
		}})
	}
}

func (h *AIHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI configuration")
		return
	}
	if !settings.AIWorkerEnabled {
		render.Error(w, http.StatusServiceUnavailable, "AI worker is currently disabled by admin configuration")
		return
	}

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
	if model.Category != payload.JobType {
		render.Error(w, http.StatusConflict, "AI model category does not match job type")
		return
	}

	deviceID, ok := h.resolveOwnedDeviceID(w, r, payload.DeviceID, user.ID)
	if !ok {
		return
	}
	source := normalizeTrimmedString(payload.Source)
	if source == nil {
		defaultSource := "omnidrive_cloud"
		source = &defaultSource
	}
	if *source == "openclaw_skill" {
		if deviceID == nil {
			render.Error(w, http.StatusConflict, "OpenClaw OmniSkill 必须绑定并启用当前 OmniBull 设备后才能使用云端 AI")
			return
		}
		device, err := h.app.Store.GetOwnedDevice(r.Context(), *deviceID, user.ID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to validate bound device")
			return
		}
		if device == nil || !device.IsEnabled {
			render.Error(w, http.StatusConflict, "当前 OmniBull 设备未启用或已解绑，无法使用云端 AI")
			return
		}
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

	message := "AI 任务已创建，等待 OmniDrive 云端生成"
	job, err := h.app.Store.CreateAIJob(r.Context(), store.CreateAIJobInput{
		ID:           uuid.NewString(),
		OwnerUserID:  user.ID,
		DeviceID:     deviceID,
		SkillID:      skillID,
		Source:       *source,
		LocalTaskID:  normalizeTrimmedString(payload.LocalTaskID),
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
			"jobType":     job.JobType,
			"modelName":   job.ModelName,
			"deviceId":    job.DeviceID,
			"skillId":     job.SkillID,
			"source":      job.Source,
			"localTaskId": job.LocalTaskID,
		}),
	})
	render.JSON(w, http.StatusCreated, job)
}

func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event string, payload any) error {
	if w == nil || flusher == nil {
		return fmt.Errorf("streaming writer is unavailable")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", strings.TrimSpace(event)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func stripChatAttachmentDrafts(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw
	}
	delete(payload, "attachments")
	sanitized, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return sanitized
}

func (h *AIHandler) prepareStreamChatPayload(ctx context.Context, ownerUserID string, jobID string, prompt *string, rawPayload []byte) ([]byte, []store.UpsertAIJobArtifactInput, []persistedChatAttachment, error) {
	payload := map[string]any{}
	if len(rawPayload) > 0 {
		if err := json.Unmarshal(rawPayload, &payload); err != nil {
			return nil, nil, nil, fmt.Errorf("inputPayload must be valid json")
		}
	}

	messages, err := parseChatMessagesForPersistence(payload, prompt)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(messages) == 0 {
		return nil, nil, nil, fmt.Errorf("chat job requires prompt or messages")
	}

	artifactInputs := make([]store.UpsertAIJobArtifactInput, 0)
	attachmentRefs := make([]persistedChatAttachment, 0)

	messageIndex := len(messages) - 1
	currentUserParts, hasStructuredContent := normalizeChatMessageContent(messages[messageIndex].Content)
	drafts, err := decodeChatAttachmentDrafts(payload["attachments"])
	if err != nil {
		return nil, nil, nil, err
	}
	for index, draft := range drafts {
		prepared, err := h.persistChatAttachment(ctx, ownerUserID, jobID, messageIndex, index, draft)
		if err != nil {
			return nil, nil, nil, err
		}
		if prepared == nil {
			continue
		}
		artifactInputs = append(artifactInputs, prepared.ArtifactInput)
		attachmentRefs = append(attachmentRefs, prepared.Reference)
		currentUserParts = append(currentUserParts, prepared.PromptParts...)
		hasStructuredContent = true
	}
	if hasStructuredContent {
		messages[messageIndex].Content = currentUserParts
	}

	payload["messages"] = messages
	if len(attachmentRefs) > 0 {
		payload["attachments"] = attachmentRefs
	} else {
		delete(payload, "attachments")
	}
	sanitized, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to encode chat payload")
	}
	return sanitized, artifactInputs, attachmentRefs, nil
}

type preparedChatAttachment struct {
	ArtifactInput store.UpsertAIJobArtifactInput
	Reference     persistedChatAttachment
	PromptParts   []map[string]any
}

func (h *AIHandler) persistChatAttachment(ctx context.Context, ownerUserID string, jobID string, messageIndex int, order int, draft chatAttachmentDraft) (*preparedChatAttachment, error) {
	fileName := sanitizeUploadFilename(draft.FileName)
	if fileName == "" {
		fileName = fmt.Sprintf("attachment-%d.bin", order+1)
	}

	data, mimeType, err := decodeChatAttachmentBytes(draft)
	if err != nil {
		return nil, err
	}
	if mimeType == "" {
		mimeType = strings.TrimSpace(draft.MimeType)
	}
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(fileName))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	object, err := h.app.Storage.SaveBytes(
		ctx,
		fmt.Sprintf("ai-jobs/%s/%s/chat-attachments/%s-%s", ownerUserID, jobID, uuid.NewString(), fileName),
		mimeType,
		data,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store chat attachment")
	}

	kind := detectChatAttachmentKind(mimeType, fileName)
	textContent := normalizeTrimmedString(draft.TextContent)
	if kind == "text" && textContent == nil && len(data) > 0 {
		derived := normalizeTrimmedStringPtr(string(data))
		textContent = derived
	}

	artifactKey := fmt.Sprintf("chat-attachment-%d-%s", order+1, buildAIArtifactKey(fileName, "chat_attachment"))
	artifactPayload := mustJSONBytes(map[string]any{
		"kind":         kind,
		"messageIndex": messageIndex,
		"order":        order,
	})

	reference := persistedChatAttachment{
		ArtifactKey:  artifactKey,
		FileName:     fileName,
		MimeType:     object.ContentType,
		PublicURL:    object.PublicURL,
		StorageKey:   object.StorageKey,
		SizeBytes:    &object.SizeBytes,
		Kind:         kind,
		TextContent:  textContent,
		MessageIndex: messageIndex,
	}

	return &preparedChatAttachment{
		ArtifactInput: store.UpsertAIJobArtifactInput{
			JobID:        jobID,
			ArtifactKey:  artifactKey,
			ArtifactType: "chat_attachment",
			Source:       "chat_upload",
			Title:        stringPtr(fileName),
			FileName:     stringPtr(fileName),
			MimeType:     stringPtr(object.ContentType),
			StorageKey:   stringPtr(object.StorageKey),
			PublicURL:    stringPtr(object.PublicURL),
			SizeBytes:    &object.SizeBytes,
			TextContent:  textContent,
			Payload:      artifactPayload,
		},
		Reference:   reference,
		PromptParts: buildChatAttachmentPromptParts(reference),
	}, nil
}

func parseChatMessagesForPersistence(payload map[string]any, prompt *string) ([]aiclient.ChatMessage, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	rawMessages, err := aiclient.BuildChatRequest(&domain.AIJob{
		JobType:      "chat",
		ModelName:    "chat",
		Prompt:       prompt,
		InputPayload: mustJSONBytes(payload),
	})
	if err != nil {
		return nil, err
	}
	return rawMessages.Messages, nil
}

func normalizeChatMessageContent(content any) ([]map[string]any, bool) {
	switch typed := content.(type) {
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			part, ok := item.(map[string]any)
			if ok {
				result = append(result, part)
			}
		}
		return result, len(result) > 0
	case []map[string]any:
		return append([]map[string]any{}, typed...), len(typed) > 0
	default:
		text := strings.TrimSpace(fmt.Sprint(content))
		if text == "" {
			return []map[string]any{}, false
		}
		return []map[string]any{{"type": "text", "text": text}}, true
	}
}

func decodeChatAttachmentDrafts(raw any) ([]chatAttachmentDraft, error) {
	if raw == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("attachments must be valid json")
	}
	var drafts []chatAttachmentDraft
	if err := json.Unmarshal(encoded, &drafts); err != nil {
		return nil, fmt.Errorf("attachments must be an array")
	}
	return drafts, nil
}

func decodeChatAttachmentBytes(draft chatAttachmentDraft) ([]byte, string, error) {
	if strings.TrimSpace(draft.DataURL) != "" {
		return decodeBase64Payload(strings.TrimSpace(draft.DataURL))
	}
	if strings.TrimSpace(draft.Base64Data) != "" {
		return decodeBase64Payload(strings.TrimSpace(draft.Base64Data))
	}
	return nil, "", fmt.Errorf("attachment %q is missing file data", strings.TrimSpace(draft.FileName))
}

func detectChatAttachmentKind(mimeType string, fileName string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(mimeType, "image/") {
		return "image"
	}
	if strings.HasPrefix(mimeType, "text/") {
		return "text"
	}
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(fileName))) {
	case ".txt", ".md", ".markdown", ".json", ".csv", ".tsv", ".yaml", ".yml", ".xml", ".html", ".htm":
		return "text"
	default:
		return "file"
	}
}

func buildChatAttachmentPromptParts(ref persistedChatAttachment) []map[string]any {
	parts := make([]map[string]any, 0, 2)
	switch ref.Kind {
	case "image":
		parts = append(parts,
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("附件图片：%s", ref.FileName),
			},
			map[string]any{
				"type": "image_url",
				"image_url": map[string]any{
					"url": ref.PublicURL,
				},
			},
		)
	case "text":
		textBody := ""
		if ref.TextContent != nil {
			textBody = strings.TrimSpace(*ref.TextContent)
		}
		if textBody == "" {
			textBody = "该文本附件未能提取到有效内容。"
		}
		parts = append(parts, map[string]any{
			"type": "text",
			"text": fmt.Sprintf("附件文件 %s 内容如下：\n%s", ref.FileName, textBody),
		})
	default:
		parts = append(parts, map[string]any{
			"type": "text",
			"text": fmt.Sprintf("用户上传了文件 %s（%s），文件已存档。当前聊天自动注入仅覆盖图片和文本附件。", ref.FileName, ref.MimeType),
		})
	}
	return parts
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
	artifacts, err := h.app.Store.ListAIJobArtifactsByOwner(r.Context(), job.ID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI artifacts")
		return
	}
	publishTasks, err := h.app.Store.ListPublishTasksByAIJobOwner(r.Context(), job.ID, user.ID, 20)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load linked publish tasks")
		return
	}
	billingUsageEvents, err := h.app.Store.ListBillingUsageEventsByUser(r.Context(), user.ID, store.BillingUsageEventListFilter{
		SourceType: "ai_job",
		SourceID:   job.ID,
		Limit:      50,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load billing usage events")
		return
	}

	render.JSON(w, http.StatusOK, domain.AIJobWorkspace{
		Job:                *job,
		Model:              sanitizeAIModelForPublic(model),
		Skill:              skill,
		Artifacts:          artifacts,
		PublishTasks:       publishTasks,
		BillingUsageEvents: billingUsageEvents,
		Bridge:             buildAIJobBridgeState(job, artifacts, publishTasks),
		Actions:            computeAIJobActions(job, len(artifacts)),
	})
}

func (h *AIHandler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
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
	items, err := h.app.Store.ListAIJobArtifactsByOwner(r.Context(), jobID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI artifacts")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AIHandler) UploadArtifact(w http.ResponseWriter, r *http.Request) {
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

	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type"))), "application/json") {
		h.uploadArtifactFromURL(w, r, user.ID, jobID)
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		render.Error(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	artifactType := strings.TrimSpace(r.FormValue("artifactType"))
	artifactKey := strings.TrimSpace(r.FormValue("artifactKey"))
	if artifactType == "" {
		render.Error(w, http.StatusBadRequest, "artifactType is required")
		return
	}
	deviceID := normalizeTrimmedStringPtr(r.FormValue("deviceId"))
	rootName := normalizeTrimmedStringPtr(r.FormValue("rootName"))
	relativePath := normalizeTrimmedStringPtr(r.FormValue("relativePath"))
	absolutePath := normalizeTrimmedStringPtr(r.FormValue("absolutePath"))
	if err := h.validateArtifactDeviceBinding(r.Context(), user.ID, deviceID, rootName, relativePath); err != nil {
		h.renderArtifactBindingError(w, err)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		render.Error(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 64<<20))
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to read file")
		return
	}

	fileName := sanitizeUploadFilename(header.Filename)
	contentType := header.Header.Get("Content-Type")
	object, err := h.app.Storage.SaveBytes(
		r.Context(),
		fmt.Sprintf("ai-jobs/%s/%s/%s-%s", user.ID, jobID, uuid.NewString(), fileName),
		contentType,
		data,
	)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to store file")
		return
	}

	if artifactKey == "" {
		artifactKey = buildAIArtifactKey(fileName, artifactType)
	}
	title := strings.TrimSpace(r.FormValue("title"))
	source := strings.TrimSpace(r.FormValue("source"))
	if source == "" {
		source = "manual_upload"
	}

	artifacts, err := h.app.Store.UpsertAIJobArtifacts(r.Context(), []store.UpsertAIJobArtifactInput{{
		JobID:        jobID,
		ArtifactKey:  artifactKey,
		ArtifactType: artifactType,
		Source:       source,
		Title:        normalizeTrimmedStringPtr(title),
		FileName:     &fileName,
		MimeType:     &object.ContentType,
		StorageKey:   &object.StorageKey,
		PublicURL:    &object.PublicURL,
		SizeBytes:    &object.SizeBytes,
		DeviceID:     deviceID,
		RootName:     rootName,
		RelativePath: relativePath,
		AbsolutePath: absolutePath,
	}})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create AI artifact")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "ai_job_artifact",
		ResourceID:   &artifacts[0].ID,
		Action:       "upload",
		Title:        "上传 AI 产物",
		Source:       artifactType,
		Status:       "success",
		Message:      auditStringPtr("AI 任务产物文件已上传"),
		Payload: mustJSONBytes(map[string]any{
			"jobId":       jobID,
			"artifactKey": artifactKey,
			"publicUrl":   artifacts[0].PublicURL,
		}),
	})

	render.JSON(w, http.StatusCreated, artifacts[0])
}

func (h *AIHandler) uploadArtifactFromURL(w http.ResponseWriter, r *http.Request, ownerUserID string, jobID string) {
	var payload uploadAIArtifactURLRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.ArtifactType = strings.TrimSpace(payload.ArtifactType)
	payload.ArtifactKey = strings.TrimSpace(payload.ArtifactKey)
	payload.Source = strings.TrimSpace(payload.Source)
	payload.PublicURL = strings.TrimSpace(payload.PublicURL)
	if payload.ArtifactType == "" || payload.PublicURL == "" {
		render.Error(w, http.StatusBadRequest, "artifactType and publicUrl are required")
		return
	}

	deviceID := normalizeTrimmedString(payload.DeviceID)
	rootName := normalizeTrimmedString(payload.RootName)
	relativePath := normalizeTrimmedString(payload.RelativePath)
	absolutePath := normalizeTrimmedString(payload.AbsolutePath)
	if err := h.validateArtifactDeviceBinding(r.Context(), ownerUserID, deviceID, rootName, relativePath); err != nil {
		h.renderArtifactBindingError(w, err)
		return
	}

	managedRef, _, err := normalizeManagedObjectRef(r.Context(), h.app, fmt.Sprintf("ai-jobs/%s/%s", ownerUserID, jobID), managedObjectRef{
		FileName:  payload.FileName,
		MimeType:  payload.MimeType,
		PublicURL: normalizeTrimmedStringPtr(payload.PublicURL),
	})
	if err != nil {
		render.Error(w, http.StatusBadGateway, "Failed to mirror remote artifact into storage")
		return
	}

	artifactKey := payload.ArtifactKey
	if artifactKey == "" {
		artifactKey = buildAIArtifactKey(managedRef.FileName, payload.ArtifactType)
	}
	title := normalizeTrimmedString(payload.Title)
	source := payload.Source
	if source == "" {
		source = "remote_url"
	}

	artifacts, err := h.app.Store.UpsertAIJobArtifacts(r.Context(), []store.UpsertAIJobArtifactInput{{
		JobID:        jobID,
		ArtifactKey:  artifactKey,
		ArtifactType: payload.ArtifactType,
		Source:       source,
		Title:        title,
		FileName:     normalizeTrimmedStringPtr(managedRef.FileName),
		MimeType:     managedRef.MimeType,
		StorageKey:   managedRef.StorageKey,
		PublicURL:    managedRef.PublicURL,
		SizeBytes:    managedRef.SizeBytes,
		DeviceID:     deviceID,
		RootName:     rootName,
		RelativePath: relativePath,
		AbsolutePath: absolutePath,
	}})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create AI artifact")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  ownerUserID,
		ResourceType: "ai_job_artifact",
		ResourceID:   &artifacts[0].ID,
		Action:       "mirror_remote_url",
		Title:        "转存 AI 远程产物",
		Source:       payload.ArtifactType,
		Status:       "success",
		Message:      auditStringPtr("AI 任务产物已从远程地址转存到对象存储"),
		Payload: mustJSONBytes(map[string]any{
			"jobId":       jobID,
			"artifactKey": artifactKey,
			"publicUrl":   artifacts[0].PublicURL,
			"sourceUrl":   payload.PublicURL,
		}),
	})

	render.JSON(w, http.StatusCreated, artifacts[0])
}

func (h *AIHandler) validateArtifactDeviceBinding(ctx context.Context, ownerUserID string, deviceID, rootName, relativePath *string) error {
	if deviceID == nil {
		return nil
	}

	device, err := h.app.Store.GetOwnedDevice(ctx, *deviceID, ownerUserID)
	if err != nil {
		return fmt.Errorf("validate_device: %w", err)
	}
	if device == nil {
		return fmt.Errorf("device_not_found")
	}
	if rootName == nil || relativePath == nil {
		return fmt.Errorf("missing_material_location")
	}
	return nil
}

func (h *AIHandler) renderArtifactBindingError(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case strings.Contains(err.Error(), "validate_device:"):
		render.Error(w, http.StatusInternalServerError, "Failed to validate artifact device")
	case err.Error() == "device_not_found":
		render.Error(w, http.StatusNotFound, "Artifact device not found")
	case err.Error() == "missing_material_location":
		render.Error(w, http.StatusBadRequest, "rootName and relativePath are required when deviceId is provided")
	default:
		render.Error(w, http.StatusBadRequest, err.Error())
	}
}

func (h *AIHandler) CreatePublishTask(w http.ResponseWriter, r *http.Request) {
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
	if job.Status != "success" && job.Status != "completed" {
		render.Error(w, http.StatusConflict, "AI job must be completed before creating publish task")
		return
	}

	var payload createPublishTaskFromAIJobRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	deviceID := firstNonEmptyString(payload.DeviceID, job.DeviceID)
	if deviceID == nil {
		render.Error(w, http.StatusBadRequest, "deviceId is required")
		return
	}
	device, err := h.app.Store.GetOwnedDevice(r.Context(), *deviceID, user.ID)
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

	payload.Platform = strings.TrimSpace(payload.Platform)
	payload.AccountName = strings.TrimSpace(payload.AccountName)
	if payload.Platform == "" || payload.AccountName == "" {
		render.Error(w, http.StatusBadRequest, "platform and accountName are required")
		return
	}

	accountID := normalizeTrimmedString(payload.AccountID)
	var account *domain.PlatformAccount
	if accountID != nil {
		account, err = h.app.Store.GetOwnedAccountByID(r.Context(), *accountID, user.ID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to validate account")
			return
		}
		if account == nil {
			render.Error(w, http.StatusNotFound, "Account not found")
			return
		}
		if account.DeviceID != device.ID || account.Platform != payload.Platform || account.AccountName != payload.AccountName {
			render.Error(w, http.StatusConflict, "Account does not match device/platform/accountName")
			return
		}
	}

	artifacts, err := h.app.Store.ListAIJobArtifactsByOwner(r.Context(), jobID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI artifacts")
		return
	}
	selectedArtifacts := filterAIArtifactsByKeys(artifacts, payload.ArtifactKeys)
	if len(selectedArtifacts) == 0 {
		render.Error(w, http.StatusConflict, "No AI artifacts are available for publish task creation")
		return
	}

	materialRefs := make([]store.ReplacePublishTaskMaterialRefInput, 0, len(selectedArtifacts))
	mediaItems := make([]map[string]any, 0, len(selectedArtifacts))
	for _, artifact := range selectedArtifacts {
		mediaItems = append(mediaItems, map[string]any{
			"artifactKey":  artifact.ArtifactKey,
			"artifactType": artifact.ArtifactType,
			"publicUrl":    artifact.PublicURL,
			"fileName":     artifact.FileName,
			"mimeType":     artifact.MimeType,
			"source":       artifact.Source,
		})
		if artifact.DeviceID == nil || strings.TrimSpace(*artifact.DeviceID) != device.ID || artifact.RootName == nil || artifact.RelativePath == nil {
			render.Error(w, http.StatusConflict, "AI artifacts are not mirrored to the selected OmniBull device")
			return
		}
		entry, entryErr := h.app.Store.GetMaterialEntryByOwner(r.Context(), user.ID, device.ID, strings.TrimSpace(*artifact.RootName), strings.TrimSpace(*artifact.RelativePath))
		if entryErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to validate mirrored AI artifact")
			return
		}
		if entry == nil || !entry.IsAvailable {
			render.Error(w, http.StatusConflict, "Mirrored AI artifact is not available on the selected OmniBull device")
			return
		}

		role := "media"
		materialRefs = append(materialRefs, store.ReplacePublishTaskMaterialRefInput{
			TaskID:       "",
			DeviceID:     device.ID,
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
		})
	}

	taskID := uuid.NewString()
	rawMediaPayload := map[string]any{
		"source":      "ai_job",
		"aiJobId":     job.ID,
		"jobType":     job.JobType,
		"modelName":   job.ModelName,
		"artifacts":   mediaItems,
		"generatedAt": job.FinishedAt,
	}
	normalizedMediaPayload, _, normalizeErr := normalizePublishTaskMediaPayload(r.Context(), h.app, user.ID, taskID, rawMediaPayload)
	if normalizeErr != nil {
		render.Error(w, http.StatusBadGateway, "Failed to mirror generated media into storage")
		return
	}
	mediaPayload, err := json.Marshal(normalizedMediaPayload)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to prepare media payload")
		return
	}

	title := strings.TrimSpace(stringValue(payload.Title))
	if title == "" {
		title = buildPublishTaskTitleFromAIJob(job, selectedArtifacts)
	}

	runAt, ok := parseOptionalRFC3339(w, payload.RunAt, "runAt")
	if !ok {
		return
	}

	taskMessage := "来自 AI 任务的发布任务，等待执行"
	task, err := h.app.Store.CreatePublishTask(r.Context(), store.CreatePublishTaskInput{
		ID:           taskID,
		DeviceID:     device.ID,
		AccountID:    accountID,
		Platform:     payload.Platform,
		AccountName:  payload.AccountName,
		Title:        title,
		ContentText:  payload.ContentText,
		MediaPayload: mediaPayload,
		Status:       "pending",
		Message:      &taskMessage,
		RunAt:        runAt,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create publish task from AI job")
		return
	}

	for i := range materialRefs {
		materialRefs[i].TaskID = task.ID
	}
	if _, err := h.app.Store.ReplacePublishTaskMaterialRefs(r.Context(), task.ID, user.ID, materialRefs); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to attach AI materials to publish task")
		return
	}
	if err := h.app.Store.LinkAIJobToPublishTask(r.Context(), store.LinkAIJobPublishTaskInput{
		JobID:       job.ID,
		TaskID:      task.ID,
		OwnerUserID: user.ID,
	}); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to link AI job to publish task")
		return
	}
	_, _ = h.app.Store.CreatePublishTaskEvent(r.Context(), store.CreatePublishTaskEventInput{
		ID:        uuid.NewString(),
		TaskID:    task.ID,
		EventType: "created_from_ai_job",
		Source:    "omnidrive",
		Status:    task.Status,
		Message:   auditStringPtr("发布任务由 AI 任务生成"),
		Payload: mustJSONBytes(map[string]any{
			"aiJobId":      job.ID,
			"artifactKeys": collectAIArtifactKeys(selectedArtifacts),
		}),
	})

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "publish_task",
		ResourceID:   &task.ID,
		Action:       "create_from_ai_job",
		Title:        "由 AI 任务创建发布任务",
		Source:       task.Platform,
		Status:       task.Status,
		Message:      task.Message,
		Payload: mustJSONBytes(map[string]any{
			"aiJobId":     job.ID,
			"deviceId":    device.ID,
			"accountId":   accountID,
			"accountName": task.AccountName,
		}),
	})

	render.JSON(w, http.StatusCreated, task)
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

	deviceID, ok := h.resolveOwnedDeviceID(w, r, payload.DeviceID, user.ID)
	if !ok {
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

	finishedAt, finishedTouched, ok := parseOptionalRFC3339Touched(w, payload.FinishedAt, "finishedAt")
	if !ok {
		return
	}
	runAt, runAtTouched, ok := parseOptionalRFC3339Touched(w, payload.RunAt, "runAt")
	if !ok {
		return
	}

	job, err := h.app.Store.UpdateAIJob(r.Context(), jobID, user.ID, store.UpdateAIJobInput{
		DeviceID:        deviceID,
		DeviceTouched:   payload.DeviceID != nil,
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
		RunAt:           runAt,
		RunAtTouched:    runAtTouched,
		FinishedAt:      finishedAt,
		FinishedTouched: finishedTouched,
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
			"deviceId":    payload.DeviceID,
			"skillId":     payload.SkillID,
			"status":      payload.Status,
			"costCredits": payload.CostCredits,
			"hasOutput":   outputTouched,
			"hasInput":    inputTouched,
			"runAt":       payload.RunAt,
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
	if !computeAIJobActions(existing, 0).CanCancel {
		render.Error(w, http.StatusConflict, "AI job cannot be cancelled")
		return
	}

	message := "AI 任务已取消"
	job, err := h.app.Store.CancelAIJob(r.Context(), jobID, user.ID, &message)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to cancel AI job")
		return
	}
	if job == nil {
		render.Error(w, http.StatusConflict, "AI job cannot be cancelled")
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
	if !computeAIJobActions(existing, 0).CanRetry {
		render.Error(w, http.StatusConflict, "AI job cannot be retried")
		return
	}

	existingArtifacts, err := h.app.Store.ListAIJobArtifactsByOwner(r.Context(), jobID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI artifacts")
		return
	}

	message := "AI 任务已重新排队"
	job, err := h.app.Store.RetryAIJob(r.Context(), jobID, user.ID, &message)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to retry AI job")
		return
	}
	if job == nil {
		render.Error(w, http.StatusConflict, "AI job cannot be retried")
		return
	}
	_, _ = h.app.Store.DeleteAIJobArtifactsByOwner(r.Context(), jobID, user.ID)
	cleanupAIArtifactFiles(h.app, r.Context(), existingArtifacts)

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

func (h *AIHandler) ForceReleaseJob(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}
	job, err := h.app.Store.ForceReleaseAIJobLeaseByOwner(r.Context(), jobID, user.ID, auditStringPtr("AI 任务租约已由云端手动释放"))
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to force release AI job")
		return
	}
	if job == nil {
		render.Error(w, http.StatusConflict, "AI job has no active lease to force release")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "ai_job",
		ResourceID:   &job.ID,
		Action:       "force_release",
		Title:        "释放 AI 任务租约",
		Source:       job.ModelName,
		Status:       job.Status,
		Message:      job.Message,
	})
	render.JSON(w, http.StatusOK, job)
}

func (h *AIHandler) resolveOwnedDeviceID(w http.ResponseWriter, r *http.Request, raw *string, ownerUserID string) (*string, bool) {
	if raw == nil {
		return nil, true
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil, true
	}
	device, err := h.app.Store.GetOwnedDevice(r.Context(), trimmed, ownerUserID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to validate device")
		return nil, false
	}
	if device == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return nil, false
	}
	return &trimmed, true
}

func parseOptionalRFC3339(w http.ResponseWriter, raw *string, fieldName string) (*time.Time, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		render.Error(w, http.StatusBadRequest, fieldName+" must be RFC3339")
		return nil, false
	}
	return &parsed, true
}

func parseOptionalRFC3339Touched(w http.ResponseWriter, raw *string, fieldName string) (*time.Time, bool, bool) {
	if raw == nil {
		return nil, false, true
	}
	if strings.TrimSpace(*raw) == "" {
		return nil, true, true
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		render.Error(w, http.StatusBadRequest, fieldName+" must be RFC3339")
		return nil, false, false
	}
	return &parsed, true, true
}

func filterAIArtifactsByKeys(items []domain.AIJobArtifact, requested []string) []domain.AIJobArtifact {
	if len(requested) == 0 {
		return items
	}
	allowed := make(map[string]struct{}, len(requested))
	for _, key := range requested {
		key = strings.TrimSpace(key)
		if key != "" {
			allowed[key] = struct{}{}
		}
	}
	filtered := make([]domain.AIJobArtifact, 0, len(items))
	for _, item := range items {
		if _, ok := allowed[item.ArtifactKey]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func collectAIArtifactKeys(items []domain.AIJobArtifact) []string {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.ArtifactKey)
	}
	return keys
}

func buildPublishTaskTitleFromAIJob(job *domain.AIJob, artifacts []domain.AIJobArtifact) string {
	if job == nil {
		return "AI 产物发布任务"
	}
	if job.Prompt != nil && strings.TrimSpace(*job.Prompt) != "" {
		return strings.TrimSpace(*job.Prompt)
	}
	if len(artifacts) > 0 && artifacts[0].FileName != nil && strings.TrimSpace(*artifacts[0].FileName) != "" {
		return strings.TrimSpace(*artifacts[0].FileName)
	}
	return fmt.Sprintf("%s 生成内容发布", job.ModelName)
}

func buildAIArtifactKey(fileName string, artifactType string) string {
	key := strings.TrimSpace(fileName)
	key = strings.ReplaceAll(key, " ", "-")
	key = strings.Trim(key, "-_/")
	if key != "" {
		return key
	}
	return strings.Trim(strings.ReplaceAll(artifactType, " ", "-"), "-_/")
}

func cleanupAIArtifactFiles(app *appstate.App, ctx context.Context, items []domain.AIJobArtifact) {
	if app == nil || app.Storage == nil {
		return
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.StorageKey == nil || strings.TrimSpace(*item.StorageKey) == "" {
			continue
		}
		key := strings.TrimSpace(*item.StorageKey)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		_ = app.Storage.DeleteObject(ctx, key)
	}
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
	case "scheduled":
		return next == "queued" || next == "running" || next == "cancelled" || next == "failed"
	case "queued":
		return next == "scheduled" || next == "running" || next == "cancelled" || next == "failed"
	case "running":
		return next == "success" || next == "completed" || next == "failed" || next == "cancelled"
	case "failed", "cancelled", "success", "completed":
		return false
	default:
		return true
	}
}

func computeAIJobActions(job *domain.AIJob, artifactCount int) domain.AIJobActionState {
	if job == nil {
		return domain.AIJobActionState{}
	}

	state := domain.AIJobActionState{
		CanCreatePublishTask: (job.Status == "success" || job.Status == "completed") && artifactCount > 0 && job.Source != "omnibull_local",
		CanForceRelease:      job.Status == "running" && job.LeaseToken != nil,
	}

	switch job.Status {
	case "queued":
		state.CanEdit = true
		state.CanCancel = true
	case "running":
		state.CanCancel = true
	case "failed", "cancelled", "success", "completed":
		state.CanEdit = true
		state.CanRetry = true
	default:
		state.CanEdit = true
	}
	return state
}

func buildAIJobBridgeState(job *domain.AIJob, artifacts []domain.AIJobArtifact, publishTasks []domain.PublishTask) domain.AIJobBridgeState {
	if job == nil {
		return domain.AIJobBridgeState{}
	}

	stage := "queued_generation"
	if job.Source == "omnibull_local" {
		switch job.Status {
		case "running":
			stage = "generating"
		case "success", "completed":
			stage = "awaiting_omnibull_import"
		case "failed":
			stage = "failed"
		case "cancelled":
			stage = "cancelled"
		}
		switch strings.TrimSpace(job.DeliveryStatus) {
		case "imported":
			stage = "mirrored_to_omnibull"
		case "publish_queued":
			stage = "publish_queued_on_omnibull"
		case "publishing":
			stage = "publishing_on_omnibull"
		case "success", "completed":
			stage = "published_on_omnibull"
		case "failed":
			stage = "publish_failed_on_omnibull"
		case "needs_verify":
			stage = "publish_needs_verify_on_omnibull"
		case "cancelled":
			stage = "cancelled_on_omnibull"
		}
	} else {
		switch job.Status {
		case "running":
			stage = "generating"
		case "success", "completed":
			stage = "output_ready"
		case "failed":
			stage = "failed"
		case "cancelled":
			stage = "cancelled"
		}
		if len(publishTasks) > 0 {
			stage = "publish_tasks_created"
		}
	}
	mirroredCount := 0
	for _, artifact := range artifacts {
		if artifact.DeviceID != nil && artifact.RootName != nil && artifact.RelativePath != nil {
			mirroredCount++
		}
	}
	return domain.AIJobBridgeState{
		Source:                 job.Source,
		GenerationSide:         "omnidrive_cloud",
		TargetDeviceID:         job.DeviceID,
		LocalTaskID:            job.LocalTaskID,
		LocalPublishTaskID:     job.LocalPublishTaskID,
		DeliveryStage:          stage,
		ArtifactCount:          len(artifacts),
		MirroredArtifactCount:  mirroredCount,
		LinkedPublishTaskCount: len(publishTasks),
	}
}
