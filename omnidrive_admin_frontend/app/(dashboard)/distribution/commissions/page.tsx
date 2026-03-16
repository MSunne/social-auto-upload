import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function DistributionCommissionsPage() {
  return (
    <PagePlaceholder
      eyebrow="Distribution"
      title="佣金明细"
      description="围绕待消费、待结算、已结算三段状态，追踪每一笔佣金从充值投影到消费释放的全过程。"
      apiGroup="/api/admin/v1/distribution/commissions/*"
      checklist={[
        "待消费、待结算、已结算金额视图",
        "按推广人、用户、订单、消费记录过滤",
        "佣金状态流转时间线",
        "汇总卡片和异常对账提示",
      ]}
    />
  );
}

