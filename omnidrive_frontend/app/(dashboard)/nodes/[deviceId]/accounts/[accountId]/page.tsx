"use client";

import { use, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  CalendarClock,
  Clock3,
  ListChecks,
  Plus,
  Sparkles,
  UserRound,
} from "lucide-react";
import Link from "next/link";
import { AccountSkillRunModal } from "@/components/ui/account-skill-run-modal";
import { EmptyState, PageHeader, StatCard, StatusBadge } from "@/components/ui/common";
import {
  createAccountSkillRun,
  getAccountWorkspace,
  getDevice,
  listAIJobs,
  listSkills,
  listTasks,
} from "@/lib/services";
import type { AIJob, Device, PlatformAccountWorkspace, Skill, Task } from "@/lib/types";
import { buildAIJobTitle, formatDateTime, shouldShowAIJobInWorkflow } from "@/lib/workflow";

type TimelineItem = {
  id: string;
  kind: "ai_job" | "publish_task";
  title: string;
  subtitle: string;
  status: string;
  scheduledAt?: string | null;
  updatedAt: string;
  href?: string;
  label: string;
};

function parseISOTime(value?: string | null) {
  if (!value) {
    return null;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return null;
  }
  return date;
}

function getAIJobPublishAt(job: AIJob) {
  const payload = (job.inputPayload || {}) as Record<string, unknown>;
  const publishAt = typeof payload.publishAt === "string" ? payload.publishAt : null;
  return publishAt || job.runAt || null;
}

function toTimelineStatus(job: AIJob | Task, kind: "ai_job" | "publish_task") {
  if (kind === "publish_task") {
    return job.status;
  }
  switch (job.status) {
    case "queued":
      return "queued_generation";
    case "scheduled":
      return "scheduled";
    case "running":
      return "generating";
    case "success":
      return "output_ready";
    default:
      return job.status;
  }
}

