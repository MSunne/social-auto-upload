package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ExpireDueQuotaAccountsResult struct {
	ExpiredCount               int   `json:"expiredCount"`
	ClearedQuotaTotal          int64 `json:"clearedQuotaTotal"`
	DistributionReleaseCredits int64 `json:"distributionReleaseCredits"`
}

type expiringQuotaAccountRecord struct {
	ID             string
	UserID         string
	MeterCode      string
	RemainingTotal int64
	ExpiresAt      *time.Time
	SourceType     *string
	SourceID       *string
}

func (s *Store) ExpireDueQuotaAccounts(ctx context.Context, limit int) (*ExpireDueQuotaAccountsResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, user_id, meter_code, remaining_total, expires_at, source_type, source_id
		FROM billing_quota_accounts
		WHERE status = 'active'
		  AND remaining_total > 0
		  AND expires_at IS NOT NULL
		  AND expires_at <= NOW()
		ORDER BY expires_at ASC, created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]expiringQuotaAccountRecord, 0, limit)
	quotaMeterCodes := make(map[string]struct{})
	for rows.Next() {
		var item expiringQuotaAccountRecord
		if scanErr := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.MeterCode,
			&item.RemainingTotal,
			&item.ExpiresAt,
			&item.SourceType,
			&item.SourceID,
		); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
		if item.RemainingTotal > 0 {
			quotaMeterCodes[item.MeterCode] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &ExpireDueQuotaAccountsResult{}, nil
	}

	quotaUnitCredits, err := loadQuotaUnitCreditMapTx(ctx, tx, quotaMeterCodes)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	result := &ExpireDueQuotaAccountsResult{}
	for _, item := range items {
		if item.RemainingTotal <= 0 {
			continue
		}

		if _, err := tx.Exec(ctx, `
			UPDATE billing_quota_accounts
			SET remaining_total = 0,
			    status = 'expired',
			    updated_at = NOW()
			WHERE id = $1
		`, item.ID); err != nil {
			return nil, err
		}

		releaseCredits := item.RemainingTotal * quotaUnitCredits[item.MeterCode]
		payload := mustJSONBytes(map[string]any{
			"expiredAt":                  now.Format(time.RFC3339),
			"meterCode":                  item.MeterCode,
			"clearedQuota":               item.RemainingTotal,
			"distributionReleaseCredits": releaseCredits,
			"sourceType":                 valueOrEmpty(item.SourceType),
			"sourceId":                   valueOrEmpty(item.SourceID),
		})
		if _, err := tx.Exec(ctx, `
			INSERT INTO billing_quota_ledgers (
				id, quota_account_id, user_id, meter_code, amount_delta, remaining_after,
				description, reference_type, reference_id, payload
			)
			VALUES ($1, $2, $3, $4, $5, 0, $6, 'quota_expiration', $2, $7)
		`, uuid.NewString(), item.ID, item.UserID, item.MeterCode, -item.RemainingTotal, stringPtr("套餐额度到期清空"), payload); err != nil {
			return nil, err
		}

		if releaseCredits > 0 {
			if err := s.releaseDistributionCommissionForUsageTx(ctx, tx, item.UserID, "quota_expiration", item.ID, releaseCredits); err != nil {
				return nil, err
			}
		}

		result.ExpiredCount++
		result.ClearedQuotaTotal += item.RemainingTotal
		result.DistributionReleaseCredits += releaseCredits
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}
