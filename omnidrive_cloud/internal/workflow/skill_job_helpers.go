package workflow

import (
	"context"
	"encoding/json"
	"fmt"
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

func ScheduledSkillGenerationTime(publishAt time.Time) time.Time {
	return publishAt.UTC().Add(-30 * time.Minute)
}

func BuildSkillJobPrompt(skill domain.ProductSkill) string {
	prompt := strings.TrimSpace(optionalStringValue(skill.PromptTemplate))
	if prompt == "" {
		prompt = strings.TrimSpace(skill.Description)
	}
	if prompt == "" {
		prompt = strings.TrimSpace(skill.Name)
	}
	return prompt
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

	storyboardPrompt, storyboardModel, storyboardReferences, err := loadSkillStoryboardConfig(ctx, app, jobType)
	if err != nil {
		return nil, err
	}

	prompt := BuildSkillJobPrompt(skill)
	payload := map[string]any{
		"prompt":           prompt,
		"skillName":        skill.Name,
		"skillDescription": skill.Description,
		"runAt":            generateAt.UTC().Format(time.RFC3339),
		"publishAt":        publishAt.UTC().Format(time.RFC3339),
		"referenceImages":  referenceImages,
		"referenceTexts":   referenceTexts,
		"storyboardConfig": map[string]any{
			"enabled":    skill.StoryboardEnabled,
			"modelName":  storyboardModel,
			"prompt":     storyboardPrompt,
			"references": storyboardReferences,
		},
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
			"title":        skill.Name,
			"contentText":  skill.Description,
			"targets":      publishTargets,
			"runAt":        publishAt.UTC().Format(time.RFC3339),
			"requestedRun": publishAt.UTC().Format(time.RFC3339),
		}
		if len(accountIDs) == 1 {
			payload["accountId"] = accountIDs[0]
		} else if len(accountIDs) > 1 {
			payload["accountIds"] = accountIDs
		}
	}

	return json.Marshal(payload)
}

func loadSkillStoryboardConfig(ctx context.Context, app *appstate.App, jobType string) (string, string, []map[string]any, error) {
	prompt := ""
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
		if strings.TrimSpace(record.ImageStoryboardPrompt) != "" {
			prompt = strings.TrimSpace(record.ImageStoryboardPrompt)
		} else {
			prompt = strings.TrimSpace(record.StoryboardPrompt)
		}
		if strings.TrimSpace(record.ImageStoryboardModel) != "" {
			model = strings.TrimSpace(record.ImageStoryboardModel)
		} else if strings.TrimSpace(record.StoryboardModel) != "" {
			model = strings.TrimSpace(record.StoryboardModel)
		}
		if len(record.ImageStoryboardReferences) > 0 {
			rawReferences = record.ImageStoryboardReferences
		}
	default:
		prompt = strings.TrimSpace(record.StoryboardPrompt)
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
