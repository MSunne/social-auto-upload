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

type scanFn func(dest ...any) error

func decodePaymentChannels(raw []byte, fallback string) []string {
	if len(raw) > 0 {
		var values []string
		if err := json.Unmarshal(raw, &values); err == nil && len(values) > 0 {
			return values
		}
	}

	if strings.TrimSpace(fallback) == "" {
		return []string{}
	}

	parts := strings.Split(fallback, ",")
	items := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if value == "manual" {
			value = "manual_cs"
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func scanBillingPackage(scan scanFn) (*domain.BillingPackage, error) {
	var item domain.BillingPackage
	var paymentChannels []byte
	var badge *string
	var description *string
	var pricingPayload []byte
	var expiresInDays *int32

	if err := scan(
		&item.ID,
		&item.Name,
		&item.PackageType,
		&item.Channel,
		&paymentChannels,
		&item.Currency,
		&item.PriceCents,
		&item.CreditAmount,
		&badge,
		&description,
		&pricingPayload,
		&expiresInDays,
		&item.IsEnabled,
		&item.SortOrder,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}

	item.Badge = badge
	item.Description = description
	item.PaymentChannels = decodePaymentChannels(paymentChannels, item.Channel)
	item.PricingPayload = bytesOrNil(pricingPayload)
	item.ExpiresInDays = expiresInDays
	item.Entitlements = []domain.BillingPackageEntitlement{}
	return &item, nil
}

func scanRechargeOrder(scan scanFn) (*domain.RechargeOrder, error) {
	var item domain.RechargeOrder
	var packageID *string
	var packageSnapshot []byte
	var body *string
	var paymentPayload []byte
	var customerServicePayload []byte
	var providerTransactionID *string
	var providerStatus *string
	var expiresAt *time.Time
	var paidAt *time.Time
	var closedAt *time.Time

	if err := scan(
		&item.ID,
		&item.OrderNo,
		&item.UserID,
		&packageID,
		&packageSnapshot,
		&item.Channel,
		&item.Status,
		&item.Subject,
		&body,
		&item.Currency,
		&item.AmountCents,
		&item.CreditAmount,
		&paymentPayload,
		&customerServicePayload,
		&providerTransactionID,
		&providerStatus,
		&expiresAt,
		&paidAt,
		&closedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}

	item.PackageID = packageID
	item.PackageSnapshot = bytesOrNil(packageSnapshot)
	item.Body = body
	item.PaymentPayload = bytesOrNil(paymentPayload)
	item.CustomerServicePayload = bytesOrNil(customerServicePayload)
	item.ProviderTransactionID = providerTransactionID
	item.ProviderStatus = providerStatus
	item.ExpiresAt = expiresAt
	item.PaidAt = paidAt
	item.ClosedAt = closedAt
	return &item, nil
}

func scanRechargeOrderEvent(scan scanFn) (*domain.RechargeOrderEvent, error) {
	var item domain.RechargeOrderEvent
	var message *string
	var payload []byte
	if err := scan(
		&item.ID,
		&item.RechargeOrderID,
		&item.UserID,
		&item.EventType,
		&item.Status,
		&message,
		&payload,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	item.Message = message
	item.Payload = bytesOrNil(payload)
	return &item, nil
}

func (s *Store) appendRechargeOrderEventTx(ctx context.Context, tx pgx.Tx, eventID string, orderID string, userID string, eventType string, status string, message *string, payload []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO recharge_order_events (
			id, recharge_order_id, user_id, event_type, status, message, payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, eventID, orderID, userID, eventType, status, message, payload)
	return err
}

func (s *Store) loadPackageEntitlements(ctx context.Context, packageIDs []string) (map[string][]domain.BillingPackageEntitlement, error) {
	if len(packageIDs) == 0 {
		return map[string][]domain.BillingPackageEntitlement{}, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.package_id, e.meter_code, m.name, m.unit, e.grant_amount, e.grant_mode,
		       e.sort_order, e.description, e.created_at, e.updated_at
		FROM billing_package_entitlements e
		LEFT JOIN billing_meters m ON m.code = e.meter_code
		WHERE e.package_id = ANY($1)
		ORDER BY e.package_id ASC, e.sort_order ASC, e.created_at ASC
	`, packageIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]domain.BillingPackageEntitlement, len(packageIDs))
	for rows.Next() {
		var item domain.BillingPackageEntitlement
		var meterName *string
		var unit *string
		var description *string
		if scanErr := rows.Scan(
			&item.ID,
			&item.PackageID,
			&item.MeterCode,
			&meterName,
			&unit,
			&item.GrantAmount,
			&item.GrantMode,
			&item.SortOrder,
			&description,
			&item.CreatedAt,
			&item.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		item.MeterName = meterName
		item.Unit = unit
		item.Description = description
		result[item.PackageID] = append(result[item.PackageID], item)
	}
	return result, rows.Err()
}

func (s *Store) ListBillingPackages(ctx context.Context) ([]domain.BillingPackage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, package_type, channel, payment_channels, currency, price_cents, credit_amount,
		       badge, description, pricing_payload, expires_in_days, is_enabled, sort_order, created_at, updated_at
		FROM billing_packages
		WHERE is_enabled = TRUE
		ORDER BY sort_order ASC, price_cents ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.BillingPackage, 0)
	packageIDs := make([]string, 0)
	for rows.Next() {
		item, scanErr := scanBillingPackage(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
		packageIDs = append(packageIDs, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	entitlementsByPackage, err := s.loadPackageEntitlements(ctx, packageIDs)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Entitlements = entitlementsByPackage[items[index].ID]
	}
	return items, nil
}

func (s *Store) GetBillingPackageByID(ctx context.Context, packageID string) (*domain.BillingPackage, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, package_type, channel, payment_channels, currency, price_cents, credit_amount,
		       badge, description, pricing_payload, expires_in_days, is_enabled, sort_order, created_at, updated_at
		FROM billing_packages
		WHERE id = $1
	`, packageID)

	item, err := scanBillingPackage(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	entitlementsByPackage, err := s.loadPackageEntitlements(ctx, []string{packageID})
	if err != nil {
		return nil, err
	}
	item.Entitlements = entitlementsByPackage[packageID]
	return item, nil
}

func (s *Store) ListBillingPricingRules(ctx context.Context) ([]domain.BillingPricingRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.name, r.meter_code, m.name, r.applies_to, r.model_name, r.job_type,
		       r.charge_mode, r.quota_meter_code, qm.name, r.unit_size, r.wallet_debit_amount,
		       r.sort_order, r.description, r.is_enabled, r.created_at, r.updated_at
		FROM billing_pricing_rules r
		LEFT JOIN billing_meters m ON m.code = r.meter_code
		LEFT JOIN billing_meters qm ON qm.code = r.quota_meter_code
		WHERE r.is_enabled = TRUE
		ORDER BY r.sort_order ASC, r.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.BillingPricingRule, 0)
	for rows.Next() {
		var item domain.BillingPricingRule
		var meterName *string
		var modelName *string
		var jobType *string
		var quotaMeterCode *string
		var quotaMeterName *string
		var description *string
		if scanErr := rows.Scan(
			&item.ID,
			&item.Name,
			&item.MeterCode,
			&meterName,
			&item.AppliesTo,
			&modelName,
			&jobType,
			&item.ChargeMode,
			&quotaMeterCode,
			&quotaMeterName,
			&item.UnitSize,
			&item.WalletDebitAmount,
			&item.SortOrder,
			&description,
			&item.IsEnabled,
			&item.CreatedAt,
			&item.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		item.MeterName = meterName
		item.ModelName = modelName
		item.JobType = jobType
		item.QuotaMeterCode = quotaMeterCode
		item.QuotaMeterName = quotaMeterName
		item.Description = description
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetBillingSummaryByUser(ctx context.Context, userID string) (*domain.BillingSummary, error) {
	summary := &domain.BillingSummary{
		QuotaBalances: []domain.BillingQuotaBalance{},
	}

	if err := s.pool.QueryRow(ctx, `
		WITH latest_ledger AS (
			SELECT balance_after
			FROM wallet_ledgers
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT 1
		)
		SELECT
			COALESCE(w.credit_balance, (SELECT balance_after FROM latest_ledger), 0)::BIGINT,
			COALESCE(w.frozen_credit_balance, 0)::BIGINT,
			COALESCE((
				SELECT COUNT(*)
				FROM recharge_orders
				WHERE user_id = $1
				  AND status IN ('awaiting_manual_review', 'pending_payment', 'processing')
			), 0)::BIGINT
		FROM (SELECT $1::text AS user_id) seed
		LEFT JOIN billing_wallets w ON w.user_id = seed.user_id
	`, userID).Scan(&summary.CreditBalance, &summary.FrozenCreditBalance, &summary.PendingRechargeCount); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT q.meter_code, m.name, m.unit, COALESCE(SUM(q.remaining_total), 0)::BIGINT,
		       MIN(q.expires_at) FILTER (WHERE q.expires_at IS NOT NULL)
		FROM billing_quota_accounts q
		INNER JOIN billing_meters m ON m.code = q.meter_code
		WHERE q.user_id = $1
		  AND q.status = 'active'
		  AND q.remaining_total > 0
		  AND (q.expires_at IS NULL OR q.expires_at > NOW())
		GROUP BY q.meter_code, m.name, m.unit
		ORDER BY q.meter_code ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.BillingQuotaBalance
		var expiresAt *time.Time
		if scanErr := rows.Scan(
			&item.MeterCode,
			&item.MeterName,
			&item.Unit,
			&item.RemainingTotal,
			&expiresAt,
		); scanErr != nil {
			return nil, scanErr
		}
		item.NearestExpiresAt = expiresAt
		summary.QuotaBalances = append(summary.QuotaBalances, item)
	}
	return summary, rows.Err()
}

func (s *Store) ListWalletLedgerByUser(ctx context.Context, userID string) ([]domain.WalletLedger, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, entry_type, amount_delta, balance_before, balance_after, meter_code, quantity, unit,
		       unit_price_credits, description, reference_type, reference_id, recharge_order_id,
		       payment_transaction_id, metadata, created_at
		FROM wallet_ledgers
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.WalletLedger, 0)
	for rows.Next() {
		var item domain.WalletLedger
		var meterCode *string
		var quantity *int64
		var unit *string
		var unitPriceCredits *int64
		var description *string
		var referenceType *string
		var referenceID *string
		var rechargeOrderID *string
		var paymentTransactionID *string
		var metadata []byte

		if scanErr := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.EntryType,
			&item.AmountDelta,
			&item.BalanceBefore,
			&item.BalanceAfter,
			&meterCode,
			&quantity,
			&unit,
			&unitPriceCredits,
			&description,
			&referenceType,
			&referenceID,
			&rechargeOrderID,
			&paymentTransactionID,
			&metadata,
			&item.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}

		item.MeterCode = meterCode
		item.Quantity = quantity
		item.Unit = unit
		item.UnitPriceCredits = unitPriceCredits
		item.Description = description
		item.ReferenceType = referenceType
		item.ReferenceID = referenceID
		item.RechargeOrderID = rechargeOrderID
		item.PaymentTransactionID = paymentTransactionID
		item.Metadata = bytesOrNil(metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListRechargeOrdersByUser(ctx context.Context, userID string, limit int) ([]domain.RechargeOrder, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, order_no, user_id, package_id, package_snapshot, channel, status, subject, body, currency,
		       amount_cents, credit_amount, payment_payload, customer_service_payload, provider_transaction_id,
		       provider_status, expires_at, paid_at, closed_at, created_at, updated_at
		FROM recharge_orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.RechargeOrder, 0)
	for rows.Next() {
		item, scanErr := scanRechargeOrder(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) getRechargeOrderByID(ctx context.Context, userID string, orderID string) (*domain.RechargeOrder, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, order_no, user_id, package_id, package_snapshot, channel, status, subject, body, currency,
		       amount_cents, credit_amount, payment_payload, customer_service_payload, provider_transaction_id,
		       provider_status, expires_at, paid_at, closed_at, created_at, updated_at
		FROM recharge_orders
		WHERE user_id = $1
		  AND id = $2
	`, userID, orderID)

	item, err := scanRechargeOrder(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) GetRechargeOrderByID(ctx context.Context, userID string, orderID string) (*domain.RechargeOrder, error) {
	return s.getRechargeOrderByID(ctx, userID, orderID)
}

func (s *Store) ListRechargeOrderEvents(ctx context.Context, userID string, orderID string) ([]domain.RechargeOrderEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.recharge_order_id, e.user_id, e.event_type, e.status, e.message, e.payload, e.created_at
		FROM recharge_order_events e
		INNER JOIN recharge_orders o ON o.id = e.recharge_order_id
		WHERE e.user_id = $1
		  AND e.recharge_order_id = $2
		ORDER BY e.created_at ASC
	`, userID, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.RechargeOrderEvent, 0)
	for rows.Next() {
		item, scanErr := scanRechargeOrderEvent(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) CreateRechargeOrder(ctx context.Context, input CreateRechargeOrderInput) (*domain.RechargeOrder, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO recharge_orders (
			id, order_no, user_id, package_id, package_snapshot, channel, status, subject, body, currency,
			amount_cents, credit_amount, payment_payload, customer_service_payload, provider_status, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, input.ID, input.OrderNo, input.UserID, input.PackageID, input.PackageSnapshot, input.Channel, input.Status,
		input.Subject, input.Body, input.Currency, input.AmountCents, input.CreditAmount, input.PaymentPayload,
		input.CustomerServicePayload, input.ProviderStatus, input.ExpiresAt); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO payment_transactions (
			id, recharge_order_id, user_id, channel, transaction_kind, out_trade_no, status, request_payload, response_payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, input.TransactionID, input.ID, input.UserID, input.Channel, input.TransactionKind, input.TransactionOutTradeNo,
		input.TransactionStatus, input.TransactionRequest, input.TransactionResponse); err != nil {
		return nil, err
	}

	if err := s.appendRechargeOrderEventTx(
		ctx,
		tx,
		fmt.Sprintf("%s-created", input.ID),
		input.ID,
		input.UserID,
		"created",
		input.Status,
		stringPtr("充值订单已创建"),
		input.PaymentPayload,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	order, err := s.getRechargeOrderByID(ctx, input.UserID, input.ID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("created recharge order not found")
	}
	return order, nil
}

func (s *Store) SubmitManualRecharge(ctx context.Context, userID string, orderID string, input SubmitManualRechargeInput) (*domain.RechargeOrder, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT id, order_no, user_id, package_id, package_snapshot, channel, status, subject, body, currency,
		       amount_cents, credit_amount, payment_payload, customer_service_payload, provider_transaction_id,
		       provider_status, expires_at, paid_at, closed_at, created_at, updated_at
		FROM recharge_orders
		WHERE user_id = $1
		  AND id = $2
		FOR UPDATE
	`, userID, orderID)

	order, err := scanRechargeOrder(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if order.Channel != "manual_cs" {
		return nil, fmt.Errorf("recharge order is not a manual customer-service order")
	}
	if isRechargeOrderCredited(order) {
		return nil, ErrRechargeOrderAlreadyCredited
	}

	if _, err := tx.Exec(ctx, `
		UPDATE recharge_orders
		SET status = $3,
		    provider_transaction_id = COALESCE($4, provider_transaction_id),
		    provider_status = COALESCE($5, provider_status),
		    customer_service_payload = $6,
		    updated_at = NOW()
		WHERE user_id = $1
		  AND id = $2
	`, userID, orderID, input.Status, input.ProviderTransactionID, input.ProviderStatus, input.CustomerServicePayload); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE payment_transactions
		SET status = 'processing',
		    provider_transaction_id = COALESCE($3, provider_transaction_id),
		    response_payload = $4,
		    error_message = NULL,
		    updated_at = NOW()
		WHERE recharge_order_id = $1
		  AND user_id = $2
	`, orderID, userID, input.ProviderTransactionID, input.CustomerServicePayload); err != nil {
		return nil, err
	}

	if err := s.appendRechargeOrderEventTx(
		ctx,
		tx,
		input.EventID,
		orderID,
		userID,
		input.EventType,
		input.EventStatus,
		input.EventMessage,
		input.EventPayload,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.getRechargeOrderByID(ctx, userID, orderID)
}
