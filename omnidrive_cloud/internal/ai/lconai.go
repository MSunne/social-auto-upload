package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"omnidrive_cloud/internal/config"
)

type LConAIProvider struct {
	*APIYIProvider
}

func NewLConAIProvider(cfg config.Config) (*LConAIProvider, error) {
	baseURL := normalizeLConBaseURL(cfg.APIYIBaseURL)
	if baseURL == "" {
		baseURL = "https://n.lconai.com"
	}

	return &LConAIProvider{
		APIYIProvider: &APIYIProvider{
			baseURL:    baseURL,
			apiKey:     strings.TrimSpace(cfg.APIYIApiKey),
			httpClient: newAPIYIHTTPClient(),
		},
	}, nil
}

func (p *LConAIProvider) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResult, error) {
	return nil, fmt.Errorf("lconai provider does not support chat generation")
}

func (p *LConAIProvider) GenerateChatStream(ctx context.Context, req ChatRequest, onChunk func(ChatStreamChunk) error) (*ChatResult, error) {
	return nil, fmt.Errorf("lconai provider does not support streaming chat generation")
}

func (p *LConAIProvider) GenerateImage(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	return nil, fmt.Errorf("lconai provider does not support image generation")
}

func (p *LConAIProvider) GenerateStoryboardPackage(ctx context.Context, req StoryboardPackageRequest) (*StoryboardPackageResult, error) {
	return nil, fmt.Errorf("lconai provider does not support storyboard generation")
}

