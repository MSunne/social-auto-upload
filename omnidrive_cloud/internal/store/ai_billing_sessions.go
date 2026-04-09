package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

// 处理扫描AI计费会话相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func scanAIBillingSession(scan scanFn) (*domain.AIBillingSession, error) {
	var item domain.AIBillingSession
	var specialRuleID *string
	var specialPriceCredits *int64
	var message *string
	var payload []byte
	if err := scan(
		&item.ID,
		&item.UserID,
		&item.SourceType,
		&item.SourceID,
		&item.WorkflowCode,
		&item.OutputType,
		&item.DurationSeconds,
		&item.SegmentSeconds,
		&specialRuleID,
		&specialPriceCredits,
		&item.PlannedCredits,
		&item.BilledCredits,
		&item.RefundedCredits,
		&item.Status,
		&message,
		&payload,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.SpecialRuleID = normalizeOptionalString(specialRuleID)
	item.SpecialPriceCredits = specialPriceCredits
	item.Message = trimOptionalString(message)
	item.Payload = bytesOrNil(payload)
	return &item, nil
}

// 处理扫描AI计费Item相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func scanAIBillingItem(scan scanFn) (*domain.AIBillingItem, error) {
	var item domain.AIBillingItem
	var modelName *string
	var modelAlias *string
	var meterCode *string
	var walletLedgerID *string
	var payload []byte
	if err := scan(
		&item.ID,
		&item.SessionID,
		&item.UserID,
		&item.SourceType,
		&item.SourceID,
		&item.ItemKey,
		&item.ItemType,
		&item.Label,
		&modelName,
		&modelAlias,
		&meterCode,
		&item.Quantity,
		&item.Unit,
		&item.UnitPriceCredits,
		&item.PlannedCredits,
		&item.BilledCredits,
		&item.RefundedCredits,
		&item.SortOrder,
		&item.IsVisible,
		&item.Status,
		&walletLedgerID,
		&payload,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.ModelName = normalizeOptionalString(modelName)
	item.ModelAlias = normalizeOptionalString(modelAlias)
	item.MeterCode = normalizeOptionalString(meterCode)
	item.WalletLedgerID = normalizeOptionalString(walletLedgerID)
	item.Payload = bytesOrNil(payload)
	return &item, nil
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListAIBillingItemsBySession(ctx context.Context, sessionID string) ([]domain.AIBillingItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, session_id, user_id, source_type, source_id, item_key, item_type, label, model_name,
		       COALESCE(am.model_alias, model_name) AS model_alias, meter_code, quantity, unit, unit_price_credits,
		       planned_credits, billed_credits, refunded_credits, sort_order, is_visible, status,
		       wallet_ledger_id, payload, created_at, updated_at
		FROM ai_billing_items
		LEFT JOIN ai_models am ON am.model_name = ai_billing_items.model_name
		WHERE session_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AIBillingItem, 0)
	for rows.Next() {
		item, scanErr := scanAIBillingItem(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetAIBillingSessionBySource(ctx context.Context, sourceType string, sourceID string) (*domain.AIBillingSession, []domain.AIBillingItem, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, source_type, source_id, workflow_code, output_type, duration_seconds, segment_seconds,
		       special_rule_id, special_price_credits, planned_credits, billed_credits, refunded_credits, status,
		       message, payload, created_at, updated_at
		FROM ai_billing_sessions
		WHERE source_type = $1
		  AND source_id = $2
	`, strings.TrimSpace(sourceType), strings.TrimSpace(sourceID))
	session, err := scanAIBillingSession(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	items, err := s.ListAIBillingItemsBySession(ctx, session.ID)
	if err != nil {
		return nil, nil, err
	}
	return session, items, nil
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) ListAIBillingSessionsByUser(ctx context.Context, userID string, page int, pageSize int) ([]domain.AIBillingSession, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM ai_billing_sessions
		WHERE user_id = $1
	`, strings.TrimSpace(userID)).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, source_type, source_id, workflow_code, output_type, duration_seconds, segment_seconds,
		       special_rule_id, special_price_credits, planned_credits, billed_credits, refunded_credits, status,
		       message, payload, created_at, updated_at
		FROM ai_billing_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, strings.TrimSpace(userID), pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.AIBillingSession, 0)
	for rows.Next() {
		item, scanErr := scanAIBillingSession(rows.Scan)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) CreateOrRechargeAIBillingSession(
	ctx context.Context,
	sessionInput CreateAIBillingSessionInput,
	itemInputs []CreateAIBillingItemInput,
	sourceSnapshot []byte,
) (*domain.AIBillingSession, []domain.AIBillingItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	session, items, err := s.createOrRechargeAIBillingSessionTx(ctx, tx, sessionInput, itemInputs, sourceSnapshot)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return session, items, nil
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) createOrRechargeAIBillingSessionTx(
	ctx context.Context,
	tx pgx.Tx,
	sessionInput CreateAIBillingSessionInput,
	itemInputs []CreateAIBillingItemInput,
	sourceSnapshot []byte,
) (*domain.AIBillingSession, []domain.AIBillingItem, error) {
	existing, err := s.getAIBillingSessionBySourceTx(ctx, tx, sessionInput.SourceType, sessionInput.SourceID)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		items, itemErr := s.listAIBillingItemsBySessionTx(ctx, tx, existing.ID)
		if itemErr != nil {
			return nil, nil, itemErr
		}
		if existing.Status == "refunded" {
			session, rechargeItems, rechargeErr := s.rechargeAIBillingSessionTx(ctx, tx, *existing, items, sourceSnapshot)
			if rechargeErr != nil {
				return nil, nil, rechargeErr
			}
			return session, rechargeItems, nil
		}
		return existing, items, nil
	}

	currentBalance, err := ensureWalletAndLockTx(ctx, tx, strings.TrimSpace(sessionInput.UserID))
	if err != nil {
		return nil, nil, err
	}
	walletLots, err := loadWalletLotsForUsageTx(ctx, tx, strings.TrimSpace(sessionInput.UserID))
	if err != nil {
		return nil, nil, err
	}

	if strings.TrimSpace(sessionInput.Status) == "" {
		sessionInput.Status = "precharged"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_billing_sessions (
			id, user_id, source_type, source_id, workflow_code, output_type, duration_seconds, segment_seconds,
			special_rule_id, special_price_credits, planned_credits, billed_credits, refunded_credits, status, message, payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 0, 0, $12, $13, $14)
	`, strings.TrimSpace(sessionInput.ID), strings.TrimSpace(sessionInput.UserID), strings.TrimSpace(sessionInput.SourceType),
		strings.TrimSpace(sessionInput.SourceID), strings.TrimSpace(sessionInput.WorkflowCode), strings.TrimSpace(sessionInput.OutputType),
		sessionInput.DurationSeconds, sessionInput.SegmentSeconds, trimOptionalString(sessionInput.SpecialRuleID),
		sessionInput.SpecialPriceCredits, sessionInput.PlannedCredits, strings.TrimSpace(sessionInput.Status),
		trimOptionalString(sessionInput.Message), bytesOrNil(sessionInput.Payload)); err != nil {
		return nil, nil, err
	}

	for index := range itemInputs {
		item := itemInputs[index]
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		item.Status = "precharged"
		item.PlannedCredits = maxInt64(item.PlannedCredits, 0)
		item.Unit = strings.TrimSpace(item.Unit)
		if item.Unit == "" {
			item.Unit = "call"
		}

		var walletLedgerID *string
		if item.PlannedCredits > 0 {
			metadata := decodeAIBillingItemPayload(item.Payload)
			metadata["sessionId"] = sessionInput.ID
			metadata["itemKey"] = item.ItemKey
			metadata["itemType"] = item.ItemType
			metadata["sourceType"] = sessionInput.SourceType
			metadata["sourceId"] = sessionInput.SourceID
			plan := walletLedgerPlan{
				meterCode:    firstNonEmptyValue(valueOrEmpty(item.MeterCode), strings.TrimSpace(item.ItemType)),
				quantity:     maxInt64(item.Quantity, 1),
				unit:         "usage",
				debitCredits: item.PlannedCredits,
				description:  strings.TrimSpace(item.Label) + " 预扣费",
				payload:      mustJSONMap(metadata),
			}
			ledgerID, nextBalance, applyErr := applyWalletLedgerPlanTx(ctx, tx, sessionInput.UserID, currentBalance, plan, stringPtr("ai_billing_session"), stringPtr(sessionInput.ID))
			if applyErr != nil {
				return nil, nil, applyErr
			}
			if applyErr := s.applyWalletLotConsumptionsTx(ctx, tx, sessionInput.UserID, sessionInput.SourceType, sessionInput.SourceID, sourceSnapshot, plan, ledgerID, walletLots); applyErr != nil {
				return nil, nil, applyErr
			}
			currentBalance = nextBalance
			walletLedgerID = stringPtr(ledgerID)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO ai_billing_items (
				id, session_id, user_id, source_type, source_id, item_key, item_type, label, model_name, meter_code,
				quantity, unit, unit_price_credits, planned_credits, billed_credits, refunded_credits, sort_order,
				is_visible, status, wallet_ledger_id, payload
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 0, 0, $15, $16, $17, $18, $19)
		`, strings.TrimSpace(item.ID), strings.TrimSpace(sessionInput.ID), strings.TrimSpace(sessionInput.UserID),
			strings.TrimSpace(sessionInput.SourceType), strings.TrimSpace(sessionInput.SourceID), strings.TrimSpace(item.ItemKey),
			strings.TrimSpace(item.ItemType), strings.TrimSpace(item.Label), trimOptionalString(item.ModelName),
			trimOptionalString(item.MeterCode), maxInt64(item.Quantity, 1), item.Unit, item.UnitPriceCredits,
			item.PlannedCredits, item.SortOrder, item.IsVisible, item.Status, trimOptionalString(walletLedgerID), bytesOrNil(item.Payload)); err != nil {
			return nil, nil, err
		}
	}

	return s.getAIBillingSessionWithItemsTx(ctx, tx, sessionInput.SourceType, sessionInput.SourceID)
}

