"use client";

import { useState } from "react";
import type { AxiosError } from "axios";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Building2,
  Copy,
  Loader2,
  RefreshCw,
  Search,
  Sparkles,
  Users,
  Wallet,
} from "lucide-react";
import { api } from "@/lib/api";

type AdminUserSummary = {
  id: string;
  email: string;
  name: string;
};

type AdminPartnerProfileRow = {
  user: AdminUserSummary;
  partnerCode: string;
  partnerName: string;
  status: string;
  currentCommissionRate: number;
  settlementThresholdCents: number;
  inviteeCount: number;
  pendingSettlementAmountCents: number;
  settledAmountCents: number;
  availableWithdrawalAmountCents: number;
  createdAt: string;
  updatedAt: string;
};

type AdminPartnerProfileSummary = {
  totalCount: number;
  activeCount: number;
  inviteeCount: number;
  pendingSettlementAmountCents: number;
  settledAmountCents: number;
};

type AdminPartnerListResponse = {
  items: AdminPartnerProfileRow[];
  summary: AdminPartnerProfileSummary;
};

function formatCurrency(cents: number) {
  return `¥ ${(cents / 100).toFixed(2)}`;
}

function formatPercent(rate: number) {
  return `${(rate * 100).toFixed(rate * 100 >= 10 ? 1 : 2)}%`;
}

function formatDateTime(value: string) {
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

async function copyText(value: string) {
  try {
    if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value);
      return true;
    }
  } catch {
    // Fall back to textarea copy when clipboard API is unavailable.
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
  const axiosError = error as AxiosError<{ error?: { message?: string } }>;
  const apiMessage = axiosError.response?.data?.error?.message;
  if (typeof apiMessage === "string" && apiMessage.trim()) {
    return apiMessage.trim();
  }
  if (error instanceof Error && error.message.trim()) {
    return error.message.trim();
  }
  return "请求失败，请稍后重试";
}

