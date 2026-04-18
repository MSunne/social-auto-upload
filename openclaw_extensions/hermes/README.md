# Hermes OpenClaw Plugin

本地 OpenClaw 插件，通过 OmniBull 暴露 Hermes bridge 的两个协作入口：

- `hermes.chat`
- `hermes.run_skill`

同时提供工具：

- `hermes_status`
- `hermes_chat`
- `hermes_run_skill`

## 安装

```bash
cd /Volumes/mud/project/github/social-auto-upload
openclaw plugins install -l ./openclaw_extensions/hermes
openclaw plugins enable hermes
openclaw gateway restart
```

## 配置

```json
{
  "plugins": {
    "entries": {
      "hermes": {
        "enabled": true,
        "config": {
          "localOmniBullBaseUrl": "http://127.0.0.1:5409",
          "localOmniBullApiKey": "replace-with-omnibull-api-key",
          "localOmniBullTimeoutMs": 15000
        }
      }
    }
  }
}
```

## 说明

- `hermes.chat` 调本地 `/api/hermes/chat`
- `hermes.run_skill` 调本地 `/api/hermes/task/run`
- 默认 `source` 为 `openclaw_via_hermes`
- 当 Hermes API 不可用时，聊天回退由 OmniBull bridge 统一处理
