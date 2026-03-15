from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    app_name: str = Field(default="OmniDrive API", alias="OMNIDRIVE_APP_NAME")
    environment: str = Field(default="development", alias="OMNIDRIVE_ENV")
    api_v1_prefix: str = Field(default="/api/v1", alias="OMNIDRIVE_API_V1_PREFIX")
    database_url: str = Field(alias="OMNIDRIVE_DATABASE_URL")
    redis_url: str = Field(default="redis://127.0.0.1:6379/0", alias="OMNIDRIVE_REDIS_URL")
    s3_endpoint_url: str | None = Field(default=None, alias="OMNIDRIVE_S3_ENDPOINT_URL")
    s3_bucket: str | None = Field(default=None, alias="OMNIDRIVE_S3_BUCKET")
    s3_access_key: str | None = Field(default=None, alias="OMNIDRIVE_S3_ACCESS_KEY")
    s3_secret_key: str | None = Field(default=None, alias="OMNIDRIVE_S3_SECRET_KEY")
    jwt_secret: str = Field(default="change-me", alias="OMNIDRIVE_JWT_SECRET")
    access_token_expire_minutes: int = Field(default=720, alias="OMNIDRIVE_ACCESS_TOKEN_EXPIRE_MINUTES")
    auto_create_tables: bool = Field(default=True, alias="OMNIDRIVE_AUTO_CREATE_TABLES")


settings = Settings()

