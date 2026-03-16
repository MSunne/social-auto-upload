import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function DistributionCommissionsPage() {
  return (
    <PagePlaceholder
      title="佣金明细"
      subtitle="核心财务模块，展示待消费、待结算、已结算的完整佣金台账。"
      focusPoints={[
        "充值触发的待消费佣金",
        "消费释放后的待结算佣金",
        "结算完成后的已结算佣金"
      ]}
    />
  );
}
