package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	aiclient "omnidrive_cloud/internal/ai"
	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/config"
)

func TestStripChatAttachmentDraftsRemovesAttachments(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"prompt": "你好",
		"messages": []map[string]any{
			{"role": "user", "content": "你好"},
		},
		"attachments": []map[string]any{
			{
				"fileName": "demo.txt",
				"dataUrl":  "data:text/plain;base64,SGVsbG8=",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	sanitized := stripChatAttachmentDrafts(raw)

	var payload map[string]any
	if err := json.Unmarshal(sanitized, &payload); err != nil {
		t.Fatalf("unmarshal sanitized payload: %v", err)
	}
	if _, exists := payload["attachments"]; exists {
		t.Fatalf("expected attachments to be removed, got %#v", payload["attachments"])
	}
	if payload["prompt"] != "你好" {
		t.Fatalf("expected prompt to be preserved, got %#v", payload["prompt"])
	}
}

func TestBuildChatAttachmentPromptParts(t *testing.T) {
	imageParts := buildChatAttachmentPromptParts(persistedChatAttachment{
		FileName:  "product.png",
		MimeType:  "image/png",
		PublicURL: "https://example.com/product.png",
		Kind:      "image",
	})
	if len(imageParts) != 2 {
		t.Fatalf("expected 2 parts for image attachment, got %d", len(imageParts))
	}
	if imageParts[0]["type"] != "text" {
		t.Fatalf("expected image attachment label part, got %#v", imageParts[0])
	}
	if imageParts[1]["type"] != "image_url" {
		t.Fatalf("expected image attachment image_url part, got %#v", imageParts[1])
	}

	textContent := "珠宝主打红宝石和金色材质。"
	textParts := buildChatAttachmentPromptParts(persistedChatAttachment{
		FileName:    "brief.txt",
		MimeType:    "text/plain",
		Kind:        "text",
		TextContent: &textContent,
	})
	if len(textParts) != 1 {
		t.Fatalf("expected 1 part for text attachment, got %d", len(textParts))
	}
	if textParts[0]["type"] != "text" {
		t.Fatalf("expected text attachment text part, got %#v", textParts[0])
	}
}

func TestDetectChatAttachmentKind(t *testing.T) {
	if got := detectChatAttachmentKind("image/png", "demo.png"); got != "image" {
		t.Fatalf("expected image kind, got %q", got)
	}
	if got := detectChatAttachmentKind("application/json", "demo.json"); got != "text" {
		t.Fatalf("expected text kind for json, got %q", got)
	}
	if got := detectChatAttachmentKind("application/pdf", "demo.pdf"); got != "file" {
		t.Fatalf("expected file kind for pdf, got %q", got)
	}
}

func TestNormalizeStreamChatProviderError(t *testing.T) {
	if got := normalizeStreamChatProviderError(context.Canceled); got != "聊天连接意外中断，请稍后重试。" {
		t.Fatalf("expected canceled error to be normalized, got %q", got)
	}
	if got := normalizeStreamChatProviderError(context.DeadlineExceeded); got != "模型响应超时，请稍后重试。" {
		t.Fatalf("expected deadline exceeded error to be normalized, got %q", got)
	}
	if got := normalizeStreamChatProviderError(nil); got != "" {
		t.Fatalf("expected nil error to normalize to empty string, got %q", got)
	}
}

func TestStreamChatGenerationTimeoutUsesConfig(t *testing.T) {
	handler := &AIHandler{
		app: &appstate.App{
			Config: config.Config{
				AIChatStreamTimeoutSeconds: 42,
			},
		},
	}
	if got := handler.streamChatGenerationTimeout(); got != 42*time.Second {
		t.Fatalf("expected chat stream timeout to follow config, got %s", got)
	}
}

func TestStreamChatHeartbeatIntervalDefaults(t *testing.T) {
	handler := &AIHandler{}
	if got := handler.streamChatHeartbeatInterval(); got != defaultStreamChatHeartbeatInterval {
		t.Fatalf("expected default heartbeat interval %s, got %s", defaultStreamChatHeartbeatInterval, got)
	}
}

func TestStartStreamChatHeartbeatEmitsPeriodically(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emits := make(chan struct{}, 4)
	stop := startStreamChatHeartbeat(ctx, 10*time.Millisecond, func() error {
		emits <- struct{}{}
		return nil
	}, nil)
	defer stop()

	for i := 0; i < 2; i++ {
		select {
		case <-emits:
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("expected heartbeat emit %d", i+1)
		}
	}
}

func TestClassifyStreamChatTermination(t *testing.T) {
	if got := classifyStreamChatTermination(nil, context.DeadlineExceeded, false); got != "timeout" {
		t.Fatalf("expected timeout classification, got %q", got)
	}
	if got := classifyStreamChatTermination(&aiclient.ChatResult{FinishReason: "stream_eof"}, nil, false); got != "eof_after_delta" {
		t.Fatalf("expected eof_after_delta classification, got %q", got)
	}
	if got := classifyStreamChatTermination(&aiclient.ChatResult{FinishReason: "stop"}, nil, false); got != "finish_reason" {
		t.Fatalf("expected finish_reason classification, got %q", got)
	}
	if got := classifyStreamChatTermination(nil, nil, true); got != "write_error" {
		t.Fatalf("expected write_error classification, got %q", got)
	}
}
