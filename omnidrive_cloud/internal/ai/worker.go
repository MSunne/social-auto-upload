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
	providers         map[string]Provider
	pollInterval      time.Duration
	videoPollInterval time.Duration
	videoTimeout      time.Duration
	staleQueueTimeout time.Duration
	concurrency       int
	activeJobs        sync.Map
	sem               chan struct{}
}

type requeueExecutionError struct {
	Message       string
	OutputPayload []byte
}

type executionBillingBlockedError struct {
	Result *store.ApplyUsageBillingResult
}

type storyboardOptimizationEnvelope struct {
	GenerationPrompt string `json:"generationPrompt"`
	PublishIntro     string `json:"publishIntro"`
}

const (
	videoArtifactFinalizeMaxAttempts = 3
	videoArtifactFinalizeRetryDelay  = 2 * time.Second
	defaultStoryboardSystemPrompt    = DefaultVideoStoryboardSystemPrompt
	defaultPublishIntroPrompt        = "请结合任务说明、基础简介、标签、分镜脚本与参考资料，生成一段适合直接发布到第三方平台的简介。要求与生成内容保持同一主题和卖点，保留客户既定风格，但每次表达都自然变化，避免模板化和完全重复。"
)

const executionBillingRetryDelay = time.Minute
const mediaFailureAutoRetryLimit = 1
const mediaFailureAutoRetryDelay = 30 * time.Second

// 返回需要重新入队的执行错误消息，供日志记录和上层重试判断复用。
func (e *requeueExecutionError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "execution should be requeued"
	}
	return strings.TrimSpace(e.Message)
}

// 返回计费阻塞错误消息，提示调用方当前作业需要等待额度恢复后再继续执行。
func (e *executionBillingBlockedError) Error() string {
	return BuildUsageBillingBlockMessage(e.Result)
}

// 创建 AI worker，装配供应方客户端、轮询参数和并发控制所需依赖。
func NewWorker(app *appstate.App) (*Worker, error) {
	if app == nil {
		return nil, fmt.Errorf("app is required")
	}
	providers, err := newProviders(app.Config)
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
	staleQueueTimeoutSeconds := app.Config.AIStaleQueueTimeoutSeconds

	return &Worker{
		app:               app,
		providers:         providers,
		pollInterval:      time.Duration(pollSeconds) * time.Second,
		videoPollInterval: time.Duration(videoPollSeconds) * time.Second,
		videoTimeout:      time.Duration(videoTimeoutSeconds) * time.Second,
		staleQueueTimeout: time.Duration(staleQueueTimeoutSeconds) * time.Second,
		concurrency:       concurrency,
		sem:               make(chan struct{}, concurrency),
	}, nil
}

// 启动 AI worker 后台循环，持续轮询可执行作业并在退出时返回停止函数。
func (w *Worker) Start(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	wg.Add(1)

	w.app.Logger.Info("ai worker started",
		"poll_interval", w.pollInterval.String(),
		"video_poll_interval", w.videoPollInterval.String(),
		"video_timeout", w.videoTimeout.String(),
		"stale_queue_timeout", w.staleQueueTimeout.String(),
		"concurrency", w.concurrency,
	)

	go func() {
		defer wg.Done()
		w.recoverInterruptedJobsOnStartup(ctx)
		w.run(ctx)
	}()

	return func() {
		cancel()
		wg.Wait()
		w.app.Logger.Info("ai worker stopped")
	}
}

// 在 worker 启动时恢复中断的作业状态，避免遗留租约导致任务长期滞留。
func (w *Worker) recoverInterruptedJobsOnStartup(ctx context.Context) {
	recovered, err := w.app.Store.RecoverInterruptedExecutableAIJobs(logctx.WithOperation(ctx, "ai_worker_startup_recovery"))
	if err != nil {
		w.app.Logger.Error("ai worker failed to recover interrupted ai jobs on startup", "error", err)
		return
	}
	if len(recovered) == 0 {
		return
	}
	w.app.Logger.Info("ai worker recovered interrupted ai jobs on startup", "count", len(recovered))
}

// 处理 AI 作业执行中的运行流程，依赖作业状态、租约和持久化结果推进链路。
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

