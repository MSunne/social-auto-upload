"use client";

import { useState } from "react";
import { useDistributionRelations } from "@/lib/hooks/useDistribution";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, RefreshCw, Network, Plus } from "lucide-react";
import { RelationDrawer } from "./relation-drawer";

export function RelationsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [statusParam, setStatusParam] = useState("");
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);

  const { data, isLoading, error, refetch } = useDistributionRelations({ 
    page, 
    pageSize: 30, 
    query: query || undefined, 
    status: statusParam || undefined 
  });

  const handleSearch = (e: React.FormEvent) => { e.preventDefault(); setQuery(searchInput); setPage(1); };

  const renderStatus = (status: string) => {
    if (status === "active") {
      return <span className="px-2 py-0.5 text-xs font-medium rounded bg-green-500/10 text-green-400 border border-green-500/20">生效中</span>;
    }
    return <span className="px-2 py-0.5 text-xs font-medium rounded bg-gray-500/10 text-gray-400 border border-gray-500/20">已失效</span>;
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="分销关系网络" subtitle="查看全平台推广员与受邀人的绑定关系，支持手动绑定补录。" />
        <div className="flex items-center gap-3">
          <button onClick={() => refetch()} className="flex items-center gap-2 px-3 py-2 border border-[var(--color-border)] rounded-lg text-sm hover:bg-[var(--color-bg-secondary)] transition-colors">
            <RefreshCw className="h-4 w-4" /> <span className="hidden sm:inline">刷新</span>
          </button>
          <button onClick={() => setIsDrawerOpen(true)} className="flex items-center gap-2 px-3 py-2 bg-[var(--color-primary)] text-white rounded-lg text-sm hover:brightness-110 transition-all font-medium">
            <Plus className="h-4 w-4" /> 人工绑定
          </button>
        </div>
      </div>

      {data && data.summary && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">全网绑定总对数</p>
            <p className="text-2xl font-bold">{data.summary.totalCount}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">当前生效关系对</p>
            <p className="text-2xl font-bold text-green-400">{data.summary.activeCount}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">已失效解绑</p>
            <p className="text-2xl font-bold text-[var(--color-text-secondary)]">{data.summary.inactiveCount}</p>
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3 justify-between items-start sm:items-center">
        <div className="flex gap-1 bg-[var(--color-bg-secondary)] p-1 rounded-lg border border-[var(--color-border)] overflow-x-auto w-full sm:w-auto">
          {[{ id: "", label: "全部关系" }, { id: "active", label: "生效中" }, { id: "inactive", label: "已失效" }].map(tab => (
            <button key={tab.id} onClick={() => { setStatusParam(tab.id); setPage(1); }}
              className={`px-4 py-1.5 text-sm rounded-md whitespace-nowrap transition-colors ${statusParam === tab.id ? "bg-[var(--color-bg-primary)] text-[var(--color-text-primary)] shadow-sm border border-[var(--color-border)] opacity-100" : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-primary)]/50 border border-transparent opacity-80"}`}>
              {tab.label}
            </button>
          ))}
        </div>
        <form onSubmit={handleSearch} className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input type="text" placeholder="搜索 推广员/受邀人 邮箱或名字..." value={searchInput} onChange={e => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors" />
        </form>
      </div>

      {/* Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-5 py-3.5 font-medium w-[240px]">推广员 (上级)</th>
                <th className="px-5 py-3.5 font-medium w-16 text-center"></th>
                <th className="px-5 py-3.5 font-medium w-[240px]">受邀人 (下级)</th>
                <th className="px-5 py-3.5 font-medium">绑定时间</th>
                <th className="px-5 py-3.5 font-medium">状态</th>
                <th className="px-5 py-3.5 font-medium">备注摘要</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr><td colSpan={6} className="px-6 py-12 text-center">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载关系网络...</p>
                </td></tr>
              )}
              {error && <tr><td colSpan={6} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td></tr>}
              {data && data.items.length === 0 && <tr><td colSpan={6} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">暂无符合条件的绑定关系</td></tr>}
              {data && data.items.map(row => (
                <tr key={row.id} className="hover:bg-[var(--color-bg-secondary)]/50 transition-colors">
                  <td className="px-5 py-3.5">
                    <div className="text-sm font-medium">{row.promoter.name}</div>
                    <div className="text-xs text-[var(--color-text-secondary)] mt-0.5">{row.promoter.email}</div>
                  </td>
                  <td className="px-5 py-3.5 text-center">
                    <Network className="h-4 w-4 mx-auto text-[var(--color-primary)]/50 opacity-50 block rotate-90 sm:rotate-0" />
                  </td>
                  <td className="px-5 py-3.5">
                    <div className="text-sm font-medium">{row.invitee.name}</div>
                    <div className="text-xs text-[var(--color-text-secondary)] mt-0.5">{row.invitee.email}</div>
                  </td>
                  <td className="px-5 py-3.5 whitespace-nowrap">
                    <div className="text-xs text-[var(--color-text-secondary)]">{new Date(row.createdAt).toLocaleDateString("zh-CN")}</div>
                    <div className="font-mono text-xs">{new Date(row.createdAt).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })}</div>
                  </td>
                  <td className="px-5 py-3.5">{renderStatus(row.status)}</td>
                  <td className="px-5 py-3.5">
                    <div className="text-xs text-[var(--color-text-secondary)] max-w-[200px] truncate" title={row.notes || ""}>
                      {row.notes || "—"}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">共 <span className="font-medium">{data.pagination.total}</span> 条关系绑定</p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}

      <RelationDrawer 
        isOpen={isDrawerOpen} 
        onClose={() => setIsDrawerOpen(false)} 
        onSuccess={() => refetch()} 
      />
    </div>
  );
}
