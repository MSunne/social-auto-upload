package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omnidrive_cloud/internal/config"
)

func TestResolveEndpointURLAvoidsDuplicatedVideoPath(t *testing.T) {
	provider, err := NewAPIYIProvider(config.Config{
		APIYIBaseURL: "https://api.apiyi.com",
		APIYIApiKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("NewAPIYIProvider returned error: %v", err)
	}

	cases := []struct {
		name     string
		baseURL  string
		endpoint string
		want     string
	}{
		{
			name:     "host_only",
			baseURL:  "https://api.apiyi.com",
			endpoint: "/v1/videos",
			want:     "https://api.apiyi.com/v1/videos",
		},
		{
			name:     "root_version_path",
			baseURL:  "https://api.apiyi.com/v1",
			endpoint: "/v1/videos",
			want:     "https://api.apiyi.com/v1/videos",
		},
		{
			name:     "full_endpoint_path",
			baseURL:  "https://api.apiyi.com/v1/videos",
			endpoint: "/v1/videos",
			want:     "https://api.apiyi.com/v1/videos",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := provider.resolveEndpointURL(tc.baseURL, tc.endpoint); got != tc.want {
				t.Fatalf("resolveEndpointURL(%q, %q) = %q, want %q", tc.baseURL, tc.endpoint, got, tc.want)
			}
		})
	}
}

func TestSubmitVideoUsesSoraBearerAuthAndOfficialFields(t *testing.T) {
	var capturedPath string
	var capturedAuth string
	var capturedPrompt string
	var capturedModel string
	var capturedSeconds string
	var capturedSize string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			t.Fatalf("ParseMultipartForm returned error: %v", err)
		}
		capturedPrompt = r.FormValue("prompt")
		capturedModel = r.FormValue("model")
		capturedSeconds = r.FormValue("seconds")
		capturedSize = r.FormValue("size")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"video_123","model":"sora-2","status":"queued","created_at":1712697600}`))
	}))
	defer server.Close()

	provider, err := NewAPIYIProvider(config.Config{
		APIYIBaseURL: server.URL,
		APIYIApiKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("NewAPIYIProvider returned error: %v", err)
	}

	duration := 8
	_, err = provider.SubmitVideo(context.Background(), VideoRequest{
		Model:           "sora-2",
		BaseURL:         server.URL + "/v1/videos",
		APIKey:          "sk-sora",
		Prompt:          "让这个产品镜头缓慢推进",
		Resolution:      "1280x720",
		AspectRatio:     "16:9",
		DurationSeconds: &duration,
		ReferenceImages: []MediaInput{{
			Data:     []byte("image-bytes"),
			MIMEType: "image/png",
			FileName: "product.png",
		}},
	})
	if err != nil {
		t.Fatalf("SubmitVideo returned error: %v", err)
	}

	if capturedPath != "/v1/videos" {
		t.Fatalf("unexpected request path %q", capturedPath)
	}
	if capturedAuth != "Bearer sk-sora" {
		t.Fatalf("unexpected authorization header %q", capturedAuth)
	}
	if capturedPrompt != "让这个产品镜头缓慢推进" || capturedModel != "sora-2" {
		t.Fatalf("unexpected multipart fields: prompt=%q model=%q", capturedPrompt, capturedModel)
	}
	if capturedSeconds != "8" {
		t.Fatalf("unexpected seconds field %q", capturedSeconds)
	}
	if capturedSize != "1280x720" {
		t.Fatalf("unexpected size field %q", capturedSize)
	}
}

func TestSubmitVideoPassesConfiguredSoraDurationThrough(t *testing.T) {
	var capturedSeconds string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			t.Fatalf("ParseMultipartForm returned error: %v", err)
		}
		capturedSeconds = r.FormValue("seconds")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"video_custom_duration","model":"sora-2-pro","status":"queued","created_at":1712697600}`))
	}))
	defer server.Close()

	provider, err := NewAPIYIProvider(config.Config{
		APIYIBaseURL: server.URL,
		APIYIApiKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("NewAPIYIProvider returned error: %v", err)
	}

	duration := 15
	_, err = provider.SubmitVideo(context.Background(), VideoRequest{
		Model:           "sora-2-pro",
		BaseURL:         server.URL + "/v1/videos",
		APIKey:          "sk-sora",
		Prompt:          "生成珠宝广告视频",
		Resolution:      "1792x1024",
		AspectRatio:     "16:9",
		DurationSeconds: &duration,
		ReferenceImages: []MediaInput{{
			Data:     []byte("image-bytes"),
			MIMEType: "image/png",
			FileName: "product.png",
		}},
	})
	if err != nil {
		t.Fatalf("SubmitVideo returned error: %v", err)
	}

	if capturedSeconds != "15" {
		t.Fatalf("unexpected seconds field %q", capturedSeconds)
	}
}

func TestSubmitVideoUsesVeoJSONWithoutReferences(t *testing.T) {
	var capturedPath string
	var capturedAuth string
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll returned error: %v", err)
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("json.Unmarshal returned error: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"video_abc","model":"veo-3.1-fast","status":"queued","created":1762181811}`))
	}))
	defer server.Close()

	provider, err := NewAPIYIProvider(config.Config{
		APIYIBaseURL: server.URL,
		APIYIApiKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("NewAPIYIProvider returned error: %v", err)
	}

	_, err = provider.SubmitVideo(context.Background(), VideoRequest{
		Model:   "veo-3.1-fast",
		BaseURL: server.URL + "/v1",
		APIKey:  "sk-veo",
		Prompt:  "生成产品展示视频",
	})
	if err != nil {
		t.Fatalf("SubmitVideo returned error: %v", err)
	}

	if capturedPath != "/v1/videos" {
		t.Fatalf("unexpected request path %q", capturedPath)
	}
	if capturedAuth != "sk-veo" {
		t.Fatalf("unexpected authorization header %q", capturedAuth)
	}
	if payload["prompt"] != "生成产品展示视频" || payload["model"] != "veo-3.1-fast" {
		t.Fatalf("unexpected payload %#v", payload)
	}
	if _, exists := payload["seconds"]; exists {
		t.Fatalf("veo payload should not contain seconds: %#v", payload)
	}
}