export default function DistributionPartnersPage() {
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("active");
  const [userIdToOpen, setUserIdToOpen] = useState("");
  const [copyState, setCopyState] = useState("");

  const partnerQuery = useQuery<AdminPartnerListResponse>({
    queryKey: ["admin-partners", query, status],
    queryFn: async () => {
      const { data } = await api.get<AdminPartnerListResponse>("/distribution/partners", {
        params: {
          page: 1,
          pageSize: 50,
          query: query.trim() || undefined,
          status: status === "all" ? undefined : status,
        },
      });
      return data;
    },
  });

  const openMutation = useMutation({
    mutationFn: async (userId: string) => {
      const { data } = await api.post("/distribution/partners", { userId });
      return data;
    },
    onSuccess: async () => {
      setUserIdToOpen("");
      await queryClient.invalidateQueries({ queryKey: ["admin-partners"] });
    },
  });

  const summary = partnerQuery.data?.summary;
  const items = partnerQuery.data?.items ?? [];

  return (
    <div className="space-y-6">
      <section className="rounded-[28px] border border-white/10 bg-white/5 p-8 shadow-[0_20px_80px_rgba(0,0,0,0.35)]">
        <div className="flex flex-col gap-6 xl:flex-row xl:items-start xl:justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/70">
              Enterprise Cooperation
            </p>
            <h1 className="mt-3 text-3xl font-semibold tracking-tight text-white">
              企业合作伙伴
            </h1>
            <p className="mt-4 max-w-3xl text-sm leading-7 text-slate-300">
              这里集中管理已经开通企业合作入口的用户，查看合作码、当前佣金比例、绑定客户数，以及待结算与已结算金额。
            </p>
          </div>

          <div className="grid gap-3 sm:grid-cols-2 xl:min-w-[420px]">
            <div className="rounded-3xl border border-white/10 bg-[rgba(8,12,18,0.78)] p-5">
              <div className="text-xs uppercase tracking-[0.18em] text-slate-500">合作伙伴</div>
              <div className="mt-3 text-3xl font-semibold text-white">{summary?.totalCount ?? 0}</div>
              <div className="mt-2 text-sm text-slate-400">活跃 {summary?.activeCount ?? 0}</div>
            </div>
            <div className="rounded-3xl border border-white/10 bg-[rgba(8,12,18,0.78)] p-5">
              <div className="text-xs uppercase tracking-[0.18em] text-slate-500">绑定客户数</div>
              <div className="mt-3 text-3xl font-semibold text-white">{summary?.inviteeCount ?? 0}</div>
              <div className="mt-2 text-sm text-slate-400">已归属到企业合作账本</div>
            </div>
            <div className="rounded-3xl border border-white/10 bg-[rgba(8,12,18,0.78)] p-5">
              <div className="text-xs uppercase tracking-[0.18em] text-slate-500">待结算佣金</div>
              <div className="mt-3 text-3xl font-semibold text-white">
                {formatCurrency(summary?.pendingSettlementAmountCents ?? 0)}
              </div>
              <div className="mt-2 text-sm text-slate-400">等待财务结算</div>
            </div>
            <div className="rounded-3xl border border-white/10 bg-[rgba(8,12,18,0.78)] p-5">
              <div className="text-xs uppercase tracking-[0.18em] text-slate-500">已结算佣金</div>
              <div className="mt-3 text-3xl font-semibold text-white">
                {formatCurrency(summary?.settledAmountCents ?? 0)}
              </div>
              <div className="mt-2 text-sm text-slate-400">可继续进入提现流程</div>
            </div>
          </div>
        </div>
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.7fr_1fr]">
        <div className="rounded-[24px] border border-white/8 bg-[rgba(8,12,18,0.72)] p-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h2 className="text-lg font-semibold text-white">合作伙伴列表</h2>
              <p className="mt-2 text-sm text-slate-400">
                支持按用户、邮箱、合作码搜索，并查看当前佣金比例和资金状态。
              </p>
            </div>

            <div className="flex flex-col gap-3 sm:flex-row">
              <label className="flex items-center gap-2 rounded-2xl border border-white/10 bg-white/[0.04] px-4 py-3 text-sm text-slate-300">
                <Search className="h-4 w-4 text-slate-500" />
                <input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="搜索用户 / 邮箱 / 合作码"
                  className="w-full min-w-[220px] bg-transparent outline-none placeholder:text-slate-500"
                />
              </label>
              <select
                value={status}
                onChange={(event) => setStatus(event.target.value)}
                className="rounded-2xl border border-white/10 bg-white/[0.04] px-4 py-3 text-sm text-slate-200 outline-none"
              >
                <option value="all">全部状态</option>
                <option value="active">活跃</option>
                <option value="inactive">停用</option>
              </select>
              <button
                type="button"
                onClick={() => partnerQuery.refetch()}
                className="inline-flex items-center justify-center gap-2 rounded-2xl border border-white/10 bg-white/[0.04] px-4 py-3 text-sm text-slate-200 transition-colors hover:bg-white/[0.08]"
              >
                {partnerQuery.isFetching ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <RefreshCw className="h-4 w-4" />
                )}
                刷新
              </button>
            </div>
          </div>

          <div className="mt-5 overflow-hidden rounded-3xl border border-white/8">
            <div className="grid grid-cols-[1.4fr_1fr_0.9fr_0.9fr_1fr_1fr] gap-4 border-b border-white/8 bg-white/[0.04] px-5 py-3 text-xs uppercase tracking-[0.18em] text-slate-500">
              <span>伙伴</span>
              <span>合作码</span>
              <span>佣金比例</span>
              <span>绑定客户</span>
              <span>待结算</span>
              <span>已结算 / 可提</span>
            </div>

            {partnerQuery.isLoading ? (
              <div className="flex min-h-[280px] items-center justify-center text-sm text-slate-400">
                <Loader2 className="mr-3 h-4 w-4 animate-spin" />
                正在加载企业合作伙伴...
              </div>
            ) : partnerQuery.isError ? (
              <div className="flex min-h-[220px] items-center justify-center px-6 text-center text-sm text-rose-200">
                {toErrorMessage(partnerQuery.error)}
              </div>
            ) : items.length === 0 ? (
              <div className="flex min-h-[220px] flex-col items-center justify-center gap-3 text-sm text-slate-400">
                <Building2 className="h-8 w-8 text-slate-600" />
                当前还没有匹配到企业合作伙伴
              </div>
            ) : (
              <div className="divide-y divide-white/8">
                {items.map((item) => (
                  <div
                    key={item.partnerCode}
                    className="grid grid-cols-[1.4fr_1fr_0.9fr_0.9fr_1fr_1fr] gap-4 px-5 py-4 text-sm text-slate-200"
                  >
                    <div className="min-w-0">
                      <div className="font-medium text-white">{item.partnerName}</div>
                      <div className="mt-1 truncate text-xs text-slate-400">
                        {item.user.name} / {item.user.email || item.user.id}
                      </div>
                      <div className="mt-1 text-xs text-slate-500">
                        开通于 {formatDateTime(item.createdAt)}
                      </div>
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm tracking-[0.18em] text-cyan-200">
                          {item.partnerCode}
                        </span>
                        <button
                          type="button"
                          onClick={async () => {
                            const copied = await copyText(item.partnerCode);
                            setCopyState(copied ? item.partnerCode : "");
                          }}
                          className="rounded-lg border border-white/10 p-1.5 text-slate-400 transition-colors hover:border-cyan-300/30 hover:text-cyan-200"
                          title="复制合作码"
                        >
                          <Copy className="h-3.5 w-3.5" />
                        </button>
                      </div>
                      <div className="mt-1 text-xs text-slate-500">
                        {copyState === item.partnerCode ? "已复制" : item.status === "active" ? "活跃中" : "已停用"}
                      </div>
                    </div>
                    <div>
                      <div className="font-medium text-white">
                        {formatPercent(item.currentCommissionRate)}
                      </div>
                      <div className="mt-1 text-xs text-slate-500">
                        门槛 {formatCurrency(item.settlementThresholdCents)}
                      </div>
                    </div>
                    <div>
                      <div className="inline-flex items-center gap-2 text-white">
                        <Users className="h-4 w-4 text-cyan-200" />
                        {item.inviteeCount}
                      </div>
                    </div>
                    <div className="font-medium text-amber-200">
                      {formatCurrency(item.pendingSettlementAmountCents)}
                    </div>
                    <div>
                      <div className="font-medium text-emerald-200">
                        {formatCurrency(item.settledAmountCents)}
                      </div>
                      <div className="mt-1 inline-flex items-center gap-2 text-xs text-slate-400">
                        <Wallet className="h-3.5 w-3.5" />
                        可提 {formatCurrency(item.availableWithdrawalAmountCents)}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <div className="rounded-[24px] border border-white/8 bg-[rgba(8,12,18,0.72)] p-6">
          <div className="inline-flex items-center gap-2 rounded-full border border-cyan-300/20 bg-cyan-300/10 px-3 py-1 text-xs uppercase tracking-[0.18em] text-cyan-100">
            <Sparkles className="h-3.5 w-3.5" />
            企业合作开通
          </div>
          <h2 className="mt-4 text-lg font-semibold text-white">代用户开通企业合作</h2>
          <p className="mt-2 text-sm leading-7 text-slate-400">
            当合作伙伴是线下签约或由运营代维护时，可以直接填写用户 ID，为对方开通企业合作身份并生成专属合作码。
          </p>

          <div className="mt-5 space-y-3">
            <label className="block">
              <div className="mb-2 text-xs uppercase tracking-[0.18em] text-slate-500">
                用户 ID
              </div>
              <input
                value={userIdToOpen}
                onChange={(event) => setUserIdToOpen(event.target.value)}
                placeholder="输入要开通的用户 ID"
                className="w-full rounded-2xl border border-white/10 bg-white/[0.04] px-4 py-3 text-sm text-slate-100 outline-none placeholder:text-slate-500"
              />
            </label>

            <button
              type="button"
              onClick={() => openMutation.mutate(userIdToOpen.trim())}
              disabled={!userIdToOpen.trim() || openMutation.isPending}
              className="inline-flex w-full items-center justify-center gap-2 rounded-2xl bg-cyan-300 px-4 py-3 text-sm font-semibold text-slate-950 transition-colors hover:bg-cyan-200 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {openMutation.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Sparkles className="h-4 w-4" />
              )}
              立即开通企业合作
            </button>
          </div>

          {openMutation.isError ? (
            <p className="mt-4 rounded-2xl border border-rose-400/20 bg-rose-400/10 px-4 py-3 text-sm text-rose-200">
              {toErrorMessage(openMutation.error)}
            </p>
          ) : null}

          {openMutation.isSuccess ? (
            <p className="mt-4 rounded-2xl border border-emerald-400/20 bg-emerald-400/10 px-4 py-3 text-sm text-emerald-200">
              企业合作已开通，列表已自动刷新。
            </p>
          ) : null}
        </div>
      </section>
    </div>
  );
}
