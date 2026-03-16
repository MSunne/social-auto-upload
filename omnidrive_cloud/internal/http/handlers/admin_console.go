package handlers

import (
	"net/http"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AdminConsoleHandler struct {
	app *appstate.App
}

func NewAdminConsoleHandler(app *appstate.App) *AdminConsoleHandler {
	return &AdminConsoleHandler{app: app}
}

func (h *AdminConsoleHandler) DashboardSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.app.Store.GetAdminDashboardSummary(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin dashboard summary")
		return
	}
	render.JSON(w, http.StatusOK, summary)
}

func (h *AdminConsoleHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, err := h.app.Store.ListAdminUsers(r.Context(), store.AdminUserListFilter{
		Query:  strings.TrimSpace(r.URL.Query().Get("query")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin users")
		return
	}

	renderAdminList(w, page, total, items, nil, map[string]any{
		"query":         strings.TrimSpace(r.URL.Query().Get("query")),
		"status":        strings.TrimSpace(r.URL.Query().Get("status")),
		"statusOptions": []string{"active", "inactive"},
	})
}

func (h *AdminConsoleHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, err := h.app.Store.ListAdminDevices(r.Context(), store.AdminDeviceListFilter{
		Query:  strings.TrimSpace(r.URL.Query().Get("query")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin devices")
		return
	}

	renderAdminList(w, page, total, items, nil, map[string]any{
		"query":         strings.TrimSpace(r.URL.Query().Get("query")),
		"status":        strings.TrimSpace(r.URL.Query().Get("status")),
		"statusOptions": []string{"online", "offline"},
	})
}

func (h *AdminConsoleHandler) ListPricingPackages(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.Store.ListBillingPackages(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load pricing packages")
		return
	}
	page := adminPageQuery{Page: 1, PageSize: max(1, len(items))}
	renderAdminList(w, page, int64(len(items)), items, map[string]any{
		"enabledCount": len(items),
	}, nil)
}

func (h *AdminConsoleHandler) ListPricingRules(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.Store.ListBillingPricingRules(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load pricing rules")
		return
	}
	page := adminPageQuery{Page: 1, PageSize: max(1, len(items))}
	renderAdminList(w, page, int64(len(items)), items, map[string]any{
		"enabledCount": len(items),
	}, nil)
}

func (h *AdminConsoleHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, summary, err := h.app.Store.ListAdminOrders(r.Context(), store.AdminOrderListFilter{
		Query:   strings.TrimSpace(r.URL.Query().Get("query")),
		Status:  strings.TrimSpace(r.URL.Query().Get("status")),
		Channel: strings.TrimSpace(r.URL.Query().Get("channel")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin orders")
		return
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":          strings.TrimSpace(r.URL.Query().Get("query")),
		"status":         strings.TrimSpace(r.URL.Query().Get("status")),
		"channel":        strings.TrimSpace(r.URL.Query().Get("channel")),
		"channelOptions": []string{"alipay", "wechatpay", "manual_cs"},
	})
}

func (h *AdminConsoleHandler) ListWalletLedgers(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, summary, err := h.app.Store.ListAdminWalletLedgers(r.Context(), store.AdminWalletLedgerListFilter{
		Query:     strings.TrimSpace(r.URL.Query().Get("query")),
		EntryType: strings.TrimSpace(r.URL.Query().Get("entryType")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin wallet ledgers")
		return
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":     strings.TrimSpace(r.URL.Query().Get("query")),
		"entryType": strings.TrimSpace(r.URL.Query().Get("entryType")),
	})
}

func (h *AdminConsoleHandler) ListSupportRecharges(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	requestedStatus := strings.TrimSpace(r.URL.Query().Get("status"))
	orderStatus := requestedStatus
	switch requestedStatus {
	case "awaiting_submission":
		orderStatus = "awaiting_manual_review"
	case "pending_review":
		orderStatus = "processing"
	case "rejected":
		orderStatus = "rejected"
	case "credited":
		orderStatus = "credited"
	}

	orderItems, total, orderSummary, err := h.app.Store.ListAdminOrders(r.Context(), store.AdminOrderListFilter{
		Query:   strings.TrimSpace(r.URL.Query().Get("query")),
		Status:  orderStatus,
		Channel: "manual_cs",
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load support recharge requests")
		return
	}

	items := make([]domain.AdminSupportRechargeRow, 0, len(orderItems))
	for _, row := range orderItems {
		items = append(items, buildAdminSupportRechargeRow(row))
	}

	summary := domain.AdminSupportRechargeSummary{
		AwaitingSubmissionCount:   orderSummary.AwaitingManualReviewCount,
		PendingReviewCount:        orderSummary.ProcessingCount,
		RejectedCount:             orderSummary.RejectedCount,
		CreditedCount:             orderSummary.PaidOrderCount,
		TotalRequestedAmountCents: orderSummary.TotalAmountCents,
		TotalBaseCredits:          orderSummary.TotalCreditAmount,
		TotalBonusCredits:         0,
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":         strings.TrimSpace(r.URL.Query().Get("query")),
		"status":        requestedStatus,
		"statusOptions": []string{"awaiting_submission", "pending_review", "credited", "rejected"},
	})
}

func (h *AdminConsoleHandler) ListDistributionRelations(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	renderAdminList(w, page, 0, []domain.AdminDistributionRelationRow{}, map[string]any{
		"enabled": false,
		"phase":   "schema_pending",
	}, nil)
}

func (h *AdminConsoleHandler) ListDistributionCommissions(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	renderAdminList(w, page, 0, []domain.AdminCommissionRow{}, map[string]any{
		"pendingConsumeAmountCents":    0,
		"pendingSettlementAmountCents": 0,
		"settledAmountCents":           0,
		"enabled":                      false,
		"phase":                        "schema_pending",
	}, nil)
}

func (h *AdminConsoleHandler) ListDistributionSettlements(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	renderAdminList(w, page, 0, []domain.AdminSettlementRow{}, map[string]any{
		"enabled": false,
		"phase":   "schema_pending",
	}, nil)
}

func (h *AdminConsoleHandler) ListWithdrawals(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	renderAdminList(w, page, 0, []domain.AdminWithdrawalRow{}, map[string]any{
		"enabled": false,
		"phase":   "schema_pending",
	}, nil)
}

func (h *AdminConsoleHandler) ListAudits(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, err := h.app.Store.ListAdminAudits(r.Context(), store.AdminAuditListFilter{
		Query:        strings.TrimSpace(r.URL.Query().Get("query")),
		ResourceType: strings.TrimSpace(r.URL.Query().Get("resourceType")),
		Status:       strings.TrimSpace(r.URL.Query().Get("status")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin audits")
		return
	}

	renderAdminList(w, page, total, items, nil, map[string]any{
		"query":        strings.TrimSpace(r.URL.Query().Get("query")),
		"resourceType": strings.TrimSpace(r.URL.Query().Get("resourceType")),
		"status":       strings.TrimSpace(r.URL.Query().Get("status")),
	})
}
