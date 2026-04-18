# OpenClaw 与 Hermes 深度融合接入说明

这套接法在当前仓库里采用“双引擎共存”：

- `OpenClaw` 保留现有本地业务入口和稳定插件能力
- `Hermes` 作为增强型运行时，承接复杂对话、任务编排、记忆和自动化
- 两边使用同一份共享模型运行时配置，不再各自维护默认模型

## 当前落地内容

### 1. 共享模型配置

OmniBull 会在本地生成一份共享运行时配置：

- 默认路径：`runtime/agent-runtime/hermes-openclaw-runtime.json`
- 配置来源：当前 OmniDrive 设备会话与模型同步结果
- 同步入口：
  - `/api/skill/omnidrive/session`
  - `/openai/v1/models`
  - `refresh_openclaw_omnidrive_runtime_config()`

配置契约包含：

- `provider.baseUrl`
- `provider.apiKey`
- `defaults.chatModel`
- `defaults.imageModel`
- `defaults.videoModel`
- `routing.mode`
- `updatedAt`
- `version`

约束很明确：

- `Hermes` 不再独立维护默认模型
- `OpenClaw` 和 `Hermes` 只认这一个共享配置源
- 云端设备默认模型切换后，两边跟着同一刷新链路一起更新

### 2. 本地 Hermes bridge

OmniBull 新增了本地桥接接口：

- `GET /api/hermes/status`
- `POST /api/hermes/chat`
- `GET /api/hermes/jobs/:id`
- `POST /api/hermes/task/run`
- `POST /api/hermes/task/schedule`

桥接规则：

- 仅面向本机使用，目标 Hermes API Server 默认应监听 `127.0.0.1`
- 走现有 `OmniBull` skill API 鉴权
- Hermes 聊天失败时，自动回退到现有 `OmniDrive OpenAI proxy`
- 不直接碰 `SAU` 的浏览器执行内核，也不直接写数据库

### 3. OpenClaw 协作插件

仓库新增 `openclaw_extensions/hermes`，提供：

- gateway methods
  - `hermes.status`
  - `hermes.chat`
  - `hermes.run_skill`
- tools
  - `hermes_status`
  - `hermes_chat`
  - `hermes_run_skill`

默认来源会被标记成 `openclaw_via_hermes`。

### 4. Hermes 薄技能

bridge 当前支持这些技能名：

- `omnibull-accounts`
- `omnibull-materials`
- `omnibull-publish`
- `omnidrive-chat`
- `omnidrive-image`
- `omnidrive-video`
- `omnidrive-jobs`

设计原则只有一条：继续复用本仓库现有 API 和任务管理器，不复制业务逻辑。

### 5. 来源与执行引擎审计

AI 任务和发布任务都已补齐：

- `sourceCategory`
- `executionEngine`
- `correlationId`

当前约定的来源分类：

- `openclaw_direct`
- `openclaw_via_hermes`
- `hermes_direct`
- `hermes_scheduled`

本地前端和原生本地接口继续归为 `omnibull_local`。

## 部署建议

### OmniBull 配置

在 `conf.py` 或生产配置中补这些字段：

```python
HERMES_PROFILE_NAME = "omnibull"
HERMES_API_SERVER_BASE_URL = "http://127.0.0.1:8642"
HERMES_API_SERVER_KEY = ""
HERMES_API_SERVER_TIMEOUT = 60
HERMES_SHARED_RUNTIME_CONFIG_PATH = BASE_DIR / "runtime" / "agent-runtime" / "hermes-openclaw-runtime.json"
```

### OpenClaw 插件安装

```bash
cd /Volumes/mud/project/github/social-auto-upload
openclaw plugins install -l ./openclaw_extensions/hermes
openclaw plugins enable hermes
openclaw gateway restart
```

### Hermes API Server 接入要求

- 仅绑定 `127.0.0.1`
- 开启 API 鉴权，并把 token 配到 `HERMES_API_SERVER_KEY`
- 不在 Hermes 侧另配一套默认模型
- 让 Hermes 读取 OmniBull 生成的共享运行时 JSON

## 典型调用

### 读取 bridge 状态

```bash
python scripts/hermes_local_bridge.py status
```

### 通过 bridge 调 Hermes 聊天

```bash
python scripts/hermes_local_bridge.py chat --json '{"prompt":"列出今天待处理的发布任务","source":"hermes_direct"}'
```

### 通过 bridge 执行本地薄技能

```bash
python scripts/hermes_local_bridge.py run --json '{"taskType":"run_skill","skillName":"omnibull-accounts","action":"list","source":"hermes_direct"}'
```

### 通过 bridge 创建编排型发布任务

```bash
python scripts/hermes_local_bridge.py schedule --json '{
  "taskType":"publish",
  "platformType":3,
  "title":"Hermes 编排发布",
  "files":[{"root":"openclawWorkspace","path":"videos/demo.mp4"}],
  "accountFilePaths":["douyin/account.json"],
  "runAt":"2026-04-16T21:30:00+08:00",
  "source":"hermes_scheduled"
}'
```

## 回退策略

- `Hermes` 不可用时，`/api/hermes/chat` 自动回退到 `OmniDrive` 直连聊天
- 账号、Cookie、发布队列、设备状态仍以 `OmniBull / SAU` 为准
- `Hermes cron/webhook` 只能调用 bridge 或现有本地 API 入队，不能绕过任务管理器

## 安全边界

- 共享运行时响应会自动脱敏 `apiKey/accessToken`
- `Hermes` 侧只应该保存偏好、工作流总结、非敏感记忆
- 不应该在 Hermes 记忆、日志、总结中写入 Cookie、平台登录态、云 token 原文

## 实施顺序

建议顺序保持为：

1. 共享模型配置
2. Hermes 本地部署与 API Server
3. 本地 bridge
4. OpenClaw 协作插件与 Hermes 薄技能
5. 路由与回退
6. cron/webhook 编排
