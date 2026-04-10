"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowLeft,
  AudioLines,
  Download,
  FileText,
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

function InfoItem({ label, value }: { label: string; value?: string | null }) {
  return (
    <div className="rounded-2xl border border-border bg-surface px-4 py-3">
      <p className="text-[11px] uppercase tracking-wider text-text-muted">{label}</p>
      <p className="mt-2 text-sm text-text-primary">{value?.trim() ? value : "—"}</p>
    </div>
  );
}

function AssetPreview({
  title,
  icon,
  asset,
  kind,
}: {
  title: string;
  icon: ReactNode;
  asset?: DigitalHumanAsset | null;
  kind: "image" | "audio" | "video";
}) {
  if (!asset?.publicUrl) {
    return (
      <div className="rounded-2xl border border-dashed border-border bg-surface px-4 py-10 text-center text-sm text-text-muted">
        <div className="mb-3 inline-flex h-10 w-10 items-center justify-center rounded-full bg-accent/10 text-accent">
          {icon}
        </div>
        <p>{title}暂未提供</p>
      </div>
    );
  }

  return (
    <div className="rounded-3xl border border-border bg-surface p-5">
      <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-text-primary">
        {icon}
        <span>{title}</span>
      </div>

      {kind === "image" ? (
        <div className="overflow-hidden rounded-2xl border border-border bg-black/40">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={asset.publicUrl} alt={asset.fileName} className="max-h-[420px] w-full object-contain" />
        </div>
      ) : null}

      {kind === "audio" ? (
        <div className="rounded-2xl border border-border bg-background/40 p-4">
          <audio controls src={asset.publicUrl} className="w-full" />
        </div>
      ) : null}

      {kind === "video" ? (
        <div className="overflow-hidden rounded-2xl border border-border bg-black/60">
          <video src={asset.publicUrl} controls preload="metadata" className="w-full bg-black" />
        </div>
      ) : null}

      <div className="mt-4 flex flex-wrap gap-x-4 gap-y-2 text-xs text-text-secondary">
        <span className="break-all">{asset.fileName}</span>
        <span>{asset.mimeType}</span>
        {typeof asset.sizeBytes === "number" ? <span>{asset.sizeBytes} bytes</span> : null}
      </div>
    </div>
  );
}

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

  return (
    <>
      <PageHeader
        title="数字人任务详情"
        subtitle="查看已提交的素材、口播文案、云端轮询进度，以及最终下载回流的视频成品。"
        actions={
          <div className="flex items-center gap-3">
            <Link
              href="/creation/digital-human/history"
              className="inline-flex items-center gap-2 rounded-xl border border-border px-4 py-2 text-sm font-medium text-text-primary transition-colors hover:border-accent hover:text-accent"
            >
              <ArrowLeft className="h-4 w-4" />
              返回历史
            </Link>
            <Link
              href="/creation/digital-human"
              className="inline-flex items-center gap-2 rounded-xl border border-border px-4 py-2 text-sm font-medium text-text-primary transition-colors hover:border-accent hover:text-accent"
            >
              <Sparkles className="h-4 w-4" />
              新建任务
            </Link>
          </div>
        }
      />

      {isLoading ? (
        <div className="flex items-center justify-center rounded-2xl border border-border py-16 text-text-secondary">
          <div className="mr-3 h-5 w-5 animate-spin rounded-full border-2 border-accent border-t-transparent" />
          正在读取任务详情...
        </div>
      ) : error || !task ? (
        <div className="rounded-2xl border border-danger/30 bg-danger/10 px-5 py-4 text-sm text-danger">
          {error instanceof Error ? error.message : "数字人任务不存在或读取失败"}
        </div>
      ) : (
        <div className="space-y-6">
          <section className="rounded-3xl border border-border bg-surface p-6">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-3">
                  <h2 className="text-2xl font-semibold text-text-primary">
                    {task.goodsTitle?.trim() || "未填写产品简介"}
                  </h2>
                  <StatusBadge status={task.status} size="md" />
                  <span className="rounded-full bg-background/60 px-3 py-1 text-xs text-text-secondary">
                    {formatDigitalHumanMode(task.mode)}
                  </span>
                </div>

                <div className="mt-4 rounded-2xl border border-warning/30 bg-warning/10 px-4 py-3 text-sm leading-6 text-text-primary">
                  {DIGITAL_HUMAN_REMINDER_TEXT}
                </div>

                <div className="mt-5">
                  <div className="flex items-center justify-between text-sm text-text-secondary">
                    <span>{task.progress?.message || "等待远端进度"}</span>
                    <span>{progressValue.toFixed(0)}%</span>
                  </div>
                  <div className="mt-2 h-2 overflow-hidden rounded-full bg-background/60">
                    <div
                      className="h-full rounded-full bg-accent transition-[width] duration-500"
                      style={{ width: `${progressValue}%` }}
                    />
                  </div>
                </div>
              </div>

              <div className="flex flex-wrap gap-3">
                {task.resultAsset?.publicUrl ? (
                  <a
                    href={task.resultAsset.publicUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-2 rounded-xl border border-border px-4 py-2 text-sm font-medium text-text-primary transition-colors hover:border-accent hover:text-accent"
                  >
                    <Download className="h-4 w-4" />
                    下载成品
                  </a>
                ) : null}
              </div>
            </div>

            <div className="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <InfoItem label="任务 ID" value={task.id} />
              <InfoItem label="远端 Task ID" value={task.remoteTaskId} />
              <InfoItem label="创建时间" value={formatDateTime(task.createdAt)} />
              <InfoItem label="更新时间" value={formatDateTime(task.updatedAt)} />
            </div>

            {task.errorMessage ? (
              <div className="mt-4 rounded-2xl border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">
                {task.errorMessage}
              </div>
            ) : null}
          </section>

          <section className="grid gap-6 xl:grid-cols-2">
            <AssetPreview
              title="人物照片"
              icon={<ImageIcon className="h-4 w-4 text-accent" />}
              asset={task.characterAsset}
              kind="image"
            />
            <AssetPreview
              title="参考语音"
              icon={<AudioLines className="h-4 w-4 text-accent" />}
              asset={task.refAudioAsset}
              kind="audio"
            />
            {task.goodsAsset ? (
              <AssetPreview
                title="参考产品图"
                icon={<Package className="h-4 w-4 text-accent" />}
                asset={task.goodsAsset}
                kind="image"
              />
            ) : (
              <div className="rounded-3xl border border-dashed border-border bg-surface px-4 py-10 text-center text-sm text-text-muted">
                当前任务未提交参考产品图。
              </div>
            )}
            <div className="rounded-3xl border border-border bg-surface p-5">
              <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-text-primary">
                <FileText className="h-4 w-4 text-accent" />
                <span>提交文案</span>
              </div>
              <div className="space-y-4">
                <InfoItem label="产品简介" value={task.goodsTitle} />
                <div className="rounded-2xl border border-border bg-background/30 px-4 py-4 text-sm leading-7 text-text-primary whitespace-pre-wrap">
                  {task.goodsText}
                </div>
              </div>
            </div>
          </section>

          <section>
            <AssetPreview
              title="视频成品"
              icon={<Video className="h-4 w-4 text-accent" />}
              asset={task.resultAsset}
              kind="video"
            />
          </section>
        </div>
      )}
    </>
  );
}
