"use client";

import { Fragment, useState } from "react";
import { ChevronDown, ChevronUp, Search, Loader2, RefreshCw, HandCoins, ArrowDownToLine, CheckCircle2, Clock } from "lucide-react";
import { useAdminCommissionReleases, useDistributionCommissions } from "@/lib/hooks/useDistribution";
import { PageHeader } from "@/components/ui/common";
import type { AdminCommissionReleaseEvent } from "@/lib/types";

const STATUS_TABS = [
  { id: "", label: "全部佣金" },
  { id: "pending_consume", label: "待消耗" },
  { id: "pending_settlement", label: "待结算", highlight: true },
  { id: "settled", label: "已结算" },
];

function formatCurrency(amountCents: number) {
  return `¥ ${(amountCents / 100).toFixed(2)}`;
}

function extractReleaseTitle(item: AdminCommissionReleaseEvent) {
  const snapshot = item.sourceSnapshot;
  if (snapshot && typeof snapshot === "object") {
    const title = snapshot.title;
    if (typeof title === "string" && title.trim()) {
      return title.trim();
    }
    const publishTask = snapshot.publishTask;
    if (publishTask && typeof publishTask === "object") {
      const publishTitle = (publishTask as Record<string, unknown>).title;
      if (typeof publishTitle === "string" && publishTitle.trim()) {
        return publishTitle.trim();
      }
    }
  }
  return item.sourceType === "ai_job" ? "AI 任务" : item.sourceType;
}

function extractReleaseMeta(item: AdminCommissionReleaseEvent) {
  const metadata = item.metadata;
  if (!metadata || typeof metadata !== "object") {
    return [];
  }
  const values: string[] = [];
  const meterCode = metadata.meterCode;
  if (typeof meterCode === "string" && meterCode.trim()) {
    values.push(meterCode.trim());
  }
  const quantity = metadata.quantity;
  if (typeof quantity === "number" && quantity > 0) {
    values.push(`数量 ${quantity}`);
  }
  const debitedCredits = metadata.debitedCredits;
  if (typeof debitedCredits === "number" && debitedCredits > 0) {
    values.push(`扣减 ${debitedCredits} 积分`);
  }
  const quotaUsed = metadata.quotaUsed;
  if (typeof quotaUsed === "number" && quotaUsed > 0) {
    values.push(`套餐抵扣 ${quotaUsed}`);
  }
  return values;
}

