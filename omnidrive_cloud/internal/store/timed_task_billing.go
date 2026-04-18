package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func ensureWalletAndLockMillisTx(ctx context.Context, tx pgx.Tx, userID string) (int64, error) {
	if _, err := tx.Exec(ctx, `
		INSERT INTO billing_wallets (user_id, credit_balance, credit_balance_millis, frozen_credit_balance)
		VALUES ($1, 0, 0, 0)
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return 0, err
	}

	var wholeCredits int64
	var fractionalMillis int64
	if err := tx.QueryRow(ctx, `
		SELECT credit_balance, credit_balance_millis
		FROM billing_wallets
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&wholeCredits, &fractionalMillis); err != nil {
		return 0, err
	}
	return CombineWalletBalanceMillis(wholeCredits, fractionalMillis), nil
}

func applyTimedTaskWalletDeltaTx(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	amountDeltaMillis int64,
	quantity int,
	unitPriceCreditMillis int64,
	entryType string,
	description string,
	referenceType string,
	referenceID string,
	meterCode string,
	unit string,
	metadata map[string]any,
) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", fmt.Errorf("user id is required")
	}
	if amountDeltaMillis == 0 {
		return "", nil
	}

	currentBalanceMillis, err := ensureWalletAndLockMillisTx(ctx, tx, userID)
	if err != nil {
		return "", err
	}
	nextBalanceMillis := currentBalanceMillis + amountDeltaMillis
	if nextBalanceMillis < 0 {
		return "", ErrDigitalHumanBillingInsufficientBalance
	}

	currentBalanceWhole, currentBalanceFraction := SplitWalletBalanceMillis(currentBalanceMillis)
	nextBalanceWhole, nextBalanceFraction := SplitWalletBalanceMillis(nextBalanceMillis)
	if _, err := tx.Exec(ctx, `
		UPDATE billing_wallets
		SET credit_balance = $2,
		    credit_balance_millis = $3,
		    updated_at = NOW()
		WHERE user_id = $1
	`, userID, nextBalanceWhole, nextBalanceFraction); err != nil {
		return "", err
	}

	ledgerID := uuid.NewString()
	var quantityValue any
	var unitValue any
	var unitPriceLegacy any
	var unitPriceMillisValue any
	if quantity > 0 {
		quantityValue = quantity
		unitValue = strings.TrimSpace(unit)
		unitPriceLegacy = RoundMillisToWholeCredits(unitPriceCreditMillis)
		unitPriceMillisValue = unitPriceCreditMillis
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["referenceType"] = strings.TrimSpace(referenceType)
	metadata["referenceId"] = strings.TrimSpace(referenceID)
	metadata["amountDelta"] = CreditsFromMillis(amountDeltaMillis)
	metadata["amountDeltaMillis"] = amountDeltaMillis
	metadata["balanceBeforeCredits"] = CreditsFromMillis(currentBalanceMillis)
	metadata["balanceBeforeMillis"] = currentBalanceMillis
	metadata["balanceAfterCredits"] = CreditsFromMillis(nextBalanceMillis)
	metadata["balanceAfterMillis"] = nextBalanceMillis

	legacyAmountDelta := nextBalanceWhole - currentBalanceWhole
	if _, err := tx.Exec(ctx, `
		INSERT INTO wallet_ledgers (
			id, user_id, entry_type, amount_delta, amount_delta_millis, balance_before, balance_before_millis,
			balance_after, balance_after_millis, meter_code, quantity, unit, unit_price_credits, unit_price_credit_millis,
			description, reference_type, reference_id, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`, ledgerID, userID, strings.TrimSpace(entryType), legacyAmountDelta, amountDeltaMillis, currentBalanceWhole, currentBalanceFraction,
		nextBalanceWhole, nextBalanceFraction, strings.TrimSpace(meterCode), quantityValue, unitValue, unitPriceLegacy, unitPriceMillisValue,
		nullableString(strings.TrimSpace(description)), stringPtr(strings.TrimSpace(referenceType)), stringPtr(strings.TrimSpace(referenceID)), mustJSONMap(metadata)); err != nil {
		return "", err
	}

	return ledgerID, nil
}

func positiveInt64FromAny(value any) int64 {
	switch typed := value.(type) {
	case int64:
		if typed > 0 {
			return typed
		}
	case int32:
		if typed > 0 {
			return int64(typed)
		}
	case int:
		if typed > 0 {
			return int64(typed)
		}
	case float64:
		if typed > 0 {
			return int64(typed)
		}
	case float32:
		if typed > 0 {
			return int64(typed)
		}
	case json.Number:
		if parsed, err := typed.Int64(); err == nil && parsed > 0 {
			return parsed
		}
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func positiveCreditMillisFromAny(value any) int64 {
	switch typed := value.(type) {
	case float64:
		if parsed, err := CreditsToMillis(typed); err == nil && parsed > 0 {
			return parsed
		}
	case float32:
		if parsed, err := CreditsToMillis(float64(typed)); err == nil && parsed > 0 {
			return parsed
		}
	case int64:
		if parsed, err := CreditsToMillis(float64(typed)); err == nil && parsed > 0 {
			return parsed
		}
	case int32:
		if parsed, err := CreditsToMillis(float64(typed)); err == nil && parsed > 0 {
			return parsed
		}
	case int:
		if parsed, err := CreditsToMillis(float64(typed)); err == nil && parsed > 0 {
			return parsed
		}
	case json.Number:
		if floatValue, err := typed.Float64(); err == nil {
			if parsed, convertErr := CreditsToMillis(floatValue); convertErr == nil && parsed > 0 {
				return parsed
			}
		}
	case string:
		if floatValue, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
			if parsed, convertErr := CreditsToMillis(floatValue); convertErr == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}
