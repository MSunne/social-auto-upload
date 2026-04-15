package handlers

import (
	"encoding/json"
	"testing"

	"omnidrive_cloud/internal/domain"
)

func TestBuildDigitalHumanSubmissionEvidenceUsesSubmissionRequest(t *testing.T) {
	remoteTaskID := "remote-123"
	requestPayload, err := json.Marshal(map[string]any{
		"sourceRequest": map[string]any{
			"mode":      "customize",
			"goodsText": "旧文案",
		},
		"submissionRequest": map[string]any{
			"character_asset_path": "/tmp/character.jpg",
			"mode":                 "customize",
			"goods_text":           "最终文案",
			"ref_audio":            "/tmp/audio.m4a",
		},
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	evidence := buildDigitalHumanSubmissionEvidence(&domain.DigitalHumanTask{
		RequestPayload: requestPayload,
		RemoteTaskID:   &remoteTaskID,
	})
	if evidence == nil {
		t.Fatal("expected submission evidence")
	}
	if evidence.Mode != "customize" {
		t.Fatalf("expected customize mode, got %q", evidence.Mode)
	}
	if evidence.HasGoodsAssetPath {
		t.Fatal("expected customize evidence to omit goods asset path")
	}
	if evidence.GoodsTitle != nil {
		t.Fatalf("expected customize evidence to omit goodsTitle, got %#v", evidence.GoodsTitle)
	}
	if evidence.RemoteTaskID == nil || *evidence.RemoteTaskID != remoteTaskID {
		t.Fatalf("unexpected remoteTaskId %#v", evidence.RemoteTaskID)
	}
}

func TestBuildDigitalHumanSubmissionEvidenceTracksDigitalGoodsFields(t *testing.T) {
	requestPayload, err := json.Marshal(map[string]any{
		"submissionRequest": map[string]any{
			"character_asset_path": "/tmp/character.jpg",
			"mode":                 "digital",
			"goods_asset_path":     "/tmp/goods.jpg",
			"goods_text":           "带货文案",
			"goods_title":          "商品标题",
		},
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	evidence := buildDigitalHumanSubmissionEvidence(&domain.DigitalHumanTask{RequestPayload: requestPayload})
	if evidence == nil {
		t.Fatal("expected submission evidence")
	}
	if evidence.Mode != "digital" {
		t.Fatalf("expected digital mode, got %q", evidence.Mode)
	}
	if !evidence.HasGoodsAssetPath {
		t.Fatal("expected goods asset path evidence")
	}
	if evidence.GoodsTitle == nil || *evidence.GoodsTitle != "商品标题" {
		t.Fatalf("unexpected goodsTitle %#v", evidence.GoodsTitle)
	}
}
