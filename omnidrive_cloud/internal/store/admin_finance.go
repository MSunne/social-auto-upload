package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

var (
	ErrBillingPackageNotFound              = errors.New("billing package not found")
	ErrBillingPackageAlreadyExists         = errors.New("billing package already exists")
	ErrWalletAdjustmentUserNotFound        = errors.New("wallet adjustment user not found")
	ErrWalletAdjustmentAmountZero          = errors.New("wallet adjustment amount must not be 0")
	ErrWalletAdjustmentInsufficientBalance = errors.New("wallet balance insufficient for debit adjustment")
)

func scanPaymentTransaction(scan scanFn) (*domain.PaymentTransaction, error) {
	var item domain.PaymentTransaction
	var providerTransactionID *string
	var requestPayload []byte
	var responsePayload []byte
	var notifyPayload []byte
	var errorMessage *string
	var paidAt *time.Time

	if err := scan(
		&item.ID,
		&item.RechargeOrderID,
		&item.UserID,
		&item.Channel,
		&item.TransactionKind,
		&item.OutTradeNo,
		&providerTransactionID,
		&item.Status,
		&requestPayload,
		&responsePayload,
		&notifyPayload,
		&errorMessage,
		&paidAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}

	item.ProviderTransactionID = providerTransactionID
	item.RequestPayload = bytesOrNil(requestPayload)
	item.ResponsePayload = bytesOrNil(responsePayload)
	item.NotifyPayload = bytesOrNil(notifyPayload)
	item.ErrorMessage = errorMessage
	item.PaidAt = paidAt
	return &item, nil
}