// 执行一轮 AI 作业拉取与处理循环，供后台 worker 按固定节奏持续轮询。
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
	if w.staleQueueTimeout > 0 {
		timedOut, err := w.app.Store.FailStaleQueuedExecutableAIJobs(pollCtx, time.Now().UTC().Add(-w.staleQueueTimeout), limit)
		if err != nil {
			w.app.Logger.Error("ai worker failed to auto-fail stale queued ai jobs", "error", err, "timeout", w.staleQueueTimeout.String())
		} else if len(timedOut) > 0 {
			w.app.Logger.Warn("ai worker auto-failed stale queued ai jobs", "count", len(timedOut), "timeout", w.staleQueueTimeout.String())
		}
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

// 处理单个 AI 作业的完整执行链路，串联租约、调用、计费和结果回写。
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
	stopLeaseHeartbeat := w.startLeaseHeartbeat(ctx, claimed.ID, leaseToken)
	defer stopLeaseHeartbeat()

	if err := w.ensureExecutionBilling(ctx, claimed); err != nil {
		var blockedErr *executionBillingBlockedError
		if errors.As(err, &blockedErr) {
			message := blockedErr.Error()
			if _, pauseErr := w.pauseJobForRecharge(ctx, claimed.ID, leaseToken, message, nil, executionBillingRetryDelay); pauseErr != nil {
				w.app.Logger.Error("ai worker failed to pause billing-blocked ai job", "job_id", claimed.ID, "error", pauseErr)
				return
			}
			w.recordBillingBlockedAudit(ctx, claimed, message, blockedErr.Result)
			w.app.Logger.Info("ai worker paused ai job before execution due to insufficient credits", "job_id", claimed.ID, "job_type", claimed.JobType, "model_name", claimed.ModelName, "bill_message", stringValue(resultBillMessage(blockedErr.Result)), "retry_after", executionBillingRetryDelay.String())
			return
		}

		message := fmt.Sprintf("任务启动前计费失败，请稍后重试: %v", err)
		if _, failErr := w.failJob(ctx, claimed.ID, leaseToken, message, nil); failErr != nil {
			w.app.Logger.Error("ai worker failed to mark billing-check ai job as failed", "job_id", claimed.ID, "error", failErr)
			return
		}
		w.recordAuditEvent(ctx, claimed, "ai_billing_precheck_error", "AI 启动前计费失败", "failed", stringPtr(message), map[string]any{
			"jobType":   claimed.JobType,
			"modelName": claimed.ModelName,
			"source":    claimed.Source,
		})
		w.app.Logger.Error("ai worker failed execution billing check", "job_id", claimed.ID, "job_type", claimed.JobType, "model_name", claimed.ModelName, "error", err)
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
	case "digital_human":
		execErr = w.executeDigitalHumanVideo(ctx, claimed, leaseToken)
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

	var requeueErr *requeueExecutionError
	if errors.As(execErr, &requeueErr) {
		message := strings.TrimSpace(requeueErr.Message)
		if message == "" {
			message = "AI 云端任务已重新排队"
		}
		if _, err := w.requeueJob(ctx, claimed.ID, leaseToken, message, requeueErr.OutputPayload); err != nil {
			w.app.Logger.Error("ai worker failed to requeue ai job", "job_id", claimed.ID, "error", err)
			return
		}
		w.app.Logger.Info("ai worker requeued ai job for follow-up polling", "job_id", claimed.ID, "job_type", claimed.JobType, "message", message)
		return
	}

	if shouldResumeRemoteVideoPolling(claimed, execErr) {
		state := parseVideoExecutionState(claimed.OutputPayload)
		message := "AI 视频服务暂时波动，继续后台回查云端结果"
		if _, err := w.requeueJobWithBackoff(ctx, claimed.ID, leaseToken, message, buildVideoOutputPayload(claimed, state, nil), mediaFailureAutoRetryDelay); err != nil {
			w.app.Logger.Error("ai worker failed to continue remote video polling after transient error", "job_id", claimed.ID, "error", err)
			return
		}
		w.app.Logger.Warn("ai worker kept remote video job queued after transient error", "job_id", claimed.ID, "job_type", claimed.JobType, "remote_video_id", state.RemoteVideoID, "error", execErr)
		return
	}

	message := buildAIExecutionFailureMessage(claimed.JobType, execErr)
	if shouldAutoRetryMediaFailure(claimed, execErr) {
		retryCount := mediaAutoRetryCountFromPayload(claimed.OutputPayload) + 1
		retryMessage := buildMediaAutoRetryMessage(claimed.JobType, retryCount)
		retryPayload := buildMediaAutoRetryPayload(claimed, message, retryCount)
		w.returnUsageCreditsForFailure(ctx, claimed, message)
		if _, err := w.requeueJobWithBackoff(ctx, claimed.ID, leaseToken, retryMessage, retryPayload, mediaFailureAutoRetryDelay); err != nil {
			w.app.Logger.Error("ai worker failed to auto-retry media ai job", "job_id", claimed.ID, "job_type", claimed.JobType, "error", err)
			return
		}
		w.recordAuditEvent(ctx, claimed, "cloud_generate_auto_retry", "AI 云端生成失败后自动重试", "queued", stringPtr(retryMessage), map[string]any{
			"jobType":        claimed.JobType,
			"modelName":      claimed.ModelName,
			"source":         claimed.Source,
			"autoRetryCount": retryCount,
			"maxRetryCount":  mediaFailureAutoRetryLimit,
			"failureMessage": message,
			"retryAfter":     mediaFailureAutoRetryDelay.String(),
		})
		w.app.Logger.Warn("ai worker auto-retrying failed media ai job", "job_id", claimed.ID, "job_type", claimed.JobType, "retry_count", retryCount, "max_retry_count", mediaFailureAutoRetryLimit, "retry_after", mediaFailureAutoRetryDelay.String(), "error", execErr)
		return
	}

	w.returnUsageCreditsForFailure(ctx, claimed, message)
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

// 处理 AI 作业执行中的执行对话流程，依赖作业状态、租约和持久化结果推进链路。
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
	model, provider, providerName, baseURL, apiKey, err := w.resolveModelRuntime(ctx, job.ModelName)
	if err != nil {
		return err
	}
	req.BaseURL = baseURL
	req.APIKey = apiKey
	if model != nil {
		req.ChatProtocol = model.ChatProtocol
	}
	result, err := provider.GenerateChat(ctx, req)
	if err != nil {
		return err
	}

	artifactPayload := mustJSON(map[string]any{
		"provider":          providerName,
		"role":              result.Role,
		"finishReason":      result.FinishReason,
		"completionState":   result.CompletionState,
		"warningCodes":      result.WarningCodes,
		"warningMessage":    result.WarningMessage,
		"protocolFamily":    result.ProtocolFamily,
		"streamDiagnostics": result.StreamDiagnostics,
		"usage":             result.Usage,
	})
	fileName := "response.txt"
	mimeType := "text/plain; charset=utf-8"
	sizeBytes := int64(len([]byte(result.Text)))
	artifacts, err := w.app.Store.UpsertAIJobArtifacts(ctx, []store.UpsertAIJobArtifactInput{{
		JobID:        job.ID,
		ArtifactKey:  "response.txt",
		ArtifactType: "text",
		Source:       providerName,
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
		"provider":          providerName,
		"kind":              "chat",
		"model":             job.ModelName,
		"text":              result.Text,
		"role":              result.Role,
		"finishReason":      result.FinishReason,
		"completionState":   result.CompletionState,
		"warningCodes":      result.WarningCodes,
		"warningMessage":    result.WarningMessage,
		"protocolFamily":    result.ProtocolFamily,
		"streamDiagnostics": result.StreamDiagnostics,
		"usage":             result.Usage,
		"billing":           billingToPayload(billing),
		"artifacts":         summarizeArtifacts(artifacts),
		"storyboard":        storyboardPayload,
		"completedAt":       time.Now().UTC().Format(time.RFC3339),
	})
	message := buildCompletionMessage(buildAIChatCompletionBaseMessage(result), billing)
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

// 处理 AI 作业执行中的执行图片流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) executeImage(ctx context.Context, job *domain.AIJob, leaseToken string) error {
	req, err := BuildImageRequest(job)
	if err != nil {
		return err
	}
	storyboardPayload, optimizedPrompt, err := w.prepareOptionalMediaStoryboardPrompt(ctx, job, leaseToken, req.Prompt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(optimizedPrompt) != "" {
		req.Prompt = optimizedPrompt
	}
	model, provider, providerName, baseURL, apiKey, err := w.resolveModelRuntime(ctx, job.ModelName)
	if err != nil {
		return err
	}
	req.BaseURL = baseURL
	req.APIKey = apiKey
	req.ExplicitBaseURL = modelHasExplicitBaseURL(model)
	result, err := provider.GenerateImage(ctx, req)
	if err != nil {
		return err
	}

	inputs := make([]store.UpsertAIJobArtifactInput, 0, len(result.Images)+1)
	for index, image := range result.Images {
		artifactKey := strings.TrimSpace(image.ArtifactKey)
		if artifactKey == "" {
			artifactKey = fmt.Sprintf("image-%d%s", index+1, extensionForMIME(image.MIMEType, ".png"))
		}
		input, err := w.saveBinaryArtifact(ctx, job, "image", artifactKey, providerName, image)
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
			Source:       providerName,
			Title:        stringPtr("图片生成说明"),
			FileName:     &fileName,
			MimeType:     &mimeType,
			SizeBytes:    &sizeBytes,
			TextContent:  stringPtr(result.Text),
			Payload: mustJSON(map[string]any{
				"provider": providerName,
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
		"provider":    providerName,
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

// 处理 AI 作业执行中的执行视频流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) executeVideo(ctx context.Context, job *domain.AIJob, leaseToken string, leaseExpiresAt time.Time) error {
	if isDigitalHumanWorkflowJob(job) {
		return w.executeDigitalHumanVideo(ctx, job, leaseToken)
	}
	if snapshot := parseWorkflowPricingSnapshot(job); snapshot != nil {
		return w.executeWorkflowVideo(ctx, job, leaseToken, leaseExpiresAt, snapshot)
	}

	req, err := BuildVideoRequest(job)
	if err != nil {
		return err
	}
	model, provider, providerName, baseURL, apiKey, err := w.resolveModelRuntime(ctx, job.ModelName)
	if err != nil {
		return err
	}
	req.BaseURL = baseURL
	req.APIKey = apiKey
	req.ExplicitBaseURL = modelHasExplicitBaseURL(model)
	req.Vendor = providerName

	state := parseVideoExecutionState(job.OutputPayload)
	if strings.TrimSpace(state.BaseURL) == "" {
		state.BaseURL = baseURL
	}
	if !state.ExplicitBaseURL {
		state.ExplicitBaseURL = req.ExplicitBaseURL
	}
	if strings.TrimSpace(state.Provider) == "" {
		state.Provider = providerName
	}
	storyboardPayload, err := w.prepareVideoGenerationInputs(ctx, job, leaseToken, &req, &state)
	if err != nil {
		return err
	}
	if strings.TrimSpace(state.FinalPrompt) != "" {
		req.Prompt = state.FinalPrompt
	} else {
		state.FinalPrompt = strings.TrimSpace(req.Prompt)
	}
	req.Model = normalizeVideoModel(strings.TrimSpace(job.ModelName), req.AspectRatio, len(req.ReferenceImages) > 0)
	if strings.TrimSpace(state.RemoteVideoID) == "" {
		submission, err := provider.SubmitVideo(ctx, req)
		if err != nil {
			if shouldRequeueVideoSubmissionError(err) {
				return buildTemporaryVideoRequeueError(job, state, err)
			}
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
		job.OutputPayload = runningPayload
		if _, err := w.syncRunningState(ctx, job, leaseToken, message, runningPayload); err != nil {
			return err
		}
	}

	deadline := state.SubmittedAt.Add(w.videoTimeout)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var renewErr error
		leaseExpiresAt, renewErr = w.renewLease(ctx, job.ID, leaseToken, leaseExpiresAt)
		if renewErr != nil {
			return renewErr
		}

		status, err := provider.GetVideo(ctx, state.RemoteVideoID, req.Model, state.BaseURL, apiKey, state.ExplicitBaseURL)
		if err != nil {
			if isTransientVideoProviderExecutionError(err) {
				return buildTemporaryVideoRequeueError(job, state, err)
			}
			return err
		}
		state.RemoteStatus = strings.TrimSpace(status.Status)
		state.ProgressPercent = status.ProgressPercent
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
			artifact, err := w.downloadAndFinalizeVideoArtifact(ctx, provider, job, req, &state, apiKey)
			if err != nil {
				if isTransientVideoProviderExecutionError(err) {
					return buildTemporaryVideoRequeueError(job, state, err)
				}
				return err
			}
			artifactKey := strings.TrimSpace(artifact.ArtifactKey)
			if artifactKey == "" {
				artifactKey = safeArtifactKey(artifact.FileName, "video.mp4")
			}
			input, err := w.saveBinaryArtifact(ctx, job, "video", artifactKey, providerName, *artifact)
			if err != nil {
				return err
			}
			artifacts, err := w.app.Store.UpsertAIJobArtifacts(ctx, []store.UpsertAIJobArtifactInput{input})
			if err != nil {
				return err
			}
			billing := w.applyUsageBilling(ctx, job, buildVideoBillingInput(job))
			outputPayload := buildVideoOutputPayload(job, state, artifacts)
			job.OutputPayload = outputPayload
			if len(storyboardPayload) > 0 {
				outputPayload = mergeMetadataIntoPayload(outputPayload, "storyboard", storyboardPayload)
				job.OutputPayload = outputPayload
			}
			outputPayload = mergeBillingIntoPayload(outputPayload, billing)
			job.OutputPayload = outputPayload
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
				message := fmt.Sprintf("AI 视频生成超过 %s，继续后台回查云端结果", w.videoTimeout)
				runningPayload := buildVideoOutputPayload(job, state, nil)
				job.OutputPayload = runningPayload
				return &requeueExecutionError{
					Message:       message,
					OutputPayload: runningPayload,
				}
			}
			message := "AI 视频生成中"
			if strings.TrimSpace(state.Message) != "" {
				message = "AI 视频生成中: " + strings.TrimSpace(state.Message)
			}
			runningPayload := buildVideoOutputPayload(job, state, nil)
			job.OutputPayload = runningPayload
			if _, err := w.syncRunningState(ctx, job, leaseToken, message, runningPayload); err != nil {
				return err
			}
			pollInterval := mediaExecutionPollInterval(state.SubmittedAt, time.Now().UTC())
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(pollInterval):
			}
		}
	}
}

// 为 AI 作业执行提供下载Finalize视频产物辅助能力，统一处理输入、状态和产物细节。
func (w *Worker) downloadAndFinalizeVideoArtifact(ctx context.Context, provider Provider, job *domain.AIJob, req VideoRequest, state *videoExecutionState, apiKey string) (*BinaryArtifact, error) {
	var lastErr error

	for attempt := 1; attempt <= videoArtifactFinalizeMaxAttempts; attempt++ {
		if attempt > 1 {
			latestStatus, statusErr := provider.GetVideo(ctx, state.RemoteVideoID, req.Model, state.BaseURL, apiKey, state.ExplicitBaseURL)
			if statusErr != nil {
				w.app.Logger.Warn(
					"ai worker failed to refresh completed video status before retry",
					"job_id", job.ID,
					"attempt", attempt,
					"max_attempts", videoArtifactFinalizeMaxAttempts,
					"error", statusErr,
				)
			} else {
				state.RemoteStatus = strings.TrimSpace(latestStatus.Status)
				state.ProgressPercent = latestStatus.ProgressPercent
				state.ContentURL = strings.TrimSpace(latestStatus.ContentURL)
				state.UpdatedAt = firstNonNilTime(latestStatus.UpdatedAt, time.Now().UTC())
				if latestStatus.Message != "" {
					state.Message = latestStatus.Message
				}
				if latestStatus.FailureCode != "" {
					state.FailureCode = latestStatus.FailureCode
				}
			}
		}

		artifact, err := provider.DownloadVideo(ctx, state.RemoteVideoID, req.Model, state.BaseURL, apiKey, state.ContentURL, state.ExplicitBaseURL)
		if err == nil {
			if strings.TrimSpace(artifact.FileName) == "" {
				artifact.FileName = "video.mp4"
			}
			if strings.TrimSpace(artifact.MIMEType) == "" {
				artifact.MIMEType = "video/mp4"
			}
			if !w.app.Config.AIVideoStandardizeEnabled {
				return artifact, nil
			}

			originalFileName := artifact.FileName
			originalSizeBytes := len(artifact.Data)
			standardizedArtifact, standardizeErr := standardizeVideoArtifact(ctx, *artifact, w.app.Config.AIVideoFFmpegPath)
			if standardizeErr == nil {
				artifact = &standardizedArtifact
				w.app.Logger.Info(
					"ai worker standardized video artifact",
					"job_id", job.ID,
					"model_name", job.ModelName,
					"source_file_name", originalFileName,
					"output_file_name", artifact.FileName,
					"source_size_bytes", originalSizeBytes,
					"output_size_bytes", len(artifact.Data),
					"video_codec", "h264",
					"fps", standardizedVideoFPS,
					"scale_ratio", standardizedVideoScaleRatio,
					"attempt", attempt,
				)
				return artifact, nil
			}
			err = standardizeErr
		}

		lastErr = err
		if attempt == videoArtifactFinalizeMaxAttempts || ctx.Err() != nil {
			break
		}

		delay := time.Duration(attempt) * videoArtifactFinalizeRetryDelay
		w.app.Logger.Warn(
			"ai worker retrying completed video download/standardize",
			"job_id", job.ID,
			"model_name", job.ModelName,
			"remote_video_id", state.RemoteVideoID,
			"content_url", state.ContentURL,
			"attempt", attempt,
			"max_attempts", videoArtifactFinalizeMaxAttempts,
			"retry_after", delay.String(),
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("video artifact finalization failed")
	}
	return nil, lastErr
}

// 处理 AI 作业执行中的准备分镜提示词流程，依赖作业状态、租约和持久化结果推进链路。
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
	publishPayload, _ := payload["publishPayload"].(map[string]any)
	seedContentText := strings.TrimSpace(stringValueFromMap(publishPayload, "contentText", "contentTemplate"))
	publishIntroEnabled := true
	if raw, exists := payload["publishIntroEnabled"]; exists {
		publishIntroEnabled = boolValue(raw)
	}
	publishPromptTemplate := strings.TrimSpace(stringValueFromMap(payload, "publishPromptTemplate"))
	if publishIntroEnabled && publishPromptTemplate == "" {
		publishPromptTemplate = defaultPublishIntroPrompt
	}

	systemPrompt := strings.TrimSpace(stringValueFromMap(config, "prompt"))
	if systemPrompt == "" {
		systemPrompt = defaultStoryboardSystemPrompt
	}

	model, provider, _, baseURL, apiKey, err := w.resolveModelRuntime(ctx, modelName)
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

	result, err := provider.GenerateChat(ctx, ChatRequest{
		Model:        modelName,
		BaseURL:      baseURL,
		APIKey:       apiKey,
		ChatProtocol: model.ChatProtocol,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return nil, originalPrompt, err
	}

	optimizedPrompt, optimizedContentText, responseMode := parseStoryboardOptimizationResponse(result.Text, originalPrompt, seedContentText)
	if !publishIntroEnabled {
		optimizedContentText = seedContentText
		responseMode = strings.TrimSpace(responseMode + "_intro_disabled")
	}

	return map[string]any{
		"modelName":             modelName,
		"promptTemplate":        systemPrompt,
		"publishPromptTemplate": publishPromptTemplate,
		"publishIntroEnabled":   publishIntroEnabled,
		"optimizedPrompt":       optimizedPrompt,
		"optimizedContentText":  optimizedContentText,
		"responseMode":          responseMode,
		"referenceCount": map[string]int{
			"images": len(referenceImages),
			"texts":  len(referenceTexts),
		},
	}, optimizedPrompt, nil
}

// 处理 AI 作业执行中的准备可选媒体分镜提示词流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) prepareOptionalMediaStoryboardPrompt(ctx context.Context, job *domain.AIJob, leaseToken string, originalPrompt string) (map[string]any, string, error) {
	storyboardPayload, optimizedPrompt, err := w.prepareStoryboardPrompt(ctx, job, leaseToken, originalPrompt)
	if err == nil {
		return storyboardPayload, optimizedPrompt, nil
	}
	if ctx.Err() != nil {
		return nil, originalPrompt, ctx.Err()
	}

	fallbackPayload := buildStoryboardFallbackPayload(job, originalPrompt, err)
	w.app.Logger.Warn(
		"ai worker failed to optimize storyboard prompt, falling back to original prompt",
		"job_id", job.ID,
		"job_type", job.JobType,
		"model_name", job.ModelName,
		"error", err,
	)
	return fallbackPayload, originalPrompt, nil
}

// 构建分镜回退载荷，为AI作业执行生成后续步骤所需的派生参数或载荷。
func buildStoryboardFallbackPayload(job *domain.AIJob, originalPrompt string, err error) map[string]any {
	payload := decodePayloadMap(job.InputPayload)
	config, _ := payload["storyboardConfig"].(map[string]any)
	referenceTexts := normalizeStoryboardTexts(payload["referenceTexts"])
	referenceImages := normalizeStoryboardImages(payload["referenceImages"])
	publishPayload, _ := payload["publishPayload"].(map[string]any)
	seedContentText := strings.TrimSpace(stringValueFromMap(publishPayload, "contentText", "contentTemplate"))

	fallbackPayload := map[string]any{
		"status":               "fallback",
		"optimizedPrompt":      originalPrompt,
		"optimizedContentText": seedContentText,
		"error":                strings.TrimSpace(errorString(err)),
		"referenceCount": map[string]int{
			"images": len(referenceImages),
			"texts":  len(referenceTexts),
		},
	}
	if modelName := strings.TrimSpace(stringValueFromMap(config, "modelName")); modelName != "" {
		fallbackPayload["modelName"] = modelName
	}
	return fallbackPayload
}

// 续租当前作业占用，防止长时间执行期间被其他 worker 误判为超时。
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

// 启动作业租约心跳续期协程，保证长耗时执行阶段持续持有租约。
func (w *Worker) startLeaseHeartbeat(parent context.Context, jobID string, leaseToken string) func() {
	heartbeatCtx, cancel := context.WithCancel(parent)
	interval := store.AIJobLeaseTTL() / 3
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				nextExpiry := time.Now().UTC().Add(store.AIJobLeaseTTL())
				renewed, err := w.app.Store.RenewCloudAIJobLease(heartbeatCtx, jobID, leaseToken, nextExpiry)
				if err != nil {
					w.app.Logger.Warn("ai worker heartbeat failed to renew ai job lease", "job_id", jobID, "error", err)
					continue
				}
				if renewed == nil {
					return
				}
			}
		}
	}()

	return cancel
}

