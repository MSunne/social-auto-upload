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

1. Dashboard overview
2. Device management
3. Social account management
4. Remote login session modal and second-factor actions
5. Material browser
6. Product skill management
7. Task center
8. History and billing

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
- `GET /api/v1/overview/summary`
- `GET /api/v1/history`
- `GET /api/v1/history?kind=...&status=...&limit=...`
- `GET /api/v1/devices`
- `GET /api/v1/devices/{deviceId}`
- `GET /api/v1/devices/{deviceId}/workspace`
- `POST /api/v1/devices/claim`
- `PATCH /api/v1/devices/{deviceId}`
- `GET /api/v1/accounts`
- `GET /api/v1/accounts/{accountId}`
- `GET /api/v1/accounts/{accountId}/workspace`
- `DELETE /api/v1/accounts/{accountId}`
- `POST /api/v1/accounts/{accountId}/validate`
- `POST /api/v1/accounts/remote-login`
- `GET /api/v1/accounts/login-sessions/{sessionId}`
- `POST /api/v1/accounts/login-sessions/{sessionId}/actions`
- `GET /api/v1/materials/roots`
- `GET /api/v1/materials/list`
- `GET /api/v1/materials/file`
- `GET /api/v1/materials/workspace`
- `GET /api/v1/skills`
- `POST /api/v1/skills`
- `GET /api/v1/skills/{skillId}`
- `GET /api/v1/skills/{skillId}/workspace`
- `GET /api/v1/skills/{skillId}/impact`
- `PATCH /api/v1/skills/{skillId}`
- `DELETE /api/v1/skills/{skillId}`
- `GET /api/v1/skills/{skillId}/assets`
- `POST /api/v1/skills/{skillId}/assets`
- `POST /api/v1/skills/{skillId}/upload`
- `GET /api/v1/tasks`
- `GET /api/v1/tasks/diagnostics`
- `POST /api/v1/tasks/bulk-repair`
- `POST /api/v1/tasks`
- `GET /api/v1/tasks/{taskId}`
- `GET /api/v1/tasks/{taskId}/workspace`
- `GET /api/v1/tasks/{taskId}/events`
- `GET /api/v1/tasks/{taskId}/artifacts`
- `GET /api/v1/tasks/{taskId}/materials`
- `POST /api/v1/tasks/{taskId}/refresh-materials`
- `POST /api/v1/tasks/{taskId}/refresh-skill`
- `POST /api/v1/tasks/{taskId}/cancel`
- `POST /api/v1/tasks/{taskId}/force-release`
- `POST /api/v1/tasks/{taskId}/resume`
- `POST /api/v1/tasks/{taskId}/manual-resolve`
- `POST /api/v1/tasks/{taskId}/retry`
- `PATCH /api/v1/tasks/{taskId}`
- `DELETE /api/v1/tasks/{taskId}`
- `GET /api/v1/ai/models`
- `GET /api/v1/ai/jobs`
- `POST /api/v1/ai/jobs`
- `GET /api/v1/ai/jobs/{jobId}`
- `GET /api/v1/ai/jobs/{jobId}/workspace`
- `PATCH /api/v1/ai/jobs/{jobId}`
- `POST /api/v1/ai/jobs/{jobId}/cancel`
- `POST /api/v1/ai/jobs/{jobId}/retry`
- `GET /api/v1/billing/packages`
- `GET /api/v1/billing/ledger`

## UX Priorities

