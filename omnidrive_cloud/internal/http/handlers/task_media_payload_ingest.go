package handlers

import (
	"context"
	"fmt"
	"strings"

	appstate "omnidrive_cloud/internal/app"
)

type taskMediaPayloadNormalizer struct {
	ctx         context.Context
	app         *appstate.App
	basePath    string
	ownerUserID string
	taskID      string
}

// 规范化发布任务媒体载荷，统一任务媒体载荷ingest链路的输入格式和后续处理行为。
func normalizePublishTaskMediaPayload(ctx context.Context, app *appstate.App, ownerUserID string, taskID string, payload interface{}) (interface{}, bool, error) {
	if payload == nil {
		return nil, false, nil
	}

	normalizer := &taskMediaPayloadNormalizer{
		ctx:         ctx,
		app:         app,
		basePath:    fmt.Sprintf("publish-media/%s/%s", strings.TrimSpace(ownerUserID), strings.TrimSpace(taskID)),
		ownerUserID: strings.TrimSpace(ownerUserID),
		taskID:      strings.TrimSpace(taskID),
	}
	return normalizer.normalizeValue("", payload, false)
}

// 规范化值，统一任务媒体载荷ingest链路的输入格式和后续处理行为。
func (n *taskMediaPayloadNormalizer) normalizeValue(fieldKey string, value interface{}, parentMediaObject bool) (interface{}, bool, error) {
	switch typed := value.(type) {
	case map[string]interface{}:
		return n.normalizeObject(fieldKey, typed, parentMediaObject)
	case []interface{}:
		return n.normalizeArray(fieldKey, typed)
	case string:
		if !shouldNormalizeArrayMediaURL(fieldKey) || !looksLikeRemoteHTTPURL(typed) {
			return value, false, nil
		}
		ref, _, err := normalizeManagedObjectRef(n.ctx, n.app, n.pathForField(fieldKey), managedObjectRef{
			PublicURL: normalizeTrimmedStringPtr(typed),
		})
		if err != nil {
			return nil, false, err
		}
		if ref.PublicURL == nil {
			return value, false, nil
		}
		return *ref.PublicURL, *ref.PublicURL != strings.TrimSpace(typed), nil
	default:
		return value, false, nil
	}
}

// 规范化Array，统一任务媒体载荷ingest链路的输入格式和后续处理行为。
func (n *taskMediaPayloadNormalizer) normalizeArray(containerKey string, items []interface{}) ([]interface{}, bool, error) {
	result := make([]interface{}, len(items))
	changed := false
	mediaContainer := isMediaObjectContainerKey(containerKey)

	for index, item := range items {
		switch typed := item.(type) {
		case string:
			if shouldNormalizeArrayMediaURL(containerKey) && looksLikeRemoteHTTPURL(typed) {
				ref, _, err := normalizeManagedObjectRef(n.ctx, n.app, n.pathForField(containerKey), managedObjectRef{
					PublicURL: normalizeTrimmedStringPtr(typed),
				})
				if err != nil {
					return nil, false, err
				}
				if ref.PublicURL != nil {
					result[index] = *ref.PublicURL
					changed = changed || *ref.PublicURL != strings.TrimSpace(typed)
					continue
				}
			}
			result[index] = item
		default:
			normalizedItem, itemChanged, err := n.normalizeValue(containerKey, item, mediaContainer)
			if err != nil {
				return nil, false, err
			}
			result[index] = normalizedItem
			changed = changed || itemChanged
		}
	}

	return result, changed, nil
}

// 规范化单个对象内容，统一媒体和运行时载荷中的字段结构。
func (n *taskMediaPayloadNormalizer) normalizeObject(fieldKey string, value map[string]interface{}, parentMediaObject bool) (map[string]interface{}, bool, error) {
	result := make(map[string]interface{}, len(value)+4)
	for key, raw := range value {
		result[key] = raw
	}

	objectIsMedia := parentMediaObject || isMediaObjectContainerKey(fieldKey) || mapLooksLikeMediaObject(value)
	fileName := stringValue(mediaMapStringPtr(value, "filename"))
	mimeType := mediaMapStringPtr(value, "mimetype")
	storageKey := mediaMapStringPtr(value, "storagekey")
	sizeBytes := mediaMapInt64Ptr(value, "sizebytes")
	changed := false

	for key, raw := range value {
		normalizedKey := normalizeMediaFieldKey(key)

		if rawString, ok := raw.(string); ok && shouldNormalizeObjectMediaURL(normalizedKey, objectIsMedia) && looksLikeRemoteHTTPURL(rawString) {
			ref, _, err := normalizeManagedObjectRef(n.ctx, n.app, n.pathForField(normalizedKey), managedObjectRef{
				FileName:   fileName,
				MimeType:   mimeType,
				StorageKey: storageKey,
				PublicURL:  normalizeTrimmedStringPtr(rawString),
				SizeBytes:  sizeBytes,
			})
			if err != nil {
				return nil, false, err
			}

			if ref.PublicURL != nil {
				result[key] = *ref.PublicURL
				changed = changed || *ref.PublicURL != strings.TrimSpace(rawString)
				if normalizedKey == "url" || normalizedKey == "publicurl" {
					result["publicUrl"] = *ref.PublicURL
				}
			}
			if ref.StorageKey != nil {
				result["storageKey"] = *ref.StorageKey
				storageKey = ref.StorageKey
			}
			if ref.MimeType != nil {
				result["mimeType"] = *ref.MimeType
				mimeType = ref.MimeType
			}
			if ref.SizeBytes != nil {
				result["sizeBytes"] = *ref.SizeBytes
				sizeBytes = ref.SizeBytes
			}
			if fileName == "" && ref.FileName != "" {
				result["fileName"] = ref.FileName
				fileName = ref.FileName
			}
			continue
		}

		normalizedValue, valueChanged, err := n.normalizeValue(normalizedKey, raw, objectIsMedia)
		if err != nil {
			return nil, false, err
		}
		if valueChanged {
			result[key] = normalizedValue
			changed = true
		}
	}

	return result, changed, nil
}

