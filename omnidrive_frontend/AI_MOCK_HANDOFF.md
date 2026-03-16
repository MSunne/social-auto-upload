# AI Mock Handoff

This file is for the frontend thread and matches the current `omnidrive_cloud` backend fields.

## File

Use this fixture file first:

- `/Volumes/mud/project/github/social-auto-upload/omnidrive_frontend/lib/mock-ai-data.ts`

It already includes:

- model list for `chat / image / video`
- AI job list rows
- AI job workspace payloads
- AI artifacts
- linked publish task example
- billing usage events
- page-friendly chat/image/video history data

## Recommended Mapping

Current login demo data stays in:

- `/Volumes/mud/project/github/social-auto-upload/omnidrive_frontend/lib/mock-data.ts`

Current AI pages can switch to these exports:

- chat page:
  - `mockChatMessages`
  - `mockAiJobWorkspaces["ai_chat_20260316_001"]`
- image page:
  - `mockImageHistoryCards`
  - `mockAiModels.filter((item) => item.category === "image")`
  - `mockAiJobWorkspaces["ai_image_20260316_001"]`
- video page:
  - `mockVideoHistoryCards`
  - `mockAiModels.filter((item) => item.category === "video")`
  - `mockAiJobWorkspaces["ai_video_20260316_001"]`
  - `mockAiJobWorkspaces["ai_video_20260316_002"]`

## Real API Mapping

These mock exports are shaped for the real backend endpoints below:

- `GET /api/v1/ai/models?category=chat|image|video`
- `GET /api/v1/ai/jobs?jobType=chat|image|video`
- `GET /api/v1/ai/jobs/{jobId}/workspace`
- `GET /api/v1/ai/jobs/{jobId}/artifacts`
- `GET /api/v1/billing/usage-events?sourceType=ai_job&sourceId={jobId}`

The fixture file also exports `mockAiFixturesByEndpoint`, so pages or temporary mock services can map endpoint strings directly to response data.

## Notes

- Chat model is `gemini-3.1-pro-preview`.
- Image model is `gemini-3-pro-image-preview`.
- Video model is `veo-3.1-fast-fl`.
- `billingUsageEvents` is already embedded inside AI workspace and does not require a second request for detail pages unless a separate billing screen wants it.
- The image workspace example includes one mirrored artifact plus one linked publish task.
- The video examples intentionally include both:
  - one `running` job
  - one `success but billing failed` job

This lets the frontend cover the three most important UI states:

- generating
- generated successfully
- generated successfully but billing still needs attention
