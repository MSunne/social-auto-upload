# OmniDriveAdmin Backend Workstreams

## Goal

Split the admin backend into parallel workstreams so multiple engineers can work at the same time with minimal merge conflict and clear ownership.

This document is for backend only.

Frontend remains in:

- `/Volumes/mud/project/github/social-auto-upload/OmniDriveAdmin`

Backend remains in:

- `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud`

## Current State

Already in place:

- admin route group `/api/admin/v1/*`
- bootstrap admin auth
- admin permission middleware
- dashboard summary
- users list
- devices list
- orders list
- wallet ledger list
- support recharge list/detail/events/credit/reject
- audit list
- API contract for Claude

Still missing or only partial:

- real admin users / roles / permissions schema
- distribution / commission / settlement data model
- withdrawal data model and review flow
- admin account / task / AI job route groups
- pricing management write APIs
- system config write APIs
- richer finance operations like manual adjustments and reconciliation

## Parallelization Rules

1. One thread owns one route group and its schema.
2. Shared files should be touched only when unavoidable.
3. `internal/http/router.go` is a hot file.
   Add routes in one short merge, not during long feature work.
4. New admin modules should prefer their own files:
   - `internal/http/handlers/admin_<module>.go`
   - `internal/store/admin_<module>.go`
5. Cross-domain mutations must go through store methods, not directly in handlers.
6. All write endpoints must emit audit events.

## Recommended Thread Split

### Thread A: Admin Auth And RBAC

Goal:

- replace bootstrap admin auth with real admin identities

Scope:

- `admin_users`
- `admin_roles`
- `admin_permissions`
- `admin_role_permissions`
- `admin_user_roles`
- `admin_sessions`
- real login/logout/session invalidation
- permission resolution

Primary files:

- `internal/database/schema.go`
- `internal/app/admin.go`
- `internal/http/middleware/admin.go`
- `internal/http/handlers/admin_auth.go`
- `internal/store/admin_users.go`
- `internal/store/admin_roles.go`

Routes:

- `POST /api/admin/v1/auth/login`
- `POST /api/admin/v1/auth/logout`
- `GET /api/admin/v1/me`
- `GET /api/admin/v1/admins`
- `GET /api/admin/v1/roles`
- `POST /api/admin/v1/roles`
- `PATCH /api/admin/v1/admins/{adminId}`

Dependencies:

- none, can start immediately

Conflicts to watch:

- `internal/http/router.go`
- `internal/app/admin.go`

Acceptance:

- bootstrap admin can be retired or left as emergency fallback
- permission checks read from DB-backed role assignments

Suggested branch:

- `codex/admin-rbac`

### Thread B: Finance Core

Goal:

- stabilize admin finance base around orders, ledgers, packages, and manual adjustments

Scope:

- package management write APIs
- order detail / events admin views
- wallet ledger detail
- manual wallet adjustment request flow
- finance summary consistency

Primary files:

- `internal/store/admin.go`
- `internal/store/billing.go`
- `internal/store/billing_grants.go`
- `internal/http/handlers/admin_console.go`
- `internal/http/handlers/admin_finance.go` (new)
- `internal/database/schema.go`

Routes:

- `GET /api/admin/v1/orders/{orderId}`
- `GET /api/admin/v1/orders/{orderId}/events`
- `GET /api/admin/v1/wallet-ledgers/{ledgerId}`
- `POST /api/admin/v1/wallet-adjustments`
- `GET /api/admin/v1/pricing/packages`
- `POST /api/admin/v1/pricing/packages`
- `PATCH /api/admin/v1/pricing/packages/{packageId}`

Dependencies:

- none, can start immediately

Conflicts to watch:

- `internal/store/billing.go`
- `internal/database/schema.go`

Acceptance:

- finance admin no longer relies only on list pages
- manual compensation uses ledger-safe write path

Suggested branch:

- `codex/admin-finance-core`

### Thread C: Support Recharge

Goal:

- finish the customer-service recharge module beyond the current review baseline

