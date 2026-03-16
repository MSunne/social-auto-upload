# OmniDriveAdmin

Internal admin console frontend for `OmniDriveAdmin`.

This project is intentionally separate from:

- `omnidrive_frontend`: customer-facing cloud console
- `sau_frontend`: local OmniBull / SAU console

## Stack

- Next.js App Router
- React 19
- TypeScript
- Tailwind CSS 4
- React Query
- Axios

## Local Run

```bash
cd /Volumes/mud/project/github/social-auto-upload/OmniDriveAdmin
npm install
npm run dev
```

Recommended environment variables:

```bash
NEXT_PUBLIC_OMNIDRIVE_ADMIN_API_BASE_URL=http://127.0.0.1:8410
```

The admin backend is expected to expose:

- `/api/admin/v1/*`

Current frontend integration contract:

- `/Volumes/mud/project/github/social-auto-upload/docs/omnidrive_admin_api_contract.md`

## Initial Route Map

- `/dashboard`
- `/users`
- `/devices`
- `/media-accounts`
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
- `/withdrawals`
- `/audits`
- `/settings`
- `/admins`
