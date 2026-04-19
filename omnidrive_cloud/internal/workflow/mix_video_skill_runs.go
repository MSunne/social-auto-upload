package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"omnidrive_cloud/internal/ai"
	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/store"
)

const mixVideoEstimateCharsPerSecond = 4

const (
	defaultMixVideoScriptRewritePrompt = "你是混剪视频脚本改写助手。请基于用户提供的脚本模板、固定素材和创作目标，输出适合继续生成混剪视频的完整脚本。保留核心信息，强化节奏、镜头衔接、情绪推进与口语化表达，不要输出与执行无关的说明。"
	defaultMixVideoPublishIntroPrompt  = "你是短视频平台发布简介改写助手。请基于平台简介模板和最终混剪脚本，输出适合第三方平台发布的简介文案。内容要简洁、自然、适合公开发布，并避免重复脚本原文。"
)

type MixVideoRunSettings struct {
	CreditsPerSecondMillis int64
	ScriptRewritePrompt    string
	PublishIntroPrompt     string
}

type PreparedAccountMixVideoRun struct {
	ModelName                string
	ScriptText               string
	PublishIntro             string
	RequestPayload           []byte
	SchedulePayload          []byte
	Status                   string
	Message                  string
	GenerateAt               time.Time
	PublishAt                time.Time
	SourceAssets             []domain.MixVideoAsset
	RefAudioAsset            domain.MixVideoAsset
	EstimatedDurationSeconds int
	EstimatedCreditsMillis   int64
	BillingPreview           domain.MixVideoBillingPreview
}

func LoadMixVideoRunSettings(ctx context.Context, app *appstate.App) (MixVideoRunSettings, error) {
	settings := MixVideoRunSettings{
		CreditsPerSecondMillis: 300,
		ScriptRewritePrompt:    defaultMixVideoScriptRewritePrompt,
		PublishIntroPrompt:     defaultMixVideoPublishIntroPrompt,
	}
	if app == nil || app.Store == nil {
		return settings, nil
	}
	record, err := app.Store.GetAdminSystemSettings(ctx)
	if err != nil {
		return settings, err
	}
	if record == nil {
		return settings, nil
	}
	if record.MixVideoCreditsPerSecondMillis > 0 {
		settings.CreditsPerSecondMillis = record.MixVideoCreditsPerSecondMillis
	} else if record.MixVideoCreditsPerSecond > 0 {
		settings.CreditsPerSecondMillis = record.MixVideoCreditsPerSecond * store.CreditMillisScale
	}
	if value := strings.TrimSpace(record.MixVideoScriptRewritePrompt); value != "" {
		settings.ScriptRewritePrompt = value
	}
	if value := strings.TrimSpace(record.MixVideoPublishIntroPrompt); value != "" {
		settings.PublishIntroPrompt = value
	}
	return settings, nil
}

