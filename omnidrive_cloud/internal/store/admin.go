package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

type AdminPageFilter struct {
	Page     int
	PageSize int
}

type AdminUserListFilter struct {
	Query  string
	Status string
	AdminPageFilter
}

type AdminDeviceListFilter struct {
	Query  string
	Status string
	AdminPageFilter
}

type AdminOrderListFilter struct {
	Query   string
	Status  string
	Channel string
	AdminPageFilter
}

type AdminWalletLedgerListFilter struct {
	Query     string
	EntryType string
	AdminPageFilter
}

type AdminAuditListFilter struct {
	Query        string
	ResourceType string
	Status       string
	AdminPageFilter
}

func normalizeAdminPage(page int, pageSize int) (int, int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize, (page - 1) * pageSize
}

func ilikePattern(value string) string {
	return "%" + strings.TrimSpace(value) + "%"
}

func paidRechargeStatuses() []string {
	return []string{"paid", "credited", "success", "completed"}
}

const adminAuditEntriesBaseQuery = `
	WITH entries AS (
		SELECT
			ae.id,
			'user'::TEXT AS actor_type,
			ae.owner_user_id,
			u.email AS owner_email,
			u.name AS owner_name,
			NULL::TEXT AS admin_id,
			NULL::TEXT AS admin_email,
			NULL::TEXT AS admin_name,
			ae.resource_type,
			ae.resource_id,
			ae.action,
			ae.title,
			ae.source,
			ae.status,
			ae.message,
			ae.payload,
			ae.created_at
		FROM audit_events ae
		LEFT JOIN users u ON u.id = ae.owner_user_id
		UNION ALL
		SELECT
			aal.id,
			'admin'::TEXT AS actor_type,
			NULL::TEXT AS owner_user_id,
			NULL::TEXT AS owner_email,
			NULL::TEXT AS owner_name,
			COALESCE(au.id, aal.admin_user_id) AS admin_id,
			COALESCE(au.email, aal.admin_email) AS admin_email,
			COALESCE(au.name, aal.admin_name) AS admin_name,
			aal.resource_type,
			aal.resource_id,
			aal.action,
			aal.title,
			aal.source,
			aal.status,
			aal.message,
			aal.payload,
			aal.created_at
		FROM admin_audit_logs aal
		LEFT JOIN admin_users au ON au.id = aal.admin_user_id
	)
`

