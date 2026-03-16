package ai

import (
	"encoding/json"
	"testing"

	"omnidrive_cloud/internal/domain"
)

func TestBuildChatRequestFromPromptAndSystemPrompt(t *testing.T) {
	job := &domain.AIJob{
		ModelName: "gemini-3.1-pro-preview",
		Prompt:    stringPtrForTest("帮我生成 3 条标题"),
		InputPayload: mustJSONForTest(map[string]any{
			"systemPrompt": "你是一个短视频运营顾问",
			"temperature":  0.7,
			"maxTokens":    512,
		}),
	}

	req, err := BuildChatRequest(job)
	if err != nil {
		t.Fatalf("BuildChatRequest returned error: %v", err)
	}
	if req.Model != "gemini-3.1-pro-preview" {
		t.Fatalf("unexpected model %q", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
		t.Fatalf("unexpected message roles: %#v", req.Messages)
	}
	if req.Temperature == nil || *req.Temperature != 0.7 {
		t.Fatalf("expected temperature 0.7, got %#v", req.Temperature)
	}
	if req.MaxTokens == nil || *req.MaxTokens != 512 {
		t.Fatalf("expected maxTokens 512, got %#v", req.MaxTokens)
	}
}

func TestBuildVideoRequestAddsLandscapeAndKeepsFLWhenReferenceImagesExist(t *testing.T) {
	job := &domain.AIJob{
		ModelName: "veo-3.1-fast-fl",
		Prompt:    stringPtrForTest("生成展示视频"),
		InputPayload: mustJSONForTest(map[string]any{
			"aspectRatio": "16:9",
			"referenceImages": []any{
				map[string]any{
					"url":      "https://example.com/product.png",
					"fileName": "product.png",
					"mimeType": "image/png",
				},
			},
			"durationSeconds": 8,
		}),
	}

	req, err := BuildVideoRequest(job)
	if err != nil {
		t.Fatalf("BuildVideoRequest returned error: %v", err)
	}
	if req.Model != "veo-3.1-landscape-fast-fl" {
		t.Fatalf("unexpected model %q", req.Model)
	}
	if len(req.ReferenceImages) != 1 {
		t.Fatalf("expected one reference image, got %d", len(req.ReferenceImages))
	}
	if req.DurationSeconds == nil || *req.DurationSeconds != 8 {
		t.Fatalf("expected durationSeconds 8, got %#v", req.DurationSeconds)
	}
}

func TestBuildVideoRequestRemovesFLWithoutReferenceImages(t *testing.T) {
	job := &domain.AIJob{
		ModelName: "veo-3.1-fast-fl",
		Prompt:    stringPtrForTest("生成展示视频"),
		InputPayload: mustJSONForTest(map[string]any{
			"aspectRatio": "9:16",
		}),
	}

	req, err := BuildVideoRequest(job)
	if err != nil {
		t.Fatalf("BuildVideoRequest returned error: %v", err)
	}
	if req.Model != "veo-3.1-fast" {
		t.Fatalf("unexpected model %q", req.Model)
	}
	if len(req.ReferenceImages) != 0 {
		t.Fatalf("expected no reference images, got %d", len(req.ReferenceImages))
	}
}

func mustJSONForTest(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func stringPtrForTest(value string) *string {
	return &value
}
