package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type billedUsageEventRecord struct {
	ID            string
	UserID        string
	SourceType    string
	SourceID      *string
	MeterCode     string
	UsageQuantity int64
	WalletLedger  *string
	QuotaLedger   *string
	BillMessage   *string
	Payload       []byte
}

type usageRefundSummary struct {
	ReturnedCredits         int64
	ReturnWalletLedgerIDs   []string
	ReversedReleaseEventIDs []string
}

type walletLedgerRefundRecord struct {
	ID               string
	MeterCode        *string
	Quantity         *int64
	Unit             *string
	UnitPriceCredits *int64
	AmountDelta      int64
	Description      *string
}

type walletLotConsumptionRefundRecord struct {
	ID             string
	WalletLotID    *string
	DebitedCredits int64
	MeterCode      *string
	Metadata       []byte
}

type quotaLedgerRefundRecord struct {
	ID             string
	QuotaAccountID *string
	UserID         string
	MeterCode      string
	AmountDelta    int64
	Description    *string
	Payload        []byte
}

type distributionReleaseEventRecord struct {
	ID                       string
	CommissionItemID         string
	PromoterUserID           string
	InviteeUserID            string
	RechargeOrderID          string
	SourceType               string
	SourceID                 *string
	SourceSnapshot           []byte
	WalletLotID              *string
	QuotaAccountID           *string
	ConsumedCreditsDelta     int64
	ReleasedAmountDeltaCents int64
	Metadata                 []byte
}