Current status:

- list/detail/events/credit/reject already available

Remaining scope:

- support recharge attachments table or S3 asset normalization
- bonus credits support
- reviewer notes history
- resubmit flow visibility
- admin dashboard widgets for support recharge

Primary files:

- `internal/http/handlers/admin_support_recharge.go`
- `internal/store/support_recharge_review.go`
- `internal/store/admin_support_recharge.go`
- `internal/http/handlers/billing.go`
- `internal/database/schema.go`

Routes:

- existing support recharge routes
- optional:
  - `POST /api/admin/v1/support-recharges/{orderId}/bonus-preview`
  - `POST /api/admin/v1/support-recharges/{orderId}/attachments`

Dependencies:

- depends on Finance Core only if bonus credits become a separate ledger grant type

Conflicts to watch:

- `internal/store/billing_grants.go`
- `internal/database/schema.go`

Acceptance:

- support recharge can represent base credits and bonus credits separately
- proof assets and review trail are inspectable

Suggested branch:

- `codex/admin-support-recharge`

### Thread D: Distribution And Commission

Goal:

- implement referral relations and commission ledger lifecycle

Scope:

- referral relation schema
- commission rules
- commission ledger
- pending consume -> pending settlement transition
- settlement batch generation
- admin list/detail views

Primary files:

- `internal/store/distribution.go` (new)
- `internal/http/handlers/admin_distribution.go` (new)
- `internal/database/schema.go`
- usage billing hook files:
  - `internal/store/billing_usage.go`
  - `internal/store/billing.go`

Routes:

- `GET /api/admin/v1/distribution/relations`
- `POST /api/admin/v1/distribution/relations`
- `GET /api/admin/v1/distribution/rules`
- `POST /api/admin/v1/distribution/rules`
- `GET /api/admin/v1/distribution/commissions`
- `GET /api/admin/v1/distribution/settlements`
- `POST /api/admin/v1/distribution/settlements`

Dependencies:

- depends on Finance Core for wallet consumption events

Conflicts to watch:

- `internal/store/billing_usage.go`
- `internal/database/schema.go`

Acceptance:

- recharge creates `pending_consume`
- user consumption releases to `pending_settlement`
- settlement batch can close eligible rows

Suggested branch:

- `codex/admin-distribution`

### Thread E: Withdrawal Review

Goal:

- build promoter withdrawal workflow

Scope:

- withdrawal request schema
- review approve/reject
- mark paid
- attach payout proof
- list/detail/admin actions

Primary files:

- `internal/store/withdrawals.go` (new)
- `internal/http/handlers/admin_withdrawals.go` (new)
- `internal/database/schema.go`

Routes:

- `GET /api/admin/v1/withdrawals`
- `GET /api/admin/v1/withdrawals/{withdrawalId}`
- `POST /api/admin/v1/withdrawals/{withdrawalId}/approve`
- `POST /api/admin/v1/withdrawals/{withdrawalId}/reject`
- `POST /api/admin/v1/withdrawals/{withdrawalId}/mark-paid`

Dependencies:

- depends on Distribution thread because available withdrawal balance comes from settled commission

Conflicts to watch:

- `internal/database/schema.go`
- future shared settlement helpers

Acceptance:

- withdrawal status changes are idempotent
- every action writes audit and review trail

Suggested branch:

- `codex/admin-withdrawals`

### Thread F: Operations Console

Goal:

- expand admin operational read/control endpoints for platform operations

Scope:

- admin accounts list/detail
- admin publish tasks list/detail/operate
- admin AI jobs list/detail/operate
- richer user detail
- richer device detail

Primary files:

- `internal/store/admin_users.go` (new or split from `admin.go`)
- `internal/store/admin_devices.go` (new or split from `admin.go`)
- `internal/store/admin_accounts.go` (new)
- `internal/store/admin_tasks.go` (new)
- `internal/store/admin_ai.go` (new)
- `internal/http/handlers/admin_operations.go` (new)

Routes:

