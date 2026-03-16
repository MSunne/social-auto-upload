package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type BillingHandler struct {
	app *appstate.App
}

type createRechargeOrderRequest struct {
	PackageID string `json:"packageId"`
	Channel   string `json:"channel"`
	Subject   string `json:"subject"`
}

type submitManualRechargeRequest struct {
	ContactChannel      string   `json:"contactChannel"`
	ContactHandle       string   `json:"contactHandle"`
	PaymentReference    string   `json:"paymentReference"`
	TransferAmountCents *int64   `json:"transferAmountCents"`
	ProofURLs           []string `json:"proofUrls"`
	CustomerNote        string   `json:"customerNote"`
}

func NewBillingHandler(app *appstate.App) *BillingHandler {
	return &BillingHandler{app: app}
}

func normalizeBillingChannel(value string) string {
	channel := strings.TrimSpace(strings.ToLower(value))
	switch channel {
	case "manual", "manual_cs", "customer_service", "customer-service":
		return "manual_cs"
	case "wechat", "wechatpay", "wechat_pay":
		return "wechatpay"
	default:
		return channel
	}
}

func packageSupportsChannel(pkgChannels []string, channel string) bool {
	for _, item := range pkgChannels {
		if normalizeBillingChannel(item) == channel {
			return true
		}
	}
	return false
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func buildRechargeBlueprint(channel string, orderNo string, pkgID string, amountCents int64) (string, *string, []byte, []byte, *time.Time, string) {
	now := time.Now().UTC()
	var (
		status                 string
		providerStatus         *string
		paymentPayload         []byte
		customerServicePayload []byte
		expiresAt              *time.Time
		transactionKind        string
	)

	switch channel {
	case "manual_cs":
		status = "awaiting_manual_review"
		transactionKind = "manual_service"
		expires := now.Add(72 * time.Hour)
		expiresAt = &expires
		value := "manual_pending"
		providerStatus = &value
		customerServicePayload, _ = json.Marshal(map[string]any{
			"provider":      "manual_cs",
			"nextAction":    "contact_support",
			"orderNo":       orderNo,
			"packageId":     pkgID,
			"amountCents":   amountCents,
			"note":          "客服充值订单已创建，后续由人工确认付款和入账。",
			"requiresAudit": true,
		})
		paymentPayload = customerServicePayload
	case "alipay":
		status = "pending_payment"
		transactionKind = "precreate"
		expires := now.Add(30 * time.Minute)
		expiresAt = &expires
		value := "sdk_pending"
		providerStatus = &value
		paymentPayload, _ = json.Marshal(map[string]any{
			"provider":               "alipay",
			"orderNo":                orderNo,
			"recommendedApi":         "alipay.trade.precreate",
			"alternativeApi":         "alipay.trade.page.pay",
			"sdkModule":              "github.com/go-pay/gopay",
			"sdkNamespace":           "alipay/v3",
			"sdkStatus":              "gopay_ready",
			"integrationMode":        "gopay_sdk",
			"merchantConfigRequired": true,
		})
	case "wechatpay":
		status = "pending_payment"
		transactionKind = "native"
		expires := now.Add(30 * time.Minute)
		expiresAt = &expires
		value := "sdk_pending"
		providerStatus = &value
		paymentPayload, _ = json.Marshal(map[string]any{
			"provider":               "wechatpay",
			"orderNo":                orderNo,
			"recommendedApi":         "v3/pay/transactions/native",
			"sdkModule":              "github.com/go-pay/gopay",
			"sdkNamespace":           "wechat/v3",
			"integrationMode":        "gopay_sdk",
			"merchantConfigRequired": true,
		})
	default:
		status = "pending_payment"
		transactionKind = "online_payment"
	}

	return status, providerStatus, paymentPayload, customerServicePayload, expiresAt, transactionKind
}

func buildManualSupportPayload(settings effectiveAdminSystemSettings, orderNo string, pkgID string, amountCents int64, creditAmount int64, manualBonusCreditAmount int64) []byte {
	payload, _ := json.Marshal(map[string]any{
		"provider":    "manual_cs",
		"nextAction":  "contact_support",
		"orderNo":     orderNo,
		"packageId":   pkgID,
		"amountCents": amountCents,
		"credits": map[string]any{
			"baseCreditAmount":        creditAmount,
			"manualBonusCreditAmount": manualBonusCreditAmount,
			"totalCreditAmount":       creditAmount + manualBonusCreditAmount,
		},
		"support": map[string]any{
			"name":      settings.BillingManualSupport.Name,
			"contact":   settings.BillingManualSupport.Contact,
			"qrCodeUrl": settings.BillingManualSupport.QRCodeURL,
			"note":      settings.BillingManualSupport.Note,
		},
		"submission": map[string]any{
			"status":      "pending",
			"proofUrls":   []string{},
			"submittedAt": nil,
		},
	})
	return payload
}

func (h *BillingHandler) Summary(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	summary, err := h.app.Store.GetBillingSummaryByUser(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load billing summary")
		return
	}
	render.JSON(w, http.StatusOK, summary)
}

func (h *BillingHandler) ListPackages(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.Store.ListBillingPackages(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load billing packages")
		return
	}

	settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load billing packages")
		return
	}

	filtered := make([]domain.BillingPackage, 0, len(items))
	for _, item := range items {
		item.PaymentChannels = filterEnabledPaymentChannels(item.PaymentChannels, settings.PaymentChannels)
		if len(item.PaymentChannels) == 0 {
			continue
		}
		filtered = append(filtered, item)
	}
	render.JSON(w, http.StatusOK, filtered)
}

