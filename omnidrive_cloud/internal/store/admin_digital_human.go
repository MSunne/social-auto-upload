package store

import (
	"context"
	"fmt"
	"strings"

	"omnidrive_cloud/internal/domain"
)

func (s *Store) ListAdminDigitalHumanTasks(ctx context.Context, filter AdminDigitalHumanTaskListFilter) ([]domain.AdminDigitalHumanTaskRow, int64, domain.AdminDigitalHumanTaskListSummary, error) {
	page, pageSize, offset := normalizeAdminPage(filter.Page, filter.PageSize)
	_ = page

	whereParts := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if query := strings.TrimSpace(filter.Query); query != "" {
		whereParts = append(whereParts, fmt.Sprintf("(dht.id ILIKE $%[1]d OR COALESCE(dht.remote_task_id, '') ILIKE $%[1]d OR COALESCE(dht.goods_title, '') ILIKE $%[1]d OR COALESCE(dht.goods_text, '') ILIKE $%[1]d OR COALESCE(dht.error_message, '') ILIKE $%[1]d OR COALESCE(u.email, '') ILIKE $%[1]d OR COALESCE(u.name, '') ILIKE $%[1]d)", argIndex))
		args = append(args, ilikePattern(query))
		argIndex++
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		whereParts = append(whereParts, fmt.Sprintf("dht.status = $%d", argIndex))
		args = append(args, status)
		argIndex++
	}
	if mode := strings.TrimSpace(filter.Mode); mode != "" {
		whereParts = append(whereParts, fmt.Sprintf("dht.mode = $%d", argIndex))
		args = append(args, mode)
		argIndex++
	}
	if userID := strings.TrimSpace(filter.UserID); userID != "" {
		whereParts = append(whereParts, fmt.Sprintf("dht.owner_user_id = $%d", argIndex))
		args = append(args, userID)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(whereParts, " AND ")
	fromClause := `
		FROM digital_human_tasks dht
		LEFT JOIN users u ON u.id = dht.owner_user_id
	`

	var total int64
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) %s %s`, fromClause, whereClause), args...).Scan(&total); err != nil {
		return nil, 0, domain.AdminDigitalHumanTaskListSummary{}, err
	}

	var summary domain.AdminDigitalHumanTaskListSummary
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*)::BIGINT,
			COUNT(*) FILTER (WHERE dht.status = 'queued')::BIGINT,
			COUNT(*) FILTER (WHERE dht.status = 'running')::BIGINT,
			COUNT(*) FILTER (WHERE dht.status = 'completed')::BIGINT,
			COUNT(*) FILTER (WHERE dht.status = 'failed')::BIGINT,
			COUNT(*) FILTER (WHERE dht.status = 'cancelled')::BIGINT,
			COUNT(*) FILTER (WHERE dht.billing_status = 'settlement_pending')::BIGINT
		%s
		%s
	`, fromClause, whereClause), args...).Scan(
		&summary.TotalTaskCount,
		&summary.QueuedCount,
		&summary.RunningCount,
		&summary.CompletedCount,
		&summary.FailedCount,
		&summary.CancelledCount,
		&summary.SettlementPendingCount,
	); err != nil {
		return nil, 0, domain.AdminDigitalHumanTaskListSummary{}, err
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			dht.id,
			u.id,
			u.email,
			u.name
		%s
		%s
		ORDER BY dht.updated_at DESC, dht.created_at DESC
		LIMIT $%d OFFSET $%d
	`, fromClause, whereClause, argIndex, argIndex+1), append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, domain.AdminDigitalHumanTaskListSummary{}, err
	}
	defer rows.Close()

	items := make([]domain.AdminDigitalHumanTaskRow, 0)
	for rows.Next() {
		var (
			taskID     string
			ownerID    *string
			ownerEmail *string
			ownerName  *string
		)
		if scanErr := rows.Scan(&taskID, &ownerID, &ownerEmail, &ownerName); scanErr != nil {
			return nil, 0, domain.AdminDigitalHumanTaskListSummary{}, scanErr
		}

		task, taskErr := s.GetDigitalHumanTaskByID(ctx, taskID)
		if taskErr != nil {
			return nil, 0, domain.AdminDigitalHumanTaskListSummary{}, taskErr
		}
		if task == nil {
			continue
		}

		item := domain.AdminDigitalHumanTaskRow{Task: *task}
		if ownerID != nil && strings.TrimSpace(*ownerID) != "" {
			item.Owner = &domain.AdminUserSummary{
				ID:    strings.TrimSpace(*ownerID),
				Email: valueOrEmpty(ownerEmail),
				Name:  valueOrEmpty(ownerName),
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, domain.AdminDigitalHumanTaskListSummary{}, err
	}
	return items, total, summary, nil
}

func (s *Store) GetAdminDigitalHumanTaskByID(ctx context.Context, taskID string) (*domain.AdminDigitalHumanTaskRow, error) {
	task, err := s.GetDigitalHumanTaskByID(ctx, taskID)
	if err != nil || task == nil {
		return nil, err
	}

	row := s.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.name
		FROM digital_human_tasks dht
		LEFT JOIN users u ON u.id = dht.owner_user_id
		WHERE dht.id = $1
	`, strings.TrimSpace(taskID))

	var (
		ownerID    *string
		ownerEmail *string
		ownerName  *string
	)
	if err := row.Scan(&ownerID, &ownerEmail, &ownerName); err != nil {
		return nil, err
	}

	item := &domain.AdminDigitalHumanTaskRow{Task: *task}
	if ownerID != nil && strings.TrimSpace(*ownerID) != "" {
		item.Owner = &domain.AdminUserSummary{
			ID:    strings.TrimSpace(*ownerID),
			Email: valueOrEmpty(ownerEmail),
			Name:  valueOrEmpty(ownerName),
		}
	}
	return item, nil
}
