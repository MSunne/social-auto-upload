package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

func scanSkill(row pgx.Row) (*domain.ProductSkill, error) {
	var skill domain.ProductSkill
	var promptTemplate *string
	var referencePayload []byte

	if err := row.Scan(
		&skill.ID,
		&skill.OwnerUserID,
		&skill.Name,
		&skill.Description,
		&skill.OutputType,
		&skill.ModelName,
		&promptTemplate,
		&referencePayload,
		&skill.IsEnabled,
		&skill.CreatedAt,
		&skill.UpdatedAt,
	); err != nil {
		return nil, err
	}

	skill.PromptTemplate = promptTemplate
	skill.ReferencePayload = bytesOrNil(referencePayload)
	return &skill, nil
}

func scanSkillWithLoad(row pgx.Row) (*domain.ProductSkill, error) {
	var skill domain.ProductSkill
	var promptTemplate *string
	var referencePayload []byte

	if err := row.Scan(
		&skill.ID,
		&skill.OwnerUserID,
		&skill.Name,
		&skill.Description,
		&skill.OutputType,
		&skill.ModelName,
		&promptTemplate,
		&referencePayload,
		&skill.IsEnabled,
		&skill.CreatedAt,
		&skill.UpdatedAt,
		&skill.Load.AssetCount,
		&skill.Load.TaskCount,
		&skill.Load.PendingTaskCount,
		&skill.Load.RunningTaskCount,
		&skill.Load.NeedsVerifyTaskCount,
		&skill.Load.FailedTaskCount,
		&skill.Load.AIJobCount,
		&skill.Load.ActiveAIJobCount,
	); err != nil {
		return nil, err
	}

	skill.PromptTemplate = promptTemplate
	skill.ReferencePayload = bytesOrNil(referencePayload)
	return &skill, nil
}

const skillSelectColumns = `
	id, owner_user_id, name, description, output_type, model_name,
	prompt_template, reference_payload, is_enabled, created_at, updated_at
`

const skillLoadColumns = `
	COALESCE((SELECT COUNT(*) FROM product_skill_assets psa WHERE psa.skill_id = product_skills.id AND psa.owner_user_id = product_skills.owner_user_id), 0)::BIGINT AS asset_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt INNER JOIN devices d ON d.id = pt.device_id WHERE pt.skill_id = product_skills.id AND d.owner_user_id = product_skills.owner_user_id), 0)::BIGINT AS task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt INNER JOIN devices d ON d.id = pt.device_id WHERE pt.skill_id = product_skills.id AND d.owner_user_id = product_skills.owner_user_id AND pt.status = 'pending'), 0)::BIGINT AS pending_task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt INNER JOIN devices d ON d.id = pt.device_id WHERE pt.skill_id = product_skills.id AND d.owner_user_id = product_skills.owner_user_id AND pt.status = 'running'), 0)::BIGINT AS running_task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt INNER JOIN devices d ON d.id = pt.device_id WHERE pt.skill_id = product_skills.id AND d.owner_user_id = product_skills.owner_user_id AND pt.status = 'needs_verify'), 0)::BIGINT AS needs_verify_task_count,
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt INNER JOIN devices d ON d.id = pt.device_id WHERE pt.skill_id = product_skills.id AND d.owner_user_id = product_skills.owner_user_id AND pt.status = 'failed'), 0)::BIGINT AS failed_task_count,
	COALESCE((SELECT COUNT(*) FROM ai_jobs aj WHERE aj.owner_user_id = product_skills.owner_user_id AND aj.skill_id = product_skills.id), 0)::BIGINT AS ai_job_count,
	COALESCE((SELECT COUNT(*) FROM ai_jobs aj WHERE aj.owner_user_id = product_skills.owner_user_id AND aj.skill_id = product_skills.id AND aj.status IN ('queued', 'running')), 0)::BIGINT AS active_ai_job_count
`

func skillQueryWithLoad(whereClause string) string {
	return fmt.Sprintf(`
		SELECT %s, %s
		FROM product_skills
		%s
	`, skillSelectColumns, skillLoadColumns, whereClause)
}

