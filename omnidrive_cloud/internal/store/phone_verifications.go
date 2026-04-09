package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const PhoneVerificationSceneRegister = "register"

type PhoneVerificationRecord struct {
	ID                   string
	Scene                string
	Phone                string
	CountryCode          string
	Status               string
	Provider             string
	ProviderRequestID    *string
	ProviderBizID        *string
	ProviderCode         *string
	ProviderMessage      *string
	VerificationCodeHash *string
	TemplateCode         *string
	ExpiresAt            time.Time
	VerifiedAt           *time.Time
	ConsumedAt           *time.Time
	AttemptCount         int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type CreatePhoneVerificationInput struct {
	ID           string
	Scene        string
	Phone        string
	CountryCode  string
	Provider     string
	TemplateCode string
	ExpiresAt    time.Time
}

// 处理扫描手机验证码相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func scanPhoneVerification(scan scanFn) (*PhoneVerificationRecord, error) {
	var item PhoneVerificationRecord
	if err := scan(
		&item.ID,
		&item.Scene,
		&item.Phone,
		&item.CountryCode,
		&item.Status,
		&item.Provider,
		&item.ProviderRequestID,
		&item.ProviderBizID,
		&item.ProviderCode,
		&item.ProviderMessage,
		&item.VerificationCodeHash,
		&item.TemplateCode,
		&item.ExpiresAt,
		&item.VerifiedAt,
		&item.ConsumedAt,
		&item.AttemptCount,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) CreatePhoneVerification(ctx context.Context, input CreatePhoneVerificationInput) (*PhoneVerificationRecord, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO phone_verification_codes (
			id,
			scene,
			phone,
			country_code,
			status,
			provider,
			template_code,
			expires_at
		)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7)
		RETURNING
			id,
			scene,
			phone,
			country_code,
			status,
			provider,
			provider_request_id,
			provider_biz_id,
			provider_code,
			provider_message,
			verification_code_hash,
			template_code,
			expires_at,
			verified_at,
			consumed_at,
			attempt_count,
			created_at,
			updated_at
	`, input.ID, input.Scene, strings.TrimSpace(input.Phone), strings.TrimSpace(input.CountryCode), strings.TrimSpace(input.Provider), strings.TrimSpace(input.TemplateCode), input.ExpiresAt.UTC())
	return scanPhoneVerification(row.Scan)
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) CountPhoneVerificationsSince(ctx context.Context, phone string, countryCode string, scene string, since time.Time) (int64, error) {
	var total int64
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM phone_verification_codes
		WHERE phone = $1
			AND country_code = $2
			AND scene = $3
			AND created_at >= $4
			AND status <> 'failed'
	`, strings.TrimSpace(phone), strings.TrimSpace(countryCode), strings.TrimSpace(scene), since.UTC()).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// 执行存储层相关的数据库查询，依赖上下文和连接池返回当前业务状态。
func (s *Store) GetLatestPhoneVerification(ctx context.Context, phone string, countryCode string, scene string) (*PhoneVerificationRecord, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			id,
			scene,
			phone,
			country_code,
			status,
			provider,
			provider_request_id,
			provider_biz_id,
			provider_code,
			provider_message,
			verification_code_hash,
			template_code,
			expires_at,
			verified_at,
			consumed_at,
			attempt_count,
			created_at,
			updated_at
		FROM phone_verification_codes
		WHERE phone = $1
			AND country_code = $2
			AND scene = $3
			AND status <> 'failed'
		ORDER BY created_at DESC
		LIMIT 1
	`, strings.TrimSpace(phone), strings.TrimSpace(countryCode), strings.TrimSpace(scene))

	item, err := scanPhoneVerification(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) MarkPhoneVerificationSent(ctx context.Context, id string, providerRequestID string, providerBizID string, providerCode string, providerMessage string, verificationCodeHash string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE phone_verification_codes
		SET
			status = 'sent',
			provider_request_id = NULLIF($2, ''),
			provider_biz_id = NULLIF($3, ''),
			provider_code = NULLIF($4, ''),
			provider_message = NULLIF($5, ''),
			verification_code_hash = NULLIF($6, ''),
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(id), strings.TrimSpace(providerRequestID), strings.TrimSpace(providerBizID), strings.TrimSpace(providerCode), strings.TrimSpace(providerMessage), strings.TrimSpace(verificationCodeHash))
	return err
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) MarkPhoneVerificationFailed(ctx context.Context, id string, providerCode string, providerMessage string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE phone_verification_codes
		SET
			status = 'failed',
			provider_code = NULLIF($2, ''),
			provider_message = NULLIF($3, ''),
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(id), strings.TrimSpace(providerCode), strings.TrimSpace(providerMessage))
	return err
}

// 处理Increment手机验证码Attempt相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (s *Store) IncrementPhoneVerificationAttempt(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE phone_verification_codes
		SET
			attempt_count = attempt_count + 1,
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(id))
	return err
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) MarkPhoneVerificationVerified(ctx context.Context, id string, providerCode string, providerMessage string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE phone_verification_codes
		SET
			status = 'verified',
			provider_code = NULLIF($2, ''),
			provider_message = NULLIF($3, ''),
			verified_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(id), strings.TrimSpace(providerCode), strings.TrimSpace(providerMessage))
	return err
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) MarkPhoneVerificationConsumed(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE phone_verification_codes
		SET
			status = 'consumed',
			consumed_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(id))
	return err
}

// 执行存储层相关的数据库写入，维护持久化状态与后续业务流转。
func (s *Store) MarkPhoneVerificationExpired(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE phone_verification_codes
		SET
			status = 'expired',
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(id))
	return err
}
