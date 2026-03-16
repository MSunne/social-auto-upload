package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ApplyUsageMetricInput struct {
	MeterCode string
	Quantity  int64
	Metadata  []byte
}

type ApplyUsageBillingInput struct {
	UserID     string
	SourceType string
	SourceID   string
	ModelName  string
	JobType    string
	Metrics    []ApplyUsageMetricInput
}

type UsageBillingDetail struct {
	MeterCode     string `json:"meterCode"`
	Quantity      int64  `json:"quantity"`
	Units         int64  `json:"units"`
	DebitCredits  int64  `json:"debitCredits"`
	QuotaUsed     int64  `json:"quotaUsed"`
	ChargeMode    string `json:"chargeMode"`
	BillStatus    string `json:"billStatus"`
	BillMessage   string `json:"billMessage,omitempty"`
	PricingRuleID string `json:"pricingRuleId,omitempty"`
}

type ApplyUsageBillingResult struct {
	TotalCredits  int64                `json:"totalCredits"`
	BillStatus    string               `json:"billStatus"`
	BillMessage   string               `json:"billMessage,omitempty"`
	AlreadyBilled bool                 `json:"alreadyBilled"`
	Details       []UsageBillingDetail `json:"details"`
}

type pricingRuleRecord struct {
	ID                string
	MeterCode         string
	ChargeMode        string
	QuotaMeterCode    *string
	UnitSize          int64
	WalletDebitAmount int64
}

type quotaAccountRecord struct {
	ID             string
	MeterCode      string
	RemainingTotal int64
	ExpiresAt      *time.Time
}

type walletLedgerPlan struct {
	meterCode    string
	quantity     int64
	unit         string
	debitCredits int64
	description  string
	payload      []byte
}

type quotaLedgerPlan struct {
	accountID     string
	meterCode     string
	amountDelta   int64
	description   string
	referenceType *string
	referenceID   *string
	payload       []byte
}

type usageLedgerRefs struct {
	walletLedgerIDs map[string][]string
	quotaLedgerIDs  map[string][]string
	quotaAccountIDs map[string][]string
}

