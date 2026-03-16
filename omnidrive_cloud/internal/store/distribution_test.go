package store

import (
	"testing"
	"time"
)

func TestAdvanceCommissionReleaseStatePartialAndFullRelease(t *testing.T) {
	now := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)
	state := commissionReleaseState{
		TotalGrantedCredits: 1000,
		ConsumedCredits:     0,
		AmountCents:         1500,
		ReleasedAmountCents: 0,
		SettledAmountCents:  0,
		Status:              "pending_consume",
	}

	nextState, consumedCredits := advanceCommissionReleaseState(state, 250, now)
	if consumedCredits != 250 {
		t.Fatalf("expected 250 consumed credits, got %d", consumedCredits)
	}
	if nextState.ConsumedCredits != 250 {
		t.Fatalf("expected consumed credits to advance to 250, got %d", nextState.ConsumedCredits)
	}
	if nextState.ReleasedAmountCents != 375 {
		t.Fatalf("expected released amount to be 375, got %d", nextState.ReleasedAmountCents)
	}
	if nextState.Status != "pending_settlement" {
		t.Fatalf("expected status to move to pending_settlement, got %s", nextState.Status)
	}
	if nextState.ReleasedAt == nil || !nextState.ReleasedAt.Equal(now) {
		t.Fatalf("expected releasedAt to be set to %s, got %#v", now, nextState.ReleasedAt)
	}

	finalState, finalConsumedCredits := advanceCommissionReleaseState(nextState, 1000, now.Add(time.Minute))
	if finalConsumedCredits != 750 {
		t.Fatalf("expected remaining 750 credits to be consumed, got %d", finalConsumedCredits)
	}
	if finalState.ConsumedCredits != 1000 {
		t.Fatalf("expected all granted credits to be consumed, got %d", finalState.ConsumedCredits)
	}
	if finalState.ReleasedAmountCents != 1500 {
		t.Fatalf("expected all commission cents to be released, got %d", finalState.ReleasedAmountCents)
	}
	if finalState.Status != "pending_settlement" {
		t.Fatalf("expected fully released but unsettled commission to remain pending_settlement, got %s", finalState.Status)
	}
}

func TestDeriveCommissionStatus(t *testing.T) {
	if status := deriveCommissionStatus(0, 0, 100); status != "pending_consume" {
		t.Fatalf("expected pending_consume, got %s", status)
	}
	if status := deriveCommissionStatus(60, 20, 100); status != "pending_settlement" {
		t.Fatalf("expected pending_settlement, got %s", status)
	}
	if status := deriveCommissionStatus(100, 100, 100); status != "settled" {
		t.Fatalf("expected settled, got %s", status)
	}
}

func TestCalculateCommissionAmountCentsRoundsHalfUp(t *testing.T) {
	amount := calculateCommissionAmountCents(999, 1500)
	if amount != 150 {
		t.Fatalf("expected rounded commission amount 150, got %d", amount)
	}
}
