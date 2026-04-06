package ai

import "strings"

func effectiveVideoReferenceMedia(req VideoRequest) []MediaInput {
	if len(req.ReferenceMedia) > 0 {
		return req.ReferenceMedia
	}
	return req.ReferenceImages
}

func containsVideoReferenceMedia(items []MediaInput) bool {
	for _, item := range items {
		if normalizeMediaKind(item.Kind, item.MIMEType, item.FileName) == "video" {
			return true
		}
	}
	return false
}

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
