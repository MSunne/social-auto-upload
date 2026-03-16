package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AdminFinanceHandler struct {
	app *appstate.App
}

type adminBillingPackageEntitlementRequest struct {
	MeterCode   string  `json:"meterCode"`
	GrantAmount int64   `json:"grantAmount"`
	GrantMode   string  `json:"grantMode"`
	SortOrder   int     `json:"sortOrder"`
	Description *string `json:"description"`
}

type createAdminBillingPackageRequest struct {
	ID                      string                                  `json:"id"`
	Name                    string                                  `json:"name"`
	PackageType             string                                  `json:"packageType"`
	PaymentChannels         []string                                `json:"paymentChannels"`
	Currency                string                                  `json:"currency"`
	PriceCents              int64                                   `json:"priceCents"`
	CreditAmount            int64                                   `json:"creditAmount"`
	ManualBonusCreditAmount int64                                   `json:"manualBonusCreditAmount"`
	Badge                   *string                                 `json:"badge"`
	Description             *string                                 `json:"description"`
	PricingPayload          json.RawMessage                         `json:"pricingPayload"`
	ExpiresInDays           *int32                                  `json:"expiresInDays"`
	IsEnabled               *bool                                   `json:"isEnabled"`
	SortOrder               *int                                    `json:"sortOrder"`
	Entitlements            []adminBillingPackageEntitlementRequest `json:"entitlements"`
}

type updateAdminBillingPackageRequest struct {
	Name                    *string                                 `json:"name"`
	PackageType             *string                                 `json:"packageType"`
	PaymentChannels         []string                                `json:"paymentChannels"`
	Currency                *string                                 `json:"currency"`
	PriceCents              *int64                                  `json:"priceCents"`
	CreditAmount            *int64                                  `json:"creditAmount"`
	ManualBonusCreditAmount *int64                                  `json:"manualBonusCreditAmount"`
	Badge                   *string                                 `json:"badge"`
	Description             *string                                 `json:"description"`
	PricingPayload          json.RawMessage                         `json:"pricingPayload"`
	ExpiresInDays           *int32                                  `json:"expiresInDays"`
	IsEnabled               *bool                                   `json:"isEnabled"`
	SortOrder               *int                                    `json:"sortOrder"`
	Entitlements            []adminBillingPackageEntitlementRequest `json:"entitlements"`
}

type createWalletAdjustmentRequest struct {
	UserID        string          `json:"userId"`
	AmountDelta   int64           `json:"amountDelta"`
	Reason        string          `json:"reason"`
	Note          *string         `json:"note"`
	EntryType     *string         `json:"entryType"`
	ReferenceType *string         `json:"referenceType"`
	ReferenceID   *string         `json:"referenceId"`
	Payload       json.RawMessage `json:"payload"`
}

func NewAdminFinanceHandler(app *appstate.App) *AdminFinanceHandler {
	return &AdminFinanceHandler{app: app}
}

