# OmniBull OpenClaw Plugin

本地 OpenClaw 插件，给同机房的 OmniBull 暴露 4 个能力：

- `omnibull_status`
- `omnibull_accounts`
- `omnibull_materials`
- `omnibull_publish`

## 安装

```bash
cd /Volumes/mud/project/github/social-auto-upload
openclaw plugins install -l ./openclaw_extensions/omnibull
openclaw plugins enable omnibull
```

重启 Gateway：

```bash
openclaw gateway restart
```

## 配置

推荐在 OpenClaw 配置里增加：

```json
{
  "plugins": {
    "entries": {
      "omnibull": {
        "enabled": true,
        "config": {
          "baseUrl": "http://127.0.0.1:5409",
          "apiKey": "replace-with-omnibull-api-key",
          "timeoutMs": 15000
        }
      }
    }
  }
}
```

## 调试

先确认 OmniBull 后端运行，再执行：

```bash
openclaw gateway call omnibull.status --json
```

如果插件已加载、后端也可用，会返回本地设备状态、账号数量和发布任务统计。
