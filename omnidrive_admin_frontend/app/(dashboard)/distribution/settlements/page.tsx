import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function DistributionSettlementsPage() {
  return (
    <PagePlaceholder
      eyebrow="Distribution"
      title="结算管理"
      description="按批次管理佣金结算，支撑人工结算、财务核对和批量状态变更。"
      apiGroup="/api/admin/v1/distribution/settlements/*"
      checklist={[
        "结算批次列表与状态",
        "批量加入待结算佣金",
        "打款确认和结算备注",
        "结算单导出与审计追踪",
      ]}
    />
  );
}

