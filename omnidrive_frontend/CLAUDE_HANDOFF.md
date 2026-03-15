# Claude Frontend Handoff

This directory is prepared for Claude Opus to implement the OmniDrive frontend.

## Product Position

- `OmniDrive`: cloud console
- `OmniBull / SAU`: local execution agent
- `OpenClaw`: local intelligent agent using `OmniSkill` and `SauSkill`
- `LocaWeb`: local backup management page

## Current Backend Status

The cloud backend is implemented in:

- `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud`

The primary API contract is:

- `/Volumes/mud/project/github/social-auto-upload/docs/omnidrive_phase1_api_contract.md`

## Screens Ready For Frontend Work

Start with these modules:

1. Device management
2. Social account management
3. Remote login session modal and second-factor actions
4. Product skill management
5. Task center

## Design Sources

All Stitch exports are already downloaded here:

- `/Volumes/mud/project/github/social-auto-upload/stitch_exports/redesign_strategy_plan`

Key screens to prioritize:

- `16_c91deda775fb473fb63320b49380ef25.png` control center
- `11_87d605e025da44b29e64b1b529c3646c.png` OmniBull account detail
- `14_b329c80d2ad04451abb415732b7387f0.png` OmniBull management detail
- `21_f62ed396c6ae4a99a0b06d069d5d6c11.png` product knowledge and skill management
- `17_c98b19117025404bb80547723cd47e0a.png` all tasks
- `08_6dbd68f1a4434aadb8c1f93892593a89.png` video task
- `19_ee22f39d0cbe46db86b9fb5f8f1a130a.png` image task

## Suggested Route Map

- `/dashboard`
- `/devices`
- `/devices/[deviceId]`
- `/devices/[deviceId]/accounts`
- `/devices/[deviceId]/skills`
- `/tasks`
- `/tasks/[taskId]`
- `/billing`
- `/history`

## Core API Groups Already Available

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/auth/me`
- `GET /api/v1/devices`
- `POST /api/v1/devices/claim`
- `PATCH /api/v1/devices/{deviceId}`
- `GET /api/v1/accounts`
- `POST /api/v1/accounts/remote-login`
- `GET /api/v1/accounts/login-sessions/{sessionId}`
- `POST /api/v1/accounts/login-sessions/{sessionId}/actions`
- `GET /api/v1/skills`
- `POST /api/v1/skills`
- `PATCH /api/v1/skills/{skillId}`
- `GET /api/v1/skills/{skillId}/assets`
- `POST /api/v1/skills/{skillId}/assets`
- `POST /api/v1/skills/{skillId}/upload`
- `GET /api/v1/tasks`
- `POST /api/v1/tasks`
- `GET /api/v1/tasks/{taskId}`

## UX Priorities

1. Device online status must be obvious.
2. Remote login modal must support QR display and second-factor action buttons.
3. Task detail must display `needs_verify` clearly.
4. Skill pages should show both metadata and attached asset previews.

## Notes For Implementation

- The backend returns ISO 8601 timestamps.
- Device and task statuses are explicit strings.
- Verification payloads are structured JSON; do not hardcode a single shape.
- Skill file upload uses `multipart/form-data` and returns a previewable `publicUrl`.
- Task verification payloads may include `screenshotUrl` instead of raw base64.
- Frontend should keep list pages and detail drawers synchronized with query invalidation.
