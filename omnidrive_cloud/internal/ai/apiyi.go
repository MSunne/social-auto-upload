package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"omnidrive_cloud/internal/config"
)

type APIYIProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewAPIYIProvider(cfg config.Config) (*APIYIProvider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.APIYIBaseURL), "/")
	apiKey := strings.TrimSpace(cfg.APIYIApiKey)
	if baseURL == "" {
		return nil, fmt.Errorf("OMNIDRIVE_APIYI_BASE_URL is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OMNIDRIVE_APIYI_API_KEY is required")
	}
	return &APIYIProvider{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 3 * time.Minute,
		},
	}, nil
}

func (p *APIYIProvider) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResult, error) {
	payload := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		payload["max_tokens"] = *req.MaxTokens
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	body, err := p.doJSON(ctx, req.BaseURL, req.APIKey, http.MethodPost, "/v1/chat/completions", data, true)
	if err != nil {
		return nil, err
	}

	var response struct {
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage map[string]any `json:"usage"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("chat response did not contain choices")
	}

	choice := response.Choices[0]
	return &ChatResult{
		Text:         extractText(choice.Message.Content),
		Role:         strings.TrimSpace(choice.Message.Role),
		Usage:        response.Usage,
		FinishReason: strings.TrimSpace(choice.FinishReason),
		RawResponse:  body,
	}, nil
}

func (p *APIYIProvider) GenerateImage(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	parts := make([]map[string]any, 0, len(req.ReferenceImages)+1)
	for _, media := range req.ReferenceImages {
		payload, err := p.mediaToGeminiPart(ctx, media)
		if err != nil {
			return nil, err
		}
		parts = append(parts, payload)
	}
	parts = append(parts, map[string]any{"text": req.Prompt})

	payload := map[string]any{
		"contents": []map[string]any{
			{"parts": parts},
		},
		"generationConfig": map[string]any{
			"responseModalities": []string{"TEXT", "IMAGE"},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1beta/models/%s:generateContent", url.PathEscape(req.Model))
	body, err := p.doJSON(ctx, req.BaseURL, req.APIKey, http.MethodPost, path, data, true)
	if err != nil {
		return nil, err
	}

	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData *struct {
						MIMEType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	result := &ImageResult{Images: []BinaryArtifact{}, RawResponse: body}
	for _, candidate := range response.Candidates {
		for index, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				result.Text = strings.TrimSpace(part.Text)
			}
			if part.InlineData == nil || strings.TrimSpace(part.InlineData.Data) == "" {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(part.InlineData.Data))
			if err != nil {
				return nil, err
			}
			mimeType := strings.TrimSpace(part.InlineData.MIMEType)
			if mimeType == "" {
				mimeType = "image/png"
			}
			fileName := fmt.Sprintf("image-%d%s", index+1, extensionForMIME(mimeType, ".png"))
			result.Images = append(result.Images, BinaryArtifact{
				FileName:  fileName,
				MIMEType:  mimeType,
				Data:      data,
				SizeBytes: int64(len(data)),
			})
		}
	}
	if len(result.Images) == 0 {
		return nil, fmt.Errorf("image response did not contain image data")
	}
	return result, nil
}

func (p *APIYIProvider) SubmitVideo(ctx context.Context, req VideoRequest) (*VideoSubmission, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("model", req.Model)
	_ = writer.WriteField("prompt", req.Prompt)
	if strings.TrimSpace(req.AspectRatio) != "" {
		_ = writer.WriteField("aspect_ratio", req.AspectRatio)
	}
	if strings.TrimSpace(req.Resolution) != "" {
		_ = writer.WriteField("resolution", req.Resolution)
	}
	if req.DurationSeconds != nil && *req.DurationSeconds > 0 {
		_ = writer.WriteField("duration", fmt.Sprintf("%d", *req.DurationSeconds))
	}

	for _, media := range req.ReferenceImages {
		data, mimeType, fileName, err := p.resolveMediaInput(ctx, media)
		if err != nil {
			return nil, err
		}
		part, err := writer.CreateFormFile("input_reference", fileName)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(data); err != nil {
			return nil, err
		}
		if mimeType != "" {
			_ = writer.WriteField("input_reference_mime_type", mimeType)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.resolveBaseURL(req.BaseURL)+"/v1/videos", &body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", p.resolveAPIKey(req.APIKey))
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := ensureHTTPStatus(resp, responseBody); err != nil {
		return nil, err
	}

	var response struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Status  string `json:"status"`
		Created int64  `json:"created"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}
	var createdAt *time.Time
	if response.Created > 0 {
		parsed := time.Unix(response.Created, 0).UTC()
		createdAt = &parsed
	}
	return &VideoSubmission{
		ID:          strings.TrimSpace(response.ID),
		Model:       strings.TrimSpace(response.Model),
		Status:      normalizeRemoteVideoStatus(response.Status),
		CreatedAt:   createdAt,
		RawResponse: responseBody,
	}, nil
}

