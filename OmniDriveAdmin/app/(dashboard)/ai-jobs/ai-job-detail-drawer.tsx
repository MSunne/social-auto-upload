"use client";

import { useMemo } from "react";
import {
  AlertTriangle,
  Clock3,
  Cpu,
  ExternalLink,
  FileJson,
  Loader2,
  Receipt,
  RefreshCw,
  Send,
  Sparkles,
  X,
} from "lucide-react";
import { useAdminAIJobWorkspace } from "@/lib/hooks/useAdminAIJobs";
import type {
  AIJobArtifact,
  AdminAIJobWorkspace,
  AdminExecutionLog,
  BillingUsageEvent,
  PublishTask,
} from "@/lib/types";

interface AIJobDetailDrawerProps {
  jobId: string | null;
  onClose: () => void;
}

const STATUS_CLASS: Record<string, string> = {
  queued: "border-yellow-500/25 bg-yellow-500/10 text-yellow-300",
  running: "border-blue-500/25 bg-blue-500/10 text-blue-300",
  completed: "border-emerald-500/25 bg-emerald-500/10 text-emerald-300",
  success: "border-emerald-500/25 bg-emerald-500/10 text-emerald-300",
  failed: "border-red-500/25 bg-red-500/10 text-red-300",
  cancelled: "border-white/10 bg-white/5 text-[var(--color-text-secondary)]",
  scheduled: "border-cyan-500/25 bg-cyan-500/10 text-cyan-300",
  created: "border-white/10 bg-white/5 text-[var(--color-text-primary)]",
  stored: "border-purple-500/25 bg-purple-500/10 text-purple-300",
};

const STATUS_LABEL: Record<string, string> = {
  queued: "排队中",
  running: "处理中",
  completed: "已完成",
  success: "已完成",
  failed: "失败",
  cancelled: "已取消",
  scheduled: "已计划",
  created: "已创建",
  stored: "已落库",
};

const STAGE_LABEL: Record<string, string> = {
  job: "作业",
  schedule: "调度",
  generation: "生成",
  artifact: "产物",
  billing: "计费",
  publish: "发布",
  audit: "审计",
};

function formatDateTime(value?: string | null) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function formatFullDateTime(value?: string | null) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN");
}