func TestGenerateImageUsesGeminiImageConfigAndInlineData(t *testing.T) {
	var capturedPath string
	var capturedAuth string
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll returned error: %v", err)
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("json.Unmarshal returned error: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}]}}]}`))
	}))
	defer server.Close()

	provider, err := NewAPIYIProvider(config.Config{
		APIYIBaseURL: server.URL,
		APIYIApiKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("NewAPIYIProvider returned error: %v", err)
	}

	_, err = provider.GenerateImage(context.Background(), ImageRequest{
		Model:       "gemini-3-pro-image-preview",
		BaseURL:     server.URL,
		APIKey:      "sk-image",
		Prompt:      "生成产品主图",
		AspectRatio: "16:9",
		Resolution:  "1344x768",
		ReferenceImages: []MediaInput{{
			Data:     []byte("img"),
			MIMEType: "image/png",
			FileName: "ref.png",
		}},
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}

	if capturedPath != "/v1beta/models/gemini-3-pro-image-preview:generateContent" {
		t.Fatalf("unexpected request path %q", capturedPath)
	}
	if capturedAuth != "Bearer sk-image" {
		t.Fatalf("unexpected authorization header %q", capturedAuth)
	}

	generationConfig, _ := payload["generationConfig"].(map[string]any)
	imageConfig, _ := generationConfig["imageConfig"].(map[string]any)
	if imageConfig["aspectRatio"] != "16:9" {
		t.Fatalf("unexpected aspectRatio %#v", imageConfig["aspectRatio"])
	}
	if imageConfig["imageSize"] != "1K" {
		t.Fatalf("unexpected imageSize %#v", imageConfig["imageSize"])
	}

	contents, _ := payload["contents"].([]any)
	content, _ := contents[0].(map[string]any)
	parts, _ := content["parts"].([]any)
	firstPart, _ := parts[0].(map[string]any)
	if _, ok := firstPart["inline_data"]; !ok {
		t.Fatalf("expected inline_data part, got %#v", firstPart)
	}
}

func TestIsRetryableProviderErrorTreatsClosedNetworkConnectionAsTransient(t *testing.T) {
	err := errors.New(`Post "https://api.apiyi.com/v1beta/models/gemini-3-pro-image-preview:generateContent": write tcp 198.18.0.1:51244->198.18.3.36:443: use of closed network connection`)
	if !isRetryableProviderError(err) {
		t.Fatalf("expected closed network connection error to be retryable")
	}
}

func TestDownloadVideoParsesSoraVideoURL(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/videos/") && strings.HasSuffix(r.URL.Path, "/content"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"video_url":"` + server.URL + `/files/result.mp4"}`))
		case r.URL.Path == "/files/result.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "result.mp4"}))
			_, _ = w.Write([]byte("video-bytes"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider, err := NewAPIYIProvider(config.Config{
		APIYIBaseURL: server.URL,
		APIYIApiKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("NewAPIYIProvider returned error: %v", err)
	}

	artifact, err := provider.DownloadVideo(context.Background(), "video_123", "sora-2", server.URL, "sk-sora")
	if err != nil {
		t.Fatalf("DownloadVideo returned error: %v", err)
	}
	if artifact.FileName != "result.mp4" {
		t.Fatalf("unexpected fileName %q", artifact.FileName)
	}
	if string(artifact.Data) != "video-bytes" {
		t.Fatalf("unexpected artifact data %q", string(artifact.Data))
	}
}

func TestVeoFLMultipartDoesNotSendUnsupportedFields(t *testing.T) {
	var formValues map[string][]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader returned error: %v", err)
		}
		formValues = map[string][]string{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart returned error: %v", err)
			}
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("ReadAll returned error: %v", err)
			}
			formValues[part.FormName()] = append(formValues[part.FormName()], string(data))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"video_fl","model":"veo-3.1-fast-fl","status":"queued","created":1762181811}`))
	}))
	defer server.Close()

	provider, err := NewAPIYIProvider(config.Config{
		APIYIBaseURL: server.URL,
		APIYIApiKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("NewAPIYIProvider returned error: %v", err)
	}

	duration := 8
	_, err = provider.SubmitVideo(context.Background(), VideoRequest{
		Model:           "veo-3.1-fast-fl",
		BaseURL:         server.URL,
		APIKey:          "sk-veo",
		Prompt:          "让首帧画面动起来",
		AspectRatio:     "16:9",
		Resolution:      "1280x720",
		DurationSeconds: &duration,
		ReferenceImages: []MediaInput{{
			Data:     []byte("frame"),
			MIMEType: "image/jpeg",
			FileName: "frame.jpg",
		}},
	})
	if err != nil {
		t.Fatalf("SubmitVideo returned error: %v", err)
	}

	if _, ok := formValues["seconds"]; ok {
		t.Fatalf("veo fl request should not contain seconds: %#v", formValues)
	}
	if _, ok := formValues["size"]; ok {
		t.Fatalf("veo fl request should not contain size: %#v", formValues)
	}
	if _, ok := formValues["input_reference_mime_type"]; ok {
		t.Fatalf("veo fl request should not contain input_reference_mime_type: %#v", formValues)
	}
	if len(formValues["input_reference"]) != 1 {
		t.Fatalf("expected one input_reference, got %#v", formValues["input_reference"])
	}
}
