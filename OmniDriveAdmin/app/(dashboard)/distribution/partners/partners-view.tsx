"use client";

import { useState } from "react";
import { Loader2, Plus, RefreshCw, Search, TicketPercent, Users } from "lucide-react";
import { PageHeader } from "@/components/ui/common";
import { useDistributionPartners } from "@/lib/hooks/useDistribution";
import { PartnerDrawer } from "./partner-drawer";

function formatCurrency(amountCents: number) {
  return `¥ ${(amountCents / 100).toFixed(2)}`;
}

export function PartnersView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState("");
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);

  const { data, isLoading, error, refetch } = useDistributionPartners({
    page,
    pageSize: 20,
    query: query || undefined,
    status: status || undefined,
  });

  const handleSearch = (event: React.FormEvent) => {
    event.preventDefault();
    setPage(1);
    setQuery(searchInput);
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="分销员档案" subtitle="开通和查看分销员档案、合作码、下级规模与可提现资金池。" />
        <div className="flex items-center gap-3">
          <button
            onClick={() => refetch()}
            className="flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2 text-sm transition-colors hover:bg-[var(--color-bg-secondary)]"
          >
            <RefreshCw className="h-4 w-4" />
            <span className="hidden sm:inline">刷新</span>
          </button>
          <button
            onClick={() => setIsDrawerOpen(true)}
            className="flex items-center gap-2 rounded-lg bg-[var(--color-primary)] px-3 py-2 text-sm font-medium text-white transition-all hover:brightness-110"
          >
            <Plus className="h-4 w-4" />
            新增分销员
          </button>
        </div>
      </div>

      {data?.summary && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
            <p className="mb-1 text-xs text-[var(--color-text-secondary)]">分销员总数</p>
            <p className="text-2xl font-bold">{data.summary.totalCount}</p>
          </div>
          <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
            <p className="mb-1 text-xs text-[var(--color-text-secondary)]">累计覆盖下级</p>
            <p className="text-2xl font-bold text-[var(--color-text-primary)]">{data.summary.inviteeCount}</p>
          </div>
          <div className="rounded-xl border border-blue-500/30 bg-blue-500/5 p-4">
            <p className="mb-1 text-xs text-blue-400">待结算佣金池</p>
            <p className="text-2xl font-bold text-blue-400">{formatCurrency(data.summary.pendingSettlementAmountCents)}</p>
          </div>
        </div>
      )}

      <div className="flex flex-col items-start justify-between gap-3 sm:flex-row sm:items-center">
        <div className="flex w-full gap-1 overflow-x-auto rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-1 sm:w-auto">
          {[
            { id: "", label: "全部" },
            { id: "active", label: "生效中" },
            { id: "inactive", label: "已停用" },
          ].map((item) => (
            <button
              key={item.id}
              onClick={() => {
                setPage(1);
                setStatus(item.id);
              }}
              className={`rounded-md px-4 py-1.5 text-sm transition-colors ${
                status === item.id
                  ? "border border-[var(--color-border)] bg-[var(--color-bg-primary)] text-[var(--color-text-primary)] shadow-sm"
                  : "border border-transparent text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)]/50 hover:text-[var(--color-text-primary)]"
              }`}
            >
              {item.label}
            </button>
          ))}
        </div>

        <form onSubmit={handleSearch} className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            placeholder="搜索邮箱、名字、合作码..."
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] py-2 pl-9 pr-4 text-sm transition-colors focus:border-[var(--color-primary)] focus:outline-none"
          />
        </form>
      </div>

      <div className="overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-xs uppercase text-[var(--color-text-secondary)]">
              <tr>
                <th className="px-5 py-3.5 font-medium">分销员</th>
                <th className="px-5 py-3.5 font-medium">合作码</th>
                <th className="px-5 py-3.5 font-medium text-right">当前佣金比例</th>
                <th className="px-5 py-3.5 font-medium text-right">下级人数</th>
                <th className="px-5 py-3.5 font-medium text-right">待结算 / 已结算</th>
                <th className="px-5 py-3.5 font-medium text-right">可提现</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center">
                    <Loader2 className="mx-auto h-6 w-6 animate-spin text-[var(--color-text-secondary)]" />
                    <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载分销员档案...</p>
                  </td>
                </tr>
              )}
              {error && (
                <tr>
                  <td colSpan={6} className="px-6 py-10 text-center text-sm text-red-500">
                    {error instanceof Error && error.message.trim()
                      ? `加载失败：${error.message.trim()}`
                      : "加载失败，请稍后重试"}
                  </td>
                </tr>
              )}
              {data && data.items.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-sm text-[var(--color-text-secondary)]">
                    暂无分销员档案
                  </td>
                </tr>
              )}
              {data?.items.map((item) => (
                <tr key={item.user.id} className="transition-colors hover:bg-[var(--color-bg-secondary)]/50">
                  <td className="px-5 py-4">
                    <div className="font-medium text-[var(--color-text-primary)]">{item.user.name}</div>
                    <div className="mt-0.5 text-xs text-[var(--color-text-secondary)]">{item.user.email}</div>
                  </td>
                  <td className="px-5 py-4">
                    <div className="inline-flex items-center gap-2 rounded-full border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-1 text-xs font-medium">
                      <TicketPercent className="h-3.5 w-3.5 text-[var(--color-primary)]" />
                      {item.partnerCode}
                    </div>
                  </td>
                  <td className="px-5 py-4 text-right font-medium">{(item.currentCommissionRate * 100).toFixed(2)}%</td>
                  <td className="px-5 py-4 text-right">
                    <div className="inline-flex items-center gap-1 text-[var(--color-text-primary)]">
                      <Users className="h-4 w-4 text-[var(--color-text-secondary)]" />
                      {item.inviteeCount}
                    </div>
                  </td>
                  <td className="px-5 py-4 text-right">
                    <div className="font-medium text-blue-400">{formatCurrency(item.pendingSettlementAmountCents)}</div>
                    <div className="mt-1 text-xs text-green-400">{formatCurrency(item.settledAmountCents)}</div>
                  </td>
                  <td className="px-5 py-4 text-right font-semibold text-[var(--color-primary)]">
                    {formatCurrency(item.availableWithdrawalAmountCents)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {data?.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">
            共 <span className="font-medium">{data.pagination.total}</span> 位分销员
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page === 1}
              className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm transition-colors hover:bg-[var(--color-bg-secondary)] disabled:opacity-50"
            >
              上一页
            </button>
            <button
              onClick={() => setPage((current) => Math.min(data.pagination.totalPages, current + 1))}
              disabled={page >= data.pagination.totalPages}
              className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm transition-colors hover:bg-[var(--color-bg-secondary)] disabled:opacity-50"
            >
              下一页
            </button>
          </div>
        </div>
      )}

      <PartnerDrawer
        isOpen={isDrawerOpen}
        onClose={() => setIsDrawerOpen(false)}
        onSuccess={() => refetch()}
      />
    </div>
  );
}
