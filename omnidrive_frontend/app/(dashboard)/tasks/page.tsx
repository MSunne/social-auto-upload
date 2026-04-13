"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, ArrowUpRight, Film, ListTodo, RefreshCw, Wand2 } from "lucide-react";
import Link from "next/link";
import { PageHeader, StatusBadge, EmptyState } from "@/components/ui/common";
import { getModelDisplayName } from "@/lib/model-display";
import { listAIJobs, listDevices, listTasks } from "@/lib/services";
import type { AIJob, Device, Task } from "@/lib/types";
import {
  buildAIJobTitle,
  formatAIJobTypeLabel,
  formatDateTime,
  resolveAIJobStage,
  resolveAIJobWorkflowTime,
  resolvePublishTaskStage,
  resolvePublishTaskWorkflowTime,
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
  workflowTime?: string | null;
  updatedAt?: string | null;
  href?: string;
};

const WORKFLOW_PAGE_SIZE = 20;

const SOURCE_FILTERS = [
  { key: "all", label: "全部流程" },
  { key: "ai", label: "AI 制作" },
  { key: "publish", label: "发布执行" },
] as const;

const STAGE_FILTERS = [
  { key: "all", label: "全部状态" },
  { key: "scheduled", label: "未开始" },
  { key: "waiting_recharge", label: "欠费" },
  { key: "generating", label: "正在做内容" },
  { key: "publishing", label: "正在发布" },
  { key: "published", label: "已发布" },
  { key: "failed", label: "失败" },
] as const;