// 同步作业运行中的阶段和元数据，便于前端和审计看到最新执行进度。
func (w *Worker) syncRunningState(ctx context.Context, job *domain.AIJob, leaseToken string, message string, outputPayload []byte) (*domain.AIJob, error) {
	return w.app.Store.SyncCloudAIJobExecution(ctx, job.ID, leaseToken, store.UpdateAIJobInput{
		Message:       stringPtr(message),
		OutputPayload: outputPayload,
		OutputTouched: len(outputPayload) > 0,
	})
}

// 写回作业成功结果并释放执行占用，结束当前 AI 作业生命周期。
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

// 写回作业失败结果并释放执行占用，为后续排障或人工处理保留上下文。
func (w *Worker) failJob(ctx context.Context, jobID string, leaseToken string, message string, outputPayload []byte) (*domain.AIJob, error) {
	status := "failed"
	return w.app.Store.SyncCloudAIJobExecution(ctx, jobID, leaseToken, store.UpdateAIJobInput{
		Status:        &status,
		Message:       stringPtr(message),
		OutputPayload: outputPayload,
		OutputTouched: len(outputPayload) > 0,
	})
}

// 将当前作业重新放回可执行队列，供后续轮询再次接手处理。
func (w *Worker) requeueJob(ctx context.Context, jobID string, leaseToken string, message string, outputPayload []byte) (*domain.AIJob, error) {
	status := "queued"
	return w.app.Store.SyncCloudAIJobExecution(ctx, jobID, leaseToken, store.UpdateAIJobInput{
		Status:        &status,
		Message:       stringPtr(message),
		OutputPayload: outputPayload,
		OutputTouched: len(outputPayload) > 0,
	})
}

// 按退避策略重新入队当前作业，降低外部服务抖动时的重试压力。
func (w *Worker) requeueJobWithBackoff(ctx context.Context, jobID string, leaseToken string, message string, outputPayload []byte, delay time.Duration) (*domain.AIJob, error) {
	if delay <= 0 {
		return w.requeueJob(ctx, jobID, leaseToken, message, outputPayload)
	}
	retryAt := time.Now().UTC().Add(delay)
	return w.app.Store.RequeueCloudAIJobWithBackoff(ctx, jobID, leaseToken, retryAt, stringPtr(message), outputPayload)
}

// 处理 AI 作业执行中的暂停作业充值流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) pauseJobForRecharge(ctx context.Context, jobID string, leaseToken string, message string, outputPayload []byte, delay time.Duration) (*domain.AIJob, error) {
	retryAt := time.Now().UTC()
	if delay > 0 {
		retryAt = retryAt.Add(delay)
	}
	return w.app.Store.MarkCloudAIJobWaitingRecharge(ctx, jobID, leaseToken, retryAt, stringPtr(message), outputPayload)
}

// 构建AIExecution失败消息，为AI作业执行生成后续步骤所需的派生参数或载荷。
func buildAIExecutionFailureMessage(jobType string, err error) string {
	if err == nil {
		return "AI 云端执行失败"
	}
	raw := strings.TrimSpace(err.Error())
	if raw == "" {
		return "AI 云端执行失败"
	}

	switch {
	case strings.EqualFold(strings.TrimSpace(jobType), "video") && isLMRootGeminiPromptRewriteErrorMessage(raw):
		return "AI 云端执行失败: 上游视频模型临时返回了异常结果，请稍后重试"
	case strings.EqualFold(strings.TrimSpace(jobType), "video") && isTransientVideoProviderExecutionError(err):
		return "AI 云端执行失败: 上游视频服务暂时不可用，请稍后重试"
	default:
		return "AI 云端执行失败: " + truncateFailureMessage(raw, 280)
	}
}

// 构建临时视频重试错误，为AI作业执行生成后续步骤所需的派生参数或载荷。
func buildTemporaryVideoRequeueError(job *domain.AIJob, state videoExecutionState, err error) error {
	message := "AI 视频服务暂时波动，任务已自动排队重试"
	if isLMRootGeminiPromptRewriteErrorMessage(errorString(err)) {
		message = "AI 视频服务临时返回了异常结果，任务已自动排队重试"
	}
	return &requeueExecutionError{
		Message:       message,
		OutputPayload: buildVideoOutputPayload(job, state, nil),
	}
}

// 判断是否应当Auto重试媒体失败，供当前链路选择后续处理策略。
func shouldAutoRetryMediaFailure(job *domain.AIJob, err error) bool {
	if job == nil {
		return false
	}
	jobType := strings.TrimSpace(strings.ToLower(job.JobType))
	if jobType != "image" && jobType != "video" {
		return false
	}
	if mediaAutoRetryCountFromPayload(job.OutputPayload) >= mediaFailureAutoRetryLimit {
		return false
	}
	switch jobType {
	case "video":
		return isTransientVideoProviderExecutionError(err)
	case "image":
		return isTransientImageProviderExecutionError(err)
	default:
		return false
	}
}

