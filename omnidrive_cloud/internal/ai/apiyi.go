package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net"
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
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: newAPIYIHTTPClient(),
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
		"generationConfig": buildGeminiImageGenerationConfig(req),
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
					InlineDataAlt *struct {
						MIMEType string `json:"mime_type"`
						Data     string `json:"data"`
					} `json:"inline_data"`
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
			inlineData := part.InlineData
			if inlineData == nil && part.InlineDataAlt != nil {
				inlineData = &struct {
					MIMEType string `json:"mimeType"`
					Data     string `json:"data"`
				}{
					MIMEType: part.InlineDataAlt.MIMEType,
					Data:     part.InlineDataAlt.Data,
				}
			}
			if inlineData == nil || strings.TrimSpace(inlineData.Data) == "" {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(inlineData.Data))
			if err != nil {
				return nil, err
			}
			mimeType := strings.TrimSpace(inlineData.MIMEType)
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
	requestBody, contentType, err := p.buildVideoSubmissionBody(ctx, req)
	if err != nil {
		return nil, err
	}

	endpointURL := p.resolveEndpointURL(req.BaseURL, "/v1/videos")
	authHeader := p.resolveVideoAuthorization(req.Model, req.APIKey)

	httpReq, err := p.newRetryableRequest(ctx, http.MethodPost, endpointURL, requestBody, func(r *http.Request) {
		r.Header.Set("Authorization", authHeader)
		r.Header.Set("Content-Type", contentType)
		r.Header.Set("Accept", "application/json")
	})
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(httpReq)
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
		ID        string `json:"id"`
		Model     string `json:"model"`
		Status    string `json:"status"`
		Created   int64  `json:"created"`
		CreatedAt int64  `json:"created_at"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}
	var createdAt *time.Time
	createdUnix := firstNonZeroInt64(response.CreatedAt, response.Created)
	if createdUnix > 0 {
		parsed := time.Unix(createdUnix, 0).UTC()
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

func (p *APIYIProvider) GetVideo(ctx context.Context, videoID string, model string, baseURL string, apiKey string) (*VideoStatus, error) {
	body, err := p.doVideoRequest(ctx, model, baseURL, apiKey, http.MethodGet, fmt.Sprintf("/v1/videos/%s", url.PathEscape(videoID)), nil, "")
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
		VideoURL  string         `json:"video_url"`
		Output    map[string]any `json:"output"`
		Updated   int64          `json:"updated"`
		UpdatedAt int64          `json:"updated_at"`
		Created   int64          `json:"created"`
		CreatedAt int64          `json:"created_at"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	status := &VideoStatus{
		ID:          strings.TrimSpace(response.ID),
		Model:       strings.TrimSpace(response.Model),
		Status:      normalizeRemoteVideoStatus(response.Status),
		ContentURL:  firstNonEmptyString(response.Content, response.OutputURL, response.VideoURL, stringValue(response.Output["url"]), stringValue(response.Output["content_url"]), stringValue(response.Output["video_url"])),
		RawResponse: body,
	}
	if response.Error != nil {
		status.FailureCode = strings.TrimSpace(response.Error.Code)
		status.Message = strings.TrimSpace(response.Error.Message)
	}
	createdUnix := firstNonZeroInt64(response.CreatedAt, response.Created)
	if createdUnix > 0 {
		parsed := time.Unix(createdUnix, 0).UTC()
		status.CreatedAt = &parsed
	}
	updatedUnix := firstNonZeroInt64(response.UpdatedAt, response.Updated)
	if updatedUnix > 0 {
		parsed := time.Unix(updatedUnix, 0).UTC()
		status.UpdatedAt = &parsed
	}
	return status, nil
}

