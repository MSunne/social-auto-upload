# OmniDrive OpenClaw Plugin

给 OpenClaw 提供 OmniDrive 云端账号登录和 AI 能力：

- `omnidrive_auth`
- `omnidrive_models`
- `omnidrive_chat`
- `omnidrive_image`
- `omnidrive_video`
- `omnidrive_jobs`
- `omnidrive_job_detail`

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

## 说明

- 插件会优先使用 `accessToken`
- 如果没有 `accessToken`，会自动用 `email/password` 登录并缓存 token
- 遇到 `401` 时，如果配置了账号密码，会自动重登一次
- 插件会优先从本机 OmniBull `/api/skill/status` 读取 `deviceCode`，再在云端定位当前账号已绑定的同一台设备
- `chat/image/video` 默认只允许使用“当前本机已绑定且启用”的 OmniBull；设备解绑后，云端 AI 会直接失效，但本地 `omnibull_*` 查询不受影响
