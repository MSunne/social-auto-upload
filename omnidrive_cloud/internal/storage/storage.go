package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"omnidrive_cloud/internal/config"
)

type Object struct {
	StorageKey  string
	PublicURL   string
	ContentType string
	SizeBytes   int64
}

type storageMode string

const (
	storageModeLocal storageMode = "local"
	storageModeS3    storageMode = "s3"
)

type Service struct {
	mode                  storageMode
	rootDir               string
	objectBaseURL         string
	managedPublicPrefixes []string
	httpClient            *http.Client
	s3Client              *minio.Client
	s3Bucket              string
	s3ImageStorePath      string
	s3VideoStorePath      string
}

func New(cfg config.Config) (*Service, error) {
	if hasS3Config(cfg) {
		return newS3Service(cfg)
	}
	return newLocalService(cfg)
}

func (s *Service) SaveBytes(ctx context.Context, storageKey string, contentType string, data []byte) (*Object, error) {
	return s.saveReader(ctx, storageKey, contentType, bytes.NewReader(data), int64(len(data)))
}

func (s *Service) SaveRemoteURL(ctx context.Context, storageKey string, contentType string, rawURL string) (*Object, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("remote url is required")
	}

	object, err := s.saveRemoteURLDirect(ctx, storageKey, contentType, rawURL)
	if err == nil {
		return object, nil
	}

	fallbackObject, fallbackErr := s.saveRemoteURLViaTempFile(ctx, storageKey, contentType, rawURL)
	if fallbackErr != nil {
		return nil, fmt.Errorf("stream remote url to storage: %w; fallback transfer failed: %w", err, fallbackErr)
	}
	return fallbackObject, nil
}

func (s *Service) ReadBytes(ctx context.Context, storageKey string) ([]byte, string, error) {
	storageKey = sanitizeStorageKey(storageKey)

	switch s.mode {
	case storageModeS3:
		object, err := s.s3Client.GetObject(ctx, s.s3Bucket, storageKey, minio.GetObjectOptions{})
		if err != nil {
			return nil, "", err
		}
		defer object.Close()

		info, err := object.Stat()
		if err != nil {
			return nil, "", err
		}
		data, err := io.ReadAll(object)
		if err != nil {
			return nil, "", err
		}

		contentType := resolveContentType(info.ContentType, storageKey)
		return data, contentType, nil
	default:
		fullPath := filepath.Join(s.rootDir, storageKey)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, "", err
		}
		return data, resolveContentType("", fullPath), nil
	}
}

func (s *Service) DeleteObject(ctx context.Context, storageKey string) error {
	storageKey = sanitizeStorageKey(storageKey)
	switch s.mode {
	case storageModeS3:
		return s.s3Client.RemoveObject(ctx, s.s3Bucket, storageKey, minio.RemoveObjectOptions{})
	default:
		fullPath := filepath.Join(s.rootDir, storageKey)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
}

func (s *Service) OwnsPublicURL(rawURL string) bool {
	_, ok := s.StorageKeyFromPublicURL(rawURL)
	return ok
}

func (s *Service) StorageKeyFromPublicURL(rawURL string) (string, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", false
	}

	trimmedURL := stripURLSuffix(rawURL)
	for _, prefix := range s.managedPublicPrefixes {
		if !strings.HasPrefix(trimmedURL, prefix) {
			continue
		}

		remainder := strings.TrimSpace(strings.TrimPrefix(trimmedURL, prefix))
		remainder = strings.TrimPrefix(remainder, "/")
		if remainder == "" {
			return "", false
		}
		return sanitizeStorageKey(remainder), true
	}
	return "", false
}

func newLocalService(cfg config.Config) (*Service, error) {
	rootDir := cfg.LocalStorageDir
	if rootDir == "" {
		rootDir = "./data"
	}
	rootDir = filepath.Clean(rootDir)

	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("create local storage dir: %w", err)
	}

	objectBaseURL := buildLocalObjectBaseURL(strings.TrimRight(cfg.PublicBaseURL, "/"))
	return &Service{
		mode:                  storageModeLocal,
		rootDir:               rootDir,
		objectBaseURL:         objectBaseURL,
		managedPublicPrefixes: []string{objectBaseURL + "/"},
		httpClient:            defaultHTTPClient(),
	}, nil
}

