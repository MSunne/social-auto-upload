package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) GrantWalletCredits(ctx context.Context, input GrantWalletCreditsInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.grantWalletCreditsTx(ctx, tx, input); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Store) GrantQuota(ctx context.Context, input GrantQuotaInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.grantQuotaTx(ctx, tx, input); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Store) grantWalletCreditsTx(ctx context.Context, tx pgx.Tx, input GrantWalletCreditsInput) error {
	if strings.TrimSpace(input.UserID) == "" {
		return fmt.Errorf("user id is required")
	}
	if input.Amount <= 0 {
		return fmt.Errorf("wallet grant amount must be positive")
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO billing_wallets (user_id, credit_balance, frozen_credit_balance)
		VALUES ($1, 0, 0)
		ON CONFLICT (user_id) DO NOTHING
	`, input.UserID); err != nil {
		return err
	}

	var before int64
	if err := tx.QueryRow(ctx, `
		SELECT credit_balance
		FROM billing_wallets
		WHERE user_id = $1
		FOR UPDATE
	`, input.UserID).Scan(&before); err != nil {
		return err
	}

	after := before + input.Amount
	if _, err := tx.Exec(ctx, `
		UPDATE billing_wallets
		SET credit_balance = $2,
		    updated_at = NOW()
		WHERE user_id = $1
	`, input.UserID, after); err != nil {
		return err
	}

	description := input.Description
	if description == nil || strings.TrimSpace(*description) == "" {
		description = stringPtr("运维发放钱包积分")
	}

	entryType := "grant"
	if input.EntryType != nil && strings.TrimSpace(*input.EntryType) != "" {
		entryType = strings.TrimSpace(*input.EntryType)
	}
	meterCode := "wallet_credit"
	quantity := input.Amount
	unit := "credit"

	if _, err := tx.Exec(ctx, `
		INSERT INTO wallet_ledgers (
			id, user_id, entry_type, amount_delta, balance_before, balance_after, meter_code, quantity,
			unit, description, reference_type, reference_id, recharge_order_id, payment_transaction_id, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, uuid.NewString(), input.UserID, entryType, input.Amount, before, after, meterCode, quantity, unit,
		description, input.ReferenceType, input.ReferenceID, input.RechargeOrderID, input.PaymentTransactionID, input.Metadata); err != nil {
		return err
	}

	return nil
}

func (s *Store) grantQuotaTx(ctx context.Context, tx pgx.Tx, input GrantQuotaInput) error {
	if strings.TrimSpace(input.UserID) == "" {
		return fmt.Errorf("user id is required")
	}
	if strings.TrimSpace(input.MeterCode) == "" {
		return fmt.Errorf("meter code is required")
	}
	if input.Amount <= 0 {
		return fmt.Errorf("quota grant amount must be positive")
	}

	accountID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO billing_quota_accounts (
			id, user_id, meter_code, granted_total, used_total, reserved_total, remaining_total,
			expires_at, source_type, source_id, status
		)
		VALUES ($1, $2, $3, $4, 0, 0, $4, $5, $6, $7, 'active')
	`, accountID, input.UserID, strings.TrimSpace(input.MeterCode), input.Amount, input.ExpiresAt, input.SourceType, input.SourceID); err != nil {
		return err
	}

	description := input.Description
	if description == nil || strings.TrimSpace(*description) == "" {
		description = stringPtr("运维发放套餐额度")
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO billing_quota_ledgers (
			id, quota_account_id, user_id, meter_code, amount_delta, remaining_after,
			description, reference_type, reference_id, payload
		)
		VALUES ($1, $2, $3, $4, $5, $5, $6, $7, $8, $9)
	`, uuid.NewString(), accountID, input.UserID, strings.TrimSpace(input.MeterCode), input.Amount,
		description, input.ReferenceType, input.ReferenceID, input.Payload); err != nil {
		return err
	}

	return nil
}

func timePtrUTC(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}
