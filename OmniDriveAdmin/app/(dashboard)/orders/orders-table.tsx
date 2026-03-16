"use client";

import { useState } from "react";
import { useOrders } from "@/lib/hooks/useFinance";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2 } from "lucide-react";

export function OrdersTable() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("");
  const [channel, setChannel] = useState("");

  // Input states before submitting search
  const [searchInput, setSearchInput] = useState("");

  const { data, isLoading, error } = useOrders({
    page,
    pageSize: 20,
    query: query || undefined,
    status: status || undefined,
    channel: channel || undefined,
  });

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setQuery(searchInput);
    setPage(1);
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "paid":
        return <span className="px-2 py-1 text-xs font-medium rounded-full bg-green-500/10 text-green-500 border border-green-500/20">已支付 (Paid)</span>;
      case "pending_payment":
        return <span className="px-2 py-1 text-xs font-medium rounded-full bg-blue-500/10 text-blue-500 border border-blue-500/20">待支付 (Pending)</span>;
      case "processing":
      case "awaiting_manual_review":
        return <span className="px-2 py-1 text-xs font-medium rounded-full bg-amber-500/10 text-amber-500 border border-amber-500/20">处理中 ({status})</span>;
      case "rejected":
      case "failed":
      case "closed":
        return <span className="px-2 py-1 text-xs font-medium rounded-full bg-red-500/10 text-red-500 border border-red-500/20">失败/关闭 ({status})</span>;
      default:
        return <span className="px-2 py-1 text-xs font-medium rounded-full bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border border-[var(--color-border)]">{status}</span>;
    }
  };

  const getChannelBadge = (channel: string) => {
    switch (channel) {
      case "alipay":
        return <span className="text-[#1677FF] font-medium">支付宝</span>;
      case "wechatpay":
        return <span className="text-[#09B83E] font-medium">微信支付</span>;
      case "manual_cs":
        return <span className="text-purple-500 font-medium">客服充值</span>;
      default:
        return <span>{channel}</span>;
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="充值与订单大盘"
        subtitle="全局跟踪所有产生的资金订单（包含三方支付与人工打款录单）。"
      />

      {/* Filters Overlay */}
      <div className="flex flex-col sm:flex-row gap-4 justify-between items-start sm:items-center bg-[var(--color-bg-secondary)] p-4 rounded-xl border border-[var(--color-border)]">
        <form onSubmit={handleSearch} className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            placeholder="搜索订单号 (OrderNo) 或用户邮箱..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] focus:ring-1 focus:ring-[var(--color-primary)] transition-all"
          />
        </form>

        <div className="flex gap-3 w-full sm:w-auto">
          <select
            value={channel}
            onChange={(e) => { setChannel(e.target.value); setPage(1); }}
            className="flex-1 sm:flex-none px-3 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]"
          >
            <option value="">所有渠道</option>
            <option value="alipay">支付宝</option>
            <option value="wechatpay">微信支付</option>
            <option value="manual_cs">客服充值</option>
          </select>

          <select
            value={status}
            onChange={(e) => { setStatus(e.target.value); setPage(1); }}
            className="flex-1 sm:flex-none px-3 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]"
          >
            <option value="">所有状态</option>
            <option value="paid">已支付 (Paid)</option>
            <option value="pending_payment">待支付 (Pending)</option>
            <option value="processing">处理中 (Processing)</option>
            <option value="rejected">已驳回 (Rejected)</option>
          </select>
        </div>
      </div>

      {/* Summary Cards */}
      {data && data.summary && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">查询总订单池</p>
            <p className="text-xl font-mono">{data.summary.totalOrderCount}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">入账/请求总额</p>
            <p className="text-xl font-mono text-green-500">¥ {(data.summary.totalAmountCents / 100).toFixed(2)}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">成功订单数</p>
            <p className="text-xl font-mono">{data.summary.paidOrderCount}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1">待客服复核数</p>
            <p className="text-xl font-mono text-amber-500">{data.summary.awaitingManualReviewCount}</p>
          </div>
        </div>
      )}

      {/* Main Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-6 py-4 font-medium">用户 / 邮箱</th>
                <th className="px-6 py-4 font-medium">订单号 (Order No)</th>
                <th className="px-6 py-4 font-medium">交易标题</th>
                <th className="px-6 py-4 font-medium text-right">金额 (RMB)</th>
                <th className="px-6 py-4 font-medium">渠道</th>
                <th className="px-6 py-4 font-medium">状态</th>
                <th className="px-6 py-4 font-medium">创建时间</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr>
                  <td colSpan={7} className="px-6 py-12 text-center">
                    <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                    <p className="mt-2 text-[var(--color-text-secondary)]">加载数据中...</p>
                  </td>
                </tr>
              )}

              {error && (
                <tr>
                  <td colSpan={7} className="px-6 py-12 text-center text-red-500">
                    加载订单失败，请检查网络或刷新页面重试。
                  </td>
                </tr>
              )}

              {data && data.data.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-6 py-12 text-center text-[var(--color-text-secondary)]">
                    未找到匹配的订单记录
                  </td>
                </tr>
              )}

              {data && data.data.map((row) => (
                <tr key={row.order.id} className="hover:bg-[var(--color-bg-secondary)]/50 transition-colors">
                  <td className="px-6 py-4">
                    <div className="font-medium text-[var(--color-text-primary)] truncate max-w-[150px]" title={row.user.name}>{row.user.name}</div>
                    <div className="text-xs text-[var(--color-text-secondary)] truncate max-w-[150px]" title={row.user.email}>{row.user.email}</div>
                  </td>
                  <td className="px-6 py-4 font-mono text-xs">{row.order.orderNo}</td>
                  <td className="px-6 py-4 max-w-[200px] truncate" title={row.order.subject}>{row.order.subject}</td>
                  <td className="px-6 py-4 font-mono text-right font-medium">
                    ¥ {(row.order.amountCents / 100).toFixed(2)}
                  </td>
                  <td className="px-6 py-4">{getChannelBadge(row.order.channel)}</td>
                  <td className="px-6 py-4">{getStatusBadge(row.order.status)}</td>
                  <td className="px-6 py-4 text-[var(--color-text-secondary)]">
                    {new Date(row.order.createdAt).toLocaleString('zh-CN', {
                      month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'
                    })}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Pagination Container */}
      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-2">
          <p className="text-sm text-[var(--color-text-secondary)]">
            共 <span className="font-medium text-[var(--color-text-primary)]">{data.pagination.total}</span> 条订单，当前第 {data.pagination.page} / {data.pagination.totalPages} 页
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page === 1}
              className="px-3 py-1.5 text-sm bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg disabled:opacity-50 disabled:cursor-not-allowed hover:bg-[var(--color-border-hover)] transition-colors"
            >
              上一页
            </button>
            <button
              onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))}
              disabled={page >= data.pagination.totalPages}
              className="px-3 py-1.5 text-sm bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg disabled:opacity-50 disabled:cursor-not-allowed hover:bg-[var(--color-border-hover)] transition-colors"
            >
              下一页
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
