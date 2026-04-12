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
		if r.URL.Path != "/api/video/generate/async" {
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

	goodsTitle := "老廖牌香薰"
	resp, rawBody, err := client.GenerateVideoAsync(t.Context(), GenerateRequest{
		Text:     "大家好，今天给大家推荐一款超好用的香薰",
		Mode:     "fixed",
		Title:    &goodsTitle,
		RefAudio: stringPtr("/tmp/ref.m4a"),
		TemplateParams: map[string]any{
			"character_asset_path": "/tmp/character.jpg",
			"goods_asset_path":     "/tmp/goods.jpg",
			"source":               "omnidrive_cloud",
		},
	})
	if err != nil {
		t.Fatalf("GenerateVideoAsync returned error: %v", err)
	}

	if captured.Text == "" {
		t.Fatal("expected text to be populated")
	}
	if captured.Mode != "fixed" {
		t.Fatalf("unexpected mode %q", captured.Mode)
	}
	if captured.Title == nil || *captured.Title != goodsTitle {
		t.Fatalf("unexpected title %+v", captured.Title)
	}
	if captured.RefAudio == nil || *captured.RefAudio != "/tmp/ref.m4a" {
		t.Fatalf("unexpected ref audio %v", captured.RefAudio)
	}
	if captured.TemplateParams["character_asset_path"] != "/tmp/character.jpg" {
		t.Fatalf("unexpected character asset path %#v", captured.TemplateParams["character_asset_path"])
	}
	if captured.TemplateParams["goods_asset_path"] != "/tmp/goods.jpg" {
		t.Fatalf("unexpected goods asset path %#v", captured.TemplateParams["goods_asset_path"])
	}
	if captured.TemplateParams["source"] != "omnidrive_cloud" {
		t.Fatalf("unexpected source %#v", captured.TemplateParams["source"])
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
		if r.URL.Path != "/api/tasks/remote-123" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"task_id":"remote-123",
			"task_type":"video_generate",
			"status":"completed",
			"progress":{"current":4,"total":4,"percentage":100,"message":"done"},
			"result":{"output":{"video_url":"https://example.com/result.mp4"}},
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
