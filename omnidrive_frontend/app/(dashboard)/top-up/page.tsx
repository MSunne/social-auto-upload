"use client";

import { useMemo, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { AnimatePresence, motion } from "framer-motion";
import {
  ArrowUpRight,
  Copy,
  CreditCard,
  Gift,
  Loader2,
  QrCode,
  ReceiptText,
  ShieldCheck,
  Wallet,
  X,
} from "lucide-react";
import {
  createRechargeOrder,
  getBillingSummary,
  listBillingPackages,
  listRechargeOrders,
  submitManualRecharge,
} from "@/lib/services";
import type { BillingPackage, BillingSummary, RechargeOrder } from "@/lib/types";
import { EmptyState, PageHeader, StatCard, StatusBadge } from "@/components/ui/common";
import { cn } from "@/lib/utils";

type ManualFormState = {
  contactChannel: string;
  contactHandle: string;
  paymentReference: string;
  transferAmount: string;
  proofUrls: string;
  customerNote: string;
};

const CHANNEL_LABELS: Record<string, string> = {
  manual_cs: "客服充值",
  alipay: "支付宝",
  wechatpay: "微信支付",
};

const CHANNEL_BUTTON_STYLES: Record<string, string> = {
  manual_cs:
    "border-warning/40 bg-gradient-to-r from-warning/15 to-amber-500/10 text-warning hover:from-warning/20 hover:to-amber-500/15",
  alipay: "border-info/30 bg-info/10 text-info hover:bg-info/15",
  wechatpay: "border-success/30 bg-success/10 text-success hover:bg-success/15",
};

const EMPTY_MANUAL_FORM: ManualFormState = {
  contactChannel: "",
  contactHandle: "",
  paymentReference: "",
  transferAmount: "",
  proofUrls: "",
  customerNote: "",
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

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function getNestedRecord(payload: Record<string, unknown> | null, key: string) {
  if (!payload) {
    return null;
  }
  return asRecord(payload[key]);
}

function getString(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function getStringArray(value: unknown) {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => (typeof item === "string" ? item.trim() : ""))
    .filter(Boolean);
}

function parseAmountToCents(value: string) {
  const normalized = value.trim();
  if (!normalized) {
    return undefined;
  }
  const parsed = Number.parseFloat(normalized);
  if (Number.isNaN(parsed) || parsed <= 0) {
    return undefined;
  }
  return Math.round(parsed * 100);
}

async function copyText(value: string) {
  try {
    if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value);
      return true;
    }
  } catch {
    // Fall back to the textarea copy path below.
  }

  if (typeof document === "undefined") {
    return false;
  }

  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "absolute";
  textarea.style.left = "-9999px";
  document.body.appendChild(textarea);
  textarea.select();

  try {
    return document.execCommand("copy");
  } finally {
    document.body.removeChild(textarea);
  }
}

function buildManualForm(order: RechargeOrder | null): ManualFormState {
  const payload = asRecord(order?.customerServicePayload) ?? {};
  const submission = getNestedRecord(payload, "submission");
  const transferAmountCents = submission?.transferAmountCents;

  return {
    contactChannel: getString(submission?.contactChannel),
    contactHandle: getString(submission?.contactHandle),
    paymentReference: getString(submission?.paymentReference),
    transferAmount:
      typeof transferAmountCents === "number" && transferAmountCents > 0
        ? (transferAmountCents / 100).toFixed(2)
        : "",
    proofUrls: getStringArray(submission?.proofUrls).join("\n"),
    customerNote: getString(submission?.customerNote),
  };
}

function getManualSupport(order: RechargeOrder | null) {
  const payload = asRecord(order?.customerServicePayload);
  return getNestedRecord(payload, "support");
}

function getManualSubmission(order: RechargeOrder | null) {
  const payload = asRecord(order?.customerServicePayload);
  return getNestedRecord(payload, "submission");
}

