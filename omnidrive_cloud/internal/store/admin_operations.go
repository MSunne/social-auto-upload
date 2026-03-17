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

type AdminAccountListFilter struct {
	Query    string
	Status   string
	Platform string
	UserID   string
	DeviceID string
	AdminPageFilter
}

type AdminTaskListFilter struct {
	Query    string
	Status   string
	Platform string
	UserID   string
	DeviceID string
	SkillID  string
	AdminPageFilter
}

type AdminAIJobListFilter struct {
	Query    string
	Status   string
	JobType  string
	Source   string
	UserID   string
	DeviceID string
	SkillID  string
	AdminPageFilter
}

func adminDeviceSummary(id string, deviceCode string, name string, isEnabled bool, lastSeenAt *time.Time) domain.AdminDeviceSummary {
	return domain.AdminDeviceSummary{
		ID:         id,
		DeviceCode: deviceCode,
		Name:       name,
		Status:     computeDeviceStatus(lastSeenAt),
		IsEnabled:  isEnabled,
		LastSeenAt: lastSeenAt,
	}
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

const adminPlatformAccountSelectColumns = `
	pa.id, pa.device_id, pa.platform, pa.account_name, pa.status, pa.last_message,
	pa.last_authenticated_at, pa.notes, pa.created_at, pa.updated_at
`

const adminAIJobSelectColumns = `
	aj.id, aj.owner_user_id, aj.device_id, aj.skill_id, aj.source, aj.local_task_id,
	aj.job_type, aj.model_name, aj.prompt, aj.status, aj.input_payload, aj.output_payload,
	aj.message, aj.notes, aj.cost_credits, aj.lease_owner_device_id, aj.lease_token,
	aj.lease_expires_at, aj.delivery_status, aj.delivery_message, aj.local_publish_task_id,
	aj.created_at, aj.updated_at, aj.delivered_at, aj.finished_at
`

func scanAdminUserRow(scan scanFn) (*domain.AdminUserRow, error) {
	var item domain.AdminUserRow
	var notes *string
	if err := scan(
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
	); err != nil {
		return nil, err
	}
	item.Notes = notes
	return &item, nil
}

func scanAdminDeviceRow(scan scanFn) (*domain.AdminDeviceRow, error) {
	var item domain.AdminDeviceRow
	var localIP *string
	var publicIP *string
	var model *string
	var chatModel *string
	var imageModel *string
	var videoModel *string
	var notes *string
	var agentKey *string
	var runtimePayload []byte
	var ownerUserID *string
	var lastSeenAt *time.Time
	var ownerSummaryID *string
	var ownerEmail *string
	var ownerName *string

	if err := scan(
		&item.Device.ID,
		&ownerUserID,
		&item.Device.DeviceCode,
		&agentKey,
		&item.Device.Name,
		&localIP,
		&publicIP,
		&model,
		&chatModel,
		&imageModel,
		&videoModel,
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
		&item.Device.Load.LeasedTaskCount,
		&item.Device.Load.LeasedAIJobCount,
		&ownerSummaryID,
		&ownerEmail,
		&ownerName,
	); err != nil {
		return nil, err
	}

	item.Device.OwnerUserID = ownerUserID
	if agentKey != nil {
		item.Device.AgentKey = *agentKey
	}
	item.Device.LocalIP = localIP
	item.Device.PublicIP = publicIP
	item.Device.DefaultReasoningModel = model
	item.Device.DefaultChatModel = chatModel
	item.Device.DefaultImageModel = imageModel
	item.Device.DefaultVideoModel = videoModel
	item.Device.RuntimePayload = bytesOrNil(runtimePayload)
	item.Device.LastSeenAt = lastSeenAt
	item.Device.Notes = notes
	item.Device.Status = computeDeviceStatus(lastSeenAt)

	if ownerSummaryID != nil {
		item.Owner = &domain.AdminUserSummary{
			ID:    strings.TrimSpace(*ownerSummaryID),
			Email: stringOrEmpty(ownerEmail),
			Name:  stringOrEmpty(ownerName),
		}
	}

	return &item, nil
}

func scanAdminMediaAccountRow(scan scanFn) (*domain.AdminMediaAccountRow, error) {
	var item domain.AdminMediaAccountRow
	var lastMessage *string
	var lastAuthenticatedAt *time.Time
	var notes *string
	var ownerID *string
	var ownerEmail *string
	var ownerName *string
	var deviceLastSeenAt *time.Time

	if err := scan(
		&item.Account.ID,
		&item.Account.DeviceID,
		&item.Account.Platform,
		&item.Account.AccountName,
		&item.Account.Status,
		&lastMessage,
		&lastAuthenticatedAt,
		&notes,
		&item.Account.CreatedAt,
		&item.Account.UpdatedAt,
		&item.Account.Load.TaskCount,
		&item.Account.Load.PendingTaskCount,
		&item.Account.Load.RunningTaskCount,
		&item.Account.Load.NeedsVerifyTaskCount,
		&item.Account.Load.FailedTaskCount,
		&item.Account.Load.ActiveLoginSessionCount,
		&item.Account.Load.VerificationLoginSessionCount,
		&ownerID,
		&ownerEmail,
		&ownerName,
		&item.Device.ID,
		&item.Device.DeviceCode,
		&item.Device.Name,
		&item.Device.IsEnabled,
		&deviceLastSeenAt,
	); err != nil {
		return nil, err
	}

	item.Account.LastMessage = lastMessage
	item.Account.LastAuthenticatedAt = lastAuthenticatedAt
	item.Notes = notes
	item.Device.Status = computeDeviceStatus(deviceLastSeenAt)
	item.Device.LastSeenAt = deviceLastSeenAt

	if ownerID != nil {
		item.Owner = &domain.AdminUserSummary{
			ID:    strings.TrimSpace(*ownerID),
			Email: stringOrEmpty(ownerEmail),
			Name:  stringOrEmpty(ownerName),
		}
	}

	return &item, nil
}

func scanAdminPublishTaskRow(scan scanFn) (*domain.AdminPublishTaskRow, error) {
	var item domain.AdminPublishTaskRow
	var contentText *string
	var mediaPayload []byte
	var message *string
	var notes *string
	var verificationPayload []byte
	var accountID *string
	var skillID *string
	var skillRevision *string
	var leaseOwnerDeviceID *string
	var leaseToken *string
	var leaseExpiresAt *time.Time
	var cancelRequestedAt *time.Time
	var runAt *time.Time
	var finishedAt *time.Time
	var ownerID *string
	var ownerEmail *string
	var ownerName *string
	var deviceLastSeenAt *time.Time
	var accountSummaryID *string
	var accountStatus *string
	var accountLastMessage *string
	var accountLastAuthenticatedAt *time.Time
	var skillSummaryID *string
	var skillName *string
	var skillOutputType *string
	var skillModelName *string
	var skillIsEnabled *bool

	if err := scan(
		&item.Task.ID,
		&item.Task.DeviceID,
		&accountID,
		&skillID,
		&skillRevision,
		&item.Task.Platform,
		&item.Task.AccountName,
		&item.Task.Title,
		&contentText,
		&mediaPayload,
		&item.Task.Status,
		&message,
		&notes,
		&verificationPayload,
		&leaseOwnerDeviceID,
		&leaseToken,
		&leaseExpiresAt,
		&item.Task.AttemptCount,
		&cancelRequestedAt,
		&runAt,
		&finishedAt,
		&item.Task.CreatedAt,
		&item.Task.UpdatedAt,
		&ownerID,
		&ownerEmail,
		&ownerName,
		&item.Device.ID,
		&item.Device.DeviceCode,
		&item.Device.Name,
		&item.Device.IsEnabled,
		&deviceLastSeenAt,
		&accountSummaryID,
		&accountStatus,
		&accountLastMessage,
		&accountLastAuthenticatedAt,
		&skillSummaryID,
		&skillName,
		&skillOutputType,
		&skillModelName,
		&skillIsEnabled,
		&item.EventCount,
		&item.ArtifactCount,
		&item.MaterialCount,
	); err != nil {
		return nil, err
	}

	item.Task.AccountID = accountID
	item.Task.SkillID = skillID
	item.Task.SkillRevision = skillRevision
	item.Task.ContentText = contentText
	item.Task.MediaPayload = bytesOrNil(mediaPayload)
	item.Task.Message = message
	item.Notes = notes
	item.Task.VerificationPayload = bytesOrNil(verificationPayload)
	item.Task.LeaseOwnerDeviceID = leaseOwnerDeviceID
	item.Task.LeaseToken = leaseToken
	item.Task.LeaseExpiresAt = leaseExpiresAt
	item.Task.CancelRequestedAt = cancelRequestedAt
	item.Task.RunAt = runAt
	item.Task.FinishedAt = finishedAt
	item.Device.Status = computeDeviceStatus(deviceLastSeenAt)
	item.Device.LastSeenAt = deviceLastSeenAt

	if ownerID != nil {
		item.Owner = &domain.AdminUserSummary{
			ID:    strings.TrimSpace(*ownerID),
			Email: stringOrEmpty(ownerEmail),
			Name:  stringOrEmpty(ownerName),
		}
	}

	accountRefID := ""
	if accountSummaryID != nil {
		accountRefID = strings.TrimSpace(*accountSummaryID)
	} else if accountID != nil {
		accountRefID = strings.TrimSpace(*accountID)
	}
	if accountRefID != "" {
		item.Account = &domain.AdminAccountSummary{
			ID:                  accountRefID,
			Platform:            item.Task.Platform,
			AccountName:         item.Task.AccountName,
			Status:              stringOrEmpty(accountStatus),
			LastMessage:         accountLastMessage,
			LastAuthenticatedAt: accountLastAuthenticatedAt,
		}
	}

	skillRefID := ""
	if skillSummaryID != nil {
		skillRefID = strings.TrimSpace(*skillSummaryID)
	} else if skillID != nil {
		skillRefID = strings.TrimSpace(*skillID)
	}
	if skillRefID != "" {
		item.Skill = &domain.AdminSkillSummary{
			ID:         skillRefID,
			Name:       stringOrEmpty(skillName),
			OutputType: stringOrEmpty(skillOutputType),
			ModelName:  stringOrEmpty(skillModelName),
			IsEnabled:  skillIsEnabled != nil && *skillIsEnabled,
		}
	}

	return &item, nil
}

func scanAdminAIJobRow(scan scanFn) (*domain.AdminAIJobRow, error) {
	var item domain.AdminAIJobRow
	var deviceID *string
	var skillID *string
	var localTaskID *string
	var prompt *string
	var inputPayload []byte
	var outputPayload []byte
	var message *string
	var notes *string
	var leaseOwnerDeviceID *string
	var leaseToken *string
	var leaseExpiresAt *time.Time
	var deliveryMessage *string
	var localPublishTaskID *string
	var deliveredAt *time.Time
	var finishedAt *time.Time
	var ownerID *string
	var ownerEmail *string
	var ownerName *string
	var deviceSummaryID *string
	var deviceCode *string
	var deviceName *string
	var deviceIsEnabled *bool
	var deviceLastSeenAt *time.Time
	var skillSummaryID *string
	var skillName *string
	var skillOutputType *string
	var skillModelName *string
	var skillIsEnabled *bool
	var modelID *string
	var modelVendor *string
	var modelCategory *string
	var modelIsEnabled *bool

	if err := scan(
		&item.Job.ID,
		&item.Job.OwnerUserID,
		&deviceID,
		&skillID,
		&item.Job.Source,
		&localTaskID,
		&item.Job.JobType,
		&item.Job.ModelName,
		&prompt,
		&item.Job.Status,
		&inputPayload,
		&outputPayload,
		&message,
		&notes,
		&item.Job.CostCredits,
		&leaseOwnerDeviceID,
		&leaseToken,
		&leaseExpiresAt,
		&item.Job.DeliveryStatus,
		&deliveryMessage,
		&localPublishTaskID,
		&item.Job.CreatedAt,
		&item.Job.UpdatedAt,
		&deliveredAt,
		&finishedAt,
		&ownerID,
		&ownerEmail,
		&ownerName,
		&deviceSummaryID,
		&deviceCode,
		&deviceName,
		&deviceIsEnabled,
		&deviceLastSeenAt,
		&skillSummaryID,
		&skillName,
		&skillOutputType,
		&skillModelName,
		&skillIsEnabled,
		&modelID,
		&modelVendor,
		&modelCategory,
		&modelIsEnabled,
		&item.ArtifactCount,
		&item.MirroredArtifactCount,
		&item.PublishTaskCount,
	); err != nil {
		return nil, err
	}

	item.Job.DeviceID = deviceID
	item.Job.SkillID = skillID
	item.Job.LocalTaskID = localTaskID
	item.Job.Prompt = prompt
	item.Job.InputPayload = bytesOrNil(inputPayload)
	item.Job.OutputPayload = bytesOrNil(outputPayload)
	item.Job.Message = message
	item.Notes = notes
	item.Job.LeaseOwnerDeviceID = leaseOwnerDeviceID
	item.Job.LeaseToken = leaseToken
	item.Job.LeaseExpiresAt = leaseExpiresAt
	item.Job.DeliveryMessage = deliveryMessage
	item.Job.LocalPublishTaskID = localPublishTaskID
	item.Job.DeliveredAt = deliveredAt
	item.Job.FinishedAt = finishedAt

	if ownerID != nil {
		item.Owner = &domain.AdminUserSummary{
			ID:    strings.TrimSpace(*ownerID),
			Email: stringOrEmpty(ownerEmail),
			Name:  stringOrEmpty(ownerName),
		}
	}

	if deviceSummaryID != nil && deviceIsEnabled != nil {
		device := adminDeviceSummary(
			strings.TrimSpace(*deviceSummaryID),
			stringOrEmpty(deviceCode),
			stringOrEmpty(deviceName),
			*deviceIsEnabled,
			deviceLastSeenAt,
		)
		item.Device = &device
	}

	skillRefID := ""
	if skillSummaryID != nil {
		skillRefID = strings.TrimSpace(*skillSummaryID)
	} else if skillID != nil {
		skillRefID = strings.TrimSpace(*skillID)
	}
	if skillRefID != "" {
		item.Skill = &domain.AdminSkillSummary{
			ID:         skillRefID,
			Name:       stringOrEmpty(skillName),
			OutputType: stringOrEmpty(skillOutputType),
			ModelName:  stringOrEmpty(skillModelName),
			IsEnabled:  skillIsEnabled != nil && *skillIsEnabled,
		}
	}

	if modelID != nil {
		item.Model = &domain.AdminAIModelSummary{
			ID:        strings.TrimSpace(*modelID),
			Vendor:    stringOrEmpty(modelVendor),
			ModelName: item.Job.ModelName,
			Category:  stringOrEmpty(modelCategory),
			IsEnabled: modelIsEnabled != nil && *modelIsEnabled,
		}
	}

	return &item, nil
}

func (s *Store) GetAdminUserByID(ctx context.Context, userID string) (*domain.AdminUserRow, error) {
	row := s.pool.QueryRow(ctx, `
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
		WHERE u.id = $2
	`, paidRechargeStatuses(), userID)

	item, err := scanAdminUserRow(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) GetAdminDeviceByID(ctx context.Context, deviceID string) (*domain.AdminDeviceRow, error) {
	row := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s, %s,
		       u.id, u.email, u.name
		FROM devices
		LEFT JOIN users u ON u.id = devices.owner_user_id
		WHERE devices.id = $1
	`, deviceSelectColumnsQualified, deviceLoadColumns), deviceID)

	item, err := scanAdminDeviceRow(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) ListAdminAccounts(ctx context.Context, filter AdminAccountListFilter) ([]domain.AdminMediaAccountRow, int64, domain.AdminMediaAccountListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(pa.id ILIKE $%[1]d OR pa.platform ILIKE $%[1]d OR pa.account_name ILIKE $%[1]d OR d.name ILIKE $%[1]d OR d.device_code ILIKE $%[1]d OR COALESCE(pa.notes, '') ILIKE $%[1]d OR COALESCE(u.email, '') ILIKE $%[1]d OR COALESCE(u.name, '') ILIKE $%[1]d)", argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		whereParts = append(whereParts, fmt.Sprintf("pa.status = $%d", argIndex))
		args = append(args, status)
		argIndex++
	}
	if platform := strings.TrimSpace(filter.Platform); platform != "" {
		whereParts = append(whereParts, fmt.Sprintf("pa.platform = $%d", argIndex))
		args = append(args, platform)
		argIndex++
	}
	if userID := strings.TrimSpace(filter.UserID); userID != "" {
		whereParts = append(whereParts, fmt.Sprintf("d.owner_user_id = $%d", argIndex))
		args = append(args, userID)
		argIndex++
	}
	if deviceID := strings.TrimSpace(filter.DeviceID); deviceID != "" {
		whereParts = append(whereParts, fmt.Sprintf("pa.device_id = $%d", argIndex))
		args = append(args, deviceID)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")
	fromClause := `
		FROM platform_accounts pa
		INNER JOIN devices d ON d.id = pa.device_id
		LEFT JOIN users u ON u.id = d.owner_user_id
	`

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) %s %s`, fromClause, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, domain.AdminMediaAccountListSummary{}, err
	}

	var summary domain.AdminMediaAccountListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*)::BIGINT,
			COUNT(*) FILTER (WHERE pa.status = 'active')::BIGINT,
			COUNT(*) FILTER (WHERE pa.status IS DISTINCT FROM 'active')::BIGINT
		%s
		%s
	`, fromClause, whereClause), args...).Scan(
		&summary.TotalAccountCount,
		&summary.ActiveAccountCount,
		&summary.InactiveAccountCount,
	); err != nil {
		return nil, 0, domain.AdminMediaAccountListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
			SELECT
				%s, %s,
				u.id, u.email, u.name,
				d.id, d.device_code, d.name, d.is_enabled, d.last_seen_at
			%s
			%s
			ORDER BY pa.updated_at DESC
			LIMIT $%d OFFSET $%d
		`, adminPlatformAccountSelectColumns, platformAccountLoadColumns, fromClause, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminMediaAccountListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminMediaAccountRow, 0)
	for rows.Next() {
		item, scanErr := scanAdminMediaAccountRow(rows.Scan)
		if scanErr != nil {
			return nil, 0, domain.AdminMediaAccountListSummary{}, scanErr
		}
		items = append(items, *item)
	}
	return items, total, summary, rows.Err()
}

