package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"omnidrive_cloud/internal/domain"
)

type BillingActivityListFilter struct {
	Query      string
	Kind       string
	Status     string
	EntryType  string
	SourceType string
	JobType    string
	Channel    string
	ModelName  string
	Page       int
	PageSize   int
}

type AdminBillingActivityListFilter struct {
	Query      string
	Kind       string
	Status     string
	EntryType  string
	SourceType string
	JobType    string
	Channel    string
	ModelName  string
	AdminPageFilter
}

func scanBillingActivity(scan scanFn) (*domain.BillingActivity, error) {
	var item domain.BillingActivity
	var resultAt *time.Time
	var detail *string
	var status *string
	var entryType *string
	var channel *string
	var sourceType *string
	var meterCode *string
	var meterName *string
	var modelName *string
	var modelAlias *string
	var jobType *string
	var reference *string
	var referenceType *string
	var referenceID *string
	var sourceID *string
	var amountCents *int64
	var creditDelta *int64
	var usageQuantity *int64
	var debitedCredits *int64
	var creditAmount *int64
	var bonusCreditAmount *int64
	var billMessage *string
	var payload []byte

	if err := scan(
		&item.ID,
		&item.Kind,
		&item.UserID,
		&item.OccurredAt,
		&resultAt,
		&item.Title,
		&detail,
		&status,
		&entryType,
		&channel,
		&sourceType,
		&meterCode,
		&meterName,
		&modelName,
		&modelAlias,
		&jobType,
		&reference,
		&referenceType,
		&referenceID,
		&sourceID,
		&amountCents,
		&creditDelta,
		&usageQuantity,
		&debitedCredits,
		&creditAmount,
		&bonusCreditAmount,
		&billMessage,
		&payload,
	); err != nil {
		return nil, err
	}

	item.ResultAt = resultAt
	item.Detail = detail
	item.Status = status
	item.EntryType = entryType
	item.Channel = channel
	item.SourceType = sourceType
	item.MeterCode = meterCode
	item.MeterName = meterName
	item.ModelName = modelName
	item.ModelAlias = modelAlias
	item.JobType = jobType
	item.Reference = reference
	item.ReferenceType = referenceType
	item.ReferenceID = referenceID
	item.SourceID = sourceID
	item.AmountCents = amountCents
	item.CreditDelta = creditDelta
	item.UsageQuantity = usageQuantity
	item.DebitedCredits = debitedCredits
	item.CreditAmount = creditAmount
	item.BonusCreditAmount = bonusCreditAmount
	item.BillMessage = billMessage
	item.Payload = bytesOrNil(payload)
	return &item, nil
}

func appendBillingActivityFilters(
	whereParts []string,
	args []any,
	argIndex int,
	query string,
	kind string,
	status string,
	entryType string,
	sourceType string,
	jobType string,
	channel string,
	modelName string,
	includeUser bool,
) ([]string, []any, int) {
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		if includeUser {
			whereParts = append(whereParts, fmt.Sprintf(`(
				COALESCE(title, '') ILIKE $%d OR
				COALESCE(detail, '') ILIKE $%d OR
				COALESCE(reference, '') ILIKE $%d OR
				COALESCE(model_name, '') ILIKE $%d OR
				COALESCE(model_alias, '') ILIKE $%d OR
				COALESCE(job_type, '') ILIKE $%d OR
				COALESCE(status, '') ILIKE $%d OR
				COALESCE(user_email, '') ILIKE $%d OR
				COALESCE(user_name, '') ILIKE $%d
			)`, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex))
		} else {
			whereParts = append(whereParts, fmt.Sprintf(`(
				COALESCE(title, '') ILIKE $%d OR
				COALESCE(detail, '') ILIKE $%d OR
				COALESCE(reference, '') ILIKE $%d OR
				COALESCE(model_name, '') ILIKE $%d OR
				COALESCE(model_alias, '') ILIKE $%d OR
				COALESCE(job_type, '') ILIKE $%d OR
				COALESCE(status, '') ILIKE $%d
			)`, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex))
		}
		args = append(args, ilikePattern(trimmed))
		argIndex++
	}
	if trimmed := strings.TrimSpace(kind); trimmed != "" {
		whereParts = append(whereParts, fmt.Sprintf("kind = $%d", argIndex))
		args = append(args, trimmed)
		argIndex++
	}
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		whereParts, args, argIndex = appendEquivalentStatusFilter(whereParts, args, argIndex, "status", trimmed)
	}
	if trimmed := strings.TrimSpace(entryType); trimmed != "" {
		whereParts = append(whereParts, fmt.Sprintf("entry_type = $%d", argIndex))
		args = append(args, trimmed)
		argIndex++
	}
	if trimmed := strings.TrimSpace(sourceType); trimmed != "" {
		whereParts = append(whereParts, fmt.Sprintf("source_type = $%d", argIndex))
		args = append(args, trimmed)
		argIndex++
	}
	if trimmed := strings.TrimSpace(jobType); trimmed != "" {
		whereParts = append(whereParts, fmt.Sprintf("job_type = $%d", argIndex))
		args = append(args, trimmed)
		argIndex++
	}
	if trimmed := strings.TrimSpace(channel); trimmed != "" {
		whereParts = append(whereParts, fmt.Sprintf("channel = $%d", argIndex))
		args = append(args, trimmed)
		argIndex++
	}
	if trimmed := strings.TrimSpace(modelName); trimmed != "" {
		whereParts = append(whereParts, fmt.Sprintf("(model_name = $%d OR model_alias = $%d)", argIndex, argIndex))
		args = append(args, trimmed)
		argIndex++
	}
	return whereParts, args, argIndex
}

