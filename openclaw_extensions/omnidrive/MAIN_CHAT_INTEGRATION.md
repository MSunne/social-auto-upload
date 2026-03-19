# OmniDrive Main Chat Integration

这份文档给 OpenClaw 主程序使用，目标是让“当前主聊天”在 OmniBull 已绑定且启用时自动走 OmniDrive 动态聊天模型，而不是继续停留在本地静态默认模型。

## 设计原则

1. 不把具体 `modelName` 写死进 OpenClaw 本地默认配置
2. OpenClaw 主聊天只切换“路由”，统一调用 gateway method `omnidrive.chat`
3. 真实模型始终以当前绑定设备的 `boundDevice.defaultChatModel` 为准
4. 如果设备解绑、停用、会话失效，主程序应自动回退到本地默认聊天链路

## Gateway Contract

### 1. 读取接入状态

调用：

```bash
openclaw gateway call omnidrive.status --json
```

关键字段：

- `authenticated`
- `headlessAgentSessionActive`
- `boundDevice`
- `recommendedMainChatRoute.ready`
- `recommendedMainChatRoute.gatewayMethod`
- `recommendedMainChatRoute.statusMethod`
- `recommendedMainChatRoute.requestSource`
- `recommendedMainChatRoute.blockedReason`

当 `recommendedMainChatRoute.ready=true` 时，说明主聊天可以直接切到 `omnidrive.chat`。

### 2. 主聊天调用

主程序应把当前聊天输入透传给 gateway method `omnidrive.chat`。

建议请求形态：

```json
{
  "messages": [
    { "role": "system", "content": "你是 OpenClaw 主助手" },
    { "role": "user", "content": "帮我总结今天的发布计划" }
  ],
  "wait": true
}
```

也可以使用：

```json
{
  "prompt": "帮我总结今天的发布计划",
  "wait": true
}
```

返回关键字段：

- `text`
- `effectiveModelName`
- `device`
- `job`
- `workspace`
- `requestSource`

其中 `requestSource` 固定为 `openclaw_main_chat`，用于和 skill 调用区分。

## 建议接入流程

### 启动时

1. 调用 `omnidrive.status`
2. 如果 `recommendedMainChatRoute.ready=true`，将主聊天路由切到 `omnidrive.chat`
3. 如果不满足条件，继续使用 OpenClaw 现有本地默认聊天链路

### 运行时刷新

建议在以下时机重新调用 `omnidrive.status`：

- OpenClaw 启动完成后
- 插件启用/重载后
- OmniBull 设备状态变化后
- 主聊天请求失败并出现鉴权/绑定错误后
- 用户手动点击“刷新 OmniDrive 连接状态”后

### 失败回退

当 `omnidrive.chat` 返回以下类型错误时，主程序应回退到本地聊天链路：

- OmniDrive 会话不可用
- 当前 OpenClaw 所在 OmniBull 未绑定或未启用
- 当前 OmniBull 设备已被停用或解绑

回退后可异步重新探测 `omnidrive.status`，当其再次变为 ready 时再切回。

## 不推荐的做法

- 不推荐把 `boundDevice.defaultChatModel` 同步写死到 OpenClaw 本地默认模型配置
- 不推荐把设备激活事件和模型名强耦合到本地配置文件
- 不推荐继续把主聊天和 `omnidrive_chat` skill 视为同一条调用来源

## 当前仓库已提供的能力

- `omnidrive.status`：返回当前接入状态和主聊天推荐路由
- `omnidrive.chat`：主聊天动态入口，请求来源为 `openclaw_main_chat`
- `omnidrive_chat`：skill 入口，请求来源保持为 `openclaw_skill`

## 后续建议

如果 OpenClaw 主程序支持 provider/router 抽象，建议新增一个 `omnidrive_dynamic` chat route：

- 健康检查调用 `omnidrive.status`
- 执行聊天调用 `omnidrive.chat`
- 失败时回退到本地默认 provider

这样设备激活、解绑、模型切换都不再需要修改本地静态默认配置。
