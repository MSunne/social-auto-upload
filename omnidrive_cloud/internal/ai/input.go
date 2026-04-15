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

const (
	defaultChatRequestMaxTokens    = 3200
	longFormChatRequestMaxTokens   = 4800
	reasoningChatRequestMaxTokens  = 4096
	reasoningLongFormChatMaxTokens = 6400
)

// 构建对话请求，为AI输入生成后续步骤所需的派生参数或载荷。
func BuildChatRequest(job *domain.AIJob) (ChatRequest, error) {
	payload := decodePayloadMap(job.InputPayload)
	prompt := strings.TrimSpace(stringValue(job.Prompt))
	if prompt == "" {
		prompt = strings.TrimSpace(stringValueFromMap(payload, "prompt"))
	}
	messages, err := parseChatMessages(payload)
	if err != nil {
		return ChatRequest{}, err
	}
	if len(messages) == 0 {
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
		MaxTokens:   normalizeChatRequestMaxTokens(strings.TrimSpace(job.ModelName), prompt, messages, intPtrFromMap(payload, "maxTokens", "max_tokens")),
	}, nil
}

// 规范化聊天请求MaxTokens，统一长文本与推理模型的最小输出预算。
func normalizeChatRequestMaxTokens(modelName string, prompt string, messages []ChatMessage, requested *int) *int {
	recommended := recommendedChatRequestMaxTokens(modelName, prompt, messages)
	if recommended <= 0 {
		return requested
	}
	if requested == nil || *requested < recommended {
		value := recommended
		return &value
	}
	return requested
}

// 计算聊天请求推荐输出预算，避免长文本或推理模型过早触发 length 截断。
func recommendedChatRequestMaxTokens(modelName string, prompt string, messages []ChatMessage) int {
	combinedText := strings.TrimSpace(prompt)
	if combinedText == "" {
		combinedText = strings.TrimSpace(extractChatMessagesPlainText(messages))
	}
	charCount := len([]rune(combinedText))
	longForm := isLongFormChatPrompt(combinedText)
	reasoningModel := isHighOutputChatModel(modelName)

	budget := 0
	if charCount >= 800 {
		budget = defaultChatRequestMaxTokens
	}
	if longForm {
		budget = longFormChatRequestMaxTokens
	}
	if reasoningModel && (longForm || charCount >= 600) && budget < reasoningChatRequestMaxTokens {
		budget = reasoningChatRequestMaxTokens
	}
	if reasoningModel && longForm && budget < reasoningLongFormChatMaxTokens {
		budget = reasoningLongFormChatMaxTokens
	}
	if budget == 0 {
		return 0
	}
	switch {
	case charCount >= 2400:
		budget += 1024
	case charCount >= 1200:
		budget += 512
	}
	return roundChatTokenBudget(budget)
}

// 提取聊天消息纯文本，用于估算输出预算。
func extractChatMessagesPlainText(messages []ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		if text := strings.TrimSpace(extractChatContentText(message.Content)); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

// 提取聊天内容文本，用于估算输出预算。
func extractChatContentText(content any) string {
	switch typed := content.(type) {
	case string:
		return typed
	case []map[string]any:
		parts := make([]string, 0, len(typed))
		for _, part := range typed {
			parts = append(parts, extractChatContentText(part))
		}
		return strings.Join(parts, "\n")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, extractChatContentText(item))
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		partType := strings.TrimSpace(stringValueFromMap(typed, "type"))
		switch partType {
		case "text":
			return strings.TrimSpace(stringValueFromMap(typed, "text"))
		case "input_text":
			return strings.TrimSpace(stringValueFromMap(typed, "text"))
		default:
			if text := strings.TrimSpace(stringValueFromMap(typed, "text")); text != "" {
				return text
			}
			return ""
		}
	default:
		return strings.TrimSpace(fmt.Sprint(content))
	}
}

