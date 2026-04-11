"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import {
  ArrowLeft,
  AudioLines,
  Calendar,
  CheckCircle2,
  Clock,
  Copy,
  Download,
  FileText,
  Hash,
  Image as ImageIcon,
  Package,
  Sparkles,
  Video,
} from "lucide-react";
import { PageHeader, StatusBadge } from "@/components/ui/common";
import {
  DIGITAL_HUMAN_POLL_INTERVAL_MS,
  DIGITAL_HUMAN_REMINDER_TEXT,
  formatDigitalHumanMode,
  isSuccessfulDigitalHumanTask,
  isTerminalDigitalHumanTask,
} from "@/lib/digital-human";
import { getDigitalHumanTask } from "@/lib/services";
import type { DigitalHumanAsset, DigitalHumanTask } from "@/lib/types";
import { formatDateTime } from "@/lib/workflow";

/* ─── Helpers ─── */

function clampProgress(task?: DigitalHumanTask | null) {
  if (!task) {
    return 0;
  }
  if (isSuccessfulDigitalHumanTask(task)) {
    return 100;
  }
  const raw = Number(task.progress?.percentage ?? 0);
  if (Number.isFinite(raw) && raw > 0) {
    return Math.max(0, Math.min(100, raw));
  }
  if (task.status === "running") {
    return 12;
  }
  return 0;
}

function formatBytes(bytes?: number | null) {
  if (typeof bytes !== "number") return null;
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text).catch(() => { /* ignore */ });
}

/* ─── Animation ─── */

const stagger = {
  hidden: {},
  show: { transition: { staggerChildren: 0.05 } },
};

const fadeUp = {
  hidden: { opacity: 0, y: 10 },
  show: { opacity: 1, y: 0, transition: { duration: 0.3, ease: "easeOut" as const } },
};

/* ─── Compact Asset Preview ─── */

function AssetCard({
  title,
  icon,
  asset,
  kind,
}: {
  title: string;
  icon: ReactNode;
  asset?: DigitalHumanAsset | null;
  kind: "image" | "audio";
}) {
  if (!asset?.publicUrl) {
    return (
      <div className="flex items-center gap-3 rounded-xl border border-dashed border-border/50 bg-surface/30 px-4 py-3 text-sm text-text-muted">
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-surface-hover text-text-muted/50">
          {icon}
        </div>
        <span>{title} 未提供</span>
      </div>
    );
  }

  if (kind === "image") {
    return (
      <div className="group glass-card overflow-hidden p-0">
        <div className="relative h-40 overflow-hidden bg-black/30">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={asset.publicUrl}
            alt={asset.fileName}
            className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-105"
          />
          <div className="absolute inset-0 bg-gradient-to-t from-black/50 to-transparent" />
          <div className="absolute bottom-0 left-0 right-0 flex items-center gap-2 px-3 py-2 text-xs text-white/90">
            {icon}
            <span className="font-medium">{title}</span>
          </div>
        </div>
        <div className="px-3 py-2 text-[11px] text-text-muted">
          <span className="truncate">{asset.fileName}</span>
          {formatBytes(asset.sizeBytes) ? (
            <span className="ml-2">{formatBytes(asset.sizeBytes)}</span>
          ) : null}
        </div>
      </div>
    );
  }

  // audio
  return (
    <div className="glass-card overflow-hidden p-0">
      <div className="flex items-center gap-3 px-3 py-2.5">
        <div className="flex items-center gap-2 text-sm font-medium text-text-primary">
          {icon}
          <span>{title}</span>
        </div>
        <audio controls src={asset.publicUrl} className="h-8 flex-1" style={{ minWidth: 0 }} />
      </div>
      <div className="border-t border-border/30 px-3 py-1.5 text-[11px] text-text-muted">
        <span className="truncate">{asset.fileName}</span>
        {formatBytes(asset.sizeBytes) ? (
          <span className="ml-2">{formatBytes(asset.sizeBytes)}</span>
        ) : null}
      </div>
    </div>
  );
}

/* ─── Copyable Info Row ─── */

function InfoRow({ icon, label, value }: { icon: ReactNode; label: string; value?: string | null }) {
  const display = value?.trim() || "—";
  const isCopyable = Boolean(value?.trim());

  return (
    <div className="flex items-center gap-3 py-2">
      <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-surface-hover text-text-muted">
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <p className="text-[11px] uppercase tracking-wider text-text-muted">{label}</p>
        <p className="mt-0.5 truncate text-sm text-text-primary">{display}</p>
      </div>
      {isCopyable && (
        <button
          type="button"
          onClick={() => copyToClipboard(display)}
          className="shrink-0 rounded-md p-1.5 text-text-muted/50 transition-colors hover:bg-surface-hover hover:text-text-secondary"
          title="复制"
        >
          <Copy className="h-3 w-3" />
        </button>
      )}
    </div>
  );
}

/* ─── Main Page ─── */

