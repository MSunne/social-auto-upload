package handlers

import "testing"

func TestIsAllowedAgentPublishTaskTransition(t *testing.T) {
	tests := []struct {
		name    string
		current string
		next    string
		want    bool
	}{
		{
			name:    "pending_can_enter_running",
			current: "pending",
			next:    "running",
			want:    true,
		},
		{
			name:    "running_can_requeue_after_restart",
			current: "running",
			next:    "pending",
			want:    true,
		},
		{
			name:    "completed_cannot_reopen",
			current: "completed",
			next:    "pending",
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAllowedAgentPublishTaskTransition(tc.current, tc.next)
			if got != tc.want {
				t.Fatalf("isAllowedAgentPublishTaskTransition(%q, %q) = %v, want %v", tc.current, tc.next, got, tc.want)
			}
		})
	}
}
