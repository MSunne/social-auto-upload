package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	logctx "omnidrive_cloud/internal/logging"
	"omnidrive_cloud/internal/store"
)

type Worker struct {
	app               *appstate.App
	provider          Provider
	pollInterval      time.Duration
	videoPollInterval time.Duration
	videoTimeout      time.Duration
	concurrency       int
	activeJobs        sync.Map
	sem               chan struct{}
}

func NewWorker(app *appstate.App) (*Worker, error) {
	if app == nil {
		return nil, fmt.Errorf("app is required")
	}
	provider, err := NewAPIYIProvider(app.Config)
	if err != nil {
		return nil, err
	}

	concurrency := app.Config.AIWorkerConcurrency
	if concurrency <= 0 {
		concurrency = 2
	}
	pollSeconds := app.Config.AIWorkerPollSeconds
	if pollSeconds <= 0 {
		pollSeconds = 5
	}
	videoPollSeconds := app.Config.AIVideoPollSeconds
	if videoPollSeconds <= 0 {
		videoPollSeconds = 6
	}
	videoTimeoutSeconds := app.Config.AIVideoTimeoutSeconds
	if videoTimeoutSeconds <= 0 {
		videoTimeoutSeconds = 600
	}

	return &Worker{
		app:               app,
		provider:          provider,
		pollInterval:      time.Duration(pollSeconds) * time.Second,
		videoPollInterval: time.Duration(videoPollSeconds) * time.Second,
		videoTimeout:      time.Duration(videoTimeoutSeconds) * time.Second,
		concurrency:       concurrency,
		sem:               make(chan struct{}, concurrency),
	}, nil
}

func (w *Worker) Start(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	wg.Add(1)

	w.app.Logger.Info("ai worker started",
		"poll_interval", w.pollInterval.String(),
		"video_poll_interval", w.videoPollInterval.String(),
		"video_timeout", w.videoTimeout.String(),
		"concurrency", w.concurrency,
	)

	go func() {
		defer wg.Done()
		w.run(ctx)
	}()

	return func() {
		cancel()
		wg.Wait()
		w.app.Logger.Info("ai worker stopped")
	}
}

func (w *Worker) run(ctx context.Context) {
	w.runOnce(ctx)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	pollCtx := logctx.WithOperation(ctx, "ai_worker_poll")

	recovered, err := w.app.Store.RecoverExpiredExecutableAIJobLeases(pollCtx)
	if err != nil {
		w.app.Logger.Error("ai worker failed to recover expired ai job leases", "error", err)
	} else if len(recovered) > 0 {
		w.app.Logger.Debug("ai worker recovered expired ai job leases", "count", len(recovered))
	}

	limit := w.concurrency * 4
	if limit < 8 {
		limit = 8
	}
	jobs, err := w.app.Store.ListExecutableAIJobs(pollCtx, limit)
	if err != nil {
		w.app.Logger.Error("ai worker failed to list executable ai jobs", "error", err, "limit", limit)
		return
	}
	if len(jobs) > 0 {
		w.app.Logger.Debug("ai worker discovered executable ai jobs", "count", len(jobs), "limit", limit)
	}

	for _, job := range jobs {
		if _, loaded := w.activeJobs.LoadOrStore(job.ID, struct{}{}); loaded {
			continue
		}
		select {
		case <-ctx.Done():
			w.activeJobs.Delete(job.ID)
			return
		case w.sem <- struct{}{}:
		}

		go func(job domain.AIJob) {
			defer func() {
				<-w.sem
				w.activeJobs.Delete(job.ID)
			}()
			w.processJob(ctx, job)
		}(job)
	}
}

