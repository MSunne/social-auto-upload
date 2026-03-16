"use client";

import { useState } from "react";
import { useAdminAIJobs, useBulkActionAIJobs } from "@/lib/hooks/useAdminAIJobs";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, RefreshCw, XCircle, RotateCcw, Cpu } from "lucide-react";

const STATUS_OPTIONS = [
  { value: "", label: "全部" },
  { value: "queued", label: "排队中" },
  { value: "running", label: "处理中" },
  { value: "completed", label: "已完成" },
  { value: "failed", label: "失败" },
  { value: "cancelled", label: "已取消" },
];

const getStatusBadge = (status: string) => {
  const map: Record<string, string> = {
    queued: "bg-yellow-500/10 text-yellow-400 border-yellow-500/20",
    running: "bg-blue-500/10 text-blue-400 border-blue-500/20",
    completed: "bg-green-500/10 text-green-400 border-green-500/20",
    failed: "bg-red-500/10 text-red-400 border-red-500/20",
    cancelled: "bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border-[var(--color-border)]",
    pending_delivery: "bg-purple-500/10 text-purple-400 border-purple-500/20",
  };
  const cls = map[status] ?? "bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border-[var(--color-border)]";
  const label: Record<string, string> = { queued: "排队中", running: "处理中", completed: "已完成", failed: "失败", cancelled: "已取消", pending_delivery: "待下发" };
  return <span className={`px-2 py-0.5 text-xs rounded-full border font-medium ${cls}`}>{label[status] || status}</span>;
};

const getCategoryColor = (category?: string) => {
  switch (category) {
    case "image": return "text-blue-400";
    case "video": return "text-purple-400";
    case "text": return "text-green-400";
    default: return "text-[var(--color-text-secondary)]";
  }
};

