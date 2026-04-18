package handlers

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	mixVideoSource                 = "runninghub"
	mixVideoMultipartCap           = 512 << 20
	mixVideoAssetCap               = 128 << 20
	mixVideoAudioCap               = 64 << 20
	mixVideoEstimateCharsPerSecond = 4
)

var (
	mixVideoAssetExtensions = map[string]struct{}{
		".mp4":  {},
		".mov":  {},
		".webm": {},
	}
	mixVideoAudioExtensions = map[string]struct{}{
		".m4a": {},
		".mp3": {},
		".wav": {},
		".aac": {},
	}
	mixVideoAssetMIMEs = map[string]struct{}{
		"video/mp4":       {},
		"video/quicktime": {},
		"video/webm":      {},
	}
	mixVideoAudioMIMEs = map[string]struct{}{
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

type MixVideoTaskHandler struct {
	app *appstate.App
}

func NewMixVideoTaskHandler(app *appstate.App) *MixVideoTaskHandler {
	return &MixVideoTaskHandler{app: app}
}

func (h *MixVideoTaskHandler) BillingPreview(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	preview, err := h.buildBillingPreviewFromSettings(r.Context(), user.ID, strings.TrimSpace(r.URL.Query().Get("scriptText")))
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to preview mix video billing")
		return
	}
	render.JSON(w, http.StatusOK, preview)
}

func (h *MixVideoTaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	if err := r.ParseMultipartForm(mixVideoMultipartCap); err != nil {
		render.Error(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	scriptText := strings.TrimSpace(r.FormValue("scriptText"))
	if scriptText == "" {
		render.Error(w, http.StatusBadRequest, "scriptText is required")
		return
	}
	if utf8.RuneCountInString(scriptText) > 5000 {
		render.Error(w, http.StatusBadRequest, "scriptText must be within 5000 characters")
		return
	}

	settings, err := loadEffectiveAdminSystemSettings(r.Context(), h.app)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load mix video billing config")
		return
	}
	if settings.MixVideoCreditsPerSecondMillis <= 0 {
		render.Error(w, http.StatusConflict, "混剪计费暂未开放，请稍后再试")
		return
	}

	preview, err := h.buildBillingPreview(r.Context(), user.ID, scriptText, settings.MixVideoCreditsPerSecondMillis)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to preview mix video billing")
		return
	}
	if !preview.CanAfford {
		render.Error(w, http.StatusConflict, fmt.Sprintf("当前积分不足，预计需要 %s 积分，还差 %s 积分", formatMixVideoCredits(preview.EstimatedCredits), formatMixVideoCredits(preview.ShortfallCredits)))
		return
	}

	taskID := uuid.NewString()
	assets, err := h.storeMultipartAssets(r, user.ID, taskID)
	if err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(assets) == 0 {
		render.Error(w, http.StatusBadRequest, "assets[] is required")
		return
	}
	refAudioAsset, err := h.storeMultipartSingleAsset(r, user.ID, taskID, "refAudio", "ref_audio", mixVideoAudioExtensions, mixVideoAudioMIMEs, mixVideoAudioCap)
	if err != nil {
		h.cleanupAssets(r.Context(), assets...)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	requestPayload := map[string]any{
		"source":         mixVideoSource,
		"scriptText":     scriptText,
		"assets":         assets,
		"refAudioAsset":  refAudioAsset,
		"billingPreview": preview,
	}

	task, err := h.app.Store.CreateMixVideoTask(r.Context(), store.CreateMixVideoTaskInput{
		ID:                       taskID,
		OwnerUserID:              user.ID,
		Source:                   mixVideoSource,
		Status:                   "queued",
		SourceAssets:             mustJSONBytes(assets),
		RefAudioAsset:            mustJSONBytes(refAudioAsset),
		ScriptText:               scriptText,
		EstimatedDurationSeconds: preview.EstimatedDurationSeconds,
		EstimatedCreditsMillis:   mustMixVideoCreditMillis(preview.EstimatedCredits),
		BillingStatus:            "pending",
		BillingPayload: mustJSONBytes(map[string]any{
			"creditsPerSecond":       preview.CreditsPerSecond,
			"creditsPerSecondMillis": settings.MixVideoCreditsPerSecondMillis,
			"estimateCharsPerSecond": mixVideoEstimateCharsPerSecond,
			"billingMessage":         "混剪任务待预扣积分",
		}),
		RequestPayload: mustJSONBytes(requestPayload),
		Progress: mustJSONBytes(domain.MixVideoProgress{
			Message: "混剪任务已创建，等待提交到生成服务",
		}),
	})
	if err != nil {
		h.cleanupAssets(r.Context(), append(assets, *refAudioAsset)...)
		render.Error(w, http.StatusInternalServerError, "Failed to create mix video task")
		return
	}
	task, err = h.app.Store.PrechargeMixVideoTask(r.Context(), task.ID)
	if err != nil {
		h.cleanupAssets(r.Context(), append(assets, *refAudioAsset)...)
		_ = h.app.Store.DeleteMixVideoTask(r.Context(), taskID, user.ID)
		if err == store.ErrMixVideoBillingInsufficientBalance {
			render.Error(w, http.StatusConflict, fmt.Sprintf("当前积分不足，预计需要 %s 积分，还差 %s 积分", formatMixVideoCredits(preview.EstimatedCredits), formatMixVideoCredits(preview.ShortfallCredits)))
			return
		}
		render.Error(w, http.StatusInternalServerError, "Failed to precharge mix video task")
		return
	}
	render.JSON(w, http.StatusCreated, task)
}

func (h *MixVideoTaskHandler) List(w http.ResponseWriter, r *http.Request) {
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
	beforeUpdatedAtRaw := strings.TrimSpace(r.URL.Query().Get("beforeUpdatedAt"))
	beforeID := strings.TrimSpace(r.URL.Query().Get("beforeId"))
	var beforeUpdatedAt *time.Time
	if beforeUpdatedAtRaw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, beforeUpdatedAtRaw)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "beforeUpdatedAt must be a valid RFC3339 timestamp")
			return
		}
		parsed = parsed.UTC()
		beforeUpdatedAt = &parsed
	}
	if (beforeUpdatedAt == nil) != (beforeID == "") {
		render.Error(w, http.StatusBadRequest, "beforeUpdatedAt and beforeId must be provided together")
		return
	}

	items, err := h.app.Store.ListMixVideoTasksByOwner(r.Context(), user.ID, store.ListMixVideoTasksFilter{
		Status:          strings.TrimSpace(r.URL.Query().Get("status")),
		BeforeUpdatedAt: beforeUpdatedAt,
		BeforeID:        beforeID,
		Limit:           limit,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load mix video tasks")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *MixVideoTaskHandler) Detail(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	task, err := h.app.Store.GetMixVideoTaskByOwner(r.Context(), taskID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load mix video task")
		return
	}
	if task == nil {
		render.Error(w, http.StatusNotFound, "Mix video task not found")
		return
	}
	render.JSON(w, http.StatusOK, task)
}

