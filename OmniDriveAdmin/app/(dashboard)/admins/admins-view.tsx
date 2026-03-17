"use client";

import { useState } from "react";
import { useAdmins } from "@/lib/hooks/useAdmins";
import { AdminIdentity } from "@/lib/types";
import { AdminDrawer } from "./admin-drawer";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, Plus, Users, Shield, KeyRound, CheckCircle2, XCircle } from "lucide-react";

export function AdminsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState("");
  const [roleId, setRoleId] = useState("");

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [selectedAdmin, setSelectedAdmin] = useState<AdminIdentity | null>(null);

  const { data, isLoading, error } = useAdmins({ page, pageSize: 20, query: query || undefined, status: status || undefined, roleId: roleId || undefined });

  const handleSearch = (e: React.FormEvent) => { e.preventDefault(); setQuery(searchInput); setPage(1); };

  const openCreate = () => { setSelectedAdmin(null); setDrawerOpen(true); };
  const openEdit = (admin: AdminIdentity) => { setSelectedAdmin(admin); setDrawerOpen(true); };

  const roles = data?.filters?.roles || [];

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="管理员与权限" subtitle="平台管理人员账号清单与 RBAC (基于角色的权限) 分配控制台。" />
        <button onClick={openCreate} className="flex items-center gap-2 px-4 py-2 bg-[var(--color-primary)] text-white rounded-lg text-sm font-medium hover:brightness-110 transition-all shadow-lg shadow-[var(--color-primary)]/20">
          <Plus className="h-4 w-4" />
          新建管理员
        </button>
      </div>

      {/* Summary Stats */}
      {data && data.summary && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] flex items-center gap-4">
            <div className="h-10 w-10 rounded-lg bg-blue-500/10 flex items-center justify-center flex-shrink-0"><Users className="h-5 w-5 text-blue-400" /></div>
            <div><p className="text-xs text-[var(--color-text-secondary)]">激活状态管理员</p><p className="text-lg font-medium">{data.summary.activeAdminCount}</p></div>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] flex items-center gap-4">
            <div className="h-10 w-10 rounded-lg bg-purple-500/10 flex items-center justify-center flex-shrink-0"><Shield className="h-5 w-5 text-purple-400" /></div>
            <div><p className="text-xs text-[var(--color-text-secondary)]">定义的角色数量</p><p className="text-lg font-medium">{data.summary.roleCount}</p></div>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] flex items-center gap-4">
            <div className="h-10 w-10 rounded-lg bg-amber-500/10 flex items-center justify-center flex-shrink-0"><KeyRound className="h-5 w-5 text-amber-400" /></div>
            <div>
              <p className="text-xs text-[var(--color-text-secondary)]">认证驱动</p>
              <p className="text-sm font-medium font-mono uppercase mt-0.5">{data.summary.authMode.replace("_", " ")}</p>
            </div>
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center bg-[var(--color-bg-secondary)] p-4 rounded-xl border border-[var(--color-border)]">
        <form onSubmit={handleSearch} className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input type="text" placeholder="搜索花名 / 邮箱..." value={searchInput} onChange={e => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]" />
        </form>
        <div className="flex gap-2">
          <select value={status} onChange={e => { setStatus(e.target.value); setPage(1); }} className="px-3 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] text-[var(--color-text-secondary)]">
            <option value="">所有状态</option>
            <option value="active">正常允许登录</option>
            <option value="inactive">已被停用</option>
          </select>
          <select value={roleId} onChange={e => { setRoleId(e.target.value); setPage(1); }} className="px-3 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] text-[var(--color-text-secondary)] max-w-[150px] truncate">
            <option value="">所有角色</option>
            {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
          </select>
        </div>
      </div>

      {/* Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-5 py-3.5 font-medium">管理员</th>
                <th className="px-5 py-3.5 font-medium">状态</th>
                <th className="px-5 py-3.5 font-medium">分配角色</th>
                <th className="px-5 py-3.5 font-medium">创建时间</th>
                <th className="px-5 py-3.5 font-medium text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr><td colSpan={5} className="px-6 py-12 text-center">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载管理员数据中...</p>
                </td></tr>
              )}
              {error && <tr><td colSpan={5} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td></tr>}
              {data && data.items.length === 0 && <tr><td colSpan={5} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">暂无符合条件的账号</td></tr>}
              {data && data.items.map(admin => (
                <tr key={admin.id} className="hover:bg-[var(--color-bg-secondary)]/50 transition-colors">
                  <td className="px-5 py-3.5">
                    <div className="flex items-center gap-3">
                      <div className="h-8 w-8 rounded-full bg-[var(--color-primary)]/10 flex items-center justify-center flex-shrink-0 border border-[var(--color-primary)]/20">
                        <span className="font-semibold text-[var(--color-primary)] text-xs">{admin.name.slice(0, 2).toUpperCase()}</span>
                      </div>
                      <div>
                        <div className="font-medium">{admin.name}</div>
                        <div className="text-xs text-[var(--color-text-secondary)] mt-0.5">{admin.email}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-5 py-3.5">
                    {admin.isActive
                      ? <span className="inline-flex items-center gap-1 text-xs text-green-400 font-medium"><CheckCircle2 className="h-3.5 w-3.5" /> 正常</span>
                      : <span className="inline-flex items-center gap-1 text-xs text-red-400 font-medium"><XCircle className="h-3.5 w-3.5" /> 已停用</span>}
                  </td>
                  <td className="px-5 py-3.5">
                    <div className="flex flex-wrap gap-1.5 max-w-[280px]">
                      {admin.roles && admin.roles.length > 0 ? admin.roles.map(r => (
                        <span key={r.id} className={`px-2 py-0.5 text-xs rounded border ${r.isSystem ? "bg-purple-500/10 text-purple-400 border-purple-500/20" : "bg-[var(--color-bg-secondary)] border-[var(--color-border)] text-[var(--color-text-secondary)]"}`}>
                          {r.name}
                        </span>
                      )) : <span className="text-xs text-[var(--color-text-secondary)]">暂无角色</span>}
                    </div>
                  </td>
                  <td className="px-5 py-3.5 text-xs text-[var(--color-text-secondary)]">
                    {new Date(admin.createdAt).toLocaleDateString("zh-CN")}
                  </td>
                  <td className="px-5 py-3.5 text-right">
                    <button onClick={() => openEdit(admin)} className="text-xs font-medium text-[var(--color-primary)] hover:underline">编辑信息</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">共 <span className="font-medium">{data.pagination.total}</span> 名管理员</p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}

      <AdminDrawer isOpen={drawerOpen} onClose={() => setDrawerOpen(false)} admin={selectedAdmin} roles={roles} />
    </div>
  );
}
