package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

func (s *Store) GetAdminOrderByID(ctx context.Context, orderID string) (*domain.AdminOrderRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			ro.id, ro.order_no, ro.user_id, ro.package_id, ro.package_snapshot, ro.channel, ro.status, ro.subject, ro.body,
			ro.currency, ro.amount_cents, ro.credit_amount, ro.payment_payload, ro.customer_service_payload,
			ro.provider_transaction_id, ro.provider_status, ro.expires_at, ro.paid_at, ro.closed_at, ro.created_at, ro.updated_at,
			u.id, u.email, u.name
		FROM recharge_orders ro
		LEFT JOIN users u ON u.id = ro.user_id
		WHERE ro.id = $1
	`, orderID)

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

	err := row.Scan(
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
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
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
	return &item, nil
}
