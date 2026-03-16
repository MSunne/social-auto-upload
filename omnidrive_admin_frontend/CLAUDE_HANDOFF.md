# Claude Handoff For OmniDriveAdmin

This directory is the dedicated frontend project for the internal admin console.

## Positioning

This is not the customer-facing `OmniDrive` console.

This frontend is for:

- internal operations staff
- finance staff
- customer support
- auditors
- administrators

## Project Goal

Create a polished internal management console for:

- user operations
- device operations
- publish-task intervention
- finance and wallet management
- support recharge review
- distribution and commission settlement
- audit and system settings

## Tech Stack

- Next.js App Router
- React 19
- TypeScript
- Tailwind CSS v4
- TanStack Query
- axios
- zustand

## Route Map

- `/login`
- `/dashboard`
- `/users`
- `/devices`
- `/accounts`
- `/publish-tasks`
- `/ai-jobs`
- `/skills`
- `/pricing`
- `/orders`
- `/wallet-ledgers`
- `/support-recharges`
- `/distribution/relations`
- `/distribution/commissions`
- `/distribution/settlements`
- `/withdraws`
- `/audits`
- `/settings`
- `/admins`

## UI Direction

This is an internal console, but it should still feel premium and intentional.

- visual tone:
  - calm, confident, dark-leaning admin workspace
  - avoid generic neon-cyber styles
  - prefer graphite, deep teal, muted gold, desaturated red, and soft green as status accents
- typography:
  - use expressive but professional sans choices
  - avoid default-looking system UI
- layout:
  - persistent left navigation
  - sticky top context bar
  - dense but readable tables
  - filter trays and detail drawers
- components:
  - command-friendly search/filter bars
  - metric cards
  - exception queues
  - settlement and review drawers
  - strong empty states with next actions

## Backend Alignment

The backend plan is here:

- `/Volumes/mud/project/github/social-auto-upload/docs/omnidrive_admin_implementation_plan.md`

The current admin frontend should expect future APIs under:

- `/api/admin/v1/*`

## First Screens To Prioritize

1. dashboard
2. support recharge review
3. distribution commissions
4. orders
5. wallet ledgers
6. users
7. devices
8. publish tasks

## Important Notes

- finance and commission pages should present state transitions clearly
- money-moving flows should always have room for evidence, review notes, and audit trails
- use list + detail drawer patterns aggressively
- assume backend enums will be stable and should be rendered as badges, tabs, and filters

