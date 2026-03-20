package handlers

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/storage"
)

func TestFileHandlerGetDecodesEscapedStorageKey(t *testing.T) {
	tempDir := t.TempDir()
	storageService, err := storage.New(config.Config{
		LocalStorageDir: tempDir,
	})
	if err != nil {
		t.Fatalf("storage.New returned error: %v", err)
	}

	object, err := storageService.SaveBytes(
		t.Context(),
		"skills/test-owner/test-skill/示例产品图.jpg",
		"image/jpeg",
		[]byte("hello-image"),
	)
	if err != nil {
		t.Fatalf("SaveBytes returned error: %v", err)
	}

	handler := NewFileHandler(&appstate.App{
		Storage: storageService,
		Logger:  slog.Default(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/"+url.PathEscape(object.StorageKey), nil)
	rec := httptest.NewRecorder()
	handler.Get(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(body) != "hello-image" {
		t.Fatalf("unexpected body %q", string(body))
	}
}
