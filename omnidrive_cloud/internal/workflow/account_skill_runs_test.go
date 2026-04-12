package workflow

import (
	"encoding/json"
	"testing"
)

func TestExtractAccountSkillTargetAccountIDPrefersRootAccountID(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"accountId": "acc-root",
		"publishPayload": map[string]any{
			"targets": []map[string]any{
				{"accountId": "acc-nested"},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if got := ExtractAccountSkillTargetAccountID(raw); got != "acc-root" {
		t.Fatalf("expected root accountId, got %q", got)
	}
}

func TestExtractAccountSkillTargetAccountIDFallsBackToNestedTarget(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"publishPayload": map[string]any{
			"targets": []map[string]any{
				{"accountId": "acc-nested"},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if got := ExtractAccountSkillTargetAccountID(raw); got != "acc-nested" {
		t.Fatalf("expected nested accountId fallback, got %q", got)
	}
}