func (w *Worker) processJob(ctx context.Context, job domain.AIJob) {
	leaseToken := uuid.NewString()
	leaseExpiresAt := time.Now().UTC().Add(store.AIJobLeaseTTL())

	claimed, err := w.app.Store.ClaimCloudAIJobLease(ctx, job.ID, leaseToken, leaseExpiresAt)
	if err != nil {
		w.app.Logger.Error("ai worker failed to claim ai job lease", "job_id", job.ID, "error", err)
		return
	}
	if claimed == nil {
		return
	}
	w.app.Logger.Debug("ai worker claimed ai job", "job_id", claimed.ID, "job_type", claimed.JobType, "model_name", claimed.ModelName)

	if claimed.LeaseExpiresAt != nil {
		leaseExpiresAt = *claimed.LeaseExpiresAt
	}
	w.recordAuditEvent(ctx, claimed, "cloud_generate_start", "AI 云端生成开始", claimed.Status, claimed.Message, map[string]any{
		"jobType":   claimed.JobType,
		"modelName": claimed.ModelName,
		"source":    claimed.Source,
		"deviceId":  claimed.DeviceID,
	})

	if _, err := w.syncRunningState(ctx, claimed, leaseToken, "AI 云端执行中", claimed.OutputPayload); err != nil {
		w.app.Logger.Warn("ai worker failed to sync running state", "job_id", claimed.ID, "error", err)
	}

	var execErr error
	switch strings.TrimSpace(claimed.JobType) {
	case "chat":
		execErr = w.executeChat(ctx, claimed, leaseToken)
	case "image":
		execErr = w.executeImage(ctx, claimed, leaseToken)
	case "video":
		execErr = w.executeVideo(ctx, claimed, leaseToken, leaseExpiresAt)
	default:
		execErr = fmt.Errorf("unsupported ai job type: %s", claimed.JobType)
	}

	if execErr == nil {
		w.app.Logger.Debug("ai worker completed ai job", "job_id", claimed.ID, "job_type", claimed.JobType, "model_name", claimed.ModelName)
		return
	}
	if ctx.Err() != nil {
		w.app.Logger.Info("ai worker interrupted while processing ai job", "job_id", claimed.ID, "error", ctx.Err())
		return
	}

	message := fmt.Sprintf("AI 云端执行失败: %v", execErr)
	if _, err := w.failJob(ctx, claimed.ID, leaseToken, message, nil); err != nil {
		w.app.Logger.Error("ai worker failed to mark ai job as failed", "job_id", claimed.ID, "error", err)
		return
	}
	w.recordAuditEvent(ctx, claimed, "cloud_generate_failed", "AI 云端生成失败", "failed", stringPtr(message), map[string]any{
		"jobType":   claimed.JobType,
		"modelName": claimed.ModelName,
		"source":    claimed.Source,
	})
	w.app.Logger.Error("ai worker ai job failed", "job_id", claimed.ID, "job_type", claimed.JobType, "model_name", claimed.ModelName, "error", execErr)
}

