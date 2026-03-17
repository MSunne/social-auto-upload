---
name: omnidrive-video
description: |
  使用 OmniDrive 云端视频模型进行文生视频和图生视频。用户明确要求“做视频”“生成镜头”“图生视频”“AI 视频”时激活。
---

# OmniDrive Video

优先使用 `omnidrive_video`。

默认模型来自当前已绑定 OmniBull 设备的 `defaultVideoModel`。默认时长按 8 秒处理；如果用户没有给时长、模型、参考图或细节提示，就先用一句简短确认，询问是否直接使用默认模型、默认时长 8 秒，并按主题自由创作。

## 最小调用

```json
{
  "prompt": "咖啡门店窗口晨光推进镜头，最后停在新品杯套标志",
  "durationSeconds": 8,
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
  "durationSeconds": 8,
  "aspectRatio": "16:9",
  "wait": false
}
```

## 规则

- 如果用户已经提供了参考图、明确提示词、模型、时长、比例或分辨率，直接生成，不要重复追问
- 如果用户只说“做个视频/生成一个镜头/做条 AI 视频”，但没有指定模型，先简短提示：
  `可以直接使用当前设备绑定的默认视频模型，按默认时长 8 秒生成；如果你不补充参考图和更多细节，我会按主题自由创作。是否继续？`
- 如果用户想知道默认模型是什么，先调用 `omnidrive_auth` 的 `action=status`，读取 `boundDevice.defaultVideoModel`
- 如果用户要求“把 OmniDrive 默认视频模型切到某个模型”，先用 `omnidrive_models` 确认模型可用，再用 `omnidrive_device_config` 的 `action=set_defaults` 更新 `defaultVideoModel`
- 如果当前话题已经明确是在 OmniDrive 做视频场景里，用户只说“换成某个模型”或“当前是什么模型”，默认理解为在问 OmniDrive 默认视频模型
- 用户确认使用默认方案后，调用 `omnidrive_video` 时优先显式传入 `durationSeconds: 8`；即使省略，工具也会自动回落到默认 8 秒
- 视频默认建议 `wait=false`，先创建 job，再用 `omnidrive_job_detail` 轮询
- 如果用户明确要求等待结果，再设置 `wait=true`
- 返回结果时优先说明 job 状态、视频链接和计费状态
