package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/store"
)

const (
	videoTextWorkflowCode             = "video_text"
	workflowSpecialPriceItemType      = "workflow_special_price_segment"
	workflowVideoSegmentItemType      = "video_segment_generation"
	workflowStoryboardPackageItemType = "storyboard_package"
	workflowCoverFrameItemType        = "cover_frame_generation"
)

type workflowPricingSnapshot struct {
	RuleID              string
	WorkflowCode        string
	OutputType          string
	DurationSeconds     int
	SegmentSeconds      int
	SpecialPriceCredits *int64
}

type workflowBillingPlan struct {
	SessionInput store.CreateAIBillingSessionInput
	ItemInputs   []store.CreateAIBillingItemInput
}

// 构建工作流计费Plan，为AI作业生成后续步骤所需的派生参数或载荷。
func BuildWorkflowBillingPlan(ctx context.Context, app *appstate.App, job *domain.AIJob) (*workflowBillingPlan, error) {
	if app == nil || job == nil {
		return nil, nil
	}

	snapshot := parseWorkflowPricingSnapshot(job)
	if snapshot == nil {
		return nil, nil
	}
	if !strings.EqualFold(strings.TrimSpace(job.JobType), "video") {
		return nil, nil
	}

	plannedSegmentCount := plannedVideoSegmentCount(snapshot.DurationSeconds, snapshot.SegmentSeconds)
	if plannedSegmentCount <= 0 {
		return nil, nil
	}

	itemInputs := make([]store.CreateAIBillingItemInput, 0, plannedSegmentCount+2)
	payload := decodePayloadMap(job.InputPayload)
	if snapshot.SpecialPriceCredits != nil && *snapshot.SpecialPriceCredits > 0 {
		distributed := distributeCredits(*snapshot.SpecialPriceCredits, plannedSegmentCount)
		for index := 0; index < plannedSegmentCount; index++ {
			plannedCredits := distributed[index]
			itemInputs = append(itemInputs, store.CreateAIBillingItemInput{
				ID:               uuid.NewString(),
				SessionID:        "",
				UserID:           strings.TrimSpace(job.OwnerUserID),
				SourceType:       "ai_job",
				SourceID:         strings.TrimSpace(job.ID),
				ItemKey:          workflowSpecialPriceSegmentItemKey(index + 1),
				ItemType:         workflowSpecialPriceItemType,
				Label:            fmt.Sprintf("视文模式 %ds 任务特价（第 %d 段）", snapshot.DurationSeconds, index+1),
				Quantity:         1,
				Unit:             "segment",
				UnitPriceCredits: plannedCredits,
				PlannedCredits:   plannedCredits,
				SortOrder:        index + 1,
				IsVisible:        false,
				Status:           "precharged",
				Payload: mustJSONBytes(map[string]any{
					"mode":            "special_price",
					"segmentIndex":    index + 1,
					"segmentCount":    plannedSegmentCount,
					"durationSeconds": snapshot.DurationSeconds,
				}),
			})
		}
	} else {
		sortOrder := 1
		preprocessEnabled := videoPreprocessEnabled(payload)
		storyboardEnabled := preprocessEnabled && videoStoryboardEnabled(payload)
		if storyboardEnabled {
			if storyboardModel, err := resolveWorkflowStoryboardBillingModel(ctx, app); err != nil {
				return nil, err
			} else if storyboardModel != nil {
				itemInputs = append(itemInputs, buildWorkflowBillingItem(
					job,
					workflowStoryboardPackageItemKey(),
					workflowStoryboardPackageItemType,
					"AI 分镜脚本与首帧生成",
					storyboardModel,
					sortOrder,
					map[string]any{
						"stage":             "storyboard_package",
						"durationSeconds":   snapshot.DurationSeconds,
						"segmentCount":      plannedSegmentCount,
						"storyboardEnabled": true,
					},
				))
				sortOrder++
			} else if coverModel, coverErr := resolveWorkflowCoverBillingModel(ctx, app); coverErr != nil {
				return nil, coverErr
			} else if coverModel != nil {
				itemInputs = append(itemInputs, buildWorkflowBillingItem(
					job,
					workflowCoverFrameItemKey(),
					workflowCoverFrameItemType,
					"AI 首帧参考图生成",
					coverModel,
					sortOrder,
					map[string]any{
						"stage":             "cover_frame_generation",
						"durationSeconds":   snapshot.DurationSeconds,
						"segmentCount":      plannedSegmentCount,
						"storyboardEnabled": true,
						"fallback":          true,
					},
				))
				sortOrder++
			}
		} else if preprocessEnabled {
			if coverModel, err := resolveWorkflowCoverBillingModel(ctx, app); err != nil {
				return nil, err
			} else if coverModel != nil {
				itemInputs = append(itemInputs, buildWorkflowBillingItem(
					job,
					workflowCoverFrameItemKey(),
					workflowCoverFrameItemType,
					"AI 首帧参考图生成",
					coverModel,
					sortOrder,
					map[string]any{
						"stage":           "cover_frame_generation",
						"durationSeconds": snapshot.DurationSeconds,
						"segmentCount":    plannedSegmentCount,
					},
				))
				sortOrder++
			}
		}

		videoModel, err := app.Store.GetAIModelByName(ctx, strings.TrimSpace(job.ModelName))
		if err != nil {
			return nil, err
		}
		for index := 0; index < plannedSegmentCount; index++ {
			itemInputs = append(itemInputs, buildWorkflowBillingItem(
				job,
				workflowVideoSegmentItemKey(index+1),
				workflowVideoSegmentItemType,
				fmt.Sprintf("AI 视频生成（第 %d 段）", index+1),
				videoModel,
				sortOrder,
				map[string]any{
					"stage":             "video_segment_generation",
					"segmentIndex":      index + 1,
					"segmentCount":      plannedSegmentCount,
					"durationSeconds":   snapshot.DurationSeconds,
					"segmentSeconds":    snapshot.SegmentSeconds,
					"storyboardEnabled": storyboardEnabled,
				},
			))
			sortOrder++
		}
	}

	var plannedCredits int64
	for _, item := range itemInputs {
		plannedCredits += maxInt64Value(item.PlannedCredits, 0)
	}
	sessionPayload := mustJSONBytes(map[string]any{
		"displayMode": func() string {
			if snapshot.SpecialPriceCredits != nil && *snapshot.SpecialPriceCredits > 0 {
				return "special_price"
			}
			return "itemized"
		}(),
		"displayLabel": fmt.Sprintf("视文模式 %ds 视频生成", snapshot.DurationSeconds),
		"segmentCount": plannedSegmentCount,
	})
	sessionInput := store.CreateAIBillingSessionInput{
		ID:                  uuid.NewString(),
		UserID:              strings.TrimSpace(job.OwnerUserID),
		SourceType:          "ai_job",
		SourceID:            strings.TrimSpace(job.ID),
		WorkflowCode:        snapshot.WorkflowCode,
		OutputType:          snapshot.OutputType,
		DurationSeconds:     snapshot.DurationSeconds,
		SegmentSeconds:      snapshot.SegmentSeconds,
		SpecialRuleID:       normalizeTrimmedStringPointer(snapshot.RuleID),
		SpecialPriceCredits: snapshot.SpecialPriceCredits,
		PlannedCredits:      plannedCredits,
		Status:              "precharged",
		Payload:             sessionPayload,
	}

	return &workflowBillingPlan{
		SessionInput: sessionInput,
		ItemInputs:   itemInputs,
	}, nil
}

