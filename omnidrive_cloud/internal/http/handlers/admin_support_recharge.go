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

// 处理裁剪StringPtr相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func trimmedStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// 处理解码管理端支持充值载荷相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理查找支持充值值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理查找支持充值String相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func lookupSupportRechargeString(payload map[string]any, parents ...string) *string {
	value, _ := lookupSupportRechargeValue(payload, parents...).(string)
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// 处理查找支持充值Int64相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 解析支持充值Bonus额度，根据当前配置和上下文确定最终使用结果。
func resolveSupportRechargeBonusCredits(order domain.RechargeOrder, payload map[string]any) int64 {
	if order.ManualBonusCreditAmount > 0 {
		return order.ManualBonusCreditAmount
	}
	if value := lookupSupportRechargeInt64(payload, "credits", "manualBonusCreditAmount"); value != nil {
		return *value
	}
	return 0
}

// 处理查找支持充值时间相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理查找支持充值Strings相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 规范化支持充值状态，统一管理端支持充值链路的输入格式和后续处理行为。
func normalizeSupportRechargeStatus(order domain.RechargeOrder, payload map[string]any) string {
	raw := strings.TrimSpace(order.Status)
	reviewStatus := ""
	if value := lookupSupportRechargeString(payload, "review", "status"); value != nil {
		reviewStatus = strings.TrimSpace(*value)
	}

	switch raw {
	case "awaiting_manual_review", "processing":
		return "pending_review"
	case "credited", "paid", "success", "completed":
		return "credited"
	case "closed":
		if reviewStatus == "invalidated" || strings.TrimSpace(valueOrEmpty(order.ProviderStatus)) == "manual_invalidated" {
			return "invalidated"
		}
		return "closed"
	default:
		if reviewStatus == "invalidated" {
			return "invalidated"
		}
		return raw
	}
}

// 处理管理端支持充值动作相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func adminSupportRechargeActions(status string) domain.AdminSupportRechargeActions {
	switch status {
	case "pending_review":
		return domain.AdminSupportRechargeActions{
			CanCredit:     true,
			CanReject:     true,
			CanInvalidate: true,
		}
	case "rejected":
		return domain.AdminSupportRechargeActions{
			CanInvalidate: true,
		}
	default:
		return domain.AdminSupportRechargeActions{}
	}
}

// 构建管理端支持充值行，为管理端支持充值生成后续步骤所需的派生参数或载荷。
func buildAdminSupportRechargeRow(item domain.AdminOrderRow) domain.AdminSupportRechargeRow {
	payload := decodeAdminSupportRechargePayload(item.Order.CustomerServicePayload)
	status := normalizeSupportRechargeStatus(item.Order, payload)
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

	bonusCredits := resolveSupportRechargeBonusCredits(item.Order, payload)

	return domain.AdminSupportRechargeRow{
		ID:             item.Order.ID,
		OrderNo:        item.Order.OrderNo,
		User:           item.User,
		RawStatus:      item.Order.Status,
		Status:         status,
		AmountCents:    item.Order.AmountCents,
		BaseCredits:    item.Order.CreditAmount,
		BonusCredits:   bonusCredits,
		TotalCredits:   item.Order.CreditAmount + bonusCredits,
		SubmittedAt:    *submittedAt,
		ReviewedAt:     reviewedAt,
		CreditedAt:     creditedAt,
		ProviderStatus: item.Order.ProviderStatus,
		Note:           note,
	}
}

// 构建管理端支持充值详情，为管理端支持充值生成后续步骤所需的派生参数或载荷。
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

	submissionStatus := "submitted"
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
	} else if record.Status == "invalidated" {
		reviewStatus = "invalidated"
	}

	return domain.AdminSupportRechargeDetail{
		Record: record,
		Order:  item.Order,
		User:   item.User,
		Submission: domain.AdminSupportRechargeSubmission{
			Status:         submissionStatus,
			ContactChannel: lookupSupportRechargeString(payload, "submission", "contactChannel"),
			ContactHandle:  lookupSupportRechargeString(payload, "submission", "contactHandle"),
			PaymentReference: func() *string {
				if value := lookupSupportRechargeString(payload, "submission", "paymentReference"); value != nil {
					return value
				}
				return stringPtr(item.Order.OrderNo)
			}(),
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

// 处理管理端Console加载管理端支持充值详情接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理管理端Console详情支持充值接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理管理端Console查找支持充值接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminConsoleHandler) LookupSupportRecharge(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		code = strings.TrimSpace(r.URL.Query().Get("rechargeCode"))
	}
	if code == "" {
		render.Error(w, http.StatusBadRequest, "code is required")
		return
	}

	item, err := h.app.Store.GetAdminOrderByOrderNo(r.Context(), code)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load support recharge")
		return
	}
	if item == nil || item.Order.Channel != "manual_cs" {
		render.Error(w, http.StatusNotFound, "Support recharge not found")
		return
	}

	events, err := h.app.Store.ListRechargeOrderEvents(r.Context(), item.Order.UserID, item.Order.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load support recharge")
		return
	}

	detail := buildAdminSupportRechargeDetail(item, events)
	render.JSON(w, http.StatusOK, detail)
}

// 处理管理端Console列表支持充值事件接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理管理端Console额度支持充值接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理管理端ConsoleReject支持充值接口，解析请求参数并调用应用状态或存储层完成业务动作。
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

// 处理管理端ConsoleInvalidate支持充值接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminConsoleHandler) InvalidateSupportRecharge(w http.ResponseWriter, r *http.Request) {
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
	order, err := h.app.Store.InvalidateSupportRecharge(r.Context(), orderID, store.InvalidateSupportRechargeInput{
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
			errors.Is(err, store.ErrRechargeOrderAlreadyClosed):
			render.Error(w, http.StatusConflict, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to invalidate support recharge")
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
		Action:       "manual_invalidate",
		Title:        "客服充值失效关闭",
		Source:       "admin_console",
		Status:       "closed",
		Message:      auditStringPtr(strings.TrimSpace(payload.Note)),
		Payload: mustJSONBytes(map[string]any{
			"orderId": order.ID,
			"orderNo": order.OrderNo,
		}),
	})

	render.JSON(w, http.StatusOK, detail)
}
