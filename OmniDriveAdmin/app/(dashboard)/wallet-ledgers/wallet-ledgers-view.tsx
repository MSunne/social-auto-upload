"use client";

import { useState } from "react";
import { Loader2, RefreshCw, Search } from "lucide-react";
import { useBillingActivities } from "@/lib/hooks/useFinance";
import { PageHeader } from "@/components/ui/common";
import type { BillingActivity } from "@/lib/types";

const KIND_OPTIONS = [
  { value: "", label: "全部记录" },
  { value: "usage_event", label: "AI 计费" },
  { value: "wallet_ledger", label: "钱包账变" },
  { value: "recharge_order", label: "充值订单" },
];

const JOB_TYPE_OPTIONS = [
  { value: "", label: "全部 AI 类型" },
  { value: "chat", label: "聊天" },
  { value: "image", label: "作图" },
  { value: "video", label: "视频" },
];

const STATUS_OPTIONS = [
  { value: "", label: "全部状态" },
  { value: "billed", label: "已计费" },
  { value: "returned", label: "已返还" },
  { value: "failed", label: "计费失败" },
  { value: "paid", label: "已支付" },
  { value: "pending_payment", label: "待支付" },
  { value: "processing", label: "处理中" },
  { value: "awaiting_manual_review", label: "待人工审核" },
];

const ENTRY_TYPE_LABELS: Record<string, string> = {
  recharge: "充值入账",
  consume: "消耗抵扣",
  refund: "售后退款",
  usage_refund: "任务失败返还积分",
  usage_return: "任务失败返还积分",
  grant: "赠送积分",
  manual_compensation: "人工补偿",
  manual_deduction: "人工扣减",
  admin_adjustment: "人工调账",
  system_adjustment: "系统调账",
};

const CHANNEL_LABELS: Record<string, string> = {
  manual_cs: "客服充值",
  alipay: "支付宝",
  wechatpay: "微信支付",
};

const JOB_TYPE_LABELS: Record<string, string> = {
  chat: "聊天",
  image: "作图",
  video: "视频",
};

function formatCurrency(cents: number) {
  return `¥ ${(cents / 100).toFixed(2)}`;
}

