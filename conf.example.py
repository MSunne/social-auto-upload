from pathlib import Path

BASE_DIR = Path(__file__).parent.resolve()
XHS_SERVER = "http://127.0.0.1:11901"
LOCAL_CHROME_PATH = ""   # change me necessary！ for example C:/Program Files/Google/Chrome/Application/chrome.exe
LOCAL_CHROME_HEADLESS = False

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
OMNIDRIVE_AGENT_KEY = "change-me"
OMNIDRIVE_AGENT_POLL_INTERVAL = 5
OMNIDRIVE_AGENT_HEARTBEAT_INTERVAL = 30
OMNIDRIVE_ACCOUNT_SYNC_INTERVAL = 60
OMNIDRIVE_MATERIAL_SYNC_INTERVAL = 300
OMNIDRIVE_SKILL_SYNC_INTERVAL = 120
OMNIDRIVE_PUBLISH_SYNC_INTERVAL = 5
OMNIDRIVE_MATERIAL_SYNC_MAX_FILES = 1000

OMNIBULL_PUBLISH_WORKERS = 3
OMNIBULL_TASK_RETENTION_DAYS = 7
OMNIBULL_API_KEY = ""
OMNIBULL_MATERIAL_ROOTS = {
    # "openclawWorkspace": "/Users/yourname/.openclaw/workspace",
}