// 处理媒体Auto重试数量载荷相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func mediaAutoRetryCountFromPayload(raw []byte) int {
	payload := decodePayloadMap(raw)
	executionPayload, _ := payload["execution"].(map[string]any)
	return intValue(executionPayload["autoRetryCount"])
}

// 构建媒体自动重试消息，为AI作业执行生成后续步骤所需的派生参数或载荷。
func buildMediaAutoRetryMessage(jobType string, retryCount int) string {
	label := "内容"
	switch strings.TrimSpace(strings.ToLower(jobType)) {
	case "image":
		label = "图片"
	case "video":
		label = "视频"
	}
	return fmt.Sprintf("AI %s生成失败，系统将自动重试第 %d/%d 次", label, retryCount, mediaFailureAutoRetryLimit)
}

// 构建媒体自动重试载荷，为AI作业执行生成后续步骤所需的派生参数或载荷。
func buildMediaAutoRetryPayload(job *domain.AIJob, failureMessage string, retryCount int) []byte {
	payload := decodePayloadMap(job.OutputPayload)
	delete(payload, "video")
	delete(payload, "contentUrl")
	delete(payload, "progressPercent")
	delete(payload, "failureCode")
	delete(payload, "artifacts")
	if providerName := strings.TrimSpace(stringValue(payload["provider"])); providerName != "" {
		payload["provider"] = providerName
	}
	payload["kind"] = strings.TrimSpace(strings.ToLower(job.JobType))
	payload["model"] = job.ModelName
	payload["execution"] = map[string]any{
		"autoRetryCount":     retryCount,
		"maxAutoRetry":       mediaFailureAutoRetryLimit,
		"lastFailureAt":      time.Now().UTC().Format(time.RFC3339),
		"lastFailureMessage": strings.TrimSpace(failureMessage),
	}
	return mustJSON(payload)
}

// 判断是否应当恢复远端视频轮询，供当前链路选择后续处理策略。
func shouldResumeRemoteVideoPolling(job *domain.AIJob, err error) bool {
	if job == nil {
		return false
	}
	if strings.TrimSpace(strings.ToLower(job.JobType)) != "video" {
		return false
	}
	if !isTransientVideoProviderExecutionError(err) {
		return false
	}
	state := parseVideoExecutionState(job.OutputPayload)
	return strings.TrimSpace(state.RemoteVideoID) != ""
}

// 判断是否应当重新入队视频提交错误，供当前链路选择后续处理策略。
func shouldRequeueVideoSubmissionError(err error) bool {
	if err == nil {
		return false
	}
	message := errorString(err)
	if isLMRootGeminiPromptRewriteErrorMessage(message) {
		return true
	}
	return strings.Contains(strings.ToLower(message), "provider request failed with status 429")
}

// 判断是否属于瞬时视频供应方Execution错误，供当前链路选择后续处理策略。
func isTransientVideoProviderExecutionError(err error) bool {
	if err == nil {
		return false
	}
	if isRetryableProviderError(err) {
		return true
	}
	message := strings.ToLower(errorString(err))
	if message == "" {
		return false
	}
	if isLMRootGeminiPromptRewriteErrorMessage(message) {
		return true
	}
	transientMarkers := []string{
		"provider request failed with status 429",
		"provider request failed with status 500",
		"provider request failed with status 502",
		"provider request failed with status 503",
		"provider request failed with status 504",
		"timeout",
		"temporary",
	}
	for _, marker := range transientMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// 判断是否属于瞬时图片供应方Execution错误，供当前链路选择后续处理策略。
func isTransientImageProviderExecutionError(err error) bool {
	if err == nil {
		return false
	}
	if isRetryableProviderError(err) {
		return true
	}
	message := strings.ToLower(errorString(err))
	if message == "" {
		return false
	}
	if containsPermanentMediaFailureMarker(message) {
		return false
	}
	transientMarkers := []string{
		"provider request failed with status 429",
		"provider request failed with status 500",
		"provider request failed with status 502",
		"provider request failed with status 503",
		"provider request failed with status 504",
		"timeout",
		"temporary",
	}
	for _, marker := range transientMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// 处理contains永久媒体失败Marker相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func containsPermanentMediaFailureMarker(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	permanentMarkers := []string{
		"provider request failed with status 400",
		"provider request failed with status 401",
		"provider request failed with status 403",
		"provider request failed with status 404",
		"provider request failed with status 422",
		"违反平台政策",
		"内容政策",
		"policy violation",
		"content policy",
		"moderation",
		"unsafe",
		"forbidden",
		"not allowed",
		"rejected",
	}
	for _, marker := range permanentMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

// 处理truncate失败消息相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func truncateFailureMessage(message string, limit int) string {
	message = strings.TrimSpace(message)
	if limit <= 0 || len(message) == 0 {
		return ""
	}
	runes := []rune(message)
	if len(runes) <= limit {
		return message
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}

// 将任意数值型输入归一为 int，供运行时配置解析逻辑复用。
func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0
		}
		var parsed int
		_, _ = fmt.Sscanf(trimmed, "%d", &parsed)
		return parsed
	default:
		return 0
	}
}

// 在错误存在时返回可读消息，统一日志和响应中的错误文案处理。
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

// 处理 AI 作业执行中的保存二进制产物流程，依赖作业状态、租约和持久化结果推进链路。
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
			"provider": source,
			"metadata": artifact.Metadata,
		}),
	}, nil
}

// 处理 AI 作业执行中的记录审计事件流程，依赖作业状态、租约和持久化结果推进链路。
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

// 处理 AI 作业执行中的记录计费阻塞审计记录流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) recordBillingBlockedAudit(ctx context.Context, job *domain.AIJob, message string, result *store.ApplyUsageBillingResult) {
	if job == nil {
		return
	}
	hasRecent, err := w.app.Store.HasRecentAuditEvent(ctx, job.OwnerUserID, "ai_billing_precheck_waiting_recharge", time.Now().UTC().Add(-6*time.Hour))
	if err == nil && hasRecent {
		return
	}
	w.recordAuditEvent(ctx, job, "ai_billing_precheck_waiting_recharge", "AI 启动前余额不足，等待充值", "waiting_recharge", stringPtr(message), map[string]any{
		"jobType":   job.JobType,
		"modelName": job.ModelName,
		"source":    job.Source,
		"billing":   result,
	})
}

// 处理 AI 作业执行中的应用用量计费流程，依赖作业状态、租约和持久化结果推进链路。
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
		if result.AlreadyBilled {
			return result
		}
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

// 处理 AI 作业执行中的确保Execution计费流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) ensureExecutionBilling(ctx context.Context, job *domain.AIJob) error {
	if isDigitalHumanAIJob(job) || isDigitalHumanWorkflowJob(job) {
		return nil
	}
	if plan, planErr := BuildWorkflowBillingPlan(ctx, w.app, job); planErr != nil {
		return planErr
	} else if plan != nil {
		summary, summaryErr := w.app.Store.GetBillingSummaryByUser(ctx, strings.TrimSpace(job.OwnerUserID))
		if summaryErr != nil {
			return summaryErr
		}
		preview := workflowBillingPlanToUsageResult(plan, summary.CreditBalance)
		if preview != nil && preview.BillStatus == "failed" {
			return &executionBillingBlockedError{Result: preview}
		}

		session, items, err := EnsureWorkflowBillingSession(ctx, w.app, job)
		if err != nil {
			return err
		}
		message := "AI 启动前任务级预扣费完成"
		if session != nil {
			message = fmt.Sprintf("AI 启动前任务级预扣费完成，预扣 %d 积分", session.PlannedCredits)
		}
		w.recordAuditEvent(ctx, job, "ai_workflow_billing_precharged", "AI 启动前任务级预扣费完成", "success", stringPtr(message), map[string]any{
			"jobType":   job.JobType,
			"modelName": job.ModelName,
			"source":    job.Source,
			"plannedCredits": func() int64 {
				if session == nil {
					return 0
				}
				return session.PlannedCredits
			}(),
			"session": session,
			"items":   items,
		})
		return nil
	}

	input := BuildEstimatedUsageBillingInput(job)
	if len(input.Metrics) == 0 {
		return nil
	}

	result, err := w.app.Store.ApplyUsageBilling(ctx, input)
	if err != nil {
		return err
	}
	if result.BillStatus == "failed" {
		return &executionBillingBlockedError{Result: result}
	}
	if result.BillStatus != "billed" {
		return nil
	}
	if result.AlreadyBilled {
		return nil
	}

	message := fmt.Sprintf("AI 启动前预扣费完成，扣减 %d 积分", result.TotalCredits)
	w.recordAuditEvent(ctx, job, "ai_billing_precharged", "AI 启动前预扣费完成", "success", stringPtr(message), map[string]any{
		"jobType":      job.JobType,
		"modelName":    job.ModelName,
		"source":       job.Source,
		"totalCredits": result.TotalCredits,
		"details":      result.Details,
	})
	return nil
}

// 处理 AI 作业执行中的返还用量额度Failure流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) returnUsageCreditsForFailure(ctx context.Context, job *domain.AIJob, failureMessage string) {
	if isDigitalHumanAIJob(job) || isDigitalHumanWorkflowJob(job) {
		return
	}
	if plan, err := BuildWorkflowBillingPlan(ctx, w.app, job); err == nil && plan != nil {
		if refundErr := w.app.Store.RefundAIBillingSessionBySource(ctx, "ai_job", job.ID, failureMessage); refundErr != nil {
			w.app.Logger.Error("ai worker failed to refund workflow billing session", "job_id", job.ID, "error", refundErr)
		}
		return
	}

	input := BuildEstimatedUsageBillingInput(job)
	if len(input.Metrics) == 0 {
		return
	}
	if err := w.app.Store.ReturnUsageCreditsForFailedSource(ctx, "ai_job", job.ID, failureMessage); err != nil {
		w.app.Logger.Error("ai worker failed to return precharged usage credits", "job_id", job.ID, "error", err)
	}
}

// 处理resultBill消息相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func resultBillMessage(result *store.ApplyUsageBillingResult) *string {
	if result == nil {
		return nil
	}
	message := strings.TrimSpace(result.BillMessage)
	if message == "" {
		return nil
	}
	return &message
}

type videoExecutionState struct {
	Provider                  string
	BaseURL                   string
	ExplicitBaseURL           bool
	RemoteVideoID             string
	RemoteStatus              string
	ProgressPercent           *int
	ContentURL                string
	Message                   string
	FailureCode               string
	SubmittedAt               time.Time
	UpdatedAt                 time.Time
	ReferenceFrames           []map[string]any
	FrameRedesign             map[string]any
	FinalPrompt               string
	Storyboard                map[string]any
	DurationSeconds           int
	SegmentSeconds            int
	PlannedSegments           int
	ActiveSegment             int
	ActualDuration            int
	PartialSuccess            bool
	CompletedSegments         []videoCompletedSegment
	SuccessfulBillingItemKeys []string
}

