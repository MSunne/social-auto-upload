"use client";

import { useState } from "react";
import { useMediaAccounts, useBulkActionMediaAccounts, useValidateMediaAccount } from "@/lib/hooks/useMediaAccounts";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, RefreshCw, ShieldCheck, Trash2 } from "lucide-react";

const PLATFORMS = ["", "douyin", "bilibili", "xiaohongshu", "kuaishou", "wechat_channel", "baijiahao", "tiktok"];

const PLATFORM_LABELS: Record<string, string> = {
  "": "全部",
  douyin: "抖音",
  bilibili: "Bilibili",
  xiaohongshu: "小红书",
  kuaishou: "快手",
  wechat_channel: "视频号",
  baijiahao: "百家号",
  tiktok: "TikTok",
};

const getStatusBadge = (status: string) => {
  const map: Record<string, string> = {
    active: "bg-green-500/10 text-green-400 border-green-500/20",
    inactive: "bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border-[var(--color-border)]",
    needs_login: "bg-orange-500/10 text-orange-400 border-orange-500/20",
    locked: "bg-red-500/10 text-red-400 border-red-500/20",
    banned: "bg-red-700/10 text-red-500 border-red-700/20",
  };
  const labels: Record<string, string> = {
    active: "正常", inactive: "未激活", needs_login: "需重登", locked: "已锁定", banned: "已封号",
  };
  const cls = map[status] ?? "bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border-[var(--color-border)]";
  return <span className={`px-2 py-0.5 text-xs rounded-full border font-medium ${cls}`}>{labels[status] || status}</span>;
};

const getPlatformColor = (platform: string) => {
  const map: Record<string, string> = {
    douyin: "bg-black text-white",
    bilibili: "bg-[#00A1D6]/10 text-[#00A1D6] border border-[#00A1D6]/20",
    xiaohongshu: "bg-red-500/10 text-red-400 border border-red-500/20",
    kuaishou: "bg-yellow-500/10 text-yellow-400 border border-yellow-500/20",
    wechat_channel: "bg-green-600/10 text-green-400 border border-green-600/20",
    tiktok: "bg-[#010101]/10 text-[var(--color-text-secondary)] border border-[var(--color-border)]",
    baijiahao: "bg-blue-500/10 text-blue-400 border border-blue-500/20",
  };
  return map[platform] ?? "bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border border-[var(--color-border)]";
};

