# omnidrive_frontend

Customer-facing OmniDrive cloud console.

## Local configuration

1. Copy the example file:

```bash
cp .env.example .env.local
```

2. Edit the values in `.env.local`:

```bash
PORT=3000
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8410/api/v1
NEXT_PUBLIC_USE_MOCK=false
```

Production deployments must set `NEXT_PUBLIC_API_BASE_URL` to the public OmniDrive API origin, for example `https://aitoplus.com/api/v1`.
If it is omitted in a browser session on a non-localhost origin, the frontend now falls back to `${window.location.origin}/api/v1` instead of `127.0.0.1`.

- `PORT`: frontend startup port for `npm run dev` and `npm run start`
- `NEXT_PUBLIC_API_BASE_URL`: customer frontend backend address
- `NEXT_PUBLIC_USE_MOCK`: whether to enable non-auth mock fallback

## Run

```bash
npm install
npm run dev
```

After startup, open `http://localhost:${PORT}`.
