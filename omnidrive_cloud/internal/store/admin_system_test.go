package store

import (
	"testing"
	"time"
)

func TestScanAdminSystemSettingsIncludesPromptOptimizeModel(t *testing.T) {
	now := time.Now().UTC()

	record, err := scanAdminSystemSettings(func(dest ...any) error {
		if len(dest) != 42 {
			t.Fatalf("scanAdminSystemSettings destination count = %d, want %d", len(dest), 42)
		}

		*(dest[0].(*string)) = "system"
		*(dest[1].(*bool)) = true
		*(dest[2].(*[]byte)) = []byte(`["manual_cs"]`)
		*(dest[27].(*string)) = "default-chat-model"
		*(dest[28].(*string)) = "gemini-3.1-pro-preview"
		*(dest[29].(*string)) = "default-image-model"
		*(dest[30].(*string)) = "default-video-model"
		*(dest[32].(*string)) = "mix-video-script-rewrite"
		*(dest[33].(*string)) = "mix-video-publish-rewrite"
		*(dest[40].(*time.Time)) = now
		*(dest[41].(*time.Time)) = now
		return nil
	})
	if err != nil {
		t.Fatalf("scanAdminSystemSettings returned error: %v", err)
	}

	if record.DefaultChatModel != "default-chat-model" {
		t.Fatalf("DefaultChatModel = %q, want %q", record.DefaultChatModel, "default-chat-model")
	}
	if record.PromptOptimizeModel != "gemini-3.1-pro-preview" {
		t.Fatalf("PromptOptimizeModel = %q, want %q", record.PromptOptimizeModel, "gemini-3.1-pro-preview")
	}
	if record.DefaultImageModel != "default-image-model" {
		t.Fatalf("DefaultImageModel = %q, want %q", record.DefaultImageModel, "default-image-model")
	}
	if record.DefaultVideoModel != "default-video-model" {
		t.Fatalf("DefaultVideoModel = %q, want %q", record.DefaultVideoModel, "default-video-model")
	}
	if record.MixVideoScriptRewritePrompt != "mix-video-script-rewrite" {
		t.Fatalf("MixVideoScriptRewritePrompt = %q, want %q", record.MixVideoScriptRewritePrompt, "mix-video-script-rewrite")
	}
	if record.MixVideoPublishIntroPrompt != "mix-video-publish-rewrite" {
		t.Fatalf("MixVideoPublishIntroPrompt = %q, want %q", record.MixVideoPublishIntroPrompt, "mix-video-publish-rewrite")
	}
	if len(record.PaymentChannels) != 1 || record.PaymentChannels[0] != "manual_cs" {
		t.Fatalf("PaymentChannels = %#v, want %#v", record.PaymentChannels, []string{"manual_cs"})
	}
}
