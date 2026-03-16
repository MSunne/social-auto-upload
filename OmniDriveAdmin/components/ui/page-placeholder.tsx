import { SectionCard, PageHeader } from "./common";

export function PagePlaceholder({
  title,
  subtitle,
  focusPoints
}: {
  title: string;
  subtitle: string;
  focusPoints: string[];
}) {
  return (
    <>
      <PageHeader title={title} subtitle={subtitle} />
      <div className="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
        <SectionCard title="页面目标" subtitle="这里已经为 Claude 预留出页面骨架和对应路由。">
          <p className="text-sm leading-7 text-[var(--color-text-secondary)]">
            当前页面先作为占位入口，后续会补齐筛选器、数据表、详情抽屉、审批动作、导出和批量操作。
          </p>
        </SectionCard>
        <SectionCard title="设计重点" subtitle="建议 UI 优先照顾高密度数据管理场景。">
          <ul className="space-y-3 text-sm leading-7 text-[var(--color-text-secondary)]">
            {focusPoints.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </SectionCard>
      </div>
    </>
  );
}