function stringifyPayload(value: unknown) {
  if (value == null) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function pickArtifactLabel(artifact: AIJobArtifact) {
  return artifact.fileName || artifact.title || artifact.artifactType || artifact.artifactKey;
}

function pickLogIcon(stage: string) {
  switch (stage) {
    case "generation":
      return <Sparkles className="h-4 w-4 text-blue-300" />;
    case "artifact":
      return <FileJson className="h-4 w-4 text-purple-300" />;
    case "billing":
      return <Receipt className="h-4 w-4 text-amber-300" />;
    case "publish":
      return <Send className="h-4 w-4 text-cyan-300" />;
    default:
      return <Clock3 className="h-4 w-4 text-[var(--color-text-secondary)]" />;
  }
}

export function AIJobDetailDrawer({ jobId, onClose }: AIJobDetailDrawerProps) {
  const { data, isLoading, error, refetch, isFetching } = useAdminAIJobWorkspace(jobId);

  const inputPayload = useMemo(
    () => stringifyPayload(data?.record.job.inputPayload),
    [data],
  );
  const outputPayload = useMemo(
    () => stringifyPayload(data?.record.job.outputPayload),
    [data],
  );

  if (!jobId) {
    return null;
  }

  return (
    <>
      <div className="fixed inset-0 z-40 bg-black/55 backdrop-blur-sm" onClick={onClose} />
      <div className="fixed inset-y-0 right-0 z-50 flex w-full justify-end">
        <div className="flex h-full w-full max-w-[1040px] flex-col border-l border-[var(--color-border)] bg-[var(--color-bg-primary)] shadow-2xl">
          <div className="flex items-start justify-between gap-4 border-b border-[var(--color-border)] bg-[var(--color-bg-primary)]/92 px-5 py-4 backdrop-blur">
            <div className="min-w-0">
              <p className="text-xs uppercase tracking-[0.24em] text-[var(--color-text-secondary)]">AI job workspace</p>
              <h2 className="mt-2 text-lg font-semibold text-[var(--color-text-primary)]">
                执行详情与真实日志
              </h2>
              <p className="mt-1 truncate font-mono text-xs text-[var(--color-text-secondary)]">{jobId}</p>
            </div>

            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => refetch()}
                className="inline-flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2 text-sm text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)]"
              >
                {isFetching ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                刷新
              </button>
              <button
                type="button"
                onClick={onClose}
                className="rounded-lg p-2 text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)]"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
          </div>

          <div className="flex-1 overflow-y-auto px-5 py-5">
            {isLoading ? (
              <div className="flex min-h-[360px] flex-col items-center justify-center text-[var(--color-text-secondary)]">
                <Loader2 className="h-8 w-8 animate-spin text-[var(--color-primary)]" />
                <p className="mt-4 text-sm">正在加载执行详情...</p>
              </div>
            ) : error ? (
              <div className="rounded-2xl border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-300">
                详情读取失败，请刷新后重试。
              </div>
            ) : data ? (
              <DrawerBody data={data} inputPayload={inputPayload} outputPayload={outputPayload} />
            ) : null}
          </div>
        </div>
      </div>
    </>
  );
}

function DrawerBody({
  data,
  inputPayload,
  outputPayload,
}: {
  data: AdminAIJobWorkspace;
  inputPayload: string;
  outputPayload: string;
}) {
  const job = data.record.job;
  const mainMessage = job.message || job.deliveryMessage;

  return (
    <div className="space-y-5">
      {mainMessage ? (
        <div className="flex items-start gap-3 rounded-2xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-red-200">
          <AlertTriangle className="mt-0.5 h-4 w-4 flex-shrink-0" />
          <div>
            <p className="font-medium">最近一次异常 / 返回信息</p>
            <p className="mt-1 break-all leading-6">{mainMessage}</p>
          </div>
        </div>
      ) : null}

      <div className="grid gap-5 xl:grid-cols-[minmax(0,1.4fr)_320px]">
        <div className="space-y-5">
          <CompactPanel
            title="执行时间线"
            subtitle="按时间顺序还原作业创建、调度、模型执行、产物落库、计费和发布衔接。"
          >
            <div className="space-y-3">
              {data.executionLogs.length === 0 ? (
                <EmptyState label="当前还没有可展示的执行日志。" />
              ) : (
                data.executionLogs.map((log) => <ExecutionLogItem key={log.id} log={log} />)
              )}
            </div>
          </CompactPanel>

          <CompactPanel
            title="原始负载"
            subtitle="保留输入与输出原文，方便直接排查参数错误、模型返回异常和桥接问题。"
          >
            <div className="grid gap-4 xl:grid-cols-2">
              <JsonBlock title="输入负载" value={inputPayload} />
              <JsonBlock title="输出负载" value={outputPayload} />
            </div>
          </CompactPanel>

          {data.recentAudits.length > 0 ? (
            <CompactPanel title="管理员操作" subtitle="这里展示后台对当前作业做过的人工干预与审计记录。">
              <div className="space-y-3">
                {data.recentAudits.map((audit) => (
                  <div
                    key={audit.id}
                    className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] px-4 py-3"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-[var(--color-text-primary)]">{audit.title}</p>
                        <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                          {audit.admin?.name || "系统"} · {audit.source} · {formatDateTime(audit.createdAt)}
                        </p>
                      </div>
                      <StatusPill status={audit.status} />
                    </div>
                    {audit.message ? (
                      <p className="mt-2 break-all text-sm leading-6 text-[var(--color-text-secondary)]">
                        {audit.message}
                      </p>
                    ) : null}
                  </div>
                ))}
              </div>
            </CompactPanel>
          ) : null}
        </div>

        <div className="space-y-5">
          <CompactPanel title="作业概览">
            <div className="space-y-3">
              <InfoRow label="当前状态" value={<StatusPill status={job.status} />} />
              <InfoRow label="模型" value={job.modelName || "—"} />
              <InfoRow label="类型" value={job.jobType || "—"} />
              <InfoRow label="来源" value={job.source || "—"} />
              <InfoRow label="投递状态" value={job.deliveryStatus || "—"} />
              <InfoRow label="消耗积分" value={job.costCredits.toLocaleString()} />
              <InfoRow label="所属用户" value={data.record.owner?.email || "—"} />
              <InfoRow label="执行设备" value={data.record.device?.name || "云端"} />
              <InfoRow label="关联技能" value={data.record.skill?.name || "—"} />
              <InfoRow label="创建时间" value={formatFullDateTime(job.createdAt)} />
              <InfoRow label="更新时间" value={formatFullDateTime(job.updatedAt)} />
              <InfoRow label="完成时间" value={formatFullDateTime(job.finishedAt)} />
            </div>
          </CompactPanel>

          <CompactPanel title="生成产物">
            <ArtifactList artifacts={data.artifacts} />
          </CompactPanel>

          <CompactPanel title="关联发布任务">
            <PublishTaskList tasks={data.publishTasks} />
          </CompactPanel>

          <CompactPanel title="计费记录">
            <BillingList items={data.billingUsageEvents} />
          </CompactPanel>
        </div>
      </div>
    </div>
  );
}

function CompactPanel({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)]/55 p-4">
      <div className="mb-4">
        <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">{title}</h3>
        {subtitle ? (
          <p className="mt-1 text-xs leading-6 text-[var(--color-text-secondary)]">{subtitle}</p>
        ) : null}
      </div>
      {children}
    </section>
  );
}

function StatusPill({ status }: { status: string }) {
  const tone = STATUS_CLASS[status] || "border-white/10 bg-white/5 text-[var(--color-text-secondary)]";
  return (
    <span className={`inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium ${tone}`}>
      {STATUS_LABEL[status] || status || "未知"}
    </span>
  );
}

