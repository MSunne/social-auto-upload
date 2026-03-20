package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

var (
	ErrRechargeOrderNotFound         = errors.New("recharge order not found")
	ErrRechargeOrderNotManual        = errors.New("recharge order is not a manual customer-service order")
	ErrRechargeOrderNotPendingReview = errors.New("support recharge is not pending review")
	ErrRechargeOrderAlreadyCredited  = errors.New("recharge order already credited")
	ErrRechargeOrderAlreadyClosed    = errors.New("recharge order already closed")
)

type CreditSupportRechargeInput struct {
	AdminID          string
	AdminEmail       string
	AdminName        string
	Note             *string
	PaymentReference *string
}

type RejectSupportRechargeInput struct {
	AdminID    string
	AdminEmail string
	AdminName  string
	Note       *string
}

type InvalidateSupportRechargeInput struct {
	AdminID    string
	AdminEmail string
	AdminName  string
	Note       *string
}

func (s *Store) getRechargeOrderByIDAnyUser(ctx context.Context, orderID string) (*domain.RechargeOrder, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, order_no, user_id, package_id, package_snapshot, channel, status, subject, body, currency,
		       amount_cents, credit_amount, manual_bonus_credit_amount, payment_payload, customer_service_payload, provider_transaction_id,
		       provider_status, expires_at, paid_at, closed_at, created_at, updated_at
		FROM recharge_orders
		WHERE id = $1
	`, orderID)

	item, err := scanRechargeOrder(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) getRechargeOrderByIDAnyUserTx(ctx context.Context, tx pgx.Tx, orderID string) (*domain.RechargeOrder, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, order_no, user_id, package_id, package_snapshot, channel, status, subject, body, currency,
		       amount_cents, credit_amount, manual_bonus_credit_amount, payment_payload, customer_service_payload, provider_transaction_id,
		       provider_status, expires_at, paid_at, closed_at, created_at, updated_at
		FROM recharge_orders
		WHERE id = $1
		FOR UPDATE
	`, orderID)

	item, err := scanRechargeOrder(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func decodeSupportRechargePayload(raw []byte) map[string]any {
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

func valueOrEmptyString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func lookupNestedString(payload map[string]any, parents ...string) string {
	var current any = payload
	for _, key := range parents {
		nested, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = nested[key]
	}
	value, _ := current.(string)
	return strings.TrimSpace(value)
}

func isRechargeOrderCredited(order *domain.RechargeOrder) bool {
	if order == nil {
		return false
	}
	if order.PaidAt != nil {
		return true
	}
	switch strings.TrimSpace(order.Status) {
	case "credited", "paid", "success", "completed":
		return true
	default:
		return false
	}
}

func isRechargeOrderPendingReview(order *domain.RechargeOrder) bool {
	if order == nil {
		return false
	}
	if strings.TrimSpace(order.Status) == "processing" {
		return true
	}
	if strings.TrimSpace(order.Status) == "rejected" {
		return false
	}
	payload := decodeSupportRechargePayload(order.CustomerServicePayload)
	return strings.EqualFold(lookupNestedString(payload, "submission", "status"), "submitted")
}

func isRechargeOrderClosed(order *domain.RechargeOrder) bool {
	if order == nil {
		return false
	}
	if order.ClosedAt != nil {
		return true
	}
	switch strings.TrimSpace(order.Status) {
	case "closed", "cancelled", "invalidated":
		return true
	default:
		return false
	}
}

func buildSupportRechargeGrantPlan(order *domain.RechargeOrder, now time.Time) ([]domain.BillingPackageEntitlement, *time.Time, string) {
	if order == nil {
		return nil, nil, ""
	}

	var pkg domain.BillingPackage
	var entitlements []domain.BillingPackageEntitlement
	var expiresAt *time.Time
	var packageName string

	if len(order.PackageSnapshot) > 0 && json.Unmarshal(order.PackageSnapshot, &pkg) == nil {
		packageName = strings.TrimSpace(pkg.Name)
		if len(pkg.Entitlements) > 0 {
			entitlements = append(entitlements, pkg.Entitlements...)
		}
		if pkg.ExpiresInDays != nil && *pkg.ExpiresInDays > 0 {
			expires := now.Add(time.Duration(*pkg.ExpiresInDays) * 24 * time.Hour)
			expiresAt = &expires
		}
		if len(entitlements) == 0 && pkg.CreditAmount > 0 {
			entitlements = append(entitlements, domain.BillingPackageEntitlement{
				MeterCode:   "wallet_credit",
				GrantAmount: pkg.CreditAmount,
				GrantMode:   "one_time",
			})
		}
	}

	if len(entitlements) == 0 && order.CreditAmount > 0 {
		entitlements = append(entitlements, domain.BillingPackageEntitlement{
			MeterCode:   "wallet_credit",
			GrantAmount: order.CreditAmount,
			GrantMode:   "one_time",
		})
	}

	if order.Channel == "manual_cs" && order.ManualBonusCreditAmount > 0 {
		entitlements = append(entitlements, domain.BillingPackageEntitlement{
			MeterCode:   "wallet_credit",
			GrantAmount: order.ManualBonusCreditAmount,
			GrantMode:   "manual_bonus",
			Description: stringPtr("客服充值自动赠送积分"),
		})
	}

	return entitlements, expiresAt, packageName
}

func loadPaymentTransactionIDTx(ctx context.Context, tx pgx.Tx, orderID string) (*string, error) {
	var paymentTransactionID string
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM payment_transactions
		WHERE recharge_order_id = $1
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE
	`, orderID).Scan(&paymentTransactionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &paymentTransactionID, nil
}

func (s *Store) CreditSupportRecharge(ctx context.Context, orderID string, input CreditSupportRechargeInput) (*domain.RechargeOrder, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	order, err := s.getRechargeOrderByIDAnyUserTx(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrRechargeOrderNotFound
	}
	if order.Channel != "manual_cs" {
		return nil, ErrRechargeOrderNotManual
	}
	if isRechargeOrderCredited(order) {
		return nil, ErrRechargeOrderAlreadyCredited
	}
	if !isRechargeOrderPendingReview(order) {
		return nil, ErrRechargeOrderNotPendingReview
	}

	now := time.Now().UTC()
	paymentTransactionID, err := loadPaymentTransactionIDTx(ctx, tx, order.ID)
	if err != nil {
		return nil, err
	}

	servicePayload := decodeSupportRechargePayload(order.CustomerServicePayload)
	trimmedNote := valueOrEmptyString(input.Note)
	paymentReference := valueOrEmptyString(input.PaymentReference)
	if paymentReference == "" {
		paymentReference = lookupNestedString(servicePayload, "submission", "paymentReference")
	}
	var providerTransactionID *string
	if paymentReference != "" {
		providerTransactionID = stringPtr(paymentReference)
	}

	servicePayload["review"] = map[string]any{
		"status":        "credited",
		"operatorId":    strings.TrimSpace(input.AdminID),
		"operatorName":  strings.TrimSpace(input.AdminName),
		"operatorEmail": strings.TrimSpace(input.AdminEmail),
		"note":          trimmedNote,
		"creditedAt":    now.Format(time.RFC3339),
	}
	servicePayload["nextAction"] = "completed"

	grants, expiresAt, packageName := buildSupportRechargeGrantPlan(order, now)
	referenceType := stringPtr("support_recharge")
	referenceID := &order.ID
	entryType := "recharge"
	grantSummaries := make([]map[string]any, 0, len(grants))

	for _, entitlement := range grants {
		if entitlement.GrantAmount <= 0 {
			continue
		}

		grantSummary := map[string]any{
			"meterCode":   entitlement.MeterCode,
			"grantAmount": entitlement.GrantAmount,
			"grantMode":   entitlement.GrantMode,
		}
		if entitlement.Description != nil && strings.TrimSpace(*entitlement.Description) != "" {
			grantSummary["description"] = strings.TrimSpace(*entitlement.Description)
		}
		if packageName != "" {
			grantSummary["packageName"] = packageName
		}
		if expiresAt != nil {
			grantSummary["expiresAt"] = expiresAt.Format(time.RFC3339)
		}

		grantMetadata, _ := json.Marshal(map[string]any{
			"orderId":          order.ID,
			"orderNo":          order.OrderNo,
			"packageName":      packageName,
			"meterCode":        entitlement.MeterCode,
			"grantAmount":      entitlement.GrantAmount,
			"paymentReference": paymentReference,
			"operator": map[string]any{
				"id":    strings.TrimSpace(input.AdminID),
				"name":  strings.TrimSpace(input.AdminName),
				"email": strings.TrimSpace(input.AdminEmail),
			},
		})

		description := entitlement.Description
		if description == nil || strings.TrimSpace(*description) == "" {
			switch entitlement.MeterCode {
			case "wallet_credit":
				description = stringPtr(fmt.Sprintf("客服充值入账 %s", order.OrderNo))
			default:
				description = stringPtr(fmt.Sprintf("客服充值额度发放 %s", order.OrderNo))
			}
		}

		if entitlement.MeterCode == "wallet_credit" {
			if err := s.grantWalletCreditsTx(ctx, tx, GrantWalletCreditsInput{
				UserID:               order.UserID,
				Amount:               entitlement.GrantAmount,
				EntryType:            &entryType,
				Description:          description,
				ReferenceType:        referenceType,
				ReferenceID:          referenceID,
				RechargeOrderID:      &order.ID,
				PaymentTransactionID: paymentTransactionID,
				Metadata:             grantMetadata,
			}); err != nil {
				return nil, err
			}
		} else {
			if err := s.grantQuotaTx(ctx, tx, GrantQuotaInput{
				UserID:        order.UserID,
				MeterCode:     entitlement.MeterCode,
				Amount:        entitlement.GrantAmount,
				ExpiresAt:     expiresAt,
				SourceType:    referenceType,
				SourceID:      referenceID,
				Description:   description,
				ReferenceType: referenceType,
				ReferenceID:   referenceID,
				Payload:       grantMetadata,
			}); err != nil {
				return nil, err
			}
		}

		grantSummaries = append(grantSummaries, grantSummary)
	}

	servicePayloadBytes, err := json.Marshal(servicePayload)
	if err != nil {
		return nil, err
	}

	providerStatus := "manual_credited"
	if _, err := tx.Exec(ctx, `
		UPDATE recharge_orders
		SET status = 'credited',
		    provider_status = $2,
		    provider_transaction_id = COALESCE($3, provider_transaction_id),
		    customer_service_payload = $4,
		    paid_at = COALESCE(paid_at, $5),
		    updated_at = NOW()
		WHERE id = $1
	`, order.ID, providerStatus, providerTransactionID, servicePayloadBytes, now); err != nil {
		return nil, err
	}

	if paymentTransactionID != nil {
		notifyPayload, _ := json.Marshal(map[string]any{
			"operator": map[string]any{
				"id":    strings.TrimSpace(input.AdminID),
				"name":  strings.TrimSpace(input.AdminName),
				"email": strings.TrimSpace(input.AdminEmail),
			},
			"note":             trimmedNote,
			"paymentReference": paymentReference,
			"grants":           grantSummaries,
			"creditedAt":       now.Format(time.RFC3339),
		})

		if _, err := tx.Exec(ctx, `
			UPDATE payment_transactions
			SET status = 'paid',
			    provider_transaction_id = COALESCE($2, provider_transaction_id),
			    response_payload = $3,
			    notify_payload = $4,
			    error_message = NULL,
			    paid_at = COALESCE(paid_at, $5),
			    updated_at = NOW()
			WHERE id = $1
		`, *paymentTransactionID, providerTransactionID, servicePayloadBytes, notifyPayload, now); err != nil {
			return nil, err
		}
	}

	eventPayload, _ := json.Marshal(map[string]any{
		"operator": map[string]any{
			"id":    strings.TrimSpace(input.AdminID),
			"name":  strings.TrimSpace(input.AdminName),
			"email": strings.TrimSpace(input.AdminEmail),
		},
		"note":             trimmedNote,
		"paymentReference": paymentReference,
		"grants":           grantSummaries,
		"creditedAt":       now.Format(time.RFC3339),
	})
	message := "客服充值已确认入账"
	if err := s.appendRechargeOrderEventTx(
		ctx,
		tx,
		fmt.Sprintf("%s-manual-credited-%d", order.ID, now.UnixNano()),
		order.ID,
		order.UserID,
		"manual_credited",
		"credited",
		&message,
		eventPayload,
	); err != nil {
		return nil, err
	}

	if err := s.ensureDistributionCommissionForRechargeOrderTx(ctx, tx, order); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.getRechargeOrderByIDAnyUser(ctx, order.ID)
}

func (s *Store) RejectSupportRecharge(ctx context.Context, orderID string, input RejectSupportRechargeInput) (*domain.RechargeOrder, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	order, err := s.getRechargeOrderByIDAnyUserTx(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrRechargeOrderNotFound
	}
	if order.Channel != "manual_cs" {
		return nil, ErrRechargeOrderNotManual
	}
	if isRechargeOrderCredited(order) {
		return nil, ErrRechargeOrderAlreadyCredited
	}
	if isRechargeOrderClosed(order) {
		return nil, ErrRechargeOrderAlreadyClosed
	}
	if !isRechargeOrderPendingReview(order) {
		return nil, ErrRechargeOrderNotPendingReview
	}

	now := time.Now().UTC()
	servicePayload := decodeSupportRechargePayload(order.CustomerServicePayload)
	trimmedNote := valueOrEmptyString(input.Note)

	servicePayload["review"] = map[string]any{
		"status":        "rejected",
		"operatorId":    strings.TrimSpace(input.AdminID),
		"operatorName":  strings.TrimSpace(input.AdminName),
		"operatorEmail": strings.TrimSpace(input.AdminEmail),
		"note":          trimmedNote,
		"reviewedAt":    now.Format(time.RFC3339),
	}
	servicePayload["nextAction"] = "resubmit_manual_proof"

	servicePayloadBytes, err := json.Marshal(servicePayload)
	if err != nil {
		return nil, err
	}

	providerStatus := "manual_rejected"
	if _, err := tx.Exec(ctx, `
		UPDATE recharge_orders
		SET status = 'rejected',
		    provider_status = $2,
		    customer_service_payload = $3,
		    updated_at = NOW()
		WHERE id = $1
	`, order.ID, providerStatus, servicePayloadBytes); err != nil {
		return nil, err
	}

	paymentTransactionID, err := loadPaymentTransactionIDTx(ctx, tx, order.ID)
	if err != nil {
		return nil, err
	}
	if paymentTransactionID != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE payment_transactions
			SET status = 'rejected',
			    response_payload = $2,
			    error_message = NULLIF($3, ''),
			    updated_at = NOW()
			WHERE id = $1
		`, *paymentTransactionID, servicePayloadBytes, trimmedNote); err != nil {
			return nil, err
		}
	}

	eventPayload, _ := json.Marshal(map[string]any{
		"operator": map[string]any{
			"id":    strings.TrimSpace(input.AdminID),
			"name":  strings.TrimSpace(input.AdminName),
			"email": strings.TrimSpace(input.AdminEmail),
		},
		"note":       trimmedNote,
		"reviewedAt": now.Format(time.RFC3339),
	})
	message := "客服充值已驳回，等待重新提交资料"
	if err := s.appendRechargeOrderEventTx(
		ctx,
		tx,
		fmt.Sprintf("%s-manual-rejected-%d", order.ID, now.UnixNano()),
		order.ID,
		order.UserID,
		"manual_rejected",
		"rejected",
		&message,
		eventPayload,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.getRechargeOrderByIDAnyUser(ctx, order.ID)
}

func (s *Store) InvalidateSupportRecharge(ctx context.Context, orderID string, input InvalidateSupportRechargeInput) (*domain.RechargeOrder, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	order, err := s.getRechargeOrderByIDAnyUserTx(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrRechargeOrderNotFound
	}
	if order.Channel != "manual_cs" {
		return nil, ErrRechargeOrderNotManual
	}
	if isRechargeOrderCredited(order) {
		return nil, ErrRechargeOrderAlreadyCredited
	}
	if isRechargeOrderClosed(order) {
		return nil, ErrRechargeOrderAlreadyClosed
	}

	now := time.Now().UTC()
	servicePayload := decodeSupportRechargePayload(order.CustomerServicePayload)
	trimmedNote := valueOrEmptyString(input.Note)

	servicePayload["review"] = map[string]any{
		"status":        "invalidated",
		"operatorId":    strings.TrimSpace(input.AdminID),
		"operatorName":  strings.TrimSpace(input.AdminName),
		"operatorEmail": strings.TrimSpace(input.AdminEmail),
		"note":          trimmedNote,
		"reviewedAt":    now.Format(time.RFC3339),
	}
	servicePayload["nextAction"] = "closed"

	servicePayloadBytes, err := json.Marshal(servicePayload)
	if err != nil {
		return nil, err
	}

	providerStatus := "manual_invalidated"
	if _, err := tx.Exec(ctx, `
		UPDATE recharge_orders
		SET status = 'closed',
		    provider_status = $2,
		    customer_service_payload = $3,
		    closed_at = COALESCE(closed_at, $4),
		    updated_at = NOW()
		WHERE id = $1
	`, order.ID, providerStatus, servicePayloadBytes, now); err != nil {
		return nil, err
	}

	paymentTransactionID, err := loadPaymentTransactionIDTx(ctx, tx, order.ID)
	if err != nil {
		return nil, err
	}
	if paymentTransactionID != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE payment_transactions
			SET status = 'closed',
			    response_payload = $2,
			    error_message = NULLIF($3, ''),
			    updated_at = NOW()
			WHERE id = $1
		`, *paymentTransactionID, servicePayloadBytes, trimmedNote); err != nil {
			return nil, err
		}
	}

	eventPayload, _ := json.Marshal(map[string]any{
		"operator": map[string]any{
			"id":    strings.TrimSpace(input.AdminID),
			"name":  strings.TrimSpace(input.AdminName),
			"email": strings.TrimSpace(input.AdminEmail),
		},
		"note":       trimmedNote,
		"reviewedAt": now.Format(time.RFC3339),
	})
	message := "客服充值已失效关闭"
	if err := s.appendRechargeOrderEventTx(
		ctx,
		tx,
		fmt.Sprintf("%s-manual-invalidated-%d", order.ID, now.UnixNano()),
		order.ID,
		order.UserID,
		"manual_invalidated",
		"closed",
		&message,
		eventPayload,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.getRechargeOrderByIDAnyUser(ctx, order.ID)
}
