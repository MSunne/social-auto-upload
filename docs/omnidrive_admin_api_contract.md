# OmniDriveAdmin API Contract

## Base

- admin base path: `/api/admin/v1`
- auth header: `Authorization: Bearer <admin-token>`
- current auth mode: database-backed admin RBAC

## Error Shape

All current admin endpoints return the existing backend error format:

```json
{
  "error": "message"
}
```

## Login

### `POST /api/admin/v1/auth/login`

Request:

```json
{
  "email": "admin@omnidrive.local",
  "password": "change-me-admin"
}
```

Response:

```json
{
  "accessToken": "jwt",
  "tokenType": "bearer",
  "admin": {
    "id": "admin-user-id",
    "email": "admin@omnidrive.local",
    "name": "OmniDriveAdmin",
    "role": "super_admin",
    "roles": ["super_admin"],
    "permissions": ["user.read", "finance.read", "distribution.read"],
    "authMode": "database_rbac"
  }
}
```

## Current Admin

### `GET /api/admin/v1/me`

Response:

```json
{
  "id": "admin-user-id",
  "email": "admin@omnidrive.local",
  "name": "OmniDriveAdmin",
  "role": "super_admin",
  "roles": ["super_admin"],
  "permissions": ["user.read", "finance.read", "distribution.read"],
  "authMode": "database_rbac"
}
```

## List Envelope

Current admin list endpoints use the same envelope:

```json
{
  "items": [],
  "pagination": {
    "page": 1,
    "pageSize": 20,
    "total": 0,
    "totalPages": 0
  },
  "summary": {},
  "filters": {}
}
```

Query conventions:

- `page`
- `pageSize`
- `query`
- route-specific filters such as `status`, `channel`, `entryType`, `resourceType`

## Dashboard

### `GET /api/admin/v1/dashboard/summary`

Response shape:

```json
{
  "metrics": {
    "userCount": 0,
    "activeUserCount": 0,
    "deviceCount": 0,
    "onlineDeviceCount": 0,
    "publishTaskCount": 0,
    "failedPublishTaskCount": 0,
    "aiJobCount": 0,
    "failedAiJobCount": 0
  },
  "finance": {
    "orderCount": 0,
    "paidOrderCount": 0,
    "rechargeAmountCents": 0,
    "walletLedgerCount": 0,
    "pendingSupportRechargeCount": 0,
    "manualSupportRechargeCount": 0
  },
  "distribution": {
    "pendingConsumeAmountCents": 0,
    "pendingSettlementAmountCents": 0,
    "settledAmountCents": 0,
    "pendingWithdrawalAmountCents": 0
  },
  "queues": {
    "needsVerifyTaskCount": 0,
    "pendingAiJobCount": 0,
    "runningAiJobCount": 0
  },
  "serverTime": "2026-03-16T00:00:00Z"
}
```

## Users

### `GET /api/admin/v1/users`

Filters:

- `query`
- `status=active|inactive`
- `page`
- `pageSize`

Item shape:

```json
{
  "user": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User",
    "isActive": true,
    "createdAt": "2026-03-16T00:00:00Z",
    "updatedAt": "2026-03-16T00:00:00Z"
  },
  "billing": {
    "creditBalance": 0,
    "frozenCreditBalance": 0,
    "totalRechargeAmountCents": 0,
    "totalRechargeCount": 0,
    "totalConsumeCredits": 0
  },
  "assets": {
    "deviceCount": 0,
    "mediaAccountCount": 0,
    "publishTaskCount": 0,
    "aiJobCount": 0
  },
  "notes": "高风险用户，已人工复核",
  "actions": {
    "canUpdate": true,
    "canDeactivate": true,
    "canActivate": false
  }
}
```

## Devices

### `GET /api/admin/v1/devices`

Filters:

- `query`
- `status=online|offline`
- `page`
- `pageSize`

Item shape:

```json
{
  "device": {
    "id": "device-id",
    "deviceCode": "ABC123",
    "name": "mac-mini",
    "status": "online",
    "isEnabled": true,
    "load": {
      "accountCount": 0,
      "pendingTaskCount": 0,
      "failedTaskCount": 0,
      "leasedTaskCount": 0,
      "leasedAiJobCount": 0
    }
  },
  "owner": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User"
  },
  "actions": {
    "canUpdate": true,
    "canDisable": true,
    "canEnable": false,
    "canForceRelease": true
  }
}
```

### `GET /api/admin/v1/users/{userId}`

Response:

- same shape as one item from `GET /api/admin/v1/users`

### `PATCH /api/admin/v1/users/{userId}`

Request:

```json
{
  "name": "Updated User",
  "isActive": false,
  "notes": "需要人工跟进"
}
```

Notes:

- `notes` is optional
- sending `""` clears the stored notes

Response:

- refreshed row, same shape as `GET /api/admin/v1/users/{userId}`

### `POST /api/admin/v1/users/bulk-action`

Request:

```json
{
  "userIds": ["user-1", "user-2"],
  "action": "deactivate"
}
```

Supported actions:

- `activate`
- `deactivate`

Response:

```json
{
  "items": [],
  "summary": {
    "selectedCount": 2,
    "processedCount": 2,
    "successCount": 2,
    "skippedCount": 0,
    "failedCount": 0,
    "byStatus": {
      "success": 2
    },
    "byAction": {
      "deactivate": 2
    }
  },
  "serverTime": "2026-03-16T10:00:00Z"
}
```

### `GET /api/admin/v1/users/{userId}/workspace`

Response shape:

```json
{
  "record": {},
  "billingSummary": {
    "creditBalance": 0,
    "frozenCreditBalance": 0,
    "pendingRechargeCount": 0,
    "quotaBalances": []
  },
  "devices": [],
  "mediaAccounts": [],
  "publishTasks": [],
  "aiJobs": [],
  "orders": [],
  "walletLedgers": [],
  "recentAudits": []
}
```

### `GET /api/admin/v1/devices/{deviceId}`

Response:

- same shape as one item from `GET /api/admin/v1/devices`

### `PATCH /api/admin/v1/devices/{deviceId}`

Request:

```json
{
  "name": "mac-mini-ops",
  "defaultReasoningModel": "gpt-5",
  "isEnabled": false
}
```

Response:

- refreshed row, same shape as `GET /api/admin/v1/devices/{deviceId}`

### `POST /api/admin/v1/devices/bulk-action`

Request:

```json
{
  "deviceIds": ["device-1", "device-2"],
  "action": "force_release"
}
```

Supported actions:

- `enable`
- `disable`
- `force_release`

Response:

```json
{
  "items": [],
  "summary": {
    "selectedCount": 2,
    "processedCount": 2,
    "successCount": 2,
    "skippedCount": 0,
    "failedCount": 0,
    "byStatus": {
      "success": 2
    },
    "byAction": {
      "force_release": 2
    }
  },
  "serverTime": "2026-03-16T10:00:00Z"
}
```

### `POST /api/admin/v1/devices/{deviceId}/force-release`

Behavior:

- releases all active publish-task leases currently held by the device
- releases all active AI-job leases currently held by the device
- refreshes the device row after cleanup

Response:

```json
{
  "record": {},
  "releasedPublishTaskIds": ["task-1", "task-2"],
  "releasedAiJobIds": ["job-1"],
  "releasedPublishTaskCount": 2,
  "releasedAiJobCount": 1,
  "serverTime": "2026-03-16T10:00:00Z"
}
```

### `GET /api/admin/v1/devices/{deviceId}/workspace`

Response shape:

```json
{
  "record": {},
  "recentTasks": [],
  "recentAiJobs": [],
  "activeLoginSessions": [],
  "recentAccounts": [],
  "materialRoots": [],
  "skillSyncStates": []
}
```

