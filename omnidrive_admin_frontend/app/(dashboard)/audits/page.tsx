import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function AuditsPage() {
  return (
    <PagePlaceholder
      eyebrow="Audits"
      title="审计日志"
      description="记录所有后台敏感操作，尤其是充值、结算、提现、冻结和任务人工处理。"
      apiGroup="/api/admin/v1/audits/*"
      checklist={[
        "按操作者、资源、动作过滤",
        "前后变更详情",
        "资金相关敏感操作视图",
        "导出与留痕审查",
      ]}
    />
  );
}