// 处理预览工作流计费相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func PreviewWorkflowBilling(ctx context.Context, app *appstate.App, job *domain.AIJob) (*store.ApplyUsageBillingResult, error) {
	plan, err := BuildWorkflowBillingPlan(ctx, app, job)
	if err != nil || plan == nil {
		return nil, err
	}

	summary, err := app.Store.GetBillingSummaryByUser(ctx, strings.TrimSpace(job.OwnerUserID))
	if err != nil {
		return nil, err
	}
	result := workflowBillingPlanToUsageResult(plan, summary.CreditBalance)
	return result, nil
}

// 确保工作流计费会话已满足执行前提，必要时补齐缺失状态或配置。
func EnsureWorkflowBillingSession(ctx context.Context, app *appstate.App, job *domain.AIJob) (*domain.AIBillingSession, []domain.AIBillingItem, error) {
	plan, err := BuildWorkflowBillingPlan(ctx, app, job)
	if err != nil || plan == nil {
		return nil, nil, err
	}
	sourceSnapshot := buildAIJobBillingSourceSnapshot(job, plan)
	return app.Store.CreateOrRechargeAIBillingSession(ctx, plan.SessionInput, plan.ItemInputs, sourceSnapshot)
}

// 处理Finalize工作流计费会话相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func FinalizeWorkflowBillingSession(
	ctx context.Context,
	app *appstate.App,
	job *domain.AIJob,
	successfulItemKeys []string,
	message string,
	payload []byte,
) (*domain.AIBillingSession, []domain.AIBillingItem, error) {
	if app == nil || job == nil {
		return nil, nil, nil
	}
	return app.Store.FinalizeAIBillingSession(ctx, "ai_job", job.ID, successfulItemKeys, message, payload)
}

