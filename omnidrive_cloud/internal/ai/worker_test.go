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

	if !shouldAutoRetryMediaFailure(job, errors.New("provider request failed with status 503")) {
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

	if shouldAutoRetryMediaFailure(job, errors.New("provider request failed with status 503")) {
		t.Fatalf("expected auto retry to stop after one retry")
	}
}

func TestShouldAutoRetryMediaFailureRejectsPolicyViolation(t *testing.T) {
	job := &domain.AIJob{JobType: "video"}

	if shouldAutoRetryMediaFailure(job, errors.New("提交中含有违反平台政策的内容，请你立即停止或调整你的提交内容")) {
		t.Fatalf("expected policy violation to stop auto retry")
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

func TestBuildVideoOutputPayloadPreservesExecutionState(t *testing.T) {
	job := &domain.AIJob{
		JobType:   "video",
		ModelName: "veo-3.1-fast-fl",
		OutputPayload: mustJSON(map[string]any{
			"execution": map[string]any{
				"autoRetryCount": 1,
			},
		}),
	}

	raw := buildVideoOutputPayload(job, videoExecutionState{
		BaseURL:       "https://example.com",
		RemoteVideoID: "video_123",
		RemoteStatus:  "processing",
	}, nil)

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unexpected payload json error: %v", err)
	}
	execution, _ := payload["execution"].(map[string]any)
	if got := int(execution["autoRetryCount"].(float64)); got != 1 {
		t.Fatalf("expected autoRetryCount=1 to be preserved, got %d", got)
	}
	videoPayload, _ := payload["video"].(map[string]any)
	if got := videoPayload["id"]; got != "video_123" {
		t.Fatalf("expected remote video id to be preserved, got %#v", got)
	}
}
