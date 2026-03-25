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

func previewAIJobBilling(ctx context.Context, app *appstate.App, job *domain.AIJob) (*store.ApplyUsageBillingResult, error) {
	if app == nil || job == nil {
		return &store.ApplyUsageBillingResult{
			BillStatus: "skipped",
			Details:    []store.UsageBillingDetail{},
		}, nil
	}
	return app.Store.PreviewUsageBilling(ctx, aiclient.BuildEstimatedUsageBillingInput(job))
}

func usageBillingShouldBlock(result *store.ApplyUsageBillingResult) bool {
	return result != nil && result.BillStatus == "failed"
}

func renderUsageBillingBlocked(w http.ResponseWriter, result *store.ApplyUsageBillingResult) {
	render.ErrorWithFields(w, http.StatusPaymentRequired, aiclient.BuildUsageBillingBlockMessage(result), map[string]any{
		"needsRecharge": true,
		"billing":       result,
	})
}
