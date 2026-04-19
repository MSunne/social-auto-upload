package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
)

type PublishTarget struct {
	AccountID   *string
	Platform    string
	AccountName string
}

func buildPublishTargets(targets []PublishTarget) ([]map[string]any, []string) {
	publishTargets := make([]map[string]any, 0, len(targets))
	accountIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		item := map[string]any{
			"platform":    strings.TrimSpace(target.Platform),
			"accountName": strings.TrimSpace(target.AccountName),
		}
		if target.AccountID != nil {
			if accountID := strings.TrimSpace(*target.AccountID); accountID != "" {
				item["accountId"] = accountID
				accountIDs = append(accountIDs, accountID)
			}
		}
		publishTargets = append(publishTargets, item)
	}
	return publishTargets, accountIDs
}

func applyPublishTargetAccountIDs(payload map[string]any, accountIDs []string) {
	switch len(accountIDs) {
	case 0:
		return
	case 1:
		payload["accountId"] = accountIDs[0]
	default:
		payload["accountIds"] = accountIDs
	}
}

const (
	defaultSkillVideoAspectRatio     = "16:9"
	defaultSkillVideoResolution      = "1280x720"
	defaultSkillVideoDurationSeconds = 8
	defaultSkillVideoSubtitleRule    = "默认不要字幕"
	defaultSkillStoryboardPrompt     = "你是内容创作分镜与脚本优化助手。请结合用户目标、参考图片和参考文本，输出适合继续交给图片、视频或文本模型执行的精炼脚本。输出中需要保留主体、场景、镜头、风格、文案和节奏等关键信息。"
	skillOutputDigitalHuman          = "数字人口播"
	skillOutputMixVideo              = "混剪"
	skillOutputRealVideo             = "真人视频"
	skillOutputRealSpeech            = "真人口播"
	skillAssetCharacterImage         = "digital_human_character_image"
	skillAssetGoodsImage             = "digital_human_goods_image"
	skillAssetRefAudio               = "digital_human_ref_audio"
	skillAssetMixVideoSourceVideo    = "mix_video_source_video"
	skillAssetMixVideoRefAudio       = "mix_video_ref_audio"
)

type mixVideoSkillConfig struct {
	ScriptRewriteEnabled bool
	PublishTemplate      string
}

type skillVideoGenerationOptions struct {
	AspectRatio     string
	Resolution      string
	DurationSeconds *int
}

// 计算计划中的技能生成时间，供技能作业helpers链路复用派生的时间结果。
func ScheduledSkillGenerationTime(publishAt time.Time) time.Time {
	return publishAt.UTC().Add(-30 * time.Minute)
}

// 构建技能作业提示词，为技能作业helpers生成后续步骤所需的派生参数或载荷。
func BuildSkillJobPrompt(skill domain.ProductSkill, jobType string) string {
	prompt := strings.TrimSpace(optionalStringValue(skill.PromptTemplate))
	if prompt == "" {
		prompt = strings.TrimSpace(skill.Description)
	}
	if prompt == "" {
		prompt = strings.TrimSpace(skill.Name)
	}
	if strings.EqualFold(strings.TrimSpace(jobType), "video") {
		prompt = appendUniqueVideoSubtitleRule(prompt)
	}
	return prompt
}

// 构建技能发布提示词Template，为技能作业helpers生成后续步骤所需的派生参数或载荷。
func BuildSkillPublishPromptTemplate(skill domain.ProductSkill) string {
	if !skill.PublishIntroEnabled {
		return ""
	}
	return strings.TrimSpace(optionalStringValue(skill.PublishPromptTemplate))
}

// 处理默认技能分镜提示词Template相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func DefaultSkillStoryboardPromptTemplate(outputType string) string {
	return strings.TrimSpace(defaultSkillStoryboardPrompt)
}

// 解析技能分镜提示词Template，根据当前配置和上下文确定最终使用结果。
func ResolveSkillStoryboardPromptTemplate(skill domain.ProductSkill, jobType string) string {
	if prompt := strings.TrimSpace(optionalStringValue(skill.StoryboardPromptTemplate)); prompt != "" {
		return prompt
	}
	if prompt := strings.TrimSpace(DefaultSkillStoryboardPromptTemplate(skill.OutputType)); prompt != "" {
		return prompt
	}
	return strings.TrimSpace(defaultSkillStoryboardPrompt)
}

