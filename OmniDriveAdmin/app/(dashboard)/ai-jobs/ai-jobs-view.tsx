"use client";

import { useMemo, useState } from "react";
import { Cpu, Loader2, RefreshCw, RotateCcw, Search, XCircle } from "lucide-react";
import { PageHeader } from "@/components/ui/common";
import { useAdminAIJobs, useBulkActionAIJobs, useDeleteAdminAIJob } from "@/lib/hooks/useAdminAIJobs";
import { getModelDisplayName } from "@/lib/model-display";
import type { AdminAIJobListItem } from "@/lib/types";
import { AIJobDetailDrawer } from "./ai-job-detail-drawer";

const STATUS_OPTIONS = [
  { value: "", label: "全部" },
  { value: "queued", label: "排队中" },
  { value: "running", label: "处理中" },
  { value: "completed", label: "已完成" },
  { value: "failed", label: "失败" },
  { value: "cancelled", label: "已取消" },
];

const STATUS_CLASS: Record<string, string> = {
  queued: "border-yellow-600/25 bg-yellow-500/10 text-yellow-700",
  running: "border-blue-600/25 bg-blue-500/10 text-blue-700",
  completed: "border-emerald-600/25 bg-emerald-500/10 text-emerald-700",
  failed: "border-red-600/25 bg-red-500/10 text-red-700",
  cancelled: "border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)]",
  pending_delivery: "border-purple-600/25 bg-purple-500/10 text-purple-700",
};

const STATUS_LABEL: Record<string, string> = {
  queued: "排队中",
  running: "处理中",
  completed: "已完成",
  failed: "失败",
  cancelled: "已取消",
  pending_delivery: "待下发",
};

const VISIBILITY_OPTIONS = [
  { value: "active", label: "进行中/正常" },
  { value: "deleted", label: "已删除" },
  { value: "all", label: "全部" },
] as const;

function formatCompactTime(value?: string | null) {
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
  });
}

function getJobMessageTone(status: string, message?: string | null) {
  if (!message) {
    return "text-[var(--color-text-secondary)]";
  }
  if (status === "failed" || status === "cancelled") {
    return "text-red-600";
  }
  if (status === "success" || status === "completed") {
    return message.includes("计费待处理") ? "text-amber-700" : "text-emerald-700";
  }
  return "text-[var(--color-text-secondary)]";
}

function StatusPill({ status }: { status: string }) {
  const tone = STATUS_CLASS[status] || "border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)]";
  return (
    <span className={`inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium ${tone}`}>
      {STATUS_LABEL[status] || status || "未知"}
    </span>
  );
}

