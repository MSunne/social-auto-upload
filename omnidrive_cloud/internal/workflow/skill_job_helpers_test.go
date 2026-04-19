package workflow

import (
	"encoding/json"
	"strings"
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

func TestApplyPublishTargetAccountIDsSetsSingleRootAccountID(t *testing.T) {
	accountID := "acc-1"
	publishTargets, accountIDs := buildPublishTargets([]PublishTarget{{
		AccountID:   &accountID,
		Platform:    "抖音",
		AccountName: "测试账号",
	}})
	if len(publishTargets) != 1 {
		t.Fatalf("expected one publish target, got %d", len(publishTargets))
	}
	if publishTargets[0]["accountId"] != accountID {
		t.Fatalf("expected nested accountId %q, got %#v", accountID, publishTargets[0]["accountId"])
	}

	payload := map[string]any{}
	applyPublishTargetAccountIDs(payload, accountIDs)
	if payload["accountId"] != accountID {
		t.Fatalf("expected root accountId %q, got %#v", accountID, payload["accountId"])
	}
	if _, exists := payload["accountIds"]; exists {
		t.Fatalf("expected single target payload to skip accountIds array")
	}
}

func TestApplyPublishTargetAccountIDsSetsMultipleAccountIDs(t *testing.T) {
	accountID1 := "acc-1"
	accountID2 := "acc-2"
	_, accountIDs := buildPublishTargets([]PublishTarget{
		{
			AccountID:   &accountID1,
			Platform:    "抖音",
			AccountName: "账号一",
		},
		{
			AccountID:   &accountID2,
			Platform:    "小红书",
			AccountName: "账号二",
		},
	})

	payload := map[string]any{}
	applyPublishTargetAccountIDs(payload, accountIDs)
	if _, exists := payload["accountId"]; exists {
		t.Fatalf("expected multiple target payload to skip single accountId")
	}
	rawAccountIDs, ok := payload["accountIds"].([]string)
	if !ok {
		t.Fatalf("expected accountIds slice, got %#v", payload["accountIds"])
	}
	if len(rawAccountIDs) != 2 || rawAccountIDs[0] != accountID1 || rawAccountIDs[1] != accountID2 {
		t.Fatalf("unexpected accountIds: %#v", rawAccountIDs)
	}
}

func TestBuildDigitalHumanSkillAIJobPayloadIncludesRootAccountIDAndForcesCustomize(t *testing.T) {
	accountID := "acc-dh-1"
	rawConfig, err := json.Marshal(map[string]any{
		"digitalHuman": map[string]any{
			"mode":       "digital",
			"goodsTitle": "遗留商品标题",
			"goodsText":  "测试商品文案",
		},
	})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	now := time.Now().UTC()
	raw, err := buildDigitalHumanSkillAIJobPayload(
		domain.ProductSkill{
			Name:                "数字人口播计划",
			Description:         "测试数字人口播",
			OutputType:          "数字人口播",
			PublishIntroEnabled: true,
			Topics:              []string{"口播"},
			ReferencePayload:    rawConfig,
		},
		[]domain.ProductSkillAsset{
			{ID: "char-1", AssetType: skillAssetCharacterImage, FileName: "character.png", CreatedAt: now},
			{ID: "goods-1", AssetType: skillAssetGoodsImage, FileName: "goods.png", CreatedAt: now.Add(time.Second)},
			{ID: "audio-1", AssetType: skillAssetRefAudio, FileName: "voice.wav", CreatedAt: now.Add(2 * time.Second)},
		},
		now,
		now.Add(30*time.Minute),
		[]PublishTarget{{
			AccountID:   &accountID,
			Platform:    "抖音",
			AccountName: "数字人账号",
		}},
	)
	if err != nil {
		t.Fatalf("buildDigitalHumanSkillAIJobPayload returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["accountId"] != accountID {
		t.Fatalf("expected root accountId %q, got %#v", accountID, payload["accountId"])
	}
	digitalHumanConfig, ok := payload["digitalHumanConfig"].(map[string]any)
	if !ok {
		t.Fatalf("expected digitalHumanConfig object, got %#v", payload["digitalHumanConfig"])
	}
	if digitalHumanConfig["mode"] != "customize" {
		t.Fatalf("expected customize mode, got %#v", digitalHumanConfig["mode"])
	}
	if digitalHumanConfig["goodsTitle"] != "" {
		t.Fatalf("expected goodsTitle to be stripped, got %#v", digitalHumanConfig["goodsTitle"])
	}
	if _, exists := digitalHumanConfig["goodsAsset"]; exists {
		t.Fatalf("expected goodsAsset to be omitted for customize mode")
	}
	publishPayload, ok := payload["publishPayload"].(map[string]any)
	if !ok {
		t.Fatalf("expected publishPayload, got %#v", payload["publishPayload"])
	}
	targets, ok := publishPayload["targets"].([]any)
	if !ok || len(targets) != 1 {
		t.Fatalf("expected one publish target, got %#v", publishPayload["targets"])
	}
	firstTarget, ok := targets[0].(map[string]any)
	if !ok {
		t.Fatalf("expected target object, got %#v", targets[0])
	}
	if firstTarget["accountId"] != accountID {
		t.Fatalf("expected nested accountId %q, got %#v", accountID, firstTarget["accountId"])
	}
}

func TestResolveMixVideoSkillConfigDefaults(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"mixVideo": map[string]any{
			"publishTemplate": "平台简介基础模板",
		},
	})
	if err != nil {
		t.Fatalf("marshal reference payload: %v", err)
	}

	config := ResolveMixVideoSkillConfig(domain.ProductSkill{
		ReferencePayload: raw,
	})
	if !config.ScriptRewriteEnabled {
		t.Fatal("expected script rewrite to default enabled")
	}
	if config.PublishTemplate != "平台简介基础模板" {
		t.Fatalf("PublishTemplate = %q, want %q", config.PublishTemplate, "平台简介基础模板")
	}
}

