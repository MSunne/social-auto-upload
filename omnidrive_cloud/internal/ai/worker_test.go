package ai

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"omnidrive_cloud/internal/domain"
)

func TestBuildAIExecutionFailureMessageUsesFriendlyLMRootCopy(t *testing.T) {
	err := errors.New("provider request failed: Error forwarded from LMRoot. Failed to get combined chunks. Invalid response from Gemini model. Failed to parse JSON. prompt_rewriter lmroot_generate veo3_prompt_rewriter_utils.cc")

	got := buildAIExecutionFailureMessage("video", err)

	want := "AI 云端执行失败: 上游视频模型临时返回了异常结果，请稍后重试"
	if got != want {
		t.Fatalf("buildAIExecutionFailureMessage() = %q, want %q", got, want)
	}
}

func TestBuildAIExecutionFailureMessageTruncatesLongErrors(t *testing.T) {
	err := errors.New(strings.Repeat("x", 400))

	got := buildAIExecutionFailureMessage("chat", err)

	if !strings.HasPrefix(got, "AI 云端执行失败: ") {
		t.Fatalf("unexpected failure prefix: %q", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected truncated suffix, got %q", got)
	}
	if len([]rune(got)) > len([]rune("AI 云端执行失败: "))+283 {
		t.Fatalf("expected truncated message length, got %d runes", len([]rune(got)))
	}
}

func TestShouldAutoRetryMediaFailureForFirstImageFailure(t *testing.T) {
	job := &domain.AIJob{JobType: "image"}

	if !shouldAutoRetryMediaFailure(job) {
		t.Fatalf("expected first image failure to auto retry")
	}
}

func TestShouldAutoRetryMediaFailureStopsAfterOneRetry(t *testing.T) {
	job := &domain.AIJob{
		JobType: "video",
		OutputPayload: buildMediaAutoRetryPayload(&domain.AIJob{
			JobType:   "video",
			ModelName: "veo-3.1-fast-fl",
		}, "first failure", 1),
	}

	if shouldAutoRetryMediaFailure(job) {
		t.Fatalf("expected auto retry to stop after one retry")
	}
}

func TestBuildMediaAutoRetryPayloadResetsVideoExecutionState(t *testing.T) {
	job := &domain.AIJob{
		JobType:   "video",
		ModelName: "veo-3.1-fast-fl",
		OutputPayload: mustJSON(map[string]any{
			"video": map[string]any{
				"id":     "video_123",
				"status": "failed",
			},
		}),
	}

	raw := buildMediaAutoRetryPayload(job, "provider request failed with status 500", 1)

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unexpected payload json error: %v", err)
	}
	if _, exists := payload["video"]; exists {
		t.Fatalf("expected video execution state to be cleared before retry")
	}
	execution, _ := payload["execution"].(map[string]any)
	if got := int(execution["autoRetryCount"].(float64)); got != 1 {
		t.Fatalf("expected autoRetryCount=1, got %d", got)
	}
	if got := payload["kind"]; got != "video" {
		t.Fatalf("expected kind=video, got %#v", got)
	}
}
