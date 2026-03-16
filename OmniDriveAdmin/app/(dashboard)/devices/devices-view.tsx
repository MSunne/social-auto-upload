"use client";

import { useState } from "react";
import { useDevices, useBulkActionDevices } from "@/lib/hooks/useDevices";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, Monitor, Wifi, WifiOff, PowerOff, Power } from "lucide-react";

const formatLastSeen = (ts?: string) => {
  if (!ts) return "从未在线";
  const diff = Date.now() - new Date(ts).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 2) return "刚刚在线";
  if (mins < 60) return `${mins} 分钟前`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs} 小时前`;
  return `${Math.floor(hrs / 24)} 天前`;
};

export function DevicesView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const { data, isLoading, error } = useDevices({ page, pageSize: 20, query: query || undefined, status: status || undefined });
  const bulkAction = useBulkActionDevices();

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
    const allIds = data.data.map(r => r.device.id);
    setSelected(selected.size === allIds.length ? new Set() : new Set(allIds));
  };

  const handleBulkAction = async (action: "disable" | "enable") => {
    if (selected.size === 0) return;
    const label = action === "disable" ? "停用" : "启用";
    if (!confirm(`确认对 ${selected.size} 台设备执行「${label}」操作？`)) return;
    try {
      await bulkAction.mutateAsync({ ids: Array.from(selected), action });
      setSelected(new Set());
    } catch {
      alert("操作失败，请重试");
    }
  };

  const getStatusIndicator = (status: string, isEnabled: boolean) => {
    if (!isEnabled) return <span className="flex items-center gap-1.5 text-xs text-[var(--color-text-secondary)]"><PowerOff className="h-3.5 w-3.5" /> 已停用</span>;
    if (status === "online") return <span className="flex items-center gap-1.5 text-xs text-green-400"><Wifi className="h-3.5 w-3.5" /> 在线</span>;
    return <span className="flex items-center gap-1.5 text-xs text-[var(--color-text-secondary)]"><WifiOff className="h-3.5 w-3.5" /> 离线</span>;
  };

  return (
    <div className="space-y-6">
      <PageHeader title="设备管理" subtitle="监控和管理全平台注册设备的在线状态、任务负载与账户分布。" />

      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center bg-[var(--color-bg-secondary)] p-4 rounded-xl border border-[var(--color-border)]">
        <form onSubmit={handleSearch} className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            placeholder="搜索设备名称 / DeviceCode / IP..."
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
          <option value="">全部设备</option>
          <option value="online">在线</option>
          <option value="offline">离线</option>
        </select>

        {selected.size > 0 && (
          <div className="flex items-center gap-2 ml-auto">
            <span className="text-sm text-[var(--color-text-secondary)]">已选 {selected.size} 台</span>
            <button
              onClick={() => handleBulkAction("enable")}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-green-400 border border-green-500/30 rounded-lg hover:bg-green-500/10 transition-colors"
            >
              <Power className="h-3.5 w-3.5" /> 批量启用
            </button>
            <button
              onClick={() => handleBulkAction("disable")}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/10 transition-colors"
            >
              <PowerOff className="h-3.5 w-3.5" /> 批量停用
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
                  <input type="checkbox" className="rounded"
                    checked={data ? selected.size === data.data.length && data.data.length > 0 : false}
                    onChange={toggleSelectAll} />
                </th>
                <th className="px-4 py-4 font-medium">设备信息</th>
                <th className="px-4 py-4 font-medium">归属用户</th>
                <th className="px-4 py-4 font-medium">状态</th>
                <th className="px-4 py-4 font-medium">账号 / 任务负载</th>
                <th className="px-4 py-4 font-medium">IP 地址</th>
                <th className="px-4 py-4 font-medium">最后在线</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr>
                  <td colSpan={7} className="px-6 py-12 text-center">
                    <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                    <p className="mt-2 text-[var(--color-text-secondary)] text-sm">加载设备数据中...</p>
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
                  <td colSpan={7} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">未找到设备</td>
                </tr>
              )}
              {data && data.data.map(row => (
                <tr
                  key={row.device.id}
                  className={`hover:bg-[var(--color-bg-secondary)]/50 transition-colors ${selected.has(row.device.id) ? "bg-[var(--color-primary)]/5" : ""}`}
                >
                  <td className="px-4 py-4">
                    <input type="checkbox" className="rounded"
                      checked={selected.has(row.device.id)}
                      onChange={() => toggleSelect(row.device.id)} />
                  </td>
                  <td className="px-4 py-4">
                    <div className="flex items-center gap-3">
                      <div className={`h-8 w-8 rounded-lg flex items-center justify-center flex-shrink-0 ${row.device.status === "online" && row.device.isEnabled ? "bg-green-500/10" : "bg-[var(--color-bg-secondary)]"}`}>
                        <Monitor className={`h-4 w-4 ${row.device.status === "online" && row.device.isEnabled ? "text-green-400" : "text-[var(--color-text-secondary)]"}`} />
                      </div>
                      <div>
                        <div className="font-medium">{row.device.name}</div>
                        <div className="text-xs font-mono text-[var(--color-text-secondary)] mt-0.5">{row.device.deviceCode}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-4">
                    {row.owner ? (
                      <div>
                        <div className="text-sm">{row.owner.name}</div>
                        <div className="text-xs text-[var(--color-text-secondary)]">{row.owner.email}</div>
                      </div>
                    ) : (
                      <span className="text-xs text-[var(--color-text-secondary)]">— 未绑定</span>
                    )}
                  </td>
                  <td className="px-4 py-4">{getStatusIndicator(row.device.status, row.device.isEnabled)}</td>
                  <td className="px-4 py-4">
                    <div className="text-sm">
                      {row.device.load.activeAccountCount}/{row.device.load.accountCount} 账号
                    </div>
                    <div className="flex gap-2 text-xs text-[var(--color-text-secondary)] mt-0.5">
                      <span className="text-amber-400">{row.device.load.runningTaskCount} 运行</span>
                      <span>{row.device.load.pendingTaskCount} 等待</span>
                      {row.device.load.failedTaskCount > 0 && (
                        <span className="text-red-400">{row.device.load.failedTaskCount} 失败</span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-4 font-mono text-xs text-[var(--color-text-secondary)]">
                    <div>{row.device.localIp || "—"}</div>
                    <div className="mt-0.5">{row.device.publicIp || ""}</div>
                  </td>
                  <td className="px-4 py-4 text-xs text-[var(--color-text-secondary)]">
                    {formatLastSeen(row.device.lastSeenAt)}
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
            共 <span className="font-medium">{data.pagination.total}</span> 台设备
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