// 构建AI作业计费来源Snapshot，为AI作业生成后续步骤所需的派生参数或载荷。
func buildAIJobBillingSourceSnapshot(job *domain.AIJob, plan *workflowBillingPlan) []byte {
	if job == nil || plan == nil {
		return mustJSONBytes(map[string]any{})
	}
	return mustJSONBytes(map[string]any{
		"jobId":               job.ID,
		"jobType":             job.JobType,
		"modelName":           job.ModelName,
		"source":              job.Source,
		"workflowCode":        plan.SessionInput.WorkflowCode,
		"outputType":          plan.SessionInput.OutputType,
		"durationSeconds":     plan.SessionInput.DurationSeconds,
		"segmentSeconds":      plan.SessionInput.SegmentSeconds,
		"specialPriceCredits": plan.SessionInput.SpecialPriceCredits,
	})
}

// 处理工作流计费Plan用量Result相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func workflowBillingPlanToUsageResult(plan *workflowBillingPlan, creditBalance int64) *store.ApplyUsageBillingResult {
	if plan == nil {
		return nil
	}
	details := make([]store.UsageBillingDetail, 0, len(plan.ItemInputs))
	totalCredits := int64(0)
	for _, item := range plan.ItemInputs {
		totalCredits += maxInt64Value(item.PlannedCredits, 0)
	}
	billStatus := "billed"
	billMessage := "workflow billing preview ready"
	if creditBalance < totalCredits {
		billStatus = "failed"
		billMessage = "wallet credits insufficient"
	}
	if plan.SessionInput.SpecialPriceCredits != nil && *plan.SessionInput.SpecialPriceCredits > 0 {
		details = append(details, store.UsageBillingDetail{
			MeterCode:             plan.SessionInput.WorkflowCode,
			Quantity:              int64(len(plan.ItemInputs)),
			Units:                 1,
			DebitCredits:          totalCredits,
			SupportsFailureRefund: true,
			ChargeMode:            "workflow_special_price",
			BillStatus:            billStatus,
			BillMessage:           billMessage,
			PricingRuleID:         strings.TrimSpace(valueOrEmptyString(plan.SessionInput.SpecialRuleID)),
		})
	} else {
		for _, item := range plan.ItemInputs {
			details = append(details, store.UsageBillingDetail{
				MeterCode:             firstNonEmptyStringValue(valueOrEmptyString(item.MeterCode), item.ItemType),
				Quantity:              maxInt64Value(item.Quantity, 1),
				Units:                 1,
				DebitCredits:          item.PlannedCredits,
				SupportsFailureRefund: true,
				ChargeMode:            "workflow_itemized",
				BillStatus:            billStatus,
				BillMessage:           billMessage,
			})
		}
	}
	return &store.ApplyUsageBillingResult{
		TotalCredits: totalCredits,
		BillStatus:   billStatus,
		BillMessage:  billMessage,
		Details:      details,
	}
}

