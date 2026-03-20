"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowUpRight, Film, ListTodo, Wand2 } from "lucide-react";
import Link from "next/link";
import { PageHeader, StatusBadge, EmptyState } from "@/components/ui/common";
import { listAIJobs, listDevices, listTasks } from "@/lib/services";
import type { AIJob, Device, Task } from "@/lib/types";
import {
  buildAIJobTitle,
  formatDateTime,
  resolveAIJobStage,
  resolvePublishTaskStage,
  shouldShowAIJobInWorkflow,
} from "@/lib/workflow";

type WorkflowRow = {
  id: string;
  rawId: string;
  kind: "ai" | "publish";
  title: string;
  typeLabel: string;
  stageKey: string;
  stageLabel: string;
  description?: string;
  deviceName: string;
  accountLabel: string;
  modelLabel: string;
  updatedAt?: string | null;
  href?: string;
};

const SOURCE_FILTERS = [
  { key: "all", label: "全部流程" },
  { key: "ai", label: "AI 制作" },
  { key: "publish", label: "发布执行" },
] as const;

const STAGE_FILTERS = [
  { key: "all", label: "全部状态" },
  { key: "scheduled", label: "未开始" },
  { key: "generating", label: "正在做内容" },
  { key: "publishing", label: "正在发布" },
  { key: "published", label: "已发布" },
  { key: "failed", label: "失败" },
] as const;

function stageGroup(stageKey: string) {
  if (stageKey === "scheduled" || stageKey === "publish_queued" || stageKey === "queued_generation") {
    return "scheduled";
  }
  if (stageKey === "storyboarding" || stageKey === "generating" || stageKey === "output_ready" || stageKey === "imported") {
    return "generating";
  }
  if (stageKey === "publishing" || stageKey === "needs_verify" || stageKey === "cancel_requested") {
    return "publishing";
  }
  if (stageKey === "published") {
    return "published";
  }
  if (stageKey === "publish_failed" || stageKey === "cancelled") {
    return "failed";
  }
  return "all";
}

