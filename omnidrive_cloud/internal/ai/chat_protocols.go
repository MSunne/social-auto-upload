package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type sseFrame struct {
	Event string
	Data  string
}

func resolveChatProtocol(req ChatRequest) string {
	explicit := NormalizeChatProtocol(req.ChatProtocol)
	if explicit != "" && explicit != ChatProtocolAuto {
		return explicit
	}
	if isAnthropicMessagesBaseURL(req.BaseURL) {
		return ChatProtocolAnthropicMessages
	}
	normalizedModel := strings.ToLower(strings.TrimSpace(req.Model))
	for _, marker := range []string{"claude", "opus", "sonnet", "haiku"} {
		if strings.Contains(normalizedModel, marker) {
			return ChatProtocolAnthropicMessages
		}
	}
	return ChatProtocolOpenAIChatCompletions
}

func isAnthropicMessagesBaseURL(rawURL string) bool {
	return strings.HasSuffix(normalizeURLPathForMatch(rawURL), "/v1/messages")
}

func scanSSEStream(reader io.Reader, onFrame func(sseFrame) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	frame := sseFrame{Event: "message"}
	dataLines := make([]string, 0, 4)
	dispatch := func() error {
		if len(dataLines) == 0 {
			frame = sseFrame{Event: "message"}
			return nil
		}
		nextFrame := frame
		if strings.TrimSpace(nextFrame.Event) == "" {
			nextFrame.Event = "message"
		}
		nextFrame.Data = strings.Join(dataLines, "\n")
		frame = sseFrame{Event: "message"}
		dataLines = dataLines[:0]
		return onFrame(nextFrame)
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.TrimSpace(line) == "":
			if err := dispatch(); err != nil {
				return err
			}
		case strings.HasPrefix(line, ":"):
			continue
		case strings.HasPrefix(line, "event:"):
			frame.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimLeft(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return dispatch()
}

func appendSSEFrameRaw(buffer *bytes.Buffer, frame sseFrame) {
	if buffer == nil {
		return
	}
	event := strings.TrimSpace(frame.Event)
	if event != "" && event != "message" {
		buffer.WriteString("event: ")
		buffer.WriteString(event)
		buffer.WriteByte('\n')
	}
	for _, line := range strings.Split(frame.Data, "\n") {
		buffer.WriteString("data: ")
		buffer.WriteString(line)
		buffer.WriteByte('\n')
	}
	buffer.WriteByte('\n')
}

func applyChatCompletionMetadata(result *ChatResult, completionState string, warningCodes ...string) {
	if result == nil {
		return
	}
	result.CompletionState = strings.TrimSpace(completionState)
	result.WarningCodes = normalizeChatWarningCodes(warningCodes)
	result.WarningMessage = buildChatWarningMessage(result.CompletionState, result.WarningCodes)
}

func buildChatWarningMessage(completionState string, warningCodes []string) string {
	if completionState != ChatCompletionStateIncomplete {
		return ""
	}
	for _, code := range warningCodes {
		if code == ChatWarningReasoningOnlyOutput {
			return "仅收到思考过程，未收到最终答复"
		}
	}
	return "本次回复已结束，但模型未返回完整终止信号，内容可能不完整"
}

func hasChatWarningCode(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(target) {
			return true
		}
	}
	return false
}

func appendThinkingDelta(fullText *strings.Builder, combinedDelta *strings.Builder, inReasoning *bool, value string) {
	if value == "" {
		return
	}
	if !*inReasoning {
		fullText.WriteString("<think>\n")
		combinedDelta.WriteString("<think>\n")
		*inReasoning = true
	}
	fullText.WriteString(value)
	combinedDelta.WriteString(value)
}

func appendAnswerDelta(fullText *strings.Builder, combinedDelta *strings.Builder, inReasoning *bool, value string) {
	if value == "" {
		return
	}
	if *inReasoning {
		fullText.WriteString("\n</think>\n")
		combinedDelta.WriteString("\n</think>\n")
		*inReasoning = false
	}
	fullText.WriteString(value)
	combinedDelta.WriteString(value)
}

func closeThinkingBlock(fullText *strings.Builder, inReasoning *bool) {
	if !*inReasoning {
		return
	}
	fullText.WriteString("\n</think>\n")
	*inReasoning = false
}

func closeThinkingBlockWithDelta(fullText *strings.Builder, combinedDelta *strings.Builder, inReasoning *bool) {
	if !*inReasoning {
		return
	}
	fullText.WriteString("\n</think>\n")
	combinedDelta.WriteString("\n</think>\n")
	*inReasoning = false
}

func (p *APIYIProvider) generateOpenAIChat(ctx context.Context, req ChatRequest) (*ChatResult, error) {
	req, err := p.normalizeChatRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	payload := buildChatPayload(req, false)

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
				Role             string `json:"role"`
				Content          any    `json:"content"`
				ReasoningContent any    `json:"reasoning_content"`
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
	text := extractText(choice.Message.Content)
	reasoning := extractText(choice.Message.ReasoningContent)

	finalText := text
	if reasoning != "" {
		finalText = "<think>\n" + reasoning + "\n</think>\n" + text
	}

	result := &ChatResult{
		Text:           finalText,
		Role:           strings.TrimSpace(choice.Message.Role),
		Usage:          response.Usage,
		FinishReason:   strings.TrimSpace(choice.FinishReason),
		ProtocolFamily: ChatProtocolOpenAIChatCompletions,
		StreamDiagnostics: &ChatStreamDiagnostics{
			FinishReason:        strings.TrimSpace(choice.FinishReason),
			ReasoningDeltaCount: boolToCount(reasoning != ""),
			AnswerDeltaCount:    boolToCount(text != ""),
		},
		RawResponse: body,
	}

	warningCodes := make([]string, 0, 2)
	if reasoning != "" && strings.TrimSpace(text) == "" {
		warningCodes = append(warningCodes, ChatWarningReasoningOnlyOutput)
	}
	if result.FinishReason != "" {
		applyChatCompletionMetadata(result, ChatCompletionStateComplete)
	} else {
		warningCodes = append(warningCodes, ChatWarningMissingFinishReason)
		applyChatCompletionMetadata(result, ChatCompletionStateIncomplete, warningCodes...)
	}
	return result, nil
}

func (p *APIYIProvider) generateOpenAIChatStream(ctx context.Context, req ChatRequest, onChunk func(ChatStreamChunk) error) (*ChatResult, error) {
	req, err := p.normalizeChatRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	payload := buildChatPayload(req, true)

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := p.newRetryableRequest(ctx, http.MethodPost, p.resolveEndpointURL(req.BaseURL, "/v1/chat/completions"), data, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+p.resolveAPIKey(req.APIKey))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "text/event-stream")
		r.Header.Set("Cache-Control", "no-cache")
	})
	if err != nil {
		return nil, err
	}

	resp, err := p.doStreamingRequest(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		if statusErr := ensureHTTPStatus(resp, body); statusErr != nil {
			return nil, statusErr
		}
		return nil, fmt.Errorf("provider request failed with status %d", resp.StatusCode)
	}

	result := &ChatResult{
		Role:              "assistant",
		ProtocolFamily:    ChatProtocolOpenAIChatCompletions,
		StreamDiagnostics: &ChatStreamDiagnostics{},
	}
	var fullText strings.Builder
	var rawResponse bytes.Buffer
	sawContent := false
	inReasoning := false

	if err := scanSSEStream(resp.Body, func(frame sseFrame) error {
		appendSSEFrameRaw(&rawResponse, frame)
		if strings.TrimSpace(frame.Data) == "[DONE]" {
			result.StreamDiagnostics.SawTerminalMarker = true
			return nil
		}

		var response struct {
			Choices []struct {
				Delta struct {
					Role             string `json:"role"`
					Content          any    `json:"content"`
					ReasoningContent any    `json:"reasoning_content"`
				} `json:"delta"`
				Message struct {
					Role             string `json:"role"`
					Content          any    `json:"content"`
					ReasoningContent any    `json:"reasoning_content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage map[string]any `json:"usage"`
		}
		if err := json.Unmarshal([]byte(frame.Data), &response); err != nil {
			return err
		}
		if response.Usage != nil {
			result.Usage = response.Usage
		}
		if len(response.Choices) == 0 {
			return nil
		}

		choice := response.Choices[0]
		if role := strings.TrimSpace(firstNonEmptyString(choice.Delta.Role, choice.Message.Role)); role != "" {
			result.Role = role
		}
		if finishReason := strings.TrimSpace(choice.FinishReason); finishReason != "" {
			result.FinishReason = finishReason
			result.StreamDiagnostics.FinishReason = finishReason
		}

		deltaText := extractDeltaText(choice.Delta.Content)
		if deltaText == "" {
			deltaText = extractDeltaText(choice.Message.Content)
		}
		reasoningText := extractDeltaText(choice.Delta.ReasoningContent)
		if reasoningText == "" {
			reasoningText = extractDeltaText(choice.Message.ReasoningContent)
		}

		var combinedDelta strings.Builder
		if reasoningText != "" {
			result.StreamDiagnostics.ReasoningDeltaCount++
			appendThinkingDelta(&fullText, &combinedDelta, &inReasoning, reasoningText)
		}
		if deltaText != "" {
			result.StreamDiagnostics.AnswerDeltaCount++
			appendAnswerDelta(&fullText, &combinedDelta, &inReasoning, deltaText)
		}

		if combinedDelta.Len() == 0 {
			return nil
		}
		sawContent = true

		if onChunk != nil {
			return onChunk(ChatStreamChunk{
				Delta:          combinedDelta.String(),
				Text:           fullText.String(),
				Role:           result.Role,
				Usage:          result.Usage,
				FinishReason:   result.FinishReason,
				ProtocolFamily: result.ProtocolFamily,
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	closeThinkingBlock(&fullText, &inReasoning)
	result.Text = fullText.String()
	result.RawResponse = rawResponse.Bytes()

	warningCodes := make([]string, 0, 3)
	if result.StreamDiagnostics.AnswerDeltaCount == 0 && result.StreamDiagnostics.ReasoningDeltaCount > 0 {
		warningCodes = append(warningCodes, ChatWarningReasoningOnlyOutput)
	}
	switch {
	case strings.TrimSpace(result.FinishReason) != "":
		applyChatCompletionMetadata(result, ChatCompletionStateComplete)
	case result.StreamDiagnostics.SawTerminalMarker:
		warningCodes = append(warningCodes, ChatWarningMissingFinishReason)
		applyChatCompletionMetadata(result, ChatCompletionStateIncomplete, warningCodes...)
	case sawContent:
		warningCodes = append(warningCodes, ChatWarningStreamEOFAfterContent)
		applyChatCompletionMetadata(result, ChatCompletionStateIncomplete, warningCodes...)
	default:
		return nil, fmt.Errorf("provider stream ended before terminal event")
	}

	if onChunk != nil {
		if err := onChunk(ChatStreamChunk{
			Text:              result.Text,
			Role:              result.Role,
			Usage:             result.Usage,
			FinishReason:      result.FinishReason,
			CompletionState:   result.CompletionState,
			WarningCodes:      result.WarningCodes,
			WarningMessage:    result.WarningMessage,
			ProtocolFamily:    result.ProtocolFamily,
			StreamDiagnostics: result.StreamDiagnostics,
			Done:              true,
		}); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (p *APIYIProvider) generateAnthropicMessagesChat(ctx context.Context, req ChatRequest) (*ChatResult, error) {
	payload, err := p.buildAnthropicMessagesPayload(ctx, req, false)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	body, err := p.doAnthropicJSON(ctx, req.BaseURL, req.APIKey, data, false)
	if err != nil {
		return nil, err
	}

	var response struct {
		Role       string         `json:"role"`
		StopReason string         `json:"stop_reason"`
		Usage      map[string]any `json:"usage"`
		Content    []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	var fullText strings.Builder
	inReasoning := false
	diagnostics := &ChatStreamDiagnostics{
		StopReason: strings.TrimSpace(response.StopReason),
	}
	for _, block := range response.Content {
		switch strings.TrimSpace(block.Type) {
		case "thinking", "redacted_thinking":
			if strings.TrimSpace(block.Thinking) == "" {
				continue
			}
			diagnostics.ReasoningDeltaCount++
			var combined strings.Builder
			appendThinkingDelta(&fullText, &combined, &inReasoning, block.Thinking)
		case "text":
			if block.Text == "" {
				continue
			}
			diagnostics.AnswerDeltaCount++
			var combined strings.Builder
			appendAnswerDelta(&fullText, &combined, &inReasoning, block.Text)
		}
	}
	closeThinkingBlock(&fullText, &inReasoning)

	result := &ChatResult{
		Text:              fullText.String(),
		Role:              strings.TrimSpace(response.Role),
		Usage:             response.Usage,
		ProtocolFamily:    ChatProtocolAnthropicMessages,
		StreamDiagnostics: diagnostics,
		RawResponse:       body,
	}

	warningCodes := make([]string, 0, 2)
	if diagnostics.AnswerDeltaCount == 0 && diagnostics.ReasoningDeltaCount > 0 {
		warningCodes = append(warningCodes, ChatWarningReasoningOnlyOutput)
	}
	if diagnostics.StopReason != "" {
		applyChatCompletionMetadata(result, ChatCompletionStateComplete)
	} else {
		warningCodes = append(warningCodes, ChatWarningMissingStopReason)
		applyChatCompletionMetadata(result, ChatCompletionStateIncomplete, warningCodes...)
	}
	return result, nil
}

func (p *APIYIProvider) generateAnthropicMessagesChatStream(ctx context.Context, req ChatRequest, onChunk func(ChatStreamChunk) error) (*ChatResult, error) {
	payload, err := p.buildAnthropicMessagesPayload(ctx, req, true)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := p.newRetryableRequest(ctx, http.MethodPost, p.resolveEndpointURL(req.BaseURL, "/v1/messages"), data, func(r *http.Request) {
		apiKey := p.resolveAPIKey(req.APIKey)
		r.Header.Set("Authorization", "Bearer "+apiKey)
		r.Header.Set("x-api-key", apiKey)
		r.Header.Set("anthropic-version", "2023-06-01")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "text/event-stream")
		r.Header.Set("Cache-Control", "no-cache")
	})
	if err != nil {
		return nil, err
	}

	resp, err := p.doStreamingRequest(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		if statusErr := ensureHTTPStatus(resp, body); statusErr != nil {
			return nil, statusErr
		}
		return nil, fmt.Errorf("provider request failed with status %d", resp.StatusCode)
	}

	result := &ChatResult{
		Role:              "assistant",
		ProtocolFamily:    ChatProtocolAnthropicMessages,
		StreamDiagnostics: &ChatStreamDiagnostics{},
	}
	blockTypes := make(map[int]string, 4)
	var fullText strings.Builder
	var rawResponse bytes.Buffer
	inReasoning := false
	sawContent := false

	if err := scanSSEStream(resp.Body, func(frame sseFrame) error {
		appendSSEFrameRaw(&rawResponse, frame)
		if strings.TrimSpace(frame.Data) == "" {
			return nil
		}

		var envelope struct {
			Type    string         `json:"type"`
			Index   int            `json:"index"`
			Usage   map[string]any `json:"usage"`
			Message struct {
				Role       string         `json:"role"`
				StopReason string         `json:"stop_reason"`
				Usage      map[string]any `json:"usage"`
			} `json:"message"`
			ContentBlock struct {
				Type string `json:"type"`
			} `json:"content_block"`
			Delta struct {
				Type       string         `json:"type"`
				Text       string         `json:"text"`
				Thinking   string         `json:"thinking"`
				StopReason string         `json:"stop_reason"`
				Usage      map[string]any `json:"usage"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(frame.Data), &envelope); err != nil {
			return err
		}

		switch strings.TrimSpace(envelope.Type) {
		case "message_start":
			if role := strings.TrimSpace(envelope.Message.Role); role != "" {
				result.Role = role
			}
			if envelope.Message.Usage != nil {
				result.Usage = envelope.Message.Usage
			}
			if stopReason := strings.TrimSpace(envelope.Message.StopReason); stopReason != "" {
				result.StreamDiagnostics.StopReason = stopReason
			}
		case "content_block_start":
			blockTypes[envelope.Index] = strings.TrimSpace(envelope.ContentBlock.Type)
		case "content_block_delta":
			blockType := strings.TrimSpace(blockTypes[envelope.Index])
			if blockType == "" {
				blockType = strings.TrimSpace(envelope.Delta.Type)
			}
			var combinedDelta strings.Builder
			switch {
			case blockType == "thinking" || strings.TrimSpace(envelope.Delta.Type) == "thinking_delta":
				if envelope.Delta.Thinking == "" {
					return nil
				}
				result.StreamDiagnostics.ReasoningDeltaCount++
				appendThinkingDelta(&fullText, &combinedDelta, &inReasoning, envelope.Delta.Thinking)
			case blockType == "text" || strings.TrimSpace(envelope.Delta.Type) == "text_delta":
				if envelope.Delta.Text == "" {
					return nil
				}
				result.StreamDiagnostics.AnswerDeltaCount++
				appendAnswerDelta(&fullText, &combinedDelta, &inReasoning, envelope.Delta.Text)
			default:
				return nil
			}
			if combinedDelta.Len() == 0 {
				return nil
			}
			sawContent = true
			if onChunk != nil {
				return onChunk(ChatStreamChunk{
					Delta:          combinedDelta.String(),
					Text:           fullText.String(),
					Role:           result.Role,
					Usage:          result.Usage,
					ProtocolFamily: result.ProtocolFamily,
				})
			}
		case "message_delta":
			if envelope.Usage != nil {
				result.Usage = envelope.Usage
			}
			if envelope.Delta.Usage != nil {
				result.Usage = envelope.Delta.Usage
			}
			if stopReason := strings.TrimSpace(envelope.Delta.StopReason); stopReason != "" {
				result.StreamDiagnostics.StopReason = stopReason
			}
		case "message_stop":
			result.StreamDiagnostics.SawTerminalMarker = true
		}
		return nil
	}); err != nil {
		return nil, err
	}

	closeThinkingBlock(&fullText, &inReasoning)
	result.Text = fullText.String()
	result.RawResponse = rawResponse.Bytes()

	warningCodes := make([]string, 0, 3)
	if result.StreamDiagnostics.AnswerDeltaCount == 0 && result.StreamDiagnostics.ReasoningDeltaCount > 0 {
		warningCodes = append(warningCodes, ChatWarningReasoningOnlyOutput)
	}
	switch {
	case strings.TrimSpace(result.StreamDiagnostics.StopReason) != "":
		applyChatCompletionMetadata(result, ChatCompletionStateComplete)
	case result.StreamDiagnostics.SawTerminalMarker:
		warningCodes = append(warningCodes, ChatWarningMissingStopReason)
		applyChatCompletionMetadata(result, ChatCompletionStateIncomplete, warningCodes...)
	case sawContent:
		warningCodes = append(warningCodes, ChatWarningStreamEOFAfterContent)
		applyChatCompletionMetadata(result, ChatCompletionStateIncomplete, warningCodes...)
	default:
		return nil, fmt.Errorf("provider stream ended before terminal event")
	}

	if onChunk != nil {
		if err := onChunk(ChatStreamChunk{
			Text:              result.Text,
			Role:              result.Role,
			Usage:             result.Usage,
			CompletionState:   result.CompletionState,
			WarningCodes:      result.WarningCodes,
			WarningMessage:    result.WarningMessage,
			ProtocolFamily:    result.ProtocolFamily,
			StreamDiagnostics: result.StreamDiagnostics,
			Done:              true,
		}); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (p *APIYIProvider) doAnthropicJSON(ctx context.Context, baseURL string, apiKey string, data []byte, stream bool) ([]byte, error) {
	req, err := p.newRetryableRequest(ctx, http.MethodPost, p.resolveEndpointURL(baseURL, "/v1/messages"), data, func(r *http.Request) {
		resolvedAPIKey := p.resolveAPIKey(apiKey)
		r.Header.Set("Authorization", "Bearer "+resolvedAPIKey)
		r.Header.Set("x-api-key", resolvedAPIKey)
		r.Header.Set("anthropic-version", "2023-06-01")
		r.Header.Set("Content-Type", "application/json")
		if stream {
			r.Header.Set("Accept", "text/event-stream")
			r.Header.Set("Cache-Control", "no-cache")
			return
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := ensureHTTPStatus(resp, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (p *APIYIProvider) buildAnthropicMessagesPayload(ctx context.Context, req ChatRequest, stream bool) (map[string]any, error) {
	systemParts := make([]string, 0, 2)
	messages := make([]map[string]any, 0, len(req.Messages))

	for _, message := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role == "system" {
			if text := strings.TrimSpace(extractChatContentText(message.Content)); text != "" {
				systemParts = append(systemParts, text)
			}
			continue
		}

		blocks, err := p.anthropicBlocksFromChatContent(ctx, message.Content)
		if err != nil {
			return nil, err
		}
		if len(blocks) == 0 {
			if text := strings.TrimSpace(extractChatContentText(message.Content)); text != "" {
				blocks = append(blocks, map[string]any{
					"type": "text",
					"text": text,
				})
			}
		}
		if len(blocks) == 0 {
			continue
		}
		messages = append(messages, map[string]any{
			"role":    normalizeAnthropicMessageRole(role),
			"content": blocks,
		})
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("anthropic chat request requires at least one user or assistant message")
	}

	payload := map[string]any{
		"model":      strings.TrimSpace(req.Model),
		"messages":   messages,
		"max_tokens": resolveAnthropicMaxTokens(req.MaxTokens),
	}
	if stream {
		payload["stream"] = true
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if len(systemParts) > 0 {
		payload["system"] = strings.Join(systemParts, "\n\n")
	}
	return payload, nil
}

func (p *APIYIProvider) anthropicBlocksFromChatContent(ctx context.Context, content any) ([]map[string]any, error) {
	normalizedContent, err := p.normalizeChatMessageContent(ctx, content)
	if err != nil {
		return nil, err
	}

	switch typed := normalizedContent.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, nil
		}
		return []map[string]any{{
			"type": "text",
			"text": typed,
		}}, nil
	case map[string]any:
		return p.anthropicBlocksFromStructuredParts([]map[string]any{typed})
	case []map[string]any:
		return p.anthropicBlocksFromStructuredParts(typed)
	case []any:
		parts := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			parts = append(parts, part)
		}
		return p.anthropicBlocksFromStructuredParts(parts)
	default:
		if text := strings.TrimSpace(extractChatContentText(typed)); text != "" {
			return []map[string]any{{
				"type": "text",
				"text": text,
			}}, nil
		}
		return nil, nil
	}
}

func (p *APIYIProvider) anthropicBlocksFromStructuredParts(parts []map[string]any) ([]map[string]any, error) {
	blocks := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		partType := strings.TrimSpace(stringValueFromMap(part, "type"))
		switch partType {
		case "text", "input_text":
			text := stringValueFromMap(part, "text", "content", "value")
			if strings.TrimSpace(text) == "" {
				continue
			}
			blocks = append(blocks, map[string]any{
				"type": "text",
				"text": text,
			})
		case "image_url":
			imageURLPayload, _ := part["image_url"].(map[string]any)
			sourceURL := strings.TrimSpace(stringValueFromMap(imageURLPayload, "url"))
			if sourceURL == "" {
				sourceURL = strings.TrimSpace(stringValueFromMap(part, "url"))
			}
			if sourceURL == "" {
				continue
			}
			mimeType, data, err := decodeDataURL(sourceURL)
			if err != nil {
				return nil, err
			}
			if mimeType == "" {
				mimeType = "image/png"
			}
			blocks = append(blocks, map[string]any{
				"type": "image",
				"source": map[string]any{
					"type":       "base64",
					"media_type": mimeType,
					"data":       base64.StdEncoding.EncodeToString(data),
				},
			})
		case "image":
			source, ok := part["source"].(map[string]any)
			if !ok || len(source) == 0 {
				continue
			}
			blocks = append(blocks, map[string]any{
				"type":   "image",
				"source": source,
			})
		default:
			text := extractChatContentText(part)
			if strings.TrimSpace(text) == "" {
				continue
			}
			blocks = append(blocks, map[string]any{
				"type": "text",
				"text": text,
			})
		}
	}
	return blocks, nil
}

func normalizeAnthropicMessageRole(role string) string {
	switch strings.TrimSpace(role) {
	case "assistant":
		return "assistant"
	default:
		return "user"
	}
}

func resolveAnthropicMaxTokens(value *int) int {
	if value != nil && *value > 0 {
		return *value
	}
	return defaultChatRequestMaxTokens
}

func boolToCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
