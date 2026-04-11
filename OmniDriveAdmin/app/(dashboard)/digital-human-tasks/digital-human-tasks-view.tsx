"use client";

import { useMemo, useState } from "react";
import { Loader2, RefreshCw, Search, Video, X } from "lucide-react";
import { PageHeader } from "@/components/ui/common";
import { useAdminDigitalHumanTask, useAdminDigitalHumanTasks } from "@/lib/hooks/useAdminDigitalHumanTasks";
import type { AdminDigitalHumanTaskRow } from "@/lib/types";

const STATUS_OPTIONS = [
  { value: "", label: "全部状态" },
  { value: "queued", label: "排队中" },
  { value: "running", label: "处理中" },
  { value: "completed", label: "已完成" },
  { value: "failed", label: "失败" },
  { value: "cancelled", label: "已取消" },
];

const MODE_OPTIONS = [
  { value: "", label: "全部模式" },
  { value: "digital", label: "数字带货" },
  { value: "customize", label: "自定义口播" },
];

const STATUS_CLASS: Record<string, string> = {
  queued: "border-yellow-600/25 bg-yellow-500/10 text-yellow-700",
  running: "border-blue-600/25 bg-blue-500/10 text-blue-700",
  completed: "border-emerald-600/25 bg-emerald-500/10 text-emerald-700",
  failed: "border-red-600/25 bg-red-500/10 text-red-700",
  cancelled: "border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)]",
};

const BILLING_STATUS_CLASS: Record<string, string> = {
  pending: "border-yellow-600/25 bg-yellow-500/10 text-yellow-700",
  precharged: "border-blue-600/25 bg-blue-500/10 text-blue-700",
  settled: "border-emerald-600/25 bg-emerald-500/10 text-emerald-700",
  refunded: "border-sky-600/25 bg-sky-500/10 text-sky-700",
  settlement_pending: "border-orange-600/25 bg-orange-500/10 text-orange-700",
};

function formatDateTime(value?: string | null) {
  if (!value) {
    return "—";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatCredits(value?: number | null) {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "—";
  }
  return value.toLocaleString("zh-CN", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 3,
  });
}

function truncateText(value?: string | null, size = 56) {
  const trimmed = (value || "").trim();
  if (trimmed.length <= size) {
    return trimmed || "—";
  }
  return `${trimmed.slice(0, size)}...`;
}

function StatusPill({ value, toneMap }: { value?: string | null; toneMap: Record<string, string> }) {
  const status = (value || "").trim();
  const tone = toneMap[status] || "border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)]";
  return (
    <span className={`inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium ${tone}`}>
      {status || "未知"}
    </span>
  );
}

function MetricCard({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
      <p className="mb-1 text-xs text-[var(--color-text-secondary)]">{label}</p>
      <p className={`text-xl font-medium ${tone || "text-[var(--color-text-primary)]"}`}>{value}</p>
    </div>
  );
}

