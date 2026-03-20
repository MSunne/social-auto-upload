package store

import (
	"context"
	"fmt"
	"strings"

	"omnidrive_cloud/internal/domain"
)

type AdminPartnerProfileListFilter struct {
	Query  string
	Status string
	AdminPageFilter
}

func (s *Store) ListAdminPartnerProfiles(ctx context.Context, filter AdminPartnerProfileListFilter) ([]domain.AdminPartnerProfileRow, int64, domain.AdminPartnerProfileSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf(`(
			u.id ILIKE $%d OR
			u.email ILIKE $%d OR
			u.name ILIKE $%d OR
			p.partner_code ILIKE $%d OR
			p.partner_name ILIKE $%d
		)`, argIndex, argIndex, argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}

	switch strings.TrimSpace(filter.Status) {
	case "active", "inactive":
		whereParts = append(whereParts, fmt.Sprintf("p.status = $%d", argIndex))
		args = append(args, strings.TrimSpace(filter.Status))
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")
	baseFrom := `
		FROM partner_profiles p
		INNER JOIN users u ON u.id = p.user_id
		LEFT JOIN LATERAL (
			SELECT COUNT(*)::BIGINT AS invitee_count
			FROM distribution_referrals r
			WHERE r.promoter_user_id = p.user_id
			  AND r.status = 'active'
		) rel ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				COALESCE(SUM(GREATEST(c.released_amount_cents - c.settled_amount_cents, 0)), 0)::BIGINT AS pending_settlement_amount_cents,
				COALESCE(SUM(c.settled_amount_cents), 0)::BIGINT AS settled_amount_cents
			FROM distribution_commission_items c
			WHERE c.promoter_user_id = p.user_id
		) comm ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				COALESCE(SUM(CASE WHEN w.status = 'requested' THEN w.amount_cents ELSE 0 END), 0)::BIGINT AS requested_amount_cents,
				COALESCE(SUM(CASE WHEN w.status = 'approved' THEN w.amount_cents ELSE 0 END), 0)::BIGINT AS approved_amount_cents,
				COALESCE(SUM(CASE WHEN w.status = 'paid' THEN w.amount_cents ELSE 0 END), 0)::BIGINT AS paid_amount_cents
			FROM withdrawal_requests w
			WHERE w.promoter_user_id = p.user_id
		) wd ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				r.commission_rate_basis_points,
				r.settlement_threshold_cents
			FROM distribution_rules r
			WHERE r.status = 'active'
			  AND (
				(r.scope = 'promoter' AND r.promoter_user_id = p.user_id) OR
				r.scope = 'default'
			  )
			ORDER BY
				CASE WHEN r.scope = 'promoter' AND r.promoter_user_id = p.user_id THEN 0 ELSE 1 END ASC,
				r.created_at DESC
			LIMIT 1
		) rule_view ON TRUE
	`

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)::BIGINT
		%s
		%s
	`, baseFrom, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, domain.AdminPartnerProfileSummary{}, err
	}

	var summary domain.AdminPartnerProfileSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*)::BIGINT,
			COUNT(*) FILTER (WHERE p.status = 'active')::BIGINT,
			COALESCE(SUM(rel.invitee_count), 0)::BIGINT,
			COALESCE(SUM(comm.pending_settlement_amount_cents), 0)::BIGINT,
			COALESCE(SUM(comm.settled_amount_cents), 0)::BIGINT
		%s
		%s
	`, baseFrom, whereClause), args...).Scan(
		&summary.TotalCount,
		&summary.ActiveCount,
		&summary.InviteeCount,
		&summary.PendingSettlementAmountCents,
		&summary.SettledAmountCents,
	); err != nil {
		return nil, 0, domain.AdminPartnerProfileSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			u.id,
			u.email,
			u.name,
			p.partner_code,
			p.partner_name,
			p.status,
			COALESCE(rule_view.commission_rate_basis_points, 0)::INT,
			COALESCE(rule_view.settlement_threshold_cents, 0)::BIGINT,
			COALESCE(rel.invitee_count, 0)::BIGINT,
			COALESCE(comm.pending_settlement_amount_cents, 0)::BIGINT,
			COALESCE(comm.settled_amount_cents, 0)::BIGINT,
			GREATEST(
				COALESCE(comm.settled_amount_cents, 0)::BIGINT -
				COALESCE(wd.requested_amount_cents, 0)::BIGINT -
				COALESCE(wd.approved_amount_cents, 0)::BIGINT -
				COALESCE(wd.paid_amount_cents, 0)::BIGINT,
				0
			)::BIGINT,
			p.created_at,
			p.updated_at
		%s
		%s
		ORDER BY p.created_at DESC
		LIMIT $%d OFFSET $%d
	`, baseFrom, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminPartnerProfileSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminPartnerProfileRow, 0, pageSize)
	for rows.Next() {
		var item domain.AdminPartnerProfileRow
		var commissionRateBasisPoints int
		if scanErr := rows.Scan(
			&item.User.ID,
			&item.User.Email,
			&item.User.Name,
			&item.PartnerCode,
			&item.PartnerName,
			&item.Status,
			&commissionRateBasisPoints,
			&item.SettlementThresholdCents,
			&item.InviteeCount,
			&item.PendingSettlementAmountCents,
			&item.SettledAmountCents,
			&item.AvailableWithdrawalAmountCents,
			&item.CreatedAt,
			&item.UpdatedAt,
		); scanErr != nil {
			return nil, 0, domain.AdminPartnerProfileSummary{}, scanErr
		}
		item.CurrentCommissionRate = basisPointsToRate(commissionRateBasisPoints)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, domain.AdminPartnerProfileSummary{}, err
	}

	return items, total, summary, nil
}
