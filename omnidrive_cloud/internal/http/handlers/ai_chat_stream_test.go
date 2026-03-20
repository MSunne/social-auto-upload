package handlers

import (
	"encoding/json"
	"testing"
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