export default function DigitalHumanTaskDetailPage() {
  const params = useParams<{ taskId: string }>();
  const taskId = Array.isArray(params.taskId) ? params.taskId[0] : params.taskId;

  const { data: task, isLoading, error } = useQuery<DigitalHumanTask>({
    queryKey: ["digitalHumanTask", taskId],
    queryFn: () => getDigitalHumanTask(taskId),
    enabled: Boolean(taskId),
    refetchInterval: ({ state }) => {
      const item = state.data as DigitalHumanTask | undefined;
      return item && !isTerminalDigitalHumanTask(item)
        ? DIGITAL_HUMAN_POLL_INTERVAL_MS
        : false;
    },
  });

  const progressValue = clampProgress(task);
  const isCompleted = task?.status === "completed";
  const isRunning = task ? ["queued", "running"].includes(task.status) : false;

  return (
    <>
      <PageHeader
        title="任务详情"
        actions={
          <div className="flex items-center gap-2">
            <Link
              href="/creation/digital-human/history"
              className="btn-neon inline-flex items-center gap-1.5 rounded-xl border border-border px-3.5 py-2 text-sm font-medium text-text-primary transition-all hover:border-accent hover:text-accent"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              历史
            </Link>
            <Link
              href="/creation/digital-human"
              className="btn-neon inline-flex items-center gap-1.5 rounded-xl border border-border px-3.5 py-2 text-sm font-medium text-text-primary transition-all hover:border-accent hover:text-accent"
            >
              <Sparkles className="h-3.5 w-3.5" />
              新建
            </Link>
          </div>
        }
      />

      {isLoading ? (
        <div className="glass-card flex items-center justify-center py-20">
          <div className="mr-3 h-5 w-5 animate-spin rounded-full border-2 border-accent border-t-transparent" />
          <span className="text-sm text-text-secondary">正在读取任务详情...</span>
        </div>
      ) : error || !task ? (
        <div className="rounded-2xl border border-danger/30 bg-danger/10 px-5 py-4 text-sm text-danger">
          {error instanceof Error ? error.message : "数字人任务不存在或读取失败"}
        </div>
      ) : (
        <motion.div
          variants={stagger}
          initial="hidden"
          animate="show"
          className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_360px]"
        >
          {/* ─── Left Column: Video + Assets ─── */}
          <div className="space-y-5">
            {/* Video Result — hero area */}
            <motion.section variants={fadeUp} className="glass-card-elevated overflow-hidden p-0">
              {task.resultAsset?.publicUrl ? (
                <>
                  <div className="relative overflow-hidden rounded-t-xl bg-black">
                    <video
                      src={task.resultAsset.publicUrl}
                      controls
                      preload="metadata"
                      className="mx-auto block w-full bg-black"
                      style={{ maxHeight: "480px", objectFit: "contain" }}
                    />
                  </div>
                  <div className="flex items-center justify-between border-t border-border/30 px-5 py-3">
                    <div className="flex items-center gap-2 text-sm font-medium text-text-primary">
                      <Video className="h-4 w-4 text-accent" />
                      视频成品
                      <CheckCircle2 className="h-3.5 w-3.5 text-success" />
                    </div>
                    <a
                      href={task.resultAsset.publicUrl}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1.5 rounded-lg bg-gradient-to-r from-accent/15 to-cyan/15 px-3 py-1.5 text-xs font-medium text-accent transition-all hover:from-accent/25 hover:to-cyan/25"
                    >
                      <Download className="h-3 w-3" />
                      下载
                    </a>
                  </div>
                </>
              ) : (
                <div className="flex flex-col items-center justify-center py-16 text-center">
                  <div className="mb-3 flex h-14 w-14 items-center justify-center rounded-2xl bg-accent/10 text-accent/50">
                    <Video className="h-6 w-6" />
                  </div>
                  <p className="text-sm font-medium text-text-secondary">
                    {isRunning ? "视频生成中..." : "视频成品暂未生成"}
                  </p>
                  {isRunning && (
                    <p className="mt-1 text-xs text-text-muted">
                      任务完成后视频将自动显示在这里
                    </p>
                  )}
                </div>
              )}
            </motion.section>

            {/* Source Materials */}
            <motion.section variants={fadeUp}>
              <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-text-primary">
                <ImageIcon className="h-4 w-4 text-accent" />
                提交素材
              </div>
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                <AssetCard
                  title="人物照片"
                  icon={<ImageIcon className="h-3.5 w-3.5" />}
                  asset={task.characterAsset}
                  kind="image"
                />
                {task.goodsAsset ? (
                  <AssetCard
                    title="产品图"
                    icon={<Package className="h-3.5 w-3.5" />}
                    asset={task.goodsAsset}
                    kind="image"
                  />
                ) : null}
                <div className="sm:col-span-2 lg:col-span-1">
                  <AssetCard
                    title="参考语音"
                    icon={<AudioLines className="h-3.5 w-3.5" />}
                    asset={task.refAudioAsset}
                    kind="audio"
                  />
                </div>
              </div>
            </motion.section>

            {/* Script Text */}
            <motion.section variants={fadeUp} className="glass-card p-5">
              <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-text-primary">
                <FileText className="h-4 w-4 text-accent" />
                口播文案
              </div>
              <div className="rounded-xl border border-border/40 bg-background/20 px-4 py-3 text-sm leading-7 text-text-primary whitespace-pre-wrap">
                {task.goodsText || "—"}
              </div>
            </motion.section>
          </div>

          {/* ─── Right Column: Status + Meta ─── */}
          <motion.div variants={fadeUp} className="space-y-4">
            {/* Status Card */}
            <div className="glass-card-elevated p-5">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <h2 className="truncate text-lg font-semibold text-text-primary">
                    {task.goodsTitle?.trim() || "自定义口播任务"}
                  </h2>
                  <div className="mt-1.5 flex flex-wrap items-center gap-2">
                    <StatusBadge status={task.status} />
                    <span className="rounded-full bg-surface-hover px-2 py-0.5 text-[11px] text-text-muted">
                      {formatDigitalHumanMode(task.mode)}
                    </span>
                    <span className="rounded-full bg-surface-hover px-2 py-0.5 text-[11px] text-text-muted">
                      {task.modelName || "未记录模型"}
                    </span>
                  </div>
                </div>
              </div>

              {/* Progress */}
              {!isTerminalDigitalHumanTask(task) || isCompleted ? (
                <div className="mt-4">
                  <div className="flex items-center justify-between text-xs text-text-secondary">
                    <span className="truncate">
                      {isCompleted
                        ? "生成完毕"
                        : task.progress?.message || "等待生成进度..."}
                    </span>
                    <span className="ml-2 shrink-0 font-mono font-medium text-accent">
                      {progressValue.toFixed(0)}%
                    </span>
                  </div>
                  <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-background/50">
                    <motion.div
                      className="h-full rounded-full bg-gradient-to-r from-accent to-cyan"
                      initial={{ width: 0 }}
                      animate={{ width: `${progressValue}%` }}
                      transition={{ duration: 0.5 }}
                      style={{
                        boxShadow: progressValue > 0 ? "0 0 8px rgba(177,73,255,0.35)" : "none",
                      }}
                    />
                  </div>
                </div>
              ) : null}

              {/* Reminder */}
              {isRunning && (
                <div className="mt-3 rounded-xl border border-warning/20 bg-warning/8 px-3 py-2 text-xs leading-5 text-text-secondary">
                  <span className="mr-1 text-warning">💡</span>
                  {DIGITAL_HUMAN_REMINDER_TEXT}
                </div>
              )}

              {/* Error */}
              {task.errorMessage ? (
                <div className="mt-3 rounded-xl border border-danger/30 bg-danger/10 px-3 py-2 text-xs text-danger">
                  {task.errorMessage}
                </div>
              ) : null}

              {/* Download button */}
              {isCompleted && task.resultAsset?.publicUrl ? (
                <a
                  href={task.resultAsset.publicUrl}
                  target="_blank"
                  rel="noreferrer"
                  className="mt-4 flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-accent to-[#7c3aed] px-4 py-2.5 text-sm font-semibold text-white shadow-[0_0_16px_rgba(177,73,255,0.2)] transition-all hover:shadow-[0_0_24px_rgba(177,73,255,0.3)] hover:brightness-110"
                >
                  <Download className="h-4 w-4" />
                  下载视频成品
                </a>
              ) : null}
            </div>

            {/* Task Info */}
            <div className="glass-card p-4">
              <div className="mb-2 text-xs font-semibold uppercase tracking-wider text-text-muted">
                任务信息
              </div>
              <div className="divide-y divide-border/30">
                <InfoRow
                  icon={<Hash className="h-3 w-3" />}
                  label="任务 ID"
                  value={task.id}
                />
                <InfoRow
                  icon={<Sparkles className="h-3 w-3" />}
                  label="执行模型"
                  value={task.modelName}
                />
                <InfoRow
                  icon={<Hash className="h-3 w-3" />}
                  label="远端 Task ID"
                  value={task.remoteTaskId}
                />
                <InfoRow
                  icon={<Calendar className="h-3 w-3" />}
                  label="创建时间"
                  value={formatDateTime(task.createdAt)}
                />
                <InfoRow
                  icon={<Clock className="h-3 w-3" />}
                  label="更新时间"
                  value={formatDateTime(task.updatedAt)}
                />
                {task.goodsTitle ? (
                  <InfoRow
                    icon={<Package className="h-3 w-3" />}
                    label="产品标题"
                    value={task.goodsTitle}
                  />
                ) : null}
              </div>
            </div>
          </motion.div>
        </motion.div>
      )}
    </>
  );
}
