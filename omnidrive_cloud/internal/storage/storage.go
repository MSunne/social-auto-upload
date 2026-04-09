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

// 根据配置创建对象存储服务，在本地存储和 S3 存储之间选择最终实现。
func New(cfg config.Config) (*Service, error) {
	if hasS3Config(cfg) {
		return newS3Service(cfg)
	}
	return newLocalService(cfg)
}

// 保存字节内容到对象存储，并统一处理内容类型和最终存储键。
func (s *Service) SaveBytes(ctx context.Context, storageKey string, contentType string, data []byte) (*Object, error) {
	return s.saveReader(ctx, storageKey, contentType, bytes.NewReader(data), int64(len(data)))
}

// 将远端 URL 指向的内容转存到对象存储，必要时回退到临时文件方案。
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

// 从对象存储读取字节内容，并返回后续响应所需的内容类型。
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

// 删除对象存储中的目标资源，并兼容本地或 S3 两种存储模式。
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

// 判断公开 URL 是否归属当前对象存储服务，供回收和去重逻辑选择分支。
func (s *Service) OwnsPublicURL(rawURL string) bool {
	_, ok := s.StorageKeyFromPublicURL(rawURL)
	return ok
}

// 从公开 URL 中提取存储键，供资源回收和关联恢复逻辑复用。
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

// 返回当前对象存储使用的底层模式，供上层逻辑区分本地或 S3 分支。
func (s *Service) Mode() string {
	return string(s.mode)
}

// 创建本地Service相关实例，组装运行所需依赖并返回给上层流程复用。
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

// 创建S3Service相关实例，组装运行所需依赖并返回给上层流程复用。
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

// 保存Reader，统一对象存储链路中的资源落盘或持久化行为。
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

// 保存远端URLDirect，统一对象存储链路中的资源落盘或持久化行为。
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

// 保存远端URLViaTemp文件，统一对象存储链路中的资源落盘或持久化行为。
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

// 处理fetch远端相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理final存储键相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 根据内容类型计算存储Root，供对象存储链路复用关键派生结果。
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

// 根据计算公开URL，供对象存储链路复用关键派生结果。
func (s *Service) publicURLFor(storageKey string) string {
	return strings.TrimRight(s.objectBaseURL, "/") + "/" + sanitizeStorageKey(storageKey)
}

// 判断是否存在S3配置，供当前链路选择后续处理策略。
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

// 解析S3Settings，根据当前配置和上下文确定最终使用结果。
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

// 构建本地对象访问基址，为对象存储生成后续步骤所需的派生参数或载荷。
func buildLocalObjectBaseURL(publicBaseURL string) string {
	if publicBaseURL == "" {
		return "/api/v1/files"
	}
	return publicBaseURL + "/api/v1/files"
}

// 处理默认HTTP客户端相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 2 * time.Minute}
}

// 解析内容类型，根据当前配置和上下文确定最终使用结果。
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

// 判断是否应当UseDNSBucket查找，供当前链路选择后续处理策略。
func shouldUseDNSBucketLookup(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	return strings.HasPrefix(host, "s3.") || strings.Contains(host, ".amazonaws.com") || strings.Contains(host, ".qiniucs.com")
}

// 剥离URLSuffix中的包装内容，便于后续解析实际数据。
func stripURLSuffix(rawURL string) string {
	if idx := strings.IndexAny(rawURL, "?#"); idx >= 0 {
		return rawURL[:idx]
	}
	return rawURL
}

// 处理清洗存储键相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
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

// 处理清洗存储路径相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func sanitizeStorePath(storePath string) string {
	storePath = strings.TrimSpace(storePath)
	if storePath == "" {
		return ""
	}
	return strings.Trim(sanitizeStorageKey(storePath), "/")
}

// 处理追加IfMissing相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func appendIfMissing(items []string, item string) []string {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

// 处理首个Non空值相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// 判断是否存在路径Prefix，供当前链路选择后续处理策略。
func hasPathPrefix(value string, prefix string) bool {
	value = strings.Trim(sanitizeStorageKey(value), "/")
	prefix = strings.Trim(sanitizeStorageKey(prefix), "/")
	return value == prefix || strings.HasPrefix(value, prefix+"/")
}
