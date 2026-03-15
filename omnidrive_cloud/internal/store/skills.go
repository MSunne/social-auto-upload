package store

import (
	"context"
	"errors"
	"fmt"

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
	COALESCE((SELECT COUNT(*) FROM publish_tasks pt INNER JOIN devices d ON d.id = pt.device_id WHERE pt.skill_id = product_skills.id AND d.owner_user_id = product_skills.owner_user_id AND pt.status = 'failed'), 0)::BIGINT AS failed_task_count
`

func skillQueryWithLoad(whereClause string) string {
	return fmt.Sprintf(`
		SELECT %s, %s
		FROM product_skills
		%s
	`, skillSelectColumns, skillLoadColumns, whereClause)
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
		SELECT pt.id, pt.device_id, pt.account_id, pt.skill_id, pt.platform, pt.account_name,
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

func (s *Store) CreateSkillAsset(ctx context.Context, input CreateSkillAssetInput) (*domain.ProductSkillAsset, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO product_skill_assets (
			id, skill_id, owner_user_id, asset_type, file_name, mime_type,
			storage_key, public_url, size_bytes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, skill_id, owner_user_id, asset_type, file_name, mime_type,
		          storage_key, public_url, size_bytes, created_at, updated_at
	`, input.ID, input.SkillID, input.OwnerUserID, input.AssetType, input.FileName,
		input.MimeType, input.StorageKey, input.PublicURL, input.SizeBytes)

	return scanSkillAsset(row)
}

func (s *Store) GetSkillUsageSummary(ctx context.Context, skillID string, ownerUserID string) (int64, int64, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)::BIGINT,
		       COUNT(DISTINCT account_name)::BIGINT
		FROM publish_tasks pt
		INNER JOIN devices d ON d.id = pt.device_id
		WHERE pt.skill_id = $1 AND d.owner_user_id = $2
	`, skillID, ownerUserID)

	var taskCount int64
	var accountCount int64
	if err := row.Scan(&taskCount, &accountCount); err != nil {
		return 0, 0, err
	}
	return taskCount, accountCount, nil
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