// 映射技能输出Type作业Type，把外部配置转换为当前业务可识别的表示。
func MapSkillOutputTypeToJobType(outputType string) (string, bool) {
	switch NormalizeSkillOutputType(outputType) {
	case "image", "image_text", "图文模式":
		return "image", true
	case "video", "video_text", "视文模式":
		return "video", true
	case skillOutputDigitalHuman:
		return "digital_human", true
	case "mix_video", skillOutputMixVideo:
		return "mix_video", true
	case "chat", "text", "text_only", "文本格式":
		return "chat", true
	default:
		return "", false
	}
}

func IsDigitalHumanSkillOutput(outputType string) bool {
	return NormalizeSkillOutputType(outputType) == skillOutputDigitalHuman
}

func IsMixVideoSkillOutput(outputType string) bool {
	return NormalizeSkillOutputType(outputType) == skillOutputMixVideo
}

// 规范化技能输出类型，兼容历史别名并确保后续新写入值保持一致。
func NormalizeSkillOutputType(outputType string) string {
	switch strings.TrimSpace(outputType) {
	case "image", "image_text", "图文模式":
		return "图文模式"
	case "video", "video_text", "视文模式":
		return "视文模式"
	case "chat", "text", "text_only", "文本格式":
		return "文本格式"
	case "digital_human", skillOutputDigitalHuman, skillOutputRealVideo, skillOutputRealSpeech:
		return skillOutputDigitalHuman
	case "mix_video", skillOutputMixVideo:
		return skillOutputMixVideo
	default:
		return strings.TrimSpace(outputType)
	}
}

// 根据Auto发布计算账号Allowed，供技能作业helpers链路复用关键派生结果。
func AccountAllowedForAutoPublish(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "invalid", "disabled", "deleted":
		return false
	default:
		return true
	}
}

