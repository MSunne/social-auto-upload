import { BarChart3, Coins, ReceiptText, ShieldCheck } from "lucide-react";
import { MetricTile, PageHeader, SectionCard } from "@/components/ui/common";

export default function DashboardPage() {
  return (
    <>
      <PageHeader
        title="管理总览"
        subtitle="这里将承接运营、财务、客服审核与分销结算的总控工作台。"
      />

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricTile label="客服充值待审核" value="0" hint="等待接入后台接口" icon={<ReceiptText className="h-5 w-5" />} />
        <MetricTile label="待结算佣金" value="0" hint="按消费释放后进入待结算" icon={<Coins className="h-5 w-5" />} />
        <MetricTile label="异常任务" value="0" hint="待接入全局任务统计" icon={<BarChart3 className="h-5 w-5" />} />
        <MetricTile label="审计风险项" value="0" hint="待接入审计中心" icon={<ShieldCheck className="h-5 w-5" />} />
      </section>

      <section className="mt-6 grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
        <SectionCard title="阶段重点" subtitle="建议先落地后台底层能力">
          <ul className="space-y-3 text-sm leading-7 text-[var(--color-text-secondary)]">
            <li>1. Admin 鉴权、角色、权限、审计日志</li>
            <li>2. 客服充值审核和钱包入账链路</li>
            <li>3. 分销关系、佣金台账、结算单</li>
            <li>4. 订单、钱包、提现的财务核对能力</li>
            <li>5. 用户、设备、任务、AI 作业的全局管理</li>
          </ul>
        </SectionCard>

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