func (s *Store) ApplyUsageBilling(ctx context.Context, input ApplyUsageBillingInput) (*ApplyUsageBillingResult, error) {
	if strings.TrimSpace(input.UserID) == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if strings.TrimSpace(input.SourceType) == "" || strings.TrimSpace(input.SourceID) == "" {
		return nil, fmt.Errorf("source type and source id are required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	existing, err := usageBillingSummaryBySourceTx(ctx, tx, input.SourceType, input.SourceID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.AlreadyBilled {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return existing, nil
	}

	rules, err := loadPricingRulesForUsageTx(ctx, tx, input.ModelName, input.JobType)
	if err != nil {
		return nil, err
	}

	metrics := make([]ApplyUsageMetricInput, 0, len(input.Metrics))
	quotaMeterCodes := make(map[string]struct{})
	for _, metric := range input.Metrics {
		if strings.TrimSpace(metric.MeterCode) == "" || metric.Quantity <= 0 {
			continue
		}
		metrics = append(metrics, metric)
		if rule, ok := rules[strings.TrimSpace(metric.MeterCode)]; ok && rule.QuotaMeterCode != nil && strings.TrimSpace(*rule.QuotaMeterCode) != "" {
			quotaMeterCodes[strings.TrimSpace(*rule.QuotaMeterCode)] = struct{}{}
		}
	}
	if len(metrics) == 0 {
		result := &ApplyUsageBillingResult{
			BillStatus:  "skipped",
			BillMessage: "no billable usage metrics",
			Details:     []UsageBillingDetail{},
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return result, nil
	}

	walletBalance, err := ensureWalletAndLockTx(ctx, tx, input.UserID)
	if err != nil {
		return nil, err
	}
	quotaAccounts, err := loadQuotaAccountsForUsageTx(ctx, tx, input.UserID, quotaMeterCodes)
	if err != nil {
		return nil, err
	}

	result := &ApplyUsageBillingResult{
		BillStatus: "billed",
		Details:    make([]UsageBillingDetail, 0, len(metrics)),
	}
	referenceType := stringPtr(input.SourceType)
	referenceID := stringPtr(input.SourceID)
	walletPlans := make([]walletLedgerPlan, 0, len(metrics))
	quotaPlans := make([]quotaLedgerPlan, 0, len(metrics))

	for _, metric := range metrics {
		detail, plannedWallet, plannedQuota, ok := planUsageCharge(metric, rules[strings.TrimSpace(metric.MeterCode)], walletBalance, quotaAccounts)
		result.Details = append(result.Details, detail)
		if !ok {
			result.BillStatus = "failed"
			if result.BillMessage == "" {
				result.BillMessage = detail.BillMessage
			}
			break
		}
		walletBalance -= plannedWallet.debitCredits
		result.TotalCredits += plannedWallet.debitCredits
		if plannedWallet.debitCredits > 0 {
			plannedWallet.payload = mustJSONMap(map[string]any{
				"sourceType":     input.SourceType,
				"sourceId":       input.SourceID,
				"meterCode":      metric.MeterCode,
				"quantity":       detail.Quantity,
				"units":          detail.Units,
				"debitedCredits": detail.DebitCredits,
				"jobType":        input.JobType,
				"modelName":      input.ModelName,
			})
			walletPlans = append(walletPlans, plannedWallet)
		}
		for _, quotaPlan := range plannedQuota {
			quotaPlan.referenceType = referenceType
			quotaPlan.referenceID = referenceID
			quotaPlans = append(quotaPlans, quotaPlan)
		}
	}

	if result.BillStatus == "failed" {
		if err := insertFailedUsageEventsTx(ctx, tx, input, result.Details); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return result, nil
	}

	refs := usageLedgerRefs{
		walletLedgerIDs: make(map[string][]string),
		quotaLedgerIDs:  make(map[string][]string),
		quotaAccountIDs: make(map[string][]string),
	}
	for _, plan := range quotaPlans {
		ledgerID, err := applyQuotaLedgerPlanTx(ctx, tx, input.UserID, plan)
		if err != nil {
			return nil, err
		}
		appendMeterReference(refs.quotaAccountIDs, plan.meterCode, plan.accountID)
		if ledgerID != "" {
			appendMeterReference(refs.quotaLedgerIDs, plan.meterCode, ledgerID)
		}
	}

	currentBalance, err := ensureWalletAndLockTx(ctx, tx, input.UserID)
	if err != nil {
		return nil, err
	}
	for _, plan := range walletPlans {
		ledgerID, nextBalance, err := applyWalletLedgerPlanTx(ctx, tx, input.UserID, currentBalance, plan, referenceType, referenceID)
		if err != nil {
			return nil, err
		}
		currentBalance = nextBalance
		if ledgerID != "" {
			appendMeterReference(refs.walletLedgerIDs, plan.meterCode, ledgerID)
		}
	}

	if err := s.releaseDistributionCommissionForUsageTx(ctx, tx, input.UserID, input.SourceType, input.SourceID, result.TotalCredits); err != nil {
		return nil, err
	}

	if err := insertBilledUsageEventsTx(ctx, tx, input, result.Details, refs); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if result.BillMessage == "" {
		result.BillMessage = fmt.Sprintf("billed %d credits", result.TotalCredits)
	}
	return result, nil
}

func usageBillingSummaryBySourceTx(ctx context.Context, tx pgx.Tx, sourceType string, sourceID string) (*ApplyUsageBillingResult, error) {
	var billedCount int64
	var totalCredits int64
	err := tx.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE bill_status = 'billed')::BIGINT,
			COALESCE(SUM(
				CASE
					WHEN bill_status = 'billed' AND payload IS NOT NULL AND payload ? 'debitedCredits'
						THEN NULLIF(payload ->> 'debitedCredits', '')::BIGINT
					ELSE 0
				END
			), 0)::BIGINT
		FROM billing_usage_events
		WHERE source_type = $1
		  AND source_id = $2
	`, sourceType, sourceID).Scan(&billedCount, &totalCredits)
	if err != nil {
		return nil, err
	}
	if billedCount == 0 {
		return nil, nil
	}
	return &ApplyUsageBillingResult{
		TotalCredits:  totalCredits,
		BillStatus:    "billed",
		BillMessage:   "usage already billed",
		AlreadyBilled: true,
		Details:       []UsageBillingDetail{},
	}, nil
}

func loadPricingRulesForUsageTx(ctx context.Context, tx pgx.Tx, modelName string, jobType string) (map[string]pricingRuleRecord, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, meter_code, charge_mode, quota_meter_code, unit_size, wallet_debit_amount
		FROM billing_pricing_rules
		WHERE is_enabled = TRUE
		  AND applies_to = 'model'
		  AND model_name = $1
		  AND job_type = $2
		ORDER BY sort_order ASC, created_at ASC
	`, strings.TrimSpace(modelName), strings.TrimSpace(jobType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]pricingRuleRecord)
	for rows.Next() {
		var item pricingRuleRecord
		if scanErr := rows.Scan(
			&item.ID,
			&item.MeterCode,
			&item.ChargeMode,
			&item.QuotaMeterCode,
			&item.UnitSize,
			&item.WalletDebitAmount,
		); scanErr != nil {
			return nil, scanErr
		}
		result[item.MeterCode] = item
	}
	return result, rows.Err()
}

