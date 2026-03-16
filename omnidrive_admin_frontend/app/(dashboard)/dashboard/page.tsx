import { PagePlaceholder } from "@/components/ui/page-placeholder";

export default function DashboardPage() {
  return (
    <PagePlaceholder
      eyebrow="Overview"
      title="运营总览"
      description="面向运营、财务、客服和审核团队的总览页。这里将汇总充值、佣金、待审核单据、异常任务、离线设备和高风险账户。"
      apiGroup="/api/admin/v1/dashboard/*"
      checklist={[
        "平台级 KPI 卡片与趋势图",
        "待审核客服充值与提现队列",
        "待结算佣金和今日消费快照",
        "异常设备、任务、支付、AI 作业告警",
      ]}
    />
  );
}

