"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { motion, AnimatePresence } from "framer-motion";
import {
  Server,
  Plus,
  Shield,
  Activity,
  Link2,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Pencil,
  BookOpen,
  Search,
  X,
  Server as ServerIcon,
} from "lucide-react";
import Link from "next/link";
import { listDevices, claimDevice } from "@/lib/services";
import type { Device } from "@/lib/types";
import { PageHeader, EmptyState } from "@/components/ui/common";

const PAGE_SIZE = 5;

/* ── Status badge configs matching reference ── */
const statusConfig: Record<
  string,
  { label: string; className: string }
> = {
  online: {
    label: "在线运行",
    className:
      "bg-cyan/15 text-cyan border-cyan/30",
  },
  offline: {
    label: "离线",
    className:
      "bg-gray-500/15 text-gray-400 border-gray-500/30",
  },
  pending: {
    label: "待审核",
    className:
      "bg-amber-500/15 text-amber-400 border-amber-500/30",
  },
  error: {
    label: "连接异常",
    className:
      "bg-rose-500/15 text-rose-400 border-rose-500/30",
  },
  unknown: {
    label: "未知",
    className:
      "bg-gray-500/15 text-gray-400 border-gray-500/30",
  },
};

/* ── Toggle Switch component ── */
function Toggle({
  checked,
  onChange,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <button
      role="switch"
      aria-checked={checked}
      onClick={() => onChange(!checked)}
      className={`group relative inline-flex h-7 w-[52px] shrink-0 cursor-pointer items-center rounded-full border transition-all duration-300 ${
        checked
          ? "border-cyan/40 bg-gradient-to-r from-accent/80 to-cyan/80 shadow-[0_0_12px_rgba(0,245,212,0.3)]"
          : "border-border bg-surface-elevated hover:border-border"
      }`}
    >
      <span
        className={`pointer-events-none inline-block h-5 w-5 rounded-full shadow-md transition-all duration-300 ${
          checked
            ? "translate-x-[26px] bg-white shadow-[0_0_6px_rgba(0,245,212,0.5)]"
            : "translate-x-[3px] bg-gray-400 group-hover:bg-gray-300"
        }`}
      />
      {checked && (
        <span className="absolute right-[22px] text-[9px] font-bold text-white/90 select-none">
          ON
        </span>
      )}
    </button>
  );
}

