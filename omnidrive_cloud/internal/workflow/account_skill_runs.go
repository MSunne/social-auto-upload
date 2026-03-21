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

type AccountSkillScheduleConfig struct {
	ScheduleKey           string `json:"scheduleKey,omitempty"`
	TimeOfDay             string `json:"timeOfDay,omitempty"`
	RepeatDaily           bool   `json:"repeatDaily"`
	Timezone              string `json:"timezone,omitempty"`
	GenerationLeadMinutes int    `json:"generationLeadMinutes,omitempty"`
}

type PreparedAccountSkillRun struct {
	JobType      string
	ModelName    string
	Prompt       string
	InputPayload []byte
	Status       string
	Message      string
	GenerateAt   time.Time
	PublishAt    time.Time
}

func NormalizeAccountSkillTimeOfDay(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("timeOfDay is required")
	}
	for _, layout := range []string{"15:04:05", "15:04"} {
		parsed, err := time.Parse(layout, value)
		if err != nil {
			continue
		}
		return parsed.Format("15:04:05"), nil
	}
	return "", fmt.Errorf("timeOfDay must be HH:MM or HH:MM:SS")
}

func NextAccountSkillPublishAt(timeOfDay string, timezone string, now time.Time) (time.Time, error) {
	normalized, err := NormalizeAccountSkillTimeOfDay(timeOfDay)
	if err != nil {
		return time.Time{}, err
	}
	location := resolveAccountSkillScheduleLocation(timezone)
	localNow := now.In(location)
	parsed, err := time.Parse("15:04:05", normalized)
	if err != nil {
		return time.Time{}, err
	}
	next := time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day(),
		parsed.Hour(),
		parsed.Minute(),
		parsed.Second(),
		0,
		location,
	)
	if !next.After(localNow) {
		next = next.Add(24 * time.Hour)
	}
	return next.UTC(), nil
}

func NormalizeAccountSkillGenerationLeadMinutes(raw int) int {
	if raw < 0 {
		return 0
	}
	if raw > 24*60 {
		return 24 * 60
	}
	return raw
}

func ScheduledAccountSkillGenerationTime(publishAt time.Time, generationLeadMinutes int) time.Time {
	return publishAt.UTC().Add(-time.Duration(NormalizeAccountSkillGenerationLeadMinutes(generationLeadMinutes)) * time.Minute)
}

func ParseAccountSkillScheduleConfig(raw []byte) (*AccountSkillScheduleConfig, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false
	}
	scheduleRaw, ok := payload["scheduleConfig"].(map[string]any)
	if !ok {
		return nil, false
	}
	timeOfDay, _ := scheduleRaw["timeOfDay"].(string)
	if strings.TrimSpace(timeOfDay) == "" {
		return nil, false
	}
	repeatDaily, _ := scheduleRaw["repeatDaily"].(bool)
	scheduleKey, _ := scheduleRaw["scheduleKey"].(string)
	timezone, _ := scheduleRaw["timezone"].(string)
	generationLeadMinutes, hasGenerationLeadMinutes := numericJSONInt(scheduleRaw["generationLeadMinutes"])
	if !hasGenerationLeadMinutes {
		generationLeadMinutes = inferAccountSkillGenerationLeadMinutesFromPayload(payload)
	}
	return &AccountSkillScheduleConfig{
		ScheduleKey:           strings.TrimSpace(scheduleKey),
		TimeOfDay:             strings.TrimSpace(timeOfDay),
		RepeatDaily:           repeatDaily,
		Timezone:              strings.TrimSpace(timezone),
		GenerationLeadMinutes: NormalizeAccountSkillGenerationLeadMinutes(generationLeadMinutes),
	}, true
}

func ExtractAccountSkillTargetAccountID(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if accountID, ok := payload["accountId"].(string); ok && strings.TrimSpace(accountID) != "" {
		return strings.TrimSpace(accountID)
	}
	publishPayload, ok := payload["publishPayload"].(map[string]any)
	if !ok {
		return ""
	}
	targets, ok := publishPayload["targets"].([]any)
	if !ok || len(targets) == 0 {
		return ""
	}
	firstTarget, ok := targets[0].(map[string]any)
	if !ok {
		return ""
	}
	accountID, _ := firstTarget["accountId"].(string)
	return strings.TrimSpace(accountID)
}

