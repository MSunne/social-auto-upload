"use client";

import { useState } from "react";
import { useAudits } from "@/lib/hooks/useAudits";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, ShieldAlert, User, Settings, CreditCard, Monitor, Cpu } from "lucide-react";

const RESOURCE_TYPES = [
  { value: "", label: "全部", icon: null },
  { value: "user", label: "用户", icon: User },
  { value: "device", label: "设备", icon: Monitor },
  { value: "media_account", label: "媒体账号", icon: null },
  { value: "ai_job", label: "AI 作业", icon: Cpu },
  { value: "billing", label: "账单", icon: CreditCard },
  { value: "admin", label: "管理员", icon: Settings },
];

const STATUS_META: Record<string, { label: string; className: string }> = {
  pending: { label: "待处理", className: "text-amber-600" },
  queued: { label: "排队中", className: "text-amber-600" },
  running: { label: "执行中", className: "text-sky-600" },
  cancel_requested: { label: "取消中", className: "text-amber-600" },
  cancelled: { label: "已取消", className: "text-slate-500" },
  needs_verify: { label: "待验证", className: "text-orange-600" },
  success: { label: "成功", className: "text-green-600 font-medium" },
  completed: { label: "已完成", className: "text-green-600 font-medium" },
  failed: { label: "失败", className: "text-red-600 font-medium" },
};

const getStatusBadge = (status: string, prefix?: string) => {
  const meta = STATUS_META[status] || { label: status || "未知", className: "text-[var(--color-text-secondary)]" };
  const content = prefix ? `${prefix}：${meta.label}` : meta.label;
  return <span className={`text-xs ${meta.className}`}>● {content}</span>;
};

const getActorBadge = (row: { actorType?: string; admin?: { name: string } }) => {
  if (row.admin) {
    return (
      <div className="flex items-center gap-1.5">
        <span className="px-1.5 py-0.5 text-xs rounded bg-purple-500/10 text-purple-400 border border-purple-500/20 font-medium">管理员</span>
        <span className="text-sm">{row.admin.name}</span>
      </div>
    );
  }
  return (
    <div className="flex items-center gap-1.5">
      <span className="px-1.5 py-0.5 text-xs rounded bg-blue-500/10 text-blue-400 border border-blue-500/20 font-medium">系统</span>
    </div>
  );
};

const getResourceIcon = (type: string) => {
  const map: Record<string, string> = {
    user: "👤",
    device: "🖥️",
    media_account: "📱",
    ai_job: "🤖",
    billing: "💳",
    admin: "⚙️",
    publish_task: "📤",
    skill: "🔧",
    pricing: "💰",
  };
  return map[type] || "📄";
};

