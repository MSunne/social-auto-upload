"use client";

import { useState } from "react";
import { useWalletLedgers } from "@/lib/hooks/useFinance";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, ArrowUpRight, ArrowDownLeft, RefreshCw } from "lucide-react";

const ENTRY_TYPES = [
  { value: "", label: "全部类型" },
  { value: "recharge", label: "充值入账", color: "text-green-400" },
  { value: "consume", label: "消耗抵扣", color: "text-orange-400" },
  { value: "refund", label: "售后退款", color: "text-blue-400" },
  { value: "admin_adjustment", label: "人工调账", color: "text-purple-400" },
  { value: "system_adjustment", label: "系统调账", color: "text-[var(--color-text-secondary)]" },
];

export function WalletLedgersView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [entryType, setEntryType] = useState("");

  const { data, isLoading, error, refetch } = useWalletLedgers({ page, pageSize: 30, query: query || undefined, entryType: entryType || undefined });

  const handleSearch = (e: React.FormEvent) => { e.preventDefault(); setQuery(searchInput); setPage(1); };

  const getEntryBadge = (type: string) => {
    const config = ENTRY_TYPES.find(t => t.value === type) || { label: type, color: "text-[var(--color-text-secondary)]" };
    const isIncome = ["recharge", "refund", "admin_adjustment"].includes(type) || type.includes("income");
    const bg = isIncome ? "bg-green-500/10 border-green-500/20" : "bg-orange-500/10 border-orange-500/20";
    return (
      <span className={`px-2 py-0.5 text-xs rounded border font-medium flex items-center gap-1 w-fit ${bg} ${config.color}`}>
        {isIncome ? <ArrowDownLeft className="h-3 w-3" /> : <ArrowUpRight className="h-3 w-3" />}
        {config.label}
      </span>
    );
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="钱包账单流水" subtitle="全局追踪每一次积分的变动、充值与消费行为明细。" />
        <button onClick={() => refetch()} className="flex items-center gap-2 px-3 py-2 border border-[var(--color-border)] rounded-lg text-sm hover:bg-[var(--color-bg-secondary)] transition-colors">
          <RefreshCw className="h-4 w-4" /> 刷新
        </button>
      </div>

      {/* Summary Cards */}
      {data && data.summary && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">总流水笔数</p>
            <p className="text-xl font-medium">{data.summary.totalEntryCount}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">累计入账积分</p>
            <p className="text-xl font-medium text-green-400">+{data.summary.totalCreditIn.toLocaleString()}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">累计消耗积分</p>
            <p className="text-xl font-medium text-orange-400">-{data.summary.totalCreditOut.toLocaleString()}</p>
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center bg-[var(--color-bg-secondary)] p-4 rounded-xl border border-[var(--color-border)]">
        <form onSubmit={handleSearch} className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input type="text" placeholder="搜索账单号 / 用户关联..." value={searchInput} onChange={e => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]" />
        </form>
        <div className="flex gap-1 flex-wrap">
          {ENTRY_TYPES.map(rt => (
            <button key={rt.value} onClick={() => { setEntryType(rt.value); setPage(1); }}
              className={`px-3 py-1.5 text-xs rounded-lg border transition-colors ${entryType === rt.value ? "bg-[var(--color-primary)]/10 border-[var(--color-primary)]/50 text-[var(--color-primary)]" : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)]"}`}>
              {rt.label}
            </button>
          ))}
        </div>
      </div>

      {/* Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-5 py-3.5 font-medium">交易时间</th>
                <th className="px-5 py-3.5 font-medium">流水单号 / 摘要</th>
                <th className="px-5 py-3.5 font-medium">归属用户</th>
                <th className="px-5 py-3.5 font-medium">业务分类</th>
                <th className="px-5 py-3.5 font-medium text-right">账变前</th>
                <th className="px-5 py-3.5 font-medium text-right">变动额</th>
                <th className="px-5 py-3.5 font-medium text-right">结余</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr><td colSpan={7} className="px-6 py-12 text-center">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载流水数据中...</p>
                </td></tr>
              )}
              {error && <tr><td colSpan={7} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td></tr>}
              {data && data.items.length === 0 && <tr><td colSpan={7} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">暂无符合条件的账单流水</td></tr>}
              {data && data.items.map(row => {
                const isIncome = row.ledger.amountDelta > 0;
                return (
                  <tr key={row.ledger.id} className="hover:bg-[var(--color-bg-secondary)]/50 transition-colors">
                    <td className="px-5 py-3.5 text-xs text-[var(--color-text-secondary)] whitespace-nowrap">
                      <div>{new Date(row.ledger.createdAt).toLocaleDateString("zh-CN")}</div>
                      <div className="mt-0.5 font-mono">{new Date(row.ledger.createdAt).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", second: "2-digit" })}</div>
                    </td>
                    <td className="px-5 py-3.5">
                      <div className="font-mono text-xs text-[var(--color-text-secondary)] mb-1">{row.ledger.id}</div>
                      <div className="font-medium max-w-[240px] truncate" title={row.ledger.description || ""}>
                        {row.ledger.description || "无摘要"}
                      </div>
                      {row.ledger.referenceType && (
                        <div className="text-xs text-[var(--color-text-secondary)] mt-0.5">
                          业务单据: <span className="font-mono">{row.ledger.referenceType} ({row.ledger.referenceId?.slice(-8)})</span>
                        </div>
                      )}
                    </td>
                    <td className="px-5 py-3.5">
                      {row.user ? (
                        <><div className="text-sm">{row.user.name}</div><div className="text-xs text-[var(--color-text-secondary)]">{row.user.email}</div></>
                      ) : <span className="text-xs text-[var(--color-text-secondary)]">—</span>}
                    </td>
                    <td className="px-5 py-3.5">{getEntryBadge(row.ledger.entryType)}</td>
                    <td className="px-5 py-3.5 text-right font-mono text-[var(--color-text-secondary)]">{row.ledger.balanceBefore}</td>
                    <td className={`px-5 py-3.5 text-right font-mono font-medium ${isIncome ? "text-green-400" : "text-orange-400"}`}>
                      {isIncome ? "+" : ""}{row.ledger.amountDelta}
                    </td>
                    <td className="px-5 py-3.5 text-right font-mono text-blue-400 font-medium">
                      {row.ledger.balanceAfter}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">共 <span className="font-medium">{data.pagination.total}</span> 条流水</p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}
    </div>
  );
}