// 构建技能AI作业载荷，为技能作业helpers生成后续步骤所需的派生参数或载荷。
func BuildSkillAIJobPayload(
	ctx context.Context,
	app *appstate.App,
	skill domain.ProductSkill,
	model *domain.AIModel,
	generateAt time.Time,
	publishAt time.Time,
	jobType string,
	targets []PublishTarget,
) ([]byte, error) {
	if app == nil || app.Store == nil {
		return nil, fmt.Errorf("app store is required")
	}

	assets, err := app.Store.ListSkillAssets(ctx, skill.ID, skill.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if IsDigitalHumanSkillOutput(skill.OutputType) {
		return buildDigitalHumanSkillAIJobPayload(skill, assets, generateAt, publishAt, targets)
	}

	referenceMediaAssets := collectOrderedSkillReferenceMediaAssets(assets, skill.ReferencePayload)
	referenceMedia := make([]map[string]any, 0, len(referenceMediaAssets))
	referenceImages := make([]map[string]any, 0, len(referenceMediaAssets))
	referenceTexts := make([]map[string]any, 0)
	imageCount := 0
	videoCount := 0
	for _, asset := range referenceMediaAssets {
		media := buildSkillReferenceMedia(asset)
		referenceMedia = append(referenceMedia, media)
		if strings.EqualFold(fmt.Sprintf("%v", media["kind"]), "video") {
			videoCount++
		} else {
			imageCount++
			referenceImages = append(referenceImages, media)
		}
	}
	for _, asset := range assets {
		if !isSkillReferenceText(asset) {
			continue
		}
		referenceTexts = append(referenceTexts, map[string]any{
			"fileName":  asset.FileName,
			"mimeType":  optionalStringValue(asset.MimeType),
			"publicUrl": optionalStringValue(asset.PublicURL),
			"content":   readSkillAssetText(ctx, app, asset),
		})
	}

	storyboardPrompt, storyboardModel, storyboardReferences, err := loadSkillStoryboardConfig(ctx, app, skill, jobType)
	if err != nil {
		return nil, err
	}

	prompt := BuildSkillJobPrompt(skill, jobType)
	topics := normalizeSkillTopics(skill.Topics)
	publishPromptTemplate := BuildSkillPublishPromptTemplate(skill)
	payload := map[string]any{
		"prompt":                prompt,
		"skillName":             skill.Name,
		"skillDescription":      skill.Description,
		"publishPromptTemplate": publishPromptTemplate,
		"publishIntroEnabled":   skill.PublishIntroEnabled,
		"storyboardEnabled":     skill.StoryboardEnabled,
		"skillTags":             topics,
		"runAt":                 generateAt.UTC().Format(time.RFC3339),
		"publishAt":             publishAt.UTC().Format(time.RFC3339),
		"referenceMedia":        referenceMedia,
		"referenceImages":       referenceImages,
		"referenceTexts":        referenceTexts,
		"storyboardConfig": map[string]any{
			"enabled":    skill.StoryboardEnabled,
			"modelName":  storyboardModel,
			"prompt":     storyboardPrompt,
			"references": storyboardReferences,
		},
	}
	if jobType == "video" {
		videoOptions := resolveSkillVideoGenerationOptions(skill, model)
		if strings.TrimSpace(videoOptions.AspectRatio) != "" {
			payload["aspectRatio"] = strings.TrimSpace(videoOptions.AspectRatio)
		}
		if strings.TrimSpace(videoOptions.Resolution) != "" {
			payload["resolution"] = strings.TrimSpace(videoOptions.Resolution)
		}
		if videoOptions.DurationSeconds != nil && *videoOptions.DurationSeconds > 0 {
			payload["durationSeconds"] = *videoOptions.DurationSeconds
		}
		if isVideoTextSkillOutput(skill.OutputType) && skill.FixedDurationSeconds != nil && *skill.FixedDurationSeconds > 0 {
			segmentSeconds := ResolveSkillVideoSegmentSeconds(model)
			rule, ruleErr := app.Store.FindEnabledWorkflowDurationRule(ctx, "video_text", "视文模式", *skill.FixedDurationSeconds)
			if ruleErr != nil {
				return nil, ruleErr
			}
			payload["fixedDurationSeconds"] = *skill.FixedDurationSeconds
			payload["durationSeconds"] = *skill.FixedDurationSeconds
			workflowPricing := map[string]any{
				"workflowCode":    "video_text",
				"outputType":      "视文模式",
				"durationSeconds": *skill.FixedDurationSeconds,
				"segmentSeconds":  segmentSeconds,
			}
			if rule != nil {
				workflowPricing["ruleId"] = rule.ID
				workflowPricing["specialPriceCredits"] = rule.SpecialPriceCredits
			}
			payload["workflowPricing"] = workflowPricing
		}
	}

	if len(referenceMedia) > 0 || len(referenceTexts) > 0 {
		payload["referenceSummary"] = map[string]any{
			"mediaCount": len(referenceMedia),
			"imageCount": len(referenceImages),
			"videoCount": videoCount,
			"textCount":  len(referenceTexts),
		}
	}

	if len(targets) > 0 {
		publishTargets, accountIDs := buildPublishTargets(targets)
		applyPublishTargetAccountIDs(payload, accountIDs)
		if jobType == "chat" {
			return json.Marshal(payload)
		}
		payload["publishPayload"] = map[string]any{
			"title":                 skill.Name,
			"contentText":           skill.Description,
			"contentTemplate":       skill.Description,
			"contentPromptTemplate": publishPromptTemplate,
			"tags":                  topics,
			"targets":               publishTargets,
			"runAt":                 publishAt.UTC().Format(time.RFC3339),
			"requestedRun":          publishAt.UTC().Format(time.RFC3339),
		}
	}

	return json.Marshal(payload)
}

// 规范化技能主题，统一技能作业helpers链路的输入格式和后续处理行为。
func normalizeSkillTopics(topics []string) []string {
	if len(topics) == 0 {
		return []string{}
	}
	normalized := make([]string, 0, len(topics))
	seen := make(map[string]struct{}, len(topics))
	for _, item := range topics {
		topic := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(item), "#"))
		if topic == "" {
			continue
		}
		key := strings.ToLower(topic)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, topic)
	}
	return normalized
}

// 判断是否属于视频Text技能输出，供当前链路选择后续处理策略。
func isVideoTextSkillOutput(outputType string) bool {
	switch strings.TrimSpace(outputType) {
	case "video", "video_text", "视文模式":
		return true
	default:
		return false
	}
}