// 返还用量额度Failed来源，把失败或回滚场景下的额度或资源归还到账户状态。
func (s *Store) ReturnUsageCreditsForFailedSource(ctx context.Context, sourceType string, sourceID string, failureMessage string) error {
	if strings.TrimSpace(sourceType) == "" || strings.TrimSpace(sourceID) == "" {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.returnUsageCreditsForFailedSourceTx(ctx, tx, sourceType, sourceID, failureMessage); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// 根据Failed来源事务计算返还用量额度，供存储层的状态判定和查询逻辑复用。
func (s *Store) returnUsageCreditsForFailedSourceTx(ctx context.Context, tx pgx.Tx, sourceType string, sourceID string, failureMessage string) error {
	sourceType = strings.TrimSpace(sourceType)
	sourceID = strings.TrimSpace(sourceID)
	if sourceType == "" || sourceID == "" {
		return nil
	}

	sourceSnapshot, err := buildDistributionSourceSnapshotTx(ctx, tx, sourceType, sourceID)
	if err != nil {
		return err
	}

	rows, err := tx.Query(ctx, `
		SELECT
			id,
			user_id,
			source_type,
			source_id,
			meter_code,
			usage_quantity,
			wallet_ledger_id,
			quota_ledger_id,
			bill_message,
			payload
		FROM billing_usage_events
		WHERE source_type = $1
		  AND source_id = $2
		  AND bill_status = 'billed'
		ORDER BY created_at ASC
		FOR UPDATE
	`, sourceType, sourceID)
	if err != nil {
		return err
	}
	defer rows.Close()

	returnMessage := strings.TrimSpace(failureMessage)
	if returnMessage == "" {
		returnMessage = "任务失败，已自动返还积分"
	}

	events := make([]billedUsageEventRecord, 0)
	for rows.Next() {
		var event billedUsageEventRecord
		if scanErr := rows.Scan(
			&event.ID,
			&event.UserID,
			&event.SourceType,
			&event.SourceID,
			&event.MeterCode,
			&event.UsageQuantity,
			&event.WalletLedger,
			&event.QuotaLedger,
			&event.BillMessage,
			&event.Payload,
		); scanErr != nil {
			return scanErr
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, event := range events {
		payload := decodeUsageEventPayload(event.Payload)
		if !usagePayloadBool(payload, "supportsFailureRefund") {
			continue
		}

		summary, err := s.refundUsageEventTx(ctx, tx, event, payload, sourceSnapshot, returnMessage)
		if err != nil {
			return err
		}
		if !summary.hasRefundEffect() {
			continue
		}

		payload["returnStatus"] = "returned"
		payload["returnReason"] = returnMessage
		payload["returnedAt"] = time.Now().UTC().Format(time.RFC3339)
		payload["returnedCredits"] = summary.ReturnedCredits
		if len(summary.ReturnWalletLedgerIDs) > 0 {
			payload["returnWalletLedgerIds"] = summary.ReturnWalletLedgerIDs
		}
		if len(summary.ReversedReleaseEventIDs) > 0 {
			payload["returnReleaseEventIds"] = summary.ReversedReleaseEventIDs
		}

		if _, err := tx.Exec(ctx, `
				UPDATE billing_usage_events
				SET bill_status = 'returned',
			    bill_message = $2,
			    payload = $3,
			    updated_at = NOW()
			WHERE id = $1
				`, event.ID, returnMessage, mustJSONMap(payload)); err != nil {
			return err
		}
	}

	return nil
}

// 处理refund用量事件事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) refundUsageEventTx(
	ctx context.Context,
	tx pgx.Tx,
	event billedUsageEventRecord,
	payload map[string]any,
	sourceSnapshot []byte,
	failureMessage string,
) (usageRefundSummary, error) {
	summary := usageRefundSummary{}

	walletLedgerIDs := usagePayloadStringSlice(payload, "walletLedgerIds")
	if event.WalletLedger != nil {
		walletLedgerIDs = append(walletLedgerIDs, strings.TrimSpace(*event.WalletLedger))
	}
	walletLedgerIDs = uniqueTrimmedStrings(walletLedgerIDs)
	for _, walletLedgerID := range walletLedgerIDs {
		returnLedgerID, returnedCredits, reversedReleaseEventIDs, err := s.refundWalletUsageLedgerTx(ctx, tx, event, walletLedgerID, sourceSnapshot, failureMessage)
		if err != nil {
			return summary, err
		}
		if returnLedgerID != "" {
			summary.ReturnWalletLedgerIDs = append(summary.ReturnWalletLedgerIDs, returnLedgerID)
		}
		summary.ReturnedCredits += returnedCredits
		summary.ReversedReleaseEventIDs = append(summary.ReversedReleaseEventIDs, reversedReleaseEventIDs...)
	}

	quotaLedgerIDs := usagePayloadStringSlice(payload, "quotaLedgerIds")
	if event.QuotaLedger != nil {
		quotaLedgerIDs = append(quotaLedgerIDs, strings.TrimSpace(*event.QuotaLedger))
	}
	quotaLedgerIDs = uniqueTrimmedStrings(quotaLedgerIDs)
	for _, quotaLedgerID := range quotaLedgerIDs {
		returnLedgerID, returnedCredits, reversedReleaseEventIDs, err := s.refundQuotaUsageLedgerTx(ctx, tx, event, quotaLedgerID, sourceSnapshot, failureMessage)
		if err != nil {
			return summary, err
		}
		if returnLedgerID != "" {
			summary.ReturnWalletLedgerIDs = append(summary.ReturnWalletLedgerIDs, returnLedgerID)
		}
		summary.ReturnedCredits += returnedCredits
		summary.ReversedReleaseEventIDs = append(summary.ReversedReleaseEventIDs, reversedReleaseEventIDs...)
	}

	summary.ReturnWalletLedgerIDs = uniqueTrimmedStrings(summary.ReturnWalletLedgerIDs)
	summary.ReversedReleaseEventIDs = uniqueTrimmedStrings(summary.ReversedReleaseEventIDs)
	return summary, nil
}

// 判断是否存在RefundEffect，供当前链路选择后续处理策略。
func (summary usageRefundSummary) hasRefundEffect() bool {
	return summary.ReturnedCredits > 0 ||
		len(summary.ReturnWalletLedgerIDs) > 0 ||
		len(summary.ReversedReleaseEventIDs) > 0
}

// 处理refund钱包用量台账事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) refundWalletUsageLedgerTx(
	ctx context.Context,
	tx pgx.Tx,
	event billedUsageEventRecord,
	originalWalletLedgerID string,
	sourceSnapshot []byte,
	failureMessage string,
) (string, int64, []string, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			id,
			meter_code,
			quantity,
			unit,
			unit_price_credits,
			amount_delta,
			description
		FROM wallet_ledgers
		WHERE id = $1
		  AND user_id = $2
		FOR UPDATE
	`, strings.TrimSpace(originalWalletLedgerID), event.UserID)

	var ledger walletLedgerRefundRecord
	if err := row.Scan(
		&ledger.ID,
		&ledger.MeterCode,
		&ledger.Quantity,
		&ledger.Unit,
		&ledger.UnitPriceCredits,
		&ledger.AmountDelta,
		&ledger.Description,
	); err != nil {
		if err == pgx.ErrNoRows {
			return "", 0, nil, nil
		}
		return "", 0, nil, err
	}

	returnedCredits := absInt64(ledger.AmountDelta)
	if returnedCredits <= 0 {
		return "", 0, nil, nil
	}

	currentBalance, err := ensureWalletAndLockTx(ctx, tx, event.UserID)
	if err != nil {
		return "", 0, nil, err
	}
	nextBalance := currentBalance + returnedCredits
	if _, err := tx.Exec(ctx, `
		UPDATE billing_wallets
		SET credit_balance = $2,
		    updated_at = NOW()
		WHERE user_id = $1
	`, event.UserID, nextBalance); err != nil {
		return "", 0, nil, err
	}

	returnLedgerID := uuid.NewString()
	description := strings.TrimSpace(valueOrEmpty(ledger.Description))
	if description == "" {
		description = strings.TrimSpace(event.MeterCode)
	}
	description = description + " 失败返还积分"
	returnMetadata := mustJSONMap(map[string]any{
		"eventKind":              "usage_return",
		"returnOfUsageEventId":   event.ID,
		"returnOfWalletLedgerId": ledger.ID,
		"sourceType":             event.SourceType,
		"sourceId":               valueOrEmpty(event.SourceID),
		"meterCode":              event.MeterCode,
		"returnedCredits":        returnedCredits,
		"failureReason":          failureMessage,
	})
	if _, err := tx.Exec(ctx, `
		INSERT INTO wallet_ledgers (
			id, user_id, entry_type, amount_delta, balance_before, balance_after, meter_code, quantity,
			unit, unit_price_credits, description, reference_type, reference_id, metadata
		)
		VALUES ($1, $2, 'usage_return', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, returnLedgerID, event.UserID, returnedCredits, currentBalance, nextBalance, ledger.MeterCode, ledger.Quantity,
		ledger.Unit, ledger.UnitPriceCredits, description, nullableString(event.SourceType), event.SourceID, returnMetadata); err != nil {
		return "", 0, nil, err
	}

	if err := s.reverseWalletLotConsumptionsTx(ctx, tx, event, ledger.ID, returnLedgerID, failureMessage); err != nil {
		return "", 0, nil, err
	}

	reversedReleaseEventIDs, err := s.reverseDistributionReleaseEventsByWalletLedgerTx(ctx, tx, event, ledger.ID, returnLedgerID, sourceSnapshot, failureMessage)
	if err != nil {
		return "", 0, nil, err
	}

	return returnLedgerID, returnedCredits, reversedReleaseEventIDs, nil
}

// 处理refund额度用量台账事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) refundQuotaUsageLedgerTx(
	ctx context.Context,
	tx pgx.Tx,
	event billedUsageEventRecord,
	originalQuotaLedgerID string,
	sourceSnapshot []byte,
	failureMessage string,
) (string, int64, []string, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			l.id,
			l.quota_account_id,
			l.user_id,
			l.meter_code,
			l.amount_delta,
			l.description,
			l.payload
		FROM billing_quota_ledgers l
		WHERE l.id = $1
		  AND l.user_id = $2
		FOR UPDATE
	`, strings.TrimSpace(originalQuotaLedgerID), event.UserID)

	var ledger quotaLedgerRefundRecord
	if err := row.Scan(
		&ledger.ID,
		&ledger.QuotaAccountID,
		&ledger.UserID,
		&ledger.MeterCode,
		&ledger.AmountDelta,
		&ledger.Description,
		&ledger.Payload,
	); err != nil {
		if err == pgx.ErrNoRows {
			return "", 0, nil, nil
		}
		return "", 0, nil, err
	}

	returnedUnits := absInt64(ledger.AmountDelta)
	if returnedUnits <= 0 || ledger.QuotaAccountID == nil || strings.TrimSpace(*ledger.QuotaAccountID) == "" {
		return "", 0, nil, nil
	}

	var rechargeOrderID *string
	var distributionCommissionItemID *string
	var releaseUnitCredits int64
	if err := tx.QueryRow(ctx, `
		SELECT recharge_order_id, distribution_commission_item_id, release_unit_credits
		FROM billing_quota_accounts
		WHERE id = $1
		  AND user_id = $2
		FOR UPDATE
	`, strings.TrimSpace(*ledger.QuotaAccountID), event.UserID).Scan(&rechargeOrderID, &distributionCommissionItemID, &releaseUnitCredits); err != nil {
		if err == pgx.ErrNoRows {
			return "", 0, nil, nil
		}
		return "", 0, nil, err
	}

	payload := decodeUsageEventPayload(ledger.Payload)
	returnedCredits := quotaUsageReturnCredits(releaseUnitCredits, payload, returnedUnits)
	if returnedCredits <= 0 {
		return "", 0, nil, nil
	}

	description := strings.TrimSpace(valueOrEmpty(ledger.Description))
	if description == "" {
		description = strings.TrimSpace(event.MeterCode)
	}
	description = description + " 失败返还积分"
	entryType := "usage_return"
	returnMetadata := mustJSONMap(map[string]any{
		"eventKind":             "usage_return",
		"returnOfUsageEventId":  event.ID,
		"returnOfQuotaLedgerId": ledger.ID,
		"sourceType":            event.SourceType,
		"sourceId":              valueOrEmpty(event.SourceID),
		"meterCode":             event.MeterCode,
		"returnedCredits":       returnedCredits,
		"quotaUnitsConsumed":    returnedUnits,
		"creditValuePerQuota":   maxInt64(releaseUnitCredits, 0),
		"failureReason":         failureMessage,
	})

	returnLedgerID, err := s.grantWalletCreditsTx(ctx, tx, GrantWalletCreditsInput{
		UserID:                       event.UserID,
		Amount:                       returnedCredits,
		EntryType:                    &entryType,
		Description:                  stringPtr(description),
		ReferenceType:                nullableString(event.SourceType),
		ReferenceID:                  event.SourceID,
		RechargeOrderID:              rechargeOrderID,
		DistributionCommissionItemID: distributionCommissionItemID,
		ReleaseUnitCredits:           1,
		Metadata:                     returnMetadata,
	})
	if err != nil {
		return "", 0, nil, err
	}

	reversedReleaseEventIDs, err := s.reverseDistributionReleaseEventsByQuotaLedgerTx(ctx, tx, event, ledger.ID, returnLedgerID, sourceSnapshot, failureMessage)
	if err != nil {
		return "", 0, nil, err
	}

	return returnLedgerID, returnedCredits, reversedReleaseEventIDs, nil
}

// 处理reverse钱包LotConsumptions事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) reverseWalletLotConsumptionsTx(
	ctx context.Context,
	tx pgx.Tx,
	event billedUsageEventRecord,
	originalWalletLedgerID string,
	returnWalletLedgerID string,
	failureMessage string,
) error {
	rows, err := tx.Query(ctx, `
		SELECT id, wallet_lot_id, debited_credits, meter_code, metadata
		FROM billing_wallet_lot_consumptions
		WHERE user_id = $1
		  AND source_type = $2
		  AND source_id = $3
		  AND wallet_ledger_id = $4
		  AND debited_credits > 0
		ORDER BY created_at DESC
		FOR UPDATE
	`, event.UserID, event.SourceType, valueOrEmpty(event.SourceID), strings.TrimSpace(originalWalletLedgerID))
	if err != nil {
		return err
	}
	defer rows.Close()

	consumptions := make([]walletLotConsumptionRefundRecord, 0)
	for rows.Next() {
		var item walletLotConsumptionRefundRecord
		if scanErr := rows.Scan(&item.ID, &item.WalletLotID, &item.DebitedCredits, &item.MeterCode, &item.Metadata); scanErr != nil {
			return scanErr
		}
		consumptions = append(consumptions, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range consumptions {
		if item.WalletLotID == nil || strings.TrimSpace(*item.WalletLotID) == "" || item.DebitedCredits <= 0 {
			continue
		}

		if _, err := tx.Exec(ctx, `
			UPDATE billing_wallet_lots
			SET consumed_credits = GREATEST(consumed_credits - $2, 0),
			    remaining_credits = remaining_credits + $2,
			    status = 'active',
			    updated_at = NOW()
			WHERE id = $1
		`, strings.TrimSpace(*item.WalletLotID), item.DebitedCredits); err != nil {
			return err
		}

		reversalMetadata := decodeUsageEventPayload(item.Metadata)
		reversalMetadata["eventKind"] = "usage_return"
		reversalMetadata["returnOfConsumptionId"] = item.ID
		reversalMetadata["returnOfUsageEventId"] = event.ID
		reversalMetadata["failureReason"] = failureMessage
		if _, err := tx.Exec(ctx, `
				INSERT INTO billing_wallet_lot_consumptions (
					id, wallet_lot_id, user_id, source_type, source_id, meter_code, debited_credits, wallet_ledger_id, metadata
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`, uuid.NewString(), strings.TrimSpace(*item.WalletLotID), event.UserID, event.SourceType, event.SourceID, item.MeterCode, -item.DebitedCredits, nullableString(strings.TrimSpace(returnWalletLedgerID)), mustJSONMap(reversalMetadata)); err != nil {
			return err
		}
	}

	return nil
}

// 处理reverse分销释放事件钱包台账事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) reverseDistributionReleaseEventsByWalletLedgerTx(
	ctx context.Context,
	tx pgx.Tx,
	event billedUsageEventRecord,
	originalWalletLedgerID string,
	returnWalletLedgerID string,
	sourceSnapshot []byte,
	failureMessage string,
) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			id,
			commission_item_id,
			promoter_user_id,
			invitee_user_id,
			recharge_order_id,
			source_type,
			source_id,
			source_snapshot,
			wallet_lot_id,
			quota_account_id,
			consumed_credits_delta,
			released_amount_delta_cents,
			metadata
		FROM distribution_commission_release_events
		WHERE source_type = $1
		  AND source_id = $2
		  AND wallet_ledger_id = $3
		  AND consumed_credits_delta > 0
		ORDER BY created_at DESC
		FOR UPDATE
	`, event.SourceType, valueOrEmpty(event.SourceID), strings.TrimSpace(originalWalletLedgerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	releaseEvents := make([]distributionReleaseEventRecord, 0)
	for rows.Next() {
		releaseEvent, scanErr := scanDistributionReleaseEventRecord(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		releaseEvents = append(releaseEvents, *releaseEvent)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(releaseEvents))
	for _, releaseEvent := range releaseEvents {
		if err := s.reverseDistributionReleaseEventTx(ctx, tx, releaseEvent, stringPtr(strings.TrimSpace(returnWalletLedgerID)), nil, sourceSnapshot, failureMessage, event.ID); err != nil {
			return nil, err
		}
		ids = append(ids, releaseEvent.ID)
	}
	return ids, nil
}

// 处理reverse分销释放事件额度台账事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) reverseDistributionReleaseEventsByQuotaLedgerTx(
	ctx context.Context,
	tx pgx.Tx,
	event billedUsageEventRecord,
	originalQuotaLedgerID string,
	returnWalletLedgerID string,
	sourceSnapshot []byte,
	failureMessage string,
) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			id,
			commission_item_id,
			promoter_user_id,
			invitee_user_id,
			recharge_order_id,
			source_type,
			source_id,
			source_snapshot,
			wallet_lot_id,
			quota_account_id,
			consumed_credits_delta,
			released_amount_delta_cents,
			metadata
		FROM distribution_commission_release_events
		WHERE source_type = $1
		  AND source_id = $2
		  AND quota_ledger_id = $3
		  AND consumed_credits_delta > 0
		ORDER BY created_at DESC
		FOR UPDATE
	`, event.SourceType, valueOrEmpty(event.SourceID), strings.TrimSpace(originalQuotaLedgerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	releaseEvents := make([]distributionReleaseEventRecord, 0)
	for rows.Next() {
		releaseEvent, scanErr := scanDistributionReleaseEventRecord(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		releaseEvents = append(releaseEvents, *releaseEvent)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(releaseEvents))
	for _, releaseEvent := range releaseEvents {
		if err := s.reverseDistributionReleaseEventTx(ctx, tx, releaseEvent, stringPtr(strings.TrimSpace(returnWalletLedgerID)), nil, sourceSnapshot, failureMessage, event.ID); err != nil {
			return nil, err
		}
		ids = append(ids, releaseEvent.ID)
	}
	return ids, nil
}

// 处理reverse分销释放事件事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) reverseDistributionReleaseEventTx(
	ctx context.Context,
	tx pgx.Tx,
	event distributionReleaseEventRecord,
	returnWalletLedgerID *string,
	returnQuotaLedgerID *string,
	sourceSnapshot []byte,
	failureMessage string,
	usageEventID string,
) error {
	consumedCreditsDelta := maxInt64(event.ConsumedCreditsDelta, 0)
	releasedAmountDelta := maxInt64(event.ReleasedAmountDeltaCents, 0)
	if consumedCreditsDelta <= 0 && releasedAmountDelta <= 0 {
		return nil
	}

	item, err := getDistributionCommissionItemByIDTx(ctx, tx, event.CommissionItemID)
	if err != nil {
		return err
	}
	if item == nil {
		return nil
	}

	nextConsumedCredits := item.ConsumedCredits - consumedCreditsDelta
	if nextConsumedCredits < 0 {
		nextConsumedCredits = 0
	}
	nextReleasedAmount := item.ReleasedAmountCents - releasedAmountDelta
	if nextReleasedAmount < item.SettledAmountCents {
		nextReleasedAmount = item.SettledAmountCents
	}
	nextStatus := deriveCommissionStatus(nextReleasedAmount, item.SettledAmountCents, item.AmountCents)

	var nextReleasedAt *time.Time
	if nextReleasedAmount > 0 {
		nextReleasedAt = item.ReleasedAt
	}
	if _, err := tx.Exec(ctx, `
		UPDATE distribution_commission_items
		SET status = $2,
		    consumed_credits = $3,
		    released_amount_cents = $4,
		    released_at = $5,
		    updated_at = NOW()
		WHERE id = $1
	`, item.ID, nextStatus, nextConsumedCredits, nextReleasedAmount, nextReleasedAt); err != nil {
		return err
	}

	reversalMetadata := decodeUsageEventPayload(event.Metadata)
	reversalMetadata["eventKind"] = "usage_return"
	reversalMetadata["returnOfReleaseEventId"] = event.ID
	reversalMetadata["returnOfUsageEventId"] = usageEventID
	reversalMetadata["failureReason"] = failureMessage
	if _, err := tx.Exec(ctx, `
		INSERT INTO distribution_commission_release_events (
			id, commission_item_id, promoter_user_id, invitee_user_id, recharge_order_id,
			source_type, source_id, source_snapshot, wallet_lot_id, quota_account_id,
			wallet_ledger_id, quota_ledger_id, consumed_credits_delta, released_amount_delta_cents,
			commission_item_consumed_credits, commission_item_released_amount_cents, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, uuid.NewString(), item.ID, event.PromoterUserID, event.InviteeUserID, event.RechargeOrderID,
		event.SourceType, event.SourceID, bytesOrNil(firstNonEmptyBytes(sourceSnapshot, event.SourceSnapshot)), event.WalletLotID, event.QuotaAccountID,
		returnWalletLedgerID, returnQuotaLedgerID, -consumedCreditsDelta, -releasedAmountDelta, nextConsumedCredits, nextReleasedAmount, mustJSONMap(reversalMetadata)); err != nil {
		return err
	}

	return nil
}

// 处理扫描分销释放事件记录相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func scanDistributionReleaseEventRecord(scan scanFn) (*distributionReleaseEventRecord, error) {
	var item distributionReleaseEventRecord
	if err := scan(
		&item.ID,
		&item.CommissionItemID,
		&item.PromoterUserID,
		&item.InviteeUserID,
		&item.RechargeOrderID,
		&item.SourceType,
		&item.SourceID,
		&item.SourceSnapshot,
		&item.WalletLotID,
		&item.QuotaAccountID,
		&item.ConsumedCreditsDelta,
		&item.ReleasedAmountDeltaCents,
		&item.Metadata,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

// 处理解码用量事件载荷相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func decodeUsageEventPayload(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return map[string]any{}
	}
	return payload
}

// 处理用量载荷Int64相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func usagePayloadInt64(payload map[string]any, key string) int64 {
	if payload == nil {
		return 0
	}
	return usageQuantityValue(payload[strings.TrimSpace(key)])
}

