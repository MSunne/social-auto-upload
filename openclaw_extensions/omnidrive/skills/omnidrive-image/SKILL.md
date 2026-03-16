---
name: omnidrive-image
description: |
  使用 OmniDrive 云端图片模型进行文生图、图生图和运营视觉草图生成。用户明确要求“作图”“海报”“封面图”“图片生成”时激活。
---

# OmniDrive Image

优先使用 `omnidrive_image`。

默认模型来自当前已绑定 OmniBull 设备的 `defaultImageModel`。如果用户没有指定模型，又明显不知道该选什么模型，先用 `omnidrive_auth` 的 `action=status` 看当前绑定设备，再用一句简短确认提示用户是否直接使用默认作图模型继续生成。

## 最小调用

```json
{
  "prompt": "春季新品咖啡海报，玻璃杯冷萃、花瓣漂浮、晨光",
  "aspectRatio": "4:5",
  "wait": true
}
```

## 带参考图

```json
{
  "prompt": "基于参考图生成新品主视觉，保留品牌留白",
  "referenceImages": [
    {
      "url": "https://example.com/ref.png",
      "role": "reference"
    }
  ],
  "aspectRatio": "4:5",
  "resolution": "1536x1920",
  "wait": true
}
```

## 规则

- 如果用户已经提供了参考图、明确提示词、模型、画幅或分辨率，直接生成，不要重复追问
- 如果用户只说“做一张图/出个封面/帮我作图”，但没有指定模型，先简短提示：
  `可以直接使用当前设备绑定的默认作图模型来生成；如果你不补充参考图和细节，我会按你的主题自由创作。是否继续？`
- 如果用户追问“默认模型是什么”，先调用 `omnidrive_auth` 的 `action=status`，读取 `boundDevice.defaultImageModel`
- 用户确认使用默认方案后，调用 `omnidrive_image` 时可以不传 `modelName`，工具会自动回落到绑定设备默认模型
- 作图通常可以 `wait=true`
- 如果任务较大或用户只想创建异步任务，可以 `wait=false`
- 输出时优先返回图片 `publicUrl` 和 job 状态