func PrepareAccountMixVideoRun(
	ctx context.Context,
	app *appstate.App,
	settings MixVideoRunSettings,
	skill domain.ProductSkill,
	account domain.PlatformAccount,
	publishAt time.Time,
	scheduleConfig *AccountSkillScheduleConfig,
) (*PreparedAccountMixVideoRun, error) {
	if app == nil || app.Store == nil {
		return nil, fmt.Errorf("app store is required")
	}
	if !IsMixVideoSkillOutput(skill.OutputType) {
		return nil, fmt.Errorf("skill outputType is not mix video")
	}
	modelName := strings.TrimSpace(skill.ModelName)
	if modelName == "" {
		return nil, fmt.Errorf("mix video skill model is required")
	}
	model, err := app.Store.GetAIModelByName(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to validate skill model: %w", err)
	}
	if model == nil || !model.IsEnabled {
		return nil, fmt.Errorf("skill model is disabled or missing")
	}
	if strings.TrimSpace(model.Category) != "chat" {
		return nil, fmt.Errorf("mix video skill model must be an enabled chat model")
	}

	config := ResolveMixVideoSkillConfig(skill)
	scriptTemplate := strings.TrimSpace(optionalStringValue(skill.PromptTemplate))
	if scriptTemplate == "" {
		return nil, fmt.Errorf("mix video skill promptTemplate is required")
	}
	if strings.TrimSpace(config.PublishTemplate) == "" {
		return nil, fmt.Errorf("mix video skill publishTemplate is required")
	}

	assets, err := app.Store.ListSkillAssets(ctx, skill.ID, skill.OwnerUserID)
	if err != nil {
		return nil, err
	}
	sourceAssets, refAudioAsset, err := CollectMixVideoSkillAssets(assets)
	if err != nil {
		return nil, err
	}

	finalScript := scriptTemplate
	if config.ScriptRewriteEnabled {
		result, rewriteErr := ai.GenerateTextWithModelPrompt(ctx, app, modelName, BuildMixVideoScriptRewriteInput(
			settings.ScriptRewritePrompt,
			optionalStringValue(skill.StoryboardPromptTemplate),
			scriptTemplate,
		))
		if rewriteErr != nil {
			return nil, rewriteErr
		}
		if rewritten := strings.TrimSpace(result.Text); rewritten != "" {
			finalScript = rewritten
		}
	}

	finalPublishIntro := strings.TrimSpace(config.PublishTemplate)
	if skill.PublishIntroEnabled {
		result, rewriteErr := ai.GenerateTextWithModelPrompt(ctx, app, modelName, BuildMixVideoPublishIntroRewriteInput(
			settings.PublishIntroPrompt,
			optionalStringValue(skill.PublishPromptTemplate),
			config.PublishTemplate,
			finalScript,
		))
		if rewriteErr != nil {
			return nil, rewriteErr
		}
		if rewritten := strings.TrimSpace(result.Text); rewritten != "" {
			finalPublishIntro = rewritten
		}
	}

	publishAt = publishAt.UTC()
	generationLeadMinutes := 0
	if scheduleConfig != nil {
		generationLeadMinutes = NormalizeAccountSkillGenerationLeadMinutes(scheduleConfig.GenerationLeadMinutes)
		scheduleConfig.GenerationLeadMinutes = generationLeadMinutes
	}
	generateAt := ScheduledAccountSkillGenerationTime(publishAt, generationLeadMinutes)
	billingPreview, err := buildMixVideoBillingPreview(ctx, app, skill.OwnerUserID, finalScript, settings.CreditsPerSecondMillis)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"source":         "account_skill_binding",
		"accountId":      account.ID,
		"skillName":      skill.Name,
		"skillId":        skill.ID,
		"skillOutput":    NormalizeSkillOutputType(skill.OutputType),
		"modelName":      modelName,
		"scriptText":     finalScript,
		"scriptTemplate": scriptTemplate,
		"publishPayload": map[string]any{
			"targets": []map[string]any{{
				"accountId":   account.ID,
				"platform":    account.Platform,
				"accountName": account.AccountName,
			}},
			"contentTemplate": config.PublishTemplate,
			"contentText":     finalPublishIntro,
			"runAt":           publishAt.Format(time.RFC3339),
		},
		"mixVideo": map[string]any{
			"scriptRewriteEnabled": config.ScriptRewriteEnabled,
			"scriptPrompt":         optionalStringValue(skill.StoryboardPromptTemplate),
			"publishPrompt":        optionalStringValue(skill.PublishPromptTemplate),
			"publishTemplate":      config.PublishTemplate,
			"publishIntro":         finalPublishIntro,
		},
		"billingPreview": billingPreview,
	}

	var schedulePayload []byte
	if scheduleConfig != nil {
		schedulePayload, err = json.Marshal(map[string]any{
			"scheduleKey":           strings.TrimSpace(scheduleConfig.ScheduleKey),
			"timeOfDay":             strings.TrimSpace(scheduleConfig.TimeOfDay),
			"repeatDaily":           scheduleConfig.RepeatDaily,
			"timezone":              strings.TrimSpace(scheduleConfig.Timezone),
			"generationLeadMinutes": NormalizeAccountSkillGenerationLeadMinutes(scheduleConfig.GenerationLeadMinutes),
		})
		if err != nil {
			return nil, err
		}
		payload["scheduleConfig"] = map[string]any{
			"scheduleKey":           strings.TrimSpace(scheduleConfig.ScheduleKey),
			"timeOfDay":             strings.TrimSpace(scheduleConfig.TimeOfDay),
			"repeatDaily":           scheduleConfig.RepeatDaily,
			"timezone":              strings.TrimSpace(scheduleConfig.Timezone),
			"generationLeadMinutes": NormalizeAccountSkillGenerationLeadMinutes(scheduleConfig.GenerationLeadMinutes),
		}
	}

	requestPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	status := "scheduled"
	message := "等待定时生成"
	if !generateAt.After(time.Now().UTC()) {
		status = "queued"
		message = "等待混剪生成"
	}

	return &PreparedAccountMixVideoRun{
		ModelName:                modelName,
		ScriptText:               finalScript,
		PublishIntro:             finalPublishIntro,
		RequestPayload:           requestPayload,
		SchedulePayload:          schedulePayload,
		Status:                   status,
		Message:                  message,
		GenerateAt:               generateAt,
		PublishAt:                publishAt,
		SourceAssets:             buildMixVideoTaskAssets(sourceAssets),
		RefAudioAsset:            buildMixVideoTaskAsset(*refAudioAsset),
		EstimatedDurationSeconds: billingPreview.EstimatedDurationSeconds,
		EstimatedCreditsMillis:   int64(billingPreview.EstimatedDurationSeconds) * settings.CreditsPerSecondMillis,
		BillingPreview:           billingPreview,
	}, nil
}

