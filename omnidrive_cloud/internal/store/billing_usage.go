package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
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
	MeterCode                  string `json:"meterCode"`
	Quantity                   int64  `json:"quantity"`
	Units                      int64  `json:"units"`
	DebitCredits               int64  `json:"debitCredits"`
	QuotaUsed                  int64  `json:"quotaUsed"`
	DistributionReleaseCredits int64  `json:"distributionReleaseCredits"`
	SupportsFailureRefund      bool   `json:"supportsFailureRefund"`
	ChargeMode                 string `json:"chargeMode"`
	BillStatus                 string `json:"billStatus"`
	BillMessage                string `json:"billMessage,omitempty"`
	PricingRuleID              string `json:"pricingRuleId,omitempty"`
}

type ApplyUsageBillingResult struct {
	TotalCredits               int64                `json:"totalCredits"`
	DistributionReleaseCredits int64                `json:"distributionReleaseCredits"`
	BillStatus                 string               `json:"billStatus"`
	BillMessage                string               `json:"billMessage,omitempty"`
	AlreadyBilled              bool                 `json:"alreadyBilled"`
	Details                    []UsageBillingDetail `json:"details"`
}

type UsageBillingQueuePreviewItem struct {
	Input               ApplyUsageBillingInput  `json:"input"`
	Result              ApplyUsageBillingResult `json:"result"`
	CreditBalanceBefore int64                   `json:"creditBalanceBefore"`
	CreditBalanceAfter  int64                   `json:"creditBalanceAfter"`
}

type usageBillingQueueEvaluation struct {
	Input   ApplyUsageBillingInput
	Metrics []ApplyUsageMetricInput
	Rules   map[string]pricingRuleRecord
}

type pricingRuleRecord struct {
	ID                string
	MeterCode         string
	ChargeMode        string
	QuotaMeterCode    *string
	UnitSize          int64
	WalletDebitAmount int64
	QuantityMetaKey   string
}

type quotaAccountRecord struct {
	ID                           string
	MeterCode                    string
	RemainingTotal               int64
	ExpiresAt                    *time.Time
	RechargeOrderID              *string
	DistributionCommissionItemID *string
	ReleaseUnitCredits           int64
}

type walletLotRecord struct {
	ID                           string
	RechargeOrderID              *string
	DistributionCommissionItemID *string
	RemainingCredits             int64
	ConsumedCredits              int64
	ReleaseUnitCredits           int64
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
	accountID                    string
	meterCode                    string
	amountDelta                  int64
	description                  string
	referenceType                *string
	referenceID                  *string
	payload                      []byte
	rechargeOrderID              *string
	distributionCommissionItemID *string
	releaseUnitCredits           int64
}

type usageLedgerRefs struct {
	walletLedgerIDs map[string][]string
	quotaLedgerIDs  map[string][]string
	quotaAccountIDs map[string][]string
}

