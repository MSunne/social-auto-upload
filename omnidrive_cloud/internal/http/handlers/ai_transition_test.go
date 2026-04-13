package handlers

import (
	"encoding/json"
	"testing"
	"time"

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

func TestComputeAIJobActionsAllowsDeleteForStaleRunningJobs(t *testing.T) {
	staleJob := &domain.AIJob{Status: "running", UpdatedAt: time.Now().UTC().Add(-16 * time.Minute)}
	staleActions := computeAIJobActions(staleJob, 0)
	if !staleActions.CanDelete {
		t.Fatalf("expected stale running job to be deletable")
	}

	freshJob := &domain.AIJob{Status: "running", UpdatedAt: time.Now().UTC().Add(-5 * time.Minute)}
	freshActions := computeAIJobActions(freshJob, 0)
	if freshActions.CanDelete {
		t.Fatalf("expected fresh running job to remain non-deletable")
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

func TestExtractAIJobPrimaryTargetFallsBackToPublishTarget(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"publishPayload": map[string]any{
			"targets": []map[string]any{
				{
					"accountId":   "acc-1",
					"platform":    "抖音",
					"accountName": "测试账号",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	target := extractAIJobPrimaryTarget(raw)
	if target.AccountID != "acc-1" || target.Platform != "抖音" || target.AccountName != "测试账号" {
		t.Fatalf("unexpected target: %#v", target)
	}
}

func TestApplySkillWorkflowPricingSnapshotSupportsItemizedMultiples(t *testing.T) {
	durationSeconds := 30
	raw, err := applySkillWorkflowPricingSnapshot(nil, &domain.ProductSkill{
		OutputType:           "视文模式",
		FixedDurationSeconds: &durationSeconds,
	}, &skillFixedDurationConfig{
		DurationSeconds: 30,
		SegmentSeconds:  10,
	})
	if err != nil {
		t.Fatalf("applySkillWorkflowPricingSnapshot returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload["fixedDurationSeconds"] != float64(30) {
		t.Fatalf("unexpected fixedDurationSeconds: %#v", payload["fixedDurationSeconds"])
	}
	workflowPricing, ok := payload["workflowPricing"].(map[string]any)
	if !ok {
		t.Fatalf("expected workflowPricing payload, got %#v", payload["workflowPricing"])
	}
	if workflowPricing["durationSeconds"] != float64(30) {
		t.Fatalf("unexpected durationSeconds: %#v", workflowPricing["durationSeconds"])
	}
	if workflowPricing["segmentSeconds"] != float64(10) {
		t.Fatalf("unexpected segmentSeconds: %#v", workflowPricing["segmentSeconds"])
	}
	if _, exists := workflowPricing["ruleId"]; exists {
		t.Fatalf("expected itemized snapshot without ruleId, got %#v", workflowPricing["ruleId"])
	}
}

func TestReadCreateAIJobPurpose(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"purpose": " Prompt_Optimize ",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if got := readCreateAIJobPurpose(raw); got != promptOptimizePurpose {
		t.Fatalf("readCreateAIJobPurpose() = %q, want %q", got, promptOptimizePurpose)
	}
}

func TestIsPromptOptimizeCreateJob(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"purpose": promptOptimizePurpose,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if !isPromptOptimizeCreateJob("chat", raw) {
		t.Fatal("expected chat prompt optimize payload to be detected")
	}
	if isPromptOptimizeCreateJob("image", raw) {
		t.Fatal("did not expect non-chat job type to be treated as prompt optimize")
	}
}

func TestPromptOptimizeModelCandidatesPreferDedicatedConfigThenFallback(t *testing.T) {
	settings := effectiveAdminSystemSettings{
		PromptOptimizeModel: "gemini-3.1-pro-preview",
		DefaultChatModel:    "default-chat-model",
	}

	got := promptOptimizeModelCandidates(settings, "legacy-request-model")
	want := []string{"gemini-3.1-pro-preview", "default-chat-model", "legacy-request-model"}
	if len(got) != len(want) {
		t.Fatalf("promptOptimizeModelCandidates() len = %d, want %d (%#v)", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("promptOptimizeModelCandidates()[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestPromptOptimizeModelCandidatesDeduplicateValues(t *testing.T) {
	settings := effectiveAdminSystemSettings{
		PromptOptimizeModel: "gemini-3.1-pro-preview",
		DefaultChatModel:    "gemini-3.1-pro-preview",
	}

	got := promptOptimizeModelCandidates(settings, "gemini-3.1-pro-preview")
	if len(got) != 1 || got[0] != "gemini-3.1-pro-preview" {
		t.Fatalf("unexpected candidate list: %#v", got)
	}
}