func (h *BillingHandler) ListPricingRules(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.Store.ListBillingPricingRules(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load billing pricing rules")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *BillingHandler) Ledger(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	items, err := h.app.Store.ListWalletLedgerByUser(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load wallet ledger")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *BillingHandler) ListUsageEvents(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	items, err := h.app.Store.ListBillingUsageEventsByUser(r.Context(), user.ID, store.BillingUsageEventListFilter{
		SourceType: strings.TrimSpace(r.URL.Query().Get("sourceType")),
		SourceID:   strings.TrimSpace(r.URL.Query().Get("sourceId")),
		MeterCode:  strings.TrimSpace(r.URL.Query().Get("meterCode")),
		BillStatus: strings.TrimSpace(r.URL.Query().Get("billStatus")),
		JobType:    strings.TrimSpace(r.URL.Query().Get("jobType")),
		ModelName:  strings.TrimSpace(r.URL.Query().Get("modelName")),
		Limit:      limit,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load billing usage events")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *BillingHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	items, err := h.app.Store.ListRechargeOrdersByUser(r.Context(), user.ID, limit)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load recharge orders")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *BillingHandler) DetailOrder(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	orderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
	if orderID == "" {
		render.Error(w, http.StatusBadRequest, "orderId is required")
		return
	}

	order, err := h.app.Store.GetRechargeOrderByID(r.Context(), user.ID, orderID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load recharge order")
		return
	}
	if order == nil {
		render.Error(w, http.StatusNotFound, "Recharge order not found")
		return
	}
	render.JSON(w, http.StatusOK, order)
}

func (h *BillingHandler) ListOrderEvents(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	orderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
	if orderID == "" {
		render.Error(w, http.StatusBadRequest, "orderId is required")
		return
	}

	items, err := h.app.Store.ListRechargeOrderEvents(r.Context(), user.ID, orderID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load recharge order events")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *BillingHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	var payload createRechargeOrderRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.PackageID = strings.TrimSpace(payload.PackageID)
	channel := normalizeBillingChannel(payload.Channel)
	if payload.PackageID == "" || channel == "" {
		render.Error(w, http.StatusBadRequest, "packageId and channel are required")
		return
	}

	settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load billing configuration")
		return
	}
	if !settings.paymentChannelEnabled(channel) {
		render.Error(w, http.StatusConflict, "Selected channel is temporarily unavailable")
		return
	}

	pkg, err := h.app.Store.GetBillingPackageByID(r.Context(), payload.PackageID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load billing package")
		return
	}
	if pkg == nil || !pkg.IsEnabled {
		render.Error(w, http.StatusNotFound, "Billing package not found")
		return
	}
	if !packageSupportsChannel(pkg.PaymentChannels, channel) {
		render.Error(w, http.StatusBadRequest, "Selected channel is not available for this package")
		return
	}

	orderNo := fmt.Sprintf("RC%s%s", time.Now().UTC().Format("20060102150405"), strings.ToUpper(uuid.NewString()[:8]))
	status, providerStatus, paymentPayload, customerServicePayload, expiresAt, transactionKind := buildRechargeBlueprint(
		channel,
		orderNo,
		pkg.ID,
		pkg.PriceCents,
	)
	if channel == "manual_cs" {
		customerServicePayload = buildManualSupportPayload(settings, orderNo, pkg.ID, pkg.PriceCents, pkg.CreditAmount, pkg.ManualBonusCreditAmount)
		paymentPayload = customerServicePayload
	}

	subject := strings.TrimSpace(payload.Subject)
	if subject == "" {
		subject = fmt.Sprintf("%s 充值", pkg.Name)
	}
	body := pkg.Description
	packageSnapshot, err := json.Marshal(pkg)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to build package snapshot")
		return
	}

	transactionRequest, _ := json.Marshal(map[string]any{
		"channel":   channel,
		"packageId": pkg.ID,
		"orderNo":   orderNo,
		"amount":    pkg.PriceCents,
		"currency":  pkg.Currency,
	})

	order, err := h.app.Store.CreateRechargeOrder(r.Context(), store.CreateRechargeOrderInput{
		ID:              uuid.NewString(),
		OrderNo:         orderNo,
		UserID:          user.ID,
		PackageID:       &pkg.ID,
		PackageSnapshot: packageSnapshot,
		Channel:         channel,
		Status:          status,
		Subject:         subject,
		Body:            body,
		Currency:        pkg.Currency,
		AmountCents:     pkg.PriceCents,
		CreditAmount:    pkg.CreditAmount,
		ManualBonusCreditAmount: func() int64 {
			if channel == "manual_cs" {
				return pkg.ManualBonusCreditAmount
			}
			return 0
		}(),
		PaymentPayload:         paymentPayload,
		CustomerServicePayload: customerServicePayload,
		ProviderStatus:         providerStatus,
		ExpiresAt:              expiresAt,
		TransactionID:          uuid.NewString(),
		TransactionKind:        transactionKind,
		TransactionStatus:      "pending",
		TransactionOutTradeNo:  orderNo,
		TransactionRequest:     transactionRequest,
		TransactionResponse:    paymentPayload,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create recharge order")
		return
	}

	render.JSON(w, http.StatusCreated, order)
}

func (h *BillingHandler) SubmitManualRecharge(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	orderID := strings.TrimSpace(chi.URLParam(r, "orderId"))
	if orderID == "" {
		render.Error(w, http.StatusBadRequest, "orderId is required")
		return
	}

	var payload submitManualRechargeRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	order, err := h.app.Store.GetRechargeOrderByID(r.Context(), user.ID, orderID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load recharge order")
		return
	}
	if order == nil {
		render.Error(w, http.StatusNotFound, "Recharge order not found")
		return
	}
	if order.Channel != "manual_cs" {
		render.Error(w, http.StatusConflict, "Recharge order is not a manual customer-service order")
		return
	}
	if order.PaidAt != nil || order.Status == "credited" || order.Status == "paid" || order.Status == "completed" || order.Status == "success" {
		render.Error(w, http.StatusConflict, "Recharge order has already been credited")
		return
	}

	servicePayload := map[string]any{}
	if len(order.CustomerServicePayload) > 0 {
		_ = json.Unmarshal(order.CustomerServicePayload, &servicePayload)
	}
	if _, ok := servicePayload["support"]; !ok {
		settings, settingsErr := loadEffectiveAdminSystemSettings(r.Context(), h.app)
		if settingsErr != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load billing configuration")
			return
		}
		_ = json.Unmarshal(buildManualSupportPayload(settings, order.OrderNo, valueOrEmpty(order.PackageID), order.AmountCents, order.CreditAmount, order.ManualBonusCreditAmount), &servicePayload)
	}

	proofURLs := payload.ProofURLs
	if proofURLs == nil {
		proofURLs = []string{}
	}
	submittedAt := time.Now().UTC().Format(time.RFC3339)
	servicePayload["submission"] = map[string]any{
		"status":              "submitted",
		"contactChannel":      strings.TrimSpace(payload.ContactChannel),
		"contactHandle":       strings.TrimSpace(payload.ContactHandle),
		"paymentReference":    strings.TrimSpace(payload.PaymentReference),
		"transferAmountCents": payload.TransferAmountCents,
		"proofUrls":           proofURLs,
		"customerNote":        strings.TrimSpace(payload.CustomerNote),
		"submittedAt":         submittedAt,
	}
	servicePayload["nextAction"] = "waiting_manual_confirmation"

	servicePayloadBytes, err := json.Marshal(servicePayload)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to encode manual recharge payload")
		return
	}

	eventPayload, _ := json.Marshal(map[string]any{
		"contactChannel":      strings.TrimSpace(payload.ContactChannel),
		"contactHandle":       strings.TrimSpace(payload.ContactHandle),
		"paymentReference":    strings.TrimSpace(payload.PaymentReference),
		"transferAmountCents": payload.TransferAmountCents,
		"proofUrls":           proofURLs,
		"customerNote":        strings.TrimSpace(payload.CustomerNote),
		"submittedAt":         submittedAt,
	})

	providerStatus := "manual_submitted"
	var providerTransactionID *string
	if paymentReference := strings.TrimSpace(payload.PaymentReference); paymentReference != "" {
		providerTransactionID = &paymentReference
	}
	message := "客服充值资料已提交，等待人工确认入账"
	updatedOrder, err := h.app.Store.SubmitManualRecharge(r.Context(), user.ID, orderID, store.SubmitManualRechargeInput{
		Status:                 "processing",
		ProviderTransactionID:  providerTransactionID,
		ProviderStatus:         &providerStatus,
		CustomerServicePayload: servicePayloadBytes,
		EventID:                uuid.NewString(),
		EventType:              "manual_submission",
		EventStatus:            "processing",
		EventMessage:           &message,
		EventPayload:           eventPayload,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to submit manual recharge information")
		return
	}
	render.JSON(w, http.StatusOK, updatedOrder)
}