// 判断聊天提示词是否属于长文本输出场景。
func isLongFormChatPrompt(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	keywords := []string{
		"详细", "做表", "表格", "逐步", "分步骤", "展开", "完整", "系统", "分析", "测算", "收入",
		"计算", "清单", "列出", "对比", "方案", "总结", "markdown table", "table", "breakdown",
		"step by step", "analysis", "detailed", "compare", "plan",
	}
	for _, keyword := range keywords {
		if strings.Contains(normalized, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// 判断聊天模型是否适合更高输出预算。
func isHighOutputChatModel(modelName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	if normalized == "" {
		return false
	}
	markers := []string{"gemini", "claude", "opus", "deepseek", "r1", "o1", "o3", "o4"}
	for _, marker := range markers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// 统一聊天预算步长，避免模型侧收到过细碎的 token 数。
func roundChatTokenBudget(value int) int {
	if value <= 0 {
		return 0
	}
	const step = 256
	if value%step == 0 {
		return value
	}
	return ((value / step) + 1) * step
}

// 构建图片请求，为AI输入生成后续步骤所需的派生参数或载荷。
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

// 构建视频请求，为AI输入生成后续步骤所需的派生参数或载荷。
func BuildVideoRequest(job *domain.AIJob) (VideoRequest, error) {
	payload := decodePayloadMap(job.InputPayload)
	prompt := strings.TrimSpace(stringValue(job.Prompt))
	if prompt == "" {
		prompt = strings.TrimSpace(stringValueFromMap(payload, "prompt"))
	}
	if prompt == "" {
		return VideoRequest{}, fmt.Errorf("video job requires prompt")
	}

	referenceMedia := collectVideoReferenceMedia(payload)
	referenceImages := filterMediaInputsByKind(referenceMedia, "image")
	aspectRatio := normalizeAspectRatio(stringValueFromMap(payload, "aspectRatio", "ratio"))
	model := normalizeVideoModel(strings.TrimSpace(job.ModelName), aspectRatio, len(referenceImages) > 0)

	return VideoRequest{
		Vendor:          strings.TrimSpace(stringValueFromMap(payload, "vendor")),
		Model:           model,
		Prompt:          prompt,
		ReferenceMedia:  referenceMedia,
		ReferenceImages: referenceImages,
		AspectRatio:     aspectRatio,
		Resolution:      normalizeResolution(stringValueFromMap(payload, "resolution", "size", "videoSize")),
		DurationSeconds: intPtrFromMap(payload, "durationSeconds", "duration"),
	}, nil
}

// 处理解码载荷映射相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 解析对话Messages，为AI输入提供结构化输入。
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

// 收集媒体输入，整合AI输入链路所需的候选输入。
func collectMediaInputs(payload map[string]any) []MediaInput {
	return collectMediaInputsForKeys(payload, []string{
		"referenceImages",
		"images",
		"imageUrls",
		"inputReferences",
		"sourceImages",
	})
}

// 收集视频参考媒体，整合AI输入链路所需的候选输入。
func collectVideoReferenceMedia(payload map[string]any) []MediaInput {
	result := collectMediaInputsForKeys(payload, []string{
		"referenceMedia",
		"referenceImages",
		"images",
		"imageUrls",
		"inputReferences",
		"sourceImages",
		"referenceVideos",
		"videoReferences",
		"videos",
		"videoUrls",
		"sourceVideos",
	})
	if len(result) > 0 {
		return result
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

// 收集媒体输入Keys，整合AI输入链路所需的候选输入。
func collectMediaInputsForKeys(payload map[string]any, keys []string) []MediaInput {
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
	return result
}

// 规范化Items，统一AI输入链路的输入格式和后续处理行为。
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

// 解析媒体输入，为AI输入提供结构化输入。
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
			media.Kind = detectMediaKind(mimeType, "")
			media.Data = data
			media.FileName = defaultMediaFileName(mimeType, fallbackRole)
			return media, true
		}
		media.URL = text
		media.FileName = filepath.Base(text)
		media.MIMEType = mime.TypeByExtension(filepath.Ext(media.FileName))
		media.Kind = detectMediaKind(media.MIMEType, media.FileName)
		return media, true
	case map[string]any:
		media := MediaInput{
			Kind:     strings.TrimSpace(stringValueFromMap(typed, "kind", "mediaKind", "type")),
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
		media.Kind = normalizeMediaKind(media.Kind, media.MIMEType, media.FileName)
		if media.URL == "" && len(media.Data) == 0 {
			return MediaInput{}, false
		}
		return media, true
	default:
		return MediaInput{}, false
	}
}

// 处理filter媒体输入Kind相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func filterMediaInputsByKind(items []MediaInput, kind string) []MediaInput {
	if len(items) == 0 {
		return nil
	}
	result := make([]MediaInput, 0, len(items))
	for _, item := range items {
		if normalizeMediaKind(item.Kind, item.MIMEType, item.FileName) != kind {
			continue
		}
		copyItem := item
		copyItem.Kind = kind
		result = append(result, copyItem)
	}
	return result
}

// 规范化媒体Kind，统一AI输入链路的输入格式和后续处理行为。
func normalizeMediaKind(value string, mimeType string, fileName string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "image", "video":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return detectMediaKind(mimeType, fileName)
	}
}

// 处理detect媒体Kind相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func detectMediaKind(mimeType string, fileName string) string {
	lowerMIME := strings.TrimSpace(strings.ToLower(mimeType))
	switch {
	case strings.HasPrefix(lowerMIME, "video/"):
		return "video"
	case strings.HasPrefix(lowerMIME, "image/"):
		return "image"
	}

	lowerName := strings.ToLower(strings.TrimSpace(fileName))
	switch filepath.Ext(lowerName) {
	case ".mp4", ".mov", ".avi", ".m4v", ".webm", ".mkv":
		return "video"
	default:
		return "image"
	}
}

// 合并媒体提示词，统一多来源数据后返回稳定结果。
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

// 规范化视频模型，统一AI输入链路的输入格式和后续处理行为。
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

// 规范化AspectRatio，统一AI输入链路的输入格式和后续处理行为。
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

// 规范化Resolution，统一AI输入链路的输入格式和后续处理行为。
func normalizeResolution(value string) string {
	return strings.TrimSpace(value)
}

// 处理string值映射相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理string值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理intPtr映射相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理floatPtr映射相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理默认媒体文件名称相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理解码DataURL相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理non空值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func nonEmpty(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			result = append(result, strings.TrimSpace(item))
		}
	}
	return result
}