- `GET /api/admin/v1/accounts`
- `GET /api/admin/v1/publish-tasks`
- `GET /api/admin/v1/publish-tasks/{taskId}`
- `POST /api/admin/v1/publish-tasks/{taskId}/retry`
- `GET /api/admin/v1/ai-jobs`
- `GET /api/admin/v1/ai-jobs/{jobId}`
- `POST /api/admin/v1/ai-jobs/{jobId}/retry`

Dependencies:

- mostly independent

Conflicts to watch:

- if still using one big `internal/store/admin.go`, split it first

Acceptance:

- support and ops can diagnose accounts, tasks, and AI jobs without using customer APIs

Suggested branch:

- `codex/admin-operations`

### Thread G: System Config, Provider, Skill Governance

Goal:

- give internal team a safe governance layer for config and model/provider operations

Scope:

- provider config read/write metadata
- model registry admin control
- skill moderation
- system config center

Primary files:

- `internal/http/handlers/admin_system.go` (new)
- `internal/http/handlers/admin_skills.go` (new)
- `internal/store/admin_system.go` (new)
- `internal/store/admin_skills.go` (new)
- `internal/database/schema.go`

Routes:

- `GET /api/admin/v1/system-config`
- `PATCH /api/admin/v1/system-config`
- `GET /api/admin/v1/models`
- `PATCH /api/admin/v1/models/{modelId}`
- `GET /api/admin/v1/providers`
- `PATCH /api/admin/v1/providers/{providerId}`
- `GET /api/admin/v1/skills`
- `PATCH /api/admin/v1/skills/{skillId}`

Dependencies:

- RBAC thread recommended first because this module is high risk

Acceptance:

- sensitive config changes become auditable and permission-gated

Suggested branch:

- `codex/admin-system-governance`

### Thread H: Reporting And Reconciliation

Goal:

- add management reports and finance reconciliation after core flows are stable

Scope:

- revenue reports
- support recharge reports
- commission reports
- reconciliation runs
- CSV/export APIs

Primary files:

- `internal/http/handlers/admin_reports.go` (new)
- `internal/store/admin_reports.go` (new)
- `internal/database/schema.go`

Routes:

- `GET /api/admin/v1/reports/revenue`
- `GET /api/admin/v1/reports/support-recharges`
- `GET /api/admin/v1/reports/commissions`
- `POST /api/admin/v1/reports/reconciliation-runs`

Dependencies:

- depends on Finance, Distribution, Withdrawal being real

Suggested branch:

- `codex/admin-reporting`

## Dependency Graph

Can start now:

- Thread A
- Thread B
- Thread C
- Thread F

Starts after finance hooks are clear:

- Thread D

Starts after distribution exists:

- Thread E

Starts after RBAC is stable:

- Thread G

Starts after finance/distribution/withdrawal are stable:

- Thread H

## Merge Strategy

To reduce conflicts, merge in this order:

1. Thread A
2. Thread B
3. Thread C
4. Thread F
5. Thread D
6. Thread E
7. Thread G
8. Thread H

## Shared Hotspots

These files will cause merge conflicts if multiple threads touch them casually:

- `internal/http/router.go`
- `internal/database/schema.go`
- `internal/domain/admin_models.go`
- `internal/store/admin.go`

Recommended handling:

- add new route registrations in one compact commit near merge time
- if `admin.go` grows further, split now into module files before more threads start
- for schema changes, each thread should add its own clearly delimited table block

## Recommended Owner Allocation

If you want the lowest-risk split:

- one person: Thread A
- one person: Thread B + Thread C
- one person: Thread D
- one person: Thread E
- one person: Thread F
- one person: Thread G

If people are fewer:

- person 1: A + G
- person 2: B + C
- person 3: D + E
- person 4: F

## Immediate Actions

Before multiple threads start, do these two prep steps:

1. Split `internal/store/admin.go` into module-specific files.
2. Freeze the admin API contract file as the frontend truth source.

After that, parallel work is much safer.
