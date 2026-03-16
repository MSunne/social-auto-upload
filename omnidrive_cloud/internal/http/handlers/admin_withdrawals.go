package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AdminWithdrawalHandler struct {
	app *appstate.App
}

type reviewWithdrawalRequest struct {
	Note             *string  `json:"note"`
	PaymentReference *string  `json:"paymentReference"`
	ProofURLs        []string `json:"proofUrls"`
}

func NewAdminWithdrawalHandler(app *appstate.App) *AdminWithdrawalHandler {
	return &AdminWithdrawalHandler{app: app}
}

func (h *AdminWithdrawalHandler) ListWithdrawals(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, summary, err := h.app.Store.ListAdminWithdrawals(r.Context(), store.AdminWithdrawalListFilter{
		Query:  strings.TrimSpace(r.URL.Query().Get("query")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load withdrawals")
		return
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":         strings.TrimSpace(r.URL.Query().Get("query")),
		"status":        strings.TrimSpace(r.URL.Query().Get("status")),
		"statusOptions": []string{"requested", "approved", "rejected", "paid"},
	})
}

func (h *AdminWithdrawalHandler) DetailWithdrawal(w http.ResponseWriter, r *http.Request) {
	withdrawalID := strings.TrimSpace(chi.URLParam(r, "withdrawalId"))
	if withdrawalID == "" {
		render.Error(w, http.StatusBadRequest, "withdrawalId is required")
		return
	}

	record, err := h.app.Store.GetAdminWithdrawalByID(r.Context(), withdrawalID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load withdrawal detail")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Withdrawal request not found")
		return
	}
	render.JSON(w, http.StatusOK, record)
}

func (h *AdminWithdrawalHandler) ApproveWithdrawal(w http.ResponseWriter, r *http.Request) {
	h.handleWithdrawalReviewAction(w, r, "approve")
}

func (h *AdminWithdrawalHandler) RejectWithdrawal(w http.ResponseWriter, r *http.Request) {
	h.handleWithdrawalReviewAction(w, r, "reject")
}

func (h *AdminWithdrawalHandler) MarkWithdrawalPaid(w http.ResponseWriter, r *http.Request) {
	h.handleWithdrawalReviewAction(w, r, "mark_paid")
}

func (h *AdminWithdrawalHandler) handleWithdrawalReviewAction(w http.ResponseWriter, r *http.Request, action string) {
	withdrawalID := strings.TrimSpace(chi.URLParam(r, "withdrawalId"))
	if withdrawalID == "" {
		render.Error(w, http.StatusBadRequest, "withdrawalId is required")
		return
	}

	var payload reviewWithdrawalRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	input := store.ReviewWithdrawalInput{
		AdminID:          admin.ID,
		AdminEmail:       admin.Email,
		AdminName:        admin.Name,
		Note:             trimmedStringPtr(valueOrEmpty(payload.Note)),
		PaymentReference: trimmedStringPtr(valueOrEmpty(payload.PaymentReference)),
		ProofURLs:        payload.ProofURLs,
	}

	var (
		record *domain.AdminWithdrawalDetail
		err    error
		title  string
	)
	switch action {
	case "approve":
		record, err = h.app.Store.ApproveWithdrawal(r.Context(), withdrawalID, input)
		title = "审核通过提现申请"
	case "reject":
		record, err = h.app.Store.RejectWithdrawal(r.Context(), withdrawalID, input)
		title = "驳回提现申请"
	case "mark_paid":
		record, err = h.app.Store.MarkWithdrawalPaid(r.Context(), withdrawalID, input)
		title = "标记提现已打款"
	default:
		render.Error(w, http.StatusBadRequest, "unsupported withdrawal action")
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, store.ErrWithdrawalNotFound):
			render.Error(w, http.StatusNotFound, "Withdrawal request not found")
		case errors.Is(err, store.ErrWithdrawalInvalidTransition):
			render.Error(w, http.StatusConflict, err.Error())
		case errors.Is(err, store.ErrWithdrawalInsufficientBalance):
			render.Error(w, http.StatusConflict, err.Error())
		default:
			render.Error(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "withdrawal_request",
		ResourceID:   stringPtr(withdrawalID),
		Action:       action,
		Title:        title,
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("提现申请状态已更新"),
		Payload: mustJSONBytes(map[string]any{
			"status":           record.Record.Status,
			"amountCents":      record.Record.AmountCents,
			"paymentReference": valueOrEmpty(payload.PaymentReference),
			"proofUrls":        payload.ProofURLs,
		}),
	})

	render.JSON(w, http.StatusOK, record)
}
