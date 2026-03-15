package render

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func JSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func Error(w http.ResponseWriter, statusCode int, message string) {
	JSON(w, statusCode, map[string]any{
		"error": message,
	})
}

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