func newS3Service(cfg config.Config) (*Service, error) {
	if strings.TrimSpace(cfg.S3Endpoint) == "" || strings.TrimSpace(cfg.S3Bucket) == "" || strings.TrimSpace(cfg.S3AccessKey) == "" || strings.TrimSpace(cfg.S3SecretKey) == "" {
		return nil, fmt.Errorf("s3 storage requires endpoint, bucket, access key, and secret key")
	}

	clientEndpoint, secure, bucketLookup, primaryBaseURL, managedPrefixes, err := resolveS3Settings(cfg)
	if err != nil {
		return nil, err
	}

	client, err := minio.New(clientEndpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure:       secure,
		BucketLookup: bucketLookup,
	})
	if err != nil {
		return nil, fmt.Errorf("init s3 client: %w", err)
	}

	return &Service{
		mode:                  storageModeS3,
		objectBaseURL:         primaryBaseURL,
		managedPublicPrefixes: managedPrefixes,
		httpClient:            defaultHTTPClient(),
		s3Client:              client,
		s3Bucket:              strings.TrimSpace(cfg.S3Bucket),
		s3ImageStorePath:      sanitizeStorePath(cfg.S3ImageStorePath),
		s3VideoStorePath:      sanitizeStorePath(cfg.S3VideoStorePath),
	}, nil
}

func (s *Service) saveReader(ctx context.Context, storageKey string, contentType string, reader io.Reader, size int64) (*Object, error) {
	storageKey = sanitizeStorageKey(storageKey)
	contentType = resolveContentType(contentType, storageKey)
	finalKey := s.finalStorageKey(storageKey, contentType)

	switch s.mode {
	case storageModeS3:
		info, err := s.s3Client.PutObject(ctx, s.s3Bucket, finalKey, reader, size, minio.PutObjectOptions{
			ContentType: contentType,
		})
		if err != nil {
			return nil, fmt.Errorf("upload object to s3: %w", err)
		}

		return &Object{
			StorageKey:  finalKey,
			PublicURL:   s.publicURLFor(finalKey),
			ContentType: contentType,
			SizeBytes:   info.Size,
		}, nil
	default:
		fullPath := filepath.Join(s.rootDir, finalKey)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return nil, fmt.Errorf("create object directory: %w", err)
		}

		file, err := os.Create(fullPath)
		if err != nil {
			return nil, fmt.Errorf("create object: %w", err)
		}
		written, copyErr := io.Copy(file, reader)
		closeErr := file.Close()
		if copyErr != nil {
			return nil, fmt.Errorf("write object: %w", copyErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("finalize object: %w", closeErr)
		}

		return &Object{
			StorageKey:  finalKey,
			PublicURL:   s.publicURLFor(finalKey),
			ContentType: contentType,
			SizeBytes:   written,
		}, nil
	}
}

func (s *Service) saveRemoteURLDirect(ctx context.Context, storageKey string, contentType string, rawURL string) (*Object, error) {
	resp, err := s.fetchRemote(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	resolvedContentType := resolveContentType(firstNonEmpty(contentType, resp.Header.Get("Content-Type")), storageKey)
	finalKey := s.finalStorageKey(storageKey, resolvedContentType)
	size := resp.ContentLength

	switch s.mode {
	case storageModeS3:
		info, putErr := s.s3Client.PutObject(ctx, s.s3Bucket, finalKey, resp.Body, size, minio.PutObjectOptions{
			ContentType: resolvedContentType,
		})
		if putErr != nil {
			_ = s.DeleteObject(context.Background(), finalKey)
			return nil, fmt.Errorf("upload streamed object to s3: %w", putErr)
		}
		return &Object{
			StorageKey:  finalKey,
			PublicURL:   s.publicURLFor(finalKey),
			ContentType: resolvedContentType,
			SizeBytes:   info.Size,
		}, nil
	default:
		data, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read remote object: %w", readErr)
		}
		return s.SaveBytes(ctx, storageKey, resolvedContentType, data)
	}
}

func (s *Service) saveRemoteURLViaTempFile(ctx context.Context, storageKey string, contentType string, rawURL string) (*Object, error) {
	if s.mode != storageModeS3 {
		return nil, fmt.Errorf("temp-file fallback is only used for s3 storage")
	}

	tempFile, err := os.CreateTemp("", "omnidrive-remote-transfer-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	resp, err := s.fetchRemote(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	resolvedContentType := resolveContentType(firstNonEmpty(contentType, resp.Header.Get("Content-Type")), storageKey)
	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return nil, fmt.Errorf("download remote object to temp file: %w", err)
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind temp file: %w", err)
	}

	info, err := tempFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat temp file: %w", err)
	}
	return s.saveReader(ctx, storageKey, resolvedContentType, tempFile, info.Size())
}

func (s *Service) fetchRemote(ctx context.Context, rawURL string) (*http.Response, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid remote url")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("remote url must use http or https")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build remote request: %w", err)
	}
	req.Header.Set("User-Agent", "omnidrive-storage-transfer/1.0")
	req.Header.Set("Accept", "*/*")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch remote url: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, fmt.Errorf("remote url responded with %s", resp.Status)
	}
	return resp, nil
}

func (s *Service) finalStorageKey(storageKey string, contentType string) string {
	storageKey = sanitizeStorageKey(storageKey)
	if s.mode != storageModeS3 {
		return storageKey
	}

	storeRoot := s.storeRootForContentType(contentType)
	if storeRoot == "" || hasPathPrefix(storageKey, storeRoot) {
		return storageKey
	}
	return sanitizeStorageKey(path.Join(storeRoot, storageKey))
}

