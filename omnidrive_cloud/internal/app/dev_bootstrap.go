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
	Phone          string
	Name           string
	Password       string
	InitialCredits int64
}

var developmentSeedUsers = []developmentSeedUser{
	{ID: "demo-user-cn-001", Phone: "18812345678", Name: "禾硕AI", InitialCredits: 12000, Password: "123456"},
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

func effectiveDevelopmentSeedUsers(cfg interface {
	GetDemoPhone() string
	GetDemoName() string
	GetDemoPassword() string
}) []developmentSeedUser {
	seeds := append([]developmentSeedUser(nil), developmentSeedUsers...)
	if len(seeds) == 0 {
		return seeds
	}
	demoPhone := strings.TrimSpace(cfg.GetDemoPhone())
	demoName := strings.TrimSpace(cfg.GetDemoName())
	demoPassword := strings.TrimSpace(cfg.GetDemoPassword())
	if demoPhone == "" && demoName == "" && demoPassword == "" {
		return seeds
	}
	for index := range seeds {
		if seeds[index].ID != "demo-user-cn-001" {
			continue
		}
		if demoPhone != "" {
			seeds[index].Phone = demoPhone
		}
		if demoName != "" {
			seeds[index].Name = demoName
		}
		if demoPassword != "" {
			seeds[index].Password = demoPassword
		}
		break
	}
	return seeds
}

func validateDevelopmentSeedPassword(password string) error {
	if len(strings.TrimSpace(password)) < 6 {
		return fmt.Errorf("dev seed user password must be at least 6 characters")
	}
	return nil
}

func findExistingDevelopmentSeedUser(ctx context.Context, repo *store.Store, seed developmentSeedUser) (*store.UserWithPassword, error) {
	if phone := strings.TrimSpace(seed.Phone); phone != "" {
		existing, err := repo.GetUserByPhone(ctx, phone)
		if err != nil {
			return nil, fmt.Errorf("query dev seed user %s: %w", phone, err)
		}
		if existing != nil {
			return existing, nil
		}
	}
	if email := strings.TrimSpace(seed.Email); email != "" {
		existing, err := repo.GetUserByEmail(ctx, email)
		if err != nil {
			return nil, fmt.Errorf("query dev seed user %s: %w", email, err)
		}
		if existing != nil {
			return existing, nil
		}
	}
	return nil, nil
}

// 确保Development种子用户已满足执行前提，必要时补齐缺失状态或配置。
func (a *App) EnsureDevelopmentSeedUsers(ctx context.Context) error {
	if !strings.EqualFold(strings.TrimSpace(a.Config.Environment), "development") {
		return nil
	}
	if !a.Config.DevSeedUsers {
		a.Logger.Debug("development demo user seeding disabled")
		return nil
	}

	defaultPassword := strings.TrimSpace(a.Config.DevSeedUserPassword)
	if err := validateDevelopmentSeedPassword(defaultPassword); err != nil {
		return err
	}

	createdSeeds := make([]string, 0)
	updatedSeeds := make([]string, 0)
	for _, seed := range effectiveDevelopmentSeedUsers(a.Config) {
		password := strings.TrimSpace(seed.Password)
		if password == "" {
			password = defaultPassword
		}
		if err := validateDevelopmentSeedPassword(password); err != nil {
			return err
		}
		passwordHash, err := a.Tokens.HashPassword(password)
		if err != nil {
			return fmt.Errorf("hash dev seed user password: %w", err)
		}

		existing, err := findExistingDevelopmentSeedUser(ctx, a.Store, seed)
		if err != nil {
			return err
		}
		if existing != nil {
			name := strings.TrimSpace(seed.Name)
			email := strings.TrimSpace(seed.Email)
			phone := strings.TrimSpace(seed.Phone)
			isActive := true
			if _, err := a.Store.UpdateDevelopmentSeedUser(ctx, existing.User.ID, store.UpdateDevelopmentSeedUserInput{
				Email:        &email,
				Phone:        &phone,
				Name:         &name,
				PasswordHash: &passwordHash,
				IsActive:     &isActive,
			}); err != nil {
				identifier := phone
				if identifier == "" {
					identifier = strings.TrimSpace(existing.User.Email)
				}
				return fmt.Errorf("update dev seed user %s: %w", identifier, err)
			}
			identity := phone
			if identity == "" {
				identity = strings.TrimSpace(existing.User.Email)
			}
			updatedSeeds = append(updatedSeeds, identity)
			continue
		}

		_, err = a.Store.CreateUser(ctx, store.CreateUserInput{
			ID:           seed.ID,
			Email:        seed.Email,
			Phone:        seed.Phone,
			Name:         seed.Name,
			PasswordHash: passwordHash,
		})
		if err != nil {
			identifier := strings.TrimSpace(seed.Phone)
			if identifier == "" {
				identifier = strings.TrimSpace(seed.Email)
			}
			return fmt.Errorf("create dev seed user %s: %w", identifier, err)
		}

		if seed.InitialCredits > 0 {
			identity := strings.TrimSpace(seed.Phone)
			if identity == "" {
				identity = strings.TrimSpace(seed.Email)
			}
			description := fmt.Sprintf("Development seed credits for %s", identity)
			referenceType := "development_seed"
			referenceID := seed.ID
			if err := a.Store.GrantWalletCredits(ctx, store.GrantWalletCreditsInput{
				UserID:        seed.ID,
				Amount:        seed.InitialCredits,
				Description:   &description,
				ReferenceType: &referenceType,
				ReferenceID:   &referenceID,
			}); err != nil {
				return fmt.Errorf("grant dev seed credits to %s: %w", identity, err)
			}
		}

		identity := strings.TrimSpace(seed.Phone)
		if identity == "" {
			identity = strings.TrimSpace(seed.Email)
		}
		createdSeeds = append(createdSeeds, identity)
	}

	if len(createdSeeds) > 0 {
		a.Logger.Info("development demo users ensured",
			"created_count", len(createdSeeds),
			"updated_count", len(updatedSeeds),
			"identities", createdSeeds,
		)
	} else if len(updatedSeeds) > 0 {
		a.Logger.Info("development demo users refreshed",
			"updated_count", len(updatedSeeds),
			"identities", updatedSeeds,
		)
	} else {
		a.Logger.Debug("development demo users already present")
	}
	return nil
}
