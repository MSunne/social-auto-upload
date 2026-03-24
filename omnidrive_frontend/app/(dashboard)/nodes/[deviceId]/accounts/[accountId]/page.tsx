"use client";

import { use, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  CalendarClock,
  Clock3,
  ListChecks,
  Loader2,
  Pencil,
  Plus,
  Sparkles,
  Trash2,
  UserRound,
} from "lucide-react";
import Link from "next/link";
import { AccountSkillRunModal } from "@/components/ui/account-skill-run-modal";
import {
  EmptyState,
  PageHeader,
  StatCard,
  StatusBadge,
} from "@/components/ui/common";
import {
  createAccountSkillRun,
  deleteTask,
  getAccountWorkspace,
  getDevice,
  listAIJobs,
  listSkills,
  listTasks,
  updateAIJob,
} from "@/lib/services";
import type {
  AIJob,
  AccountSkillScheduleSlot,
  Device,
  PlatformAccountWorkspace,
  Skill,
  Task,
} from "@/lib/types";
import {
  buildAIJobTitle,
  formatDateTime,
  resolveAIJobStage,
  shouldShowAIJobInWorkflow,
} from "@/lib/workflow";

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

type AccountSkillPlan = {
  job: AIJob;
  skill: Skill | null;
  publishAt?: string | null;
  generateAt?: string | null;
  schedule: AccountSkillScheduleSlot | null;
  status: string;
  stageLabel: string;
  stageDescription?: string;
  generationLeadMinutes: number;
  editable: boolean;
};

const DEFAULT_GENERATION_LEAD_MINUTES = 0;

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
  const publishAt =
    typeof payload.publishAt === "string" ? payload.publishAt : null;
  return publishAt || job.runAt || null;
}

function normalizeTimeOfDay(value?: string | null) {
  const trimmed = (value || "").trim();
  if (!trimmed) {
    return "";
  }
  return trimmed.length === 5 ? `${trimmed}:00` : trimmed;
}

function resolveScheduleTimezone(value?: string | null) {
  return (
    value ||
    Intl.DateTimeFormat().resolvedOptions().timeZone ||
    "Asia/Shanghai"
  ).trim();
}

function normalizeGenerationLeadMinutes(value?: number | null) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric < 0) {
    return DEFAULT_GENERATION_LEAD_MINUTES;
  }
  return Math.min(24 * 60, Math.round(numeric));
}

function inferGenerationLeadMinutes(
  publishAt?: string | null,
  generateAt?: string | null,
) {
  if (!publishAt || !generateAt) {
    return DEFAULT_GENERATION_LEAD_MINUTES;
  }
  const publishDate = new Date(publishAt);
  const generateDate = new Date(generateAt);
  if (
    Number.isNaN(publishDate.getTime()) ||
    Number.isNaN(generateDate.getTime())
  ) {
    return DEFAULT_GENERATION_LEAD_MINUTES;
  }
  return normalizeGenerationLeadMinutes(
    (publishDate.getTime() - generateDate.getTime()) / 60000,
  );
}

function formatGenerationLeadMinutes(minutes: number) {
  const normalized = normalizeGenerationLeadMinutes(minutes);
  if (normalized <= 0) {
    return "按发布时间生成";
  }
  return `提前 ${normalized} 分钟`;
}

function nextPublishAtFromTimeOfDay(timeOfDay: string) {
  const normalized = normalizeTimeOfDay(timeOfDay);
  if (!normalized) {
    throw new Error("时间不能为空");
  }
  const parts = normalized.split(":").map((item) => Number(item));
  const [hours, minutes, seconds = 0] = parts;
  if ([hours, minutes, seconds].some((item) => Number.isNaN(item))) {
    throw new Error("时间格式无效");
  }
  const next = new Date();
  next.setHours(hours, minutes, seconds, 0);
  if (next.getTime() <= Date.now()) {
    next.setDate(next.getDate() + 1);
  }
  return next.toISOString();
}

