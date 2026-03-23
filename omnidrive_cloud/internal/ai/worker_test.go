package ai

import (
	"errors"
	"strings"
	"testing"
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