func (p *APIYIProvider) DownloadVideo(ctx context.Context, videoID string, model string, baseURL string, apiKey string) (*BinaryArtifact, error) {
	endpointURL := p.resolveEndpointURL(baseURL, fmt.Sprintf("/v1/videos/%s/content", url.PathEscape(videoID)))
	httpReq, err := p.newRetryableRequest(ctx, http.MethodGet, endpointURL, nil, func(r *http.Request) {
		r.Header.Set("Authorization", p.resolveVideoAuthorization(model, apiKey))
		r.Header.Set("Accept", "application/json")
	})
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(httpReq)
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
		contentURL := firstNonEmptyString(stringValue(payload["url"]), stringValue(payload["content_url"]), stringValue(payload["video_url"]))
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

func (p *APIYIProvider) buildVideoSubmissionBody(ctx context.Context, req VideoRequest) ([]byte, string, error) {
	if isVeoVideoModel(req.Model) && len(req.ReferenceImages) == 0 {
		payload := map[string]any{
			"model":  req.Model,
			"prompt": req.Prompt,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, "", err
		}
		return data, "application/json", nil
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("model", req.Model)
	_ = writer.WriteField("prompt", req.Prompt)
	if isSoraVideoModel(req.Model) {
		if seconds := normalizeSoraVideoSeconds(req.DurationSeconds); seconds != "" {
			_ = writer.WriteField("seconds", seconds)
		}
		if size := normalizeSoraVideoSize(req.Resolution, req.AspectRatio); size != "" {
			_ = writer.WriteField("size", size)
		}
	}

	for _, media := range req.ReferenceImages {
		data, mimeType, fileName, err := p.resolveMediaInput(ctx, media)
		if err != nil {
			return nil, "", err
		}
		part, err := writer.CreateFormFile("input_reference", fileName)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(data); err != nil {
			return nil, "", err
		}
		if mimeType != "" && isSoraVideoModel(req.Model) {
			_ = writer.WriteField("input_reference_mime_type", mimeType)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func (p *APIYIProvider) doJSON(ctx context.Context, baseURL string, apiKey string, method string, path string, data []byte, bearer bool) ([]byte, error) {
	req, err := p.newRetryableRequest(ctx, method, p.resolveEndpointURL(baseURL, path), data, func(r *http.Request) {
		resolvedAPIKey := p.resolveAPIKey(apiKey)
		if bearer {
			r.Header.Set("Authorization", "Bearer "+resolvedAPIKey)
		} else {
			r.Header.Set("Authorization", resolvedAPIKey)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json")
	})
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(req)
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

func (p *APIYIProvider) doVideoRequest(ctx context.Context, model string, baseURL string, apiKey string, method string, path string, body []byte, contentType string) ([]byte, error) {
	req, err := p.newRetryableRequest(ctx, method, p.resolveEndpointURL(baseURL, path), body, func(r *http.Request) {
		r.Header.Set("Authorization", p.resolveVideoAuthorization(model, apiKey))
		if strings.TrimSpace(contentType) != "" {
			r.Header.Set("Content-Type", contentType)
		}
		r.Header.Set("Accept", "application/json")
	})
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(req)
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

func (p *APIYIProvider) resolveEndpointURL(override string, endpointPath string) string {
	baseURL := p.resolveBaseURL(override)
	endpointPath = "/" + strings.Trim(strings.TrimSpace(endpointPath), "/")
	if endpointPath == "/" {
		return baseURL
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + endpointPath
	}

	basePath := sanitizeEndpointPath(parsedURL.Path)
	switch {
	case basePath == "":
		parsedURL.Path = endpointPath
	case basePath == endpointPath, strings.HasSuffix(basePath, endpointPath):
		parsedURL.Path = basePath
	case strings.HasPrefix(endpointPath, basePath+"/"), endpointPath == basePath:
		parsedURL.Path = endpointPath
	default:
		parsedURL.Path = sanitizeEndpointPath(basePath + "/" + strings.TrimPrefix(endpointPath, "/"))
	}
	parsedURL.RawPath = ""
	return strings.TrimRight(parsedURL.String(), "/")
}

func (p *APIYIProvider) resolveAPIKey(override string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed
	}
	return p.apiKey
}

func (p *APIYIProvider) resolveVideoAuthorization(model string, override string) string {
	apiKey := p.resolveAPIKey(override)
	if isSoraVideoModel(model) {
		return "Bearer " + apiKey
	}
	return apiKey
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
		"inline_data": map[string]any{
			"mime_type": mimeType,
			"data":      base64.StdEncoding.EncodeToString(data),
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
	req, err := p.newRetryableRequest(ctx, http.MethodGet, rawURL, nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.doRequest(req)
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

func newAPIYIHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ForceAttemptHTTP2 = true
	transport.MaxIdleConns = 64
	transport.MaxIdleConnsPerHost = 16
	transport.MaxConnsPerHost = 32
	transport.IdleConnTimeout = 90 * time.Second

	return &http.Client{
		Timeout:   3 * time.Minute,
		Transport: transport,
	}
}

func buildGeminiImageGenerationConfig(req ImageRequest) map[string]any {
	config := map[string]any{
		"responseModalities": []string{"IMAGE"},
	}

	imageConfig := map[string]any{}
	if aspectRatio := normalizeAspectRatio(req.AspectRatio); aspectRatio != "" {
		imageConfig["aspectRatio"] = aspectRatio
	}
	if imageSize := normalizeGeminiImageSize(req.Resolution); imageSize != "" {
		imageConfig["imageSize"] = imageSize
	}
	if len(imageConfig) > 0 {
		config["imageConfig"] = imageConfig
	}
	return config
}

func normalizeGeminiImageSize(value string) string {
	trimmed := strings.TrimSpace(strings.ToUpper(value))
	switch trimmed {
	case "1K", "2K", "4K":
		return trimmed
	}

	width, height, ok := parseResolutionDimensions(trimmed)
	if !ok {
		return ""
	}
	maxEdge := width
	if height > maxEdge {
		maxEdge = height
	}
	switch {
	case maxEdge <= 1536:
		return "1K"
	case maxEdge <= 3072:
		return "2K"
	default:
		return "4K"
	}
}

func normalizeSoraVideoSeconds(durationSeconds *int) string {
	if durationSeconds == nil || *durationSeconds <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", *durationSeconds)
}

func normalizeSoraVideoSize(resolution string, aspectRatio string) string {
	if width, height, ok := parseResolutionDimensions(resolution); ok {
		return fmt.Sprintf("%dx%d", width, height)
	}
	switch normalizeAspectRatio(aspectRatio) {
	case "16:9":
		return "1280x720"
	case "9:16":
		return "720x1280"
	default:
		return ""
	}
}

func parseResolutionDimensions(value string) (int, int, bool) {
	var width, height int
	if _, err := fmt.Sscanf(strings.TrimSpace(strings.ToLower(value)), "%dx%d", &width, &height); err != nil {
		return 0, 0, false
	}
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func isSoraVideoModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "sora-")
}

func isVeoVideoModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "veo-")
}

func sanitizeEndpointPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return ""
	}
	return "/" + strings.Trim(strings.ReplaceAll(value, "//", "/"), "/")
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (p *APIYIProvider) newRetryableRequest(ctx context.Context, method string, targetURL string, body []byte, applyHeaders func(*http.Request)) (*http.Request, error) {
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, reader)
	if err != nil {
		return nil, err
	}
	if applyHeaders != nil {
		applyHeaders(req)
	}
	return req, nil
}

func (p *APIYIProvider) doRequest(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	bodySnapshot := []byte(nil)
	if req.Body != nil && req.GetBody != nil {
		bodyReader, err := req.GetBody()
		if err == nil {
			bodySnapshot, _ = io.ReadAll(bodyReader)
			_ = bodyReader.Close()
		}
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		currentReq := req
		if attempt > 0 {
			cloned, cloneErr := cloneHTTPRequest(req, bodySnapshot)
			if cloneErr != nil {
				return nil, cloneErr
			}
			currentReq = cloned
			time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
		}

		resp, err := p.httpClient.Do(currentReq)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryableProviderError(err) {
			break
		}
	}
	return nil, lastErr
}

func cloneHTTPRequest(req *http.Request, body []byte) (*http.Request, error) {
	cloned := req.Clone(req.Context())
	if len(body) > 0 {
		cloned.Body = io.NopCloser(bytes.NewReader(body))
		cloned.ContentLength = int64(len(body))
		cloned.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
		return cloned, nil
	}
	cloned.Body = nil
	cloned.ContentLength = 0
	cloned.GetBody = nil
	return cloned, nil
}

func isRetryableProviderError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, fs.ErrClosed) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "unexpected eof"),
		strings.Contains(message, "connection reset by peer"),
		strings.Contains(message, "broken pipe"),
		strings.Contains(message, "use of closed network connection"),
		strings.Contains(message, "clientconn is closed"),
		strings.Contains(message, "stream error"),
		strings.Contains(message, "server sent goaway"),
		strings.Contains(message, "timeout awaiting response headers"):
		return true
	default:
		return false
	}
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
