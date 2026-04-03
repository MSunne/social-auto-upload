package workflow

import (
	"context"
	"encoding/json"
	"fmt"
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

const (
	defaultSkillVideoAspectRatio     = "16:9"
	defaultSkillVideoResolution      = "1280x720"
	defaultSkillVideoDurationSeconds = 8
	defaultSkillVideoSubtitleRule    = "默认不要字幕"
	defaultSkillStoryboardPrompt     = "你是内容创作分镜与脚本优化助手。请结合用户目标、参考图片和参考文本，输出适合继续交给图片、视频或文本模型执行的精炼脚本。输出中需要保留主体、场景、镜头、风格、文案和节奏等关键信息。"
)

type skillVideoGenerationOptions struct {
	AspectRatio     string
	Resolution      string
	DurationSeconds *int
}

func ScheduledSkillGenerationTime(publishAt time.Time) time.Time {
	return publishAt.UTC().Add(-30 * time.Minute)
}

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

func BuildSkillPublishPromptTemplate(skill domain.ProductSkill) string {
	if !skill.PublishIntroEnabled {
		return ""
	}
	return strings.TrimSpace(optionalStringValue(skill.PublishPromptTemplate))
}

func DefaultSkillStoryboardPromptTemplate(outputType string) string {
	return strings.TrimSpace(defaultSkillStoryboardPrompt)
}

func ResolveSkillStoryboardPromptTemplate(skill domain.ProductSkill, jobType string) string {
	if prompt := strings.TrimSpace(optionalStringValue(skill.StoryboardPromptTemplate)); prompt != "" {
		return prompt
	}
	if prompt := strings.TrimSpace(DefaultSkillStoryboardPromptTemplate(skill.OutputType)); prompt != "" {
		return prompt
	}
	return strings.TrimSpace(defaultSkillStoryboardPrompt)
}

func MapSkillOutputTypeToJobType(outputType string) (string, bool) {
	switch strings.TrimSpace(outputType) {
	case "image", "image_text", "图文模式":
		return "image", true
	case "video", "video_text", "视文模式":
		return "video", true
	case "chat", "text", "text_only", "文本格式":
		return "chat", true
	default:
		return "", false
	}
}

func AccountAllowedForAutoPublish(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "invalid", "disabled", "deleted":
		return false
	default:
		return true
	}
}

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

	referenceImages := make([]map[string]any, 0)
	referenceTexts := make([]map[string]any, 0)
	for _, asset := range assets {
		if isSkillReferenceImage(asset) {
			referenceImages = append(referenceImages, map[string]any{
				"publicUrl": optionalStringValue(asset.PublicURL),
				"url":       optionalStringValue(asset.PublicURL),
				"fileName":  asset.FileName,
				"mimeType":  optionalStringValue(asset.MimeType),
				"role":      "reference",
			})
			continue
		}
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
		"skillTags":             topics,
		"runAt":                 generateAt.UTC().Format(time.RFC3339),
		"publishAt":             publishAt.UTC().Format(time.RFC3339),
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
	}

	if len(referenceImages) > 0 || len(referenceTexts) > 0 {
		payload["referenceSummary"] = map[string]any{
			"imageCount": len(referenceImages),
			"textCount":  len(referenceTexts),
		}
	}

	if jobType != "chat" && len(targets) > 0 {
		publishTargets := make([]map[string]any, 0, len(targets))
		accountIDs := make([]string, 0, len(targets))
		for _, target := range targets {
			item := map[string]any{
				"platform":    strings.TrimSpace(target.Platform),
				"accountName": strings.TrimSpace(target.AccountName),
			}
			if target.AccountID != nil && strings.TrimSpace(*target.AccountID) != "" {
				accountID := strings.TrimSpace(*target.AccountID)
				item["accountId"] = accountID
				accountIDs = append(accountIDs, accountID)
			}
			publishTargets = append(publishTargets, item)
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
		if len(accountIDs) == 1 {
			payload["accountId"] = accountIDs[0]
		} else if len(accountIDs) > 1 {
			payload["accountIds"] = accountIDs
		}
	}

	return json.Marshal(payload)
}

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

func isSkillReferenceImage(asset domain.ProductSkillAsset) bool {
	if asset.MimeType != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(*asset.MimeType)), "image/") {
		return true
	}
	value := strings.ToLower(strings.TrimSpace(asset.AssetType))
	return strings.Contains(value, "image") || strings.Contains(value, "cover")
}

func isSkillReferenceText(asset domain.ProductSkillAsset) bool {
	if asset.MimeType != nil {
		mimeType := strings.ToLower(strings.TrimSpace(*asset.MimeType))
		if strings.HasPrefix(mimeType, "text/") || strings.Contains(mimeType, "json") || strings.Contains(mimeType, "xml") || strings.Contains(mimeType, "markdown") {
			return true
		}
	}
	value := strings.ToLower(strings.TrimSpace(asset.AssetType))
	return strings.Contains(value, "text") || strings.Contains(value, "prompt") || strings.Contains(value, "reference")
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

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

func resolveSkillVideoGenerationOptions(skill domain.ProductSkill, model *domain.AIModel) skillVideoGenerationOptions {
	maps := decodeSkillReferencePayloadMaps(skill.ReferencePayload)
	resolution := firstStringValueFromMaps(maps, "resolution", "videoSize", "size")
	aspectRatio := firstStringValueFromMaps(maps, "aspectRatio", "ratio")
	durationSeconds, _ := firstIntValueFromMaps(maps, "durationSeconds", "duration")

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

	if durationSeconds <= 0 && model != nil {
		durationSeconds = firstSupportedSkillVideoDurationSeconds(model.VideoSupportedDurations)
	}
	if durationSeconds <= 0 {
		durationSeconds = defaultSkillVideoDurationSeconds
	}

	return skillVideoGenerationOptions{
		AspectRatio:     strings.TrimSpace(aspectRatio),
		Resolution:      strings.TrimSpace(resolution),
		DurationSeconds: intPtr(durationSeconds),
	}
}

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

func firstSupportedSkillVideoResolution(values []string) string {
	for _, item := range values {
		resolution, _ := parseSkillVideoResolutionOption(item)
		if resolution != "" {
			return resolution
		}
	}
	return ""
}

func firstSupportedSkillVideoDurationSeconds(values []string) int {
	for _, item := range values {
		seconds, ok := parseSkillVideoDurationSeconds(item)
		if ok {
			return seconds
		}
	}
	return 0
}

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

func skillAspectRatioFromResolution(value string) string {
	_, aspectRatio := parseSkillVideoResolutionOption(value)
	return aspectRatio
}

func skillAspectRatioFromDimensions(width int, height int) string {
	divisor := skillGreatestCommonDivisor(width, height)
	if divisor <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", width/divisor, height/divisor)
}

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

func intPtr(value int) *int {
	if value <= 0 {
		return nil
	}
	result := value
	return &result
}
