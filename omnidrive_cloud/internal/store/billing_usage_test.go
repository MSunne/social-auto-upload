package store

import (
	"strings"
	"testing"
)

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
	if detail.Units != 2 || detail.DebitCredits != 4 || detail.DistributionReleaseCredits != 4 || detail.BillStatus != "billed" {
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
			{ID: "quota-1", MeterCode: quotaCode, RemainingTotal: 1, ReleaseUnitCredits: 80},
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
	if detail.Units != 2 || detail.QuotaUsed != 1 || detail.DebitCredits != 80 || detail.DistributionReleaseCredits != 160 {
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

func TestPlanUsageChargeAllowsFallbackRuleWithoutPersistentID(t *testing.T) {
	detail, walletPlan, _, ok := planUsageCharge(
		ApplyUsageMetricInput{MeterCode: "chat_output_tokens", Quantity: 1500},
		pricingRuleRecord{
			MeterCode:         "chat_output_tokens",
			ChargeMode:        "wallet_only",
			UnitSize:          1000,
			WalletDebitAmount: 2,
		},
		10,
		map[string][]*quotaAccountRecord{},
	)

	if !ok {
		t.Fatalf("expected synthesized pricing rule to succeed: %#v", detail)
	}
	if detail.PricingRuleID != "" {
		t.Fatalf("expected synthesized pricing rule to keep empty pricing rule id, got %#v", detail)
	}
	if detail.DebitCredits != 4 || walletPlan.debitCredits != 4 {
		t.Fatalf("unexpected synthesized rule billing detail: %#v %#v", detail, walletPlan)
	}
}

func TestPlanUsageChargeSupportsDynamicQuantityFromMetricMetadata(t *testing.T) {
	detail, walletPlan, _, ok := planUsageCharge(
		ApplyUsageMetricInput{
			MeterCode: "video_generations",
			Quantity:  1,
			Metadata:  mustJSONMap(map[string]any{"durationSeconds": 8}),
		},
		pricingRuleRecord{
			MeterCode:         "video_generations",
			ChargeMode:        "wallet_only",
			UnitSize:          1,
			WalletDebitAmount: 5,
			QuantityMetaKey:   "durationSeconds",
		},
		100,
		map[string][]*quotaAccountRecord{},
	)

	if !ok {
		t.Fatalf("expected dynamic quantity pricing to succeed: %#v", detail)
	}
	if detail.Quantity != 8 || detail.DebitCredits != 40 {
		t.Fatalf("unexpected dynamic quantity billing detail: %#v", detail)
	}
	if walletPlan.quantity != 8 || walletPlan.debitCredits != 40 {
		t.Fatalf("unexpected dynamic quantity wallet plan: %#v", walletPlan)
	}
}

func TestSupportsFailureRefundForUsage(t *testing.T) {
	if supportsFailureRefundForUsage(pricingRuleRecord{QuantityMetaKey: "durationSeconds"}, "video_generations") {
		t.Fatalf("expected per-second video billing to skip failure refund")
	}
	if supportsFailureRefundForUsage(pricingRuleRecord{}, "chat_output_tokens") {
		t.Fatalf("expected chat token billing to skip failure refund")
	}
	if !supportsFailureRefundForUsage(pricingRuleRecord{}, "image_generations") {
		t.Fatalf("expected per-count image billing to support failure refund")
	}
}

func TestQuotaUsageReturnCredits(t *testing.T) {
	if got := quotaUsageReturnCredits(6, nil, 3); got != 18 {
		t.Fatalf("expected quota return credits from stored snapshot, got %d", got)
	}

	payload := decodeUsageEventPayload(mustJSONMap(map[string]any{"creditValue": 15}))
	if got := quotaUsageReturnCredits(0, payload, 2); got != 15 {
		t.Fatalf("expected quota return credits from payload credit value, got %d", got)
	}

	payload = decodeUsageEventPayload(mustJSONMap(map[string]any{"releaseUnitCredits": 4}))
	if got := quotaUsageReturnCredits(0, payload, 2); got != 8 {
		t.Fatalf("expected quota return credits from payload release unit credits, got %d", got)
	}
}

func TestAIModelSelectColumnsIncludeSupportedFileTypes(t *testing.T) {
	if !strings.Contains(aiModelSelectColumns, "supported_file_types") {
		t.Fatalf("aiModelSelectColumns must include supported_file_types for scanAIModel")
	}
}

func TestAdminAIJobSelectColumnsIncludeRunAt(t *testing.T) {
	if !strings.Contains(adminAIJobSelectColumns, "aj.run_at") {
		t.Fatalf("adminAIJobSelectColumns must include aj.run_at so scheduled jobs render correctly")
	}
}
