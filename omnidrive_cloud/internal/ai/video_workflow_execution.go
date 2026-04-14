package ai

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/store"
)

const workflowSegmentRetryLimit = 2

// 处理 AI 作业执行中的执行工作流视频流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) executeWorkflowVideo(
	ctx context.Context,
	job *domain.AIJob,
	leaseToken string,
	leaseExpiresAt time.Time,
	snapshot *workflowPricingSnapshot,
) error {
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
	req.Model = normalizeVideoModel(strings.TrimSpace(job.ModelName), req.AspectRatio, len(req.ReferenceImages) > 0)

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
	if state.DurationSeconds <= 0 {
		state.DurationSeconds = snapshot.DurationSeconds
	}
	if state.SegmentSeconds <= 0 {
		state.SegmentSeconds = snapshot.SegmentSeconds
	}
	if state.PlannedSegments <= 0 {
		state.PlannedSegments = plannedVideoSegmentCount(snapshot.DurationSeconds, snapshot.SegmentSeconds)
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
	markWorkflowPreparationBillingSuccess(&state, snapshot)
	if state.ActiveSegment <= 0 {
		state.ActiveSegment = len(state.CompletedSegments) + 1
	}

	partialFailureMessage := ""
	for segmentIndex := state.ActiveSegment; segmentIndex <= state.PlannedSegments; segmentIndex++ {
		state.ActiveSegment = segmentIndex
		segmentReq := req
		segmentSeconds := state.SegmentSeconds
		segmentReq.DurationSeconds = intPointer(segmentSeconds)
		segmentReq.ReferenceImages = buildWorkflowSegmentReferenceImages(req.ReferenceImages, state)
		segmentReq.Prompt = buildWorkflowSegmentPrompt(req.Prompt, state, segmentIndex, state.PlannedSegments)

		var segmentArtifact *BinaryArtifact
		var segmentErr error
		for attempt := 1; attempt <= workflowSegmentRetryLimit; attempt++ {
			segmentArtifact, leaseExpiresAt, segmentErr = w.executeWorkflowVideoSegment(ctx, provider, job, leaseToken, leaseExpiresAt, &segmentReq, &state, apiKey, attempt)
			if segmentErr == nil {
				break
			}
			var requeueErr *requeueExecutionError
			if errorsAsRequeue(segmentErr, &requeueErr) {
				return segmentErr
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempt >= workflowSegmentRetryLimit {
				break
			}
			resetActiveVideoSegmentState(&state)
			retryMessage := fmt.Sprintf("AI 视频第 %d/%d 段失败，正在重试", segmentIndex, state.PlannedSegments)
			w.syncVideoRunningStage(ctx, job, leaseToken, state, "segment_retry", retryMessage, map[string]any{
				"segmentIndex": segmentIndex,
				"attempt":      attempt + 1,
				"maxAttempts":  workflowSegmentRetryLimit,
			})
		}
		if segmentErr != nil {
			if len(state.CompletedSegments) == 0 {
				return segmentErr
			}
			state.PartialSuccess = true
			partialFailureMessage = truncateFailureMessage(segmentErr.Error(), 120)
			resetActiveVideoSegmentState(&state)
			break
		}

		if err := w.persistCompletedVideoSegment(ctx, job, &state, segmentIndex, segmentSeconds, *segmentArtifact); err != nil {
			return err
		}
		state.ActualDuration = len(state.CompletedSegments) * segmentSeconds
		state.SuccessfulBillingItemKeys = appendUniqueString(state.SuccessfulBillingItemKeys, workflowSuccessfulSegmentItemKey(snapshot, segmentIndex))
		resetActiveVideoSegmentState(&state)
		progressMessage := fmt.Sprintf("AI 视频已完成第 %d/%d 段，继续生成下一段", segmentIndex, state.PlannedSegments)
		w.syncVideoRunningStage(ctx, job, leaseToken, state, "segment_completed", progressMessage, map[string]any{
			"segmentIndex": segmentIndex,
			"segmentCount": state.PlannedSegments,
		})
	}

	if len(state.CompletedSegments) == 0 {
		return fmt.Errorf("workflow video did not complete any segment")
	}

	finalArtifact, err := w.buildWorkflowFinalVideoArtifact(ctx, &state)
	if err != nil {
		return err
	}
	finalArtifactKey := strings.TrimSpace(finalArtifact.ArtifactKey)
	if finalArtifactKey == "" {
		finalArtifactKey = safeArtifactKey(finalArtifact.FileName, "video-final.mp4")
	}
	input, err := w.saveBinaryArtifact(ctx, job, "video", finalArtifactKey, providerName, *finalArtifact)
	if err != nil {
		return err
	}
	artifacts, err := w.app.Store.UpsertAIJobArtifacts(ctx, []store.UpsertAIJobArtifactInput{input})
	if err != nil {
		return err
	}

	baseMessage := "AI 视频生成完成"
	if state.PartialSuccess {
		baseMessage = "AI 视频已按成功片段交付，未完成部分已退款"
	}
	session, items, err := FinalizeWorkflowBillingSession(ctx, w.app, job, state.SuccessfulBillingItemKeys, buildWorkflowBillingMessage(baseMessage, partialFailureMessage), nil)
	if err != nil {
		return err
	}
	billing := WorkflowBillingResultFromSession(session, items)

	outputPayload := buildVideoOutputPayload(job, state, artifacts)
	if len(storyboardPayload) > 0 {
		outputPayload = mergeMetadataIntoPayload(outputPayload, "storyboard", storyboardPayload)
	}
	if state.PartialSuccess {
		outputPayload = mergeMetadataIntoPayload(outputPayload, "partialFailureMessage", partialFailureMessage)
	}
	outputPayload = mergeBillingIntoPayload(outputPayload, billing)
	session, items, err = FinalizeWorkflowBillingSession(ctx, w.app, job, state.SuccessfulBillingItemKeys, buildWorkflowBillingMessage(baseMessage, partialFailureMessage), outputPayload)
	if err != nil {
		return err
	}
	billing = WorkflowBillingResultFromSession(session, items)
	outputPayload = mergeBillingIntoPayload(outputPayload, billing)

	message := baseMessage
	if state.PartialSuccess && partialFailureMessage != "" {
		message = fmt.Sprintf("%s：%s", baseMessage, partialFailureMessage)
	} else if !state.PartialSuccess {
		message = buildCompletionMessage(baseMessage, billing)
	}
	if _, err := w.completeJob(ctx, job, leaseToken, message, outputPayload, billingCreditsPtr(billing)); err != nil {
		return err
	}
	w.recordAuditEvent(ctx, job, "cloud_generate_success", "AI 云端生成完成", "success", stringPtr(message), map[string]any{
		"jobType":             job.JobType,
		"modelName":           job.ModelName,
		"artifactCount":       len(artifacts),
		"segmentCount":        len(state.CompletedSegments),
		"plannedSegmentCount": state.PlannedSegments,
		"partialSuccess":      state.PartialSuccess,
	})
	return nil
}

// 处理 AI 作业执行中的执行工作流视频分段流程，依赖作业状态、租约和持久化结果推进链路。
func (w *Worker) executeWorkflowVideoSegment(
	ctx context.Context,
	provider Provider,
	job *domain.AIJob,
	leaseToken string,
	leaseExpiresAt time.Time,
	req *VideoRequest,
	state *videoExecutionState,
	apiKey string,
	attempt int,
) (*BinaryArtifact, time.Time, error) {
	if req == nil || state == nil {
		return nil, leaseExpiresAt, fmt.Errorf("video segment request is incomplete")
	}

	if strings.TrimSpace(state.RemoteVideoID) == "" {
		submission, err := provider.SubmitVideo(ctx, *req)
		if err != nil {
			if shouldRequeueVideoSubmissionError(err) {
				return nil, leaseExpiresAt, buildTemporaryVideoRequeueError(job, *state, err)
			}
			return nil, leaseExpiresAt, err
		}
		if strings.TrimSpace(submission.ID) == "" {
			return nil, leaseExpiresAt, fmt.Errorf("video submission did not return id")
		}
		state.RemoteVideoID = submission.ID
		state.RemoteStatus = strings.TrimSpace(submission.Status)
		state.SubmittedAt = firstNonNilTime(submission.CreatedAt, time.Now().UTC())
		state.UpdatedAt = state.SubmittedAt

		message := fmt.Sprintf("AI 视频第 %d/%d 段已提交，等待生成完成", state.ActiveSegment, state.PlannedSegments)
		runningPayload := buildVideoOutputPayload(job, *state, nil)
		if _, err := w.syncRunningState(ctx, job, leaseToken, message, runningPayload); err != nil {
			return nil, leaseExpiresAt, err
		}
	}

	deadline := state.SubmittedAt.Add(w.videoTimeout)
	for {
		if ctx.Err() != nil {
			return nil, leaseExpiresAt, ctx.Err()
		}

		var renewErr error
		leaseExpiresAt, renewErr = w.renewLease(ctx, job.ID, leaseToken, leaseExpiresAt)
		if renewErr != nil {
			return nil, leaseExpiresAt, renewErr
		}

		status, err := provider.GetVideo(ctx, state.RemoteVideoID, req.Model, state.BaseURL, apiKey, state.ExplicitBaseURL)
		if err != nil {
			if isTransientVideoProviderExecutionError(err) {
				return nil, leaseExpiresAt, buildTemporaryVideoRequeueError(job, *state, err)
			}
			return nil, leaseExpiresAt, err
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
			artifact, err := w.downloadAndFinalizeVideoArtifact(ctx, provider, job, *req, state, apiKey)
			if err != nil {
				if isTransientVideoProviderExecutionError(err) {
					return nil, leaseExpiresAt, buildTemporaryVideoRequeueError(job, *state, err)
				}
				return nil, leaseExpiresAt, err
			}
			return artifact, leaseExpiresAt, nil
		case "failed":
			if state.FailureCode != "" && state.Message != "" {
				return nil, leaseExpiresAt, fmt.Errorf("%s: %s", state.FailureCode, state.Message)
			}
			if state.Message != "" {
				return nil, leaseExpiresAt, fmt.Errorf("%s", state.Message)
			}
			if state.FailureCode != "" {
				return nil, leaseExpiresAt, fmt.Errorf("%s", state.FailureCode)
			}
			return nil, leaseExpiresAt, fmt.Errorf("video generation segment failed")
		default:
			if time.Now().UTC().After(deadline) {
				message := fmt.Sprintf("AI 视频第 %d/%d 段超过 %s，继续后台回查云端结果", state.ActiveSegment, state.PlannedSegments, w.videoTimeout)
				return nil, leaseExpiresAt, &requeueExecutionError{
					Message:       message,
					OutputPayload: buildVideoOutputPayload(job, *state, nil),
				}
			}
			message := fmt.Sprintf("AI 视频正在生成第 %d/%d 段", state.ActiveSegment, state.PlannedSegments)
			if strings.TrimSpace(state.Message) != "" {
				message = fmt.Sprintf("%s：%s", message, strings.TrimSpace(state.Message))
			}
			runningPayload := buildVideoOutputPayload(job, *state, nil)
			runningPayload = mergeMetadataIntoPayload(runningPayload, "segmentAttempt", attempt)
			if _, err := w.syncRunningState(ctx, job, leaseToken, message, runningPayload); err != nil {
				return nil, leaseExpiresAt, err
			}
			pollInterval := mediaExecutionPollInterval(state.SubmittedAt, time.Now().UTC())
			select {
			case <-ctx.Done():
				return nil, leaseExpiresAt, ctx.Err()
			case <-time.After(pollInterval):
			}
		}
	}
}

// 为 AI 作业执行提供persist已完成视频分段辅助能力，统一处理输入、状态和产物细节。
func (w *Worker) persistCompletedVideoSegment(ctx context.Context, job *domain.AIJob, state *videoExecutionState, segmentIndex int, durationSeconds int, artifact BinaryArtifact) error {
	artifactKey := fmt.Sprintf("video-segment-%02d", segmentIndex)
	input, err := w.saveBinaryArtifact(ctx, job, "video_segment", artifactKey, state.Provider, artifact)
	if err != nil {
		return err
	}
	artifacts, err := w.app.Store.UpsertAIJobArtifacts(ctx, []store.UpsertAIJobArtifactInput{input})
	if err != nil {
		return err
	}
	if len(artifacts) == 0 {
		return fmt.Errorf("video segment artifact was not persisted")
	}
	record := artifacts[0]
	lastFrameImage, err := extractVideoLastFrame(ctx, artifact, w.app.Config.AIVideoFFmpegPath)
	if err != nil {
		return err
	}
	frameMetadata, _, err := w.saveVideoReferenceFrame(ctx, job, lastFrameImage, fmt.Sprintf("segment-%02d-last", segmentIndex), fmt.Sprintf("video segment %d last frame", segmentIndex), state.Provider, job.ModelName)
	if err != nil {
		return err
	}
	state.ReferenceFrames = []map[string]any{frameMetadata}
	state.CompletedSegments = append(state.CompletedSegments, videoCompletedSegment{
		SegmentIndex:    segmentIndex,
		ArtifactKey:     record.ArtifactKey,
		FileName:        stringValue(record.FileName),
		MimeType:        stringValue(record.MimeType),
		StorageKey:      stringValue(record.StorageKey),
		PublicURL:       stringValue(record.PublicURL),
		SizeBytes:       int64ValueOrZero(record.SizeBytes),
		DurationSeconds: durationSeconds,
		ReferenceFrame:  frameMetadata,
	})
	return nil
}

// 为 AI 作业执行提供构建工作流Final视频产物辅助能力，统一处理输入、状态和产物细节。
func (w *Worker) buildWorkflowFinalVideoArtifact(ctx context.Context, state *videoExecutionState) (*BinaryArtifact, error) {
	if state == nil || len(state.CompletedSegments) == 0 {
		return nil, fmt.Errorf("workflow video segments are missing")
	}
	if len(state.CompletedSegments) == 1 {
		return w.loadCompletedSegmentArtifact(ctx, state.CompletedSegments[0])
	}

	segments := make([]BinaryArtifact, 0, len(state.CompletedSegments))
	for _, segment := range state.CompletedSegments {
		artifact, err := w.loadCompletedSegmentArtifact(ctx, segment)
		if err != nil {
			return nil, err
		}
		segments = append(segments, *artifact)
	}
	merged, err := concatVideoArtifacts(ctx, segments, w.app.Config.AIVideoFFmpegPath)
	if err != nil {
		return nil, err
	}
	if !w.app.Config.AIVideoStandardizeEnabled {
		return &merged, nil
	}
	standardized, err := standardizeVideoArtifact(ctx, merged, w.app.Config.AIVideoFFmpegPath)
	if err != nil {
		return nil, err
	}
	return &standardized, nil
}

// 为 AI 作业执行提供加载已完成分段产物辅助能力，统一处理输入、状态和产物细节。
func (w *Worker) loadCompletedSegmentArtifact(ctx context.Context, segment videoCompletedSegment) (*BinaryArtifact, error) {
	storageKey := strings.TrimSpace(segment.StorageKey)
	if storageKey == "" && strings.TrimSpace(segment.PublicURL) != "" {
		if derived, ok := w.app.Storage.StorageKeyFromPublicURL(segment.PublicURL); ok {
			storageKey = derived
		}
	}
	if storageKey == "" {
		return nil, fmt.Errorf("segment artifact %s storage key is missing", segment.ArtifactKey)
	}
	data, contentType, err := w.app.Storage.ReadBytes(ctx, storageKey)
	if err != nil {
		return nil, err
	}
	fileName := strings.TrimSpace(segment.FileName)
	if fileName == "" {
		fileName = "video-segment.mp4"
	}
	if contentType == "" {
		contentType = strings.TrimSpace(segment.MimeType)
	}
	if contentType == "" {
		contentType = "video/mp4"
	}
	return &BinaryArtifact{
		FileName:  fileName,
		MIMEType:  contentType,
		Data:      data,
		SizeBytes: int64(len(data)),
	}, nil
}

// 构建工作流分段参考图片s，为视频工作流生成后续步骤所需的派生参数或载荷。
func buildWorkflowSegmentReferenceImages(base []MediaInput, state videoExecutionState) []MediaInput {
	if len(state.ReferenceFrames) > 0 {
		if refs := mediaInputsFromVideoReferenceFrames(state.ReferenceFrames); len(refs) > 0 {
			return refs
		}
	}
	return append([]MediaInput(nil), base...)
}

// 构建工作流分段提示词，为视频工作流生成后续步骤所需的派生参数或载荷。
func buildWorkflowSegmentPrompt(basePrompt string, state videoExecutionState, segmentIndex int, plannedSegments int) string {
	trimmed := strings.TrimSpace(basePrompt)
	if trimmed == "" {
		return ""
	}
	if plannedSegments <= 1 || segmentIndex <= 0 {
		return trimmed
	}

	completedCount := len(state.CompletedSegments)
	stageInstruction := "作为中段内容，延续上一段最后一帧自然推进动作、镜头和情绪，不要重新起镜。"
	switch {
	case segmentIndex <= 1:
		stageInstruction = "作为开场段，建立主体、场景和节奏，并与提供的首帧参考图保持一致，同时给后续段落留出自然延展空间。"
	case segmentIndex >= plannedSegments:
		stageInstruction = "作为结尾段，延续上一段最后一帧自然收束，完成动作闭环和卖点落点，不要突然切换主体、场景或风格。"
	}

	var builder strings.Builder
	builder.WriteString(trimmed)
	builder.WriteString("\n\n分段续写要求:\n")
	builder.WriteString(fmt.Sprintf("1. 当前生成第 %d/%d 段。", segmentIndex, plannedSegments))
	if completedCount > 0 {
		builder.WriteString(fmt.Sprintf(" 前面已完成 %d 段，必须保持产品、角色、场景、光线、色调和镜头语言连续一致。", completedCount))
	}
	builder.WriteString("\n2. ")
	builder.WriteString(stageInstruction)
	builder.WriteString("\n3. 系统会附带上一段最后一帧作为参考图，请把它视为当前段落的起始画面，确保动作、构图和主体位置自然衔接。")
	builder.WriteString("\n4. 不要新增无关主体，不要突然改变服装、材质、品牌元素、空间关系或叙事方向。")
	return builder.String()
}

// 处理reset活跃视频分段状态相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func resetActiveVideoSegmentState(state *videoExecutionState) {
	if state == nil {
		return
	}
	state.RemoteVideoID = ""
	state.RemoteStatus = ""
	state.ProgressPercent = nil
	state.ContentURL = ""
	state.Message = ""
	state.FailureCode = ""
	state.SubmittedAt = time.Time{}
	state.UpdatedAt = time.Time{}
}

// 处理追加UniqueString相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func appendUniqueString(values []string, extra string) []string {
	trimmed := strings.TrimSpace(extra)
	if trimmed == "" {
		return values
	}
	for _, value := range values {
		if strings.TrimSpace(value) == trimmed {
			return values
		}
	}
	return append(values, trimmed)
}

