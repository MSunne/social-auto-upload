# OmniDrive Billing Distribution API

用户侧分销与提现接口都挂在 `/api/v1/billing`，并沿用现有用户登录态。

## 1. 分销收益汇总

- `GET /api/v1/billing/distribution/summary`

响应示例：

```json
{
  "inviteeCount": 3,
  "pendingConsumeAmountCents": 1200,
  "pendingSettlementAmountCents": 800,
  "settledAmountCents": 5600,
  "availableWithdrawalAmountCents": 4300,
  "requestedWithdrawalAmountCents": 500,
  "approvedWithdrawalAmountCents": 300,
  "paidWithdrawalAmountCents": 500
}
```

## 2. 佣金明细

- `GET /api/v1/billing/commissions`

查询参数：

- `status=pending_consume|pending_settlement|settled`
- `limit`

返回每条佣金的 invitee、订单、状态、释放金额、已结算金额。

## 3. 提现单列表

- `GET /api/v1/billing/withdrawals`

查询参数：

- `limit`

## 4. 提现单详情

- `GET /api/v1/billing/withdrawals/{withdrawalId}`

## 5. 提现申请

- `POST /api/v1/billing/withdrawals`

请求示例：

```json
{
  "amountCents": 1200,
  "payoutChannel": "wechat",
  "accountMasked": "wxid****88",
  "accountPayload": {
    "accountName": "张三"
  },
  "note": "优先工作日打款",
  "proofUrls": []
}
```

当前约束：

- `amountCents` 必须大于 0
- `payoutChannel` 和 `accountMasked` 必填
- 可申请金额按 `已结算佣金 - requested - approved - paid` 计算
- 创建成功后状态为 `requested`