type videoCompletedSegment struct {
	SegmentIndex    int            `json:"segmentIndex"`
	ArtifactKey     string         `json:"artifactKey"`
	FileName        string         `json:"fileName,omitempty"`
	MimeType        string         `json:"mimeType,omitempty"`
	StorageKey      string         `json:"storageKey,omitempty"`
	PublicURL       string         `json:"publicUrl,omitempty"`
	SizeBytes       int64          `json:"sizeBytes,omitempty"`
	DurationSeconds int            `json:"durationSeconds,omitempty"`
	ReferenceFrame  map[string]any `json:"referenceFrame,omitempty"`
}

// 构建视频输出载荷，为AI作业执行生成后续步骤所需的派生参数或载荷。
func buildVideoOutputPayload(job *domain.AIJob, state videoExecutionState, artifacts []domain.AIJobArtifact) []byte {
	payload := decodePayloadMap(job.OutputPayload)
	providerName := strings.TrimSpace(state.Provider)
	if providerName == "" {
		providerName = strings.TrimSpace(stringValue(payload["provider"]))
	}
	videoPayload := map[string]any{
		"provider":        providerName,
		"baseUrl":         state.BaseURL,
		"explicitBaseUrl": state.ExplicitBaseURL,
		"id":              state.RemoteVideoID,
		"status":          state.RemoteStatus,
		"contentUrl":      state.ContentURL,
		"message":         state.Message,
		"failureCode":     state.FailureCode,
		"submittedAt":     state.SubmittedAt.Format(time.RFC3339),
		"updatedAt":       state.UpdatedAt.Format(time.RFC3339),
	}
	if providerName != "" {
		payload["provider"] = providerName
	}
	payload["kind"] = "video"
	payload["model"] = job.ModelName
	payload["baseUrl"] = state.BaseURL
	payload["explicitBaseUrl"] = state.ExplicitBaseURL
	payload["video"] = videoPayload
	payload["artifacts"] = summarizeArtifacts(artifacts)
	if len(state.ReferenceFrames) > 0 {
		videoPayload["referenceFrames"] = state.ReferenceFrames
		payload["videoReferenceFrames"] = state.ReferenceFrames
	}
	if len(state.FrameRedesign) > 0 {
		payload["videoFrameRedesign"] = state.FrameRedesign
	}
	if strings.TrimSpace(state.FinalPrompt) != "" {
		videoPayload["generationPrompt"] = state.FinalPrompt
		payload["generationPrompt"] = state.FinalPrompt
	}
	if len(state.Storyboard) > 0 {
		payload["storyboard"] = state.Storyboard
	}
	if state.ProgressPercent != nil {
		videoPayload["progressPercent"] = *state.ProgressPercent
		payload["progressPercent"] = *state.ProgressPercent
	}
	if state.DurationSeconds > 0 {
		videoPayload["durationSeconds"] = state.DurationSeconds
		payload["durationSeconds"] = state.DurationSeconds
	}
	if state.SegmentSeconds > 0 {
		videoPayload["segmentSeconds"] = state.SegmentSeconds
		payload["segmentSeconds"] = state.SegmentSeconds
	}
	if state.PlannedSegments > 0 {
		videoPayload["plannedSegments"] = state.PlannedSegments
		payload["plannedSegments"] = state.PlannedSegments
	}
	if state.ActiveSegment > 0 {
		videoPayload["activeSegment"] = state.ActiveSegment
		payload["activeSegment"] = state.ActiveSegment
	}
	if state.ActualDuration > 0 {
		videoPayload["actualDurationSeconds"] = state.ActualDuration
		payload["actualDurationSeconds"] = state.ActualDuration
	}
	if state.PartialSuccess {
		videoPayload["partialSuccess"] = true
		payload["partialSuccess"] = true
	}
	if len(state.CompletedSegments) > 0 {
		videoPayload["completedSegments"] = state.CompletedSegments
		payload["completedSegments"] = state.CompletedSegments
	}
	if len(state.SuccessfulBillingItemKeys) > 0 {
		videoPayload["successfulBillingItemKeys"] = state.SuccessfulBillingItemKeys
		payload["successfulBillingItemKeys"] = state.SuccessfulBillingItemKeys
	}
	return mustJSON(payload)
}

// 合并计费Into载荷，统一多来源数据后返回稳定结果。
func mergeBillingIntoPayload(raw []byte, billing *store.ApplyUsageBillingResult) []byte {
	payload := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &payload)
	}
	payload["billing"] = billingToPayload(billing)
	return mustJSON(payload)
}

// 合并MetadataInto载荷，统一多来源数据后返回稳定结果。
func mergeMetadataIntoPayload(raw []byte, key string, value any) []byte {
	payload := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &payload)
	}
	payload[key] = value
	return mustJSON(payload)
}

// 解析视频Execution状态，为AI作业执行提供结构化输入。
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
	state.Provider = strings.TrimSpace(firstNonEmptyString(
		stringValue(payload["provider"]),
		stringValue(videoPayload["provider"]),
	))
	state.BaseURL = strings.TrimSpace(firstNonEmptyString(
		stringValue(payload["baseUrl"]),
		stringValue(videoPayload["baseUrl"]),
	))
	state.ExplicitBaseURL = boolValue(firstNonNilValue(payload["explicitBaseUrl"], videoPayload["explicitBaseUrl"]))
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
	state.ProgressPercent = firstNonNilInt(
		extractProgressPercentValue(payload, 0),
		extractProgressPercentValue(videoPayload, 0),
	)
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
	state.ReferenceFrames = firstNonEmptyObjectSlice(
		normalizeObjectSlice(payload["videoReferenceFrames"]),
		normalizeObjectSlice(videoPayload["referenceFrames"]),
	)
	state.FrameRedesign = normalizeObject(payload["videoFrameRedesign"])
	state.FinalPrompt = strings.TrimSpace(firstNonEmptyString(
		stringValue(payload["generationPrompt"]),
		stringValue(videoPayload["generationPrompt"]),
	))
	state.Storyboard = normalizeObject(payload["storyboard"])
	state.DurationSeconds = intValue(firstNonNilValue(payload["durationSeconds"], videoPayload["durationSeconds"]))
	state.SegmentSeconds = intValue(firstNonNilValue(payload["segmentSeconds"], videoPayload["segmentSeconds"]))
	state.PlannedSegments = intValue(firstNonNilValue(payload["plannedSegments"], videoPayload["plannedSegments"]))
	state.ActiveSegment = intValue(firstNonNilValue(payload["activeSegment"], videoPayload["activeSegment"]))
	state.ActualDuration = intValue(firstNonNilValue(payload["actualDurationSeconds"], videoPayload["actualDurationSeconds"]))
	state.PartialSuccess = boolValue(firstNonNilValue(payload["partialSuccess"], videoPayload["partialSuccess"]))
	state.CompletedSegments = normalizeCompletedVideoSegments(firstNonNilValue(payload["completedSegments"], videoPayload["completedSegments"]))
	state.SuccessfulBillingItemKeys = normalizeStringSlice(firstNonNilValue(payload["successfulBillingItemKeys"], videoPayload["successfulBillingItemKeys"]))
	return state
}

// 处理 AI 作业执行中的解析模型运行时流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) resolveModelRuntime(ctx context.Context, modelName string) (*domain.AIModel, Provider, string, string, string, error) {
	model, err := w.app.Store.GetAIModelByName(ctx, modelName)
	if err != nil {
		return nil, nil, "", "", "", err
	}
	if model == nil {
		return nil, nil, "", "", "", fmt.Errorf("ai model not found: %s", modelName)
	}
	provider, err := resolveProviderForModel(w.providers, model)
	if err != nil {
		return nil, nil, "", "", "", err
	}
	vendor := providerSource(model)
	baseURL, apiKey := ResolveModelRuntimeConfig(w.app.Config, model)
	return model, provider, vendor, baseURL, apiKey, nil
}

// 处理 AI 作业执行中的解析模型运行时配置流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) resolveModelRuntimeConfig(ctx context.Context, modelName string) (string, string, error) {
	_, _, _, baseURL, apiKey, err := w.resolveModelRuntime(ctx, modelName)
	if err != nil {
		return "", "", err
	}
	return baseURL, apiKey, nil
}

// 汇总产物，供接口响应或后续统计逻辑直接复用。
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

// 处理 AI 作业执行中的准备视频生成输入流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) prepareVideoGenerationInputs(ctx context.Context, job *domain.AIJob, leaseToken string, req *VideoRequest, state *videoExecutionState) (map[string]any, error) {
	if job == nil || req == nil || state == nil {
		return nil, nil
	}

	payload := decodePayloadMap(job.InputPayload)
	if !videoPreprocessEnabled(payload) {
		return nil, nil
	}
	if videoStoryboardEnabled(payload) {
		storyboardPayload, err := w.prepareVideoStoryboardPackage(ctx, job, leaseToken, payload, req, state)
		if err == nil {
			return storyboardPayload, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		storyboardPayload = buildVideoStoryboardFallbackPayload(job, req.Prompt, err)
		state.Storyboard = storyboardPayload
		w.app.Logger.Warn(
			"ai worker failed to generate storyboard package, falling back to cover-only flow",
			"job_id", job.ID,
			"model_name", job.ModelName,
			"error", err,
		)

		coverPayload, coverErr := w.prepareVideoCoverReferenceFrame(ctx, job, leaseToken, payload, req, state)
		if coverErr != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			state.FrameRedesign = map[string]any{
				"enabled": true,
				"status":  "fallback",
				"mode":    "cover_only",
				"error":   strings.TrimSpace(coverErr.Error()),
			}
			w.app.Logger.Warn(
				"ai worker failed to generate fallback cover frame, using original references",
				"job_id", job.ID,
				"model_name", job.ModelName,
				"error", coverErr,
			)
			return storyboardPayload, nil
		}
		if len(coverPayload) > 0 {
			storyboardPayload["cover"] = coverPayload
		}
		return storyboardPayload, nil
	}

	if _, err := w.prepareVideoCoverReferenceFrame(ctx, job, leaseToken, payload, req, state); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		state.FrameRedesign = map[string]any{
			"enabled": true,
			"status":  "fallback",
			"mode":    "cover_only",
			"error":   strings.TrimSpace(err.Error()),
		}
		w.app.Logger.Warn(
			"ai worker failed to generate video cover frame, using original references",
			"job_id", job.ID,
			"model_name", job.ModelName,
			"error", err,
		)
	}
	return nil, nil
}

// 处理视频Preprocess启用相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func videoPreprocessEnabled(payload map[string]any) bool {
	if raw, exists := payload["disableVideoPreprocess"]; exists {
		return !boolValue(raw)
	}
	return true
}