// 处理额度用量返还额度相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func quotaUsageReturnCredits(releaseUnitCredits int64, payload map[string]any, returnedUnits int64) int64 {
	if returnedUnits <= 0 {
		return 0
	}
	if releaseUnitCredits > 0 {
		return returnedUnits * releaseUnitCredits
	}
	if totalCreditValue := usagePayloadInt64(payload, "creditValue"); totalCreditValue > 0 {
		return totalCreditValue
	}
	if payloadReleaseUnitCredits := usagePayloadInt64(payload, "releaseUnitCredits"); payloadReleaseUnitCredits > 0 {
		return returnedUnits * payloadReleaseUnitCredits
	}
	return 0
}

// 处理用量载荷Bool相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func usagePayloadBool(payload map[string]any, key string) bool {
	value, ok := payload[strings.TrimSpace(key)]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

// 处理用量载荷StringSlice相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func usagePayloadStringSlice(payload map[string]any, key string) []string {
	value, ok := payload[strings.TrimSpace(key)]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				items = append(items, strings.TrimSpace(text))
			}
		}
		return items
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{strings.TrimSpace(typed)}
	default:
		return nil
	}
}

// 处理unique裁剪Strings相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func uniqueTrimmedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// 处理首个Non空值Bytes相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstNonEmptyBytes(values ...[]byte) []byte {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}
