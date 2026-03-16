"use client";

import { useDashboardSummary } from "@/lib/hooks/useDashboard";
import { SectionCard } from "@/components/ui/common";
import { Activity, AlertTriangle, PlayCircle } from "lucide-react";

export function QueuesOverview() {
  const { data, isLoading, error } = useDashboardSummary();

  if (isLoading || error || !data) {
    return null; // Handle loading/error in the main overview cards to avoid duplicate spinners
  }

  const queues = data.queues;
  const metrics = data.metrics;

  const queueItems = [
    {
      title: "待人工校验任务 (Publish Tasks)",
      count: queues.needsVerifyTaskCount,
      icon: <Activity className="h-4 w-4 text-[var(--color-primary)]" />,
      desc: "由于账号未登录或平台风控，需管理员手动介入的任务列表",
      alert: queues.needsVerifyTaskCount > 0,
    },
    {
      title: "排队中 AI 作业 (AI Jobs)",
      count: queues.pendingAiJobCount,
      icon: <PlayCircle className="h-4 w-4 text-blue-500" />,
      desc: "已提交但尚未分配到 OmniBull 边缘节点或云端节点的 AI 任务",
      alert: queues.pendingAiJobCount > 50,
    },
    {
      title: "失败异常任务 (Failed)",
      count: metrics.failedPublishTaskCount + metrics.failedAiJobCount,
      icon: <AlertTriangle className="h-4 w-4 text-red-500" />,
      desc: `发布失败: ${metrics.failedPublishTaskCount} | AI作业失败: ${metrics.failedAiJobCount}`,
      alert: metrics.failedPublishTaskCount + metrics.failedAiJobCount > 0,
    }
  ];

  return (
    <SectionCard title="核心业务流队列 (Queues)" subtitle="实时监控执行池与风控异常">
      <div className="flex flex-col gap-4 mt-2">
        {queueItems.map((item, idx) => (
          <div key={idx} className="flex items-start justify-between p-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] hover:border-[var(--color-border-hover)] transition-colors">
            <div className="flex items-start gap-3">
              <div className="mt-1">{item.icon}</div>
              <div>
                <p className="text-sm font-medium text-[var(--color-text-primary)]">{item.title}</p>
                <p className="text-xs text-[var(--color-text-secondary)] mt-1">{item.desc}</p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <span className={`text-lg font-mono font-semibold ${item.alert ? "text-amber-500" : "text-[var(--color-text-primary)]"}`}>
                {item.count}
              </span>
            </div>
          </div>
        ))}
      </div>
    </SectionCard>
  );
}
