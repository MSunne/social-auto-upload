---
name: hermes-run-skill
description: |
  通过本地 Hermes bridge 调用 OmniBull / OmniDrive 薄技能。用户要让 Hermes 代为操作账号、素材、发布、AI 任务时激活。
---

# Hermes Run Skill

优先使用 `hermes_run_skill` 或 gateway method `hermes.run_skill`。

当前支持的 `skillName`：

- `omnibull-accounts`
- `omnibull-materials`
- `omnibull-publish`
- `omnidrive-chat`
- `omnidrive-image`
- `omnidrive-video`
- `omnidrive-jobs`

## 示例

```json
{
  "skillName": "omnibull-accounts",
  "action": "list"
}
```
