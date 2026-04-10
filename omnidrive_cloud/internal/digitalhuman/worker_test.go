package digitalhuman

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/storage"
)

func TestWorkerMaterializeTaskAssetsBuildsGenerateRequest(t *testing.T) {
	storageService, err := storage.New(config.Config{
		LocalStorageDir: t.TempDir(),
		PublicBaseURL:   "http://localhost:5409",
	})
	if err != nil {
		t.Fatalf("storage.New returned error: %v", err)
	}

	characterObject, err := storageService.SaveBytes(t.Context(), "digital-human/test-user/task-1/character/character.jpg", "image/jpeg", []byte("character-bytes"))
	if err != nil {
		t.Fatalf("SaveBytes character returned error: %v", err)
	}
	audioObject, err := storageService.SaveBytes(t.Context(), "digital-human/test-user/task-1/ref/audio.m4a", "audio/mp4", []byte("audio-bytes"))
	if err != nil {
		t.Fatalf("SaveBytes audio returned error: %v", err)
	}
	goodsObject, err := storageService.SaveBytes(t.Context(), "digital-human/test-user/task-1/goods/goods.jpg", "image/jpeg", []byte("goods-bytes"))
	if err != nil {
		t.Fatalf("SaveBytes goods returned error: %v", err)
	}

	worker := &Worker{
		app: &appstate.App{
			Config:  config.Config{},
			Logger:  slog.Default(),
			Storage: storageService,
		},
	}

	goodsTitle := "老廖牌香薰"
	task := &domain.DigitalHumanTask{
		ID:          "task-1",
		OwnerUserID: "test-user",
		Mode:        "digital",
		Source:      "runninghub",
		GoodsTitle:  &goodsTitle,
		GoodsText:   "大家好，今天给大家推荐一款超好用的香薰",
		CharacterAsset: domain.DigitalHumanAsset{
			StorageKey: characterObject.StorageKey,
			FileName:   "character.jpg",
			MimeType:   "image/jpeg",
		},
		RefAudioAsset: domain.DigitalHumanAsset{
			StorageKey: audioObject.StorageKey,
			FileName:   "voice.m4a",
			MimeType:   "audio/mp4",
		},
		GoodsAsset: &domain.DigitalHumanAsset{
			StorageKey: goodsObject.StorageKey,
			FileName:   "goods.jpg",
			MimeType:   "image/jpeg",
		},
	}

	tempDir, request, err := worker.materializeTaskAssets(t.Context(), task)
	if err != nil {
		t.Fatalf("materializeTaskAssets returned error: %v", err)
	}
	defer cleanupWorkingDir(slog.Default(), tempDir)

	if request.Mode != "digital" {
		t.Fatalf("unexpected mode %q", request.Mode)
	}
	if request.Source != "runninghub" {
		t.Fatalf("unexpected source %q", request.Source)
	}
	if request.GoodsTitle == nil || *request.GoodsTitle != goodsTitle {
		t.Fatalf("unexpected goods title %+v", request.GoodsTitle)
	}
	if request.GoodsAssetPath == nil || *request.GoodsAssetPath == "" {
		t.Fatalf("expected goods asset path, got %+v", request.GoodsAssetPath)
	}

	characterBytes, err := os.ReadFile(request.CharacterAssetPath)
	if err != nil {
		t.Fatalf("ReadFile character returned error: %v", err)
	}
	if string(characterBytes) != "character-bytes" {
		t.Fatalf("unexpected character temp file content %q", string(characterBytes))
	}
	audioBytes, err := os.ReadFile(request.RefAudio)
	if err != nil {
		t.Fatalf("ReadFile audio returned error: %v", err)
	}
	if string(audioBytes) != "audio-bytes" {
		t.Fatalf("unexpected audio temp file content %q", string(audioBytes))
	}
}

func TestWorkerSaveResultAssetMirrorsRemoteVideo(t *testing.T) {
	videoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = io.WriteString(w, "video-bytes")
	}))
	defer videoServer.Close()

	storageService, err := storage.New(config.Config{
		LocalStorageDir: t.TempDir(),
		PublicBaseURL:   "http://localhost:5409",
	})
	if err != nil {
		t.Fatalf("storage.New returned error: %v", err)
	}

	worker := &Worker{
		app: &appstate.App{
			Config:  config.Config{},
			Logger:  slog.Default(),
			Storage: storageService,
		},
		client: NewClient(config.Config{}),
		probeDuration: func(ctx context.Context, path string) (float64, error) {
			return 4.2, nil
		},
	}

	task := &domain.DigitalHumanTask{
		ID:          "task-2",
		OwnerUserID: "test-user",
	}

	result, err := worker.saveResultAsset(t.Context(), task, &RemoteTask{
		Result: map[string]any{
			"video_url": videoServer.URL + "/result.mp4",
		},
	})
	if err != nil {
		t.Fatalf("saveResultAsset returned error: %v", err)
	}
	if result == nil || result.asset == nil {
		t.Fatal("expected result asset")
	}
	if result.actualDurationSeconds == nil || *result.actualDurationSeconds != 5 {
		t.Fatalf("expected rounded actual duration, got %+v", result.actualDurationSeconds)
	}
	if result.asset.StorageKey == "" || result.asset.PublicURL == "" {
		t.Fatalf("expected mirrored storage metadata, got %+v", result.asset)
	}
	data, contentType, err := storageService.ReadBytes(t.Context(), result.asset.StorageKey)
	if err != nil {
		t.Fatalf("ReadBytes returned error: %v", err)
	}
	if string(data) != "video-bytes" {
		t.Fatalf("unexpected mirrored bytes %q", string(data))
	}
	if contentType != "video/mp4" {
		t.Fatalf("unexpected content type %q", contentType)
	}
}
