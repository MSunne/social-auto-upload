"use client";

import { useState } from "react";
import { useUsers, useBulkActionUsers } from "@/lib/hooks/useUsers";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, ShieldOff, ShieldCheck, UserCircle } from "lucide-react";

export function UsersView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const { data, isLoading, error } = useUsers({ page, pageSize: 20, query: query || undefined, status: status || undefined });
  const bulkAction = useBulkActionUsers();

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setQuery(searchInput);
    setPage(1);
    setSelected(new Set());
  };

  const toggleSelect = (id: string) => {
    setSelected(prev => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (!data) return;
    const allIds = data.data.map(r => r.user.id);
    if (selected.size === allIds.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(allIds));
    }
  };

  const handleBulkAction = async (action: "deactivate" | "activate") => {
    if (selected.size === 0) return;
    const label = action === "deactivate" ? "封禁" : "解封";
    if (!confirm(`确认对 ${selected.size} 个用户执行「${label}」操作？`)) return;
    try {
      await bulkAction.mutateAsync({ ids: Array.from(selected), action });
      setSelected(new Set());
    } catch {
      alert("操作失败，请重试");
    }
  };

  const getStatusBadge = (isActive: boolean) =>
    isActive
      ? <span className="px-2 py-0.5 text-xs rounded-full font-medium bg-green-500/10 text-green-400 border border-green-500/20">正常</span>
      : <span className="px-2 py-0.5 text-xs rounded-full font-medium bg-red-500/10 text-red-400 border border-red-500/20">已停用</span>;

  return (
    <div className="space-y-6">
      <PageHeader title="用户管理" subtitle="查看全平台注册用户、账户资产概况与账户状态控制。" />

      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center bg-[var(--color-bg-secondary)] p-4 rounded-xl border border-[var(--color-border)]">
        <form onSubmit={handleSearch} className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            placeholder="搜索邮箱 / 用户名..."
            value={searchInput}
            onChange={e => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-all"
          />
        </form>
        <select
          value={status}
          onChange={e => { setStatus(e.target.value); setPage(1); setSelected(new Set()); }}
          className="px-3 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]"
        >
          <option value="">全部用户</option>
          <option value="active">正常用户</option>
          <option value="inactive">已停用</option>
        </select>

        {selected.size > 0 && (
          <div className="flex items-center gap-2 ml-auto">
            <span className="text-sm text-[var(--color-text-secondary)]">已选 {selected.size} 个</span>
            <button
              onClick={() => handleBulkAction("activate")}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-green-400 border border-green-500/30 rounded-lg hover:bg-green-500/10 transition-colors"
            >
              <ShieldCheck className="h-3.5 w-3.5" /> 解封
            </button>
            <button
              onClick={() => handleBulkAction("deactivate")}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/10 transition-colors"
            >
              <ShieldOff className="h-3.5 w-3.5" /> 封禁
            </button>
          </div>
        )}
      </div>

      {/* Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-4 py-4">
                  <input
                    type="checkbox"
                    className="rounded"
                    checked={data ? selected.size === data.data.length && data.data.length > 0 : false}
                    onChange={toggleSelectAll}
                  />
                </th>
                <th className="px-4 py-4 font-medium">用户信息</th>
                <th className="px-4 py-4 font-medium">状态</th>
                <th className="px-4 py-4 font-medium">余额 / 总充值</th>
                <th className="px-4 py-4 font-medium">设备 / 媒体号</th>
                <th className="px-4 py-4 font-medium">任务 / AI作业</th>
                <th className="px-4 py-4 font-medium">注册时间</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr>
                  <td colSpan={7} className="px-6 py-12 text-center">
                    <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                    <p className="mt-2 text-[var(--color-text-secondary)] text-sm">加载用户数据中...</p>
                  </td>
                </tr>
              )}
              {error && (
                <tr>
                  <td colSpan={7} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td>
                </tr>
              )}
              {data && data.data.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">未找到符合条件的用户</td>
                </tr>
              )}
              {data && data.data.map(row => (
                <tr
                  key={row.user.id}
                  className={`hover:bg-[var(--color-bg-secondary)]/50 transition-colors ${selected.has(row.user.id) ? "bg-[var(--color-primary)]/5" : ""}`}
                >
                  <td className="px-4 py-4">
                    <input
                      type="checkbox"
                      className="rounded"
                      checked={selected.has(row.user.id)}
                      onChange={() => toggleSelect(row.user.id)}
                    />
                  </td>
                  <td className="px-4 py-4">
                    <div className="flex items-center gap-3">
                      <div className="h-8 w-8 rounded-full bg-[var(--color-primary)]/10 flex items-center justify-center flex-shrink-0">
                        <UserCircle className="h-5 w-5 text-[var(--color-primary)]" />
                      </div>
                      <div>
                        <div className="font-medium text-[var(--color-text-primary)]">{row.user.name}</div>
                        <div className="text-xs text-[var(--color-text-secondary)] mt-0.5">{row.user.email}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-4">{getStatusBadge(row.user.isActive)}</td>
                  <td className="px-4 py-4">
                    <div className="font-mono text-sm font-medium">{row.billing.creditBalance.toLocaleString()}</div>
                    <div className="text-xs text-[var(--color-text-secondary)] mt-0.5">
                      充 ¥{(row.billing.totalRechargeAmountCents / 100).toFixed(0)}
                    </div>
                  </td>
                  <td className="px-4 py-4">
                    <div className="text-sm">{row.assets.deviceCount} 设备</div>
                    <div className="text-xs text-[var(--color-text-secondary)]">{row.assets.mediaAccountCount} 媒体号</div>
                  </td>
                  <td className="px-4 py-4">
                    <div className="text-sm">{row.assets.publishTaskCount.toLocaleString()} 任务</div>
                    <div className="text-xs text-[var(--color-text-secondary)]">{row.assets.aiJobCount.toLocaleString()} AI作业</div>
                  </td>
                  <td className="px-4 py-4 text-xs text-[var(--color-text-secondary)]">
                    {new Date(row.user.createdAt).toLocaleDateString("zh-CN")}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Pagination */}
      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">
            共 <span className="font-medium">{data.pagination.total}</span> 名用户
          </p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1}
              className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages}
              className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}
    </div>
  );
}
