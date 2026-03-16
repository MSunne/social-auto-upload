package store

import "testing"

func TestPlanUsageChargeWalletOnly(t *testing.T) {
	detail, walletPlan, quotaPlans, ok := planUsageCharge(
		ApplyUsageMetricInput{MeterCode: "chat_input_tokens", Quantity: 1200},
		pricingRuleRecord{
			ID:                "rule-chat-in",
			MeterCode:         "chat_input_tokens",
			ChargeMode:        "wallet_only",
			UnitSize:          1000,
			WalletDebitAmount: 2,
		},
		10,
		map[string][]*quotaAccountRecord{},
	)

	if !ok {
		t.Fatalf("expected wallet-only plan to succeed: %#v", detail)
	}
	if detail.Units != 2 || detail.DebitCredits != 4 || detail.BillStatus != "billed" {
		t.Fatalf("unexpected billing detail: %#v", detail)
	}
	if walletPlan.debitCredits != 4 {
		t.Fatalf("unexpected wallet plan: %#v", walletPlan)
	}
	if len(quotaPlans) != 0 {
		t.Fatalf("expected no quota plans, got %#v", quotaPlans)
	}
}

func TestPlanUsageChargeQuotaFallbackUsesQuotaBeforeWallet(t *testing.T) {
	quotaCode := "image_generation_quota"
	quotaAccounts := map[string][]*quotaAccountRecord{
		quotaCode: {
			{ID: "quota-1", MeterCode: quotaCode, RemainingTotal: 1},
		},
	}

	detail, walletPlan, quotaPlans, ok := planUsageCharge(
		ApplyUsageMetricInput{MeterCode: "image_generations", Quantity: 2},
		pricingRuleRecord{
			ID:                "rule-image",
			MeterCode:         "image_generations",
			ChargeMode:        "quota_first_wallet_fallback",
			QuotaMeterCode:    &quotaCode,
			UnitSize:          1,
			WalletDebitAmount: 80,
		},
		500,
		quotaAccounts,
	)

	if !ok {
		t.Fatalf("expected quota fallback to succeed: %#v", detail)
	}
	if detail.Units != 2 || detail.QuotaUsed != 1 || detail.DebitCredits != 80 {
		t.Fatalf("unexpected billing detail: %#v", detail)
	}
	if len(quotaPlans) != 1 || quotaPlans[0].amountDelta != -1 {
		t.Fatalf("unexpected quota plans: %#v", quotaPlans)
	}
	if walletPlan.debitCredits != 80 {
		t.Fatalf("unexpected wallet plan: %#v", walletPlan)
	}
	if quotaAccounts[quotaCode][0].RemainingTotal != 0 {
		t.Fatalf("expected quota account to be decremented, got %d", quotaAccounts[quotaCode][0].RemainingTotal)
	}
}

func TestPlanUsageChargeFailsWhenWalletInsufficient(t *testing.T) {
	detail, _, _, ok := planUsageCharge(
		ApplyUsageMetricInput{MeterCode: "video_generations", Quantity: 1},
		pricingRuleRecord{
			ID:                "rule-video",
			MeterCode:         "video_generations",
			ChargeMode:        "wallet_only",
			UnitSize:          1,
			WalletDebitAmount: 400,
		},
		200,
		map[string][]*quotaAccountRecord{},
	)

	if ok {
		t.Fatalf("expected billing to fail when wallet is insufficient")
	}
	if detail.BillStatus != "failed" {
		t.Fatalf("expected failed bill status, got %#v", detail)
	}
}