function JobRow({
  row,
  selected,
  selectable,
  onToggleSelect,
  onOpenDetail,
  onDelete,
}: {
  row: AdminAIJobListItem;
  selected: boolean;
  selectable: boolean;
  onToggleSelect: () => void;
  onOpenDetail: () => void;
  onDelete: () => void;
}) {
  const messageTone = getJobMessageTone(row.status, row.messagePreview);

  return (
    <tr className={row.deletedAt ? "bg-[var(--color-bg-secondary)]/20 text-[var(--color-text-secondary)]" : selected ? "bg-[var(--color-primary)]/5" : "hover:bg-[var(--color-bg-secondary)]/35"}>
      <td className="px-3 py-3 align-top">
        <input type="checkbox" checked={selected} onChange={onToggleSelect} className="rounded" disabled={!selectable} />
      </td>
      <td className="px-3 py-3 align-top">
        <div className="flex items-start gap-3">
          <div className="mt-0.5 rounded-xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] p-2">
            <Cpu className="h-4 w-4 text-[var(--color-text-secondary)]" />
          </div>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <p className="font-medium text-[var(--color-text-primary)]">{getModelDisplayName(row)}</p>
              <StatusPill status={row.status} />
              {row.deletedAt ? (
                <span className="inline-flex items-center rounded-full border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-2.5 py-1 text-xs font-medium text-[var(--color-text-secondary)]">
                  已删除
                </span>
              ) : null}
            </div>
            <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
              {row.jobType === "digital_human" ? "数字人口播生成" : row.jobType} · {row.source}
            </p>
            <p className="mt-1 truncate font-mono text-[11px] text-[var(--color-text-secondary)]">{row.id}</p>
          </div>
        </div>
      </td>
      <td className="px-3 py-3 align-top">
        <div className="space-y-1">
          <p className="text-sm text-[var(--color-text-primary)]">{row.owner?.name || "—"}</p>
          <p className="text-xs text-[var(--color-text-secondary)]">{row.owner?.email || "—"}</p>
          <p className="text-xs text-[var(--color-text-secondary)]">{row.device?.name || "云端"}</p>
        </div>
      </td>
      <td className="px-3 py-3 align-top">
        <div className="space-y-1">
          <p className="text-sm text-[var(--color-text-primary)]">{row.skill?.name || "未绑定技能"}</p>
          <p className="text-xs text-[var(--color-text-secondary)]">积分 {row.costCredits.toLocaleString()}</p>
          <p className="text-xs text-[var(--color-text-secondary)]">投递 {row.deliveryStatus || "—"}</p>
        </div>
      </td>
      <td className="px-3 py-3 align-top">
        <div className="space-y-1">
          <p className="text-sm text-[var(--color-text-primary)]">{formatCompactTime(row.createdAt)}</p>
          {row.runAt ? (
            <p className="text-xs text-[var(--color-text-secondary)]">计划生成 {formatCompactTime(row.runAt)}</p>
          ) : (
            <p className="text-xs text-[var(--color-text-secondary)]">更新 {formatCompactTime(row.updatedAt)}</p>
          )}
          {row.messagePreview ? (
            <p className={`line-clamp-2 max-w-[240px] text-xs leading-5 ${messageTone}`} title={row.messagePreview}>
              {row.messagePreview}
            </p>
          ) : null}
        </div>
      </td>
      <td className="px-3 py-3 align-top">
        <div className="flex justify-end gap-2">
          {row.actions.canDelete ? (
            <button
              type="button"
              onClick={onDelete}
              className="rounded-lg border border-red-500/25 px-3 py-1.5 text-xs font-medium text-red-600 transition-colors hover:bg-red-500/10"
            >
              删除
            </button>
          ) : null}
          <button
            type="button"
            onClick={onOpenDetail}
            className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-xs font-medium text-[var(--color-text-primary)] transition-colors hover:bg-[var(--color-bg-secondary)]"
          >
            查看日志
          </button>
        </div>
      </td>
    </tr>
  );
}