function DetailDrawer({ taskId, onClose }: { taskId: string | null; onClose: () => void }) {
  const { data, isLoading, error } = useAdminDigitalHumanTask(taskId);
  if (!taskId) {
    return null;
  }

  const record = data;
  const task = record?.task;

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/40 backdrop-blur-sm">
      <div className="flex h-full w-full max-w-3xl flex-col border-l border-[var(--color-border)] bg-[var(--color-bg-primary)] shadow-2xl">
        <div className="flex items-center justify-between border-b border-[var(--color-border)] p-6">
          <div>
            <p className="text-xs uppercase tracking-[0.24em] text-[var(--color-text-secondary)]">digital human task</p>
            <h2 className="mt-2 text-xl font-semibold">数字人任务详情</h2>
            <p className="mt-1 font-mono text-xs text-[var(--color-text-secondary)]">{taskId}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-2 text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)] hover:text-[var(--color-text-primary)]"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-6">
          {isLoading ? (
            <div className="flex flex-col items-center justify-center py-24 text-[var(--color-text-secondary)]">
              <Loader2 className="mb-4 h-8 w-8 animate-spin" />
              <p>正在读取任务详情...</p>
            </div>
          ) : error || !record || !task ? (
            <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-600">
              读取数字人任务详情失败。
            </div>
          ) : (
            <div className="space-y-6">
              <section className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-5">
                <div className="flex flex-wrap items-center gap-2">
                  <StatusPill value={task.status} toneMap={STATUS_CLASS} />
                  <StatusPill value={task.billingStatus} toneMap={BILLING_STATUS_CLASS} />
                </div>
                <div className="mt-4 grid gap-4 md:grid-cols-2">
                  <div>
                    <p className="text-xs text-[var(--color-text-secondary)]">提交用户</p>
                    <p className="mt-1 text-sm font-medium">{record.owner?.name || "—"}</p>
                    <p className="text-xs text-[var(--color-text-secondary)]">{record.owner?.email || "—"}</p>
                  </div>
                  <div>
                    <p className="text-xs text-[var(--color-text-secondary)]">模式 / 来源</p>
                    <p className="mt-1 text-sm font-medium">{task.mode} / {task.source}</p>
                    <p className="text-xs text-[var(--color-text-secondary)]">远端任务 ID {task.remoteTaskId || "—"}</p>
                  </div>
                  <div>
                    <p className="text-xs text-[var(--color-text-secondary)]">执行模型</p>
                    <p className="mt-1 text-sm font-medium">{task.modelName || "—"}</p>
                  </div>
                </div>
              </section>

              <section className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-5">
                <h3 className="text-sm font-semibold">提交内容</h3>
                <div className="mt-4 grid gap-4 md:grid-cols-2">
                  <div>
                    <p className="text-xs text-[var(--color-text-secondary)]">商品标题</p>
                    <p className="mt-1 text-sm">{task.goodsTitle || "—"}</p>
                  </div>
                  <div>
                    <p className="text-xs text-[var(--color-text-secondary)]">创建时间</p>
                    <p className="mt-1 text-sm">{formatDateTime(task.createdAt)}</p>
                  </div>
                  <div>
                    <p className="text-xs text-[var(--color-text-secondary)]">开始时间</p>
                    <p className="mt-1 text-sm">{formatDateTime(task.startedAt)}</p>
                  </div>
                  <div>
                    <p className="text-xs text-[var(--color-text-secondary)]">完成时间</p>
                    <p className="mt-1 text-sm">{formatDateTime(task.completedAt)}</p>
                  </div>
                </div>
                <div className="mt-4">
                  <p className="text-xs text-[var(--color-text-secondary)]">文案全文</p>
                  <div className="mt-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4 text-sm leading-7 whitespace-pre-wrap">
                    {task.goodsText}
                  </div>
                </div>
              </section>

              <section className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-5">
                <h3 className="text-sm font-semibold">素材与成品</h3>
                <div className="mt-4 grid gap-5 lg:grid-cols-2">
                  <div className="space-y-4">
                    <div>
                      <p className="mb-2 text-xs text-[var(--color-text-secondary)]">人物照片</p>
                      {/* eslint-disable-next-line @next/next/no-img-element */}
                      <img src={task.characterAsset.publicUrl} alt="人物照片" className="h-56 w-full rounded-xl border border-[var(--color-border)] object-cover" />
                    </div>
                    {task.goodsAsset ? (
                      <div>
                        <p className="mb-2 text-xs text-[var(--color-text-secondary)]">商品素材</p>
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img src={task.goodsAsset.publicUrl} alt="商品素材" className="h-56 w-full rounded-xl border border-[var(--color-border)] object-cover" />
                      </div>
                    ) : null}
                  </div>
                  <div className="space-y-4">
                    <div>
                      <p className="mb-2 text-xs text-[var(--color-text-secondary)]">参考音频</p>
                      <audio controls className="w-full">
                        <source src={task.refAudioAsset.publicUrl} />
                      </audio>
                    </div>
                    {task.resultAsset ? (
                      <div>
                        <div className="mb-2 flex items-center justify-between gap-3">
                          <p className="text-xs text-[var(--color-text-secondary)]">成品视频</p>
                          <a
                            href={task.resultAsset.publicUrl}
                            target="_blank"
                            rel="noreferrer"
                            className="text-xs font-medium text-[var(--color-primary)] hover:underline"
                          >
                            下载成品
                          </a>
                        </div>
                        <video controls className="w-full rounded-xl border border-[var(--color-border)] bg-black">
                          <source src={task.resultAsset.publicUrl} />
                        </video>
                      </div>
                    ) : (
                      <div className="rounded-xl border border-dashed border-[var(--color-border)] p-6 text-sm text-[var(--color-text-secondary)]">
                        当前还没有生成成品视频。
                      </div>
                    )}
                  </div>
                </div>
              </section>

              <section className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-5">
                <h3 className="text-sm font-semibold">计费与进度</h3>
                <div className="mt-4 grid gap-4 md:grid-cols-2">
                  <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
                    <p className="text-xs text-[var(--color-text-secondary)]">预计结算</p>
                    <p className="mt-1 text-sm">预计时长 {task.estimatedDurationSeconds} 秒</p>
                    <p className="mt-1 text-sm">预计消耗 {formatCredits(task.estimatedCredits)} 积分</p>
                  </div>
                  <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
                    <p className="text-xs text-[var(--color-text-secondary)]">实际结算</p>
                    <p className="mt-1 text-sm">实际时长 {task.actualDurationSeconds ?? "—"} 秒</p>
                    <p className="mt-1 text-sm">最终消耗 {formatCredits(task.finalCredits)} 积分</p>
                  </div>
                </div>
                <div className="mt-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
                  <p className="text-xs text-[var(--color-text-secondary)]">当前进度</p>
                  <p className="mt-1 text-sm font-medium">{task.progress?.message || "—"}</p>
                  <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                    {task.progress ? `进度 ${task.progress.current}/${task.progress.total} · ${task.progress.percentage}%` : "暂无远端进度回传"}
                  </p>
                  {task.errorMessage ? (
                    <p className="mt-3 text-sm text-red-600">{task.errorMessage}</p>
                  ) : null}
                </div>
              </section>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export function DigitalHumanTasksView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState("");
  const [mode, setMode] = useState("");
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);

  const { data, isLoading, error, refetch, isFetching } = useAdminDigitalHumanTasks({
    page,
    pageSize: 20,
    query: query || undefined,
    status: status || undefined,
    mode: mode || undefined,
  });

  const rows = useMemo(() => data?.items || [], [data?.items]);

  return (
    <div className="space-y-6">
      <PageHeader
        title="数字人历史"
        subtitle="集中查看前台提交的数字人视频任务、素材内容、进度回传、结算状态和最终成品。"
        actions={(
          <button
            type="button"
            onClick={() => refetch()}
            className="inline-flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2 text-sm text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)]"
          >
            <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
            刷新
          </button>
        )}
      />

      {data?.summary ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <MetricCard label="总任务数" value={data.summary.totalTaskCount.toLocaleString("zh-CN")} />
          <MetricCard label="处理中" value={data.summary.runningCount.toLocaleString("zh-CN")} tone="text-blue-600" />
          <MetricCard label="已完成" value={data.summary.completedCount.toLocaleString("zh-CN")} tone="text-emerald-600" />
          <MetricCard label="待补结算" value={data.summary.settlementPendingCount.toLocaleString("zh-CN")} tone="text-orange-600" />
        </div>
      ) : null}

      <div className="space-y-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4">
        <form
          onSubmit={(event) => {
            event.preventDefault();
            setQuery(searchInput.trim());
            setPage(1);
          }}
          className="relative max-w-md"
        >
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            placeholder="搜索任务 ID、用户、商品标题、文案"
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] py-2 pl-9 pr-4 text-sm focus:border-[var(--color-primary)] focus:outline-none"
          />
        </form>

        <div className="flex flex-wrap gap-2">
          {STATUS_OPTIONS.map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => {
                setStatus(option.value);
                setPage(1);
              }}
              className={`rounded-lg border px-3 py-1.5 text-xs transition-colors ${
                status === option.value
                  ? "border-[var(--color-primary)]/50 bg-[var(--color-primary)]/10 text-[var(--color-primary)]"
                  : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)]"
              }`}
            >
              {option.label}
            </button>
          ))}
          {MODE_OPTIONS.map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => {
                setMode(option.value);
                setPage(1);
              }}
              className={`rounded-lg border px-3 py-1.5 text-xs transition-colors ${
                mode === option.value
                  ? "border-[var(--color-primary)]/50 bg-[var(--color-primary)]/10 text-[var(--color-primary)]"
                  : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)]"
              }`}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      <div className="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
        <table className="min-w-full divide-y divide-[var(--color-border)] text-sm">
          <thead className="bg-[var(--color-bg-secondary)] text-left text-xs uppercase tracking-[0.2em] text-[var(--color-text-secondary)]">
            <tr>
              <th className="px-4 py-3">任务</th>
              <th className="px-4 py-3">用户</th>
              <th className="px-4 py-3">内容</th>
              <th className="px-4 py-3">计费</th>
              <th className="px-4 py-3">时间</th>
              <th className="px-4 py-3 text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--color-border)]">
            {isLoading ? (
              <tr>
                <td colSpan={6} className="px-4 py-16 text-center text-[var(--color-text-secondary)]">
                  <div className="flex flex-col items-center justify-center">
                    <Loader2 className="mb-3 h-6 w-6 animate-spin" />
                    正在读取数字人任务...
                  </div>
                </td>
              </tr>
            ) : error ? (
              <tr>
                <td colSpan={6} className="px-4 py-16 text-center text-red-600">
                  读取数字人任务失败，请稍后重试。
                </td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-16 text-center text-[var(--color-text-secondary)]">
                  当前没有符合条件的数字人任务。
                </td>
              </tr>
            ) : (
              rows.map((row: AdminDigitalHumanTaskRow) => (
                <tr key={row.task.id} className="hover:bg-[var(--color-bg-secondary)]/35">
                  <td className="px-4 py-4 align-top">
                    <div className="flex items-start gap-3">
                      <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] p-2 text-[var(--color-accent)]">
                        <Video className="h-4 w-4" />
                      </div>
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <StatusPill value={row.task.status} toneMap={STATUS_CLASS} />
                          <StatusPill value={row.task.billingStatus} toneMap={BILLING_STATUS_CLASS} />
                        </div>
                        <p className="mt-2 text-xs text-[var(--color-text-secondary)]">{row.task.mode} / {row.task.source}</p>
                        <p className="mt-1 text-xs text-[var(--color-text-secondary)]">模型 {row.task.modelName || "—"}</p>
                        <p className="mt-1 truncate font-mono text-[11px] text-[var(--color-text-secondary)]">{row.task.id}</p>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-4 align-top">
                    <p className="text-sm font-medium">{row.owner?.name || "—"}</p>
                    <p className="mt-1 text-xs text-[var(--color-text-secondary)]">{row.owner?.email || "—"}</p>
                  </td>
                  <td className="px-4 py-4 align-top">
                    <p className="text-sm font-medium">{row.task.goodsTitle || "未填写标题"}</p>
                    <p className="mt-1 max-w-[320px] text-xs leading-6 text-[var(--color-text-secondary)]" title={row.task.goodsText}>
                      {truncateText(row.task.goodsText)}
                    </p>
                    {row.task.errorMessage ? (
                      <p className="mt-2 max-w-[320px] text-xs text-red-600" title={row.task.errorMessage}>
                        {truncateText(row.task.errorMessage, 72)}
                      </p>
                    ) : null}
                  </td>
                  <td className="px-4 py-4 align-top">
                    <p className="text-sm">预计 {formatCredits(row.task.estimatedCredits)} 积分</p>
                    <p className="mt-1 text-xs text-[var(--color-text-secondary)]">预计 {row.task.estimatedDurationSeconds} 秒</p>
                    <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                      最终 {formatCredits(row.task.finalCredits)} 积分
                    </p>
                  </td>
                  <td className="px-4 py-4 align-top">
                    <p className="text-sm">{formatDateTime(row.task.createdAt)}</p>
                    <p className="mt-1 text-xs text-[var(--color-text-secondary)]">更新 {formatDateTime(row.task.updatedAt)}</p>
                  </td>
                  <td className="px-4 py-4 text-right align-top">
                    <button
                      type="button"
                      onClick={() => setSelectedTaskId(row.task.id)}
                      className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-xs font-medium text-[var(--color-text-primary)] transition-colors hover:bg-[var(--color-bg-secondary)]"
                    >
                      查看详情
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {data?.pagination && data.pagination.totalPages > 1 ? (
        <div className="flex items-center justify-between text-sm text-[var(--color-text-secondary)]">
          <p>
            第 {data.pagination.page} / {data.pagination.totalPages} 页，共 {data.pagination.total.toLocaleString("zh-CN")} 条
          </p>
          <div className="flex items-center gap-2">
            <button
              type="button"
              disabled={page <= 1}
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 disabled:opacity-50"
            >
              上一页
            </button>
            <button
              type="button"
              disabled={page >= data.pagination.totalPages}
              onClick={() => setPage((current) => current + 1)}
              className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 disabled:opacity-50"
            >
              下一页
            </button>
          </div>
        </div>
      ) : null}

      <DetailDrawer taskId={selectedTaskId} onClose={() => setSelectedTaskId(null)} />
    </div>
  );
}
