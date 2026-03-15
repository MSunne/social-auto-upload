# OmniDrive Phase 1 API Contract

This document is the contract target for the cloud backend and the frontend implementation.

## Base

- Base URL: `/api/v1`
- Auth: `Authorization: Bearer <token>`
- Device agent auth: `X-Agent-Key: <agent-key>`

## Auth

### `POST /auth/register`

- create a cloud user
- request: `email`, `name`, `password`
- response: user profile

### `POST /auth/login`

- request: `email`, `password`
- response: access token

### `GET /auth/me`

- response: current user profile

## Devices

### `GET /devices`

- list all devices owned by the current user
- each device includes a nested `load` object with account, material, task, and login-session counts

### `GET /devices/{deviceId}`

- fetch one device detail
- includes the same nested `load` object as the device list

### `GET /devices/{deviceId}/workspace`

- fetch one device workspace payload
- includes:
  - `device`
  - `recentTasks`
  - `activeLoginSessions`
  - `recentAccounts`
  - `materialRoots`
  - `skillSyncStates`

### `POST /devices/claim`

- request: `deviceCode`
- claims an already-online OmniBull device into the current user account

### `PATCH /devices/{deviceId}`

- fields: `name`, `defaultReasoningModel`, `isEnabled`

## Overview

### `GET /overview/summary`

- fetch dashboard summary counts for the current user
- includes recent publish tasks and recent AI jobs
- also includes `materialRootCount` and `materialEntryCount`
- also includes task breakdown counts such as `pendingTaskCount`, `runningTaskCount`, `needsVerifyTaskCount`, `failedTaskCount`
- also includes `activeLoginSessionCount`
- also includes AI breakdown counts such as `queuedAiJobCount`, `runningAiJobCount`, `failedAiJobCount`

## History

### `GET /history?kind=...&status=...&limit=...`

- returns a merged recent activity feed
- combines publish tasks, AI jobs, and cloud-side audit events
- optional filters:
  - `kind`
  - `status`
  - `limit`

## Accounts

### `GET /accounts?deviceId=...`

- list mirrored platform accounts
- each account includes a nested `load` object with related task and login-session counters

### `GET /accounts/{accountId}`

- fetch one account detail
- includes the same nested `load` object as the account list

### `GET /accounts/{accountId}/workspace`

- fetch one account workspace payload
- includes:
  - `account`
  - `recentTasks`
  - `activeLoginSessions`

### `DELETE /accounts/{accountId}`

- remove one mirrored account from cloud
- returns `409` with usage summary if the account is still referenced by publish tasks or active login sessions

### `POST /accounts/{accountId}/validate`

- create a revalidation login session for an existing account

### `POST /accounts/remote-login`

- request: `deviceId`, `platform`, `accountName`
- creates a login session for a selected device

### `GET /accounts/login-sessions/{sessionId}`

- fetch the latest QR, verification payload, and login status for a session

### `POST /accounts/login-sessions/{sessionId}/actions`

- request: `actionType`, `payload`
- enqueue a remote verification action for the local device
- typical actions:
  - `click_option`
  - `fill_text`
  - `fill_text_and_submit`
  - `press_enter`

## Materials

### `GET /materials/roots?deviceId=...`

- list mirrored material roots for one device or all owned devices

### `GET /materials/list?deviceId=...&root=...&path=...`

- list one mirrored directory snapshot
- returns directory entries sorted with folders first

### `GET /materials/file?deviceId=...&root=...&path=...`

- fetch one mirrored file preview and metadata
- text previews may be truncated

## Skills

### `GET /skills`

- list product skills under the current user
- each skill includes a nested `load` object with asset and related task counters
- `load` also includes `aiJobCount` and `activeAiJobCount`

### `POST /skills`

- create skill

### `PATCH /skills/{skillId}`

- update skill

### `GET /skills/{skillId}`

- fetch one skill detail
- includes the same nested `load` object as the skill list

### `GET /skills/{skillId}/workspace`

- fetch one skill workspace payload
- includes:
  - `skill`
  - `assets`
  - `recentTasks`
  - `recentAiJobs`
  - `deviceSyncs`

### `DELETE /skills/{skillId}`

- delete a skill
- returns `409` with usage summary if the skill is still referenced by publish tasks or AI jobs
- deletes stored skill asset files referenced by that skill when possible

### `GET /skills/{skillId}/assets`

- list metadata for files attached to a product skill

### `POST /skills/{skillId}/assets`

- create a skill asset metadata record
- current phase stores metadata only, not direct binary upload

### `POST /skills/{skillId}/upload`

- multipart upload for real skill asset files
- form fields:
  - `assetType`
  - `file`
