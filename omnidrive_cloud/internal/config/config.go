package config

import (
	"os"
	"strconv"

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
		S3Endpoint:               envOrDefault("OMNIDRIVE_S3_ENDPOINT", ""),
		S3Bucket:                 envOrDefault("OMNIDRIVE_S3_BUCKET", ""),
		S3AccessKey:              envOrDefault("OMNIDRIVE_S3_ACCESS_KEY", ""),
		S3SecretKey:              envOrDefault("OMNIDRIVE_S3_SECRET_KEY", ""),
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