export function AIJobsView() {
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState("");
  const [visibility, setVisibility] = useState<"active" | "deleted" | "all">("active");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);

  const { data, isLoading, error, refetch, isFetching, fetchNextPage, hasNextPage, isFetchingNextPage } = useAdminAIJobs({
    limit: 20,
    query: query || undefined,
    status: status || undefined,
    visibility,
  });
  const bulkAction = useBulkActionAIJobs();
  const deleteAIJob = useDeleteAdminAIJob();
  const rows = useMemo(() => data?.pages.flatMap((page) => page.items) ?? [], [data?.pages]);
  const selectableRows = useMemo(() => rows.filter((item) => !item.deletedAt), [rows]);

  const handleSearch = (event: React.FormEvent) => {
    event.preventDefault();
    setQuery(searchInput.trim());
    setSelected(new Set());
  };

  const toggleSelect = (id: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });

  const toggleSelectAll = () => {
    if (!selectableRows.length) {
      return;
    }
    const ids = selectableRows.map((item) => item.id);
    setSelected(selected.size === ids.length ? new Set() : new Set(ids));
  };

  const handleDelete = async (jobId: string) => {
    if (!window.confirm("确认软删除这条 AI 作业吗？删除后普通前端用户将不再看到它。")) {
      return;
    }
    try {
      await deleteAIJob.mutateAsync(jobId);
      setSelected((current) => {
        const next = new Set(current);
        next.delete(jobId);
        return next;
      });
    } catch {
      window.alert("删除失败，请稍后重试");
    }
  };

  const handleBulkAction = async (action: string, label: string) => {
    if (selected.size === 0) {
      return;
    }
    if (!window.confirm(`确认对 ${selected.size} 个 AI 作业执行“${label}”？`)) {
      return;
    }
    try {
      await bulkAction.mutateAsync({ ids: Array.from(selected), action });
      setSelected(new Set());
    } catch {
      window.alert("操作失败，请稍后重试");
    }
  };

  return (
    <div className="space-y-5">
      <PageHeader
        title="AI 作业管理"
        subtitle="列表只保留最核心的作业摘要，详细执行内容进入单条作业查看。"
        actions={
          <button
            type="button"
            onClick={() => refetch()}
            className="inline-flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2 text-sm text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)]"
          >
            {isFetching && !isFetchingNextPage ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4" />
            )}
            刷新
          </button>
        }
      />

      <section className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)]/55 p-3">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
          <form onSubmit={handleSearch} className="relative flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-secondary)]" />
            <input
              type="text"
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder="搜索作业 ID / 用户邮箱"
              className="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] py-2 pl-9 pr-4 text-sm focus:border-[var(--color-primary)] focus:outline-none"
            />
          </form>

          <div className="flex flex-wrap gap-2">
            {STATUS_OPTIONS.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => {
                  setStatus(option.value);
                  setSelected(new Set());
                }}
                className={`rounded-lg border px-3 py-1.5 text-xs transition-colors ${
                  status === option.value
                    ? "border-[var(--color-primary)]/45 bg-[var(--color-primary)]/10 text-[var(--color-primary)]"
                    : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)]"
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>

          <div className="flex flex-wrap gap-2">
            {VISIBILITY_OPTIONS.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => {
                  setVisibility(option.value);
                  setSelected(new Set());
                }}
                className={`rounded-lg border px-3 py-1.5 text-xs transition-colors ${
                  visibility === option.value
                    ? "border-[var(--color-primary)]/45 bg-[var(--color-primary)]/10 text-[var(--color-primary)]"
                    : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)]"
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>

          {selected.size > 0 ? (
            <div className="flex flex-wrap items-center gap-2 lg:ml-auto">
              <span className="text-xs text-[var(--color-text-secondary)]">已选 {selected.size}</span>
              <button
                type="button"
                onClick={() => handleBulkAction("retry", "重试")}
                className="inline-flex items-center gap-1 rounded-lg border border-blue-500/25 px-3 py-1.5 text-xs text-blue-600 transition-colors hover:bg-blue-500/10"
              >
                <RotateCcw className="h-3 w-3" />
                重试
              </button>
              <button
                type="button"
                onClick={() => handleBulkAction("cancel", "取消")}
                className="inline-flex items-center gap-1 rounded-lg border border-red-500/25 px-3 py-1.5 text-xs text-red-600 transition-colors hover:bg-red-500/10"
              >
                <XCircle className="h-3 w-3" />
                取消
              </button>
            </div>
          ) : null}
        </div>
      </section>

      <div className="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[980px] text-left text-sm">
            <thead className="border-b border-[var(--color-border)] bg-[var(--color-bg-secondary)]/85 text-xs uppercase tracking-[0.18em] text-[var(--color-text-secondary)]">
              <tr>
                <th className="px-3 py-3">
                  <input
                    type="checkbox"
                    className="rounded"
                    checked={Boolean(selectableRows.length > 0 && selected.size === selectableRows.length)}
                    onChange={toggleSelectAll}
                  />
                </th>
                <th className="px-3 py-3 font-medium">作业</th>
                <th className="px-3 py-3 font-medium">用户 / 设备</th>
                <th className="px-3 py-3 font-medium">链路信息</th>
                <th className="px-3 py-3 font-medium">时间 / 消息</th>
                <th className="px-3 py-3 font-medium">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading ? (
                <tr>
                  <td colSpan={6} className="px-6 py-14 text-center">
                    <Loader2 className="mx-auto h-6 w-6 animate-spin text-[var(--color-text-secondary)]" />
                    <p className="mt-3 text-sm text-[var(--color-text-secondary)]">正在加载 AI 作业...</p>
                  </td>
                </tr>
              ) : error ? (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-sm text-red-600">
                    读取失败，请刷新后重试。
                  </td>
                </tr>
              ) : rows.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-14 text-center text-sm text-[var(--color-text-secondary)]">
                    当前没有匹配的 AI 作业。
                  </td>
                </tr>
              ) : (
                rows.map((row) => (
                  <JobRow
                    key={row.id}
                    row={row}
                    selected={selected.has(row.id)}
                    selectable={!row.deletedAt}
                    onToggleSelect={() => toggleSelect(row.id)}
                    onOpenDetail={() => setSelectedJobId(row.id)}
                    onDelete={() => void handleDelete(row.id)}
                  />
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {rows.length > 0 ? (
        <div className="flex items-center justify-end gap-3">
          {hasNextPage ? (
            <button
              type="button"
              onClick={() => fetchNextPage()}
              disabled={isFetchingNextPage}
              className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm transition-colors hover:bg-[var(--color-bg-secondary)] disabled:opacity-50"
            >
              {isFetchingNextPage ? "加载中..." : "加载更多"}
            </button>
          ) : (
            <p className="text-sm text-[var(--color-text-secondary)]">已加载全部结果</p>
          )}
        </div>
      ) : null}

      <AIJobDetailDrawer jobId={selectedJobId} onClose={() => setSelectedJobId(null)} />
    </div>
  );
}
