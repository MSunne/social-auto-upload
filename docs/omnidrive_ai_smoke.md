# OmniDrive AI Smoke

用这个命令可以在不依赖前端的情况下，直接验证 `omnidrive_cloud` 的真实聊天、作图、做视频链路。

## 前置条件

- `omnidrive_cloud` 已启动
- `OMNIDRIVE_APIYI_API_KEY` 已配置
- 数据库和对象存储配置可用
- 已有一个可登录的 OmniDrive 用户，或者你手里已经有 Bearer token

## 隔离建议

- 多线程协作时，不要直接拿主开发库做真实 smoke。
- 推荐单独准备一个隔离数据库，例如 `omnidrive_smoke`。
- 推荐单独使用一个隔离端口，例如 `8411`。
- 这样可以避免主库里已有的 queued job、测试用户和运行产物影响本次验证。
- 如果只是验证后端链路，允许继续使用本地存储目录，例如 `omnidrive_cloud/data-smoke/`。

示例：

```bash
cd /Volumes/mud/project/github/social-auto-upload/omnidrive_cloud

env OMNIDRIVE_DATABASE_DSN='postgres://postgres:YOUR_PASSWORD@127.0.0.1:5432/omnidrive_smoke?sslmode=disable' \
  go run ./cmd/omnidrive-bootstrap-db

env OMNIDRIVE_DATABASE_DSN='postgres://postgres:YOUR_PASSWORD@127.0.0.1:5432/omnidrive_smoke?sslmode=disable' \
  OMNIDRIVE_BIND_ADDR=':8411' \
  OMNIDRIVE_PUBLIC_BASE_URL='http://127.0.0.1:8411' \
  OMNIDRIVE_LOCAL_STORAGE_DIR='./data-smoke' \
  OMNIDRIVE_APIYI_API_KEY='YOUR_APIYI_KEY' \
  go run ./cmd/omnidrive-api
```

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
