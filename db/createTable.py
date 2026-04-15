import os
import sqlite3

# 数据库文件路径（如果不存在会自动创建）
db_file = os.environ.get('OMNIBULL_DB_PATH', './database.db')

# 如果数据库已存在，则删除旧的表（可选）
# if os.path.exists(db_file):
#     os.remove(db_file)

# 连接到SQLite数据库（如果文件不存在则会自动创建）
conn = sqlite3.connect(db_file)
cursor = conn.cursor()

# 创建账号记录表
cursor.execute('''
CREATE TABLE IF NOT EXISTS user_info (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type INTEGER NOT NULL,
    filePath TEXT NOT NULL,  -- 存储文件路径
    userName TEXT NOT NULL,
    status INTEGER DEFAULT 0,
    storageStateJson TEXT,   -- 数据库主存的 Playwright storage_state JSON
    storageStateUpdatedAt DATETIME
)
''')

# 创建文件记录表
cursor.execute('''CREATE TABLE IF NOT EXISTS file_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT, -- 唯一标识每条记录
    filename TEXT NOT NULL,               -- 文件名
    filesize REAL,                     -- 文件大小（单位：MB）
    upload_time DATETIME DEFAULT CURRENT_TIMESTAMP, -- 上传时间，默认当前时间
    file_path TEXT                        -- 文件路径
)
''')

cursor.execute('''CREATE TABLE IF NOT EXISTS publish_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_uuid TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL DEFAULT 'local_api',
    platform_type INTEGER NOT NULL,
    platform_name TEXT NOT NULL,
    account_name TEXT NOT NULL,
    account_file_path TEXT NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    title TEXT NOT NULL,
    run_at DATETIME,
    platform_publish_at DATETIME,
    status TEXT NOT NULL DEFAULT 'pending',
    message TEXT,
    payload_json TEXT NOT NULL,
    verification_data TEXT,
    artifact_path TEXT,
    worker_name TEXT,
    auto_retry_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    finished_at DATETIME
)
''')

cursor.execute('''CREATE TABLE IF NOT EXISTS platform_capabilities (
    platform_type INTEGER PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    label TEXT NOT NULL,
    display_order INTEGER NOT NULL DEFAULT 0,
    visible INTEGER NOT NULL DEFAULT 1,
    login_enabled INTEGER NOT NULL DEFAULT 1,
    publish_enabled INTEGER NOT NULL DEFAULT 1,
    disabled_reason TEXT,
    source_revision TEXT,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)
''')

cursor.executemany(
    '''
    INSERT OR IGNORE INTO platform_capabilities (
        platform_type, slug, label, display_order, visible, login_enabled,
        publish_enabled, disabled_reason, source_revision
    )
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    ''',
    [
        (3, 'douyin', '抖音', 10, 1, 1, 1, None, None),
        (4, 'kuaishou', '快手', 20, 1, 1, 1, None, None),
        (2, 'wechat_channel', '视频号', 30, 1, 1, 1, None, None),
        (1, 'xiaohongshu', '小红书', 40, 1, 0, 0, '本期未开放', None),
    ]
)

cursor.execute('''CREATE TABLE IF NOT EXISTS omnidrive_ai_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_uuid TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL DEFAULT 'local_ui',
    job_type TEXT NOT NULL,
    model_name TEXT NOT NULL,
    skill_id TEXT,
    prompt TEXT,
    status TEXT NOT NULL DEFAULT 'queued_cloud',
    message TEXT,
    payload_json TEXT NOT NULL,
    cloud_job_id TEXT,
    cloud_status TEXT,
    cloud_sync_dirty INTEGER NOT NULL DEFAULT 0,
    last_cloud_sync_hash TEXT,
    last_cloud_sync_at DATETIME,
    linked_publish_task_uuid TEXT,
    artifact_refs_json TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME
)
''')

cursor.execute('''CREATE TABLE IF NOT EXISTS omnidrive_material_sync_state (
    root_name TEXT NOT NULL,
    relative_path TEXT NOT NULL,
    kind TEXT NOT NULL,
    size_bytes INTEGER,
    modified_at TEXT,
    sync_hash TEXT,
    is_deleted INTEGER NOT NULL DEFAULT 0,
    last_synced_at DATETIME,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (root_name, relative_path)
)
''')


# 提交更改
conn.commit()
print("✅ 表创建成功")
# 关闭连接
conn.close()
