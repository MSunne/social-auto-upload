package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

var (
	ErrWithdrawalNotFound            = errors.New("withdrawal request not found")
	ErrWithdrawalInvalidTransition   = errors.New("withdrawal request status transition is invalid")
	ErrWithdrawalInsufficientBalance = errors.New("withdrawal amount exceeds settled commission balance")
)

type AdminWithdrawalListFilter struct {
	Query  string
	Status string
	AdminPageFilter
}

type ReviewWithdrawalInput struct {
	AdminID          string
	AdminEmail       string
	AdminName        string
	Note             *string
	PaymentReference *string
	ProofURLs        []string
}

type adminWithdrawalRecord struct {
	ID                 string
	PromoterUserID     string
	PromoterEmail      string
	PromoterName       string
	Status             string
	AmountCents        int64
	PayoutChannel      *string
	AccountMasked      *string
	Note               *string
	ProofURLs          []string
	PaymentReference   *string
	ReviewerAdminID    *string
	ReviewerAdminEmail *string
	ReviewerAdminName  *string
	OperatorAdminID    *string
	OperatorAdminEmail *string
	OperatorAdminName  *string
	CreatedAt          time.Time
	ReviewedAt         *time.Time
	PaidAt             *time.Time
}

func decodeProofURLs(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err != nil || items == nil {
		return []string{}
	}
	return items
}

