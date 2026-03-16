import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function AuditsPage() {
  return (
    <PagePlaceholder
      title="审计日志"
      subtitle="记录所有后台关键操作，支撑审核、追责和风险分析。"
      focusPoints={[
        "操作人、时间、动作、资源",
        "变更前后快照",
        "充值、佣金、提现、任务人工处理"
      ]}
    />
  );
}