func (s *Service) storeRootForContentType(contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return s.s3ImageStorePath
	case strings.HasPrefix(contentType, "video/"):
		return s.s3VideoStorePath
	default:
		return ""
	}
}

func (s *Service) publicURLFor(storageKey string) string {
	return strings.TrimRight(s.objectBaseURL, "/") + "/" + sanitizeStorageKey(storageKey)
}

func hasS3Config(cfg config.Config) bool {
	values := []string{
		cfg.S3Endpoint,
		cfg.S3Bucket,
		cfg.S3AccessKey,
		cfg.S3SecretKey,
	}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func resolveS3Settings(cfg config.Config) (string, bool, minio.BucketLookupType, string, []string, error) {
	bucket := strings.TrimSpace(cfg.S3Bucket)
	rawEndpoint := strings.TrimSpace(cfg.S3Endpoint)
	if !strings.Contains(rawEndpoint, "://") {
		rawEndpoint = "https://" + rawEndpoint
	}

	parsedURL, err := url.Parse(rawEndpoint)
	if err != nil {
		return "", false, minio.BucketLookupAuto, "", nil, fmt.Errorf("parse s3 endpoint: %w", err)
	}
	if parsedURL.Host == "" {
		return "", false, minio.BucketLookupAuto, "", nil, fmt.Errorf("s3 endpoint host is required")
	}

	host := parsedURL.Host
	lowerBucketHost := strings.ToLower(bucket + ".")
	bucketInHost := strings.HasPrefix(strings.ToLower(host), lowerBucketHost)
	clientHost := host
	if bucketInHost {
		clientHost = host[len(bucket)+1:]
	}

	useDNS := bucketInHost || shouldUseDNSBucketLookup(clientHost)
	bucketLookup := minio.BucketLookupPath
	if useDNS {
		bucketLookup = minio.BucketLookupDNS
	}

	primaryBaseURL := strings.TrimRight(strings.TrimSpace(cfg.S3PublicBaseURL), "/")
	managedPrefixes := make([]string, 0, 3)
	if primaryBaseURL != "" {
		managedPrefixes = append(managedPrefixes, primaryBaseURL+"/")
	}

	scheme := parsedURL.Scheme
	if scheme == "" {
		scheme = "https"
	}
	if bucketInHost {
		endpointBaseURL := strings.TrimRight(fmt.Sprintf("%s://%s", scheme, host), "/")
		managedPrefixes = appendIfMissing(managedPrefixes, endpointBaseURL+"/")
		if primaryBaseURL == "" {
			primaryBaseURL = endpointBaseURL
		}
	} else if useDNS {
		dnsBaseURL := strings.TrimRight(fmt.Sprintf("%s://%s.%s", scheme, bucket, clientHost), "/")
		managedPrefixes = appendIfMissing(managedPrefixes, dnsBaseURL+"/")
		if primaryBaseURL == "" {
			primaryBaseURL = dnsBaseURL
		}
	} else {
		pathBaseURL := strings.TrimRight(fmt.Sprintf("%s://%s/%s", scheme, clientHost, bucket), "/")
		managedPrefixes = appendIfMissing(managedPrefixes, pathBaseURL+"/")
		if primaryBaseURL == "" {
			primaryBaseURL = pathBaseURL
		}
	}

	if primaryBaseURL == "" {
		return "", false, minio.BucketLookupAuto, "", nil, fmt.Errorf("failed to derive s3 public base url")
	}

	return clientHost, scheme != "http", bucketLookup, primaryBaseURL, managedPrefixes, nil
}

func buildLocalObjectBaseURL(publicBaseURL string) string {
	if publicBaseURL == "" {
		return "/api/v1/files"
	}
	return publicBaseURL + "/api/v1/files"
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 2 * time.Minute}
}

func resolveContentType(contentType string, storageKey string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(storageKey))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return contentType
}

func shouldUseDNSBucketLookup(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	return strings.HasPrefix(host, "s3.") || strings.Contains(host, ".amazonaws.com") || strings.Contains(host, ".qiniucs.com")
}

func stripURLSuffix(rawURL string) string {
	if idx := strings.IndexAny(rawURL, "?#"); idx >= 0 {
		return rawURL[:idx]
	}
	return rawURL
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
	return path.Clean(strings.Join(cleanParts, "/"))
}

func sanitizeStorePath(storePath string) string {
	storePath = strings.TrimSpace(storePath)
	if storePath == "" {
		return ""
	}
	return strings.Trim(sanitizeStorageKey(storePath), "/")
}

func appendIfMissing(items []string, item string) []string {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func hasPathPrefix(value string, prefix string) bool {
	value = strings.Trim(sanitizeStorageKey(value), "/")
	prefix = strings.Trim(sanitizeStorageKey(prefix), "/")
	return value == prefix || strings.HasPrefix(value, prefix+"/")
}