## Media Accounts

Compatibility alias:

- every `/api/admin/v1/media-accounts/*` endpoint is also exposed under `/api/admin/v1/accounts/*`

### `GET /api/admin/v1/media-accounts`

Filters:

- `query`
- `status`
- `platform`
- `userId`
- `deviceId`
- `page`
- `pageSize`

Item shape:

```json
{
  "account": {
    "id": "account-id",
    "deviceId": "device-id",
    "platform": "douyin",
    "accountName": "creator_a",
    "status": "active",
    "load": {
      "taskCount": 0,
      "pendingTaskCount": 0,
      "runningTaskCount": 0,
      "needsVerifyTaskCount": 0,
      "failedTaskCount": 0,
      "activeLoginSessionCount": 0,
      "verificationLoginSessionCount": 0
    }
  },
  "owner": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User"
  },
  "device": {
    "id": "device-id",
    "deviceCode": "ABC123",
    "name": "mac-mini",
    "status": "online",
    "isEnabled": true
  },
  "notes": "通过商务邀请绑定",
  "actions": {
    "canUpdate": true,
    "canValidate": true,
    "canDelete": false
  }
}
```

Summary shape:

```json
{
  "totalAccountCount": 0,
  "activeAccountCount": 0,
  "inactiveAccountCount": 0
}
```

### `GET /api/admin/v1/media-accounts/{accountId}`

Response:

- same shape as one item from `GET /api/admin/v1/media-accounts`

### `PATCH /api/admin/v1/media-accounts/{accountId}`

Request:

```json
{
  "notes": "需要人工复核登录状态"
}
```

Notes:

- `notes` is optional
- sending `""` clears the stored notes

Response:

- refreshed row, same shape as `GET /api/admin/v1/media-accounts/{accountId}`

### `GET /api/admin/v1/media-accounts/{accountId}/workspace`

Response shape:

```json
{
  "record": {},
  "recentTasks": [],
  "activeLoginSessions": [],
  "recentAudits": []
}
```

### `POST /api/admin/v1/media-accounts/{accountId}/validate`

Behavior:

- creates a new login validation session for the target account owner/device
- rejected when the mirrored device is disabled

Response:

- returns the created `LoginSession`

### `DELETE /api/admin/v1/media-accounts/{accountId}`

Behavior:

- only allowed when the account has no dependent publish tasks and no active login sessions

Success response:

```json
{
  "deleted": true
}
```

### `POST /api/admin/v1/media-accounts/bulk-action`

Request:

```json
{
  "accountIds": ["account-1", "account-2"],
  "action": "validate"
}
```

Supported actions:

- `validate`
- `delete`

Response:

```json
{
  "items": [],
  "summary": {
    "selectedCount": 2,
    "processedCount": 2,
    "successCount": 2,
    "skippedCount": 0,
    "failedCount": 0,
    "byStatus": {
      "success": 2
    },
    "byAction": {
      "validate": 2
    }
  },
  "serverTime": "2026-03-16T10:00:00Z"
}
```

## Publish Tasks

### `GET /api/admin/v1/publish-tasks`

Filters:

- `query`
- `status`
- `platform`
- `userId`
- `deviceId`
- `skillId`
- `page`
- `pageSize`

Item shape:

```json
{
  "task": {
    "id": "task-id",
    "deviceId": "device-id",
    "platform": "douyin",
    "accountName": "creator_a",
    "title": "Launch Post",
    "status": "needs_verify",
    "message": "等待人工确认"
  },
  "owner": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User"
  },
  "device": {
    "id": "device-id",
    "deviceCode": "ABC123",
    "name": "mac-mini",
    "status": "online",
    "isEnabled": true
  },
  "account": {
    "id": "account-id",
    "platform": "douyin",
    "accountName": "creator_a",
    "status": "active"
  },
  "skill": {
    "id": "skill-id",
    "name": "Launch Script",
    "outputType": "video",
    "modelName": "gpt-image-1",
    "isEnabled": true
  },
  "readiness": {
    "deviceReady": true,
    "accountReady": true,
    "skillReady": true,
    "materialsReady": true,
    "issueCodes": [],
    "issues": []
  },
  "blockingDimensions": [],
  "bridge": {
    "origin": "cloud",
    "hasActiveLease": false
  },
  "actions": {
    "canCancel": true,
    "canRetry": false,
    "canForceRelease": false,
    "canResume": true,
    "canResolveManual": true
  },
  "eventCount": 0,
  "artifactCount": 0,
  "materialCount": 0
}
```