function ExecutionLogItem({ log }: { log: AdminExecutionLog }) {
  const payload = stringifyPayload(log.payload);

  return (
    <div className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] px-4 py-3">
      <div className="flex items-start gap-3">
        <div className="mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
          {pickLogIcon(log.stage)}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-sm font-medium text-[var(--color-text-primary)]">{log.title}</p>
            <span className="text-xs text-[var(--color-text-secondary)]">{STAGE_LABEL[log.stage] || log.stage}</span>
            <StatusPill status={log.status} />
          </div>
          <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
            {formatDateTime(log.timestamp)} · {log.source || "system"}
          </p>
          {log.message ? (
            <p className="mt-2 break-all text-sm leading-6 text-[var(--color-text-secondary)]">{log.message}</p>
          ) : null}
          {payload ? (
            <pre className="mt-3 overflow-x-auto rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-3 text-xs leading-6 text-[var(--color-text-secondary)]">
              {payload}
            </pre>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function ArtifactList({ artifacts }: { artifacts: AIJobArtifact[] }) {
  if (artifacts.length === 0) {
    return <EmptyState label="还没有生成产物。" />;
  }

  return (
    <div className="space-y-3">
      {artifacts.map((artifact) => (
        <div
          key={artifact.id}
          className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] px-4 py-3"
        >
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-[var(--color-text-primary)]">{pickArtifactLabel(artifact)}</p>
              <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                {artifact.artifactType} · {formatDateTime(artifact.createdAt)}
              </p>
            </div>
            {artifact.publicUrl ? (
              <a
                href={artifact.publicUrl}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 rounded-lg border border-[var(--color-border)] px-2.5 py-1.5 text-xs text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-primary)]"
              >
                查看
                <ExternalLink className="h-3 w-3" />
              </a>
            ) : null}
          </div>
          {artifact.textContent ? (
            <p className="mt-2 line-clamp-4 text-sm leading-6 text-[var(--color-text-secondary)]">
              {artifact.textContent}
            </p>
          ) : null}
        </div>
      ))}
    </div>
  );
}

function PublishTaskList({ tasks }: { tasks: PublishTask[] }) {
  if (tasks.length === 0) {
    return <EmptyState label="还没有挂接到发布任务。" />;
  }

  return (
    <div className="space-y-3">
      {tasks.map((task) => (
        <div
          key={task.id}
          className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] px-4 py-3"
        >
          <div className="flex items-center justify-between gap-3">
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-[var(--color-text-primary)]">{task.title || "未命名任务"}</p>
              <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                {task.platform} · {task.accountName} · {formatDateTime(task.createdAt)}
              </p>
            </div>
            <StatusPill status={task.status} />
          </div>
          {task.message ? (
            <p className="mt-2 break-all text-sm leading-6 text-[var(--color-text-secondary)]">{task.message}</p>
          ) : null}
        </div>
      ))}
    </div>
  );
}

function BillingList({ items }: { items: BillingUsageEvent[] }) {
  if (items.length === 0) {
    return <EmptyState label="当前作业还没有计费事件。" />;
  }

  return (
    <div className="space-y-3">
      {items.map((item) => (
        <div
          key={item.id}
          className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] px-4 py-3"
        >
          <div className="flex items-center justify-between gap-3">
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-[var(--color-text-primary)]">
                {item.meterName || item.meterCode}
              </p>
              <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                数量 {item.usageQuantity} · {formatDateTime(item.createdAt)}
              </p>
            </div>
            <StatusPill status={item.billStatus} />
          </div>
          {item.billMessage ? (
            <p className="mt-2 break-all text-sm leading-6 text-[var(--color-text-secondary)]">{item.billMessage}</p>
          ) : null}
        </div>
      ))}
    </div>
  );
}

function JsonBlock({ title, value }: { title: string; value: string }) {
  return (
    <div className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-panel-muted)]">
      <div className="flex items-center gap-2 border-b border-[var(--color-border)] px-4 py-3 text-sm font-medium text-[var(--color-text-primary)]">
        <FileJson className="h-4 w-4 text-[var(--color-text-secondary)]" />
        {title}
      </div>
      <div className="px-4 py-3">
        {value ? (
          <pre className="max-h-[420px] overflow-auto rounded-xl bg-[var(--color-bg-primary)] p-3 text-xs leading-6 text-[var(--color-text-secondary)]">
            {value}
          </pre>
        ) : (
          <EmptyState label="没有可展示的内容。" />
        )}
      </div>
    </div>
  );
}

function InfoRow({
  label,
  value,
}: {
  label: string;
  value: React.ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] px-3 py-2.5">
      <span className="text-xs text-[var(--color-text-secondary)]">{label}</span>
      <div className="text-right text-sm text-[var(--color-text-primary)]">{value}</div>
    </div>
  );
}

function EmptyState({ label }: { label: string }) {
  return (
    <div className="rounded-xl border border-dashed border-[var(--color-border)] bg-[var(--color-panel-muted)] px-4 py-6 text-center text-sm text-[var(--color-text-secondary)]">
      {label}
    </div>
  );
}
