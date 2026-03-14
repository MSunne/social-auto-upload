---
name: omnibull-materials
description: |
  OmniBull 本地素材浏览。用户提到本地文件、素材目录、导出视频、OpenClaw 生成内容、本地文件内容时激活。
---

# OmniBull 素材工具

优先使用 `omnibull_materials` 浏览 OmniBull 允许访问的目录。

## 动作

### 查看根目录

```json
{ "action": "roots" }
```

### 列目录

```json
{ "action": "list", "root": "openclawWorkspace", "path": "", "limit": 100 }
```

### 读文件

```json
{ "action": "read", "root": "openclawWorkspace", "path": "exports/demo.json", "maxBytes": 65536 }
```

说明：

- `root` 必须先通过 `roots` 拿到。
- `path` 是相对对应根目录的路径。
- 文本文件会返回预览内容，二进制文件只返回元数据。