Summary shape:

```json
{
  "totalTaskCount": 0,
  "pendingCount": 0,
  "runningCount": 0,
  "needsVerifyCount": 0,
  "cancelRequestedCount": 0,
  "failedCount": 0,
  "completedCount": 0
}
```

### `GET /api/admin/v1/publish-tasks/{taskId}`

Response:

- same shape as one item from `GET /api/admin/v1/publish-tasks`

### `GET /api/admin/v1/publish-tasks/{taskId}/workspace`

Response shape:

```json
{
  "record": {},
  "events": [],
  "artifacts": [],
  "materials": [],
  "runtime": null
}
```

### `GET /api/admin/v1/publish-tasks/{taskId}/events`
### `GET /api/admin/v1/publish-tasks/{taskId}/artifacts`
### `GET /api/admin/v1/publish-tasks/{taskId}/materials`

Response:

- same shapes as the customer task workspace sub-resources

### `POST /api/admin/v1/publish-tasks/{taskId}/cancel`
### `POST /api/admin/v1/publish-tasks/{taskId}/retry`
### `POST /api/admin/v1/publish-tasks/{taskId}/force-release`

Response:

- refreshed task row, same shape as `GET /api/admin/v1/publish-tasks/{taskId}`

### `POST /api/admin/v1/publish-tasks/{taskId}/resume`

Request:

```json
{
  "message": "继续自动化执行"
}
```

Response:

- refreshed task row, same shape as `GET /api/admin/v1/publish-tasks/{taskId}`

### `POST /api/admin/v1/publish-tasks/{taskId}/manual-resolve`

Request:

```json
{
  "status": "completed",
  "message": "人工确认已发布",
  "textEvidence": "运营复核通过",
  "payload": {}
}
```

Response:

- refreshed task row, same shape as `GET /api/admin/v1/publish-tasks/{taskId}`

### `POST /api/admin/v1/publish-tasks/bulk-action`

Request:

```json
{
  "taskIds": ["task-1", "task-2"],
  "action": "retry"
}
```

`manual_resolve` also supports:

```json
{
  "taskIds": ["task-1", "task-2"],
  "action": "manual_resolve",
  "resolveStatus": "completed",
  "message": "人工确认已发布",
  "textEvidence": "运营复核通过",
  "payload": {}
}
```

Response shape:

```json
{
  "items": [],
  "summary": {
    "selectedCount": 0,
    "processedCount": 0,
    "successCount": 0,
    "skippedCount": 0,
    "failedCount": 0,
    "byStatus": {},
    "byAction": {}
  },
  "serverTime": "2026-03-16T00:00:00Z"
}
```

## AI Jobs

### `GET /api/admin/v1/ai-jobs`

Filters:

- `query`
- `status`
- `jobType`
- `source`
- `userId`
- `deviceId`
- `skillId`
- `page`
- `pageSize`

Item shape:

```json
{
  "job": {
    "id": "job-id",
    "ownerUserId": "user-id",
    "deviceId": "device-id",
    "jobType": "image",
    "modelName": "gpt-image-1",
    "status": "running",
    "deliveryStatus": "pending"
  },
  "owner": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User"
  },
  "device": {
    "id": "device-id",
    "deviceCode": "ABC123",
    "name": "mac-mini",
    "status": "online",
    "isEnabled": true
  },
  "skill": {
    "id": "skill-id",
    "name": "Poster Generator",
    "outputType": "image",
    "modelName": "gpt-image-1",
    "isEnabled": true
  },
  "model": {
    "id": "model-id",
    "vendor": "openai",
    "modelName": "gpt-image-1",
    "category": "image",
    "isEnabled": true
  },
  "bridge": {
    "source": "omnidrive_cloud",
    "deliveryStage": "generating",
    "artifactCount": 0,
    "mirroredArtifactCount": 0,
    "linkedPublishTaskCount": 0
  },
  "actions": {
    "canCancel": true,
    "canRetry": false,
    "canCreatePublishTask": false,
    "canForceRelease": false
  },
  "artifactCount": 0,
  "mirroredArtifactCount": 0,
  "publishTaskCount": 0
}
```

Summary shape:

```json
{
  "totalJobCount": 0,
  "queuedCount": 0,
  "runningCount": 0,
  "completedCount": 0,
  "failedCount": 0,
  "cancelledCount": 0,
  "pendingDeliveryCount": 0
}
```

### `GET /api/admin/v1/ai-jobs/{jobId}`

Response:

- same shape as one item from `GET /api/admin/v1/ai-jobs`

### `GET /api/admin/v1/ai-jobs/{jobId}/workspace`

Response shape:

```json
{
  "record": {},
  "artifacts": [],
  "publishTasks": []
}
```

### `GET /api/admin/v1/ai-jobs/{jobId}/artifacts`

Response:

- same shape as the customer AI job artifacts list

### `POST /api/admin/v1/ai-jobs/{jobId}/cancel`
### `POST /api/admin/v1/ai-jobs/{jobId}/retry`
### `POST /api/admin/v1/ai-jobs/{jobId}/force-release`

Response:

- refreshed AI job row, same shape as `GET /api/admin/v1/ai-jobs/{jobId}`

### `POST /api/admin/v1/ai-jobs/bulk-action`

Request:

```json
{
  "jobIds": ["job-1", "job-2"],
  "action": "retry"
}
```

Supported actions:

- `cancel`
- `retry`
- `force_release`

Response shape:

```json
{
  "items": [],
  "summary": {
    "selectedCount": 0,
    "processedCount": 0,
    "successCount": 0,
    "skippedCount": 0,
    "failedCount": 0,
    "byStatus": {},
    "byAction": {}
  },
  "serverTime": "2026-03-16T00:00:00Z"
}
```

## Pricing

### `GET /api/admin/v1/pricing/packages`

- returns all packages, including disabled packages
- still uses the admin list envelope
- summary currently includes:
  - `enabledCount`
  - `disabledCount`

### `POST /api/admin/v1/pricing/packages`

Request shape:

```json
{
  "id": "growth-plus",
  "name": "增长包 Plus",
  "packageType": "credit_topup",
  "paymentChannels": ["alipay", "wechatpay", "manual_cs"],
  "currency": "CNY",
  "priceCents": 39900,
  "creditAmount": 4500,
  "manualBonusCreditAmount": 500,
  "badge": "Popular",
  "description": "适合稳定生产和补量",
  "expiresInDays": 30,
  "isEnabled": true,
  "sortOrder": 25,
  "entitlements": [
    {
      "meterCode": "image_generation_quota",
      "grantAmount": 20,
      "grantMode": "one_time",
      "sortOrder": 20,
      "description": "额外赠送 20 次图片生成额度"
    }
  ]
}
```

Response:

- returns the created `BillingPackage`
- backend will automatically normalize one `wallet_credit` entitlement using `creditAmount + manualBonusCreditAmount`

### `PATCH /api/admin/v1/pricing/packages/{packageId}`

- partial update
- nullable fields such as `badge`, `description`, `expiresInDays`, `pricingPayload` can be cleared by sending `null`
- if `entitlements` is omitted, existing non-wallet entitlements are retained and the wallet entitlement is re-synced from credits + bonus

