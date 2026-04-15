package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment                   string
	BindAddr                      string
	DatabaseDSN                   string
	RedisAddr                     string
	PublicBaseURL                 string
	CORSAllowedOrigins            []string
	LogLevel                      string
	LogFormat                     string
	LocalStorageDir               string
	S3Endpoint                    string
	S3Bucket                      string
	S3AccessKey                   string
	S3SecretKey                   string
	S3PublicBaseURL               string
	S3ImageStorePath              string
	S3VideoStorePath              string
	APIYIBaseURL                  string
	APIYIApiKey                   string
	DigitalHumanBaseURL           string
	DigitalHumanPollSeconds       int
	AIWorkerEnabled               bool
	AIWorkerPollSeconds           int
	AIWorkerConcurrency           int
	AIStaleQueueTimeoutSeconds    int
	AIVideoPollSeconds            int
	AIVideoTimeoutSeconds         int
	AIChatStreamTimeoutSeconds    int
	AIChatStreamHeartbeatSeconds  int
	AIVideoStandardizeEnabled     bool
	AIVideoFFmpegPath             string
	DefaultChatModel              string
	DefaultImageModel             string
	DefaultVideoModel             string
	JWTSecret                     string
	AccessTokenExpireMinutes      int
	AdminJWTSecret                string
	AdminEmail                    string
	AdminName                     string
	AdminPassword                 string
	AdminAccessTokenExpireMinutes int
	DevSeedUsers                  bool
	DevSeedUserPassword           string
	AutoCreateSchema              bool
	BillingManualSupportName      string
	BillingManualSupportContact   string
	BillingManualSupportQRCodeURL string
	BillingManualSupportNote      string
}

