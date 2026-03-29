package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"omnidrive_cloud/internal/domain"
)

func TestShouldPrepareSkillVideoReferenceFrames(t *testing.T) {
	videoJob := &domain.AIJob{JobType: "video", Source: "account_skill_binding"}
	if !shouldPrepareSkillVideoReferenceFrames(videoJob, map[string]any{}) {
		t.Fatalf("expected account skill video job to enable frame redesign")
	}

	openClawSkillJob := &domain.AIJob{JobType: "video", Source: "openclaw_skill"}
	if shouldPrepareSkillVideoReferenceFrames(openClawSkillJob, map[string]any{}) {
		t.Fatalf("expected openclaw video job without skillId to skip frame redesign")
	}
	openClawSkillJob.SkillID = stringPtr("skill-1")
	if !shouldPrepareSkillVideoReferenceFrames(openClawSkillJob, map[string]any{}) {
		t.Fatalf("expected openclaw video job with skillId to enable frame redesign")
	}

	imageJob := &domain.AIJob{JobType: "image", Source: "account_skill_binding"}
	if shouldPrepareSkillVideoReferenceFrames(imageJob, map[string]any{}) {
		t.Fatalf("expected non-video job to skip frame redesign")
	}
}

func TestBuildSkillVideoFramePromptContainsProductGuardrails(t *testing.T) {
	prompt := buildSkillVideoFramePrompt(
		&domain.AIJob{JobType: "video"},
		map[string]any{
			"skillName":        "玩具车推广",
			"skillDescription": "强调遥控、越野、防摔卖点",
		},
		"让产品在户外越野场景中有强烈速度感",
		[]map[string]string{{
			"fileName": "卖点.txt",
			"content":  "主打越野、防摔、儿童礼物场景",
		}},
		"first",
		1,
	)

	requiredSnippets := []string{
		"主体必须是客户的主要产品",
		"不要直接复用客户当前固定首帧或固定构图",
		"不能偏离产品特性、卖点",
		"来源: 技能中心视文模式",
		"玩具车推广",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("expected prompt to contain %q, got %q", snippet, prompt)
		}
	}
}

func TestVideoExecutionStateRoundTripsReferenceFrames(t *testing.T) {
	raw := buildVideoOutputPayload(&domain.AIJob{ModelName: "veo-3.1-fast-fl"}, videoExecutionState{
		BaseURL:       "https://api.apiyi.com",
		RemoteVideoID: "video-1",
		RemoteStatus:  "queued",
		ReferenceFrames: []map[string]any{
			{
				"role":      "first",
				"publicUrl": "https://example.com/first.png",
				"fileName":  "video-first-frame.png",
				"mimeType":  "image/png",
			},
		},
		FrameRedesign: map[string]any{
			"enabled": true,
			"status":  "generated",
		},
	}, nil)

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if _, ok := payload["videoReferenceFrames"]; !ok {
		t.Fatalf("expected videoReferenceFrames in payload")
	}
	if _, ok := payload["videoFrameRedesign"]; !ok {
		t.Fatalf("expected videoFrameRedesign in payload")
	}

	state := parseVideoExecutionState(raw)
	if len(state.ReferenceFrames) != 1 {
		t.Fatalf("expected 1 reference frame, got %d", len(state.ReferenceFrames))
	}
	if stringValue(state.ReferenceFrames[0]["publicUrl"]) != "https://example.com/first.png" {
		t.Fatalf("unexpected reference frame URL: %#v", state.ReferenceFrames[0])
	}
	if stringValue(state.FrameRedesign["status"]) != "generated" {
		t.Fatalf("unexpected frame redesign state: %#v", state.FrameRedesign)
	}
}
