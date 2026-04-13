package store

import (
	"testing"
	"time"
)

func TestScanAdminSystemSettingsIncludesPromptOptimizeModel(t *testing.T) {
	now := time.Now().UTC()

	record, err := scanAdminSystemSettings(func(dest ...any) error {
		if len(dest) != 38 {
			t.Fatalf("scanAdminSystemSettings destination count = %d, want %d", len(dest), 38)
		}

		*(dest[0].(*string)) = "system"
		*(dest[1].(*bool)) = true
		*(dest[2].(*[]byte)) = []byte(`["manual_cs"]`)
		*(dest[25].(*string)) = "default-chat-model"
		*(dest[26].(*string)) = "gemini-3.1-pro-preview"
		*(dest[27].(*string)) = "default-image-model"
		*(dest[28].(*string)) = "default-video-model"
		*(dest[36].(*time.Time)) = now
		*(dest[37].(*time.Time)) = now
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
	if len(record.PaymentChannels) != 1 || record.PaymentChannels[0] != "manual_cs" {
		t.Fatalf("PaymentChannels = %#v, want %#v", record.PaymentChannels, []string{"manual_cs"})
	}
}
