package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

const (
	mixVideoBillingReferenceType = "mix_video_task"
	mixVideoBillingMeterCode     = "mix_video_seconds"
	mixVideoBillingUnit          = "second"
)

var (
	ErrMixVideoTaskNotFound               = errors.New("mix video task not found")
	ErrMixVideoBillingInsufficientBalance = ErrDigitalHumanBillingInsufficientBalance
)

func (s *Store) PrechargeMixVideoTask(ctx context.Context, taskID string) (*domain.MixVideoTask, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	task, err := getMixVideoTaskForUpdateTx(ctx, tx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrMixVideoTaskNotFound
	}
	if task.BillingStatus == "precharged" || task.BillingStatus == "settled" || task.BillingStatus == "refunded" {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return task, nil
	}

	payload := decodeMixVideoBillingPayload(task.BillingPayload)
	if task.EstimatedCreditsMillis > 0 {
		creditsPerSecondMillis := mixVideoCreditsPerSecondMillis(task, payload)
		ledgerID, applyErr := applyTimedTaskWalletDeltaTx(ctx, tx, task.OwnerUserID, -task.EstimatedCreditsMillis, task.EstimatedDurationSeconds, creditsPerSecondMillis, "usage_debit", "混剪任务预扣积分", mixVideoBillingReferenceType, task.ID, mixVideoBillingMeterCode, mixVideoBillingUnit, map[string]any{
			"stage":                    "precharge",
			"estimatedDurationSeconds": task.EstimatedDurationSeconds,
			"estimatedCredits":         task.EstimatedCredits,
			"estimatedCreditsMillis":   task.EstimatedCreditsMillis,
		})
		if applyErr != nil {
			return nil, applyErr
		}
		payload["prechargeWalletLedgerId"] = ledgerID
	}
	payload["billingMessage"] = "混剪任务已预扣预计积分"
	updated, err := updateMixVideoTaskBillingTx(ctx, tx, task.ID, "precharged", nil, nil, payload)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Store) RefundMixVideoTaskOnFailure(ctx context.Context, taskID string, failureMessage string) (*domain.MixVideoTask, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	task, err := getMixVideoTaskForUpdateTx(ctx, tx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrMixVideoTaskNotFound
	}
	if task.BillingStatus == "refunded" {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return task, nil
	}

	payload := decodeMixVideoBillingPayload(task.BillingPayload)
	if task.EstimatedCreditsMillis > 0 && task.BillingStatus == "precharged" {
		creditsPerSecondMillis := mixVideoCreditsPerSecondMillis(task, payload)
		ledgerID, applyErr := applyTimedTaskWalletDeltaTx(ctx, tx, task.OwnerUserID, task.EstimatedCreditsMillis, task.EstimatedDurationSeconds, creditsPerSecondMillis, "usage_return", "混剪任务失败退回积分", mixVideoBillingReferenceType, task.ID, mixVideoBillingMeterCode, mixVideoBillingUnit, map[string]any{
			"stage":                    "failure_refund",
			"estimatedDurationSeconds": task.EstimatedDurationSeconds,
			"estimatedCredits":         task.EstimatedCredits,
			"estimatedCreditsMillis":   task.EstimatedCreditsMillis,
			"failureReason":            strings.TrimSpace(failureMessage),
		})
		if applyErr != nil {
			return nil, applyErr
		}
		payload["refundWalletLedgerId"] = ledgerID
	}
	payload["billingMessage"] = firstNonEmptyValue(strings.TrimSpace(failureMessage), "混剪任务失败，已退回预扣积分")
	zeroCreditsMillis := int64(0)
	updated, err := updateMixVideoTaskBillingTx(ctx, tx, task.ID, "refunded", nil, &zeroCreditsMillis, payload)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Store) SettleMixVideoTask(ctx context.Context, taskID string, actualDurationSeconds int) (*domain.MixVideoTask, error) {
	if actualDurationSeconds <= 0 {
		return nil, fmt.Errorf("actual duration must be positive")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	task, err := getMixVideoTaskForUpdateTx(ctx, tx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrMixVideoTaskNotFound
	}

	payload := decodeMixVideoBillingPayload(task.BillingPayload)
	creditsPerSecondMillis := mixVideoCreditsPerSecondMillis(task, payload)
	finalCreditsMillis := int64(actualDurationSeconds) * creditsPerSecondMillis
	deltaCreditsMillis := finalCreditsMillis - task.EstimatedCreditsMillis
	payload["billingMessage"] = "混剪任务已按实际时长完成结算"

	switch {
	case deltaCreditsMillis > 0:
		deltaSeconds := maxInt64(int64(actualDurationSeconds-task.EstimatedDurationSeconds), 0)
		ledgerID, applyErr := applyTimedTaskWalletDeltaTx(ctx, tx, task.OwnerUserID, -deltaCreditsMillis, int(deltaSeconds), creditsPerSecondMillis, "usage_debit", "混剪任务补扣积分", mixVideoBillingReferenceType, task.ID, mixVideoBillingMeterCode, mixVideoBillingUnit, map[string]any{
			"stage":                    "settlement_debit",
			"estimatedCredits":         task.EstimatedCredits,
			"estimatedCreditsMillis":   task.EstimatedCreditsMillis,
			"finalCredits":             CreditsFromMillis(finalCreditsMillis),
			"finalCreditsMillis":       finalCreditsMillis,
			"actualDurationSeconds":    actualDurationSeconds,
			"estimatedDurationSeconds": task.EstimatedDurationSeconds,
		})
		if applyErr != nil {
			return nil, applyErr
		}
		payload["settlementWalletLedgerId"] = ledgerID
		payload["billingMessage"] = "混剪任务已按实际时长补扣积分"
	case deltaCreditsMillis < 0:
		refundSeconds := maxInt64(int64(task.EstimatedDurationSeconds-actualDurationSeconds), 0)
		ledgerID, applyErr := applyTimedTaskWalletDeltaTx(ctx, tx, task.OwnerUserID, -deltaCreditsMillis, int(refundSeconds), creditsPerSecondMillis, "usage_return", "混剪任务退回差额积分", mixVideoBillingReferenceType, task.ID, mixVideoBillingMeterCode, mixVideoBillingUnit, map[string]any{
			"stage":                    "settlement_refund",
			"estimatedCredits":         task.EstimatedCredits,
			"estimatedCreditsMillis":   task.EstimatedCreditsMillis,
			"finalCredits":             CreditsFromMillis(finalCreditsMillis),
			"finalCreditsMillis":       finalCreditsMillis,
			"actualDurationSeconds":    actualDurationSeconds,
			"estimatedDurationSeconds": task.EstimatedDurationSeconds,
		})
		if applyErr != nil {
			return nil, applyErr
		}
		payload["refundWalletLedgerId"] = ledgerID
		payload["billingMessage"] = "混剪任务已按实际时长退回差额积分"
	}

	updated, err := updateMixVideoTaskBillingTx(ctx, tx, task.ID, "settled", &actualDurationSeconds, &finalCreditsMillis, payload)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Store) MarkMixVideoTaskSettlementPending(ctx context.Context, taskID string, actualDurationSeconds *int, finalCreditsMillis *int64, message string) (*domain.MixVideoTask, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	task, err := getMixVideoTaskForUpdateTx(ctx, tx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrMixVideoTaskNotFound
	}

	payload := decodeMixVideoBillingPayload(task.BillingPayload)
	payload["billingMessage"] = strings.TrimSpace(message)
	updated, err := updateMixVideoTaskBillingTx(ctx, tx, task.ID, "settlement_pending", actualDurationSeconds, finalCreditsMillis, payload)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func getMixVideoTaskForUpdateTx(ctx context.Context, tx pgx.Tx, taskID string) (*domain.MixVideoTask, error) {
	row := tx.QueryRow(ctx, `
		SELECT `+mixVideoTaskSelectColumns+`
		FROM mix_video_tasks
		WHERE id = $1
		FOR UPDATE
	`, strings.TrimSpace(taskID))

	task, err := scanMixVideoTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func updateMixVideoTaskBillingTx(
	ctx context.Context,
	tx pgx.Tx,
	taskID string,
	billingStatus string,
	actualDurationSeconds *int,
	finalCreditsMillis *int64,
	payload map[string]any,
) (*domain.MixVideoTask, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var finalCreditsLegacy any
	var finalCreditsMillisValue any
	if finalCreditsMillis != nil {
		finalCreditsLegacy = RoundMillisToWholeCredits(*finalCreditsMillis)
		finalCreditsMillisValue = *finalCreditsMillis
	}

	row := tx.QueryRow(ctx, `
		UPDATE mix_video_tasks
		SET billing_status = $2,
		    actual_duration_seconds = COALESCE($3, actual_duration_seconds),
		    final_credits = COALESCE($4, final_credits),
		    final_credits_millis = COALESCE($5, final_credits_millis),
		    billing_payload = $6::jsonb,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING `+mixVideoTaskSelectColumns+`
	`, strings.TrimSpace(taskID), strings.TrimSpace(billingStatus), actualDurationSeconds, finalCreditsLegacy, finalCreditsMillisValue, bytesOrNil(payloadJSON))

	return scanMixVideoTask(row)
}

func decodeMixVideoBillingPayload(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return map[string]any{}
	}
	return payload
}

func mixVideoCreditsPerSecondMillis(task *domain.MixVideoTask, payload map[string]any) int64 {
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