func appendEquivalentStatusFilter(whereParts []string, args []any, argIndex int, column string, status string) ([]string, []any, int) {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return whereParts, args, argIndex
	}
	if trimmed == "returned" || trimmed == "refunded" {
		whereParts = append(whereParts, fmt.Sprintf("(%s = $%d OR %s = $%d)", column, argIndex, column, argIndex+1))
		args = append(args, "returned", "refunded")
		return whereParts, args, argIndex + 2
	}
	whereParts = append(whereParts, fmt.Sprintf("%s = $%d", column, argIndex))
	args = append(args, trimmed)
	return whereParts, args, argIndex + 1
}

const billingActivitiesUserBaseQuery = `
	WITH activities AS (
		SELECT
			ro.id,
			'recharge_order'::TEXT AS kind,
			ro.user_id,
			ro.created_at AS occurred_at,
			COALESCE(ro.paid_at, ro.closed_at, ro.updated_at) AS result_at,
			ro.subject AS title,
			COALESCE(ro.body, ro.subject) AS detail,
			ro.status,
			NULL::TEXT AS entry_type,
			ro.channel,
			NULL::TEXT AS source_type,
			NULL::TEXT AS meter_code,
			NULL::TEXT AS meter_name,
			NULL::TEXT AS model_name,
			NULL::TEXT AS model_alias,
			NULL::TEXT AS job_type,
			ro.order_no AS reference,
			NULL::TEXT AS reference_type,
			NULL::TEXT AS reference_id,
			NULL::TEXT AS source_id,
			ro.amount_cents,
			NULL::BIGINT AS credit_delta,
			NULL::BIGINT AS usage_quantity,
			NULL::BIGINT AS debited_credits,
			ro.credit_amount,
			ro.manual_bonus_credit_amount AS bonus_credit_amount,
			NULL::TEXT AS bill_message,
			NULL::JSONB AS payload
		FROM recharge_orders ro
		WHERE ro.user_id = $1

		UNION ALL

		SELECT
			wl.id,
			'wallet_ledger'::TEXT AS kind,
			wl.user_id,
			wl.created_at AS occurred_at,
			wl.created_at AS result_at,
			COALESCE(wl.description, wl.entry_type, '钱包账变') AS title,
			COALESCE(wl.description, wl.reference_type, '钱包账变') AS detail,
			NULL::TEXT AS status,
			wl.entry_type,
			NULL::TEXT AS channel,
			NULL::TEXT AS source_type,
			wl.meter_code,
			NULL::TEXT AS meter_name,
			NULL::TEXT AS model_name,
			NULL::TEXT AS model_alias,
			NULL::TEXT AS job_type,
			COALESCE(wl.reference_id, wl.id) AS reference,
			wl.reference_type,
			wl.reference_id,
			NULL::TEXT AS source_id,
			NULL::BIGINT AS amount_cents,
			wl.amount_delta AS credit_delta,
			wl.quantity AS usage_quantity,
			NULL::BIGINT AS debited_credits,
			NULL::BIGINT AS credit_amount,
			NULL::BIGINT AS bonus_credit_amount,
			NULL::TEXT AS bill_message,
			wl.metadata::JSONB AS payload
		FROM wallet_ledgers wl
		WHERE wl.user_id = $1

		UNION ALL

		SELECT
			e.id,
			'usage_event'::TEXT AS kind,
			e.user_id,
			e.created_at AS occurred_at,
			e.updated_at AS result_at,
			COALESCE(e.job_type, e.source_type, e.meter_code) AS title,
			COALESCE(e.bill_message, m.name, e.meter_code) AS detail,
				CASE WHEN e.bill_status = 'refunded' THEN 'returned' ELSE e.bill_status END AS status,
			NULL::TEXT AS entry_type,
			NULL::TEXT AS channel,
			e.source_type,
			e.meter_code,
			m.name AS meter_name,
			e.model_name,
			COALESCE(am.model_alias, e.model_name) AS model_alias,
			e.job_type,
			COALESCE(e.source_id, e.id) AS reference,
			NULL::TEXT AS reference_type,
			NULL::TEXT AS reference_id,
			e.source_id,
			NULL::BIGINT AS amount_cents,
			NULL::BIGINT AS credit_delta,
			e.usage_quantity,
			CASE
				WHEN e.payload IS NOT NULL AND e.payload ? 'debitedCredits'
					THEN NULLIF(e.payload ->> 'debitedCredits', '')::BIGINT
				ELSE NULL::BIGINT
			END AS debited_credits,
			NULL::BIGINT AS credit_amount,
			NULL::BIGINT AS bonus_credit_amount,
			e.bill_message,
			e.payload::JSONB AS payload
		FROM billing_usage_events e
		LEFT JOIN billing_meters m ON m.code = e.meter_code
		LEFT JOIN ai_models am ON am.model_name = e.model_name
		WHERE e.user_id = $1
	)
`