func (h *MixVideoTaskHandler) buildBillingPreview(ctx context.Context, userID string, scriptText string, creditsPerSecondMillis int64) (domain.MixVideoBillingPreview, error) {
	estimatedDurationSeconds := estimateMixVideoDurationSeconds(scriptText)
	if creditsPerSecondMillis < 0 {
		creditsPerSecondMillis = 0
	}
	estimatedCreditsMillis := int64(estimatedDurationSeconds) * creditsPerSecondMillis
	creditBalanceMillis, err := h.app.Store.GetWalletCreditBalanceMillisByUser(ctx, userID)
	if err != nil {
		return domain.MixVideoBillingPreview{}, err
	}
	shortfallCreditsMillis := estimatedCreditsMillis - creditBalanceMillis
	if shortfallCreditsMillis < 0 {
		shortfallCreditsMillis = 0
	}
	canAfford := creditsPerSecondMillis > 0 && creditBalanceMillis >= estimatedCreditsMillis
	return domain.MixVideoBillingPreview{
		CreditsPerSecond:         store.CreditsFromMillis(creditsPerSecondMillis),
		EstimatedDurationSeconds: estimatedDurationSeconds,
		EstimatedCredits:         store.CreditsFromMillis(estimatedCreditsMillis),
		CanAfford:                canAfford,
		CreditBalance:            store.CreditsFromMillis(creditBalanceMillis),
		ShortfallCredits:         store.CreditsFromMillis(shortfallCreditsMillis),
	}, nil
}

func (h *MixVideoTaskHandler) buildBillingPreviewFromSettings(ctx context.Context, userID string, scriptText string) (domain.MixVideoBillingPreview, error) {
	settings, err := loadEffectiveAdminSystemSettings(ctx, h.app)
	if err != nil {
		return domain.MixVideoBillingPreview{}, err
	}
	return h.buildBillingPreview(ctx, userID, scriptText, settings.MixVideoCreditsPerSecondMillis)
}

