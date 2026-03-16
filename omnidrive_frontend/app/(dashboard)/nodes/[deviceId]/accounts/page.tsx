"use client";

import { use, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import {
  ArrowLeft,
  Users,
  Layout,
  ListChecks,
  Sparkles,
  ShieldCheck,
  ShieldAlert,
  Trash2,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
} from "lucide-react";
import Link from "next/link";
import api from "@/lib/api";
import type { Device, Account, Skill, Task } from "@/lib/types";
import { StatusBadge } from "@/components/ui/common";

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

export default function DeviceAccountsPage({
  params,
}: {
  params: Promise<{ deviceId: string }>;
}) {
  const { deviceId } = use(params);

  const { data: device } = useQuery<Device>({
    queryKey: ["device", deviceId],
    queryFn: () => api.get(`/devices/${deviceId}`).then((r) => r.data),
  });

  const { data: accounts = [] } = useQuery<Account[]>({
    queryKey: ["accounts", deviceId],
    queryFn: () =>
      api.get(`/accounts?deviceId=${deviceId}`).then((r) => r.data),
  });

  const { data: skills = [] } = useQuery<Skill[]>({
    queryKey: ["skills"],
    queryFn: () => api.get("/skills").then((r) => r.data),
  });

  const { data: tasks = [] } = useQuery<Task[]>({
    queryKey: ["tasks"],
    queryFn: () => api.get("/tasks").then((r) => r.data),
  });

  /* Platform filter */
  const [platformFilter, setPlatformFilter] = useState<string | null>(null);

  /* Computed stats */
  const uniquePlatforms = new Set(accounts.map((a) => a.platform));
  const deviceTasks = tasks.filter((t) => t.deviceId === deviceId);
  const platformCounts: Record<string, number> = {};
  accounts.forEach((a) => {
    platformCounts[a.platform] = (platformCounts[a.platform] || 0) + 1;
  });

  /* Filtered accounts */
  const filteredAccounts = platformFilter
    ? accounts.filter((a) => a.platform === platformFilter)
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
                onClick={() => count > 0 && setPlatformFilter(isActive ? null : p.key)}
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
                    认证时间
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
                    (p) => p.key === acc.platform
                  );
                  const accTasks = tasks.filter(
                    (t) =>
                      t.accountId === acc.id || t.accountName === acc.accountName
                  );
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
                          {acc.accountName}
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
                          {acc.platform}
                        </span>
                      </td>

                      {/* Created At */}
                      <td className="px-6 py-4 font-mono text-xs text-text-muted">
                        {new Date(acc.createdAt).toLocaleDateString("zh-CN")}
                      </td>

                      {/* Status */}
                      <td className="px-6 py-4">
                        <StatusBadge status={acc.status} />
                      </td>

                      {/* Last Authenticated */}
                      <td className="px-6 py-4 font-mono text-xs text-text-muted">
                        {acc.lastAuthenticatedAt
                          ? new Date(
                              acc.lastAuthenticatedAt
                            ).toLocaleString("zh-CN", {
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
                          href="/tasks"
                          className="inline-flex items-center gap-1.5 rounded-full bg-gradient-to-r from-accent/15 to-cyan/15 border border-accent/25 px-3 py-1.5 text-xs font-semibold text-accent transition-all hover:from-accent/25 hover:to-cyan/25 hover:border-accent/40 hover:shadow-[0_0_12px_rgba(177,73,255,0.2)] hover:-translate-y-px"
                        >
                          <ExternalLink className="h-3 w-3" />
                          详情/增加任务
                        </Link>
                      </td>

                      {/* Actions */}
                      <td className="px-6 py-4">
                        <div className="flex items-center justify-center gap-2">
                          {acc.status === "invalid" ? (
                            <button
                              className="flex h-8 w-8 items-center justify-center rounded-lg border border-amber-500/40 bg-amber-500/10 text-amber-400 transition-all hover:border-amber-400/60 hover:bg-amber-500/20 hover:shadow-[0_0_10px_rgba(245,158,11,0.2)]"
                              title="重新认证"
                            >
                              <ShieldAlert className="h-3.5 w-3.5" />
                            </button>
                          ) : (
                            <button
                              className="flex h-8 w-8 items-center justify-center rounded-lg border border-emerald-500/40 bg-emerald-500/10 text-emerald-400 transition-all hover:border-emerald-400/60 hover:bg-emerald-500/20 hover:shadow-[0_0_10px_rgba(16,185,129,0.2)]"
                              title="已认证"
                            >
                              <ShieldCheck className="h-3.5 w-3.5" />
                            </button>
                          )}
                          <button
                            className="flex h-8 w-8 items-center justify-center rounded-lg border border-border/60 bg-surface text-text-muted transition-all hover:border-danger/50 hover:text-danger hover:bg-danger/10 hover:shadow-[0_0_8px_rgba(239,68,68,0.15)]"
                            title="删除账号"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
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
    </>
  );
}