// 处理工作流计费Result会话相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func WorkflowBillingResultFromSession(session *domain.AIBillingSession, items []domain.AIBillingItem) *store.ApplyUsageBillingResult {
	if session == nil {
		return nil
	}
	details := make([]store.UsageBillingDetail, 0)
	if session.SpecialPriceCredits != nil && *session.SpecialPriceCredits > 0 {
		details = append(details, store.UsageBillingDetail{
			MeterCode:             session.WorkflowCode,
			Quantity:              int64(plannedVideoSegmentCount(session.DurationSeconds, session.SegmentSeconds)),
			Units:                 1,
			DebitCredits:          session.BilledCredits,
			SupportsFailureRefund: true,
			ChargeMode:            "workflow_special_price",
			BillStatus:            session.Status,
			BillMessage:           strings.TrimSpace(valueOrEmptyString(session.Message)),
			PricingRuleID:         strings.TrimSpace(valueOrEmptyString(session.SpecialRuleID)),
		})
	} else {
		for _, item := range items {
			if item.BilledCredits <= 0 && item.RefundedCredits <= 0 {
				continue
			}
			details = append(details, store.UsageBillingDetail{
				MeterCode:             firstNonEmptyStringValue(valueOrEmptyString(item.MeterCode), item.ItemType),
				Quantity:              maxInt64Value(item.Quantity, 1),
				Units:                 1,
				DebitCredits:          item.BilledCredits,
				SupportsFailureRefund: true,
				ChargeMode:            "workflow_itemized",
				BillStatus:            item.Status,
				BillMessage:           strings.TrimSpace(valueOrEmptyString(session.Message)),
			})
		}
	}
	return &store.ApplyUsageBillingResult{
		TotalCredits:  session.BilledCredits,
		BillStatus:    session.Status,
		BillMessage:   strings.TrimSpace(valueOrEmptyString(session.Message)),
		AlreadyBilled: session.Status == "billed" || session.Status == "partially_refunded",
		Details:       details,
	}
}

// 解析工作流定价Snapshot，为AI作业提供结构化输入。
func parseWorkflowPricingSnapshot(job *domain.AIJob) *workflowPricingSnapshot {
	if job == nil {
		return nil
	}
	payload := decodePayloadMap(job.InputPayload)
	raw, ok := payload["workflowPricing"].(map[string]any)
	if !ok {
		return nil
	}

	snapshot := &workflowPricingSnapshot{
		RuleID:          strings.TrimSpace(stringValueFromMap(raw, "ruleId")),
		WorkflowCode:    strings.TrimSpace(stringValueFromMap(raw, "workflowCode")),
		OutputType:      strings.TrimSpace(stringValueFromMap(raw, "outputType")),
		DurationSeconds: intValue(raw["durationSeconds"]),
		SegmentSeconds:  intValue(raw["segmentSeconds"]),
	}
	if snapshot.WorkflowCode == "" {
		snapshot.WorkflowCode = strings.TrimSpace(stringValueFromMap(payload, "workflowCode"))
	}
	if snapshot.DurationSeconds <= 0 {
		snapshot.DurationSeconds = intValue(payload["durationSeconds"])
	}
	if snapshot.SegmentSeconds <= 0 {
		snapshot.SegmentSeconds = 8
	}
	if value, ok := int64Value(raw["specialPriceCredits"]); ok && value > 0 {
		snapshot.SpecialPriceCredits = &value
	}
	if !strings.EqualFold(snapshot.WorkflowCode, videoTextWorkflowCode) {
		return nil
	}
	return snapshot
}

// 构建工作流计费Item，为AI作业生成后续步骤所需的派生参数或载荷。
func buildWorkflowBillingItem(
	job *domain.AIJob,
	itemKey string,
	itemType string,
	label string,
	model *domain.AIModel,
	sortOrder int,
	payload map[string]any,
) store.CreateAIBillingItemInput {
	var (
		modelName  *string
		modelAlias *string
	)
	if model != nil {
		modelName = normalizeTrimmedStringPointer(model.ModelName)
		modelAlias = normalizeTrimmedStringPointer(model.ModelAlias)
	}
	plannedCredits := billingCreditsForModel(model)
	return store.CreateAIBillingItemInput{
		ID:               uuid.NewString(),
		UserID:           strings.TrimSpace(job.OwnerUserID),
		SourceType:       "ai_job",
		SourceID:         strings.TrimSpace(job.ID),
		ItemKey:          itemKey,
		ItemType:         itemType,
		Label:            label,
		ModelName:        modelName,
		ModelAlias:       modelAlias,
		Quantity:         1,
		Unit:             "call",
		UnitPriceCredits: plannedCredits,
		PlannedCredits:   plannedCredits,
		SortOrder:        sortOrder,
		IsVisible:        true,
		Status:           "precharged",
		Payload:          mustJSONBytes(payload),
	}
}