func (w *Worker) executeChat(ctx context.Context, job *domain.AIJob, leaseToken string) error {
	req, err := BuildChatRequest(job)
	if err != nil {
		return err
	}
	originalPrompt := resolveChatPrompt(job, req.Messages)
	storyboardPayload, optimizedPrompt, err := w.prepareStoryboardPrompt(ctx, job, leaseToken, originalPrompt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(optimizedPrompt) != "" {
		req.Messages = replaceLastUserMessage(req.Messages, optimizedPrompt)
	}
	baseURL, apiKey, err := w.resolveModelRuntimeConfig(ctx, job.ModelName)
	if err != nil {
		return err
	}
	req.BaseURL = baseURL
	req.APIKey = apiKey
	result, err := w.provider.GenerateChat(ctx, req)
	if err != nil {
		return err
	}

	artifactPayload := mustJSON(map[string]any{
		"provider":     "apiyi",
		"role":         result.Role,
		"finishReason": result.FinishReason,
		"usage":        result.Usage,
	})
	fileName := "response.txt"
	mimeType := "text/plain; charset=utf-8"
	sizeBytes := int64(len([]byte(result.Text)))
	artifacts, err := w.app.Store.UpsertAIJobArtifacts(ctx, []store.UpsertAIJobArtifactInput{{
		JobID:        job.ID,
		ArtifactKey:  "response.txt",
		ArtifactType: "text",
		Source:       "apiyi",
		Title:        stringPtr("聊天回复"),
		FileName:     &fileName,
		MimeType:     &mimeType,
		SizeBytes:    &sizeBytes,
		TextContent:  stringPtr(result.Text),
		Payload:      artifactPayload,
	}})
	if err != nil {
		return err
	}
	billing := w.applyUsageBilling(ctx, job, buildChatBillingInput(job, result))

	outputPayload := mustJSON(map[string]any{
		"provider":     "apiyi",
		"kind":         "chat",
		"model":        job.ModelName,
		"text":         result.Text,
		"role":         result.Role,
		"finishReason": result.FinishReason,
		"usage":        result.Usage,
		"billing":      billingToPayload(billing),
		"artifacts":    summarizeArtifacts(artifacts),
		"storyboard":   storyboardPayload,
		"completedAt":  time.Now().UTC().Format(time.RFC3339),
	})
	message := buildCompletionMessage("AI 聊天已完成", billing)
	if _, err := w.completeJob(ctx, job, leaseToken, message, outputPayload, billingCreditsPtr(billing)); err != nil {
		return err
	}
	w.recordAuditEvent(ctx, job, "cloud_generate_success", "AI 云端生成完成", "success", stringPtr(message), map[string]any{
		"jobType":       job.JobType,
		"modelName":     job.ModelName,
		"artifactCount": len(artifacts),
	})
	return nil
}

func (w *Worker) executeImage(ctx context.Context, job *domain.AIJob, leaseToken string) error {
	req, err := BuildImageRequest(job)
	if err != nil {
		return err
	}
	storyboardPayload, optimizedPrompt, err := w.prepareStoryboardPrompt(ctx, job, leaseToken, req.Prompt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(optimizedPrompt) != "" {
		req.Prompt = optimizedPrompt
	}
	baseURL, apiKey, err := w.resolveModelRuntimeConfig(ctx, job.ModelName)
	if err != nil {
		return err
	}
	req.BaseURL = baseURL
	req.APIKey = apiKey
	result, err := w.provider.GenerateImage(ctx, req)
	if err != nil {
		return err
	}

	inputs := make([]store.UpsertAIJobArtifactInput, 0, len(result.Images)+1)
	for index, image := range result.Images {
		artifactKey := strings.TrimSpace(image.ArtifactKey)
		if artifactKey == "" {
			artifactKey = fmt.Sprintf("image-%d%s", index+1, extensionForMIME(image.MIMEType, ".png"))
		}
		input, err := w.saveBinaryArtifact(ctx, job, "image", artifactKey, "apiyi", image)
		if err != nil {
			return err
		}
		inputs = append(inputs, input)
	}
	if strings.TrimSpace(result.Text) != "" {
		fileName := "response.txt"
		mimeType := "text/plain; charset=utf-8"
		sizeBytes := int64(len([]byte(result.Text)))
		inputs = append(inputs, store.UpsertAIJobArtifactInput{
			JobID:        job.ID,
			ArtifactKey:  "response.txt",
			ArtifactType: "text",
			Source:       "apiyi",
			Title:        stringPtr("图片生成说明"),
			FileName:     &fileName,
			MimeType:     &mimeType,
			SizeBytes:    &sizeBytes,
			TextContent:  stringPtr(result.Text),
			Payload: mustJSON(map[string]any{
				"provider": "apiyi",
				"kind":     "image",
			}),
		})
	}

	artifacts, err := w.app.Store.UpsertAIJobArtifacts(ctx, inputs)
	if err != nil {
		return err
	}
	billing := w.applyUsageBilling(ctx, job, buildImageBillingInput(job, len(result.Images)))
	outputPayload := mustJSON(map[string]any{
		"provider":    "apiyi",
		"kind":        "image",
		"model":       job.ModelName,
		"text":        result.Text,
		"billing":     billingToPayload(billing),
		"artifacts":   summarizeArtifacts(artifacts),
		"storyboard":  storyboardPayload,
		"completedAt": time.Now().UTC().Format(time.RFC3339),
	})
	message := buildCompletionMessage(fmt.Sprintf("AI 图片生成完成，共生成 %d 个结果", len(result.Images)), billing)
	if _, err := w.completeJob(ctx, job, leaseToken, message, outputPayload, billingCreditsPtr(billing)); err != nil {
		return err
	}
	w.recordAuditEvent(ctx, job, "cloud_generate_success", "AI 云端生成完成", "success", stringPtr(message), map[string]any{
		"jobType":       job.JobType,
		"modelName":     job.ModelName,
		"artifactCount": len(artifacts),
	})
	return nil
}

func (w *Worker) executeVideo(ctx context.Context, job *domain.AIJob, leaseToken string, leaseExpiresAt time.Time) error {
	req, err := BuildVideoRequest(job)
	if err != nil {
		return err
	}
	storyboardPayload, optimizedPrompt, err := w.prepareStoryboardPrompt(ctx, job, leaseToken, req.Prompt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(optimizedPrompt) != "" {
		req.Prompt = optimizedPrompt
	}
	baseURL, apiKey, err := w.resolveModelRuntimeConfig(ctx, job.ModelName)
	if err != nil {
		return err
	}
	req.BaseURL = baseURL
	req.APIKey = apiKey

	state := parseVideoExecutionState(job.OutputPayload)
	if strings.TrimSpace(state.BaseURL) == "" {
		state.BaseURL = baseURL
	}
	if strings.TrimSpace(state.RemoteVideoID) == "" {
		submission, err := w.provider.SubmitVideo(ctx, req)
		if err != nil {
			return err
		}
		if strings.TrimSpace(submission.ID) == "" {
			return fmt.Errorf("video submission did not return id")
		}
		state.RemoteVideoID = submission.ID
		state.RemoteStatus = strings.TrimSpace(submission.Status)
		state.SubmittedAt = firstNonNilTime(submission.CreatedAt, time.Now().UTC())
		state.UpdatedAt = state.SubmittedAt

		message := "AI 视频任务已提交，等待生成完成"
		runningPayload := buildVideoOutputPayload(job, state, nil)
		if _, err := w.syncRunningState(ctx, job, leaseToken, message, runningPayload); err != nil {
			return err
		}
	}

	deadline := state.SubmittedAt.Add(w.videoTimeout)
	if deadline.Before(time.Now().UTC()) {
		return fmt.Errorf("video generation timeout before polling started")
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var renewErr error
		leaseExpiresAt, renewErr = w.renewLease(ctx, job.ID, leaseToken, leaseExpiresAt)
		if renewErr != nil {
			return renewErr
		}

		status, err := w.provider.GetVideo(ctx, state.RemoteVideoID, req.Model, state.BaseURL, apiKey)
		if err != nil {
			return err
		}
		state.RemoteStatus = strings.TrimSpace(status.Status)
		state.ContentURL = strings.TrimSpace(status.ContentURL)
		state.UpdatedAt = firstNonNilTime(status.UpdatedAt, time.Now().UTC())
		if status.Message != "" {
			state.Message = status.Message
		}
		if status.FailureCode != "" {
			state.FailureCode = status.FailureCode
		}

		switch state.RemoteStatus {
		case "completed":
			artifact, err := w.provider.DownloadVideo(ctx, state.RemoteVideoID, req.Model, state.BaseURL, apiKey)
			if err != nil {
				return err
			}
			if strings.TrimSpace(artifact.FileName) == "" {
				artifact.FileName = "video.mp4"
			}
			if strings.TrimSpace(artifact.MIMEType) == "" {
				artifact.MIMEType = "video/mp4"
			}
			artifactKey := strings.TrimSpace(artifact.ArtifactKey)
			if artifactKey == "" {
				artifactKey = safeArtifactKey(artifact.FileName, "video.mp4")
			}
			input, err := w.saveBinaryArtifact(ctx, job, "video", artifactKey, "apiyi", *artifact)
			if err != nil {
				return err
			}
			artifacts, err := w.app.Store.UpsertAIJobArtifacts(ctx, []store.UpsertAIJobArtifactInput{input})
			if err != nil {
				return err
			}
			billing := w.applyUsageBilling(ctx, job, buildVideoBillingInput(job))
			outputPayload := buildVideoOutputPayload(job, state, artifacts)
			if len(storyboardPayload) > 0 {
				outputPayload = mergeMetadataIntoPayload(outputPayload, "storyboard", storyboardPayload)
			}
			outputPayload = mergeBillingIntoPayload(outputPayload, billing)
			message := buildCompletionMessage("AI 视频生成完成", billing)
			if _, err := w.completeJob(ctx, job, leaseToken, message, outputPayload, billingCreditsPtr(billing)); err != nil {
				return err
			}
			w.recordAuditEvent(ctx, job, "cloud_generate_success", "AI 云端生成完成", "success", stringPtr(message), map[string]any{
				"jobType":       job.JobType,
				"modelName":     job.ModelName,
				"artifactCount": len(artifacts),
				"remoteVideoId": state.RemoteVideoID,
			})
			return nil
		case "failed":
			if state.FailureCode != "" && state.Message != "" {
				return fmt.Errorf("%s: %s", state.FailureCode, state.Message)
			}
			if state.Message != "" {
				return errors.New(state.Message)
			}
			if state.FailureCode != "" {
				return errors.New(state.FailureCode)
			}
			return fmt.Errorf("video generation failed")
		default:
			if time.Now().UTC().After(deadline) {
				return fmt.Errorf("video generation timed out after %s", w.videoTimeout)
			}
			message := "AI 视频生成中"
			if strings.TrimSpace(state.Message) != "" {
				message = "AI 视频生成中: " + strings.TrimSpace(state.Message)
			}
			runningPayload := buildVideoOutputPayload(job, state, nil)
			if _, err := w.syncRunningState(ctx, job, leaseToken, message, runningPayload); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(w.videoPollInterval):
			}
		}
	}
}

func (w *Worker) prepareStoryboardPrompt(ctx context.Context, job *domain.AIJob, leaseToken string, originalPrompt string) (map[string]any, string, error) {
	payload := decodePayloadMap(job.InputPayload)
	config, _ := payload["storyboardConfig"].(map[string]any)
	if !boolValue(config["enabled"]) {
		return nil, originalPrompt, nil
	}

	modelName := strings.TrimSpace(stringValueFromMap(config, "modelName"))
	if modelName == "" {
		return nil, originalPrompt, nil
	}

	referenceTexts := normalizeStoryboardTexts(payload["referenceTexts"])
	referenceImages := normalizeStoryboardImages(payload["referenceImages"])

	systemPrompt := strings.TrimSpace(stringValueFromMap(config, "prompt"))
	if systemPrompt == "" {
		systemPrompt = "你是内容创作分镜与脚本优化助手。请结合用户目标、参考图片和参考文本，输出适合继续交给图片、视频或文本模型执行的精炼脚本。输出中需要保留主体、场景、镜头、风格、文案和节奏等关键信息。"
	}

	baseURL, apiKey, err := w.resolveModelRuntimeConfig(ctx, modelName)
	if err != nil {
		return nil, originalPrompt, err
	}
	userPrompt := buildStoryboardPrompt(job, originalPrompt, payload, referenceTexts, referenceImages, config["references"])

	stagePayload := mustJSON(map[string]any{
		"stage":     "storyboarding",
		"modelName": modelName,
		"startedAt": time.Now().UTC().Format(time.RFC3339),
	})
	if _, err := w.syncRunningState(ctx, job, leaseToken, "AI 正在优化分镜脚本", stagePayload); err != nil {
		w.app.Logger.Warn("ai worker failed to sync storyboarding state", "job_id", job.ID, "error", err)
	}

	result, err := w.provider.GenerateChat(ctx, ChatRequest{
		Model:   modelName,
		BaseURL: baseURL,
		APIKey:  apiKey,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return nil, originalPrompt, err
	}

	optimizedPrompt := strings.TrimSpace(result.Text)
	if optimizedPrompt == "" {
		optimizedPrompt = originalPrompt
	}

	return map[string]any{
		"modelName":       modelName,
		"promptTemplate":  systemPrompt,
		"optimizedPrompt": optimizedPrompt,
		"referenceCount": map[string]int{
			"images": len(referenceImages),
			"texts":  len(referenceTexts),
		},
	}, optimizedPrompt, nil
}

func (w *Worker) renewLease(ctx context.Context, jobID string, leaseToken string, leaseExpiresAt time.Time) (time.Time, error) {
	if time.Until(leaseExpiresAt) > store.AIJobLeaseTTL()/2 {
		return leaseExpiresAt, nil
	}
	nextExpiry := time.Now().UTC().Add(store.AIJobLeaseTTL())
	renewed, err := w.app.Store.RenewCloudAIJobLease(ctx, jobID, leaseToken, nextExpiry)
	if err != nil {
		return leaseExpiresAt, err
	}
	if renewed == nil || renewed.LeaseExpiresAt == nil {
		return leaseExpiresAt, fmt.Errorf("failed to renew lease for job %s", jobID)
	}
	return *renewed.LeaseExpiresAt, nil
}

func (w *Worker) syncRunningState(ctx context.Context, job *domain.AIJob, leaseToken string, message string, outputPayload []byte) (*domain.AIJob, error) {
	return w.app.Store.SyncCloudAIJobExecution(ctx, job.ID, leaseToken, store.UpdateAIJobInput{
		Message:       stringPtr(message),
		OutputPayload: outputPayload,
		OutputTouched: len(outputPayload) > 0,
	})
}

func (w *Worker) completeJob(ctx context.Context, job *domain.AIJob, leaseToken string, message string, outputPayload []byte, costCredits *int64) (*domain.AIJob, error) {
	status := "success"
	return w.app.Store.SyncCloudAIJobExecution(ctx, job.ID, leaseToken, store.UpdateAIJobInput{
		Status:        &status,
		Message:       stringPtr(message),
		OutputPayload: outputPayload,
		OutputTouched: len(outputPayload) > 0,
		CostCredits:   costCredits,
	})
}

func (w *Worker) failJob(ctx context.Context, jobID string, leaseToken string, message string, outputPayload []byte) (*domain.AIJob, error) {
	status := "failed"
	return w.app.Store.SyncCloudAIJobExecution(ctx, jobID, leaseToken, store.UpdateAIJobInput{
		Status:        &status,
		Message:       stringPtr(message),
		OutputPayload: outputPayload,
		OutputTouched: len(outputPayload) > 0,
	})
}

func (w *Worker) saveBinaryArtifact(ctx context.Context, job *domain.AIJob, artifactType string, artifactKey string, source string, artifact BinaryArtifact) (store.UpsertAIJobArtifactInput, error) {
	fileName := safeFileName(artifact.FileName)
	if fileName == "" {
		fileName = artifactType + extensionForMIME(artifact.MIMEType, ".bin")
	}
	object, err := w.app.Storage.SaveBytes(
		ctx,
		fmt.Sprintf("ai-jobs/%s/%s/%s/%s", job.OwnerUserID, job.ID, artifactType, uuid.NewString()+"-"+fileName),
		artifact.MIMEType,
		artifact.Data,
	)
	if err != nil {
		return store.UpsertAIJobArtifactInput{}, err
	}

	return store.UpsertAIJobArtifactInput{
		JobID:        job.ID,
		ArtifactKey:  safeArtifactKey(artifactKey, fileName),
		ArtifactType: artifactType,
		Source:       source,
		Title:        stringPtr(fileName),
		FileName:     &fileName,
		MimeType:     &object.ContentType,
		StorageKey:   &object.StorageKey,
		PublicURL:    &object.PublicURL,
		SizeBytes:    &object.SizeBytes,
		Payload: mustJSON(map[string]any{
			"provider": "apiyi",
			"metadata": artifact.Metadata,
		}),
	}, nil
}

func (w *Worker) recordAuditEvent(ctx context.Context, job *domain.AIJob, action string, title string, status string, message *string, payload map[string]any) {
	if job == nil {
		return
	}
	input := store.CreateAuditEventInput{
		ID:           uuid.NewString(),
		OwnerUserID:  job.OwnerUserID,
		ResourceType: "ai_job",
		ResourceID:   &job.ID,
		Action:       action,
		Title:        title,
		Source:       job.ModelName,
		Status:       status,
		Message:      message,
		Payload:      mustJSON(payload),
	}
	_ = w.app.Store.CreateAuditEvent(ctx, input)
}

func (w *Worker) applyUsageBilling(ctx context.Context, job *domain.AIJob, input store.ApplyUsageBillingInput) *store.ApplyUsageBillingResult {
	if strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.SourceID) == "" {
		return &store.ApplyUsageBillingResult{
			BillStatus:  "skipped",
			BillMessage: "billing input incomplete",
			Details:     []store.UsageBillingDetail{},
		}
	}

	result, err := w.app.Store.ApplyUsageBilling(ctx, input)
	if err != nil {
		message := fmt.Sprintf("AI 计费失败: %v", err)
		w.recordAuditEvent(ctx, job, "ai_billing_failed", "AI 计费失败", "failed", stringPtr(message), map[string]any{
			"jobType":   job.JobType,
			"modelName": job.ModelName,
			"source":    job.Source,
		})
		return &store.ApplyUsageBillingResult{
			BillStatus:  "failed",
			BillMessage: message,
			Details:     []store.UsageBillingDetail{},
		}
	}

	switch result.BillStatus {
	case "billed":
		message := fmt.Sprintf("AI 计费完成，扣减 %d 积分", result.TotalCredits)
		w.recordAuditEvent(ctx, job, "ai_billing_billed", "AI 计费完成", "success", stringPtr(message), map[string]any{
			"jobType":      job.JobType,
			"modelName":    job.ModelName,
			"source":       job.Source,
			"totalCredits": result.TotalCredits,
			"details":      result.Details,
		})
	case "failed":
		message := result.BillMessage
		if strings.TrimSpace(message) == "" {
			message = "AI 计费失败"
		}
		w.recordAuditEvent(ctx, job, "ai_billing_failed", "AI 计费失败", "failed", stringPtr(message), map[string]any{
			"jobType":   job.JobType,
			"modelName": job.ModelName,
			"source":    job.Source,
			"details":   result.Details,
		})
	}

	return result
}

type videoExecutionState struct {
	BaseURL       string
	RemoteVideoID string
	RemoteStatus  string
	ContentURL    string
	Message       string
	FailureCode   string
	SubmittedAt   time.Time
	UpdatedAt     time.Time
}

func buildVideoOutputPayload(job *domain.AIJob, state videoExecutionState, artifacts []domain.AIJobArtifact) []byte {
	return mustJSON(map[string]any{
		"provider": "apiyi",
		"kind":     "video",
		"model":    job.ModelName,
		"baseUrl":  state.BaseURL,
		"video": map[string]any{
			"baseUrl":     state.BaseURL,
			"id":          state.RemoteVideoID,
			"status":      state.RemoteStatus,
			"contentUrl":  state.ContentURL,
			"message":     state.Message,
			"failureCode": state.FailureCode,
			"submittedAt": state.SubmittedAt.Format(time.RFC3339),
			"updatedAt":   state.UpdatedAt.Format(time.RFC3339),
		},
		"artifacts": summarizeArtifacts(artifacts),
	})
}

func mergeBillingIntoPayload(raw []byte, billing *store.ApplyUsageBillingResult) []byte {
	payload := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &payload)
	}
	payload["billing"] = billingToPayload(billing)
	return mustJSON(payload)
}

