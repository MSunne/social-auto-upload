# OmniDrive / OmniBull Deploy Quickstart

This is the shortest stable deployment path for the current repo.

## 1. OmniBull image

Recommended target:

- `Deepin` desktop image
- one dedicated desktop user, for example `omnibull`
- auto-login enabled
- sleep, hibernate, screen lock, and automatic display power-off disabled

Recommended runtime layout:

- code: `/opt/omnibull/social-auto-upload`
- venv: `/opt/omnibull/venv`
- service env: `/etc/omnibull/omnibull.env`
- app config: `/opt/omnibull/social-auto-upload/conf.py`

Suggested install steps inside the golden image:

```bash
apt-get update
apt-get install -y python3 python3-venv python3-pip ffmpeg chromium

cd /opt/omnibull/social-auto-upload
python3 -m venv /opt/omnibull/venv
/opt/omnibull/venv/bin/pip install -r requirements.txt
/opt/omnibull/venv/bin/playwright install chromium
cp deploy/omnibull/conf.production.example.py conf.py
```

Then:

1. Fill `conf.py`
2. Copy [omnibull.service](/Volumes/mud/project/github/social-auto-upload/deploy/systemd/omnibull.service) to `/etc/systemd/system/omnibull.service`
3. Copy [omnibull.env.example](/Volumes/mud/project/github/social-auto-upload/deploy/env/omnibull.env.example) to `/etc/omnibull/omnibull.env`
4. Run `systemctl daemon-reload`
5. Run `systemctl enable --now omnibull`

Important notes:

- Do not bake a pre-generated `/etc/omnibull/device.json` into the image.
- OmniBull now auto-generates device identity on first boot and will report browser and directory health through `/api/skill/status`.
- Browser/runtime checks are implemented in [browser_hook.py](/Volumes/mud/project/github/social-auto-upload/utils/browser_hook.py), [runtime_health.py](/Volumes/mud/project/github/social-auto-upload/utils/runtime_health.py), and [sau_backend.py](/Volumes/mud/project/github/social-auto-upload/sau_backend.py).

## 2. OmniDrive Go API

Cloud prerequisites:

- PostgreSQL
- Redis
- S3-compatible object storage
- `ffmpeg` on the Linux host if AI video standardization is enabled

Build and bootstrap:

```bash
cd /opt/omnidrive/social-auto-upload/omnidrive_cloud
go build -o /opt/omnidrive/bin/omnidrive-bootstrap-db ./cmd/omnidrive-bootstrap-db
go build -o /opt/omnidrive/bin/omnidrive-api ./cmd/omnidrive-api

/opt/omnidrive/bin/omnidrive-bootstrap-db
```

Then:

1. Copy [omnidrive-api.service](/Volumes/mud/project/github/social-auto-upload/deploy/systemd/omnidrive-api.service) to `/etc/systemd/system/omnidrive-api.service`
2. Copy [omnidrive-api.env.example](/Volumes/mud/project/github/social-auto-upload/deploy/env/omnidrive-api.env.example) to `/etc/omnidrive/omnidrive-api.env`
3. Fill the real values
4. Run `systemctl daemon-reload`
5. Run `systemctl enable --now omnidrive-api`

Related repo references:

- cloud stack and startup: [omnidrive_cloud/README.md](/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud/README.md)
- env loading and defaults: [config.go](/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud/internal/config/config.go)

## 3. OmniDrive frontends

Both frontends are Next apps, not plain static bundles.

- customer frontend scripts: [omnidrive_frontend/package.json](/Volumes/mud/project/github/social-auto-upload/omnidrive_frontend/package.json)
- admin frontend scripts: [OmniDriveAdmin/package.json](/Volumes/mud/project/github/social-auto-upload/OmniDriveAdmin/package.json)

Build customer frontend:

```bash
cd /opt/omnidrive/social-auto-upload/omnidrive_frontend
cp /opt/omnidrive/social-auto-upload/deploy/env/omnidrive-frontend.env.example .env.production.local
npm install
npm run build
```

Build admin frontend:

```bash
cd /opt/omnidrive/social-auto-upload/OmniDriveAdmin
cp /opt/omnidrive/social-auto-upload/deploy/env/omnidrive-admin.env.example .env.production.local
npm install
npm run build
```

Then:

1. Copy [omnidrive-frontend.service](/Volumes/mud/project/github/social-auto-upload/deploy/systemd/omnidrive-frontend.service) to `/etc/systemd/system/omnidrive-frontend.service`
2. Copy [omnidrive-admin.service](/Volumes/mud/project/github/social-auto-upload/deploy/systemd/omnidrive-admin.service) to `/etc/systemd/system/omnidrive-admin.service`
3. Copy the matching env examples from `deploy/env/` to `/etc/omnidrive/`
4. Run `systemctl daemon-reload`
5. Run `systemctl enable --now omnidrive-frontend omnidrive-admin`

Important note:

- `NEXT_PUBLIC_*` values must be present before `npm run build`, otherwise the wrong API host will be bundled into the frontend.

## 4. Reverse proxy

Suggested split:

- `https://omnidrive.example.com/` -> customer frontend
- `https://omnidrive.example.com/api/` -> Go API
- `https://admin.example.com/` -> admin frontend

If you want to keep one domain for everything, make sure the reverse proxy forwards:

- `/api/` and `/api/admin/` to `omnidrive-api`
- `/` to the correct Next frontend
