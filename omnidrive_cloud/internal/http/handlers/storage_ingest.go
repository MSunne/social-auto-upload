package handlers

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
)

type managedObjectRef struct {
	FileName   string
	MimeType   *string
	StorageKey *string
	PublicURL  *string
	SizeBytes  *int64
}

func normalizeManagedObjectRef(ctx context.Context, app *appstate.App, basePath string, ref managedObjectRef) (managedObjectRef, bool, error) {
	ref.FileName = deriveManagedFileName(ref.FileName, ref.PublicURL)
	ref.MimeType = normalizeTrimmedString(ref.MimeType)
	ref.StorageKey = normalizeTrimmedString(ref.StorageKey)
	ref.PublicURL = normalizeTrimmedString(ref.PublicURL)
	ref.SizeBytes = normalizeSizeBytes(ref.SizeBytes)

	if ref.PublicURL == nil {
		return ref, false, nil
	}
	if app == nil || app.Storage == nil {
		return ref, false, fmt.Errorf("storage service is not available")
	}

	if storageKey, ok := app.Storage.StorageKeyFromPublicURL(*ref.PublicURL); ok {
		ref.StorageKey = &storageKey
		return ref, false, nil
	}

	object, err := app.Storage.SaveRemoteURL(
		ctx,
		path.Join(basePath, uuid.NewString()+"-"+ref.FileName),
		stringValue(ref.MimeType),
		*ref.PublicURL,
	)
	if err != nil {
		return ref, false, err
	}

	ref.MimeType = &object.ContentType
	ref.StorageKey = &object.StorageKey
	ref.PublicURL = &object.PublicURL
	ref.SizeBytes = &object.SizeBytes
	return ref, true, nil
}

func deriveManagedFileName(fileName string, rawURL *string) string {
	fileName = strings.TrimSpace(fileName)
	if fileName != "" {
		return sanitizeUploadFilename(fileName)
	}

	if rawURL != nil {
		if parsedURL, err := url.Parse(strings.TrimSpace(*rawURL)); err == nil {
			candidate := strings.TrimSpace(path.Base(parsedURL.Path))
			if candidate != "" && candidate != "." && candidate != "/" {
				return sanitizeUploadFilename(candidate)
			}
		}
	}
	return "file.bin"
}

func normalizeSizeBytes(sizeBytes *int64) *int64 {
	if sizeBytes == nil || *sizeBytes <= 0 {
		return nil
	}
	return sizeBytes
}