function CommissionTracePanel({ commissionId }: { commissionId: string }) {
  const { data, isLoading, error } = useAdminCommissionReleases(commissionId, true);

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 px-1 py-6 text-sm text-[var(--color-text-secondary)]">
        <Loader2 className="h-4 w-4 animate-spin" />
        正在读取精确释放轨迹...
      </div>
    );
  }

  if (error) {
    return <div className="px-1 py-4 text-sm text-red-500">追溯数据加载失败，请稍后重试</div>;
  }

  if (!data || data.length === 0) {
    return <div className="px-1 py-4 text-sm text-[var(--color-text-secondary)]">该佣金还没有释放记录</div>;
  }

  return (
    <div className="space-y-3">
      {data.map((item) => (
        <div key={item.id} className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <div className="text-sm font-medium text-[var(--color-text-primary)]">{extractReleaseTitle(item)}</div>
              <div className="mt-1 text-xs text-[var(--color-text-secondary)]">
                {item.rechargeOrderNo || item.rechargeOrderId} · {new Date(item.createdAt).toLocaleString("zh-CN")}
              </div>
              <div className="mt-2 flex flex-wrap gap-2 text-[11px] text-[var(--color-text-secondary)]">
                {extractReleaseMeta(item).map((value) => (
                  <span key={value} className="rounded-full border border-[var(--color-border)] px-2 py-1">
                    {value}
                  </span>
                ))}
              </div>
            </div>
            <div className="grid min-w-[180px] grid-cols-2 gap-2 text-right text-xs">
              <div className="rounded-lg bg-[var(--color-bg-secondary)] px-3 py-2">
                <div className="text-[var(--color-text-secondary)]">释放积分</div>
                <div className="mt-1 font-semibold text-[var(--color-text-primary)]">{item.consumedCreditsDelta}</div>
              </div>
              <div className="rounded-lg bg-[var(--color-bg-secondary)] px-3 py-2">
                <div className="text-[var(--color-text-secondary)]">新增佣金</div>
                <div className="mt-1 font-semibold text-[var(--color-primary)]">{formatCurrency(item.releasedAmountDeltaCents)}</div>
              </div>
              <div className="rounded-lg bg-[var(--color-bg-secondary)] px-3 py-2">
                <div className="text-[var(--color-text-secondary)]">累计消耗</div>
                <div className="mt-1 font-semibold text-[var(--color-text-primary)]">{item.commissionItemConsumedCredits}</div>
              </div>
              <div className="rounded-lg bg-[var(--color-bg-secondary)] px-3 py-2">
                <div className="text-[var(--color-text-secondary)]">累计释放</div>
                <div className="mt-1 font-semibold text-blue-400">{formatCurrency(item.commissionItemReleasedAmountCents)}</div>
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

export function CommissionsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [statusParam, setStatusParam] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const { data, isLoading, error, refetch } = useDistributionCommissions({ 
    page, 
    pageSize: 30, 
    query: query || undefined, 
    status: statusParam || undefined 
  });

  const handleSearch = (e: React.FormEvent) => { e.preventDefault(); setQuery(searchInput); setPage(1); };

  const renderStatus = (status: string) => {
    switch (status) {
      case "pending_consume": return <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-gray-500/10 text-[var(--color-text-secondary)] border border-gray-500/20 flex items-center gap-1 w-fit"><Clock className="h-3 w-3" /> 待消耗</span>;
      case "pending_settlement": return <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20 flex items-center gap-1 w-fit"><ArrowDownToLine className="h-3 w-3" /> 待结算</span>;
      case "settled": return <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-green-500/10 text-green-400 border border-green-500/20 flex items-center gap-1 w-fit"><CheckCircle2 className="h-3 w-3" /> 已结算</span>;
      default: return <span className="text-xs text-[var(--color-text-secondary)]">{status}</span>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="分销佣金流水" subtitle="监控所有基于充值或消费产生的上级返佣流水明细。" />
        <button onClick={() => refetch()} className="flex items-center gap-2 px-3 py-2 border border-[var(--color-border)] rounded-lg text-sm hover:bg-[var(--color-bg-secondary)] transition-colors">
          <RefreshCw className="h-4 w-4" /> <span className="hidden sm:inline">刷新</span>
        </button>
      </div>

      {/* Summary Stats */}
      {data && data.summary && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1 flex items-center gap-1"><HandCoins className="h-3.5 w-3.5" /> 累计产生佣金</p>
            <p className="text-xl font-medium text-[var(--color-text-primary)]">¥ {(data.summary.totalCommissionAmountCents / 100).toFixed(2)}</p>
          </div>
          <div className="p-4 rounded-xl border border-blue-500/30 bg-blue-500/5">
            <p className="text-xs text-blue-400 mb-1 flex items-center gap-1"><ArrowDownToLine className="h-3.5 w-3.5" /> 待结算池 (可提现总额)</p>
            <p className="text-xl font-bold text-blue-400">¥ {(data.summary.pendingSettlementAmountCents / 100).toFixed(2)}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1 flex items-center gap-1"><Clock className="h-3.5 w-3.5" /> 待消耗 (未解锁佣金)</p>
            <p className="text-xl font-medium text-[var(--color-text-secondary)]">¥ {(data.summary.pendingConsumeAmountCents / 100).toFixed(2)}</p>
          </div>
          <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
            <p className="text-xs text-[var(--color-text-secondary)] mb-1 flex items-center gap-1"><CheckCircle2 className="h-3.5 w-3.5" /> 历史已结算打款</p>
            <p className="text-xl font-medium text-green-400">¥ {(data.summary.settledAmountCents / 100).toFixed(2)}</p>
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3 justify-between items-start sm:items-center">
        <div className="flex gap-1 bg-[var(--color-bg-secondary)] p-1 rounded-lg border border-[var(--color-border)] overflow-x-auto w-full sm:w-auto">
          {STATUS_TABS.map(tab => (
            <button key={tab.id} onClick={() => { setStatusParam(tab.id); setPage(1); }}
              className={`px-4 py-1.5 text-sm rounded-md whitespace-nowrap transition-colors ${statusParam === tab.id ? "bg-[var(--color-bg-primary)] text-[var(--color-text-primary)] shadow-sm border border-[var(--color-border)] opacity-100 font-medium" : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-primary)]/50 border border-transparent opacity-80"} ${tab.highlight && statusParam !== tab.id ? "text-blue-400 hover:text-blue-300" : ""}`}>
              {tab.label}
            </button>
          ))}
        </div>
        <form onSubmit={handleSearch} className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input type="text" placeholder="搜索 推广员/受邀人..." value={searchInput} onChange={e => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors" />
        </form>
      </div>

      {/* Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-5 py-3.5 font-medium">流水号 / 时间</th>
                <th className="px-5 py-3.5 font-medium">获佣推广员</th>
                <th className="px-5 py-3.5 font-medium">成单受邀人</th>
                <th className="px-5 py-3.5 font-medium text-right">成金基数 / 比例</th>
                <th className="px-5 py-3.5 font-medium text-right">佣金 / 已释放</th>
                <th className="px-5 py-3.5 font-medium text-right">积分释放进度</th>
                <th className="px-5 py-3.5 font-medium">状态</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr><td colSpan={7} className="px-6 py-12 text-center">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载佣金明细...</p>
                </td></tr>
              )}
              {error && <tr><td colSpan={7} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td></tr>}
              {data && data.items.length === 0 && <tr><td colSpan={7} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">暂无符合条件的佣金流水</td></tr>}
              {data && data.items.map(row => {
                const isExpanded = expandedId === row.id;
                const releasePercent = row.totalGrantedCredits > 0 ? Math.min(100, (row.consumedCredits / row.totalGrantedCredits) * 100) : 0;

                return (
                  <Fragment key={row.id}>
                    <tr className="hover:bg-[var(--color-bg-secondary)]/50 transition-colors">
                      <td className="px-5 py-3.5 whitespace-nowrap">
                        <button
                          onClick={() => setExpandedId(current => current === row.id ? null : row.id)}
                          className="mb-2 inline-flex items-center gap-1 rounded-full border border-[var(--color-border)] px-2 py-1 text-[11px] text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)]"
                        >
                          {isExpanded ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
                          精确追溯
                        </button>
                        <div className="font-mono text-xs text-[var(--color-text-secondary)] mb-1">{row.rechargeOrderNo || row.id}</div>
                        <div className="text-xs">{new Date(row.createdAt).toLocaleString("zh-CN")}</div>
                      </td>
                      <td className="px-5 py-3.5">
                        <div className="text-sm font-medium">{row.promoter.name}</div>
                        <div className="text-xs text-[var(--color-text-secondary)]">{row.promoter.email}</div>
                      </td>
                      <td className="px-5 py-3.5">
                        <div className="text-sm font-medium">{row.invitee.name}</div>
                        <div className="text-xs text-[var(--color-text-secondary)]">{row.invitee.email}</div>
                      </td>
                      <td className="px-5 py-3.5 text-right">
                        <div className="text-sm">{formatCurrency(row.commissionBaseAmountCents)}</div>
                        <div className="text-xs text-[var(--color-text-secondary)]">{(row.commissionRate * 100).toFixed(2)}%</div>
                      </td>
                      <td className="px-5 py-3.5 text-right">
                        <div className="text-lg font-medium text-[var(--color-primary)]">{formatCurrency(row.amountCents)}</div>
                        <div className="text-xs text-blue-400">已释放 {formatCurrency(row.releasedAmountCents)}</div>
                      </td>
                      <td className="px-5 py-3.5 text-right">
                        <div className="text-sm font-medium">{row.consumedCredits} / {row.totalGrantedCredits}</div>
                        <div className="mt-2 h-2 overflow-hidden rounded-full bg-[var(--color-bg-secondary)]">
                          <div className="h-full rounded-full bg-[var(--color-primary)]" style={{ width: `${releasePercent}%` }} />
                        </div>
                        <div className="mt-1 text-[10px] text-[var(--color-text-secondary)]">{releasePercent.toFixed(1)}% · {row.releaseEventCount} 条释放事件</div>
                      </td>
                      <td className="px-5 py-3.5 space-y-1">
                        {renderStatus(row.status)}
                        {row.status === "settled" && row.settledAt && (
                          <div className="text-[10px] text-[var(--color-text-secondary)]">于 {new Date(row.settledAt).toLocaleDateString()} 结算</div>
                        )}
                      </td>
                    </tr>
                    {isExpanded && (
                      <tr className="bg-[var(--color-bg-secondary)]/35">
                        <td colSpan={7} className="px-5 py-5">
                          <CommissionTracePanel commissionId={row.id} />
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">共 <span className="font-medium">{data.pagination.total}</span> 条佣金明细</p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}
    </div>
  );
}