func normalizeProofURLs(items []string) []string {
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, raw := range items {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func scanAdminWithdrawalRecord(scan scanFn) (*adminWithdrawalRecord, error) {
	var item adminWithdrawalRecord
	var proofURLs []byte
	if err := scan(
		&item.ID,
		&item.PromoterUserID,
		&item.PromoterEmail,
		&item.PromoterName,
		&item.Status,
		&item.AmountCents,
		&item.PayoutChannel,
		&item.AccountMasked,
		&item.Note,
		&proofURLs,
		&item.PaymentReference,
		&item.ReviewerAdminID,
		&item.ReviewerAdminEmail,
		&item.ReviewerAdminName,
		&item.OperatorAdminID,
		&item.OperatorAdminEmail,
		&item.OperatorAdminName,
		&item.CreatedAt,
		&item.ReviewedAt,
		&item.PaidAt,
	); err != nil {
		return nil, err
	}
	item.ProofURLs = decodeProofURLs(proofURLs)
	return &item, nil
}

func buildAdminWithdrawalRow(record *adminWithdrawalRecord) domain.AdminWithdrawalRow {
	return domain.AdminWithdrawalRow{
		ID: record.ID,
		Promoter: domain.AdminUserSummary{
			ID:    record.PromoterUserID,
			Email: record.PromoterEmail,
			Name:  record.PromoterName,
		},
		Status:        record.Status,
		AmountCents:   record.AmountCents,
		PayoutChannel: record.PayoutChannel,
		AccountMasked: record.AccountMasked,
		RequestedAt:   record.CreatedAt,
		ReviewedAt:    record.ReviewedAt,
		PaidAt:        record.PaidAt,
	}
}

func buildAdminWithdrawalDetail(record *adminWithdrawalRecord, availableAmountCents int64) domain.AdminWithdrawalDetail {
	detail := domain.AdminWithdrawalDetail{
		Record:               buildAdminWithdrawalRow(record),
		AvailableAmountCents: availableAmountCents,
		Note:                 record.Note,
		ProofURLs:            record.ProofURLs,
		PaymentReference:     record.PaymentReference,
	}
	if record.ReviewerAdminID != nil || record.ReviewerAdminEmail != nil || record.ReviewerAdminName != nil {
		detail.Reviewer = &domain.AdminSummary{
			ID:    valueOrEmpty(record.ReviewerAdminID),
			Email: valueOrEmpty(record.ReviewerAdminEmail),
			Name:  valueOrEmpty(record.ReviewerAdminName),
		}
	}
	if record.OperatorAdminID != nil || record.OperatorAdminEmail != nil || record.OperatorAdminName != nil {
		detail.Operator = &domain.AdminSummary{
			ID:    valueOrEmpty(record.OperatorAdminID),
			Email: valueOrEmpty(record.OperatorAdminEmail),
			Name:  valueOrEmpty(record.OperatorAdminName),
		}
	}
	return detail
}

func getAdminWithdrawalByIDTx(ctx context.Context, tx pgx.Tx, withdrawalID string) (*adminWithdrawalRecord, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			w.id,
			u.id,
			u.email,
			u.name,
			w.status,
			w.amount_cents,
			w.payout_channel,
			w.account_masked,
			w.note,
			w.proof_urls,
			w.payment_reference,
			w.reviewer_admin_user_id,
			w.reviewer_admin_email,
			w.reviewer_admin_name,
			w.operator_admin_user_id,
			w.operator_admin_email,
			w.operator_admin_name,
			w.created_at,
			w.reviewed_at,
			w.paid_at
		FROM withdrawal_requests w
		INNER JOIN users u ON u.id = w.promoter_user_id
		WHERE w.id = $1
		FOR UPDATE
	`, withdrawalID)
	item, err := scanAdminWithdrawalRecord(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) GetAdminWithdrawalByID(ctx context.Context, withdrawalID string) (*domain.AdminWithdrawalDetail, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			w.id,
			u.id,
			u.email,
			u.name,
			w.status,
			w.amount_cents,
			w.payout_channel,
			w.account_masked,
			w.note,
			w.proof_urls,
			w.payment_reference,
			w.reviewer_admin_user_id,
			w.reviewer_admin_email,
			w.reviewer_admin_name,
			w.operator_admin_user_id,
			w.operator_admin_email,
			w.operator_admin_name,
			w.created_at,
			w.reviewed_at,
			w.paid_at
		FROM withdrawal_requests w
		INNER JOIN users u ON u.id = w.promoter_user_id
		WHERE w.id = $1
	`, withdrawalID)
	record, err := scanAdminWithdrawalRecord(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	availableAmountCents, err := s.computePromoterAvailableWithdrawalAmount(ctx, record.PromoterUserID, record.ID)
	if err != nil {
		return nil, err
	}
	detail := buildAdminWithdrawalDetail(record, availableAmountCents)
	return &detail, nil
}

func (s *Store) computePromoterAvailableWithdrawalAmount(ctx context.Context, promoterUserID string, excludeWithdrawalID string) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	amount, err := computePromoterAvailableWithdrawalAmountTx(ctx, tx, promoterUserID, excludeWithdrawalID)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return amount, nil
}

func computePromoterAvailableWithdrawalAmountTx(ctx context.Context, tx pgx.Tx, promoterUserID string, excludeWithdrawalID string) (int64, error) {
	var settledAmountCents int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(settled_amount_cents), 0)::BIGINT
		FROM distribution_commission_items
		WHERE promoter_user_id = $1
	`, promoterUserID).Scan(&settledAmountCents); err != nil {
		return 0, err
	}

	args := []any{promoterUserID}
	whereExclude := ""
	if strings.TrimSpace(excludeWithdrawalID) != "" {
		whereExclude = " AND id <> $2"
		args = append(args, strings.TrimSpace(excludeWithdrawalID))
	}

	var reservedAmountCents int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(amount_cents), 0)::BIGINT
		FROM withdrawal_requests
		WHERE promoter_user_id = $1
		  AND status IN ('requested', 'approved', 'paid')
		  %s
	`, whereExclude), args...).Scan(&reservedAmountCents); err != nil {
		return 0, err
	}

	availableAmountCents := settledAmountCents - reservedAmountCents
	if availableAmountCents < 0 {
		return 0, nil
	}
	return availableAmountCents, nil
}