type digitalHumanSkillConfig struct {
	Mode       string `json:"mode"`
	GoodsTitle string `json:"goodsTitle"`
	GoodsText  string `json:"goodsText"`
}

func buildDigitalHumanSkillAIJobPayload(
	skill domain.ProductSkill,
	assets []domain.ProductSkillAsset,
	generateAt time.Time,
	publishAt time.Time,
	targets []PublishTarget,
) ([]byte, error) {
	config, err := parseDigitalHumanSkillConfig(skill.ReferencePayload)
	if err != nil {
		return nil, err
	}
	config = normalizeDigitalHumanSkillConfig(config)
	if strings.TrimSpace(config.Mode) == "" {
		return nil, fmt.Errorf("digital human skill mode is required")
	}
	if strings.TrimSpace(config.GoodsText) == "" {
		return nil, fmt.Errorf("digital human skill goodsText is required")
	}
	characterAsset, goodsAsset, refAudioAsset, err := collectDigitalHumanSkillAssets(assets, config.Mode)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"workflowKind":          "digital_human",
		"prompt":                BuildSkillJobPrompt(skill, "digital_human"),
		"skillName":             skill.Name,
		"skillDescription":      skill.Description,
		"publishPromptTemplate": BuildSkillPublishPromptTemplate(skill),
		"publishIntroEnabled":   skill.PublishIntroEnabled,
		"skillTags":             normalizeSkillTopics(skill.Topics),
		"runAt":                 generateAt.UTC().Format(time.RFC3339),
		"publishAt":             publishAt.UTC().Format(time.RFC3339),
		"digitalHumanConfig": map[string]any{
			"mode":           strings.TrimSpace(config.Mode),
			"goodsTitle":     strings.TrimSpace(config.GoodsTitle),
			"goodsText":      strings.TrimSpace(config.GoodsText),
			"characterAsset": buildDigitalHumanSkillAssetPayload(characterAsset),
			"refAudioAsset":  buildDigitalHumanSkillAssetPayload(refAudioAsset),
		},
	}
	if goodsAsset != nil {
		payload["digitalHumanConfig"].(map[string]any)["goodsAsset"] = buildDigitalHumanSkillAssetPayload(*goodsAsset)
	}
	if len(targets) > 0 {
		publishTargets, accountIDs := buildPublishTargets(targets)
		applyPublishTargetAccountIDs(payload, accountIDs)
		payload["publishPayload"] = map[string]any{
			"targets":               publishTargets,
			"contentPromptTemplate": BuildSkillPublishPromptTemplate(skill),
		}
	}
	return json.Marshal(payload)
}

func normalizeDigitalHumanSkillConfig(config *digitalHumanSkillConfig) *digitalHumanSkillConfig {
	if config == nil {
		return &digitalHumanSkillConfig{Mode: "customize"}
	}
	return &digitalHumanSkillConfig{
		Mode:      "customize",
		GoodsText: strings.TrimSpace(config.GoodsText),
	}
}

