# OmniDrive Backend Plan

## Goal

Build a production-oriented cloud backend for `OmniDrive` that coordinates:

- cloud user accounts and billing
- `OmniBull` device activation and heartbeat
- remote login sessions for SAU platform accounts
- product skills and knowledge files
- AI generation jobs for image, video, and chat
- publish task tracking and remote intervention workflows

The backend must support:

- a dedicated cloud web console
- OpenClaw skills that call cloud capabilities
- local `OmniBull` agents that pull tasks and push status

## Recommended Stack

- API service: `Go 1.26+`
- router: `chi`
- database: `PostgreSQL`
- cache / background coordination: `Redis`
- object storage: S3-compatible storage
- auth: JWT access tokens with refresh token rotation in a later milestone
- deployment: single binary with containerized delivery

## Why This Stack

- Better fit for a cloud control plane that must serve many `OmniBull` agents concurrently.
- Easier to deploy and operate as a single static service binary.
- Clear separation from the local Python-based `SAU` execution engine.
- Good long-term maintenance characteristics for a multi-tenant service.
- Still allows reuse of the existing Python implementation as a domain and protocol prototype.

## System Boundaries

### OmniDrive Cloud

- owns user identity and ownership
- owns billing and AI generation history
- owns device claim / activation
- owns the canonical cloud task mirror
- sends login and publish intents to `OmniBull`

### OmniBull Local

- owns platform cookies and browser sessions
- owns local media files and generated file backups
- owns actual browser automation and publish execution
- pushes account state, publish state, and verification screenshots back to cloud

### OpenClaw / Skills

- `OmniSkill` calls OmniDrive cloud APIs for AI, billing, and cloud task queries
- `SauSkill` calls local OmniBull APIs for local account, material, and publish operations

## Phase 1 Scope

Phase 1 targets the operational backbone required by the current UI and the already-proven demo flows:

1. User registration and login
2. Device heartbeat and claim by device code
3. Device list and device status
4. Platform account mirror
5. Remote login session creation and verification action relay
6. Product skill CRUD
7. Publish task mirror and manual verification tracking
8. Overview summary and history feed
9. Basic AI model registry, AI job records, and package listing
10. Local material root and file mirror for OmniBull devices

## Core Modules

### Auth

- user registration
- email + password login
- current session lookup

### Devices

- claim device by `device_code`
- enable / disable device
- list device online status and basic runtime metrics

### Agent Bridge

- heartbeat from local agent
- poll login tasks
- consume login actions
- push login session events
- mirror publish task state

### Accounts

- list mirrored social accounts per device
- view account status and latest auth time
- create remote login session for a device

### Skills

- create, update, delete product skills
- attach files and generation policy
- detect whether a skill is used by accounts or tasks before delete

### Tasks

- list publish tasks
- view publish task details
- track `needs_verify` state and screenshot metadata
- keep a per-task event timeline for cloud edits and agent execution evidence
- store structured task artifacts such as verification screenshots, logs, and local output references
- store task-to-material snapshot references so local input files remain traceable even after directory changes
- support task claim / lease / renew so one device execution worker owns the task for a bounded time

### AI Jobs

- store image, video, and chat generation job requests
- store request parameters, queue state, and future output metadata

### Billing

- package listing
- wallet balance
- ledger records

## Data Model Overview

### Users

- `users`
- `wallet_ledgers`

### Devices

- `devices`
- device online status is derived from `last_seen_at`
- local material roots and mirrored directory/file snapshots are attached to devices

### Social Publishing

- `platform_accounts`
- `login_sessions`
- `login_session_actions`
- `publish_tasks`
- `publish_task_events`
- `publish_task_artifacts`

### Product Skills

- `product_skills`
- `product_skill_assets`

### AI

- `ai_models`
- `ai_jobs`
- output metadata can stay in `ai_jobs.output_payload` in the current phase

### Billing

- `billing_packages`
- `wallet_ledgers`

## Communication Model

### Cloud -> OmniBull

- cloud creates a login session or publish intent
- local agent polls for pending work
- local agent claims work atomically

### OmniBull -> Cloud

- local agent sends heartbeats
- local agent sends login QR / verification updates
- local agent mirrors account and publish task status

This keeps the local device behind NAT and avoids direct inbound access from cloud.

## API Groups

- `/api/v1/auth`
- `/api/v1/devices`
- `/api/v1/accounts`
- `/api/v1/skills`
- `/api/v1/tasks`
- `/api/v1/ai`
- `/api/v1/billing`
- `/api/v1/agent`

## Current Execution Strategy

### Start Now

- create the backend project skeleton
- define base domain models and transport contracts
- expose initial route groups for devices, accounts, skills, tasks, and agent bridge
- port the proven `cloud_demo` login session concepts into structured Go services

### Defer Slightly

- payment gateways
- refresh token rotation
- complex AI workflow execution
- recurring publish scheduling

## Launch Blockers To Resolve Before Production

1. Rotate all keys currently written in `AlTask.md`.
2. Move all secrets to environment variables or a secret manager.
3. Add database migrations instead of relying on implicit table creation.
4. Add operation audit logs for account login, skill changes, and manual verification.
5. Add bucket lifecycle rules for screenshots and generated media.