export function MediaAccountsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [platform, setPlatform] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const { data, isLoading, error, refetch } = useMediaAccounts({ page, pageSize: 20, query: query || undefined, platform: platform || undefined });
  const bulkAction = useBulkActionMediaAccounts();
  const validateAccount = useValidateMediaAccount();

  const handleSearch = (e: React.FormEvent) => { e.preventDefault(); setQuery(searchInput); setPage(1); setSelected(new Set()); };
  const toggleSelect = (id: string) => setSelected(prev => { const n = new Set(prev); if (n.has(id)) { n.delete(id); } else { n.add(id); } return n; });
  const toggleSelectAll = () => {
    if (!data || !data.items) return;
    const all = data.items.map(r => r.account.id);
    setSelected(selected.size === all.length ? new Set() : new Set(all));
  };

  const handleBulkDelete = async () => {
    if (selected.size === 0) return;
    if (!confirm(`确认永久删除 ${selected.size} 个媒体账号？此操作不可撤销！`)) return;
    try { await bulkAction.mutateAsync({ ids: Array.from(selected), action: "delete" }); setSelected(new Set()); }
    catch { alert("操作失败，请重试"); }
  };

  const handleValidate = async (id: string) => {
    try { await validateAccount.mutateAsync(id); }
    catch { alert("验证失败，请重试"); }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="媒体账号管理" subtitle="查看平台账号状态、任务负载，并执行批量运维操作。" />
        <button onClick={() => refetch()} className="flex items-center gap-2 px-3 py-2 border border-[var(--color-border)] rounded-lg text-sm hover:bg-[var(--color-bg-secondary)] transition-colors">
          <RefreshCw className="h-4 w-4" /> 刷新
        </button>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center bg-[var(--color-bg-secondary)] p-4 rounded-xl border border-[var(--color-border)]">
        <form onSubmit={handleSearch} className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input type="text" placeholder="搜索账号名称 / 用户 / 设备..." value={searchInput} onChange={e => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]" />
        </form>
        <div className="flex gap-1 flex-wrap">
          {PLATFORMS.map(p => (
            <button key={p} onClick={() => { setPlatform(p); setPage(1); setSelected(new Set()); }}
              className={`px-3 py-1.5 text-xs rounded-lg border transition-colors ${platform === p ? "bg-[var(--color-primary)]/10 border-[var(--color-primary)]/50 text-[var(--color-primary)]" : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)]"}`}>
              {PLATFORM_LABELS[p]}
            </button>
          ))}
        </div>
        {selected.size > 0 && (
          <div className="flex items-center gap-2 ml-auto">
            <span className="text-xs text-[var(--color-text-secondary)]">已选 {selected.size}</span>
            <button
              onClick={handleBulkDelete}
              className="flex items-center gap-1 px-2.5 py-1.5 text-xs text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/10 transition-colors"
            >
              <Trash2 className="h-3 w-3" /> 批量删除
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
                <th className="px-4 py-3.5">
                  <input type="checkbox" className="rounded" checked={data ? selected.size === data.items.length && data.items.length > 0 : false} onChange={toggleSelectAll} />
                </th>
                <th className="px-4 py-3.5 font-medium">账号信息</th>
                <th className="px-4 py-3.5 font-medium">归属用户</th>
                <th className="px-4 py-3.5 font-medium">运行设备</th>
                <th className="px-4 py-3.5 font-medium">状态</th>
                <th className="px-4 py-3.5 font-medium">任务负载</th>
                <th className="px-4 py-3.5 font-medium">最近验证</th>
                <th className="px-4 py-3.5 font-medium text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr><td colSpan={8} className="px-6 py-12 text-center">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载账号数据中...</p>
                </td></tr>
              )}
              {error && <tr><td colSpan={8} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td></tr>}
              {data && data.items.length === 0 && <tr><td colSpan={8} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">未找到媒体账号</td></tr>}
              {data && data.items.map(row => (
                <tr key={row.account.id} className={`hover:bg-[var(--color-bg-secondary)]/50 transition-colors ${selected.has(row.account.id) ? "bg-[var(--color-primary)]/5" : ""}`}>
                  <td className="px-4 py-3.5">
                    <input type="checkbox" className="rounded" checked={selected.has(row.account.id)} onChange={() => toggleSelect(row.account.id)} />
                  </td>
                  <td className="px-4 py-3.5">
                    <div className="flex items-center gap-2.5">
                      <span className={`px-2 py-0.5 text-xs rounded font-medium ${getPlatformColor(row.account.platform)}`}>
                        {PLATFORM_LABELS[row.account.platform] || row.account.platform}
                      </span>
                      <div>
                        <div className="font-medium text-[var(--color-text-primary)]">{row.account.accountName}</div>
                        {row.account.lastMessage && (
                          <div className="text-xs text-[var(--color-text-secondary)] mt-0.5 max-w-[160px] truncate" title={row.account.lastMessage}>
                            {row.account.lastMessage}
                          </div>
                        )}
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3.5">
                    {row.owner ? (
                      <><div className="text-sm">{row.owner.name}</div><div className="text-xs text-[var(--color-text-secondary)]">{row.owner.email}</div></>
                    ) : <span className="text-xs text-[var(--color-text-secondary)]">—</span>}
                  </td>
                  <td className="px-4 py-3.5">
                    <div className="text-sm font-medium">{row.device.name}</div>
                    <div className="text-xs font-mono text-[var(--color-text-secondary)]">{row.device.deviceCode}</div>
                  </td>
                  <td className="px-4 py-3.5">{getStatusBadge(row.account.status)}</td>
                  <td className="px-4 py-3.5">
                    <div className="text-xs space-y-0.5">
                      <div className="flex gap-2">
                        {row.account.load.runningTaskCount > 0 && <span className="text-blue-400">{row.account.load.runningTaskCount} 运行</span>}
                        {row.account.load.pendingTaskCount > 0 && <span className="text-[var(--color-text-secondary)]">{row.account.load.pendingTaskCount} 等待</span>}
                        {row.account.load.needsVerifyTaskCount > 0 && <span className="text-purple-400">{row.account.load.needsVerifyTaskCount} 待核</span>}
                        {row.account.load.failedTaskCount > 0 && <span className="text-red-400">{row.account.load.failedTaskCount} 失败</span>}
                        {row.account.load.taskCount === 0 && <span className="text-[var(--color-text-secondary)]">无任务</span>}
                      </div>
                      {row.account.load.activeLoginSessionCount > 0 && (
                        <div className="text-orange-400">{row.account.load.activeLoginSessionCount} 登录会话</div>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3.5 text-xs text-[var(--color-text-secondary)]">
                    {row.account.lastAuthenticatedAt
                      ? new Date(row.account.lastAuthenticatedAt).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
                      : "从未"}
                  </td>
                  <td className="px-4 py-3.5 text-right">
                    {row.actions.canValidate && (
                      <button
                        onClick={() => handleValidate(row.account.id)}
                        disabled={validateAccount.isPending}
                        className="inline-flex items-center gap-1 px-2.5 py-1.5 text-xs text-[var(--color-primary)] border border-[var(--color-primary)]/30 rounded-lg hover:bg-[var(--color-primary)]/10 transition-colors disabled:opacity-50"
                      >
                        <ShieldCheck className="h-3 w-3" /> 验证
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">共 <span className="font-medium">{data.pagination.total}</span> 个媒体账号</p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}
    </div>
  );
}
