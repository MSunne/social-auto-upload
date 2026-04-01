# Windows Runtime Paths

Windows test host deployment paths for OmniBull / SAU:

- Code root: `C:\OmniBull\social-auto-upload`
- Python runtime: `C:\OmniBull\social-auto-upload\.venv\Scripts\python.exe`
- Frontend npm: `C:\Program Files\nodejs\npm.cmd`
- Environment file: `C:\OmniBull\social-auto-upload\deploy\env\omnibull.windows.env`
- Runtime directory: `C:\OmniBull\social-auto-upload\runtime`
- Log directory: `C:\OmniBull\social-auto-upload\logs`
- Installer staging: `C:\sau-setup`

Windows startup entries:

- Startup scheduled task: `OmniBull SAU Startup`
- Logon scheduled task: `OmniBull SAU`

Sync guidance:

- Future source syncs should overwrite `C:\OmniBull\social-auto-upload`
- Preserve local runtime state when needed:
  - `conf.py`
  - `deploy\env\omnibull.windows.env`
  - `db\database.db`
  - `cookiesFile\`
  - `videoFile\`
  - `omnidriveSync\`
