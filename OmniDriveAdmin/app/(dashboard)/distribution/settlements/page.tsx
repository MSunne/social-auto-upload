import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function DistributionSettlementsPage() {
  return (
    <PagePlaceholder
      title="分销结算"
      subtitle="用于创建结算批次、审核结算结果并留存付款凭证。"
      focusPoints={[
        "按周期结算",
        "批次、明细、状态、付款凭证",
        "与佣金台账联动"
      ]}
    />
  );
}
