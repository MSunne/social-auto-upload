"use client";

import { useMemo, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { motion } from "framer-motion";
import {
  ArrowUpRight,
  BadgePercent,
  Building2,
  Copy,
  Loader2,
  ReceiptText,
  ShieldCheck,
  Sparkles,
  Users,
  Wallet,
} from "lucide-react";
import {
  createWithdrawalRequest,
  getPartnerOverview,
  listCommissionItems,
  listWithdrawalRequests,
  openPartnerProfile,
} from "@/lib/services";
import type { CommissionItem, PartnerOverview, WithdrawalRequest } from "@/lib/types";
import { EmptyState, PageHeader, StatCard, StatusBadge } from "@/components/ui/common";

type WithdrawalFormState = {
  amount: string;
  payoutChannel: string;
  accountMasked: string;
  note: string;
};

const EMPTY_WITHDRAWAL_FORM: WithdrawalFormState = {
  amount: "",
  payoutChannel: "wechat",
  accountMasked: "",
  note: "",
};

const PAYOUT_LABELS: Record<string, string> = {
  wechat: "微信收款",
  alipay: "支付宝",
  bank: "银行卡",
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
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatPercent(value: number) {
  return `${(value * 100).toFixed(value * 100 >= 10 ? 1 : 2)}%`;
}

async function copyText(value: string) {
  try {
    if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value);
      return true;
    }
  } catch {
    // Fall through to the textarea-based copy path.
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

function toErrorMessage(error: unknown) {
  if (error instanceof Error && error.message.trim()) {
    return error.message.trim();
  }
  return "操作失败，请稍后重试";
}

function parseAmountToCents(value: string) {
  const normalized = value.trim();
  if (!normalized) {
    return 0;
  }
  const parsed = Number.parseFloat(normalized);
  if (Number.isNaN(parsed) || parsed <= 0) {
    return 0;
  }
  return Math.round(parsed * 100);
}

export default function EnterpriseCooperationPage() {
  const queryClient = useQueryClient();
  const [copyState, setCopyState] = useState<"idle" | "success" | "error">("idle");
  const [withdrawalForm, setWithdrawalForm] = useState<WithdrawalFormState>(EMPTY_WITHDRAWAL_FORM);
  const [withdrawalError, setWithdrawalError] = useState("");

  const {
    data: overview,
    isLoading: overviewLoading,
  } = useQuery<PartnerOverview>({
    queryKey: ["partnerOverview"],
    queryFn: getPartnerOverview,
  });

  const {
    data: commissions = [],
    isLoading: commissionsLoading,
  } = useQuery<CommissionItem[]>({
    queryKey: ["commissionItems", { limit: 8 }],
    queryFn: () => listCommissionItems({ limit: 8 }),
  });

  const {
    data: withdrawals = [],
    isLoading: withdrawalsLoading,
  } = useQuery<WithdrawalRequest[]>({
    queryKey: ["withdrawalRequests", { limit: 8 }],
    queryFn: () => listWithdrawalRequests({ limit: 8 }),
  });

  const summary = overview?.summary;
  const profile = overview?.profile ?? null;

  const activationMutation = useMutation({
    mutationFn: openPartnerProfile,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["partnerOverview"] }),
        queryClient.invalidateQueries({ queryKey: ["commissionItems"] }),
        queryClient.invalidateQueries({ queryKey: ["withdrawalRequests"] }),
      ]);
    },
  });

  const withdrawalMutation = useMutation({
    mutationFn: (payload: {
      amountCents: number;
      payoutChannel: string;
      accountMasked: string;
      accountPayload: Record<string, unknown>;
      note?: string;
    }) => createWithdrawalRequest(payload),
    onSuccess: async () => {
      setWithdrawalForm(EMPTY_WITHDRAWAL_FORM);
      setWithdrawalError("");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["partnerOverview"] }),
        queryClient.invalidateQueries({ queryKey: ["withdrawalRequests"] }),
      ]);
    },
  });

  const pendingWithdrawalAmount = useMemo(() => {
    if (!summary) {
      return 0;
    }
    return summary.requestedWithdrawalAmountCents + summary.approvedWithdrawalAmountCents;
  }, [summary]);

  const handleCopyCode = async () => {
    if (!profile?.partnerCode) {
      return;
    }
    const copied = await copyText(profile.partnerCode);
    setCopyState(copied ? "success" : "error");
  };

  const handleSubmitWithdrawal = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setWithdrawalError("");

    const amountCents = parseAmountToCents(withdrawalForm.amount);
    if (amountCents <= 0) {
      setWithdrawalError("请输入正确的提现金额");
      return;
    }
    if (amountCents > (summary?.availableWithdrawalAmountCents ?? 0)) {
      setWithdrawalError("提现金额不能超过当前可提现金额");
      return;
    }
    if (!withdrawalForm.accountMasked.trim()) {
      setWithdrawalError("请输入收款账号");
      return;
    }

    withdrawalMutation.mutate({
      amountCents,
      payoutChannel: withdrawalForm.payoutChannel,
      accountMasked: withdrawalForm.accountMasked.trim(),
      accountPayload: {
        payoutChannel: withdrawalForm.payoutChannel,
        accountMasked: withdrawalForm.accountMasked.trim(),
      },
      note: withdrawalForm.note.trim() || undefined,
    });
  };

  return (
    <>
      <PageHeader
        title="企业合作"
        subtitle="把你的专属客服码分享给新用户，他们注册并消费后，合作业绩和佣金会自动沉淀到这里。"
        actions={
          <Link
            href="/top-up"
            className="inline-flex items-center gap-2 rounded-xl border border-border bg-surface px-4 py-2 text-sm font-medium text-text-primary transition-colors hover:border-accent hover:text-accent"
          >
            查看充值中心
            <ArrowUpRight className="h-4 w-4" />
          </Link>
        }
      />

      {overviewLoading ? (
        <div className="glass-card flex min-h-[420px] items-center justify-center text-text-secondary">
          <Loader2 className="mr-3 h-5 w-5 animate-spin" />
          正在加载企业合作数据...
        </div>
      ) : !profile ? (
        <div className="glass-card p-6">
          <EmptyState
            icon={<Building2 className="h-6 w-6" />}
            title="还没有开通企业合作"
            description="开通后你会获得一个专属客服码。新用户注册时填写这个客服码，后续消费会自动计入你的企业合作账本。"
            action={
              <button
                type="button"
                onClick={() => activationMutation.mutate()}
                disabled={activationMutation.isPending}
                className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2.5 text-sm font-semibold text-background shadow-lg shadow-accent/20 transition-all hover:shadow-xl hover:shadow-accent/30 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {activationMutation.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Sparkles className="h-4 w-4" />
                )}
                立即开通企业合作
              </button>
            }
          />
          {activationMutation.isError ? (
            <p className="mt-4 text-center text-sm text-danger">
              {toErrorMessage(activationMutation.error)}
            </p>
          ) : null}
        </div>
      ) : (
        <>
          <div className="mb-6 grid gap-4 xl:grid-cols-[1.35fr_1fr]">
            <motion.div
              initial={{ opacity: 0, y: 14 }}
              animate={{ opacity: 1, y: 0 }}
              className="glass-card glow-border p-6"
            >
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <div className="inline-flex items-center gap-2 rounded-full border border-accent/20 bg-accent/10 px-3 py-1 text-xs font-medium text-accent">
                    <ShieldCheck className="h-3.5 w-3.5" />
                    企业合作已开通
                  </div>
                  <h2 className="mt-4 text-2xl font-bold tracking-tight text-text-primary">
                    {profile.partnerName}
                  </h2>
                  <p className="mt-2 max-w-2xl text-sm leading-6 text-text-secondary">
                    当前合作客户注册时填写你的专属客服码后，聊天按 token 计费、图片与视频按作业计费产生的消费，都会统一进入合作账本。
                  </p>
                </div>

                <div className="rounded-2xl border border-accent/20 bg-surface/80 p-4 lg:min-w-[260px]">
                  <div className="text-xs uppercase tracking-[0.2em] text-text-muted">
                    专属客服码
                  </div>
                  <div className="mt-3 flex items-center gap-3">
                    <div className="font-mono text-2xl font-bold tracking-[0.22em] text-text-primary">
                      {profile.partnerCode}
                    </div>
                    <button
                      type="button"
                      onClick={handleCopyCode}
                      className="inline-flex h-10 w-10 items-center justify-center rounded-xl border border-border bg-surface text-text-secondary transition-colors hover:border-accent hover:text-accent"
                      title="复制客服码"
                    >
                      <Copy className="h-4 w-4" />
                    </button>
                  </div>
                  <p className="mt-3 text-xs text-text-muted">
                    {copyState === "success"
                      ? "客服码已复制"
                      : copyState === "error"
                        ? "复制失败，请手动复制"
                        : "注册页填写这个客服码，就会自动绑定到你的合作账本。"}
                  </p>
                  <div className="mt-4 flex items-center justify-between">
                    <StatusBadge status={profile.status} />
                    <span className="text-xs text-text-muted">
                      结算门槛 {formatCurrency(summary?.settlementThresholdCents ?? 0)}
                    </span>
                  </div>
                </div>
              </div>
            </motion.div>

            <motion.div
              initial={{ opacity: 0, y: 14 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.04 }}
              className="glass-card p-6"
            >
              <h3 className="text-base font-semibold text-text-primary">合作说明</h3>
              <div className="mt-4 space-y-3 text-sm leading-6 text-text-secondary">
                <p>1. 新用户注册时填写你的专属客服码，会自动和你的企业合作账号绑定。</p>
                <p>2. 用户消费后，消费金额会转成“已生效合作金额”，对应佣金进入“待结算”。</p>
                <p>3. 后台完成结算后，佣金会进入“已结算 / 可提现”，你可以在这里提交提现申请。</p>
              </div>
            </motion.div>
          </div>

          <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
            <StatCard
              label="合作客户数"
              value={summary?.inviteeCount ?? 0}
              change="已绑定到当前客服码的注册用户"
              changeType="positive"
              icon={<Users className="h-5 w-5" />}
            />
            <StatCard
              label="当前佣金比例"
              value={formatPercent(summary?.currentCommissionRate ?? 0)}
              change="后台规则实时生效"
              changeType="neutral"
              icon={<BadgePercent className="h-5 w-5" />}
            />
            <StatCard
              label="待生效合作金额"
              value={formatCurrency(summary?.pendingConsumeBaseAmountCents ?? 0)}
              change="已充值但尚未形成有效消费"
              changeType="neutral"
              icon={<ReceiptText className="h-5 w-5" />}
            />
            <StatCard
              label="已生效合作金额"
              value={formatCurrency(summary?.activatedBaseAmountCents ?? 0)}
              change="已被实际消费的累计充值金额"
              changeType="positive"
              icon={<Sparkles className="h-5 w-5" />}
            />
            <StatCard
              label="待结算佣金"
              value={formatCurrency(summary?.pendingSettlementAmountCents ?? 0)}
              change="等待后台结算入账"
              changeType="neutral"
              icon={<Wallet className="h-5 w-5" />}
            />
            <StatCard
              label="已结算佣金"
              value={formatCurrency(summary?.settledAmountCents ?? 0)}
              change="后台已确认可提的佣金"
              changeType="positive"
              icon={<ShieldCheck className="h-5 w-5" />}
            />
            <StatCard
              label="可提现佣金"
              value={formatCurrency(summary?.availableWithdrawalAmountCents ?? 0)}
              change={`处理中 ${formatCurrency(pendingWithdrawalAmount)}`}
              changeType="neutral"
              icon={<ArrowUpRight className="h-5 w-5" />}
            />
            <StatCard
              label="已打款金额"
              value={formatCurrency(summary?.paidWithdrawalAmountCents ?? 0)}
              change="已完成提现的累计金额"
              changeType="positive"
              icon={<Building2 className="h-5 w-5" />}
            />
          </div>

          <div className="grid gap-6 xl:grid-cols-[1.4fr_1fr]">
            <div className="glass-card overflow-hidden">
              <div className="flex items-center justify-between border-b border-border px-6 py-5">
                <div>
                  <h2 className="text-base font-semibold text-text-primary">最近合作佣金</h2>
                  <p className="mt-1 text-sm text-text-secondary">
                    这里会展示每位客户最近一笔充值形成的合作业绩与佣金流转情况。
                  </p>
                </div>
                <div className="text-sm text-text-muted">最近 8 条</div>
              </div>

              {commissionsLoading ? (
                <div className="flex min-h-72 items-center justify-center text-text-secondary">
                  <Loader2 className="mr-3 h-5 w-5 animate-spin" />
                  正在读取佣金明细...
                </div>
              ) : commissions.length === 0 ? (
                <div className="p-6">
                  <EmptyState
                    icon={<Users className="h-6 w-6" />}
                    title="还没有合作佣金"
                    description="当新用户通过你的客服码注册并产生消费后，这里会自动出现合作业绩和佣金明细。"
                  />
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-sm">
                    <thead className="border-b border-border bg-surface-hover/40 text-xs uppercase tracking-wider text-text-muted">
                      <tr>
                        <th className="px-6 py-4">客户</th>
                        <th className="px-6 py-4">状态</th>
                        <th className="px-6 py-4 text-right">合作金额</th>
                        <th className="px-6 py-4 text-right">佣金</th>
                        <th className="px-6 py-4 text-right">已释放</th>
                        <th className="px-6 py-4">时间</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border">
                      {commissions.map((item) => (
                        <tr key={item.id} className="transition-colors hover:bg-surface-hover/20">
                          <td className="px-6 py-4">
                            <div className="font-medium text-text-primary">{item.inviteeName || "未命名客户"}</div>
                            <div className="mt-1 text-xs text-text-muted">{item.inviteeEmail}</div>
                          </td>
                          <td className="px-6 py-4">
                            <StatusBadge status={item.status} />
                            <div className="mt-1 text-xs text-text-muted">
                              佣金比例 {formatPercent(item.commissionRate)}
                            </div>
                          </td>
                          <td className="px-6 py-4 text-right font-mono text-text-primary">
                            {formatCurrency(item.commissionBaseAmountCents)}
                          </td>
                          <td className="px-6 py-4 text-right font-mono text-text-primary">
                            {formatCurrency(item.amountCents)}
                          </td>
                          <td className="px-6 py-4 text-right font-mono text-text-primary">
                            {formatCurrency(item.releasedAmountCents)}
                          </td>
                          <td className="px-6 py-4 text-xs text-text-secondary">
                            <div>{formatDateTime(item.createdAt)}</div>
                            <div className="mt-1 font-mono text-[11px] text-text-muted">
                              {item.rechargeOrderNo || item.rechargeOrderId}
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>

            <div className="space-y-6">
              <div className="glass-card p-6">
                <h2 className="text-base font-semibold text-text-primary">申请提现</h2>
                <p className="mt-1 text-sm text-text-secondary">
                  结算完成后的佣金会进入可提现余额。提交申请后，后台会进行人工审核和打款。
                </p>

                <div className="mt-4 rounded-2xl border border-border bg-surface/70 p-4">
                  <div className="text-xs uppercase tracking-[0.16em] text-text-muted">当前可提现</div>
                  <div className="mt-2 text-3xl font-bold tracking-tight text-text-primary">
                    {formatCurrency(summary?.availableWithdrawalAmountCents ?? 0)}
                  </div>
                </div>

                <form onSubmit={handleSubmitWithdrawal} className="mt-5 space-y-4">
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-text-secondary">
                      提现金额
                    </label>
                    <input
                      type="text"
                      value={withdrawalForm.amount}
                      onChange={(event) =>
                        setWithdrawalForm((current) => ({
                          ...current,
                          amount: event.target.value,
                        }))
                      }
                      placeholder="例如 168.00"
                      className="w-full rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary placeholder-text-muted outline-none transition-all focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
                    />
                  </div>

                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-text-secondary">
                      收款方式
                    </label>
                    <select
                      value={withdrawalForm.payoutChannel}
                      onChange={(event) =>
                        setWithdrawalForm((current) => ({
                          ...current,
                          payoutChannel: event.target.value,
                        }))
                      }
                      className="w-full rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary outline-none transition-all focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
                    >
                      {Object.entries(PAYOUT_LABELS).map(([value, label]) => (
                        <option key={value} value={value}>
                          {label}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-text-secondary">
                      收款账号
                    </label>
                    <input
                      type="text"
                      value={withdrawalForm.accountMasked}
                      onChange={(event) =>
                        setWithdrawalForm((current) => ({
                          ...current,
                          accountMasked: event.target.value,
                        }))
                      }
                      placeholder="填写收款账号或收款人信息"
                      className="w-full rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary placeholder-text-muted outline-none transition-all focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
                    />
                  </div>

                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-text-secondary">
                      备注说明
                    </label>
                    <textarea
                      value={withdrawalForm.note}
                      onChange={(event) =>
                        setWithdrawalForm((current) => ({
                          ...current,
                          note: event.target.value,
                        }))
                      }
                      rows={3}
                      placeholder="可填写收款备注、联系人或其他说明"
                      className="w-full rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary placeholder-text-muted outline-none transition-all focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
                    />
                  </div>

                  {withdrawalError ? (
                    <div className="rounded-xl border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">
                      {withdrawalError}
                    </div>
                  ) : null}

                  {withdrawalMutation.isError ? (
                    <div className="rounded-xl border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">
                      {toErrorMessage(withdrawalMutation.error)}
                    </div>
                  ) : null}

                  <button
                    type="submit"
                    disabled={withdrawalMutation.isPending || (summary?.availableWithdrawalAmountCents ?? 0) <= 0}
                    className="inline-flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-3 text-sm font-semibold text-background shadow-lg shadow-accent/20 transition-all hover:shadow-xl hover:shadow-accent/30 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    {withdrawalMutation.isPending ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Wallet className="h-4 w-4" />
                    )}
                    提交提现申请
                  </button>
                </form>
              </div>

              <div className="glass-card overflow-hidden">
                <div className="border-b border-border px-6 py-5">
                  <h2 className="text-base font-semibold text-text-primary">最近提现申请</h2>
                  <p className="mt-1 text-sm text-text-secondary">展示最近 8 条提现申请状态。</p>
                </div>

                {withdrawalsLoading ? (
                  <div className="flex min-h-56 items-center justify-center text-text-secondary">
                    <Loader2 className="mr-3 h-5 w-5 animate-spin" />
                    正在读取提现申请...
                  </div>
                ) : withdrawals.length === 0 ? (
                  <div className="p-6">
                    <EmptyState
                      icon={<Wallet className="h-6 w-6" />}
                      title="还没有提现申请"
                      description="当可提现佣金累计到合适金额后，可以直接在这里提交提现。"
                    />
                  </div>
                ) : (
                  <div className="divide-y divide-border">
                    {withdrawals.map((item) => (
                      <div key={item.id} className="px-6 py-4">
                        <div className="flex items-start justify-between gap-3">
                          <div>
                            <div className="font-medium text-text-primary">
                              {formatCurrency(item.amountCents)}
                            </div>
                            <div className="mt-1 text-xs text-text-muted">
                              {PAYOUT_LABELS[item.payoutChannel ?? ""] || item.payoutChannel || "未设置"}
                              {item.accountMasked ? ` · ${item.accountMasked}` : ""}
                            </div>
                          </div>
                          <StatusBadge status={item.status} />
                        </div>
                        <div className="mt-3 flex items-center justify-between text-xs text-text-muted">
                          <span>{formatDateTime(item.createdAt)}</span>
                          <span>{item.note || "无备注"}</span>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </div>
        </>
      )}
    </>
  );
}