func parseDigitalHumanSkillConfig(raw []byte) (*digitalHumanSkillConfig, error) {
	if len(raw) == 0 {
		return &digitalHumanSkillConfig{}, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	config := &digitalHumanSkillConfig{}
	digitalHumanRaw, ok := payload["digitalHuman"]
	if ok && len(digitalHumanRaw) > 0 {
		if err := json.Unmarshal(digitalHumanRaw, config); err != nil {
			return nil, fmt.Errorf("digitalHuman config must be valid json: %w", err)
		}
		return config, nil
	}
	if err := json.Unmarshal(raw, config); err == nil {
		return config, nil
	}
	return &digitalHumanSkillConfig{}, nil
}

func collectDigitalHumanSkillAssets(assets []domain.ProductSkillAsset, mode string) (domain.ProductSkillAsset, *domain.ProductSkillAsset, domain.ProductSkillAsset, error) {
	var characterAsset *domain.ProductSkillAsset
	var goodsAsset *domain.ProductSkillAsset
	var refAudioAsset *domain.ProductSkillAsset
	for index := range assets {
		asset := assets[index]
		switch strings.TrimSpace(asset.AssetType) {
		case skillAssetCharacterImage:
			if characterAsset == nil || asset.CreatedAt.After(characterAsset.CreatedAt) {
				characterAsset = &asset
			}
		case skillAssetGoodsImage:
			if goodsAsset == nil || asset.CreatedAt.After(goodsAsset.CreatedAt) {
				goodsAsset = &asset
			}
		case skillAssetRefAudio:
			if refAudioAsset == nil || asset.CreatedAt.After(refAudioAsset.CreatedAt) {
				refAudioAsset = &asset
			}
		}
	}
	if characterAsset == nil {
		return domain.ProductSkillAsset{}, nil, domain.ProductSkillAsset{}, fmt.Errorf("digital human skill is missing character image asset")
	}
	if refAudioAsset == nil {
		return domain.ProductSkillAsset{}, nil, domain.ProductSkillAsset{}, fmt.Errorf("digital human skill is missing reference audio asset")
	}
	if strings.EqualFold(strings.TrimSpace(mode), "digital") && goodsAsset == nil {
		return domain.ProductSkillAsset{}, nil, domain.ProductSkillAsset{}, fmt.Errorf("digital human skill digital mode requires goods image asset")
	}
	if !strings.EqualFold(strings.TrimSpace(mode), "digital") {
		goodsAsset = nil
	}
	return *characterAsset, goodsAsset, *refAudioAsset, nil
}

func ResolveMixVideoSkillConfig(skill domain.ProductSkill) mixVideoSkillConfig {
	config := mixVideoSkillConfig{
		ScriptRewriteEnabled: true,
	}
	if len(skill.ReferencePayload) == 0 {
		return config
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(skill.ReferencePayload, &payload); err != nil {
		return config
	}
	var raw map[string]any
	if nested, ok := payload["mixVideo"]; ok && len(nested) > 0 {
		if err := json.Unmarshal(nested, &raw); err != nil {
			return config
		}
	} else if err := json.Unmarshal(skill.ReferencePayload, &raw); err != nil {
		return config
	}
	if value, ok := raw["scriptRewriteEnabled"].(bool); ok {
		config.ScriptRewriteEnabled = value
	}
	config.PublishTemplate = strings.TrimSpace(fmt.Sprintf("%v", raw["publishTemplate"]))
	if config.PublishTemplate == "<nil>" {
		config.PublishTemplate = ""
	}
	return config
}

func CollectMixVideoSkillAssets(assets []domain.ProductSkillAsset) ([]domain.ProductSkillAsset, *domain.ProductSkillAsset, error) {
	sourceAssets := make([]domain.ProductSkillAsset, 0)
	var refAudioAsset *domain.ProductSkillAsset
	for index := range assets {
		asset := assets[index]
		switch strings.TrimSpace(asset.AssetType) {
		case skillAssetMixVideoSourceVideo:
			sourceAssets = append(sourceAssets, asset)
		case skillAssetMixVideoRefAudio:
			if refAudioAsset == nil || asset.CreatedAt.After(refAudioAsset.CreatedAt) {
				refAudioAsset = &asset
			}
		}
	}
	sort.SliceStable(sourceAssets, func(i, j int) bool {
		if !sourceAssets[i].CreatedAt.Equal(sourceAssets[j].CreatedAt) {
			return sourceAssets[i].CreatedAt.Before(sourceAssets[j].CreatedAt)
		}
		return strings.TrimSpace(sourceAssets[i].ID) < strings.TrimSpace(sourceAssets[j].ID)
	})
	if len(sourceAssets) == 0 {
		return nil, nil, fmt.Errorf("mix video skill requires at least one source video asset")
	}
	if refAudioAsset == nil {
		return nil, nil, fmt.Errorf("mix video skill requires exactly one reference audio asset")
	}
	return sourceAssets, refAudioAsset, nil
}

func BuildMixVideoScriptRewriteInput(adminPrompt string, skillPrompt string, scriptTemplate string) string {
	parts := []string{
		strings.TrimSpace(adminPrompt),
		strings.TrimSpace(skillPrompt),
		"请基于以下混剪脚本模板输出最终脚本：",
		strings.TrimSpace(scriptTemplate),
	}
	return strings.TrimSpace(strings.Join(filterNonEmptyStrings(parts), "\n\n"))
}

func BuildMixVideoPublishIntroRewriteInput(adminPrompt string, skillPrompt string, publishTemplate string, finalScript string) string {
	parts := []string{
		strings.TrimSpace(adminPrompt),
		strings.TrimSpace(skillPrompt),
		"平台简介模板：",
		strings.TrimSpace(publishTemplate),
		"最终混剪脚本：",
		strings.TrimSpace(finalScript),
	}
	return strings.TrimSpace(strings.Join(filterNonEmptyStrings(parts), "\n\n"))
}

func buildDigitalHumanSkillAssetPayload(asset domain.ProductSkillAsset) map[string]any {
	payload := map[string]any{
		"fileName":   asset.FileName,
		"mimeType":   optionalStringValue(asset.MimeType),
		"storageKey": optionalStringValue(asset.StorageKey),
		"publicUrl":  optionalStringValue(asset.PublicURL),
	}
	if asset.SizeBytes != nil {
		payload["sizeBytes"] = *asset.SizeBytes
	}
	return payload
}

// 加载技能分镜配置，供技能作业helpers继续处理当前业务状态。
func loadSkillStoryboardConfig(ctx context.Context, app *appstate.App, skill domain.ProductSkill, jobType string) (string, string, []map[string]any, error) {
	prompt := ResolveSkillStoryboardPromptTemplate(skill, jobType)
	model := strings.TrimSpace(app.Config.DefaultChatModel)
	references := make([]map[string]any, 0)

	record, err := app.Store.GetAdminSystemSettings(ctx)
	if err != nil {
		return "", "", nil, err
	}
	if record == nil {
		return prompt, model, references, nil
	}

	rawReferences := record.StoryboardReferences
	switch strings.TrimSpace(jobType) {
	case "image":
		if strings.TrimSpace(record.ImageStoryboardModel) != "" {
			model = strings.TrimSpace(record.ImageStoryboardModel)
		} else if strings.TrimSpace(record.StoryboardModel) != "" {
			model = strings.TrimSpace(record.StoryboardModel)
		}
		if len(record.ImageStoryboardReferences) > 0 {
			rawReferences = record.ImageStoryboardReferences
		}
	default:
		if strings.TrimSpace(record.StoryboardModel) != "" {
			model = strings.TrimSpace(record.StoryboardModel)
		}
	}
	if len(rawReferences) > 0 {
		_ = json.Unmarshal(rawReferences, &references)
	}
	return prompt, model, references, nil
}

// 读取技能资源Text，按当前存储模式返回后续流程需要的数据内容。
func readSkillAssetText(ctx context.Context, app *appstate.App, asset domain.ProductSkillAsset) string {
	if app == nil || app.Storage == nil || asset.StorageKey == nil || strings.TrimSpace(*asset.StorageKey) == "" {
		return ""
	}
	data, _, err := app.Storage.ReadBytes(ctx, strings.TrimSpace(*asset.StorageKey))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// 收集Ordered技能参考媒体Assets，整合技能作业helpers链路所需的候选输入。
func collectOrderedSkillReferenceMediaAssets(assets []domain.ProductSkillAsset, referencePayload []byte) []domain.ProductSkillAsset {
	if len(assets) == 0 {
		return nil
	}
	orderIndex := make(map[string]int)
	for index, assetID := range decodeSkillReferenceMediaOrder(referencePayload) {
		trimmed := strings.TrimSpace(assetID)
		if trimmed == "" {
			continue
		}
		if _, exists := orderIndex[trimmed]; exists {
			continue
		}
		orderIndex[trimmed] = index
	}
	items := make([]domain.ProductSkillAsset, 0, len(assets))
	for _, asset := range assets {
		if isSkillReferenceImage(asset) || isSkillReferenceVideo(asset) {
			items = append(items, asset)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		leftOrder, leftHas := orderIndex[items[i].ID]
		rightOrder, rightHas := orderIndex[items[j].ID]
		switch {
		case leftHas && rightHas:
			return leftOrder < rightOrder
		case leftHas:
			return true
		case rightHas:
			return false
		default:
			if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
				return items[i].CreatedAt.Before(items[j].CreatedAt)
			}
			return strings.TrimSpace(items[i].ID) < strings.TrimSpace(items[j].ID)
		}
	})
	return items
}

// 处理解码技能参考媒体订单相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func decodeSkillReferenceMediaOrder(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	values, ok := payload["referenceMediaOrder"]
	if !ok {
		return nil
	}
	return normalizeReferenceMediaOrderValues(values)
}

// 规范化参考媒体订单值，统一技能作业helpers链路的输入格式和后续处理行为。
func normalizeReferenceMediaOrderValues(value any) []string {
	rawValues, ok := value.([]any)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(rawValues))
	seen := make(map[string]struct{}, len(rawValues))
	for _, item := range rawValues {
		trimmed := strings.TrimSpace(fmt.Sprintf("%v", item))
		if trimmed == "" || trimmed == "<nil>" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// 构建技能参考媒体，为技能作业helpers生成后续步骤所需的派生参数或载荷。
func buildSkillReferenceMedia(asset domain.ProductSkillAsset) map[string]any {
	kind := "image"
	if isSkillReferenceVideo(asset) {
		kind = "video"
	}
	return map[string]any{
		"id":        asset.ID,
		"kind":      kind,
		"assetType": asset.AssetType,
		"publicUrl": optionalStringValue(asset.PublicURL),
		"url":       optionalStringValue(asset.PublicURL),
		"fileName":  asset.FileName,
		"mimeType":  optionalStringValue(asset.MimeType),
		"role":      "reference",
	}
}

// 判断是否属于技能参考图片，供当前链路选择后续处理策略。
func isSkillReferenceImage(asset domain.ProductSkillAsset) bool {
	if asset.MimeType != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(*asset.MimeType)), "image/") {
		return true
	}
	value := strings.ToLower(strings.TrimSpace(asset.AssetType))
	return strings.Contains(value, "image") || strings.Contains(value, "cover")
}

