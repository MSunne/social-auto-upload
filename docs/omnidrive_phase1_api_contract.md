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
- each `skillSyncStates` item also includes:
  - `desiredRevision`
  - `isCurrent`
  - `needsSync`

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

### `GET /materials/workspace?deviceId=...&root=...&path=...&scope=...&limit=...`

- fetch one material workspace payload
- `path` can point to either a mirrored file or a mirrored directory
- `scope` can be:
  - `exact`
  - `subtree`
  - `auto`
- `auto` uses `subtree` for directories and `exact` for files
- response includes:
  - `deviceId`
  - `root`
  - `entry`
  - `scope`
  - `referencingTasks`
  - `summary`
- each `referencingTasks` row is a `PublishTaskDiagnosticItem`
- `summary` includes:
  - `taskCount`
  - `readyCount`
  - `blockedCount`
  - `byStatus`
  - `byDimension`
  - `byIssueCode`

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
- each `deviceSyncs` item also includes:
  - `desiredRevision`
  - `isCurrent`
  - `needsSync`

### `GET /skills/{skillId}/impact?readiness=...&issueCode=...&status=...&limit=...`

- fetch one skill impact payload focused on publish tasks that currently reference this skill
- supports optional filters:
  - `readiness=ready|blocked`
  - `issueCode=<stable readiness code>`
  - `status=<task status>`
  - `limit`
- response includes:
  - `skill`
  - `items`
  - `summary`
  - `serverTime`
- each `items` row is a `PublishTaskDiagnosticItem`
- `summary` uses the same structure as `/tasks/diagnostics`

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

### `GET /tasks/diagnostics`

- list publish-task diagnostics with backend-computed readiness
- supports the same filters as `GET /tasks`
- extra filters:
  - `dimension=device|account|skill|materials`
  - `issueCode=<stable readiness code>`
- optional `readiness=ready|blocked`
- returns:
  - `items`
  - `summary`
  - `serverTime`
- each item includes:
  - `task`
  - `readiness`
  - `blockingDimensions`
- `summary` includes:
  - `totalCount`
  - `readyCount`
  - `blockedCount`
  - `byStatus`
  - `byDimension`
  - `byIssueCode`

### `POST /tasks/bulk-repair`

- batch remediation endpoint for publish tasks
- request supports:
  - `taskIds`
  - `operations`
  - `deviceId`
  - `status`
  - `platform`
  - `accountName`
  - `skillId`
  - `readiness=ready|blocked`
  - `dimension=device|account|skill|materials`
  - `issueCode`
  - `limit`
- valid `operations`:
  - `refresh_materials`
  - `refresh_skill`
- returns:
  - `items`
  - `summary`
  - `serverTime`
- each item includes:
  - `task`
  - `status=success|skipped|failed`
  - `message`
  - `appliedOperations`
  - `readinessBefore`
  - `readinessAfter`
  - optional `materialRefresh`
  - optional `skillRefresh`
- `summary` includes:
  - `selectedCount`
  - `processedCount`
  - `successCount`
  - `skippedCount`
  - `failedCount`
  - `byStatus`
  - `byOperation`

### `POST /tasks/bulk-action`

- batch operator action endpoint for publish tasks
- request supports:
  - `taskIds`
  - `action=cancel|retry|force_release|resume|manual_resolve`
  - `deviceId`
  - `status`
  - `platform`
  - `accountName`
  - `skillId`
  - `readiness=ready|blocked`
  - `dimension=device|account|skill|materials`
  - `issueCode`
  - `limit`
  - optional `message`
  - when `action=manual_resolve`:
    - `resolveStatus=success|completed|failed|cancelled`
    - optional `textEvidence`
    - optional `payload`
- returns:
  - `items`
  - `summary`
  - `serverTime`
- each item includes:
  - `taskBefore`
  - optional `taskAfter`
  - `status=success|skipped|failed`
  - `message`
  - `action`
  - optional `artifactCount`
- `summary` includes:
  - `selectedCount`
  - `processedCount`
  - `successCount`
  - `skippedCount`
  - `failedCount`
  - `byStatus`
  - `byAction`

### `POST /tasks`

- create a cloud publish task for a specific device and account
- current status starts as `pending`
- when `skillId` is provided, the backend stores `task.skillRevision` as a cloud-side skill snapshot
- `skillRevision` now reflects both the skill record and its attached assets
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
  - `bridge`
- `actions` exposes backend-computed booleans such as `canEdit`, `canCancel`, `canRetry`, `canDelete`
- `actions` also includes `canResume` and `canResolveManual` for `needs_verify` handling
- `actions` also includes:
  - `canRefreshMaterials`
  - `canRefreshSkill`
- `readiness` exposes backend-computed execution checks for device/account/skill/material availability
- `readiness.skillRevisionMatched` becomes `false` when the linked skill changed after task creation
- `readiness.skillSyncedToDevice` becomes `false` when the target device has not synced the linked skill revision successfully
- `readiness.driftedMaterialCount` counts mirrored materials whose current metadata no longer matches the task snapshot
- `readiness.issueCodes` exposes stable machine-friendly reasons such as:
  - `device_disabled`
  - `account_inactive`
  - `device_skill_missing`
  - `device_skill_outdated`
  - `skill_revision_changed`
  - `material_missing`
  - `material_drifted`