// 处理预览用量计费相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) PreviewUsageBilling(ctx context.Context, input ApplyUsageBillingInput) (*ApplyUsageBillingResult, error) {
	if strings.TrimSpace(input.UserID) == "" {
		return nil, fmt.Errorf("user id is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

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
	if err := s.backfillDistributionGrantTrackingTx(ctx, tx, input.UserID); err != nil {
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

	for _, metric := range metrics {
		detail, plannedWallet, _, ok := planUsageCharge(metric, rules[strings.TrimSpace(metric.MeterCode)], walletBalance, quotaAccounts)
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
		result.DistributionReleaseCredits += detail.DistributionReleaseCredits
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// 处理预览用量计费Queue相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) PreviewUsageBillingQueue(ctx context.Context, inputs []ApplyUsageBillingInput) ([]UsageBillingQueuePreviewItem, error) {
	if len(inputs) == 0 {
		return []UsageBillingQueuePreviewItem{}, nil
	}

	userID := strings.TrimSpace(inputs[0].UserID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	walletBalance, err := ensureWalletAndLockTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	quotaAccounts := make(map[string][]*quotaAccountRecord)
	loadedQuotaMeters := make(map[string]struct{})
	evaluations := make([]usageBillingQueueEvaluation, 0, len(inputs))

	for _, input := range inputs {
		if strings.TrimSpace(input.UserID) != userID {
			return nil, fmt.Errorf("preview usage billing queue requires a single user")
		}

		rules, err := loadPricingRulesForUsageTx(ctx, tx, input.ModelName, input.JobType)
		if err != nil {
			return nil, err
		}

		metrics := make([]ApplyUsageMetricInput, 0, len(input.Metrics))
		missingQuotaMeterCodes := make(map[string]struct{})
		for _, metric := range input.Metrics {
			if strings.TrimSpace(metric.MeterCode) == "" || metric.Quantity <= 0 {
				continue
			}
			metrics = append(metrics, metric)
			if rule, ok := rules[strings.TrimSpace(metric.MeterCode)]; ok && rule.QuotaMeterCode != nil {
				quotaMeterCode := strings.TrimSpace(*rule.QuotaMeterCode)
				if quotaMeterCode != "" {
					if _, loaded := loadedQuotaMeters[quotaMeterCode]; !loaded {
						missingQuotaMeterCodes[quotaMeterCode] = struct{}{}
					}
				}
			}
		}

		if len(missingQuotaMeterCodes) > 0 {
			loadedAccounts, err := loadQuotaAccountsForUsageTx(ctx, tx, userID, missingQuotaMeterCodes)
			if err != nil {
				return nil, err
			}
			for meterCode, accounts := range loadedAccounts {
				quotaAccounts[meterCode] = accounts
				loadedQuotaMeters[meterCode] = struct{}{}
			}
			for meterCode := range missingQuotaMeterCodes {
				if _, exists := loadedQuotaMeters[meterCode]; !exists {
					loadedQuotaMeters[meterCode] = struct{}{}
				}
			}
		}

		evaluations = append(evaluations, usageBillingQueueEvaluation{
			Input:   input,
			Metrics: metrics,
			Rules:   rules,
		})
	}

	return previewUsageBillingQueueEvaluations(evaluations, walletBalance, quotaAccounts), nil
}

// 处理预览用量计费QueueEvaluations相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func previewUsageBillingQueueEvaluations(evaluations []usageBillingQueueEvaluation, walletBalance int64, quotaAccounts map[string][]*quotaAccountRecord) []UsageBillingQueuePreviewItem {
	items := make([]UsageBillingQueuePreviewItem, 0, len(evaluations))
	for _, evaluation := range evaluations {
		localWalletBalance := walletBalance
		localQuotaAccounts := cloneQuotaAccountsForPreview(quotaAccounts)
		item := UsageBillingQueuePreviewItem{
			Input:               evaluation.Input,
			CreditBalanceBefore: walletBalance,
			Result: ApplyUsageBillingResult{
				BillStatus: "billed",
				Details:    make([]UsageBillingDetail, 0, len(evaluation.Metrics)),
			},
		}
		if len(evaluation.Metrics) == 0 {
			item.Result.BillStatus = "skipped"
			item.Result.BillMessage = "no billable usage metrics"
			item.CreditBalanceAfter = walletBalance
			items = append(items, item)
			continue
		}

		for _, metric := range evaluation.Metrics {
			detail, plannedWallet, _, ok := planUsageCharge(metric, evaluation.Rules[strings.TrimSpace(metric.MeterCode)], localWalletBalance, localQuotaAccounts)
			item.Result.Details = append(item.Result.Details, detail)
			if !ok {
				item.Result.BillStatus = "failed"
				if item.Result.BillMessage == "" {
					item.Result.BillMessage = detail.BillMessage
				}
				break
			}
			localWalletBalance -= plannedWallet.debitCredits
			item.Result.TotalCredits += plannedWallet.debitCredits
			item.Result.DistributionReleaseCredits += detail.DistributionReleaseCredits
		}

		if item.Result.BillStatus == "billed" {
			walletBalance = localWalletBalance
			quotaAccounts = localQuotaAccounts
		}
		item.CreditBalanceAfter = walletBalance
		items = append(items, item)
	}
	return items
}

// 根据预览计算clone额度账号，供存储层链路复用关键派生结果。
func cloneQuotaAccountsForPreview(source map[string][]*quotaAccountRecord) map[string][]*quotaAccountRecord {
	if len(source) == 0 {
		return map[string][]*quotaAccountRecord{}
	}

	cloned := make(map[string][]*quotaAccountRecord, len(source))
	for meterCode, accounts := range source {
		copiedAccounts := make([]*quotaAccountRecord, 0, len(accounts))
		for _, account := range accounts {
			if account == nil {
				continue
			}
			copied := *account
			copiedAccounts = append(copiedAccounts, &copied)
		}
		cloned[meterCode] = copiedAccounts
	}
	return cloned
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
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
	if err := s.backfillDistributionGrantTrackingTx(ctx, tx, input.UserID); err != nil {
		return nil, err
	}
	quotaAccounts, err := loadQuotaAccountsForUsageTx(ctx, tx, input.UserID, quotaMeterCodes)
	if err != nil {
		return nil, err
	}
	walletLots, err := loadWalletLotsForUsageTx(ctx, tx, input.UserID)
	if err != nil {
		return nil, err
	}
	sourceSnapshot, err := buildDistributionSourceSnapshotTx(ctx, tx, input.SourceType, input.SourceID)
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
		result.DistributionReleaseCredits += detail.DistributionReleaseCredits
		if plannedWallet.debitCredits > 0 {
			plannedWallet.payload = mustJSONMap(map[string]any{
				"sourceType":                 input.SourceType,
				"sourceId":                   input.SourceID,
				"meterCode":                  metric.MeterCode,
				"quantity":                   detail.Quantity,
				"units":                      detail.Units,
				"debitedCredits":             detail.DebitCredits,
				"distributionReleaseCredits": detail.DistributionReleaseCredits,
				"jobType":                    input.JobType,
				"modelName":                  input.ModelName,
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
		if plan.distributionCommissionItemID != nil && strings.TrimSpace(*plan.distributionCommissionItemID) != "" && plan.releaseUnitCredits > 0 {
			if err := s.applyDistributionCommissionReleaseTx(ctx, tx, distributionCommissionReleaseInput{
				CommissionItemID:     strings.TrimSpace(*plan.distributionCommissionItemID),
				SourceType:           input.SourceType,
				SourceID:             input.SourceID,
				SourceSnapshot:       sourceSnapshot,
				QuotaAccountID:       &plan.accountID,
				QuotaLedgerID:        stringPtr(ledgerID),
				ConsumedCreditsDelta: absInt64(plan.amountDelta) * plan.releaseUnitCredits,
				Metadata:             plan.payload,
			}); err != nil {
				return nil, err
			}
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
		if err := s.applyWalletLotConsumptionsTx(ctx, tx, input.UserID, input.SourceType, input.SourceID, sourceSnapshot, plan, ledgerID, walletLots); err != nil {
			return nil, err
		}
		currentBalance = nextBalance
		if ledgerID != "" {
			appendMeterReference(refs.walletLedgerIDs, plan.meterCode, ledgerID)
		}
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

// 处理用量计费Summary来源事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func usageBillingSummaryBySourceTx(ctx context.Context, tx pgx.Tx, sourceType string, sourceID string) (*ApplyUsageBillingResult, error) {
	var billedCount int64
	var totalCredits int64
	var distributionReleaseCredits int64
	err := tx.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE bill_status = 'billed')::BIGINT,
			COALESCE(SUM(
				CASE
					WHEN bill_status = 'billed' AND payload IS NOT NULL AND payload ? 'debitedCredits'
						THEN NULLIF(payload ->> 'debitedCredits', '')::BIGINT
					ELSE 0
				END
			), 0)::BIGINT,
			COALESCE(SUM(
				CASE
					WHEN bill_status = 'billed' AND payload IS NOT NULL AND payload ? 'distributionReleaseCredits'
						THEN NULLIF(payload ->> 'distributionReleaseCredits', '')::BIGINT
					WHEN bill_status = 'billed' AND payload IS NOT NULL AND payload ? 'debitedCredits'
						THEN NULLIF(payload ->> 'debitedCredits', '')::BIGINT
					ELSE 0
				END
			), 0)::BIGINT
		FROM billing_usage_events
		WHERE source_type = $1
		  AND source_id = $2
	`, sourceType, sourceID).Scan(&billedCount, &totalCredits, &distributionReleaseCredits)
	if err != nil {
		return nil, err
	}
	if billedCount == 0 {
		return nil, nil
	}
	return &ApplyUsageBillingResult{
		TotalCredits:               totalCredits,
		DistributionReleaseCredits: distributionReleaseCredits,
		BillStatus:                 "billed",
		BillMessage:                "usage already billed",
		AlreadyBilled:              true,
		Details:                    []UsageBillingDetail{},
	}, nil
}

// 加载定价规则用量事务，供存储层继续处理当前业务状态。
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	model, err := loadAIModelForUsageBillingTx(ctx, tx, modelName)
	if err != nil {
		return nil, err
	}
	for meterCode, fallbackRule := range buildFallbackPricingRulesForUsage(model, jobType) {
		if _, exists := result[meterCode]; exists {
			continue
		}
		result[meterCode] = fallbackRule
	}
	return result, nil
}

// 加载AI模型用量计费事务，供存储层继续处理当前业务状态。
func loadAIModelForUsageBillingTx(ctx context.Context, tx pgx.Tx, modelName string) (*domain.AIModel, error) {
	row := tx.QueryRow(ctx, `
		SELECT `+aiModelSelectColumns+`
		FROM ai_models
		WHERE model_name = $1
	`, strings.TrimSpace(modelName))

	model, err := scanAIModel(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return model, nil
}

// 构建回退定价规则用量，为存储层生成后续步骤所需的派生参数或载荷。
func buildFallbackPricingRulesForUsage(model *domain.AIModel, jobType string) map[string]pricingRuleRecord {
	if model == nil {
		return nil
	}

	jobType = strings.TrimSpace(strings.ToLower(jobType))
	category := strings.TrimSpace(strings.ToLower(model.Category))
	if jobType == "" {
		jobType = category
	}

	switch jobType {
	case "chat":
		if category != "chat" {
			return nil
		}
		result := make(map[string]pricingRuleRecord)
		if amount := fallbackWalletDebitAmount(model.ChatInputBillingAmount, model.BillingAmount, model.ChatInputRawRate, model.RawRate); amount > 0 {
			result["chat_input_tokens"] = pricingRuleRecord{
				MeterCode:         "chat_input_tokens",
				ChargeMode:        "wallet_only",
				UnitSize:          1000,
				WalletDebitAmount: amount,
			}
		}
		if amount := fallbackWalletDebitAmount(model.ChatOutputBillingAmount, model.BillingAmount, model.ChatOutputRawRate, model.RawRate); amount > 0 {
			result["chat_output_tokens"] = pricingRuleRecord{
				MeterCode:         "chat_output_tokens",
				ChargeMode:        "wallet_only",
				UnitSize:          1000,
				WalletDebitAmount: amount,
			}
		}
		return result
	case "image":
		if category != "image" {
			return nil
		}
		if amount := fallbackWalletDebitAmount(model.BillingAmount, model.RawRate); amount > 0 {
			rule := pricingRuleRecord{
				MeterCode:         "image_generations",
				ChargeMode:        "quota_first_wallet_fallback",
				UnitSize:          1,
				WalletDebitAmount: amount,
			}
			quotaMeterCode := "image_generation_quota"
			rule.QuotaMeterCode = &quotaMeterCode
			return map[string]pricingRuleRecord{
				"image_generations": rule,
			}
		}
	case "video":
		if category != "video" {
			return nil
		}
		if amount := fallbackWalletDebitAmount(model.BillingAmount, model.RawRate); amount > 0 {
			rule := pricingRuleRecord{
				MeterCode:         "video_generations",
				UnitSize:          1,
				WalletDebitAmount: amount,
			}
			if strings.EqualFold(strings.TrimSpace(model.BillingMode), "per_second") {
				rule.ChargeMode = "wallet_only"
				rule.QuantityMetaKey = "durationSeconds"
			} else {
				rule.ChargeMode = "quota_first_wallet_fallback"
				quotaMeterCode := "video_generation_quota"
				rule.QuotaMeterCode = &quotaMeterCode
			}
			return map[string]pricingRuleRecord{
				"video_generations": rule,
			}
		}
	}

	return nil
}

// 处理回退钱包DebitAmount相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func fallbackWalletDebitAmount(candidates ...*float64) int64 {
	for _, candidate := range candidates {
		if candidate == nil || *candidate <= 0 {
			continue
		}
		return int64(math.Ceil(*candidate))
	}
	return 0
}

// 确保钱包Lock事务已满足执行前提，必要时补齐缺失状态或配置。
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

// 加载额度账号用量事务，供存储层继续处理当前业务状态。
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
		SELECT id, meter_code, remaining_total, expires_at, recharge_order_id, distribution_commission_item_id, release_unit_credits
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
		if scanErr := rows.Scan(&item.ID, &item.MeterCode, &item.RemainingTotal, &item.ExpiresAt, &item.RechargeOrderID, &item.DistributionCommissionItemID, &item.ReleaseUnitCredits); scanErr != nil {
			return nil, scanErr
		}
		result[item.MeterCode] = append(result[item.MeterCode], item)
	}
	return result, rows.Err()
}

// 加载钱包Lots用量事务，供存储层继续处理当前业务状态。
func loadWalletLotsForUsageTx(ctx context.Context, tx pgx.Tx, userID string) ([]*walletLotRecord, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, recharge_order_id, distribution_commission_item_id, remaining_credits, consumed_credits, release_unit_credits
		FROM billing_wallet_lots
		WHERE user_id = $1
		  AND status = 'active'
		  AND remaining_credits > 0
		ORDER BY created_at ASC
		FOR UPDATE
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*walletLotRecord, 0)
	for rows.Next() {
		item := &walletLotRecord{}
		if scanErr := rows.Scan(&item.ID, &item.RechargeOrderID, &item.DistributionCommissionItemID, &item.RemainingCredits, &item.ConsumedCredits, &item.ReleaseUnitCredits); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// 处理plan用量Charge相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func planUsageCharge(metric ApplyUsageMetricInput, rule pricingRuleRecord, walletBalance int64, quotaAccounts map[string][]*quotaAccountRecord) (UsageBillingDetail, walletLedgerPlan, []quotaLedgerPlan, bool) {
	quantity := metric.Quantity
	if override := metricQuantityFromMetadata(metric.Metadata, rule.QuantityMetaKey); override > 0 {
		quantity = override
	}
	detail := UsageBillingDetail{
		MeterCode:     strings.TrimSpace(metric.MeterCode),
		Quantity:      quantity,
		PricingRuleID: strings.TrimSpace(rule.ID),
		ChargeMode:    strings.TrimSpace(rule.ChargeMode),
		BillStatus:    "billed",
	}
	detail.SupportsFailureRefund = supportsFailureRefundForUsage(rule, detail.MeterCode)

	if detail.MeterCode == "" || detail.ChargeMode == "" {
		detail.BillStatus = "failed"
		detail.BillMessage = "pricing rule not found"
		return detail, walletLedgerPlan{}, nil, false
	}

	unitSize := rule.UnitSize
	if unitSize <= 0 {
		unitSize = 1
	}
	detail.Units = ceilDiv(detail.Quantity, unitSize)

	walletPlan := walletLedgerPlan{
		meterCode:   detail.MeterCode,
		quantity:    detail.Quantity,
		unit:        "credit",
		description: fmt.Sprintf("AI %s 计费", detail.MeterCode),
	}
	quotaPlans := make([]quotaLedgerPlan, 0)

	switch strings.TrimSpace(rule.ChargeMode) {
	case "wallet_only":
		detail.DebitCredits = detail.Units * rule.WalletDebitAmount
		detail.DistributionReleaseCredits = detail.DebitCredits
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
					accountID:                    account.ID,
					meterCode:                    quotaMeterCode,
					amountDelta:                  -used,
					description:                  fmt.Sprintf("AI %s 套餐抵扣", detail.MeterCode),
					rechargeOrderID:              account.RechargeOrderID,
					distributionCommissionItemID: account.DistributionCommissionItemID,
					releaseUnitCredits:           maxInt64(account.ReleaseUnitCredits, 0),
					payload: mustJSONMap(map[string]any{
						"meterCode":          metric.MeterCode,
						"quantity":           detail.Quantity,
						"units":              detail.Units,
						"quotaUsed":          used,
						"chargeMode":         rule.ChargeMode,
						"releaseUnitCredits": maxInt64(account.ReleaseUnitCredits, 0),
						"creditValue":        used * maxInt64(account.ReleaseUnitCredits, 0),
					}),
				})
				detail.DistributionReleaseCredits += used * maxInt64(account.ReleaseUnitCredits, 0)
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
		detail.DistributionReleaseCredits += detail.DebitCredits
	default:
		detail.BillStatus = "failed"
		detail.BillMessage = "unsupported charge mode"
		return detail, walletLedgerPlan{}, nil, false
	}

	return detail, walletPlan, quotaPlans, true
}

// 应用额度台账Plan事务，把外部输入转换为当前链路的最终状态变更。
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

// 应用钱包台账Plan事务，把外部输入转换为当前链路的最终状态变更。
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

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) applyWalletLotConsumptionsTx(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	sourceType string,
	sourceID string,
	sourceSnapshot []byte,
	plan walletLedgerPlan,
	walletLedgerID string,
	walletLots []*walletLotRecord,
) error {
	remainingCredits := plan.debitCredits
	if remainingCredits <= 0 || len(walletLots) == 0 {
		return nil
	}

	for _, lot := range walletLots {
		if remainingCredits <= 0 {
			break
		}
		if lot == nil || lot.RemainingCredits <= 0 {
			continue
		}

		usedCredits := minInt64(lot.RemainingCredits, remainingCredits)
		nextRemaining := lot.RemainingCredits - usedCredits
		nextConsumed := lot.ConsumedCredits + usedCredits
		status := "active"
		if nextRemaining <= 0 {
			nextRemaining = 0
			status = "consumed"
		}

		if _, err := tx.Exec(ctx, `
			UPDATE billing_wallet_lots
			SET consumed_credits = $2,
			    remaining_credits = $3,
			    status = $4,
			    updated_at = NOW()
			WHERE id = $1
		`, lot.ID, nextConsumed, nextRemaining, status); err != nil {
			return err
		}

		consumptionMetadata := mustJSONMap(map[string]any{
			"meterCode":      plan.meterCode,
			"debitedCredits": usedCredits,
			"quantity":       plan.quantity,
			"description":    plan.description,
		})
		if _, err := tx.Exec(ctx, `
			INSERT INTO billing_wallet_lot_consumptions (
				id, wallet_lot_id, user_id, source_type, source_id, meter_code, debited_credits, wallet_ledger_id, metadata
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, uuid.NewString(), lot.ID, userID, strings.TrimSpace(sourceType), nullableString(strings.TrimSpace(sourceID)), nullableString(strings.TrimSpace(plan.meterCode)), usedCredits, nullableString(strings.TrimSpace(walletLedgerID)), consumptionMetadata); err != nil {
			return err
		}

		if lot.DistributionCommissionItemID != nil && strings.TrimSpace(*lot.DistributionCommissionItemID) != "" {
			if err := s.applyDistributionCommissionReleaseTx(ctx, tx, distributionCommissionReleaseInput{
				CommissionItemID:     strings.TrimSpace(*lot.DistributionCommissionItemID),
				SourceType:           sourceType,
				SourceID:             sourceID,
				SourceSnapshot:       sourceSnapshot,
				WalletLotID:          &lot.ID,
				WalletLedgerID:       nullableString(strings.TrimSpace(walletLedgerID)),
				ConsumedCreditsDelta: usedCredits * maxInt64(lot.ReleaseUnitCredits, 1),
				Metadata:             consumptionMetadata,
			}); err != nil {
				return err
			}
		}

		lot.RemainingCredits = nextRemaining
		lot.ConsumedCredits = nextConsumed
		remainingCredits -= usedCredits
	}

	return nil
}

// 处理insertBilled用量事件事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func insertBilledUsageEventsTx(ctx context.Context, tx pgx.Tx, input ApplyUsageBillingInput, details []UsageBillingDetail, refs usageLedgerRefs) error {
	for _, detail := range details {
		payload := map[string]any{
			"debitedCredits":             detail.DebitCredits,
			"distributionReleaseCredits": detail.DistributionReleaseCredits,
			"quantity":                   detail.Quantity,
			"units":                      detail.Units,
			"quotaUsed":                  detail.QuotaUsed,
			"supportsFailureRefund":      detail.SupportsFailureRefund,
			"chargeMode":                 detail.ChargeMode,
			"pricingRuleId":              detail.PricingRuleID,
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

// 处理insertFailed用量事件事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func insertFailedUsageEventsTx(ctx context.Context, tx pgx.Tx, input ApplyUsageBillingInput, details []UsageBillingDetail) error {
	for _, detail := range details {
		payload := map[string]any{
			"debitedCredits":             detail.DebitCredits,
			"distributionReleaseCredits": detail.DistributionReleaseCredits,
			"quantity":                   detail.Quantity,
			"units":                      detail.Units,
			"quotaUsed":                  detail.QuotaUsed,
			"supportsFailureRefund":      detail.SupportsFailureRefund,
			"chargeMode":                 detail.ChargeMode,
			"pricingRuleId":              detail.PricingRuleID,
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

// 处理ceilDiv相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func ceilDiv(value int64, divisor int64) int64 {
	if divisor <= 0 {
		return value
	}
	return (value + divisor - 1) / divisor
}

// 处理minInt64相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// 处理absInt64相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

// 处理maxInt64相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// 处理mustJSON映射相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理nullableString相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func nullableString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

// 处理追加Meter参考相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func appendMeterReference(target map[string][]string, meterCode string, value string) {
	meterCode = strings.TrimSpace(meterCode)
	value = strings.TrimSpace(value)
	if meterCode == "" || value == "" {
		return
	}
	target[meterCode] = append(target[meterCode], value)
}

// 处理首个StringPtr相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstStringPtr(values []string) *string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return &trimmed
		}
	}
	return nil
}

// 处理metricMetadataMeter相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理metricQuantityMetadata相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func metricQuantityFromMetadata(raw []byte, key string) int64 {
	key = strings.TrimSpace(key)
	if key == "" || len(raw) == 0 {
		return 0
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0
	}
	return usageQuantityValue(payload[key])
}

// 处理用量Quantity值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func usageQuantityValue(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return maxInt64(typed, 0)
	case int:
		return maxInt64(int64(typed), 0)
	case float64:
		return maxInt64(int64(math.Round(typed)), 0)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return maxInt64(parsed, 0)
		}
	case string:
		if parsed, err := json.Number(strings.TrimSpace(typed)).Int64(); err == nil {
			return maxInt64(parsed, 0)
		}
	}
	return 0
}

// 根据用量计算supportsFailureRefund，供存储层链路复用关键派生结果。
func supportsFailureRefundForUsage(rule pricingRuleRecord, meterCode string) bool {
	meterCode = strings.TrimSpace(strings.ToLower(meterCode))
	if meterCode == "" {
		return false
	}
	if strings.HasPrefix(meterCode, "chat_") {
		return false
	}
	if strings.TrimSpace(rule.QuantityMetaKey) != "" {
		return false
	}
	return true
}
