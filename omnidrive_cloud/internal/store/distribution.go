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
	ErrDistributionRelationNotFound       = errors.New("distribution relation not found")
	ErrDistributionRelationStatusInvalid  = errors.New("distribution relation status must be active or inactive")
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

type scannedAdminUserSummary struct {
	ID    string
	Email *string
	Name  *string
}

func (u scannedAdminUserSummary) summary() domain.AdminUserSummary {
	return domain.AdminUserSummary{
		ID:    u.ID,
		Email: valueOrEmpty(u.Email),
		Name:  valueOrEmpty(u.Name),
	}
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

type UpdateDistributionRelationInput struct {
	RelationID string
	Status     string
	Notes      *string
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

type distributionGrantSnapshot struct {
	TotalGrantedCredits int64
	WalletGrantCredits  int64
	QuotaUnitCredits    map[string]int64
}

type distributionCommissionItemRecord struct {
	ID                        string
	ReferralID                *string
	RuleID                    *string
	PromoterUserID            string
	InviteeUserID             string
	RechargeOrderID           string
	Status                    string
	CommissionRateBasisPoint  int
	SettlementThresholdCents  int64
	CommissionBaseAmountCents int64
	AmountCents               int64
	TotalGrantedCredits       int64
	ConsumedCredits           int64
	ReleasedAmountCents       int64
	SettledAmountCents        int64
	ReleasedAt                *time.Time
	SettledAt                 *time.Time
}

type distributionCommissionReleaseInput struct {
	CommissionItemID     string
	SourceType           string
	SourceID             string
	SourceSnapshot       []byte
	WalletLotID          *string
	QuotaAccountID       *string
	WalletLedgerID       *string
	QuotaLedgerID        *string
	ConsumedCreditsDelta int64
	Metadata             []byte
}

// 加载额度Unit额度映射事务，供存储层继续处理当前业务状态。
func loadQuotaUnitCreditMapTx(ctx context.Context, tx pgx.Tx, meterCodes map[string]struct{}) (map[string]int64, error) {
	result := make(map[string]int64)
	if len(meterCodes) == 0 {
		return result, nil
	}

	codes := make([]string, 0, len(meterCodes))
	for code := range meterCodes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		codes = append(codes, trimmed)
	}
	if len(codes) == 0 {
		return result, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT DISTINCT ON (quota_meter_code)
			quota_meter_code,
			wallet_debit_amount
		FROM billing_pricing_rules
		WHERE is_enabled = TRUE
		  AND charge_mode = 'quota_first_wallet_fallback'
		  AND quota_meter_code = ANY($1)
		  AND wallet_debit_amount > 0
		ORDER BY quota_meter_code, sort_order ASC, created_at ASC
	`, codes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var meterCode string
		var walletDebitAmount int64
		if scanErr := rows.Scan(&meterCode, &walletDebitAmount); scanErr != nil {
			return nil, scanErr
		}
		result[strings.TrimSpace(meterCode)] = walletDebitAmount
	}
	return result, rows.Err()
}

// 处理calculate分销Grant额度Entitlements相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func calculateDistributionGrantCreditsFromEntitlements(entitlements []domain.BillingPackageEntitlement, quotaUnitCredits map[string]int64) (int64, int64) {
	var total int64
	var walletGrantCredits int64
	for _, entitlement := range entitlements {
		if entitlement.GrantAmount <= 0 {
			continue
		}
		meterCode := strings.TrimSpace(entitlement.MeterCode)
		switch meterCode {
		case "", "wallet_credit":
			total += entitlement.GrantAmount
			walletGrantCredits += entitlement.GrantAmount
		default:
			if unitCredits := quotaUnitCredits[meterCode]; unitCredits > 0 {
				total += entitlement.GrantAmount * unitCredits
			}
		}
	}
	return total, walletGrantCredits
}

// 根据充值订单事务计算分销GrantSnapshot，供存储层的状态判定和查询逻辑复用。
func (s *Store) distributionGrantSnapshotForRechargeOrderTx(ctx context.Context, tx pgx.Tx, order *domain.RechargeOrder) (distributionGrantSnapshot, error) {
	snapshot := distributionGrantSnapshot{
		QuotaUnitCredits: make(map[string]int64),
	}
	if order == nil {
		return snapshot, nil
	}

	entitlements, _, _ := buildSupportRechargeGrantPlan(order, time.Now().UTC())
	if len(entitlements) == 0 {
		snapshot.TotalGrantedCredits = order.CreditAmount + order.ManualBonusCreditAmount
		snapshot.WalletGrantCredits = order.CreditAmount + order.ManualBonusCreditAmount
		if snapshot.TotalGrantedCredits < 0 {
			snapshot.TotalGrantedCredits = 0
		}
		if snapshot.WalletGrantCredits < 0 {
			snapshot.WalletGrantCredits = 0
		}
		return snapshot, nil
	}

	quotaMeterCodes := make(map[string]struct{})
	for _, entitlement := range entitlements {
		meterCode := strings.TrimSpace(entitlement.MeterCode)
		if meterCode == "" || meterCode == "wallet_credit" || entitlement.GrantAmount <= 0 {
			continue
		}
		quotaMeterCodes[meterCode] = struct{}{}
	}

	quotaUnitCredits, err := loadQuotaUnitCreditMapTx(ctx, tx, quotaMeterCodes)
	if err != nil {
		return snapshot, err
	}
	snapshot.QuotaUnitCredits = quotaUnitCredits

	totalGrantedCredits, walletGrantCredits := calculateDistributionGrantCreditsFromEntitlements(entitlements, quotaUnitCredits)
	if totalGrantedCredits <= 0 {
		totalGrantedCredits = order.CreditAmount + order.ManualBonusCreditAmount
		walletGrantCredits = totalGrantedCredits
	}
	if totalGrantedCredits < 0 {
		totalGrantedCredits = 0
	}
	if walletGrantCredits < 0 {
		walletGrantCredits = 0
	}
	snapshot.TotalGrantedCredits = totalGrantedCredits
	snapshot.WalletGrantCredits = walletGrantCredits
	return snapshot, nil
}

// 处理calculateCommissionRateBasisPoints相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理basisPointsRate相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func basisPointsToRate(basisPoints int) float64 {
	if basisPoints <= 0 {
		return 0
	}
	return float64(basisPoints) / 10000
}

// 处理calculateCommissionAmountCents相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func calculateCommissionAmountCents(baseAmountCents int64, basisPoints int) int64 {
	if baseAmountCents <= 0 || basisPoints <= 0 {
		return 0
	}
	return (baseAmountCents*int64(basisPoints) + 5000) / 10000
}

// 处理deriveCommission状态相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func deriveCommissionStatus(releasedAmountCents int64, settledAmountCents int64, amountCents int64) string {
	if amountCents > 0 && settledAmountCents >= amountCents {
		return "settled"
	}
	if releasedAmountCents > settledAmountCents {
		return "pending_settlement"
	}
	return "pending_consume"
}

// 处理advanceCommission释放状态相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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
		var promoter scannedAdminUserSummary
		var invitee scannedAdminUserSummary
		if scanErr := rows.Scan(
			&item.ID,
			&promoter.ID,
			&promoter.Email,
			&promoter.Name,
			&invitee.ID,
			&invitee.Email,
			&invitee.Name,
			&item.Status,
			&item.CreatedAt,
			&item.Notes,
		); scanErr != nil {
			return nil, 0, domain.AdminDistributionRelationSummary{}, scanErr
		}
		item.Promoter = promoter.summary()
		item.Invitee = invitee.summary()
		items = append(items, item)
	}
	return items, total, summary, rows.Err()
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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
	var promoter scannedAdminUserSummary
	var invitee scannedAdminUserSummary
	if err := row.Scan(
		&item.ID,
		&promoter.ID,
		&promoter.Email,
		&promoter.Name,
		&invitee.ID,
		&invitee.Email,
		&invitee.Name,
		&item.Status,
		&item.CreatedAt,
		&item.Notes,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.Promoter = promoter.summary()
	item.Invitee = invitee.summary()
	return &item, nil
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
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

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) UpdateDistributionRelation(ctx context.Context, input UpdateDistributionRelationInput) (*domain.AdminDistributionRelationRow, error) {
	relationID := strings.TrimSpace(input.RelationID)
	status := strings.TrimSpace(input.Status)
	if relationID == "" {
		return nil, ErrDistributionRelationNotFound
	}
	if status != "active" && status != "inactive" {
		return nil, ErrDistributionRelationStatusInvalid
	}

	commandTag, err := s.pool.Exec(ctx, `
		UPDATE distribution_referrals
		SET status = $2,
		    notes = CASE
		      WHEN $3::TEXT IS NULL THEN notes
		      ELSE $3::TEXT
		    END,
		    updated_at = NOW()
		WHERE id = $1
	`, relationID, status, trimOptionalString(input.Notes))
	if err != nil {
		return nil, err
	}
	if commandTag.RowsAffected() == 0 {
		return nil, ErrDistributionRelationNotFound
	}
	return s.GetAdminDistributionRelationByID(ctx, relationID)
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
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

// 获取分销ReferralInvitee用户ID事务，为当前链路返回后续处理所需的数据内容。
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

// 处理扫描分销CommissionItem相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func scanDistributionCommissionItem(scan scanFn) (*distributionCommissionItemRecord, error) {
	var item distributionCommissionItemRecord
	if err := scan(
		&item.ID,
		&item.ReferralID,
		&item.RuleID,
		&item.PromoterUserID,
		&item.InviteeUserID,
		&item.RechargeOrderID,
		&item.Status,
		&item.CommissionRateBasisPoint,
		&item.SettlementThresholdCents,
		&item.CommissionBaseAmountCents,
		&item.AmountCents,
		&item.TotalGrantedCredits,
		&item.ConsumedCredits,
		&item.ReleasedAmountCents,
		&item.SettledAmountCents,
		&item.ReleasedAt,
		&item.SettledAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

// 获取分销CommissionItem充值订单事务，为当前链路返回后续处理所需的数据内容。
func getDistributionCommissionItemByRechargeOrderTx(ctx context.Context, tx pgx.Tx, rechargeOrderID string) (*distributionCommissionItemRecord, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id,
			referral_id,
			rule_id,
			promoter_user_id,
			invitee_user_id,
			recharge_order_id,
			status,
			commission_rate_basis_points,
			settlement_threshold_cents,
			commission_base_amount_cents,
			amount_cents,
			total_granted_credits,
			consumed_credits,
			released_amount_cents,
			settled_amount_cents,
			released_at,
			settled_at
		FROM distribution_commission_items
		WHERE recharge_order_id = $1
		LIMIT 1
	`, strings.TrimSpace(rechargeOrderID))

	item, err := scanDistributionCommissionItem(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

// 获取分销CommissionItemID事务，为当前链路返回后续处理所需的数据内容。
func getDistributionCommissionItemByIDTx(ctx context.Context, tx pgx.Tx, commissionItemID string) (*distributionCommissionItemRecord, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id,
			referral_id,
			rule_id,
			promoter_user_id,
			invitee_user_id,
			recharge_order_id,
			status,
			commission_rate_basis_points,
			settlement_threshold_cents,
			commission_base_amount_cents,
			amount_cents,
			total_granted_credits,
			consumed_credits,
			released_amount_cents,
			settled_amount_cents,
			released_at,
			settled_at
		FROM distribution_commission_items
		WHERE id = $1
		FOR UPDATE
	`, strings.TrimSpace(commissionItemID))

	item, err := scanDistributionCommissionItem(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

// 构建分销来源Snapshot事务，为存储层生成后续步骤所需的派生参数或载荷。
func buildDistributionSourceSnapshotTx(ctx context.Context, tx pgx.Tx, sourceType string, sourceID string) ([]byte, error) {
	trimmedType := strings.TrimSpace(sourceType)
	trimmedID := strings.TrimSpace(sourceID)
	if trimmedType == "" {
		return nil, nil
	}

	switch trimmedType {
	case "ai_job":
		var jobType string
		var modelName string
		var status string
		var source string
		var localTaskID *string
		var localPublishTaskID *string
		var createdAt time.Time
		err := tx.QueryRow(ctx, `
			SELECT job_type, model_name, status, source, local_task_id, local_publish_task_id, created_at
			FROM ai_jobs
			WHERE id = $1
		`, trimmedID).Scan(&jobType, &modelName, &status, &source, &localTaskID, &localPublishTaskID, &createdAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				break
			}
			return nil, err
		}

		snapshot := map[string]any{
			"type":      "ai_job",
			"id":        trimmedID,
			"title":     fmt.Sprintf("AI %s 任务", strings.ToUpper(strings.TrimSpace(jobType))),
			"jobType":   jobType,
			"modelName": modelName,
			"status":    status,
			"source":    source,
			"createdAt": createdAt.Format(time.RFC3339),
		}
		if localTaskID != nil {
			snapshot["localTaskId"] = strings.TrimSpace(*localTaskID)
		}
		if localPublishTaskID != nil {
			snapshot["publishTaskId"] = strings.TrimSpace(*localPublishTaskID)
			var title *string
			var platform string
			var accountName string
			var taskStatus string
			var runAt *time.Time
			var taskCreatedAt time.Time
			if err := tx.QueryRow(ctx, `
				SELECT title, platform, account_name, status, run_at, created_at
				FROM publish_tasks
				WHERE id = $1
			`, strings.TrimSpace(*localPublishTaskID)).Scan(&title, &platform, &accountName, &taskStatus, &runAt, &taskCreatedAt); err == nil {
				snapshot["publishTask"] = map[string]any{
					"id":          strings.TrimSpace(*localPublishTaskID),
					"title":       valueOrEmpty(title),
					"platform":    platform,
					"accountName": accountName,
					"status":      taskStatus,
					"runAt":       timePtrValue(runAt),
					"createdAt":   taskCreatedAt.Format(time.RFC3339),
				}
			}
		}
		return mustJSONBytes(snapshot), nil
	case "publish_task":
		var title *string
		var platform string
		var accountName string
		var status string
		var runAt *time.Time
		var createdAt time.Time
		err := tx.QueryRow(ctx, `
			SELECT title, platform, account_name, status, run_at, created_at
			FROM publish_tasks
			WHERE id = $1
		`, trimmedID).Scan(&title, &platform, &accountName, &status, &runAt, &createdAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				break
			}
			return nil, err
		}
		return mustJSONBytes(map[string]any{
			"type":        "publish_task",
			"id":          trimmedID,
			"title":       valueOrEmpty(title),
			"platform":    platform,
			"accountName": accountName,
			"status":      status,
			"runAt":       timePtrValue(runAt),
			"createdAt":   createdAt.Format(time.RFC3339),
		}), nil
	}

	return mustJSONBytes(map[string]any{
		"type": trimmedType,
		"id":   trimmedID,
	}), nil
}

// 处理时间Ptr值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func timePtrValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) applyDistributionCommissionReleaseTx(ctx context.Context, tx pgx.Tx, input distributionCommissionReleaseInput) error {
	if strings.TrimSpace(input.CommissionItemID) == "" || input.ConsumedCreditsDelta <= 0 {
		return nil
	}

	item, err := getDistributionCommissionItemByIDTx(ctx, tx, input.CommissionItemID)
	if err != nil {
		return err
	}
	if item == nil {
		return nil
	}

	now := time.Now().UTC()
	state := commissionReleaseState{
		TotalGrantedCredits: item.TotalGrantedCredits,
		ConsumedCredits:     item.ConsumedCredits,
		AmountCents:         item.AmountCents,
		ReleasedAmountCents: item.ReleasedAmountCents,
		SettledAmountCents:  item.SettledAmountCents,
		Status:              item.Status,
		ReleasedAt:          item.ReleasedAt,
	}
	nextState, consumedCredits := advanceCommissionReleaseState(state, input.ConsumedCreditsDelta, now)
	if consumedCredits <= 0 {
		return nil
	}

	releasedDelta := nextState.ReleasedAmountCents - state.ReleasedAmountCents
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
	`, item.ID, nextState.Status, nextState.ConsumedCredits, nextState.ReleasedAmountCents, nextState.ReleasedAt, strings.TrimSpace(input.SourceType), strings.TrimSpace(input.SourceID)); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO distribution_commission_release_events (
			id, commission_item_id, promoter_user_id, invitee_user_id, recharge_order_id,
			source_type, source_id, source_snapshot, wallet_lot_id, quota_account_id,
			wallet_ledger_id, quota_ledger_id, consumed_credits_delta, released_amount_delta_cents,
			commission_item_consumed_credits, commission_item_released_amount_cents, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, uuid.NewString(), item.ID, item.PromoterUserID, item.InviteeUserID, item.RechargeOrderID, strings.TrimSpace(input.SourceType),
		nullableString(strings.TrimSpace(input.SourceID)), bytesOrNil(input.SourceSnapshot), input.WalletLotID, input.QuotaAccountID,
		input.WalletLedgerID, input.QuotaLedgerID, consumedCredits, releasedDelta, nextState.ConsumedCredits, nextState.ReleasedAmountCents, bytesOrNil(input.Metadata)); err != nil {
		return err
	}

	return nil
}

// 处理backfill分销GrantTracking事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) backfillDistributionGrantTrackingTx(ctx context.Context, tx pgx.Tx, inviteeUserID string) error {
	if strings.TrimSpace(inviteeUserID) == "" {
		return nil
	}

	rows, err := tx.Query(ctx, `
		SELECT
			id,
			recharge_order_id,
			consumed_credits,
			total_granted_credits
		FROM distribution_commission_items
		WHERE invitee_user_id = $1
		  AND status IN ('pending_consume', 'pending_settlement')
		ORDER BY created_at ASC
	`, strings.TrimSpace(inviteeUserID))
	if err != nil {
		return err
	}
	defer rows.Close()

	type candidate struct {
		ID                  string
		RechargeOrderID     string
		ConsumedCredits     int64
		TotalGrantedCredits int64
	}

	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if scanErr := rows.Scan(&item.ID, &item.RechargeOrderID, &item.ConsumedCredits, &item.TotalGrantedCredits); scanErr != nil {
			return scanErr
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, candidate := range candidates {
		order, err := s.getRechargeOrderByIDAnyUserTx(ctx, tx, candidate.RechargeOrderID)
		if err != nil || order == nil {
			if err != nil {
				return err
			}
			continue
		}

		snapshot, err := s.distributionGrantSnapshotForRechargeOrderTx(ctx, tx, order)
		if err != nil {
			return err
		}

		quotaRows, err := tx.Query(ctx, `
			SELECT id, meter_code, used_total, distribution_commission_item_id, release_unit_credits
			FROM billing_quota_accounts
			WHERE user_id = $1
			  AND COALESCE(recharge_order_id, source_id, '') = $2
			  AND (source_type = 'support_recharge' OR recharge_order_id IS NOT NULL)
			FOR UPDATE
		`, inviteeUserID, candidate.RechargeOrderID)
		if err != nil {
			return err
		}

		type quotaAccountCandidate struct {
			AccountID          string
			MeterCode          string
			UsedTotal          int64
			CommissionItemID   *string
			ReleaseUnitCredits int64
		}

		quotaAccounts := make([]quotaAccountCandidate, 0)
		for quotaRows.Next() {
			var quotaAccount quotaAccountCandidate
			if scanErr := quotaRows.Scan(
				&quotaAccount.AccountID,
				&quotaAccount.MeterCode,
				&quotaAccount.UsedTotal,
				&quotaAccount.CommissionItemID,
				&quotaAccount.ReleaseUnitCredits,
			); scanErr != nil {
				quotaRows.Close()
				return scanErr
			}
			quotaAccounts = append(quotaAccounts, quotaAccount)
		}
		if err := quotaRows.Err(); err != nil {
			quotaRows.Close()
			return err
		}
		quotaRows.Close()

		var quotaConsumedCredits int64
		for _, quotaAccount := range quotaAccounts {
			snapshotUnitCredits := snapshot.QuotaUnitCredits[strings.TrimSpace(quotaAccount.MeterCode)]
			if snapshotUnitCredits < 0 {
				snapshotUnitCredits = 0
			}
			releaseUnitCredits := quotaAccount.ReleaseUnitCredits
			if quotaAccount.CommissionItemID == nil || strings.TrimSpace(*quotaAccount.CommissionItemID) == "" || releaseUnitCredits <= 0 || strings.TrimSpace(candidate.RechargeOrderID) == "" {
				if _, err := tx.Exec(ctx, `
					UPDATE billing_quota_accounts
					SET recharge_order_id = COALESCE(NULLIF(recharge_order_id, ''), $2),
					    distribution_commission_item_id = COALESCE(NULLIF(distribution_commission_item_id, ''), $3),
					    release_unit_credits = CASE
					        WHEN release_unit_credits <= 0 THEN $4
					        ELSE release_unit_credits
					    END,
					    updated_at = NOW()
					WHERE id = $1
				`, quotaAccount.AccountID, candidate.RechargeOrderID, candidate.ID, snapshotUnitCredits); err != nil {
					return err
				}
				if releaseUnitCredits <= 0 {
					releaseUnitCredits = snapshotUnitCredits
				}
			}
			quotaConsumedCredits += quotaAccount.UsedTotal * maxInt64(releaseUnitCredits, 0)
		}

		var existingLotCount int64
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)::BIGINT
			FROM billing_wallet_lots
			WHERE user_id = $1
			  AND recharge_order_id = $2
			  AND distribution_commission_item_id = $3
		`, inviteeUserID, candidate.RechargeOrderID, candidate.ID).Scan(&existingLotCount); err != nil {
			return err
		}
		if existingLotCount > 0 || snapshot.WalletGrantCredits <= 0 {
			continue
		}

		walletConsumedCredits := candidate.ConsumedCredits - quotaConsumedCredits
		walletConsumedCredits = minInt64(maxInt64(walletConsumedCredits, 0), snapshot.WalletGrantCredits)
		remainingCredits := snapshot.WalletGrantCredits - walletConsumedCredits
		status := "active"
		if remainingCredits <= 0 {
			status = "consumed"
			remainingCredits = 0
		}

		metadata := mustJSONBytes(map[string]any{
			"backfilled": true,
			"orderId":    order.ID,
			"orderNo":    order.OrderNo,
		})
		if _, err := tx.Exec(ctx, `
			INSERT INTO billing_wallet_lots (
				id, user_id, recharge_order_id, distribution_commission_item_id, source_type, source_id,
				granted_credits, consumed_credits, remaining_credits, release_unit_credits, status, metadata
			)
			VALUES ($1, $2, $3, $4, 'support_recharge', $3, $5, $6, $7, 1, $8, $9)
		`, uuid.NewString(), inviteeUserID, candidate.RechargeOrderID, candidate.ID, snapshot.WalletGrantCredits, walletConsumedCredits, remainingCredits, status, metadata); err != nil {
			return err
		}
	}

	return nil
}

// 处理findApplicable分销规则事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 根据充值订单事务计算确保分销Commission，供存储层的状态判定和查询逻辑复用。
func (s *Store) ensureDistributionCommissionForRechargeOrderTx(ctx context.Context, tx pgx.Tx, order *domain.RechargeOrder) (*distributionCommissionItemRecord, distributionGrantSnapshot, error) {
	snapshot := distributionGrantSnapshot{QuotaUnitCredits: make(map[string]int64)}
	if order == nil || strings.TrimSpace(order.UserID) == "" || strings.TrimSpace(order.ID) == "" {
		return nil, snapshot, nil
	}

	snapshotValue, err := s.distributionGrantSnapshotForRechargeOrderTx(ctx, tx, order)
	if err != nil {
		return nil, snapshot, err
	}
	snapshot = snapshotValue

	existing, err := getDistributionCommissionItemByRechargeOrderTx(ctx, tx, order.ID)
	if err != nil {
		return nil, snapshot, err
	}
	if existing != nil {
		return existing, snapshot, nil
	}

	referral, err := getDistributionReferralByInviteeUserIDTx(ctx, tx, order.UserID)
	if err != nil {
		return nil, snapshot, err
	}
	if referral == nil || referral.Status != "active" {
		return nil, snapshot, nil
	}

	rule, err := findApplicableDistributionRuleTx(ctx, tx, referral.PromoterUserID)
	if err != nil {
		return nil, snapshot, err
	}
	if rule == nil {
		return nil, snapshot, nil
	}

	totalGrantedCredits := snapshot.TotalGrantedCredits
	if totalGrantedCredits <= 0 {
		return nil, snapshot, nil
	}
	commissionAmount := calculateCommissionAmountCents(order.AmountCents, rule.CommissionRateBasisPoint)
	if commissionAmount <= 0 {
		return nil, snapshot, nil
	}

	metadata, _ := json.Marshal(map[string]any{
		"orderId":            order.ID,
		"orderNo":            order.OrderNo,
		"channel":            order.Channel,
		"amountCents":        order.AmountCents,
		"creditAmount":       order.CreditAmount,
		"bonusCreditAmount":  order.ManualBonusCreditAmount,
		"grantCredits":       totalGrantedCredits,
		"walletGrantCredits": snapshot.WalletGrantCredits,
		"quotaUnitCredits":   snapshot.QuotaUnitCredits,
	})
	itemID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO distribution_commission_items (
			id, referral_id, rule_id, promoter_user_id, invitee_user_id, recharge_order_id, status,
			commission_rate_basis_points, settlement_threshold_cents, commission_base_amount_cents,
			amount_cents, total_granted_credits, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending_consume', $7, $8, $9, $10, $11, $12)
	`, itemID, referral.ID, rule.ID, referral.PromoterUserID, referral.InviteeUserID, order.ID, rule.CommissionRateBasisPoint, rule.SettlementThresholdCents, order.AmountCents, commissionAmount, totalGrantedCredits, metadata); err != nil {
		return nil, snapshot, err
	}

	item, err := getDistributionCommissionItemByIDTx(ctx, tx, itemID)
	if err != nil {
		return nil, snapshot, err
	}
	return item, snapshot, nil
}

// 根据用量事务计算释放分销Commission，供存储层的状态判定和查询逻辑复用。
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

	type commissionReleaseCandidate struct {
		ItemID string
		State  commissionReleaseState
	}

	releaseCandidates := make([]commissionReleaseCandidate, 0)
	for rows.Next() {
		var candidate commissionReleaseCandidate
		if scanErr := rows.Scan(
			&candidate.ItemID,
			&candidate.State.Status,
			&candidate.State.AmountCents,
			&candidate.State.TotalGrantedCredits,
			&candidate.State.ConsumedCredits,
			&candidate.State.ReleasedAmountCents,
			&candidate.State.SettledAmountCents,
			&candidate.State.ReleasedAt,
		); scanErr != nil {
			return scanErr
		}
		releaseCandidates = append(releaseCandidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	remainingCredits := debitedCredits
	now := time.Now().UTC()
	trimmedSourceType := strings.TrimSpace(sourceType)
	trimmedSourceID := strings.TrimSpace(sourceID)
	for _, candidate := range releaseCandidates {
		if remainingCredits <= 0 {
			break
		}
		nextState, consumedCredits := advanceCommissionReleaseState(candidate.State, remainingCredits, now)
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
		`, candidate.ItemID, nextState.Status, nextState.ConsumedCredits, nextState.ReleasedAmountCents, nextState.ReleasedAt, trimmedSourceType, trimmedSourceID); err != nil {
			return err
		}
	}
	return nil
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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
			c.total_granted_credits,
			c.consumed_credits,
			c.released_amount_cents,
			c.settled_amount_cents,
			COALESCE(rel.release_event_count, 0)::BIGINT,
			c.recharge_order_id,
			ro.order_no,
			c.created_at,
			c.released_at,
			c.settled_at
		FROM distribution_commission_items c
		INNER JOIN users pu ON pu.id = c.promoter_user_id
		INNER JOIN users iu ON iu.id = c.invitee_user_id
		LEFT JOIN recharge_orders ro ON ro.id = c.recharge_order_id
		LEFT JOIN LATERAL (
			SELECT COUNT(*)::BIGINT AS release_event_count
			FROM distribution_commission_release_events e
			WHERE e.commission_item_id = c.id
		) rel ON TRUE
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
		var promoter scannedAdminUserSummary
		var invitee scannedAdminUserSummary
		if scanErr := rows.Scan(
			&item.ID,
			&promoter.ID,
			&promoter.Email,
			&promoter.Name,
			&invitee.ID,
			&invitee.Email,
			&invitee.Name,
			&item.Status,
			&basisPoints,
			&item.CommissionBaseAmountCents,
			&item.AmountCents,
			&item.TotalGrantedCredits,
			&item.ConsumedCredits,
			&item.ReleasedAmountCents,
			&item.SettledAmountCents,
			&item.ReleaseEventCount,
			&item.RechargeOrderID,
			&item.RechargeOrderNo,
			&item.CreatedAt,
			&item.ReleasedAt,
			&item.SettledAt,
		); scanErr != nil {
			return nil, 0, domain.AdminCommissionListSummary{}, scanErr
		}
		item.Promoter = promoter.summary()
		item.Invitee = invitee.summary()
		item.CommissionRate = basisPointsToRate(basisPoints)
		items = append(items, item)
	}
	return items, total, summary, rows.Err()
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListAdminCommissionReleaseEvents(ctx context.Context, commissionItemID string, limit int) ([]domain.CommissionReleaseEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			e.id,
			e.commission_item_id,
			e.recharge_order_id,
			ro.order_no,
			e.source_type,
			e.source_id,
			e.source_snapshot,
			e.wallet_lot_id,
			e.quota_account_id,
			e.wallet_ledger_id,
			e.quota_ledger_id,
			e.consumed_credits_delta,
			e.released_amount_delta_cents,
			e.commission_item_consumed_credits,
			e.commission_item_released_amount_cents,
			e.metadata,
			e.created_at
		FROM distribution_commission_release_events e
		LEFT JOIN recharge_orders ro ON ro.id = e.recharge_order_id
		WHERE e.commission_item_id = $1
		ORDER BY e.created_at DESC
		LIMIT $2
	`, strings.TrimSpace(commissionItemID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.CommissionReleaseEvent, 0, limit)
	for rows.Next() {
		var item domain.CommissionReleaseEvent
		if scanErr := rows.Scan(
			&item.ID,
			&item.CommissionItemID,
			&item.RechargeOrderID,
			&item.RechargeOrderNo,
			&item.SourceType,
			&item.SourceID,
			&item.SourceSnapshot,
			&item.WalletLotID,
			&item.QuotaAccountID,
			&item.WalletLedgerID,
			&item.QuotaLedgerID,
			&item.ConsumedCreditsDelta,
			&item.ReleasedAmountDeltaCents,
			&item.CommissionItemConsumedCredits,
			&item.CommissionItemReleasedAmountCents,
			&item.Metadata,
			&item.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// 处理format分销SettlementBatchNo相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func formatDistributionSettlementBatchNo(now time.Time) string {
	return fmt.Sprintf("SET-%s-%s", now.UTC().Format("20060102150405"), strings.ToUpper(uuid.NewString()[:6]))
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
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

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
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
