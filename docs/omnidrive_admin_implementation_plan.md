# OmniDriveAdmin Implementation Plan

## Goal

Build `OmniDriveAdmin` as the internal operations and finance console for the OmniDrive ecosystem.

It is not the same product as:

- `omnidrive_frontend`: customer-facing cloud console
- `sau_frontend`: local OmniBull / SAU console

`OmniDriveAdmin` is for internal roles:

- super admin
- operations
- customer support
- finance
- audit / compliance
- engineering support

The admin backend should live in the existing Go service `omnidrive_cloud`, exposed under a separate route group:

- `/api/admin/v1/*`

This keeps all domain data in one service, but separates:

- customer APIs
- agent APIs
- admin APIs

## Core Principles

1. All financial mutations must be ledger-based.
2. All review flows must be ticket-based.
3. All admin actions must be auditable.
4. All settlement flows must be idempotent.
5. Admin permissions must be role-based, not hardcoded.
6. Customer-facing data and admin-only data must be clearly separated.

## Admin Scope

### 1. Admin Auth And RBAC

Required capabilities:

- admin login
- admin user management
- role management
- permission matrix
- token/session invalidation
- operation audit log

Recommended backend entities:

- `admin_users`
- `admin_roles`
- `admin_permissions`
- `admin_role_permissions`
- `admin_user_roles`
- `admin_sessions`
- `admin_audit_logs`

###+ Suggested Permissions

- `user.read`
- `user.update`
- `user.freeze`
- `device.read`
- `device.update`
- `task.read`
- `task.operate`
- `finance.read`
- `finance.adjust`
- `support_recharge.review`
- `distribution.read`
- `distribution.settle`
- `withdrawal.review`
- `system.config`
- `admin.manage`

### 2. User Management

Required capabilities:

- list/search/filter users
- view user profile and current status
- inspect wallet, recharge, consume, refund totals
- inspect related devices, accounts, tasks, AI jobs
- freeze user
- disable recharge
- disable withdrawal
- add internal notes and risk tags

Recommended additions:

- `user_internal_notes`
- `user_risk_flags`

### 3. Device Management

Required capabilities:

- list all OmniBull devices
- inspect heartbeat and online status
- inspect runtime payload
- inspect related tasks and accounts
- claim / unbind support actions
- enable / disable device
- force release stuck task lease
- inspect skill sync drift

This mostly reuses current domain data and admin endpoints will aggregate it.

### 4. Media Account And Publish Task Management

Required capabilities:

- list platform accounts across all users
- inspect login sessions and verification history
- inspect publish tasks globally
- view task evidence, screenshot, logs, materials, runtime
- cancel / retry / force release / manual resolve
- filter by platform, status, user, device, account

This can reuse the current task/account domain but needs admin-level list and detail endpoints.

### 5. AI Job Management

Required capabilities:

- list all AI jobs
- inspect model, source, owner, cost, output artifacts
- cancel / retry failed jobs
- inspect provider errors
- inspect S3 artifact storage state

This also depends on the separate AI executor work becoming real.

### 6. Skill And Model Management

Required capabilities:

- inspect all user skills
- inspect asset files and storage keys
- disable bad or risky skills
- inspect model registry
- enable / disable model
- adjust default model selection
- manage provider configuration visibility and health state

Recommended additions:

- `provider_configs`
- `provider_health_checks`
- `model_cost_rules`

### 7. Recharge, Orders, Wallet, Finance

Required capabilities:

- package management
- payment order management
- wallet ledger inspection
- compensation / manual adjustment flow
- refund support
- reconciliation views

Recommended backend entities:

- `payment_orders`
- `payment_callbacks`
- `wallet_adjustment_requests`
- `finance_reconciliation_runs`

### 8. Support Recharge Review

This is a first-class admin module.

Business rules:

- supported channels: Alipay, WeChat, Support
- support recharge can include bonus credits
- support recharge must be reviewed before crediting user wallet
- every review must be auditable
- duplicate crediting must be prevented

Recommended backend entities:

- `support_recharge_requests`
- `support_recharge_reviews`
- `support_recharge_attachments`

Suggested status flow:

- `pending_review`
- `approved`
- `rejected`
- `credited`
- `cancelled`

Required fields:

- target user
- amount
- base credits
- bonus credits
- payment proof
- submitter
- reviewer
- review note
- credited ledger id

### 9. Distribution / Referral / Commission

This is another first-class admin module.

Business rules from current requirements:

- promoter receives a percentage of invited user recharge amount
- commission is not instantly available
- invited user recharge creates a commission record in `pending_consume`
- only user consumption releases commission into `pending_settlement`
- settlement moves commission into `settled`
- settled amount may later be withdrawn or already paid manually

Recommended backend entities:

- `distributor_profiles`
- `referral_relations`
- `commission_rules`
- `commission_ledger`
- `commission_settlement_batches`
- `commission_settlement_items`
- `withdrawal_requests`
- `withdrawal_reviews`

Commission ledger statuses:

- `pending_consume`
- `pending_settlement`
- `settled`
- `voided`

Required admin capabilities:

- manage referral relation validity
- inspect downstream invited users
- inspect recharge-triggered commission rows
- inspect consumption-triggered released commission
- inspect settlement batches
- inspect withdrawal status

### 10. Withdrawal And Settlement

If promoters can withdraw cash, admin must support:

- list withdrawal requests
- review approve / reject
- mark paid
- attach payment proof
- handle manual offline settlement

If the product starts with manual settlement only, keep the same module but limit payout modes to:

- offline bank transfer
- manual Alipay
- manual WeChat