func scanAdminAuditRows(rows pgx.Rows) ([]domain.AdminAuditRow, error) {
	defer rows.Close()

	items := make([]domain.AdminAuditRow, 0)
	for rows.Next() {
		var item domain.AdminAuditRow
		var actorType string
		var payload []byte
		var ownerID *string
		var ownerEmail *string
		var ownerName *string
		var adminID *string
		var adminEmail *string
		var adminName *string
		if scanErr := rows.Scan(
			&item.ID,
			&actorType,
			&ownerID,
			&ownerEmail,
			&ownerName,
			&adminID,
			&adminEmail,
			&adminName,
			&item.ResourceType,
			&item.ResourceID,
			&item.Action,
			&item.Title,
			&item.Source,
			&item.Status,
			&item.Message,
			&payload,
			&item.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		item.ActorType = actorType
		if ownerID != nil {
			item.OwnerUserID = *ownerID
		}
		item.Payload = bytesOrNil(payload)
		if ownerID != nil && ownerEmail != nil && ownerName != nil {
			item.OwnerUser = &domain.AdminUserSummary{
				ID:    *ownerID,
				Email: *ownerEmail,
				Name:  *ownerName,
			}
		}
		if adminID != nil && adminEmail != nil && adminName != nil {
			item.Admin = &domain.AdminSummary{
				ID:    *adminID,
				Email: *adminEmail,
				Name:  *adminName,
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListRecentAdminAuditsByUserID(ctx context.Context, userID string, limit int) ([]domain.AdminAuditRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, adminAuditEntriesBaseQuery+`
		SELECT
			entries.id,
			entries.actor_type,
			entries.owner_user_id,
			entries.owner_email,
			entries.owner_name,
			entries.admin_id,
			entries.admin_email,
			entries.admin_name,
			entries.resource_type,
			entries.resource_id,
			entries.action,
			entries.title,
			entries.source,
			entries.status,
			entries.message,
			entries.payload,
			entries.created_at
		FROM entries
		WHERE entries.owner_user_id = $1
		   OR (entries.resource_type = 'user' AND entries.resource_id = $1)
		ORDER BY entries.created_at DESC
		LIMIT $2
	`, strings.TrimSpace(userID), limit)
	if err != nil {
		return nil, err
	}
	return scanAdminAuditRows(rows)
}

func (s *Store) ListRecentAdminAuditsByMediaAccountID(ctx context.Context, accountID string, limit int) ([]domain.AdminAuditRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, adminAuditEntriesBaseQuery+`
		SELECT
			entries.id,
			entries.actor_type,
			entries.owner_user_id,
			entries.owner_email,
			entries.owner_name,
			entries.admin_id,
			entries.admin_email,
			entries.admin_name,
			entries.resource_type,
			entries.resource_id,
			entries.action,
			entries.title,
			entries.source,
			entries.status,
			entries.message,
			entries.payload,
			entries.created_at
		FROM entries
		WHERE (
			entries.resource_type IN ('account', 'media_account')
			AND entries.resource_id = $1
		) OR (
			entries.resource_type = 'login_session'
			AND COALESCE(entries.payload->>'accountId', '') = $1
		)
		ORDER BY entries.created_at DESC
		LIMIT $2
	`, strings.TrimSpace(accountID), limit)
	if err != nil {
		return nil, err
	}
	return scanAdminAuditRows(rows)
}

func (s *Store) GetAdminDashboardSummary(ctx context.Context) (*domain.AdminDashboardSummary, error) {
	summary := &domain.AdminDashboardSummary{
		ServerTime: time.Now().UTC(),
	}

	err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE((SELECT COUNT(*) FROM users), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM users WHERE is_active = TRUE), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM devices), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM devices WHERE last_seen_at IS NOT NULL AND last_seen_at >= NOW() - INTERVAL '45 seconds'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_tasks), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_tasks WHERE status = 'failed'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_tasks WHERE status = 'needs_verify'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM ai_jobs), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM ai_jobs WHERE status = 'failed'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM ai_jobs WHERE status = 'queued'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM ai_jobs WHERE status = 'running'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM recharge_orders), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM recharge_orders WHERE paid_at IS NOT NULL OR status = ANY($1)), 0)::BIGINT,
			COALESCE((SELECT SUM(amount_cents) FROM recharge_orders WHERE paid_at IS NOT NULL OR status = ANY($1)), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM wallet_ledgers), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM recharge_orders WHERE channel = 'manual_cs' AND status = 'processing'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM recharge_orders WHERE channel = 'manual_cs'), 0)::BIGINT,
			COALESCE((SELECT SUM(GREATEST(amount_cents - released_amount_cents, 0)) FROM distribution_commission_items), 0)::BIGINT,
			COALESCE((SELECT SUM(GREATEST(released_amount_cents - settled_amount_cents, 0)) FROM distribution_commission_items), 0)::BIGINT,
			COALESCE((SELECT SUM(settled_amount_cents) FROM distribution_commission_items), 0)::BIGINT,
			COALESCE((SELECT SUM(amount_cents) FROM withdrawal_requests WHERE status IN ('requested', 'approved')), 0)::BIGINT
	`, paidRechargeStatuses()).Scan(
		&summary.Metrics.UserCount,
		&summary.Metrics.ActiveUserCount,
		&summary.Metrics.DeviceCount,
		&summary.Metrics.OnlineDeviceCount,
		&summary.Metrics.PublishTaskCount,
		&summary.Metrics.FailedPublishTaskCount,
		&summary.Queues.NeedsVerifyTaskCount,
		&summary.Metrics.AIJobCount,
		&summary.Metrics.FailedAIJobCount,
		&summary.Queues.PendingAIJobCount,
		&summary.Queues.RunningAIJobCount,
		&summary.Finance.OrderCount,
		&summary.Finance.PaidOrderCount,
		&summary.Finance.RechargeAmountCents,
		&summary.Finance.WalletLedgerCount,
		&summary.Finance.PendingSupportRechargeCount,
		&summary.Finance.ManualSupportRechargeCount,
		&summary.Distribution.PendingConsumeAmountCents,
		&summary.Distribution.PendingSettlementAmountCents,
		&summary.Distribution.SettledAmountCents,
		&summary.Distribution.PendingWithdrawalAmountCents,
	)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

func (s *Store) ListAdminUsers(ctx context.Context, filter AdminUserListFilter) ([]domain.AdminUserRow, int64, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(u.email ILIKE $%d OR u.name ILIKE $%d OR u.id ILIKE $%d OR COALESCE(u.notes, '') ILIKE $%d)", argIndex, argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}

	switch strings.TrimSpace(filter.Status) {
	case "active":
		whereParts = append(whereParts, fmt.Sprintf("u.is_active = $%d", argIndex))
		args = append(args, true)
		argIndex++
	case "inactive":
		whereParts = append(whereParts, fmt.Sprintf("u.is_active = $%d", argIndex))
		args = append(args, false)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM users u
		%s
	`, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
			SELECT
				u.id,
				u.email,
				u.name,
				u.is_active,
				u.notes,
				u.created_at,
				u.updated_at,
			COALESCE(bw.credit_balance, 0)::BIGINT,
			COALESCE(bw.frozen_credit_balance, 0)::BIGINT,
			COALESCE(ro.total_recharge_amount_cents, 0)::BIGINT,
			COALESCE(ro.total_recharge_count, 0)::BIGINT,
			COALESCE(wl.total_consume_credits, 0)::BIGINT,
			COALESCE(dev.device_count, 0)::BIGINT,
			COALESCE(acc.account_count, 0)::BIGINT,
			COALESCE(pt.publish_task_count, 0)::BIGINT,
			COALESCE(ai.ai_job_count, 0)::BIGINT
		FROM users u
		LEFT JOIN billing_wallets bw ON bw.user_id = u.id
		LEFT JOIN (
			SELECT user_id, COUNT(*)::BIGINT AS total_recharge_count,
			       COALESCE(SUM(amount_cents) FILTER (WHERE paid_at IS NOT NULL OR status = ANY($1)), 0)::BIGINT AS total_recharge_amount_cents
			FROM recharge_orders
			GROUP BY user_id
		) ro ON ro.user_id = u.id
		LEFT JOIN (
			SELECT user_id, COALESCE(SUM(ABS(amount_delta)) FILTER (WHERE amount_delta < 0), 0)::BIGINT AS total_consume_credits
			FROM wallet_ledgers
			GROUP BY user_id
		) wl ON wl.user_id = u.id
		LEFT JOIN (
			SELECT owner_user_id AS user_id, COUNT(*)::BIGINT AS device_count
			FROM devices
			WHERE owner_user_id IS NOT NULL
			GROUP BY owner_user_id
		) dev ON dev.user_id = u.id
		LEFT JOIN (
			SELECT d.owner_user_id AS user_id, COUNT(*)::BIGINT AS account_count
			FROM devices d
			INNER JOIN platform_accounts pa ON pa.device_id = d.id
			WHERE d.owner_user_id IS NOT NULL
			GROUP BY d.owner_user_id
		) acc ON acc.user_id = u.id
		LEFT JOIN (
			SELECT d.owner_user_id AS user_id, COUNT(*)::BIGINT AS publish_task_count
			FROM devices d
			INNER JOIN publish_tasks pt ON pt.device_id = d.id
			WHERE d.owner_user_id IS NOT NULL
			GROUP BY d.owner_user_id
		) pt ON pt.user_id = u.id
		LEFT JOIN (
			SELECT owner_user_id AS user_id, COUNT(*)::BIGINT AS ai_job_count
			FROM ai_jobs
			GROUP BY owner_user_id
		) ai ON ai.user_id = u.id
		%s
		ORDER BY u.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append([]any{paidRechargeStatuses()}, append(args, pageSize, offset)...)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.AdminUserRow, 0)
	for rows.Next() {
		var item domain.AdminUserRow
		var notes *string
		if scanErr := rows.Scan(
			&item.User.ID,
			&item.User.Email,
			&item.User.Name,
			&item.User.IsActive,
			&notes,
			&item.User.CreatedAt,
			&item.User.UpdatedAt,
			&item.Billing.CreditBalance,
			&item.Billing.FrozenCreditBalance,
			&item.Billing.TotalRechargeAmountCents,
			&item.Billing.TotalRechargeCount,
			&item.Billing.TotalConsumeCredits,
			&item.Assets.DeviceCount,
			&item.Assets.MediaAccountCount,
			&item.Assets.PublishTaskCount,
			&item.Assets.AIJobCount,
		); scanErr != nil {
			return nil, 0, scanErr
		}
		item.Notes = notes
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Store) ListAdminDevices(ctx context.Context, filter AdminDeviceListFilter) ([]domain.AdminDeviceRow, int64, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(devices.name ILIKE $%d OR devices.device_code ILIKE $%d OR COALESCE(devices.local_ip, '') ILIKE $%d OR COALESCE(devices.public_ip, '') ILIKE $%d)", argIndex, argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}

	switch strings.TrimSpace(filter.Status) {
	case "online":
		whereParts = append(whereParts, "devices.last_seen_at IS NOT NULL AND devices.last_seen_at >= NOW() - INTERVAL '45 seconds'")
	case "offline":
		whereParts = append(whereParts, "(devices.last_seen_at IS NULL OR devices.last_seen_at < NOW() - INTERVAL '45 seconds')")
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM devices
		%s
	`, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			%s,
			%s,
			u.id,
			u.email,
			u.name
		FROM devices
		LEFT JOIN users u ON u.id = devices.owner_user_id
		%s
		ORDER BY devices.updated_at DESC
		LIMIT $%d OFFSET $%d
	`, deviceSelectColumns, deviceLoadColumns, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.AdminDeviceRow, 0)
	for rows.Next() {
		var item domain.AdminDeviceRow
		var localIP *string
		var publicIP *string
		var model *string
		var notes *string
		var agentKey *string
		var runtimePayload []byte
		var ownerUserID *string
		var lastSeenAt *time.Time
		var ownerID *string
		var ownerEmail *string
		var ownerName *string

		if scanErr := rows.Scan(
			&item.Device.ID,
			&ownerUserID,
			&item.Device.DeviceCode,
			&agentKey,
			&item.Device.Name,
			&localIP,
			&publicIP,
			&model,
			&item.Device.IsEnabled,
			&runtimePayload,
			&lastSeenAt,
			&notes,
			&item.Device.CreatedAt,
			&item.Device.UpdatedAt,
			&item.Device.Load.AccountCount,
			&item.Device.Load.ActiveAccountCount,
			&item.Device.Load.MaterialRootCount,
			&item.Device.Load.MaterialEntryCount,
			&item.Device.Load.PendingTaskCount,
			&item.Device.Load.RunningTaskCount,
			&item.Device.Load.NeedsVerifyTaskCount,
			&item.Device.Load.CancelRequestedTaskCount,
			&item.Device.Load.FailedTaskCount,
			&item.Device.Load.ActiveLoginSessionCount,
			&item.Device.Load.VerificationLoginSessionCount,
			&ownerID,
			&ownerEmail,
			&ownerName,
		); scanErr != nil {
			return nil, 0, scanErr
		}

		item.Device.OwnerUserID = ownerUserID
		if agentKey != nil {
			item.Device.AgentKey = *agentKey
		}
		item.Device.LocalIP = localIP
		item.Device.PublicIP = publicIP
		item.Device.DefaultReasoningModel = model
		item.Device.RuntimePayload = bytesOrNil(runtimePayload)
		item.Device.LastSeenAt = lastSeenAt
		item.Device.Notes = notes
		item.Device.Status = computeDeviceStatus(lastSeenAt)

		if ownerID != nil && ownerEmail != nil && ownerName != nil {
			item.Owner = &domain.AdminUserSummary{
				ID:    *ownerID,
				Email: *ownerEmail,
				Name:  *ownerName,
			}
		}

		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Store) ListAdminOrders(ctx context.Context, filter AdminOrderListFilter) ([]domain.AdminOrderRow, int64, domain.AdminOrderListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(ro.order_no ILIKE $%d OR ro.subject ILIKE $%d OR u.email ILIKE $%d OR u.name ILIKE $%d)", argIndex, argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		whereParts = append(whereParts, fmt.Sprintf("ro.status = $%d", argIndex))
		args = append(args, status)
		argIndex++
	}
	if channel := strings.TrimSpace(filter.Channel); channel != "" {
		whereParts = append(whereParts, fmt.Sprintf("ro.channel = $%d", argIndex))
		args = append(args, channel)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var summary domain.AdminOrderListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*)::BIGINT,
			COALESCE(SUM(ro.amount_cents), 0)::BIGINT,
			COALESCE(SUM(ro.credit_amount), 0)::BIGINT,
			COALESCE(SUM(ro.manual_bonus_credit_amount), 0)::BIGINT,
			COUNT(*) FILTER (WHERE ro.paid_at IS NOT NULL OR ro.status = ANY($1))::BIGINT,
			COUNT(*) FILTER (WHERE ro.status = 'awaiting_manual_review')::BIGINT,
			COUNT(*) FILTER (WHERE ro.status = 'pending_payment')::BIGINT,
			COUNT(*) FILTER (WHERE ro.status = 'processing')::BIGINT,
			COUNT(*) FILTER (WHERE ro.status = 'rejected')::BIGINT,
			COUNT(*) FILTER (WHERE ro.channel = 'manual_cs')::BIGINT
		FROM recharge_orders ro
		LEFT JOIN users u ON u.id = ro.user_id
		%s
	`, whereClause), append([]any{paidRechargeStatuses()}, args...)...).Scan(
		&summary.TotalOrderCount,
		&summary.TotalAmountCents,
		&summary.TotalCreditAmount,
		&summary.TotalBonusCreditAmount,
		&summary.PaidOrderCount,
		&summary.AwaitingManualReviewCount,
		&summary.PendingPaymentCount,
		&summary.ProcessingCount,
		&summary.RejectedCount,
		&summary.ManualChannelCount,
	); err != nil {
		return nil, 0, domain.AdminOrderListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			ro.id, ro.order_no, ro.user_id, ro.package_id, ro.package_snapshot, ro.channel, ro.status, ro.subject, ro.body,
			ro.currency, ro.amount_cents, ro.credit_amount, ro.manual_bonus_credit_amount, ro.payment_payload, ro.customer_service_payload,
			ro.provider_transaction_id, ro.provider_status, ro.expires_at, ro.paid_at, ro.closed_at, ro.created_at, ro.updated_at,
			u.id, u.email, u.name
		FROM recharge_orders ro
		LEFT JOIN users u ON u.id = ro.user_id
		%s
		ORDER BY ro.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminOrderListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminOrderRow, 0)
	for rows.Next() {
		var item domain.AdminOrderRow
		var packageID *string
		var packageSnapshot []byte
		var body *string
		var paymentPayload []byte
		var customerServicePayload []byte
		var providerTransactionID *string
		var providerStatus *string
		var expiresAt *time.Time
		var paidAt *time.Time
		var closedAt *time.Time
		if scanErr := rows.Scan(
			&item.Order.ID,
			&item.Order.OrderNo,
			&item.Order.UserID,
			&packageID,
			&packageSnapshot,
			&item.Order.Channel,
			&item.Order.Status,
			&item.Order.Subject,
			&body,
			&item.Order.Currency,
			&item.Order.AmountCents,
			&item.Order.CreditAmount,
			&item.Order.ManualBonusCreditAmount,
			&paymentPayload,
			&customerServicePayload,
			&providerTransactionID,
			&providerStatus,
			&expiresAt,
			&paidAt,
			&closedAt,
			&item.Order.CreatedAt,
			&item.Order.UpdatedAt,
			&item.User.ID,
			&item.User.Email,
			&item.User.Name,
		); scanErr != nil {
			return nil, 0, domain.AdminOrderListSummary{}, scanErr
		}
		item.Order.PackageID = packageID
		item.Order.PackageSnapshot = bytesOrNil(packageSnapshot)
		item.Order.Body = body
		item.Order.PaymentPayload = bytesOrNil(paymentPayload)
		item.Order.CustomerServicePayload = bytesOrNil(customerServicePayload)
		item.Order.ProviderTransactionID = providerTransactionID
		item.Order.ProviderStatus = providerStatus
		item.Order.ExpiresAt = expiresAt
		item.Order.PaidAt = paidAt
		item.Order.ClosedAt = closedAt
		items = append(items, item)
	}
	return items, summary.TotalOrderCount, summary, rows.Err()
}

func (s *Store) ListAdminWalletLedgers(ctx context.Context, filter AdminWalletLedgerListFilter) ([]domain.AdminWalletLedgerRow, int64, domain.AdminWalletLedgerListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(u.email ILIKE $%d OR u.name ILIKE $%d OR COALESCE(wl.description, '') ILIKE $%d OR COALESCE(wl.reference_id, '') ILIKE $%d)", argIndex, argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}
	if entryType := strings.TrimSpace(filter.EntryType); entryType != "" {
		whereParts = append(whereParts, fmt.Sprintf("wl.entry_type = $%d", argIndex))
		args = append(args, entryType)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	var summary domain.AdminWalletLedgerListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*)::BIGINT,
			COALESCE(SUM(wl.amount_delta) FILTER (WHERE wl.amount_delta > 0), 0)::BIGINT,
			COALESCE(SUM(ABS(wl.amount_delta)) FILTER (WHERE wl.amount_delta < 0), 0)::BIGINT
		FROM wallet_ledgers wl
		INNER JOIN users u ON u.id = wl.user_id
		%s
	`, whereClause), args...).Scan(
		&summary.TotalEntryCount,
		&summary.TotalCreditIn,
		&summary.TotalCreditOut,
	); err != nil {
		return nil, 0, domain.AdminWalletLedgerListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			wl.id, wl.user_id, wl.entry_type, wl.amount_delta, wl.balance_before, wl.balance_after, wl.meter_code,
			wl.quantity, wl.unit, wl.unit_price_credits, wl.description, wl.reference_type, wl.reference_id,
			wl.recharge_order_id, wl.payment_transaction_id, wl.metadata, wl.created_at,
			u.id, u.email, u.name
		FROM wallet_ledgers wl
		INNER JOIN users u ON u.id = wl.user_id
		%s
		ORDER BY wl.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminWalletLedgerListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminWalletLedgerRow, 0)
	for rows.Next() {
		var item domain.AdminWalletLedgerRow
		var meterCode *string
		var quantity *int64
		var unit *string
		var unitPriceCredits *int64
		var description *string
		var referenceType *string
		var referenceID *string
		var rechargeOrderID *string
		var paymentTransactionID *string
		var metadata []byte

		if scanErr := rows.Scan(
			&item.Ledger.ID,
			&item.Ledger.UserID,
			&item.Ledger.EntryType,
			&item.Ledger.AmountDelta,
			&item.Ledger.BalanceBefore,
			&item.Ledger.BalanceAfter,
			&meterCode,
			&quantity,
			&unit,
			&unitPriceCredits,
			&description,
			&referenceType,
			&referenceID,
			&rechargeOrderID,
			&paymentTransactionID,
			&metadata,
			&item.Ledger.CreatedAt,
			&item.User.ID,
			&item.User.Email,
			&item.User.Name,
		); scanErr != nil {
			return nil, 0, domain.AdminWalletLedgerListSummary{}, scanErr
		}

		item.Ledger.MeterCode = meterCode
		item.Ledger.Quantity = quantity
		item.Ledger.Unit = unit
		item.Ledger.UnitPriceCredits = unitPriceCredits
		item.Ledger.Description = description
		item.Ledger.ReferenceType = referenceType
		item.Ledger.ReferenceID = referenceID
		item.Ledger.RechargeOrderID = rechargeOrderID
		item.Ledger.PaymentTransactionID = paymentTransactionID
		item.Ledger.Metadata = bytesOrNil(metadata)
		items = append(items, item)
	}
	return items, summary.TotalEntryCount, summary, rows.Err()
}

