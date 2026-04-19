package mixvideo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"omnidrive_cloud/internal/config"
)

func TestWorkerResolveResultVideoURLKeepsAbsoluteLocalPath(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "result.mp4")
	if err := os.WriteFile(sourcePath, []byte("video-bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	worker := &Worker{
		client: NewClient(config.Config{
			MixVideoBaseURL: "http://127.0.0.1:8000",
		}),
	}

	resolved, err := worker.resolveResultVideoURL(sourcePath)
	if err != nil {
		t.Fatalf("resolveResultVideoURL returned error: %v", err)
	}
	if resolved != sourcePath {
		t.Fatalf("resolveResultVideoURL = %q, want %q", resolved, sourcePath)
	}
}

func TestWorkerDownloadResultVideoCopiesAbsoluteLocalPath(t *testing.T) {
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "result.mp4")
	if err := os.WriteFile(sourcePath, []byte("video-bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "copied.mp4")

	worker := &Worker{}
	if err := worker.downloadResultVideo(context.Background(), sourcePath, targetPath); err != nil {
		t.Fatalf("downloadResultVideo returned error: %v", err)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != "video-bytes" {
		t.Fatalf("copied data = %q, want %q", string(data), "video-bytes")
	}
}
