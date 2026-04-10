package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

const (
	digitalHumanBillingReferenceType = "digital_human_task"
	digitalHumanBillingMeterCode     = "digital_human_video_seconds"
	digitalHumanBillingUnit          = "second"
)

var (
	ErrDigitalHumanTaskNotFound               = errors.New("digital human task not found")
	ErrDigitalHumanBillingInsufficientBalance = errors.New("digital human wallet balance insufficient")
)

func (s *Store) DeleteDigitalHumanTask(ctx context.Context, taskID string, ownerUserID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM digital_human_tasks
		WHERE id = $1
		  AND owner_user_id = $2
		  AND status = 'queued'
		  AND remote_task_id IS NULL
	`, strings.TrimSpace(taskID), strings.TrimSpace(ownerUserID))
	return err
}

func (s *Store) PrechargeDigitalHumanTask(ctx context.Context, taskID string) (*domain.DigitalHumanTask, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	task, err := getDigitalHumanTaskForUpdateTx(ctx, tx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrDigitalHumanTaskNotFound
	}
	if task.BillingStatus == "precharged" || task.BillingStatus == "settled" || task.BillingStatus == "refunded" {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return task, nil
	}

	payload := decodeDigitalHumanBillingPayload(task.BillingPayload)
	if task.EstimatedCreditsMillis > 0 {
		creditsPerSecondMillis := digitalHumanCreditsPerSecondMillis(task, payload)
		ledgerID, applyErr := applyDigitalHumanWalletDeltaTx(ctx, tx, task.OwnerUserID, -task.EstimatedCreditsMillis, task.EstimatedDurationSeconds, creditsPerSecondMillis, "usage_debit", "数字人视频预扣积分", task.ID, map[string]any{
			"stage":                    "precharge",
			"estimatedDurationSeconds": task.EstimatedDurationSeconds,
			"estimatedCredits":         task.EstimatedCredits,
			"estimatedCreditsMillis":   task.EstimatedCreditsMillis,
			"goodsTitle":               valueOrEmpty(task.GoodsTitle),
			"mode":                     task.Mode,
		})
		if applyErr != nil {
			return nil, applyErr
		}
		payload["prechargeWalletLedgerId"] = ledgerID
	}
	payload["billingMessage"] = "数字人任务已预扣预计积分"
	updated, err := updateDigitalHumanTaskBillingTx(ctx, tx, task.ID, "precharged", nil, nil, payload)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Store) RefundDigitalHumanTaskOnFailure(ctx context.Context, taskID string, failureMessage string) (*domain.DigitalHumanTask, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	task, err := getDigitalHumanTaskForUpdateTx(ctx, tx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrDigitalHumanTaskNotFound
	}
	if task.BillingStatus == "refunded" {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return task, nil
	}

	payload := decodeDigitalHumanBillingPayload(task.BillingPayload)
	if task.EstimatedCreditsMillis > 0 && task.BillingStatus == "precharged" {
		creditsPerSecondMillis := digitalHumanCreditsPerSecondMillis(task, payload)
		ledgerID, applyErr := applyDigitalHumanWalletDeltaTx(ctx, tx, task.OwnerUserID, task.EstimatedCreditsMillis, task.EstimatedDurationSeconds, creditsPerSecondMillis, "usage_return", "数字人视频失败退回积分", task.ID, map[string]any{
			"stage":                    "failure_refund",
			"estimatedDurationSeconds": task.EstimatedDurationSeconds,
			"estimatedCredits":         task.EstimatedCredits,
			"estimatedCreditsMillis":   task.EstimatedCreditsMillis,
			"failureReason":            strings.TrimSpace(failureMessage),
			"goodsTitle":               valueOrEmpty(task.GoodsTitle),
			"mode":                     task.Mode,
		})
		if applyErr != nil {
			return nil, applyErr
		}
		payload["refundWalletLedgerId"] = ledgerID
	}
	payload["billingMessage"] = firstNonEmptyValue(strings.TrimSpace(failureMessage), "数字人任务失败，已退回预扣积分")
	zeroCreditsMillis := int64(0)
	updated, err := updateDigitalHumanTaskBillingTx(ctx, tx, task.ID, "refunded", nil, &zeroCreditsMillis, payload)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Store) SettleDigitalHumanTask(ctx context.Context, taskID string, actualDurationSeconds int) (*domain.DigitalHumanTask, error) {
	if actualDurationSeconds <= 0 {
		return nil, fmt.Errorf("actual duration must be positive")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	task, err := getDigitalHumanTaskForUpdateTx(ctx, tx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrDigitalHumanTaskNotFound
	}

	payload := decodeDigitalHumanBillingPayload(task.BillingPayload)
	creditsPerSecondMillis := digitalHumanCreditsPerSecondMillis(task, payload)
	finalCreditsMillis := int64(actualDurationSeconds) * creditsPerSecondMillis
	payload["billingMessage"] = "数字人任务已按实际时长完成结算"

	deltaCreditsMillis := finalCreditsMillis - task.EstimatedCreditsMillis
	switch {
	case deltaCreditsMillis > 0:
		deltaSeconds := maxInt64(int64(actualDurationSeconds-task.EstimatedDurationSeconds), 0)
		ledgerID, applyErr := applyDigitalHumanWalletDeltaTx(ctx, tx, task.OwnerUserID, -deltaCreditsMillis, int(deltaSeconds), creditsPerSecondMillis, "usage_debit", "数字人视频补扣积分", task.ID, map[string]any{
			"stage":                    "settlement_debit",
			"estimatedCredits":         task.EstimatedCredits,
			"estimatedCreditsMillis":   task.EstimatedCreditsMillis,
			"finalCredits":             DigitalHumanCreditsFromMillis(finalCreditsMillis),
			"finalCreditsMillis":       finalCreditsMillis,
			"actualDurationSeconds":    actualDurationSeconds,
			"estimatedDurationSeconds": task.EstimatedDurationSeconds,
			"goodsTitle":               valueOrEmpty(task.GoodsTitle),
			"mode":                     task.Mode,
		})
		if applyErr != nil {
			return nil, applyErr
		}
		payload["settlementWalletLedgerId"] = ledgerID
		payload["billingMessage"] = "数字人任务已按实际时长补扣积分"
		updated, updateErr := updateDigitalHumanTaskBillingTx(ctx, tx, task.ID, "settled", &actualDurationSeconds, &finalCreditsMillis, payload)
		if updateErr != nil {
			return nil, updateErr
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return updated, nil
	case deltaCreditsMillis < 0:
		refundSeconds := maxInt64(int64(task.EstimatedDurationSeconds-actualDurationSeconds), 0)
		ledgerID, applyErr := applyDigitalHumanWalletDeltaTx(ctx, tx, task.OwnerUserID, -deltaCreditsMillis, int(refundSeconds), creditsPerSecondMillis, "usage_return", "数字人视频退回差额积分", task.ID, map[string]any{
			"stage":                    "settlement_refund",
			"estimatedCredits":         task.EstimatedCredits,
			"estimatedCreditsMillis":   task.EstimatedCreditsMillis,
			"finalCredits":             DigitalHumanCreditsFromMillis(finalCreditsMillis),
			"finalCreditsMillis":       finalCreditsMillis,
			"actualDurationSeconds":    actualDurationSeconds,
			"estimatedDurationSeconds": task.EstimatedDurationSeconds,
			"goodsTitle":               valueOrEmpty(task.GoodsTitle),
			"mode":                     task.Mode,
		})
		if applyErr != nil {
			return nil, applyErr
		}
		payload["refundWalletLedgerId"] = ledgerID
		payload["billingMessage"] = "数字人任务已按实际时长退回差额积分"
		updated, updateErr := updateDigitalHumanTaskBillingTx(ctx, tx, task.ID, "refunded", &actualDurationSeconds, &finalCreditsMillis, payload)
		if updateErr != nil {
			return nil, updateErr
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return updated, nil
	default:
		updated, updateErr := updateDigitalHumanTaskBillingTx(ctx, tx, task.ID, "settled", &actualDurationSeconds, &finalCreditsMillis, payload)
		if updateErr != nil {
			return nil, updateErr
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return updated, nil
	}
}

func (s *Store) MarkDigitalHumanTaskSettlementPending(ctx context.Context, taskID string, actualDurationSeconds *int, finalCreditsMillis *int64, message string) (*domain.DigitalHumanTask, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	task, err := getDigitalHumanTaskForUpdateTx(ctx, tx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrDigitalHumanTaskNotFound
	}

	payload := decodeDigitalHumanBillingPayload(task.BillingPayload)
	payload["billingMessage"] = strings.TrimSpace(message)
	updated, err := updateDigitalHumanTaskBillingTx(ctx, tx, task.ID, "settlement_pending", actualDurationSeconds, finalCreditsMillis, payload)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func getDigitalHumanTaskForUpdateTx(ctx context.Context, tx pgx.Tx, taskID string) (*domain.DigitalHumanTask, error) {
	row := tx.QueryRow(ctx, `
		SELECT `+digitalHumanTaskSelectColumns+`
		FROM digital_human_tasks
		WHERE id = $1
		FOR UPDATE
	`, strings.TrimSpace(taskID))

	task, err := scanDigitalHumanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func updateDigitalHumanTaskBillingTx(
	ctx context.Context,
	tx pgx.Tx,
	taskID string,
	billingStatus string,
	actualDurationSeconds *int,
	finalCreditsMillis *int64,
	payload map[string]any,
) (*domain.DigitalHumanTask, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var finalCreditsLegacy any
	var finalCreditsMillisValue any
	if finalCreditsMillis != nil {
		finalCreditsLegacy = DigitalHumanRoundMillisToWholeCredits(*finalCreditsMillis)
		finalCreditsMillisValue = *finalCreditsMillis
	}

	row := tx.QueryRow(ctx, `
		UPDATE digital_human_tasks
		SET billing_status = $2,
		    actual_duration_seconds = COALESCE($3, actual_duration_seconds),
		    final_credits = COALESCE($4, final_credits),
		    final_credits_millis = COALESCE($5, final_credits_millis),
		    billing_payload = $6::jsonb,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING `+digitalHumanTaskSelectColumns+`
	`, strings.TrimSpace(taskID), strings.TrimSpace(billingStatus), actualDurationSeconds, finalCreditsLegacy, finalCreditsMillisValue, bytesOrNil(payloadJSON))

	return scanDigitalHumanTask(row)
}

func applyDigitalHumanWalletDeltaTx(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	amountDeltaMillis int64,
	quantity int,
	unitPriceCreditMillis int64,
	entryType string,
	description string,
	taskID string,
	metadata map[string]any,
) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", fmt.Errorf("user id is required")
	}
	if amountDeltaMillis == 0 {
		return "", nil
	}

	currentBalanceMillis, err := ensureDigitalHumanWalletAndLockMillisTx(ctx, tx, userID)
	if err != nil {
		return "", err
	}
	nextBalanceMillis := currentBalanceMillis + amountDeltaMillis
	if nextBalanceMillis < 0 {
		return "", ErrDigitalHumanBillingInsufficientBalance
	}

	currentBalanceWhole, currentBalanceFraction := DigitalHumanSplitWalletBalanceMillis(currentBalanceMillis)
	nextBalanceWhole, nextBalanceFraction := DigitalHumanSplitWalletBalanceMillis(nextBalanceMillis)
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
		unitValue = digitalHumanBillingUnit
		unitPriceLegacy = DigitalHumanRoundMillisToWholeCredits(unitPriceCreditMillis)
		unitPriceMillisValue = unitPriceCreditMillis
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["referenceType"] = digitalHumanBillingReferenceType
	metadata["referenceId"] = strings.TrimSpace(taskID)
	metadata["amountDelta"] = DigitalHumanCreditsFromMillis(amountDeltaMillis)
	metadata["amountDeltaMillis"] = amountDeltaMillis
	metadata["balanceBeforeCredits"] = DigitalHumanCreditsFromMillis(currentBalanceMillis)
	metadata["balanceBeforeMillis"] = currentBalanceMillis
	metadata["balanceAfterCredits"] = DigitalHumanCreditsFromMillis(nextBalanceMillis)
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
		nextBalanceWhole, nextBalanceFraction, digitalHumanBillingMeterCode, quantityValue, unitValue, unitPriceLegacy, unitPriceMillisValue,
		nullableString(strings.TrimSpace(description)), stringPtr(digitalHumanBillingReferenceType), stringPtr(strings.TrimSpace(taskID)), mustJSONMap(metadata)); err != nil {
		return "", err
	}

	return ledgerID, nil
}

func ensureDigitalHumanWalletAndLockMillisTx(ctx context.Context, tx pgx.Tx, userID string) (int64, error) {
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
	return DigitalHumanCombineWalletBalanceMillis(wholeCredits, fractionalMillis), nil
}

func decodeDigitalHumanBillingPayload(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return map[string]any{}
	}
	return payload
}

func digitalHumanCreditsPerSecondMillis(task *domain.DigitalHumanTask, payload map[string]any) int64 {
	if payload != nil {
		if parsed := positiveInt64FromAny(payload["creditsPerSecondMillis"]); parsed > 0 {
			return parsed
		}
		if parsed := positiveCreditMillisFromAny(payload["creditsPerSecond"]); parsed > 0 {
			return parsed
		}
	}
	if task != nil && task.EstimatedDurationSeconds > 0 && task.EstimatedCreditsMillis > 0 {
		return task.EstimatedCreditsMillis / int64(task.EstimatedDurationSeconds)
	}
	return 0
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
		if parsed, err := DigitalHumanCreditsToMillis(typed); err == nil && parsed > 0 {
			return parsed
		}
	case float32:
		if parsed, err := DigitalHumanCreditsToMillis(float64(typed)); err == nil && parsed > 0 {
			return parsed
		}
	case int64:
		if parsed, err := DigitalHumanCreditsToMillis(float64(typed)); err == nil && parsed > 0 {
			return parsed
		}
	case int32:
		if parsed, err := DigitalHumanCreditsToMillis(float64(typed)); err == nil && parsed > 0 {
			return parsed
		}
	case int:
		if parsed, err := DigitalHumanCreditsToMillis(float64(typed)); err == nil && parsed > 0 {
			return parsed
		}
	case json.Number:
		if floatValue, err := typed.Float64(); err == nil {
			if parsed, convertErr := DigitalHumanCreditsToMillis(floatValue); convertErr == nil && parsed > 0 {
				return parsed
			}
		}
	case string:
		if floatValue, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
			if parsed, convertErr := DigitalHumanCreditsToMillis(floatValue); convertErr == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}
