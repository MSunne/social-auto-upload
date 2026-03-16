package store

import (
	"context"
	"fmt"
	"strings"

	"omnidrive_cloud/internal/domain"
)

type BillingUsageEventListFilter struct {
	SourceType string
	SourceID   string
	MeterCode  string
	BillStatus string
	JobType    string
	ModelName  string
	Limit      int
}

type AdminBillingUsageEventListFilter struct {
	Query      string
	SourceType string
	MeterCode  string
	BillStatus string
	JobType    string
	ModelName  string
	AdminPageFilter
}

func scanBillingUsageEvent(scan scanFn) (*domain.BillingUsageEvent, error) {
	var item domain.BillingUsageEvent
	var sourceID *string
	var meterName *string
	var modelName *string
	var jobType *string
	var pricingRuleID *string
	var pricingRuleName *string
	var quotaAccountID *string
	var walletLedgerID *string
	var quotaLedgerID *string
	var billMessage *string
	var payload []byte

	if err := scan(
		&item.ID,
		&item.UserID,
		&item.SourceType,
		&sourceID,
		&item.MeterCode,
		&meterName,
		&modelName,
		&jobType,
		&item.UsageQuantity,
		&pricingRuleID,
		&pricingRuleName,
		&quotaAccountID,
		&walletLedgerID,
		&quotaLedgerID,
		&item.BillStatus,
		&billMessage,
		&payload,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}

	item.SourceID = sourceID
	item.MeterName = meterName
	item.ModelName = modelName
	item.JobType = jobType
	item.PricingRuleID = pricingRuleID
	item.PricingRuleName = pricingRuleName
	item.QuotaAccountID = quotaAccountID
	item.WalletLedgerID = walletLedgerID
	item.QuotaLedgerID = quotaLedgerID
	item.BillMessage = billMessage
	item.Payload = bytesOrNil(payload)
	return &item, nil
}