func (s *Store) ListAdminAudits(ctx context.Context, filter AdminAuditListFilter) ([]domain.AdminAuditRow, int64, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf(`(
			entries.title ILIKE $%d OR
			entries.action ILIKE $%d OR
			entries.source ILIKE $%d OR
			COALESCE(entries.message, '') ILIKE $%d OR
			COALESCE(entries.owner_email, '') ILIKE $%d OR
			COALESCE(entries.owner_name, '') ILIKE $%d OR
			COALESCE(entries.admin_email, '') ILIKE $%d OR
			COALESCE(entries.admin_name, '') ILIKE $%d
		)`, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}
	if resourceType := strings.TrimSpace(filter.ResourceType); resourceType != "" {
		whereParts = append(whereParts, fmt.Sprintf("entries.resource_type = $%d", argIndex))
		args = append(args, resourceType)
		argIndex++
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		whereParts = append(whereParts, fmt.Sprintf("entries.status = $%d", argIndex))
		args = append(args, status)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")
	baseQuery := adminAuditEntriesBaseQuery

	var total int64
	if err := s.pool.QueryRow(ctx, baseQuery+`
		SELECT COUNT(*)::BIGINT
		FROM entries
		`+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, baseQuery+fmt.Sprintf(`
		SELECT
			entries.id,
			entries.actor_type,
			entries.owner_user_id,
			entries.owner_email,
			entries.owner_name,
			entries.admin_id,
			entries.admin_email,
			entries.admin_name,
			entries.resource_type,
			entries.resource_id,
			entries.action,
			entries.title,
			entries.source,
			entries.status,
			entries.message,
			entries.payload,
			entries.created_at
		FROM entries
		%s
		ORDER BY entries.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	items, err := scanAdminAuditRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Store) GetBillingPackageByCode(ctx context.Context, packageID string) (*domain.BillingPackage, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, package_type, channel, payment_channels, currency, price_cents, credit_amount,
		       manual_bonus_credit_amount, badge, description, pricing_payload, expires_in_days, is_enabled, sort_order, created_at, updated_at
		FROM billing_packages
		WHERE id = $1
	`, packageID)
	item, err := scanBillingPackage(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}
