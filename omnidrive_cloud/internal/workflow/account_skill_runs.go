package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

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

// 规范化账号技能发布时间，统一账号技能定时任务链路的输入格式和后续处理行为。
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

// 计算下一次账号技能发布时间，供账号技能定时任务链路统一发布时间或调度判定结果。
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

// 规范化账号技能生成提前分钟数，统一账号技能定时任务链路的输入格式和后续处理行为。
func NormalizeAccountSkillGenerationLeadMinutes(raw int) int {
	if raw < 0 {
		return 0
	}
	if raw > 24*60 {
		return 24 * 60
	}
	return raw
}

// 计算计划中的账号技能生成时间，供账号技能定时任务链路复用派生的时间结果。
func ScheduledAccountSkillGenerationTime(publishAt time.Time, generationLeadMinutes int) time.Time {
	return publishAt.UTC().Add(-time.Duration(NormalizeAccountSkillGenerationLeadMinutes(generationLeadMinutes)) * time.Minute)
}

// 解析账号技能调度配置，为账号技能定时任务提供结构化输入。
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

// 提取账号技能目标账号ID，供账号技能定时任务后续关联和分支判断复用。
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

// 准备账号技能运行，补齐执行前依赖的上下文、输入和默认值。
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
		model,
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
		Prompt:       BuildSkillJobPrompt(skill, jobType),
		InputPayload: inputPayload,
		Status:       status,
		Message:      message,
		GenerateAt:   generateAt,
		PublishAt:    publishAt,
	}, nil
}

// 构建默认账号技能调度键，为账号技能定时任务生成后续步骤所需的派生参数或载荷。
func BuildDefaultAccountSkillScheduleKey(skillID string, accountID string, config AccountSkillScheduleConfig) string {
	seed := strings.Join([]string{
		"account-skill-schedule",
		strings.TrimSpace(skillID),
		strings.TrimSpace(accountID),
		strings.TrimSpace(config.TimeOfDay),
		strings.TrimSpace(config.Timezone),
		fmt.Sprintf("%t", config.RepeatDaily),
		fmt.Sprintf("%d", NormalizeAccountSkillGenerationLeadMinutes(config.GenerationLeadMinutes)),
	}, "|")
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(seed)).String()
}

// 应用账号技能调度配置，把外部输入转换为当前链路的最终状态变更。
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

// 从 JSON 反序列化结果中提取整数值，兼容不同数字类型的输入。
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

// 推断账号技能生成提前分钟数载荷，在输入缺省时补齐账号技能定时任务链路需要的派生值。
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

// 解析账号技能调度使用的时区位置，统一发布时间计算基准。
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
