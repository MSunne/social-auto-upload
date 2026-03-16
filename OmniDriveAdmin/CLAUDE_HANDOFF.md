# Claude Handoff: OmniDriveAdmin

This directory is reserved for Claude to build the internal admin UI.

## Product Position

- `OmniDriveAdmin` is the internal operations console
- it is not customer-facing
- it is not the local SAU frontend

Primary users:

- operations
- finance
- support
- audit
- engineering support

## Backend Plan

The backend implementation plan lives here:

- `/Volumes/mud/project/github/social-auto-upload/docs/omnidrive_admin_implementation_plan.md`

The current callable admin API contract lives here:

- `/Volumes/mud/project/github/social-auto-upload/docs/omnidrive_admin_api_contract.md`

The admin backend should be served from the existing Go service under:

- `/api/admin/v1/*`

## Design Direction

Please make this feel like a serious operations product, not a generic SaaS dashboard.

Preferred visual language:

- editorial operations room
- graphite / bone / oxidized teal / amber palette
- strong typography hierarchy
- dense but calm data tables
- visible financial status states
- clearer distinction between danger, pending review, and settled states

Avoid:

- neon cyber style copied from the customer console
- purple-heavy UI
- over-rounded toy-like cards
- overly playful empty states

Recommended feel:

- Bloomberg terminal meets modern audit dashboard
- sharp spacing
- deliberate table and ledger design
- side-by-side detail drawers

## Route Priorities

Build these first:

1. `/dashboard`
2. `/support-recharges`
3. `/distribution/commissions`
4. `/distribution/settlements`
5. `/orders`
6. `/wallet-ledgers`
7. `/users`
8. `/devices`

Then continue with:

- `/media-accounts`
- `/publish-tasks`
- `/ai-jobs`
- `/skills`
- `/withdrawals`
- `/audits`
- `/settings`
- `/admins`

## UX Priorities

1. Support recharge review must feel like a formal approval workflow.
2. Commission states must clearly show:
   - pending consume
   - pending settlement
   - settled
3. Finance pages should privilege:
   - summary metrics
   - filters
   - ledgers
   - detail drawers
4. Task and AI pages should make failure reasons easy to scan.
5. User detail should cross-link finance, devices, tasks, and media accounts.
6. Audit pages should make operator, action, and before/after snapshots discoverable.

## Suggested Frontend Information Architecture

- Dashboard
- Users
- Devices
- Media Accounts
- Publish Tasks
- AI Jobs
- Skills
- Pricing
- Orders
- Wallet Ledgers
- Support Recharges
- Distribution Relations
- Distribution Commissions
- Distribution Settlements
- Withdrawals
- Audits
- Settings
- Admins

## Notes

- Keep components reusable across tables, filters, drawers, and approval panels.
- Admin UIs should optimize for dense information and trust, not marketing polish.
- If you need API mocks first, mirror the route names above so backend integration stays straightforward.
- Support recharge status set is now `awaiting_submission | pending_review | rejected | credited`.
- Support recharge detail/review routes are available under:
  - `GET /api/admin/v1/support-recharges/:orderId`
  - `GET /api/admin/v1/support-recharges/:orderId/events`
  - `POST /api/admin/v1/support-recharges/:orderId/credit`
  - `POST /api/admin/v1/support-recharges/:orderId/reject`
