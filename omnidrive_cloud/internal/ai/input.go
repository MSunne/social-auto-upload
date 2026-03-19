package ai

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"strconv"
	"strings"

	"omnidrive_cloud/internal/domain"
)

func BuildChatRequest(job *domain.AIJob) (ChatRequest, error) {
	payload := decodePayloadMap(job.InputPayload)
	messages, err := parseChatMessages(payload)
	if err != nil {
		return ChatRequest{}, err
	}
	if len(messages) == 0 {
		prompt := strings.TrimSpace(stringValue(job.Prompt))
		if prompt == "" {
			if raw := strings.TrimSpace(stringValueFromMap(payload, "prompt")); raw != "" {
				prompt = raw
			}
		}
		if prompt == "" {
			return ChatRequest{}, fmt.Errorf("chat job requires prompt or messages")
		}
		systemPrompt := strings.TrimSpace(stringValueFromMap(payload, "systemPrompt"))
		if systemPrompt != "" {
			messages = append(messages, ChatMessage{Role: "system", Content: systemPrompt})
		}
		messages = append(messages, ChatMessage{Role: "user", Content: prompt})
	}

	return ChatRequest{
		Model:       strings.TrimSpace(job.ModelName),
		Messages:    messages,
		Temperature: floatPtrFromMap(payload, "temperature"),
		MaxTokens:   intPtrFromMap(payload, "maxTokens", "max_tokens"),
	}, nil
}

func BuildImageRequest(job *domain.AIJob) (ImageRequest, error) {
	payload := decodePayloadMap(job.InputPayload)
	prompt := strings.TrimSpace(stringValue(job.Prompt))
	if prompt == "" {
		prompt = strings.TrimSpace(stringValueFromMap(payload, "prompt"))
	}
	if prompt == "" {
		return ImageRequest{}, fmt.Errorf("image job requires prompt")
	}

	return ImageRequest{
		Model:           strings.TrimSpace(job.ModelName),
		Prompt:          mergeMediaPrompt(prompt, payload),
		ReferenceImages: collectMediaInputs(payload),
		AspectRatio:     normalizeAspectRatio(stringValueFromMap(payload, "aspectRatio", "ratio")),
		Resolution:      normalizeResolution(stringValueFromMap(payload, "resolution", "size", "imageSize")),
	}, nil
}

func BuildVideoRequest(job *domain.AIJob) (VideoRequest, error) {
	payload := decodePayloadMap(job.InputPayload)
	prompt := strings.TrimSpace(stringValue(job.Prompt))
	if prompt == "" {
		prompt = strings.TrimSpace(stringValueFromMap(payload, "prompt"))
	}
	if prompt == "" {
		return VideoRequest{}, fmt.Errorf("video job requires prompt")
	}

	references := collectMediaInputs(payload)
	aspectRatio := normalizeAspectRatio(stringValueFromMap(payload, "aspectRatio", "ratio"))
	model := normalizeVideoModel(strings.TrimSpace(job.ModelName), aspectRatio, len(references) > 0)

	return VideoRequest{
		Model:           model,
		Prompt:          mergeMediaPrompt(prompt, payload),
		ReferenceImages: references,
		AspectRatio:     aspectRatio,
		Resolution:      normalizeResolution(stringValueFromMap(payload, "resolution", "size", "videoSize")),
		DurationSeconds: intPtrFromMap(payload, "durationSeconds", "duration"),
	}, nil
}

func decodePayloadMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

func parseChatMessages(payload map[string]any) ([]ChatMessage, error) {
	raw, ok := payload["messages"]
	if !ok {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("messages must be an array")
	}
	result := make([]ChatMessage, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("messages items must be objects")
		}
		role := strings.TrimSpace(stringValueFromMap(obj, "role"))
		if role == "" {
			return nil, fmt.Errorf("messages items require role")
		}
		content, exists := obj["content"]
		if !exists {
			return nil, fmt.Errorf("messages items require content")
		}
		result = append(result, ChatMessage{Role: role, Content: content})
	}
	return result, nil
}

func collectMediaInputs(payload map[string]any) []MediaInput {
	keys := []string{
		"referenceImages",
		"images",
		"imageUrls",
		"inputReferences",
		"sourceImages",
	}
	result := make([]MediaInput, 0)
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		items, single := normalizeToItems(raw)
		if single {
			if media, ok := parseMediaInput(raw, ""); ok {
				result = append(result, media)
			}
			continue
		}
		for _, item := range items {
			if media, ok := parseMediaInput(item, ""); ok {
				result = append(result, media)
			}
		}
		if len(result) > 0 {
			return result
		}
	}

	firstFrame := payload["firstFrame"]
	lastFrame := payload["lastFrame"]
	if media, ok := parseMediaInput(firstFrame, "first"); ok {
		result = append(result, media)
	}
	if media, ok := parseMediaInput(lastFrame, "last"); ok {
		result = append(result, media)
	}
	return result
}

func normalizeToItems(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, false
	case []string:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
		return items, false
	default:
		return nil, true
	}
}

