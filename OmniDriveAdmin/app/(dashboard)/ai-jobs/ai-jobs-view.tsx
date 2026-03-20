"use client";

import { useState } from "react";
import { Cpu, Loader2, RefreshCw, RotateCcw, Search, XCircle } from "lucide-react";
import { PageHeader } from "@/components/ui/common";
import { useAdminAIJobs, useBulkActionAIJobs } from "@/lib/hooks/useAdminAIJobs";
import type { AdminAIJobRow } from "@/lib/types";
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

function getCategoryColor(category?: string) {
  switch (category) {
    case "image":
      return "text-blue-600";
    case "video":
      return "text-purple-600";
    case "chat":
      return "text-emerald-600";
    case "music":
      return "text-amber-600";
    default:
      return "text-[var(--color-text-secondary)]";
  }
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
  onToggleSelect,
  onOpenDetail,
}: {
  row: AdminAIJobRow;
  selected: boolean;
  onToggleSelect: () => void;
  onOpenDetail: () => void;
}) {
  return (
    <tr className={selected ? "bg-[var(--color-primary)]/5" : "hover:bg-[var(--color-bg-secondary)]/35"}>
      <td className="px-3 py-3 align-top">
        <input type="checkbox" checked={selected} onChange={onToggleSelect} className="rounded" />
      </td>
      <td className="px-3 py-3 align-top">
        <div className="flex items-start gap-3">
          <div className="mt-0.5 rounded-xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] p-2">
            <Cpu className={`h-4 w-4 ${getCategoryColor(row.model?.category)}`} />
          </div>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <p className="font-medium text-[var(--color-text-primary)]">{row.job.modelName}</p>
              <StatusPill status={row.job.status} />
            </div>
            <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
              {row.job.jobType} · {row.job.source}
            </p>
            <p className="mt-1 truncate font-mono text-[11px] text-[var(--color-text-secondary)]">{row.job.id}</p>
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
          <p className="text-xs text-[var(--color-text-secondary)]">
            积分 {row.job.costCredits.toLocaleString()} · 产物 {row.artifactCount} · 发布 {row.publishTaskCount}
          </p>
          <p className="text-xs text-[var(--color-text-secondary)]">投递 {row.job.deliveryStatus || "—"}</p>
        </div>
      </td>
      <td className="px-3 py-3 align-top">
        <div className="space-y-1">
          <p className="text-sm text-[var(--color-text-primary)]">{formatCompactTime(row.job.createdAt)}</p>
          <p className="text-xs text-[var(--color-text-secondary)]">更新 {formatCompactTime(row.job.updatedAt)}</p>
          {row.job.message ? (
            <p className="line-clamp-2 max-w-[240px] text-xs leading-5 text-red-600" title={row.job.message}>
              {row.job.message}
            </p>
          ) : null}
        </div>
      </td>
      <td className="px-3 py-3 align-top">
        <button
          type="button"
          onClick={onOpenDetail}
          className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-xs font-medium text-[var(--color-text-primary)] transition-colors hover:bg-[var(--color-bg-secondary)]"
        >
          查看日志
        </button>
      </td>
    </tr>
  );
}

export function AIJobsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);

  const { data, isLoading, error, refetch, isFetching } = useAdminAIJobs({
    page,
    pageSize: 20,
    query: query || undefined,
    status: status || undefined,
  });
  const bulkAction = useBulkActionAIJobs();

  const handleSearch = (event: React.FormEvent) => {
    event.preventDefault();
    setQuery(searchInput.trim());
    setPage(1);
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
    if (!data) {
      return;
    }
    const ids = data.items.map((item) => item.job.id);
    setSelected(selected.size === ids.length ? new Set() : new Set(ids));
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
        subtitle="列表保持紧凑，重点问题直接点开看完整执行时间线、参数负载、产物和发布衔接。"
        actions={
          <button
            type="button"
            onClick={() => refetch()}
            className="inline-flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2 text-sm text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)]"
          >
            {isFetching ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
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
              placeholder="搜索作业 ID、模型名称、用户邮箱"
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
                  setPage(1);
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
                    checked={Boolean(data && data.items.length > 0 && selected.size === data.items.length)}
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
              ) : data && data.items.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-14 text-center text-sm text-[var(--color-text-secondary)]">
                    当前没有匹配的 AI 作业。
                  </td>
                </tr>
              ) : (
                data?.items.map((row) => (
                  <JobRow
                    key={row.job.id}
                    row={row}
                    selected={selected.has(row.job.id)}
                    onToggleSelect={() => toggleSelect(row.job.id)}
                    onOpenDetail={() => setSelectedJobId(row.job.id)}
                  />
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {data && data.pagination.totalPages > 1 ? (
        <div className="flex items-center justify-between">
          <p className="text-sm text-[var(--color-text-secondary)]">
            共 <span className="font-medium text-[var(--color-text-primary)]">{data.pagination.total}</span> 个 AI 作业
          </p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page === 1}
              className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm transition-colors hover:bg-[var(--color-bg-secondary)] disabled:opacity-50"
            >
              上一页
            </button>
            <button
              type="button"
              onClick={() => setPage((current) => Math.min(data.pagination.totalPages, current + 1))}
              disabled={page >= data.pagination.totalPages}
              className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm transition-colors hover:bg-[var(--color-bg-secondary)] disabled:opacity-50"
            >
              下一页
            </button>
          </div>
        </div>
      ) : null}

      <AIJobDetailDrawer jobId={selectedJobId} onClose={() => setSelectedJobId(null)} />
    </div>
  );
}
