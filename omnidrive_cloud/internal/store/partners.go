package store

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

var (
	ErrPartnerCodeInvalid     = errors.New("partner code is invalid")
	ErrPartnerProfileUserMiss = errors.New("partner profile user not found")
)

const partnerCodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

func normalizePartnerCode(value string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, char := range trimmed {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func scanPartnerProfile(scan scanFn) (*domain.PartnerProfile, error) {
	var item domain.PartnerProfile
	if err := scan(
		&item.UserID,
		&item.PartnerCode,
		&item.PartnerName,
		&item.ContactName,
		&item.ContactPhone,
		&item.ContactWechat,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) GetPartnerProfileByUserID(ctx context.Context, userID string) (*domain.PartnerProfile, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT user_id, partner_code, partner_name, contact_name, contact_phone, contact_wechat, status, created_at, updated_at
		FROM partner_profiles
		WHERE user_id = $1
	`, strings.TrimSpace(userID))

	item, err := scanPartnerProfile(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func getPartnerProfileByCodeTx(ctx context.Context, tx pgx.Tx, partnerCode string) (*domain.PartnerProfile, error) {
	row := tx.QueryRow(ctx, `
		SELECT user_id, partner_code, partner_name, contact_name, contact_phone, contact_wechat, status, created_at, updated_at
		FROM partner_profiles
		WHERE partner_code = $1
		LIMIT 1
	`, normalizePartnerCode(partnerCode))

	item, err := scanPartnerProfile(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Store) OpenPartnerProfile(ctx context.Context, userID string) (*domain.PartnerProfile, error) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return nil, ErrPartnerProfileUserMiss
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	existing, err := getPartnerProfileByUserIDTx(ctx, tx, trimmedUserID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return existing, nil
	}

	var userName string
	if err := tx.QueryRow(ctx, `
		SELECT name
		FROM users
		WHERE id = $1
	`, trimmedUserID).Scan(&userName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPartnerProfileUserMiss
		}
		return nil, err
	}

	partnerName := strings.TrimSpace(userName)
	if partnerName == "" {
		partnerName = "企业合作伙伴"
	}

	var created *domain.PartnerProfile
	for attempts := 0; attempts < 8; attempts++ {
		partnerCode, genErr := generatePartnerCode()
		if genErr != nil {
			return nil, genErr
		}

		row := tx.QueryRow(ctx, `
			INSERT INTO partner_profiles (
				user_id, partner_code, partner_name, status
			)
			VALUES ($1, $2, $3, 'active')
			ON CONFLICT (partner_code) DO NOTHING
			RETURNING user_id, partner_code, partner_name, contact_name, contact_phone, contact_wechat, status, created_at, updated_at
		`, trimmedUserID, partnerCode, partnerName)

		created, err = scanPartnerProfile(row.Scan)
		if err == nil {
			break
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	if created == nil {
		return nil, fmt.Errorf("failed to allocate a unique partner code")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return created, nil
}

func getPartnerProfileByUserIDTx(ctx context.Context, tx pgx.Tx, userID string) (*domain.PartnerProfile, error) {
	row := tx.QueryRow(ctx, `
		SELECT user_id, partner_code, partner_name, contact_name, contact_phone, contact_wechat, status, created_at, updated_at
		FROM partner_profiles
		WHERE user_id = $1
	`, strings.TrimSpace(userID))

	item, err := scanPartnerProfile(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func generatePartnerCode() (string, error) {
	const randomLength = 6

	buffer := make([]byte, randomLength)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.Grow(2 + randomLength)
	builder.WriteString("QY")
	for _, value := range buffer {
		builder.WriteByte(partnerCodeAlphabet[int(value)%len(partnerCodeAlphabet)])
	}
	return builder.String(), nil
}

func (s *Store) CreateUserRegistration(ctx context.Context, input CreateUserRegistrationInput) (*domain.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var email any
	if trimmed := strings.ToLower(strings.TrimSpace(input.Email)); trimmed != "" {
		email = trimmed
	}
	var phone any
	if trimmed := strings.TrimSpace(input.Phone); trimmed != "" {
		phone = trimmed
	}

	var partnerProfile *domain.PartnerProfile
	if code := normalizePartnerCode(input.PartnerCode); code != "" {
		partnerProfile, err = getPartnerProfileByCodeTx(ctx, tx, code)
		if err != nil {
			return nil, err
		}
		if partnerProfile == nil || strings.TrimSpace(partnerProfile.Status) != "active" {
			return nil, ErrPartnerCodeInvalid
		}
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+userSelectColumns+`
	`, input.ID, email, phone, input.Name, input.PasswordHash)

	var user domain.User
	if err := row.Scan(&user.ID, &user.Email, &user.Phone, &user.Name, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return nil, err
	}

	if partnerProfile != nil {
		metadata, _ := json.Marshal(map[string]any{
			"source":      "register_partner_code",
			"partnerCode": partnerProfile.PartnerCode,
		})
		if _, err := tx.Exec(ctx, `
			INSERT INTO distribution_referrals (
				id, promoter_user_id, invitee_user_id, status, notes, metadata
			)
			VALUES ($1, $2, $3, 'active', $4, $5)
		`, input.ID+"-referral", partnerProfile.UserID, user.ID, "用户注册填写专属客服码自动绑定", metadata); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetPartnerOverviewByUserID(ctx context.Context, userID string) (*domain.PartnerOverview, error) {
	profile, err := s.GetPartnerProfileByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	summary, err := s.GetDistributionSummaryByPromoter(ctx, userID)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		summary = &domain.DistributionSummary{}
	}

	return &domain.PartnerOverview{
		Profile: profile,
		Summary: *summary,
	}, nil
}
