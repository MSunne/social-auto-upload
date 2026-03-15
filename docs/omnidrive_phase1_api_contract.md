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

### `POST /devices/claim`

- request: `deviceCode`
- claims an already-online OmniBull device into the current user account

### `PATCH /devices/{deviceId}`

- fields: `name`, `defaultReasoningModel`, `isEnabled`

## Accounts

### `GET /accounts?deviceId=...`

- list mirrored platform accounts

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

## Skills

### `GET /skills`

- list product skills under the current user

### `POST /skills`

- create skill

### `PATCH /skills/{skillId}`

- update skill

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

### `POST /tasks`

- create a cloud publish task for a specific device and account
- current status starts as `pending`

### `GET /tasks/{taskId}`

- fetch publish task detail, including verification payload if present

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

### `POST /agent/login-sessions/{sessionId}/event`

- push QR updates, verification updates, success, or failure

### `GET /agent/login-sessions/{sessionId}/actions`

- the local agent consumes pending verification actions for a login session
- current implementation behaves like a one-time queue

### `GET /agent/publish-tasks/{deviceCode}`

- local agent polls pending or in-progress publish tasks for the device

### `POST /agent/publish-tasks/sync`

- local agent mirrors task execution state back to cloud
- supports task creation from local side or updating an existing cloud task
- intended for `pending`, `running`, `success`, `failed`, `needs_verify` and similar states
- if `verificationPayload.screenshotData` is provided as base64 or data URL, the backend stores it and rewrites the payload to `screenshotUrl` and storage metadata

## Public File Delivery

### `GET /files/{storageKey}`

- serves uploaded skill assets and stored verification screenshots
- current phase uses the OmniDrive API process as the file server

## Phase 1 Response Rules

- all list endpoints return arrays
- all timestamps use ISO 8601 UTC strings
- device and task status must be explicit enum values
- verification payloads always keep screenshots and button actions in structured JSON
