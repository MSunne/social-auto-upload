package ai

import (
	"encoding/json"
	"testing"

	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/store"
)

func TestBuildChatBillingInputUsesTokenFallbackFields(t *testing.T) {
	job := &domain.AIJob{
		ID:          "job-chat-1",
		OwnerUserID: "user-1",
		ModelName:   "gemini-3.1-pro-preview",
		JobType:     "chat",
	}
	result := &ChatResult{
		Text: "ok",
		Usage: map[string]any{
			"input_tokens":           1234,
			"candidates_token_count": 567,
		},
	}

	input := buildChatBillingInput(job, result)
	if input.UserID != "user-1" || input.SourceID != "job-chat-1" {
		t.Fatalf("unexpected billing source: %#v", input)
	}
	if len(input.Metrics) != 2 {
		t.Fatalf("expected 2 billing metrics, got %d", len(input.Metrics))
	}
	if input.Metrics[0].MeterCode != "chat_input_tokens" || input.Metrics[0].Quantity != 1234 {
		t.Fatalf("unexpected input token metric: %#v", input.Metrics[0])
	}
	if input.Metrics[1].MeterCode != "chat_output_tokens" || input.Metrics[1].Quantity != 567 {
		t.Fatalf("unexpected output token metric: %#v", input.Metrics[1])
	}
}

func TestBuildImageAndVideoBillingInputs(t *testing.T) {
	job := &domain.AIJob{
		ID:          "job-media-1",
		OwnerUserID: "user-2",
		ModelName:   "gemini-3-pro-image-preview",
		JobType:     "image",
	}

	imageInput := buildImageBillingInput(job, 2)
	if len(imageInput.Metrics) != 1 {
		t.Fatalf("expected one image billing metric, got %d", len(imageInput.Metrics))
	}
	if imageInput.Metrics[0].MeterCode != "image_generations" || imageInput.Metrics[0].Quantity != 2 {
		t.Fatalf("unexpected image billing metric: %#v", imageInput.Metrics[0])
	}

	job.ModelName = "veo-3.1-fast-fl"
	job.JobType = "video"
	videoInput := buildVideoBillingInput(job)
	if len(videoInput.Metrics) != 1 {
		t.Fatalf("expected one video billing metric, got %d", len(videoInput.Metrics))
	}
	if videoInput.Metrics[0].MeterCode != "video_generations" || videoInput.Metrics[0].Quantity != 1 {
		t.Fatalf("unexpected video billing metric: %#v", videoInput.Metrics[0])
	}
}

func TestBuildVideoBillingInputCarriesDurationMetadata(t *testing.T) {
	job := &domain.AIJob{
		ID:          "job-video-2",
		OwnerUserID: "user-3",
		ModelName:   "veo-3.1-fast-fl",
		JobType:     "video",
		InputPayload: mustJSONBytes(map[string]any{
			"durationSeconds": 12,
		}),
	}

	videoInput := buildVideoBillingInput(job)
	if len(videoInput.Metrics) != 1 {
		t.Fatalf("expected one video billing metric, got %d", len(videoInput.Metrics))
	}

	var metadata map[string]any
	if err := json.Unmarshal(videoInput.Metrics[0].Metadata, &metadata); err != nil {
		t.Fatalf("expected video billing metadata to be json: %v", err)
	}
	if metadata["durationSeconds"] != float64(12) {
		t.Fatalf("expected duration metadata 12, got %#v", metadata)
	}
}

func TestBuildEstimatedUsageBillingInputForMediaJobs(t *testing.T) {
	job := &domain.AIJob{
		ID:          "job-image-estimate",
		OwnerUserID: "user-4",
		ModelName:   "gemini-3-pro-image-preview",
		JobType:     "image",
		InputPayload: mustJSONBytes(map[string]any{
			"count": 3,
		}),
	}

	imageInput := BuildEstimatedUsageBillingInput(job)
	if len(imageInput.Metrics) != 1 || imageInput.Metrics[0].Quantity != 3 {
		t.Fatalf("expected image estimate to honor count, got %#v", imageInput.Metrics)
	}

	job.JobType = "video"
	job.ModelName = "veo-3.1-fast-fl"
	job.InputPayload = mustJSONBytes(map[string]any{
		"durationSeconds": 6,
	})
	videoInput := BuildEstimatedUsageBillingInput(job)
	if len(videoInput.Metrics) != 1 || videoInput.Metrics[0].MeterCode != "video_generations" {
		t.Fatalf("expected video estimate metric, got %#v", videoInput.Metrics)
	}
}

func TestBuildUsageBillingBlockMessage(t *testing.T) {
	message := BuildUsageBillingBlockMessage(&store.ApplyUsageBillingResult{
		BillStatus:  "failed",
		BillMessage: "wallet credits insufficient for fallback debit",
	})
	if message != "当前套餐额度已用尽，且钱包积分不足，任务开始前需要预扣费，请先充值后再试。" {
		t.Fatalf("unexpected fallback block message: %q", message)
	}

	message = BuildUsageBillingBlockMessage(&store.ApplyUsageBillingResult{
		BillStatus:  "failed",
		BillMessage: "pricing rule not found",
	})
	if message != "当前任务缺少可用计费规则，暂时无法启动，请联系管理员检查计费配置。" {
		t.Fatalf("unexpected pricing block message: %q", message)
	}
}

func TestBuildCompletionMessageReflectsBillingState(t *testing.T) {
	message := buildCompletionMessage("AI 视频生成完成", &store.ApplyUsageBillingResult{
		BillStatus:   "billed",
		TotalCredits: 400,
	})
	if message != "AI 视频生成完成，已扣减 400 积分" {
		t.Fatalf("unexpected billed completion message: %q", message)
	}

	message = buildCompletionMessage("AI 视频生成完成", &store.ApplyUsageBillingResult{
		BillStatus:  "failed",
		BillMessage: "wallet credits insufficient",
	})
	if message != "AI 视频生成完成，计费待处理: wallet credits insufficient" {
		t.Fatalf("unexpected failed completion message: %q", message)
	}
}