- `runtime` exposes the latest agent-side execution snapshot and `lastAgentSyncAt`
- `bridge` exposes a normalized cloud/local execution relationship view:
  - `origin=cloud|local|imported`
  - optional `localSource`
  - optional `stage`
  - optional `localStatus`
  - optional `workerName`
  - optional `updatedAt`
  - optional `startedAt`
  - optional `finishedAt`
  - optional `lastAgentSyncAt`
  - `hasActiveLease`

### `GET /tasks/{taskId}/events`

- fetch the publish task timeline
- includes cloud-side edits and agent-side execution / verification events

### `GET /tasks/{taskId}/artifacts`

- fetch structured task artifacts
- may include verification screenshots, text evidence, and future local output files

### `GET /tasks/{taskId}/materials`

- fetch mirrored material references attached to one task
- returns a snapshot of the local file metadata at selection time

### `POST /tasks/{taskId}/refresh-materials`

- rebuild the task's mirrored material snapshot from the latest mirrored file metadata on the target device
- rejects `running` and `cancel_requested` tasks
- intended to clear `material_drifted` after the operator confirms the local files changed as expected
- response includes:
  - `task`
  - `materials`
  - `readiness`
  - `refreshedCount`
  - `changedCount`
  - `missingCount`
  - `issues`
- if one or more mirrored files are now missing, the backend returns `409` with the same response shape and does not partially update the task snapshot

### `POST /tasks/{taskId}/refresh-skill`

- update `task.skillRevision` to the latest effective cloud revision of the linked skill
- rejects `running` and `cancel_requested` tasks
- rejects tasks that do not use a linked cloud skill
- intended to clear `skill_revision_changed` after the operator accepts the updated skill strategy
- response includes:
  - `task`
  - `skill`
  - `readiness`
  - `previousRevision`
  - `currentRevision`
  - `revisionChanged`
- note: after this call, `readiness.skillSyncedToDevice` may still be `false` until the target OmniBull syncs the new skill revision

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
- optional filters: `jobType`, `status`, `skillId`, `deviceId`, `source`, `limit`

### `POST /ai/jobs`

- create a queued AI job record
- request: `jobType`, `modelName`, optional `deviceId`, optional `skillId`, optional `prompt`, optional `inputPayload`
- optional advanced fields:
  - `source`
  - `localTaskId`
- when `skillId` is provided, the backend validates that the referenced skill belongs to the user and its `outputType` matches `jobType`
- `deviceId` here means the target `OmniBull` device for later task sync / publish handoff, not the cloud-side AI executor

### `GET /ai/jobs/{jobId}`

- fetch one AI job detail

### `GET /ai/jobs/{jobId}/workspace`

- fetch one AI job workspace payload
- includes:
  - `job`
  - `model`
  - `skill`
  - `artifacts`
  - `publishTasks`
  - `bridge`
  - `actions`
- `bridge` is the normalized cloud-to-OmniBull handoff view for this AI job:
  - `source`
  - `generationSide`
  - `targetDeviceId`
  - `localTaskId`
  - `localPublishTaskId`
  - `deliveryStage`
  - `artifactCount`
  - `mirroredArtifactCount`
  - `linkedPublishTaskCount`
- when `job.source = "omnibull_local"`, `deliveryStage` should be interpreted as:
  - `queued_generation`
  - `generating`
  - `awaiting_omnibull_import`
  - `mirrored_to_omnibull`
  - `publish_queued_on_omnibull`
  - `publishing_on_omnibull`
  - `published_on_omnibull`
  - `publish_failed_on_omnibull`
  - `publish_needs_verify_on_omnibull`

### `GET /ai/jobs/{jobId}/artifacts`

- fetch structured AI output artifacts
- each artifact may optionally include local mirror information:
  - `deviceId`
  - `rootName`
  - `relativePath`
  - `absolutePath`
- this is how the cloud backend knows whether one generated output is already mirrored onto a target `OmniBull` device

### `POST /ai/jobs/{jobId}/artifacts/upload`

- upload one AI output artifact into cloud storage
- uses `multipart/form-data`
- fields:
  - `artifactType`
  - optional `artifactKey`
  - optional `source`
  - optional `title`
  - `file`
  - optional `deviceId`
  - optional `rootName`
  - optional `relativePath`
  - optional `absolutePath`
- when `deviceId` is provided, `rootName` and `relativePath` are also required
- this supports the real chain where OmniDrive generates media first, then later marks that output as mirrored into one `OmniBull` material root

### `POST /ai/jobs/{jobId}/publish-task`

- create one publish task from completed AI outputs
- request:
  - optional `deviceId`
  - optional `accountId`
  - `platform`
  - `accountName`
  - optional `title`
  - optional `contentText`
  - optional `artifactKeys`
  - optional `runAt`
- the backend only allows this when:
  - AI job status is `success` or `completed`
  - selected AI artifacts are already mirrored to the chosen `OmniBull` device
