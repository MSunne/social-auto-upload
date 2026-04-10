"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import {
  CheckCircle2,
  Download,
  History,
  LayoutGrid,
  Search,
  Sparkles,
  Video,
  Zap,
} from "lucide-react";
import { EmptyState, PageHeader, StatCard, StatusBadge } from "@/components/ui/common";
import {
  DIGITAL_HUMAN_POLL_INTERVAL_MS,
  formatDigitalHumanMode,
  isTerminalDigitalHumanTask,
  truncateDigitalHumanText,
} from "@/lib/digital-human";
import { listDigitalHumanTasks } from "@/lib/services";
import type { DigitalHumanTask } from "@/lib/types";
import { formatDateTime } from "@/lib/workflow";

type FilterKey = "all" | "running" | "completed" | "failed";

const FILTERS: Array<{ key: FilterKey; label: string; icon: React.ReactNode }> = [
  { key: "all", label: "全部", icon: <LayoutGrid className="h-3.5 w-3.5" /> },
  { key: "running", label: "进行中", icon: <Zap className="h-3.5 w-3.5" /> },
  { key: "completed", label: "已完成", icon: <CheckCircle2 className="h-3.5 w-3.5" /> },
  { key: "failed", label: "失败 / 取消", icon: <History className="h-3.5 w-3.5" /> },
];

/* ─── Animation variants ─── */
const staggerContainer = {
  hidden: {},
  show: { transition: { staggerChildren: 0.06 } },
};

const fadeUp = {
  hidden: { opacity: 0, y: 14 },
  show: { opacity: 1, y: 0, transition: { duration: 0.35, ease: "easeOut" as const } },
};

const listItem = {
  hidden: { opacity: 0, y: 10 },
  show: { opacity: 1, y: 0, transition: { duration: 0.3, ease: "easeOut" as const } },
};

/* ─── Skeleton Card ─── */
function SkeletonCard() {
  return (
    <div className="glass-card overflow-hidden p-5">
      <div className="flex gap-4">
        <div className="skeleton h-20 w-20 shrink-0 rounded-xl" />
        <div className="min-w-0 flex-1 space-y-3">
          <div className="flex items-center gap-3">
            <div className="skeleton h-5 w-36 rounded-lg" />
            <div className="skeleton h-5 w-16 rounded-full" />
          </div>
          <div className="skeleton h-4 w-full rounded-lg" />
          <div className="flex gap-4">
            <div className="skeleton h-3 w-28 rounded-lg" />
            <div className="skeleton h-3 w-28 rounded-lg" />
          </div>
        </div>
      </div>
    </div>
  );
}

/* ─── Mini Progress Bar ─── */
function MiniProgress({ task }: { task: DigitalHumanTask }) {
  const raw = Number(task.progress?.percentage ?? 0);
  const pct = Math.max(0, Math.min(100, Number.isFinite(raw) && raw > 0 ? raw : 12));

  return (
    <div className="mt-3">
      <div className="flex items-center justify-between text-[11px] text-text-muted">
        <span className="truncate">{task.progress?.message || "生成中..."}</span>
        <span className="ml-2 shrink-0 font-mono font-medium text-accent">{pct.toFixed(0)}%</span>
      </div>
      <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-background/50">
        <motion.div
          className="h-full rounded-full bg-gradient-to-r from-accent to-cyan"
          initial={{ width: 0 }}
          animate={{ width: `${pct}%` }}
          transition={{ duration: 0.5 }}
          style={{
            boxShadow: "0 0 8px rgba(177,73,255,0.35)",
          }}
        />
      </div>
    </div>
  );
}