function getAIJobScheduleConfig(job: AIJob): AccountSkillScheduleSlot | null {
  const payload = (job.inputPayload || {}) as Record<string, unknown>;
  const publishAt = getAIJobPublishAt(job);
  const generateAt = getAIJobGenerateAt(job);
  const scheduleRaw =
    payload.scheduleConfig && typeof payload.scheduleConfig === "object"
      ? (payload.scheduleConfig as Record<string, unknown>)
      : null;
  if (scheduleRaw) {
    return {
      scheduleKey:
        typeof scheduleRaw.scheduleKey === "string"
          ? scheduleRaw.scheduleKey
          : undefined,
      timeOfDay: normalizeTimeOfDay(
        typeof scheduleRaw.timeOfDay === "string" ? scheduleRaw.timeOfDay : "",
      ),
      repeatDaily: Boolean(scheduleRaw.repeatDaily),
      timezone: resolveScheduleTimezone(
        typeof scheduleRaw.timezone === "string" ? scheduleRaw.timezone : null,
      ),
      generationLeadMinutes:
        typeof scheduleRaw.generationLeadMinutes === "number"
          ? normalizeGenerationLeadMinutes(scheduleRaw.generationLeadMinutes)
          : inferGenerationLeadMinutes(publishAt, generateAt),
    };
  }
  return publishAt
    ? {
        scheduleKey: job.id,
        timeOfDay: normalizeTimeOfDay(
          new Intl.DateTimeFormat("zh-CN", {
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
            hour12: false,
          }).format(new Date(publishAt)),
        ),
        repeatDaily: false,
        timezone: resolveScheduleTimezone(null),
        generationLeadMinutes: inferGenerationLeadMinutes(
          publishAt,
          generateAt,
        ),
      }
    : null;
}

function formatScheduleRule(schedule: AccountSkillScheduleSlot | null) {
  if (!schedule || !schedule.timeOfDay) {
    return "未设置";
  }
  return schedule.repeatDaily
    ? `每天 ${schedule.timeOfDay}`
    : `单次 ${schedule.timeOfDay}`;
}

function getAIJobGenerateAt(job: AIJob) {
  return job.runAt || null;
}

function isEditableAccountSkillRun(job: AIJob) {
  return (
    job.source === "account_skill_binding" &&
    (job.status === "scheduled" || job.status === "queued") &&
    !job.localPublishTaskId
  );
}

