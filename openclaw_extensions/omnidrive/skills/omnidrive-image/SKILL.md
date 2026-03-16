---
name: omnidrive-image
description: |
  使用 OmniDrive 云端图片模型进行文生图、图生图和运营视觉草图生成。用户明确要求“作图”“海报”“封面图”“图片生成”时激活。
---

# OmniDrive Image

优先使用 `omnidrive_image`。

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

- 作图通常可以 `wait=true`
- 如果任务较大或用户只想创建异步任务，可以 `wait=false`
- 输出时优先返回图片 `publicUrl` 和 job 状态
