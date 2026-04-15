package ai

import (
	"context"
	"strings"
	"time"
)

type Provider interface {
	GenerateChat(ctx context.Context, req ChatRequest) (*ChatResult, error)
	GenerateChatStream(ctx context.Context, req ChatRequest, onChunk func(ChatStreamChunk) error) (*ChatResult, error)
	GenerateImage(ctx context.Context, req ImageRequest) (*ImageResult, error)
	GenerateStoryboardPackage(ctx context.Context, req StoryboardPackageRequest) (*StoryboardPackageResult, error)
	SubmitVideo(ctx context.Context, req VideoRequest) (*VideoSubmission, error)
	GetVideo(ctx context.Context, videoID string, model string, baseURL string, apiKey string, explicitBaseURL bool) (*VideoStatus, error)
	DownloadVideo(ctx context.Context, videoID string, model string, baseURL string, apiKey string, contentURL string, explicitBaseURL bool) (*BinaryArtifact, error)
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type ChatRequest struct {
	Model        string        `json:"model"`
	BaseURL      string        `json:"baseUrl,omitempty"`
	APIKey       string        `json:"apiKey,omitempty"`
	ChatProtocol string        `json:"chatProtocol,omitempty"`
	Messages     []ChatMessage `json:"messages"`
	Temperature  *float64      `json:"temperature,omitempty"`
	MaxTokens    *int          `json:"maxTokens,omitempty"`
}

const (
	ChatProtocolAuto                  = "auto"
	ChatProtocolOpenAIChatCompletions = "openai_chat_completions"
	ChatProtocolAnthropicMessages     = "anthropic_messages"

	ChatCompletionStateComplete   = "complete"
	ChatCompletionStateIncomplete = "incomplete"

	ChatWarningMissingFinishReason   = "missing_finish_reason"
	ChatWarningMissingStopReason     = "missing_stop_reason"
	ChatWarningReasoningOnlyOutput   = "reasoning_only_output"
	ChatWarningStreamEOFAfterContent = "stream_eof_after_content"
)

type ChatStreamDiagnostics struct {
	SawTerminalMarker   bool   `json:"sawTerminalMarker,omitempty"`
	FinishReason        string `json:"finishReason,omitempty"`
	StopReason          string `json:"stopReason,omitempty"`
	ReasoningDeltaCount int    `json:"reasoningDeltaCount,omitempty"`
	AnswerDeltaCount    int    `json:"answerDeltaCount,omitempty"`
}

type ChatResult struct {
	Text              string                 `json:"text"`
	Role              string                 `json:"role"`
	Usage             map[string]any         `json:"usage,omitempty"`
	FinishReason      string                 `json:"finishReason,omitempty"`
	CompletionState   string                 `json:"completionState,omitempty"`
	WarningCodes      []string               `json:"warningCodes,omitempty"`
	WarningMessage    string                 `json:"warningMessage,omitempty"`
	ProtocolFamily    string                 `json:"protocolFamily,omitempty"`
	StreamDiagnostics *ChatStreamDiagnostics `json:"streamDiagnostics,omitempty"`
	RawResponse       []byte                 `json:"rawResponse,omitempty"`
}

type ChatStreamChunk struct {
	Delta             string                 `json:"delta,omitempty"`
	Text              string                 `json:"text,omitempty"`
	Role              string                 `json:"role,omitempty"`
	Usage             map[string]any         `json:"usage,omitempty"`
	FinishReason      string                 `json:"finishReason,omitempty"`
	CompletionState   string                 `json:"completionState,omitempty"`
	WarningCodes      []string               `json:"warningCodes,omitempty"`
	WarningMessage    string                 `json:"warningMessage,omitempty"`
	ProtocolFamily    string                 `json:"protocolFamily,omitempty"`
	StreamDiagnostics *ChatStreamDiagnostics `json:"streamDiagnostics,omitempty"`
	Progressed        bool                   `json:"progressed,omitempty"`
	Done              bool                   `json:"done,omitempty"`
}

func NormalizeChatProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", ChatProtocolAuto:
		return ChatProtocolAuto
	case ChatProtocolOpenAIChatCompletions:
		return ChatProtocolOpenAIChatCompletions
	case ChatProtocolAnthropicMessages:
		return ChatProtocolAnthropicMessages
	default:
		return ""
	}
}

func normalizeChatWarningCodes(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

type MediaInput struct {
	Kind     string `json:"kind,omitempty"`
	URL      string `json:"url,omitempty"`
	Base64   string `json:"base64,omitempty"`
	Data     []byte `json:"-"`
	MIMEType string `json:"mimeType,omitempty"`
	FileName string `json:"fileName,omitempty"`
	Role     string `json:"role,omitempty"`
}

type ImageRequest struct {
	Model           string       `json:"model"`
	BaseURL         string       `json:"baseUrl,omitempty"`
	APIKey          string       `json:"apiKey,omitempty"`
	ExplicitBaseURL bool         `json:"-"`
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

type StoryboardPackageRequest struct {
	Model           string       `json:"model"`
	BaseURL         string       `json:"baseUrl,omitempty"`
	APIKey          string       `json:"apiKey,omitempty"`
	ExplicitBaseURL bool         `json:"-"`
	SystemPrompt    string       `json:"systemPrompt,omitempty"`
	Prompt          string       `json:"prompt"`
	ReferenceImages []MediaInput `json:"referenceImages,omitempty"`
	AspectRatio     string       `json:"aspectRatio,omitempty"`
	Resolution      string       `json:"resolution,omitempty"`
}

type StoryboardPackageResult struct {
	Cover            BinaryArtifact `json:"cover"`
	GenerationPrompt string         `json:"generationPrompt"`
	PublishIntro     string         `json:"publishIntro,omitempty"`
	Text             string         `json:"text,omitempty"`
	RawResponse      []byte         `json:"rawResponse,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type VideoRequest struct {
	Vendor          string       `json:"vendor,omitempty"`
	Model           string       `json:"model"`
	BaseURL         string       `json:"baseUrl,omitempty"`
	APIKey          string       `json:"apiKey,omitempty"`
	ExplicitBaseURL bool         `json:"-"`
	Prompt          string       `json:"prompt"`
	ReferenceMedia  []MediaInput `json:"referenceMedia,omitempty"`
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
	ID              string         `json:"id"`
	Model           string         `json:"model"`
	Status          string         `json:"status"`
	ProgressPercent *int           `json:"progressPercent,omitempty"`
	FailureCode     string         `json:"failureCode,omitempty"`
	Message         string         `json:"message,omitempty"`
	ContentURL      string         `json:"contentUrl,omitempty"`
	CreatedAt       *time.Time     `json:"createdAt,omitempty"`
	UpdatedAt       *time.Time     `json:"updatedAt,omitempty"`
	RawResponse     []byte         `json:"rawResponse,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}
