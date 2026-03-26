package workflow

import (
	"testing"
	"time"

	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/store"
)

func TestFirstTomorrowBillingShortageFindsProjectedBalanceForTomorrowJob(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	startToday := time.Date(2026, 3, 26, 0, 0, 0, 0, location)
	startTomorrow := startToday.Add(24 * time.Hour)
	endTomorrow := startTomorrow.Add(24 * time.Hour)

	jobs := []domain.AIJob{
		{
			ID:          "job-today",
			OwnerUserID: "user-1",
			RunAt:       timePtr(startToday.Add(20 * time.Hour).UTC()),
			CreatedAt:   startToday.UTC(),
		},
		{
			ID:          "job-tomorrow",
			OwnerUserID: "user-1",
			RunAt:       timePtr(startTomorrow.Add(2 * time.Hour).UTC()),
			CreatedAt:   startToday.Add(2 * time.Hour).UTC(),
		},
	}

	previews := []store.UsageBillingQueuePreviewItem{
		{
			Result: store.ApplyUsageBillingResult{
				BillStatus: "billed",
			},
			CreditBalanceBefore: 800,
			CreditBalanceAfter:  400,
		},
		{
			Result: store.ApplyUsageBillingResult{
				BillStatus:  "failed",
				BillMessage: "wallet credits insufficient",
			},
			CreditBalanceBefore: 400,
			CreditBalanceAfter:  400,
		},
	}

	candidate := firstTomorrowBillingShortage(jobs, previews, startTomorrow, endTomorrow, location)
	if candidate == nil {
		t.Fatalf("expected tomorrow shortage candidate")
	}
	if candidate.Job.ID != "job-tomorrow" {
		t.Fatalf("expected job-tomorrow, got %#v", candidate.Job.ID)
	}
	if candidate.ProjectedBalance != 400 {
		t.Fatalf("expected projected balance 400, got %d", candidate.ProjectedBalance)
	}
}

func TestBuildBillingAlertTemplateParam(t *testing.T) {
	param, err := buildBillingAlertTemplateParam(1234)
	if err != nil {
		t.Fatalf("buildBillingAlertTemplateParam() returned error: %v", err)
	}
	if param != "{\"balance\":\"1234\"}" {
		t.Fatalf("unexpected template param: %q", param)
	}
}

func TestUsageBillingLooksInsufficientOnlyMatchesBalanceFailures(t *testing.T) {
	if !usageBillingLooksInsufficient(store.ApplyUsageBillingResult{BillStatus: "failed", BillMessage: "wallet credits insufficient"}) {
		t.Fatalf("expected wallet shortage to be treated as insufficient")
	}
	if usageBillingLooksInsufficient(store.ApplyUsageBillingResult{BillStatus: "failed", BillMessage: "pricing rule not found"}) {
		t.Fatalf("expected pricing rule failures to skip sms shortage alerts")
	}
	if usageBillingLooksInsufficient(store.ApplyUsageBillingResult{BillStatus: "billed", BillMessage: "wallet credits insufficient"}) {
		t.Fatalf("expected billed result to skip sms shortage alerts")
	}
}

func TestBillingAlertWindowBounds(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 3, 26, 15, 30, 0, 0, location)
	startToday, startTomorrow, endTomorrow := billingAlertWindowBounds(now)

	if !startToday.Equal(time.Date(2026, 3, 26, 0, 0, 0, 0, location)) {
		t.Fatalf("unexpected startToday: %s", startToday)
	}
	if !startTomorrow.Equal(time.Date(2026, 3, 27, 0, 0, 0, 0, location)) {
		t.Fatalf("unexpected startTomorrow: %s", startTomorrow)
	}
	if !endTomorrow.Equal(time.Date(2026, 3, 28, 0, 0, 0, 0, location)) {
		t.Fatalf("unexpected endTomorrow: %s", endTomorrow)
	}
}

func TestBillingAlertLocationFallsBackToSomethingUsable(t *testing.T) {
	location := billingAlertLocation()
	if location == nil || location.String() == "" {
		t.Fatalf("expected billingAlertLocation() to return a valid location")
	}
}

func timePtr(value time.Time) *time.Time {
	utc := value.UTC()
	return &utc
}
