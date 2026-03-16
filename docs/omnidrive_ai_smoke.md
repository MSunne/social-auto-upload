# OmniDrive AI Smoke

用这个命令可以在不依赖前端的情况下，直接验证 `omnidrive_cloud` 的真实聊天、作图、做视频链路。

## 前置条件

- `omnidrive_cloud` 已启动
- `OMNIDRIVE_APIYI_API_KEY` 已配置
- 数据库和对象存储配置可用
- 已有一个可登录的 OmniDrive 用户，或者你手里已经有 Bearer token

## 运行方式

```bash
cd /Volumes/mud/project/github/social-auto-upload/omnidrive_cloud

go run ./cmd/omnidrive-ai-smoke \
  -base-url http://127.0.0.1:8410 \
  -email your-user@example.com \
  -password 'your-password'
```

如果你已经有 access token，也可以直接传 token：

```bash
cd /Volumes/mud/project/github/social-auto-upload/omnidrive_cloud

go run ./cmd/omnidrive-ai-smoke \
  -base-url http://127.0.0.1:8410 \
  -token 'your-access-token'
```

## 常用参数

- `-mode`
  - `all`
  - `chat`
  - `image`
  - `video`
- `-chat-prompt`
- `-image-prompt`
- `-video-prompt`
- `-video-ratio`
- `-video-seconds`
- `-poll-interval`
- `-timeout`

## 参考图

图片和视频 smoke 都支持通过环境变量传参考图 URL：

```bash
export OMNIDRIVE_SMOKE_IMAGE_REFS="https://example.com/ref1.png,https://example.com/ref2.png"
export OMNIDRIVE_SMOKE_VIDEO_REFS="https://example.com/ref1.png"
```

## 输出内容

命令会打印：

- 创建出来的 job ID
- job 轮询状态
- 最终 `costCredits`
- artifact 列表
- `outputPayload` 的摘要

这样可以快速确认：

- 云端 worker 是否真的在消费 AI job
- chat/image/video 是否真正调用了三方 provider
- 产物是否已经存进对象存储
- AI job 状态是否已经回写数据库
