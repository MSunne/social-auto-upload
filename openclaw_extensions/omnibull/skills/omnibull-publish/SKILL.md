---
name: omnibull-publish
description: |
  OmniBull 媒体发布与任务查询。用户明确要求发布到抖音、视频号、快手，或要求查询发布任务状态时激活。
---

# OmniBull 发布工具

发布前先确认：

1. 账号 ID 是否存在。
2. 素材文件是否存在。
3. 平台类型是否正确：`2=视频号`，`3=抖音`，`4=快手`。

账号先用 `omnibull_accounts` 查，素材先用 `omnibull_materials` 查。

## 发布任务

```json
{
  "action": "enqueue",
  "platformType": 3,
  "title": "示例标题",
  "tags": ["测试"],
  "accountIds": [12],
  "files": [
    {
      "root": "openclawWorkspace",
      "path": "exports/video/demo.mp4"
    }
  ]
}
```

## 查询任务列表

```json
{ "action": "tasks", "limit": 20 }
```

按状态筛选：

```json
{ "action": "tasks", "status": "needs_verify", "limit": 20 }
```

## 查询单个任务

```json
{ "action": "task_detail", "taskUuid": "task-uuid" }
```

## 规则

- 只有在用户明确要求发布时才使用 `enqueue`。
- 如果用户只是想确认素材或账号，不要直接发布。
- 遇到 `needs_verify`，应明确告诉用户该任务需要人工处理。