const billingActivitiesAdminBaseQuery = `
	WITH activities AS (
		SELECT
			ro.id,
			'recharge_order'::TEXT AS kind,
			ro.user_id,
			COALESCE(u.email, '') AS user_email,
			COALESCE(u.name, '') AS user_name,
			ro.created_at AS occurred_at,
			COALESCE(ro.paid_at, ro.closed_at, ro.updated_at) AS result_at,
			ro.subject AS title,
			COALESCE(ro.body, ro.subject) AS detail,
			ro.status,
			NULL::TEXT AS entry_type,
			ro.channel,
			NULL::TEXT AS source_type,
			NULL::TEXT AS meter_code,
			NULL::TEXT AS meter_name,
			NULL::TEXT AS model_name,
			NULL::TEXT AS model_alias,
			NULL::TEXT AS job_type,
			ro.order_no AS reference,
			NULL::TEXT AS reference_type,
			NULL::TEXT AS reference_id,
			NULL::TEXT AS source_id,
			ro.amount_cents,
			NULL::BIGINT AS credit_delta,
			NULL::BIGINT AS usage_quantity,
			NULL::BIGINT AS debited_credits,
			ro.credit_amount,
			ro.manual_bonus_credit_amount AS bonus_credit_amount,
			NULL::TEXT AS bill_message,
			NULL::JSONB AS payload
		FROM recharge_orders ro
		LEFT JOIN users u ON u.id = ro.user_id

		UNION ALL

		SELECT
			wl.id,
			'wallet_ledger'::TEXT AS kind,
			wl.user_id,
			COALESCE(u.email, '') AS user_email,
			COALESCE(u.name, '') AS user_name,
			wl.created_at AS occurred_at,
			wl.created_at AS result_at,
			COALESCE(wl.description, wl.entry_type, '钱包账变') AS title,
			COALESCE(wl.description, wl.reference_type, '钱包账变') AS detail,
			NULL::TEXT AS status,
			wl.entry_type,
			NULL::TEXT AS channel,
			NULL::TEXT AS source_type,
			wl.meter_code,
			NULL::TEXT AS meter_name,
			NULL::TEXT AS model_name,
			NULL::TEXT AS model_alias,
			NULL::TEXT AS job_type,
			COALESCE(wl.reference_id, wl.id) AS reference,
			wl.reference_type,
			wl.reference_id,
			NULL::TEXT AS source_id,
			NULL::BIGINT AS amount_cents,
			wl.amount_delta AS credit_delta,
			wl.quantity AS usage_quantity,
			NULL::BIGINT AS debited_credits,
			NULL::BIGINT AS credit_amount,
			NULL::BIGINT AS bonus_credit_amount,
			NULL::TEXT AS bill_message,
			wl.metadata::JSONB AS payload
		FROM wallet_ledgers wl
		LEFT JOIN users u ON u.id = wl.user_id

		UNION ALL

		SELECT
			e.id,
			'usage_event'::TEXT AS kind,
			e.user_id,
			COALESCE(u.email, '') AS user_email,
			COALESCE(u.name, '') AS user_name,
			e.created_at AS occurred_at,
			e.updated_at AS result_at,
			COALESCE(e.job_type, e.source_type, e.meter_code) AS title,
			COALESCE(e.bill_message, m.name, e.meter_code) AS detail,
				CASE WHEN e.bill_status = 'refunded' THEN 'returned' ELSE e.bill_status END AS status,
			NULL::TEXT AS entry_type,
			NULL::TEXT AS channel,
			e.source_type,
			e.meter_code,
			m.name AS meter_name,
			e.model_name,
			COALESCE(am.model_alias, e.model_name) AS model_alias,
			e.job_type,
			COALESCE(e.source_id, e.id) AS reference,
			NULL::TEXT AS reference_type,
			NULL::TEXT AS reference_id,
			e.source_id,
			NULL::BIGINT AS amount_cents,
			NULL::BIGINT AS credit_delta,
			e.usage_quantity,
			CASE
				WHEN e.payload IS NOT NULL AND e.payload ? 'debitedCredits'
					THEN NULLIF(e.payload ->> 'debitedCredits', '')::BIGINT
				ELSE NULL::BIGINT
			END AS debited_credits,
			NULL::BIGINT AS credit_amount,
			NULL::BIGINT AS bonus_credit_amount,
			e.bill_message,
			e.payload::JSONB AS payload
		FROM billing_usage_events e
		LEFT JOIN users u ON u.id = e.user_id
		LEFT JOIN billing_meters m ON m.code = e.meter_code
		LEFT JOIN ai_models am ON am.model_name = e.model_name
	)
`