func (s *Store) GetAdminAccountByID(ctx context.Context, accountID string) (*domain.AdminMediaAccountRow, error) {
	row := s.pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT
				%s, %s,
				u.id, u.email, u.name,
				d.id, d.device_code, d.name, d.is_enabled, d.last_seen_at
			FROM platform_accounts pa
			INNER JOIN devices d ON d.id = pa.device_id
			LEFT JOIN users u ON u.id = d.owner_user_id
			WHERE pa.id = $1
		`, adminPlatformAccountSelectColumns, platformAccountLoadColumns), accountID)

	item, err := scanAdminMediaAccountRow(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) ListAdminTasks(ctx context.Context, filter AdminTaskListFilter) ([]domain.AdminPublishTaskRow, int64, domain.AdminPublishTaskListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(pt.id ILIKE $%[1]d OR pt.title ILIKE $%[1]d OR pt.platform ILIKE $%[1]d OR pt.account_name ILIKE $%[1]d OR COALESCE(pt.message, '') ILIKE $%[1]d OR COALESCE(pt.notes, '') ILIKE $%[1]d OR d.name ILIKE $%[1]d OR d.device_code ILIKE $%[1]d OR COALESCE(u.email, '') ILIKE $%[1]d OR COALESCE(u.name, '') ILIKE $%[1]d)", argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		whereParts = append(whereParts, fmt.Sprintf("pt.status = $%d", argIndex))
		args = append(args, status)
		argIndex++
	}
	if platform := strings.TrimSpace(filter.Platform); platform != "" {
		whereParts = append(whereParts, fmt.Sprintf("pt.platform = $%d", argIndex))
		args = append(args, platform)
		argIndex++
	}
	if userID := strings.TrimSpace(filter.UserID); userID != "" {
		whereParts = append(whereParts, fmt.Sprintf("d.owner_user_id = $%d", argIndex))
		args = append(args, userID)
		argIndex++
	}
	if deviceID := strings.TrimSpace(filter.DeviceID); deviceID != "" {
		whereParts = append(whereParts, fmt.Sprintf("pt.device_id = $%d", argIndex))
		args = append(args, deviceID)
		argIndex++
	}
	if skillID := strings.TrimSpace(filter.SkillID); skillID != "" {
		whereParts = append(whereParts, fmt.Sprintf("pt.skill_id = $%d", argIndex))
		args = append(args, skillID)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")
	fromClause := `
		FROM publish_tasks pt
		INNER JOIN devices d ON d.id = pt.device_id
		LEFT JOIN users u ON u.id = d.owner_user_id
		LEFT JOIN platform_accounts pa ON pa.id = pt.account_id
		LEFT JOIN product_skills ps ON ps.id = pt.skill_id AND ps.owner_user_id = d.owner_user_id
	`

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) %s %s`, fromClause, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, domain.AdminPublishTaskListSummary{}, err
	}

	var summary domain.AdminPublishTaskListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*)::BIGINT,
			COUNT(*) FILTER (WHERE pt.status = 'pending')::BIGINT,
			COUNT(*) FILTER (WHERE pt.status = 'running')::BIGINT,
			COUNT(*) FILTER (WHERE pt.status = 'needs_verify')::BIGINT,
			COUNT(*) FILTER (WHERE pt.status = 'cancel_requested')::BIGINT,
			COUNT(*) FILTER (WHERE pt.status = 'failed')::BIGINT,
			COUNT(*) FILTER (WHERE pt.status IN ('success', 'completed'))::BIGINT
		%s
		%s
	`, fromClause, whereClause), args...).Scan(
		&summary.TotalTaskCount,
		&summary.PendingCount,
		&summary.RunningCount,
		&summary.NeedsVerifyCount,
		&summary.CancelRequestedCount,
		&summary.FailedCount,
		&summary.CompletedCount,
	); err != nil {
		return nil, 0, domain.AdminPublishTaskListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
			pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.notes,
			pt.verification_payload, pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at,
			pt.attempt_count, pt.cancel_requested_at, pt.run_at, pt.finished_at, pt.created_at, pt.updated_at,
			u.id, u.email, u.name,
			d.id, d.device_code, d.name, d.is_enabled, d.last_seen_at,
			pa.id, pa.status, pa.last_message, pa.last_authenticated_at,
			ps.id, ps.name, ps.output_type, ps.model_name, ps.is_enabled,
			COALESCE((SELECT COUNT(*) FROM publish_task_events e WHERE e.task_id = pt.id), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_task_artifacts a WHERE a.task_id = pt.id), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_task_material_refs m WHERE m.task_id = pt.id), 0)::BIGINT
		%s
		%s
		ORDER BY pt.updated_at DESC
		LIMIT $%d OFFSET $%d
	`, fromClause, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminPublishTaskListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminPublishTaskRow, 0)
	for rows.Next() {
		item, scanErr := scanAdminPublishTaskRow(rows.Scan)
		if scanErr != nil {
			return nil, 0, domain.AdminPublishTaskListSummary{}, scanErr
		}
		items = append(items, *item)
	}
	return items, total, summary, rows.Err()
}