export default function TasksPage() {
  const [sourceFilter, setSourceFilter] = useState<(typeof SOURCE_FILTERS)[number]["key"]>("all");
  const [stageFilter, setStageFilter] = useState<(typeof STAGE_FILTERS)[number]["key"]>("all");

  const { data: devices = [] } = useQuery<Device[]>({
    queryKey: ["devices"],
    queryFn: listDevices,
  });
  const { data: publishTasks = [], isLoading: publishLoading } = useQuery<Task[]>({
    queryKey: ["tasks"],
    queryFn: () => listTasks({ limit: 200 }),
  });
  const { data: aiJobs = [], isLoading: aiLoading } = useQuery<AIJob[]>({
    queryKey: ["aiJobs"],
    queryFn: () => listAIJobs({ limit: 200, excludeSource: "omnidrive_chat" }),
  });

  const deviceMap = useMemo(() => {
    return Object.fromEntries(devices.map((device) => [device.id, device.name]));
  }, [devices]);

  const rows = useMemo<WorkflowRow[]>(() => {
    const aiRows = aiJobs.filter(shouldShowAIJobInWorkflow).map((job) => {
      const stage = resolveAIJobStage(job);
      return {
        id: `ai-${job.id}`,
        rawId: job.id,
        kind: "ai" as const,
        title: buildAIJobTitle(job),
        typeLabel: job.jobType === "video" ? "视频制作" : job.jobType === "image" ? "图文制作" : "文本制作",
        stageKey: stage.key,
        stageLabel: stage.label,
        description: stage.description,
        deviceName: job.deviceId ? deviceMap[job.deviceId] || job.deviceId : "未绑定节点",
        accountLabel: job.localPublishTaskId ? `已生成本地发布任务 ${job.localPublishTaskId}` : "尚未进入发布",
        modelLabel: job.modelName,
        updatedAt: job.updatedAt,
      };
    });

    const publishRows = publishTasks.map((task) => {
      const stage = resolvePublishTaskStage(task);
      return {
        id: `publish-${task.id}`,
        rawId: task.id,
        kind: "publish" as const,
        title: task.title,
        typeLabel: "发布执行",
        stageKey: stage.key,
        stageLabel: stage.label,
        description: stage.description,
        deviceName: deviceMap[task.deviceId] || task.deviceId,
        accountLabel: `${task.platform} / ${task.accountName}`,
        modelLabel: task.skillId ? `技能 ${task.skillId}` : "直接发布",
        updatedAt: task.updatedAt,
        href: `/tasks/${task.id}`,
      };
    });

    return [...aiRows, ...publishRows].sort((left, right) => {
      return new Date(right.updatedAt || 0).getTime() - new Date(left.updatedAt || 0).getTime();
    });
  }, [aiJobs, deviceMap, publishTasks]);

  const filteredRows = rows.filter((row) => {
    if (sourceFilter !== "all" && row.kind !== sourceFilter) {
      return false;
    }
    if (stageFilter !== "all" && stageGroup(row.stageKey) !== stageFilter) {
      return false;
    }
    return true;
  });

  const isLoading = publishLoading || aiLoading;

  return (
    <>
      <PageHeader
        title="OpenClaw 任务"
        subtitle="把 AI 制作、分镜优化、产物回流、发布执行放在同一张表里查看，方便随时追踪整条链路。"
      />

      <div className="mb-4 flex flex-wrap items-center gap-2">
        {SOURCE_FILTERS.map((filter) => (
          <button
            key={filter.key}
            type="button"
            onClick={() => setSourceFilter(filter.key)}
            className={`rounded-full px-3 py-1.5 text-sm transition-colors ${
              sourceFilter === filter.key
                ? "bg-accent/15 text-accent"
                : "bg-surface text-text-secondary hover:text-text-primary"
            }`}
          >
            {filter.label}
          </button>
        ))}
      </div>

      <div className="mb-6 flex flex-wrap items-center gap-2">
        {STAGE_FILTERS.map((filter) => (
          <button
            key={filter.key}
            type="button"
            onClick={() => setStageFilter(filter.key)}
            className={`rounded-full px-3 py-1 text-xs transition-colors ${
              stageFilter === filter.key
                ? "bg-accent/20 text-accent ring-1 ring-accent/40"
                : "bg-surface text-text-muted hover:text-text-primary"
            }`}
          >
            {filter.label}
          </button>
        ))}
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center rounded-2xl border border-border py-16 text-text-secondary">
          <div className="mr-3 h-5 w-5 animate-spin rounded-full border-2 border-accent border-t-transparent" />
          正在读取流程任务...
        </div>
      ) : filteredRows.length > 0 ? (
        <div className="overflow-hidden rounded-3xl border border-border bg-surface">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b border-border bg-surface-hover/40 text-left text-xs uppercase tracking-wider text-text-muted">
                <tr>
                  <th className="px-5 py-4">任务标题</th>
                  <th className="px-5 py-4">类型</th>
                  <th className="px-5 py-4">节点 / 账号</th>
                  <th className="px-5 py-4">当前阶段</th>
                  <th className="px-5 py-4">最近更新时间</th>
                  <th className="px-5 py-4 text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filteredRows.map((row) => (
                  <tr key={row.id} className="transition-colors hover:bg-surface-hover/20">
                    <td className="px-5 py-4 align-top">
                      <div className="flex items-start gap-3">
                        <div className="mt-0.5 flex h-9 w-9 items-center justify-center rounded-xl bg-accent/10">
                          {row.kind === "ai" ? (
                            <Wand2 className="h-4 w-4 text-accent" />
                          ) : (
                            <Film className="h-4 w-4 text-cyan" />
                          )}
                        </div>
                        <div>
                          <p className="font-semibold text-text-primary">{row.title}</p>
                          <p className="mt-1 text-xs text-text-secondary">{row.modelLabel}</p>
                          {row.description ? (
                            <p className="mt-2 max-w-md text-xs leading-5 text-text-secondary">
                              {row.description}
                            </p>
                          ) : null}
                        </div>
                      </div>
                    </td>
                    <td className="px-5 py-4 align-top">
                      <div className="inline-flex rounded-full bg-surface-hover px-2.5 py-1 text-xs text-text-primary">
                        {row.typeLabel}
                      </div>
                    </td>
                    <td className="px-5 py-4 align-top">
                      <p className="text-sm text-text-primary">{row.deviceName}</p>
                      <p className="mt-1 text-xs text-text-secondary">{row.accountLabel}</p>
                    </td>
                    <td className="px-5 py-4 align-top">
                      <div className="space-y-2">
                        <StatusBadge status={row.stageKey} />
                        <p className="text-xs text-text-secondary">{row.stageLabel}</p>
                      </div>
                    </td>
                    <td className="px-5 py-4 align-top text-text-secondary">
                      {formatDateTime(row.updatedAt)}
                    </td>
                    <td className="px-5 py-4 align-top">
                      <div className="flex justify-end">
                        {row.href ? (
                          <Link
                            href={row.href}
                            className="inline-flex items-center gap-1 text-xs font-medium text-accent transition-colors hover:text-cyan"
                          >
                            详情
                            <ArrowUpRight className="h-3 w-3" />
                          </Link>
                        ) : (
                          <span className="text-xs text-text-muted">生成任务已在本表跟踪</span>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : (
        <EmptyState
          icon={<ListTodo className="h-6 w-6" />}
          title="暂无任务"
          description="当前还没有可展示的 AI 制作或发布执行记录。"
        />
      )}
    </>
  );
}
