package store

import (
	"context"
	"errors"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

func scanMaterialRoot(row pgx.Row) (*domain.MaterialRoot, error) {
	var item domain.MaterialRoot
	if err := row.Scan(
		&item.ID,
		&item.DeviceID,
		&item.RootName,
		&item.RootPath,
		&item.IsAvailable,
		&item.IsDirectory,
		&item.LastSyncedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanMaterialEntry(row pgx.Row) (*domain.MaterialEntry, error) {
	var item domain.MaterialEntry
	var absolutePath *string
	var sizeBytes *int64
	var modifiedAt *string
	var extension *string
	var mimeType *string
	var previewText *string

	if err := row.Scan(
		&item.ID,
		&item.DeviceID,
		&item.RootName,
		&item.RootPath,
		&item.RelativePath,
		&item.ParentPath,
		&item.Name,
		&item.Kind,
		&absolutePath,
		&sizeBytes,
		&modifiedAt,
		&extension,
		&mimeType,
		&item.IsText,
		&previewText,
		&item.IsAvailable,
		&item.LastSyncedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}

	item.AbsolutePath = absolutePath
	item.SizeBytes = sizeBytes
	item.ModifiedAt = modifiedAt
	item.Extension = extension
	item.MimeType = mimeType
	item.PreviewText = previewText
	return &item, nil
}

func (s *Store) ListMaterialRootsByOwner(ctx context.Context, ownerUserID string, deviceID string) ([]domain.MaterialRoot, error) {
	query := `
		SELECT r.id, r.device_id, r.root_name, r.root_path, r.is_available, r.is_directory, r.last_synced_at, r.created_at, r.updated_at
		FROM device_material_roots r
		INNER JOIN devices d ON d.id = r.device_id
		WHERE d.owner_user_id = $1
	`
	args := []any{ownerUserID}
	if strings.TrimSpace(deviceID) != "" {
		query += ` AND r.device_id = $2`
		args = append(args, deviceID)
	}
	query += ` ORDER BY r.root_name ASC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.MaterialRoot, 0)
	for rows.Next() {
		item, scanErr := scanMaterialRoot(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) ListMaterialEntriesByOwner(ctx context.Context, ownerUserID string, deviceID string, rootName string, parentPath string) ([]domain.MaterialEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.device_id, e.root_name, e.root_path, e.relative_path, e.parent_path, e.name, e.kind,
		       e.absolute_path, e.size_bytes, e.modified_at, e.extension, e.mime_type, e.is_text,
		       e.preview_text, e.is_available, e.last_synced_at, e.created_at, e.updated_at
		FROM device_material_entries e
		INNER JOIN devices d ON d.id = e.device_id
		WHERE d.owner_user_id = $1
		  AND e.device_id = $2
		  AND e.root_name = $3
		  AND e.parent_path = $4
		  AND e.is_available = TRUE
		ORDER BY CASE WHEN e.kind = 'directory' THEN 0 ELSE 1 END ASC, LOWER(e.name) ASC
	`, ownerUserID, deviceID, rootName, normalizeMaterialPath(parentPath))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.MaterialEntry, 0)
	for rows.Next() {
		item, scanErr := scanMaterialEntry(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) GetMaterialRootByOwner(ctx context.Context, ownerUserID string, deviceID string, rootName string) (*domain.MaterialRoot, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT r.id, r.device_id, r.root_name, r.root_path, r.is_available, r.is_directory, r.last_synced_at, r.created_at, r.updated_at
		FROM device_material_roots r
		INNER JOIN devices d ON d.id = r.device_id
		WHERE d.owner_user_id = $1
		  AND r.device_id = $2
		  AND r.root_name = $3
	`, ownerUserID, deviceID, rootName)

	item, err := scanMaterialRoot(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) GetMaterialEntryByOwner(ctx context.Context, ownerUserID string, deviceID string, rootName string, relativePath string) (*domain.MaterialEntry, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT e.id, e.device_id, e.root_name, e.root_path, e.relative_path, e.parent_path, e.name, e.kind,
		       e.absolute_path, e.size_bytes, e.modified_at, e.extension, e.mime_type, e.is_text,
		       e.preview_text, e.is_available, e.last_synced_at, e.created_at, e.updated_at
		FROM device_material_entries e
		INNER JOIN devices d ON d.id = e.device_id
		WHERE d.owner_user_id = $1
		  AND e.device_id = $2
		  AND e.root_name = $3
		  AND e.relative_path = $4
	`, ownerUserID, deviceID, rootName, normalizeMaterialPath(relativePath))

	item, err := scanMaterialEntry(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) SyncMaterialRoots(ctx context.Context, deviceID string, roots []SyncMaterialRootInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `
		UPDATE device_material_roots
		SET is_available = FALSE,
		    updated_at = NOW()
		WHERE device_id = $1
	`, deviceID); err != nil {
		return err
	}

	for _, root := range roots {
		if _, err = tx.Exec(ctx, `
			INSERT INTO device_material_roots (
				id, device_id, root_name, root_path, is_available, is_directory, last_synced_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
			ON CONFLICT (device_id, root_name) DO UPDATE
			SET root_path = EXCLUDED.root_path,
			    is_available = EXCLUDED.is_available,
			    is_directory = EXCLUDED.is_directory,
			    last_synced_at = NOW(),
			    updated_at = NOW()
		`, uuid.NewString(), deviceID, root.RootName, root.RootPath, root.IsAvailable, root.IsDirectory); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) SyncMaterialDirectory(ctx context.Context, deviceID string, rootName string, rootPath string, directoryPath string, directoryAbsolutePath *string, entries []SyncMaterialEntryInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `
		INSERT INTO device_material_roots (id, device_id, root_name, root_path, is_available, is_directory, last_synced_at)
		VALUES ($1, $2, $3, $4, TRUE, TRUE, NOW())
		ON CONFLICT (device_id, root_name) DO UPDATE
		SET root_path = EXCLUDED.root_path,
		    is_available = TRUE,
		    is_directory = TRUE,
		    last_synced_at = NOW(),
		    updated_at = NOW()
	`, uuid.NewString(), deviceID, rootName, rootPath); err != nil {
		return err
	}

	parentPath := normalizeMaterialPath(directoryPath)
	if parentPath != "" {
		parentParent := normalizeMaterialParent(parentPath)
		parentName := path.Base(parentPath)
		if _, err = tx.Exec(ctx, `
			INSERT INTO device_material_entries (
				id, device_id, root_name, root_path, relative_path, parent_path, name, kind, absolute_path,
				is_available, last_synced_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'directory', $8, TRUE, NOW())
			ON CONFLICT (device_id, root_name, relative_path) DO UPDATE
			SET root_path = EXCLUDED.root_path,
			    parent_path = EXCLUDED.parent_path,
			    name = EXCLUDED.name,
			    kind = 'directory',
			    absolute_path = EXCLUDED.absolute_path,
			    is_available = TRUE,
			    last_synced_at = NOW(),
			    updated_at = NOW()
		`, uuid.NewString(), deviceID, rootName, rootPath, parentPath, parentParent, parentName, directoryAbsolutePath); err != nil {
			return err
		}
	}

	if _, err = tx.Exec(ctx, `
		UPDATE device_material_entries
		SET is_available = FALSE,
		    updated_at = NOW()
		WHERE device_id = $1 AND root_name = $2 AND parent_path = $3
	`, deviceID, rootName, parentPath); err != nil {
		return err
	}

	for _, entry := range entries {
		if _, err = tx.Exec(ctx, `
			INSERT INTO device_material_entries (
				id, device_id, root_name, root_path, relative_path, parent_path, name, kind, absolute_path,
				size_bytes, modified_at, extension, mime_type, is_text, preview_text, is_available, last_synced_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW())
			ON CONFLICT (device_id, root_name, relative_path) DO UPDATE
			SET root_path = EXCLUDED.root_path,
			    parent_path = EXCLUDED.parent_path,
			    name = EXCLUDED.name,
			    kind = EXCLUDED.kind,
			    absolute_path = EXCLUDED.absolute_path,
			    size_bytes = EXCLUDED.size_bytes,
			    modified_at = EXCLUDED.modified_at,
			    extension = EXCLUDED.extension,
			    mime_type = EXCLUDED.mime_type,
			    is_text = EXCLUDED.is_text,
			    preview_text = EXCLUDED.preview_text,
			    is_available = EXCLUDED.is_available,
			    last_synced_at = NOW(),
			    updated_at = NOW()
		`, uuid.NewString(), deviceID, rootName, rootPath, normalizeMaterialPath(entry.RelativePath), normalizeMaterialPath(entry.ParentPath),
			entry.Name, entry.Kind, entry.AbsolutePath, entry.SizeBytes, entry.ModifiedAt, entry.Extension, entry.MimeType, entry.IsText, entry.PreviewText, entry.IsAvailable); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) SyncMaterialFile(ctx context.Context, input SyncMaterialEntryInput) (*domain.MaterialEntry, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO device_material_entries (
			id, device_id, root_name, root_path, relative_path, parent_path, name, kind, absolute_path,
			size_bytes, modified_at, extension, mime_type, is_text, preview_text, is_available, last_synced_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW())
		ON CONFLICT (device_id, root_name, relative_path) DO UPDATE
		SET root_path = EXCLUDED.root_path,
		    parent_path = EXCLUDED.parent_path,
		    name = EXCLUDED.name,
		    kind = EXCLUDED.kind,
		    absolute_path = EXCLUDED.absolute_path,
		    size_bytes = EXCLUDED.size_bytes,
		    modified_at = EXCLUDED.modified_at,
		    extension = EXCLUDED.extension,
		    mime_type = EXCLUDED.mime_type,
		    is_text = EXCLUDED.is_text,
		    preview_text = EXCLUDED.preview_text,
		    is_available = EXCLUDED.is_available,
		    last_synced_at = NOW(),
		    updated_at = NOW()
		RETURNING id, device_id, root_name, root_path, relative_path, parent_path, name, kind,
		          absolute_path, size_bytes, modified_at, extension, mime_type, is_text,
		          preview_text, is_available, last_synced_at, created_at, updated_at
	`, uuid.NewString(), input.DeviceID, input.RootName, input.RootPath, normalizeMaterialPath(input.RelativePath), normalizeMaterialPath(input.ParentPath),
		input.Name, input.Kind, input.AbsolutePath, input.SizeBytes, input.ModifiedAt, input.Extension, input.MimeType, input.IsText, input.PreviewText, input.IsAvailable)

	return scanMaterialEntry(row)
}

func normalizeMaterialPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == "/" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimPrefix(value, "/")
	return path.Clean(value)
}

func normalizeMaterialParent(relativePath string) string {
	normalized := normalizeMaterialPath(relativePath)
	if normalized == "" {
		return ""
	}
	parent := path.Dir(normalized)
	if parent == "." || parent == "/" {
		return ""
	}
	return parent
}
