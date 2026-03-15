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
