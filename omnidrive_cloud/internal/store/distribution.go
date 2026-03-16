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

var (
	ErrDistributionRelationUserNotFound   = errors.New("distribution relation user not found")
	ErrDistributionRelationSelfInvite     = errors.New("promoter and invitee must be different users")
	ErrDistributionRelationInviteeBound   = errors.New("invitee already has a distribution relation")
	ErrDistributionRuleInvalidRate        = errors.New("distribution commission rate must be between 0 and 1")
	ErrDistributionSettlementNoEligible   = errors.New("no eligible commission items for settlement")
	ErrDistributionSettlementPromoterMiss = errors.New("distribution settlement promoter not found")
)

type AdminDistributionRelationListFilter struct {
	Query  string
	Status string
	AdminPageFilter
}

type AdminCommissionListFilter struct {
	Query  string
	Status string
	AdminPageFilter
}

type AdminSettlementListFilter struct {
	Query  string
	Status string
	AdminPageFilter
}

type CreateDistributionRelationInput struct {
	PromoterUserID   string
	InviteeUserID    string
	Notes            *string
	CreatedByAdminID *string
}

type CreateDistributionRuleInput struct {
	Name                     string
	PromoterUserID           *string
	Status                   string
	CommissionRate           float64
	SettlementThresholdCents int64
	Notes                    *string
	CreatedByAdminID         *string
}

type CreateDistributionSettlementInput struct {
	PromoterUserID *string
	Note           *string
	AdminID        string
	AdminEmail     string
	AdminName      string
}

type distributionReferralRecord struct {
	ID             string
	PromoterUserID string
	InviteeUserID  string
	Status         string
	Notes          *string
}

type distributionRuleRecord struct {
	ID                       string
	Name                     string
	Scope                    string
	PromoterUserID           *string
	Status                   string
	CommissionRateBasisPoint int
	SettlementThresholdCents int64
	Notes                    *string
}

type commissionReleaseState struct {
	TotalGrantedCredits int64
	ConsumedCredits     int64
	AmountCents         int64
	ReleasedAmountCents int64
	SettledAmountCents  int64
	Status              string
	ReleasedAt          *time.Time
}

type commissionSettlementCandidate struct {
	ID                       string
	PromoterUserID           string
	ReleasedAmountCents      int64
	SettledAmountCents       int64
	AmountCents              int64
	SettlementThresholdCents int64
}

func calculateCommissionRateBasisPoints(rate float64) (int, error) {
	if rate <= 0 || rate > 1 {
		return 0, ErrDistributionRuleInvalidRate
	}
	basisPoints := int(math.Round(rate * 10000))
	if basisPoints <= 0 || basisPoints > 10000 {
		return 0, ErrDistributionRuleInvalidRate
	}
	return basisPoints, nil
}

func basisPointsToRate(basisPoints int) float64 {
	if basisPoints <= 0 {
		return 0
	}
	return float64(basisPoints) / 10000
}

func calculateCommissionAmountCents(baseAmountCents int64, basisPoints int) int64 {
	if baseAmountCents <= 0 || basisPoints <= 0 {
		return 0
	}
	return (baseAmountCents*int64(basisPoints) + 5000) / 10000
}

func deriveCommissionStatus(releasedAmountCents int64, settledAmountCents int64, amountCents int64) string {
	if amountCents > 0 && settledAmountCents >= amountCents {
		return "settled"
	}
	if releasedAmountCents > settledAmountCents {
		return "pending_settlement"
	}
	return "pending_consume"
}

