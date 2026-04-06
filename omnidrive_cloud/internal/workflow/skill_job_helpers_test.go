package workflow

import (
	"encoding/json"
	"testing"
	"time"

	"omnidrive_cloud/internal/domain"
)

func TestDecodeSkillReferenceMediaOrderDeduplicatesValues(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"referenceMediaOrder": []string{"asset-b", "asset-a", "asset-b", ""},
	})
	if err != nil {
		t.Fatalf("marshal reference payload: %v", err)
	}

	got := decodeSkillReferenceMediaOrder(raw)
	if len(got) != 2 || got[0] != "asset-b" || got[1] != "asset-a" {
		t.Fatalf("unexpected order: %#v", got)
	}
}

func TestCollectOrderedSkillReferenceMediaAssetsHonorsReferenceMediaOrder(t *testing.T) {
	referencePayload, err := json.Marshal(map[string]any{
		"referenceMediaOrder": []string{"video-1", "image-2"},
	})
	if err != nil {
		t.Fatalf("marshal reference payload: %v", err)
	}

	now := time.Now().UTC()
	assets := []domain.ProductSkillAsset{
		{
			ID:        "image-1",
			AssetType: "reference_image",
			FileName:  "image-1.png",
			MimeType:  testStringPtr("image/png"),
			CreatedAt: now.Add(1 * time.Minute),
		},
		{
			ID:        "video-1",
			AssetType: "reference_video",
			FileName:  "video-1.mp4",
			MimeType:  testStringPtr("video/mp4"),
			CreatedAt: now.Add(2 * time.Minute),
		},
		{
			ID:        "image-2",
			AssetType: "reference_image",
			FileName:  "image-2.png",
			MimeType:  testStringPtr("image/png"),
			CreatedAt: now.Add(3 * time.Minute),
		},
		{
			ID:        "text-1",
			AssetType: "reference_text",
			FileName:  "notes.txt",
			MimeType:  testStringPtr("text/plain"),
			CreatedAt: now,
		},
	}

	got := collectOrderedSkillReferenceMediaAssets(assets, referencePayload)
	if len(got) != 3 {
		t.Fatalf("expected 3 media assets, got %d", len(got))
	}
	if got[0].ID != "video-1" || got[1].ID != "image-2" || got[2].ID != "image-1" {
		t.Fatalf("unexpected media order: %#v", []string{got[0].ID, got[1].ID, got[2].ID})
	}
}

func TestBuildSkillReferenceMediaMarksVideoKind(t *testing.T) {
	asset := domain.ProductSkillAsset{
		ID:        "video-1",
		AssetType: "reference_video",
		FileName:  "video-1.mp4",
		MimeType:  testStringPtr("video/mp4"),
	}

	got := buildSkillReferenceMedia(asset)
	if got["kind"] != "video" {
		t.Fatalf("expected video kind, got %#v", got["kind"])
	}
}

func testStringPtr(value string) *string {
	return &value
}