### `GET /api/admin/v1/pricing/rules`

These currently reuse the existing billing package and billing rule models, wrapped in the admin list envelope.

## Orders

### `GET /api/admin/v1/orders`

Filters:

- `query`
- `status`
- `channel=alipay|wechatpay|manual_cs`
- `page`
- `pageSize`

Item shape:

```json
{
  "order": {
    "id": "order-id",
    "orderNo": "RC202603160001",
    "userId": "user-id",
    "channel": "manual_cs",
    "status": "awaiting_manual_review",
    "amountCents": 9900,
    "creditAmount": 1000,
    "createdAt": "2026-03-16T00:00:00Z"
  },
  "user": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User"
  }
}
```

Summary shape:

```json
{
  "totalOrderCount": 0,
  "totalAmountCents": 0,
  "totalCreditAmount": 0,
  "totalBonusCreditAmount": 0,
  "paidOrderCount": 0,
  "awaitingManualReviewCount": 0,
  "pendingPaymentCount": 0,
  "processingCount": 0,
  "manualChannelCount": 0
}
```

### `GET /api/admin/v1/orders/{orderId}`

Response shape:

```json
{
  "record": {},
  "events": [],
  "paymentTransactions": [],
  "walletLedgers": []
}
```

- `record` has the same shape as one item from `GET /api/admin/v1/orders`
- `paymentTransactions` includes current precreate/native/manual transaction state, request/response payloads, callback payloads, and paid time
- `walletLedgers` shows到账关联的积分流水，方便排查补单或重复入账

### `GET /api/admin/v1/orders/{orderId}/events`

- returns `RechargeOrderEvent[]`
- ordered by `createdAt ASC`

## Wallet Ledgers

### `GET /api/admin/v1/wallet-ledgers`

Filters:

- `query`
- `entryType`
- `page`
- `pageSize`

Item shape:

```json
{
  "ledger": {
    "id": "ledger-id",
    "userId": "user-id",
    "entryType": "recharge_credit",
    "amountDelta": 1000,
    "balanceBefore": 0,
    "balanceAfter": 1000,
    "createdAt": "2026-03-16T00:00:00Z"
  },
  "user": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User"
  }
}
```

Summary shape:

```json
{
  "totalEntryCount": 0,
  "totalCreditIn": 0,
  "totalCreditOut": 0
}
```

### `GET /api/admin/v1/wallet-ledgers/{ledgerId}`

Response shape:

```json
{
  "record": {},
  "order": null,
  "paymentTransaction": null,
  "adjustment": null
}
```

- `record` has the same shape as one item from `GET /api/admin/v1/wallet-ledgers`
- `order` is populated when the ledger comes from a recharge order
- `paymentTransaction` is populated when the ledger is linked to a payment transaction
- `adjustment` is populated when the ledger comes from a manual compensation / deduction flow

### `POST /api/admin/v1/wallet-adjustments`

Request shape:

```json
{
  "userId": "user-id",
  "amountDelta": 500,
  "reason": "任务失败补偿",
  "note": "补偿 5 次图片生成消耗",
  "entryType": "manual_compensation",
  "referenceType": "support_ticket",
  "referenceId": "SUP-20260316-001",
  "payload": {
    "operatorNote": "客服确认后补发"
  }
}
```

Behavior:

- positive `amountDelta` creates a compensation ledger
- negative `amountDelta` creates a manual deduction ledger
- wallet balance is updated and the ledger is created inside one transaction
- backend also persists a `wallet_adjustment_requests` record for later追溯

Response shape:

```json
{
  "adjustment": {},
  "ledger": {}
}
```

## Support Recharges

### `GET /api/admin/v1/support-recharges`

Filters:

- `query`
- `status=awaiting_submission|pending_review|credited|rejected`
- `page`
- `pageSize`

Item shape:

```json
{
  "id": "order-id",
  "orderNo": "RC202603160001",
  "user": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User"
  },
  "rawStatus": "awaiting_manual_review",
  "status": "awaiting_submission",
  "amountCents": 9900,
  "baseCredits": 1000,
  "bonusCredits": 0,
  "totalCredits": 1000,
  "submittedAt": "2026-03-16T00:00:00Z",
  "reviewedAt": null,
  "creditedAt": null,
  "providerStatus": "manual_pending",
  "note": "optional"
}
```

Summary shape:

```json
{
  "awaitingSubmissionCount": 0,
  "pendingReviewCount": 0,
  "rejectedCount": 0,
  "creditedCount": 0,
  "totalRequestedAmountCents": 0,
  "totalBaseCredits": 0,
  "totalBonusCredits": 0
}
```

Status mapping:

- `awaiting_submission` -> order exists but user has not submitted manual proof yet
- `pending_review` -> user has submitted manual proof and admin can `credit` or `reject`
- `rejected` -> admin rejected the current proof; user can resubmit from the user side
- `credited` -> recharge has been confirmed and wallet/quota grants are already applied

### `GET /api/admin/v1/support-recharges/{orderId}`

Response shape:

```json
{
  "record": {
    "id": "order-id",
    "orderNo": "RC202603160001",
    "user": {
      "id": "user-id",
      "email": "user@example.com",
      "name": "User"
    },
    "rawStatus": "processing",
    "status": "pending_review",
    "amountCents": 9900,
    "baseCredits": 1000,
    "bonusCredits": 0,
    "totalCredits": 1000,
    "submittedAt": "2026-03-16T00:00:00Z",
    "reviewedAt": null,
    "creditedAt": null,
    "providerStatus": "manual_submitted",
    "note": "线下转账完成"
  },
  "order": {
    "id": "order-id",
    "orderNo": "RC202603160001",
    "userId": "user-id",
    "channel": "manual_cs",
    "status": "processing",
    "amountCents": 9900,
    "creditAmount": 1000,
    "customerServicePayload": {}
  },
  "user": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User"
  },
  "submission": {
    "status": "submitted",
    "contactChannel": "wechat",
    "contactHandle": "support_wechat",
    "paymentReference": "WX20260316001",
    "transferAmountCents": 9900,
    "proofUrls": ["https://.../proof-1.png"],
    "customerNote": "线下转账完成",
    "submittedAt": "2026-03-16T00:00:00Z"
  },
  "review": {
    "status": "pending",
    "operatorId": null,
    "operatorName": null,
    "operatorEmail": null,
    "note": null,
    "reviewedAt": null,
    "creditedAt": null
  },
  "events": [
    {
      "id": "event-id",
      "eventType": "manual_submission",
      "status": "processing",
      "message": "客服充值资料已提交，等待人工确认入账",
      "createdAt": "2026-03-16T00:00:00Z"
    }
  ],
  "actions": {
    "canCredit": true,
    "canReject": true
  }
}
```

### `GET /api/admin/v1/support-recharges/{orderId}/events`

Response:

- standard `RechargeOrderEvent[]`

### `POST /api/admin/v1/support-recharges/{orderId}/credit`

Request:

```json
{
  "note": "已核对到账",
  "paymentReference": "WX20260316001"
}
```

Behavior constraints:

- only allowed when current support recharge status is `pending_review`
- writes wallet/quota grants immediately
- appends recharge order event and admin audit event
- response shape is the same as `GET /api/admin/v1/support-recharges/{orderId}`

### `POST /api/admin/v1/support-recharges/{orderId}/reject`

Request:

```json
{
  "note": "转账截图信息不足，请重新提交"
}
```

Behavior constraints:

- only allowed when current support recharge status is `pending_review`
- does not change wallet balance
- appends recharge order event and admin audit event
- response shape is the same as `GET /api/admin/v1/support-recharges/{orderId}`