func buildMixVideoTaskAssets(assets []domain.ProductSkillAsset) []domain.MixVideoAsset {
	items := make([]domain.MixVideoAsset, 0, len(assets))
	for _, asset := range assets {
		items = append(items, buildMixVideoTaskAsset(asset))
	}
	return items
}

func buildMixVideoTaskAsset(asset domain.ProductSkillAsset) domain.MixVideoAsset {
	return domain.MixVideoAsset{
		StorageKey: optionalStringValue(asset.StorageKey),
		PublicURL:  optionalStringValue(asset.PublicURL),
		FileName:   asset.FileName,
		MimeType:   optionalStringValue(asset.MimeType),
		SizeBytes:  asset.SizeBytes,
	}
}

func buildMixVideoBillingPreview(ctx context.Context, app *appstate.App, ownerUserID string, scriptText string, creditsPerSecondMillis int64) (domain.MixVideoBillingPreview, error) {
	estimatedDurationSeconds := estimateMixVideoDurationSeconds(scriptText)
	if creditsPerSecondMillis < 0 {
		creditsPerSecondMillis = 0
	}
	estimatedCreditsMillis := int64(estimatedDurationSeconds) * creditsPerSecondMillis
	creditBalanceMillis, err := app.Store.GetWalletCreditBalanceMillisByUser(ctx, ownerUserID)
	if err != nil {
		return domain.MixVideoBillingPreview{}, err
	}
	shortfallCreditsMillis := estimatedCreditsMillis - creditBalanceMillis
	if shortfallCreditsMillis < 0 {
		shortfallCreditsMillis = 0
	}
	canAfford := creditsPerSecondMillis > 0 && creditBalanceMillis >= estimatedCreditsMillis
	return domain.MixVideoBillingPreview{
		CreditsPerSecond:         store.CreditsFromMillis(creditsPerSecondMillis),
		EstimatedDurationSeconds: estimatedDurationSeconds,
		EstimatedCredits:         store.CreditsFromMillis(estimatedCreditsMillis),
		CanAfford:                canAfford,
		CreditBalance:            store.CreditsFromMillis(creditBalanceMillis),
		ShortfallCredits:         store.CreditsFromMillis(shortfallCreditsMillis),
	}, nil
}

func estimateMixVideoDurationSeconds(scriptText string) int {
	nonWhitespaceRunes := 0
	for _, value := range scriptText {
		if unicode.IsSpace(value) {
			continue
		}
		nonWhitespaceRunes++
	}
	estimated := (nonWhitespaceRunes + mixVideoEstimateCharsPerSecond - 1) / mixVideoEstimateCharsPerSecond
	if estimated <= 0 {
		return 1
	}
	return estimated
}

func mustJSONBytes(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return payload
}
