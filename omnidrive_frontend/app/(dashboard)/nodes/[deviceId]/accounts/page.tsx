"use client";

import { use, useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "framer-motion";
import {
  AlertTriangle,
  ArrowLeft,
  Clock3,
  Users,
  Layout,
  ListChecks,
  Sparkles,
  KeyRound,
  BadgeCheck,
  Trash2,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Plus,
  Loader2,
  X,
} from "lucide-react";
import Link from "next/link";
import {
  getDevice,
  listAccounts,
  listSkills,
  listTasks,
  deleteAccount,
  validateAccount,
} from "@/lib/services";
import type { Device, Account, Skill, Task, LoginSession } from "@/lib/types";
import { StatusBadge } from "@/components/ui/common";
import { AddAccountModal } from "@/components/ui/add-account-modal";

/* Platform icon config */
const PLATFORMS = [
  { key: "抖音", color: "text-pink-400", bg: "bg-pink-500/10" },
  { key: "视频号", color: "text-emerald-400", bg: "bg-emerald-500/10" },
  { key: "快手", color: "text-orange-400", bg: "bg-orange-500/10" },
  { key: "小红书", color: "text-rose-400", bg: "bg-rose-500/10" },
  { key: "TikTok", color: "text-cyan", bg: "bg-cyan/10" },
  { key: "Instagram", color: "text-fuchsia-400", bg: "bg-fuchsia-500/10" },
  { key: "Facebook", color: "text-blue-400", bg: "bg-blue-500/10" },
  { key: "YouTube", color: "text-red-400", bg: "bg-red-500/10" },
  { key: "Bilibili", color: "text-sky-400", bg: "bg-sky-500/10" },
];

function getErrorMessage(error: unknown, fallback: string): string {
  if (typeof error === "object" && error !== null && "message" in error) {
    const message = String(
      (error as { message?: string }).message || "",
    ).trim();
    if (message) {
      return message;
    }
  }
  return fallback;
}

function asDisplayText(value: unknown, fallback = "-") {
  if (typeof value === "string") {
    const trimmed = value.trim();
    return trimmed || fallback;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return fallback;
}

function getAccountDeleteUsage(account: Account | null) {
  const load = account?.load;
  const activeTaskCount =
    (load?.pendingTaskCount || 0) +
    (load?.runningTaskCount || 0) +
    (load?.needsVerifyTaskCount || 0) +
    (load?.cancelRequestedTaskCount || 0);
  const totalTaskCount = load?.taskCount || 0;
  const historicalTaskCount = Math.max(totalTaskCount - activeTaskCount, 0);
  const activeLoginSessionCount = load?.activeLoginSessionCount || 0;

  return {
    activeTaskCount,
    totalTaskCount,
    historicalTaskCount,
    activeLoginSessionCount,
    hasBlockingUsage: activeTaskCount > 0 || activeLoginSessionCount > 0,
  };
}

export default function DeviceAccountsPage({
  params,
}: {
  params: Promise<{ deviceId: string }>;
}) {
  const { deviceId } = use(params);
  const queryClient = useQueryClient();

  const { data: device } = useQuery<Device>({
    queryKey: ["device", deviceId],
    queryFn: () => getDevice(deviceId),
  });

  const { data: accounts = [] } = useQuery<Account[]>({
    queryKey: ["accounts", deviceId],
    queryFn: () => listAccounts(deviceId),
  });

  const { data: skills = [] } = useQuery<Skill[]>({
    queryKey: ["skills"],
    queryFn: () => listSkills(),
  });

  const { data: tasks = [] } = useQuery<Task[]>({
    queryKey: ["tasks"],
    queryFn: () => listTasks(),
  });

  /* Platform filter */
  const [platformFilter, setPlatformFilter] = useState<string | null>(null);

  /* Modal state */
  const [isAccountModalOpen, setIsAccountModalOpen] = useState(false);
  const [validationSession, setValidationSession] =
    useState<LoginSession | null>(null);
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null);
  const [deleteCountdown, setDeleteCountdown] = useState(5);
  const [deleteDialogError, setDeleteDialogError] = useState<string | null>(
    null,
  );

  const [isDeleting, setIsDeleting] = useState<string | null>(null);
  const [isValidating, setIsValidating] = useState<string | null>(null);

  const deleteTarget = useMemo(
    () => accounts.find((account) => account.id === deleteTargetId) || null,
    [accounts, deleteTargetId],
  );
  const deleteUsage = getAccountDeleteUsage(deleteTarget);

  useEffect(() => {
    if (!deleteTargetId) {
      setDeleteCountdown(5);
      setDeleteDialogError(null);
      return;
    }
    setDeleteCountdown(5);
    setDeleteDialogError(null);
  }, [deleteTargetId]);

  useEffect(() => {
    if (!deleteTarget || deleteUsage.hasBlockingUsage || deleteCountdown <= 0) {
      return;
    }
    const timer = window.setTimeout(() => {
      setDeleteCountdown((current) => Math.max(0, current - 1));
    }, 1000);
    return () => window.clearTimeout(timer);
  }, [deleteCountdown, deleteTarget, deleteUsage.hasBlockingUsage]);

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      setDeleteDialogError(null);
      setIsDeleting(deleteTarget.id);
      await deleteAccount(deleteTarget.id);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["accounts", deviceId] }),
        queryClient.invalidateQueries({ queryKey: ["device", deviceId] }),
        queryClient.invalidateQueries({ queryKey: ["tasks"] }),
      ]);
      setDeleteTargetId(null);
    } catch (err: unknown) {
      setDeleteDialogError(getErrorMessage(err, "解绑失败，请稍后重试"));
      await queryClient.invalidateQueries({ queryKey: ["accounts", deviceId] });
    } finally {
      setIsDeleting(null);
    }
  };

  const handleValidate = async (accountId: string) => {
    try {
      setIsValidating(accountId);
      const session = await validateAccount(accountId);
      queryClient.invalidateQueries({ queryKey: ["accounts", deviceId] });
      setValidationSession(session);
      setIsAccountModalOpen(true);
    } catch (err: unknown) {
      alert(getErrorMessage(err, "发起重新认证失败"));
    } finally {
      setIsValidating(null);
    }
  };

  /* Computed stats */
  const uniquePlatforms = new Set(
    accounts.map((a) => asDisplayText(a.platform, "未知平台")),
  );
  const deviceTasks = tasks.filter((t) => t.deviceId === deviceId);
  const platformCounts: Record<string, number> = {};
  accounts.forEach((a) => {
    const platformKey = asDisplayText(a.platform, "未知平台");
    platformCounts[platformKey] = (platformCounts[platformKey] || 0) + 1;
  });

  /* Filtered accounts */
  const filteredAccounts = platformFilter
    ? accounts.filter(
        (a) => asDisplayText(a.platform, "未知平台") === platformFilter,
      )
    : accounts;

  if (!device) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="skeleton h-8 w-48" />
      </div>
    );
  }

  return (
    <>
      {/* Top Bar */}
      <div className="mb-6">
        <Link
          href="/nodes"
          className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-accent to-cyan px-5 py-2.5 text-sm font-bold text-white shadow-lg shadow-accent/25 transition-all hover:shadow-xl hover:shadow-accent/35 hover:-translate-y-0.5 active:translate-y-0"
        >
          <ArrowLeft className="h-4 w-4" /> 返回列表
        </Link>
      </div>

      {/* ───── Summary Stats ───── */}
      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        {[
          {
            icon: <Users className="h-5 w-5 text-cyan" />,
            label: "账号数量",
            value: accounts.length,
            bg: "bg-cyan/10",
            border: "border-cyan/20",
          },
          {
            icon: <Layout className="h-5 w-5 text-accent" />,
            label: "平台数量",
            value: uniquePlatforms.size,
            bg: "bg-accent/10",
            border: "border-accent/20",
          },
          {
            icon: <ListChecks className="h-5 w-5 text-emerald-400" />,
            label: "任务数量",
            value: deviceTasks.length,
            bg: "bg-emerald-500/10",
            border: "border-emerald-500/20",
          },
          {
            icon: <Sparkles className="h-5 w-5 text-amber-400" />,
            label: "技能数量",
            value: skills.length,
            bg: "bg-amber-500/10",
            border: "border-amber-500/20",
          },
        ].map((stat) => (
          <motion.div
            key={stat.label}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            className={`glass-card flex items-center gap-4 px-5 py-4 border ${stat.border}`}
          >
            <div
              className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-xl ${stat.bg}`}
            >
              {stat.icon}
            </div>
            <div>
              <p className="text-xs text-text-muted">{stat.label}</p>
              <p className="text-2xl font-bold text-text-primary">
                {stat.value}
              </p>
            </div>
          </motion.div>
        ))}
      </div>

      {/* ───── Platform Breakdown ───── */}
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.05 }}
        className="mb-6 glass-card p-5"
      >
        <h3 className="mb-3 text-xs font-bold uppercase tracking-widest text-text-muted">
          各平台账号分布
        </h3>
        <div className="flex flex-wrap gap-2">
          {/* All button */}
          <button
            onClick={() => setPlatformFilter(null)}
            className={`inline-flex items-center gap-2 rounded-full border px-3.5 py-1.5 text-xs font-semibold transition-all cursor-pointer ${
              platformFilter === null
                ? "border-accent/50 bg-accent/15 text-accent shadow-[0_0_10px_rgba(177,73,255,0.15)]"
                : "border-border/60 text-text-muted hover:border-accent/30 hover:text-text-secondary"
            }`}
          >
            全部
            <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-white/15 text-[10px] font-bold">
              {accounts.length}
            </span>
          </button>
          {PLATFORMS.map((p) => {
            const count = platformCounts[p.key] || 0;
            const isActive = platformFilter === p.key;
            return (
              <button
                key={p.key}
                onClick={() =>
                  count > 0 && setPlatformFilter(isActive ? null : p.key)
                }
                className={`inline-flex items-center gap-2 rounded-full border px-3.5 py-1.5 text-xs font-medium transition-all ${
                  isActive
                    ? `${p.bg} ${p.color} border-current/40 shadow-[0_0_10px_rgba(255,255,255,0.06)] scale-105`
                    : count > 0
                      ? `${p.bg} ${p.color} border-current/20 cursor-pointer hover:scale-105 hover:shadow-[0_0_8px_rgba(255,255,255,0.04)]`
                      : "text-text-muted bg-surface/50 opacity-40 cursor-default"
                }`}
              >
                <span>{p.key}</span>
                <span
                  className={`flex h-5 min-w-5 items-center justify-center rounded-full text-[10px] font-bold ${
                    count > 0
                      ? "bg-white/15 text-current"
                      : "bg-surface text-text-muted"
                  }`}
                >
                  {count}
                </span>
              </button>
            );
          })}
        </div>
      </motion.div>

      {/* ───── Accounts Table ───── */}
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.1 }}
        className="glass-card glow-border p-0 overflow-hidden"
      >
        {/* Table Header */}
        <div className="flex items-center justify-between border-b border-border/50 px-6 py-5 bg-gradient-to-r from-surface-elevated/80 to-surface/50 backdrop-blur-md">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-cyan/10 border border-cyan/20">
              <Users className="h-5 w-5 text-cyan" />
            </div>
            <div>
              <h2 className="text-base font-bold text-text-primary uppercase tracking-wider">
                平台账号
              </h2>
              <p className="text-xs text-text-muted mt-0.5">
                共{" "}
                <span className="text-cyan font-semibold">
                  {accounts.length}
                </span>{" "}
                个账号已同步
              </p>
            </div>
          </div>
          <button
            onClick={() => setIsAccountModalOpen(true)}
            className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2 text-sm font-bold text-white shadow-lg shadow-accent/25 transition-all hover:shadow-xl hover:shadow-accent/35 hover:-translate-y-0.5 active:translate-y-0 cursor-pointer"
          >
            <Plus className="h-4 w-4" />
            增加账号
          </button>
        </div>

        {accounts.length > 0 ? (
          <div className="overflow-x-auto w-full">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b border-border/50 bg-surface/30">
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted">
                    账号名称
                  </th>
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted">
                    平台
                  </th>
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted">
                    添加时间
                  </th>
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted">
                    状态
                  </th>
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted">
                    心跳时间
                  </th>
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted">
                    任务表
                  </th>
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted text-center">
                    操作
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/30">
                {filteredAccounts.map((acc, idx) => {
                  const platformCfg = PLATFORMS.find(
                    (p) => p.key === acc.platform,
                  );
                  const hasActiveLoginSession =
                    (acc.load?.activeLoginSessionCount || 0) > 0;
                  const needsAttention =
                    acc.status !== "active" || hasActiveLoginSession;
                  return (
                    <motion.tr
                      key={acc.id}
                      initial={{ opacity: 0, x: -10 }}
                      animate={{ opacity: 1, x: 0 }}
                      transition={{ delay: 0.04 * idx }}
                      className="transition-colors hover:bg-accent/[0.03]"
                    >
                      {/* Account Name */}
                      <td className="px-6 py-4">
                        <span className="font-semibold text-text-primary">
                          {asDisplayText(acc.accountName, "未命名账号")}
                        </span>
                      </td>

                      {/* Platform */}
                      <td className="px-6 py-4">
                        <span
                          className={`inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold ${
                            platformCfg
                              ? `${platformCfg.bg} ${platformCfg.color}`
                              : "bg-surface text-text-muted"
                          }`}
                        >
                          {asDisplayText(acc.platform, "未知平台")}
                        </span>
                      </td>

                      {/* Created At */}
                      <td className="px-6 py-4 font-mono text-xs text-text-muted">
                        {new Date(acc.createdAt).toLocaleDateString("zh-CN")}
                      </td>

                      {/* Status */}
                      <td className="px-6 py-4">
                        <div className="space-y-1">
                          <StatusBadge status={acc.status} />
                          {hasActiveLoginSession && (
                            <div className="text-xs text-amber-400">
                              正在重新认证，等待二维码或二次认证
                            </div>
                          )}
                          {acc.lastMessage && (
                            <div
                              className={`max-w-xs truncate text-xs ${
                                needsAttention
                                  ? "text-amber-300"
                                  : "text-text-muted"
                              }`}
                              title={acc.lastMessage}
                            >
                              {asDisplayText(acc.lastMessage)}
                            </div>
                          )}
                        </div>
                      </td>

                      {/* Heartbeat Time */}
                      <td className="px-6 py-4 font-mono text-xs text-text-muted">
                        {acc.updatedAt
                          ? new Date(acc.updatedAt).toLocaleString("zh-CN", {
                              month: "2-digit",
                              day: "2-digit",
                              hour: "2-digit",
                              minute: "2-digit",
                            })
                          : "-"}
                      </td>

                      {/* Task Table */}
                      <td className="px-6 py-4">
                        <Link
                          href={`/nodes/${deviceId}/accounts/${acc.id}`}
                          className="inline-flex items-center gap-1.5 rounded-full bg-gradient-to-r from-accent/15 to-cyan/15 border border-accent/25 px-3 py-1.5 text-xs font-semibold text-accent transition-all hover:from-accent/25 hover:to-cyan/25 hover:border-accent/40 hover:shadow-[0_0_12px_rgba(177,73,255,0.2)] hover:-translate-y-px"
                        >
                          <ExternalLink className="h-3 w-3" />
                          详情/任务列表
                        </Link>
                      </td>

                      {/* Actions */}
                      <td className="px-6 py-4">
                        <div className="flex items-center justify-center gap-2">
                          {needsAttention ? (
                            <button
                              onClick={() => handleValidate(acc.id)}
                              disabled={isValidating === acc.id}
                              className="inline-flex items-center gap-1.5 rounded-full border border-amber-500/40 bg-amber-500/10 px-3 py-1.5 text-xs font-semibold text-amber-400 cursor-pointer transition-all hover:border-amber-400/60 hover:bg-amber-500/20 hover:shadow-[0_0_10px_rgba(245,158,11,0.25)] hover:-translate-y-px disabled:opacity-50"
                              title={
                                hasActiveLoginSession
                                  ? "重新打开认证流程"
                                  : "重新认证 (账号已失效或待确认)"
                              }
                            >
                              {isValidating === acc.id ? (
                                <Loader2 className="h-3 w-3 animate-spin" />
                              ) : (
                                <KeyRound className="h-3 w-3" />
                              )}
                              {hasActiveLoginSession
                                ? "重新打开认证"
                                : "重新认证"}
                            </button>
                          ) : (
                            <button
                              onClick={() => handleValidate(acc.id)}
                              disabled={isValidating === acc.id}
                              className="inline-flex items-center gap-1.5 rounded-full border border-emerald-500/40 bg-emerald-500/10 px-3 py-1.5 text-xs font-semibold text-emerald-400 cursor-pointer transition-all hover:border-emerald-400/60 hover:bg-emerald-500/20 hover:shadow-[0_0_10px_rgba(16,185,129,0.25)] hover:-translate-y-px disabled:opacity-50"
                              title="登录状态有效"
                            >
                              {isValidating === acc.id ? (
                                <Loader2 className="h-3 w-3 animate-spin" />
                              ) : (
                                <BadgeCheck className="h-3 w-3" />
                              )}
                              重新认证
                            </button>
                          )}
                          <button
                            onClick={() => setDeleteTargetId(acc.id)}
                            disabled={isDeleting === acc.id}
                            className="inline-flex items-center gap-1.5 rounded-full border border-border/60 bg-surface px-3 py-1.5 text-xs font-semibold text-text-muted cursor-pointer transition-all hover:border-danger/50 hover:text-danger hover:bg-danger/10 hover:shadow-[0_0_8px_rgba(239,68,68,0.15)] disabled:opacity-50"
                            title="删除账号关联"
                          >
                            {isDeleting === acc.id ? (
                              <Loader2 className="h-3 w-3 animate-spin" />
                            ) : (
                              <Trash2 className="h-3 w-3" />
                            )}
                            解绑删除
                          </button>
                        </div>
                      </td>
                    </motion.tr>
                  );
                })}
              </tbody>
            </table>

            {/* Pagination */}
            <div className="flex items-center justify-between border-t border-border/50 px-6 py-4 bg-surface/20">
              <span className="text-sm text-text-muted">
                第 <span className="font-semibold text-text-primary">1</span>{" "}
                页，共{" "}
                <span className="font-semibold text-text-primary">1</span> 页
              </span>
              <div className="flex gap-2">
                <button
                  className="flex h-8 w-8 items-center justify-center rounded-lg border border-border bg-surface text-text-muted transition-colors hover:border-accent hover:text-accent disabled:opacity-40"
                  disabled
                >
                  <ChevronLeft className="h-4 w-4" />
                </button>
                <button
                  className="flex h-8 w-8 items-center justify-center rounded-lg border border-border bg-surface text-text-muted transition-colors hover:border-accent hover:text-accent disabled:opacity-40"
                  disabled
                >
                  <ChevronRight className="h-4 w-4" />
                </button>
              </div>
            </div>
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center py-16 text-text-muted">
            <Users className="h-10 w-10 mb-3 opacity-40" />
            <p className="text-sm font-medium">暂无同步账号</p>
            <p className="text-xs mt-1 opacity-60">
              为此设备添加自媒体平台账号
            </p>
          </div>
        )}
      </motion.div>

      <AnimatePresence>
        {deleteTarget && (
          <div className="fixed inset-0 z-[70] flex items-center justify-center px-4 py-6">
            <motion.button
              type="button"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => !isDeleting && setDeleteTargetId(null)}
              className="fixed inset-0 bg-black/70 backdrop-blur-md"
            />
            <motion.div
              initial={{ opacity: 0, scale: 0.96, y: 14 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.96, y: 14 }}
              transition={{ duration: 0.18, ease: "easeOut" }}
              className="relative z-10 w-full max-w-lg overflow-hidden rounded-3xl border border-white/10 bg-[#0A0A14]/95 shadow-[0_0_80px_rgba(239,68,68,0.18)] backdrop-blur-xl"
            >
              <div className="border-b border-white/5 bg-gradient-to-r from-red-500/12 via-amber-500/8 to-transparent px-6 py-5">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex items-start gap-3">
                    <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-red-500/15 text-red-300 shadow-[0_0_24px_rgba(239,68,68,0.18)]">
                      <AlertTriangle className="h-5 w-5" />
                    </div>
                    <div>
                      <h3 className="text-lg font-black tracking-wide text-white">
                        解除 OmniBull 账号绑定
                      </h3>
                      <p className="mt-1 text-sm text-text-muted">
                        {deleteTarget.platform} / {deleteTarget.accountName}
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => !isDeleting && setDeleteTargetId(null)}
                    className="rounded-full bg-white/5 p-2 text-text-muted transition-all hover:rotate-90 hover:bg-red-500/20 hover:text-red-300"
                  >
                    <X className="h-4 w-4" />
                  </button>
                </div>
              </div>

              <div className="space-y-4 px-6 py-6">
                <div className="grid gap-3 sm:grid-cols-3">
                  <div className="rounded-2xl border border-white/8 bg-white/5 px-4 py-3">
                    <p className="text-xs font-semibold uppercase tracking-widest text-text-muted">
                      计划中任务
                    </p>
                    <p className="mt-2 text-2xl font-black text-white">
                      {deleteUsage.activeTaskCount}
                    </p>
                  </div>
                  <div className="rounded-2xl border border-white/8 bg-white/5 px-4 py-3">
                    <p className="text-xs font-semibold uppercase tracking-widest text-text-muted">
                      历史任务
                    </p>
                    <p className="mt-2 text-2xl font-black text-white">
                      {deleteUsage.historicalTaskCount}
                    </p>
                  </div>
                  <div className="rounded-2xl border border-white/8 bg-white/5 px-4 py-3">
                    <p className="text-xs font-semibold uppercase tracking-widest text-text-muted">
                      活跃认证
                    </p>
                    <p className="mt-2 text-2xl font-black text-white">
                      {deleteUsage.activeLoginSessionCount}
                    </p>
                  </div>
                </div>

                {deleteUsage.hasBlockingUsage ? (
                  <div className="rounded-2xl border border-amber-500/30 bg-amber-500/12 px-4 py-4 text-sm leading-6 text-amber-100">
                    当前账号还有计划中任务或正在进行的认证流程，暂时不能解绑。
                    {deleteUsage.historicalTaskCount > 0
                      ? ` 等这些阻塞项处理完后，再解绑时会一并清空 ${deleteUsage.historicalTaskCount} 条历史任务记录。`
                      : ""}
                  </div>
                ) : (
                  <>
                    <div className="rounded-2xl border border-red-500/30 bg-red-500/12 px-4 py-4 text-sm leading-6 text-red-100">
                      {deleteUsage.historicalTaskCount > 0
                        ? `检测到账户下仍保留 ${deleteUsage.historicalTaskCount} 条历史任务记录。解除绑定后，这些任务记录和相关登录记录会一并清空，且不会再被 OmniBull 自动补回。`
                        : "当前没有计划中任务，可以正常解绑。解除绑定后，会清空相关历史登录记录，并阻止 OmniBull 自动把这个账号重新同步回来。"}
                    </div>
                    <div className="flex items-center gap-3 rounded-2xl border border-cyan/25 bg-cyan/10 px-4 py-3 text-sm text-cyan-100">
                      <Clock3 className="h-4 w-4 shrink-0" />
                      {deleteCountdown > 0
                        ? `请等待 ${deleteCountdown} 秒后再确认解绑。`
                        : "倒计时结束，现在可以确认解绑。"}
                    </div>
                  </>
                )}

                {deleteDialogError ? (
                  <div className="rounded-2xl border border-red-500/35 bg-red-500/10 px-4 py-3 text-sm text-red-100">
                    {deleteDialogError}
                  </div>
                ) : null}
              </div>

              <div className="flex flex-col gap-3 border-t border-white/5 bg-black/30 px-6 py-5 sm:flex-row sm:items-center sm:justify-end">
                {deleteUsage.hasBlockingUsage ? (
                  <Link
                    href={`/nodes/${deviceId}/accounts/${deleteTarget.id}`}
                    className="inline-flex items-center justify-center gap-2 rounded-full border border-amber-500/35 bg-amber-500/10 px-5 py-2.5 text-sm font-semibold text-amber-200 transition-all hover:border-amber-400/60 hover:bg-amber-500/20"
                  >
                    查看任务
                  </Link>
                ) : null}
                <button
                  type="button"
                  onClick={() => setDeleteTargetId(null)}
                  disabled={Boolean(isDeleting)}
                  className="rounded-full px-5 py-2.5 text-sm font-bold text-text-muted transition-all hover:bg-white/8 hover:text-white disabled:opacity-50"
                >
                  取消
                </button>
                {!deleteUsage.hasBlockingUsage ? (
                  <button
                    type="button"
                    onClick={handleDelete}
                    disabled={
                      deleteCountdown > 0 || isDeleting === deleteTarget.id
                    }
                    className={`inline-flex items-center justify-center gap-2 rounded-full px-5 py-2.5 text-sm font-bold transition-all ${
                      deleteCountdown > 0 || isDeleting === deleteTarget.id
                        ? "cursor-not-allowed bg-white/10 text-white/35"
                        : "bg-gradient-to-r from-red-500 to-orange-500 text-white shadow-[0_0_24px_rgba(239,68,68,0.28)] hover:scale-[1.02]"
                    }`}
                  >
                    {isDeleting === deleteTarget.id ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Trash2 className="h-4 w-4" />
                    )}
                    {deleteCountdown > 0
                      ? `解除绑定 (${deleteCountdown}s)`
                      : "确认解除绑定"}
                  </button>
                ) : null}
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>

      {/* Add Account Modal */}
      <AddAccountModal
        isOpen={isAccountModalOpen}
        onClose={() => {
          setIsAccountModalOpen(false);
          setValidationSession(null);
          queryClient.invalidateQueries({ queryKey: ["accounts", deviceId] });
          queryClient.invalidateQueries({ queryKey: ["device", deviceId] });
        }}
        deviceId={deviceId}
        initialSession={validationSession}
      />
    </>
  );
}