func advanceCommissionReleaseState(state commissionReleaseState, debitCredits int64, now time.Time) (commissionReleaseState, int64) {
	if debitCredits <= 0 || state.TotalGrantedCredits <= 0 || state.ConsumedCredits >= state.TotalGrantedCredits {
		return state, 0
	}
	remainingCredits := state.TotalGrantedCredits - state.ConsumedCredits
	consume := debitCredits
	if consume > remainingCredits {
		consume = remainingCredits
	}
	state.ConsumedCredits += consume
	nextReleased := (state.AmountCents * state.ConsumedCredits) / state.TotalGrantedCredits
	if nextReleased > state.AmountCents {
		nextReleased = state.AmountCents
	}
	if nextReleased > state.ReleasedAmountCents {
		state.ReleasedAmountCents = nextReleased
		if state.ReleasedAt == nil {
			releasedAt := now.UTC()
			state.ReleasedAt = &releasedAt
		}
	}
	state.Status = deriveCommissionStatus(state.ReleasedAmountCents, state.SettledAmountCents, state.AmountCents)
	return state, consume
}

func (s *Store) ListAdminDistributionRelations(ctx context.Context, filter AdminDistributionRelationListFilter) ([]domain.AdminDistributionRelationRow, int64, domain.AdminDistributionRelationSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf(`(
			r.id ILIKE $%d OR
			pu.id ILIKE $%d OR pu.email ILIKE $%d OR pu.name ILIKE $%d OR
			iu.id ILIKE $%d OR iu.email ILIKE $%d OR iu.name ILIKE $%d
		)`, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}

	switch strings.TrimSpace(filter.Status) {
	case "active", "inactive":
		whereParts = append(whereParts, fmt.Sprintf("r.status = $%d", argIndex))
		args = append(args, strings.TrimSpace(filter.Status))
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM distribution_referrals r
		INNER JOIN users pu ON pu.id = r.promoter_user_id
		INNER JOIN users iu ON iu.id = r.invitee_user_id
		%s
	`, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, domain.AdminDistributionRelationSummary{}, err
	}

	var summary domain.AdminDistributionRelationSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*)::BIGINT,
			COUNT(*) FILTER (WHERE r.status = 'active')::BIGINT,
			COUNT(*) FILTER (WHERE r.status <> 'active')::BIGINT
		FROM distribution_referrals r
		INNER JOIN users pu ON pu.id = r.promoter_user_id
		INNER JOIN users iu ON iu.id = r.invitee_user_id
		%s
	`, whereClause), args...).Scan(&summary.TotalCount, &summary.ActiveCount, &summary.InactiveCount); err != nil {
		return nil, 0, domain.AdminDistributionRelationSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			r.id,
			pu.id, pu.email, pu.name,
			iu.id, iu.email, iu.name,
			r.status,
			r.created_at,
			r.notes
		FROM distribution_referrals r
		INNER JOIN users pu ON pu.id = r.promoter_user_id
		INNER JOIN users iu ON iu.id = r.invitee_user_id
		%s
		ORDER BY r.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminDistributionRelationSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminDistributionRelationRow, 0)
	for rows.Next() {
		var item domain.AdminDistributionRelationRow
		if scanErr := rows.Scan(
			&item.ID,
			&item.Promoter.ID,
			&item.Promoter.Email,
			&item.Promoter.Name,
			&item.Invitee.ID,
			&item.Invitee.Email,
			&item.Invitee.Name,
			&item.Status,
			&item.CreatedAt,
			&item.Notes,
		); scanErr != nil {
			return nil, 0, domain.AdminDistributionRelationSummary{}, scanErr
		}
		items = append(items, item)
	}
	return items, total, summary, rows.Err()
}