// 处理视频分镜启用相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func videoStoryboardEnabled(payload map[string]any) bool {
	if raw, exists := payload["storyboardEnabled"]; exists {
		return boolValue(raw)
	}
	config, _ := payload["storyboardConfig"].(map[string]any)
	return boolValue(config["enabled"])
}

// 处理 AI 作业执行中的准备视频分镜包流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) prepareVideoStoryboardPackage(ctx context.Context, job *domain.AIJob, leaseToken string, payload map[string]any, req *VideoRequest, state *videoExecutionState) (map[string]any, error) {
	if existingRefs := mediaInputsFromVideoReferenceFrames(state.ReferenceFrames); len(existingRefs) > 0 && strings.TrimSpace(state.FinalPrompt) != "" {
		req.ReferenceImages = existingRefs
		req.Prompt = state.FinalPrompt
		if len(state.Storyboard) > 0 {
			return state.Storyboard, nil
		}
		return map[string]any{
			"status":          "reused",
			"optimizedPrompt": state.FinalPrompt,
		}, nil
	}

	sourceReferenceImages := append([]MediaInput(nil), req.ReferenceImages...)
	referenceTexts := normalizeStoryboardTexts(payload["referenceTexts"])
	storyboardPrompt, storyboardModel, storyboardReferences, err := w.resolveVideoStoryboardConfig(ctx)
	if err != nil {
		return nil, err
	}
	publishPayload, _ := payload["publishPayload"].(map[string]any)
	seedContentText := strings.TrimSpace(stringValueFromMap(publishPayload, "contentText", "contentTemplate"))
	publishIntroEnabled := true
	if raw, exists := payload["publishIntroEnabled"]; exists {
		publishIntroEnabled = boolValue(raw)
	}
	publishPromptTemplate := strings.TrimSpace(stringValueFromMap(payload, "publishPromptTemplate"))
	if publishIntroEnabled && publishPromptTemplate == "" {
		publishPromptTemplate = defaultPublishIntroPrompt
	}

	userPrompt := buildVideoStoryboardPackagePrompt(job, req.Prompt, payload, referenceTexts, sourceReferenceImages, storyboardReferences)
	w.syncVideoRunningStage(ctx, job, leaseToken, *state, "storyboarding", "AI 正在生成封面与视频脚本", map[string]any{
		"storyboard": map[string]any{
			"status":    "running",
			"modelName": storyboardModel,
			"startedAt": time.Now().UTC().Format(time.RFC3339),
		},
	})

	model, provider, providerName, baseURL, apiKey, err := w.resolveModelRuntime(ctx, storyboardModel)
	if err != nil {
		return nil, err
	}
	result, err := provider.GenerateStoryboardPackage(ctx, StoryboardPackageRequest{
		Model:           storyboardModel,
		BaseURL:         baseURL,
		APIKey:          apiKey,
		ExplicitBaseURL: modelHasExplicitBaseURL(model),
		SystemPrompt:    storyboardPrompt,
		Prompt:          userPrompt,
		ReferenceImages: sourceReferenceImages,
		AspectRatio:     req.AspectRatio,
		Resolution:      req.Resolution,
	})
	if err != nil {
		return nil, err
	}

	frameMetadata, framePayload, err := w.saveVideoReferenceFrame(ctx, job, result.Cover, "first", userPrompt, providerName, storyboardModel)
	if err != nil {
		return nil, err
	}

	optimizedContentText := seedContentText
	if publishIntroEnabled && strings.TrimSpace(result.PublishIntro) != "" {
		optimizedContentText = strings.TrimSpace(result.PublishIntro)
	}
	responseMode := strings.TrimSpace(stringValue(result.Metadata["responseMode"]))
	if responseMode == "" {
		responseMode = "json"
	}

	state.ReferenceFrames = []map[string]any{frameMetadata}
	state.FrameRedesign = map[string]any{
		"enabled":                   true,
		"status":                    "generated",
		"mode":                      "storyboard_package",
		"modelName":                 storyboardModel,
		"frameCount":                1,
		"sourceReferenceImageCount": len(sourceReferenceImages),
		"sourceReferenceTextCount":  len(referenceTexts),
	}
	state.FinalPrompt = strings.TrimSpace(result.GenerationPrompt)
	state.Storyboard = map[string]any{
		"status":                "generated",
		"mode":                  "cover_and_script",
		"modelName":             storyboardModel,
		"promptTemplate":        storyboardPrompt,
		"publishPromptTemplate": publishPromptTemplate,
		"publishIntroEnabled":   publishIntroEnabled,
		"optimizedPrompt":       state.FinalPrompt,
		"optimizedContentText":  optimizedContentText,
		"responseMode":          responseMode,
		"referenceCount": map[string]int{
			"images": len(sourceReferenceImages),
			"texts":  len(referenceTexts),
		},
		"coverArtifactKey": stringValue(frameMetadata["artifactKey"]),
	}
	if framePayload != nil {
		state.Storyboard["cover"] = framePayload
	}

	req.ReferenceImages = mediaInputsFromVideoReferenceFrames(state.ReferenceFrames)
	req.Prompt = state.FinalPrompt
	w.syncVideoRunningStage(ctx, job, leaseToken, *state, "storyboarding", "AI 已生成封面与视频脚本，准备提交视频生成", map[string]any{
		"storyboard": state.Storyboard,
	})
	return state.Storyboard, nil
}

// 处理 AI 作业执行中的准备视频封面参考帧流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) prepareVideoCoverReferenceFrame(ctx context.Context, job *domain.AIJob, leaseToken string, payload map[string]any, req *VideoRequest, state *videoExecutionState) (map[string]any, error) {
	if existingRefs := mediaInputsFromVideoReferenceFrames(state.ReferenceFrames); len(existingRefs) > 0 {
		req.ReferenceImages = existingRefs
		if len(state.FrameRedesign) == 0 {
			state.FrameRedesign = map[string]any{
				"enabled":    true,
				"status":     "reused",
				"mode":       "cover_only",
				"frameCount": len(existingRefs),
			}
		}
		return map[string]any{
			"status":     "reused",
			"frameCount": len(existingRefs),
		}, nil
	}

	sourceReferenceImages := append([]MediaInput(nil), req.ReferenceImages...)
	referenceTexts := normalizeStoryboardTexts(payload["referenceTexts"])
	coverPromptTemplate, err := w.resolveSkillVideoCoverPromptTemplate(ctx, job)
	if err != nil {
		return nil, err
	}
	frameModelName, imageProvider, imageProviderName, imageBaseURL, imageAPIKey, imageExplicitBaseURL, err := w.resolveVideoCoverModelRuntimeConfig(ctx)
	if err != nil {
		return nil, err
	}

	w.syncVideoRunningStage(ctx, job, leaseToken, *state, "covering", "AI 正在生成封面首帧", nil)

	prompt := buildSkillVideoFramePrompt(coverPromptTemplate, job, payload, req.Prompt, referenceTexts, "first", len(sourceReferenceImages))
	result, err := imageProvider.GenerateImage(ctx, ImageRequest{
		Model:           frameModelName,
		BaseURL:         imageBaseURL,
		APIKey:          imageAPIKey,
		ExplicitBaseURL: imageExplicitBaseURL,
		Prompt:          prompt,
		ReferenceImages: sourceReferenceImages,
		AspectRatio:     req.AspectRatio,
		Resolution:      req.Resolution,
	})
	if err != nil {
		return nil, err
	}
	if len(result.Images) == 0 {
		return nil, fmt.Errorf("cover generation did not return any image")
	}

	frameMetadata, framePayload, err := w.saveVideoReferenceFrame(ctx, job, result.Images[0], "first", prompt, imageProviderName, frameModelName)
	if err != nil {
		return nil, err
	}

	state.ReferenceFrames = []map[string]any{frameMetadata}
	state.FrameRedesign = map[string]any{
		"enabled":                   true,
		"status":                    "generated",
		"mode":                      "cover_only",
		"modelName":                 frameModelName,
		"frameCount":                1,
		"sourceReferenceImageCount": len(sourceReferenceImages),
		"sourceReferenceTextCount":  len(referenceTexts),
	}
	req.ReferenceImages = mediaInputsFromVideoReferenceFrames(state.ReferenceFrames)
	w.syncVideoRunningStage(ctx, job, leaseToken, *state, "covering", "AI 已生成封面首帧，准备提交视频生成", nil)
	return framePayload, nil
}

// 处理 AI 作业执行中的同步视频RunningStage流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) syncVideoRunningStage(ctx context.Context, job *domain.AIJob, leaseToken string, state videoExecutionState, stage string, message string, extras map[string]any) {
	if job == nil {
		return
	}
	outputPayload := mergeMetadataIntoPayload(buildVideoOutputPayload(job, state, nil), "stage", strings.TrimSpace(stage))
	for key, value := range extras {
		outputPayload = mergeMetadataIntoPayload(outputPayload, key, value)
	}
	if _, err := w.syncRunningState(ctx, job, leaseToken, message, outputPayload); err != nil {
		w.app.Logger.Warn("ai worker failed to sync video stage", "job_id", job.ID, "stage", stage, "error", err)
	}
}

// 处理 AI 作业执行中的保存视频参考帧流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) saveVideoReferenceFrame(ctx context.Context, job *domain.AIJob, image BinaryArtifact, role string, prompt string, source string, modelName string) (map[string]any, map[string]any, error) {
	artifactKey := fmt.Sprintf("video-%s-frame%s", strings.TrimSpace(role), extensionForMIME(image.MIMEType, ".png"))
	input, err := w.saveBinaryArtifact(ctx, job, "image", artifactKey, source, image)
	if err != nil {
		return nil, nil, err
	}
	artifacts, err := w.app.Store.UpsertAIJobArtifacts(ctx, []store.UpsertAIJobArtifactInput{input})
	if err != nil {
		return nil, nil, err
	}
	if len(artifacts) == 0 {
		return nil, nil, fmt.Errorf("reference frame artifact was not persisted")
	}
	artifact := artifacts[0]
	frameMetadata := map[string]any{
		"role":        role,
		"artifactKey": artifact.ArtifactKey,
		"prompt":      prompt,
		"modelName":   modelName,
		"fileName":    stringValue(artifact.FileName),
		"mimeType":    stringValue(artifact.MimeType),
		"publicUrl":   stringValue(artifact.PublicURL),
		"storageKey":  stringValue(artifact.StorageKey),
		"sizeBytes":   artifact.SizeBytes,
	}
	framePayload := map[string]any{
		"role":        role,
		"artifactKey": artifact.ArtifactKey,
		"fileName":    stringValue(artifact.FileName),
		"mimeType":    stringValue(artifact.MimeType),
		"publicUrl":   stringValue(artifact.PublicURL),
	}
	return frameMetadata, framePayload, nil
}

