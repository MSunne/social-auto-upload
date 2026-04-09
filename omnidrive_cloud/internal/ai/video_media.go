package ai

import "strings"

// 处理生效视频参考媒体相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func effectiveVideoReferenceMedia(req VideoRequest) []MediaInput {
	if len(req.ReferenceMedia) > 0 {
		return req.ReferenceMedia
	}
	return req.ReferenceImages
}

// 处理contains视频参考媒体相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func containsVideoReferenceMedia(items []MediaInput) bool {
	for _, item := range items {
		if normalizeMediaKind(item.Kind, item.MIMEType, item.FileName) == "video" {
			return true
		}
	}
	return false
}

// 处理filter视频参考媒体相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func filterVideoReferenceMedia(items []MediaInput, kind string) []MediaInput {
	result := make([]MediaInput, 0, len(items))
	for _, item := range items {
		resolvedKind := normalizeMediaKind(item.Kind, item.MIMEType, item.FileName)
		if strings.TrimSpace(kind) != "" && resolvedKind != kind {
			continue
		}
		copyItem := item
		copyItem.Kind = resolvedKind
		result = append(result, copyItem)
	}
	return result
}