func mergeMetadataIntoPayload(raw []byte, key string, value any) []byte {
	payload := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &payload)
	}
	payload[key] = value
	return mustJSON(payload)
}

func parseVideoExecutionState(raw []byte) videoExecutionState {
	state := videoExecutionState{
		SubmittedAt: time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if len(raw) == 0 {
		return state
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return state
	}
	videoPayload, _ := payload["video"].(map[string]any)
	state.BaseURL = strings.TrimSpace(firstNonEmptyString(
		stringValue(payload["baseUrl"]),
		stringValue(videoPayload["baseUrl"]),
	))
	state.RemoteVideoID = strings.TrimSpace(firstNonEmptyString(
		stringValue(payload["remoteVideoId"]),
		stringValue(videoPayload["id"]),
	))
	state.RemoteStatus = strings.TrimSpace(firstNonEmptyString(
		stringValue(payload["remoteStatus"]),
		stringValue(videoPayload["status"]),
	))
	state.ContentURL = strings.TrimSpace(firstNonEmptyString(
		stringValue(payload["contentUrl"]),
		stringValue(videoPayload["contentUrl"]),
	))
	state.Message = strings.TrimSpace(firstNonEmptyString(
		stringValue(payload["message"]),
		stringValue(videoPayload["message"]),
	))
	state.FailureCode = strings.TrimSpace(firstNonEmptyString(
		stringValue(payload["failureCode"]),
		stringValue(videoPayload["failureCode"]),
	))
	if parsed, ok := parseRFC3339(firstNonEmptyString(stringValue(payload["submittedAt"]), stringValue(videoPayload["submittedAt"]))); ok {
		state.SubmittedAt = parsed
	}
	if parsed, ok := parseRFC3339(firstNonEmptyString(stringValue(payload["updatedAt"]), stringValue(videoPayload["updatedAt"]))); ok {
		state.UpdatedAt = parsed
	}
	return state
}

func (w *Worker) resolveModelRuntimeConfig(ctx context.Context, modelName string) (string, string, error) {
	model, err := w.app.Store.GetAIModelByName(ctx, modelName)
	if err != nil {
		return "", "", err
	}
	if model == nil {
		return "", "", fmt.Errorf("ai model not found: %s", modelName)
	}
	baseURL := ""
	if model.BaseURL == nil {
		baseURL = ""
	} else {
		baseURL = strings.TrimSpace(*model.BaseURL)
	}
	apiKey := ""
	if model.APIKey != nil {
		apiKey = strings.TrimSpace(*model.APIKey)
	}
	return baseURL, apiKey, nil
}

func summarizeArtifacts(items []domain.AIJobArtifact) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id":           item.ID,
			"artifactKey":  item.ArtifactKey,
			"artifactType": item.ArtifactType,
			"fileName":     stringValue(item.FileName),
			"mimeType":     stringValue(item.MimeType),
			"publicUrl":    stringValue(item.PublicURL),
			"storageKey":   stringValue(item.StorageKey),
			"sizeBytes":    item.SizeBytes,
			"textContent":  stringValue(item.TextContent),
		})
	}
	return result
}

