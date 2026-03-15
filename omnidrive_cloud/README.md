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
- mirrored platform accounts
- remote login sessions
- product skills
- publish task mirror

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
- local agent pushing login result back to cloud
- successful login event mirroring back into platform account state
- product skill asset metadata
- product skill multipart upload with public file URL
- publish task create, detail, device polling, and task status sync
- verification screenshot extraction and file URL generation during task sync
