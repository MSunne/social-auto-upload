package handlers

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

const (
	digitalHumanSource                 = "runninghub"
	digitalHumanMultipartCap           = 96 << 20
	digitalHumanAssetCap               = 64 << 20
	digitalHumanEstimateCharsPerSecond = 4
)

var (
	digitalHumanImageExtensions = map[string]struct{}{
		".jpg":  {},
		".jpeg": {},
		".png":  {},
		".webp": {},
	}
	digitalHumanAudioExtensions = map[string]struct{}{
		".m4a": {},
		".mp3": {},
		".wav": {},
		".aac": {},
	}
	digitalHumanImageMIMEs = map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	}
	digitalHumanAudioMIMEs = map[string]struct{}{
		"audio/mp4":      {},
		"audio/x-m4a":    {},
		"audio/m4a":      {},
		"video/mp4":      {},
		"audio/mpeg":     {},
		"audio/mp3":      {},
		"audio/wav":      {},
		"audio/x-wav":    {},
		"audio/vnd.wave": {},
		"audio/aac":      {},
	}
)

type DigitalHumanTaskHandler struct {
	app *appstate.App
}

func NewDigitalHumanTaskHandler(app *appstate.App) *DigitalHumanTaskHandler {
	return &DigitalHumanTaskHandler{app: app}
}

func (h *DigitalHumanTaskHandler) BillingPreview(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load digital human billing config")
		return
	}

	preview, err := h.buildBillingPreview(r.Context(), user.ID, strings.TrimSpace(r.URL.Query().Get("goodsText")), settings.DigitalHumanCreditsPerSecondMillis)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to preview digital human billing")
		return
	}
	render.JSON(w, http.StatusOK, preview)
}

