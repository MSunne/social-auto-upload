package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment              string
	BindAddr                 string
	DatabaseDSN              string
	RedisAddr                string
	PublicBaseURL            string
	LocalStorageDir          string
	S3Endpoint               string
	S3Bucket                 string
	S3AccessKey              string
	S3SecretKey              string
	S3PublicBaseURL          string
	S3ImageStorePath         string
	S3VideoStorePath         string
	APIYIBaseURL             string
	APIYIApiKey              string
	AIWorkerEnabled          bool
	AIWorkerPollSeconds      int
	AIWorkerConcurrency      int
	AIVideoPollSeconds       int
	AIVideoTimeoutSeconds    int
	JWTSecret                string
	AccessTokenExpireMinutes int
	AutoCreateSchema         bool
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		Environment:              envOrDefault("OMNIDRIVE_ENV", "development"),
		BindAddr:                 envOrDefault("OMNIDRIVE_BIND_ADDR", ":8410"),
		DatabaseDSN:              envOrDefault("OMNIDRIVE_DATABASE_DSN", ""),
		RedisAddr:                envOrDefault("OMNIDRIVE_REDIS_ADDR", ""),
		PublicBaseURL:            envOrDefault("OMNIDRIVE_PUBLIC_BASE_URL", ""),
		LocalStorageDir:          envOrDefault("OMNIDRIVE_LOCAL_STORAGE_DIR", "./data"),
		S3Endpoint:               envFirst("", "OMNIDRIVE_S3_ENDPOINT", "S3_ENDPOINT_URL"),
		S3Bucket:                 envFirst("", "OMNIDRIVE_S3_BUCKET", "S3_BUCKET_NAME"),
		S3AccessKey:              envFirst("", "OMNIDRIVE_S3_ACCESS_KEY", "S3_ACCESS_KEY_ID"),
		S3SecretKey:              envFirst("", "OMNIDRIVE_S3_SECRET_KEY", "S3_SECRET_ACCESS_KEY"),
		S3PublicBaseURL:          envFirst("", "OMNIDRIVE_S3_PUBLIC_BASE_URL", "S3_PRIVATE_URL"),
		S3ImageStorePath:         envFirst("", "OMNIDRIVE_S3_IMAGE_STORE_PATH", "IMAGE_STORE_PATH"),
		S3VideoStorePath:         envFirst("", "OMNIDRIVE_S3_VIDEO_STORE_PATH", "VIDEO_STORE_PATH"),
		APIYIBaseURL:             envOrDefault("OMNIDRIVE_APIYI_BASE_URL", "https://api.apiyi.com"),
		APIYIApiKey:              envFirst("", "OMNIDRIVE_APIYI_API_KEY", "APIYI_API_KEY"),
		AIWorkerEnabled:          envAsBool("OMNIDRIVE_AI_WORKER_ENABLED", true),
		AIWorkerPollSeconds:      envAsInt("OMNIDRIVE_AI_WORKER_POLL_SECONDS", 5),
		AIWorkerConcurrency:      envAsInt("OMNIDRIVE_AI_WORKER_CONCURRENCY", 2),
		AIVideoPollSeconds:       envAsInt("OMNIDRIVE_AI_VIDEO_POLL_SECONDS", 6),
		AIVideoTimeoutSeconds:    envAsInt("OMNIDRIVE_AI_VIDEO_TIMEOUT_SECONDS", 600),
		JWTSecret:                envOrDefault("OMNIDRIVE_JWT_SECRET", "change-me"),
		AccessTokenExpireMinutes: envAsInt("OMNIDRIVE_ACCESS_TOKEN_EXPIRE_MINUTES", 720),
		AutoCreateSchema:         envAsBool("OMNIDRIVE_AUTO_CREATE_SCHEMA", true),
	}
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envFirst(fallback string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return fallback
}

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