// 处理mark工作流Preparation计费Success相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func markWorkflowPreparationBillingSuccess(state *videoExecutionState, snapshot *workflowPricingSnapshot) {
	if state == nil || snapshot == nil || (snapshot.SpecialPriceCredits != nil && *snapshot.SpecialPriceCredits > 0) {
		return
	}
	storyboardStatus := strings.TrimSpace(stringValue(state.Storyboard["status"]))
	storyboardMode := strings.TrimSpace(stringValue(state.Storyboard["mode"]))
	if storyboardStatus != "" && storyboardMode == "cover_and_script" {
		state.SuccessfulBillingItemKeys = appendUniqueString(state.SuccessfulBillingItemKeys, workflowStoryboardPackageItemKey())
		return
	}
	frameStatus := strings.TrimSpace(stringValue(state.FrameRedesign["status"]))
	frameMode := strings.TrimSpace(stringValue(state.FrameRedesign["mode"]))
	if frameStatus != "" && frameMode == "cover_only" {
		state.SuccessfulBillingItemKeys = appendUniqueString(state.SuccessfulBillingItemKeys, workflowCoverFrameItemKey())
	}
}

// 处理工作流Successful分段Item键相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func workflowSuccessfulSegmentItemKey(snapshot *workflowPricingSnapshot, segmentIndex int) string {
	if snapshot != nil && snapshot.SpecialPriceCredits != nil && *snapshot.SpecialPriceCredits > 0 {
		return workflowSpecialPriceSegmentItemKey(segmentIndex)
	}
	return workflowVideoSegmentItemKey(segmentIndex)
}