## Distribution

### `GET /api/admin/v1/distribution/relations`

Filters:

- `query`
- `status=active|inactive`
- `page`
- `pageSize`

### `POST /api/admin/v1/distribution/relations`

Request:

```json
{
  "promoterUserId": "user-promoter-id",
  "inviteeUserId": "user-invitee-id",
  "notes": "通过商务邀请绑定"
}
```

### `GET /api/admin/v1/distribution/rules`

### `POST /api/admin/v1/distribution/rules`

Request:

```json
{
  "name": "默认一级分销",
  "commissionRate": 0.15,
  "settlementThresholdCents": 1000,
  "status": "active"
}
```

### `GET /api/admin/v1/distribution/commissions`

Filters:

- `query`
- `status=pending_consume|pending_settlement|settled`
- `page`
- `pageSize`

Summary shape:

```json
{
  "totalCommissionAmountCents": 0,
  "pendingConsumeAmountCents": 0,
  "pendingSettlementAmountCents": 0,
  "settledAmountCents": 0,
  "releasedButUnsettledAmountCents": 0
}
```

### `GET /api/admin/v1/distribution/settlements`

Filters:

- `query`
- `status=pending|completed`
- `page`
- `pageSize`

### `POST /api/admin/v1/distribution/settlements`

Request:

```json
{
  "promoterUserId": "user-promoter-id",
  "note": "按当前可结算佣金生成结算批次"
}
```

Commission lifecycle:

- credited recharge creates a `pending_consume` commission item
- billed wallet usage releases commission into `pending_settlement`
- settlement batch closes currently releasable amount into `settled`

## Withdrawals

### `GET /api/admin/v1/withdrawals`

Filters:

- `query`
- `status=requested|approved|rejected|paid`
- `page`
- `pageSize`

### `GET /api/admin/v1/withdrawals/{withdrawalId}`

Response shape:

```json
{
  "record": {
    "id": "withdrawal-id",
    "promoter": {
      "id": "user-id",
      "email": "promoter@example.com",
      "name": "Promoter"
    },
    "status": "approved",
    "amountCents": 1200,
    "payoutChannel": "wechat",
    "accountMasked": "wxid****88",
    "requestedAt": "2026-03-16T00:00:00Z"
  },
  "availableAmountCents": 5600,
  "note": "优先周内打款",
  "proofUrls": [],
  "paymentReference": null
}
```

### `POST /api/admin/v1/withdrawals/{withdrawalId}/approve`
### `POST /api/admin/v1/withdrawals/{withdrawalId}/reject`
### `POST /api/admin/v1/withdrawals/{withdrawalId}/mark-paid`

Common request:

```json
{
  "note": "审核备注",
  "paymentReference": "payout-20260316-01",
  "proofUrls": ["https://example.com/proof.png"]
}
```

## Audits

### `GET /api/admin/v1/audits`

Filters:

- `query`
- `resourceType`
- `status`
- `page`
- `pageSize`

Item shape:

```json
{
  "id": "audit-id",
  "ownerUserId": "user-id",
  "ownerUser": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User"
  },
  "resourceType": "publish_task",
  "resourceId": "task-id",
  "action": "retry",
  "title": "重试任务",
  "source": "cloud_console",
  "status": "success",
  "message": "任务已重试",
  "payload": {},
  "createdAt": "2026-03-16T00:00:00Z"
}
```

## Admins

### `GET /api/admin/v1/admins`

Returns database-backed admin users with roles, permissions, and active status.

## System Config

### `GET /api/admin/v1/system-config`

Response shape:

```json
{
  "authMode": "database_rbac",
  "adminEmail": "admin@omnidrive.local",
  "s3Configured": true,
  "s3Endpoint": "https://example.com",
  "s3Bucket": "bucket",
  "aiWorkerEnabled": true,
  "paymentChannels": ["alipay", "wechatpay", "manual_cs"],
  "notes": []
}
```