### 11. Audit And Operations

Required capabilities:

- inspect admin audit logs
- inspect support recharge reviews
- inspect commission settlement actions
- inspect wallet adjustments
- inspect risky mutations

All admin write endpoints must log:

- operator
- resource type
- resource id
- before snapshot
- after snapshot
- action
- reason
- request id

## Delivery Phases

### Phase 0. Foundation

Goal:

- create admin route group
- create admin auth and RBAC
- create admin audit log framework
- align storage to real S3 usage
- align environment variable strategy

Deliverables:

- `/api/admin/v1/auth/login`
- `/api/admin/v1/me`
- `/api/admin/v1/admin-users`
- `/api/admin/v1/roles`
- admin JWT middleware
- permission middleware
- audit logging helper
- S3 storage implementation replacing local-only object storage

Blockers removed:

- finance and settlement flows can now be safely implemented

### Phase 1. Finance Core

Goal:

- build ledger-safe finance base

Deliverables:

- payment orders
- wallet ledger admin views
- recharge package management
- support recharge request + review flow
- manual adjustment flow

Admin route groups:

- `/api/admin/v1/packages`
- `/api/admin/v1/orders`
- `/api/admin/v1/wallet-ledgers`
- `/api/admin/v1/support-recharges`

### Phase 2. Distribution And Settlement

Goal:

- implement distributor, commission, settlement, withdrawal

Deliverables:

- referral relation management
- commission rule management
- commission ledger generation hooks
- settlement batch flow
- withdrawal review flow

Admin route groups:

- `/api/admin/v1/distribution/relations`
- `/api/admin/v1/distribution/rules`
- `/api/admin/v1/distribution/commissions`
- `/api/admin/v1/distribution/settlements`
- `/api/admin/v1/withdrawals`

### Phase 3. Operations Domain

Goal:

- build user, device, account, task, AI admin views and controls

Deliverables:

- admin user list/detail
- admin device list/detail
- admin account list/detail
- admin publish task center
- admin AI job center

Admin route groups:

- `/api/admin/v1/users`
- `/api/admin/v1/devices`
- `/api/admin/v1/accounts`
- `/api/admin/v1/publish-tasks`
- `/api/admin/v1/ai-jobs`

### Phase 4. Model, Skill, And Provider Ops

Goal:

- give internal team model and skill governance

Deliverables:

- skill moderation endpoints
- model registry management
- provider health views
- system config center

Admin route groups:

- `/api/admin/v1/skills`
- `/api/admin/v1/models`
- `/api/admin/v1/providers`
- `/api/admin/v1/system-config`

### Phase 5. Reporting And Reconciliation

Goal:

- support finance and management reporting

Deliverables:

- daily revenue dashboard
- commission dashboard
- support recharge dashboard
- reconciliation jobs and reports
- export APIs

## Backend Work Breakdown

### A. New Route Group

- add admin router group in `omnidrive_cloud/internal/http/router.go`
- add admin auth middleware
- add role/permission middleware

### B. New Domain Models

- admin identity models
- finance order models
- support recharge models
- distribution and settlement models
- withdrawal models

### C. New Store Layer

- `internal/store/admin_users.go`
- `internal/store/admin_roles.go`
- `internal/store/payment_orders.go`
- `internal/store/support_recharges.go`
- `internal/store/distribution.go`
- `internal/store/withdrawals.go`

### D. New Handlers

- `internal/http/handlers/admin_auth.go`
- `internal/http/handlers/admin_users.go`
- `internal/http/handlers/admin_finance.go`
- `internal/http/handlers/admin_support_recharges.go`
- `internal/http/handlers/admin_distribution.go`
- `internal/http/handlers/admin_withdrawals.go`

### E. Database Schema Expansion

Add tables in the bootstrap schema first, then migrate to explicit migration files.

Priority additions:

- admin identity tables
- support recharge tables
- payment order tables
- commission tables
- settlement tables
- withdrawal tables

### F. Event And Ledger Hooks

Need hook points for:

- payment success -> wallet credit
- wallet consume -> commission release
- support recharge approve -> wallet credit
- settlement complete -> commission status update
- withdrawal pay -> withdrawal status update

These must be idempotent.

## Milestone Plan

### Milestone 1

- admin auth
- RBAC
- audit log
- S3 storage

### Milestone 2

- packages
- orders
- wallet ledgers
- support recharge review

### Milestone 3

- distribution relations
- commission ledger
- settlements
- withdrawals

### Milestone 4

- users
- devices
- accounts
- publish tasks
- AI jobs

### Milestone 5

- models
- skills
- provider config
- system config
- reports

## Frontend Coordination

The admin frontend scaffold is created in:

- `/Volumes/mud/project/github/social-auto-upload/OmniDriveAdmin`

This frontend is intentionally separate from:

- `/Volumes/mud/project/github/social-auto-upload/omnidrive_frontend`

Claude can build UI independently without blocking backend implementation.

Suggested frontend route map:

- `/dashboard`
- `/users`
- `/devices`
- `/media-accounts`
- `/publish-tasks`
- `/ai-jobs`
- `/skills`
- `/pricing`
- `/orders`
- `/wallet-ledgers`
- `/support-recharges`
- `/distribution/relations`
- `/distribution/commissions`
- `/distribution/settlements`
- `/withdrawals`
- `/audits`
- `/settings`
- `/admins`

## Immediate Next Steps

1. Finish real S3 storage integration.
2. Add admin auth and RBAC schema.
3. Add support recharge schema and review APIs.
4. Add distribution and commission schema.
5. Add admin frontend API contract for Claude.
