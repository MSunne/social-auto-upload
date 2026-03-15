package store

import (
	"context"
	"errors"

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

func (s *Store) ListSkillsByOwner(ctx context.Context, ownerUserID string) ([]domain.ProductSkill, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, owner_user_id, name, description, output_type, model_name,
		       prompt_template, reference_payload, is_enabled, created_at, updated_at
		FROM product_skills
		WHERE owner_user_id = $1
		ORDER BY updated_at DESC
	`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ProductSkill, 0)
	for rows.Next() {
		skill, scanErr := scanSkill(rows)
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
	row := s.pool.QueryRow(ctx, `
		SELECT id, owner_user_id, name, description, output_type, model_name,
		       prompt_template, reference_payload, is_enabled, created_at, updated_at
		FROM product_skills
		WHERE id = $1 AND owner_user_id = $2
	`, skillID, ownerUserID)

	skill, err := scanSkill(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return skill, nil
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
