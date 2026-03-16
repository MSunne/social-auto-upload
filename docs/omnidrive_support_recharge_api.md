# OmniDrive 客服充值联调说明

## 流程

1. 用户创建客服充值订单
   - `POST /api/v1/billing/orders`
   - body:

```json
{
  "packageId": "starter",
  "channel": "manual_cs"
}
```

2. 用户提交转账资料
   - `POST /api/v1/billing/orders/{orderId}/manual-submit`
   - body:

```json
{
  "contactChannel": "wechat",
  "contactHandle": "support_user",
  "paymentReference": "TRANSFER-20260316-001",
  "transferAmountCents": 9900,
  "proofUrls": [
    "https://example.com/proof-1.png"
  ],
  "customerNote": "已联系微信客服"
}
```

3. 管理端查看待审核订单
   - `GET /api/admin/v1/support-recharges`
   - `status` 支持:
     - `awaiting_submission`
     - `pending_review`
     - `credited`
     - `rejected`

4. 管理端查看订单详情和事件流
   - `GET /api/admin/v1/support-recharges/{orderId}`
   - `GET /api/admin/v1/support-recharges/{orderId}/events`

5. 管理端确认入账
   - `POST /api/admin/v1/support-recharges/{orderId}/credit`
   - body:

```json
{
  "note": "已核对客服收款记录",
  "paymentReference": "TRANSFER-20260316-001"
}
```

6. 管理端驳回
   - `POST /api/admin/v1/support-recharges/{orderId}/reject`
   - body:

```json
{
  "note": "凭证不清晰，请重新上传"
}
```

## 状态说明

- `awaiting_manual_review`: 用户已创建客服充值订单，尚未提交转账资料
- `processing`: 用户已提交资料，等待人工确认
- `credited`: 已完成入账，并已发放钱包积分/套餐额度
- `rejected`: 已驳回，用户可重新提交资料

## 入账行为

- 钱包积分会写入 `billing_wallets` 和 `wallet_ledgers`
- 套餐次数会写入 `billing_quota_accounts` 和 `billing_quota_ledgers`
- 支付流水会更新 `payment_transactions`
- 订单事件会写入 `recharge_order_events`
- 管理端操作会写入 `audit_events`
