# Current Engineering Graphs

这份文档只描述“当前代码现状”，不描述理想架构。

目标：

- 用图确认仓库里哪些模块是真正活跃的
- 说明本地 `SAU / OmniBull` 主链怎么跑
- 说明 `OmniDrive`、`OpenClaw`、`cloud_demo` 分别怎么接进来
- 标出当前已经确认的边界和漂移点

## 1. 仓库级模块关系图

```mermaid
flowchart LR
  subgraph LOCAL["本地执行侧: SAU / OmniBull"]
    FE["sau_frontend<br/>Vue 本地控制台"]
    BE["sau_backend.py<br/>Flask 总控入口"]
    CLI["cli_main.py<br/>命令行直连入口"]
    TASK["utils/publish_task_manager.py<br/>发布任务状态机"]
    AI["utils/omnidrive_ai_task_manager.py<br/>本地 AI 任务表"]
    LOGIN["myUtils/login.py<br/>扫码登录"]
    AUTH["myUtils/auth.py<br/>Cookie 校验"]
    ACC["utils/account_storage.py<br/>账号登录态主存"]
    MAT["utils/materials.py<br/>素材根目录访问"]
    UP["uploader/*<br/>平台执行器"]
    DB["db/database.db<br/>SQLite"]
    VF["videoFile/"]
    CK["cookiesFile/"]
    SYNC["omnidriveSync/"]
  end

  subgraph CLOUD["云端侧: OmniDrive"]
    ODC["omnidrive_cloud<br/>Go API"]
    ODF["omnidrive_frontend<br/>用户前端"]
    ODA["OmniDriveAdmin<br/>管理前端"]
  end

  subgraph CLAW["代理侧: OpenClaw"]
    OCB["openclaw_extensions/omnibull"]
    OCD["openclaw_extensions/omnidrive"]
  end

  subgraph DEMO["历史原型"]
    CD["cloud_demo<br/>远端扫码 demo"]
    CAG["utils/cloud_agent.py"]
  end

  FE -->|"HTTP"| BE
  CLI -->|"直接调用"| UP

  BE --> TASK
  BE --> AI
  BE --> LOGIN
  BE --> AUTH
  BE --> ACC
  BE --> MAT
  BE --> DB
  BE --> VF
  BE --> CK
  BE --> SYNC

  TASK --> DB
  TASK --> ACC
  TASK --> MAT
  TASK --> UP

  LOGIN --> ACC
  LOGIN --> CK
  AUTH --> ACC

  ODF -->|"/api/v1/*"| ODC
  ODA -->|"/api/admin/v1/*"| ODC

  BE -->|"OmniDriveBridge"| ODC
  OCB -->|"/api/skill/*"| BE
  OCD -->|"/api/v1/*"| ODC
  OCD -->|"/api/skill/omnidrive/session"| BE

  CAG -->|"/api/agents/*"| CD
  BE --> CAG
```

## 2. 本地 SAU 主链

```mermaid
flowchart TD
  USER["用户"] --> WEB["sau_frontend"]
  USER --> SHELL["cli_main.py"]

  WEB -->|"账号/素材/发布/任务"| API["sau_backend.py"]

  API -->|"账号查询/校验"| ACC["account_storage + user_info"]
  API -->|"素材上传/查询"| FILES["file_records + videoFile"]
  API -->|"任务入队"| PTM["PublishTaskManager"]
  API -->|"AI 任务入队"| AIM["OmniDriveAITaskManager"]
  API -->|"登录线程"| LG["myUtils/login.py"]

  PTM -->|"claim -> run"| EXEC["uploader 执行层"]
  EXEC -->|"Playwright"| PLATFORM["平台创作者后台"]

  SHELL -->|"直接实例化 uploader"| EXEC
```

## 3. 本地发布数据流

```mermaid
sequenceDiagram
  participant U as 用户
  participant FE as sau_frontend
  participant BE as sau_backend.py
  participant TM as PublishTaskManager
  participant DB as publish_tasks
  participant W as worker
  participant UP as uploader/*
  participant P as 平台后台

  U->>FE: 选择平台 / 视频 / 账号 / 标题 / 定时参数
  FE->>BE: POST /postVideo 或 /postVideoBatch
  BE->>TM: enqueue_from_request(data)
  TM->>DB: INSERT publish_tasks
  BE-->>FE: taskUuid / taskCount

  loop 后台调度
    W->>TM: claim next ready task
    TM->>DB: pending/scheduled -> running
    TM->>UP: _execute_payload(payload)
    UP->>P: Playwright 自动上传
  end

  alt 成功
    UP-->>TM: success
    TM->>DB: running -> success
  else 需要人工验证
    UP-->>TM: PublishManualVerificationRequired
    TM->>DB: running -> needs_verify
  else 失败
    UP-->>TM: exception
    TM->>DB: running -> failed
  end
```