1. Device online status must be obvious.
2. Device cards and device detail pages should render the nested `device.load` counters.
3. Device detail can call `/devices/{deviceId}/workspace` to populate recent tasks, recent accounts, active login sessions, material roots, and `skillSyncStates` in one round-trip.
Device `skillSyncStates` now also expose `desiredRevision`, `isCurrent`, and `needsSync`, which is the safest way to render “已同步 / 待同步 / 已过期”.
4. Remote login modal must support QR display and second-factor action buttons.
5. Account list and account detail pages should render the nested `account.load` counters.
6. Account detail can call `/accounts/{accountId}/workspace` to show related publish tasks and active verification sessions.
7. Skill list and skill detail pages should render the nested `skill.load` counters.
8. Skill detail can call `/skills/{skillId}/workspace` to get attached assets, recent dependent publish tasks, recent AI jobs, and `deviceSyncs` in one request.
Skill detail and skill-management drilldowns can call `/skills/{skillId}/impact` when they need the full blocked/ready publish-task set for one skill, not just the recent 8 tasks from workspace.
Skill-related UI should treat `task.skillRevision` as an effective version that also changes when skill assets change, not only when the skill form itself changes.
Skill `deviceSyncs` now also expose `desiredRevision`, `isCurrent`, and `needsSync`.
9. Task detail can call `/tasks/{taskId}/workspace` to get the related device, account, skill, events, artifacts, materials, backend-computed action flags, `readiness` checks, and optional `runtime` snapshot in one request.
Task detail should surface `task.skillRevision`, `readiness.skillRevisionMatched`, and `readiness.driftedMaterialCount` so operators can see when a task's skill or mirrored materials have drifted since creation.
Task detail should also surface `readiness.skillSyncedToDevice`, because a task can look valid in the cloud but still be blocked until the target OmniBull has synced the linked skill revision.
Task detail and diagnostics views should also surface `readiness.issueCodes`, because these are the stable machine-readable reasons for badges, filters, and empty-state CTAs.
10. Task detail must display `needs_verify` clearly.
11. Task detail should also show the event timeline from `/tasks/{taskId}/events`.
12. Task detail should also render task artifacts from `/tasks/{taskId}/artifacts`, especially verification screenshots and text evidence.
13. Task detail should also render selected input materials from `/tasks/{taskId}/materials`.
14. Task detail should support explicit cancel, retry, force-release, resume, manual-resolve, refresh-materials, and refresh-skill actions and can trust the backend `actions` booleans from `/tasks/{taskId}/workspace`.
If `readiness.driftedMaterialCount > 0`, prefer showing a “刷新素材快照” CTA that calls `POST /tasks/{taskId}/refresh-materials`.
If `readiness.skillRevisionMatched === false`, prefer showing a “刷新技能版本” CTA that calls `POST /tasks/{taskId}/refresh-skill`.
After `refresh-skill`, the task may still remain blocked until `readiness.skillSyncedToDevice` becomes `true`; the UI should explain that the local OmniBull still needs to sync the new cloud skill package.
If the local device tries to claim a task while readiness is still false, `/agent/publish-tasks/{taskId}/claim` now returns `409` with the same `readiness` payload, so frontend copy and badges should stay consistent with task detail.
Internal tooling that inspects agent queues can use `/agent/publish-tasks/{deviceCode}?includeBlocked=true` to surface blocked tasks and their readiness reasons.
The blocked queue response now also includes `summary` and each blocked item exposes `blockingDimensions`, which is more reliable than inferring categories from free-form text.
Task center list pages can use `/tasks/diagnostics` when they need one query that includes both task rows and backend-computed readiness / blocking information.
Both `/tasks/diagnostics` and `includeBlocked=true` queue responses now expose `summary.byIssueCode`, which is the best source for aggregate issue badges like “账号失效 12 条” or “素材漂移 5 条”.
`/tasks/diagnostics` also supports backend-side filtering by `dimension` and `issueCode`, so task-center tabs and filters do not need to rescan the full diagnostics list on the client.
Task center bulk actions can call `/tasks/bulk-repair` with the same filtering dimensions when an operator wants to repair many blocked tasks together.
The bulk-repair response already includes per-task `status`, `message`, `appliedOperations`, `readinessBefore`, and `readinessAfter`, so the UI can show a batch-result drawer without refetching every row immediately.
15. Materials page should let users switch by device, root, and path, with file preview for text content.
Materials detail can now call `/materials/workspace?deviceId=...&root=...&path=...&scope=auto` to show not only the mirrored file/directory itself, but also which publish tasks currently depend on it and whether those tasks are blocked.
16. Skill pages should show both metadata and attached asset previews.
Agent-side skill sync tooling should treat `/agent/skills/{deviceCode}` as a manifest, not just a download list: `items` are active skills to install/update, while `retiredItems` are disabled or deleted skills that should be removed from the local cache.
If the local agent implements cleanup confirmation, it can call `POST /agent/skills/retired-ack` after removing those retired skills; acknowledged retired items stop appearing in later manifests until the cloud changes again.
17. Skill delete should surface the backend `409` usage summary instead of silently failing.
18. Account delete should also surface the backend `409` usage summary instead of silently failing.
19. Dashboard cards should read directly from `/overview/summary`, including material counts, task breakdown counts, `activeLoginSessionCount`, and AI breakdown counts.
20. History should render mixed item types using `kind`, `source`, and `status`, including `audit` items, and can use backend filters for tabs.
21. Billing can start read-only with package cards and ledger table.
22. AI detail pages can call `/ai/jobs/{jobId}/workspace` for model metadata, optional linked skill, and backend action flags, then use `PATCH/cancel/retry` to drive the lifecycle UI.
23. AI job create/update can optionally bind `skillId`; when used, the chosen skill must match the job `jobType`.
24. Skill delete should surface the backend `409` usage summary for both publish-task and AI-job references.
25. AI list pages can use backend filters `jobType/status/skillId/limit` instead of client-side slicing.

## Notes For Implementation

- The backend returns ISO 8601 timestamps.
- Device and task statuses are explicit strings.
- Verification payloads are structured JSON; do not hardcode a single shape.
- Skill file upload uses `multipart/form-data` and returns a previewable `publicUrl`.
- Task verification payloads may include `screenshotUrl` instead of raw base64.
- Task list supports optional filters: `deviceId`, `status`, `platform`, `accountName`, `limit`.
- Task create and task update both support optional `materialRefs`, each item using `root`, `path`, and optional `role`.
- Frontend should keep list pages and detail drawers synchronized with query invalidation.
- AI jobs are currently queue records; build UI around creation, listing, and detail, not around immediate completion.
