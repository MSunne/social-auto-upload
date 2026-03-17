---
name: omnidrive-chat
description: |
  使用 OmniDrive 云端聊天模型处理文案、问答、运营建议、任务解释等场景。用户明确要求“聊天”“写文案”“优化标题”“解释 AI 输出”，或者询问/切换“OmniDrive chat skill 当前用什么模型”时激活。
---

# OmniDrive Chat

优先使用 `omnidrive_chat`。

默认聊天模型来自当前已绑定 OmniBull 设备的 `defaultChatModel`。用户没有指定模型时，直接使用默认聊天模型即可；只有当用户明确追问当前模型或要求切换模型时，才先查模型信息。

## 最小调用

```json
{
  "prompt": "帮我写 3 个短视频标题",
  "wait": true
}
```

## 带系统提示

```json
{
  "prompt": "帮我总结这个视频脚本的卖点",
  "systemPrompt": "你是内容运营专家，输出简洁中文。",
  "wait": true
}
```

## 多轮消息

```json
{
  "messages": [
    { "role": "system", "content": "你是 OmniDrive 助手" },
    { "role": "user", "content": "帮我写 3 个探店标题" }
  ],
  "wait": true
}
```

## 规则

- 需要直接答案时，默认 `wait=true`
- 如果用户只是问模型有哪些可用，先用 `omnidrive_models`
- 如果用户追问“当前默认用什么聊天模型”，先用 `omnidrive_auth` 的 `action=status` 读取 `boundDevice.defaultChatModel`
- 如果 `omnidrive_auth` 返回 `authSource=local_agent_session` 或 `headlessAgentSessionActive=true`，说明当前机器已经通过 OmniBull agent 自动获得 OmniDrive 会话；这时不要再要求用户配置 `accessToken`、`email` 或 `password`
- 如果用户问的是 `OmniDrive chat skill` 当前使用什么模型，不要回答 OpenClaw 自己的主对话模型；应以 `boundDevice.defaultChatModel` 或本次 `modelName` 为准
- 如果用户要求“把 OmniDrive 聊天模型切到某个模型”，先用 `omnidrive_models` 确认模型可用，再用 `omnidrive_device_config` 的 `action=set_defaults` 更新 `defaultChatModel`
- 如果当前话题已经在 OmniDrive / OmniBull 云端 AI 上下文里，用户只说“把模型切到 X”或“现在用哪个模型”，默认理解为在问 OmniDrive chat skill，不要跳去回答 OpenClaw 主对话模型
- 只有当用户明确说“切换龙虾当前对话模型”“切换 OpenClaw 主模型”时，才把它理解为 OpenClaw 自己的模型管理
- 用户没有指定模型时，可以不传 `modelName`，工具会自动回落到绑定设备默认聊天模型
- 返回结果时优先引用最终文本，不要把整个 workspace 原样倾倒给用户