func (s *Store) ListBillingActivitiesByUser(ctx context.Context, userID string, filter BillingActivityListFilter) ([]domain.BillingActivity, int64, domain.BillingActivityListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{userID}
	argIndex := 2
	whereParts, args, argIndex = appendBillingActivityFilters(
		whereParts,
		args,
		argIndex,
		filter.Query,
		filter.Kind,
		filter.Status,
		filter.EntryType,
		filter.SourceType,
		filter.JobType,
		filter.Channel,
		filter.ModelName,
		false,
	)
	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var summary domain.BillingActivityListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		%s
		SELECT
			COUNT(*)::BIGINT,
			COUNT(*) FILTER (WHERE kind = 'recharge_order')::BIGINT,
			COUNT(*) FILTER (WHERE kind = 'wallet_ledger')::BIGINT,
			COUNT(*) FILTER (WHERE kind = 'usage_event')::BIGINT,
			COALESCE(SUM(amount_cents) FILTER (WHERE kind = 'recharge_order'), 0)::BIGINT,
			COALESCE(SUM(credit_delta) FILTER (WHERE kind = 'wallet_ledger' AND credit_delta > 0), 0)::BIGINT,
			COALESCE(SUM(ABS(credit_delta)) FILTER (WHERE kind = 'wallet_ledger' AND credit_delta < 0), 0)::BIGINT,
			COALESCE(SUM(debited_credits) FILTER (WHERE kind = 'usage_event' AND status = 'billed'), 0)::BIGINT
		FROM activities
		%s
	`, billingActivitiesUserBaseQuery, whereClause), args...).Scan(
		&summary.TotalActivityCount,
		&summary.OrderCount,
		&summary.WalletLedgerCount,
		&summary.UsageEventCount,
		&summary.TotalRechargeAmountCents,
		&summary.TotalCreditIn,
		&summary.TotalCreditOut,
		&summary.TotalDebitedCredits,
	); err != nil {
		return nil, 0, domain.BillingActivityListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		%s
		SELECT
			id, kind, user_id, occurred_at, result_at, title, detail, status, entry_type, channel,
			source_type, meter_code, meter_name, model_name, model_alias, job_type, reference, reference_type,
			reference_id, source_id, amount_cents, credit_delta, usage_quantity, debited_credits,
			credit_amount, bonus_credit_amount, bill_message, payload
		FROM activities
		%s
		ORDER BY occurred_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, billingActivitiesUserBaseQuery, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.BillingActivityListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.BillingActivity, 0)
	for rows.Next() {
		item, scanErr := scanBillingActivity(rows.Scan)
		if scanErr != nil {
			return nil, 0, domain.BillingActivityListSummary{}, scanErr
		}
		items = append(items, *item)
	}
	return items, summary.TotalActivityCount, summary, rows.Err()
}

func (s *Store) ListAdminBillingActivities(ctx context.Context, filter AdminBillingActivityListFilter) ([]domain.AdminBillingActivityRow, int64, domain.AdminBillingActivityListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1
	whereParts, args, argIndex = appendBillingActivityFilters(
		whereParts,
		args,
		argIndex,
		filter.Query,
		filter.Kind,
		filter.Status,
		filter.EntryType,
		filter.SourceType,
		filter.JobType,
		filter.Channel,
		filter.ModelName,
		true,
	)
	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var summary domain.AdminBillingActivityListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		%s
		SELECT
			COUNT(*)::BIGINT,
			COUNT(*) FILTER (WHERE kind = 'recharge_order')::BIGINT,
			COUNT(*) FILTER (WHERE kind = 'wallet_ledger')::BIGINT,
			COUNT(*) FILTER (WHERE kind = 'usage_event')::BIGINT,
			COALESCE(SUM(amount_cents) FILTER (WHERE kind = 'recharge_order'), 0)::BIGINT,
			COALESCE(SUM(credit_delta) FILTER (WHERE kind = 'wallet_ledger' AND credit_delta > 0), 0)::BIGINT,
			COALESCE(SUM(ABS(credit_delta)) FILTER (WHERE kind = 'wallet_ledger' AND credit_delta < 0), 0)::BIGINT,
			COALESCE(SUM(debited_credits) FILTER (WHERE kind = 'usage_event' AND status = 'billed'), 0)::BIGINT
		FROM activities
		%s
	`, billingActivitiesAdminBaseQuery, whereClause), args...).Scan(
		&summary.TotalActivityCount,
		&summary.OrderCount,
		&summary.WalletLedgerCount,
		&summary.UsageEventCount,
		&summary.TotalRechargeAmountCents,
		&summary.TotalCreditIn,
		&summary.TotalCreditOut,
		&summary.TotalDebitedCredits,
	); err != nil {
		return nil, 0, domain.AdminBillingActivityListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		%s
		SELECT
			id, kind, user_id, occurred_at, result_at, title, detail, status, entry_type, channel,
			source_type, meter_code, meter_name, model_name, model_alias, job_type, reference, reference_type,
			reference_id, source_id, amount_cents, credit_delta, usage_quantity, debited_credits,
			credit_amount, bonus_credit_amount, bill_message, payload,
			user_id, user_email, user_name
		FROM activities
		%s
		ORDER BY occurred_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, billingActivitiesAdminBaseQuery, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminBillingActivityListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminBillingActivityRow, 0)
	for rows.Next() {
		var item domain.AdminBillingActivityRow
		activity, scanErr := scanBillingActivity(func(dest ...any) error {
			dest = append(dest, &item.User.ID, &item.User.Email, &item.User.Name)
			return rows.Scan(dest...)
		})
		if scanErr != nil {
			return nil, 0, domain.AdminBillingActivityListSummary{}, scanErr
		}
		item.Activity = *activity
		items = append(items, item)
	}
	return items, summary.TotalActivityCount, summary, rows.Err()
}