// 判断是否属于技能参考视频，供当前链路选择后续处理策略。
func isSkillReferenceVideo(asset domain.ProductSkillAsset) bool {
	if asset.MimeType != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(*asset.MimeType)), "video/") {
		return true
	}
	value := strings.ToLower(strings.TrimSpace(asset.AssetType))
	return strings.Contains(value, "video")
}

// 判断是否属于技能参考Text，供当前链路选择后续处理策略。
func isSkillReferenceText(asset domain.ProductSkillAsset) bool {
	if asset.MimeType != nil {
		mimeType := strings.ToLower(strings.TrimSpace(*asset.MimeType))
		if strings.HasPrefix(mimeType, "text/") || strings.Contains(mimeType, "json") || strings.Contains(mimeType, "xml") || strings.Contains(mimeType, "markdown") {
			return true
		}
	}
	value := strings.ToLower(strings.TrimSpace(asset.AssetType))
	return strings.Contains(value, "text") || strings.Contains(value, "prompt")
}

// 处理可选String值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func filterNonEmptyStrings(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

// 处理追加Unique视频Subtitle规则相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func appendUniqueVideoSubtitleRule(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return defaultSkillVideoSubtitleRule
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "不要字幕") || strings.Contains(lower, "无字幕") || strings.Contains(lower, "纯净画面") {
		return trimmed
	}
	return trimmed + "\n" + defaultSkillVideoSubtitleRule
}

