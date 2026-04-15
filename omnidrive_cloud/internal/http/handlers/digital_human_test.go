package handlers

import (
	"net/http"
	"testing"
)

func TestEstimateDigitalHumanDurationSeconds(t *testing.T) {
	cases := []struct {
		name      string
		goodsText string
		expected  int
	}{
		{name: "empty text still reserves one second", goodsText: "", expected: 1},
		{name: "ignore whitespace runes", goodsText: "你 好 \n 世\t界", expected: 1},
		{name: "round up every four runes", goodsText: "一二三四五", expected: 2},
	}

	for _, tc := range cases {
		if actual := estimateDigitalHumanDurationSeconds(tc.goodsText); actual != tc.expected {
			t.Fatalf("%s: expected %d, got %d", tc.name, tc.expected, actual)
		}
	}
}

func TestValidateDigitalHumanMimeAcceptsSupportedTypes(t *testing.T) {
	jpegData := []byte{
		0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46,
		0x49, 0x46, 0x00, 0x01, 0x01, 0x01, 0x00, 0x48,
		0x00, 0x48, 0x00, 0x00,
	}

	contentType, err := validateDigitalHumanMime("portrait.jpg", "image/jpeg", jpegData, digitalHumanImageMIMEs)
	if err != nil {
		t.Fatalf("validateDigitalHumanMime returned error: %v", err)
	}
	if contentType != "image/jpeg" {
		t.Fatalf("unexpected content type %q", contentType)
	}

	m4aData := []byte("....ftypM4A ....")
	contentType, err = validateDigitalHumanMime("voice.m4a", "audio/mp4", m4aData, digitalHumanAudioMIMEs)
	if err != nil {
		t.Fatalf("validateDigitalHumanMime audio returned error: %v", err)
	}
	if contentType != "audio/mp4" {
		t.Fatalf("unexpected audio content type %q", contentType)
	}
}

func TestValidateDigitalHumanMimeRejectsUnsupportedTypes(t *testing.T) {
	pdfData := []byte("%PDF-1.4")
	_, err := validateDigitalHumanMime("document.pdf", "application/pdf", pdfData, digitalHumanImageMIMEs)
	if err == nil {
		t.Fatal("expected unsupported mime error")
	}
	if err.Error() != "mime type is not supported" {
		t.Fatalf("unexpected error %q", err.Error())
	}
}

func TestValidateDigitalHumanMimeFallsBackToSniffedType(t *testing.T) {
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	}

	contentType, err := validateDigitalHumanMime("goods.png", "", pngData, digitalHumanImageMIMEs)
	if err != nil {
		t.Fatalf("validateDigitalHumanMime returned error: %v", err)
	}
	if contentType != http.DetectContentType(pngData) {
		t.Fatalf("expected sniffed content type, got %q", contentType)
	}
}

func TestResolveDigitalHumanDefaultModelPrefersAdminDefaultOverCurrent(t *testing.T) {
	models := []digitalHumanModelOption{
		{ID: "model-a"},
		{ID: "model-b", IsCurrent: true},
	}

	if got := resolveDigitalHumanDefaultModel("model-a", "model-b", models); got != "model-a" {
		t.Fatalf("expected admin default to win, got %q", got)
	}
}

func TestResolveDigitalHumanDefaultModelFallsBackToCurrentThenFirst(t *testing.T) {
	models := []digitalHumanModelOption{
		{ID: "model-a"},
		{ID: "model-b", IsCurrent: true},
	}

	if got := resolveDigitalHumanDefaultModel("", "model-b", models); got != "model-b" {
		t.Fatalf("expected current model fallback, got %q", got)
	}

	if got := resolveDigitalHumanDefaultModel("", "missing-model", models); got != "model-a" {
		t.Fatalf("expected first available model fallback, got %q", got)
	}
}

func TestResolveDigitalHumanDefaultModelByModeUsesModeSpecificAdminDefault(t *testing.T) {
	config := &digitalHumanModelsResponse{
		CurrentModelID: "current-model",
		DefaultModelByMode: digitalHumanModeDefaults{
			Digital:   "shopping-model",
			Customize: "speech-model",
		},
		Models: []digitalHumanModelOption{
			{ID: "shopping-model"},
			{ID: "speech-model"},
			{ID: "current-model", IsCurrent: true},
		},
	}

	if got := resolveDigitalHumanDefaultModelByMode(config, "digital"); got != "shopping-model" {
		t.Fatalf("expected digital mode default, got %q", got)
	}
	if got := resolveDigitalHumanDefaultModelByMode(config, "customize"); got != "speech-model" {
		t.Fatalf("expected customize mode default, got %q", got)
	}
}
