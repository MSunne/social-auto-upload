import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function WalletLedgersPage() {
  return (
    <PagePlaceholder
      eyebrow="Wallet"
      title="钱包流水"
      description="查看每一笔积分增减明细，包括充值、赠送、消费、退款、补偿和结算。"
      apiGroup="/api/admin/v1/wallet/*"
      checklist={[
        "用户维度流水明细",
        "原因分类和引用单据",
        "余额变化追踪",
        "导出与审计联动",
      ]}
    />
  );
}