func decodeAdminFinanceRequest(r *http.Request, destination any) (map[string]json.RawMessage, error) {
	if r.Body == nil {
		return nil, errors.New("empty request body")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, errors.New("empty request body")
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return nil, err
	}

	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func financeFieldTouched(raw map[string]json.RawMessage, key string) bool {
	_, exists := raw[key]
	return exists
}

func toStoreBillingEntitlements(items []adminBillingPackageEntitlementRequest) []store.BillingPackageEntitlementInput {
	result := make([]store.BillingPackageEntitlementInput, 0, len(items))
	for _, item := range items {
		result = append(result, store.BillingPackageEntitlementInput{
			MeterCode:   strings.TrimSpace(item.MeterCode),
			GrantAmount: item.GrantAmount,
			GrantMode:   strings.TrimSpace(item.GrantMode),
			SortOrder:   item.SortOrder,
			Description: trimmedStringPtr(valueOrEmpty(item.Description)),
		})
	}
	return result
}

func (h *AdminFinanceHandler) DetailOrder(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
	if orderID == "" {
		render.Error(w, http.StatusBadRequest, "orderId is required")
		return
	}

	record, err := h.app.Store.GetAdminOrderByID(r.Context(), orderID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load order detail")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Order not found")
		return
	}

	events, err := h.app.Store.ListRechargeOrderEvents(r.Context(), record.Order.UserID, orderID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load order events")
		return
	}

	paymentTransactions, err := h.app.Store.ListPaymentTransactionsByRechargeOrderID(r.Context(), orderID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load payment transactions")
		return
	}

	walletLedgers, err := h.app.Store.ListWalletLedgersByRechargeOrderID(r.Context(), orderID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load wallet ledgers")
		return
	}

	render.JSON(w, http.StatusOK, domain.AdminOrderDetail{
		Record:              *record,
		Events:              events,
		PaymentTransactions: paymentTransactions,
		WalletLedgers:       walletLedgers,
	})
}

func (h *AdminFinanceHandler) ListOrderEvents(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
	if orderID == "" {
		render.Error(w, http.StatusBadRequest, "orderId is required")
		return
	}

	record, err := h.app.Store.GetAdminOrderByID(r.Context(), orderID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load order")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Order not found")
		return
	}

	items, err := h.app.Store.ListRechargeOrderEvents(r.Context(), record.Order.UserID, orderID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load order events")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AdminFinanceHandler) DetailWalletLedger(w http.ResponseWriter, r *http.Request) {
	ledgerID := strings.TrimSpace(chi.URLParam(r, "ledgerId"))
	if ledgerID == "" {
		render.Error(w, http.StatusBadRequest, "ledgerId is required")
		return
	}

	record, err := h.app.Store.GetAdminWalletLedgerByID(r.Context(), ledgerID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load wallet ledger")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Wallet ledger not found")
		return
	}

	detail := domain.AdminWalletLedgerDetail{
		Record: *record,
	}

	if record.Ledger.RechargeOrderID != nil {
		order, loadErr := h.app.Store.GetAdminOrderByID(r.Context(), *record.Ledger.RechargeOrderID)
		if loadErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load related order")
			return
		}
		detail.Order = order
	}

	if record.Ledger.PaymentTransactionID != nil {
		transaction, loadErr := h.app.Store.GetPaymentTransactionByID(r.Context(), *record.Ledger.PaymentTransactionID)
		if loadErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load related payment transaction")
			return
		}
		detail.PaymentTransaction = transaction
	}

	adjustment, err := h.app.Store.GetWalletAdjustmentRequestByLedgerID(r.Context(), ledgerID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load related wallet adjustment")
		return
	}
	if adjustment == nil && record.Ledger.ReferenceType != nil && *record.Ledger.ReferenceType == "wallet_adjustment" && record.Ledger.ReferenceID != nil {
		adjustment, err = h.app.Store.GetWalletAdjustmentRequestByID(r.Context(), *record.Ledger.ReferenceID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load related wallet adjustment")
			return
		}
	}
	detail.Adjustment = adjustment

	render.JSON(w, http.StatusOK, detail)
}

