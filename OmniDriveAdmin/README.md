# OmniDriveAdmin

Internal admin console frontend for OmniDrive.

## Local configuration

1. Copy the example file:

```bash
cp .env.example .env.local
```

2. Edit the values in `.env.local`:

```bash
PORT=3001
NEXT_PUBLIC_OMNIDRIVE_ADMIN_API_BASE_URL=http://127.0.0.1:8410
```

- `PORT`: admin frontend startup port for `npm run dev` and `npm run start`
- `NEXT_PUBLIC_OMNIDRIVE_ADMIN_API_BASE_URL`: admin backend host address

The admin frontend will request admin APIs under:

- `/api/admin/v1/*`

## Run

```bash
npm install
npm run dev
```

After startup, open `http://localhost:${PORT}`.