// 处理 AI 作业执行中的解析视频分镜配置流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) resolveVideoStoryboardConfig(ctx context.Context) (string, string, []map[string]any, error) {
	prompt := strings.TrimSpace(DefaultVideoStoryboardSystemPrompt)
	references := make([]map[string]any, 0)
	fallbackModel := strings.TrimSpace(w.app.Config.DefaultChatModel)

	settings, err := w.app.Store.GetAdminSystemSettings(ctx)
	if err != nil {
		return "", "", nil, err
	}
	if settings != nil {
		if value := strings.TrimSpace(settings.StoryboardPrompt); value != "" {
			prompt = value
		}
		if value := strings.TrimSpace(settings.DefaultChatModel); value != "" {
			fallbackModel = value
		}
		if len(settings.StoryboardReferences) > 0 {
			var parsed []map[string]any
			if err := json.Unmarshal(settings.StoryboardReferences, &parsed); err == nil {
				references = parsed
			}
		}
	}

	candidates := nonEmpty([]string{
		func() string {
			if settings == nil {
				return ""
			}
			return strings.TrimSpace(settings.StoryboardModel)
		}(),
		strings.TrimSpace(DefaultVideoStoryboardModelName),
		fallbackModel,
	})
	for _, candidate := range candidates {
		model, modelErr := w.app.Store.GetAIModelByName(ctx, candidate)
		if modelErr != nil {
			return "", "", nil, modelErr
		}
		if SupportsStoryboardPackageModel(model) {
			return prompt, candidate, references, nil
		}
	}
	return "", "", nil, fmt.Errorf("storyboard model is not configured or does not support cover+script generation")
}

// 处理 AI 作业执行中的解析视频封面模型运行时配置流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) resolveVideoCoverModelRuntimeConfig(ctx context.Context) (string, Provider, string, string, string, bool, error) {
	candidates := nonEmpty([]string{
		strings.TrimSpace(DefaultVideoCoverModelName),
		strings.TrimSpace(w.app.Config.DefaultImageModel),
	})
	for _, candidate := range candidates {
		model, provider, providerName, baseURL, apiKey, err := w.resolveModelRuntime(ctx, candidate)
		if err != nil {
			return "", nil, "", "", "", false, err
		}
		if model == nil || !model.IsEnabled || strings.TrimSpace(model.Category) != "image" {
			continue
		}
		if strings.TrimSpace(baseURL) == "" {
			continue
		}
		return candidate, provider, providerName, baseURL, apiKey, modelHasExplicitBaseURL(model), nil
	}
	return "", nil, "", "", "", false, fmt.Errorf("video cover model is not configured")
}

// 构建视频分镜回退载荷，为AI作业执行生成后续步骤所需的派生参数或载荷。
func buildVideoStoryboardFallbackPayload(job *domain.AIJob, originalPrompt string, err error) map[string]any {
	payload := decodePayloadMap(job.InputPayload)
	publishPayload, _ := payload["publishPayload"].(map[string]any)
	seedContentText := strings.TrimSpace(stringValueFromMap(publishPayload, "contentText", "contentTemplate"))
	return map[string]any{
		"status":               "fallback",
		"mode":                 "cover_only",
		"optimizedPrompt":      strings.TrimSpace(originalPrompt),
		"optimizedContentText": seedContentText,
		"error":                strings.TrimSpace(errorString(err)),
	}
}

// 构建分镜提示词，为AI作业执行生成后续步骤所需的派生参数或载荷。
func buildStoryboardPrompt(job *domain.AIJob, originalPrompt string, payload map[string]any, referenceTexts []map[string]string, referenceImages []map[string]string, references any) string {
	var builder strings.Builder
	publishIntroEnabled := true
	if raw, exists := payload["publishIntroEnabled"]; exists {
		publishIntroEnabled = boolValue(raw)
	}
	builder.WriteString("请优化下面的内容创作需求，并同时生成后续发布用的简介。\n")
	builder.WriteString("任务类型: ")
	builder.WriteString(strings.TrimSpace(job.JobType))
	builder.WriteString("\n")
	if name := strings.TrimSpace(stringValueFromMap(payload, "skillName")); name != "" {
		builder.WriteString("技能名称: ")
		builder.WriteString(name)
		builder.WriteString("\n")
	}
	if desc := strings.TrimSpace(stringValueFromMap(payload, "skillDescription")); desc != "" {
		builder.WriteString("基础简介: ")
		builder.WriteString(desc)
		builder.WriteString("\n")
	}
	if publishIntroEnabled {
		if publishPrompt := strings.TrimSpace(stringValueFromMap(payload, "publishPromptTemplate")); publishPrompt != "" {
			builder.WriteString("简介优化说明: ")
			builder.WriteString(publishPrompt)
			builder.WriteString("\n")
		}
	}
	if tags := normalizeStringSlice(payload["skillTags"]); len(tags) > 0 {
		builder.WriteString("标签: ")
		builder.WriteString(strings.Join(tags, "、"))
		builder.WriteString("\n")
	}
	builder.WriteString("用户提示词: ")
	builder.WriteString(strings.TrimSpace(originalPrompt))
	builder.WriteString("\n")
	if publishIntroEnabled {
		if publishPayload, ok := payload["publishPayload"].(map[string]any); ok {
			if baseIntro := strings.TrimSpace(stringValueFromMap(publishPayload, "contentText", "contentTemplate")); baseIntro != "" {
				builder.WriteString("当前基础发布简介: ")
				builder.WriteString(baseIntro)
				builder.WriteString("\n")
			}
		}
	}

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
	if publishIntroEnabled {
		builder.WriteString(`

输出要求:
1. 必须只输出一个 JSON 对象，不要使用 Markdown 代码块，不要补充解释。
2. JSON 字段固定为:
   - "generationPrompt": 给图片、视频或文本模型继续执行的最终脚本。
   - "publishIntro": 给第三方平台发布时使用的简介；如果当前任务不需要简介，也请返回空字符串。
3. generationPrompt 和 publishIntro 必须保持同一主题、同一卖点、同一风格，不能彼此矛盾。
4. publishIntro 要保留客户预设风格，但表达要自然变化，避免与过往内容完全重复。
`)
	} else {
		builder.WriteString(`

输出要求:
1. 必须只输出一个 JSON 对象，不要使用 Markdown 代码块，不要补充解释。
2. JSON 字段固定为:
   - "generationPrompt": 给图片、视频或文本模型继续执行的最终脚本。
   - "publishIntro": 固定返回空字符串，不要改写发布简介。
3. 只优化 generationPrompt，让它与基础简介、标签、任务说明和参考资料保持同一主题、同一卖点、同一风格。
`)
	}
	return builder.String()
}

// 构建视频分镜包提示词，为AI作业执行生成后续步骤所需的派生参数或载荷。
func buildVideoStoryboardPackagePrompt(job *domain.AIJob, originalPrompt string, payload map[string]any, referenceTexts []map[string]string, referenceImages []MediaInput, references []map[string]any) string {
	var builder strings.Builder
	publishIntroEnabled := true
	if raw, exists := payload["publishIntroEnabled"]; exists {
		publishIntroEnabled = boolValue(raw)
	}
	builder.WriteString("请基于当前业务提示词和参考素材，同时完成视频首帧封面设计与视频脚本优化。\n")
	builder.WriteString("任务类型: video\n")
	if source := strings.TrimSpace(job.Source); source != "" {
		builder.WriteString("任务来源: ")
		builder.WriteString(source)
		builder.WriteString("\n")
	}
	if name := strings.TrimSpace(stringValueFromMap(payload, "skillName")); name != "" {
		builder.WriteString("技能名称: ")
		builder.WriteString(name)
		builder.WriteString("\n")
	}
	if desc := strings.TrimSpace(stringValueFromMap(payload, "skillDescription")); desc != "" {
		builder.WriteString("技能简介: ")
		builder.WriteString(desc)
		builder.WriteString("\n")
	}
	builder.WriteString("业务提示词: ")
	builder.WriteString(strings.TrimSpace(originalPrompt))
	builder.WriteString("\n")
	if publishIntroEnabled {
		if publishPrompt := strings.TrimSpace(stringValueFromMap(payload, "publishPromptTemplate")); publishPrompt != "" {
			builder.WriteString("发布简介规则: ")
			builder.WriteString(publishPrompt)
			builder.WriteString("\n")
		}
		if publishPayload, ok := payload["publishPayload"].(map[string]any); ok {
			if baseIntro := strings.TrimSpace(stringValueFromMap(publishPayload, "contentText", "contentTemplate")); baseIntro != "" {
				builder.WriteString("当前基础发布简介: ")
				builder.WriteString(baseIntro)
				builder.WriteString("\n")
			}
		}
	}
	if tags := normalizeStringSlice(payload["skillTags"]); len(tags) > 0 {
		builder.WriteString("标签: ")
		builder.WriteString(strings.Join(tags, "、"))
		builder.WriteString("\n")
	}
	builder.WriteString("参考图片数量: ")
	builder.WriteString(fmt.Sprintf("%d", len(referenceImages)))
	builder.WriteString("\n")
	if len(referenceImages) > 0 {
		builder.WriteString("参考图片文件:\n")
		for _, item := range referenceImages {
			builder.WriteString("- ")
			builder.WriteString(strings.TrimSpace(item.FileName))
			builder.WriteString("\n")
		}
	}
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
	if len(references) > 0 {
		if raw, err := json.Marshal(references); err == nil && len(raw) > 0 {
			builder.WriteString("\n管理员补充参考:\n")
			builder.Write(raw)
			builder.WriteString("\n")
		}
	}
	builder.WriteString(`

输出要求:
1. 你必须同时生成一张可直接用作视频首帧封面的图片。
2. 你还必须输出一个 JSON 对象，不要使用 Markdown 代码块，不要补充解释。
3. JSON 字段固定为:
   - "generationPrompt": 给视频模型继续执行的最终脚本。
   - "publishIntro": 给第三方平台发布时使用的简介；如果当前任务不需要简介，也请返回空字符串。
4. 封面图片和 generationPrompt 必须使用同一主体、同一场景逻辑、同一卖点与同一风格，不能互相矛盾。
5. generationPrompt 需要保留真实产品、镜头节奏、动作与场景变化，不要输出泛泛描述。
6. 封面必须保留真实产品特征，不要替换商品，不要只做纯文字海报。
`)
	if !publishIntroEnabled {
		builder.WriteString(`7. publishIntro 固定返回空字符串，不要改写发布简介。
`)
	}
	return builder.String()
}

