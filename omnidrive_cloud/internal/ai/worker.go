package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
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

	go func() {
		defer wg.Done()
		w.run(ctx)
	}()

	return func() {
		cancel()
		wg.Wait()
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
	recovered, err := w.app.Store.RecoverExpiredExecutableAIJobLeases(ctx)
	if err != nil {
		log.Printf("omnidrive ai worker: recover expired leases failed: %v", err)
	} else if len(recovered) > 0 {
		log.Printf("omnidrive ai worker: recovered %d expired AI job leases", len(recovered))
	}

	limit := w.concurrency * 4
	if limit < 8 {
		limit = 8
	}
	jobs, err := w.app.Store.ListExecutableAIJobs(ctx, limit)
	if err != nil {
		log.Printf("omnidrive ai worker: list executable jobs failed: %v", err)
		return
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
		log.Printf("omnidrive ai worker: claim job %s failed: %v", job.ID, err)
		return
	}
	if claimed == nil {
		return
	}

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
		log.Printf("omnidrive ai worker: update running state for job %s failed: %v", claimed.ID, err)
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

	if execErr == nil || ctx.Err() != nil {
		return
	}

	message := fmt.Sprintf("AI 云端执行失败: %v", execErr)
	if _, err := w.failJob(ctx, claimed.ID, leaseToken, message, nil); err != nil {
		log.Printf("omnidrive ai worker: mark job %s failed: %v", claimed.ID, err)
		return
	}
	w.recordAuditEvent(ctx, claimed, "cloud_generate_failed", "AI 云端生成失败", "failed", stringPtr(message), map[string]any{
		"jobType":   claimed.JobType,
		"modelName": claimed.ModelName,
		"source":    claimed.Source,
	})
	log.Printf("omnidrive ai worker: job %s failed: %v", claimed.ID, execErr)
}

func (w *Worker) executeChat(ctx context.Context, job *domain.AIJob, leaseToken string) error {
	req, err := BuildChatRequest(job)
	if err != nil {
		return err
	}
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

	state := parseVideoExecutionState(job.OutputPayload)
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

		status, err := w.provider.GetVideo(ctx, state.RemoteVideoID)
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
			artifact, err := w.provider.DownloadVideo(ctx, state.RemoteVideoID)
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
		"video": map[string]any{
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
