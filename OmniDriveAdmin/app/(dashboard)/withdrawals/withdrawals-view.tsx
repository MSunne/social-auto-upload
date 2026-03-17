"use client";

import { useState } from "react";
import { useWithdrawals } from "@/lib/hooks/useWithdrawals";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, RefreshCw, Eye } from "lucide-react";
import { WithdrawalDrawer } from "./withdrawal-drawer";

const STATUS_TABS = [
  { id: "", label: "全部申请" },
  { id: "pending_review", label: "待审核", countProp: "pending" },
  { id: "approved", label: "待发款" },
  { id: "paid", label: "已发款" },
  { id: "rejected", label: "已驳回" },
];

export function WithdrawalsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [statusParam, setStatusParam] = useState("");
  
  const [selectedWithdrawalId, setSelectedWithdrawalId] = useState<string | null>(null);

  const { data, isLoading, error, refetch } = useWithdrawals({ 
    page, 
    pageSize: 30, 
    query: query || undefined, 
    status: statusParam || undefined 
  });

  const handleSearch = (e: React.FormEvent) => { e.preventDefault(); setQuery(searchInput); setPage(1); };

  const renderStatus = (status: string) => {
    switch (status) {
      case "pending_review": return <span className="px-2 py-0.5 text-xs font-medium rounded bg-orange-500/10 text-orange-400 border border-orange-500/20">待审核</span>;
      case "approved": return <span className="px-2 py-0.5 text-xs font-medium rounded bg-blue-500/10 text-blue-400 border border-blue-500/20">待打款</span>;
      case "rejected": return <span className="px-2 py-0.5 text-xs font-medium rounded bg-red-500/10 text-red-400 border border-red-500/20">已驳回</span>;
      case "paid": return <span className="px-2 py-0.5 text-xs font-medium rounded bg-green-500/10 text-green-400 border border-green-500/20">已打款</span>;
      default: return <span className="text-xs text-[var(--color-text-secondary)]">{status}</span>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="分销提现申请" subtitle="审核分销用户的提现请求，并进行资金打款发放管理。" />
        <button onClick={() => refetch()} className="flex items-center gap-2 px-3 py-2 border border-[var(--color-border)] rounded-lg text-sm hover:bg-[var(--color-bg-secondary)] transition-colors">
          <RefreshCw className="h-4 w-4" /> 刷新
        </button>
      </div>

      {data && data.summary && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">待审提现总额 (含待打款)</p>
            <p className="text-2xl font-bold text-orange-400">¥ {(data.summary.pendingWithdrawalAmountCents / 100).toFixed(2)}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">历史累计已发款</p>
            <p className="text-2xl font-bold text-green-400">¥ {(data.summary.paidWithdrawalAmountCents / 100).toFixed(2)}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">总提现单数</p>
            <p className="text-2xl font-bold">{data.summary.totalRecords}</p>
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3 justify-between items-start sm:items-center">
        <div className="flex gap-1 bg-[var(--color-bg-secondary)] p-1 rounded-lg border border-[var(--color-border)] overflow-x-auto w-full sm:w-auto">
          {STATUS_TABS.map(tab => (
            <button key={tab.id} onClick={() => { setStatusParam(tab.id); setPage(1); }}
              className={`px-4 py-1.5 text-sm rounded-md whitespace-nowrap transition-colors ${statusParam === tab.id ? "bg-[var(--color-bg-primary)] text-[var(--color-text-primary)] shadow-sm border border-[var(--color-border)] opacity-100" : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-primary)]/50 border border-transparent opacity-80"}`}>
              {tab.label}
            </button>
          ))}
        </div>
        <form onSubmit={handleSearch} className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input type="text" placeholder="搜索提现单号 / 用户..." value={searchInput} onChange={e => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors" />
        </form>
      </div>

      {/* Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-5 py-3.5 font-medium">单号 / 时间</th>
                <th className="px-5 py-3.5 font-medium">推广员</th>
                <th className="px-5 py-3.5 font-medium">提现额度</th>
                <th className="px-5 py-3.5 font-medium">收款账户</th>
                <th className="px-5 py-3.5 font-medium">工单状态</th>
                <th className="px-5 py-3.5 font-medium text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr><td colSpan={6} className="px-6 py-12 text-center">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载提现列表...</p>
                </td></tr>
              )}
              {error && <tr><td colSpan={6} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td></tr>}
              {data && data.items.length === 0 && <tr><td colSpan={6} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">暂无符合条件的提现申请</td></tr>}
              {data && data.items.map(row => (
                <tr key={row.id} className="hover:bg-[var(--color-bg-secondary)]/50 transition-colors">
                  <td className="px-5 py-3.5 whitespace-nowrap">
                    <div className="font-mono text-xs text-[var(--color-text-secondary)] mb-1">{row.id}</div>
                    <div className="text-xs">{new Date(row.requestedAt).toLocaleString("zh-CN")}</div>
                  </td>
                  <td className="px-5 py-3.5">
                    <div className="text-sm font-medium">{row.promoter.name}</div>
                    <div className="text-xs text-[var(--color-text-secondary)]">{row.promoter.email}</div>
                  </td>
                  <td className="px-5 py-3.5 font-medium text-[var(--color-primary)] text-lg">
                    ¥ {(row.amountCents / 100).toFixed(2)}
                  </td>
                  <td className="px-5 py-3.5">
                    <div className="text-xs text-[var(--color-text-secondary)] mb-0.5">{row.payoutChannel || "线下转账"}</div>
                    <div className="font-mono text-xs max-w-[160px] truncate">{row.accountMasked}</div>
                  </td>
                  <td className="px-5 py-3.5">{renderStatus(row.status)}</td>
                  <td className="px-5 py-3.5 text-right">
                    <button onClick={() => setSelectedWithdrawalId(row.id)} className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-[var(--color-primary)]/10 text-[var(--color-primary)] hover:bg-[var(--color-primary)]/20 border border-[var(--color-primary)]/20 rounded-md transition-colors">
                      <Eye className="h-3.5 w-3.5" /> 处理单据
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
          <p className="text-sm text-[var(--color-text-secondary)]">共 <span className="font-medium">{data.pagination.total}</span> 条申请</p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}

      <WithdrawalDrawer 
        isOpen={selectedWithdrawalId !== null} 
        onClose={() => setSelectedWithdrawalId(null)} 
        withdrawalId={selectedWithdrawalId} 
        onSuccess={() => refetch()} 
      />
    </div>
  );
}
