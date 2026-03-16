# Project Structure

这份文档用于明确当前仓库中几个“工程”的边界，避免把 `SAU`、`OmniBull`、`OmniDrive`、`OpenClaw` 混在一起理解。

## 总体对应关系

- `SAU`
  - 当前这个 `social-auto-upload` 的本地执行器主体。
- `OmniBull`
  - 安装了 `SAU + OpenClaw` 的本地 Linux / macOS 机器，产品视角下的本地执行节点。
- `OmniDrive`
  - 云端控制台和云端后端。
- `OpenClaw`
  - 本地智能代理，通过技能调用 `OmniDrive` 和 `SAU`。

## 目录职责

### SAU / OmniBull 本地执行器

- `sau_backend.py`
  - Flask 本地后端入口。
- `sau_frontend`
  - 当前 `OmniBull / SAU / LocaWeb` 本地前端工程。
- `omnibull_frontend`
  - `sau_frontend` 的直达别名，方便按产品名快速定位。
- `uploader`
  - 各平台真实发布实现。
- `myUtils`
  - 登录、鉴权、平台辅助逻辑。
- `utils`
  - 本地任务、云端桥接、素材镜像、设备元数据等通用能力。
- `db`
  - 本地 SQLite 数据和建表脚本。

### OmniDrive 云端

- `omnidrive_cloud`
  - Go 云端后端工程。
- `omnidrive_frontend`
  - 云端前端工程。

### OpenClaw 集成

- `openclaw_extensions/omnibull`
  - `OpenClaw -> OmniBull / SAU` 本地插件。

### 工程入口

- `projects`
  - 统一工程入口目录，里面是几个活跃工程的软链接别名，便于快速进入。

### 设计与文档

- `docs`
  - 接口契约、后端规划、工程结构说明。
- `stitch_exports`
  - Stitch 导出的设计稿和 HTML。

### 过渡 / 历史目录

- `cloud_demo`
  - 早期远程扫码登录原型。

## 当前结论

- 如果你在找 `OmniBull` 前端工程目录：
  - 就是 `sau_frontend`
  - 也可以直接进入 `omnibull_frontend`
- 如果你在找 `OmniDrive` 前端工程目录：
  - 就是 `omnidrive_frontend`
- 如果你在找 `OmniDrive` 后端工程目录：
  - 就是 `omnidrive_cloud`

## 当前整理策略

当前采用的是“非破坏式整理”：

- 删除旧的 `omnidrive_backend` Python 原型
- 新增 `omnibull_frontend -> sau_frontend` 别名
- 新增 `projects/` 作为统一工程入口

当前不建议直接移动 `sau_frontend`、`omnidrive_frontend`、`omnidrive_cloud` 这些目录，
因为它们已经被前端开发、运行脚本、以及已有文档引用。