- response includes persisted asset metadata and a `publicUrl`

## Tasks

### `GET /tasks`

- list mirrored publish tasks
- optional filters:
  - `deviceId`
  - `status`
  - `platform`
  - `accountName`
  - `limit`

### `POST /tasks`

- create a cloud publish task for a specific device and account
- current status starts as `pending`
- supports optional `materialRefs`
- each material ref uses `root`, `path`, optional `role`

### `GET /tasks/{taskId}`

- fetch publish task detail, including verification payload if present

### `GET /tasks/{taskId}/workspace`

- fetch one publish task workspace payload
- includes:
  - `task`
  - `device`
  - `account`
  - `skill`
  - `events`
  - `artifacts`
  - `materials`
  - `actions`
  - `readiness`
  - optional `runtime`
- `actions` exposes backend-computed booleans such as `canEdit`, `canCancel`, `canRetry`, `canDelete`
- `actions` also includes `canResume` and `canResolveManual` for `needs_verify` handling
- `readiness` exposes backend-computed execution checks for device/account/skill/material availability
- `runtime` exposes the latest agent-side execution snapshot and `lastAgentSyncAt`

### `GET /tasks/{taskId}/events`

- fetch the publish task timeline
- includes cloud-side edits and agent-side execution / verification events

### `GET /tasks/{taskId}/artifacts`

- fetch structured task artifacts
- may include verification screenshots, text evidence, and future local output files

### `GET /tasks/{taskId}/materials`

- fetch mirrored material references attached to one task
- returns a snapshot of the local file metadata at selection time

### `POST /tasks/{taskId}/cancel`

- request cancellation for one task
- `pending` tasks become `cancelled`
- `running` or `needs_verify` tasks become `cancel_requested`

### `POST /tasks/{taskId}/force-release`

- manually releases an active publish-task lease from the cloud side
- `running` tasks return to `pending`
- `cancel_requested` tasks become `cancelled`
- intended for stuck executions that should not wait for lease expiry

### `POST /tasks/{taskId}/resume`

- resume a `needs_verify` task back to `pending`
- clears verification payload and any active lease state
- preserves prior artifacts and timeline so the operator can keep the evidence trail

### `POST /tasks/{taskId}/manual-resolve`

- manually resolve a `needs_verify` task into one final state
- request:
  - `status` must be one of `success`, `completed`, `failed`, `cancelled`
  - optional `message`
  - optional `textEvidence`
  - optional `payload`
- stores optional manual evidence as a structured task artifact with key `manual-resolution`

### `POST /tasks/{taskId}/retry`

- reset a non-running task back to `pending`
- clears verification payload and any lease state
- clears previous task artifacts so the next attempt starts with a clean evidence set
- deletes stored artifact files referenced by the previous attempt when possible

### `PATCH /tasks/{taskId}`

- update editable task fields like title, content, status, message, media, and run time
- supports replacing task `materialRefs`

### `DELETE /tasks/{taskId}`

- delete one publish task
- also removes stored artifact files referenced by that task when possible

## AI

### `GET /ai/models?category=...`

- list enabled AI models
- optional `category`: `chat`, `image`, `video`

### `GET /ai/jobs`

- list AI jobs created by the current user
- optional filters: `jobType`, `status`, `skillId`, `limit`

### `POST /ai/jobs`

- create a queued AI job record
- request: `jobType`, `modelName`, optional `skillId`, optional `prompt`, optional `inputPayload`
- when `skillId` is provided, the backend validates that the referenced skill belongs to the user and its `outputType` matches `jobType`

### `GET /ai/jobs/{jobId}`

- fetch one AI job detail

### `GET /ai/jobs/{jobId}/workspace`

- fetch one AI job workspace payload
- includes:
  - `job`
  - `model`
  - `skill`
  - `actions`

### `PATCH /ai/jobs/{jobId}`

- update editable AI job fields
- supports:
  - `skillId`
  - `prompt`
  - `status`
  - `inputPayload`
  - `outputPayload`
  - `message`
  - `costCredits`
  - `finishedAt`
- AI job status transitions are validated by the backend

### `POST /ai/jobs/{jobId}/cancel`

- cancel a queued or running AI job

### `POST /ai/jobs/{jobId}/retry`

- move a finished or failed AI job back to `queued`
- clears previous `outputPayload`
- clears `finishedAt`

## Billing

### `GET /billing/packages`

- list enabled recharge / package plans

### `GET /billing/ledger`

- list wallet ledger records for the current user
- current phase may return an empty array before充值或消费记录接入

## Agent Bridge

### `POST /agent/heartbeat`