// 处理FinalizeAI计费会话相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) FinalizeAIBillingSession(
	ctx context.Context,
	sourceType string,
	sourceID string,
	successfulItemKeys []string,
	message string,
	payload []byte,
) (*domain.AIBillingSession, []domain.AIBillingItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	session, items, err := s.finalizeAIBillingSessionTx(ctx, tx, sourceType, sourceID, successfulItemKeys, message, payload)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return session, items, nil
}

// 处理RefundAI计费会话来源相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) RefundAIBillingSessionBySource(ctx context.Context, sourceType string, sourceID string, failureMessage string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, _, err := s.finalizeAIBillingSessionTx(ctx, tx, sourceType, sourceID, nil, failureMessage, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// 处理finalizeAI计费会话事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) finalizeAIBillingSessionTx(
	ctx context.Context,
	tx pgx.Tx,
	sourceType string,
	sourceID string,
	successfulItemKeys []string,
	message string,
	payload []byte,
) (*domain.AIBillingSession, []domain.AIBillingItem, error) {
	session, items, err := s.getAIBillingSessionWithItemsTx(ctx, tx, sourceType, sourceID)
	if err != nil || session == nil {
		return session, items, err
	}

	sourceSnapshot, err := buildDistributionSourceSnapshotTx(ctx, tx, session.SourceType, session.SourceID)
	if err != nil {
		sourceSnapshot = mustJSONMap(map[string]any{
			"sourceType": session.SourceType,
			"sourceId":   session.SourceID,
			"sessionId":  session.ID,
		})
	}

	successSet := make(map[string]struct{}, len(successfulItemKeys))
	for _, key := range successfulItemKeys {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			successSet[trimmed] = struct{}{}
		}
	}

	for _, item := range items {
		if item.Status == "refunded" {
			continue
		}
		if _, ok := successSet[strings.TrimSpace(item.ItemKey)]; ok {
			if _, err := tx.Exec(ctx, `
				UPDATE ai_billing_items
				SET billed_credits = planned_credits,
				    status = 'billed',
				    updated_at = NOW()
				WHERE id = $1
			`, item.ID); err != nil {
				return nil, nil, err
			}
			continue
		}
		if item.WalletLedgerID != nil && strings.TrimSpace(*item.WalletLedgerID) != "" {
			event := billedUsageEventRecord{
				ID:         session.ID,
				UserID:     session.UserID,
				SourceType: session.SourceType,
				SourceID:   &session.SourceID,
				MeterCode:  firstNonEmptyValue(valueOrEmpty(item.MeterCode), item.ItemType),
			}
			_, returnedCredits, _, refundErr := s.refundWalletUsageLedgerTx(ctx, tx, event, strings.TrimSpace(*item.WalletLedgerID), sourceSnapshot, message)
			if refundErr != nil {
				return nil, nil, refundErr
			}
			if _, err := tx.Exec(ctx, `
				UPDATE ai_billing_items
				SET refunded_credits = $2,
				    billed_credits = 0,
				    status = 'refunded',
				    updated_at = NOW()
				WHERE id = $1
			`, item.ID, returnedCredits); err != nil {
				return nil, nil, err
			}
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE ai_billing_items
			SET status = 'refunded',
			    updated_at = NOW()
			WHERE id = $1
		`, item.ID); err != nil {
			return nil, nil, err
		}
	}

	return s.refreshAIBillingSessionSummaryTx(ctx, tx, session.ID, message, payload)
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) getAIBillingSessionWithItemsTx(ctx context.Context, tx pgx.Tx, sourceType string, sourceID string) (*domain.AIBillingSession, []domain.AIBillingItem, error) {
	session, err := s.getAIBillingSessionBySourceTx(ctx, tx, sourceType, sourceID)
	if err != nil || session == nil {
		return session, nil, err
	}
	items, err := s.listAIBillingItemsBySessionTx(ctx, tx, session.ID)
	if err != nil {
		return nil, nil, err
	}
	return session, items, nil
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) getAIBillingSessionBySourceTx(ctx context.Context, tx pgx.Tx, sourceType string, sourceID string) (*domain.AIBillingSession, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, user_id, source_type, source_id, workflow_code, output_type, duration_seconds, segment_seconds,
		       special_rule_id, special_price_credits, planned_credits, billed_credits, refunded_credits, status,
		       message, payload, created_at, updated_at
		FROM ai_billing_sessions
		WHERE source_type = $1
		  AND source_id = $2
		FOR UPDATE
	`, strings.TrimSpace(sourceType), strings.TrimSpace(sourceID))
	item, err := scanAIBillingSession(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) listAIBillingItemsBySessionTx(ctx context.Context, tx pgx.Tx, sessionID string) ([]domain.AIBillingItem, error) {
	rows, err := tx.Query(ctx, `
		SELECT i.id, i.session_id, i.user_id, i.source_type, i.source_id, i.item_key, i.item_type, i.label, i.model_name,
		       COALESCE(am.model_alias, i.model_name) AS model_alias, i.meter_code, i.quantity, i.unit, i.unit_price_credits,
		       i.planned_credits, i.billed_credits, i.refunded_credits, i.sort_order, i.is_visible, i.status,
		       i.wallet_ledger_id, i.payload, i.created_at, i.updated_at
		FROM ai_billing_items i
		LEFT JOIN ai_models am ON am.model_name = i.model_name
		WHERE i.session_id = $1
		ORDER BY i.sort_order ASC, i.created_at ASC
		FOR UPDATE
	`, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AIBillingItem, 0)
	for rows.Next() {
		item, scanErr := scanAIBillingItem(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

// 处理充值AI计费会话事务相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) rechargeAIBillingSessionTx(ctx context.Context, tx pgx.Tx, session domain.AIBillingSession, items []domain.AIBillingItem, sourceSnapshot []byte) (*domain.AIBillingSession, []domain.AIBillingItem, error) {
	currentBalance, err := ensureWalletAndLockTx(ctx, tx, session.UserID)
	if err != nil {
		return nil, nil, err
	}
	walletLots, err := loadWalletLotsForUsageTx(ctx, tx, session.UserID)
	if err != nil {
		return nil, nil, err
	}

	for _, item := range items {
		if item.PlannedCredits <= 0 {
			if _, err := tx.Exec(ctx, `
				UPDATE ai_billing_items
				SET billed_credits = 0,
				    refunded_credits = 0,
				    status = 'precharged',
				    updated_at = NOW()
				WHERE id = $1
			`, item.ID); err != nil {
				return nil, nil, err
			}
			continue
		}
		payload := decodeAIBillingItemPayload(item.Payload)
		payload["sessionId"] = session.ID
		payload["itemKey"] = item.ItemKey
		plan := walletLedgerPlan{
			meterCode:    firstNonEmptyValue(valueOrEmpty(item.MeterCode), item.ItemType),
			quantity:     maxInt64(item.Quantity, 1),
			unit:         "usage",
			debitCredits: item.PlannedCredits,
			description:  strings.TrimSpace(item.Label) + " 预扣费",
			payload:      mustJSONMap(payload),
		}
		ledgerID, nextBalance, applyErr := applyWalletLedgerPlanTx(ctx, tx, session.UserID, currentBalance, plan, stringPtr("ai_billing_session"), stringPtr(session.ID))
		if applyErr != nil {
			return nil, nil, applyErr
		}
		if applyErr := s.applyWalletLotConsumptionsTx(ctx, tx, session.UserID, session.SourceType, session.SourceID, sourceSnapshot, plan, ledgerID, walletLots); applyErr != nil {
			return nil, nil, applyErr
		}
		currentBalance = nextBalance
		if _, err := tx.Exec(ctx, `
			UPDATE ai_billing_items
			SET wallet_ledger_id = $2,
			    billed_credits = 0,
			    refunded_credits = 0,
			    status = 'precharged',
			    updated_at = NOW()
			WHERE id = $1
		`, item.ID, ledgerID); err != nil {
			return nil, nil, err
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE ai_billing_sessions
		SET billed_credits = 0,
		    refunded_credits = 0,
		    status = 'precharged',
		    updated_at = NOW()
		WHERE id = $1
	`, session.ID); err != nil {
		return nil, nil, err
	}
	return s.getAIBillingSessionWithItemsTx(ctx, tx, session.SourceType, session.SourceID)
}

// 刷新AI计费会话Summary事务，重新计算依赖信息并同步最新执行上下文。
func (s *Store) refreshAIBillingSessionSummaryTx(ctx context.Context, tx pgx.Tx, sessionID string, message string, payload []byte) (*domain.AIBillingSession, []domain.AIBillingItem, error) {
	var plannedCredits int64
	var billedCredits int64
	var refundedCredits int64
	var billedCount int64
	var refundedCount int64
	if err := tx.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(planned_credits), 0)::BIGINT,
			COALESCE(SUM(billed_credits), 0)::BIGINT,
			COALESCE(SUM(refunded_credits), 0)::BIGINT,
			COALESCE(SUM(CASE WHEN status = 'billed' THEN 1 ELSE 0 END), 0)::BIGINT,
			COALESCE(SUM(CASE WHEN status = 'refunded' THEN 1 ELSE 0 END), 0)::BIGINT
		FROM ai_billing_items
		WHERE session_id = $1
	`, sessionID).Scan(&plannedCredits, &billedCredits, &refundedCredits, &billedCount, &refundedCount); err != nil {
		return nil, nil, err
	}

	status := "precharged"
	switch {
	case refundedCount > 0 && billedCount == 0:
		status = "refunded"
	case refundedCount > 0 && billedCount > 0:
		status = "partially_refunded"
	case billedCount > 0:
		status = "billed"
	}

	if _, err := tx.Exec(ctx, `
		UPDATE ai_billing_sessions
		SET planned_credits = $2,
		    billed_credits = $3,
		    refunded_credits = $4,
		    status = $5,
		    message = $6,
		    payload = COALESCE($7, payload),
		    updated_at = NOW()
		WHERE id = $1
	`, sessionID, plannedCredits, billedCredits, refundedCredits, status, trimOptionalString(stringPtr(strings.TrimSpace(message))), bytesOrNil(payload)); err != nil {
		return nil, nil, err
	}

	row := tx.QueryRow(ctx, `
		SELECT id, user_id, source_type, source_id, workflow_code, output_type, duration_seconds, segment_seconds,
		       special_rule_id, special_price_credits, planned_credits, billed_credits, refunded_credits, status,
		       message, payload, created_at, updated_at
		FROM ai_billing_sessions
		WHERE id = $1
	`, sessionID)
	session, err := scanAIBillingSession(row.Scan)
	if err != nil {
		return nil, nil, err
	}
	items, err := s.listAIBillingItemsBySessionTx(ctx, tx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	return session, items, nil
}

// 处理解码AI计费Item载荷相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func decodeAIBillingItemPayload(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

// 处理首个Non空值值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