export default function NodesPage() {
  const { data: devices = [], refetch } = useQuery<Device[]>({
    queryKey: ["devices"],
    queryFn: listDevices,
  });

  const [page, setPage] = useState(1);
  const [searchQuery, setSearchQuery] = useState("");
  const [claiming, setClaiming] = useState(false);
  const [error, setError] = useState("");
  const [toggleState, setToggleState] = useState<Record<string, boolean>>({});

  // Modal State
  const [isClaimModalOpen, setIsClaimModalOpen] = useState(false);
  const [claimInputCode, setClaimInputCode] = useState("");

  /* filtering and pagination math */
  const filteredDevices = devices.filter(
    (d) =>
      d.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      (d.localIp && d.localIp.includes(searchQuery)) ||
      (d.deviceCode && d.deviceCode.includes(searchQuery))
  );

  const totalPages = Math.max(1, Math.ceil(filteredDevices.length / PAGE_SIZE));
  const pagedDevices = filteredDevices.slice(
    (page - 1) * PAGE_SIZE,
    page * PAGE_SIZE,
  );
  const startIdx = filteredDevices.length === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const endIdx = Math.min(page * PAGE_SIZE, filteredDevices.length);

  /* stats */
  const onlineCount = devices.filter((d) => d.status === "online").length;
  const enabledCount = devices.filter((d) => d.isEnabled).length;

  async function handleClaimSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!claimInputCode.trim()) return;
    setClaiming(true);
    setError("");
    try {
      await claimDevice(claimInputCode.trim());
      setIsClaimModalOpen(false);
      setClaimInputCode("");
      refetch();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "认领失败，请检查全平台授权");
    } finally {
      setClaiming(false);
    }
  }

  /* page number array for pagination */
  function getPageNumbers() {
    const pages: (number | "...")[] = [];
    if (totalPages <= 7) {
      for (let i = 1; i <= totalPages; i++) pages.push(i);
    } else {
      pages.push(1);
      if (page > 3) pages.push("...");
      for (
        let i = Math.max(2, page - 1);
        i <= Math.min(totalPages - 1, page + 1);
        i++
      ) {
        pages.push(i);
      }
      if (page < totalPages - 2) pages.push("...");
      pages.push(totalPages);
    }
    return pages;
  }

  return (
    <>
      {/* Search + Add Device — single row */}
      <div className="mb-5 flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted" />
          <input
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              setPage(1);
            }}
            placeholder="搜索设备名称、编码或 IP 地址..."
            className="w-full rounded-xl border border-border bg-surface pl-9 pr-3 py-2.5 text-sm text-text-primary placeholder-text-muted outline-none transition-all focus:border-accent/50 focus:ring-2 focus:ring-accent/20 focus:shadow-[0_0_12px_rgba(177,73,255,0.1)]"
          />
        </div>
        <button
          onClick={() => {
            setError("");
            setClaimInputCode("");
            setIsClaimModalOpen(true);
          }}
          disabled={claiming}
          className="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2.5 text-sm font-semibold text-background transition-all hover:shadow-lg hover:shadow-accent/25 disabled:opacity-50"
        >
          <Plus className="h-4 w-4" />
          添加设备
        </button>
      </div>

      {error && (
        <div className="mb-4 rounded-xl border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">
          {error}
        </div>
      )}

      {/* ───── Stats Cards ───── */}
      <div className="mb-5 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <motion.div
          initial={{ opacity: 0, y: 15 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="glass-card px-6 py-5"
        >
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-cyan/10">
              <Shield className="h-6 w-6 text-cyan" />
            </div>
            <div>
              <p className="text-xs text-text-muted">安全审计评分</p>
              <div className="flex items-baseline gap-1">
                <span className="text-2xl font-bold text-text-primary">98.4</span>
                <span className="text-sm font-medium text-emerald-400">Excellent</span>
              </div>
            </div>
          </div>
        </motion.div>
        <motion.div
          initial={{ opacity: 0, y: 15 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="glass-card px-6 py-5"
        >
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-accent/10">
              <Activity className="h-6 w-6 text-accent" />
            </div>
            <div>
              <p className="text-xs text-text-muted">实时吞吐量</p>
              <div className="flex items-baseline gap-1">
                <span className="text-2xl font-bold text-text-primary">1.2</span>
                <span className="text-sm font-medium text-text-secondary">GB/s</span>
              </div>
            </div>
          </div>
        </motion.div>
        <motion.div
          initial={{ opacity: 0, y: 15 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
          className="glass-card px-6 py-5"
        >
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-rose-500/10">
              <Link2 className="h-6 w-6 text-rose-400" />
            </div>
            <div>
              <p className="text-xs text-text-muted">总节点链路</p>
              <div className="flex items-baseline gap-1">
                <span className="text-2xl font-bold text-text-primary">
                  {filteredDevices.length > 0 ? (filteredDevices.length * 104).toLocaleString() : "0"}
                </span>
                <span className="text-sm font-medium text-text-secondary">活跃</span>
              </div>
            </div>
          </div>
        </motion.div>
      </div>

      {/* ───── Main Table ───── */}
      {filteredDevices.length > 0 || devices.length > 0 ? (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="glass-card overflow-hidden"
        >
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left">
                  <th className="px-5 py-4 text-xs font-bold tracking-wider text-cyan">
                    名称
                  </th>
                  <th className="px-5 py-4 text-xs font-bold tracking-wider text-cyan">
                    状态
                  </th>
                  <th className="px-5 py-4 text-xs font-bold tracking-wider text-cyan">
                    推理模型
                  </th>
                  <th className="px-5 py-4 text-xs font-bold tracking-wider text-cyan">
                    汇报时间
                  </th>
                  <th className="px-5 py-4 text-xs font-bold tracking-wider text-cyan">
                    产品知识和技能
                  </th>
                  <th className="px-5 py-4 text-xs font-bold tracking-wider text-cyan">
                    OMNIBULL
                  </th>
                  <th className="px-5 py-4 text-xs font-bold tracking-wider text-cyan">
                    操作
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/50">
                {pagedDevices.map((device) => {
                  const cfg =
                    statusConfig[device.status] ?? statusConfig.unknown;
                  return (
                    <tr
                      key={device.id}
                      className="transition-colors hover:bg-surface-hover/50"
                    >
                      {/* 名称 */}
                      <td className="px-5 py-4 font-medium text-text-primary">
                        {device.name}
                      </td>

                      {/* 状态 */}
                      <td className="px-5 py-4">
                        <span
                          className={`inline-flex items-center rounded-md border px-2.5 py-1 text-xs font-semibold ${cfg.className}`}
                        >
                          {cfg.label}
                        </span>
                      </td>

                      {/* 推理模型 */}
                      <td className="px-5 py-4 text-text-secondary">
                        {device.defaultReasoningModel ?? (
                          <span className="text-text-muted">未设置</span>
                        )}
                      </td>

                      {/* 汇报时间 */}
                      <td className="px-5 py-4 font-mono text-xs text-text-muted">
                        {device.lastSeenAt
                          ? new Date(device.lastSeenAt).toLocaleString(
                              "zh-CN",
                              {
                                year: "numeric",
                                month: "2-digit",
                                day: "2-digit",
                                hour: "2-digit",
                                minute: "2-digit",
                              },
                            )
                          : "-"}
                      </td>

                      {/* 产品知识和技能 */}
                      <td className="px-5 py-4">
                        <Link
                          href={`/nodes/${device.id}`}
                          className="inline-flex items-center gap-1.5 rounded-full bg-gradient-to-r from-accent/15 to-cyan/15 border border-accent/25 px-3.5 py-1.5 text-xs font-semibold text-accent transition-all hover:from-accent/25 hover:to-cyan/25 hover:border-accent/40 hover:shadow-[0_0_14px_rgba(177,73,255,0.2)] hover:-translate-y-px"
                        >
                          <BookOpen className="h-3 w-3" />
                          详情/编辑技能
                        </Link>
                      </td>

                      {/* OMNIBULL — 详情/编辑 */}
                      <td className="px-5 py-4">
                        <Link
                          href={`/nodes/${device.id}/accounts`}
                          className="inline-flex items-center gap-1.5 rounded-full bg-gradient-to-r from-cyan/15 to-emerald-500/15 border border-cyan/25 px-3.5 py-1.5 text-xs font-semibold text-cyan transition-all hover:from-cyan/25 hover:to-emerald-500/25 hover:border-cyan/40 hover:shadow-[0_0_14px_rgba(0,245,212,0.2)] hover:-translate-y-px"
                        >
                          <ExternalLink className="h-3 w-3" />
                          详情/编辑
                        </Link>
                      </td>

                      {/* 操作 — 启用/关闭开关 */}
                      <td className="px-5 py-4">
                        <div className="flex items-center gap-3">
                          <Toggle
                            checked={toggleState[device.id] ?? device.isEnabled}
                            onChange={(v) => {
                              setToggleState(prev => ({ ...prev, [device.id]: v }));
                            }}
                          />
                          <span className={`text-xs font-medium ${
                            (toggleState[device.id] ?? device.isEnabled) ? "text-cyan" : "text-text-muted"
                          }`}>
                            {(toggleState[device.id] ?? device.isEnabled) ? "已启用" : "已关闭"}
                          </span>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          {/* ───── Pagination ───── */}
          <div className="flex items-center justify-between border-t border-border px-5 py-4">
            <p className="text-sm text-text-muted">
              显示 {startIdx} 到 {endIdx} / 共{" "}
              <span className="font-semibold text-text-secondary">
                {filteredDevices.length}
              </span>{" "}
              个节点
            </p>

            <div className="flex items-center gap-1">
              {/* Prev */}
              <button
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                className="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface-hover disabled:opacity-30"
              >
                <ChevronLeft className="h-4 w-4" />
              </button>

              {/* Page numbers */}
              {getPageNumbers().map((n, i) =>
                n === "..." ? (
                  <span
                    key={`dot-${i}`}
                    className="flex h-8 w-8 items-center justify-center text-text-muted"
                  >
                    …
                  </span>
                ) : (
                  <button
                    key={n}
                    onClick={() => setPage(n as number)}
                    className={`flex h-8 w-8 items-center justify-center rounded-lg text-xs font-semibold transition-all ${
                      page === n
                        ? "bg-accent text-background shadow-md shadow-accent/30"
                        : "text-text-muted hover:bg-surface-hover hover:text-text-primary"
                    }`}
                  >
                    {n}
                  </button>
                ),
              )}

              {/* Next */}
              <button
                disabled={page >= totalPages}
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                className="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface-hover disabled:opacity-30"
              >
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>
        </motion.div>
      ) : (
        <EmptyState
          icon={<Server className="h-6 w-6" />}
          title="暂无设备"
          description="在 OmniBull 所在的 Linux 主机启动 Agent 后，输入设备编码进行认领。"
        />
      )}

      {/* ───── Claim Device Modal ───── */}
      <AnimatePresence>
        {isClaimModalOpen && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.15 }}
              className="fixed inset-0 bg-black/60 backdrop-blur-md"
              onClick={() => !claiming && setIsClaimModalOpen(false)}
            />
            <motion.div
              initial={{ opacity: 0, scale: 0.95, y: 10 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 10 }}
              transition={{ duration: 0.2, ease: "easeOut" }}
              className="relative z-10 w-full max-w-md overflow-hidden rounded-3xl border border-white/10 bg-[#0A0A14]/95 shadow-[0_0_60px_rgba(0,245,212,0.15)] backdrop-blur-xl"
            >
              {/* Header */}
              <div className="relative border-b border-white/5 bg-gradient-to-r from-cyan/10 to-transparent px-6 py-5">
                <div className="absolute inset-0 bg-noise opacity-[0.03] mix-blend-overlay pointer-events-none" />
                <div className="relative z-10 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-cyan to-blue-500 shadow-lg shadow-cyan/20">
                      <ServerIcon className="h-5 w-5 text-white" />
                    </div>
                    <div>
                      <h3 className="text-lg font-black tracking-wide text-white">
                        认领新设备节点
                      </h3>
                      <p className="text-xs font-medium text-text-muted/80">
                        绑定 OmniBull 终端到当前账户
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => !claiming && setIsClaimModalOpen(false)}
                    className="rounded-full bg-white/5 p-2 text-text-muted transition-all hover:rotate-90 hover:bg-red-500/20 hover:text-red-400"
                  >
                    <X className="h-4 w-4" />
                  </button>
                </div>
              </div>

              {/* Body */}
              <form onSubmit={handleClaimSubmit} className="p-6">
                <div className="mb-5 space-y-4">
                  <div>
                    <label className="mb-1.5 block text-xs font-bold uppercase tracking-widest text-text-muted">
                      设备认领凭证 (Device Code)
                    </label>
                    <input
                      type="text"
                      autoFocus
                      required
                      placeholder="例如: 8A9B2C3D"
                      value={claimInputCode}
                      onChange={(e) => setClaimInputCode(e.target.value)}
                      className="w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3.5 text-sm font-medium text-white placeholder-text-muted/50 transition-all focus:border-cyan/50 focus:bg-white/10 focus:outline-none focus:ring-4 focus:ring-cyan/10 uppercase"
                    />
                  </div>
                </div>

                {/* Footer */}
                <div className="flex items-center justify-end gap-3 pt-4 border-t border-white/5">
                  <button
                    type="button"
                    onClick={() => setIsClaimModalOpen(false)}
                    disabled={claiming}
                    className="rounded-full px-5 py-2.5 text-sm font-bold text-text-muted transition-all hover:bg-white/10 hover:text-white"
                  >
                    取消
                  </button>
                  <button
                    type="submit"
                    disabled={claiming || !claimInputCode.trim()}
                    className={`relative flex items-center justify-center overflow-hidden rounded-full px-8 py-2.5 text-sm font-black shadow-xl transition-all duration-300 ${
                      claiming || !claimInputCode.trim()
                        ? "bg-white/10 text-white/30 cursor-not-allowed"
                        : "bg-gradient-to-r from-cyan to-blue-500 text-white hover:scale-[1.03] hover:shadow-[0_0_30px_rgba(0,245,212,0.4)]"
                    }`}
                  >
                    <span className="relative z-10 drop-shadow-sm">
                      {claiming ? "认领中..." : "确认认领"}
                    </span>
                  </button>
                </div>
              </form>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </>
  );
}