- body:
  - `deviceCode`
  - `deviceName`
  - `agentKey`
  - `localIp`
  - `publicIp`
  - `runtimePayload`

### `GET /agent/login-tasks/{deviceCode}`

- returns pending login sessions for the device
- disabled devices receive `409`

### `POST /agent/login-sessions/{sessionId}/event`

- push QR updates, verification updates, success, or failure

### `GET /agent/login-sessions/{sessionId}/actions`

- the local agent consumes pending verification actions for a login session
- current implementation behaves like a one-time queue

### `POST /agent/accounts/sync`

- local agent upserts a mirrored social account state into cloud

### `GET /agent/skills/{deviceCode}?since=...&limit=...`

- local agent pulls enabled product skills for one claimed device
- each item includes:
  - `revision`
  - `skill`
  - `assets`
  - optional current `sync` state for that device
- optional `since` must be RFC3339

### `POST /agent/skills/sync`

- local agent reports per-skill sync status back to cloud
- each item supports:
  - `skillId`
  - `syncStatus`
  - optional `syncedRevision`
  - optional `assetCount`
  - optional `message`
  - optional `lastSyncedAt`

### `POST /agent/materials/roots/sync`

- sync allowed material roots from the local OmniBull machine

### `POST /agent/materials/directory/sync`

- sync one directory snapshot from a local material root
- replaces the visible child entries under that mirrored directory

### `POST /agent/materials/file/sync`

- sync one mirrored file preview from a local material root
- intended for text previews, prompt templates, notes, and similar lightweight files

### `GET /agent/publish-tasks/{deviceCode}`

- local agent polls actionable publish tasks for the device
- current phase returns `pending` and `running`
- tasks with future `runAt` stay hidden until their scheduled time arrives
- disabled devices receive `409`
- `needs_verify` stays in task detail and timeline, but is no longer re-queued for automatic execution
- expired `running` leases are automatically recovered back to `pending`
- expired `cancel_requested` leases are automatically recovered to `cancelled`

### `GET /agent/publish-tasks/{taskId}/package?deviceCode=...`

- local agent fetches one publish-task execution package
- returns:
  - `task`
  - optional `account`
  - optional `skill`
  - `skillAssets`
  - `materials`
- also includes `readiness` so SAU can detect missing materials or disabled dependencies before it touches third-party platforms
- may include `runtime` so SAU or OpenClaw can inspect the latest local execution snapshot
- intended to give SAU one complete execution payload before talking to third-party platforms

### `POST /agent/publish-tasks/{taskId}/claim`

- request body: `deviceCode`
- atomically moves a `pending` task into `running`
- tasks with future `runAt` cannot be claimed yet
- disabled devices cannot claim tasks
- returns `leaseToken` and `leaseExpiresAt`

### `POST /agent/publish-tasks/{taskId}/renew`

- request body: `deviceCode`, `leaseToken`
- extends the task lease for a running or cancel-requested task
- if an old lease has already expired, the task may be recovered before the next poll

### `POST /agent/publish-tasks/{taskId}/release`

- request body: `deviceCode`, `leaseToken`, optional `message`
- lets the local agent voluntarily release a claimed task lease
- `running` tasks go back to `pending`
- `cancel_requested` tasks become `cancelled`
- intended for local preflight failures or fast cancel confirmation without waiting for lease expiry

### `POST /agent/publish-tasks/sync`

- local agent mirrors task execution state back to cloud
- supports task creation from local side or updating an existing cloud task
- intended for `pending`, `running`, `success`, `failed`, `needs_verify` and similar states
- supports optional `artifacts` for screenshots, logs, previews, and other structured evidence
- supports optional `executionPayload` for current local progress, step, or executor context
- if `verificationPayload.screenshotData` is provided as base64 or data URL, the backend stores it and rewrites the payload to `screenshotUrl` and storage metadata
- if the task already exists under another device, the backend returns `409`
- if the task currently has an active lease, `leaseToken` must match or the backend returns `409`
- if the incoming status violates the cloud task state machine, the backend returns `409`
- if an artifact is re-synced with the same `artifactKey` but a new stored file, the backend replaces the old file and cleans up the previous object when possible
- runtime state is automatically cleared when a task returns to `pending` or reaches a terminal state

## Public File Delivery

### `GET /files/{storageKey}`

- serves uploaded skill assets and stored verification screenshots
- current phase uses the OmniDrive API process as the file server

## Phase 1 Response Rules

- all list endpoints return arrays
- all timestamps use ISO 8601 UTC strings
- device and task status must be explicit enum values
- verification payloads always keep screenshots and button actions in structured JSON
- dashboard summary counts are integers and recent lists are already sorted by newest first
