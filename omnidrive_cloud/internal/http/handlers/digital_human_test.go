package handlers

import (
	"net/http"
	"testing"
)

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
