# OmniDrive Backend Prototype

This directory contains an early Python domain and API prototype for `OmniDrive`.

## Status

- useful as a domain model and contract drafting reference
- not the final production cloud implementation
- the production-oriented cloud service is being built separately in `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud`

## Prototype Run

1. Create a virtual environment and install dependencies.
2. Copy `.env.example` to `.env` and fill in the required values.
3. Start the API:

```bash
cd /Volumes/mud/project/github/social-auto-upload/omnidrive_backend
uvicorn app.main:app --reload --port 8410
```

## Prototype Modules

- `auth`: cloud users
- `devices`: OmniBull registration, claim, and status
- `accounts`: mirrored social accounts and remote login sessions
- `skills`: product knowledge and generation skill definitions
- `tasks`: publish task mirror
- `agent`: local agent heartbeat and login relay

## Notes

- `cloud_demo` remains the fast login and agent relay sandbox.
- this Python prototype is kept to preserve domain exploration work
- the production-ready cloud backend should target the Go service

