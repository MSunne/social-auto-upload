package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
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
			Data:     mustPNGData(t, 1280, 720),
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
			Data:     mustPNGData(t, 1792, 1024),
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

func TestSubmitVideoResizesSoraReferenceImageToRequestedSize(t *testing.T) {
	var capturedWidth int
	var capturedHeight int
	var capturedMime string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			t.Fatalf("ParseMultipartForm returned error: %v", err)
		}
		capturedMime = r.FormValue("input_reference_mime_type")
		file, _, err := r.FormFile("input_reference")
		if err != nil {
			t.Fatalf("FormFile returned error: %v", err)
		}
		defer file.Close()

		config, _, err := image.DecodeConfig(file)
		if err != nil {
			t.Fatalf("DecodeConfig returned error: %v", err)
		}
		capturedWidth = config.Width
		capturedHeight = config.Height

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"video_resized","model":"sora-2","status":"queued","created_at":1712697600}`))
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
		BaseURL:         server.URL,
		APIKey:          "sk-sora",
		Prompt:          "让这个珠宝首图动起来",
		Resolution:      "1280x720",
		AspectRatio:     "16:9",
		DurationSeconds: &duration,
		ReferenceImages: []MediaInput{{
			Data:     mustPNGData(t, 640, 360),
			MIMEType: "image/png",
			FileName: "product.png",
		}},
	})
	if err != nil {
		t.Fatalf("SubmitVideo returned error: %v", err)
	}

	if capturedWidth != 1280 || capturedHeight != 720 {
		t.Fatalf("unexpected resized image dimensions %dx%d", capturedWidth, capturedHeight)
	}
	if capturedMime != "image/png" {
		t.Fatalf("unexpected input_reference_mime_type %q", capturedMime)
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

func TestGenerateChatStreamAggregatesSSEChunks(t *testing.T) {
	var capturedPath string
	var capturedAuth string
	var capturedPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll returned error: %v", err)
		}
		if err := json.Unmarshal(body, &capturedPayload); err != nil {
			t.Fatalf("json.Unmarshal returned error: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"你好\"},\"finish_reason\":\"\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"，世界\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":8}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	provider, err := NewAPIYIProvider(config.Config{
		APIYIBaseURL: server.URL,
		APIYIApiKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("NewAPIYIProvider returned error: %v", err)
	}

	chunks := make([]ChatStreamChunk, 0, 4)
	result, err := provider.GenerateChatStream(context.Background(), ChatRequest{
		Model:   "gpt-5.4",
		BaseURL: server.URL,
		APIKey:  "sk-chat",
		Messages: []ChatMessage{
			{Role: "user", Content: "say hello"},
		},
	}, func(chunk ChatStreamChunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("GenerateChatStream returned error: %v", err)
	}

	if capturedPath != "/v1/chat/completions" {
		t.Fatalf("unexpected request path %q", capturedPath)
	}
	if capturedAuth != "Bearer sk-chat" {
		t.Fatalf("unexpected authorization header %q", capturedAuth)
	}
	if capturedPayload["stream"] != true {
		t.Fatalf("expected stream=true payload, got %#v", capturedPayload["stream"])
	}
	if result.Text != "你好，世界" {
		t.Fatalf("unexpected aggregated text %q", result.Text)
	}
	if result.Role != "assistant" {
		t.Fatalf("unexpected role %q", result.Role)
	}
	if result.FinishReason != "stop" {
		t.Fatalf("unexpected finish reason %q", result.FinishReason)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 stream callbacks, got %d", len(chunks))
	}
	if chunks[0].Delta != "你好" || chunks[1].Delta != "，世界" {
		t.Fatalf("unexpected delta sequence %#v", chunks)
	}
	if !chunks[2].Done {
		t.Fatalf("expected last callback to mark done, got %#v", chunks[2])
	}
}

func TestGenerateChatStreamFailsWhenTerminalEventIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"半截回复\"},\"finish_reason\":\"\"}]}\n\n"))
	}))
	defer server.Close()

	provider, err := NewAPIYIProvider(config.Config{
		APIYIBaseURL: server.URL,
		APIYIApiKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("NewAPIYIProvider returned error: %v", err)
	}

	_, err = provider.GenerateChatStream(context.Background(), ChatRequest{
		Model:   "gpt-5.4",
		BaseURL: server.URL,
		APIKey:  "sk-chat",
		Messages: []ChatMessage{
			{Role: "user", Content: "say hello"},
		},
	}, nil)
	if err == nil {
		t.Fatal("expected GenerateChatStream to fail without terminal event")
	}
	if !strings.Contains(err.Error(), "terminal event") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestBuildChatPayloadUsesGPT5SpecificFields(t *testing.T) {
	temperature := 0.5
	maxTokens := 1200

	payload := buildChatPayload(ChatRequest{
		Model:       "gpt-5.4",
		Messages:    []ChatMessage{{Role: "user", Content: "hello"}},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	}, true)

	if payload["stream"] != true {
		t.Fatalf("expected stream flag to be set")
	}
	if _, exists := payload["temperature"]; exists {
		t.Fatalf("gpt-5 payload should omit temperature when it is not 1: %#v", payload)
	}
	if _, exists := payload["max_tokens"]; exists {
		t.Fatalf("gpt-5 payload should not use max_tokens: %#v", payload)
	}
	if payload["max_completion_tokens"] != 1200 {
		t.Fatalf("unexpected max_completion_tokens %#v", payload["max_completion_tokens"])
	}
}

func TestGenerateChatStreamInlinesImageURLsForGPT5Models(t *testing.T) {
	var capturedPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/img.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte{
				0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
				0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
				0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
				0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
				0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
				0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
				0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
				0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
				0x44, 0xae, 0x42, 0x60, 0x82,
			})
		case "/v1/chat/completions":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll returned error: %v", err)
			}
			if err := json.Unmarshal(body, &capturedPayload); err != nil {
				t.Fatalf("json.Unmarshal returned error: %v", err)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
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

	_, err = provider.GenerateChatStream(context.Background(), ChatRequest{
		Model:   "gpt-5.4",
		BaseURL: server.URL,
		APIKey:  "sk-chat",
		Messages: []ChatMessage{
			{
				Role: "user",
				Content: []map[string]any{
					{"type": "text", "text": "看看这张图"},
					{
						"type": "image_url",
						"image_url": map[string]any{
							"url": server.URL + "/img.png",
						},
					},
				},
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("GenerateChatStream returned error: %v", err)
	}

	messages, ok := capturedPayload["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("unexpected messages payload %#v", capturedPayload["messages"])
	}
	message, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected message payload %#v", messages[0])
	}
	parts, ok := message["content"].([]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("unexpected content payload %#v", message["content"])
	}
	imagePart, ok := parts[1].(map[string]any)
	if !ok {
		t.Fatalf("unexpected image part %#v", parts[1])
	}
	imageURL, ok := imagePart["image_url"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected image_url payload %#v", imagePart["image_url"])
	}
	value, _ := imageURL["url"].(string)
	if !strings.HasPrefix(value, "data:image/png;base64,") {
		t.Fatalf("expected gpt-5 image attachment to be inlined, got %q", value)
	}
}

func TestBuildChatPayloadUsesStandardFieldsForNonGPT5(t *testing.T) {
	temperature := 0.4
	maxTokens := 600

	payload := buildChatPayload(ChatRequest{
		Model:       "gemini-3.1-pro-preview",
		Messages:    []ChatMessage{{Role: "user", Content: "hello"}},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	}, false)

	if payload["temperature"] != 0.4 {
		t.Fatalf("expected standard chat payload to keep temperature, got %#v", payload["temperature"])
	}
	if payload["max_tokens"] != 600 {
		t.Fatalf("expected standard chat payload to use max_tokens, got %#v", payload["max_tokens"])
	}
	if _, exists := payload["max_completion_tokens"]; exists {
		t.Fatalf("non gpt-5 payload should not use max_completion_tokens: %#v", payload)
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

func TestVeoFLMultipartSendsTwoInputReferencesInOrder(t *testing.T) {
	firstFrame := mustPNGData(t, 320, 320)
	lastFrame := mustPNGData(t, 480, 480)
	var inputReferences [][]byte
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
			if part.FormName() == "input_reference" {
				inputReferences = append(inputReferences, append([]byte(nil), data...))
				continue
			}
			formValues[part.FormName()] = append(formValues[part.FormName()], string(data))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"video_fl_multi","model":"veo-3.1-fast-fl","status":"queued","created":1762181811}`))
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
		Prompt:          "从白天过渡到夜晚，镜头保持不动",
		AspectRatio:     "16:9",
		Resolution:      "1280x720",
		DurationSeconds: &duration,
		ReferenceImages: []MediaInput{
			{
				Data:     firstFrame,
				MIMEType: "image/png",
				FileName: "first.png",
			},
			{
				Data:     lastFrame,
				MIMEType: "image/png",
				FileName: "last.png",
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitVideo returned error: %v", err)
	}

	if len(inputReferences) != 2 {
		t.Fatalf("expected 2 input_reference parts, got %d", len(inputReferences))
	}
	if !bytes.Equal(inputReferences[0], firstFrame) {
		t.Fatalf("first input_reference was not submitted in original order")
	}
	if !bytes.Equal(inputReferences[1], lastFrame) {
		t.Fatalf("second input_reference was not submitted in original order")
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
}

func mustPNGData(t *testing.T, width int, height int) []byte {
	t.Helper()

	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			canvas.Set(x, y, color.NRGBA{
				R: uint8((x * 255) / max(1, width)),
				G: uint8((y * 255) / max(1, height)),
				B: 180,
				A: 255,
			})
		}
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		t.Fatalf("png.Encode returned error: %v", err)
	}
	return buffer.Bytes()
}
