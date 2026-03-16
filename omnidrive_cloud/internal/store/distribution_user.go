package store

import (
	"context"
	"strconv"
	"strings"

	"omnidrive_cloud/internal/domain"
)

type CommissionListFilter struct {
	Status string
	Limit  int
}

func (s *Store) GetDistributionSummaryByPromoter(ctx context.Context, promoterUserID string) (*domain.DistributionSummary, error) {
	summary := &domain.DistributionSummary{}

	if err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(DISTINCT invitee_user_id)::BIGINT,
			COALESCE(SUM(GREATEST(amount_cents - released_amount_cents, 0)), 0)::BIGINT,
			COALESCE(SUM(GREATEST(released_amount_cents - settled_amount_cents, 0)), 0)::BIGINT,
			COALESCE(SUM(settled_amount_cents), 0)::BIGINT
		FROM distribution_commission_items
		WHERE promoter_user_id = $1
	`, promoterUserID).Scan(
		&summary.InviteeCount,
		&summary.PendingConsumeAmountCents,
		&summary.PendingSettlementAmountCents,
		&summary.SettledAmountCents,
	); err != nil {
		return nil, err
	}

	if err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'requested' THEN amount_cents ELSE 0 END), 0)::BIGINT,
			COALESCE(SUM(CASE WHEN status = 'approved' THEN amount_cents ELSE 0 END), 0)::BIGINT,
			COALESCE(SUM(CASE WHEN status = 'paid' THEN amount_cents ELSE 0 END), 0)::BIGINT
		FROM withdrawal_requests
		WHERE promoter_user_id = $1
	`, promoterUserID).Scan(
		&summary.RequestedWithdrawalAmountCents,
		&summary.ApprovedWithdrawalAmountCents,
		&summary.PaidWithdrawalAmountCents,
	); err != nil {
		return nil, err
	}

	availableAmountCents := summary.SettledAmountCents - summary.RequestedWithdrawalAmountCents - summary.ApprovedWithdrawalAmountCents - summary.PaidWithdrawalAmountCents
	if availableAmountCents < 0 {
		availableAmountCents = 0
	}
	summary.AvailableWithdrawalAmountCents = availableAmountCents
	return summary, nil
}

func (s *Store) ListCommissionItemsByPromoter(ctx context.Context, promoterUserID string, filter CommissionListFilter) ([]domain.CommissionItem, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	whereParts := []string{"c.promoter_user_id = $1"}
	args := []any{promoterUserID}
	argIndex := 2

	switch strings.TrimSpace(filter.Status) {
	case "pending_consume", "pending_settlement", "settled":
		whereParts = append(whereParts, "c.status = $2")
		args = append(args, strings.TrimSpace(filter.Status))
		argIndex++
	}

	args = append(args, limit)

	rows, err := s.pool.Query(ctx, `
		SELECT
			c.id,
			iu.id,
			iu.email,
			iu.name,
			c.status,
			c.commission_rate_basis_points,
			c.commission_base_amount_cents,
			c.amount_cents,
			c.released_amount_cents,
			c.settled_amount_cents,
			c.recharge_order_id,
			ro.order_no,
			c.created_at,
			c.released_at,
			c.settled_at
		FROM distribution_commission_items c
		INNER JOIN users iu ON iu.id = c.invitee_user_id
		LEFT JOIN recharge_orders ro ON ro.id = c.recharge_order_id
		WHERE `+strings.Join(whereParts, " AND ")+`
		ORDER BY c.created_at DESC
		LIMIT $`+strconv.Itoa(argIndex), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.CommissionItem, 0)
	for rows.Next() {
		var item domain.CommissionItem
		var basisPoints int
		if scanErr := rows.Scan(
			&item.ID,
			&item.InviteeUserID,
			&item.InviteeEmail,
			&item.InviteeName,
			&item.Status,
			&basisPoints,
			&item.CommissionBaseAmountCents,
			&item.AmountCents,
			&item.ReleasedAmountCents,
			&item.SettledAmountCents,
			&item.RechargeOrderID,
			&item.RechargeOrderNo,
			&item.CreatedAt,
			&item.ReleasedAt,
			&item.SettledAt,
		); scanErr != nil {
			return nil, scanErr
		}
		item.CommissionRate = basisPointsToRate(basisPoints)
		items = append(items, item)
	}
	return items, rows.Err()
}
