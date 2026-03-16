import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function WalletLedgersPage() {
  return (
    <PagePlaceholder
      title="钱包流水"
      subtitle="查看充值、消费、补偿、退款和佣金相关的账务流水。"
      focusPoints={[
        "按用户和来源筛选",
        "账务追溯到订单或审核单",
        "手工调整和审计关联"
      ]}
    />
  );
}
