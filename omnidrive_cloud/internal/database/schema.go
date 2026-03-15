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

CREATE TABLE IF NOT EXISTS device_material_roots (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    root_name TEXT NOT NULL,
    root_path TEXT NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    is_directory BOOLEAN NOT NULL DEFAULT TRUE,
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(device_id, root_name)
);

CREATE TABLE IF NOT EXISTS device_material_entries (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    root_name TEXT NOT NULL,
    root_path TEXT NOT NULL,
    relative_path TEXT NOT NULL,
    parent_path TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    absolute_path TEXT,
    size_bytes BIGINT,
    modified_at TEXT,
    extension TEXT,
    mime_type TEXT,
    is_text BOOLEAN NOT NULL DEFAULT FALSE,
    preview_text TEXT,
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(device_id, root_name, relative_path)
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
    lease_owner_device_id TEXT REFERENCES devices(id) ON DELETE SET NULL,
    lease_token TEXT,
    lease_expires_at TIMESTAMPTZ,
    attempt_count INT NOT NULL DEFAULT 0,
    cancel_requested_at TIMESTAMPTZ,
    run_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS publish_task_events (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES publish_tasks(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    source TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS publish_task_artifacts (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES publish_tasks(id) ON DELETE CASCADE,
    artifact_key TEXT NOT NULL,
    artifact_type TEXT NOT NULL,
    source TEXT NOT NULL,
    title TEXT,
    file_name TEXT,
    mime_type TEXT,
    storage_key TEXT,
    public_url TEXT,
    size_bytes BIGINT,
    text_content TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(task_id, artifact_key)
);

CREATE TABLE IF NOT EXISTS publish_task_material_refs (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES publish_tasks(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    root_name TEXT NOT NULL,
    relative_path TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'media',
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    absolute_path TEXT,
    size_bytes BIGINT,
    modified_at TEXT,
    extension TEXT,
    mime_type TEXT,
    is_text BOOLEAN NOT NULL DEFAULT FALSE,
    preview_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(task_id, root_name, relative_path, role)
);

ALTER TABLE publish_tasks ADD COLUMN IF NOT EXISTS lease_owner_device_id TEXT REFERENCES devices(id) ON DELETE SET NULL;
ALTER TABLE publish_tasks ADD COLUMN IF NOT EXISTS lease_token TEXT;
ALTER TABLE publish_tasks ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ;
ALTER TABLE publish_tasks ADD COLUMN IF NOT EXISTS attempt_count INT NOT NULL DEFAULT 0;
ALTER TABLE publish_tasks ADD COLUMN IF NOT EXISTS cancel_requested_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS ai_models (
    id TEXT PRIMARY KEY,
    vendor TEXT NOT NULL,
    model_name TEXT NOT NULL,
    category TEXT NOT NULL,
    description TEXT,
    pricing_payload JSONB,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_jobs (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_id TEXT REFERENCES product_skills(id) ON DELETE SET NULL,
    job_type TEXT NOT NULL,
    model_name TEXT NOT NULL,
    prompt TEXT,
    status TEXT NOT NULL,
    input_payload JSONB,
    output_payload JSONB,
    message TEXT,
    cost_credits BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS skill_id TEXT REFERENCES product_skills(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS billing_packages (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    channel TEXT NOT NULL,
    price_cents BIGINT NOT NULL,
    credit_amount BIGINT NOT NULL,
    badge TEXT,
    description TEXT,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wallet_ledgers (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_type TEXT NOT NULL,
    amount_delta BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    description TEXT,
    reference_type TEXT,
    reference_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_events (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    action TEXT NOT NULL,
    title TEXT NOT NULL,
    source TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_devices_owner_user_id ON devices(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_platform_accounts_device_id ON platform_accounts(device_id);
CREATE INDEX IF NOT EXISTS idx_login_sessions_device_id ON login_sessions(device_id);
CREATE INDEX IF NOT EXISTS idx_login_sessions_user_id ON login_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_login_session_actions_session_id ON login_session_actions(session_id);
CREATE INDEX IF NOT EXISTS idx_product_skills_owner_user_id ON product_skills(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_product_skill_assets_skill_id ON product_skill_assets(skill_id);
CREATE INDEX IF NOT EXISTS idx_device_material_roots_device_id ON device_material_roots(device_id);
CREATE INDEX IF NOT EXISTS idx_device_material_entries_device_id ON device_material_entries(device_id);
CREATE INDEX IF NOT EXISTS idx_device_material_entries_parent_path ON device_material_entries(device_id, root_name, parent_path);
CREATE INDEX IF NOT EXISTS idx_publish_tasks_device_id ON publish_tasks(device_id);
CREATE INDEX IF NOT EXISTS idx_publish_tasks_lease_owner_device_id ON publish_tasks(lease_owner_device_id);
CREATE INDEX IF NOT EXISTS idx_publish_tasks_lease_expires_at ON publish_tasks(lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_publish_tasks_status_platform ON publish_tasks(status, platform);
CREATE INDEX IF NOT EXISTS idx_publish_task_events_task_id ON publish_task_events(task_id);
CREATE INDEX IF NOT EXISTS idx_publish_task_artifacts_task_id ON publish_task_artifacts(task_id);
CREATE INDEX IF NOT EXISTS idx_publish_task_material_refs_task_id ON publish_task_material_refs(task_id);
CREATE INDEX IF NOT EXISTS idx_ai_models_category ON ai_models(category);
CREATE INDEX IF NOT EXISTS idx_ai_jobs_owner_user_id ON ai_jobs(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_ai_jobs_job_type ON ai_jobs(job_type);
CREATE INDEX IF NOT EXISTS idx_wallet_ledgers_user_id ON wallet_ledgers(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_owner_user_id ON audit_events(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_resource_type ON audit_events(resource_type);

INSERT INTO ai_models (id, vendor, model_name, category, description, pricing_payload, is_enabled)
VALUES
    ('gemini-3.1-pro-preview', 'apiyi', 'gemini-3.1-pro-preview', 'chat', '默认思考与多模态理解模型', '{"unit":"credits","price":"dynamic"}', TRUE),
    ('gemini-3-pro-image-preview', 'apiyi', 'gemini-3-pro-image-preview', 'image', '默认图片生成与编辑模型', '{"unit":"credits","price":"dynamic"}', TRUE),
    ('veo-3.1-fast-fl', 'apiyi', 'veo-3.1-fast-fl', 'video', '默认视频生成模型', '{"unit":"credits","price":"dynamic"}', TRUE)
ON CONFLICT (id) DO NOTHING;

INSERT INTO billing_packages (id, name, channel, price_cents, credit_amount, badge, description, is_enabled, sort_order)
VALUES
    ('starter', '入门包', 'alipay,wechat,manual', 9900, 1000, 'Starter', '适合轻量创作和初次体验', TRUE, 10),
    ('growth', '增长包', 'alipay,wechat,manual', 29900, 3500, 'Popular', '适合稳定做图做视频和日常运营', TRUE, 20),
    ('studio', '工作室包', 'alipay,wechat,manual', 69900, 9000, 'Studio', '适合内容工作室和多技能协同', TRUE, 30),
    ('enterprise', '企业包', 'alipay,wechat,manual', 149900, 22000, 'Enterprise', '适合设备多、任务多的运营场景', TRUE, 40)
ON CONFLICT (id) DO NOTHING;
`

func (db *Database) EnsureSchema(ctx context.Context) error {
	if _, err := db.Pool.Exec(ctx, bootstrapSQL); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	return nil
}