func ensureWalletAndLockTx(ctx context.Context, tx pgx.Tx, userID string) (int64, error) {
	if _, err := tx.Exec(ctx, `
		INSERT INTO billing_wallets (user_id, credit_balance, frozen_credit_balance)
		VALUES ($1, 0, 0)
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return 0, err
	}

	var balance int64
	if err := tx.QueryRow(ctx, `
		SELECT credit_balance
		FROM billing_wallets
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&balance); err != nil {
		return 0, err
	}
	return balance, nil
}

func loadQuotaAccountsForUsageTx(ctx context.Context, tx pgx.Tx, userID string, meterCodes map[string]struct{}) (map[string][]*quotaAccountRecord, error) {
	result := make(map[string][]*quotaAccountRecord)
	if len(meterCodes) == 0 {
		return result, nil
	}

	codes := make([]string, 0, len(meterCodes))
	for code := range meterCodes {
		codes = append(codes, code)
	}

	rows, err := tx.Query(ctx, `
		SELECT id, meter_code, remaining_total, expires_at
		FROM billing_quota_accounts
		WHERE user_id = $1
		  AND meter_code = ANY($2)
		  AND status = 'active'
		  AND remaining_total > 0
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY expires_at ASC NULLS LAST, created_at ASC
		FOR UPDATE
	`, userID, codes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		item := &quotaAccountRecord{}
		if scanErr := rows.Scan(&item.ID, &item.MeterCode, &item.RemainingTotal, &item.ExpiresAt); scanErr != nil {
			return nil, scanErr
		}
		result[item.MeterCode] = append(result[item.MeterCode], item)
	}
	return result, rows.Err()
}

func planUsageCharge(metric ApplyUsageMetricInput, rule pricingRuleRecord, walletBalance int64, quotaAccounts map[string][]*quotaAccountRecord) (UsageBillingDetail, walletLedgerPlan, []quotaLedgerPlan, bool) {
	detail := UsageBillingDetail{
		MeterCode:     strings.TrimSpace(metric.MeterCode),
		Quantity:      metric.Quantity,
		PricingRuleID: rule.ID,
		ChargeMode:    rule.ChargeMode,
		BillStatus:    "billed",
	}

	if rule.ID == "" {
		detail.BillStatus = "failed"
		detail.BillMessage = "pricing rule not found"
		return detail, walletLedgerPlan{}, nil, false
	}

	unitSize := rule.UnitSize
	if unitSize <= 0 {
		unitSize = 1
	}
	detail.Units = ceilDiv(metric.Quantity, unitSize)

	walletPlan := walletLedgerPlan{
		meterCode:   detail.MeterCode,
		quantity:    metric.Quantity,
		unit:        "credit",
		description: fmt.Sprintf("AI %s 计费", detail.MeterCode),
	}
	quotaPlans := make([]quotaLedgerPlan, 0)

	switch strings.TrimSpace(rule.ChargeMode) {
	case "wallet_only":
		detail.DebitCredits = detail.Units * rule.WalletDebitAmount
		if walletBalance < detail.DebitCredits {
			detail.BillStatus = "failed"
			detail.BillMessage = "wallet credits insufficient"
			return detail, walletLedgerPlan{}, nil, false
		}
		walletPlan.debitCredits = detail.DebitCredits
	case "quota_first_wallet_fallback":
		quotaMeterCode := ""
		if rule.QuotaMeterCode != nil {
			quotaMeterCode = strings.TrimSpace(*rule.QuotaMeterCode)
		}
		remainingUnits := detail.Units
		if quotaMeterCode != "" {
			for _, account := range quotaAccounts[quotaMeterCode] {
				if remainingUnits <= 0 {
					break
				}
				if account.RemainingTotal <= 0 {
					continue
				}
				used := minInt64(account.RemainingTotal, remainingUnits)
				account.RemainingTotal -= used
				remainingUnits -= used
				detail.QuotaUsed += used
				quotaPlans = append(quotaPlans, quotaLedgerPlan{
					accountID:   account.ID,
					meterCode:   quotaMeterCode,
					amountDelta: -used,
					description: fmt.Sprintf("AI %s 套餐抵扣", detail.MeterCode),
					payload: mustJSONMap(map[string]any{
						"meterCode": metric.MeterCode,
						"quantity":  metric.Quantity,
						"units":     detail.Units,
						"quotaUsed": used,
					}),
				})
			}
		}
		if remainingUnits > 0 {
			detail.DebitCredits = remainingUnits * rule.WalletDebitAmount
			if walletBalance < detail.DebitCredits {
				detail.BillStatus = "failed"
				detail.BillMessage = "wallet credits insufficient for fallback debit"
				return detail, walletLedgerPlan{}, nil, false
			}
			walletPlan.debitCredits = detail.DebitCredits
		}
	default:
		detail.BillStatus = "failed"
		detail.BillMessage = "unsupported charge mode"
		return detail, walletLedgerPlan{}, nil, false
	}

	return detail, walletPlan, quotaPlans, true
}