func (s *Store) ListAdminWithdrawals(ctx context.Context, filter AdminWithdrawalListFilter) ([]domain.AdminWithdrawalRow, int64, domain.AdminWithdrawalListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf(`(
			w.id ILIKE $%d OR
			u.id ILIKE $%d OR u.email ILIKE $%d OR u.name ILIKE $%d OR
			COALESCE(w.account_masked, '') ILIKE $%d OR
			COALESCE(w.payment_reference, '') ILIKE $%d
		)`, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}

	switch strings.TrimSpace(filter.Status) {
	case "requested", "approved", "rejected", "paid":
		whereParts = append(whereParts, fmt.Sprintf("w.status = $%d", argIndex))
		args = append(args, strings.TrimSpace(filter.Status))
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM withdrawal_requests w
		INNER JOIN users u ON u.id = w.promoter_user_id
		%s
	`, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, domain.AdminWithdrawalListSummary{}, err
	}

	var summary domain.AdminWithdrawalListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE w.status = 'requested')::BIGINT,
			COUNT(*) FILTER (WHERE w.status = 'approved')::BIGINT,
			COUNT(*) FILTER (WHERE w.status = 'rejected')::BIGINT,
			COUNT(*) FILTER (WHERE w.status = 'paid')::BIGINT,
			COALESCE(SUM(CASE WHEN w.status IN ('requested', 'approved') THEN w.amount_cents ELSE 0 END), 0)::BIGINT,
			COALESCE(SUM(CASE WHEN w.status = 'paid' THEN w.amount_cents ELSE 0 END), 0)::BIGINT
		FROM withdrawal_requests w
		INNER JOIN users u ON u.id = w.promoter_user_id
		%s
	`, whereClause), args...).Scan(
		&summary.RequestedCount,
		&summary.ApprovedCount,
		&summary.RejectedCount,
		&summary.PaidCount,
		&summary.PendingWithdrawalAmountCts,
		&summary.PaidWithdrawalAmountCents,
	); err != nil {
		return nil, 0, domain.AdminWithdrawalListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			w.id,
			u.id,
			u.email,
			u.name,
			w.status,
			w.amount_cents,
			w.payout_channel,
			w.account_masked,
			w.note,
			w.proof_urls,
			w.payment_reference,
			w.reviewer_admin_user_id,
			w.reviewer_admin_email,
			w.reviewer_admin_name,
			w.operator_admin_user_id,
			w.operator_admin_email,
			w.operator_admin_name,
			w.created_at,
			w.reviewed_at,
			w.paid_at
		FROM withdrawal_requests w
		INNER JOIN users u ON u.id = w.promoter_user_id
		%s
		ORDER BY w.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminWithdrawalListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminWithdrawalRow, 0)
	for rows.Next() {
		record, scanErr := scanAdminWithdrawalRecord(rows.Scan)
		if scanErr != nil {
			return nil, 0, domain.AdminWithdrawalListSummary{}, scanErr
		}
		items = append(items, buildAdminWithdrawalRow(record))
	}
	return items, total, summary, rows.Err()
}