export function AIJobsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const { data, isLoading, error, refetch } = useAdminAIJobs({ page, pageSize: 20, query: query || undefined, status: status || undefined });
  const bulkAction = useBulkActionAIJobs();

  const handleSearch = (e: React.FormEvent) => { e.preventDefault(); setQuery(searchInput); setPage(1); setSelected(new Set()); };
  const toggleSelect = (id: string) => setSelected(prev => { const n = new Set(prev); n.has(id) ? n.delete(id) : n.add(id); return n; });
  const toggleSelectAll = () => {
    if (!data) return;
    const all = data.data.map(r => r.job.id);
    setSelected(selected.size === all.length ? new Set() : new Set(all));
  };
  const handleBulkAction = async (action: string, label: string) => {
    if (selected.size === 0) return;
    if (!confirm(`确认对 ${selected.size} 个 AI 作业执行「${label}」操作？`)) return;
    try { await bulkAction.mutateAsync({ ids: Array.from(selected), action }); setSelected(new Set()); }
    catch { alert("操作失败，请重试"); }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="AI 作业管理" subtitle="监控全平台 AI 生成作业状态、模型消耗、产物交付与异常处理。" />
        <button onClick={() => refetch()} className="flex items-center gap-2 px-3 py-2 border border-[var(--color-border)] rounded-lg text-sm hover:bg-[var(--color-bg-secondary)] transition-colors">
          <RefreshCw className="h-4 w-4" /> 刷新
        </button>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center bg-[var(--color-bg-secondary)] p-4 rounded-xl border border-[var(--color-border)]">
        <form onSubmit={handleSearch} className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input type="text" placeholder="搜索作业ID / 模型名称 / 用户..." value={searchInput} onChange={e => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]" />
        </form>
        <div className="flex gap-1 flex-wrap">
          {STATUS_OPTIONS.map(opt => (
            <button key={opt.value} onClick={() => { setStatus(opt.value); setPage(1); setSelected(new Set()); }}
              className={`px-3 py-1.5 text-xs rounded-lg border transition-colors ${status === opt.value ? "bg-[var(--color-primary)]/10 border-[var(--color-primary)]/50 text-[var(--color-primary)]" : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)]"}`}>
              {opt.label}
            </button>
          ))}
        </div>
        {selected.size > 0 && (
          <div className="flex items-center gap-2 ml-auto">
            <span className="text-xs text-[var(--color-text-secondary)]">已选 {selected.size}</span>
            <button onClick={() => handleBulkAction("retry", "重试")} className="flex items-center gap-1 px-2.5 py-1.5 text-xs text-blue-400 border border-blue-500/30 rounded-lg hover:bg-blue-500/10 transition-colors"><RotateCcw className="h-3 w-3" /> 重试</button>
            <button onClick={() => handleBulkAction("cancel", "取消")} className="flex items-center gap-1 px-2.5 py-1.5 text-xs text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/10 transition-colors"><XCircle className="h-3 w-3" /> 取消</button>
          </div>
        )}
      </div>

      {/* Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-4 py-3.5">
                  <input type="checkbox" className="rounded" checked={data ? selected.size === data.data.length && data.data.length > 0 : false} onChange={toggleSelectAll} />
                </th>
                <th className="px-4 py-3.5 font-medium">模型 / 类型</th>
                <th className="px-4 py-3.5 font-medium">归属用户</th>
                <th className="px-4 py-3.5 font-medium">执行设备</th>
                <th className="px-4 py-3.5 font-medium">状态</th>
                <th className="px-4 py-3.5 font-medium">积分消耗</th>
                <th className="px-4 py-3.5 font-medium">产物 / 任务</th>
                <th className="px-4 py-3.5 font-medium">创建时间</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr><td colSpan={8} className="px-6 py-12 text-center">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载中...</p>
                </td></tr>
              )}
              {error && <tr><td colSpan={8} className="px-6 py-10 text-center text-red-500 text-sm">加载失败</td></tr>}
              {data && data.data.length === 0 && <tr><td colSpan={8} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">无 AI 作业记录</td></tr>}
              {data && data.data.map(row => (
                <tr key={row.job.id} className={`hover:bg-[var(--color-bg-secondary)]/50 transition-colors ${selected.has(row.job.id) ? "bg-[var(--color-primary)]/5" : ""}`}>
                  <td className="px-4 py-3.5">
                    <input type="checkbox" className="rounded" checked={selected.has(row.job.id)} onChange={() => toggleSelect(row.job.id)} />
                  </td>
                  <td className="px-4 py-3.5">
                    <div className="flex items-center gap-2">
                      <Cpu className={`h-4 w-4 flex-shrink-0 ${getCategoryColor(row.model?.category)}`} />
                      <div>
                        <div className="font-medium font-mono text-xs">{row.job.modelName}</div>
                        <div className="text-xs text-[var(--color-text-secondary)]">{row.job.jobType} · {row.job.source}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3.5">
                    {row.owner ? <><div className="text-sm">{row.owner.name}</div><div className="text-xs text-[var(--color-text-secondary)]">{row.owner.email}</div></>
                      : <span className="text-xs text-[var(--color-text-secondary)]">—</span>}
                  </td>
                  <td className="px-4 py-3.5">
                    {row.device ? <><div className="text-sm">{row.device.name}</div><div className="text-xs font-mono text-[var(--color-text-secondary)]">{row.device.deviceCode}</div></>
                      : <span className="text-xs text-[var(--color-text-secondary)]">— 云端</span>}
                  </td>
                  <td className="px-4 py-3.5">
                    {getStatusBadge(row.job.status)}
                    {row.job.message && <div className="text-xs text-[var(--color-text-secondary)] mt-1 max-w-[140px] truncate" title={row.job.message}>{row.job.message}</div>}
                  </td>
                  <td className="px-4 py-3.5 font-mono text-sm">
                    {row.job.costCredits > 0 ? <span className="text-amber-400">{row.job.costCredits.toLocaleString()}</span> : <span className="text-[var(--color-text-secondary)]">0</span>}
                  </td>
                  <td className="px-4 py-3.5 text-xs text-[var(--color-text-secondary)]">
                    <div>{row.artifactCount} 产物</div>
                    <div>{row.publishTaskCount} 任务</div>
                  </td>
                  <td className="px-4 py-3.5 text-xs text-[var(--color-text-secondary)]">
                    {new Date(row.job.createdAt).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">共 <span className="font-medium">{data.pagination.total}</span> 个 AI 作业</p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}
    </div>
  );
}