// 构建工作流计费消息，为视频工作流生成后续步骤所需的派生参数或载荷。
func buildWorkflowBillingMessage(base string, detail string) string {
	base = strings.TrimSpace(base)
	detail = strings.TrimSpace(detail)
	if base == "" {
		return detail
	}
	if detail == "" {
		return base
	}
	return base + "：" + detail
}

// 处理intPointer相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func intPointer(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

// 处理int64值Zero相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func int64ValueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

// 处理concat视频产物相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func concatVideoArtifacts(ctx context.Context, artifacts []BinaryArtifact, ffmpegPath string) (BinaryArtifact, error) {
	if len(artifacts) == 0 {
		return BinaryArtifact{}, fmt.Errorf("video artifacts are required")
	}

	tempDir, err := os.MkdirTemp("", "omnidrive-video-concat-*")
	if err != nil {
		return BinaryArtifact{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	listPath := filepath.Join(tempDir, "segments.txt")
	var lines []string
	for index, artifact := range artifacts {
		fileName := safeFileName(artifact.FileName)
		if fileName == "" {
			fileName = fmt.Sprintf("segment-%02d.mp4", index+1)
		}
		inputPath := filepath.Join(tempDir, fmt.Sprintf("%02d-%s", index+1, fileName))
		if err := os.WriteFile(inputPath, artifact.Data, 0o600); err != nil {
			return BinaryArtifact{}, fmt.Errorf("write temp segment: %w", err)
		}
		lines = append(lines, fmt.Sprintf("file '%s'", strings.ReplaceAll(inputPath, "'", "'\\''")))
	}
	if err := os.WriteFile(listPath, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return BinaryArtifact{}, fmt.Errorf("write concat list: %w", err)
	}

	outputPath := filepath.Join(tempDir, "merged.mp4")
	command := exec.CommandContext(
		ctx,
		ffmpegBinary(ffmpegPath),
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		"-movflags", "+faststart",
		outputPath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return BinaryArtifact{}, fmt.Errorf("ffmpeg concat video failed: %s", formatFFmpegExecutionError(err, output, ffmpegPath))
	}

	mergedBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return BinaryArtifact{}, fmt.Errorf("read concat output: %w", err)
	}
	return BinaryArtifact{
		FileName:  "video-final.mp4",
		MIMEType:  "video/mp4",
		Data:      mergedBytes,
		SizeBytes: int64(len(mergedBytes)),
	}, nil
}