func (s *Store) ApproveWithdrawal(ctx context.Context, withdrawalID string, input ReviewWithdrawalInput) (*domain.AdminWithdrawalDetail, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	record, err := getAdminWithdrawalByIDTx(ctx, tx, withdrawalID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrWithdrawalNotFound
	}
	if record.Status == "approved" {
		availableAmountCents, amountErr := computePromoterAvailableWithdrawalAmountTx(ctx, tx, record.PromoterUserID, record.ID)
		if amountErr != nil {
			return nil, amountErr
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		detail := buildAdminWithdrawalDetail(record, availableAmountCents)
		return &detail, nil
	}
	if record.Status != "requested" {
		return nil, ErrWithdrawalInvalidTransition
	}

	availableAmountCents, err := computePromoterAvailableWithdrawalAmountTx(ctx, tx, record.PromoterUserID, record.ID)
	if err != nil {
		return nil, err
	}
	if availableAmountCents < record.AmountCents {
		return nil, ErrWithdrawalInsufficientBalance
	}

	now := time.Now().UTC()
	trimmedNote := trimOptionalString(input.Note)
	if _, err := tx.Exec(ctx, `
		UPDATE withdrawal_requests
		SET status = 'approved',
		    note = COALESCE($2, note),
		    reviewer_admin_user_id = $3,
		    reviewer_admin_email = $4,
		    reviewer_admin_name = $5,
		    reviewed_at = $6,
		    updated_at = NOW()
		WHERE id = $1
	`, withdrawalID, trimmedNote, strings.TrimSpace(input.AdminID), strings.TrimSpace(input.AdminEmail), strings.TrimSpace(input.AdminName), now); err != nil {
		return nil, err
	}

	record, err = getAdminWithdrawalByIDTx(ctx, tx, withdrawalID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	detail := buildAdminWithdrawalDetail(record, availableAmountCents)
	return &detail, nil
}

func (s *Store) RejectWithdrawal(ctx context.Context, withdrawalID string, input ReviewWithdrawalInput) (*domain.AdminWithdrawalDetail, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	record, err := getAdminWithdrawalByIDTx(ctx, tx, withdrawalID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrWithdrawalNotFound
	}
	if record.Status == "rejected" {
		availableAmountCents, amountErr := computePromoterAvailableWithdrawalAmountTx(ctx, tx, record.PromoterUserID, record.ID)
		if amountErr != nil {
			return nil, amountErr
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		detail := buildAdminWithdrawalDetail(record, availableAmountCents)
		return &detail, nil
	}
	if record.Status != "requested" && record.Status != "approved" {
		return nil, ErrWithdrawalInvalidTransition
	}

	now := time.Now().UTC()
	trimmedNote := trimOptionalString(input.Note)
	if _, err := tx.Exec(ctx, `
		UPDATE withdrawal_requests
		SET status = 'rejected',
		    note = COALESCE($2, note),
		    reviewer_admin_user_id = $3,
		    reviewer_admin_email = $4,
		    reviewer_admin_name = $5,
		    reviewed_at = $6,
		    updated_at = NOW()
		WHERE id = $1
	`, withdrawalID, trimmedNote, strings.TrimSpace(input.AdminID), strings.TrimSpace(input.AdminEmail), strings.TrimSpace(input.AdminName), now); err != nil {
		return nil, err
	}

	record, err = getAdminWithdrawalByIDTx(ctx, tx, withdrawalID)
	if err != nil {
		return nil, err
	}
	availableAmountCents, err := computePromoterAvailableWithdrawalAmountTx(ctx, tx, record.PromoterUserID, record.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	detail := buildAdminWithdrawalDetail(record, availableAmountCents)
	return &detail, nil
}

func (s *Store) MarkWithdrawalPaid(ctx context.Context, withdrawalID string, input ReviewWithdrawalInput) (*domain.AdminWithdrawalDetail, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	record, err := getAdminWithdrawalByIDTx(ctx, tx, withdrawalID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrWithdrawalNotFound
	}
	if record.Status == "paid" {
		availableAmountCents, amountErr := computePromoterAvailableWithdrawalAmountTx(ctx, tx, record.PromoterUserID, record.ID)
		if amountErr != nil {
			return nil, amountErr
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		detail := buildAdminWithdrawalDetail(record, availableAmountCents)
		return &detail, nil
	}
	if record.Status != "approved" {
		return nil, ErrWithdrawalInvalidTransition
	}

	now := time.Now().UTC()
	proofURLs := normalizeProofURLs(input.ProofURLs)
	if len(proofURLs) == 0 {
		proofURLs = record.ProofURLs
	}
	proofPayload, _ := json.Marshal(proofURLs)
	trimmedNote := trimOptionalString(input.Note)
	paymentReference := trimOptionalString(input.PaymentReference)
	if _, err := tx.Exec(ctx, `
		UPDATE withdrawal_requests
		SET status = 'paid',
		    note = COALESCE($2, note),
		    proof_urls = $3,
		    payment_reference = COALESCE($4, payment_reference),
		    operator_admin_user_id = $5,
		    operator_admin_email = $6,
		    operator_admin_name = $7,
		    paid_at = $8,
		    updated_at = NOW()
		WHERE id = $1
	`, withdrawalID, trimmedNote, proofPayload, paymentReference, strings.TrimSpace(input.AdminID), strings.TrimSpace(input.AdminEmail), strings.TrimSpace(input.AdminName), now); err != nil {
		return nil, err
	}

	record, err = getAdminWithdrawalByIDTx(ctx, tx, withdrawalID)
	if err != nil {
		return nil, err
	}
	availableAmountCents, err := computePromoterAvailableWithdrawalAmountTx(ctx, tx, record.PromoterUserID, record.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	detail := buildAdminWithdrawalDetail(record, availableAmountCents)
	return &detail, nil
}
