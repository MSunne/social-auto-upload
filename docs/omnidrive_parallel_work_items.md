# OmniDrive Parallel Work Items

当前主线程由 Codex 负责：

- `openclaw_extensions/omnidrive`
- OpenClaw 使用 OmniDrive 账号直接调用聊天、作图、做视频

下面这些任务适合并行开新线程。

## Thread A: 生产阻断项

目标：

- 解决上线前必须收口的基础设施问题

任务：

- 迁移 `AlTask.md` 里的敏感配置到环境变量或 secret manager
- 轮换现有密钥
- 建立正式 migration 体系，替换 `AutoCreateSchema`
- 补全 account login、skill change、manual verification 审计
- 制定 AI 图片、视频、截图生命周期规则

关键文件：

- `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud/internal/config/config.go`
- `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud/internal/database/database.go`
- `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud/internal/database/schema.go`
- `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud/internal/http/handlers/audit.go`

## Thread B: 后台系统配置扩展

目标：

- 把后台系统配置从“当前基础开关”扩成“AI/provider/model 可运营配置”

任务：

- 在 `/api/admin/v1/system-config` 上增加 AI provider 配置
- 增加默认 chat/image/video 模型配置
- 增加模型启停、功能开关、视频时长上限、参考图上限等配置
- 增加 OpenClaw 插件分发配置：当前稳定版本、下载地址、sha256、最低版本

关键文件：

- `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud/internal/http/handlers/admin_system.go`
- `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud/internal/store/admin_system.go`
- `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud/internal/domain/admin_models.go`

## Thread C: OmniBull 本地 AI 补全

目标：

- 让本地 OmniBull AI 任务链路也支持 `chat`

任务：

- 扩展本地 AI task manager 支持 `chat`
- 补 `/api/skill/ai/tasks` 的 chat 输入校验
- 明确 chat 结果在本地如何存储、如何展示
- 评估是否需要本地 chat 历史页

关键文件：

- `/Volumes/mud/project/github/social-auto-upload/utils/omnidrive_ai_task_manager.py`
- `/Volumes/mud/project/github/social-auto-upload/sau_backend.py`

## Thread D: 客服充值增强

目标：

- 继续完善人工充值链，但不做在线支付接入

任务：

- 附件表或凭证资产规范化
- bonus credits 表达与入账
- reviewer notes history
- resubmit 可见性
- 管理后台 support recharge 小组件

关键文件：

- `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud/internal/http/handlers/admin_support_recharge.go`
- `/Volumes/mud/project/github/social-auto-upload/omnidrive_cloud/internal/store/support_recharge_review.go`
- `/Volumes/mud/project/github/social-auto-upload/docs/omnidrive_admin_backend_workstreams.md`

## Thread E: OpenClaw 插件打包与分发自动化

目标：

- 让多个 OpenClaw 设备能更方便复制安装 OmniDrive 插件

任务：

- 增加 `npm pack` 或统一打包脚本
- 生成安装说明和版本说明
- 可选：生成内部下载包目录和 checksum

关键目录：

- `/Volumes/mud/project/github/social-auto-upload/openclaw_extensions/omnidrive`
