"use client";

import { useState } from "react";
import type { AdminSupportRechargeRow } from "@/lib/types";
import { useSupportRechargeLookup, useSupportRecharges } from "@/lib/hooks/useFinance";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2 } from "lucide-react";
import { SupportRechargeDrawer } from "./support-recharge-drawer";

function SupportRechargeStatusBadge({ value }: { value: string }) {
  switch (value) {
    case "awaiting_submission":
      return <span className="inline-flex rounded-full border border-amber-500/20 bg-amber-500/10 px-2 py-1 text-xs font-medium text-amber-500">待人工审核</span>;
    case "pending_review":
      return <span className="inline-flex rounded-full border border-blue-500/20 bg-blue-500/10 px-2 py-1 text-xs font-medium text-blue-500">待人工审核</span>;
    case "credited":
      return <span className="inline-flex rounded-full border border-green-500/20 bg-green-500/10 px-2 py-1 text-xs font-medium text-green-500">已入账</span>;
    case "rejected":
      return <span className="inline-flex rounded-full border border-red-500/20 bg-red-500/10 px-2 py-1 text-xs font-medium text-red-500">已驳回</span>;
    case "invalidated":
    case "closed":
      return <span className="inline-flex rounded-full border border-slate-500/20 bg-slate-500/10 px-2 py-1 text-xs font-medium text-slate-300">已失效</span>;
    default:
      return <span className="inline-flex rounded-full border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-2 py-1 text-xs font-medium text-[var(--color-text-secondary)]">{value}</span>;
  }
}

function SupportRechargeTableRow({
  row,
  onOpen,
}: {
  row: AdminSupportRechargeRow;
  onOpen: (orderId: string) => void;
}) {
  return (
    <tr className="hover:bg-[var(--color-bg-secondary)]/50 transition-colors">
      <td className="px-6 py-4">
        <div className="font-mono text-xs">{row.orderNo}</div>
        <div className="mt-1 text-xs text-[var(--color-text-secondary)]">{row.user.email}</div>
      </td>
      <td className="px-6 py-4 font-mono font-medium text-green-500">
        ¥ {(row.amountCents / 100).toFixed(2)}
      </td>
      <td className="px-6 py-4 text-xs">
        <div className="font-mono text-[var(--color-text-primary)]">基础: {row.baseCredits}</div>
        {row.bonusCredits > 0 ? (
          <div className="font-mono text-amber-500">赠送: {row.bonusCredits}</div>
        ) : null}
      </td>
      <td className="px-6 py-4 text-xs text-[var(--color-text-secondary)]">
        <SupportRechargeStatusBadge value={row.status} />
      </td>
      <td className="px-6 py-4 text-xs text-[var(--color-text-secondary)]">
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
          onClick={() => onOpen(row.id)}
          className="rounded-lg border border-[var(--color-primary)]/20 bg-[var(--color-primary)]/10 px-3 py-1.5 text-xs font-medium text-[var(--color-primary)] transition-colors hover:bg-[var(--color-primary)]/20"
        >
          {row.status === "pending_review" ? "审核工单" : "查看详情"}
        </button>
      </td>
    </tr>
  );
}

