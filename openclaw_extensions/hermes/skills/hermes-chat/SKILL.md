---
name: hermes-chat
description: |
  通过本地 Hermes bridge 调用 Hermes 聊天运行时。需要长链路规划、跨会话记忆或让 OpenClaw 走 Hermes 时激活。
---

# Hermes Chat

优先使用 `hermes_chat` 或 gateway method `hermes.chat`。

- 默认来源标记会是 `openclaw_via_hermes`
- 如果 Hermes API 当前不可用，本地 bridge 会自动回退到 OmniDrive 直连聊天
- 需要确认当前共享模型源时，先调用 `hermes_status`

## 最小调用

```json
{
  "prompt": "帮我梳理今天的发布执行计划"
}
```
