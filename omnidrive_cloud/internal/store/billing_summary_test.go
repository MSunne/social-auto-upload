package store

import (
	"testing"
	"time"

	"omnidrive_cloud/internal/domain"
)

func TestApplyRechargeAlertClearsHistoricalFailureStyleReminderWithoutActiveBlockedJobs(t *testing.T) {
	summary := &domain.BillingSummary{
		CreditBalance: 320,
		QuotaBalances: []domain.BillingQuotaBalance{},
	}

	applyRechargeAlert(summary, false, 0, nil, nil)

	if summary.NeedsRecharge {
		t.Fatalf("expected recharge reminder to clear when there are no blocked jobs and balance is available")
	}
	if summary.RechargeAlertReason != nil || summary.RechargeAlertMessage != nil || summary.LastBillingFailedAt != nil {
		t.Fatalf("expected recharge alert metadata to be cleared, got %#v", summary)
	}
}

func TestApplyRechargeAlertUsesWaitingRechargeJobs(t *testing.T) {
	summary := &domain.BillingSummary{
		CreditBalance: 800,
		QuotaBalances: []domain.BillingQuotaBalance{},
	}
	lastWaitingAt := time.Date(2026, 3, 26, 12, 30, 0, 0, time.UTC)
	message := "当前积分不足，充值后会自动继续执行。"

	applyRechargeAlert(summary, false, 2, &message, &lastWaitingAt)

	if !summary.NeedsRecharge {
		t.Fatalf("expected waiting recharge jobs to trigger reminder")
	}
	if summary.RechargeAlertReason == nil || *summary.RechargeAlertReason != "waiting_recharge" {
		t.Fatalf("unexpected recharge alert reason: %#v", summary.RechargeAlertReason)
	}
	if summary.RechargeAlertMessage == nil || *summary.RechargeAlertMessage != message {
		t.Fatalf("unexpected recharge alert message: %#v", summary.RechargeAlertMessage)
	}
	if summary.LastBillingFailedAt == nil || !summary.LastBillingFailedAt.Equal(lastWaitingAt) {
		t.Fatalf("unexpected last billing failed at: %#v", summary.LastBillingFailedAt)
	}
}

func TestApplyRechargeAlertFallsBackToBalanceExhausted(t *testing.T) {
	summary := &domain.BillingSummary{
		CreditBalance: 0,
		QuotaBalances: []domain.BillingQuotaBalance{},
	}

	applyRechargeAlert(summary, false, 0, nil, nil)

	if !summary.NeedsRecharge {
		t.Fatalf("expected exhausted balance to trigger reminder")
	}
	if summary.RechargeAlertReason == nil || *summary.RechargeAlertReason != "balance_exhausted" {
		t.Fatalf("unexpected recharge alert reason: %#v", summary.RechargeAlertReason)
	}
}