function buildUpdatedAccountSkillRun(
  job: AIJob,
  schedule: AccountSkillScheduleSlot,
) {
  const publishAt = nextPublishAtFromTimeOfDay(schedule.timeOfDay);
  const publishDate = new Date(publishAt);
  const generationLeadMinutes = normalizeGenerationLeadMinutes(
    schedule.generationLeadMinutes,
  );
  const generateAt = new Date(
    publishDate.getTime() - generationLeadMinutes * 60 * 1000,
  ).toISOString();
  const inputPayload = JSON.parse(
    JSON.stringify(job.inputPayload || {}),
  ) as Record<string, unknown>;
  const currentPublishPayload =
    inputPayload.publishPayload &&
    typeof inputPayload.publishPayload === "object"
      ? (inputPayload.publishPayload as Record<string, unknown>)
      : {};
  const currentSchedule =
    inputPayload.scheduleConfig &&
    typeof inputPayload.scheduleConfig === "object"
      ? (inputPayload.scheduleConfig as Record<string, unknown>)
      : {};

  inputPayload.publishAt = publishAt;
  inputPayload.runAt = generateAt;
  inputPayload.publishPayload = {
    ...currentPublishPayload,
    runAt: publishAt,
    requestedRun: publishAt,
  };
  inputPayload.scheduleConfig = {
    ...currentSchedule,
    scheduleKey: schedule.scheduleKey || currentSchedule.scheduleKey || job.id,
    timeOfDay: normalizeTimeOfDay(schedule.timeOfDay),
    repeatDaily: schedule.repeatDaily,
    timezone: resolveScheduleTimezone(schedule.timezone),
    generationLeadMinutes,
  };

  const nextStatus =
    new Date(generateAt).getTime() > Date.now() ? "scheduled" : "queued";
  const nextMessage =
    nextStatus === "scheduled" ? "等待定时生成" : "等待云端生成";

  return {
    inputPayload,
    runAt: generateAt,
    status: nextStatus,
    message: nextMessage,
  };
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
  const [editingJob, setEditingJob] = useState<AIJob | null>(null);

  const { data: device } = useQuery<Device>({
    queryKey: ["device", deviceId],
    queryFn: () => getDevice(deviceId),
  });

  const { data: workspace, isLoading: workspaceLoading } =
    useQuery<PlatformAccountWorkspace>({
      queryKey: ["accountWorkspace", accountId],
      queryFn: () => getAccountWorkspace(accountId),
    });

  const { data: tasks = [], isLoading: tasksLoading } = useQuery<Task[]>({
    queryKey: ["tasks", "account", accountId],
    queryFn: () => listTasks({ deviceId, accountId, limit: 100 }),
  });

  const { data: skillRuns = [], isLoading: aiLoading } = useQuery<AIJob[]>({
    queryKey: ["aiJobs", "account", accountId],
    queryFn: () =>
      listAIJobs({
        deviceId,
        accountId,
        limit: 100,
        excludeSource: "omnidrive_chat",
      }),
  });

  const { data: skills = [] } = useQuery<Skill[]>({
    queryKey: ["skills", deviceId],
    queryFn: () => listSkills(deviceId),
  });

  const createMutation = useMutation({
    mutationFn: async (payload: {
      skillId: string;
      scheduleSlots: AccountSkillScheduleSlot[];
    }) => createAccountSkillRun(accountId, payload),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ["aiJobs", "account", accountId],
        }),
        queryClient.invalidateQueries({
          queryKey: ["tasks", "account", accountId],
        }),
        queryClient.invalidateQueries({
          queryKey: ["accountWorkspace", accountId],
        }),
      ]);
      setIsCreateOpen(false);
    },
    onError: (error) => {
      window.alert(
        error instanceof Error ? error.message : "创建账号任务失败，请稍后重试",
      );
    },
  });

  const updateMutation = useMutation({
    mutationFn: async (payload: {
      job: AIJob;
      schedule: AccountSkillScheduleSlot;
    }) => {
      const nextPayload = buildUpdatedAccountSkillRun(
        payload.job,
        payload.schedule,
      );
      return updateAIJob(payload.job.id, nextPayload);
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ["aiJobs", "account", accountId],
        }),
        queryClient.invalidateQueries({
          queryKey: ["tasks", "account", accountId],
        }),
        queryClient.invalidateQueries({
          queryKey: ["accountWorkspace", accountId],
        }),
      ]);
      setEditingJob(null);
      setIsCreateOpen(false);
    },
    onError: (error) => {
      window.alert(
        error instanceof Error ? error.message : "修改发布时间失败，请稍后重试",
      );
    },
  });

  const deleteTaskMutation = useMutation({
    mutationFn: async (taskId: string) => deleteTask(taskId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["tasks"] }),
        queryClient.invalidateQueries({
          queryKey: ["tasks", "account", accountId],
        }),
        queryClient.invalidateQueries({
          queryKey: ["accountWorkspace", accountId],
        }),
        queryClient.invalidateQueries({ queryKey: ["accounts", deviceId] }),
        queryClient.invalidateQueries({ queryKey: ["device", deviceId] }),
      ]);
    },
    onError: (error) => {
      window.alert(
        error instanceof Error ? error.message : "删除任务失败，请稍后重试",
      );
    },
  });

  const skillMap = useMemo(
    () => new Map(skills.map((item) => [item.id, item])),
    [skills],
  );

  const timelineItems = useMemo(() => {
    const aiItems: TimelineItem[] = skillRuns
      .filter(shouldShowAIJobInWorkflow)
      .map((job) => ({
        id: job.id,
        kind: "ai_job",
        title: buildAIJobTitle(job),
        subtitle: job.skillId
          ? `技能 ${skillMap.get(job.skillId)?.name || job.skillId}`
          : job.modelName,
        status: toTimelineStatus(job, "ai_job"),
        scheduledAt: getAIJobPublishAt(job),
        updatedAt: job.updatedAt,
        href: `/tasks/ai/${job.id}`,
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
      const leftDate =
        parseISOTime(left.scheduledAt) ||
        parseISOTime(left.updatedAt) ||
        new Date(0);
      const rightDate =
        parseISOTime(right.scheduledAt) ||
        parseISOTime(right.updatedAt) ||
        new Date(0);
      return leftDate.getTime() - rightDate.getTime();
    });
  }, [skillMap, skillRuns, tasks]);

  const accountSkillPlans = useMemo<AccountSkillPlan[]>(() => {
    return skillRuns
      .filter(
        (job) =>
          job.source === "account_skill_binding" &&
          ["scheduled", "queued", "running"].includes(job.status),
      )
      .map((job) => {
        const stage = resolveAIJobStage(job);
        return {
          job,
          skill: job.skillId ? skillMap.get(job.skillId) || null : null,
          publishAt: getAIJobPublishAt(job),
          generateAt: getAIJobGenerateAt(job),
          schedule: getAIJobScheduleConfig(job),
          status: toTimelineStatus(job, "ai_job"),
          stageLabel: stage.label,
          stageDescription: stage.description,
          generationLeadMinutes: normalizeGenerationLeadMinutes(
            getAIJobScheduleConfig(job)?.generationLeadMinutes,
          ),
          editable: isEditableAccountSkillRun(job),
        };
      })
      .sort((left, right) => {
        const leftDate =
          parseISOTime(left.publishAt) ||
          parseISOTime(left.generateAt) ||
          parseISOTime(left.job.updatedAt) ||
          new Date(0);
        const rightDate =
          parseISOTime(right.publishAt) ||
          parseISOTime(right.generateAt) ||
          parseISOTime(right.job.updatedAt) ||
          new Date(0);
        return leftDate.getTime() - rightDate.getTime();
      });
  }, [skillMap, skillRuns]);

  const account = workspace?.account;
  const enabledSkills = useMemo(
    () => skills.filter((item) => item.isEnabled),
    [skills],
  );
  const isSkillRunModalOpen = isCreateOpen || Boolean(editingJob);

  const handleDeleteTask = async (taskId: string, title: string) => {
    const confirmed = window.confirm(
      `确认删除任务“${title}”吗？删除后无法恢复。`,
    );
    if (!confirmed) {
      return;
    }
    await deleteTaskMutation.mutateAsync(taskId);
  };

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
          value={
            skillRuns.filter(
              (item) =>
                item.status === "scheduled" ||
                item.status === "queued" ||
                item.status === "running",
            ).length
          }
          icon={<Sparkles className="h-5 w-5" />}
        />
        <StatCard
          label="待发布任务"
          value={
            tasks.filter(
              (item) =>
                item.status === "scheduled" ||
                item.status === "pending" ||
                item.status === "running",
            ).length
          }
          icon={<CalendarClock className="h-5 w-5" />}
        />
      </div>

      <div className="mb-6 overflow-hidden rounded-3xl border border-border bg-surface">
        <div className="border-b border-border px-6 py-5">
          <h2 className="text-lg font-semibold text-text-primary">
            账号技能计划
          </h2>
          <p className="mt-1 text-sm text-text-secondary">
            同一条技能可以被多个账号复用，发布时间在这里为当前账号单独安排和修改。
          </p>
        </div>

        {accountSkillPlans.length === 0 ? (
          <div className="px-6 py-10">
            <EmptyState
              icon={<Sparkles className="h-6 w-6" />}
              title="还没有账号专属技能计划"
              description={
                enabledSkills.length > 0
                  ? "先为当前账号创建一条技能任务，再按发布时间进入生成和发布链路。"
                  : "当前没有可用技能，请先去技能中心创建并启用技能。"
              }
              action={
                enabledSkills.length > 0 ? (
                  <button
                    type="button"
                    onClick={() => setIsCreateOpen(true)}
                    className="rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2 text-sm font-semibold text-background"
                  >
                    新建账号计划
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
          <div className="grid gap-4 p-5">
            {accountSkillPlans.map((plan) => (
              <div
                key={plan.job.id}
                className="group overflow-hidden rounded-2xl border border-border bg-surface transition-all hover:border-accent/30 hover:shadow-lg hover:shadow-accent/5"
              >
                <div className="flex">
                  <div className="w-1 shrink-0 bg-gradient-to-b from-accent via-pink to-cyan" />
                  <div className="flex flex-1 flex-col gap-4 p-5 xl:flex-row xl:items-start xl:justify-between">
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <StatusBadge status={plan.status} />
                        <span className="rounded-lg bg-white/6 px-2 py-0.5 text-[11px] font-medium text-text-muted">
                          {formatScheduleRule(plan.schedule)}
                        </span>
                      </div>
                      <p className="mt-2.5 text-base font-semibold text-text-primary">
                        {plan.skill?.name || buildAIJobTitle(plan.job)}
                      </p>
                      <p className="mt-1 line-clamp-2 text-sm leading-relaxed text-text-secondary">
                        {plan.skill?.description || "这条计划先生成内容，再进入发布链路。"}
                      </p>

                      <div className="mt-3 flex flex-wrap gap-2">
                        <span className="inline-flex items-center gap-1.5 rounded-lg border border-border/60 bg-surface-hover/50 px-2.5 py-1 text-xs text-text-secondary">
                          <CalendarClock className="h-3 w-3 text-cyan" />
                          发布 <span className="font-medium text-text-primary">{plan.publishAt ? formatDateTime(plan.publishAt) : "未设置"}</span>
                        </span>
                        <span className="inline-flex items-center gap-1.5 rounded-lg border border-border/60 bg-surface-hover/50 px-2.5 py-1 text-xs text-text-secondary">
                          <Sparkles className="h-3 w-3 text-accent" />
                          生成 <span className="font-medium text-text-primary">{plan.generateAt ? formatDateTime(plan.generateAt) : "未设置"}</span>
                        </span>
                        <span className="inline-flex items-center gap-1.5 rounded-lg border border-border/60 bg-surface-hover/50 px-2.5 py-1 text-xs text-text-secondary">
                          提前 <span className="font-medium text-text-primary">{formatGenerationLeadMinutes(plan.generationLeadMinutes)}</span>
                        </span>
                      </div>
                    </div>

                    <div className="flex shrink-0 flex-col items-start gap-2 xl:items-end">
                      <span className="rounded-lg bg-white/6 px-2 py-1 text-[11px] text-text-muted">
                        {plan.stageLabel}{plan.stageDescription ? ` · ${plan.stageDescription}` : ""}
                      </span>
                      <span className="text-[11px] text-text-muted">
                        更新 {formatDateTime(plan.job.updatedAt)}
                      </span>
                      {plan.editable ? (
                        <button
                          type="button"
                          onClick={() => setEditingJob(plan.job)}
                          className="mt-1 inline-flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-accent/15 to-cyan/10 border border-accent/25 px-3.5 py-1.5 text-xs font-semibold text-accent transition-all hover:from-accent/25 hover:to-cyan/15 hover:shadow-md hover:shadow-accent/10"
                        >
                          <Pencil className="h-3 w-3" />
                          修改时间
                        </button>
                      ) : (
                        <span className="mt-1 text-[11px] text-text-muted italic">不可修改</span>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="overflow-hidden rounded-3xl border border-border bg-surface">
        <div className="border-b border-border px-6 py-5">
          <h2 className="text-lg font-semibold text-text-primary">
            账号任务时间线
          </h2>
          <p className="mt-1 text-sm text-text-secondary">
            按执行或发布时间从近到远排序，先看计划，再看状态。
          </p>
        </div>

        {timelineItems.length === 0 ? (
          <div className="px-6 py-10">
            <EmptyState
              icon={<Clock3 className="h-6 w-6" />}
              title="当前账号还没有任务"
              description={
                enabledSkills.length > 0
                  ? "先从一个技能创建账号专属任务，它会先生成，再进入发布链路。"
                  : "当前没有可用技能，请先去技能中心创建并启用技能。"
              }
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
          <div className="grid gap-4 p-5">
            {timelineItems.map((item) => (
              <div
                key={`${item.kind}-${item.id}`}
                className="group overflow-hidden rounded-2xl border border-border bg-surface transition-all hover:border-accent/30 hover:shadow-lg hover:shadow-accent/5"
              >
                <div className="flex">
                  <div className={`w-1 shrink-0 ${item.kind === "ai_job" ? "bg-gradient-to-b from-accent to-pink" : "bg-gradient-to-b from-cyan to-emerald-400"}`} />
                  <div className="flex flex-1 flex-col gap-4 p-5 md:flex-row md:items-start md:justify-between">
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className={`rounded-lg px-2 py-0.5 text-[11px] font-semibold ${item.kind === "ai_job" ? "bg-accent/12 text-accent" : "bg-cyan/12 text-cyan"}`}>
                          {item.label}
                        </span>
                        <StatusBadge status={item.status} />
                      </div>
                      <p className="mt-2 text-sm font-semibold text-text-primary">
                        {item.title}
                      </p>
                      <p className="mt-0.5 text-xs text-text-secondary">
                        {item.subtitle}
                      </p>
                      <div className="mt-2.5 flex flex-wrap items-center gap-2">
                        <span className="inline-flex items-center gap-1.5 rounded-lg border border-border/60 bg-surface-hover/50 px-2.5 py-1 text-[11px] text-text-secondary">
                          <CalendarClock className="h-3 w-3 text-cyan" />
                          {item.scheduledAt ? formatDateTime(item.scheduledAt) : "未设置"}
                        </span>
                        <span className="text-[11px] text-text-muted">
                          更新 {formatDateTime(item.updatedAt)}
                        </span>
                      </div>
                    </div>

                    {item.href ? (
                      <div className="flex shrink-0 flex-wrap items-center gap-2">
                        <Link
                          href={item.href}
                          className="inline-flex items-center gap-1.5 rounded-xl border border-border bg-surface-hover px-3 py-1.5 text-xs font-medium text-text-primary transition-all hover:border-accent hover:text-accent hover:shadow-sm"
                        >
                          查看详情
                        </Link>
                        {item.kind === "publish_task" ? (
                          <button
                            type="button"
                            onClick={() => handleDeleteTask(item.id, item.title)}
                            disabled={deleteTaskMutation.isPending}
                            className="inline-flex items-center gap-1.5 rounded-xl border border-red-500/25 bg-red-500/8 px-3 py-1.5 text-xs font-medium text-red-300 transition-all hover:border-red-400/50 hover:bg-red-500/14 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            {deleteTaskMutation.isPending &&
                            deleteTaskMutation.variables === item.id ? (
                              <Loader2 className="h-3 w-3 animate-spin" />
                            ) : (
                              <Trash2 className="h-3 w-3" />
                            )}
                            删除
                          </button>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <AccountSkillRunModal
        key={editingJob?.id || (isCreateOpen ? "create" : "closed")}
        isOpen={isSkillRunModalOpen}
        accountName={account.accountName}
        job={editingJob}
        skills={skills}
        submitting={createMutation.isPending || updateMutation.isPending}
        onClose={() => {
          setIsCreateOpen(false);
          setEditingJob(null);
        }}
        onSubmit={async (payload) => {
          if (editingJob) {
            await updateMutation.mutateAsync({
              job: editingJob,
              schedule: payload.scheduleSlots[0],
            });
            return;
          }
          await createMutation.mutateAsync(payload);
        }}
      />
    </>
  );
}