/* ─── Task Card ─── */
function TaskCard({ task }: { task: DigitalHumanTask }) {
  const isRunning = ["queued", "running"].includes(task.status);
  const isCompleted = task.status === "completed";
  const thumbnailUrl = task.characterAsset?.publicUrl;

  return (
    <motion.div
      variants={listItem}
      whileHover={{ y: -2 }}
      transition={{ duration: 0.2 }}
      className="glass-card glow-border overflow-hidden p-5 transition-shadow duration-300"
    >
      <div className="flex gap-4">
        {/* Thumbnail */}
        <div className="relative h-20 w-20 shrink-0 overflow-hidden rounded-xl border border-border/50 bg-black/30">
          {thumbnailUrl ? (
            <>
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={thumbnailUrl}
                alt="人物"
                className="h-full w-full object-cover"
              />
              <div className="absolute inset-0 bg-gradient-to-t from-black/40 to-transparent" />
            </>
          ) : (
            <div className="flex h-full w-full items-center justify-center text-text-muted/40">
              <Video className="h-6 w-6" />
            </div>
          )}
          {/* Mode badge on thumbnail */}
          <div className="absolute bottom-1 left-1 rounded-md bg-black/60 px-1.5 py-0.5 text-[10px] font-medium text-white/90 backdrop-blur-sm">
            {formatDigitalHumanMode(task.mode)}
          </div>
        </div>

        {/* Content */}
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2.5">
            <h2 className="truncate text-base font-semibold text-text-primary">
              {task.goodsTitle?.trim() || "自定义口播任务"}
            </h2>
            <StatusBadge status={task.status} />
          </div>

          <p className="mt-2 line-clamp-2 text-sm leading-6 text-text-secondary">
            {truncateDigitalHumanText(task.goodsText, 100)}
          </p>

          <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1.5 text-xs text-text-muted">
            <span>创建：{formatDateTime(task.createdAt)}</span>
            <span>更新：{formatDateTime(task.updatedAt)}</span>
          </div>

          {/* Inline progress for running tasks */}
          {isRunning ? <MiniProgress task={task} /> : null}
        </div>
      </div>

      {/* Actions bar */}
      <div className="mt-4 flex items-center gap-2.5 border-t border-border/30 pt-4">
        <Link
          href={`/creation/digital-human/${task.id}`}
          className="btn-neon inline-flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-text-primary transition-all hover:border-accent hover:text-accent"
        >
          <History className="h-3 w-3" />
          查看详情
        </Link>
        {isCompleted && task.resultAsset?.publicUrl ? (
          <a
            href={task.resultAsset.publicUrl}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1.5 rounded-lg bg-gradient-to-r from-accent/15 to-cyan/15 px-3 py-1.5 text-xs font-medium text-accent transition-all hover:from-accent/25 hover:to-cyan/25"
          >
            <Download className="h-3 w-3" />
            下载成品
          </a>
        ) : null}

        {/* Result video thumbnail for completed tasks */}
        {isCompleted && task.resultAsset?.publicUrl ? (
          <div className="ml-auto flex items-center gap-1.5 text-xs text-success">
            <CheckCircle2 className="h-3.5 w-3.5" />
            <span>视频已就绪</span>
          </div>
        ) : null}
      </div>
    </motion.div>
  );
}

/* ─── Page ─── */