func (h *AdminFinanceHandler) CreateWalletAdjustment(w http.ResponseWriter, r *http.Request) {
	var payload createWalletAdjustmentRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	adjustment, err := h.app.Store.CreateWalletAdjustment(r.Context(), store.CreateWalletAdjustmentInput{
		UserID:        strings.TrimSpace(payload.UserID),
		AmountDelta:   payload.AmountDelta,
		Reason:        strings.TrimSpace(payload.Reason),
		Note:          trimmedStringPtr(valueOrEmpty(payload.Note)),
		EntryType:     trimmedStringPtr(valueOrEmpty(payload.EntryType)),
		ReferenceType: trimmedStringPtr(valueOrEmpty(payload.ReferenceType)),
		ReferenceID:   trimmedStringPtr(valueOrEmpty(payload.ReferenceID)),
		AdminID:       admin.ID,
		AdminEmail:    admin.Email,
		AdminName:     admin.Name,
		Payload:       bytes.TrimSpace(payload.Payload),
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrWalletAdjustmentUserNotFound):
			render.Error(w, http.StatusNotFound, "Wallet adjustment user not found")
		case errors.Is(err, store.ErrWalletAdjustmentAmountZero):
			render.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, store.ErrWalletAdjustmentInsufficientBalance):
			render.Error(w, http.StatusConflict, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to create wallet adjustment")
		}
		return
	}
	if adjustment == nil || adjustment.WalletLedgerID == nil {
		render.Error(w, http.StatusInternalServerError, "Wallet adjustment created without ledger")
		return
	}

	ledger, err := h.app.Store.GetAdminWalletLedgerByID(r.Context(), *adjustment.WalletLedgerID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load wallet ledger")
		return
	}
	if ledger == nil {
		render.Error(w, http.StatusInternalServerError, "Wallet ledger not found")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  adjustment.UserID,
		ResourceType: "wallet_adjustment",
		ResourceID:   &adjustment.ID,
		Action:       "manual_adjustment",
		Title:        "管理员钱包调账",
		Source:       "admin_console",
		Status:       adjustment.Status,
		Message:      auditStringPtr(strings.TrimSpace(payload.Reason)),
		Payload: mustJSONBytes(map[string]any{
			"walletAdjustmentId": adjustment.ID,
			"walletLedgerId":     *adjustment.WalletLedgerID,
			"amountDelta":        adjustment.AmountDelta,
			"entryType":          adjustment.EntryType,
		}),
	})
	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  auditStringPtr(admin.ID),
		AdminEmail:   auditStringPtr(admin.Email),
		AdminName:    auditStringPtr(admin.Name),
		ResourceType: "wallet_adjustment",
		ResourceID:   &adjustment.ID,
		Action:       "create",
		Title:        "创建钱包调账单",
		Source:       "admin_console",
		Status:       adjustment.Status,
		Message:      auditStringPtr(strings.TrimSpace(payload.Reason)),
		Payload: mustJSONBytes(map[string]any{
			"userId":         adjustment.UserID,
			"walletLedgerId": *adjustment.WalletLedgerID,
			"amountDelta":    adjustment.AmountDelta,
		}),
	})

	render.JSON(w, http.StatusCreated, domain.AdminWalletAdjustmentResult{
		Adjustment: *adjustment,
		Ledger:     *ledger,
	})
}

func (h *AdminFinanceHandler) CreatePricingPackage(w http.ResponseWriter, r *http.Request) {
	var payload createAdminBillingPackageRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	packageID := strings.TrimSpace(payload.ID)
	if packageID == "" {
		packageID = "pkg-" + uuid.NewString()
	}

	isEnabled := true
	if payload.IsEnabled != nil {
		isEnabled = *payload.IsEnabled
	}
	sortOrder := 0
	if payload.SortOrder != nil {
		sortOrder = *payload.SortOrder
	}

	item, err := h.app.Store.CreateBillingPackage(r.Context(), store.CreateBillingPackageInput{
		ID:                      packageID,
		Name:                    strings.TrimSpace(payload.Name),
		PackageType:             strings.TrimSpace(payload.PackageType),
		PaymentChannels:         payload.PaymentChannels,
		Currency:                strings.TrimSpace(payload.Currency),
		PriceCents:              payload.PriceCents,
		CreditAmount:            payload.CreditAmount,
		ManualBonusCreditAmount: payload.ManualBonusCreditAmount,
		Badge:                   trimmedStringPtr(valueOrEmpty(payload.Badge)),
		Description:             trimmedStringPtr(valueOrEmpty(payload.Description)),
		PricingPayload:          bytes.TrimSpace(payload.PricingPayload),
		ExpiresInDays:           payload.ExpiresInDays,
		IsEnabled:               isEnabled,
		SortOrder:               sortOrder,
		Entitlements:            toStoreBillingEntitlements(payload.Entitlements),
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrBillingPackageAlreadyExists):
			render.Error(w, http.StatusConflict, err.Error())
		default:
			render.Error(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  auditStringPtr(admin.ID),
		AdminEmail:   auditStringPtr(admin.Email),
		AdminName:    auditStringPtr(admin.Name),
		ResourceType: "pricing_package",
		ResourceID:   &item.ID,
		Action:       "create",
		Title:        "创建充值套餐",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr(item.Name),
		Payload: mustJSONBytes(map[string]any{
			"paymentChannels":         item.PaymentChannels,
			"priceCents":              item.PriceCents,
			"creditAmount":            item.CreditAmount,
			"manualBonusCreditAmount": item.ManualBonusCreditAmount,
			"isEnabled":               item.IsEnabled,
		}),
	})

	render.JSON(w, http.StatusCreated, item)
}