// 解析技能视频生成Options，根据当前配置和上下文确定最终使用结果。
func resolveSkillVideoGenerationOptions(skill domain.ProductSkill, model *domain.AIModel) skillVideoGenerationOptions {
	maps := decodeSkillReferencePayloadMaps(skill.ReferencePayload)
	resolution := firstStringValueFromMaps(maps, "resolution", "videoSize", "size")
	aspectRatio := firstStringValueFromMaps(maps, "aspectRatio", "ratio")
	durationSeconds, _ := firstIntValueFromMaps(maps, "durationSeconds", "duration")
	if skill.FixedDurationSeconds != nil && *skill.FixedDurationSeconds > 0 {
		durationSeconds = *skill.FixedDurationSeconds
	}

	if strings.TrimSpace(resolution) == "" && model != nil {
		resolution = firstSupportedSkillVideoResolution(model.VideoSupportedResolutions)
	}
	if strings.TrimSpace(resolution) == "" {
		resolution = defaultSkillVideoResolution
	}

	if strings.TrimSpace(aspectRatio) == "" {
		aspectRatio = skillAspectRatioFromResolution(resolution)
	}
	if strings.TrimSpace(aspectRatio) == "" {
		aspectRatio = defaultSkillVideoAspectRatio
	}

	if durationSeconds <= 0 {
		durationSeconds = ResolveSkillVideoSegmentSeconds(model)
	}

	return skillVideoGenerationOptions{
		AspectRatio:     strings.TrimSpace(aspectRatio),
		Resolution:      strings.TrimSpace(resolution),
		DurationSeconds: intPtr(durationSeconds),
	}
}

