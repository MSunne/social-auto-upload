from pathlib import Path

BASE_DIR = Path(__file__).parent.resolve()
XHS_SERVER = "http://127.0.0.1:11901"
# Leave empty on Linux/Deepin. OmniBull will prefer a detected system Chrome/Chromium
# and will fall back to auto-installing Playwright Chromium when nothing is available.
LOCAL_CHROME_PATH = ""
LOCAL_CHROME_HEADLESS = False
SAU_LOG_LEVEL = "INFO"

# cloud demo minimal agent config
CLOUD_AGENT_ENABLED = False
CLOUD_DEMO_URL = ""  # for example: https://your-cloud-demo.example.com
CLOUD_DEVICE_NAME = ""
CLOUD_AGENT_KEY = "change-me"
CLOUD_AGENT_POLL_INTERVAL = 5
CLOUD_AGENT_HEARTBEAT_INTERVAL = 30
CLOUD_DEVICE_CODE = ""

# production OmniDrive agent bridge
OMNIDRIVE_AGENT_ENABLED = False
OMNIDRIVE_BASE_URL = ""  # for example: https://omnidrive.example.com
OMNIDRIVE_AGENT_KEY = ""  # keep empty for factory/master images so each device generates its own agentKey
OMNIDRIVE_AGENT_POLL_INTERVAL = 5
OMNIDRIVE_AGENT_AI_POLL_INTERVAL = 15
OMNIDRIVE_AGENT_HEARTBEAT_INTERVAL = 30
OMNIDRIVE_ACCOUNT_SYNC_INTERVAL = 60
OMNIDRIVE_ACCOUNT_VALIDATION_INTERVAL = 21600
OMNIDRIVE_MATERIAL_SYNC_INTERVAL = 1800
OMNIDRIVE_SKILL_SYNC_INTERVAL = 120
OMNIDRIVE_PUBLISH_SYNC_INTERVAL = 5
OMNIDRIVE_MATERIAL_SYNC_MAX_FILES = 1000
OMNIBULL_DEVICE_IDENTITY_FILE = "/etc/omnibull/device.json"  # first boot auto-creates this file when missing

OMNIBULL_PUBLISH_WORKERS = 1
OMNIBULL_PUBLISH_DISPATCH_INTERVAL_SECONDS = 5
OMNIBULL_TASK_RETENTION_DAYS = 7
OMNIBULL_API_KEY = ""
OMNIBULL_CORS_ALLOWED_ORIGINS = "*"  # dev 默认允许所有来源；生产可改成逗号分隔白名单
OMNIBULL_CORS_ALLOWED_METHODS = "GET,POST,PUT,PATCH,DELETE,OPTIONS"
OMNIBULL_CORS_ALLOWED_HEADERS = "Authorization,Content-Type,X-Requested-With,X-Omnibull-Key"
OMNIBULL_CORS_EXPOSE_HEADERS = "Content-Disposition,X-Accel-Buffering"
OMNIBULL_CORS_ALLOW_CREDENTIALS = False
OMNIBULL_CORS_MAX_AGE = 86400

# Hermes / OpenClaw shared runtime
HERMES_PROFILE_NAME = "omnibull"
HERMES_API_SERVER_BASE_URL = "http://127.0.0.1:8642"
HERMES_API_SERVER_KEY = ""
HERMES_API_SERVER_TIMEOUT = 60
HERMES_SHARED_RUNTIME_CONFIG_PATH = BASE_DIR / "runtime" / "agent-runtime" / "hermes-openclaw-runtime.json"
OPENCLAW_GATEWAY_DAILY_RELOAD_ENABLED = True
OPENCLAW_GATEWAY_DAILY_RELOAD_HOUR = 0
OPENCLAW_GATEWAY_DAILY_RELOAD_MINUTE = 0
OPENCLAW_GATEWAY_DAILY_RELOAD_DEFER_SECONDS = 300
OPENCLAW_GATEWAY_DAILY_RELOAD_TIMEOUT_SECONDS = 180
OPENCLAW_GATEWAY_DAILY_RELOAD_COMMAND = "openclaw gateway restart"

OMNIBULL_MATERIAL_ROOTS = {
    # "openclawWorkspace": "/Users/yourname/.openclaw/workspace",
}