func buildStoryboardPrompt(job *domain.AIJob, originalPrompt string, payload map[string]any, referenceTexts []map[string]string, referenceImages []map[string]string, references any) string {
	var builder strings.Builder
	builder.WriteString("请优化下面的内容创作需求，并输出适合继续交给生成模型执行的分镜脚本。\n")
	builder.WriteString("任务类型: ")
	builder.WriteString(strings.TrimSpace(job.JobType))
	builder.WriteString("\n")
	if name := strings.TrimSpace(stringValueFromMap(payload, "skillName")); name != "" {
		builder.WriteString("技能名称: ")
		builder.WriteString(name)
		builder.WriteString("\n")
	}
	if desc := strings.TrimSpace(stringValueFromMap(payload, "skillDescription")); desc != "" {
		builder.WriteString("技能说明: ")
		builder.WriteString(desc)
		builder.WriteString("\n")
	}
	builder.WriteString("用户提示词: ")
	builder.WriteString(strings.TrimSpace(originalPrompt))
	builder.WriteString("\n")

	if len(referenceTexts) > 0 {
		builder.WriteString("\n参考文本:\n")
		for _, item := range referenceTexts {
			builder.WriteString("- ")
			builder.WriteString(item["fileName"])
			builder.WriteString(": ")
			builder.WriteString(item["content"])
			builder.WriteString("\n")
		}
	}
	if len(referenceImages) > 0 {
		builder.WriteString("\n参考图片:\n")
		for _, item := range referenceImages {
			builder.WriteString("- ")
			builder.WriteString(item["fileName"])
			if item["publicUrl"] != "" {
				builder.WriteString(" (")
				builder.WriteString(item["publicUrl"])
				builder.WriteString(")")
			}
			builder.WriteString("\n")
		}
	}
	if references != nil {
		if raw, err := json.Marshal(references); err == nil && len(raw) > 0 {
			builder.WriteString("\n管理员补充参考:\n")
			builder.Write(raw)
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\n请直接输出最终可执行脚本，不要解释过程。")
	return builder.String()
}

func resolveChatPrompt(job *domain.AIJob, messages []ChatMessage) string {
	prompt := strings.TrimSpace(stringValue(job.Prompt))
	if prompt != "" {
		return prompt
	}
	payload := decodePayloadMap(job.InputPayload)
	if raw := strings.TrimSpace(stringValueFromMap(payload, "prompt")); raw != "" {
		return raw
	}
	for index := len(messages) - 1; index >= 0; index-- {
		if strings.EqualFold(strings.TrimSpace(messages[index].Role), "user") {
			return strings.TrimSpace(fmt.Sprint(messages[index].Content))
		}
	}
	return ""
}

func replaceLastUserMessage(messages []ChatMessage, prompt string) []ChatMessage {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return messages
	}
	for index := len(messages) - 1; index >= 0; index-- {
		if strings.EqualFold(strings.TrimSpace(messages[index].Role), "user") {
			messages[index].Content = prompt
			return messages
		}
	}
	return append(messages, ChatMessage{Role: "user", Content: prompt})
}

func normalizeStoryboardTexts(raw any) []map[string]string {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]string, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		content := strings.TrimSpace(stringValueFromMap(obj, "content"))
		if content == "" {
			continue
		}
		result = append(result, map[string]string{
			"fileName": strings.TrimSpace(stringValueFromMap(obj, "fileName")),
			"content":  content,
		})
	}
	return result
}