// 解析分镜Optimization响应，为AI作业执行提供结构化输入。
func parseStoryboardOptimizationResponse(raw string, fallbackPrompt string, fallbackContentText string) (string, string, string) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return strings.TrimSpace(fallbackPrompt), strings.TrimSpace(fallbackContentText), "empty"
	}

	var envelope storyboardOptimizationEnvelope
	if err := json.Unmarshal([]byte(cleaned), &envelope); err == nil {
		prompt := strings.TrimSpace(envelope.GenerationPrompt)
		if prompt == "" {
			prompt = strings.TrimSpace(fallbackPrompt)
		}
		contentText := strings.TrimSpace(envelope.PublishIntro)
		if contentText == "" {
			contentText = strings.TrimSpace(fallbackContentText)
		}
		return prompt, contentText, "json"
	}

	stripped := stripMarkdownCodeFence(cleaned)
	if stripped != cleaned {
		var fenced storyboardOptimizationEnvelope
		if err := json.Unmarshal([]byte(stripped), &fenced); err == nil {
			prompt := strings.TrimSpace(fenced.GenerationPrompt)
			if prompt == "" {
				prompt = strings.TrimSpace(fallbackPrompt)
			}
			contentText := strings.TrimSpace(fenced.PublishIntro)
			if contentText == "" {
				contentText = strings.TrimSpace(fallbackContentText)
			}
			return prompt, contentText, "json_fenced"
		}
	}

	return stripped, strings.TrimSpace(fallbackContentText), "text_fallback"
}

// 剥离 Markdown 代码块围栏，便于后续解析模型输出正文。
func stripMarkdownCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 2 {
		return trimmed
	}
	if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		lines = lines[1 : len(lines)-1]
	} else {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// 规范化字符串切片，去除空项后供提示词和媒体输入复用。
func normalizeStringSlice(raw any) []string {
	items, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]string); ok {
			result := make([]string, 0, len(typed))
			for _, item := range typed {
				trimmed := strings.TrimSpace(item)
				if trimmed != "" {
					result = append(result, trimmed)
				}
			}
			return result
		}
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(stringValue(item))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// 返回首个非 nil 值，供多来源配置回退逻辑复用。
func firstNonNilValue(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

// 规范化已完成视频分段，统一AI作业执行链路的输入格式和后续处理行为。
func normalizeCompletedVideoSegments(raw any) []videoCompletedSegment {
	items, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]videoCompletedSegment); ok {
			return typed
		}
		return nil
	}
	result := make([]videoCompletedSegment, 0, len(items))
	for _, item := range items {
		typed, ok := item.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, videoCompletedSegment{
			SegmentIndex:    intValue(typed["segmentIndex"]),
			ArtifactKey:     strings.TrimSpace(stringValue(typed["artifactKey"])),
			FileName:        strings.TrimSpace(stringValue(typed["fileName"])),
			MimeType:        strings.TrimSpace(stringValue(typed["mimeType"])),
			StorageKey:      strings.TrimSpace(stringValue(typed["storageKey"])),
			PublicURL:       strings.TrimSpace(stringValue(typed["publicUrl"])),
			SizeBytes:       int64(intValue(typed["sizeBytes"])),
			DurationSeconds: intValue(typed["durationSeconds"]),
			ReferenceFrame:  normalizeObject(typed["referenceFrame"]),
		})
	}
	return result
}

// 处理 AI 作业执行中的解析技能视频封面提示词模板流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) resolveSkillVideoCoverPromptTemplate(ctx context.Context, job *domain.AIJob) (string, error) {
	defaultPrompt := strings.TrimSpace(DefaultSkillVideoCoverPromptTemplate)
	if job == nil {
		return defaultPrompt, nil
	}

	if job.SkillID != nil {
		skillID := strings.TrimSpace(*job.SkillID)
		if skillID != "" {
			skill, err := w.app.Store.GetOwnedSkillByID(ctx, skillID, job.OwnerUserID)
			if err != nil {
				return "", err
			}
			if skill != nil && skill.CoverPromptTemplate != nil {
				if value := strings.TrimSpace(*skill.CoverPromptTemplate); value != "" {
					return value, nil
				}
			}
		}
	}

	settings, err := w.app.Store.GetAdminSystemSettings(ctx)
	if err != nil {
		return "", err
	}
	if settings != nil {
		if value := strings.TrimSpace(settings.VideoCoverPrompt); value != "" {
			return value, nil
		}
	}
	return defaultPrompt, nil
}

// 构建技能视频帧提示词，为AI作业执行生成后续步骤所需的派生参数或载荷。
func buildSkillVideoFramePrompt(coverPromptTemplate string, job *domain.AIJob, payload map[string]any, optimizedVideoPrompt string, referenceTexts []map[string]string, role string, sourceImageCount int) string {
	var builder strings.Builder
	role = strings.TrimSpace(strings.ToLower(role))
	frameName := "首帧"
	if role == "last" {
		frameName = "尾帧"
	}

	builder.WriteString("请为一个即将生成的营销短视频设计")
	builder.WriteString(frameName)
	builder.WriteString("封面参考图，后续会把这张图继续交给视频模型作为参考。\n")
	builder.WriteString("请严格执行下面这段封面提示词，并结合客户的原始图片和参考资料完成设计。\n\n")
	builder.WriteString("封面提示词:\n")
	builder.WriteString(strings.TrimSpace(coverPromptTemplate))
	builder.WriteString("\n\n补充上下文:\n")
	builder.WriteString("任务类型: video\n")
	builder.WriteString("来源: 技能中心视文模式\n")
	if name := strings.TrimSpace(stringValueFromMap(payload, "skillName")); name != "" {
		builder.WriteString("技能名称: ")
		builder.WriteString(name)
		builder.WriteString("\n")
	}
	if desc := strings.TrimSpace(stringValueFromMap(payload, "skillDescription")); desc != "" {
		builder.WriteString("基础简介: ")
		builder.WriteString(desc)
		builder.WriteString("\n")
	}
	if prompt := strings.TrimSpace(optimizedVideoPrompt); prompt != "" {
		builder.WriteString("视频生成提示词: ")
		builder.WriteString(prompt)
		builder.WriteString("\n")
	}
	builder.WriteString("客户上传参考图数量: ")
	builder.WriteString(fmt.Sprintf("%d", sourceImageCount))
	builder.WriteString("\n")
	if len(referenceTexts) > 0 {
		builder.WriteString("\n客户参考文本:\n")
		for _, item := range referenceTexts {
			builder.WriteString("- ")
			builder.WriteString(strings.TrimSpace(item["fileName"]))
			builder.WriteString(": ")
			builder.WriteString(strings.TrimSpace(item["content"]))
			builder.WriteString("\n")
		}
	}
	if role == "last" {
		builder.WriteString("\n当前目标: 生成尾帧。要求和首帧保持同一产品与风格体系，但更偏收束、成交或记忆点，不做纯字幕尾卡。\n")
	} else {
		builder.WriteString("\n当前目标: 生成首帧。要求具备开场吸引力、清晰主体和可延展的运动空间。\n")
	}
	builder.WriteString("输出要求: 直接输出一张可用于视频参考的高质量画面，不要返回解释文字。")
	return builder.String()
}

// 解析对话提示词，根据当前配置和上下文确定最终使用结果。
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

// 替换对话消息中的最后一条用户消息，保持提示词拼装顺序稳定。
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

// 规范化分镜Texts，统一AI作业执行链路的输入格式和后续处理行为。
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

// 规范化分镜Images，统一AI作业执行链路的输入格式和后续处理行为。
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

// 判断是否应当准备技能视频参考帧，供当前链路选择后续处理策略。
func shouldPrepareSkillVideoReferenceFrames(job *domain.AIJob, payload map[string]any) bool {
	_ = payload
	return job != nil && strings.EqualFold(strings.TrimSpace(job.JobType), "video")
}

// 收集技能视频来源图片，整合AI作业执行链路所需的候选输入。
func collectSkillVideoSourceImages(payload map[string]any) []MediaInput {
	if payload == nil {
		return nil
	}
	return collectMediaInputs(map[string]any{
		"referenceImages": payload["referenceImages"],
	})
}

// 处理媒体输入视频参考帧相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func mediaInputsFromVideoReferenceFrames(items []map[string]any) []MediaInput {
	result := make([]MediaInput, 0, len(items))
	for _, item := range items {
		url := strings.TrimSpace(stringValue(item["publicUrl"]))
		if url == "" {
			continue
		}
		result = append(result, MediaInput{
			URL:      url,
			FileName: strings.TrimSpace(stringValue(item["fileName"])),
			MIMEType: strings.TrimSpace(stringValue(item["mimeType"])),
			Role:     strings.TrimSpace(stringValue(item["role"])),
		})
	}
	return result
}

// 规范化对象切片结构，过滤空值并统一后续 JSON 处理。
func normalizeObjectSlice(raw any) []map[string]any {
	items, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]map[string]any); ok {
			return typed
		}
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		typed, ok := item.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, typed)
	}
	return result
}

// 规范化单个对象内容，统一媒体和运行时载荷中的字段结构。
func normalizeObject(raw any) map[string]any {
	typed, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return typed
}

// 返回首个非空对象切片，供媒体和分镜输入回退逻辑复用。
func firstNonEmptyObjectSlice(values ...[]map[string]any) []map[string]any {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

// 处理bool值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理计费载荷相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 构建Completion消息，为AI作业执行生成后续步骤所需的派生参数或载荷。
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

func buildAIChatCompletionBaseMessage(result *ChatResult) string {
	if result == nil || strings.TrimSpace(result.CompletionState) != ChatCompletionStateIncomplete {
		return "AI 聊天已完成"
	}
	for _, warningCode := range result.WarningCodes {
		if strings.TrimSpace(warningCode) == ChatWarningReasoningOnlyOutput {
			return "AI 聊天已完成（仅收到思考过程，未收到最终答复）"
		}
	}
	if strings.Contains(strings.TrimSpace(result.WarningMessage), "仅收到思考过程") {
		return "AI 聊天已完成（仅收到思考过程，未收到最终答复）"
	}
	return "AI 聊天已完成（内容可能不完整）"
}

// 处理计费额度Ptr相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理mustJSON相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理safe文件名称相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func safeFileName(value string) string {
	value = strings.TrimSpace(filepath.Base(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Trim(value, "-_/")
	return value
}

// 处理safe产物键相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func safeArtifactKey(primary string, fallback string) string {
	primary = safeFileName(primary)
	if primary != "" {
		return primary
	}
	return safeFileName(fallback)
}

// 解析RFC3339，为AI作业执行提供结构化输入。
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

// 将非空字符串转换为指针，统一存储层对可选字符串字段的入参表达。
func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

// 处理首个NonNil时间相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstNonNilTime(value *time.Time, fallback time.Time) time.Time {
	if value != nil {
		return value.UTC()
	}
	return fallback.UTC()
}