- this is primarily for cloud-native AI jobs
- when `job.source = "omnibull_local"`, the preferred chain is:
  - OmniBull creates the local AI task
  - OmniDrive generates the output
  - OmniBull pulls the output back locally
  - SAU publishes locally
- on success, the backend:
  - creates a publish task
  - attaches mirrored material refs
  - records `created_from_ai_job` in the publish-task timeline
  - links the AI job and publish task in cloud history

### `PATCH /ai/jobs/{jobId}`

- update editable AI job fields
- supports:
  - `deviceId`
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
- clears previous AI artifacts so the next generation run starts clean

### `POST /ai/jobs/{jobId}/force-release`

- manually releases a running AI job lease from the cloud side
- current purpose is to support later real executor recovery, even though current chain is still cloud-generation-first

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

### `GET /agent/ai-jobs/{deviceCode}`

- local agent polls AI jobs that belong to one claimed device
- current phase is primarily for `source = omnibull_local`
- returns items with:
  - `job`
  - `artifacts`
  - `bridge`
  - `actions`

### `POST /agent/ai-jobs/sync`

- local OmniBull syncs one locally-created AI task into OmniDrive
- request body:
  - `id` as the local OmniBull task UUID
  - `deviceCode`
  - optional `skillId`
  - `jobType`
  - `modelName`
  - optional `prompt`
  - optional `inputPayload`
  - optional `publishPayload`
  - optional `status`
  - optional `message`
  - optional `runAt`
- backend behavior:
  - creates a cloud AI job with `source = omnibull_local` when this local task is first seen
  - binds `localTaskId`
  - keeps `deviceId` as the target OmniBull device for later result delivery

### `POST /agent/ai-jobs/{jobId}/delivery`

- local OmniBull acknowledges cloud AI result handoff progress
- request body:
  - `deviceCode`
  - `status`
  - optional `message`
  - optional `localPublishTaskId`
  - optional `deliveredAt`
- current delivery statuses include:
  - `imported`
  - `publish_queued`
  - `publishing`
  - `success`
  - `needs_verify`
  - `failed`
  - `cancelled`

### `GET /agent/skills/{deviceCode}?since=...&limit=...`

- local agent pulls enabled product skills for one claimed device
- response includes:
  - `items`
  - `retiredItems`
  - `summary`
  - `serverTime`
- each active item includes:
  - `revision`
  - `skill`
  - `assets`
  - optional current `sync` state for that device
- each `retiredItems` row includes:
  - `skillId`
  - `reason` in `disabled | deleted`
  - optional `name`
  - optional `outputType`
  - optional `message`
  - optional `syncedRevision`
  - optional `lastSyncedAt`
  - `lastChangedAt`
- `summary` includes:
  - `activeCount`
  - `retiredCount`
  - `disabledCount`
  - `deletedCount`
- `revision` reflects the effective cloud skill version, including attached asset changes
- the related workspace sync-state objects expose `desiredRevision`, `isCurrent`, and `needsSync`
- optional `since` must be RFC3339

### `POST /agent/skills/retired-ack`

- local agent acknowledges that one or more retired skills were removed from the local cache
- request:
  - `deviceCode`
  - `items`
- each item supports:
  - `skillId`
  - `reason` in `disabled | deleted`
  - optional `message`
  - optional `acknowledgedAt` in RFC3339
- after a successful ack, already-acknowledged retired skills are no longer returned by `/agent/skills/{deviceCode}` unless the cloud-side change happens again later

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
- default response only includes tasks whose backend `readiness` currently allows execution
- optional `includeBlocked=true` returns:
  - `readyItems`
  - `blockedItems`
  - `summary`
  - `serverTime`
- each blocked item includes:
  - `task`
  - `readiness`
  - `blockingDimensions`
- `summary` includes:
  - `readyCount`
  - `blockedCount`
  - `byStatus`
  - `byDimension`
  - `byIssueCode`
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
- `task.skillRevision` is the cloud-side skill snapshot captured when the task was created
- `readiness` uses the same skill-revision and material-drift checks as `/tasks/{taskId}/workspace`
- may include `runtime` so SAU or OpenClaw can inspect the latest local execution snapshot
- intended to give SAU one complete execution payload before talking to third-party platforms

### `POST /agent/publish-tasks/{taskId}/claim`

- request body: `deviceCode`
- atomically moves a `pending` task into `running`
- tasks with future `runAt` cannot be claimed yet
- disabled devices cannot claim tasks
- if backend readiness is not satisfied, claim is rejected with `409` and the response includes the same `readiness` payload returned by task workspace / package
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
- request body also supports:
  - `skillRevision`
  - `runAt`
  - `finishedAt`
  - `materialRefs`
- supports optional `artifacts` for screenshots, logs, previews, and other structured evidence
- supports optional `executionPayload` for current local progress, step, or executor context
- when `materialRefs` is provided, the backend replaces the mirrored task material snapshot using the device's current mirrored file entries
- local OpenClaw / OmniBull tasks can therefore be mirrored into OmniDrive with enough material context to participate in readiness diagnostics
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
