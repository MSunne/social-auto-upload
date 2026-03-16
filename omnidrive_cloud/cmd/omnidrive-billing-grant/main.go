package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/database"
	"omnidrive_cloud/internal/store"
)

func main() {
	var (
		userID        string
		email         string
		credits       int64
		imageQuota    int64
		videoQuota    int64
		expiresInDays int
		reason        string
		referenceType string
		referenceID   string
	)

	flag.StringVar(&userID, "user-id", "", "Target user ID")
	flag.StringVar(&email, "email", "", "Target user email")
	flag.Int64Var(&credits, "credits", 0, "Wallet credits to grant")
	flag.Int64Var(&imageQuota, "image-quota", 0, "Image generation quota to grant")
	flag.Int64Var(&videoQuota, "video-quota", 0, "Video generation quota to grant")
	flag.IntVar(&expiresInDays, "expires-in-days", 0, "Quota expiration in days, 0 means no expiration")
	flag.StringVar(&reason, "reason", "运维手动发放", "Grant description")
	flag.StringVar(&referenceType, "reference-type", "ops_grant", "Reference type")
	flag.StringVar(&referenceID, "reference-id", "", "Reference ID")
	flag.Parse()

	if strings.TrimSpace(userID) == "" && strings.TrimSpace(email) == "" {
		log.Fatal("user-id or email is required")
	}
	if credits <= 0 && imageQuota <= 0 && videoQuota <= 0 {
		log.Fatal("at least one of credits, image-quota, video-quota must be positive")
	}

	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := database.New(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	repo := store.New(db.Pool)
	targetUserID, err := resolveUserID(ctx, repo, strings.TrimSpace(userID), strings.TrimSpace(email))
	if err != nil {
		log.Fatal(err)
	}

	description := stringPtr(strings.TrimSpace(reason))
	refType := stringPtr(strings.TrimSpace(referenceType))
	refID := stringPtr(strings.TrimSpace(referenceID))
	var expiresAt *time.Time
	if expiresInDays > 0 {
		parsed := time.Now().UTC().Add(time.Duration(expiresInDays) * 24 * time.Hour)
		expiresAt = &parsed
	}

	if credits > 0 {
		if err := repo.GrantWalletCredits(ctx, store.GrantWalletCreditsInput{
			UserID:        targetUserID,
			Amount:        credits,
			Description:   description,
			ReferenceType: refType,
			ReferenceID:   refID,
		}); err != nil {
			log.Fatalf("grant wallet credits failed: %v", err)
		}
		fmt.Printf("granted %d wallet credits to user %s\n", credits, targetUserID)
	}

	if imageQuota > 0 {
		if err := repo.GrantQuota(ctx, store.GrantQuotaInput{
			UserID:        targetUserID,
			MeterCode:     "image_generation_quota",
			Amount:        imageQuota,
			ExpiresAt:     expiresAt,
			SourceType:    refType,
			SourceID:      refID,
			Description:   description,
			ReferenceType: refType,
			ReferenceID:   refID,
		}); err != nil {
			log.Fatalf("grant image quota failed: %v", err)
		}
		fmt.Printf("granted %d image quota to user %s\n", imageQuota, targetUserID)
	}

	if videoQuota > 0 {
		if err := repo.GrantQuota(ctx, store.GrantQuotaInput{
			UserID:        targetUserID,
			MeterCode:     "video_generation_quota",
			Amount:        videoQuota,
			ExpiresAt:     expiresAt,
			SourceType:    refType,
			SourceID:      refID,
			Description:   description,
			ReferenceType: refType,
			ReferenceID:   refID,
		}); err != nil {
			log.Fatalf("grant video quota failed: %v", err)
		}
		fmt.Printf("granted %d video quota to user %s\n", videoQuota, targetUserID)
	}

	summary, err := repo.GetBillingSummaryByUser(ctx, targetUserID)
	if err != nil {
		log.Fatalf("load billing summary failed: %v", err)
	}
	fmt.Printf("wallet credits=%d frozen=%d pendingRecharge=%d\n", summary.CreditBalance, summary.FrozenCreditBalance, summary.PendingRechargeCount)
	for _, quota := range summary.QuotaBalances {
		fmt.Printf("quota meter=%s remaining=%d unit=%s expiresAt=%v\n", quota.MeterCode, quota.RemainingTotal, quota.Unit, quota.NearestExpiresAt)
	}
}

func resolveUserID(ctx context.Context, repo *store.Store, userID string, email string) (string, error) {
	if strings.TrimSpace(userID) != "" {
		user, err := repo.GetUserByID(ctx, userID)
		if err != nil {
			return "", err
		}
		if user == nil {
			return "", fmt.Errorf("user %s not found", userID)
		}
		return user.ID, nil
	}

	userWithPassword, err := repo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if userWithPassword == nil {
		return "", fmt.Errorf("user %s not found", email)
	}
	return userWithPassword.User.ID, nil
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}
