package logging

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const DefaultPreviewLimit = 8192

var sensitiveKeys = map[string]struct{}{
	"access_token":   {},
	"accessToken":    {},
	"adminJwtSecret": {},
	"admin_password": {},
	"adminPassword":  {},
	"agentKey":       {},
	"agent_key":      {},
	"apiKey":         {},
	"api_key":        {},
	"authorization":  {},
	"base64Data":     {},
	"base64_data":    {},
	"data":           {},
	"jwt":            {},
	"jwtSecret":      {},
	"leaseToken":     {},
	"lease_token":    {},
	"password":       {},
	"passwordHash":   {},
	"password_hash":  {},
	"qrData":         {},
	"qr_data":        {},
	"s3SecretKey":    {},
	"s3_secret_key":  {},
	"secret":         {},
	"secretKey":      {},
	"secret_key":     {},
	"token":          {},
	"x-agent-key":    {},
}

func PreviewBody(contentType string, body []byte, truncated bool) string {
	if len(body) == 0 {
		return ""
	}

	normalizedType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	var preview string

	switch {
	case normalizedType == "", strings.Contains(normalizedType, "json"):
		preview = sanitizeJSONPreview(body)
	case strings.Contains(normalizedType, "x-www-form-urlencoded"):
		values, err := url.ParseQuery(string(body))
		if err != nil {
			preview = TruncateString(string(body), DefaultPreviewLimit)
			break
		}
		sanitized := make(map[string]any, len(values))
		for key, items := range values {
			sanitized[key] = RedactValue(items)
		}
		if encoded, err := json.Marshal(sanitized); err == nil {
			preview = string(encoded)
		} else {
			preview = TruncateString(string(body), DefaultPreviewLimit)
		}
	case strings.HasPrefix(normalizedType, "text/"), strings.Contains(normalizedType, "xml"):
		preview = TruncateString(string(body), DefaultPreviewLimit)
	default:
		return fmt.Sprintf("[omitted body content type=%s size=%d]", normalizedType, len(body))
	}

	if truncated {
		return preview + " ...(truncated)"
	}
	return preview
}

func PreviewArgs(args []any, limit int) []any {
	if len(args) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 256
	}

	sanitized := make([]any, 0, len(args))
	for _, arg := range args {
		sanitized = append(sanitized, sanitizeArg(arg, limit))
	}
	return sanitized
}

func RedactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for key, item := range typed {
			if isSensitiveKey(key) {
				sanitized[key] = "[REDACTED]"
				continue
			}
			sanitized[key] = RedactValue(item)
		}
		return sanitized
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, RedactValue(item))
		}
		return items
	case map[string]string:
		sanitized := make(map[string]any, len(typed))
		for key, item := range typed {
			if isSensitiveKey(key) {
				sanitized[key] = "[REDACTED]"
				continue
			}
			sanitized[key] = sanitizeStringLiteral(item, 256)
		}
		return sanitized
	case []string:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, sanitizeStringLiteral(item, 256))
		}
		return items
	case string:
		return sanitizeStringLiteral(typed, 256)
	case []byte:
		return fmt.Sprintf("[binary %d bytes]", len(typed))
	default:
		return value
	}
}

func TruncateString(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}

func sanitizeJSONPreview(body []byte) string {
	var payload any
	if err := json.Unmarshal(body, &payload); err == nil {
		if encoded, marshalErr := json.Marshal(RedactValue(payload)); marshalErr == nil {
			return TruncateString(string(encoded), DefaultPreviewLimit)
		}
	}
	return TruncateString(string(body), DefaultPreviewLimit)
}

func sanitizeArg(arg any, limit int) any {
	switch typed := arg.(type) {
	case nil:
		return nil
	case string:
		return sanitizeStringLiteral(typed, limit)
	case []byte:
		return fmt.Sprintf("[binary %d bytes]", len(typed))
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	case map[string]any, []any, map[string]string, []string:
		return RedactValue(typed)
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return typed
	default:
		return TruncateString(fmt.Sprintf("%v", typed), limit)
	}
}

func sanitizeStringLiteral(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if looksSensitiveString(value) {
		return "[REDACTED]"
	}
	return TruncateString(value, limit)
}

func isSensitiveKey(key string) bool {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return false
	}
	if _, ok := sensitiveKeys[trimmed]; ok {
		return true
	}
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(trimmed, "-", "_"), " ", "_"))
	_, ok := sensitiveKeys[normalized]
	return ok
}

func looksSensitiveString(value string) bool {
	switch {
	case strings.HasPrefix(value, "$2a$"), strings.HasPrefix(value, "$2b$"), strings.HasPrefix(value, "$2y$"):
		return true
	case strings.Count(value, ".") == 2 && len(value) > 40:
		return true
	case len(value) > 64 && !strings.ContainsAny(value, " \t\r\n"):
		return true
	default:
		return false
	}
}
