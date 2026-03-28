"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { motion } from "framer-motion";
import {
  ArrowUpRight,
  Coins,
  CreditCard,
  Loader2,
  ReceiptText,
  Search,
  Sparkles,
  Wallet,
} from "lucide-react";
import { EmptyState, PageHeader, StatCard, StatusBadge } from "@/components/ui/common";
import { getModelDisplayName } from "@/lib/model-display";
import { getBillingSummary, listBillingActivities } from "@/lib/services";
import type { BillingActivity, BillingActivityListResponse, BillingSummary } from "@/lib/types";

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
  consume: "算力消耗",
  refund: "退款返还",
  usage_refund: "任务失败返还积分",
  usage_return: "任务失败返还积分",
  grant: "赠送积分",
  manual_compensation: "人工补偿",
  manual_deduction: "人工扣减",
  admin_adjustment: "后台调账",
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

function formatCurrency(cents: number) {
  return `¥ ${(cents / 100).toFixed(2)}`;
}

function renderActivityStatus(status?: string | null) {
  if (!status) {
    return <span className="text-xs text-text-muted">已记账</span>;
  }
  if (status === "billed") {
    return <span className="inline-flex rounded-full bg-success/10 px-2.5 py-1 text-xs font-medium text-success">已计费</span>;
  }
  if (status === "returned" || status === "refunded") {
    return <span className="inline-flex rounded-full bg-info/10 px-2.5 py-1 text-xs font-medium text-info">已返还</span>;
  }
  return <StatusBadge status={status} />;
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

function getActivityTypeLabel(item: BillingActivity) {
  if (item.kind === "recharge_order") {
    return "充值订单";
  }
  if (item.kind === "wallet_ledger") {
    if ((item.creditDelta ?? 0) > 0) {
      return "钱包入账";
    }
    return "钱包扣减";
  }
  return "AI 计费";
}

function getActivityBusinessLabel(item: BillingActivity) {
  if (item.kind === "recharge_order") {
    return CHANNEL_LABELS[item.channel ?? ""] || item.channel || "充值";
  }
  if (item.kind === "wallet_ledger") {
    return ENTRY_TYPE_LABELS[item.entryType ?? ""] || item.entryType || "钱包账变";
  }
  if (item.jobType) {
    return JOB_TYPE_LABELS[item.jobType] || item.jobType;
  }
  if (item.sourceType) {
    return item.sourceType;
  }
  return item.meterName || item.meterCode || "AI 用量";
}

function getActivityAmount(item: BillingActivity) {
  if (item.kind === "recharge_order" && typeof item.amountCents === "number") {
    const credits = (item.creditAmount ?? 0) + (item.bonusCreditAmount ?? 0);
    return {
      text: formatCurrency(item.amountCents),
      meta: credits > 0 ? `到账 ${credits.toLocaleString("zh-CN")} 积分` : "",
      tone: "text-info",
    };
  }
  if (item.kind === "wallet_ledger" && typeof item.creditDelta === "number") {
    const isIncome = item.creditDelta > 0;
    return {
      text: `${isIncome ? "+" : ""}${item.creditDelta.toLocaleString("zh-CN")} 积分`,
      meta: "",
      tone: isIncome ? "text-success" : "text-warning",
    };
  }
  if (item.status === "returned" || item.status === "refunded") {
    const returnedCredits = getPayloadNumber(item, "returnedCredits") || getPayloadNumber(item, "refundedCredits");
    if (returnedCredits > 0) {
      return {
        text: `+${returnedCredits.toLocaleString("zh-CN")} 积分`,
        meta: "任务失败已自动返还积分",
        tone: "text-success",
      };
    }
  }
  if (typeof item.debitedCredits === "number") {
    return {
      text: `-${item.debitedCredits.toLocaleString("zh-CN")} 积分`,
      meta: typeof item.usageQuantity === "number" ? `${item.usageQuantity.toLocaleString("zh-CN")} ${item.meterName || item.meterCode || "单位"}` : "",
      tone: "text-warning",
    };
  }
  return {
    text: typeof item.usageQuantity === "number" ? `${item.usageQuantity.toLocaleString("zh-CN")} ${item.meterName || item.meterCode || "单位"}` : "—",
    meta: item.status === "failed" ? "本次未成功扣费" : "",
    tone: item.status === "failed" ? "text-danger" : "text-text-secondary",
  };
}

function buildActivityMeta(item: BillingActivity) {
  const modelLabel =
    item.modelName || item.modelAlias
      ? getModelDisplayName(item, "")
      : "";
  const parts = [modelLabel, item.meterName || item.meterCode, item.reference];
  return parts.filter(Boolean).join(" · ");
}

export default function FinancePage() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [kind, setKind] = useState("");
  const [jobType, setJobType] = useState("");
  const [status, setStatus] = useState("");

  const { data: summary, isLoading: summaryLoading } = useQuery<BillingSummary>({
    queryKey: ["billingSummary"],
    queryFn: getBillingSummary,
  });

  const {
    data: activitiesData,
    isLoading: activitiesLoading,
    error,
  } = useQuery<BillingActivityListResponse>({
    queryKey: ["billingActivities", { page, query, kind, jobType, status }],
    queryFn: () =>
      listBillingActivities({
        page,
        pageSize: 20,
        query: query || undefined,
        kind: kind || undefined,
        jobType: jobType || undefined,
        status: status || undefined,
      }),
  });

  const activeQuotaCount = (Array.isArray(summary?.quotaBalances) ? summary!.quotaBalances : []).filter((item) => item.remainingTotal > 0).length;

  return (
    <>
      <PageHeader
        title="财务管理"
        subtitle="统一查看 OmniDrive 里的聊天、作图、做视频、充值和钱包账变记录。"
        actions={
          <Link
            href="/top-up"
            className="inline-flex items-center gap-2 rounded-xl border border-border bg-surface px-4 py-2 text-sm font-medium text-text-primary transition-colors hover:border-accent hover:text-accent"
          >
            去充值
            <ArrowUpRight className="h-4 w-4" />
          </Link>
        }
      />

      <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-4">
        <StatCard
          label="钱包积分"
          value={summaryLoading ? "..." : (summary?.creditBalance ?? 0).toLocaleString("zh-CN")}
          change="可直接用于 AI 生成与任务执行"
          changeType="positive"
          icon={<Wallet className="h-5 w-5" />}
        />
        <StatCard
          label="冻结积分"
          value={summaryLoading ? "..." : (summary?.frozenCreditBalance ?? 0).toLocaleString("zh-CN")}
          change="等待最终结算的预留积分"
          changeType="neutral"
          icon={<Coins className="h-5 w-5" />}
        />
        <StatCard
          label="待处理充值"
          value={summaryLoading ? "..." : summary?.pendingRechargeCount ?? 0}
          change="包含待支付和人工审核中的订单"
          changeType="neutral"
          icon={<CreditCard className="h-5 w-5" />}
        />
        <StatCard
          label="AI 已计费"
          value={activitiesLoading ? "..." : (activitiesData?.summary.totalDebitedCredits ?? 0).toLocaleString("zh-CN")}
          change="聊天、作图、做视频的累计扣减积分"
          changeType="positive"
          icon={<Sparkles className="h-5 w-5" />}
        />
      </div>

      {summary && Array.isArray(summary.quotaBalances) && summary.quotaBalances.length > 0 ? (
        <div className="mb-6 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {summary.quotaBalances.map((quota, index) => (
            <motion.div
              key={`${quota.meterCode}-${quota.nearestExpiresAt ?? index}`}
              initial={{ opacity: 0, y: 14 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: index * 0.04 }}
              className="glass-card p-5"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-text-primary">{quota.meterName}</p>
                  <p className="mt-1 text-xs uppercase tracking-wide text-text-muted">{quota.meterCode}</p>
                </div>
                <div className="rounded-xl bg-accent/10 px-3 py-2 text-sm font-semibold text-accent">
                  {quota.remainingTotal.toLocaleString("zh-CN")} {quota.unit}
                </div>
              </div>
              <p className="mt-3 text-sm text-text-secondary">
                最近到期时间：{formatDateTime(quota.nearestExpiresAt)}
              </p>
            </motion.div>
          ))}
        </div>
      ) : null}

      <div className="mb-4 flex flex-col gap-3 rounded-2xl border border-border bg-surface/70 p-4">
        <form
          onSubmit={(event) => {
            event.preventDefault();
            setQuery(searchInput.trim());
            setPage(1);
          }}
          className="relative"
        >
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted" />
          <input
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            placeholder="搜索模型、业务、订单号或引用 ID"
            className="w-full rounded-xl border border-border bg-background pl-9 pr-4 py-2.5 text-sm text-text-primary outline-none transition-colors focus:border-accent"
          />
        </form>

        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div className="flex flex-wrap gap-2">
            {KIND_OPTIONS.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => {
                  setKind(option.value);
                  setPage(1);
                }}
                className={`rounded-full border px-3 py-1.5 text-xs font-medium transition-colors ${
                  kind === option.value
                    ? "border-accent bg-accent/10 text-accent"
                    : "border-border text-text-secondary hover:bg-surface-hover/60"
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
              className="rounded-xl border border-border bg-background px-3 py-2 text-sm text-text-primary outline-none transition-colors focus:border-accent"
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
              className="rounded-xl border border-border bg-background px-3 py-2 text-sm text-text-primary outline-none transition-colors focus:border-accent"
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

      <div className="glass-card overflow-hidden">
        <div className="flex items-center justify-between border-b border-border px-6 py-5">
          <div>
            <h2 className="text-base font-semibold text-text-primary">财务明细列表</h2>
            <p className="mt-1 text-sm text-text-secondary">统一展示聊天、作图、视频、充值和钱包变动。</p>
          </div>
          <div className="text-xs text-text-muted">共 {activitiesData?.pagination.total ?? 0} 条</div>
        </div>

        {activitiesLoading ? (
          <div className="flex min-h-72 items-center justify-center text-text-secondary">
            <Loader2 className="mr-3 h-5 w-5 animate-spin" />
            正在读取财务明细...
          </div>
        ) : error ? (
          <div className="p-6 text-sm text-danger">加载财务明细失败，请刷新页面重试。</div>
        ) : !activitiesData || !Array.isArray(activitiesData.items) || activitiesData.items.length === 0 ? (
          <div className="p-6">
            <EmptyState
              icon={<Wallet className="h-6 w-6" />}
              title="还没有财务记录"
              description="创建第一笔订单或产生第一次 AI 消费后，这里会自动展示完整明细。"
            />
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead className="border-b border-border bg-surface-hover/40 text-xs uppercase tracking-wider text-text-muted">
                  <tr>
                    <th className="px-6 py-4">时间</th>
                    <th className="px-6 py-4">记录类型</th>
                    <th className="px-6 py-4">业务</th>
                    <th className="px-6 py-4">细节</th>
                    <th className="px-6 py-4">状态</th>
                    <th className="px-6 py-4 text-right">费用 / 用量</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {activitiesData.items.map((item) => {
                    const amount = getActivityAmount(item);
                    const meta = buildActivityMeta(item);
                    return (
                      <tr key={`${item.kind}-${item.id}`} className="transition-colors hover:bg-surface-hover/20">
                        <td className="px-6 py-4 text-xs text-text-secondary">{formatDateTime(item.occurredAt)}</td>
                        <td className="px-6 py-4">
                          <div className="font-medium text-text-primary">{getActivityTypeLabel(item)}</div>
                          <div className="mt-1 text-xs text-text-muted">{item.kind}</div>
                        </td>
                        <td className="px-6 py-4">
                          <div className="font-medium text-text-primary">{getActivityBusinessLabel(item)}</div>
                        </td>
                        <td className="px-6 py-4">
                          <div className="max-w-sm truncate text-text-primary" title={item.detail || item.title}>
                            {item.detail || item.title}
                          </div>
                          {meta ? (
                            <div className="mt-1 text-xs text-text-muted">{meta}</div>
                          ) : null}
                        </td>
                        <td className="px-6 py-4">{renderActivityStatus(item.status)}</td>
                        <td className={`px-6 py-4 text-right font-mono font-semibold ${amount.tone}`}>
                          <div>{amount.text}</div>
                          {amount.meta ? <div className="mt-1 text-xs text-text-muted">{amount.meta}</div> : null}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>

            {activitiesData.pagination.totalPages > 1 ? (
              <div className="flex items-center justify-between border-t border-border px-6 py-4">
                <div className="text-sm text-text-secondary">
                  共 {activitiesData.pagination.total} 条，当前第 {activitiesData.pagination.page} / {activitiesData.pagination.totalPages} 页
                </div>
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={() => setPage((current) => Math.max(1, current - 1))}
                    disabled={page === 1}
                    className="rounded-lg border border-border px-3 py-1.5 text-sm text-text-primary transition-colors hover:bg-surface-hover/60 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    上一页
                  </button>
                  <button
                    type="button"
                    onClick={() => setPage((current) => Math.min(activitiesData.pagination.totalPages, current + 1))}
                    disabled={page >= activitiesData.pagination.totalPages}
                    className="rounded-lg border border-border px-3 py-1.5 text-sm text-text-primary transition-colors hover:bg-surface-hover/60 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    下一页
                  </button>
                </div>
              </div>
            ) : null}
          </>
        )}
      </div>

      <div className="mt-6 grid grid-cols-1 gap-4 md:grid-cols-4">
        <StatCard
          label="总记录数"
          value={(activitiesData?.summary.totalActivityCount ?? 0).toLocaleString("zh-CN")}
          change="订单、钱包账变和 AI 计费统一统计"
          changeType="neutral"
          icon={<ReceiptText className="h-5 w-5" />}
        />
        <StatCard
          label="充值总额"
          value={formatCurrency(activitiesData?.summary.totalRechargeAmountCents ?? 0)}
          change="当前筛选条件下的充值订单金额"
          changeType="positive"
          icon={<CreditCard className="h-5 w-5" />}
        />
        <StatCard
          label="入账积分"
          value={`+${(activitiesData?.summary.totalCreditIn ?? 0).toLocaleString("zh-CN")}`}
          change="钱包账变中的所有入账积分"
          changeType="positive"
          icon={<Coins className="h-5 w-5" />}
        />
        <StatCard
          label="可用配额"
          value={activeQuotaCount.toLocaleString("zh-CN")}
          change="仍有剩余次数或配额的套餐数量"
          changeType="neutral"
          icon={<Wallet className="h-5 w-5" />}
        />
      </div>
    </>
  );
}
