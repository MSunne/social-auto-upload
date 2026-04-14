package ai

import (
	"testing"
	"time"

	"omnidrive_cloud/internal/domain"
)

func TestMediaExecutionPollIntervalUsesFastAndSlowCadence(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)

	if got := mediaExecutionPollInterval(time.Time{}, now); got != longTaskFastPollInterval {
		t.Fatalf("expected zero anchor to use fast poll interval, got %s", got)
	}
	if got := mediaExecutionPollInterval(now.Add(-4*time.Minute), now); got != longTaskFastPollInterval {
		t.Fatalf("expected fast poll interval before 5 minute window, got %s", got)
	}
	if got := mediaExecutionPollInterval(now.Add(-6*time.Minute), now); got != longTaskSlowPollInterval {
		t.Fatalf("expected slow poll interval after 5 minute window, got %s", got)
	}
}

func TestDigitalHumanTaskExecutionAnchorPrefersStartedAt(t *testing.T) {
	createdAt := time.Date(2026, 4, 14, 9, 55, 0, 0, time.UTC)
	startedAt := time.Date(2026, 4, 14, 9, 58, 0, 0, time.UTC)
	task := &domain.DigitalHumanTask{
		CreatedAt: createdAt,
		StartedAt: &startedAt,
	}

	if got := digitalHumanTaskExecutionAnchor(task); !got.Equal(startedAt) {
		t.Fatalf("expected startedAt anchor, got %s", got)
	}

	task.StartedAt = nil
	if got := digitalHumanTaskExecutionAnchor(task); !got.Equal(createdAt) {
		t.Fatalf("expected createdAt fallback anchor, got %s", got)
	}
}

func TestShouldContinueDigitalHumanInBackground(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	createdAt := now.Add(-20 * time.Minute)
	task := &domain.DigitalHumanTask{
		CreatedAt: createdAt,
	}

	if !shouldContinueDigitalHumanInBackground(task, 10*time.Minute, now) {
		t.Fatalf("expected timed out digital human task to continue in background")
	}
	if shouldContinueDigitalHumanInBackground(task, 30*time.Minute, now) {
		t.Fatalf("did not expect digital human task to requeue before timeout")
	}
}

func TestAllowAutoPublishJobSource(t *testing.T) {
	if !allowAutoPublishJobSource("account_skill_binding") {
		t.Fatalf("expected account_skill_binding to allow auto publish")
	}
	if !allowAutoPublishJobSource("openclaw_skill") {
		t.Fatalf("expected openclaw_skill to allow auto publish")
	}
	if allowAutoPublishJobSource("omnidrive_cloud") {
		t.Fatalf("did not expect omnidrive_cloud to allow auto publish")
	}
}

func TestExtractAutoPublishTargetAndRunAt(t *testing.T) {
	raw := mustJSON(map[string]any{
		"publishAt": "2026-04-14T12:30:00Z",
		"publishPayload": map[string]any{
			"targets": []map[string]any{{
				"accountId":   "account-1",
				"platform":    "douyin",
				"accountName": "demo-account",
			}},
		},
	})

	target := extractAutoPublishTarget(raw)
	if target.AccountID != "account-1" || target.Platform != "douyin" || target.AccountName != "demo-account" {
		t.Fatalf("unexpected auto publish target: %#v", target)
	}

	runAt, err := extractAutoPublishRunAt(raw)
	if err != nil {
		t.Fatalf("expected publishAt to parse, got error: %v", err)
	}
	want := time.Date(2026, 4, 14, 12, 30, 0, 0, time.UTC)
	if !runAt.Equal(want) {
		t.Fatalf("unexpected auto publish runAt %s, want %s", runAt, want)
	}
}
