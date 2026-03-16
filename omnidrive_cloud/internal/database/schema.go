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

CREATE TABLE IF NOT EXISTS device_skill_sync_states (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    skill_id TEXT NOT NULL REFERENCES product_skills(id) ON DELETE CASCADE,
    sync_status TEXT NOT NULL DEFAULT 'pending',
    synced_revision TEXT,
    asset_count BIGINT NOT NULL DEFAULT 0,
    message TEXT,
    last_synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(device_id, skill_id)
);

CREATE TABLE IF NOT EXISTS device_retired_skill_acks (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    skill_id TEXT NOT NULL,
    reason TEXT NOT NULL,
    message TEXT,
    last_acknowledged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(device_id, skill_id, reason)
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
    skill_revision TEXT,
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

CREATE TABLE IF NOT EXISTS publish_task_runtime_states (
    task_id TEXT PRIMARY KEY REFERENCES publish_tasks(id) ON DELETE CASCADE,
    execution_payload JSONB,
    last_agent_sync_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
ALTER TABLE publish_tasks ADD COLUMN IF NOT EXISTS skill_revision TEXT;

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
    device_id TEXT REFERENCES devices(id) ON DELETE CASCADE,
    skill_id TEXT REFERENCES product_skills(id) ON DELETE SET NULL,
    source TEXT NOT NULL DEFAULT 'omnidrive_cloud',
    local_task_id TEXT,
    job_type TEXT NOT NULL,
    model_name TEXT NOT NULL,
    prompt TEXT,
    status TEXT NOT NULL,
    input_payload JSONB,
    output_payload JSONB,
    message TEXT,
    cost_credits BIGINT NOT NULL DEFAULT 0,
    lease_owner_device_id TEXT REFERENCES devices(id) ON DELETE SET NULL,
    lease_token TEXT,
    lease_expires_at TIMESTAMPTZ,
    delivery_status TEXT NOT NULL DEFAULT 'pending',
    delivery_message TEXT,
    local_publish_task_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS device_id TEXT REFERENCES devices(id) ON DELETE CASCADE;
ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS skill_id TEXT REFERENCES product_skills(id) ON DELETE SET NULL;
ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'omnidrive_cloud';
ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS local_task_id TEXT;
ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS lease_owner_device_id TEXT REFERENCES devices(id) ON DELETE SET NULL;
ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS lease_token TEXT;
ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ;
ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS delivery_status TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS delivery_message TEXT;
ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS local_publish_task_id TEXT;
ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS ai_job_artifacts (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL REFERENCES ai_jobs(id) ON DELETE CASCADE,
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
    device_id TEXT REFERENCES devices(id) ON DELETE SET NULL,
    root_name TEXT,
    relative_path TEXT,
    absolute_path TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (job_id, artifact_key)
);

CREATE TABLE IF NOT EXISTS ai_job_publish_links (
    job_id TEXT NOT NULL REFERENCES ai_jobs(id) ON DELETE CASCADE,
    task_id TEXT NOT NULL REFERENCES publish_tasks(id) ON DELETE CASCADE,
    owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (job_id, task_id)
);

CREATE TABLE IF NOT EXISTS billing_packages (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    package_type TEXT NOT NULL DEFAULT 'credit_topup',
    channel TEXT NOT NULL,
    payment_channels JSONB NOT NULL DEFAULT '["manual_cs","alipay","wechatpay"]'::jsonb,
    currency TEXT NOT NULL DEFAULT 'CNY',
    price_cents BIGINT NOT NULL,
    credit_amount BIGINT NOT NULL,
    badge TEXT,
    description TEXT,
    pricing_payload JSONB,
    expires_in_days INT,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE billing_packages ADD COLUMN IF NOT EXISTS package_type TEXT NOT NULL DEFAULT 'credit_topup';
ALTER TABLE billing_packages ADD COLUMN IF NOT EXISTS payment_channels JSONB NOT NULL DEFAULT '["manual_cs","alipay","wechatpay"]'::jsonb;
ALTER TABLE billing_packages ADD COLUMN IF NOT EXISTS currency TEXT NOT NULL DEFAULT 'CNY';
ALTER TABLE billing_packages ADD COLUMN IF NOT EXISTS pricing_payload JSONB;
ALTER TABLE billing_packages ADD COLUMN IF NOT EXISTS expires_in_days INT;

CREATE TABLE IF NOT EXISTS billing_meters (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    unit TEXT NOT NULL,
    description TEXT,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS billing_package_entitlements (
    id TEXT PRIMARY KEY,
    package_id TEXT NOT NULL REFERENCES billing_packages(id) ON DELETE CASCADE,
    meter_code TEXT NOT NULL REFERENCES billing_meters(code) ON DELETE CASCADE,
    grant_amount BIGINT NOT NULL,
    grant_mode TEXT NOT NULL DEFAULT 'one_time',
    sort_order INT NOT NULL DEFAULT 0,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(package_id, meter_code)
);

CREATE TABLE IF NOT EXISTS billing_pricing_rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    meter_code TEXT NOT NULL REFERENCES billing_meters(code) ON DELETE CASCADE,
    applies_to TEXT NOT NULL DEFAULT 'global',
    model_name TEXT,
    job_type TEXT,
    charge_mode TEXT NOT NULL DEFAULT 'wallet_only',
    quota_meter_code TEXT REFERENCES billing_meters(code) ON DELETE SET NULL,
    unit_size BIGINT NOT NULL DEFAULT 1,
    wallet_debit_amount BIGINT NOT NULL DEFAULT 0,
    sort_order INT NOT NULL DEFAULT 0,
    description TEXT,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS billing_wallets (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    credit_balance BIGINT NOT NULL DEFAULT 0,
    frozen_credit_balance BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wallet_ledgers (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_type TEXT NOT NULL,
    amount_delta BIGINT NOT NULL,
    balance_before BIGINT NOT NULL DEFAULT 0,
    balance_after BIGINT NOT NULL,
    meter_code TEXT,
    quantity BIGINT,
    unit TEXT,
    unit_price_credits BIGINT,
    description TEXT,
    reference_type TEXT,
    reference_id TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE wallet_ledgers ADD COLUMN IF NOT EXISTS balance_before BIGINT NOT NULL DEFAULT 0;
ALTER TABLE wallet_ledgers ADD COLUMN IF NOT EXISTS meter_code TEXT;
ALTER TABLE wallet_ledgers ADD COLUMN IF NOT EXISTS quantity BIGINT;
ALTER TABLE wallet_ledgers ADD COLUMN IF NOT EXISTS unit TEXT;
ALTER TABLE wallet_ledgers ADD COLUMN IF NOT EXISTS unit_price_credits BIGINT;
ALTER TABLE wallet_ledgers ADD COLUMN IF NOT EXISTS metadata JSONB;

CREATE TABLE IF NOT EXISTS recharge_orders (
    id TEXT PRIMARY KEY,
    order_no TEXT NOT NULL UNIQUE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    package_id TEXT REFERENCES billing_packages(id) ON DELETE SET NULL,
    package_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    channel TEXT NOT NULL,
    status TEXT NOT NULL,
    subject TEXT NOT NULL,
    body TEXT,
    currency TEXT NOT NULL DEFAULT 'CNY',
    amount_cents BIGINT NOT NULL,
    credit_amount BIGINT NOT NULL DEFAULT 0,
    payment_payload JSONB,
    customer_service_payload JSONB,
    provider_transaction_id TEXT,
    provider_status TEXT,
    expires_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payment_transactions (
    id TEXT PRIMARY KEY,
    recharge_order_id TEXT NOT NULL REFERENCES recharge_orders(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel TEXT NOT NULL,
    transaction_kind TEXT NOT NULL,
    out_trade_no TEXT NOT NULL,
    provider_transaction_id TEXT,
    status TEXT NOT NULL,
    request_payload JSONB,
    response_payload JSONB,
    notify_payload JSONB,
    error_message TEXT,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(channel, out_trade_no)
);

ALTER TABLE wallet_ledgers ADD COLUMN IF NOT EXISTS recharge_order_id TEXT REFERENCES recharge_orders(id) ON DELETE SET NULL;
ALTER TABLE wallet_ledgers ADD COLUMN IF NOT EXISTS payment_transaction_id TEXT REFERENCES payment_transactions(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS billing_quota_accounts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    meter_code TEXT NOT NULL REFERENCES billing_meters(code) ON DELETE CASCADE,
    granted_total BIGINT NOT NULL DEFAULT 0,
    used_total BIGINT NOT NULL DEFAULT 0,
    reserved_total BIGINT NOT NULL DEFAULT 0,
    remaining_total BIGINT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    source_type TEXT,
    source_id TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS billing_quota_ledgers (
    id TEXT PRIMARY KEY,
    quota_account_id TEXT REFERENCES billing_quota_accounts(id) ON DELETE SET NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    meter_code TEXT NOT NULL REFERENCES billing_meters(code) ON DELETE CASCADE,
    amount_delta BIGINT NOT NULL,
    remaining_after BIGINT NOT NULL,
    description TEXT,
    reference_type TEXT,
    reference_id TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS billing_usage_events (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,
    source_id TEXT,
    meter_code TEXT NOT NULL REFERENCES billing_meters(code) ON DELETE CASCADE,
    model_name TEXT,
    job_type TEXT,
    usage_quantity BIGINT NOT NULL,
    pricing_rule_id TEXT REFERENCES billing_pricing_rules(id) ON DELETE SET NULL,
    quota_account_id TEXT REFERENCES billing_quota_accounts(id) ON DELETE SET NULL,
    wallet_ledger_id TEXT REFERENCES wallet_ledgers(id) ON DELETE SET NULL,
    quota_ledger_id TEXT REFERENCES billing_quota_ledgers(id) ON DELETE SET NULL,
    bill_status TEXT NOT NULL DEFAULT 'pending',
    bill_message TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
CREATE INDEX IF NOT EXISTS idx_device_skill_sync_states_device_id ON device_skill_sync_states(device_id);
CREATE INDEX IF NOT EXISTS idx_device_skill_sync_states_skill_id ON device_skill_sync_states(skill_id);
CREATE INDEX IF NOT EXISTS idx_device_retired_skill_acks_device_id ON device_retired_skill_acks(device_id);
CREATE INDEX IF NOT EXISTS idx_device_material_roots_device_id ON device_material_roots(device_id);
CREATE INDEX IF NOT EXISTS idx_device_material_entries_device_id ON device_material_entries(device_id);
CREATE INDEX IF NOT EXISTS idx_device_material_entries_parent_path ON device_material_entries(device_id, root_name, parent_path);
CREATE INDEX IF NOT EXISTS idx_publish_tasks_device_id ON publish_tasks(device_id);
CREATE INDEX IF NOT EXISTS idx_publish_tasks_lease_owner_device_id ON publish_tasks(lease_owner_device_id);
CREATE INDEX IF NOT EXISTS idx_publish_tasks_lease_expires_at ON publish_tasks(lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_publish_tasks_status_platform ON publish_tasks(status, platform);
CREATE INDEX IF NOT EXISTS idx_publish_task_events_task_id ON publish_task_events(task_id);
CREATE INDEX IF NOT EXISTS idx_publish_task_artifacts_task_id ON publish_task_artifacts(task_id);
CREATE INDEX IF NOT EXISTS idx_publish_task_runtime_states_last_agent_sync_at ON publish_task_runtime_states(last_agent_sync_at);
CREATE INDEX IF NOT EXISTS idx_publish_task_material_refs_task_id ON publish_task_material_refs(task_id);
CREATE INDEX IF NOT EXISTS idx_ai_models_category ON ai_models(category);
CREATE INDEX IF NOT EXISTS idx_ai_jobs_owner_user_id ON ai_jobs(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_ai_jobs_job_type ON ai_jobs(job_type);
CREATE INDEX IF NOT EXISTS idx_ai_jobs_device_id ON ai_jobs(device_id);
CREATE INDEX IF NOT EXISTS idx_ai_jobs_source ON ai_jobs(source);
CREATE INDEX IF NOT EXISTS idx_ai_jobs_lease_expires_at ON ai_jobs(lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_ai_job_artifacts_job_id ON ai_job_artifacts(job_id);
CREATE INDEX IF NOT EXISTS idx_ai_job_publish_links_job_id ON ai_job_publish_links(job_id);
CREATE INDEX IF NOT EXISTS idx_billing_package_entitlements_package_id ON billing_package_entitlements(package_id);
CREATE INDEX IF NOT EXISTS idx_billing_pricing_rules_meter_code ON billing_pricing_rules(meter_code);
CREATE INDEX IF NOT EXISTS idx_billing_pricing_rules_model_name ON billing_pricing_rules(model_name);
CREATE INDEX IF NOT EXISTS idx_billing_wallets_user_id ON billing_wallets(user_id);
CREATE INDEX IF NOT EXISTS idx_wallet_ledgers_user_id ON wallet_ledgers(user_id);
CREATE INDEX IF NOT EXISTS idx_recharge_orders_user_id ON recharge_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_recharge_orders_status ON recharge_orders(status);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_recharge_order_id ON payment_transactions(recharge_order_id);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_user_id ON payment_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_billing_quota_accounts_user_id ON billing_quota_accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_billing_quota_accounts_meter_code ON billing_quota_accounts(meter_code);
CREATE INDEX IF NOT EXISTS idx_billing_quota_ledgers_user_id ON billing_quota_ledgers(user_id);
CREATE INDEX IF NOT EXISTS idx_billing_usage_events_user_id ON billing_usage_events(user_id);
CREATE INDEX IF NOT EXISTS idx_billing_usage_events_source ON billing_usage_events(source_type, source_id);
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

INSERT INTO billing_meters (code, name, category, unit, description, is_enabled)
VALUES
    ('wallet_credit', '钱包积分', 'wallet', 'credit', '统一钱包积分，可用于模型调用和视频生成扣费', TRUE),
    ('chat_input_tokens', '聊天输入 Token', 'usage', 'token', '聊天类模型输入 token 用量', TRUE),
    ('chat_output_tokens', '聊天输出 Token', 'usage', 'token', '聊天类模型输出 token 用量', TRUE),
    ('image_generations', '图片生成次数', 'usage', 'job', '图片生成作业次数', TRUE),
    ('video_generations', '视频生成次数', 'usage', 'job', '视频生成作业次数', TRUE),
    ('image_generation_quota', '图片生成套餐次数', 'quota', 'job', '套餐内可抵扣的图片生成次数', TRUE),
    ('video_generation_quota', '视频生成套餐次数', 'quota', 'job', '套餐内可抵扣的视频生成次数', TRUE)
ON CONFLICT (code) DO NOTHING;

INSERT INTO billing_package_entitlements (id, package_id, meter_code, grant_amount, grant_mode, sort_order, description)
VALUES
    ('starter-wallet-credit', 'starter', 'wallet_credit', 1000, 'one_time', 10, '购买入门包后发放 1000 钱包积分'),
    ('growth-wallet-credit', 'growth', 'wallet_credit', 3500, 'one_time', 10, '购买增长包后发放 3500 钱包积分'),
    ('studio-wallet-credit', 'studio', 'wallet_credit', 9000, 'one_time', 10, '购买工作室包后发放 9000 钱包积分'),
    ('enterprise-wallet-credit', 'enterprise', 'wallet_credit', 22000, 'one_time', 10, '购买企业包后发放 22000 钱包积分')
ON CONFLICT (id) DO NOTHING;

INSERT INTO billing_pricing_rules (
    id, name, meter_code, applies_to, model_name, job_type, charge_mode, quota_meter_code,
    unit_size, wallet_debit_amount, sort_order, description, is_enabled
)
VALUES
    (
        'rule-gemini-chat-input',
        'Gemini 聊天输入计费',
        'chat_input_tokens',
        'model',
        'gemini-3.1-pro-preview',
        'chat',
        'wallet_only',
        NULL,
        1000,
        1,
        10,
        '默认占位规则：每 1000 个输入 token 扣 1 钱包积分，后续可在后台调整',
        TRUE
    ),
    (
        'rule-gemini-chat-output',
        'Gemini 聊天输出计费',
        'chat_output_tokens',
        'model',
        'gemini-3.1-pro-preview',
        'chat',
        'wallet_only',
        NULL,
        1000,
        2,
        20,
        '默认占位规则：每 1000 个输出 token 扣 2 钱包积分，后续可在后台调整',
        TRUE
    ),
    (
        'rule-image-generation-default',
        '图片生成计费',
        'image_generations',
        'model',
        'gemini-3-pro-image-preview',
        'image',
        'quota_first_wallet_fallback',
        'image_generation_quota',
        1,
        80,
        30,
        '默认占位规则：优先扣套餐图片次数，不足时每次扣 80 钱包积分',
        TRUE
    ),
    (
        'rule-video-generation-default',
        '视频生成计费',
        'video_generations',
        'model',
        'veo-3.1-fast-fl',
        'video',
        'quota_first_wallet_fallback',
        'video_generation_quota',
        1,
        400,
        40,
        '默认占位规则：优先扣套餐视频次数，不足时每次扣 400 钱包积分',
        TRUE
    )
ON CONFLICT (id) DO NOTHING;
`

func (db *Database) EnsureSchema(ctx context.Context) error {
	if _, err := db.Pool.Exec(ctx, bootstrapSQL); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	return nil
}