// 根据Field计算路径，供任务媒体载荷ingest链路复用关键派生结果。
func (n *taskMediaPayloadNormalizer) pathForField(fieldKey string) string {
	segment := normalizeMediaFieldKey(fieldKey)
	if segment == "" {
		segment = "media"
	}
	return fmt.Sprintf("%s/%s", n.basePath, segment)
}

// 规范化媒体Field键，统一任务媒体载荷ingest链路的输入格式和后续处理行为。
func normalizeMediaFieldKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

// 处理looksLike远端HTTPURL相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func looksLikeRemoteHTTPURL(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://")
}

// 判断是否应当规范化Array媒体URL，供当前链路选择后续处理策略。
func shouldNormalizeArrayMediaURL(containerKey string) bool {
	switch normalizeMediaFieldKey(containerKey) {
	case "images", "imageurls", "videos", "videourls", "attachments", "artifacts", "assets", "media", "mediaurls", "references", "previews", "keyframes", "frames":
		return true
	default:
		return false
	}
}

// 判断是否应当规范化Object媒体URL，供当前链路选择后续处理策略。
func shouldNormalizeObjectMediaURL(fieldKey string, objectIsMedia bool) bool {
	switch fieldKey {
	case "productlink", "link", "pageurl", "landingurl", "website", "weburl", "redirecturl", "callbackurl":
		return false
	case "image", "imageurl", "thumbnail", "thumbnailurl", "cover", "coverurl", "poster", "posterurl", "video", "videourl", "mediaurl", "fileurl", "downloadurl", "previewurl", "previewimage", "previewimageurl", "publicurl", "sourceurl", "src":
		return true
	case "url":
		return objectIsMedia
	default:
		return false
	}
}

// 判断是否属于媒体ObjectContainer键，供当前链路选择后续处理策略。
func isMediaObjectContainerKey(fieldKey string) bool {
	switch normalizeMediaFieldKey(fieldKey) {
	case "images", "imageurls", "videos", "videourls", "thumbnail", "cover", "poster", "preview", "attachment", "artifacts", "assets", "media", "mediaurls", "references", "previews", "keyframes", "frames":
		return true
	default:
		return false
	}
}

// 映射LooksLike媒体Object，把外部配置转换为当前业务可识别的表示。
func mapLooksLikeMediaObject(value map[string]interface{}) bool {
	for key := range value {
		switch normalizeMediaFieldKey(key) {
		case "publicurl", "storagekey", "mimetype", "filename", "artifacttype", "assettype", "downloadurl", "previewurl", "imageurl", "videourl", "thumbnailurl", "coverurl", "posterurl":
			return true
		}
	}
	return false
}

// 处理媒体映射StringPtr相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func mediaMapStringPtr(value map[string]interface{}, normalizedKey string) *string {
	for key, raw := range value {
		if normalizeMediaFieldKey(key) != normalizedKey {
			continue
		}
		text, ok := raw.(string)
		if !ok {
			continue
		}
		return normalizeTrimmedStringPtr(text)
	}
	return nil
}

// 处理媒体映射Int64Ptr相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func mediaMapInt64Ptr(value map[string]interface{}, normalizedKey string) *int64 {
	for key, raw := range value {
		if normalizeMediaFieldKey(key) != normalizedKey {
			continue
		}
		switch typed := raw.(type) {
		case int64:
			if typed <= 0 {
				return nil
			}
			return &typed
		case int:
			if typed <= 0 {
				return nil
			}
			converted := int64(typed)
			return &converted
		case float64:
			if typed <= 0 {
				return nil
			}
			converted := int64(typed)
			return &converted
		}
	}
	return nil
}