export function AuditsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [resourceType, setResourceType] = useState("");

  const { data, isLoading, error } = useAudits({ page, pageSize: 30, query: query || undefined, resourceType: resourceType || undefined });

  const handleSearch = (e: React.FormEvent) => { e.preventDefault(); setQuery(searchInput); setPage(1); };

  return (
    <div className="space-y-6">
      <PageHeader title="审计日志" subtitle="查看系统内所有管理员操作与自动化行为的完整审计追踪记录。" />

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center bg-[var(--color-bg-secondary)] p-4 rounded-xl border border-[var(--color-border)]">
        <form onSubmit={handleSearch} className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input type="text" placeholder="搜索操作标题 / 用户 / 资源 ID..." value={searchInput} onChange={e => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]" />
        </form>
        <div className="flex gap-1 flex-wrap">
          {RESOURCE_TYPES.map(rt => (
            <button key={rt.value} onClick={() => { setResourceType(rt.value); setPage(1); }}
              className={`px-3 py-1.5 text-xs rounded-lg border transition-colors ${resourceType === rt.value ? "bg-[var(--color-primary)]/10 border-[var(--color-primary)]/50 text-[var(--color-primary)]" : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)]"}`}>
              {rt.label}
            </button>
          ))}
        </div>
      </div>

      {/* Timeline / Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-5 py-3.5 font-medium">时间</th>
                <th className="px-5 py-3.5 font-medium">操作内容</th>
                <th className="px-5 py-3.5 font-medium">归属用户</th>
                <th className="px-5 py-3.5 font-medium">执行方</th>
                <th className="px-5 py-3.5 font-medium">资源</th>
                <th className="px-5 py-3.5 font-medium">来源</th>
                <th className="px-5 py-3.5 font-medium">结果 / 当前状态</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr><td colSpan={7} className="px-6 py-12 text-center">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载审计日志中...</p>
                </td></tr>
              )}
              {error && <tr><td colSpan={7} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td></tr>}
              {data && data.items.length === 0 && <tr><td colSpan={7} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">暂无审计记录</td></tr>}
              {data && data.items.map(row => (
                <tr key={row.id} className="hover:bg-[var(--color-bg-secondary)]/50 transition-colors">
                  <td className="px-5 py-3.5 text-xs text-[var(--color-text-secondary)] whitespace-nowrap">
                    <div>{new Date(row.createdAt).toLocaleDateString("zh-CN")}</div>
                    <div className="mt-0.5 font-mono">{new Date(row.createdAt).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", second: "2-digit" })}</div>
                  </td>
                  <td className="px-5 py-3.5 max-w-[220px]">
                    <div className="font-medium">{row.title}</div>
                    {row.message && (
                      <div className="text-xs text-[var(--color-text-secondary)] mt-0.5 truncate max-w-[200px]" title={row.message}>{row.message}</div>
                    )}
                    <div className="text-xs font-mono text-[var(--color-text-secondary)] mt-0.5 opacity-60">{row.action}</div>
                  </td>
                  <td className="px-5 py-3.5">
                    {row.ownerUser ? (
                      <><div className="text-sm">{row.ownerUser.name}</div><div className="text-xs text-[var(--color-text-secondary)]">{row.ownerUser.email}</div></>
                    ) : <span className="text-xs text-[var(--color-text-secondary)] font-mono">{row.ownerUserId.slice(0, 8)}…</span>}
                  </td>
                  <td className="px-5 py-3.5">{getActorBadge(row)}</td>
                  <td className="px-5 py-3.5">
                    <div className="flex items-center gap-2">
                      <span className="text-base">{getResourceIcon(row.resourceType)}</span>
                      <div>
                        <span className="text-xs px-1.5 py-0.5 rounded bg-[var(--color-bg-secondary)] border border-[var(--color-border)]">{row.resourceType}</span>
                        {row.resourceId && <div className="text-xs font-mono text-[var(--color-text-secondary)] mt-0.5">{row.resourceId.slice(0, 12)}…</div>}
                      </div>
                    </div>
                  </td>
                  <td className="px-5 py-3.5">
                    <span className={`px-2 py-0.5 text-xs rounded border ${row.source === "admin" ? "bg-purple-500/10 text-purple-400 border-purple-500/20" : "bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border-[var(--color-border)]"}`}>
                      {row.source}
                    </span>
                  </td>
                  <td className="px-5 py-3.5">
                    <div className="space-y-1">
                      <div>{getStatusBadge(row.status, "事件")}</div>
                      {row.currentStatus && row.currentStatus !== row.status ? (
                        <div>{getStatusBadge(row.currentStatus, "当前")}</div>
                      ) : null}
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
          <p className="text-sm text-[var(--color-text-secondary)]">共 <span className="font-medium">{data.pagination.total}</span> 条审计记录</p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}

      {/* Security Notice */}
      <div className="flex items-start gap-3 p-4 rounded-xl bg-amber-500/5 border border-amber-500/20">
        <ShieldAlert className="h-5 w-5 text-amber-600 flex-shrink-0 mt-0.5" />
        <div>
          <p className="text-sm font-medium text-amber-700">安全审计说明</p>
          <p className="text-xs text-[var(--color-text-secondary)] mt-1">审计日志为只读记录，包含所有管理员操作和系统自动行为。每条记录包含操作时间、执行方、目标资源和操作结果，不可修改或删除。</p>
        </div>
      </div>
    </div>
  );
}