func (s *Store) ListBillingUsageEventsByUser(ctx context.Context, userID string, filter BillingUsageEventListFilter) ([]domain.BillingUsageEvent, error) {
	whereParts := []string{"e.user_id = $1"}
	args := []any{userID}
	argIndex := 2

	if sourceType := strings.TrimSpace(filter.SourceType); sourceType != "" {
		whereParts = append(whereParts, fmt.Sprintf("e.source_type = $%d", argIndex))
		args = append(args, sourceType)
		argIndex++
	}
	if sourceID := strings.TrimSpace(filter.SourceID); sourceID != "" {
		whereParts = append(whereParts, fmt.Sprintf("e.source_id = $%d", argIndex))
		args = append(args, sourceID)
		argIndex++
	}
	if meterCode := strings.TrimSpace(filter.MeterCode); meterCode != "" {
		whereParts = append(whereParts, fmt.Sprintf("e.meter_code = $%d", argIndex))
		args = append(args, meterCode)
		argIndex++
	}
	if billStatus := strings.TrimSpace(filter.BillStatus); billStatus != "" {
		whereParts = append(whereParts, fmt.Sprintf("e.bill_status = $%d", argIndex))
		args = append(args, billStatus)
		argIndex++
	}
	if jobType := strings.TrimSpace(filter.JobType); jobType != "" {
		whereParts = append(whereParts, fmt.Sprintf("e.job_type = $%d", argIndex))
		args = append(args, jobType)
		argIndex++
	}
	if modelName := strings.TrimSpace(filter.ModelName); modelName != "" {
		whereParts = append(whereParts, fmt.Sprintf("e.model_name = $%d", argIndex))
		args = append(args, modelName)
		argIndex++
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			e.id, e.user_id, e.source_type, e.source_id, e.meter_code, m.name, e.model_name, e.job_type,
			e.usage_quantity, e.pricing_rule_id, pr.name, e.quota_account_id, e.wallet_ledger_id,
			e.quota_ledger_id, e.bill_status, e.bill_message, e.payload, e.created_at, e.updated_at
		FROM billing_usage_events e
		LEFT JOIN billing_meters m ON m.code = e.meter_code
		LEFT JOIN billing_pricing_rules pr ON pr.id = e.pricing_rule_id
		WHERE %s
		ORDER BY e.created_at DESC
		LIMIT $%d
	`, strings.Join(whereParts, " AND "), argIndex), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.BillingUsageEvent, 0)
	for rows.Next() {
		item, scanErr := scanBillingUsageEvent(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) ListAdminBillingUsageEvents(ctx context.Context, filter AdminBillingUsageEventListFilter) ([]domain.AdminBillingUsageEventRow, int64, domain.AdminBillingUsageEventListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf(`(
			u.email ILIKE $%d OR
			u.name ILIKE $%d OR
			COALESCE(e.source_id, '') ILIKE $%d OR
			COALESCE(e.model_name, '') ILIKE $%d OR
			e.meter_code ILIKE $%d OR
			COALESCE(e.bill_message, '') ILIKE $%d
		)`, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}
	if sourceType := strings.TrimSpace(filter.SourceType); sourceType != "" {
		whereParts = append(whereParts, fmt.Sprintf("e.source_type = $%d", argIndex))
		args = append(args, sourceType)
		argIndex++
	}
	if meterCode := strings.TrimSpace(filter.MeterCode); meterCode != "" {
		whereParts = append(whereParts, fmt.Sprintf("e.meter_code = $%d", argIndex))
		args = append(args, meterCode)
		argIndex++
	}
	if billStatus := strings.TrimSpace(filter.BillStatus); billStatus != "" {
		whereParts = append(whereParts, fmt.Sprintf("e.bill_status = $%d", argIndex))
		args = append(args, billStatus)
		argIndex++
	}
	if jobType := strings.TrimSpace(filter.JobType); jobType != "" {
		whereParts = append(whereParts, fmt.Sprintf("e.job_type = $%d", argIndex))
		args = append(args, jobType)
		argIndex++
	}
	if modelName := strings.TrimSpace(filter.ModelName); modelName != "" {
		whereParts = append(whereParts, fmt.Sprintf("e.model_name = $%d", argIndex))
		args = append(args, modelName)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var summary domain.AdminBillingUsageEventListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*)::BIGINT,
			COUNT(*) FILTER (WHERE e.bill_status = 'billed')::BIGINT,
			COUNT(*) FILTER (WHERE e.bill_status = 'failed')::BIGINT,
			COALESCE(SUM(
				CASE
					WHEN e.payload IS NOT NULL AND e.payload ? 'debitedCredits'
						THEN NULLIF(e.payload ->> 'debitedCredits', '')::BIGINT
					ELSE 0
				END
			), 0)::BIGINT
		FROM billing_usage_events e
		INNER JOIN users u ON u.id = e.user_id
		%s
	`, whereClause), args...).Scan(
		&summary.TotalEventCount,
		&summary.BilledCount,
		&summary.FailedCount,
		&summary.TotalDebitedCredits,
	); err != nil {
		return nil, 0, domain.AdminBillingUsageEventListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			e.id, e.user_id, e.source_type, e.source_id, e.meter_code, m.name, e.model_name, e.job_type,
			e.usage_quantity, e.pricing_rule_id, pr.name, e.quota_account_id, e.wallet_ledger_id,
			e.quota_ledger_id, e.bill_status, e.bill_message, e.payload, e.created_at, e.updated_at,
			u.id, u.email, u.name
		FROM billing_usage_events e
		INNER JOIN users u ON u.id = e.user_id
		LEFT JOIN billing_meters m ON m.code = e.meter_code
		LEFT JOIN billing_pricing_rules pr ON pr.id = e.pricing_rule_id
		%s
		ORDER BY e.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminBillingUsageEventListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminBillingUsageEventRow, 0)
	for rows.Next() {
		var item domain.AdminBillingUsageEventRow
		event, scanErr := scanBillingUsageEvent(func(dest ...any) error {
			dest = append(dest, &item.User.ID, &item.User.Email, &item.User.Name)
			return rows.Scan(dest...)
		})
		if scanErr != nil {
			return nil, 0, domain.AdminBillingUsageEventListSummary{}, scanErr
		}
		item.Event = *event
		items = append(items, item)
	}
	return items, summary.TotalEventCount, summary, rows.Err()
}
