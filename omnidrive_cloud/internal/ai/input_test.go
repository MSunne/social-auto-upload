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

func TestBuildChatRequestRaisesMaxTokensForLongFormGeminiPrompt(t *testing.T) {
	job := &domain.AIJob{
		ModelName: "gemini-3.1-pro-preview",
		Prompt:    stringPtrForTest("请详细分析商业模式，并按月份做表，给我完整的收入测算和分成 breakdown。"),
		InputPayload: mustJSONForTest(map[string]any{
			"temperature": 0.5,
			"maxTokens":   1800,
		}),
	}

	req, err := BuildChatRequest(job)
	if err != nil {
		t.Fatalf("BuildChatRequest returned error: %v", err)
	}
	if req.MaxTokens == nil {
		t.Fatal("expected maxTokens to be normalized")
	}
	if *req.MaxTokens < reasoningLongFormChatMaxTokens {
		t.Fatalf("expected maxTokens >= %d, got %d", reasoningLongFormChatMaxTokens, *req.MaxTokens)
	}
}

func TestBuildChatRequestKeepsHigherExplicitMaxTokens(t *testing.T) {
	job := &domain.AIJob{
		ModelName: "claude-opus-4-1-preview",
		Prompt:    stringPtrForTest("请详细分析并做表。"),
		InputPayload: mustJSONForTest(map[string]any{
			"maxTokens": 8192,
		}),
	}

	req, err := BuildChatRequest(job)
	if err != nil {
		t.Fatalf("BuildChatRequest returned error: %v", err)
	}
	if req.MaxTokens == nil || *req.MaxTokens != 8192 {
		t.Fatalf("expected explicit maxTokens to be preserved, got %#v", req.MaxTokens)
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

func TestBuildVideoRequestKeepsSoraModelNameFromBackendConfig(t *testing.T) {
	job := &domain.AIJob{
		ModelName: "sora-2",
		Prompt:    stringPtrForTest("生成珠宝广告视频"),
		InputPayload: mustJSONForTest(map[string]any{
			"aspectRatio": "16:9",
			"referenceImages": []any{
				map[string]any{
					"url":      "https://example.com/product.png",
					"fileName": "product.png",
					"mimeType": "image/png",
				},
			},
			"durationSeconds": 10,
		}),
	}

	req, err := BuildVideoRequest(job)
	if err != nil {
		t.Fatalf("BuildVideoRequest returned error: %v", err)
	}
	if req.Model != "sora-2" {
		t.Fatalf("unexpected model %q", req.Model)
	}
	if len(req.ReferenceImages) != 1 {
		t.Fatalf("expected one reference image, got %d", len(req.ReferenceImages))
	}
	if req.DurationSeconds == nil || *req.DurationSeconds != 10 {
		t.Fatalf("expected durationSeconds 10, got %#v", req.DurationSeconds)
	}
}

func TestBuildVideoRequestPreservesOrderedReferenceMedia(t *testing.T) {
	job := &domain.AIJob{
		ModelName: "seedance-2.0",
		Prompt:    stringPtrForTest("生成参考素材驱动的视频"),
		InputPayload: mustJSONForTest(map[string]any{
			"referenceMedia": []any{
				map[string]any{
					"kind":     "image",
					"url":      "https://example.com/shot-1.png",
					"fileName": "shot-1.png",
					"mimeType": "image/png",
				},
				map[string]any{
					"kind":     "video",
					"url":      "https://example.com/ref.mp4",
					"fileName": "ref.mp4",
					"mimeType": "video/mp4",
				},
				map[string]any{
					"kind":     "image",
					"url":      "https://example.com/shot-2.png",
					"fileName": "shot-2.png",
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
	if len(req.ReferenceMedia) != 3 {
		t.Fatalf("expected 3 reference media items, got %d", len(req.ReferenceMedia))
	}
	if req.ReferenceMedia[0].FileName != "shot-1.png" || req.ReferenceMedia[1].FileName != "ref.mp4" || req.ReferenceMedia[2].FileName != "shot-2.png" {
		t.Fatalf("unexpected reference media order: %#v", req.ReferenceMedia)
	}
	if len(req.ReferenceImages) != 2 {
		t.Fatalf("expected 2 compatibility reference images, got %d", len(req.ReferenceImages))
	}
	if req.ReferenceImages[0].FileName != "shot-1.png" || req.ReferenceImages[1].FileName != "shot-2.png" {
		t.Fatalf("unexpected compatibility image order: %#v", req.ReferenceImages)
	}
}

func TestBuildVideoRequestKeepsRawBusinessPromptWithoutAspectRatioOrResolutionLines(t *testing.T) {
	job := &domain.AIJob{
		ModelName: "seedance-2.0",
		Prompt:    stringPtrForTest("图一是账号管理，图二是任务列表。介绍万象引擎。"),
		InputPayload: mustJSONForTest(map[string]any{
			"aspectRatio": "16:9",
			"resolution":  "1280x720",
		}),
	}

	req, err := BuildVideoRequest(job)
	if err != nil {
		t.Fatalf("BuildVideoRequest returned error: %v", err)
	}
	if req.Prompt != "图一是账号管理，图二是任务列表。介绍万象引擎。" {
		t.Fatalf("unexpected video prompt %q", req.Prompt)
	}
	if req.AspectRatio != "16:9" {
		t.Fatalf("expected aspect ratio 16:9, got %q", req.AspectRatio)
	}
	if req.Resolution != "1280x720" {
		t.Fatalf("expected resolution 1280x720, got %q", req.Resolution)
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