func (s *Store) GetAdminDistributionRelationByID(ctx context.Context, relationID string) (*domain.AdminDistributionRelationRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			r.id,
			pu.id, pu.email, pu.name,
			iu.id, iu.email, iu.name,
			r.status,
			r.created_at,
			r.notes
		FROM distribution_referrals r
		INNER JOIN users pu ON pu.id = r.promoter_user_id
		INNER JOIN users iu ON iu.id = r.invitee_user_id
		WHERE r.id = $1
	`, relationID)

	var item domain.AdminDistributionRelationRow
	if err := row.Scan(
		&item.ID,
		&item.Promoter.ID,
		&item.Promoter.Email,
		&item.Promoter.Name,
		&item.Invitee.ID,
		&item.Invitee.Email,
		&item.Invitee.Name,
		&item.Status,
		&item.CreatedAt,
		&item.Notes,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *Store) CreateDistributionRelation(ctx context.Context, input CreateDistributionRelationInput) (*domain.AdminDistributionRelationRow, error) {
	promoterUserID := strings.TrimSpace(input.PromoterUserID)
	inviteeUserID := strings.TrimSpace(input.InviteeUserID)
	if promoterUserID == "" || inviteeUserID == "" {
		return nil, ErrDistributionRelationUserNotFound
	}
	if promoterUserID == inviteeUserID {
		return nil, ErrDistributionRelationSelfInvite
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var matchedUsers int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)::INT
		FROM users
		WHERE id = ANY($1)
	`, []string{promoterUserID, inviteeUserID}).Scan(&matchedUsers); err != nil {
		return nil, err
	}
	if matchedUsers != 2 {
		return nil, ErrDistributionRelationUserNotFound
	}

	existing, err := getDistributionReferralByInviteeUserIDTx(ctx, tx, inviteeUserID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.PromoterUserID == promoterUserID && existing.Status == "active" {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return s.GetAdminDistributionRelationByID(ctx, existing.ID)
		}
		return nil, ErrDistributionRelationInviteeBound
	}

	relationID := uuid.NewString()
	metadata, _ := json.Marshal(map[string]any{
		"source": "admin_console",
	})
	if _, err := tx.Exec(ctx, `
		INSERT INTO distribution_referrals (
			id, promoter_user_id, invitee_user_id, status, notes, metadata, created_by_admin_user_id
		)
		VALUES ($1, $2, $3, 'active', $4, $5, $6)
	`, relationID, promoterUserID, inviteeUserID, trimOptionalString(input.Notes), metadata, input.CreatedByAdminID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAdminDistributionRelationByID(ctx, relationID)
}

func (s *Store) ListAdminDistributionRules(ctx context.Context) ([]domain.AdminDistributionRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			r.id,
			r.name,
			r.scope,
			r.status,
			r.commission_rate_basis_points,
			r.settlement_threshold_cents,
			r.notes,
			r.created_at,
			r.updated_at,
			u.id,
			u.email,
			u.name
		FROM distribution_rules r
		LEFT JOIN users u ON u.id = r.promoter_user_id
		ORDER BY
			CASE WHEN r.scope = 'promoter' THEN 0 ELSE 1 END ASC,
			r.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AdminDistributionRule, 0)
	for rows.Next() {
		var item domain.AdminDistributionRule
		var promoterID *string
		var promoterEmail *string
		var promoterName *string
		var notes *string
		var basisPoints int
		if scanErr := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Scope,
			&item.Status,
			&basisPoints,
			&item.SettlementThresholdCents,
			&notes,
			&item.CreatedAt,
			&item.UpdatedAt,
			&promoterID,
			&promoterEmail,
			&promoterName,
		); scanErr != nil {
			return nil, scanErr
		}
		item.CommissionRate = basisPointsToRate(basisPoints)
		item.Notes = notes
		if promoterID != nil {
			item.Promoter = &domain.AdminUserSummary{
				ID:    *promoterID,
				Email: valueOrEmpty(promoterEmail),
				Name:  valueOrEmpty(promoterName),
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetAdminDistributionRuleByID(ctx context.Context, ruleID string) (*domain.AdminDistributionRule, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			r.id,
			r.name,
			r.scope,
			r.status,
			r.commission_rate_basis_points,
			r.settlement_threshold_cents,
			r.notes,
			r.created_at,
			r.updated_at,
			u.id,
			u.email,
			u.name
		FROM distribution_rules r
		LEFT JOIN users u ON u.id = r.promoter_user_id
		WHERE r.id = $1
	`, ruleID)

	var item domain.AdminDistributionRule
	var promoterID *string
	var promoterEmail *string
	var promoterName *string
	var notes *string
	var basisPoints int
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Scope,
		&item.Status,
		&basisPoints,
		&item.SettlementThresholdCents,
		&notes,
		&item.CreatedAt,
		&item.UpdatedAt,
		&promoterID,
		&promoterEmail,
		&promoterName,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.CommissionRate = basisPointsToRate(basisPoints)
	item.Notes = notes
	if promoterID != nil {
		item.Promoter = &domain.AdminUserSummary{
			ID:    *promoterID,
			Email: valueOrEmpty(promoterEmail),
			Name:  valueOrEmpty(promoterName),
		}
	}
	return &item, nil
}

func (s *Store) CreateDistributionRule(ctx context.Context, input CreateDistributionRuleInput) (*domain.AdminDistributionRule, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("distribution rule name is required")
	}
	if input.SettlementThresholdCents < 0 {
		return nil, fmt.Errorf("settlement threshold must be greater than or equal to 0")
	}

	basisPoints, err := calculateCommissionRateBasisPoints(input.CommissionRate)
	if err != nil {
		return nil, err
	}

	scope := "default"
	promoterUserID := trimOptionalString(input.PromoterUserID)
	if promoterUserID != nil {
		scope = "promoter"
		var exists bool
		if err := s.pool.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)
		`, *promoterUserID).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrDistributionRelationUserNotFound
		}
	}

	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "active"
	}

	ruleID := uuid.NewString()
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO distribution_rules (
			id, name, scope, promoter_user_id, status, commission_rate_basis_points,
			settlement_threshold_cents, notes, created_by_admin_user_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, ruleID, name, scope, promoterUserID, status, basisPoints, input.SettlementThresholdCents, trimOptionalString(input.Notes), input.CreatedByAdminID); err != nil {
		return nil, err
	}
	return s.GetAdminDistributionRuleByID(ctx, ruleID)
}

