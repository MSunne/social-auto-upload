package store

import "context"

func (s *Store) CreateAuditEvent(ctx context.Context, input CreateAuditEventInput) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_events (
			id, owner_user_id, resource_type, resource_id, action, title, source, status, message, payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, input.ID, input.OwnerUserID, input.ResourceType, input.ResourceID, input.Action, input.Title, input.Source, input.Status, input.Message, input.Payload)
	return err
}

type CreateAdminAuditLogInput struct {
	ID           string
	AdminUserID  *string
	AdminEmail   *string
	AdminName    *string
	ResourceType string
	ResourceID   *string
	Action       string
	Title        string
	Source       string
	Status       string
	Message      *string
	Payload      []byte
}

func (s *Store) CreateAdminAuditLog(ctx context.Context, input CreateAdminAuditLogInput) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO admin_audit_logs (
			id, admin_user_id, admin_email, admin_name, resource_type, resource_id, action, title, source, status, message, payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, input.ID, input.AdminUserID, input.AdminEmail, input.AdminName, input.ResourceType, input.ResourceID, input.Action, input.Title, input.Source, input.Status, input.Message, input.Payload)
	return err
}