func (h *DigitalHumanTaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	if err := r.ParseMultipartForm(digitalHumanMultipartCap); err != nil {
		render.Error(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	mode := strings.ToLower(strings.TrimSpace(r.FormValue("mode")))
	if mode != "digital" && mode != "customize" {
		render.Error(w, http.StatusBadRequest, "mode must be digital or customize")
		return
	}

	goodsText := strings.TrimSpace(r.FormValue("goodsText"))
	if goodsText == "" {
		render.Error(w, http.StatusBadRequest, "goodsText is required")
		return
	}
	if utf8.RuneCountInString(goodsText) > 1000 {
		render.Error(w, http.StatusBadRequest, "goodsText must be within 1000 characters")
		return
	}

	settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load digital human billing config")
		return
	}
	if settings.DigitalHumanCreditsPerSecondMillis <= 0 {
		render.Error(w, http.StatusConflict, "数字人计费暂未开放，请稍后再试")
		return
	}
	preview, err := h.buildBillingPreview(r.Context(), user.ID, goodsText, settings.DigitalHumanCreditsPerSecondMillis)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to preview digital human billing")
		return
	}
	if !preview.CanAfford {
		render.Error(w, http.StatusConflict, fmt.Sprintf("当前积分不足，预计需要 %s 积分，还差 %s 积分", formatDigitalHumanCredits(preview.EstimatedCredits), formatDigitalHumanCredits(preview.ShortfallCredits)))
		return
	}

	goodsTitle := strings.TrimSpace(r.FormValue("goodsTitle"))
	if mode == "digital" {
		if goodsTitle == "" {
			render.Error(w, http.StatusBadRequest, "goodsTitle is required for digital mode")
			return
		}
		if utf8.RuneCountInString(goodsTitle) > 20 {
			render.Error(w, http.StatusBadRequest, "goodsTitle must be within 20 characters")
			return
		}
	} else if goodsTitle != "" {
		render.Error(w, http.StatusBadRequest, "goodsTitle is not allowed for customize mode")
		return
	}

	taskID := uuid.NewString()
	characterAsset, err := h.storeMultipartAsset(r, user.ID, taskID, "characterImage", "character", digitalHumanImageExtensions, digitalHumanImageMIMEs, true)
	if err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	refAudioAsset, err := h.storeMultipartAsset(r, user.ID, taskID, "refAudio", "ref_audio", digitalHumanAudioExtensions, digitalHumanAudioMIMEs, true)
	if err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var goodsAsset *domain.DigitalHumanAsset
	switch mode {
	case "digital":
		goodsAsset, err = h.storeMultipartAsset(r, user.ID, taskID, "goodsImage", "goods", digitalHumanImageExtensions, digitalHumanImageMIMEs, true)
		if err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	case "customize":
		goodsAsset, err = h.storeMultipartAsset(r, user.ID, taskID, "goodsImage", "goods", digitalHumanImageExtensions, digitalHumanImageMIMEs, false)
		if err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		if goodsAsset != nil {
			render.Error(w, http.StatusBadRequest, "goodsImage is not allowed for customize mode")
			return
		}
	}

	requestPayload := map[string]any{
		"mode":           mode,
		"source":         digitalHumanSource,
		"goodsText":      goodsText,
		"billingPreview": preview,
		"characterAsset": characterAsset,
		"refAudioAsset":  refAudioAsset,
	}
	if goodsTitle != "" {
		requestPayload["goodsTitle"] = goodsTitle
	}
	if goodsAsset != nil {
		requestPayload["goodsAsset"] = goodsAsset
	}

	task, err := h.app.Store.CreateDigitalHumanTask(r.Context(), store.CreateDigitalHumanTaskInput{
		ID:                       taskID,
		OwnerUserID:              user.ID,
		Mode:                     mode,
		Source:                   digitalHumanSource,
		Status:                   "queued",
		CharacterAsset:           mustJSONBytes(characterAsset),
		GoodsAsset:               mustJSONBytes(goodsAsset),
		RefAudioAsset:            mustJSONBytes(refAudioAsset),
		GoodsTitle:               stringPtr(goodsTitle),
		GoodsText:                goodsText,
		EstimatedDurationSeconds: preview.EstimatedDurationSeconds,
		EstimatedCreditsMillis:   mustCreditMillis(preview.EstimatedCredits),
		BillingStatus:            "pending",
		BillingPayload: mustJSONBytes(map[string]any{
			"creditsPerSecond":       preview.CreditsPerSecond,
			"creditsPerSecondMillis": settings.DigitalHumanCreditsPerSecondMillis,
			"estimateCharsPerSecond": digitalHumanEstimateCharsPerSecond,
			"billingMessage":         "数字人任务待预扣积分",
		}),
		RequestPayload: mustJSONBytes(requestPayload),
		Progress: mustJSONBytes(domain.DigitalHumanProgress{
			Current:    0,
			Total:      0,
			Percentage: 0,
			Message:    "数字人任务已创建，等待提交到生成服务",
		}),
	})
	if err != nil {
		h.cleanupAssets(r.Context(), characterAsset, goodsAsset, refAudioAsset)
		render.Error(w, http.StatusInternalServerError, "Failed to create digital human task")
		return
	}
	task, err = h.app.Store.PrechargeDigitalHumanTask(r.Context(), task.ID)
	if err != nil {
		h.cleanupAssets(r.Context(), characterAsset, goodsAsset, refAudioAsset)
		_ = h.app.Store.DeleteDigitalHumanTask(r.Context(), taskID, user.ID)
		if err == store.ErrDigitalHumanBillingInsufficientBalance {
			render.Error(w, http.StatusConflict, fmt.Sprintf("当前积分不足，预计需要 %s 积分，还差 %s 积分", formatDigitalHumanCredits(preview.EstimatedCredits), formatDigitalHumanCredits(preview.ShortfallCredits)))
			return
		}
		render.Error(w, http.StatusInternalServerError, "Failed to precharge digital human task")
		return
	}
	render.JSON(w, http.StatusCreated, task)
}

func (h *DigitalHumanTaskHandler) List(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	limit := 0
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			render.Error(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = parsed
	}
	items, err := h.app.Store.ListDigitalHumanTasksByOwner(r.Context(), user.ID, store.ListDigitalHumanTasksFilter{
		Mode:   strings.TrimSpace(r.URL.Query().Get("mode")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:  limit,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load digital human tasks")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *DigitalHumanTaskHandler) Detail(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	task, err := h.app.Store.GetDigitalHumanTaskByOwner(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load digital human task")
		return
	}
	if task == nil {
		render.Error(w, http.StatusNotFound, "Digital human task not found")
		return
	}
	render.JSON(w, http.StatusOK, task)
}

func (h *DigitalHumanTaskHandler) storeMultipartAsset(
	r *http.Request,
	ownerUserID string,
	taskID string,
	fieldName string,
	assetFolder string,
	allowedExts map[string]struct{},
	allowedMIMEs map[string]struct{},
	required bool,
) (*domain.DigitalHumanAsset, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		if !required && strings.Contains(strings.ToLower(err.Error()), "no such file") {
			return nil, nil
		}
		if !required && err == http.ErrMissingFile {
			return nil, nil
		}
		if required {
			return nil, fmt.Errorf("%s is required", fieldName)
		}
		return nil, nil
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, digitalHumanAssetCap))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s", fieldName)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%s is empty", fieldName)
	}

	fileName := sanitizeUploadFilename(header.Filename)
	extension := strings.ToLower(filepath.Ext(fileName))
	if _, ok := allowedExts[extension]; !ok {
		return nil, fmt.Errorf("%s format is not supported", fieldName)
	}
	contentType, err := validateDigitalHumanMime(fileName, header.Header.Get("Content-Type"), data, allowedMIMEs)
	if err != nil {
		return nil, fmt.Errorf("%s %s", fieldName, err.Error())
	}

	object, err := h.app.Storage.SaveBytes(
		r.Context(),
		fmt.Sprintf("digital-human/%s/%s/%s/%s-%s", ownerUserID, taskID, assetFolder, uuid.NewString(), fileName),
		contentType,
		data,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store %s", fieldName)
	}

	sizeBytes := object.SizeBytes
	return &domain.DigitalHumanAsset{
		StorageKey: object.StorageKey,
		PublicURL:  object.PublicURL,
		FileName:   fileName,
		MimeType:   object.ContentType,
		SizeBytes:  &sizeBytes,
	}, nil
}

