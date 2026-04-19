package app

import "testing"

func TestDevelopmentSeedUsersIncludeHeshuoAIDemoPhone(t *testing.T) {
	for _, seed := range developmentSeedUsers {
		if seed.Phone != "18812345678" {
			continue
		}
		if seed.Name != "禾硕AI" {
			t.Fatalf("expected 禾硕AI demo name, got %q", seed.Name)
		}
		if seed.Email != "" {
			t.Fatalf("expected phone demo user to avoid fixed email, got %q", seed.Email)
		}
		return
	}
	t.Fatal("expected development seed users to include 18812345678")
}

func TestValidateDevelopmentSeedPasswordAllowsSixDigits(t *testing.T) {
	if err := validateDevelopmentSeedPassword("123456"); err != nil {
		t.Fatalf("expected 6-digit demo password to be accepted, got %v", err)
	}
	if err := validateDevelopmentSeedPassword("12345"); err == nil {
		t.Fatal("expected password shorter than 6 chars to be rejected")
	}
}
