package ai

import (
	"context"
	"time"
)

type Provider interface {
	GenerateChat(ctx context.Context, req ChatRequest) (*ChatResult, error)
	GenerateImage(ctx context.Context, req ImageRequest) (*ImageResult, error)
	SubmitVideo(ctx context.Context, req VideoRequest) (*VideoSubmission, error)
	GetVideo(ctx context.Context, videoID string) (*VideoStatus, error)
	DownloadVideo(ctx context.Context, videoID string) (*BinaryArtifact, error)
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"maxTokens,omitempty"`
}

type ChatResult struct {
	Text         string         `json:"text"`
	Role         string         `json:"role"`
	Usage        map[string]any `json:"usage,omitempty"`
	FinishReason string         `json:"finishReason,omitempty"`
	RawResponse  []byte         `json:"rawResponse,omitempty"`
}

type MediaInput struct {
	URL      string `json:"url,omitempty"`
	Base64   string `json:"base64,omitempty"`
	Data     []byte `json:"-"`
	MIMEType string `json:"mimeType,omitempty"`
	FileName string `json:"fileName,omitempty"`
	Role     string `json:"role,omitempty"`
}

type ImageRequest struct {
	Model           string       `json:"model"`
	Prompt          string       `json:"prompt"`
	ReferenceImages []MediaInput `json:"referenceImages,omitempty"`
	AspectRatio     string       `json:"aspectRatio,omitempty"`
	Resolution      string       `json:"resolution,omitempty"`
}

type BinaryArtifact struct {
	FileName    string         `json:"fileName"`
	MIMEType    string         `json:"mimeType"`
	Data        []byte         `json:"-"`
	Text        string         `json:"text,omitempty"`
	SizeBytes   int64          `json:"sizeBytes,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	ArtifactKey string         `json:"artifactKey,omitempty"`
}

type ImageResult struct {
	Images      []BinaryArtifact `json:"images"`
	Text        string           `json:"text,omitempty"`
	RawResponse []byte           `json:"rawResponse,omitempty"`
}

type VideoRequest struct {
	Model           string       `json:"model"`
	Prompt          string       `json:"prompt"`
	ReferenceImages []MediaInput `json:"referenceImages,omitempty"`
	AspectRatio     string       `json:"aspectRatio,omitempty"`
	Resolution      string       `json:"resolution,omitempty"`
	DurationSeconds *int         `json:"durationSeconds,omitempty"`
}

type VideoSubmission struct {
	ID          string         `json:"id"`
	Model       string         `json:"model"`
	Status      string         `json:"status"`
	CreatedAt   *time.Time     `json:"createdAt,omitempty"`
	RawResponse []byte         `json:"rawResponse,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type VideoStatus struct {
	ID          string         `json:"id"`
	Model       string         `json:"model"`
	Status      string         `json:"status"`
	FailureCode string         `json:"failureCode,omitempty"`
	Message     string         `json:"message,omitempty"`
	ContentURL  string         `json:"contentUrl,omitempty"`
	CreatedAt   *time.Time     `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time     `json:"updatedAt,omitempty"`
	RawResponse []byte         `json:"rawResponse,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