func scanWalletAdjustmentRequest(scan scanFn) (*domain.WalletAdjustmentRequest, error) {
	var item domain.WalletAdjustmentRequest
	var note *string
	var referenceType *string
	var referenceID *string
	var walletLedgerID *string
	var reviewedByAdminID *string
	var reviewedByAdminEmail *string
	var reviewedByAdminName *string
	var payload []byte
	var reviewedAt *time.Time
	var completedAt *time.Time

	if err := scan(
		&item.ID,
		&item.UserID,
		&item.EntryType,
		&item.AmountDelta,
		&item.Reason,
		&note,
		&item.Status,
		&referenceType,
		&referenceID,
		&walletLedgerID,
		&item.RequestedByAdminID,
		&item.RequestedByAdminEmail,
		&item.RequestedByAdminName,
		&reviewedByAdminID,
		&reviewedByAdminEmail,
		&reviewedByAdminName,
		&payload,
		&item.CreatedAt,
		&item.UpdatedAt,
		&reviewedAt,
		&completedAt,
	); err != nil {
		return nil, err
	}

	item.Note = note
	item.ReferenceType = referenceType
	item.ReferenceID = referenceID
	item.WalletLedgerID = walletLedgerID
	item.ReviewedByAdminID = reviewedByAdminID
	item.ReviewedByAdminEmail = reviewedByAdminEmail
	item.ReviewedByAdminName = reviewedByAdminName
	item.Payload = bytesOrNil(payload)
	item.ReviewedAt = reviewedAt
	item.CompletedAt = completedAt
	return &item, nil
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizePackagePaymentChannels(channels []string, allowDefault bool) ([]string, error) {
	if len(channels) == 0 {
		if allowDefault {
			return []string{"manual_cs", "alipay", "wechatpay"}, nil
		}
		return nil, fmt.Errorf("payment channels are required")
	}

	items := make([]string, 0, len(channels))
	seen := make(map[string]struct{}, len(channels))
	for _, raw := range channels {
		value := strings.TrimSpace(strings.ToLower(raw))
		switch value {
		case "manual", "manual_cs", "customer_service", "customer-service":
			value = "manual_cs"
		case "wechat", "wechatpay", "wechat_pay":
			value = "wechatpay"
		case "alipay":
		default:
			return nil, fmt.Errorf("unsupported payment channel: %s", raw)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("payment channels are required")
	}
	return items, nil
}

func buildPackageEntitlementInputsFromDomain(items []domain.BillingPackageEntitlement) []BillingPackageEntitlementInput {
	result := make([]BillingPackageEntitlementInput, 0, len(items))
	for _, item := range items {
		result = append(result, BillingPackageEntitlementInput{
			MeterCode:   item.MeterCode,
			GrantAmount: item.GrantAmount,
			GrantMode:   item.GrantMode,
			SortOrder:   item.SortOrder,
			Description: item.Description,
		})
	}
	return result
}

func normalizePackageEntitlements(packageID string, creditAmount int64, manualBonusCreditAmount int64, raw []BillingPackageEntitlementInput) ([]BillingPackageEntitlementInput, error) {
	if creditAmount < 0 {
		return nil, fmt.Errorf("credit amount must be greater than or equal to 0")
	}
	if manualBonusCreditAmount < 0 {
		return nil, fmt.Errorf("manual bonus credit amount must be greater than or equal to 0")
	}

	items := make([]BillingPackageEntitlementInput, 0, len(raw)+1)
	totalWalletCredits := creditAmount + manualBonusCreditAmount
	if totalWalletCredits > 0 {
		description := stringPtr(fmt.Sprintf("购买套餐后发放 %d 钱包积分", totalWalletCredits))
		items = append(items, BillingPackageEntitlementInput{
			MeterCode:   "wallet_credit",
			GrantAmount: totalWalletCredits,
			GrantMode:   "one_time",
			SortOrder:   10,
			Description: description,
		})
	}

	seen := map[string]struct{}{}
	nextSort := 20
	for _, rawItem := range raw {
		meterCode := strings.TrimSpace(rawItem.MeterCode)
		if meterCode == "" {
			return nil, fmt.Errorf("entitlement meterCode is required")
		}
		if meterCode == "wallet_credit" {
			continue
		}
		if _, exists := seen[meterCode]; exists {
			return nil, fmt.Errorf("duplicate entitlement meterCode: %s", meterCode)
		}
		if rawItem.GrantAmount <= 0 {
			return nil, fmt.Errorf("entitlement grantAmount must be positive")
		}

		grantMode := strings.TrimSpace(rawItem.GrantMode)
		if grantMode == "" {
			grantMode = "one_time"
		}
		sortOrder := rawItem.SortOrder
		if sortOrder == 0 {
			sortOrder = nextSort
			nextSort += 10
		}

		items = append(items, BillingPackageEntitlementInput{
			MeterCode:   meterCode,
			GrantAmount: rawItem.GrantAmount,
			GrantMode:   grantMode,
			SortOrder:   sortOrder,
			Description: trimOptionalString(rawItem.Description),
		})
		seen[meterCode] = struct{}{}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("at least one entitlement is required")
	}
	return items, nil
}

func syncBillingPackageEntitlementsTx(ctx context.Context, tx pgx.Tx, packageID string, entitlements []BillingPackageEntitlementInput) error {
	if _, err := tx.Exec(ctx, `
		DELETE FROM billing_package_entitlements
		WHERE package_id = $1
	`, packageID); err != nil {
		return err
	}

	for _, item := range entitlements {
		entitlementID := fmt.Sprintf("%s-%s", packageID, strings.ReplaceAll(item.MeterCode, "_", "-"))
		if _, err := tx.Exec(ctx, `
			INSERT INTO billing_package_entitlements (
				id, package_id, meter_code, grant_amount, grant_mode, sort_order, description
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, entitlementID, packageID, item.MeterCode, item.GrantAmount, item.GrantMode, item.SortOrder, item.Description); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListAdminBillingPackages(ctx context.Context) ([]domain.BillingPackage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, package_type, channel, payment_channels, currency, price_cents, credit_amount,
		       manual_bonus_credit_amount, badge, description, pricing_payload, expires_in_days, is_enabled, sort_order, created_at, updated_at
		FROM billing_packages
		ORDER BY sort_order ASC, price_cents ASC, created_at ASC
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

func (s *Store) CreateBillingPackage(ctx context.Context, input CreateBillingPackageInput) (*domain.BillingPackage, error) {
	packageID := strings.TrimSpace(input.ID)
	if packageID == "" {
		return nil, fmt.Errorf("package id is required")
	}
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("package name is required")
	}

	existing, err := s.GetBillingPackageByID(ctx, packageID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrBillingPackageAlreadyExists
	}

	paymentChannels, err := normalizePackagePaymentChannels(input.PaymentChannels, true)
	if err != nil {
		return nil, err
	}
	entitlements, err := normalizePackageEntitlements(packageID, input.CreditAmount, input.ManualBonusCreditAmount, input.Entitlements)
	if err != nil {
		return nil, err
	}

	packageType := strings.TrimSpace(input.PackageType)
	if packageType == "" {
		packageType = "credit_topup"
	}
	currency := strings.TrimSpace(input.Currency)
	if currency == "" {
		currency = "CNY"
	}
	paymentChannelsPayload, _ := json.Marshal(paymentChannels)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO billing_packages (
			id, name, package_type, channel, payment_channels, currency, price_cents, credit_amount,
			manual_bonus_credit_amount, badge, description, pricing_payload, expires_in_days, is_enabled, sort_order
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, packageID, strings.TrimSpace(input.Name), packageType, strings.Join(paymentChannels, ","), paymentChannelsPayload,
		currency, input.PriceCents, input.CreditAmount, input.ManualBonusCreditAmount, trimOptionalString(input.Badge),
		trimOptionalString(input.Description), bytesOrNil(input.PricingPayload), input.ExpiresInDays, input.IsEnabled, input.SortOrder); err != nil {
		return nil, err
	}

	if err := syncBillingPackageEntitlementsTx(ctx, tx, packageID, entitlements); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetBillingPackageByID(ctx, packageID)
}

func (s *Store) UpdateBillingPackage(ctx context.Context, packageID string, input UpdateBillingPackageInput) (*domain.BillingPackage, error) {
	current, err := s.GetBillingPackageByID(ctx, packageID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrBillingPackageNotFound
	}

	name := current.Name
	if input.Name != nil {
		name = strings.TrimSpace(*input.Name)
	}
	if name == "" {
		return nil, fmt.Errorf("package name is required")
	}

	packageType := current.PackageType
	if input.PackageType != nil {
		packageType = strings.TrimSpace(*input.PackageType)
	}
	if packageType == "" {
		packageType = "credit_topup"
	}

	currency := current.Currency
	if input.Currency != nil {
		currency = strings.TrimSpace(*input.Currency)
	}
	if currency == "" {
		currency = "CNY"
	}

	priceCents := current.PriceCents
	if input.PriceCents != nil {
		priceCents = *input.PriceCents
	}

	creditAmount := current.CreditAmount
	if input.CreditAmount != nil {
		creditAmount = *input.CreditAmount
	}

	manualBonusCreditAmount := current.ManualBonusCreditAmount
	if input.ManualBonusCreditAmount != nil {
		manualBonusCreditAmount = *input.ManualBonusCreditAmount
	}

	badge := current.Badge
	if input.BadgeTouched {
		badge = trimOptionalString(input.Badge)
	}

	description := current.Description
	if input.DescriptionTouched {
		description = trimOptionalString(input.Description)
	}

	pricingPayload := current.PricingPayload
	if input.PricingPayloadTouched {
		pricingPayload = bytesOrNil(input.PricingPayload)
	}

	expiresInDays := current.ExpiresInDays
	if input.ExpiresInDaysTouched {
		expiresInDays = input.ExpiresInDays
	}

	isEnabled := current.IsEnabled
	if input.IsEnabled != nil {
		isEnabled = *input.IsEnabled
	}

	sortOrder := current.SortOrder
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	}

	paymentChannels := current.PaymentChannels
	if input.PaymentChannelsTouched {
		paymentChannels, err = normalizePackagePaymentChannels(input.PaymentChannels, false)
		if err != nil {
			return nil, err
		}
	}
	if len(paymentChannels) == 0 {
		paymentChannels, err = normalizePackagePaymentChannels(nil, true)
		if err != nil {
			return nil, err
		}
	}

	entitlementSource := buildPackageEntitlementInputsFromDomain(current.Entitlements)
	if input.Entitlements != nil {
		entitlementSource = append([]BillingPackageEntitlementInput{}, (*input.Entitlements)...)
	}
	entitlements, err := normalizePackageEntitlements(packageID, creditAmount, manualBonusCreditAmount, entitlementSource)
	if err != nil {
		return nil, err
	}
	paymentChannelsPayload, _ := json.Marshal(paymentChannels)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE billing_packages
		SET name = $2,
		    package_type = $3,
		    channel = $4,
		    payment_channels = $5,
		    currency = $6,
		    price_cents = $7,
		    credit_amount = $8,
		    manual_bonus_credit_amount = $9,
		    badge = $10,
		    description = $11,
		    pricing_payload = $12,
		    expires_in_days = $13,
		    is_enabled = $14,
		    sort_order = $15,
		    updated_at = NOW()
		WHERE id = $1
	`, packageID, name, packageType, strings.Join(paymentChannels, ","), paymentChannelsPayload, currency, priceCents,
		creditAmount, manualBonusCreditAmount, badge, description, pricingPayload, expiresInDays, isEnabled, sortOrder); err != nil {
		return nil, err
	}

	if err := syncBillingPackageEntitlementsTx(ctx, tx, packageID, entitlements); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetBillingPackageByID(ctx, packageID)
}

func (s *Store) ListPaymentTransactionsByRechargeOrderID(ctx context.Context, orderID string) ([]domain.PaymentTransaction, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, recharge_order_id, user_id, channel, transaction_kind, out_trade_no, provider_transaction_id,
		       status, request_payload, response_payload, notify_payload, error_message, paid_at, created_at, updated_at
		FROM payment_transactions
		WHERE recharge_order_id = $1
		ORDER BY created_at ASC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.PaymentTransaction, 0)
	for rows.Next() {
		item, scanErr := scanPaymentTransaction(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) GetPaymentTransactionByID(ctx context.Context, transactionID string) (*domain.PaymentTransaction, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, recharge_order_id, user_id, channel, transaction_kind, out_trade_no, provider_transaction_id,
		       status, request_payload, response_payload, notify_payload, error_message, paid_at, created_at, updated_at
		FROM payment_transactions
		WHERE id = $1
	`, transactionID)

	item, err := scanPaymentTransaction(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) ListWalletLedgersByRechargeOrderID(ctx context.Context, orderID string) ([]domain.WalletLedger, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, entry_type, amount_delta, balance_before, balance_after, meter_code, quantity, unit,
		       unit_price_credits, description, reference_type, reference_id, recharge_order_id, payment_transaction_id,
		       metadata, created_at
		FROM wallet_ledgers
		WHERE recharge_order_id = $1
		ORDER BY created_at ASC
	`, orderID)
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

func (s *Store) GetAdminWalletLedgerByID(ctx context.Context, ledgerID string) (*domain.AdminWalletLedgerRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			wl.id, wl.user_id, wl.entry_type, wl.amount_delta, wl.balance_before, wl.balance_after, wl.meter_code,
			wl.quantity, wl.unit, wl.unit_price_credits, wl.description, wl.reference_type, wl.reference_id,
			wl.recharge_order_id, wl.payment_transaction_id, wl.metadata, wl.created_at,
			u.id, u.email, u.name
		FROM wallet_ledgers wl
		INNER JOIN users u ON u.id = wl.user_id
		WHERE wl.id = $1
	`, ledgerID)

	var item domain.AdminWalletLedgerRow
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

	if err := row.Scan(
		&item.Ledger.ID,
		&item.Ledger.UserID,
		&item.Ledger.EntryType,
		&item.Ledger.AmountDelta,
		&item.Ledger.BalanceBefore,
		&item.Ledger.BalanceAfter,
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
		&item.Ledger.CreatedAt,
		&item.User.ID,
		&item.User.Email,
		&item.User.Name,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	item.Ledger.MeterCode = meterCode
	item.Ledger.Quantity = quantity
	item.Ledger.Unit = unit
	item.Ledger.UnitPriceCredits = unitPriceCredits
	item.Ledger.Description = description
	item.Ledger.ReferenceType = referenceType
	item.Ledger.ReferenceID = referenceID
	item.Ledger.RechargeOrderID = rechargeOrderID
	item.Ledger.PaymentTransactionID = paymentTransactionID
	item.Ledger.Metadata = bytesOrNil(metadata)
	return &item, nil
}

func (s *Store) GetWalletAdjustmentRequestByID(ctx context.Context, adjustmentID string) (*domain.WalletAdjustmentRequest, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, entry_type, amount_delta, reason, note, status, reference_type, reference_id,
		       wallet_ledger_id, admin_user_id, admin_email, admin_name, admin_user_id, admin_email, admin_name,
		       payload, created_at, updated_at, reviewed_at, completed_at
		FROM wallet_adjustment_requests
		WHERE id = $1
	`, adjustmentID)

	item, err := scanWalletAdjustmentRequest(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) GetWalletAdjustmentRequestByLedgerID(ctx context.Context, ledgerID string) (*domain.WalletAdjustmentRequest, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, entry_type, amount_delta, reason, note, status, reference_type, reference_id,
		       wallet_ledger_id, admin_user_id, admin_email, admin_name, admin_user_id, admin_email, admin_name,
		       payload, created_at, updated_at, reviewed_at, completed_at
		FROM wallet_adjustment_requests
		WHERE wallet_ledger_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, ledgerID)

	item, err := scanWalletAdjustmentRequest(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) CreateWalletAdjustment(ctx context.Context, input CreateWalletAdjustmentInput) (*domain.WalletAdjustmentRequest, error) {
	if strings.TrimSpace(input.UserID) == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return nil, fmt.Errorf("wallet adjustment reason is required")
	}
	if input.AmountDelta == 0 {
		return nil, ErrWalletAdjustmentAmountZero
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var userID string
	if err := tx.QueryRow(ctx, `
		SELECT id
		FROM users
		WHERE id = $1
	`, strings.TrimSpace(input.UserID)).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWalletAdjustmentUserNotFound
		}
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO billing_wallets (user_id, credit_balance, frozen_credit_balance)
		VALUES ($1, 0, 0)
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return nil, err
	}

	var before int64
	if err := tx.QueryRow(ctx, `
		SELECT credit_balance
		FROM billing_wallets
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&before); err != nil {
		return nil, err
	}

	after := before + input.AmountDelta
	if after < 0 {
		return nil, ErrWalletAdjustmentInsufficientBalance
	}

	if _, err := tx.Exec(ctx, `
		UPDATE billing_wallets
		SET credit_balance = $2,
		    updated_at = NOW()
		WHERE user_id = $1
	`, userID, after); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	adjustmentID := uuid.NewString()
	ledgerID := uuid.NewString()
	entryType := "manual_compensation"
	if input.AmountDelta < 0 {
		entryType = "manual_deduction"
	}
	if input.EntryType != nil && strings.TrimSpace(*input.EntryType) != "" {
		entryType = strings.TrimSpace(*input.EntryType)
	}

	referenceType := trimOptionalString(input.ReferenceType)
	if referenceType == nil {
		referenceType = stringPtr("wallet_adjustment")
	}
	referenceID := trimOptionalString(input.ReferenceID)
	if referenceID == nil {
		referenceID = &adjustmentID
	}

	payload := bytesOrNil(input.Payload)
	if payload == nil {
		payload, _ = json.Marshal(map[string]any{
			"reason":      strings.TrimSpace(input.Reason),
			"note":        valueOrEmpty(input.Note),
			"amountDelta": input.AmountDelta,
			"operator": map[string]any{
				"id":    strings.TrimSpace(input.AdminID),
				"email": strings.TrimSpace(input.AdminEmail),
				"name":  strings.TrimSpace(input.AdminName),
			},
			"referenceType": valueOrEmpty(referenceType),
			"referenceId":   valueOrEmpty(referenceID),
		})
	}

	description := trimOptionalString(input.Note)
	if description == nil {
		description = stringPtr(strings.TrimSpace(input.Reason))
	}
	meterCode := "wallet_credit"
	quantity := int64(1)
	unit := "adjustment"
	unitPriceCredits := input.AmountDelta
	if _, err := tx.Exec(ctx, `
		INSERT INTO wallet_ledgers (
			id, user_id, entry_type, amount_delta, balance_before, balance_after, meter_code, quantity,
			unit, unit_price_credits, description, reference_type, reference_id, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, ledgerID, userID, entryType, input.AmountDelta, before, after, meterCode, quantity, unit, unitPriceCredits,
		description, referenceType, referenceID, payload); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO wallet_adjustment_requests (
			id, user_id, wallet_ledger_id, entry_type, amount_delta, reason, note, status,
			reference_type, reference_id, admin_user_id, admin_email, admin_name, payload,
			reviewed_at, completed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'applied', $8, $9, $10, $11, $12, $13, $14, $14)
	`, adjustmentID, userID, ledgerID, entryType, input.AmountDelta, strings.TrimSpace(input.Reason), trimOptionalString(input.Note),
		referenceType, referenceID, trimOptionalString(stringPtr(input.AdminID)), trimOptionalString(stringPtr(input.AdminEmail)),
		trimOptionalString(stringPtr(input.AdminName)), payload, now); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetWalletAdjustmentRequestByID(ctx, adjustmentID)
}