func validateDigitalHumanMime(fileName string, headerContentType string, data []byte, allowedMIMEs map[string]struct{}) (string, error) {
	candidates := make([]string, 0, 3)
	if trimmed := strings.ToLower(strings.TrimSpace(strings.Split(headerContentType, ";")[0])); trimmed != "" {
		candidates = append(candidates, trimmed)
	}
	if sniffed := strings.ToLower(strings.TrimSpace(http.DetectContentType(data))); sniffed != "" {
		candidates = append(candidates, sniffed)
	}
	if byExt := strings.ToLower(strings.TrimSpace(mime.TypeByExtension(strings.ToLower(filepath.Ext(fileName))))); byExt != "" {
		candidates = append(candidates, byExt)
	}
	for _, candidate := range candidates {
		if _, ok := allowedMIMEs[candidate]; ok {
			return candidate, nil
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("mime type is missing")
	}
	return "", fmt.Errorf("mime type is not supported")
}

func (h *DigitalHumanTaskHandler) buildBillingPreview(ctx context.Context, userID string, goodsText string, creditsPerSecondMillis int64) (domain.DigitalHumanBillingPreview, error) {
	estimatedDurationSeconds := estimateDigitalHumanDurationSeconds(goodsText)
	if creditsPerSecondMillis < 0 {
		creditsPerSecondMillis = 0
	}
	estimatedCreditsMillis := int64(estimatedDurationSeconds) * creditsPerSecondMillis
	creditBalanceMillis, err := h.app.Store.GetWalletCreditBalanceMillisByUser(ctx, userID)
	if err != nil {
		return domain.DigitalHumanBillingPreview{}, err
	}
	shortfallCreditsMillis := estimatedCreditsMillis - creditBalanceMillis
	if shortfallCreditsMillis < 0 {
		shortfallCreditsMillis = 0
	}
	canAfford := creditsPerSecondMillis > 0 && creditBalanceMillis >= estimatedCreditsMillis
	return domain.DigitalHumanBillingPreview{
		CreditsPerSecond:         store.DigitalHumanCreditsFromMillis(creditsPerSecondMillis),
		EstimatedDurationSeconds: estimatedDurationSeconds,
		EstimatedCredits:         store.DigitalHumanCreditsFromMillis(estimatedCreditsMillis),
		CanAfford:                canAfford,
		CreditBalance:            store.DigitalHumanCreditsFromMillis(creditBalanceMillis),
		ShortfallCredits:         store.DigitalHumanCreditsFromMillis(shortfallCreditsMillis),
	}, nil
}

func estimateDigitalHumanDurationSeconds(goodsText string) int {
	nonWhitespaceRunes := 0
	for _, value := range goodsText {
		if unicode.IsSpace(value) {
			continue
		}
		nonWhitespaceRunes++
	}
	estimated := (nonWhitespaceRunes + digitalHumanEstimateCharsPerSecond - 1) / digitalHumanEstimateCharsPerSecond
	if estimated <= 0 {
		return 1
	}
	return estimated
}

func (h *DigitalHumanTaskHandler) cleanupAssets(ctx context.Context, assets ...*domain.DigitalHumanAsset) {
	if h.app == nil || h.app.Storage == nil {
		return
	}
	for _, asset := range assets {
		if asset == nil || strings.TrimSpace(asset.StorageKey) == "" {
			continue
		}
		_ = h.app.Storage.DeleteObject(ctx, asset.StorageKey)
	}
}

func formatDigitalHumanCredits(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func mustCreditMillis(value float64) int64 {
	millis, err := store.DigitalHumanCreditsToMillis(value)
	if err != nil {
		panic(err)
	}
	return millis
}