// 提取视频Last帧，供视频工作流后续关联和分支判断复用。
func extractVideoLastFrame(ctx context.Context, artifact BinaryArtifact, ffmpegPath string) (BinaryArtifact, error) {
	if len(artifact.Data) == 0 {
		return BinaryArtifact{}, fmt.Errorf("video artifact data is empty")
	}

	tempDir, err := os.MkdirTemp("", "omnidrive-video-frame-*")
	if err != nil {
		return BinaryArtifact{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	inputName := safeFileName(artifact.FileName)
	if inputName == "" {
		inputName = "input.mp4"
	}
	inputPath := filepath.Join(tempDir, inputName)
	outputPath := filepath.Join(tempDir, "last-frame.png")
	if err := os.WriteFile(inputPath, artifact.Data, 0o600); err != nil {
		return BinaryArtifact{}, fmt.Errorf("write temp input video: %w", err)
	}

	command := exec.CommandContext(
		ctx,
		ffmpegBinary(ffmpegPath),
		"-y",
		"-sseof", "-0.1",
		"-i", inputPath,
		"-frames:v", "1",
		outputPath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return BinaryArtifact{}, fmt.Errorf("ffmpeg extract video frame failed: %s", formatFFmpegExecutionError(err, output, ffmpegPath))
	}

	imageBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return BinaryArtifact{}, fmt.Errorf("read extracted frame: %w", err)
	}
	return BinaryArtifact{
		FileName:  "last-frame.png",
		MIMEType:  "image/png",
		Data:      imageBytes,
		SizeBytes: int64(len(imageBytes)),
	}, nil
}

// 处理errorsAs重新入队相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func errorsAsRequeue(err error, target **requeueExecutionError) bool {
	if err == nil {
		return false
	}
	if typed, ok := err.(*requeueExecutionError); ok {
		*target = typed
		return true
	}
	return false
}