func TestCollectMixVideoSkillAssetsRequiresSingleRefAudio(t *testing.T) {
	now := time.Now().UTC()
	sourceAssets, refAudio, err := CollectMixVideoSkillAssets([]domain.ProductSkillAsset{
		{ID: "video-1", AssetType: skillAssetMixVideoSourceVideo, FileName: "a.mp4", CreatedAt: now},
		{ID: "video-2", AssetType: skillAssetMixVideoSourceVideo, FileName: "b.mp4", CreatedAt: now.Add(time.Second)},
		{ID: "audio-1", AssetType: skillAssetMixVideoRefAudio, FileName: "voice.m4a", CreatedAt: now.Add(2 * time.Second)},
	})
	if err != nil {
		t.Fatalf("CollectMixVideoSkillAssets returned error: %v", err)
	}
	if len(sourceAssets) != 2 {
		t.Fatalf("expected 2 source assets, got %d", len(sourceAssets))
	}
	if refAudio == nil || refAudio.ID != "audio-1" {
		t.Fatalf("unexpected refAudio: %#v", refAudio)
	}

	_, _, err = CollectMixVideoSkillAssets([]domain.ProductSkillAsset{
		{ID: "video-1", AssetType: skillAssetMixVideoSourceVideo, FileName: "a.mp4", CreatedAt: now},
	})
	if err == nil || !strings.Contains(err.Error(), "reference audio") {
		t.Fatalf("expected missing reference audio error, got %v", err)
	}
}

func TestBuildMixVideoRewritePrompts(t *testing.T) {
	scriptPrompt := BuildMixVideoScriptRewriteInput("全局脚本提示", "技能脚本提示", "请写一个混剪脚本")
	if !strings.Contains(scriptPrompt, "全局脚本提示") {
		t.Fatalf("expected admin script prompt in %q", scriptPrompt)
	}
	if !strings.Contains(scriptPrompt, "技能脚本提示") {
		t.Fatalf("expected skill script prompt in %q", scriptPrompt)
	}
	if !strings.Contains(scriptPrompt, "请写一个混剪脚本") {
		t.Fatalf("expected script template in %q", scriptPrompt)
	}

	publishPrompt := BuildMixVideoPublishIntroRewriteInput("全局简介提示", "技能简介提示", "平台简介模板", "最终脚本")
	for _, expected := range []string{"全局简介提示", "技能简介提示", "平台简介模板", "最终脚本"} {
		if !strings.Contains(publishPrompt, expected) {
			t.Fatalf("expected %q in %q", expected, publishPrompt)
		}
	}
}

func TestMapSkillOutputTypeToJobTypeReturnsDigitalHuman(t *testing.T) {
	cases := []string{"数字人口播", "真人口播", "真人视频", "digital_human"}
	for _, outputType := range cases {
		jobType, ok := MapSkillOutputTypeToJobType(outputType)
		if !ok {
			t.Fatalf("expected %q to be supported", outputType)
		}
		if jobType != "digital_human" {
			t.Fatalf("expected %q to map to digital_human job type, got %q", outputType, jobType)
		}
	}
}

func TestNormalizeSkillOutputTypeLegacyRealVideoAliases(t *testing.T) {
	if normalized := NormalizeSkillOutputType("真人口播"); normalized != "数字人口播" {
		t.Fatalf("expected 真人口播 to normalize to 数字人口播, got %q", normalized)
	}
	if normalized := NormalizeSkillOutputType("真人视频"); normalized != "数字人口播" {
		t.Fatalf("expected 真人视频 to normalize to 数字人口播, got %q", normalized)
	}
}

func TestMapSkillOutputTypeToJobTypeReturnsMixVideo(t *testing.T) {
	jobType, ok := MapSkillOutputTypeToJobType("混剪")
	if !ok {
		t.Fatal(`expected "混剪" to be supported`)
	}
	if jobType != "mix_video" {
		t.Fatalf(`expected "混剪" to map to "mix_video", got %q`, jobType)
	}
}

func TestNormalizeSkillOutputTypePreservesMixVideo(t *testing.T) {
	if normalized := NormalizeSkillOutputType("混剪"); normalized != "混剪" {
		t.Fatalf(`expected "混剪" to remain "混剪", got %q`, normalized)
	}
}

func testStringPtr(value string) *string {
	return &value
}