// 处理解码技能参考载荷Maps相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func decodeSkillReferencePayloadMaps(raw []byte) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}

	result := make([]map[string]any, 0, 4)
	for _, key := range []string{"videoSettings", "generationOptions", "video", "settings"} {
		nested, ok := payload[key].(map[string]any)
		if !ok {
			continue
		}
		result = append(result, nested)
	}
	result = append(result, payload)
	return result
}

// 处理首个String值Maps相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstStringValueFromMaps(maps []map[string]any, keys ...string) string {
	for _, item := range maps {
		for _, key := range keys {
			raw, ok := item[key]
			if !ok {
				continue
			}
			value := strings.TrimSpace(fmt.Sprintf("%v", raw))
			if value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return ""
}

// 处理首个Int值Maps相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstIntValueFromMaps(maps []map[string]any, keys ...string) (int, bool) {
	for _, item := range maps {
		for _, key := range keys {
			raw, ok := item[key]
			if !ok {
				continue
			}
			if value, ok := numericJSONInt(raw); ok {
				return value, true
			}
			if typed, ok := raw.(string); ok {
				parsed, parsedOK := parseSkillVideoDurationSeconds(typed)
				if parsedOK {
					return parsed, true
				}
			}
		}
	}
	return 0, false
}

// 处理首个Supported技能视频Resolution相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstSupportedSkillVideoResolution(values []string) string {
	for _, item := range values {
		resolution, _ := parseSkillVideoResolutionOption(item)
		if resolution != "" {
			return resolution
		}
	}
	return ""
}

// 处理首个Supported技能视频时长Seconds相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstSupportedSkillVideoDurationSeconds(values []string) int {
	for _, item := range values {
		seconds, ok := parseSkillVideoDurationSeconds(item)
		if ok {
			return seconds
		}
	}
	return 0
}

// 解析技能视频分段Seconds，根据当前配置和上下文确定最终使用结果。
func ResolveSkillVideoSegmentSeconds(model *domain.AIModel) int {
	if model != nil {
		if seconds := firstSupportedSkillVideoDurationSeconds(model.VideoSupportedDurations); seconds > 0 {
			return seconds
		}
	}
	return defaultSkillVideoDurationSeconds
}

// 解析技能视频Resolution选项，为技能作业helpers提供结构化输入。
func parseSkillVideoResolutionOption(value string) (string, string) {
	normalized := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(value, "×", "x"), "*", "x")))
	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return "", ""
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || width <= 0 {
		return "", ""
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || height <= 0 {
		return "", ""
	}
	return fmt.Sprintf("%dx%d", width, height), skillAspectRatioFromDimensions(width, height)
}

// 解析技能视频时长Seconds，为技能作业helpers提供结构化输入。
func parseSkillVideoDurationSeconds(value string) (int, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.TrimSuffix(normalized, "s")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(normalized)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

// 处理技能AspectRatioResolution相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func skillAspectRatioFromResolution(value string) string {
	_, aspectRatio := parseSkillVideoResolutionOption(value)
	return aspectRatio
}

// 处理技能AspectRatioDimensions相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func skillAspectRatioFromDimensions(width int, height int) string {
	divisor := skillGreatestCommonDivisor(width, height)
	if divisor <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", width/divisor, height/divisor)
}

// 处理技能Greatest公共Divisor相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func skillGreatestCommonDivisor(left int, right int) int {
	for right != 0 {
		left, right = right, left%right
	}
	if left < 0 {
		return -left
	}
	if left == 0 {
		return 1
	}
	return left
}

// 处理intPtr相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func intPtr(value int) *int {
	if value <= 0 {
		return nil
	}
	result := value
	return &result
}