export default function TopUpPage() {
  const queryClient = useQueryClient();
  const [selectedOrderId, setSelectedOrderId] = useState<string | null>(null);
  const [activationOrder, setActivationOrder] = useState<RechargeOrder | null>(null);
  const [activationCopied, setActivationCopied] = useState<"idle" | "success" | "error">("idle");
  const [manualDraft, setManualDraft] = useState<{
    orderId: string | null;
    form: ManualFormState;
  }>({
    orderId: null,
    form: EMPTY_MANUAL_FORM,
  });

  const { data: summary, isLoading: summaryLoading } = useQuery<BillingSummary>({
    queryKey: ["billingSummary"],
    queryFn: getBillingSummary,
  });

  const { data: packages = [], isLoading: packagesLoading } = useQuery<BillingPackage[]>({
    queryKey: ["billingPackages"],
    queryFn: listBillingPackages,
  });

  const { data: orders = [], isLoading: ordersLoading } = useQuery<RechargeOrder[]>({
    queryKey: ["rechargeOrders", { limit: 20 }],
    queryFn: () => listRechargeOrders({ limit: 20 }),
  });

  const sortedPackages = useMemo(
    () => [...packages].sort((left, right) => left.sortOrder - right.sortOrder),
    [packages],
  );

  const manualOrders = useMemo(
    () => orders.filter((item) => item.channel === "manual_cs"),
    [orders],
  );

  const activeOrder =
    orders.find((item) => item.id === selectedOrderId) ??
    manualOrders[0] ??
    orders[0] ??
    null;

  const activeSupport = getManualSupport(activeOrder);
  const activeSubmission = getManualSubmission(activeOrder);
  const manualForm =
    manualDraft.orderId === activeOrder?.id ? manualDraft.form : buildManualForm(activeOrder);
  const activeOrderAllowsManualSubmit =
    activeOrder?.channel === "manual_cs" &&
    activeOrder.status !== "credited" &&
    activeOrder.status !== "paid" &&
    activeOrder.status !== "success" &&
    activeOrder.status !== "completed" &&
    activeOrder.status !== "closed";

  const copyActivationCode = async (code: string) => {
    const copied = await copyText(code);
    setActivationCopied(copied ? "success" : "error");
  };

  const createOrderMutation = useMutation({
    mutationFn: (payload: { packageId: string; channel: string; subject?: string }) =>
      createRechargeOrder(payload),
    onSuccess: async (order, variables) => {
      setSelectedOrderId(order.id);
      setManualDraft({
        orderId: order.id,
        form: buildManualForm(order),
      });
      if (variables.channel === "manual_cs") {
        setActivationOrder(order);
        setActivationCopied("idle");
        await copyActivationCode(order.orderNo);
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["billingSummary"] }),
        queryClient.invalidateQueries({ queryKey: ["rechargeOrders"] }),
      ]);
    },
    onError: (error) => {
      window.alert(error instanceof Error ? error.message : "创建充值订单失败，请稍后重试");
    },
  });

  const submitManualMutation = useMutation({
    mutationFn: (payload: {
      orderId: string;
      contactChannel: string;
      contactHandle: string;
      paymentReference: string;
      transferAmountCents?: number;
      proofUrls: string[];
      customerNote: string;
    }) =>
      submitManualRecharge(payload.orderId, {
        contactChannel: payload.contactChannel,
        contactHandle: payload.contactHandle,
        paymentReference: payload.paymentReference,
        transferAmountCents: payload.transferAmountCents,
        proofUrls: payload.proofUrls,
        customerNote: payload.customerNote,
      }),
    onSuccess: async (order) => {
      setSelectedOrderId(order.id);
      setManualDraft({
        orderId: order.id,
        form: buildManualForm(order),
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["billingSummary"] }),
        queryClient.invalidateQueries({ queryKey: ["rechargeOrders"] }),
      ]);
    },
    onError: (error) => {
      window.alert(error instanceof Error ? error.message : "提交客服充值资料失败，请稍后重试");
    },
  });

  const handleCreateOrder = (pkg: BillingPackage, channel: string) => {
    createOrderMutation.mutate({
      packageId: pkg.id,
      channel,
      subject: `${pkg.name} 充值`,
    });
  };

  const handleManualSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!activeOrder || activeOrder.channel !== "manual_cs") {
      return;
    }

    const proofUrls = manualForm.proofUrls
      .split("\n")
      .map((item) => item.trim())
      .filter(Boolean);

    if (!manualForm.paymentReference.trim() && proofUrls.length === 0) {
      window.alert("请至少填写付款参考号或上传一条凭证链接。");
      return;
    }

    submitManualMutation.mutate({
      orderId: activeOrder.id,
      contactChannel: manualForm.contactChannel.trim(),
      contactHandle: manualForm.contactHandle.trim(),
      paymentReference: manualForm.paymentReference.trim(),
      transferAmountCents: parseAmountToCents(manualForm.transferAmount),
      proofUrls,
      customerNote: manualForm.customerNote.trim(),
    });
  };

  const updateManualForm = (field: keyof ManualFormState, value: string) => {
    setManualDraft({
      orderId: activeOrder?.id ?? null,
      form: {
        ...manualForm,
        [field]: value,
      },
    });
  };

  return (
    <>
      <PageHeader
        title="充值中心"
        subtitle="选择套餐后直接创建充值订单。客服充值会先弹出充值激活码，在线支付保留原订单链路。"
        actions={
          <Link
            href="/finance"
            className="inline-flex items-center gap-2 rounded-xl border border-border bg-surface px-4 py-2 text-sm font-medium text-text-primary transition-colors hover:border-accent hover:text-accent"
          >
            查看财务流水
            <ArrowUpRight className="h-4 w-4" />
          </Link>
        }
      />

      <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-3">
        <StatCard
          label="钱包积分"
          value={summaryLoading ? "..." : (summary?.creditBalance ?? 0).toLocaleString("zh-CN")}
          change="支付完成后会自动入账积分"
          changeType="positive"
          icon={<Wallet className="h-5 w-5" />}
        />
        <StatCard
          label="待处理充值"
          value={summaryLoading ? "..." : summary?.pendingRechargeCount ?? 0}
          change="包含待支付与人工审核中的订单"
          changeType="neutral"
          icon={<ReceiptText className="h-5 w-5" />}
        />
        <StatCard
          label="生效套餐额度"
          value={summaryLoading ? "..." : summary?.quotaBalances.length ?? 0}
          change="已到账的套餐次数会显示在这里"
          changeType="positive"
          icon={<ShieldCheck className="h-5 w-5" />}
        />
      </div>

      {packagesLoading ? (
        <div className="glass-card flex min-h-64 items-center justify-center p-6 text-text-secondary">
          <Loader2 className="mr-3 h-5 w-5 animate-spin" />
          正在读取充值套餐...
        </div>
      ) : sortedPackages.length === 0 ? (
        <EmptyState
          icon={<CreditCard className="h-6 w-6" />}
          title="当前没有可购买套餐"
          description="请先在后台开启支付渠道或启用充值套餐。"
        />
      ) : (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 xl:grid-cols-3">
          {sortedPackages.map((pkg, index) => (
            <motion.div
              key={pkg.id}
              initial={{ opacity: 0, y: 18 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: index * 0.05 }}
              className="glass-card flex h-full flex-col p-6"
            >
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="flex items-center gap-2">
                    <h2 className="text-lg font-semibold text-text-primary">{pkg.name}</h2>
                    {pkg.badge ? (
                      <span className="rounded-full bg-accent/15 px-2.5 py-1 text-xs font-medium text-accent">
                        {pkg.badge}
                      </span>
                    ) : null}
                  </div>
                  <p className="mt-2 text-sm leading-6 text-text-secondary">
                    {pkg.description || "适用于常规账号运营和 AI 内容生成场景。"}
                  </p>
                </div>
                <div className="rounded-2xl bg-accent/10 px-4 py-3 text-right">
                  <div className="text-2xl font-bold text-text-primary">{formatCurrency(pkg.priceCents)}</div>
                  <div className="mt-1 text-xs text-text-muted">
                    到账 {pkg.creditAmount.toLocaleString("zh-CN")} 积分
                  </div>
                </div>
              </div>

              <div className="mt-5 grid gap-3 sm:grid-cols-2">
                <div className="rounded-2xl border border-border bg-surface p-4">
                  <p className="text-xs uppercase tracking-wide text-text-muted">基础积分</p>
                  <p className="mt-2 text-xl font-semibold text-text-primary">
                    {pkg.creditAmount.toLocaleString("zh-CN")}
                  </p>
                </div>
                <div className="rounded-2xl border border-border bg-surface p-4">
                  <p className="text-xs uppercase tracking-wide text-text-muted">客服充值赠送</p>
                  <p className="mt-2 text-xl font-semibold text-text-primary">
                    {pkg.manualBonusCreditAmount > 0
                      ? `+${pkg.manualBonusCreditAmount.toLocaleString("zh-CN")}`
                      : "无赠送"}
                  </p>
                </div>
              </div>

              <div className="mt-5 flex-1 rounded-2xl border border-border/70 bg-surface/60 p-4">
                <p className="text-xs uppercase tracking-wide text-text-muted">套餐权益</p>
                <div className="mt-3 space-y-2">
                  {pkg.entitlements.length > 0 ? (
                    pkg.entitlements.map((item) => (
                      <div key={item.id} className="flex items-start justify-between gap-3 text-sm">
                        <div className="text-text-primary">
                          {item.meterName || item.meterCode}
                          {item.description ? (
                            <span className="ml-2 text-text-muted">{item.description}</span>
                          ) : null}
                        </div>
                        <div className="whitespace-nowrap font-medium text-accent">
                          {item.grantAmount.toLocaleString("zh-CN")}
                          {item.unit ? ` ${item.unit}` : ""}
                        </div>
                      </div>
                    ))
                  ) : (
                    <p className="text-sm text-text-muted">当前套餐暂无额外次数型权益。</p>
                  )}
                </div>
              </div>

              <div className="mt-5">
                <p className="mb-3 text-xs uppercase tracking-wide text-text-muted">选择充值渠道</p>
                <div className="flex flex-wrap gap-3">
                  {pkg.paymentChannels.map((channel) => {
                    const isManual = channel === "manual_cs";
                    return (
                      <button
                        key={`${pkg.id}-${channel}`}
                        type="button"
                        onClick={() => handleCreateOrder(pkg, channel)}
                        disabled={createOrderMutation.isPending}
                        className={cn(
                          "rounded-xl border px-4 py-2.5 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-60",
                          CHANNEL_BUTTON_STYLES[channel] ||
                            "border-border bg-surface text-text-primary hover:border-accent/30",
                        )}
                      >
                        <span className="flex items-center gap-2">
                          {isManual ? <Gift className="h-4 w-4" /> : <CreditCard className="h-4 w-4" />}
                          <span>{createOrderMutation.isPending ? "创建中..." : CHANNEL_LABELS[channel] || channel}</span>
                          {isManual ? (
                            <span className="rounded-full bg-warning/15 px-2 py-0.5 text-[10px] font-semibold tracking-wide text-warning">
                              额外优惠
                            </span>
                          ) : null}
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>
            </motion.div>
          ))}
        </div>
      )}

      <div className="mt-6 space-y-6">
        <div className="glass-card overflow-hidden">
          <div className="flex items-center justify-between border-b border-border px-6 py-5">
            <div>
              <h2 className="text-base font-semibold text-text-primary">充值记录</h2>
              <p className="mt-1 text-sm text-text-secondary">查看已创建订单、激活码和当前状态。</p>
            </div>
            <div className="text-xs text-text-muted">共 {orders.length} 条</div>
          </div>

          {ordersLoading ? (
            <div className="flex min-h-48 items-center justify-center text-text-secondary">
              <Loader2 className="mr-3 h-5 w-5 animate-spin" />
              正在读取订单...
            </div>
          ) : orders.length === 0 ? (
            <div className="p-6">
              <EmptyState
                icon={<ReceiptText className="h-6 w-6" />}
                title="暂无充值记录"
                description="创建第一笔充值订单后，这里会直接显示订单、激活码和当前状态。"
              />
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead className="border-b border-border bg-surface-hover/40 text-xs uppercase tracking-wider text-text-muted">
                  <tr>
                    <th className="px-6 py-4">时间</th>
                    <th className="px-6 py-4">充值方式</th>
                    <th className="px-6 py-4">套餐 / 激活码</th>
                    <th className="px-6 py-4">状态</th>
                    <th className="px-6 py-4 text-right">金额</th>
                    <th className="px-6 py-4 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {orders.map((order) => {
                    const isManual = order.channel === "manual_cs";
                    return (
                      <tr
                        key={order.id}
                        className={cn(
                          "transition-colors hover:bg-surface-hover/20",
                          activeOrder?.id === order.id ? "bg-surface-hover/15" : "",
                        )}
                      >
                        <td className="px-6 py-4 text-xs text-text-secondary">
                          {formatDateTime(order.createdAt)}
                        </td>
                        <td className="px-6 py-4">
                          <div className="flex items-center gap-2 font-medium text-text-primary">
                            {isManual ? <Gift className="h-4 w-4 text-warning" /> : <CreditCard className="h-4 w-4 text-accent" />}
                            {CHANNEL_LABELS[order.channel] || order.channel}
                          </div>
                          {isManual && order.manualBonusCreditAmount > 0 ? (
                            <div className="mt-1 text-xs text-warning">
                              额外赠送 {order.manualBonusCreditAmount.toLocaleString("zh-CN")} 积分
                            </div>
                          ) : null}
                        </td>
                        <td className="px-6 py-4">
                          <div className="font-medium text-text-primary">{order.subject}</div>
                          <div className="mt-1 font-mono text-xs text-text-muted">
                            {isManual ? "充值激活码" : "订单号"}：{order.orderNo}
                          </div>
                        </td>
                        <td className="px-6 py-4">
                          <StatusBadge status={order.status} />
                        </td>
                        <td className="px-6 py-4 text-right font-mono font-semibold text-text-primary">
                          {formatCurrency(order.amountCents)}
                        </td>
                        <td className="px-6 py-4">
                          <div className="flex justify-end gap-2">
                            <button
                              type="button"
                              onClick={() => {
                                setSelectedOrderId(order.id);
                                setManualDraft({
                                  orderId: order.id,
                                  form: buildManualForm(order),
                                });
                              }}
                              className="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-text-primary transition-colors hover:border-accent/30 hover:text-accent"
                            >
                              查看
                            </button>
                            {isManual ? (
                              <button
                                type="button"
                                onClick={() => {
                                  void copyActivationCode(order.orderNo);
                                  setActivationOrder(order);
                                }}
                                className="rounded-lg border border-warning/30 bg-warning/10 px-3 py-1.5 text-xs font-medium text-warning transition-colors hover:bg-warning/15"
                              >
                                复制激活码
                              </button>
                            ) : null}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {activeOrder?.channel === "manual_cs" ? (
          <div className="space-y-6">
            <div className="glass-card p-6">
              <div className="flex flex-col gap-4 border-b border-border pb-5 md:flex-row md:items-center md:justify-between">
                <div>
                  <h2 className="text-base font-semibold text-text-primary">客服充值资料</h2>
                  <p className="mt-1 text-sm text-text-secondary">当前订单的激活码、客服联系方式和提交状态。</p>
                </div>
                <div className="flex items-center gap-3">
                  <StatusBadge status={activeOrder.status} />
                  <button
                    type="button"
                    onClick={() => void copyActivationCode(activeOrder.orderNo)}
                    className="inline-flex items-center gap-2 rounded-xl border border-warning/30 bg-warning/10 px-4 py-2 text-sm font-medium text-warning transition-colors hover:bg-warning/15"
                  >
                    <Copy className="h-4 w-4" />
                    复制激活码
                  </button>
                </div>
              </div>

              <div className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                <div className="rounded-2xl border border-border bg-surface p-4">
                  <p className="text-xs uppercase tracking-wide text-text-muted">充值激活码</p>
                  <p className="mt-2 break-all font-mono text-sm font-semibold text-text-primary">{activeOrder.orderNo}</p>
                </div>
                <div className="rounded-2xl border border-border bg-surface p-4">
                  <p className="text-xs uppercase tracking-wide text-text-muted">充值金额</p>
                  <p className="mt-2 text-lg font-semibold text-text-primary">{formatCurrency(activeOrder.amountCents)}</p>
                </div>
                <div className="rounded-2xl border border-border bg-surface p-4">
                  <p className="text-xs uppercase tracking-wide text-text-muted">到账积分</p>
                  <p className="mt-2 text-lg font-semibold text-text-primary">
                    {(activeOrder.creditAmount + activeOrder.manualBonusCreditAmount).toLocaleString("zh-CN")}
                  </p>
                </div>
                <div className="rounded-2xl border border-border bg-surface p-4">
                  <p className="text-xs uppercase tracking-wide text-text-muted">提交状态</p>
                  <p className="mt-2 text-sm font-medium text-text-primary">{getString(activeSubmission?.status) || "pending"}</p>
                </div>
              </div>

              <div className="mt-4 grid gap-4 md:grid-cols-2">
                <div className="rounded-2xl border border-border bg-surface/70 p-4 text-sm">
                  <p className="text-xs uppercase tracking-wide text-text-muted">客服联系方式</p>
                  <p className="mt-2 font-medium text-text-primary">
                    {getString(activeSupport?.name) || "人工客服"} · {getString(activeSupport?.contact) || "请联系后台配置客服联系方式"}
                  </p>
                  {getString(activeSupport?.note) ? (
                    <p className="mt-2 leading-6 text-text-secondary">{getString(activeSupport?.note)}</p>
                  ) : null}
                </div>
                <div className="rounded-2xl border border-border bg-surface/70 p-4 text-sm">
                  <p className="text-xs uppercase tracking-wide text-text-muted">时间信息</p>
                  <div className="mt-2 space-y-2 text-text-secondary">
                    <div className="flex items-center justify-between gap-3">
                      <span>创建时间</span>
                      <span className="font-medium text-text-primary">{formatDateTime(activeOrder.createdAt)}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span>更新时间</span>
                      <span className="font-medium text-text-primary">{formatDateTime(activeOrder.updatedAt)}</span>
                    </div>
                    {getString(activeSubmission?.submittedAt) ? (
                      <div className="flex items-center justify-between gap-3">
                        <span>最近提交</span>
                        <span className="font-medium text-text-primary">{formatDateTime(getString(activeSubmission?.submittedAt))}</span>
                      </div>
                    ) : null}
                  </div>
                </div>
              </div>

              {getString(activeSupport?.qrCodeUrl) ? (
                <div className="mt-4 flex items-center gap-3 rounded-2xl border border-border bg-surface px-4 py-4 text-sm">
                  <div className="rounded-lg bg-info/10 p-2 text-info">
                    <QrCode className="h-4 w-4" />
                  </div>
                  <a
                    href={getString(activeSupport?.qrCodeUrl)}
                    target="_blank"
                    rel="noreferrer"
                    className="font-medium text-accent hover:underline"
                  >
                    打开收款二维码
                  </a>
                </div>
              ) : null}
            </div>

            {activeOrderAllowsManualSubmit ? (
              <form onSubmit={handleManualSubmit} className="glass-card space-y-4 p-6">
                <div className="border-b border-border pb-5">
                  <h2 className="text-base font-semibold text-text-primary">提交付款凭证</h2>
                  <p className="mt-1 text-sm text-text-secondary">填写已付款信息后，订单会进入后台客服审核流程。</p>
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <label className="block text-sm">
                    <span className="mb-2 block text-text-secondary">联系渠道</span>
                    <input
                      value={manualForm.contactChannel}
                      onChange={(event) => updateManualForm("contactChannel", event.target.value)}
                      placeholder="例如：微信 / 企业微信"
                      className="w-full rounded-xl border border-border bg-background px-4 py-3 text-text-primary outline-none transition-colors focus:border-accent/40"
                    />
                  </label>
                  <label className="block text-sm">
                    <span className="mb-2 block text-text-secondary">联系账号</span>
                    <input
                      value={manualForm.contactHandle}
                      onChange={(event) => updateManualForm("contactHandle", event.target.value)}
                      placeholder="例如：support_user"
                      className="w-full rounded-xl border border-border bg-background px-4 py-3 text-text-primary outline-none transition-colors focus:border-accent/40"
                    />
                  </label>
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <label className="block text-sm">
                    <span className="mb-2 block text-text-secondary">付款参考号</span>
                    <input
                      value={manualForm.paymentReference}
                      onChange={(event) => updateManualForm("paymentReference", event.target.value)}
                      placeholder="银行流水号 / 转账单号"
                      className="w-full rounded-xl border border-border bg-background px-4 py-3 text-text-primary outline-none transition-colors focus:border-accent/40"
                    />
                  </label>
                  <label className="block text-sm">
                    <span className="mb-2 block text-text-secondary">实付金额</span>
                    <input
                      value={manualForm.transferAmount}
                      onChange={(event) => updateManualForm("transferAmount", event.target.value)}
                      placeholder="单位元，例如 299.00"
                      className="w-full rounded-xl border border-border bg-background px-4 py-3 text-text-primary outline-none transition-colors focus:border-accent/40"
                    />
                  </label>
                </div>

                <label className="block text-sm">
                  <span className="mb-2 block text-text-secondary">凭证链接</span>
                  <textarea
                    value={manualForm.proofUrls}
                    onChange={(event) => updateManualForm("proofUrls", event.target.value)}
                    placeholder={"每行一条链接，例如\nhttps://example.com/proof-1.png"}
                    rows={4}
                    className="w-full rounded-xl border border-border bg-background px-4 py-3 text-text-primary outline-none transition-colors focus:border-accent/40"
                  />
                </label>

                <label className="block text-sm">
                  <span className="mb-2 block text-text-secondary">补充说明</span>
                  <textarea
                    value={manualForm.customerNote}
                    onChange={(event) => updateManualForm("customerNote", event.target.value)}
                    placeholder="例如：已联系微信客服，尾号 2481 的公司账户付款。"
                    rows={3}
                    className="w-full rounded-xl border border-border bg-background px-4 py-3 text-text-primary outline-none transition-colors focus:border-accent/40"
                  />
                </label>

                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className={cn(
                    "text-sm",
                    activationCopied === "success" && "text-success",
                    activationCopied === "error" && "text-warning",
                    activationCopied === "idle" && "text-text-muted",
                  )}>
                    {activationCopied === "success"
                      ? "充值激活码已复制。"
                      : activationCopied === "error"
                        ? "自动复制失败，可手动点击复制。"
                        : "提交前可再次复制充值激活码。 "}
                  </div>
                  <div className="flex gap-3">
                    <button
                      type="button"
                      onClick={() => void copyActivationCode(activeOrder.orderNo)}
                      className="inline-flex items-center gap-2 rounded-xl border border-warning/30 bg-warning/10 px-4 py-2.5 text-sm font-medium text-warning transition-colors hover:bg-warning/15"
                    >
                      <Copy className="h-4 w-4" />
                      复制激活码
                    </button>
                    <button
                      type="submit"
                      disabled={submitManualMutation.isPending}
                      className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2.5 text-sm font-semibold text-background transition-opacity disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      {submitManualMutation.isPending ? (
                        <>
                          <Loader2 className="h-4 w-4 animate-spin" />
                          提交中...
                        </>
                      ) : (
                        "提交付款凭证"
                      )}
                    </button>
                  </div>
                </div>
              </form>
            ) : null}
          </div>
        ) : null}
      </div>

      <AnimatePresence>
        {activationOrder ? (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6">
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="fixed inset-0 bg-black/60 backdrop-blur-md"
              onClick={() => setActivationOrder(null)}
            />

            <motion.div
              initial={{ opacity: 0, scale: 0.96, y: 20 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.96, y: 20 }}
              className="relative z-10 w-full max-w-lg overflow-hidden rounded-3xl border border-white/10 bg-[#0A0A14]/95 shadow-2xl backdrop-blur-xl"
            >
              <div className="border-b border-white/8 bg-gradient-to-r from-warning/15 via-warning/10 to-transparent px-6 py-5">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <div className="inline-flex items-center gap-2 rounded-full border border-warning/30 bg-warning/10 px-3 py-1 text-xs font-semibold text-warning">
                      <Gift className="h-3.5 w-3.5" />
                      客服充值专享优惠
                    </div>
                    <h3 className="mt-3 text-xl font-semibold text-white">充值激活码已生成</h3>
                    <p className="mt-2 text-sm leading-6 text-white/70">
                      激活码已经自动复制，请将此码发送给客服即可。后续审核和入账都会绑定这笔订单。
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => setActivationOrder(null)}
                    className="rounded-full bg-white/6 p-2 text-white/70 transition-colors hover:bg-white/12 hover:text-white"
                    title="关闭"
                  >
                    <X className="h-4 w-4" />
                  </button>
                </div>
              </div>

              <div className="space-y-5 p-6">
                <div className="rounded-3xl border border-warning/30 bg-warning/10 p-5 text-center">
                  <p className="text-xs uppercase tracking-[0.25em] text-warning/80">Recharge Code</p>
                  <div className="mt-3 break-all font-mono text-2xl font-semibold tracking-[0.12em] text-white">
                    {activationOrder.orderNo}
                  </div>
                </div>

                <div className="grid gap-3 sm:grid-cols-3">
                  <div className="rounded-2xl border border-white/8 bg-white/4 p-4">
                    <p className="text-xs uppercase tracking-wide text-white/45">套餐</p>
                    <p className="mt-2 text-sm font-medium text-white">{activationOrder.subject}</p>
                  </div>
                  <div className="rounded-2xl border border-white/8 bg-white/4 p-4">
                    <p className="text-xs uppercase tracking-wide text-white/45">金额</p>
                    <p className="mt-2 text-sm font-medium text-white">{formatCurrency(activationOrder.amountCents)}</p>
                  </div>
                  <div className="rounded-2xl border border-white/8 bg-white/4 p-4">
                    <p className="text-xs uppercase tracking-wide text-white/45">到账积分</p>
                    <p className="mt-2 text-sm font-medium text-white">
                      {(activationOrder.creditAmount + activationOrder.manualBonusCreditAmount).toLocaleString("zh-CN")}
                    </p>
                  </div>
                </div>

                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div
                    className={cn(
                      "text-sm",
                      activationCopied === "success" && "text-success",
                      activationCopied === "error" && "text-warning",
                      activationCopied === "idle" && "text-white/60",
                    )}
                  >
                    {activationCopied === "success"
                      ? "充值激活码已复制，联系客服发送此码即可。"
                      : activationCopied === "error"
                        ? "自动复制失败，请点击右侧按钮重新复制。"
                        : "正在准备复制充值激活码..."}
                  </div>
                  <div className="flex gap-3">
                    <button
                      type="button"
                      onClick={() => void copyActivationCode(activationOrder.orderNo)}
                      className="inline-flex items-center gap-2 rounded-xl border border-white/12 bg-white/6 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-white/10"
                    >
                      <Copy className="h-4 w-4" />
                      再次复制
                    </button>
                    <button
                      type="button"
                      onClick={() => setActivationOrder(null)}
                      className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2.5 text-sm font-semibold text-background"
                    >
                      我知道了
                    </button>
                  </div>
                </div>
              </div>
            </motion.div>
          </div>
        ) : null}
      </AnimatePresence>
    </>
  );
}
