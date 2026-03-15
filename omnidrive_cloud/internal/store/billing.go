package store

import (
	"context"

	"omnidrive_cloud/internal/domain"
)

func (s *Store) ListBillingPackages(ctx context.Context) ([]domain.BillingPackage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, channel, price_cents, credit_amount, badge, description,
		       is_enabled, sort_order, created_at, updated_at
		FROM billing_packages
		WHERE is_enabled = TRUE
		ORDER BY sort_order ASC, price_cents ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.BillingPackage, 0)
	for rows.Next() {
		var item domain.BillingPackage
		var badge *string
		var description *string

		if scanErr := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Channel,
			&item.PriceCents,
			&item.CreditAmount,
			&badge,
			&description,
			&item.IsEnabled,
			&item.SortOrder,
			&item.CreatedAt,
			&item.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}

		item.Badge = badge
		item.Description = description
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListWalletLedgerByUser(ctx context.Context, userID string) ([]domain.WalletLedger, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, entry_type, amount_delta, balance_after, description,
		       reference_type, reference_id, created_at
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
		var description *string
		var referenceType *string
		var referenceID *string

		if scanErr := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.EntryType,
			&item.AmountDelta,
			&item.BalanceAfter,
			&description,
			&referenceType,
			&referenceID,
			&item.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}

		item.Description = description
		item.ReferenceType = referenceType
		item.ReferenceID = referenceID
		items = append(items, item)
	}
	return items, rows.Err()
}