// 加载配置，供配置继续处理当前业务状态。
func Load() Config {
	_ = godotenv.Load()
	environment := envOrDefault("OMNIDRIVE_ENV", "development")

	jwtSecret := envOrDefault("OMNIDRIVE_JWT_SECRET", "change-me")
	accessTokenExpireMinutes := envAsInt("OMNIDRIVE_ACCESS_TOKEN_EXPIRE_MINUTES", 720)
	adminJWTSecret := envFirst("", "OMNIDRIVE_ADMIN_JWT_SECRET")
	if adminJWTSecret == "" {
		adminJWTSecret = jwtSecret
	}
	adminAccessTokenExpireMinutes := envAsInt("OMNIDRIVE_ADMIN_ACCESS_TOKEN_EXPIRE_MINUTES", accessTokenExpireMinutes)

	return Config{
		Environment:                   environment,
		BindAddr:                      envOrDefault("OMNIDRIVE_BIND_ADDR", ":8410"),
		DatabaseDSN:                   envOrDefault("OMNIDRIVE_DATABASE_DSN", ""),
		RedisAddr:                     envOrDefault("OMNIDRIVE_REDIS_ADDR", ""),
		PublicBaseURL:                 envOrDefault("OMNIDRIVE_PUBLIC_BASE_URL", ""),
		CORSAllowedOrigins:            envAsCSV("OMNIDRIVE_CORS_ALLOWED_ORIGINS", defaultCORSAllowedOrigins(environment)),
		LogLevel:                      envOrDefault("OMNIDRIVE_LOG_LEVEL", defaultLogLevel(environment)),
		LogFormat:                     envOrDefault("OMNIDRIVE_LOG_FORMAT", defaultLogFormat(environment)),
		LocalStorageDir:               envOrDefault("OMNIDRIVE_LOCAL_STORAGE_DIR", "./data"),
		S3Endpoint:                    envFirst("", "OMNIDRIVE_S3_ENDPOINT", "S3_ENDPOINT_URL"),
		S3Bucket:                      envFirst("", "OMNIDRIVE_S3_BUCKET", "S3_BUCKET_NAME"),
		S3AccessKey:                   envFirst("", "OMNIDRIVE_S3_ACCESS_KEY", "S3_ACCESS_KEY_ID"),
		S3SecretKey:                   envFirst("", "OMNIDRIVE_S3_SECRET_KEY", "S3_SECRET_ACCESS_KEY"),
		S3PublicBaseURL:               envFirst("", "OMNIDRIVE_S3_PUBLIC_BASE_URL", "S3_PRIVATE_URL"),
		S3ImageStorePath:              envFirst("", "OMNIDRIVE_S3_IMAGE_STORE_PATH", "IMAGE_STORE_PATH"),
		S3VideoStorePath:              envFirst("", "OMNIDRIVE_S3_VIDEO_STORE_PATH", "VIDEO_STORE_PATH"),
		APIYIBaseURL:                  envOrDefault("OMNIDRIVE_APIYI_BASE_URL", "https://api.apiyi.com"),
		APIYIApiKey:                   envFirst("", "OMNIDRIVE_APIYI_API_KEY", "APIYI_API_KEY"),
		DigitalHumanBaseURL:           envOrDefault("OMNIDRIVE_DIGITAL_HUMAN_BASE_URL", "http://127.0.0.1:8000"),
		DigitalHumanPollSeconds:       envAsInt("OMNIDRIVE_DIGITAL_HUMAN_POLL_SECONDS", 5),
		AIWorkerEnabled:               envAsBool("OMNIDRIVE_AI_WORKER_ENABLED", true),
		AIWorkerPollSeconds:           envAsInt("OMNIDRIVE_AI_WORKER_POLL_SECONDS", 5),
		AIWorkerConcurrency:           envAsInt("OMNIDRIVE_AI_WORKER_CONCURRENCY", 2),
		AIStaleQueueTimeoutSeconds:    envAsInt("OMNIDRIVE_AI_STALE_QUEUE_TIMEOUT_SECONDS", 600),
		AIVideoPollSeconds:            envAsInt("OMNIDRIVE_AI_VIDEO_POLL_SECONDS", 6),
		AIVideoTimeoutSeconds:         envAsInt("OMNIDRIVE_AI_VIDEO_TIMEOUT_SECONDS", 600),
		AIChatStreamTimeoutSeconds:    envAsInt("OMNIDRIVE_AI_CHAT_STREAM_TIMEOUT_SECONDS", 600),
		AIChatStreamHeartbeatSeconds:  envAsInt("OMNIDRIVE_AI_CHAT_STREAM_HEARTBEAT_SECONDS", 15),
		AIVideoStandardizeEnabled:     envAsBool("OMNIDRIVE_AI_VIDEO_STANDARDIZE_ENABLED", true),
		AIVideoFFmpegPath:             envOrDefault("OMNIDRIVE_AI_VIDEO_FFMPEG_PATH", "ffmpeg"),
		DefaultChatModel:              envOrDefault("OMNIDRIVE_DEFAULT_CHAT_MODEL", "gemini-3.1-pro-preview"),
		DefaultImageModel:             envOrDefault("OMNIDRIVE_DEFAULT_IMAGE_MODEL", "gemini-3-pro-image-preview"),
		DefaultVideoModel:             envOrDefault("OMNIDRIVE_DEFAULT_VIDEO_MODEL", "veo-3.1-fast-fl"),
		JWTSecret:                     jwtSecret,
		AccessTokenExpireMinutes:      accessTokenExpireMinutes,
		AdminJWTSecret:                adminJWTSecret,
		AdminEmail:                    envFirst("admin", "OMNIDRIVE_ADMIN_USERNAME", "OMNIDRIVE_ADMIN_EMAIL"),
		AdminName:                     envOrDefault("OMNIDRIVE_ADMIN_NAME", "Super Admin"),
		AdminPassword:                 envOrDefault("OMNIDRIVE_ADMIN_PASSWORD", "123456"),
		AdminAccessTokenExpireMinutes: adminAccessTokenExpireMinutes,
		DevSeedUsers:                  envAsBool("OMNIDRIVE_DEV_SEED_USERS", strings.EqualFold(strings.TrimSpace(environment), "development")),
		DevSeedUserPassword:           envOrDefault("OMNIDRIVE_DEV_SEED_USER_PASSWORD", "demo123456"),
		AutoCreateSchema:              envAsBool("OMNIDRIVE_AUTO_CREATE_SCHEMA", true),
		BillingManualSupportName:      envOrDefault("OMNIDRIVE_BILLING_MANUAL_SUPPORT_NAME", "客服充值"),
		BillingManualSupportContact:   envOrDefault("OMNIDRIVE_BILLING_MANUAL_SUPPORT_CONTACT", ""),
		BillingManualSupportQRCodeURL: envOrDefault("OMNIDRIVE_BILLING_MANUAL_SUPPORT_QRCODE_URL", ""),
		BillingManualSupportNote:      envOrDefault("OMNIDRIVE_BILLING_MANUAL_SUPPORT_NOTE", "请联系客服完成转账，并在订单内补充转账说明或凭证。"),
	}
}

// 处理env默认相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

// 处理env首个相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func envFirst(fallback string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return fallback
}

// 处理envAsInt相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func envAsInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

// 处理envAsBool相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func envAsBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

// 处理envAsCSV相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func envAsCSV(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		if len(fallback) == 0 {
			return nil
		}
		return append([]string(nil), fallback...)
	}

	items := make([]string, 0)
	for _, raw := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

// 处理默认跨域AllowedOrigins相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func defaultCORSAllowedOrigins(environment string) []string {
	if strings.EqualFold(strings.TrimSpace(environment), "development") {
		return []string{"*"}
	}
	return nil
}

// 处理默认LogLevel相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func defaultLogLevel(environment string) string {
	return "info"
}

// 根据mat计算默认Log，供配置链路复用关键派生结果。
func defaultLogFormat(environment string) string {
	_ = environment
	return "json"
}