// 解析工作流分镜计费模型，根据当前配置和上下文确定最终使用结果。
func resolveWorkflowStoryboardBillingModel(ctx context.Context, app *appstate.App) (*domain.AIModel, error) {
	if app == nil {
		return nil, nil
	}
	settings, err := app.Store.GetAdminSystemSettings(ctx)
	if err != nil {
		return nil, err
	}
	fallbackModel := strings.TrimSpace(app.Config.DefaultChatModel)
	candidates := nonEmpty([]string{
		func() string {
			if settings == nil {
				return ""
			}
			return strings.TrimSpace(settings.StoryboardModel)
		}(),
		strings.TrimSpace(DefaultVideoStoryboardModelName),
		fallbackModel,
	})
	for _, candidate := range candidates {
		model, modelErr := app.Store.GetAIModelByName(ctx, candidate)
		if modelErr != nil {
			return nil, modelErr
		}
		if SupportsStoryboardPackageModel(model) {
			return model, nil
		}
	}
	return nil, nil
}

// 解析工作流封面计费模型，根据当前配置和上下文确定最终使用结果。
func resolveWorkflowCoverBillingModel(ctx context.Context, app *appstate.App) (*domain.AIModel, error) {
	if app == nil {
		return nil, nil
	}
	candidates := nonEmpty([]string{
		strings.TrimSpace(DefaultVideoCoverModelName),
		strings.TrimSpace(app.Config.DefaultImageModel),
	})
	for _, candidate := range candidates {
		model, err := app.Store.GetAIModelByName(ctx, candidate)
		if err != nil {
			return nil, err
		}
		if model != nil && model.IsEnabled && strings.EqualFold(strings.TrimSpace(model.Category), "image") {
			return model, nil
		}
	}
	return nil, nil
}

// 根据模型计算计费额度，供AI作业链路复用关键派生结果。
func billingCreditsForModel(model *domain.AIModel) int64 {
	if model == nil {
		return 0
	}
	for _, candidate := range []*float64{model.BillingAmount, model.RawRate} {
		if candidate == nil || *candidate <= 0 {
			continue
		}
		return int64(math.Ceil(*candidate))
	}
	return 0
}

// 处理planned视频分段数量相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func plannedVideoSegmentCount(durationSeconds int, segmentSeconds int) int {
	if durationSeconds <= 0 {
		return 0
	}
	if segmentSeconds <= 0 {
		segmentSeconds = 8
	}
	return int(math.Ceil(float64(durationSeconds) / float64(segmentSeconds)))
}

// 处理distribute额度相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func distributeCredits(totalCredits int64, itemCount int) []int64 {
	if itemCount <= 0 {
		return nil
	}
	base := totalCredits / int64(itemCount)
	remainder := totalCredits % int64(itemCount)
	result := make([]int64, 0, itemCount)
	for index := 0; index < itemCount; index++ {
		credits := base
		if int64(index) < remainder {
			credits++
		}
		result = append(result, credits)
	}
	return result
}

// 处理工作流分镜包Item键相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func workflowStoryboardPackageItemKey() string {
	return workflowStoryboardPackageItemType
}

// 处理工作流封面帧Item键相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func workflowCoverFrameItemKey() string {
	return workflowCoverFrameItemType
}

// 处理工作流视频分段Item键相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func workflowVideoSegmentItemKey(index int) string {
	return fmt.Sprintf("%s:%d", workflowVideoSegmentItemType, index)
}

// 处理工作流SpecialPrice分段Item键相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func workflowSpecialPriceSegmentItemKey(index int) string {
	return fmt.Sprintf("%s:%d", workflowSpecialPriceItemType, index)
}

// 处理首个Non空值String值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstNonEmptyStringValue(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// 处理int64值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func int64Value(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case string:
		parsed, err := json.Number(strings.TrimSpace(typed)).Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

// 处理值空值String相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func valueOrEmptyString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// 规范化裁剪StringPointer，统一AI作业链路的输入格式和后续处理行为。
func normalizeTrimmedStringPointer(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// 处理maxInt64值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func maxInt64Value(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
