package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type SkillHandler struct {
	app *appstate.App
}

type createSkillRequest struct {
	Name             string      `json:"name"`
	Description      string      `json:"description"`
	OutputType       string      `json:"outputType"`
	ModelName        string      `json:"modelName"`
	PromptTemplate   *string     `json:"promptTemplate"`
	ReferencePayload interface{} `json:"referencePayload"`
	IsEnabled        *bool       `json:"isEnabled"`
}

type updateSkillRequest struct {
	Name             *string     `json:"name"`
	Description      *string     `json:"description"`
	OutputType       *string     `json:"outputType"`
	ModelName        *string     `json:"modelName"`
	PromptTemplate   *string     `json:"promptTemplate"`
	ReferencePayload interface{} `json:"referencePayload"`
	IsEnabled        *bool       `json:"isEnabled"`
}

type createSkillAssetRequest struct {
	AssetType  string  `json:"assetType"`
	FileName   string  `json:"fileName"`
	MimeType   *string `json:"mimeType"`
	StorageKey *string `json:"storageKey"`
	PublicURL  *string `json:"publicUrl"`
	SizeBytes  *int64  `json:"sizeBytes"`
}

func NewSkillHandler(app *appstate.App) *SkillHandler {
	return &SkillHandler{app: app}
}

func (h *SkillHandler) List(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	items, err := h.app.Store.ListSkillsByOwner(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skills")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *SkillHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	var payload createSkillRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Description = strings.TrimSpace(payload.Description)
	payload.OutputType = strings.TrimSpace(payload.OutputType)
	payload.ModelName = strings.TrimSpace(payload.ModelName)
	if payload.Name == "" || payload.Description == "" || payload.OutputType == "" || payload.ModelName == "" {
		render.Error(w, http.StatusBadRequest, "name, description, outputType, and modelName are required")
		return
	}

	var referenceBytes []byte
	var err error
	if payload.ReferencePayload != nil {
		referenceBytes, err = json.Marshal(payload.ReferencePayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "referencePayload must be valid json")
			return
		}
	}

	isEnabled := true
	if payload.IsEnabled != nil {
		isEnabled = *payload.IsEnabled
	}

	skill, err := h.app.Store.CreateSkill(r.Context(), store.CreateSkillInput{
		ID:               uuid.NewString(),
		OwnerUserID:      user.ID,
		Name:             payload.Name,
		Description:      payload.Description,
		OutputType:       payload.OutputType,
		ModelName:        payload.ModelName,
		PromptTemplate:   payload.PromptTemplate,
		ReferencePayload: referenceBytes,
		IsEnabled:        isEnabled,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create skill")
		return
	}

	render.JSON(w, http.StatusCreated, skill)
}

func (h *SkillHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
	if skillID == "" {
		render.Error(w, http.StatusBadRequest, "skillId is required")
		return
	}

	var payload updateSkillRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var referenceBytes []byte
	var err error
	referenceTouched := payload.ReferencePayload != nil
	if referenceTouched {
		referenceBytes, err = json.Marshal(payload.ReferencePayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "referencePayload must be valid json")
			return
		}
	}

	skill, err := h.app.Store.UpdateSkill(r.Context(), skillID, user.ID, store.UpdateSkillInput{
		Name:             payload.Name,
		Description:      payload.Description,
		OutputType:       payload.OutputType,
		ModelName:        payload.ModelName,
		PromptTemplate:   payload.PromptTemplate,
		ReferencePayload: referenceBytes,
		ReferenceTouched: referenceTouched,
		IsEnabled:        payload.IsEnabled,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update skill")
		return
	}
	if skill == nil {
		render.Error(w, http.StatusNotFound, "Skill not found")
		return
	}

	render.JSON(w, http.StatusOK, skill)
}

func (h *SkillHandler) ListAssets(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
	if skillID == "" {
		render.Error(w, http.StatusBadRequest, "skillId is required")
		return
	}

	skill, err := h.app.Store.GetOwnedSkillByID(r.Context(), skillID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill")
		return
	}
	if skill == nil {
		render.Error(w, http.StatusNotFound, "Skill not found")
		return
	}

	items, err := h.app.Store.ListSkillAssets(r.Context(), skillID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill assets")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *SkillHandler) CreateAsset(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
	if skillID == "" {
		render.Error(w, http.StatusBadRequest, "skillId is required")
		return
	}

	skill, err := h.app.Store.GetOwnedSkillByID(r.Context(), skillID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill")
		return
	}
	if skill == nil {
		render.Error(w, http.StatusNotFound, "Skill not found")
		return
	}

	var payload createSkillAssetRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.AssetType = strings.TrimSpace(payload.AssetType)
	payload.FileName = strings.TrimSpace(payload.FileName)
	if payload.AssetType == "" || payload.FileName == "" {
		render.Error(w, http.StatusBadRequest, "assetType and fileName are required")
		return
	}

	asset, err := h.app.Store.CreateSkillAsset(r.Context(), store.CreateSkillAssetInput{
		ID:          uuid.NewString(),
		SkillID:     skillID,
		OwnerUserID: user.ID,
		AssetType:   payload.AssetType,
		FileName:    payload.FileName,
		MimeType:    payload.MimeType,
		StorageKey:  payload.StorageKey,
		PublicURL:   payload.PublicURL,
		SizeBytes:   payload.SizeBytes,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create skill asset")
		return
	}

	render.JSON(w, http.StatusCreated, asset)
}

func (h *SkillHandler) UploadAsset(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	skillID := strings.TrimSpace(chi.URLParam(r, "skillId"))
	if skillID == "" {
		render.Error(w, http.StatusBadRequest, "skillId is required")
		return
	}

	skill, err := h.app.Store.GetOwnedSkillByID(r.Context(), skillID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skill")
		return
	}
	if skill == nil {
		render.Error(w, http.StatusNotFound, "Skill not found")
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		render.Error(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	assetType := strings.TrimSpace(r.FormValue("assetType"))
	if assetType == "" {
		render.Error(w, http.StatusBadRequest, "assetType is required")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		render.Error(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 32<<20))
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to read file")
		return
	}

	fileName := sanitizeUploadFilename(header.Filename)
	contentType := header.Header.Get("Content-Type")
	object, err := h.app.Storage.SaveBytes(
		r.Context(),
		fmt.Sprintf("skills/%s/%s/%s-%s", user.ID, skillID, uuid.NewString(), fileName),
		contentType,
		data,
	)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to store file")
		return
	}

	asset, err := h.app.Store.CreateSkillAsset(r.Context(), store.CreateSkillAssetInput{
		ID:          uuid.NewString(),
		SkillID:     skillID,
		OwnerUserID: user.ID,
		AssetType:   assetType,
		FileName:    fileName,
		MimeType:    &object.ContentType,
		StorageKey:  &object.StorageKey,
		PublicURL:   &object.PublicURL,
		SizeBytes:   &object.SizeBytes,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create asset metadata")
		return
	}

	render.JSON(w, http.StatusCreated, asset)
}

func sanitizeUploadFilename(fileName string) string {
	base := strings.TrimSpace(filepath.Base(fileName))
	if base == "" || base == "." || base == "/" {
		return "file.bin"
	}
	return strings.ReplaceAll(base, " ", "_")
}