## 4. 本地登录数据流

```mermaid
sequenceDiagram
  participant U as 用户
  participant FE as sau_frontend
  participant BE as sau_backend.py
  participant T as run_async_function
  participant LG as myUtils/login.py
  participant PW as Playwright
  participant ACC as account_storage
  participant DB as user_info

  U->>FE: 发起扫码登录
  FE->>BE: GET /login (SSE)
  BE->>BE: 创建 status_queue
  BE->>T: 新线程 + 新事件循环
  T->>T: 先做本地 cookie 快速校验

  alt 旧登录态有效
    T-->>BE: 直接返回完成状态
  else 旧登录态无效
    T->>LG: 按平台分发登录函数
    LG->>PW: 打开平台登录页
    LG-->>BE: 推送二维码 / 验证状态 / 完成状态
    LG->>ACC: 写 storage_state
    ACC->>DB: 更新 user_info.storageStateJson
  end

  BE-->>FE: SSE 持续输出状态
```

## 5. OmniDrive 与本地执行器关系图

```mermaid
flowchart LR
  subgraph CLOUD["omnidrive_cloud"]
    UAPI["/api/v1/*"]
    AAPI["/api/v1/agent/*"]
    ADMIN["/api/admin/v1/*"]
  end

  subgraph LOCAL["sau_backend.py"]
    OAG["utils/omnidrive_agent.py"]
    AIT["utils/omnidrive_ai_task_manager.py"]
    PUB["utils/publish_task_manager.py"]
    ROOTS["omnidriveSync/generated<br/>videoFile<br/>materials roots"]
  end

  subgraph UI["前端与插件"]
    ODUI["omnidrive_frontend"]
    ODA["OmniDriveAdmin"]
    OMBP["OpenClaw omnibull"]
    ODP["OpenClaw omnidrive"]
  end

  ODUI --> UAPI
  ODA --> ADMIN
  OMBP -->|本地 skill API| LOCAL
  ODP -->|云端 API| UAPI
  ODP -->|取本机云端会话| LOCAL

  OAG -->|heartbeat / account sync / materials sync| AAPI
  OAG -->|publish task sync / AI sync / skill sync| AAPI
  AAPI -->|远端登录任务 / 发布任务包 / AI 任务| OAG

  OAG --> AIT
  OAG --> PUB
  OAG --> ROOTS
```

## 6. OpenClaw 调用面

```mermaid
flowchart LR
  CLAW["OpenClaw"] --> P1["openclaw_extensions/omnibull"]
  CLAW --> P2["openclaw_extensions/omnidrive"]

  P1 -->|"omnibull_status / accounts / materials / publish"| SBE["sau_backend.py /api/skill/*"]
  P2 -->|"云端 AI / 账号 / 任务"| ODC["omnidrive_cloud /api/v1/*"]
  P2 -->|"本机 OmniDrive session"| SBE
```

## 7. 当前已确认边界

### 7.1 当前真正接入统一发布队列的平台

```mermaid
flowchart LR
  PTM["PublishTaskManager"]
  PTM --> T2["2 视频号<br/>TencentVideo"]
  PTM --> T3["3 抖音<br/>DouYinVideo"]
  PTM --> T4["4 快手<br/>KSVideo"]
  PTM -. UI可选但执行器未接入 .-> T1["1 小红书"]
```

### 7.2 Web 与 CLI 不是同一条链

```mermaid
flowchart TD
  WEB["Web: sau_frontend"] --> API["sau_backend.py"]
  API --> DBQ["publish_tasks + 状态追踪"]
  DBQ --> WORKER["worker"]
  WORKER --> UP1["uploader"]

  CLI["CLI: cli_main.py"] --> UP2["uploader"]

  WEB -. 主账号存储 .-> DBACC["user_info.storageStateJson + cookiesFile"]
  CLI -. 默认账号路径 .-> CK["cookies/"]
```

### 7.3 已确认的前后端契约漂移

```mermaid
flowchart TD
  A1["前端登录 SSE 参数<br/>platform + account"] --> X1["后端 /login 实际读取<br/>type + id"]
  A2["前端任务详情参数<br/>uuid"] --> X2["后端 /publishTaskDetail 实际读取<br/>id 或 taskUuid"]
  A3["前端 addAccount()<br/>POST /account"] --> X3["后端当前不存在 /account 路由"]
```

## 8. 结论

当前最值得作为后续开发基线的理解是：

> 仓库的真实核心不是某个前端页面，而是 `sau_backend.py` 这个本地总控，加上 `PublishTaskManager`、`account_storage`、`uploader/*` 这条执行链。

同时，`OmniDrive` 已经不是旁路原型，而是通过 `utils/omnidrive_agent.py` 深度接入了本地账号、素材、AI 和发布任务。
