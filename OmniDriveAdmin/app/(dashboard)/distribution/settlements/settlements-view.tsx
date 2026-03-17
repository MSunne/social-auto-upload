"use client";

import { useState } from "react";
import { useDistributionSettlements, useCreateSettlement } from "@/lib/hooks/useDistribution";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, RefreshCw, Layers, CheckCircle2, Factory } from "lucide-react";

export function SettlementsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");

  const { data, isLoading, error, refetch } = useDistributionSettlements({ 
    page, 
    pageSize: 30, 
    query: query || undefined 
  });
  
  const createM = useCreateSettlement();

  const handleSearch = (e: React.FormEvent) => { e.preventDefault(); setQuery(searchInput); setPage(1); };

  const handleCreateSettlement = async () => {
    if (!confirm("是否确认生成新的结算批次？系统将找出所有达到打款门槛且待结算的佣金，生成结算单并划转入提现池。")) return;
    try {
      await createM.mutateAsync({});
      refetch();
    } catch (err: unknown) {
      alert("创建结算批次失败，请检查是否有符合条件的佣金流水或系统异常。");
    }
  };

  const renderStatus = (status: string) => {
    switch (status) {
      case "pending": return <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-orange-500/10 text-orange-400 border border-orange-500/20">结算处理中</span>;
      case "completed": return <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-green-500/10 text-green-400 border border-green-500/20 flex items-center gap-1 w-fit"><CheckCircle2 className="h-3 w-3" /> 已完成归集</span>;
      case "failed": return <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-red-500/10 text-red-500 border border-red-500/20">结算异常</span>;
      default: return <span className="text-xs text-[var(--color-text-secondary)]">{status}</span>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="佣金周期结算" subtitle="对达到提现门槛的推广员佣金进行周期性批量结算，划转至可提现余额池。" />
        <div className="flex items-center gap-2">
          <button onClick={() => refetch()} className="flex items-center gap-2 px-3 py-2 border border-[var(--color-border)] rounded-lg text-sm hover:bg-[var(--color-bg-secondary)] transition-colors">
            <RefreshCw className="h-4 w-4" /> <span className="hidden sm:inline">刷新</span>
          </button>
          <button onClick={handleCreateSettlement} disabled={createM.isPending} className="flex items-center gap-2 px-3 py-2 bg-[var(--color-primary)] text-white rounded-lg text-sm font-medium hover:brightness-110 transition-colors disabled:opacity-50">
            {createM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Factory className="h-4 w-4" />} 
            执行月度/周期结算
          </button>
        </div>
      </div>

      {/* Summary Stats */}
      {data && data.summary && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1 flex items-center gap-1"><Layers className="h-3.5 w-3.5" /> 累计生成批次</p>
            <p className="text-xl font-bold">{data.summary.totalBatchCount} <span className="text-sm font-normal text-[var(--color-text-secondary)]">批</span></p>
          </div>
          <div className="p-4 rounded-xl border border-green-500/30 bg-green-500/5">
            <p className="text-xs text-green-400 mb-1">历史结算总额</p>
            <p className="text-xl font-bold text-green-400">¥ {(data.summary.totalAmountCents / 100).toFixed(2)}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">成功完成批次</p>
            <p className="text-xl font-medium text-[var(--color-text-primary)]">{data.summary.completedBatchCount}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">待理/异常批次</p>
            <p className="text-xl font-medium text-orange-400">{data.summary.pendingBatchCount}</p>
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="flex justify-end">
        <form onSubmit={handleSearch} className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input type="text" placeholder="搜索 结算批次号..." value={searchInput} onChange={e => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors" />
        </form>
      </div>

      {/* Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-5 py-3.5 font-medium">结算处理时间</th>
                <th className="px-5 py-3.5 font-medium">批次单号</th>
                <th className="px-5 py-3.5 font-medium text-right">包含明细条数</th>
                <th className="px-5 py-3.5 font-medium text-right">结算总金额</th>
                <th className="px-5 py-3.5 font-medium">批次状态</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr><td colSpan={5} className="px-6 py-12 text-center">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载结算批次...</p>
                </td></tr>
              )}
              {error && <tr><td colSpan={5} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td></tr>}
              {data && data.items.length === 0 && <tr><td colSpan={5} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">暂无结算记录</td></tr>}
              {data && data.items.map(row => (
                <tr key={row.id} className="hover:bg-[var(--color-bg-secondary)]/50 transition-colors">
                  <td className="px-5 py-3.5">
                    <div className="text-sm font-medium">{new Date(row.createdAt).toLocaleDateString("zh-CN")}</div>
                    <div className="text-xs text-[var(--color-text-secondary)]">{new Date(row.createdAt).toLocaleTimeString("zh-CN")}</div>
                  </td>
                  <td className="px-5 py-3.5">
                    <div className="font-mono text-xs">{row.batchNo}</div>
                    {row.notes && <div className="text-[10px] text-[var(--color-text-secondary)] truncate max-w-[200px] mt-1" title={row.notes}>{row.notes}</div>}
                  </td>
                  <td className="px-5 py-3.5 text-right font-medium text-[var(--color-text-primary)]">
                    {row.itemCount} <span className="text-xs font-normal text-[var(--color-text-secondary)]">笔</span>
                  </td>
                  <td className="px-5 py-3.5 text-right font-bold text-[var(--color-primary)] text-lg">
                    ¥ {(row.totalAmountCents / 100).toFixed(2)}
                  </td>
                  <td className="px-5 py-3.5">{renderStatus(row.status)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">共 <span className="font-medium">{data.pagination.total}</span> 个结算批次</p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}
    </div>
  );
}