function stageGroup(stageKey: string) {
  if (stageKey === "scheduled" || stageKey === "publish_queued" || stageKey === "queued_generation") {
    return "scheduled";
  }
  if (stageKey === "waiting_recharge") {
    return "waiting_recharge";
  }
  if (stageKey === "covering" || stageKey === "storyboarding" || stageKey === "generating" || stageKey === "output_ready" || stageKey === "imported") {
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

function getWorkflowRowTime(row: WorkflowRow) {
  return new Date(row.workflowTime || row.updatedAt || 0).getTime();
}

function mergeAIJobs(...groups: AIJob[][]) {
  const jobMap = new Map<string, AIJob>();
  groups.flat().forEach((job) => {
    jobMap.set(job.id, job);
  });
  return Array.from(jobMap.values()).sort((left, right) => {
    const timeDiff = new Date(right.updatedAt || 0).getTime() - new Date(left.updatedAt || 0).getTime();
    if (timeDiff !== 0) {
      return timeDiff;
    }
    return right.id.localeCompare(left.id);
  });
}

function mergePublishTasks(...groups: Task[][]) {
  const taskMap = new Map<string, Task>();
  groups.flat().forEach((task) => {
    taskMap.set(task.id, task);
  });
  return Array.from(taskMap.values()).sort((left, right) => {
    const timeDiff = new Date(right.updatedAt || 0).getTime() - new Date(left.updatedAt || 0).getTime();
    if (timeDiff !== 0) {
      return timeDiff;
    }
    return right.id.localeCompare(left.id);
  });
}

function isTerminalAIJob(job: AIJob) {
  return ["success", "completed", "failed", "cancelled"].includes(job.status);
}

function isTerminalPublishTask(task: Task) {
  return ["success", "completed", "failed", "cancelled"].includes(task.status);
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) {
    return error.message.trim();
  }
  return fallback;
}

export default function TasksPage() {
  const [sourceFilter, setSourceFilter] = useState<(typeof SOURCE_FILTERS)[number]["key"]>("all");
  const [stageFilter, setStageFilter] = useState<(typeof STAGE_FILTERS)[number]["key"]>("all");
  const [olderPublishTasks, setOlderPublishTasks] = useState<Task[]>([]);
  const [olderAIJobs, setOlderAIJobs] = useState<AIJob[]>([]);
  const [hasMorePublishTasks, setHasMorePublishTasks] = useState(true);
  const [hasMoreAIJobs, setHasMoreAIJobs] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [loadMoreError, setLoadMoreError] = useState("");

  const { data: devices = [] } = useQuery<Device[]>({
    queryKey: ["devices"],
    queryFn: listDevices,
  });
  const {
    data: latestPublishTasks = [],
    isLoading: publishLoading,
    error: publishError,
    isRefetchError: publishRefetchError,
    refetch: refetchPublishTasks,
  } = useQuery<Task[]>({
    queryKey: ["tasks", "workflow", "recent"],
    queryFn: () => listTasks({ limit: WORKFLOW_PAGE_SIZE }),
    refetchInterval: ({ state }) => {
      const items = state.data as Task[] | undefined;
      return items?.some((task) => !isTerminalPublishTask(task)) ? 10_000 : false;
    },
    staleTime: 10_000,
  });
  const {
    data: latestAIJobs = [],
    isLoading: aiLoading,
    error: aiError,
    isRefetchError: aiRefetchError,
    refetch: refetchAIJobs,
  } = useQuery<AIJob[]>({
    queryKey: ["aiJobs", "workflow", "recent"],
    queryFn: () => listAIJobs({ limit: WORKFLOW_PAGE_SIZE, excludeSource: "omnidrive_chat", payloadMode: "summary" }),
    refetchInterval: ({ state }) => {
      const items = state.data as AIJob[] | undefined;
      return items?.some((job) => shouldShowAIJobInWorkflow(job) && !isTerminalAIJob(job)) ? 10_000 : false;
    },
    staleTime: 10_000,
  });

  useEffect(() => {
    if (olderPublishTasks.length === 0) {
      setHasMorePublishTasks(latestPublishTasks.length === WORKFLOW_PAGE_SIZE);
    }
  }, [latestPublishTasks, olderPublishTasks.length]);

  useEffect(() => {
    if (olderAIJobs.length === 0) {
      setHasMoreAIJobs(latestAIJobs.length === WORKFLOW_PAGE_SIZE);
    }
  }, [latestAIJobs, olderAIJobs.length]);

  const publishTasks = useMemo(() => mergePublishTasks(latestPublishTasks, olderPublishTasks), [latestPublishTasks, olderPublishTasks]);
  const aiJobs = useMemo(() => mergeAIJobs(latestAIJobs, olderAIJobs), [latestAIJobs, olderAIJobs]);

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
        typeLabel: formatAIJobTypeLabel(job.jobType),
        stageKey: stage.key,
        stageLabel: stage.label,
        description: stage.description,
        deviceName: job.deviceId ? deviceMap[job.deviceId] || job.deviceId : "未绑定节点",
        accountLabel: job.localPublishTaskId ? `已生成本地发布任务 ${job.localPublishTaskId}` : "尚未进入发布",
        modelLabel: getModelDisplayName(job),
        workflowTime: resolveAIJobWorkflowTime(job),
        updatedAt: job.updatedAt,
        href: `/tasks/ai/${job.id}`,
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
        workflowTime: resolvePublishTaskWorkflowTime(task),
        updatedAt: task.updatedAt,
        href: `/tasks/${task.id}`,
      };
    });

    return [...aiRows, ...publishRows].sort((left, right) => {
      const timeDiff = getWorkflowRowTime(right) - getWorkflowRowTime(left);
      if (timeDiff !== 0) {
        return timeDiff;
      }
      return right.id.localeCompare(left.id);
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
  const hasMore = hasMorePublishTasks || hasMoreAIJobs;
  const initialErrorMessage = getErrorMessage(aiError || publishError, "任务列表加载失败，请稍后重试");

  async function handleLoadMore() {
    if (isLoadingMore || !hasMore) {
      return;
    }

    const publishCursor = publishTasks[publishTasks.length - 1];
    const aiCursor = aiJobs[aiJobs.length - 1];
    setIsLoadingMore(true);
    setLoadMoreError("");

    try {
      const [nextPublishTasks, nextAIJobs] = await Promise.all([
        hasMorePublishTasks && publishCursor?.updatedAt
          ? listTasks({
              limit: WORKFLOW_PAGE_SIZE,
              beforeUpdatedAt: publishCursor.updatedAt,
              beforeId: publishCursor.id,
            })
          : Promise.resolve([] as Task[]),
        hasMoreAIJobs && aiCursor?.updatedAt
          ? listAIJobs({
              limit: WORKFLOW_PAGE_SIZE,
              excludeSource: "omnidrive_chat",
              payloadMode: "summary",
              beforeUpdatedAt: aiCursor.updatedAt,
              beforeId: aiCursor.id,
            })
          : Promise.resolve([] as AIJob[]),
      ]);

      if (nextPublishTasks.length > 0) {
        setOlderPublishTasks((current) => mergePublishTasks(current, nextPublishTasks));
      }
      if (nextAIJobs.length > 0) {
        setOlderAIJobs((current) => mergeAIJobs(current, nextAIJobs));
      }
      if (hasMorePublishTasks) {
        setHasMorePublishTasks(nextPublishTasks.length === WORKFLOW_PAGE_SIZE);
      }
      if (hasMoreAIJobs) {
        setHasMoreAIJobs(nextAIJobs.length === WORKFLOW_PAGE_SIZE);
      }
    } catch (loadError) {
      setLoadMoreError(getErrorMessage(loadError, "加载更多失败，请稍后重试"));
    } finally {
      setIsLoadingMore(false);
    }
  }

  return (
    <>
      <PageHeader
        title="OpenClaw 任务"
        subtitle="把 AI 制作、分镜优化、产物回流、发布执行放在同一张表里查看，方便随时追踪整条链路。"
        actions={
          <button
            type="button"
            onClick={() => {
              void refetchPublishTasks();
              void refetchAIJobs();
            }}
            className="inline-flex items-center gap-2 rounded-lg border border-border px-3 py-2 text-sm text-text-secondary transition-colors hover:bg-surface"
          >
            <RefreshCw className="h-4 w-4" />
            刷新
          </button>
        }
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

      {isLoading && rows.length === 0 ? (
        <div className="flex items-center justify-center rounded-2xl border border-border py-16 text-text-secondary">
          <div className="mr-3 h-5 w-5 animate-spin rounded-full border-2 border-accent border-t-transparent" />
          正在读取流程任务...
        </div>
      ) : (aiError || publishError) && rows.length === 0 ? (
        <div className="flex min-h-[260px] flex-col items-center justify-center gap-3 rounded-2xl border border-border px-6 text-center">
          <AlertTriangle className="h-8 w-8 text-danger" />
          <div className="space-y-1">
            <p className="text-sm font-medium text-text-primary">OpenClaw 任务暂时无法加载</p>
            <p className="text-xs text-text-secondary">{initialErrorMessage}</p>
          </div>
          <button
            type="button"
            onClick={() => {
              void refetchPublishTasks();
              void refetchAIJobs();
            }}
            className="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-strong"
          >
            重试
          </button>
        </div>
      ) : filteredRows.length > 0 ? (
        <div className="overflow-hidden rounded-3xl border border-border bg-surface">
          {(aiRefetchError || publishRefetchError || loadMoreError) ? (
            <div className="border-b border-warning/20 bg-warning/10 px-5 py-3 text-xs text-warning">
              {loadMoreError || "任务列表刷新失败，已保留上次成功结果。"}
            </div>
          ) : null}
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b border-border bg-surface-hover/40 text-left text-xs uppercase tracking-wider text-text-muted">
                <tr>
                  <th className="px-5 py-4">任务标题</th>
                  <th className="px-5 py-4">类型</th>
                  <th className="px-5 py-4">节点 / 账号</th>
                  <th className="px-5 py-4">当前阶段</th>
                  <th className="px-5 py-4">最近进度时间</th>
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
                      {formatDateTime(row.workflowTime || row.updatedAt)}
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
          <div className="flex justify-end border-t border-border px-5 py-4">
            {hasMore ? (
              <button
                type="button"
                onClick={() => void handleLoadMore()}
                disabled={isLoadingMore}
                className="rounded-lg border border-border px-4 py-2 text-sm text-text-primary transition-colors hover:bg-surface-hover disabled:opacity-50"
              >
                {isLoadingMore ? "加载中..." : "加载更多"}
              </button>
            ) : (
              <p className="text-sm text-text-secondary">已加载全部结果</p>
            )}
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