func applyQuotaLedgerPlanTx(ctx context.Context, tx pgx.Tx, userID string, plan quotaLedgerPlan) (string, error) {
	var remainingAfter int64
	if err := tx.QueryRow(ctx, `
		UPDATE billing_quota_accounts
		SET used_total = used_total + ABS($3),
		    remaining_total = remaining_total + $3,
		    updated_at = NOW()
		WHERE id = $1
		  AND user_id = $2
		  AND remaining_total >= ABS($3)
		RETURNING remaining_total
	`, plan.accountID, userID, plan.amountDelta).Scan(&remainingAfter); err != nil {
		return "", err
	}

	ledgerID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO billing_quota_ledgers (
			id, quota_account_id, user_id, meter_code, amount_delta, remaining_after,
			description, reference_type, reference_id, payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, ledgerID, plan.accountID, userID, plan.meterCode, plan.amountDelta, remainingAfter,
		stringPtr(plan.description), plan.referenceType, plan.referenceID, plan.payload); err != nil {
		return "", err
	}
	return ledgerID, nil
}

func applyWalletLedgerPlanTx(ctx context.Context, tx pgx.Tx, userID string, currentBalance int64, plan walletLedgerPlan, referenceType *string, referenceID *string) (string, int64, error) {
	if plan.debitCredits <= 0 {
		return "", currentBalance, nil
	}
	if currentBalance < plan.debitCredits {
		return "", currentBalance, fmt.Errorf("wallet credits insufficient")
	}

	nextBalance := currentBalance - plan.debitCredits
	if _, err := tx.Exec(ctx, `
		UPDATE billing_wallets
		SET credit_balance = $2,
		    updated_at = NOW()
		WHERE user_id = $1
	`, userID, nextBalance); err != nil {
		return "", currentBalance, err
	}

	ledgerID := uuid.NewString()
	amountDelta := -plan.debitCredits
	meterCode := strings.TrimSpace(plan.meterCode)
	unitPrice := plan.debitCredits
	if plan.quantity > 0 {
		unitPrice = int64(math.Ceil(float64(plan.debitCredits) / float64(maxInt64(plan.quantity, 1))))
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO wallet_ledgers (
			id, user_id, entry_type, amount_delta, balance_before, balance_after, meter_code, quantity,
			unit, unit_price_credits, description, reference_type, reference_id, metadata
		)
		VALUES ($1, $2, 'usage_debit', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, ledgerID, userID, amountDelta, currentBalance, nextBalance, meterCode, plan.quantity, "usage",
		unitPrice, stringPtr(plan.description), referenceType, referenceID, plan.payload); err != nil {
		return "", currentBalance, err
	}

	return ledgerID, nextBalance, nil
}

func insertBilledUsageEventsTx(ctx context.Context, tx pgx.Tx, input ApplyUsageBillingInput, details []UsageBillingDetail, refs usageLedgerRefs) error {
	for _, detail := range details {
		payload := map[string]any{
			"debitedCredits": detail.DebitCredits,
			"quantity":       detail.Quantity,
			"units":          detail.Units,
			"quotaUsed":      detail.QuotaUsed,
			"chargeMode":     detail.ChargeMode,
			"pricingRuleId":  detail.PricingRuleID,
		}
		if metricMetadata := metricMetadataByMeter(input.Metrics, detail.MeterCode); metricMetadata != nil {
			payload["metricMetadata"] = metricMetadata
		}
		if ids := refs.walletLedgerIDs[detail.MeterCode]; len(ids) > 0 {
			payload["walletLedgerIds"] = ids
		}
		if ids := refs.quotaLedgerIDs[detail.MeterCode]; len(ids) > 0 {
			payload["quotaLedgerIds"] = ids
		}
		if ids := refs.quotaAccountIDs[detail.MeterCode]; len(ids) > 0 {
			payload["quotaAccountIds"] = ids
		}
		walletLedgerID := firstStringPtr(refs.walletLedgerIDs[detail.MeterCode])
		quotaLedgerID := firstStringPtr(refs.quotaLedgerIDs[detail.MeterCode])
		quotaAccountID := firstStringPtr(refs.quotaAccountIDs[detail.MeterCode])
		if _, err := tx.Exec(ctx, `
			INSERT INTO billing_usage_events (
				id, user_id, source_type, source_id, meter_code, model_name, job_type, usage_quantity,
				pricing_rule_id, quota_account_id, wallet_ledger_id, quota_ledger_id, bill_status, bill_message, payload
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'billed', $13, $14)
		`, uuid.NewString(), input.UserID, input.SourceType, input.SourceID, detail.MeterCode, input.ModelName, input.JobType,
			detail.Quantity, nullableString(detail.PricingRuleID), quotaAccountID, walletLedgerID, quotaLedgerID, nullableString(detail.BillMessage), mustJSONMap(payload)); err != nil {
			return err
		}
	}
	return nil
}

func insertFailedUsageEventsTx(ctx context.Context, tx pgx.Tx, input ApplyUsageBillingInput, details []UsageBillingDetail) error {
	for _, detail := range details {
		payload := map[string]any{
			"debitedCredits": detail.DebitCredits,
			"quantity":       detail.Quantity,
			"units":          detail.Units,
			"quotaUsed":      detail.QuotaUsed,
			"chargeMode":     detail.ChargeMode,
			"pricingRuleId":  detail.PricingRuleID,
		}
		if metricMetadata := metricMetadataByMeter(input.Metrics, detail.MeterCode); metricMetadata != nil {
			payload["metricMetadata"] = metricMetadata
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO billing_usage_events (
				id, user_id, source_type, source_id, meter_code, model_name, job_type, usage_quantity,
				pricing_rule_id, bill_status, bill_message, payload
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'failed', $10, $11)
		`, uuid.NewString(), input.UserID, input.SourceType, input.SourceID, detail.MeterCode, input.ModelName, input.JobType,
			detail.Quantity, nullableString(detail.PricingRuleID), nullableString(detail.BillMessage), mustJSONMap(payload)); err != nil {
			return err
		}
	}
	return nil
}

func ceilDiv(value int64, divisor int64) int64 {
	if divisor <= 0 {
		return value
	}
	return (value + divisor - 1) / divisor
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func mustJSONMap(value map[string]any) []byte {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return data
}

func nullableString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func appendMeterReference(target map[string][]string, meterCode string, value string) {
	meterCode = strings.TrimSpace(meterCode)
	value = strings.TrimSpace(value)
	if meterCode == "" || value == "" {
		return
	}
	target[meterCode] = append(target[meterCode], value)
}

func firstStringPtr(values []string) *string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return &trimmed
		}
	}
	return nil
}

func metricMetadataByMeter(metrics []ApplyUsageMetricInput, meterCode string) any {
	meterCode = strings.TrimSpace(meterCode)
	for _, metric := range metrics {
		if strings.TrimSpace(metric.MeterCode) != meterCode || len(metric.Metadata) == 0 {
			continue
		}
		var payload any
		if err := json.Unmarshal(metric.Metadata, &payload); err == nil {
			return payload
		}
		break
	}
	return nil
}
