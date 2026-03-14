---
name: omnibull-accounts
description: |
  OmniBull 本地账号管理。用户提到本地自媒体账号、账号状态、账号校验、Cookie 有效性时激活。
---

# OmniBull 账号工具

优先使用 `omnibull_accounts`，不要自己猜账号列表。

## 动作

### 列表

```json
{ "action": "list" }
```

需要即时校验 Cookie 时：

```json
{ "action": "list", "validateCookies": true }
```

### 详情

```json
{ "action": "detail", "accountId": 12 }
```

### 校验

单个账号：

```json
{ "action": "validate", "accountId": 12 }
```

多个账号：

```json
{ "action": "validate", "accountIds": [12, 18] }
```

全部账号：

```json
{ "action": "validate", "validateAll": true }
```
