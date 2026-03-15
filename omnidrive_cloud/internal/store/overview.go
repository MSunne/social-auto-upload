package store

import (
	"context"
	"fmt"
	"strings"

	"omnidrive_cloud/internal/domain"
)

func (s *Store) GetOverviewSummary(ctx context.Context, ownerUserID string) (*OverviewSummary, error) {
	summary := &OverviewSummary{}

	if err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE((SELECT COUNT(*) FROM devices WHERE owner_user_id = $1), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM devices WHERE owner_user_id = $1 AND last_seen_at >= NOW() - INTERVAL '45 seconds'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM platform_accounts pa INNER JOIN devices d ON d.id = pa.device_id WHERE d.owner_user_id = $1), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM device_material_roots mr INNER JOIN devices d ON d.id = mr.device_id WHERE d.owner_user_id = $1 AND mr.is_available = TRUE), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM device_material_entries me INNER JOIN devices d ON d.id = me.device_id WHERE d.owner_user_id = $1 AND me.is_available = TRUE), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM product_skills WHERE owner_user_id = $1), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_tasks pt INNER JOIN devices d ON d.id = pt.device_id WHERE d.owner_user_id = $1), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_tasks pt INNER JOIN devices d ON d.id = pt.device_id WHERE d.owner_user_id = $1 AND pt.status = 'pending'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_tasks pt INNER JOIN devices d ON d.id = pt.device_id WHERE d.owner_user_id = $1 AND pt.status = 'running'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_tasks pt INNER JOIN devices d ON d.id = pt.device_id WHERE d.owner_user_id = $1 AND pt.status = 'needs_verify'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM publish_tasks pt INNER JOIN devices d ON d.id = pt.device_id WHERE d.owner_user_id = $1 AND pt.status = 'failed'), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM login_sessions ls INNER JOIN devices d ON d.id = ls.device_id WHERE d.owner_user_id = $1 AND ls.status IN ('pending', 'running', 'verification_required')), 0)::BIGINT,
			COALESCE((SELECT COUNT(*) FROM ai_jobs WHERE owner_user_id = $1), 0)::BIGINT,
			COALESCE((SELECT balance_after FROM wallet_ledgers WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1), 0)::BIGINT
	`, ownerUserID).Scan(
		&summary.DeviceCount,
		&summary.OnlineDeviceCount,
		&summary.AccountCount,
		&summary.MaterialRootCount,
		&summary.MaterialEntryCount,
		&summary.SkillCount,
		&summary.TaskCount,
		&summary.PendingTaskCount,
		&summary.RunningTaskCount,
		&summary.NeedsVerifyTaskCount,
		&summary.FailedTaskCount,
		&summary.ActiveLoginSessionCount,
		&summary.AIJobCount,
		&summary.BalanceCredits,
	); err != nil {
		return nil, err
	}

	recentTasks, err := s.ListPublishTasksByOwner(ctx, ownerUserID, ListPublishTasksFilter{Limit: 5})
	if err != nil {
		return nil, err
	}
	summary.RecentTasks = recentTasks

	recentAIJobs, err := s.ListAIJobsByOwner(ctx, ownerUserID, "", "")
	if err != nil {
		return nil, err
	}
	if len(recentAIJobs) > 5 {
		recentAIJobs = recentAIJobs[:5]
	}
	summary.RecentAIJobs = recentAIJobs
	return summary, nil
}

func (s *Store) ListHistoryByOwner(ctx context.Context, ownerUserID string, filter ListHistoryFilter) ([]domain.HistoryItem, error) {
	query := `
		SELECT id, kind, title, status, source, message, created_at, updated_at, finished_at
		FROM (
			SELECT
				pt.id AS id,
				'publish' AS kind,
				pt.title AS title,
				pt.status AS status,
				pt.platform AS source,
				pt.message AS message,
				pt.created_at AS created_at,
				pt.updated_at AS updated_at,
				pt.finished_at AS finished_at
			FROM publish_tasks pt
			INNER JOIN devices d ON d.id = pt.device_id
			WHERE d.owner_user_id = $1

			UNION ALL

			SELECT
				aj.id AS id,
				aj.job_type AS kind,
				COALESCE(aj.prompt, aj.model_name) AS title,
				aj.status AS status,
				aj.model_name AS source,
				aj.message AS message,
				aj.created_at AS created_at,
				aj.updated_at AS updated_at,
				aj.finished_at AS finished_at
			FROM ai_jobs aj
			WHERE aj.owner_user_id = $1

			UNION ALL

			SELECT
				ae.id AS id,
				'audit' AS kind,
				ae.title AS title,
				ae.status AS status,
				ae.source AS source,
				ae.message AS message,
				ae.created_at AS created_at,
				ae.created_at AS updated_at,
				NULL::TIMESTAMPTZ AS finished_at
			FROM audit_events ae
			WHERE ae.owner_user_id = $1
		) history
		WHERE 1 = 1
	`
	args := []any{ownerUserID}
	argIndex := 2
	if kind := strings.TrimSpace(filter.Kind); kind != "" {
		query += fmt.Sprintf(" AND kind = $%d", argIndex)
		args = append(args, kind)
		argIndex++
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, status)
		argIndex++
	}
	query += " ORDER BY updated_at DESC"
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	query += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.HistoryItem, 0)
	for rows.Next() {
		var item domain.HistoryItem
		var message *string
		if scanErr := rows.Scan(
			&item.ID,
			&item.Kind,
			&item.Title,
			&item.Status,
			&item.Source,
			&message,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.FinishedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		item.Message = message
		items = append(items, item)
	}
	return items, rows.Err()
}
