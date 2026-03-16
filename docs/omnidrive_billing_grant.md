# OmniDrive Billing Grant

这个命令用于给测试账号手动发放钱包积分、图片次数、视频次数。

当前充值链路还没有把订单真正结算进钱包时，这个工具可以先把真实 AI 生成功能跑起来。

## 用法

```bash
cd /Volumes/mud/project/github/social-auto-upload/omnidrive_cloud

go run ./cmd/omnidrive-billing-grant \
  -email your-user@example.com \
  -credits 2000 \
  -image-quota 20 \
  -video-quota 5 \
  -expires-in-days 30 \
  -reason "内测发放"
```

也可以直接按用户 ID 发放：

```bash
cd /Volumes/mud/project/github/social-auto-upload/omnidrive_cloud

go run ./cmd/omnidrive-billing-grant \
  -user-id your-user-id \
  -credits 1000
```

## 参数

- `-user-id`
- `-email`
- `-credits`
- `-image-quota`
- `-video-quota`
- `-expires-in-days`
- `-reason`
- `-reference-type`
- `-reference-id`

## 效果

命令会：

- 更新 `billing_wallets`
- 写入 `wallet_ledgers`
- 新增 `billing_quota_accounts`
- 写入 `billing_quota_ledgers`
- 最后输出当前用户的 billing summary