func PrepareAccountSkillRun(
	ctx context.Context,
	app *appstate.App,
	skill domain.ProductSkill,
	account domain.PlatformAccount,
	publishAt time.Time,
	scheduleConfig *AccountSkillScheduleConfig,
) (*PreparedAccountSkillRun, error) {
	jobType, ok := MapSkillOutputTypeToJobType(skill.OutputType)
	if !ok {
		return nil, fmt.Errorf("skill outputType is not supported")
	}
	model, err := app.Store.GetAIModelByName(ctx, strings.TrimSpace(skill.ModelName))
	if err != nil {
		return nil, fmt.Errorf("failed to validate skill model: %w", err)
	}
	if model == nil || !model.IsEnabled {
		return nil, fmt.Errorf("skill model is disabled or missing")
	}
	if model.Category != jobType {
		return nil, fmt.Errorf("skill model category does not match skill output type")
	}

	publishAt = publishAt.UTC()
	generationLeadMinutes := 0
	if scheduleConfig != nil {
		generationLeadMinutes = NormalizeAccountSkillGenerationLeadMinutes(scheduleConfig.GenerationLeadMinutes)
		scheduleConfig.GenerationLeadMinutes = generationLeadMinutes
	}
	generateAt := ScheduledAccountSkillGenerationTime(publishAt, generationLeadMinutes)
	inputPayload, err := BuildSkillAIJobPayload(
		ctx,
		app,
		skill,
		generateAt,
		publishAt,
		jobType,
		[]PublishTarget{{
			AccountID:   &account.ID,
			Platform:    account.Platform,
			AccountName: account.AccountName,
		}},
	)
	if err != nil {
		return nil, err
	}
	if scheduleConfig != nil {
		inputPayload, err = applyAccountSkillScheduleConfig(inputPayload, *scheduleConfig)
		if err != nil {
			return nil, err
		}
	}

	status := "scheduled"
	message := "等待定时生成"
	if !generateAt.After(time.Now().UTC()) {
		status = "queued"
		message = "等待云端生成"
	}

	return &PreparedAccountSkillRun{
		JobType:      jobType,
		ModelName:    strings.TrimSpace(skill.ModelName),
		Prompt:       BuildSkillJobPrompt(skill),
		InputPayload: inputPayload,
		Status:       status,
		Message:      message,
		GenerateAt:   generateAt,
		PublishAt:    publishAt,
	}, nil
}

func applyAccountSkillScheduleConfig(raw []byte, config AccountSkillScheduleConfig) ([]byte, error) {
	payload := make(map[string]any)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
	}
	payload["scheduleConfig"] = map[string]any{
		"scheduleKey":           strings.TrimSpace(config.ScheduleKey),
		"timeOfDay":             strings.TrimSpace(config.TimeOfDay),
		"repeatDaily":           config.RepeatDaily,
		"timezone":              strings.TrimSpace(config.Timezone),
		"generationLeadMinutes": NormalizeAccountSkillGenerationLeadMinutes(config.GenerationLeadMinutes),
	}
	return json.Marshal(payload)
}

func numericJSONInt(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case int32:
		return int(typed), true
	default:
		return 0, false
	}
}

func inferAccountSkillGenerationLeadMinutesFromPayload(payload map[string]any) int {
	publishAtRaw, _ := payload["publishAt"].(string)
	runAtRaw, _ := payload["runAt"].(string)
	if strings.TrimSpace(publishAtRaw) == "" || strings.TrimSpace(runAtRaw) == "" {
		return 0
	}
	publishAt, err := time.Parse(time.RFC3339, strings.TrimSpace(publishAtRaw))
	if err != nil {
		return 0
	}
	runAt, err := time.Parse(time.RFC3339, strings.TrimSpace(runAtRaw))
	if err != nil {
		return 0
	}
	if !publishAt.After(runAt) {
		return 0
	}
	return NormalizeAccountSkillGenerationLeadMinutes(int(publishAt.Sub(runAt).Minutes()))
}

func resolveAccountSkillScheduleLocation(timezone string) *time.Location {
	trimmed := strings.TrimSpace(timezone)
	if trimmed == "" || strings.EqualFold(trimmed, "local") {
		return time.Local
	}
	location, err := time.LoadLocation(trimmed)
	if err != nil {
		return time.Local
	}
	return location
}
