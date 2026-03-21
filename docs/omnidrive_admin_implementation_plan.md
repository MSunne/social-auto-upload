# OmniDriveAdmin Implementation Plan

## Goal

Build an internal admin system for `OmniDrive` that supports:

- user operations and risk control
- OmniBull device operations
- media account and publish-task support workflows
- AI job inspection and manual intervention
- package, order, wallet, and manual top-up operations
- distribution commission lifecycle management
- admin auth, RBAC, audit logs, and system configuration

This admin system is for internal operators, finance staff, customer support, and administrators.

## Product Boundary

`OmniDrive`

- end-user cloud console
- users manage their own devices, skills, AI jobs, accounts, and publish tasks

`OmniDriveAdmin`

- internal management console
- admins manage users, money, orders, commissions, audits, and exception handling

## Recommended Architecture

### Backend

- reuse `omnidrive_cloud` as the admin backend foundation
- extend current API with `/api/admin/v1/*`
- reuse PostgreSQL as the system of record
- reuse the same JWT system, but add admin identity and RBAC
- reuse S3 object storage for evidence, vouchers, and settlement exports

### Frontend

- frontend project directory: `OmniDriveAdmin`
- Next.js App Router
- React 19
- TypeScript
- Tailwind CSS v4
- TanStack Query
- axios
- zustand

## Roles

- `super_admin`
  - full access
- `ops_admin`
  - user, device, task, and skill operations
- `finance_admin`
  - orders, wallet, manual top-up, commission settlement, withdrawals
- `support_admin`
  - customer support, device troubleshooting, account login issues
- `review_admin`
  - manual top-up review, withdrawal review, exception review
- `audit_admin`
  - read-only access to audit logs and finance evidence

## Phase Plan

### Phase 0: Foundation

Goal:

- establish admin frontend project
- define admin backend scope and route map
- define admin roles and permission matrix
- define new database tables and status enums

Backend tasks:

- add admin auth model on top of current `users`
- define RBAC middleware for `/api/admin/v1/*`
- define audit-event extension strategy
- define finance/distribution/manual-top-up schema
- normalize environment variable strategy for database, S3, AI provider, and payment config

Frontend tasks:

- create `OmniDriveAdmin`
- establish route groups and layout shell
- create Claude handoff doc with page map and UI expectations

Exit criteria:

- admin route namespace decided
- table list decided
- permission matrix decided
- frontend skeleton created

### Phase 1: Core Admin Backbone

Goal:

- deliver the minimum internal admin workflows required to operate the platform safely

Modules:

1. Admin auth and RBAC
2. Dashboard overview
3. User management
4. Device management
5. Account management
6. Publish-task management
7. Audit log

Backend tasks:

- add admin login/session APIs
- add admin list/detail APIs for users, devices, accounts, publish tasks
- add user freeze/disable operations
- add device enable/disable and force-release operations
- add task retry/cancel/manual-resolve admin operations
- add admin-only search and filters
- write audit logs for every admin mutation

Exit criteria:

- internal team can inspect and intervene in user/device/task issues
- all admin mutations leave audit records

### Phase 2: Finance and Manual Top-Up

Goal:

- make recharge and wallet operations controllable by finance and support

Modules:

1. package management
2. order management
3. wallet ledger
4. support recharge review

Backend tasks:

- add package CRUD APIs
- add payment-order query and detail APIs
- add support recharge application table and APIs
- add support recharge approval/reject/revoke workflow
- add gifted-credit fields and separate ledger reasons
- enforce idempotent balance mutations
- keep review evidence and operator notes

Recommended support recharge statuses:

- `pending_review`
- `approved`
- `rejected`
- `credited`
- `revoked`

Exit criteria:

- manual customer-service recharge is reviewable, traceable, and idempotent
- wallet changes are fully traceable to order or review records

### Phase 3: Distribution and Commission

Goal:

- support affiliate/distribution revenue sharing with deferred settlement

Modules:

1. distributor management
2. referral relationship management
3. commission detail
4. settlement management
5. withdrawal management

Backend tasks:

- add distributor profile and referral binding model
- add commission rule model
- add recharge-to-commission projection logic
- add consumption-to-commission release logic
- add settlement batch model
- add withdrawal request model
- add commission summary endpoints

Required commission lifecycle:

- `pending_consume`
  - user paid, but did not consume yet
- `pending_settlement`
  - user consumed, commission confirmed, not settled yet