func (p *LConAIProvider) SubmitVideo(ctx context.Context, req VideoRequest) (*VideoSubmission, error) {
	requestBody, contentType, err := p.buildSeedanceVideoSubmissionBody(ctx, req)
	if err != nil {
		return nil, err
	}
	responseBody, err := p.doLConVideoRequest(ctx, req.BaseURL, req.APIKey, http.MethodPost, "/v1/videos", requestBody, contentType)
	if err != nil {
		return nil, err
	}

	var response struct {
		ID        string `json:"id"`
		TaskID    string `json:"task_id"`
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

	metadata := map[string]any{}
	if strings.TrimSpace(response.ID) != "" {
		metadata["remoteId"] = strings.TrimSpace(response.ID)
	}
	if strings.TrimSpace(response.TaskID) != "" {
		metadata["taskId"] = strings.TrimSpace(response.TaskID)
	}

	return &VideoSubmission{
		ID:          firstNonEmptyString(response.TaskID, response.ID),
		Model:       strings.TrimSpace(response.Model),
		Status:      normalizeRemoteVideoStatus(response.Status),
		CreatedAt:   createdAt,
		RawResponse: responseBody,
		Metadata:    metadata,
	}, nil
}

func (p *LConAIProvider) GetVideo(ctx context.Context, videoID string, model string, baseURL string, apiKey string) (*VideoStatus, error) {
	body, err := p.doLConVideoRequest(ctx, baseURL, apiKey, http.MethodGet, fmt.Sprintf("/v1/videos/%s", url.PathEscape(videoID)), nil, "")
	if err != nil {
		return nil, err
	}

	var response struct {
		ID              string `json:"id"`
		Model           string `json:"model"`
		Status          string `json:"status"`
		Progress        any    `json:"progress"`
		ProgressPercent any    `json:"progress_percent"`
		Percent         any    `json:"percent"`
		Percentage      any    `json:"percentage"`
		ContentURL      string `json:"content_url"`
		VideoURL        string `json:"video_url"`
		Created         int64  `json:"created"`
		CreatedAt       int64  `json:"created_at"`
		Updated         int64  `json:"updated"`
		UpdatedAt       int64  `json:"updated_at"`
		Error           *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	status := &VideoStatus{
		ID:          strings.TrimSpace(response.ID),
		Model:       strings.TrimSpace(response.Model),
		Status:      normalizeRemoteVideoStatus(response.Status),
		ContentURL:  firstNonEmptyString(response.VideoURL, response.ContentURL),
		RawResponse: body,
	}
	status.ProgressPercent = firstNonNilInt(
		normalizePercentValue(response.ProgressPercent),
		normalizePercentValue(response.Progress),
		normalizePercentValue(response.Percent),
		normalizePercentValue(response.Percentage),
	)
	if response.Error != nil {
		status.FailureCode = strings.TrimSpace(response.Error.Code)
		status.Message = strings.TrimSpace(response.Error.Message)
	}
	if createdUnix := firstNonZeroInt64(response.CreatedAt, response.Created); createdUnix > 0 {
		parsed := time.Unix(createdUnix, 0).UTC()
		status.CreatedAt = &parsed
	}
	if updatedUnix := firstNonZeroInt64(response.UpdatedAt, response.Updated); updatedUnix > 0 {
		parsed := time.Unix(updatedUnix, 0).UTC()
		status.UpdatedAt = &parsed
	}
	return status, nil
}

func (p *LConAIProvider) DownloadVideo(ctx context.Context, videoID string, model string, baseURL string, apiKey string, contentURL string) (*BinaryArtifact, error) {
	if directURL := strings.TrimSpace(contentURL); directURL != "" {
		return p.downloadBinary(ctx, directURL, fmt.Sprintf("%s.mp4", videoID), "video/mp4")
	}

	status, err := p.GetVideo(ctx, videoID, model, baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(status.ContentURL) == "" {
		return nil, fmt.Errorf("seedance video response did not contain a download URL")
	}
	return p.downloadBinary(ctx, status.ContentURL, fmt.Sprintf("%s.mp4", videoID), "video/mp4")
}

func (p *LConAIProvider) buildSeedanceVideoSubmissionBody(ctx context.Context, req VideoRequest) ([]byte, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("model", strings.TrimSpace(req.Model))
	_ = writer.WriteField("prompt", sanitizeSeedancePrompt(req.Prompt, req.AspectRatio, req.Resolution))
	if size := normalizeSeedanceVideoSize(req.Resolution, req.AspectRatio); size != "" {
		_ = writer.WriteField("size", size)
	}
	if seconds := normalizeSeedanceVideoSeconds(req.DurationSeconds); seconds != "" {
		_ = writer.WriteField("seconds", seconds)
	}

	for _, media := range filterVideoReferenceMedia(effectiveVideoReferenceMedia(req), "") {
		data, _, fileName, err := p.resolveMediaInput(ctx, media)
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
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func sanitizeSeedancePrompt(prompt string, aspectRatio string, resolution string) string {
	parts := strings.Split(strings.TrimSpace(prompt), "\n")
	result := make([]string, 0, len(parts))
	ratioLine := ""
	if ratio := normalizeAspectRatio(aspectRatio); ratio != "" {
		ratioLine = "Aspect ratio: " + ratio
	}
	resolutionLine := ""
	if normalized := normalizeResolution(resolution); normalized != "" {
		resolutionLine = "Resolution: " + normalized
	}
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if ratioLine != "" && trimmed == ratioLine {
			continue
		}
		if resolutionLine != "" && trimmed == resolutionLine {
			continue
		}
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}

func (p *LConAIProvider) doLConVideoRequest(ctx context.Context, baseURL string, apiKey string, method string, path string, body []byte, contentType string) ([]byte, error) {
	normalizedBaseURL := normalizeLConBaseURL(baseURL)
	if normalizedBaseURL == "" {
		normalizedBaseURL = normalizeLConBaseURL(p.baseURL)
	}
	req, err := p.newRetryableRequest(ctx, method, p.APIYIProvider.resolveEndpointURL(normalizedBaseURL, path), body, func(r *http.Request) {
		r.Header.Set("Authorization", p.resolveAPIKey(apiKey))
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

func normalizeSeedanceVideoSize(resolution string, aspectRatio string) string {
	if trimmed := strings.TrimSpace(resolution); trimmed != "" {
		return trimmed
	}
	switch strings.TrimSpace(aspectRatio) {
	case "16:9":
		return "1280x720"
	default:
		return "720x1280"
	}
}

func normalizeSeedanceVideoSeconds(value *int) string {
	if value == nil || *value <= 0 {
		return ""
	}
	return strconv.Itoa(*value)
}

func normalizeLConBaseURL(value string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(value), "/")
	if baseURL == "" {
		return ""
	}
	switch {
	case strings.HasSuffix(baseURL, "/v1/videos"):
		return strings.TrimSuffix(baseURL, "/v1/videos")
	case strings.HasSuffix(baseURL, "/v1/video"):
		return strings.TrimSuffix(baseURL, "/v1/video")
	default:
		return baseURL
	}
}
