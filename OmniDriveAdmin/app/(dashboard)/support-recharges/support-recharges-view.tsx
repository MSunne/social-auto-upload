"use client";

import { useState } from "react";
import { useSupportRecharges } from "@/lib/hooks/useFinance";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2 } from "lucide-react";
import { SupportRechargeDrawer } from "./support-recharge-drawer";

export function SupportRechargesView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<"pending_review" | "awaiting_submission" | "credited" | "rejected">("pending_review");
  const [searchInput, setSearchInput] = useState("");
  const [selectedOrderId, setSelectedOrderId] = useState<string | null>(null);

  const { data, isLoading, error } = useSupportRecharges({
    page,
    pageSize: 20,
    query: query || undefined,
    status,
  });

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setQuery(searchInput);
    setPage(1);
  };

  const tabs = [
    { label: "待审核", value: "pending_review", count: data?.summary?.pendingReviewCount || 0 },
    { label: "待用户传凭单", value: "awaiting_submission", count: data?.summary?.awaitingSubmissionCount || 0 },
    { label: "已入账", value: "credited", count: data?.summary?.creditedCount || 0 },
    { label: "已驳回", value: "rejected", count: data?.summary?.rejectedCount || 0 },
  ] as const;

  const renderStatusBadge = (value: string) => {
    switch (value) {
      case "awaiting_submission":
        return <span className="inline-flex rounded-full border border-amber-500/20 bg-amber-500/10 px-2 py-1 text-xs font-medium text-amber-500">待提交凭证</span>;
      case "pending_review":
        return <span className="inline-flex rounded-full border border-blue-500/20 bg-blue-500/10 px-2 py-1 text-xs font-medium text-blue-500">待人工审核</span>;
      case "credited":
        return <span className="inline-flex rounded-full border border-green-500/20 bg-green-500/10 px-2 py-1 text-xs font-medium text-green-500">已入账</span>;
      case "rejected":
        return <span className="inline-flex rounded-full border border-red-500/20 bg-red-500/10 px-2 py-1 text-xs font-medium text-red-500">已驳回</span>;
      default:
        return <span className="inline-flex rounded-full border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-2 py-1 text-xs font-medium text-[var(--color-text-secondary)]">{value}</span>;
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="客服充值审核"
        subtitle="处理用户通过企业公户、微信客服等渠道进行的大额对公充值。"
      />

      <div className="flex flex-col sm:flex-row gap-4 justify-between items-end sm:items-center">
        <div className="flex space-x-1 border-b border-[var(--color-border)] w-full sm:w-auto">
          {tabs.map(tab => (
            <button
              key={tab.value}
              onClick={() => { setStatus(tab.value); setPage(1); }}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors relative whitespace-nowrap
                ${status === tab.value
                  ? "border-[var(--color-primary)] text-[var(--color-primary)] bg-[var(--color-primary)]/5"
                  : "border-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-secondary)]"
                }`}
            >
              {tab.label}
              {tab.count > 0 && tab.value === "pending_review" && (
                <span className="ml-2 bg-red-500 text-white text-[10px] px-1.5 py-0.5 rounded-full">
                  {tab.count}
                </span>
              )}
            </button>
          ))}
        </div>

        <form onSubmit={handleSearch} className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            placeholder="搜索订单头或邮箱..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-all"
          />
        </form>
      </div>

      {/* Main Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-6 py-4 font-medium">工单流水 / 用户</th>
                <th className="px-6 py-4 font-medium">期望充值金额</th>
                <th className="px-6 py-4 font-medium">核给积分 / 赠送</th>
                <th className="px-6 py-4 font-medium">状态</th>
                <th className="px-6 py-4 font-medium">提交时间</th>
                <th className="px-6 py-4 font-medium text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center">
                    <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                    <p className="mt-2 text-[var(--color-text-secondary)]">加载工单中...</p>
                  </td>
                </tr>
              )}

              {error && (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-red-500">
                    加载失败，请重试。
                  </td>
                </tr>
              )}

              {data && data.items.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-[var(--color-text-secondary)]">
                    {status === "pending_review" ? "太棒了，所有充值审核已处理完毕！" : "当前分类下没有工单记录。"}
                  </td>
                </tr>
              )}

              {data && data.items.map((row) => (
                <tr key={row.id} className="hover:bg-[var(--color-bg-secondary)]/50 transition-colors">
                  <td className="px-6 py-4">
                    <div className="font-mono text-xs">{row.orderNo}</div>
                    <div className="text-xs text-[var(--color-text-secondary)] mt-1">{row.user.email}</div>
                  </td>
                  <td className="px-6 py-4 font-mono font-medium text-green-500">
                    ¥ {(row.amountCents / 100).toFixed(2)}
                  </td>
                  <td className="px-6 py-4 text-xs">
                    <div className="font-mono text-[var(--color-text-primary)]">基础: {row.baseCredits}</div>
                    {row.bonusCredits > 0 && <div className="font-mono text-amber-500">赠送: {row.bonusCredits}</div>}
                  </td>
                  <td className="px-6 py-4 text-xs text-[var(--color-text-secondary)]">
                    {renderStatusBadge(row.status)}
                  </td>
                  <td className="px-6 py-4 text-[var(--color-text-secondary)] text-xs">
                    {row.submittedAt
                      ? new Date(row.submittedAt).toLocaleString("zh-CN", {
                          month: "2-digit",
                          day: "2-digit",
                          hour: "2-digit",
                          minute: "2-digit",
                        })
                      : "—"}
                  </td>
                  <td className="px-6 py-4 text-right">
                    <button 
                      onClick={() => setSelectedOrderId(row.id)}
                      className="text-xs font-medium text-[var(--color-primary)] bg-[var(--color-primary)]/10 hover:bg-[var(--color-primary)]/20 px-3 py-1.5 rounded-lg transition-colors border border-[var(--color-primary)]/20"
                    >
                      {status === "pending_review" ? "审批查单" : "工单详情"}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex justify-center p-4">
           {/* Pagination... */}
           <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1 text-sm border rounded hover:bg-white/5 disabled:opacity-50">上页</button>
            <span className="px-3 py-1 text-sm">{page} / {data.pagination.totalPages}</span>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page === data.pagination.totalPages} className="px-3 py-1 text-sm border rounded hover:bg-white/5 disabled:opacity-50">下页</button>
          </div>
        </div>
      )}

      {selectedOrderId && (
        <SupportRechargeDrawer 
          orderId={selectedOrderId} 
          onClose={() => setSelectedOrderId(null)} 
        />
      )}
    </div>
  );
}