export function SupportRechargesView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<"all" | "pending_review" | "credited" | "rejected" | "invalidated">("all");
  const [searchInput, setSearchInput] = useState("");
  const [rechargeCode, setRechargeCode] = useState("");
  const [selectedOrderId, setSelectedOrderId] = useState<string | null>(null);
  const [lookupResult, setLookupResult] = useState<AdminSupportRechargeRow | null>(null);
  const lookupMutation = useSupportRechargeLookup();

  const { data, isLoading, error } = useSupportRecharges({
    page,
    pageSize: 20,
    query: query || undefined,
    status: status === "all" ? undefined : status,
  });

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setQuery(searchInput);
    setPage(1);
  };

  const handleLookup = async (e: React.FormEvent) => {
    e.preventDefault();
    const code = rechargeCode.trim();
    if (!code) {
      return;
    }
    try {
      const detail = await lookupMutation.mutateAsync(code);
      setLookupResult(detail.record);
    } catch {
      // Error is already normalized by the API client and rendered below.
    }
  };

  const tabs = [
    { label: "全部", value: "all", count: data?.pagination?.total || 0 },
    { label: "待审核", value: "pending_review", count: data?.summary?.pendingReviewCount || 0 },
    { label: "已入账", value: "credited", count: data?.summary?.creditedCount || 0 },
    { label: "已驳回", value: "rejected", count: data?.summary?.rejectedCount || 0 },
    { label: "已失效", value: "invalidated", count: data?.summary?.invalidatedCount || 0 },
  ] as const;

  return (
    <div className="space-y-6">
      <PageHeader
        title="客服充值审核"
        subtitle="处理用户通过企业公户、微信客服等渠道进行的大额对公充值。"
      />

      <div className="grid gap-4 xl:grid-cols-[1.1fr_1fr]">
        <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
          <div className="flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
            充值码直达
          </div>
          <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
            直接输入客户发来的充值激活码，系统会先在下方生成一条查找结果，客服再手动点击进入审核。
          </p>
          <form onSubmit={handleLookup} className="mt-3 flex gap-3">
            <input
              type="text"
              placeholder="输入充值激活码，例如 RC202603..."
              value={rechargeCode}
              onChange={(e) => setRechargeCode(e.target.value)}
              className="flex-1 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:outline-none focus:border-[var(--color-primary)]"
            />
            <button
              type="submit"
              disabled={!rechargeCode.trim() || lookupMutation.isPending}
              className="rounded-lg bg-[var(--color-primary)] px-4 py-2 text-sm font-medium text-white disabled:cursor-not-allowed disabled:opacity-60"
            >
              {lookupMutation.isPending ? "查找中..." : "查找"}
            </button>
          </form>
          {lookupResult ? (
            <div className="mt-3 flex items-center justify-between rounded-lg border border-emerald-500/20 bg-emerald-500/8 px-3 py-2 text-xs text-emerald-300">
              <span>已找到充值码 {lookupResult.orderNo}，结果已显示在下方“查找结果”区域。</span>
              <button
                type="button"
                onClick={() => setLookupResult(null)}
                className="font-medium text-emerald-200 hover:text-white"
              >
                清空结果
              </button>
            </div>
          ) : null}
          {lookupMutation.isError ? (
            <p className="mt-3 text-xs text-red-500">{lookupMutation.error instanceof Error ? lookupMutation.error.message : "查找失败，请稍后重试"}</p>
          ) : null}
        </div>

        <div className="flex flex-col sm:flex-row gap-4 justify-between items-end sm:items-center">
          <div className="flex space-x-1 border-b border-[var(--color-border)] w-full sm:w-auto">
            {tabs.map((tab) => (
              <button
                key={tab.value}
                onClick={() => {
                  setStatus(tab.value);
                  setPage(1);
                }}
                className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors relative whitespace-nowrap
                  ${status === tab.value
                    ? "border-[var(--color-primary)] text-[var(--color-primary)] bg-[var(--color-primary)]/5"
                    : "border-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-secondary)]"
                  }`}
              >
                {tab.label}
                {(tab.count > 0 && (tab.value === "pending_review" || tab.value === "all")) ? (
                  <span className="ml-2 bg-red-500 text-white text-[10px] px-1.5 py-0.5 rounded-full">
                    {tab.count}
                  </span>
                ) : null}
              </button>
            ))}
          </div>

          <form onSubmit={handleSearch} className="relative w-full sm:w-72">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
            <input
              type="text"
              placeholder="搜索激活码 / 邮箱 / 套餐..."
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-all"
            />
          </form>
        </div>
      </div>

      {lookupResult ? (
        <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/5 overflow-hidden">
          <div className="flex items-center justify-between border-b border-emerald-500/15 px-6 py-4">
            <div>
              <h2 className="text-sm font-semibold text-[var(--color-text-primary)]">查找结果</h2>
              <p className="mt-1 text-xs text-[var(--color-text-secondary)]">通过充值码精确定位到的工单，客服可手动点击进入审核。</p>
            </div>
            <button
              type="button"
              onClick={() => setLookupResult(null)}
              className="text-xs font-medium text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
            >
              关闭结果
            </button>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm text-left">
              <thead className="border-b border-emerald-500/15 bg-black/10 text-xs uppercase text-[var(--color-text-secondary)]">
                <tr>
                  <th className="px-6 py-4 font-medium">工单流水 / 用户</th>
                  <th className="px-6 py-4 font-medium">期望充值金额</th>
                  <th className="px-6 py-4 font-medium">核给积分 / 赠送</th>
                  <th className="px-6 py-4 font-medium">状态</th>
                  <th className="px-6 py-4 font-medium">提交时间</th>
                  <th className="px-6 py-4 font-medium text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                <SupportRechargeTableRow row={lookupResult} onOpen={setSelectedOrderId} />
              </tbody>
            </table>
          </div>
        </div>
      ) : null}

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
                    {error instanceof Error ? error.message : "加载失败，请重试。"}
                  </td>
                </tr>
              )}

              {data && data.items.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-[var(--color-text-secondary)]">
                    {status === "pending_review"
                      ? "当前没有待审核工单，可以切换到“全部”查看历史记录。"
                      : "当前分类下没有工单记录。"}
                  </td>
                </tr>
              )}

              {data && data.items.map((row) => (
                <SupportRechargeTableRow key={row.id} row={row} onOpen={setSelectedOrderId} />
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
