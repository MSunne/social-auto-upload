package handlers

import (
	"testing"
	"time"

	"omnidrive_cloud/internal/store"
)

func TestAdminAIJobListCursorRoundTrip(t *testing.T) {
	expected := &store.AdminAIJobListCursor{
		CreatedAt: time.Date(2026, time.April, 11, 10, 30, 0, 123456000, time.UTC),
		ID:        "job-123",
	}

	encoded, err := encodeAdminAIJobListCursor(expected)
	if err != nil {
		t.Fatalf("encodeAdminAIJobListCursor returned error: %v", err)
	}
	if encoded == nil || *encoded == "" {
		t.Fatalf("expected encoded cursor, got %#v", encoded)
	}

	decoded, err := decodeAdminAIJobListCursor(*encoded)
	if err != nil {
		t.Fatalf("decodeAdminAIJobListCursor returned error: %v", err)
	}
	if decoded == nil {
		t.Fatal("expected decoded cursor, got nil")
	}
	if !decoded.CreatedAt.Equal(expected.CreatedAt) || decoded.ID != expected.ID {
		t.Fatalf("expected decoded cursor %#v, got %#v", expected, decoded)
	}
}

func TestDecodeAdminAIJobListCursorRejectsInvalidValue(t *testing.T) {
	if _, err := decodeAdminAIJobListCursor("not-a-valid-cursor"); err == nil {
		t.Fatal("expected invalid cursor to return an error")
	}
}
