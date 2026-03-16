import { OverviewCards } from "./overview-cards";
import { QueuesOverview } from "./queues-overview";
import { PageHeader, SectionCard } from "@/components/ui/common";

export default function DashboardPage() {
  return (
    <>
      <PageHeader
        title="管理总览"
        subtitle="这里将承接运营、财务、客服审核与分销结算的总控工作台。"
      />

      <OverviewCards />

      <section className="mt-6 grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
        <QueuesOverview />

        <SectionCard title="目录状态" subtitle="前端目录已为 Claude 准备好">
          <p className="text-sm leading-7 text-[var(--color-text-secondary)]">
            这套管理端是独立工程，不与现有客户端控制台共用路由。接下来可由 Claude
            直接在本目录扩展页面、表格、筛选器、审批抽屉和财务视图。
          </p>
        </SectionCard>
      </section>
    </>
  );
}
