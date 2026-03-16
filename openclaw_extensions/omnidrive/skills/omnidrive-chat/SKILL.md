---
name: omnidrive-chat
description: |
  使用 OmniDrive 云端聊天模型处理文案、问答、运营建议、任务解释等场景。用户明确要求“聊天”“写文案”“优化标题”“解释 AI 输出”时激活。
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
- 用户没有指定模型时，可以不传 `modelName`，工具会自动回落到绑定设备默认聊天模型
- 返回结果时优先引用最终文本，不要把整个 workspace 原样倾倒给用户