func (s *Store) GetAdminTaskByID(ctx context.Context, taskID string) (*domain.AdminPublishTaskRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
			pt.title, pt.content_text, pt.media_payload, pt.status, pt.message, pt.notes,
			pt.verification_payload, pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at,
			pt.attempt_count, pt.cancel_requested_at, pt.run_at, pt.finished_at, pt.created_at, pt.updated_at,
			u.id, u.email, u.name,
			d.id, d.device_code, d.name, d.is_enabled, d.last_seen_at,
			pa.id, pa.status, pa.last_message, pa.last_authenticated_at,
			ps.id, ps.name, ps.output_type, ps.model_name, ps.is_enabled,
			COALESCE((SELECT COUNT(*) FROM publish_task_events e WHERE e.task_id = pt.id), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_task_artifacts a WHERE a.task_id = pt.id), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_task_material_refs m WHERE m.task_id = pt.id), 0)::BIGINT
		FROM publish_tasks pt
		INNER JOIN devices d ON d.id = pt.device_id
		LEFT JOIN users u ON u.id = d.owner_user_id
		LEFT JOIN platform_accounts pa ON pa.id = pt.account_id
		LEFT JOIN product_skills ps ON ps.id = pt.skill_id AND ps.owner_user_id = d.owner_user_id
		WHERE pt.id = $1
	`, taskID)

	item, err := scanAdminPublishTaskRow(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) ListAdminAIJobs(ctx context.Context, filter AdminAIJobListFilter) ([]domain.AdminAIJobRow, int64, domain.AdminAIJobListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(aj.id ILIKE $%[1]d OR aj.job_type ILIKE $%[1]d OR aj.model_name ILIKE $%[1]d OR COALESCE(aj.prompt, '') ILIKE $%[1]d OR COALESCE(aj.message, '') ILIKE $%[1]d OR COALESCE(aj.notes, '') ILIKE $%[1]d OR COALESCE(u.email, '') ILIKE $%[1]d OR COALESCE(u.name, '') ILIKE $%[1]d OR COALESCE(d.name, '') ILIKE $%[1]d OR COALESCE(d.device_code, '') ILIKE $%[1]d)", argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		whereParts = append(whereParts, fmt.Sprintf("aj.status = $%d", argIndex))
		args = append(args, status)
		argIndex++
	}
	if jobType := strings.TrimSpace(filter.JobType); jobType != "" {
		whereParts = append(whereParts, fmt.Sprintf("aj.job_type = $%d", argIndex))
		args = append(args, jobType)
		argIndex++
	}
	if source := strings.TrimSpace(filter.Source); source != "" {
		whereParts = append(whereParts, fmt.Sprintf("aj.source = $%d", argIndex))
		args = append(args, source)
		argIndex++
	}
	if userID := strings.TrimSpace(filter.UserID); userID != "" {
		whereParts = append(whereParts, fmt.Sprintf("aj.owner_user_id = $%d", argIndex))
		args = append(args, userID)
		argIndex++
	}
	if deviceID := strings.TrimSpace(filter.DeviceID); deviceID != "" {
		whereParts = append(whereParts, fmt.Sprintf("aj.device_id = $%d", argIndex))
		args = append(args, deviceID)
		argIndex++
	}
	if skillID := strings.TrimSpace(filter.SkillID); skillID != "" {
		whereParts = append(whereParts, fmt.Sprintf("aj.skill_id = $%d", argIndex))
		args = append(args, skillID)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")
	fromClause := `
		FROM ai_jobs aj
		LEFT JOIN users u ON u.id = aj.owner_user_id
		LEFT JOIN devices d ON d.id = aj.device_id
		LEFT JOIN product_skills ps ON ps.id = aj.skill_id AND ps.owner_user_id = aj.owner_user_id
		LEFT JOIN ai_models am ON am.model_name = aj.model_name
	`

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) %s %s`, fromClause, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, domain.AdminAIJobListSummary{}, err
	}

	var summary domain.AdminAIJobListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*)::BIGINT,
			COUNT(*) FILTER (WHERE aj.status = 'queued')::BIGINT,
			COUNT(*) FILTER (WHERE aj.status = 'running')::BIGINT,
			COUNT(*) FILTER (WHERE aj.status IN ('success', 'completed'))::BIGINT,
			COUNT(*) FILTER (WHERE aj.status = 'failed')::BIGINT,
			COUNT(*) FILTER (WHERE aj.status = 'cancelled')::BIGINT,
			COUNT(*) FILTER (WHERE COALESCE(aj.delivery_status, '') IN ('pending', 'queued', 'importing', 'publishing', 'publish_queued'))::BIGINT
		%s
		%s
	`, fromClause, whereClause), args...).Scan(
		&summary.TotalJobCount,
		&summary.QueuedCount,
		&summary.RunningCount,
		&summary.CompletedCount,
		&summary.FailedCount,
		&summary.CancelledCount,
		&summary.PendingDeliveryCount,
	); err != nil {
		return nil, 0, domain.AdminAIJobListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			%s,
			u.id, u.email, u.name,
			d.id, d.device_code, d.name, d.is_enabled, d.last_seen_at,
			ps.id, ps.name, ps.output_type, ps.model_name, ps.is_enabled,
			am.id, am.vendor, am.category, am.is_enabled,
			COALESCE((SELECT COUNT(*) FROM ai_job_artifacts a WHERE a.job_id = aj.id), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM ai_job_artifacts a WHERE a.job_id = aj.id AND a.device_id IS NOT NULL AND a.root_name IS NOT NULL AND a.relative_path IS NOT NULL), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.id = aj.local_publish_task_id OR (pt.media_payload ->> 'aiJobId') = aj.id), 0)::BIGINT
		%s
		%s
		ORDER BY aj.updated_at DESC
		LIMIT $%d OFFSET $%d
	`, adminAIJobSelectColumns, fromClause, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminAIJobListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminAIJobRow, 0)
	for rows.Next() {
		item, scanErr := scanAdminAIJobRow(rows.Scan)
		if scanErr != nil {
			return nil, 0, domain.AdminAIJobListSummary{}, scanErr
		}
		items = append(items, *item)
	}
	return items, total, summary, rows.Err()
}

