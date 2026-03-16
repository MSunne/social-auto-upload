# OmniDrive Cloud

This is the production-oriented cloud backend for `OmniDrive`.

## Stack

- Go 1.26+
- chi router
- PostgreSQL
- Redis
- S3-compatible object storage

## Why This Service Exists

- `OmniDrive` is a cloud control plane.
- `OmniBull` is a local execution agent installed on many user machines.
- These are different deployment units and should not share one runtime process.

## Phase 1 Scope

- user auth
- device claim and heartbeat
- dashboard overview and history feed
- mirrored platform accounts
- mirrored local material roots and file previews
- remote login sessions
- product skills
- publish task mirror
- AI model registry and AI job queue records
- billing package and ledger read APIs

## Local Run

```bash
cd /Volumes/mud/project/github/social-auto-upload/omnidrive_cloud
OMNIDRIVE_DATABASE_DSN='postgres://postgres:YOUR_PASSWORD@127.0.0.1:5432/omnidrive?sslmode=disable' go run ./cmd/omnidrive-bootstrap-db
go run ./cmd/omnidrive-api
```

## What Works Now

- user register / login / `me`
- device heartbeat, claim, list, update
- device list and detail now include workload counters for mirrored accounts, materials, tasks, and login sessions
- device workspace endpoint aggregates recent tasks, active login sessions, recent accounts, material roots, and device skill sync states
- device and skill workspace sync-state rows now expose `desiredRevision`, `isCurrent`, and `needsSync` so the UI can show whether a target OmniBull is truly on the latest skill package
- account mirror list
- account list and detail now include workload counters for related tasks and active login sessions
- account workspace endpoint aggregates recent tasks and active login sessions for one mirrored platform account
- account delete is guarded and returns usage summary when tasks or active verification sessions still reference that account
- remote login session create and query
- remote login action queue for second-factor input
- local agent polling for login tasks and actions
- agent-driven account state sync
- local agent pushing login result back to cloud
- successful login event mirroring back into platform account state
- product skill asset metadata
- skill list and detail now include workload counters for attached assets and related publish tasks
- skill workspace endpoint aggregates attached assets, recent publish tasks, recent AI jobs, and device sync states for one skill
- skill impact endpoint can now return the full publish-task diagnostics set for one skill, including blocked summaries by issue code
- product skill multipart upload with public file URL
- effective skill revision now includes attached asset changes, and skill asset creation bumps the parent skill update time for incremental sync
- skill detail and guarded delete
- publish task create, detail, update, delete, device polling, and task status sync
- publish task workspace endpoint aggregates related device/account/skill, timeline, artifacts, materials, and backend-computed action flags
- publish task workspace action flags now also expose `canRefreshMaterials` and `canRefreshSkill`
- publish task workspace now also includes a backend-computed readiness block so cloud and local executors can detect missing materials or disabled dependencies early
- publish tasks now store a `skillRevision` snapshot and readiness can flag when the linked skill has changed since task creation
- readiness also checks whether the target OmniBull device has already synced the linked skill revision successfully
- readiness can also flag mirrored material drift when current file metadata no longer matches the task's material snapshot
- `needs_verify` tasks can now either be resumed back to `pending` or manually resolved into a final state with structured evidence
- cloud-side force-release endpoint can manually free a stuck leased publish task before the lease naturally expires
- task workspace and agent package can now expose a lightweight runtime snapshot from the local executor, such as current step or progress
- publish task event timeline for cloud edits and agent execution evidence
- structured publish task artifacts for verification screenshots and future outputs
- task-to-material snapshot references for mirrored local files
- agent-side publish-task package endpoint that resolves task, account, skill, skill assets, and materials into one execution payload
- agent-side publish-task package also includes readiness checks for device/account/skill/material availability
- agent-side claim now uses the same readiness guard, so unsynced skills or drifted materials are blocked before a lease is granted
- agent-side task polling now returns only executable tasks by default; `includeBlocked=true` exposes blocked tasks with readiness reasons for diagnostics
- task diagnostics now expose structured `issueCodes`, `blockingDimensions`, and summary counters so cloud-side triage does not need to parse Chinese error strings
- both task diagnostics and blocked queue summaries now also expose `byIssueCode`, which is useful for operator dashboards and bulk remediation hints
- cloud task center now also supports batch remediation, so one operator action can refresh material snapshots or skill revisions for many blocked tasks at once
- agent-side blocked queue diagnostics now also expose a `summary` object with ready/blocked counts and per-dimension counts
- publish task lease claim / renew flow for safer device-side execution
- agent-side lease release endpoint so local SAU can requeue work or confirm cancellation without waiting for TTL expiry
- `runAt` is respected by device polling and claim, so future tasks are not executed early
- invalid agent-side status regressions are rejected with `409`
- retry clears old task artifacts so each new attempt starts clean
- retry and delete attempt to remove stored artifact files from local object storage
- repeated artifact sync with the same `artifactKey` replaces the old stored file and cleans up the previous object when possible
- disabled devices can stay online but cannot poll or claim new cloud work
- deleting a skill also attempts to remove uploaded skill asset files from local object storage
- expired running leases auto-recover so tasks do not remain stuck forever
- material root, directory, and file-preview mirror APIs for local OmniBull content browsing
- material workspace endpoint can now show which publish tasks depend on a given mirrored file or directory subtree, including blocked/ready diagnostics
- cloud operators can now refresh a task's material snapshot after mirrored files drift, without deleting and recreating the task
- cloud operators can now refresh a task's linked skill revision snapshot after skill assets or prompts change, without deleting and recreating the task
- agent-side skill package pull and skill sync-state push APIs for local OmniBull / OpenClaw consumers
- agent-side skill pull now also returns `retiredItems`, so disabled or deleted skills can be actively removed from the local agent cache
- local agents can now acknowledge retired-skill cleanup, which suppresses already-handled disabled/deleted items until the cloud-side change happens again
- local OmniBull can now mirror locally-created publish tasks back into OmniDrive, including material references, so OpenClaw-triggered tasks show up in the cloud task center with readiness diagnostics
- agent-side publish-task sync now also accepts `skillRevision`, `runAt`, `finishedAt`, and `materialRefs`, which lets one local task stay traceable across OmniDrive, OmniBull, OpenClaw, and the real third-party platform executor
- verification screenshot extraction and file URL generation during task sync
- `needs_verify` tasks are retained for manual handling but no longer re-polled as executable device tasks
- device/task mismatch during agent sync is rejected with `409`
- active lease token mismatch during agent sync is rejected with `409`
- task list filtering by device, status, platform, account name, and limit
- dashboard summary and merged history feed
- dashboard summary now includes task breakdown counters and active login-session count for richer control-center cards
- dashboard summary now also includes AI queue/running/failed counters
- history feed supports filtering by kind, status, and limit
- cloud-side audit trail for device, skill, task, AI job, and login-session actions
- AI model listing, AI job create/list/detail
- AI job workspace, update, cancel, and retry endpoints for future model executors and richer cloud-side operations
- AI jobs can optionally reference a product skill, and skill deletion is guarded against both publish-task and AI-job dependencies
- AI job list supports `jobType/status/skillId/limit` filtering for richer cloud control views
- billing package list and wallet ledger read
