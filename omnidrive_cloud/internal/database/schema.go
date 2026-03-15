package database

import (
	"context"
	"fmt"
)

const bootstrapSQL = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS devices (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    device_code TEXT NOT NULL UNIQUE,
    agent_key TEXT,
    name TEXT NOT NULL,
    local_ip TEXT,
    public_ip TEXT,
    default_reasoning_model TEXT,
    is_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    runtime_payload JSONB,
    last_seen_at TIMESTAMPTZ,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform_accounts (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    account_name TEXT NOT NULL,
    status TEXT NOT NULL,
    last_message TEXT,
    last_authenticated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(device_id, platform, account_name)
);

CREATE TABLE IF NOT EXISTS login_sessions (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    account_name TEXT NOT NULL,
    status TEXT NOT NULL,
    qr_data TEXT,
    verification_payload JSONB,
    message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS login_session_actions (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES login_sessions(id) ON DELETE CASCADE,
    action_type TEXT NOT NULL,
    payload JSONB,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    consumed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS product_skills (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    output_type TEXT NOT NULL,
    model_name TEXT NOT NULL,
    prompt_template TEXT,
    reference_payload JSONB,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS product_skill_assets (
    id TEXT PRIMARY KEY,
    skill_id TEXT NOT NULL REFERENCES product_skills(id) ON DELETE CASCADE,
    owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    asset_type TEXT NOT NULL,
    file_name TEXT NOT NULL,
    mime_type TEXT,
    storage_key TEXT,
    public_url TEXT,
    size_bytes BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS publish_tasks (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    account_id TEXT REFERENCES platform_accounts(id) ON DELETE SET NULL,
    skill_id TEXT REFERENCES product_skills(id) ON DELETE SET NULL,
    platform TEXT NOT NULL,
    account_name TEXT NOT NULL,
    title TEXT NOT NULL,
    content_text TEXT,
    media_payload JSONB,
    status TEXT NOT NULL,
    message TEXT,
    verification_payload JSONB,
    run_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_devices_owner_user_id ON devices(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_platform_accounts_device_id ON platform_accounts(device_id);
CREATE INDEX IF NOT EXISTS idx_login_sessions_device_id ON login_sessions(device_id);
CREATE INDEX IF NOT EXISTS idx_login_sessions_user_id ON login_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_login_session_actions_session_id ON login_session_actions(session_id);
CREATE INDEX IF NOT EXISTS idx_product_skills_owner_user_id ON product_skills(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_product_skill_assets_skill_id ON product_skill_assets(skill_id);
CREATE INDEX IF NOT EXISTS idx_publish_tasks_device_id ON publish_tasks(device_id);
`

func (db *Database) EnsureSchema(ctx context.Context) error {
	if _, err := db.Pool.Exec(ctx, bootstrapSQL); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	return nil
}
