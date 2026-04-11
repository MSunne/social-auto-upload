package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	aiclient "omnidrive_cloud/internal/ai"
	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

// 处理预览AI作业计费相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func previewAIJobBilling(ctx context.Context, app *appstate.App, job *domain.AIJob) (*store.ApplyUsageBillingResult, error) {
	if app == nil || job == nil {
		return &store.ApplyUsageBillingResult{
			BillStatus: "skipped",
			Details:    []store.UsageBillingDetail{},
		}, nil
	}
	if isDigitalHumanWorkflowInput(job.InputPayload) {
		return previewDigitalHumanWorkflowBilling(ctx, app, job)
	}
	workflowPreview, err := aiclient.PreviewWorkflowBilling(ctx, app, job)
	if err != nil {
		return nil, err
	}
	if workflowPreview != nil {
		return workflowPreview, nil
	}
	return app.Store.PreviewUsageBilling(ctx, aiclient.BuildEstimatedUsageBillingInput(job))
}

// 处理用量计费判断是否应当Block相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func usageBillingShouldBlock(result *store.ApplyUsageBillingResult) bool {
	return result != nil && result.BillStatus == "failed"
}

// 处理渲染用量计费阻塞相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func renderUsageBillingBlocked(w http.ResponseWriter, result *store.ApplyUsageBillingResult) {
	render.ErrorWithFields(w, http.StatusPaymentRequired, aiclient.BuildUsageBillingBlockMessage(result), map[string]any{
		"needsRecharge": true,
		"billing":       result,
	})
}

func isDigitalHumanWorkflowInput(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(stringValueFromBillingAny(payload["workflowKind"])), "digital_human")
}

func stringValueFromBillingAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case *string:
		if typed == nil {
			return ""
		}
		return *typed
	default:
		return ""
	}
}

func previewDigitalHumanWorkflowBilling(ctx context.Context, app *appstate.App, job *domain.AIJob) (*store.ApplyUsageBillingResult, error) {
	goodsText := extractDigitalHumanGoodsText(job.InputPayload)
	preview, err := (&DigitalHumanTaskHandler{app: app}).buildBillingPreviewFromSettings(ctx, strings.TrimSpace(job.OwnerUserID), goodsText)
	if err != nil {
		return nil, err
	}
	result := &store.ApplyUsageBillingResult{
		TotalCredits: int64(preview.EstimatedCredits),
		BillStatus:   "skipped",
		BillMessage:  "数字人口播将在执行时按数字人任务链路预扣积分",
		Details:      []store.UsageBillingDetail{},
	}
	if !preview.CanAfford {
		result.BillStatus = "failed"
		result.BillMessage = "当前积分不足，无法执行数字人口播"
	}
	return result, nil
}

func extractDigitalHumanGoodsText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	config, _ := payload["digitalHumanConfig"].(map[string]any)
	if config == nil {
		return ""
	}
	return strings.TrimSpace(stringValueFromBillingAny(config["goodsText"]))
}