func normalizeStoryboardImages(raw any) []map[string]string {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]string, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		url := strings.TrimSpace(stringValueFromMap(obj, "publicUrl", "url"))
		if url == "" {
			continue
		}
		result = append(result, map[string]string{
			"fileName":  strings.TrimSpace(stringValueFromMap(obj, "fileName")),
			"publicUrl": url,
		})
	}
	return result
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		value := strings.TrimSpace(strings.ToLower(typed))
		return value == "true" || value == "1" || value == "yes"
	case float64:
		return typed != 0
	default:
		return false
	}
}

func billingToPayload(result *store.ApplyUsageBillingResult) map[string]any {
	if result == nil {
		return map[string]any{
			"billStatus": "skipped",
		}
	}
	return map[string]any{
		"billStatus":    result.BillStatus,
		"billMessage":   result.BillMessage,
		"totalCredits":  result.TotalCredits,
		"alreadyBilled": result.AlreadyBilled,
		"details":       result.Details,
	}
}

func buildCompletionMessage(base string, billing *store.ApplyUsageBillingResult) string {
	if billing == nil {
		return base
	}
	switch billing.BillStatus {
	case "billed":
		if billing.TotalCredits > 0 {
			return fmt.Sprintf("%s，已扣减 %d 积分", base, billing.TotalCredits)
		}
		return base
	case "failed":
		if strings.TrimSpace(billing.BillMessage) != "" {
			return fmt.Sprintf("%s，计费待处理: %s", base, strings.TrimSpace(billing.BillMessage))
		}
		return base + "，计费待处理"
	default:
		return base
	}
}

func billingCreditsPtr(result *store.ApplyUsageBillingResult) *int64 {
	if result == nil {
		return nil
	}
	if result.BillStatus != "billed" {
		return nil
	}
	credits := result.TotalCredits
	return &credits
}

func mustJSON(payload any) []byte {
	if payload == nil {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return data
}

func safeFileName(value string) string {
	value = strings.TrimSpace(filepath.Base(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Trim(value, "-_/")
	return value
}

func safeArtifactKey(primary string, fallback string) string {
	primary = safeFileName(primary)
	if primary != "" {
		return primary
	}
	return safeFileName(fallback)
}

func parseRFC3339(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

func firstNonNilTime(value *time.Time, fallback time.Time) time.Time {
	if value != nil {
		return value.UTC()
	}
	return fallback.UTC()
}