func estimateMixVideoDurationSeconds(scriptText string) int {
	nonWhitespaceRunes := 0
	for _, value := range scriptText {
		if unicode.IsSpace(value) {
			continue
		}
		nonWhitespaceRunes++
	}
	estimated := (nonWhitespaceRunes + mixVideoEstimateCharsPerSecond - 1) / mixVideoEstimateCharsPerSecond
	if estimated <= 0 {
		return 1
	}
	return estimated
}

func (h *MixVideoTaskHandler) storeMultipartAssets(r *http.Request, ownerUserID string, taskID string) ([]domain.MixVideoAsset, error) {
	headers := r.MultipartForm.File["assets[]"]
	if len(headers) == 0 {
		headers = r.MultipartForm.File["assets"]
	}
	if len(headers) == 0 {
		return nil, nil
	}
	assets := make([]domain.MixVideoAsset, 0, len(headers))
	for idx, header := range headers {
		asset, err := h.storeMultipartAssetFromHeader(r.Context(), ownerUserID, taskID, header, fmt.Sprintf("asset-%02d", idx+1), "assets[]", mixVideoAssetExtensions, mixVideoAssetMIMEs, mixVideoAssetCap)
		if err != nil {
			h.cleanupAssets(r.Context(), assets...)
			return nil, err
		}
		assets = append(assets, *asset)
	}
	return assets, nil
}

func (h *MixVideoTaskHandler) storeMultipartSingleAsset(r *http.Request, ownerUserID string, taskID string, fieldName string, folder string, allowedExts map[string]struct{}, allowedMIMEs map[string]struct{}, sizeCap int64) (*domain.MixVideoAsset, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return nil, fmt.Errorf("%s is required", fieldName)
	}
	defer file.Close()
	return h.storeMultipartAsset(r.Context(), ownerUserID, taskID, header, file, folder, fieldName, allowedExts, allowedMIMEs, sizeCap)
}

func (h *MixVideoTaskHandler) storeMultipartAssetFromHeader(ctx context.Context, ownerUserID string, taskID string, header *multipart.FileHeader, folder string, fieldName string, allowedExts map[string]struct{}, allowedMIMEs map[string]struct{}, sizeCap int64) (*domain.MixVideoAsset, error) {
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to read %s", fieldName)
	}
	defer file.Close()
	return h.storeMultipartAsset(ctx, ownerUserID, taskID, header, file, folder, fieldName, allowedExts, allowedMIMEs, sizeCap)
}

func (h *MixVideoTaskHandler) storeMultipartAsset(ctx context.Context, ownerUserID string, taskID string, header *multipart.FileHeader, reader io.Reader, folder string, fieldName string, allowedExts map[string]struct{}, allowedMIMEs map[string]struct{}, sizeCap int64) (*domain.MixVideoAsset, error) {
	data, err := io.ReadAll(io.LimitReader(reader, sizeCap))
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
	contentType, err := validateMixVideoMime(fileName, header.Header.Get("Content-Type"), data, allowedMIMEs)
	if err != nil {
		return nil, fmt.Errorf("%s %s", fieldName, err.Error())
	}

	object, err := h.app.Storage.SaveBytes(ctx, fmt.Sprintf("mix-video/%s/%s/%s/%s-%s", ownerUserID, taskID, folder, uuid.NewString(), fileName), contentType, data)
	if err != nil {
		return nil, fmt.Errorf("failed to store %s", fieldName)
	}
	sizeBytes := object.SizeBytes
	return &domain.MixVideoAsset{
		StorageKey: object.StorageKey,
		PublicURL:  object.PublicURL,
		FileName:   fileName,
		MimeType:   object.ContentType,
		SizeBytes:  &sizeBytes,
	}, nil
}

func validateMixVideoMime(fileName string, headerContentType string, data []byte, allowedMIMEs map[string]struct{}) (string, error) {
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

func (h *MixVideoTaskHandler) cleanupAssets(ctx context.Context, assets ...domain.MixVideoAsset) {
	if h.app == nil || h.app.Storage == nil {
		return
	}
	for _, asset := range assets {
		if strings.TrimSpace(asset.StorageKey) == "" {
			continue
		}
		_ = h.app.Storage.DeleteObject(ctx, asset.StorageKey)
	}
}

func formatMixVideoCredits(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func mustMixVideoCreditMillis(value float64) int64 {
	millis, err := store.CreditsToMillis(value)
	if err != nil {
		panic(err)
	}
	return millis
}
