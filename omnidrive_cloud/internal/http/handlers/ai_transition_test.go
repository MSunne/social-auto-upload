package handlers

import "testing"

func TestIsAllowedAIJobTransition(t *testing.T) {
	tests := []struct {
		name    string
		current string
		next    string
		want    bool
	}{
		{
			name:    "scheduled_can_queue_when_time_moves_forward",
			current: "scheduled",
			next:    "queued",
			want:    true,
		},
		{
			name:    "queued_can_reschedule_for_future",
			current: "queued",
			next:    "scheduled",
			want:    true,
		},
		{
			name:    "running_cannot_return_to_scheduled",
			current: "running",
			next:    "scheduled",
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAllowedAIJobTransition(tc.current, tc.next)
			if got != tc.want {
				t.Fatalf("isAllowedAIJobTransition(%q, %q) = %v, want %v", tc.current, tc.next, got, tc.want)
			}
		})
	}
}