func getDistributionReferralByInviteeUserIDTx(ctx context.Context, tx pgx.Tx, inviteeUserID string) (*distributionReferralRecord, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, promoter_user_id, invitee_user_id, status, notes
		FROM distribution_referrals
		WHERE invitee_user_id = $1
		LIMIT 1
	`, inviteeUserID)

	var item distributionReferralRecord
	if err := row.Scan(&item.ID, &item.PromoterUserID, &item.InviteeUserID, &item.Status, &item.Notes); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func findApplicableDistributionRuleTx(ctx context.Context, tx pgx.Tx, promoterUserID string) (*distributionRuleRecord, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id,
			name,
			scope,
			promoter_user_id,
			status,
			commission_rate_basis_points,
			settlement_threshold_cents,
			notes
		FROM distribution_rules
		WHERE status = 'active'
		  AND (
			(scope = 'promoter' AND promoter_user_id = $1) OR
			scope = 'default'
		  )
		ORDER BY
			CASE WHEN scope = 'promoter' AND promoter_user_id = $1 THEN 0 ELSE 1 END ASC,
			created_at DESC
		LIMIT 1
	`, promoterUserID)

	var item distributionRuleRecord
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Scope,
		&item.PromoterUserID,
		&item.Status,
		&item.CommissionRateBasisPoint,
		&item.SettlementThresholdCents,
		&item.Notes,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *Store) ensureDistributionCommissionForRechargeOrderTx(ctx context.Context, tx pgx.Tx, order *domain.RechargeOrder) error {
	if order == nil || strings.TrimSpace(order.UserID) == "" || strings.TrimSpace(order.ID) == "" {
		return nil
	}

	var existingCount int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)::INT
		FROM distribution_commission_items
		WHERE recharge_order_id = $1
	`, order.ID).Scan(&existingCount); err != nil {
		return err
	}
	if existingCount > 0 {
		return nil
	}

	referral, err := getDistributionReferralByInviteeUserIDTx(ctx, tx, order.UserID)
	if err != nil {
		return err
	}
	if referral == nil || referral.Status != "active" {
		return nil
	}

	rule, err := findApplicableDistributionRuleTx(ctx, tx, referral.PromoterUserID)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}

	totalGrantedCredits := order.CreditAmount + order.ManualBonusCreditAmount
	if totalGrantedCredits <= 0 {
		return nil
	}
	commissionAmount := calculateCommissionAmountCents(order.AmountCents, rule.CommissionRateBasisPoint)
	if commissionAmount <= 0 {
		return nil
	}

	metadata, _ := json.Marshal(map[string]any{
		"orderId":           order.ID,
		"orderNo":           order.OrderNo,
		"channel":           order.Channel,
		"amountCents":       order.AmountCents,
		"creditAmount":      order.CreditAmount,
		"bonusCreditAmount": order.ManualBonusCreditAmount,
	})
	_, err = tx.Exec(ctx, `
		INSERT INTO distribution_commission_items (
			id, referral_id, rule_id, promoter_user_id, invitee_user_id, recharge_order_id, status,
			commission_rate_basis_points, settlement_threshold_cents, commission_base_amount_cents,
			amount_cents, total_granted_credits, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending_consume', $7, $8, $9, $10, $11, $12)
	`, uuid.NewString(), referral.ID, rule.ID, referral.PromoterUserID, referral.InviteeUserID, order.ID, rule.CommissionRateBasisPoint, rule.SettlementThresholdCents, order.AmountCents, commissionAmount, totalGrantedCredits, metadata)
	return err
}

func (s *Store) releaseDistributionCommissionForUsageTx(ctx context.Context, tx pgx.Tx, inviteeUserID string, sourceType string, sourceID string, debitedCredits int64) error {
	if strings.TrimSpace(inviteeUserID) == "" || debitedCredits <= 0 {
		return nil
	}

	rows, err := tx.Query(ctx, `
		SELECT id, status, amount_cents, total_granted_credits, consumed_credits,
		       released_amount_cents, settled_amount_cents, released_at
		FROM distribution_commission_items
		WHERE invitee_user_id = $1
		  AND status IN ('pending_consume', 'pending_settlement')
		  AND total_granted_credits > consumed_credits
		ORDER BY created_at ASC
		FOR UPDATE
	`, inviteeUserID)
	if err != nil {
		return err
	}
	defer rows.Close()

	remainingCredits := debitedCredits
	now := time.Now().UTC()
	for rows.Next() {
		if remainingCredits <= 0 {
			break
		}
		var itemID string
		var state commissionReleaseState
		if scanErr := rows.Scan(
			&itemID,
			&state.Status,
			&state.AmountCents,
			&state.TotalGrantedCredits,
			&state.ConsumedCredits,
			&state.ReleasedAmountCents,
			&state.SettledAmountCents,
			&state.ReleasedAt,
		); scanErr != nil {
			return scanErr
		}

		nextState, consumedCredits := advanceCommissionReleaseState(state, remainingCredits, now)
		if consumedCredits <= 0 {
			continue
		}
		remainingCredits -= consumedCredits

		if _, err := tx.Exec(ctx, `
			UPDATE distribution_commission_items
			SET status = $2,
			    consumed_credits = $3,
			    released_amount_cents = $4,
			    released_at = $5,
			    last_release_source_type = $6,
			    last_release_source_id = $7,
			    updated_at = NOW()
			WHERE id = $1
		`, itemID, nextState.Status, nextState.ConsumedCredits, nextState.ReleasedAmountCents, nextState.ReleasedAt, strings.TrimSpace(sourceType), strings.TrimSpace(sourceID)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Store) ListAdminCommissions(ctx context.Context, filter AdminCommissionListFilter) ([]domain.AdminCommissionRow, int64, domain.AdminCommissionListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf(`(
			c.id ILIKE $%d OR
			ro.order_no ILIKE $%d OR
			pu.id ILIKE $%d OR pu.email ILIKE $%d OR pu.name ILIKE $%d OR
			iu.id ILIKE $%d OR iu.email ILIKE $%d OR iu.name ILIKE $%d
		)`, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}

	switch strings.TrimSpace(filter.Status) {
	case "pending_consume", "pending_settlement", "settled":
		whereParts = append(whereParts, fmt.Sprintf("c.status = $%d", argIndex))
		args = append(args, strings.TrimSpace(filter.Status))
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM distribution_commission_items c
		INNER JOIN users pu ON pu.id = c.promoter_user_id
		INNER JOIN users iu ON iu.id = c.invitee_user_id
		LEFT JOIN recharge_orders ro ON ro.id = c.recharge_order_id
		%s
	`, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, domain.AdminCommissionListSummary{}, err
	}

	var summary domain.AdminCommissionListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(SUM(c.amount_cents), 0)::BIGINT,
			COALESCE(SUM(GREATEST(c.amount_cents - c.released_amount_cents, 0)), 0)::BIGINT,
			COALESCE(SUM(GREATEST(c.released_amount_cents - c.settled_amount_cents, 0)), 0)::BIGINT,
			COALESCE(SUM(c.settled_amount_cents), 0)::BIGINT,
			COALESCE(SUM(GREATEST(c.released_amount_cents - c.settled_amount_cents, 0)), 0)::BIGINT
		FROM distribution_commission_items c
		INNER JOIN users pu ON pu.id = c.promoter_user_id
		INNER JOIN users iu ON iu.id = c.invitee_user_id
		LEFT JOIN recharge_orders ro ON ro.id = c.recharge_order_id
		%s
	`, whereClause), args...).Scan(
		&summary.TotalCommissionAmountCents,
		&summary.PendingConsumeAmountCents,
		&summary.PendingSettlementAmountCents,
		&summary.SettledAmountCents,
		&summary.ReleasedButUnsettledAmountCts,
	); err != nil {
		return nil, 0, domain.AdminCommissionListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			c.id,
			pu.id, pu.email, pu.name,
			iu.id, iu.email, iu.name,
			c.status,
			c.commission_rate_basis_points,
			c.commission_base_amount_cents,
			c.amount_cents,
			c.created_at,
			c.released_at,
			c.settled_at
		FROM distribution_commission_items c
		INNER JOIN users pu ON pu.id = c.promoter_user_id
		INNER JOIN users iu ON iu.id = c.invitee_user_id
		LEFT JOIN recharge_orders ro ON ro.id = c.recharge_order_id
		%s
		ORDER BY c.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminCommissionListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminCommissionRow, 0)
	for rows.Next() {
		var item domain.AdminCommissionRow
		var basisPoints int
		if scanErr := rows.Scan(
			&item.ID,
			&item.Promoter.ID,
			&item.Promoter.Email,
			&item.Promoter.Name,
			&item.Invitee.ID,
			&item.Invitee.Email,
			&item.Invitee.Name,
			&item.Status,
			&basisPoints,
			&item.CommissionBaseAmountCents,
			&item.AmountCents,
			&item.CreatedAt,
			&item.ReleasedAt,
			&item.SettledAt,
		); scanErr != nil {
			return nil, 0, domain.AdminCommissionListSummary{}, scanErr
		}
		item.CommissionRate = basisPointsToRate(basisPoints)
		items = append(items, item)
	}
	return items, total, summary, rows.Err()
}

func formatDistributionSettlementBatchNo(now time.Time) string {
	return fmt.Sprintf("SET-%s-%s", now.UTC().Format("20060102150405"), strings.ToUpper(uuid.NewString()[:6]))
}

func (s *Store) ListAdminSettlements(ctx context.Context, filter AdminSettlementListFilter) ([]domain.AdminSettlementRow, int64, domain.AdminSettlementListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf(`(
			b.id ILIKE $%d OR
			b.batch_no ILIKE $%d OR
			COALESCE(b.reviewer_admin_name, '') ILIKE $%d OR
			COALESCE(b.operator_admin_name, '') ILIKE $%d
		)`, argIndex, argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}

	switch strings.TrimSpace(filter.Status) {
	case "pending", "completed":
		whereParts = append(whereParts, fmt.Sprintf("b.status = $%d", argIndex))
		args = append(args, strings.TrimSpace(filter.Status))
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM distribution_settlement_batches b
		%s
	`, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, domain.AdminSettlementListSummary{}, err
	}

	var summary domain.AdminSettlementListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*)::BIGINT,
			COUNT(*) FILTER (WHERE b.status = 'pending')::BIGINT,
			COUNT(*) FILTER (WHERE b.status = 'completed')::BIGINT,
			COALESCE(SUM(si.total_amount_cents), 0)::BIGINT,
			COALESCE(SUM(CASE WHEN b.status = 'completed' THEN si.total_amount_cents ELSE 0 END), 0)::BIGINT,
			COALESCE(SUM(CASE WHEN b.status <> 'completed' THEN si.total_amount_cents ELSE 0 END), 0)::BIGINT
		FROM distribution_settlement_batches b
		LEFT JOIN (
			SELECT batch_id, COUNT(*)::BIGINT AS item_count, COALESCE(SUM(amount_cents), 0)::BIGINT AS total_amount_cents
			FROM distribution_settlement_items
			GROUP BY batch_id
		) si ON si.batch_id = b.id
		%s
	`, whereClause), args...).Scan(
		&summary.TotalBatchCount,
		&summary.PendingBatchCount,
		&summary.CompletedBatchCount,
		&summary.TotalAmountCents,
		&summary.PaidOutAmountCents,
		&summary.OutstandingAmountCts,
	); err != nil {
		return nil, 0, domain.AdminSettlementListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			b.id,
			b.batch_no,
			b.status,
			COALESCE(si.item_count, 0)::BIGINT,
			COALESCE(si.total_amount_cents, 0)::BIGINT,
			b.created_at,
			b.reviewed_at,
			b.paid_at,
			b.reviewer_admin_name,
			b.operator_admin_name,
			b.notes
		FROM distribution_settlement_batches b
		LEFT JOIN (
			SELECT batch_id, COUNT(*)::BIGINT AS item_count, COALESCE(SUM(amount_cents), 0)::BIGINT AS total_amount_cents
			FROM distribution_settlement_items
			GROUP BY batch_id
		) si ON si.batch_id = b.id
		%s
		ORDER BY b.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminSettlementListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminSettlementRow, 0)
	for rows.Next() {
		var item domain.AdminSettlementRow
		if scanErr := rows.Scan(
			&item.ID,
			&item.BatchNo,
			&item.Status,
			&item.ItemCount,
			&item.TotalAmountCents,
			&item.CreatedAt,
			&item.ReviewedAt,
			&item.PaidAt,
			&item.Reviewer,
			&item.Operator,
			&item.Notes,
		); scanErr != nil {
			return nil, 0, domain.AdminSettlementListSummary{}, scanErr
		}
		items = append(items, item)
	}
	return items, total, summary, rows.Err()
}

func (s *Store) GetAdminSettlementByID(ctx context.Context, batchID string) (*domain.AdminSettlementRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			b.id,
			b.batch_no,
			b.status,
			COALESCE(si.item_count, 0)::BIGINT,
			COALESCE(si.total_amount_cents, 0)::BIGINT,
			b.created_at,
			b.reviewed_at,
			b.paid_at,
			b.reviewer_admin_name,
			b.operator_admin_name,
			b.notes
		FROM distribution_settlement_batches b
		LEFT JOIN (
			SELECT batch_id, COUNT(*)::BIGINT AS item_count, COALESCE(SUM(amount_cents), 0)::BIGINT AS total_amount_cents
			FROM distribution_settlement_items
			GROUP BY batch_id
		) si ON si.batch_id = b.id
		WHERE b.id = $1
	`, batchID)

	var item domain.AdminSettlementRow
	if err := row.Scan(
		&item.ID,
		&item.BatchNo,
		&item.Status,
		&item.ItemCount,
		&item.TotalAmountCents,
		&item.CreatedAt,
		&item.ReviewedAt,
		&item.PaidAt,
		&item.Reviewer,
		&item.Operator,
		&item.Notes,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *Store) CreateDistributionSettlementBatch(ctx context.Context, input CreateDistributionSettlementInput) (*domain.AdminSettlementRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if input.PromoterUserID != nil {
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)
		`, strings.TrimSpace(*input.PromoterUserID)).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrDistributionSettlementPromoterMiss
		}
	}

	query := `
		SELECT id, promoter_user_id, released_amount_cents, settled_amount_cents, amount_cents, settlement_threshold_cents
		FROM distribution_commission_items
		WHERE released_amount_cents > settled_amount_cents
	`
	args := []any{}
	if input.PromoterUserID != nil {
		query += " AND promoter_user_id = $1"
		args = append(args, strings.TrimSpace(*input.PromoterUserID))
	}
	query += " ORDER BY created_at ASC FOR UPDATE"

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grouped := make(map[string][]commissionSettlementCandidate)
	groupTotal := make(map[string]int64)
	groupThreshold := make(map[string]int64)
	for rows.Next() {
		var item commissionSettlementCandidate
		if scanErr := rows.Scan(
			&item.ID,
			&item.PromoterUserID,
			&item.ReleasedAmountCents,
			&item.SettledAmountCents,
			&item.AmountCents,
			&item.SettlementThresholdCents,
		); scanErr != nil {
			return nil, scanErr
		}
		available := item.ReleasedAmountCents - item.SettledAmountCents
		if available <= 0 {
			continue
		}
		grouped[item.PromoterUserID] = append(grouped[item.PromoterUserID], item)
		groupTotal[item.PromoterUserID] += available
		if item.SettlementThresholdCents > groupThreshold[item.PromoterUserID] {
			groupThreshold[item.PromoterUserID] = item.SettlementThresholdCents
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	eligible := make([]commissionSettlementCandidate, 0)
	for promoterUserID, items := range grouped {
		if groupTotal[promoterUserID] < groupThreshold[promoterUserID] {
			continue
		}
		eligible = append(eligible, items...)
	}
	if len(eligible) == 0 {
		return nil, ErrDistributionSettlementNoEligible
	}

	now := time.Now().UTC()
	batchID := uuid.NewString()
	batchNo := formatDistributionSettlementBatchNo(now)
	trimmedNote := trimOptionalString(input.Note)
	if _, err := tx.Exec(ctx, `
		INSERT INTO distribution_settlement_batches (
			id, batch_no, status, notes,
			reviewer_admin_user_id, reviewer_admin_email, reviewer_admin_name,
			operator_admin_user_id, operator_admin_email, operator_admin_name,
			reviewed_at, paid_at
		)
		VALUES ($1, $2, 'completed', $3, $4, $5, $6, $4, $5, $6, $7, $7)
	`, batchID, batchNo, trimmedNote, strings.TrimSpace(input.AdminID), strings.TrimSpace(input.AdminEmail), strings.TrimSpace(input.AdminName), now); err != nil {
		return nil, err
	}

	for _, item := range eligible {
		available := item.ReleasedAmountCents - item.SettledAmountCents
		if available <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO distribution_settlement_items (
				id, batch_id, commission_item_id, promoter_user_id, amount_cents
			)
			VALUES ($1, $2, $3, $4, $5)
		`, uuid.NewString(), batchID, item.ID, item.PromoterUserID, available); err != nil {
			return nil, err
		}

		newSettledAmount := item.ReleasedAmountCents
		nextStatus := deriveCommissionStatus(item.ReleasedAmountCents, newSettledAmount, item.AmountCents)
		var settledAt *time.Time
		if newSettledAmount >= item.AmountCents {
			settledAt = &now
		}
		if _, err := tx.Exec(ctx, `
			UPDATE distribution_commission_items
			SET status = $2,
			    settled_amount_cents = $3,
			    settled_at = $4,
			    settlement_batch_id = $5,
			    updated_at = NOW()
			WHERE id = $1
		`, item.ID, nextStatus, newSettledAmount, settledAt, batchID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAdminSettlementByID(ctx, batchID)
}