export default function DigitalHumanHistoryPage() {
  const [filter, setFilter] = useState<FilterKey>("all");
  const [query, setQuery] = useState("");

  const { data: tasks = [], isLoading } = useQuery<DigitalHumanTask[]>({
    queryKey: ["digitalHumanTasks", "history"],
    queryFn: () => listDigitalHumanTasks({ limit: 100 }),
    refetchInterval: ({ state }) => {
      const items = state.data as DigitalHumanTask[] | undefined;
      return items?.some((task) => !isTerminalDigitalHumanTask(task))
        ? DIGITAL_HUMAN_POLL_INTERVAL_MS
        : false;
    },
  });

  const normalizedQuery = query.trim().toLowerCase();

  const filteredTasks = useMemo(() => {
    return tasks.filter((task) => {
      if (filter === "running" && !["queued", "running"].includes(task.status)) {
        return false;
      }
      if (filter === "completed" && task.status !== "completed") {
        return false;
      }
      if (filter === "failed" && !["failed", "cancelled"].includes(task.status)) {
        return false;
      }
      if (!normalizedQuery) {
        return true;
      }

      const haystack = [
        task.goodsTitle || "",
        task.goodsText,
        task.id,
        task.mode,
        task.status,
      ]
        .join(" ")
        .toLowerCase();

      return haystack.includes(normalizedQuery);
    });
  }, [filter, normalizedQuery, tasks]);

  const runningCount = tasks.filter((task) => ["queued", "running"].includes(task.status)).length;
  const completedCount = tasks.filter((task) => task.status === "completed").length;

  return (
    <>
      <PageHeader
        title="数字人历史"
        subtitle="查看所有数字人视频任务的创建记录、实时状态和生成结果。"
        actions={
          <Link
            href="/creation/digital-human"
            className="btn-neon inline-flex items-center gap-2 rounded-xl border border-border px-4 py-2 text-sm font-medium text-text-primary transition-all hover:border-accent hover:text-accent"
          >
            <Sparkles className="h-4 w-4" />
            新建任务
          </Link>
        }
      />

      {/* ─── Stats ─── */}
      <motion.div
        variants={staggerContainer}
        initial="hidden"
        animate="show"
        className="mb-6 grid gap-4 md:grid-cols-3"
      >
        <motion.div variants={fadeUp}>
          <StatCard
            label="总任务数"
            value={tasks.length}
            icon={<LayoutGrid className="h-5 w-5" />}
          />
        </motion.div>
        <motion.div variants={fadeUp}>
          <StatCard
            label="进行中"
            value={runningCount}
            change={runningCount > 0 ? "有任务正在生成" : undefined}
            changeType="neutral"
            icon={<Zap className="h-5 w-5" />}
          />
        </motion.div>
        <motion.div variants={fadeUp}>
          <StatCard
            label="已完成"
            value={completedCount}
            icon={<CheckCircle2 className="h-5 w-5" />}
          />
        </motion.div>
      </motion.div>

      {/* ─── Filters & Search ─── */}
      <motion.div
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.2, duration: 0.3 }}
        className="mb-6 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between"
      >
        <div className="flex flex-wrap gap-2">
          {FILTERS.map((item) => (
            <button
              key={item.key}
              type="button"
              onClick={() => setFilter(item.key)}
              className={`inline-flex items-center gap-1.5 rounded-full px-3.5 py-2 text-sm font-medium transition-all duration-200 ${
                filter === item.key
                  ? "bg-accent/15 text-accent shadow-[0_0_12px_rgba(177,73,255,0.1)]"
                  : "bg-surface/50 text-text-secondary hover:bg-surface-hover hover:text-text-primary"
              }`}
            >
              {item.icon}
              {item.label}
            </button>
          ))}
        </div>

        <div className="relative max-w-md flex-1">
          <Search className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted" />
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="搜索产品简介、文案或任务 ID..."
            className="w-full rounded-2xl border border-border bg-surface/50 py-3 pl-11 pr-4 text-sm text-text-primary outline-none transition-all duration-300 focus:border-accent focus:shadow-[0_0_16px_rgba(177,73,255,0.1)] placeholder:text-text-muted/50"
          />
        </div>
      </motion.div>

      {/* ─── Task List ─── */}
      {isLoading ? (
        <div className="space-y-4">
          <SkeletonCard />
          <SkeletonCard />
          <SkeletonCard />
        </div>
      ) : filteredTasks.length > 0 ? (
        <motion.div
          variants={staggerContainer}
          initial="hidden"
          animate="show"
          className="space-y-4"
        >
          {filteredTasks.map((task) => (
            <TaskCard key={task.id} task={task} />
          ))}
        </motion.div>
      ) : (
        <EmptyState
          icon={<History className="h-6 w-6" />}
          title="没有匹配的数字人任务"
          description="提交一条数字人视频任务后，历史记录会在这里自动汇总。"
          action={
            <Link
              href="/creation/digital-human"
              className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-accent to-[#7c3aed] px-5 py-2.5 text-sm font-semibold text-white shadow-[0_0_20px_rgba(177,73,255,0.2)] transition-all hover:shadow-[0_0_30px_rgba(177,73,255,0.3)] hover:brightness-110"
            >
              <Sparkles className="h-4 w-4" />
              去创建
            </Link>
          }
        />
      )}
    </>
  );
}
