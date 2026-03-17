"use client";

import { useState } from "react";
import { useAdminSkills, useUpdateAdminSkill } from "@/lib/hooks/useSkills";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, RefreshCw, Layers, CheckCircle, XCircle } from "lucide-react";
import { AdminSkillSummary } from "@/lib/types";

export function SkillsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [statusParam, setStatusParam] = useState("");

  const { data, isLoading, error, refetch } = useAdminSkills({ 
    page, 
    pageSize: 30, 
    query: query || undefined, 
    status: statusParam || undefined 
  });
  
  const updateM = useUpdateAdminSkill();

  const handleSearch = (e: React.FormEvent) => { e.preventDefault(); setQuery(searchInput); setPage(1); };

  const handleToggleStatus = async (skill: AdminSkillSummary) => {
    const newStatus = !skill.isEnabled;
    if (!confirm(`确定要${newStatus ? "启用" : "停用"}技能 "${skill.name}" 吗？`)) return;
    
    try {
      await updateM.mutateAsync({ skillId: skill.id, isEnabled: newStatus });
      refetch();
    } catch {
      alert("操作失败，请重试");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="产品技能配置" subtitle="管理系统所有对外的AI处理技能，控制技能上下线状态。" />
        <button onClick={() => refetch()} className="flex items-center gap-2 px-3 py-2 border border-[var(--color-border)] rounded-lg text-sm hover:bg-[var(--color-bg-secondary)] transition-colors">
          <RefreshCw className="h-4 w-4" /> <span className="hidden sm:inline">刷新</span>
        </button>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3 justify-between items-start sm:items-center">
        <div className="flex gap-1 bg-[var(--color-bg-secondary)] p-1 rounded-lg border border-[var(--color-border)] overflow-x-auto w-full sm:w-auto">
          {[{ id: "", label: "全部技能" }, { id: "active", label: "已启用" }, { id: "inactive", label: "已停用" }].map(tab => (
            <button key={tab.id} onClick={() => { setStatusParam(tab.id); setPage(1); }}
              className={`px-4 py-1.5 text-sm rounded-md whitespace-nowrap transition-colors ${statusParam === tab.id ? "bg-[var(--color-bg-primary)] text-[var(--color-text-primary)] shadow-sm border border-[var(--color-border)] opacity-100" : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-primary)]/50 border border-transparent opacity-80"}`}>
              {tab.label}
            </button>
          ))}
        </div>
        <form onSubmit={handleSearch} className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input type="text" placeholder="搜索 技能名称..." value={searchInput} onChange={e => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors" />
        </form>
      </div>

      {/* Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-5 py-3.5 font-medium">服务 ID</th>
                <th className="px-5 py-3.5 font-medium">技能名称</th>
                <th className="px-5 py-3.5 font-medium">模型标识</th>
                <th className="px-5 py-3.5 font-medium text-center">状态</th>
                <th className="px-5 py-3.5 font-medium text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr><td colSpan={5} className="px-6 py-12 text-center">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载技能列表...</p>
                </td></tr>
              )}
              {error && <tr><td colSpan={5} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td></tr>}
              {data && data.items.length === 0 && <tr><td colSpan={5} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">暂无技能配置</td></tr>}
              {data && data.items.map(row => (
                <tr key={row.id} className={`transition-colors ${row.isEnabled ? "hover:bg-[var(--color-bg-secondary)]/50" : "bg-[var(--color-bg-secondary)]/30 opacity-75"}`}>
                  <td className="px-5 py-3.5">
                    <div className="font-mono text-xs text-[var(--color-text-secondary)]">{row.id}</div>
                  </td>
                  <td className="px-5 py-3.5">
                    <div className="flex items-center gap-2">
                      <div className="w-8 h-8 rounded-lg bg-[var(--color-primary)]/10 flex items-center justify-center text-[var(--color-primary)]">
                        <Layers className="h-4 w-4" />
                      </div>
                      <div>
                        <div className="text-sm font-medium text-[var(--color-text-primary)]">{row.name}</div>
                        <div className="text-xs text-[var(--color-text-secondary)]">{row.outputType.toUpperCase()}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-5 py-3.5">
                    <div className="inline-flex items-center px-2 py-1 rounded bg-[var(--color-bg-secondary)] border border-[var(--color-border)] text-xs font-mono">
                      {row.modelName}
                    </div>
                  </td>
                  <td className="px-5 py-3.5 text-center">
                    {row.isEnabled ? (
                      <span className="inline-flex items-center gap-1 text-green-500 text-xs font-medium"><CheckCircle className="h-3.5 w-3.5" /> 已启用</span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-[var(--color-text-secondary)] text-xs"><XCircle className="h-3.5 w-3.5" /> 已停用</span>
                    )}
                  </td>
                  <td className="px-5 py-3.5 text-right">
                    <button 
                      onClick={() => handleToggleStatus(row)}
                      disabled={updateM.isPending}
                      className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium border rounded-md transition-all ${row.isEnabled ? "text-red-500 hover:bg-red-500/10 border-red-500/20" : "text-green-500 hover:bg-green-500/10 border-green-500/20"} disabled:opacity-50`}>
                      {row.isEnabled ? "停用服务" : "启用服务"}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">共 <span className="font-medium">{data.pagination.total}</span> 个技能</p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}
    </div>
  );
}
