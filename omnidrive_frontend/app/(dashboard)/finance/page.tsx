"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { motion } from "framer-motion";
import {
  ArrowUpRight,
  Coins,
  CreditCard,
  Loader2,
  ReceiptText,
  Wallet,
} from "lucide-react";
import { EmptyState, PageHeader, StatCard, StatusBadge } from "@/components/ui/common";
import { getBillingSummary, listRechargeOrders, listWalletLedger } from "@/lib/services";
import type { BillingSummary, RechargeOrder, WalletLedger } from "@/lib/types";

const ENTRY_TYPE_LABELS: Record<string, string> = {
  recharge: "充值入账",
  consume: "算力消耗",
  refund: "退款返还",
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

type FinanceActivityItem = {
  id: string;
  occurredAt: string;
  kind: "order" | "ledger";
  eventLabel: string;
  businessLabel: string;
  detail: string;
  reference: string;
  status?: string;
  amountText: string;
  amountTone: string;
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

function formatLedgerType(type: string) {
  return ENTRY_TYPE_LABELS[type] || type;
}

function buildFinanceActivities(orders: RechargeOrder[], ledger: WalletLedger[]) {
  const orderItems: FinanceActivityItem[] = orders.map((order) => ({
    id: `order-${order.id}`,
    occurredAt: order.createdAt,
    kind: "order",
    eventLabel: "新建订单",
    businessLabel: CHANNEL_LABELS[order.channel] || order.channel,
    detail: order.subject,
    reference: order.orderNo,
    status: order.status,
    amountText: formatCurrency(order.amountCents),
    amountTone: "text-info",
  }));

  const ledgerItems: FinanceActivityItem[] = ledger.map((item) => {
    const isIncome = item.amountDelta > 0;
    return {
      id: `ledger-${item.id}`,
      occurredAt: item.createdAt,
      kind: "ledger",
      eventLabel: item.entryType === "consume" ? "消费" : isIncome ? "入账" : "账变",
      businessLabel: formatLedgerType(item.entryType),
      detail: item.description || item.referenceType || "钱包变更",
      reference: item.referenceId || item.id,
      amountText: `${isIncome ? "+" : ""}${item.amountDelta.toLocaleString("zh-CN")} 积分`,
      amountTone: isIncome ? "text-success" : "text-warning",
    };
  });

  return [...orderItems, ...ledgerItems].sort((left, right) => {
    return new Date(right.occurredAt).getTime() - new Date(left.occurredAt).getTime();
  });
}

export default function FinancePage() {
  const { data: summary, isLoading: summaryLoading } = useQuery<BillingSummary>({
    queryKey: ["billingSummary"],
    queryFn: getBillingSummary,
  });

  const { data: ledger = [], isLoading: ledgerLoading } = useQuery<WalletLedger[]>({
    queryKey: ["walletLedger", { limit: 40 }],
    queryFn: () => listWalletLedger({ limit: 40 }),
  });

  const { data: orders = [], isLoading: ordersLoading } = useQuery<RechargeOrder[]>({
    queryKey: ["rechargeOrders", { limit: 20 }],
    queryFn: () => listRechargeOrders({ limit: 20 }),
  });

  const activities = useMemo(() => buildFinanceActivities(orders, ledger), [orders, ledger]);
  const activeQuotaCount = summary?.quotaBalances.filter((item) => item.remainingTotal > 0).length ?? 0;

  return (
    <>
      <PageHeader
        title="财务管理"
        subtitle="直接看财务流水明细，订单创建、消费、入账、订单类型和状态都汇总在一张表里。"
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
          label="生效套餐"
          value={summaryLoading ? "..." : activeQuotaCount}
          change="到账后的套餐次数会显示在这里"
          changeType="positive"
          icon={<ReceiptText className="h-5 w-5" />}
        />
      </div>

      {summary && summary.quotaBalances.length > 0 ? (
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

      <div className="glass-card overflow-hidden">
        <div className="flex items-center justify-between border-b border-border px-6 py-5">
          <div>
            <h2 className="text-base font-semibold text-text-primary">财务明细列表</h2>
            <p className="mt-1 text-sm text-text-secondary">统一展示新建订单、消费、入账、订单类型和业务细节。</p>
          </div>
          <div className="text-xs text-text-muted">共 {activities.length} 条</div>
        </div>

        {ledgerLoading || ordersLoading ? (
          <div className="flex min-h-72 items-center justify-center text-text-secondary">
            <Loader2 className="mr-3 h-5 w-5 animate-spin" />
            正在读取财务明细...
          </div>
        ) : activities.length === 0 ? (
          <div className="p-6">
            <EmptyState
              icon={<Wallet className="h-6 w-6" />}
              title="还没有财务记录"
              description="创建第一笔订单或产生第一次消费后，这里会自动展示完整明细。"
            />
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border bg-surface-hover/40 text-xs uppercase tracking-wider text-text-muted">
                <tr>
                  <th className="px-6 py-4">时间</th>
                  <th className="px-6 py-4">事件</th>
                  <th className="px-6 py-4">订单类型 / 业务</th>
                  <th className="px-6 py-4">细节</th>
                  <th className="px-6 py-4">状态</th>
                  <th className="px-6 py-4 text-right">金额 / 积分</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {activities.map((item) => (
                  <tr key={item.id} className="transition-colors hover:bg-surface-hover/20">
                    <td className="px-6 py-4 text-xs text-text-secondary">
                      {formatDateTime(item.occurredAt)}
                    </td>
                    <td className="px-6 py-4">
                      <div className="font-medium text-text-primary">{item.eventLabel}</div>
                    </td>
                    <td className="px-6 py-4">
                      <div className="font-medium text-text-primary">{item.businessLabel}</div>
                    </td>
                    <td className="px-6 py-4">
                      <div className="max-w-sm truncate text-text-primary" title={item.detail}>
                        {item.detail}
                      </div>
                      <div className="mt-1 font-mono text-xs text-text-muted">{item.reference}</div>
                    </td>
                    <td className="px-6 py-4">
                      {item.kind === "order" && item.status ? (
                        <StatusBadge status={item.status} />
                      ) : (
                        <span className="text-xs text-text-muted">已记账</span>
                      )}
                    </td>
                    <td className={`px-6 py-4 text-right font-mono font-semibold ${item.amountTone}`}>
                      {item.amountText}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </>
  );
}
