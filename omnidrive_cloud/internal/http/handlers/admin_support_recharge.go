package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type adminSupportRechargeDecisionRequest struct {
	Note             string `json:"note"`
	PaymentReference string `json:"paymentReference"`
}

func trimmedStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func decodeAdminSupportRechargePayload(raw []byte) map[string]any {
	payload := map[string]any{}
	if len(raw) == 0 {
		return payload
	}
	_ = json.Unmarshal(raw, &payload)
	if payload == nil {
		return map[string]any{}
	}
	return payload
}

func lookupSupportRechargeValue(payload map[string]any, parents ...string) any {
	var current any = payload
	for _, key := range parents {
		nested, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = nested[key]
	}
	return current
}

func lookupSupportRechargeString(payload map[string]any, parents ...string) *string {
	value, _ := lookupSupportRechargeValue(payload, parents...).(string)
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func lookupSupportRechargeInt64(payload map[string]any, parents ...string) *int64 {
	value := lookupSupportRechargeValue(payload, parents...)
	switch typed := value.(type) {
	case float64:
		result := int64(typed)
		return &result
	case int64:
		result := typed
		return &result
	case int:
		result := int64(typed)
		return &result
	default:
		return nil
	}
}

func lookupSupportRechargeTime(payload map[string]any, parents ...string) *time.Time {
	value, _ := lookupSupportRechargeValue(payload, parents...).(string)
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}

func lookupSupportRechargeStrings(payload map[string]any, parents ...string) []string {
	rawItems, ok := lookupSupportRechargeValue(payload, parents...).([]any)
	if !ok {
		return []string{}
	}
	items := make([]string, 0, len(rawItems))
	for _, raw := range rawItems {
		value, _ := raw.(string)
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	return items
}

func normalizeSupportRechargeStatus(raw string) string {
	switch strings.TrimSpace(raw) {
	case "awaiting_manual_review":
		return "awaiting_submission"
	case "processing":
		return "pending_review"
	case "credited", "paid", "success", "completed":
		return "credited"
	default:
		return strings.TrimSpace(raw)
	}
}

func adminSupportRechargeActions(status string) domain.AdminSupportRechargeActions {
	switch status {
	case "pending_review":
		return domain.AdminSupportRechargeActions{
			CanCredit: true,
			CanReject: true,
		}
	default:
		return domain.AdminSupportRechargeActions{}
	}
}

func buildAdminSupportRechargeRow(item domain.AdminOrderRow) domain.AdminSupportRechargeRow {
	payload := decodeAdminSupportRechargePayload(item.Order.CustomerServicePayload)
	status := normalizeSupportRechargeStatus(item.Order.Status)
	submittedAt := lookupSupportRechargeTime(payload, "submission", "submittedAt")
	if submittedAt == nil {
		submittedAt = &item.Order.CreatedAt
	}

	reviewedAt := lookupSupportRechargeTime(payload, "review", "reviewedAt")
	creditedAt := lookupSupportRechargeTime(payload, "review", "creditedAt")
	if reviewedAt == nil && status == "credited" {
		reviewedAt = creditedAt
	}
	if reviewedAt == nil && item.Order.PaidAt != nil && status == "credited" {
		reviewedAt = item.Order.PaidAt
	}
	if creditedAt == nil && item.Order.PaidAt != nil && status == "credited" {
		creditedAt = item.Order.PaidAt
	}

	note := lookupSupportRechargeString(payload, "review", "note")
	if note == nil {
		note = lookupSupportRechargeString(payload, "submission", "customerNote")
	}
	if note == nil {
		note = item.Order.Body
	}

	return domain.AdminSupportRechargeRow{
		ID:             item.Order.ID,
		OrderNo:        item.Order.OrderNo,
		User:           item.User,
		RawStatus:      item.Order.Status,
		Status:         status,
		AmountCents:    item.Order.AmountCents,
		BaseCredits:    item.Order.CreditAmount,
		BonusCredits:   0,
		TotalCredits:   item.Order.CreditAmount,
		SubmittedAt:    *submittedAt,
		ReviewedAt:     reviewedAt,
		CreditedAt:     creditedAt,
		ProviderStatus: item.Order.ProviderStatus,
		Note:           note,
	}
}

func buildAdminSupportRechargeDetail(item *domain.AdminOrderRow, events []domain.RechargeOrderEvent) domain.AdminSupportRechargeDetail {
	payload := decodeAdminSupportRechargePayload(item.Order.CustomerServicePayload)
	record := buildAdminSupportRechargeRow(*item)
	reviewedAt := lookupSupportRechargeTime(payload, "review", "reviewedAt")
	if reviewedAt == nil {
		reviewedAt = record.ReviewedAt
	}
	creditedAt := lookupSupportRechargeTime(payload, "review", "creditedAt")
	if creditedAt == nil {
		creditedAt = record.CreditedAt
	}

	submissionStatus := "pending"
	if value := lookupSupportRechargeString(payload, "submission", "status"); value != nil {
		submissionStatus = *value
	}

	reviewStatus := "pending"
	if value := lookupSupportRechargeString(payload, "review", "status"); value != nil {
		reviewStatus = *value
	} else if record.Status == "credited" {
		reviewStatus = "credited"
	} else if record.Status == "rejected" {
		reviewStatus = "rejected"
	}

	return domain.AdminSupportRechargeDetail{
		Record: record,
		Order:  item.Order,
		User:   item.User,
		Submission: domain.AdminSupportRechargeSubmission{
			Status:              submissionStatus,
			ContactChannel:      lookupSupportRechargeString(payload, "submission", "contactChannel"),
			ContactHandle:       lookupSupportRechargeString(payload, "submission", "contactHandle"),
			PaymentReference:    lookupSupportRechargeString(payload, "submission", "paymentReference"),
			TransferAmountCents: lookupSupportRechargeInt64(payload, "submission", "transferAmountCents"),
			ProofURLs:           lookupSupportRechargeStrings(payload, "submission", "proofUrls"),
			CustomerNote:        lookupSupportRechargeString(payload, "submission", "customerNote"),
			SubmittedAt:         lookupSupportRechargeTime(payload, "submission", "submittedAt"),
		},
		Review: domain.AdminSupportRechargeReview{
			Status:        reviewStatus,
			OperatorID:    lookupSupportRechargeString(payload, "review", "operatorId"),
			OperatorName:  lookupSupportRechargeString(payload, "review", "operatorName"),
			OperatorEmail: lookupSupportRechargeString(payload, "review", "operatorEmail"),
			Note:          lookupSupportRechargeString(payload, "review", "note"),
			ReviewedAt:    reviewedAt,
			CreditedAt:    creditedAt,
		},
		Events:  events,
		Actions: adminSupportRechargeActions(record.Status),
	}
}

func (h *AdminConsoleHandler) loadAdminSupportRechargeDetail(r *http.Request, orderID string) (*domain.AdminSupportRechargeDetail, error) {
	item, err := h.app.Store.GetAdminOrderByID(r.Context(), orderID)
	if err != nil {
		return nil, err
	}
	if item == nil || item.Order.Channel != "manual_cs" {
		return nil, nil
	}

	events, err := h.app.Store.ListRechargeOrderEvents(r.Context(), item.Order.UserID, orderID)
	if err != nil {
		return nil, err
	}

	detail := buildAdminSupportRechargeDetail(item, events)
	return &detail, nil
}

func (h *AdminConsoleHandler) DetailSupportRecharge(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
	if orderID == "" {
		render.Error(w, http.StatusBadRequest, "orderId is required")
		return
	}

	detail, err := h.loadAdminSupportRechargeDetail(r, orderID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load support recharge")
		return
	}
	if detail == nil {
		render.Error(w, http.StatusNotFound, "Support recharge not found")
		return
	}
	render.JSON(w, http.StatusOK, detail)
}

func (h *AdminConsoleHandler) ListSupportRechargeEvents(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
	if orderID == "" {
		render.Error(w, http.StatusBadRequest, "orderId is required")
		return
	}

	detail, err := h.loadAdminSupportRechargeDetail(r, orderID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load support recharge events")
		return
	}
	if detail == nil {
		render.Error(w, http.StatusNotFound, "Support recharge not found")
		return
	}
	render.JSON(w, http.StatusOK, detail.Events)
}

func (h *AdminConsoleHandler) CreditSupportRecharge(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
	if orderID == "" {
		render.Error(w, http.StatusBadRequest, "orderId is required")
		return
	}

	var payload adminSupportRechargeDecisionRequest
	if r.ContentLength != 0 {
		if err := render.DecodeJSON(r, &payload); err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	order, err := h.app.Store.CreditSupportRecharge(r.Context(), orderID, store.CreditSupportRechargeInput{
		AdminID:          admin.ID,
		AdminEmail:       admin.Email,
		AdminName:        admin.Name,
		Note:             trimmedStringPtr(payload.Note),
		PaymentReference: trimmedStringPtr(payload.PaymentReference),
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRechargeOrderNotFound):
			render.Error(w, http.StatusNotFound, "Support recharge not found")
		case errors.Is(err, store.ErrRechargeOrderNotManual),
			errors.Is(err, store.ErrRechargeOrderAlreadyCredited),
			errors.Is(err, store.ErrRechargeOrderNotPendingReview):
			render.Error(w, http.StatusConflict, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to credit support recharge")
		}
		return
	}

	detail, err := h.loadAdminSupportRechargeDetail(r, order.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load support recharge detail")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  order.UserID,
		ResourceType: "support_recharge",
		ResourceID:   &order.ID,
		Action:       "manual_credit",
		Title:        "客服充值确认入账",
		Source:       "admin_console",
		Status:       "credited",
		Message:      auditStringPtr(strings.TrimSpace(payload.Note)),
		Payload: mustJSONBytes(map[string]any{
			"orderId":          order.ID,
			"orderNo":          order.OrderNo,
			"paymentReference": strings.TrimSpace(payload.PaymentReference),
		}),
	})

	render.JSON(w, http.StatusOK, detail)
}

func (h *AdminConsoleHandler) RejectSupportRecharge(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
	if orderID == "" {
		render.Error(w, http.StatusBadRequest, "orderId is required")
		return
	}

	var payload adminSupportRechargeDecisionRequest
	if r.ContentLength != 0 {
		if err := render.DecodeJSON(r, &payload); err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	order, err := h.app.Store.RejectSupportRecharge(r.Context(), orderID, store.RejectSupportRechargeInput{
		AdminID:    admin.ID,
		AdminEmail: admin.Email,
		AdminName:  admin.Name,
		Note:       trimmedStringPtr(payload.Note),
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRechargeOrderNotFound):
			render.Error(w, http.StatusNotFound, "Support recharge not found")
		case errors.Is(err, store.ErrRechargeOrderNotManual),
			errors.Is(err, store.ErrRechargeOrderAlreadyCredited),
			errors.Is(err, store.ErrRechargeOrderNotPendingReview):
			render.Error(w, http.StatusConflict, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to reject support recharge")
		}
		return
	}

	detail, err := h.loadAdminSupportRechargeDetail(r, order.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load support recharge detail")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  order.UserID,
		ResourceType: "support_recharge",
		ResourceID:   &order.ID,
		Action:       "manual_reject",
		Title:        "客服充值驳回",
		Source:       "admin_console",
		Status:       "rejected",
		Message:      auditStringPtr(strings.TrimSpace(payload.Note)),
		Payload: mustJSONBytes(map[string]any{
			"orderId": order.ID,
			"orderNo": order.OrderNo,
		}),
	})

	render.JSON(w, http.StatusOK, detail)
}
