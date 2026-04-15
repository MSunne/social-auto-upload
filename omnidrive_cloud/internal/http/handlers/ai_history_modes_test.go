package handlers

import "testing"

func TestNormalizeAIJobDetailPayloadMode(t *testing.T) {
	if got := normalizeAIJobDetailPayloadMode("history_detail"); got != "history_detail" {
		t.Fatalf("expected history_detail mode, got %q", got)
	}
	if got := normalizeAIJobDetailPayloadMode("summary"); got != "summary" {
		t.Fatalf("expected summary mode, got %q", got)
	}
	if got := normalizeAIJobDetailPayloadMode("unexpected"); got != "full" {
		t.Fatalf("expected invalid detail mode to fall back to full, got %q", got)
	}
}

func TestNormalizeAIArtifactMode(t *testing.T) {
	if got := normalizeAIArtifactMode("preview"); got != "preview" {
		t.Fatalf("expected preview mode, got %q", got)
	}
	if got := normalizeAIArtifactMode("chat"); got != "chat" {
		t.Fatalf("expected chat mode, got %q", got)
	}
	if got := normalizeAIArtifactMode("unexpected"); got != "full" {
		t.Fatalf("expected invalid artifact mode to fall back to full, got %q", got)
	}
}
