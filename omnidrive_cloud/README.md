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
- account mirror list
- remote login session create and query
- remote login action queue for second-factor input
- local agent polling for login tasks and actions
- agent-driven account state sync
- local agent pushing login result back to cloud
- successful login event mirroring back into platform account state
- product skill asset metadata
- product skill multipart upload with public file URL
- skill detail and guarded delete
- publish task create, detail, update, delete, device polling, and task status sync
- publish task event timeline for cloud edits and agent execution evidence
- structured publish task artifacts for verification screenshots and future outputs
- task-to-material snapshot references for mirrored local files
- publish task lease claim / renew flow for safer device-side execution
- `runAt` is respected by device polling and claim, so future tasks are not executed early
- invalid agent-side status regressions are rejected with `409`
- retry clears old task artifacts so each new attempt starts clean
- expired running leases auto-recover so tasks do not remain stuck forever
- material root, directory, and file-preview mirror APIs for local OmniBull content browsing
- verification screenshot extraction and file URL generation during task sync
- `needs_verify` tasks are retained for manual handling but no longer re-polled as executable device tasks
- device/task mismatch during agent sync is rejected with `409`
- active lease token mismatch during agent sync is rejected with `409`
- task list filtering by device, status, platform, account name, and limit
- dashboard summary and merged history feed
- cloud-side audit trail for device, skill, task, AI job, and login-session actions
- AI model listing, AI job create/list/detail
- billing package list and wallet ledger read