func (h *AdminFinanceHandler) UpdatePricingPackage(w http.ResponseWriter, r *http.Request) {
	packageID := strings.TrimSpace(chi.URLParam(r, "packageId"))
	if packageID == "" {
		render.Error(w, http.StatusBadRequest, "packageId is required")
		return
	}

	var payload updateAdminBillingPackageRequest
	raw, err := decodeAdminFinanceRequest(r, &payload)
	if err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	input := store.UpdateBillingPackageInput{
		Name:                    payload.Name,
		PackageType:             payload.PackageType,
		Currency:                payload.Currency,
		PriceCents:              payload.PriceCents,
		CreditAmount:            payload.CreditAmount,
		ManualBonusCreditAmount: payload.ManualBonusCreditAmount,
		IsEnabled:               payload.IsEnabled,
		SortOrder:               payload.SortOrder,
	}
	if financeFieldTouched(raw, "paymentChannels") {
		input.PaymentChannelsTouched = true
		input.PaymentChannels = payload.PaymentChannels
	}
	if financeFieldTouched(raw, "badge") {
		input.BadgeTouched = true
		input.Badge = trimmedStringPtr(valueOrEmpty(payload.Badge))
	}
	if financeFieldTouched(raw, "description") {
		input.DescriptionTouched = true
		input.Description = trimmedStringPtr(valueOrEmpty(payload.Description))
	}
	if financeFieldTouched(raw, "pricingPayload") {
		input.PricingPayloadTouched = true
		input.PricingPayload = bytes.TrimSpace(payload.PricingPayload)
	}
	if financeFieldTouched(raw, "expiresInDays") {
		input.ExpiresInDaysTouched = true
		input.ExpiresInDays = payload.ExpiresInDays
	}
	if financeFieldTouched(raw, "entitlements") {
		entitlements := toStoreBillingEntitlements(payload.Entitlements)
		input.Entitlements = &entitlements
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	item, err := h.app.Store.UpdateBillingPackage(r.Context(), packageID, input)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrBillingPackageNotFound):
			render.Error(w, http.StatusNotFound, err.Error())
		default:
			render.Error(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  auditStringPtr(admin.ID),
		AdminEmail:   auditStringPtr(admin.Email),
		AdminName:    auditStringPtr(admin.Name),
		ResourceType: "pricing_package",
		ResourceID:   &item.ID,
		Action:       "update",
		Title:        "更新充值套餐",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr(item.Name),
		Payload: mustJSONBytes(map[string]any{
			"paymentChannels":         item.PaymentChannels,
			"priceCents":              item.PriceCents,
			"creditAmount":            item.CreditAmount,
			"manualBonusCreditAmount": item.ManualBonusCreditAmount,
			"isEnabled":               item.IsEnabled,
		}),
	})

	render.JSON(w, http.StatusOK, item)
}
