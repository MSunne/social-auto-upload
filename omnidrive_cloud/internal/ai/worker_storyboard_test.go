package ai

import (
	"testing"

	"omnidrive_cloud/internal/domain"
)

func TestResolveChatPromptFallsBackToLatestUserMessage(t *testing.T) {
	job := &domain.AIJob{}
	messages := []ChatMessage{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "ack"},
		{Role: "user", Content: "final prompt"},
	}

	got := resolveChatPrompt(job, messages)

	if got != "final prompt" {
		t.Fatalf("resolveChatPrompt() = %q, want %q", got, "final prompt")
	}
}

func TestReplaceLastUserMessageRewritesLatestUserPrompt(t *testing.T) {
	messages := []ChatMessage{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "old"},
		{Role: "assistant", Content: "ack"},
		{Role: "user", Content: "latest"},
	}

	got := replaceLastUserMessage(messages, "optimized")

	if len(got) != 4 {
		t.Fatalf("replaceLastUserMessage() len = %d, want 4", len(got))
	}
	if got[3].Content != "optimized" {
		t.Fatalf("replaceLastUserMessage() latest content = %#v, want %#v", got[3].Content, "optimized")
	}
	if got[1].Content != "old" {
		t.Fatalf("replaceLastUserMessage() should keep earlier user message, got %#v", got[1].Content)
	}
}
