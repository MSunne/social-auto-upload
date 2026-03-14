# Cloud QR Demo

这是一个最小可用的云端扫码登录 demo。

它做三件事：

1. 接收本地 agent 推送过来的二维码和登录状态
2. 提供一个远端可访问的二维码页面给用户扫码
3. 把账号状态镜像写到云端 SQLite，方便远程查看

现在这版已经支持：

1. 本地后端启动后自动向云端报到
2. 云端首页显示在线设备
3. 云端直接创建“扫码登录任务”
4. 本地 agent 自动领取任务并拉起浏览器
5. 远端扫码页实时显示二维码和状态

## 启动云端 demo

先准备依赖：

```bash
pip install Flask
```

```bash
cd cloud_demo
python3 app.py
```

默认监听 `http://0.0.0.0:5410`。

部署到公网后，记下你的云端地址，例如：

```text
https://your-cloud-demo.example.com
```

## 配置本地 agent

在本地 `conf.py` 增加这些配置：

```python
CLOUD_AGENT_ENABLED = True
CLOUD_DEMO_URL = "https://your-cloud-demo.example.com"
CLOUD_DEVICE_NAME = "my-mac-mini"
CLOUD_AGENT_KEY = "replace-with-a-long-random-string"
CLOUD_AGENT_POLL_INTERVAL = 5
```

说明：

1. `CLOUD_DEMO_URL` 就是你说的“云端 ip / 地址”，这里直接改
2. `CLOUD_DEVICE_NAME` 是云端显示的设备名
3. `CLOUD_AGENT_KEY` 用来校验本地 agent 身份，云端不会主动知道它
4. 本地后端启动时会自动连云端，不需要再手动调用 `/remoteLogin`
5. 云端首页会每 3 秒自动刷新一次，设备上线后会自动显示，不需要手动刷新页面

## 完整测试流程

1. 部署并启动云端 demo
2. 在本地 `conf.py` 填好云端地址和 agent key
3. 启动本地 `sau_backend.py`
4. 打开云端首页 `/`
5. 确认设备显示为 `online`
6. 在云端首页选择设备、平台、账号名称，点“发起扫码登录”
7. 页面会跳转到远端扫码页
8. 本地 agent 自动领取任务，本地拉起浏览器
9. 二维码同步到云端页面
10. 远端扫码后，本地保存 token，云端状态更新

## 排错

如果云端首页一直看不到你的设备，先检查这两项：

1. 本地 `conf.py` 里必须是 `CLOUD_AGENT_ENABLED = True`
2. 本地打开：

```text
http://127.0.0.1:5409/cloudAgentStatus
```

返回里的 `blockedReason` 为空才表示 agent 具备启动条件。

## 手动触发方式

如果你临时不想开自动 agent，也可以继续调用本地接口：

```bash
curl "http://127.0.0.1:5409/remoteLogin?type=3&id=测试抖音号&cloudUrl=https://your-cloud-demo.example.com"
```

## 当前限制

这是一个单机单进程 demo，适合先验证链路：

1. 云端 SSE 订阅者存在内存里，不适合多实例横向扩容
2. 还没有做用户鉴权和完整权限模型
3. 账号状态镜像是云端 SQLite，后续建议换成 MySQL/Postgres
4. 任务派发是“心跳 + 轮询”，后续可以升级为 WebSocket