- `settled`
  - settled to distributor

Exit criteria:

- finance can track pending-consume, pending-settlement, and settled amounts
- each commission item can be traced back to recharge and consumption records

### Phase 4: AI / Skill / Provider Operations

Goal:

- enable internal operation of AI production pipelines

Modules:

1. AI model management
2. AI provider configuration
3. AI job review and retry
4. skill template and platform policy operations

Backend tasks:

- add provider config model or config registry abstraction
- add model enable/disable and pricing config
- add AI job retry/cancel/manual-fix APIs
- add cloud skill templates and internal visibility controls
- add provider health and quota monitoring views

Exit criteria:

- ops team can manage model availability and recover failed AI jobs

### Phase 5: Reports and Production Hardening

Goal:

- make the admin system ready for long-term operation

Modules:

1. reporting and export
2. reconciliation
3. alert center
4. security hardening

Backend tasks:

- add daily finance reports and settlement exports
- add payment reconciliation job
- add anomaly alerts for devices, tasks, AI jobs, and payments
- add stricter audit queries and retention policy
- replace schema auto-create with migrations
- add integration tests for money-moving workflows

Exit criteria:

- admin system is safe for production finance and support workflows

## Backend Functional Map

### 1. Dashboard

- summary cards
- exception counters
- finance and commission snapshots
- pending review queues

### 2. User Management

- list/detail/search
- status control
- wallet snapshot
- device/task/account linkage
- operator notes

### 3. Device Management

- list/detail/search
- online/offline status
- sync-state inspection
- force disable
- stuck-task lease release

### 4. Media Account and Publish Task Support

- account list/detail
- login session inspection
- publish task detail, artifact, evidence, readiness
- manual resolve/retry/cancel

### 5. Finance

- packages
- orders
- wallet ledgers
- payment anomalies

### 6. Support Recharge Review

- application queue
- voucher review
- gifted credits
- approval and rejection
- immutable ledger trace

### 7. Distribution

- distributors
- invited users
- commission items
- commission summaries
- settlement batches
- withdrawal requests

### 8. AI / Skills / Providers

- model list and status
- provider config and health
- AI job operations
- skill template operations

### 9. Audit and Security

- audit event search
- admin action details
- permission management
- sensitive operation review

### 10. System Configuration

- defaults
- business rules
- payment channel switches
- settlement thresholds
- risk-control toggles

## Suggested Database Additions

### Admin and RBAC

- `admin_users`
- `admin_roles`
- `admin_role_bindings`
- `admin_permission_bindings`

### Finance

- `payment_orders`
- `support_recharge_requests`
- `support_recharge_vouchers`

### Distribution

- `distributors`
- `distribution_referrals`
- `distribution_rules`
- `distribution_commission_items`
- `distribution_settlement_batches`
- `distribution_settlement_items`
- `withdrawal_requests`

### Audit

- extend `audit_events` usage
- optionally add `admin_operation_logs` if finance-grade separation is needed

## API Namespace Suggestion

- `/api/admin/v1/auth/*`
- `/api/admin/v1/dashboard/*`
- `/api/admin/v1/users/*`
- `/api/admin/v1/devices/*`
- `/api/admin/v1/accounts/*`
- `/api/admin/v1/publish-tasks/*`
- `/api/admin/v1/ai/*`
- `/api/admin/v1/skills/*`
- `/api/admin/v1/packages/*`
- `/api/admin/v1/orders/*`
- `/api/admin/v1/wallet/*`
- `/api/admin/v1/support-recharges/*`
- `/api/admin/v1/distribution/*`
- `/api/admin/v1/withdrawals/*`
- `/api/admin/v1/audits/*`
- `/api/admin/v1/settings/*`
- `/api/admin/v1/admins/*`

## Recommended Build Order

1. S3 and storage unification in `omnidrive_cloud`
2. admin auth + RBAC + audit foundation
3. support recharge workflow
4. distribution commission lifecycle
5. package/order/wallet admin APIs
6. user/device/task admin APIs
7. AI/provider admin APIs
8. reporting and reconciliation

## Coordination Notes

- backend should expose stable enums before Claude starts deep UI work
- finance and commission modules should be modeled as documents, not only computed totals
- every balance mutation must be idempotent
- every review and settlement action must be auditable
- admin frontend should remain independent from the end-user frontend to keep permissions, layout, and deployment clean