function formatDateTime(value?: string | null) {
  if (!value) {
    return "—";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "—";
  }
  return parsed.toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function getActivityTypeLabel(item: BillingActivity) {
  if (item.kind === "recharge_order") {
    return "充值订单";
  }
  if (item.kind === "wallet_ledger") {
    return (item.creditDelta ?? 0) > 0 ? "钱包入账" : "钱包扣减";
  }
  return "AI 计费";
}

function getPayloadRecord(item: BillingActivity): Record<string, unknown> | null {
  if (!item.payload || typeof item.payload !== "object") {
    return null;
  }
  return item.payload as Record<string, unknown>;
}

function getPayloadNumber(item: BillingActivity, key: string) {
  const payload = getPayloadRecord(item);
  const value = payload?.[key];
  return typeof value === "number" ? value : 0;
}

function getBusinessLabel(item: BillingActivity) {
  if (item.kind === "recharge_order") {
    return CHANNEL_LABELS[item.channel ?? ""] || item.channel || "充值";
  }
  if (item.kind === "wallet_ledger") {
    return ENTRY_TYPE_LABELS[item.entryType ?? ""] || item.entryType || "钱包账变";
  }
  if (item.jobType) {
    return JOB_TYPE_LABELS[item.jobType] || item.jobType;
  }
  return item.sourceType || item.meterName || item.meterCode || "AI 用量";
}

function getActivityAmount(item: BillingActivity) {
  if (item.kind === "recharge_order" && typeof item.amountCents === "number") {
    const credits = (item.creditAmount ?? 0) + (item.bonusCreditAmount ?? 0);
    return {
      text: formatCurrency(item.amountCents),
      meta: credits > 0 ? `到账 ${credits.toLocaleString("zh-CN")} 积分` : "",
      tone: "text-green-400",
    };
  }
  if (item.kind === "wallet_ledger" && typeof item.creditDelta === "number") {
    const isIncome = item.creditDelta > 0;
    return {
      text: `${isIncome ? "+" : ""}${item.creditDelta.toLocaleString("zh-CN")} 积分`,
      meta: "",
      tone: isIncome ? "text-green-400" : "text-orange-400",
    };
  }
  if (item.status === "returned" || item.status === "refunded") {
    const returnedCredits = getPayloadNumber(item, "returnedCredits") || getPayloadNumber(item, "refundedCredits");
    if (returnedCredits > 0) {
      return {
        text: `+${returnedCredits.toLocaleString("zh-CN")} 积分`,
        meta: "任务失败已自动返还积分",
        tone: "text-green-400",
      };
    }
  }
  if (typeof item.debitedCredits === "number" && item.debitedCredits > 0) {
    return {
      text: `-${item.debitedCredits.toLocaleString("zh-CN")} 积分`,
      meta: typeof item.usageQuantity === "number" ? `${item.usageQuantity.toLocaleString("zh-CN")} ${item.meterName || item.meterCode || "单位"}` : "",
      tone: "text-orange-400",
    };
  }
  return {
    text: typeof item.usageQuantity === "number" ? `${item.usageQuantity.toLocaleString("zh-CN")} ${item.meterName || item.meterCode || "单位"}` : "—",
    meta: item.status === "failed" ? "本次未成功扣费" : "",
    tone: item.status === "failed" ? "text-red-400" : "text-[var(--color-text-secondary)]",
  };
}

function renderStatus(status?: string | null) {
  if (!status) {
    return <span className="text-xs text-[var(--color-text-secondary)]">已记账</span>;
  }
  if (status === "billed") {
    return <span className="inline-flex rounded-full border border-green-500/20 bg-green-500/10 px-2 py-0.5 text-xs font-medium text-green-400">已计费</span>;
  }
  if (status === "returned" || status === "refunded") {
    return <span className="inline-flex rounded-full border border-sky-500/20 bg-sky-500/10 px-2 py-0.5 text-xs font-medium text-sky-400">已返还</span>;
  }
  if (status === "failed") {
    return <span className="inline-flex rounded-full border border-red-500/20 bg-red-500/10 px-2 py-0.5 text-xs font-medium text-red-400">计费失败</span>;
  }
  return <span className="text-xs text-[var(--color-text-secondary)]">{status}</span>;
}

export function WalletLedgersView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [kind, setKind] = useState("");
  const [jobType, setJobType] = useState("");
  const [status, setStatus] = useState("");

  const { data, isLoading, error, refetch } = useBillingActivities({
    page,
    pageSize: 30,
    query: query || undefined,
    kind: kind || undefined,
    jobType: jobType || undefined,
    status: status || undefined,
  });

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="财务流水" subtitle="统一查看充值订单、钱包账变和 OmniDrive 全部 AI 计费记录，并按来源或状态筛选。" />
        <button
          onClick={() => refetch()}
          className="flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2 text-sm transition-colors hover:bg-[var(--color-bg-secondary)]"
        >
          <RefreshCw className="h-4 w-4" />
          刷新
        </button>
      </div>

      {data?.summary ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
            <p className="mb-1 text-xs text-[var(--color-text-secondary)]">总流水笔数</p>
            <p className="text-xl font-medium">{data.summary.totalActivityCount.toLocaleString()}</p>
          </div>
          <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
            <p className="mb-1 text-xs text-[var(--color-text-secondary)]">充值总额</p>
            <p className="text-xl font-medium text-green-400">{formatCurrency(data.summary.totalRechargeAmountCents)}</p>
          </div>
          <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
            <p className="mb-1 text-xs text-[var(--color-text-secondary)]">累计入账积分</p>
            <p className="text-xl font-medium text-green-400">+{data.summary.totalCreditIn.toLocaleString()}</p>
          </div>
          <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4">
            <p className="mb-1 text-xs text-[var(--color-text-secondary)]">AI 已计费积分</p>
            <p className="text-xl font-medium text-orange-400">-{data.summary.totalDebitedCredits.toLocaleString()}</p>
          </div>
        </div>
      ) : null}

      <div className="space-y-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4">
        <form
          onSubmit={(event) => {
            event.preventDefault();
            setQuery(searchInput.trim());
            setPage(1);
          }}
          className="relative max-w-md"
        >
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            placeholder="搜索用户、模型、订单号或引用 ID"
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] py-2 pl-9 pr-4 text-sm focus:border-[var(--color-primary)] focus:outline-none"
          />
        </form>

        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap gap-2">
            {KIND_OPTIONS.map((option) => (
              <button
                key={option.value}
                onClick={() => {
                  setKind(option.value);
                  setPage(1);
                }}
                className={`rounded-lg border px-3 py-1.5 text-xs transition-colors ${
                  kind === option.value
                    ? "border-[var(--color-primary)]/50 bg-[var(--color-primary)]/10 text-[var(--color-primary)]"
                    : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)]"
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>

          <div className="flex flex-col gap-2 sm:flex-row">
            <select
              value={jobType}
              onChange={(event) => {
                setJobType(event.target.value);
                setPage(1);
              }}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
            >
              {JOB_TYPE_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <select
              value={status}
              onChange={(event) => {
                setStatus(event.target.value);
                setPage(1);
              }}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
            >
              {STATUS_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      <div className="overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-xs uppercase text-[var(--color-text-secondary)]">
              <tr>
                <th className="px-5 py-3.5 font-medium">时间</th>
                <th className="px-5 py-3.5 font-medium">用户 / 邮箱</th>
                <th className="px-5 py-3.5 font-medium">记录类型</th>
                <th className="px-5 py-3.5 font-medium">业务</th>
                <th className="px-5 py-3.5 font-medium">细节</th>
                <th className="px-5 py-3.5 font-medium">状态</th>
                <th className="px-5 py-3.5 font-medium text-right">费用 / 用量</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading ? (
                <tr>
                  <td colSpan={7} className="px-6 py-12 text-center">
                    <Loader2 className="mx-auto h-6 w-6 animate-spin text-[var(--color-text-secondary)]" />
                    <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载财务流水中...</p>
                  </td>
                </tr>
              ) : error ? (
                <tr>
                  <td colSpan={7} className="px-6 py-10 text-center text-sm text-red-500">
                    加载失败，请重试
                  </td>
                </tr>
              ) : !data || data.items.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-6 py-12 text-center text-sm text-[var(--color-text-secondary)]">
                    暂无符合条件的财务流水
                  </td>
                </tr>
              ) : (
                data.items.map((row) => {
                  const amount = getActivityAmount(row.activity);
                  const meta = [row.activity.modelName, row.activity.meterName || row.activity.meterCode, row.activity.reference].filter(Boolean).join(" · ");
                  return (
                    <tr key={`${row.activity.kind}-${row.activity.id}`} className="transition-colors hover:bg-[var(--color-bg-secondary)]/50">
                      <td className="whitespace-nowrap px-5 py-3.5 text-xs text-[var(--color-text-secondary)]">
                        {formatDateTime(row.activity.occurredAt)}
                      </td>
                      <td className="px-5 py-3.5">
                        <div className="max-w-[180px] truncate text-sm" title={row.user.name}>
                          {row.user.name}
                        </div>
                        <div className="max-w-[220px] truncate text-xs text-[var(--color-text-secondary)]" title={row.user.email}>
                          {row.user.email}
                        </div>
                      </td>
                      <td className="px-5 py-3.5">
                        <div className="font-medium">{getActivityTypeLabel(row.activity)}</div>
                        <div className="mt-0.5 text-xs text-[var(--color-text-secondary)]">{row.activity.kind}</div>
                      </td>
                      <td className="px-5 py-3.5">{getBusinessLabel(row.activity)}</td>
                      <td className="px-5 py-3.5">
                        <div className="max-w-[260px] truncate" title={row.activity.detail || row.activity.title}>
                          {row.activity.detail || row.activity.title}
                        </div>
                        {meta ? <div className="mt-0.5 text-xs text-[var(--color-text-secondary)]">{meta}</div> : null}
                      </td>
                      <td className="px-5 py-3.5">{renderStatus(row.activity.status)}</td>
                      <td className={`px-5 py-3.5 text-right font-mono font-medium ${amount.tone}`}>
                        <div>{amount.text}</div>
                        {amount.meta ? <div className="mt-0.5 text-xs text-[var(--color-text-secondary)]">{amount.meta}</div> : null}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>

      {data?.pagination && data.pagination.totalPages > 1 ? (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">
            共 <span className="font-medium">{data.pagination.total}</span> 条流水
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
      ) : null}
    </div>
  );
}
