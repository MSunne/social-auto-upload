package handlers

import (
	"testing"
	"time"

	"omnidrive_cloud/internal/domain"
)

func TestIsReusableLoginSession(t *testing.T) {
	now := time.Date(2026, time.April, 1, 18, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		session *domain.LoginSession
		want    bool
	}{
		{
			name: "reuses_recent_running_session",
			session: &domain.LoginSession{
				Status:    "running",
				UpdatedAt: now.Add(-90 * time.Second),
			},
			want: true,
		},
		{
			name: "reuses_recent_verification_session",
			session: &domain.LoginSession{
				Status:    "verification_required",
				UpdatedAt: now.Add(-10 * time.Minute),
			},
			want: true,
		},
		{
			name: "ignores_stale_verification_session",
			session: &domain.LoginSession{
				Status:    "verification_required",
				UpdatedAt: now.Add(-25 * time.Minute),
			},
			want: false,
		},
		{
			name: "ignores_final_session",
			session: &domain.LoginSession{
				Status:    "failed",
				UpdatedAt: now.Add(-30 * time.Second),
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isReusableLoginSession(tc.session, now)
			if got != tc.want {
				t.Fatalf("isReusableLoginSession(%q) = %v, want %v", tc.session.Status, got, tc.want)
			}
		})
	}
}
