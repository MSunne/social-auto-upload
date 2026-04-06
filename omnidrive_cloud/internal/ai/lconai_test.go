package ai

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/domain"
)

func TestLConAISubmitVideoUsesSeedanceMultipartAndReturnsTaskID(t *testing.T) {
	var (
		capturedAuth    string
		capturedModel   string
		capturedPrompt  string
		capturedSize    string
		capturedSeconds string
		capturedFiles   []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("ParseMediaType returned error: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("unexpected content type %q", mediaType)
		}

		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart returned error: %v", err)
			}
			body, readErr := io.ReadAll(part)
			if readErr != nil {
				t.Fatalf("ReadAll returned error: %v", readErr)
			}
			switch part.FormName() {
			case "model":
				capturedModel = string(body)
			case "prompt":
				capturedPrompt = string(body)
			case "size":
				capturedSize = string(body)
			case "seconds":
				capturedSeconds = string(body)
			case "input_reference":
				capturedFiles = append(capturedFiles, part.FileName())
				if len(body) == 0 {
					t.Fatalf("expected reference media body for %q", part.FileName())
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"video_123","task_id":"task_456","model":"seedance-2.0","status":"queued","created_at":1712697600}`))
	}))
	defer server.Close()

	provider, err := NewLConAIProvider(config.Config{})
	if err != nil {
		t.Fatalf("NewLConAIProvider returned error: %v", err)
	}

	duration := 10
	result, err := provider.SubmitVideo(context.Background(), VideoRequest{
		Model:           "seedance-2.0",
		BaseURL:         server.URL,
		APIKey:          "sk-seedance",
		Prompt:          "生成多参考媒体的视频",
		Resolution:      "1280x720",
		DurationSeconds: &duration,
		ReferenceMedia: []MediaInput{
			{
				Kind:     "image",
				FileName: "frame-1.png",
				MIMEType: "image/png",
				Data:     mustPNGData(t, 1280, 720),
			},
			{
				Kind:     "video",
				FileName: "motion-ref.mp4",
				MIMEType: "video/mp4",
				Data:     []byte("video-ref"),
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitVideo returned error: %v", err)
	}

	if capturedAuth != "sk-seedance" {
		t.Fatalf("unexpected authorization header %q", capturedAuth)
	}
	if capturedModel != "seedance-2.0" || capturedPrompt != "生成多参考媒体的视频" {
		t.Fatalf("unexpected multipart fields model=%q prompt=%q", capturedModel, capturedPrompt)
	}
	if capturedSize != "1280x720" {
		t.Fatalf("unexpected size %q", capturedSize)
	}
	if capturedSeconds != "10" {
		t.Fatalf("unexpected seconds %q", capturedSeconds)
	}
	if got, want := strings.Join(capturedFiles, ","), "frame-1.png,motion-ref.mp4"; got != want {
		t.Fatalf("unexpected input_reference order %q, want %q", got, want)
	}
	if result.ID != "task_456" {
		t.Fatalf("VideoSubmission.ID = %q, want task_456", result.ID)
	}
	if result.Metadata["taskId"] != "task_456" {
		t.Fatalf("expected taskId metadata, got %#v", result.Metadata)
	}
	if result.Metadata["remoteId"] != "video_123" {
		t.Fatalf("expected remoteId metadata, got %#v", result.Metadata)
	}
}

func TestLConAISubmitVideoStripsMergedAspectRatioAndResolutionFromPrompt(t *testing.T) {
	var capturedPrompt string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("ParseMediaType returned error: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("unexpected content type %q", mediaType)
		}

		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart returned error: %v", err)
			}
			body, readErr := io.ReadAll(part)
			if readErr != nil {
				t.Fatalf("ReadAll returned error: %v", readErr)
			}
			if part.FormName() == "prompt" {
				capturedPrompt = string(body)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task_id":"task_987","status":"queued"}`))
	}))
	defer server.Close()

	provider, err := NewLConAIProvider(config.Config{})
	if err != nil {
		t.Fatalf("NewLConAIProvider returned error: %v", err)
	}

	result, err := provider.SubmitVideo(context.Background(), VideoRequest{
		Model:       "seedance-2.0",
		BaseURL:     server.URL,
		APIKey:      "sk-seedance",
		Prompt:      "业务提示词\nAspect ratio: 16:9\nResolution: 1280x720",
		AspectRatio: "16:9",
		Resolution:  "1280x720",
	})
	if err != nil {
		t.Fatalf("SubmitVideo returned error: %v", err)
	}
	if result.ID != "task_987" {
		t.Fatalf("VideoSubmission.ID = %q, want task_987", result.ID)
	}
	if capturedPrompt != "业务提示词" {
		t.Fatalf("prompt = %q, want raw business prompt", capturedPrompt)
	}
}

func TestLConAIGetVideoPrefersVideoURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v1/videos/task_456" {
			t.Fatalf("unexpected request path %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task_456","model":"seedance-2.0","status":"processing","progress":76,"video_url":"https://cdn.example.com/output.mp4","updated_at":1712697600}`))
	}))
	defer server.Close()

	provider, err := NewLConAIProvider(config.Config{})
	if err != nil {
		t.Fatalf("NewLConAIProvider returned error: %v", err)
	}

	status, err := provider.GetVideo(context.Background(), "task_456", "seedance-2.0", server.URL, "sk-seedance")
	if err != nil {
		t.Fatalf("GetVideo returned error: %v", err)
	}
	if status.Status != "running" {
		t.Fatalf("VideoStatus.Status = %q, want running", status.Status)
	}
	if status.ProgressPercent == nil || *status.ProgressPercent != 76 {
		t.Fatalf("VideoStatus.ProgressPercent = %#v, want 76", status.ProgressPercent)
	}
	if status.ContentURL != "https://cdn.example.com/output.mp4" {
		t.Fatalf("VideoStatus.ContentURL = %q, want video_url", status.ContentURL)
	}
}

func TestLConAIDownloadVideoUsesDirectContentURL(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		switch r.URL.Path {
		case "/content/output.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("video-data"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider, err := NewLConAIProvider(config.Config{})
	if err != nil {
		t.Fatalf("NewLConAIProvider returned error: %v", err)
	}

	artifact, err := provider.DownloadVideo(context.Background(), "task_456", "seedance-2.0", server.URL, "sk-seedance", fmt.Sprintf("%s/content/output.mp4", server.URL))
	if err != nil {
		t.Fatalf("DownloadVideo returned error: %v", err)
	}
	if got := strings.Join(requestedPaths, ","); got != "/content/output.mp4" {
		t.Fatalf("unexpected request sequence %q", got)
	}
	if artifact.FileName != "task_456.mp4" {
		t.Fatalf("unexpected filename %q", artifact.FileName)
	}
	if string(artifact.Data) != "video-data" {
		t.Fatalf("unexpected artifact data %q", string(artifact.Data))
	}
}

func TestLConAISubmitVideoNormalizesBaseURLEndpoint(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task_id":"task_789","status":"queued"}`))
	}))
	defer server.Close()

	provider, err := NewLConAIProvider(config.Config{})
	if err != nil {
		t.Fatalf("NewLConAIProvider returned error: %v", err)
	}

	result, err := provider.SubmitVideo(context.Background(), VideoRequest{
		Model:   "seedance-2.0",
		BaseURL: server.URL + "/v1/videos",
		APIKey:  "sk-seedance",
		Prompt:  "测试录入了末级接口的 base_url",
	})
	if err != nil {
		t.Fatalf("SubmitVideo returned error: %v", err)
	}
	if capturedPath != "/v1/videos" {
		t.Fatalf("unexpected request path %q", capturedPath)
	}
	if result.ID != "task_789" {
		t.Fatalf("VideoSubmission.ID = %q, want task_789", result.ID)
	}
}

func TestProviderSourceNormalizesLConVendorAliases(t *testing.T) {
	model := &domain.AIModel{Vendor: "万象龙坤"}
	if got := providerSource(model); got != "lconai" {
		t.Fatalf("providerSource returned %q, want lconai", got)
	}
}
