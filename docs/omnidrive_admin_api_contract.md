# OmniDriveAdmin API Contract

## Base

- admin base path: `/api/admin/v1`
- auth header: `Authorization: Bearer <admin-token>`
- current auth mode: bootstrap env admin

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
    "id": "bootstrap-admin",
    "email": "admin@omnidrive.local",
    "name": "OmniDriveAdmin",
    "role": "super_admin",
    "roles": ["super_admin"],
    "permissions": ["user.read", "finance.read"],
    "authMode": "bootstrap_env"
  }
}
```

## Current Admin

### `GET /api/admin/v1/me`

Response:

```json
{
  "id": "bootstrap-admin",
  "email": "admin@omnidrive.local",
  "name": "OmniDriveAdmin",
  "role": "super_admin",
  "roles": ["super_admin"],
  "permissions": ["user.read", "finance.read"],
  "authMode": "bootstrap_env"
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
      "failedTaskCount": 0
    }
  },
  "owner": {
    "id": "user-id",
    "email": "user@example.com",
    "name": "User"
  }
}
```

## Pricing

### `GET /api/admin/v1/pricing/packages`
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
  "paidOrderCount": 0,
  "awaitingManualReviewCount": 0,
  "pendingPaymentCount": 0,
  "processingCount": 0,
  "manualChannelCount": 0
}
```

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

Current phase returns stable empty envelopes for these routes:

- `GET /api/admin/v1/distribution/relations`
- `GET /api/admin/v1/distribution/commissions`
- `GET /api/admin/v1/distribution/settlements`
- `GET /api/admin/v1/withdrawals`

The `summary.phase` field is currently:

- `schema_pending`

This allows frontend pages to build empty states now without waiting for schema delivery.

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

Current phase returns the bootstrap admin only, wrapped in the list envelope.

## System Config

### `GET /api/admin/v1/system-config`

Response shape:

```json
{
  "authMode": "bootstrap_env",
  "adminEmail": "admin@omnidrive.local",
  "s3Configured": true,
  "s3Endpoint": "https://example.com",
  "s3Bucket": "bucket",
  "aiWorkerEnabled": true,
  "paymentChannels": ["alipay", "wechatpay", "manual_cs"],
  "notes": []
}
```
