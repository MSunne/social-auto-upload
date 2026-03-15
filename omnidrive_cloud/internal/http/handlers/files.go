package handlers

import (
	"net/http"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/http/render"
)

type FileHandler struct {
	app *appstate.App
}

func NewFileHandler(app *appstate.App) *FileHandler {
	return &FileHandler{app: app}
}

func (h *FileHandler) Get(w http.ResponseWriter, r *http.Request) {
	storageKey := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/files/"))
	if storageKey == "" {
		render.Error(w, http.StatusBadRequest, "file key is required")
		return
	}

	data, contentType, err := h.app.Storage.ReadBytes(r.Context(), storageKey)
	if err != nil {
		render.Error(w, http.StatusNotFound, "file not found")
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
