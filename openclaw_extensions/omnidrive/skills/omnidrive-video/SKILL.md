---
name: omnidrive-video
description: |
  使用 OmniDrive 云端视频模型进行文生视频和图生视频。用户明确要求“做视频”“生成镜头”“图生视频”“AI 视频”时激活。
---

# OmniDrive Video

优先使用 `omnidrive_video`。

## 最小调用

```json
{
  "prompt": "咖啡门店窗口晨光推进镜头，最后停在新品杯套标志",
  "durationSeconds": 10,
  "aspectRatio": "9:16",
  "wait": false
}
```

## 图生视频

```json
{
  "prompt": "让参考图中的人物缓慢转头并微笑，镜头轻推",
  "referenceImages": [
    {
      "url": "https://example.com/first-frame.png",
      "role": "first"
    }
  ],
  "durationSeconds": 10,
  "aspectRatio": "16:9",
  "wait": false
}
```

## 规则

- 视频默认建议 `wait=false`，先创建 job，再用 `omnidrive_job_detail` 轮询
- 如果用户明确要求等待结果，再设置 `wait=true`
- 返回结果时优先说明 job 状态、视频链接和计费状态
