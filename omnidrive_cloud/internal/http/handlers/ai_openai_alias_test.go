package handlers

import "testing"

func TestOpenAIModelAliasCandidatesIncludesDotAndDashVariants(t *testing.T) {
	candidates := openAIModelAliasCandidates("gpt-5.4")
	seen := map[string]bool{}
	for _, item := range candidates {
		seen[item] = true
	}
	if !seen["gpt-5.4"] {
		t.Fatalf("expected exact model id to be preserved")
	}
	if !seen["gpt-5-4"] {
		t.Fatalf("expected dashed alias to be generated")
	}
}

func TestIsDefaultOpenAIChatModelAlias(t *testing.T) {
	for _, item := range []string{"default-chat", "default", "omnidrive-default-chat"} {
		if !isDefaultOpenAIChatModelAlias(item) {
			t.Fatalf("expected %q to be treated as default alias", item)
		}
	}
	if isDefaultOpenAIChatModelAlias("gpt-5.4") {
		t.Fatalf("did not expect ordinary model id to be treated as default alias")
	}
}
