package handlers

import (
	"context"
	"net/http"

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