export default function AccountTaskPage({
  params,
}: {
  params: Promise<{ deviceId: string; accountId: string }>;
}) {
  const { deviceId, accountId } = use(params);
  const queryClient = useQueryClient();
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  const { data: device } = useQuery<Device>({
    queryKey: ["device", deviceId],
    queryFn: () => getDevice(deviceId),
  });

  const { data: workspace, isLoading: workspaceLoading } = useQuery<PlatformAccountWorkspace>({
    queryKey: ["accountWorkspace", accountId],
    queryFn: () => getAccountWorkspace(accountId),
  });

  const { data: tasks = [], isLoading: tasksLoading } = useQuery<Task[]>({
    queryKey: ["tasks", "account", accountId],
    queryFn: () => listTasks({ deviceId, accountId, limit: 100 }),
  });

  const { data: skillRuns = [], isLoading: aiLoading } = useQuery<AIJob[]>({
    queryKey: ["aiJobs", "account", accountId],
    queryFn: () => listAIJobs({ deviceId, accountId, limit: 100, excludeSource: "omnidrive_chat" }),
  });

  const { data: skills = [] } = useQuery<Skill[]>({
    queryKey: ["skills", deviceId],
    queryFn: () => listSkills(deviceId),
  });

  const createMutation = useMutation({
    mutationFn: async (payload: { skillId: string; publishAt: string }) =>
      createAccountSkillRun(accountId, payload),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["aiJobs", "account", accountId] }),
        queryClient.invalidateQueries({ queryKey: ["tasks", "account", accountId] }),
        queryClient.invalidateQueries({ queryKey: ["accountWorkspace", accountId] }),
      ]);
      setIsCreateOpen(false);
    },
    onError: (error) => {
      window.alert(error instanceof Error ? error.message : "创建账号任务失败，请稍后重试");
    },
  });

  const timelineItems = useMemo(() => {
    const aiItems: TimelineItem[] = skillRuns.filter(shouldShowAIJobInWorkflow).map((job) => ({
      id: job.id,
      kind: "ai_job",
      title: buildAIJobTitle(job),
      subtitle: job.skillId ? `技能 ${job.skillId}` : job.modelName,
      status: toTimelineStatus(job, "ai_job"),
      scheduledAt: getAIJobPublishAt(job),
      updatedAt: job.updatedAt,
      label: "技能生成",
    }));
    const publishItems: TimelineItem[] = tasks.map((task) => ({
      id: task.id,
      kind: "publish_task",
      title: task.title,
      subtitle: task.accountName,
      status: toTimelineStatus(task, "publish_task"),
      scheduledAt: task.runAt,
      updatedAt: task.updatedAt,
      href: `/tasks/${task.id}`,
      label: "发布任务",
    }));

    return [...aiItems, ...publishItems].sort((left, right) => {
      const leftDate = parseISOTime(left.scheduledAt) || parseISOTime(left.updatedAt) || new Date(0);
      const rightDate = parseISOTime(right.scheduledAt) || parseISOTime(right.updatedAt) || new Date(0);
      return leftDate.getTime() - rightDate.getTime();
    });
  }, [skillRuns, tasks]);

  const account = workspace?.account;
  const enabledSkills = useMemo(() => skills.filter((item) => item.isEnabled), [skills]);

  if (workspaceLoading || tasksLoading || aiLoading) {
    return (
      <div className="flex h-72 items-center justify-center">
        <div className="flex items-center gap-3 text-text-secondary">
          <div className="h-5 w-5 animate-spin rounded-full border-2 border-accent border-t-transparent" />
          正在读取账号任务信息...
        </div>
      </div>
    );
  }

  if (!account) {
    return (
      <EmptyState
        icon={<UserRound className="h-6 w-6" />}
        title="账号不存在"
        description="这个 OmniBull 账号可能已经解绑，或者你当前没有访问权限。"
      />
    );
  }

  return (
    <>
      <div className="mb-4">
        <Link
          href={`/nodes/${deviceId}/accounts`}
          className="inline-flex items-center gap-2 rounded-xl border border-border bg-surface px-4 py-2 text-sm font-medium text-text-primary transition-colors hover:border-accent hover:text-accent"
        >
          <ArrowLeft className="h-4 w-4" />
          返回账号列表
        </Link>
      </div>

      <PageHeader
        title={`${account.accountName} · 任务列表`}
        subtitle={`${device?.name || "当前节点"} / ${account.platform}。这里按执行时间展示该账号的生成与发布链路。`}
        actions={
          <button
            type="button"
            onClick={() => setIsCreateOpen(true)}
            className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2 text-sm font-semibold text-background"
          >
            <Plus className="h-4 w-4" />
            新增任务
          </button>
        }
      />

      <div className="mb-6 grid gap-4 md:grid-cols-3">
        <StatCard
          label="账号任务总数"
          value={timelineItems.length}
          icon={<ListChecks className="h-5 w-5" />}
        />
        <StatCard
          label="待生成技能任务"
          value={skillRuns.filter((item) => item.status === "scheduled" || item.status === "queued" || item.status === "running").length}
          icon={<Sparkles className="h-5 w-5" />}
        />
        <StatCard
          label="待发布任务"
          value={tasks.filter((item) => item.status === "scheduled" || item.status === "pending" || item.status === "running").length}
          icon={<CalendarClock className="h-5 w-5" />}
        />
      </div>

      <div className="overflow-hidden rounded-3xl border border-border bg-surface">
        <div className="border-b border-border px-6 py-5">
          <h2 className="text-lg font-semibold text-text-primary">账号任务时间线</h2>
          <p className="mt-1 text-sm text-text-secondary">按执行或发布时间从近到远排序，先看计划，再看状态。</p>
        </div>

        {timelineItems.length === 0 ? (
          <div className="px-6 py-10">
            <EmptyState
              icon={<Clock3 className="h-6 w-6" />}
              title="当前账号还没有任务"
              description={enabledSkills.length > 0 ? "先从一个技能创建账号专属任务，它会先生成，再进入发布链路。" : "当前没有可用技能，请先去技能中心创建并启用技能。"}
              action={
                enabledSkills.length > 0 ? (
                  <button
                    type="button"
                    onClick={() => setIsCreateOpen(true)}
                    className="rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2 text-sm font-semibold text-background"
                  >
                    创建第一条任务
                  </button>
                ) : (
                  <Link
                    href={`/nodes/${deviceId}`}
                    className="rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2 text-sm font-semibold text-background"
                  >
                    去配置技能
                  </Link>
                )
              }
            />
          </div>
        ) : (
          <div className="divide-y divide-border">
            {timelineItems.map((item) => (
              <div key={`${item.kind}-${item.id}`} className="flex flex-col gap-4 px-6 py-5 md:flex-row md:items-center md:justify-between">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="rounded-full bg-white/6 px-2.5 py-1 text-xs font-medium text-text-secondary">
                      {item.label}
                    </span>
                    <StatusBadge status={item.status} />
                  </div>
                  <p className="mt-3 text-base font-semibold text-text-primary">{item.title}</p>
                  <p className="mt-1 text-sm text-text-secondary">{item.subtitle}</p>
                </div>

                <div className="flex flex-col items-start gap-3 md:items-end">
                  <div className="text-sm text-text-secondary">
                    <p>计划时间：<span className="font-medium text-text-primary">{item.scheduledAt ? formatDateTime(item.scheduledAt) : "未设置"}</span></p>
                    <p className="mt-1">最近更新：{formatDateTime(item.updatedAt)}</p>
                  </div>
                  {item.href ? (
                    <Link
                      href={item.href}
                      className="inline-flex items-center gap-2 rounded-full border border-border bg-surface-hover px-3 py-1.5 text-xs font-medium text-text-primary transition-colors hover:border-accent hover:text-accent"
                    >
                      查看详情
                    </Link>
                  ) : null}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <AccountSkillRunModal
        isOpen={isCreateOpen}
        accountName={account.accountName}
        skills={enabledSkills}
        submitting={createMutation.isPending}
        onClose={() => setIsCreateOpen(false)}
        onSubmit={async (payload) => {
          await createMutation.mutateAsync(payload);
        }}
      />
    </>
  );
}
