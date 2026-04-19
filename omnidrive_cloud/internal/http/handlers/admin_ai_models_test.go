package handlers

import (
	"testing"

	aiclient "omnidrive_cloud/internal/ai"
)

func TestNormalizeCreateAIModelPayloadUsesTerminalBaseURLProtocol(t *testing.T) {
	baseURL := "https://api.apiyi.com/v1/chat/completions"
	input, err := normalizeCreateAIModelPayload(adminCreateAIModelRequest{
		Vendor:       "万象易",
		ModelName:    "claude-opus-4-6-thinking",
		ModelAlias:   "claude-opus-4-6-thinking",
		Category:     "chat",
		BillingMode:  "per_token",
		ChatProtocol: aiclient.ChatProtocolAnthropicMessages,
		BaseURL:      &baseURL,
		IsEnabled:    true,
	})
	if err != nil {
		t.Fatalf("normalizeCreateAIModelPayload returned error: %v", err)
	}
	if input.ChatProtocol != aiclient.ChatProtocolOpenAIChatCompletions {
		t.Fatalf("expected create payload to persist openai protocol, got %q", input.ChatProtocol)
	}
}

func TestNormalizeUpdateAIModelPayloadUsesTerminalBaseURLProtocol(t *testing.T) {
	baseURL := "https://api.apiyi.com/v1/messages"
	input, err := normalizeUpdateAIModelPayload(adminUpdateAIModelRequest{
		BaseURL: &baseURL,
	}, "chat")
	if err != nil {
		t.Fatalf("normalizeUpdateAIModelPayload returned error: %v", err)
	}
	if input.ChatProtocol == nil {
		t.Fatal("expected update payload to set chat protocol")
	}
	if *input.ChatProtocol != aiclient.ChatProtocolAnthropicMessages {
		t.Fatalf("expected update payload to persist anthropic protocol, got %q", *input.ChatProtocol)
	}
}