func buildEffectiveSkillRevision(skillUpdatedAt time.Time, latestAssetUpdatedAt *time.Time, assetCount int64) string {
	parts := []string{
		skillUpdatedAt.UTC().Format(time.RFC3339Nano),
		fmt.Sprintf("%d", assetCount),
	}
	if latestAssetUpdatedAt != nil {
		parts = append(parts, latestAssetUpdatedAt.UTC().Format(time.RFC3339Nano))
	} else {
		parts = append(parts, "no-assets")
	}
	return strings.Join(parts, "|")
}

func trimmedStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *Store) ListSkillsByOwner(ctx context.Context, ownerUserID string) ([]domain.ProductSkill, error) {
	rows, err := s.pool.Query(ctx, skillQueryWithLoad(`
		WHERE owner_user_id = $1
		ORDER BY updated_at DESC
	`), ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ProductSkill, 0)
	for rows.Next() {
		skill, scanErr := scanSkillWithLoad(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *skill)
	}
	return items, rows.Err()
}

func (s *Store) ListEnabledSkillsByOwner(ctx context.Context, ownerUserID string, since *time.Time, limit int) ([]domain.ProductSkill, error) {
	query := skillQueryWithLoad(`
		WHERE owner_user_id = $1
		  AND is_enabled = TRUE
	`)
	args := []any{ownerUserID}
	if since != nil {
		query += ` AND updated_at > $2`
		args = append(args, since.UTC())
	}
	query += ` ORDER BY updated_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, len(args)+1)
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ProductSkill, 0)
	for rows.Next() {
		skill, scanErr := scanSkillWithLoad(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *skill)
	}
	return items, rows.Err()
}

func (s *Store) ListSkillsForAgentSyncByOwner(ctx context.Context, ownerUserID string, since *time.Time, limit int) ([]domain.ProductSkill, error) {
	query := skillQueryWithLoad(`
		WHERE owner_user_id = $1
	`)
	args := []any{ownerUserID}
	if since != nil {
		query += ` AND updated_at > $2`
		args = append(args, since.UTC())
	}
	query += ` ORDER BY updated_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, len(args)+1)
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ProductSkill, 0)
	for rows.Next() {
		skill, scanErr := scanSkillWithLoad(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *skill)
	}
	return items, rows.Err()
}

