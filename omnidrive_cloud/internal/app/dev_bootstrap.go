package app

import (
	"context"
	"fmt"
	"strings"

	"omnidrive_cloud/internal/store"
)

type developmentSeedUser struct {
	ID             string
	Email          string
	Name           string
	InitialCredits int64
}

var developmentSeedUsers = []developmentSeedUser{
	{ID: "demo-user-01", Email: "demo01@omnidrive.local", Name: "Demo User 01", InitialCredits: 1200},
	{ID: "demo-user-02", Email: "demo02@omnidrive.local", Name: "Demo User 02", InitialCredits: 1800},
	{ID: "demo-user-03", Email: "demo03@omnidrive.local", Name: "Demo User 03", InitialCredits: 2600},
	{ID: "demo-user-04", Email: "demo04@omnidrive.local", Name: "Demo User 04", InitialCredits: 3200},
	{ID: "demo-user-05", Email: "demo05@omnidrive.local", Name: "Demo User 05", InitialCredits: 4000},
	{ID: "demo-user-06", Email: "demo06@omnidrive.local", Name: "Demo User 06", InitialCredits: 4800},
	{ID: "demo-user-07", Email: "demo07@omnidrive.local", Name: "Demo User 07", InitialCredits: 5600},
	{ID: "demo-user-08", Email: "demo08@omnidrive.local", Name: "Demo User 08", InitialCredits: 6400},
	{ID: "demo-user-09", Email: "demo09@omnidrive.local", Name: "Demo User 09", InitialCredits: 7200},
	{ID: "demo-user-10", Email: "demo10@omnidrive.local", Name: "Demo User 10", InitialCredits: 8000},
	{ID: "demo-user-11", Email: "demo11@omnidrive.local", Name: "Demo User 11", InitialCredits: 8800},
	{ID: "demo-user-12", Email: "demo12@omnidrive.local", Name: "Demo User 12", InitialCredits: 9600},
}

func (a *App) EnsureDevelopmentSeedUsers(ctx context.Context) error {
	if !strings.EqualFold(strings.TrimSpace(a.Config.Environment), "development") {
		return nil
	}
	if !a.Config.DevSeedUsers {
		a.Logger.Debug("development demo user seeding disabled")
		return nil
	}

	password := strings.TrimSpace(a.Config.DevSeedUserPassword)
	if len(password) < 8 {
		return fmt.Errorf("dev seed user password must be at least 8 characters")
	}

	passwordHash, err := a.Tokens.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash dev seed user password: %w", err)
	}

	createdEmails := make([]string, 0)
	for _, seed := range developmentSeedUsers {
		existing, err := a.Store.GetUserByEmail(ctx, seed.Email)
		if err != nil {
			return fmt.Errorf("query dev seed user %s: %w", seed.Email, err)
		}
		if existing != nil {
			continue
		}

		_, err = a.Store.CreateUser(ctx, store.CreateUserInput{
			ID:           seed.ID,
			Email:        seed.Email,
			Name:         seed.Name,
			PasswordHash: passwordHash,
		})
		if err != nil {
			return fmt.Errorf("create dev seed user %s: %w", seed.Email, err)
		}

		if seed.InitialCredits > 0 {
			description := fmt.Sprintf("Development seed credits for %s", seed.Email)
			referenceType := "development_seed"
			referenceID := seed.ID
			if err := a.Store.GrantWalletCredits(ctx, store.GrantWalletCreditsInput{
				UserID:        seed.ID,
				Amount:        seed.InitialCredits,
				Description:   &description,
				ReferenceType: &referenceType,
				ReferenceID:   &referenceID,
			}); err != nil {
				return fmt.Errorf("grant dev seed credits to %s: %w", seed.Email, err)
			}
		}

		createdEmails = append(createdEmails, seed.Email)
	}

	if len(createdEmails) > 0 {
		a.Logger.Info("development demo users ensured",
			"created_count", len(createdEmails),
			"emails", createdEmails,
		)
	} else {
		a.Logger.Debug("development demo users already present")
	}
	return nil
}
