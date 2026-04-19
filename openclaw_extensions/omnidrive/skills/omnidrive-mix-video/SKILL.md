# OmniDrive Mix Video

当用户要做“混剪 / 剪辑 / 二创 / 混剪视频”时，优先使用 `omnidrive_mix_video`。

## 适用场景

- 用固定源视频 + 固定参考音频生成混剪成片
- 查询混剪积分预估
- 查看混剪任务列表和单个任务详情
- 混剪完成后按已绑定账号自动创建发布任务

## 工具调用建议

### 1. 创建混剪

调用 `omnidrive_mix_video`，参数：

- `action="create"`
- `scriptText`
- `sourceVideos`: 数组，每项至少提供 `absolutePath` 或 `url`
- `refAudio`: 提供 `absolutePath` 或 `url`

可选自动发布参数：

- `accountId`
- `platform`
- `accountName`
- `publishAt`

### 2. 计费预估

- `action="billing_preview"`
- `scriptText`

### 3. 查询任务

- `action="tasks"`
- 可选 `status`、`limit`

### 4. 查询单个任务详情

- `action="task_detail"`
- `taskId`