func (p *APIYIProvider) GetVideo(ctx context.Context, videoID string, baseURL string, apiKey string) (*VideoStatus, error) {
	body, err := p.doVideoRequest(ctx, baseURL, apiKey, http.MethodGet, fmt.Sprintf("/v1/videos/%s", url.PathEscape(videoID)), nil, "")
	if err != nil {
		return nil, err
	}

	var response struct {
		ID     string `json:"id"`
		Model  string `json:"model"`
		Status string `json:"status"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Content   string         `json:"content_url"`
		OutputURL string         `json:"url"`
		Output    map[string]any `json:"output"`
		Updated   int64          `json:"updated"`
		Created   int64          `json:"created"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	status := &VideoStatus{
		ID:          strings.TrimSpace(response.ID),
		Model:       strings.TrimSpace(response.Model),
		Status:      normalizeRemoteVideoStatus(response.Status),
		ContentURL:  firstNonEmptyString(response.Content, response.OutputURL, stringValue(response.Output["url"]), stringValue(response.Output["content_url"])),
		RawResponse: body,
	}
	if response.Error != nil {
		status.FailureCode = strings.TrimSpace(response.Error.Code)
		status.Message = strings.TrimSpace(response.Error.Message)
	}
	if response.Created > 0 {
		parsed := time.Unix(response.Created, 0).UTC()
		status.CreatedAt = &parsed
	}
	if response.Updated > 0 {
		parsed := time.Unix(response.Updated, 0).UTC()
		status.UpdatedAt = &parsed
	}
	return status, nil
}

func (p *APIYIProvider) DownloadVideo(ctx context.Context, videoID string, baseURL string, apiKey string) (*BinaryArtifact, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.resolveBaseURL(baseURL)+fmt.Sprintf("/v1/videos/%s/content", url.PathEscape(videoID)), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", p.resolveAPIKey(apiKey))

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := ensureHTTPStatus(resp, body); err != nil {
		return nil, err
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "application/json") {
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
		contentURL := firstNonEmptyString(stringValue(payload["url"]), stringValue(payload["content_url"]))
		if contentURL == "" {
			return nil, fmt.Errorf("video content response did not contain a download URL")
		}
		return p.downloadBinary(ctx, contentURL, "video.mp4", "video/mp4")
	}

	fileName := fileNameFromResponse(resp.Header.Get("Content-Disposition"), fmt.Sprintf("%s.mp4", videoID))
	if contentType == "" {
		contentType = "video/mp4"
	}
	return &BinaryArtifact{
		FileName:  fileName,
		MIMEType:  contentType,
		Data:      body,
		SizeBytes: int64(len(body)),
	}, nil
}

func (p *APIYIProvider) doJSON(ctx context.Context, baseURL string, apiKey string, method string, path string, data []byte, bearer bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, p.resolveBaseURL(baseURL)+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	resolvedAPIKey := p.resolveAPIKey(apiKey)
	if bearer {
		req.Header.Set("Authorization", "Bearer "+resolvedAPIKey)
	} else {
		req.Header.Set("Authorization", resolvedAPIKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := ensureHTTPStatus(resp, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (p *APIYIProvider) doVideoRequest(ctx context.Context, baseURL string, apiKey string, method string, path string, body io.Reader, contentType string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, p.resolveBaseURL(baseURL)+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", p.resolveAPIKey(apiKey))
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := ensureHTTPStatus(resp, responseBody); err != nil {
		return nil, err
	}
	return responseBody, nil
}

func (p *APIYIProvider) resolveBaseURL(override string) string {
	if trimmed := strings.TrimRight(strings.TrimSpace(override), "/"); trimmed != "" {
		return trimmed
	}
	return p.baseURL
}

func (p *APIYIProvider) resolveAPIKey(override string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed
	}
	return p.apiKey
}

func (p *APIYIProvider) mediaToGeminiPart(ctx context.Context, media MediaInput) (map[string]any, error) {
	data, mimeType, _, err := p.resolveMediaInput(ctx, media)
	if err != nil {
		return nil, err
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	return map[string]any{
		"inlineData": map[string]any{
			"mimeType": mimeType,
			"data":     base64.StdEncoding.EncodeToString(data),
		},
	}, nil
}

func (p *APIYIProvider) resolveMediaInput(ctx context.Context, media MediaInput) ([]byte, string, string, error) {
	if len(media.Data) > 0 {
		mimeType := strings.TrimSpace(media.MIMEType)
		if mimeType == "" {
			mimeType = mime.TypeByExtension(filepath.Ext(media.FileName))
		}
		fileName := strings.TrimSpace(media.FileName)
		if fileName == "" {
			fileName = defaultMediaFileName(mimeType, media.Role)
		}
		return media.Data, mimeType, fileName, nil
	}
	if strings.TrimSpace(media.Base64) != "" {
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(media.Base64))
		if err != nil {
			return nil, "", "", err
		}
		fileName := strings.TrimSpace(media.FileName)
		if fileName == "" {
			fileName = defaultMediaFileName(media.MIMEType, media.Role)
		}
		return data, strings.TrimSpace(media.MIMEType), fileName, nil
	}
	if strings.TrimSpace(media.URL) != "" {
		artifact, err := p.downloadBinary(ctx, media.URL, strings.TrimSpace(media.FileName), strings.TrimSpace(media.MIMEType))
		if err != nil {
			return nil, "", "", err
		}
		return artifact.Data, artifact.MIMEType, artifact.FileName, nil
	}
	return nil, "", "", fmt.Errorf("media input must contain url or data")
}

func (p *APIYIProvider) downloadBinary(ctx context.Context, rawURL string, fallbackName string, fallbackMime string) (*BinaryArtifact, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := ensureHTTPStatus(resp, body); err != nil {
		return nil, err
	}
	fileName := fileNameFromResponse(resp.Header.Get("Content-Disposition"), fallbackName)
	if fileName == "" {
		if parsed, err := url.Parse(rawURL); err == nil {
			fileName = filepath.Base(parsed.Path)
		}
	}
	if fileName == "" {
		fileName = "artifact" + extensionForMIME(fallbackMime, ".bin")
	}
	mimeType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = fallbackMime
	}
	return &BinaryArtifact{
		FileName:  fileName,
		MIMEType:  mimeType,
		Data:      body,
		SizeBytes: int64(len(body)),
	}, nil
}

func ensureHTTPStatus(resp *http.Response, body []byte) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var payload struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if payload.Error != nil {
			return fmt.Errorf("provider request failed: %s (%s)", firstNonEmptyString(payload.Error.Message, payload.Message), payload.Error.Code)
		}
		if strings.TrimSpace(payload.Message) != "" {
			return fmt.Errorf("provider request failed: %s", payload.Message)
		}
	}
	return fmt.Errorf("provider request failed with status %d", resp.StatusCode)
}

func extractText(content any) string {
	switch typed := content.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if obj, ok := item.(map[string]any); ok {
				if text := strings.TrimSpace(stringValue(obj["text"])); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return strings.TrimSpace(stringValue(content))
	}
}

func normalizeRemoteVideoStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "completed", "succeeded", "success", "ready":
		return "completed"
	case "failed", "error", "cancelled", "canceled":
		return "failed"
	case "processing", "running", "in_progress":
		return "running"
	case "queued", "pending", "created":
		return "queued"
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}

func fileNameFromResponse(contentDisposition string, fallback string) string {
	contentDisposition = strings.TrimSpace(contentDisposition)
	if contentDisposition == "" {
		return fallback
	}
	for _, part := range strings.Split(contentDisposition, ";") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(strings.ToLower(part), "filename=") {
			continue
		}
		return strings.Trim(strings.TrimSpace(strings.TrimPrefix(part, "filename=")), "\"")
	}
	return fallback
}

func extensionForMIME(mimeType string, fallback string) string {
	if mimeType == "" {
		return fallback
	}
	if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
		return exts[0]
	}
	return fallback
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
