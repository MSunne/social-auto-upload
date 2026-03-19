# OmniDrive OpenClaw Plugin

给 OpenClaw 提供 OmniDrive 云端账号登录和 AI 能力：

- `omnidrive_auth`
- `omnidrive_models`
- `omnidrive_chat`
- `omnidrive_image`
- `omnidrive_video`
- `omnidrive_jobs`
- `omnidrive_job_detail`

同时暴露两个可供 OpenClaw 主聊天链路复用的 gateway method：

- `omnidrive.status`
- `omnidrive.chat`

## 安装

开发期直接本地安装：

```bash
cd /Volumes/mud/project/github/social-auto-upload
openclaw plugins install -l ./openclaw_extensions/omnidrive
openclaw plugins enable omnidrive
```

## 打包分发

这个插件不依赖 skills 商店，推荐直接打包复制到其他设备：

```bash
cd /Volumes/mud/project/github/social-auto-upload/openclaw_extensions/omnidrive
npm pack
```

生成的 `omnidrive-0.1.0.tgz` 可直接复制到其他 OpenClaw 设备安装：

```bash
openclaw plugins install ./omnidrive-0.1.0.tgz
openclaw plugins enable omnidrive
```

## 配置

推荐在 OpenClaw 配置里增加：

```json
{
  "plugins": {
    "entries": {
      "omnidrive": {
        "enabled": true,
        "config": {
          "baseUrl": "http://127.0.0.1:8410",
          "localOmniBullBaseUrl": "http://127.0.0.1:5409",
          "localOmniBullApiKey": "replace-with-omnibull-api-key",
          "email": "user@example.com",
          "password": "replace-with-strong-password",
          "timeoutMs": 45000,
          "defaultChatModel": "gemini-3.1-pro-preview",
          "defaultImageModel": "gemini-3-pro-image-preview",
          "defaultVideoModel": "veo-3.1-fast-fl",
          "defaultVideoDurationSeconds": 8
        }
      }
    }
  }
}
```

也可以只配置 `accessToken`，不配 `email/password`。

## 环境变量

若不想写进插件配置，也支持：

- `OMNIDRIVE_BASE_URL`
- `OMNIDRIVE_ACCESS_TOKEN`
- `OMNIDRIVE_EMAIL`
- `OMNIDRIVE_PASSWORD`
- `OMNIDRIVE_TIMEOUT_MS`
- `OMNIBULL_BASE_URL`
- `OMNIBULL_API_KEY`
- `OMNIBULL_TIMEOUT_MS`
- `OMNIBULL_DEVICE_CODE`
- `OMNIDRIVE_DEFAULT_CHAT_MODEL`
- `OMNIDRIVE_DEFAULT_IMAGE_MODEL`
- `OMNIDRIVE_DEFAULT_VIDEO_MODEL`
- `OMNIDRIVE_DEFAULT_VIDEO_DURATION_SECONDS`

## 使用建议

- 聊天场景优先调用 `omnidrive_chat`
- 文生图、图生图调用 `omnidrive_image`
- 文生视频、图生视频调用 `omnidrive_video`
- 需要查看异步任务时，调用 `omnidrive_jobs` 或 `omnidrive_job_detail`

## 主聊天接入

如果目标是“OmniBull 绑定并激活后，OpenClaw 主聊天默认就走 OmniDrive 当前绑定设备的聊天模型”，不要把具体 `modelName` 写死到本地主配置里。推荐做法是：

1. OpenClaw 主聊天层改为调用 gateway method `omnidrive.chat`
2. 每次请求都动态读取当前设备绑定关系和 `boundDevice.defaultChatModel`
3. 具体模型仍以 OmniDrive 云端设备配置为准，本地只切换“聊天路由”，不保存死模型

这样做的好处是：

- 设备换绑、停用、解绑后，主聊天会立即感知状态变化
- 设备默认聊天模型变更后，不需要再次改 OpenClaw 本地静态配置
- 继续兼容本地 OmniBull 自动下发的 OmniDrive 会话，不必额外手填 token

可先验证 gateway 是否就绪：

```bash
openclaw gateway call omnidrive.status --json
```

返回结果里的 `recommendedMainChatRoute` 会告诉你：

- 推荐调用的 gateway method
- 当前是否已具备切主聊天的条件
- 阻塞原因是什么

更偏工程接入的细节见 [MAIN_CHAT_INTEGRATION.md](./MAIN_CHAT_INTEGRATION.md)。

如果主程序侧暂时还不能直接把主聊天挂到 gateway method 上，才建议退而求其次做“本地配置写回”。即便如此，也建议只把“主聊天 source”切到 OmniDrive 动态路由，而不是把具体模型名写死。

## 说明

- 插件会优先使用 `accessToken`
- 如果没有 `accessToken`，会自动用 `email/password` 登录并缓存 token
- 遇到 `401` 时，如果配置了账号密码，会自动重登一次
- 插件会优先从本机 OmniBull `/api/skill/status` 读取 `deviceCode`，再在云端定位当前账号已绑定的同一台设备
- `omnidrive.chat` gateway method 与 `omnidrive_chat` 工具共用同一套默认模型解析逻辑，都会优先读取 `boundDevice.defaultChatModel`
- `omnidrive.chat` 的请求来源会标记为 `openclaw_main_chat`，便于和 `omnidrive_chat` 的 `openclaw_skill` 区分
- `chat/image/video` 默认只允许使用“当前本机已绑定且启用”的 OmniBull；设备解绑后，云端 AI 会直接失效，但本地 `omnibull_*` 查询不受影响
