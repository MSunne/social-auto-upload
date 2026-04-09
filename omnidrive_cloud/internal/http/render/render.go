package render

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// 统一JSON响应输出，保证处理器返回的状态码和载荷格式一致。
func JSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

// 统一错误响应输出，保证处理器返回的状态码和载荷格式一致。
func Error(w http.ResponseWriter, statusCode int, message string) {
	JSON(w, statusCode, map[string]any{
		"error": message,
	})
}

// 统一错误Fields响应输出，保证处理器返回的状态码和载荷格式一致。
func ErrorWithFields(w http.ResponseWriter, statusCode int, message string, fields map[string]any) {
	payload := map[string]any{
		"error": message,
	}
	for key, value := range fields {
		payload[key] = value
	}
	JSON(w, statusCode, payload)
}

// 统一解码JSON响应输出，保证处理器返回的状态码和载荷格式一致。
func DecodeJSON(r *http.Request, destination any) error {
	if r.Body == nil {
		return errors.New("empty request body")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("empty request body")
		}
		return err
	}

	if decoder.More() {
		return errors.New("request body must contain a single json object")
	}
	return nil
}
