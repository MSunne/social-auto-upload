package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
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
		transactionKind = "page_pay"
		expires := now.Add(30 * time.Minute)
		expiresAt = &expires
		value := "sdk_pending"
		providerStatus = &value
		paymentPayload, _ = json.Marshal(map[string]any{
			"provider":               "alipay",
			"orderNo":                orderNo,
			"recommendedApi":         "alipay.trade.page.pay",
			"alternativeApi":         "alipay.trade.precreate",
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
	render.JSON(w, http.StatusOK, items)
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
		ID:                     uuid.NewString(),
		OrderNo:                orderNo,
		UserID:                 user.ID,
		PackageID:              &pkg.ID,
		PackageSnapshot:        packageSnapshot,
		Channel:                channel,
		Status:                 status,
		Subject:                subject,
		Body:                   body,
		Currency:               pkg.Currency,
		AmountCents:            pkg.PriceCents,
		CreditAmount:           pkg.CreditAmount,
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
