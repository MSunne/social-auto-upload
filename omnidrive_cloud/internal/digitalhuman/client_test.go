package digitalhuman

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientGenerateVideoEncodesRequestPayload(t *testing.T) {
	var captured GenerateRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/digital-human-flow/step3-generate-video" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"queued","task_id":"remote-123"}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	goodsAssetPath := "/tmp/goods.jpg"
	goodsTitle := "老廖牌香薰"
	resp, rawBody, err := client.GenerateVideo(t.Context(), GenerateRequest{
		CharacterAssetPath: "/tmp/character.jpg",
		Mode:               "digital",
		GoodsAssetPath:     &goodsAssetPath,
		GoodsText:          "大家好，今天给大家推荐一款超好用的香薰",
		GoodsTitle:         &goodsTitle,
		Source:             "runninghub",
		RefAudio:           "/tmp/ref.m4a",
	})
	if err != nil {
		t.Fatalf("GenerateVideo returned error: %v", err)
	}

	if captured.CharacterAssetPath != "/tmp/character.jpg" {
		t.Fatalf("unexpected character path %q", captured.CharacterAssetPath)
	}
	if captured.Mode != "digital" {
		t.Fatalf("unexpected mode %q", captured.Mode)
	}
	if captured.GoodsAssetPath == nil || *captured.GoodsAssetPath != goodsAssetPath {
		t.Fatalf("unexpected goods asset path %+v", captured.GoodsAssetPath)
	}
	if captured.GoodsTitle == nil || *captured.GoodsTitle != goodsTitle {
		t.Fatalf("unexpected goods title %+v", captured.GoodsTitle)
	}
	if captured.GoodsText == "" {
		t.Fatal("expected goods text to be populated")
	}
	if captured.Source != "runninghub" {
		t.Fatalf("unexpected source %q", captured.Source)
	}
	if captured.RefAudio != "/tmp/ref.m4a" {
		t.Fatalf("unexpected ref audio %q", captured.RefAudio)
	}
	if resp.TaskID != "remote-123" {
		t.Fatalf("unexpected task id %q", resp.TaskID)
	}
	if !strings.Contains(string(rawBody), `"task_id":"remote-123"`) {
		t.Fatalf("expected raw body to include task id, got %s", string(rawBody))
	}
}

func TestClientGetTaskDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/digital-human-flow/step4-check-status/remote-123" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"task_id":"remote-123",
			"task_type":"digital_human",
			"status":"completed",
			"progress":{"current":4,"total":4,"percentage":100,"message":"done"},
			"result":{"video_url":"https://example.com/result.mp4"},
			"request_params":{"mode":"digital"}
		}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	task, rawBody, err := client.GetTask(t.Context(), "remote-123")
	if err != nil {
		t.Fatalf("GetTask returned error: %v", err)
	}
	if task.TaskID != "remote-123" {
		t.Fatalf("unexpected task id %q", task.TaskID)
	}
	if task.Status != "completed" {
		t.Fatalf("unexpected status %q", task.Status)
	}
	if task.Progress == nil || task.Progress.Percentage != 100 {
		t.Fatalf("unexpected progress %+v", task.Progress)
	}
	if got := extractResultVideoURL(task.Result); got != "https://example.com/result.mp4" {
		t.Fatalf("unexpected result video url %q", got)
	}
	if !strings.Contains(string(rawBody), `"status":"completed"`) {
		t.Fatalf("expected raw body to include status, got %s", string(rawBody))
	}
}