func (s *Store) GetSkillRevision(ctx context.Context, skillID string, ownerUserID string) (string, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			ps.updated_at,
			MAX(psa.updated_at),
			COUNT(psa.id)::BIGINT
		FROM product_skills ps
		LEFT JOIN product_skill_assets psa
		  ON psa.skill_id = ps.id
		 AND psa.owner_user_id = ps.owner_user_id
		WHERE ps.id = $1
		  AND ps.owner_user_id = $2
		GROUP BY ps.updated_at
	`, skillID, ownerUserID)

	var skillUpdatedAt time.Time
	var latestAssetUpdatedAt *time.Time
	var assetCount int64
	if err := row.Scan(&skillUpdatedAt, &latestAssetUpdatedAt, &assetCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return buildEffectiveSkillRevision(skillUpdatedAt, latestAssetUpdatedAt, assetCount), nil
}

func (s *Store) ListDeletedSkillEventsByOwner(ctx context.Context, ownerUserID string, since *time.Time, limit int) ([]domain.AgentRetiredSkillItem, error) {
	query := `
		SELECT resource_id, payload, message, created_at
		FROM audit_events
		WHERE owner_user_id = $1
		  AND resource_type = 'skill'
		  AND action = 'delete'
		  AND status = 'success'
	`
	args := []any{ownerUserID}
	if since != nil {
		query += ` AND created_at > $2`
		args = append(args, since.UTC())
	}
	query += ` ORDER BY created_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, len(args)+1)
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AgentRetiredSkillItem, 0)
	for rows.Next() {
		var skillID *string
		var payload []byte
		var message *string
		var createdAt time.Time
		if err := rows.Scan(&skillID, &payload, &message, &createdAt); err != nil {
			return nil, err
		}
		if skillID == nil || strings.TrimSpace(*skillID) == "" {
			continue
		}

		item := domain.AgentRetiredSkillItem{
			SkillID:       strings.TrimSpace(*skillID),
			Reason:        "deleted",
			Message:       message,
			LastChangedAt: createdAt.UTC(),
		}

		if len(payload) > 0 {
			var parsed struct {
				Name       *string `json:"name"`
				OutputType *string `json:"outputType"`
			}
			if jsonErr := json.Unmarshal(payload, &parsed); jsonErr == nil {
				item.Name = trimmedStringPointer(parsed.Name)
				item.OutputType = trimmedStringPointer(parsed.OutputType)
			}
		}

		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateSkill(ctx context.Context, input CreateSkillInput) (*domain.ProductSkill, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO product_skills (
			id, owner_user_id, name, description, output_type, model_name,
			prompt_template, reference_payload, is_enabled
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, owner_user_id, name, description, output_type, model_name,
		          prompt_template, reference_payload, is_enabled, created_at, updated_at
	`, input.ID, input.OwnerUserID, input.Name, input.Description, input.OutputType, input.ModelName,
		input.PromptTemplate, input.ReferencePayload, input.IsEnabled)

	return scanSkill(row)
}

func (s *Store) UpdateSkill(ctx context.Context, skillID string, ownerUserID string, input UpdateSkillInput) (*domain.ProductSkill, error) {
	referencePayload := any(nil)
	if input.ReferenceTouched {
		referencePayload = input.ReferencePayload
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE product_skills
		SET name = COALESCE($3, name),
		    description = COALESCE($4, description),
		    output_type = COALESCE($5, output_type),
		    model_name = COALESCE($6, model_name),
		    prompt_template = COALESCE($7, prompt_template),
		    reference_payload = COALESCE($8, reference_payload),
		    is_enabled = COALESCE($9, is_enabled),
		    updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2
		RETURNING id, owner_user_id, name, description, output_type, model_name,
		          prompt_template, reference_payload, is_enabled, created_at, updated_at
	`, skillID, ownerUserID, input.Name, input.Description, input.OutputType, input.ModelName,
		input.PromptTemplate, referencePayload, input.IsEnabled)

	skill, err := scanSkill(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return skill, nil
}

func (s *Store) GetOwnedSkillByID(ctx context.Context, skillID string, ownerUserID string) (*domain.ProductSkill, error) {
	row := s.pool.QueryRow(ctx, skillQueryWithLoad(`
		WHERE id = $1 AND owner_user_id = $2
	`), skillID, ownerUserID)

	skill, err := scanSkillWithLoad(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return skill, nil
}

func (s *Store) ListPublishTasksBySkill(ctx context.Context, ownerUserID string, skillID string, limit int) ([]domain.PublishTask, error) {
	query := `
		SELECT pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.skill_revision, pt.platform, pt.account_name,
		       pt.title, pt.content_text, pt.media_payload, pt.status, pt.message,
		       pt.verification_payload, pt.lease_owner_device_id, pt.lease_token, pt.lease_expires_at,
		       pt.attempt_count, pt.cancel_requested_at, pt.run_at, pt.finished_at, pt.created_at, pt.updated_at
		FROM publish_tasks pt
		INNER JOIN devices d ON d.id = pt.device_id
		WHERE pt.skill_id = $1 AND d.owner_user_id = $2
		ORDER BY pt.updated_at DESC
	`
	args := []any{skillID, ownerUserID}
	if limit > 0 {
		query += ` LIMIT $3`
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.PublishTask, 0)
	for rows.Next() {
		task, scanErr := scanPublishTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *task)
	}
	return items, rows.Err()
}

func scanSkillAsset(row pgx.Row) (*domain.ProductSkillAsset, error) {
	var asset domain.ProductSkillAsset
	var mimeType *string
	var storageKey *string
	var publicURL *string
	var sizeBytes *int64

	if err := row.Scan(
		&asset.ID,
		&asset.SkillID,
		&asset.OwnerUserID,
		&asset.AssetType,
		&asset.FileName,
		&mimeType,
		&storageKey,
		&publicURL,
		&sizeBytes,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	); err != nil {
		return nil, err
	}

	asset.MimeType = mimeType
	asset.StorageKey = storageKey
	asset.PublicURL = publicURL
	asset.SizeBytes = sizeBytes
	return &asset, nil
}

func scanDeviceSkillSyncState(row pgx.Row) (*domain.DeviceSkillSyncState, error) {
	var item domain.DeviceSkillSyncState
	var syncedRevision *string
	var message *string
	var lastSyncedAt *time.Time

	if err := row.Scan(
		&item.ID,
		&item.DeviceID,
		&item.SkillID,
		&item.SyncStatus,
		&syncedRevision,
		&item.AssetCount,
		&message,
		&lastSyncedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}

	item.SyncedRevision = syncedRevision
	item.Message = message
	item.LastSyncedAt = lastSyncedAt
	return &item, nil
}

func scanDeviceRetiredSkillAck(row pgx.Row) (*domain.DeviceRetiredSkillAck, error) {
	var item domain.DeviceRetiredSkillAck
	var message *string
	if err := row.Scan(
		&item.ID,
		&item.DeviceID,
		&item.SkillID,
		&item.Reason,
		&message,
		&item.LastAcknowledgedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.Message = message
	return &item, nil
}

func (s *Store) ListSkillAssets(ctx context.Context, skillID string, ownerUserID string) ([]domain.ProductSkillAsset, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, skill_id, owner_user_id, asset_type, file_name, mime_type,
		       storage_key, public_url, size_bytes, created_at, updated_at
		FROM product_skill_assets
		WHERE skill_id = $1 AND owner_user_id = $2
		ORDER BY created_at ASC
	`, skillID, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ProductSkillAsset, 0)
	for rows.Next() {
		asset, scanErr := scanSkillAsset(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *asset)
	}
	return items, rows.Err()
}

func (s *Store) ListSkillSyncStatesByDevice(ctx context.Context, ownerUserID string, deviceID string, limit int) ([]domain.DeviceSkillSyncState, error) {
	query := `
		SELECT dsss.id, dsss.device_id, dsss.skill_id, dsss.sync_status, dsss.synced_revision,
		       dsss.asset_count, dsss.message, dsss.last_synced_at, dsss.created_at, dsss.updated_at
		FROM device_skill_sync_states dsss
		INNER JOIN devices d ON d.id = dsss.device_id
		INNER JOIN product_skills ps ON ps.id = dsss.skill_id
		WHERE d.id = $1
		  AND d.owner_user_id = $2
		  AND ps.owner_user_id = $2
		ORDER BY COALESCE(dsss.last_synced_at, dsss.updated_at) DESC, dsss.updated_at DESC
	`
	args := []any{deviceID, ownerUserID}
	if limit > 0 {
		query += ` LIMIT $3`
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.DeviceSkillSyncState, 0)
	for rows.Next() {
		item, scanErr := scanDeviceSkillSyncState(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) ListSkillSyncStatesBySkill(ctx context.Context, ownerUserID string, skillID string, limit int) ([]domain.DeviceSkillSyncState, error) {
	query := `
		SELECT dsss.id, dsss.device_id, dsss.skill_id, dsss.sync_status, dsss.synced_revision,
		       dsss.asset_count, dsss.message, dsss.last_synced_at, dsss.created_at, dsss.updated_at
		FROM device_skill_sync_states dsss
		INNER JOIN devices d ON d.id = dsss.device_id
		INNER JOIN product_skills ps ON ps.id = dsss.skill_id
		WHERE dsss.skill_id = $1
		  AND d.owner_user_id = $2
		  AND ps.owner_user_id = $2
		ORDER BY COALESCE(dsss.last_synced_at, dsss.updated_at) DESC, dsss.updated_at DESC
	`
	args := []any{skillID, ownerUserID}
	if limit > 0 {
		query += ` LIMIT $3`
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.DeviceSkillSyncState, 0)
	for rows.Next() {
		item, scanErr := scanDeviceSkillSyncState(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) GetDeviceSkillSyncState(ctx context.Context, deviceID string, skillID string) (*domain.DeviceSkillSyncState, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, device_id, skill_id, sync_status, synced_revision, asset_count, message,
		       last_synced_at, created_at, updated_at
		FROM device_skill_sync_states
		WHERE device_id = $1 AND skill_id = $2
	`, deviceID, skillID)

	item, err := scanDeviceSkillSyncState(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) ListDeviceRetiredSkillAcks(ctx context.Context, deviceID string) ([]domain.DeviceRetiredSkillAck, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, device_id, skill_id, reason, message, last_acknowledged_at, created_at, updated_at
		FROM device_retired_skill_acks
		WHERE device_id = $1
		ORDER BY updated_at DESC
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.DeviceRetiredSkillAck, 0)
	for rows.Next() {
		item, scanErr := scanDeviceRetiredSkillAck(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) UpsertDeviceRetiredSkillAck(ctx context.Context, input UpsertDeviceRetiredSkillAckInput) (*domain.DeviceRetiredSkillAck, error) {
	ackTime := time.Now().UTC()
	if input.LastAcknowledgedAt != nil && !input.LastAcknowledgedAt.IsZero() {
		ackTime = input.LastAcknowledgedAt.UTC()
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO device_retired_skill_acks (
			id, device_id, skill_id, reason, message, last_acknowledged_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (device_id, skill_id, reason) DO UPDATE
		SET message = EXCLUDED.message,
		    last_acknowledged_at = EXCLUDED.last_acknowledged_at,
		    updated_at = NOW()
		RETURNING id, device_id, skill_id, reason, message, last_acknowledged_at, created_at, updated_at
	`, uuid.NewString(), input.DeviceID, input.SkillID, input.Reason, input.Message, ackTime)

	return scanDeviceRetiredSkillAck(row)
}

func (s *Store) UpsertDeviceSkillSyncState(ctx context.Context, input UpsertDeviceSkillSyncStateInput) (*domain.DeviceSkillSyncState, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO device_skill_sync_states (
			id, device_id, skill_id, sync_status, synced_revision, asset_count, message, last_synced_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (device_id, skill_id) DO UPDATE
		SET sync_status = EXCLUDED.sync_status,
		    synced_revision = EXCLUDED.synced_revision,
		    asset_count = EXCLUDED.asset_count,
		    message = EXCLUDED.message,
		    last_synced_at = EXCLUDED.last_synced_at,
		    updated_at = NOW()
		RETURNING id, device_id, skill_id, sync_status, synced_revision, asset_count, message,
		          last_synced_at, created_at, updated_at
	`, uuid.NewString(), input.DeviceID, input.SkillID, input.SyncStatus, input.SyncedRevision, input.AssetCount, input.Message, input.LastSyncedAt)

	return scanDeviceSkillSyncState(row)
}

func (s *Store) CreateSkillAsset(ctx context.Context, input CreateSkillAssetInput) (*domain.ProductSkillAsset, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		INSERT INTO product_skill_assets (
			id, skill_id, owner_user_id, asset_type, file_name, mime_type,
			storage_key, public_url, size_bytes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, skill_id, owner_user_id, asset_type, file_name, mime_type,
		          storage_key, public_url, size_bytes, created_at, updated_at
	`, input.ID, input.SkillID, input.OwnerUserID, input.AssetType, input.FileName,
		input.MimeType, input.StorageKey, input.PublicURL, input.SizeBytes)

	asset, err := scanSkillAsset(row)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE product_skills
		SET updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2
	`, input.SkillID, input.OwnerUserID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return asset, nil
}

func (s *Store) GetSkillUsageSummary(ctx context.Context, skillID string, ownerUserID string) (int64, int64, int64, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE((
				SELECT COUNT(*)
				FROM publish_tasks pt
				INNER JOIN devices d ON d.id = pt.device_id
				WHERE pt.skill_id = $1 AND d.owner_user_id = $2
			), 0)::BIGINT,
			COALESCE((
				SELECT COUNT(DISTINCT pt.account_name)
				FROM publish_tasks pt
				INNER JOIN devices d ON d.id = pt.device_id
				WHERE pt.skill_id = $1 AND d.owner_user_id = $2
			), 0)::BIGINT,
			COALESCE((
				SELECT COUNT(*)
				FROM ai_jobs aj
				WHERE aj.skill_id = $1 AND aj.owner_user_id = $2
			), 0)::BIGINT
	`, skillID, ownerUserID)

	var taskCount int64
	var accountCount int64
	var aiJobCount int64
	if err := row.Scan(&taskCount, &accountCount, &aiJobCount); err != nil {
		return 0, 0, 0, err
	}
	return taskCount, accountCount, aiJobCount, nil
}

func (s *Store) DeleteSkill(ctx context.Context, skillID string, ownerUserID string) (bool, error) {
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM product_skills
		WHERE id = $1 AND owner_user_id = $2
	`, skillID, ownerUserID)
	if err != nil {
		return false, err
	}
	return commandTag.RowsAffected() > 0, nil
}
