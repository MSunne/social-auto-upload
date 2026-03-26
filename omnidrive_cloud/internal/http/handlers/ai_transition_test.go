package handlers

import (
	"testing"

	"omnidrive_cloud/internal/domain"
)

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
		{
			name:    "running_can_requeue_after_restart",
			current: "running",
			next:    "queued",
			want:    true,
		},
		{
			name:    "running_can_pause_for_recharge",
			current: "running",
			next:    "waiting_recharge",
			want:    true,
		},
		{
			name:    "waiting_recharge_can_resume_running",
			current: "waiting_recharge",
			next:    "running",
			want:    true,
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

func TestComputeAIJobActionsWaitingRecharge(t *testing.T) {
	job := &domain.AIJob{Status: "waiting_recharge"}
	actions := computeAIJobActions(job, 0)

	if !actions.CanEdit {
		t.Fatalf("expected waiting_recharge job to remain editable")
	}
	if !actions.CanCancel {
		t.Fatalf("expected waiting_recharge job to remain cancellable")
	}
	if actions.CanRetry {
		t.Fatalf("expected waiting_recharge job to rely on auto resume instead of manual retry")
	}
}

func TestBuildAIJobBridgeStateWaitingRecharge(t *testing.T) {
	job := &domain.AIJob{
		Status: "waiting_recharge",
		Source: "account_skill_binding",
	}

	state := buildAIJobBridgeState(job, nil, nil)
	if state.DeliveryStage != "waiting_recharge" {
		t.Fatalf("expected waiting_recharge stage, got %q", state.DeliveryStage)
	}
}