func parseMediaInput(raw any, fallbackRole string) (MediaInput, bool) {
	switch typed := raw.(type) {
	case nil:
		return MediaInput{}, false
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return MediaInput{}, false
		}
		media := MediaInput{Role: fallbackRole}
		if strings.HasPrefix(text, "data:") {
			mimeType, data, err := decodeDataURL(text)
			if err != nil {
				return MediaInput{}, false
			}
			media.MIMEType = mimeType
			media.Data = data
			media.FileName = defaultMediaFileName(mimeType, fallbackRole)
			return media, true
		}
		media.URL = text
		media.FileName = filepath.Base(text)
		media.MIMEType = mime.TypeByExtension(filepath.Ext(media.FileName))
		return media, true
	case map[string]any:
		media := MediaInput{
			URL:      strings.TrimSpace(stringValueFromMap(typed, "url", "publicUrl", "fileUrl")),
			Base64:   strings.TrimSpace(stringValueFromMap(typed, "base64", "data")),
			MIMEType: strings.TrimSpace(stringValueFromMap(typed, "mimeType", "contentType")),
			FileName: strings.TrimSpace(stringValueFromMap(typed, "fileName", "name")),
			Role:     strings.TrimSpace(stringValueFromMap(typed, "role")),
		}
		if media.Role == "" {
			media.Role = fallbackRole
		}
		if media.Base64 != "" {
			data, err := base64.StdEncoding.DecodeString(media.Base64)
			if err == nil {
				media.Data = data
			}
		}
		if media.FileName == "" {
			if media.URL != "" {
				media.FileName = filepath.Base(media.URL)
			} else {
				media.FileName = defaultMediaFileName(media.MIMEType, media.Role)
			}
		}
		if media.MIMEType == "" && media.FileName != "" {
			media.MIMEType = mime.TypeByExtension(filepath.Ext(media.FileName))
		}
		if media.URL == "" && len(media.Data) == 0 {
			return MediaInput{}, false
		}
		return media, true
	default:
		return MediaInput{}, false
	}
}

func mergeMediaPrompt(prompt string, payload map[string]any) string {
	parts := []string{strings.TrimSpace(prompt)}
	if ratio := normalizeAspectRatio(stringValueFromMap(payload, "aspectRatio", "ratio")); ratio != "" {
		parts = append(parts, "Aspect ratio: "+ratio)
	}
	if resolution := normalizeResolution(stringValueFromMap(payload, "resolution", "size", "imageSize", "videoSize")); resolution != "" {
		parts = append(parts, "Resolution: "+resolution)
	}
	return strings.Join(nonEmpty(parts), "\n")
}

func normalizeVideoModel(model string, aspectRatio string, hasReference bool) string {
	result := strings.TrimSpace(model)
	if result == "" {
		result = "veo-3.1-fast"
	}
	if !strings.HasPrefix(strings.ToLower(result), "veo-") {
		return result
	}
	isLandscape := aspectRatio == "16:9" || strings.Contains(strings.ToLower(result), "landscape")
	if isLandscape && !strings.Contains(strings.ToLower(result), "landscape") {
		switch {
		case strings.HasSuffix(result, "-fast-fl"):
			result = strings.TrimSuffix(result, "-fast-fl") + "-landscape-fast-fl"
		case strings.HasSuffix(result, "-fast"):
			result = strings.TrimSuffix(result, "-fast") + "-landscape-fast"
		case strings.HasSuffix(result, "-fl"):
			result = strings.TrimSuffix(result, "-fl") + "-landscape-fl"
		default:
			result += "-landscape"
		}
	}
	if hasReference && !strings.HasSuffix(strings.ToLower(result), "-fl") {
		result += "-fl"
	}
	if !hasReference && strings.HasSuffix(strings.ToLower(result), "-fl") {
		result = strings.TrimSuffix(result, "-fl")
	}
	return result
}

func normalizeAspectRatio(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "9:16", "portrait", "vertical":
		return "9:16"
	case "16:9", "landscape", "horizontal":
		return "16:9"
	default:
		return ""
	}
}

func normalizeResolution(value string) string {
	return strings.TrimSpace(value)
}

func stringValueFromMap(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			if str := strings.TrimSpace(stringValue(value)); str != "" {
				return str
			}
		}
	}
	return ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case *string:
		if typed == nil {
			return ""
		}
		return *typed
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func intPtrFromMap(payload map[string]any, keys ...string) *int {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return &typed
		case int32:
			parsed := int(typed)
			return &parsed
		case int64:
			parsed := int(typed)
			return &parsed
		case float64:
			parsed := int(typed)
			return &parsed
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			parsed, err := strconv.Atoi(trimmed)
			if err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func floatPtrFromMap(payload map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return &typed
		case float32:
			parsed := float64(typed)
			return &parsed
		case int:
			parsed := float64(typed)
			return &parsed
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			parsed, err := strconv.ParseFloat(trimmed, 64)
			if err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func defaultMediaFileName(mimeType string, role string) string {
	ext := ".bin"
	if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
		ext = exts[0]
	}
	base := "reference"
	if strings.TrimSpace(role) != "" {
		base += "-" + strings.TrimSpace(role)
	}
	return base + ext
}

func decodeDataURL(value string) (string, []byte, error) {
	prefix, encoded, found := strings.Cut(value, ",")
	if !found {
		return "", nil, fmt.Errorf("invalid data url")
	}
	mimeType := "application/octet-stream"
	meta := strings.TrimPrefix(prefix, "data:")
	if mediaType, _, foundType := strings.Cut(meta, ";"); foundType && strings.TrimSpace(mediaType) != "" {
		mimeType = strings.TrimSpace(mediaType)
	} else if strings.TrimSpace(meta) != "" {
		mimeType = strings.TrimSpace(meta)
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", nil, err
	}
	return mimeType, data, nil
}

func nonEmpty(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			result = append(result, strings.TrimSpace(item))
		}
	}
	return result
}
