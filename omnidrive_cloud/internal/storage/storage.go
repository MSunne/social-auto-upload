package storage

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"omnidrive_cloud/internal/config"
)

type Object struct {
	StorageKey  string
	PublicURL   string
	ContentType string
	SizeBytes   int64
}

type Service struct {
	rootDir       string
	publicBaseURL string
}

func New(cfg config.Config) (*Service, error) {
	rootDir := cfg.LocalStorageDir
	if rootDir == "" {
		rootDir = "./data"
	}
	rootDir = filepath.Clean(rootDir)

	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("create local storage dir: %w", err)
	}

	return &Service{
		rootDir:       rootDir,
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
	}, nil
}

func (s *Service) SaveBytes(_ context.Context, storageKey string, contentType string, data []byte) (*Object, error) {
	storageKey = sanitizeStorageKey(storageKey)
	fullPath := filepath.Join(s.rootDir, storageKey)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return nil, fmt.Errorf("create object directory: %w", err)
	}
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("write object: %w", err)
	}

	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(fullPath))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return &Object{
		StorageKey:  storageKey,
		PublicURL:   s.publicURLFor(storageKey),
		ContentType: contentType,
		SizeBytes:   int64(len(data)),
	}, nil
}

func (s *Service) ReadBytes(_ context.Context, storageKey string) ([]byte, string, error) {
	storageKey = sanitizeStorageKey(storageKey)
	fullPath := filepath.Join(s.rootDir, storageKey)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, "", err
	}

	contentType := mime.TypeByExtension(filepath.Ext(fullPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return data, contentType, nil
}

func (s *Service) DeleteObject(_ context.Context, storageKey string) error {
	storageKey = sanitizeStorageKey(storageKey)
	fullPath := filepath.Join(s.rootDir, storageKey)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Service) publicURLFor(storageKey string) string {
	path := "/api/v1/files/" + storageKey
	if s.publicBaseURL == "" {
		return path
	}
	return s.publicBaseURL + path
}

func sanitizeStorageKey(storageKey string) string {
	parts := strings.Split(strings.ReplaceAll(storageKey, "\\", "/"), "/")
	cleanParts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		cleanParts = append(cleanParts, part)
	}
	if len(cleanParts) == 0 {
		return "objects/unknown"
	}
	return filepath.ToSlash(filepath.Join(cleanParts...))
}