func (s *Store) GetAdminAIJobByID(ctx context.Context, jobID string) (*domain.AdminAIJobRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			`+adminAIJobSelectColumns+`,
			u.id, u.email, u.name,
			d.id, d.device_code, d.name, d.is_enabled, d.last_seen_at,
			ps.id, ps.name, ps.output_type, ps.model_name, ps.is_enabled,
			am.id, am.vendor, am.category, am.is_enabled,
			COALESCE((SELECT COUNT(*) FROM ai_job_artifacts a WHERE a.job_id = aj.id), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM ai_job_artifacts a WHERE a.job_id = aj.id AND a.device_id IS NOT NULL AND a.root_name IS NOT NULL AND a.relative_path IS NOT NULL), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_tasks pt WHERE pt.id = aj.local_publish_task_id OR (pt.media_payload ->> 'aiJobId') = aj.id), 0)::BIGINT
		FROM ai_jobs aj
		LEFT JOIN users u ON u.id = aj.owner_user_id
		LEFT JOIN devices d ON d.id = aj.device_id
		LEFT JOIN product_skills ps ON ps.id = aj.skill_id AND ps.owner_user_id = aj.owner_user_id
		LEFT JOIN ai_models am ON am.model_name = aj.model_name
		WHERE aj.id = $1
	`, jobID)

	item, err := scanAdminAIJobRow(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}
