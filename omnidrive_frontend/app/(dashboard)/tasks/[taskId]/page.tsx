"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { motion } from "framer-motion";
import {
  AlertTriangle,
  ArrowLeft,
  Boxes,
  Calendar,
  Check,
  CheckCircle2,
  Clock,
  ExternalLink,
  MonitorSmartphone,
  Package,
  PlaySquare,
  RefreshCcw,
  X,
} from "lucide-react";
import { getTaskWorkspace } from "@/lib/services";
import type { PublishTaskArtifact, PublishTaskMaterialRef, PublishTaskWorkspace } from "@/lib/types";
import { PageHeader, StatusBadge } from "@/components/ui/common";
import { formatDateTime } from "@/lib/workflow";

function prettyJSON(value: unknown) {
  if (!value || (typeof value === "object" && Object.keys(value as Record<string, unknown>).length === 0)) {
    return "";
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function sortByLatest<T extends { updatedAt?: string | null; createdAt?: string | null }>(items: T[]) {
  return [...items].sort((left, right) => {
    const leftTime = new Date(left.updatedAt || left.createdAt || 0).getTime();
    const rightTime = new Date(right.updatedAt || right.createdAt || 0).getTime();
    return rightTime - leftTime;
  });
}

function isImageArtifact(artifact: PublishTaskArtifact) {
  return (
    artifact.artifactType === "image" ||
    (artifact.mimeType || "").startsWith("image/") ||
    /\.(png|jpe?g|gif|webp|bmp)$/i.test(artifact.fileName || "")
  );
}

function isVideoArtifact(artifact: PublishTaskArtifact) {
  return (
    artifact.artifactType === "video" ||
    (artifact.mimeType || "").startsWith("video/") ||
    /\.(mp4|mov|webm|m4v)$/i.test(artifact.fileName || "")
  );
}

function ArtifactCard({ artifact }: { artifact: PublishTaskArtifact }) {
  const title = artifact.title || artifact.fileName || artifact.artifactKey || artifact.artifactType;

  return (
    <div className="overflow-hidden rounded-2xl border border-border bg-surface">
      <div className="flex items-start justify-between gap-3 border-b border-border/60 bg-surface-hover/30 px-4 py-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-text-primary">{title}</p>
          <p className="mt-1 text-xs text-text-secondary">
            {artifact.artifactType} · {formatDateTime(artifact.updatedAt || artifact.createdAt)}
          </p>
        </div>
        {artifact.publicUrl ? (
          <a
            href={artifact.publicUrl}
            target="_blank"
            rel="noreferrer"
            className="inline-flex shrink-0 items-center gap-1 rounded-lg border border-border px-2.5 py-1.5 text-xs font-medium text-text-primary transition-colors hover:border-accent hover:text-accent"
          >
            查看
            <ExternalLink className="h-3 w-3" />
          </a>
        ) : null}
      </div>

      <div className="space-y-3 p-4">
        {artifact.publicUrl && isImageArtifact(artifact) ? (
          <div className="overflow-hidden rounded-xl border border-border bg-black/40">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={artifact.publicUrl} alt={title} className="max-h-[420px] w-full object-contain" />
          </div>
        ) : null}

        {artifact.publicUrl && isVideoArtifact(artifact) ? (
          <div className="overflow-hidden rounded-xl border border-border bg-black/50">
            <video src={artifact.publicUrl} controls preload="metadata" className="max-h-[420px] w-full bg-black" />
          </div>
        ) : null}

        {artifact.textContent ? (
          <div className="rounded-xl border border-border bg-surface-hover/50 px-4 py-3 text-sm leading-6 text-text-secondary whitespace-pre-wrap">
            {artifact.textContent}
          </div>
        ) : null}

        {artifact.payload ? (
          <pre className="overflow-x-auto rounded-xl border border-border bg-background/70 p-3 text-xs leading-6 text-text-secondary">
            {prettyJSON(artifact.payload)}
          </pre>
        ) : null}
      </div>
    </div>
  );
}

function MaterialCard({ material }: { material: PublishTaskMaterialRef }) {
  return (
    <div className="rounded-2xl border border-border bg-surface px-4 py-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-text-primary">{material.name}</p>
          <p className="mt-1 text-xs text-text-secondary">
            {material.rootName} / {material.relativePath}
          </p>
          <p className="mt-2 text-xs text-text-secondary">
            角色：{material.role} · 类型：{material.kind}
            {material.sizeBytes ? ` · ${(material.sizeBytes / 1024 / 1024).toFixed(2)} MB` : ""}
          </p>
        </div>
        <StatusBadge status={material.kind === "directory" ? "active" : "success"} />
      </div>
      {material.previewText ? (
        <p className="mt-3 line-clamp-4 text-sm leading-6 text-text-secondary">{material.previewText}</p>
      ) : null}
    </div>
  );
}

function DetailField({
  label,
  value,
}: {
  label: string;
  value?: string | null;
}) {
  return (
    <div className="rounded-xl border border-border bg-surface px-4 py-3">
      <p className="text-[11px] uppercase tracking-wider text-text-muted">{label}</p>
      <p className="mt-2 text-sm font-medium text-text-primary">{value && value.trim() ? value : "—"}</p>
    </div>
  );
}

export default function TaskDetailPage() {
  const params = useParams();
  const router = useRouter();
  const taskId = params.taskId as string;

  const { data: workspace, isLoading } = useQuery<PublishTaskWorkspace>({
    queryKey: ["taskWorkspace", taskId],
    queryFn: () => getTaskWorkspace(taskId),
    enabled: Boolean(taskId),
    refetchInterval: 5000,
  });

  const task = workspace?.task;
  const artifacts = useMemo(() => sortByLatest(workspace?.artifacts || []), [workspace?.artifacts]);
  const materials = useMemo(() => sortByLatest(workspace?.materials || []), [workspace?.materials]);
  const skillName = workspace?.skill?.name || "直接发布";
  const requestedRun = task?.runAt || null;
  const executionTime = workspace?.bridge.startedAt || null;
  const resultTime = task?.finishedAt || workspace?.bridge.finishedAt || null;

  if (isLoading && !workspace) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center text-text-muted">
        <div className="mb-4 h-8 w-8 animate-spin rounded-full border-2 border-accent border-t-transparent" />
        正在读取任务详情...
      </div>
    );
  }

  if (!task || !workspace) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center text-text-muted">
        <div className="mb-4 h-8 w-8 rounded-full border border-border bg-surface-hover" />
        任务不存在，或者你当前没有访问权限。
      </div>
    );
  }

  const isNeedsVerify = task.status === "needs_verify";
  const vp = task.verificationPayload as Record<string, string> | null | undefined;

  return (
    <>
      <button
        onClick={() => router.back()}
        className="mb-4 flex items-center gap-2 text-sm font-medium text-text-muted transition-colors hover:text-text-primary"
      >
        <ArrowLeft className="h-4 w-4" />
        返回任务列表
      </button>

      <PageHeader
        title={task.title || "未命名任务"}
        subtitle={`任务详情 · ID: ${task.id}`}
        actions={<StatusBadge status={task.status} />}
      />

      {isNeedsVerify && vp ? (
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="mb-6 overflow-hidden rounded-2xl border border-amber-500/30 bg-amber-500/10 shadow-lg shadow-amber-500/5">
          <div className="flex items-center gap-3 border-b border-amber-500/20 bg-amber-500/15 p-4 text-amber-500">
            <AlertTriangle className="h-5 w-5" />
            <h3 className="font-bold">拦截人工验证：请确认内容无误后再发布</h3>
          </div>
          <div className="p-6">
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
              <div>
                <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-text-muted">截屏预览</p>
                <div className="overflow-hidden rounded-xl border border-border bg-black">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src={vp.screenshotUrl} alt="验证预留图" className="w-full object-contain" />
                </div>
              </div>
              <div className="space-y-4">
                <div>
                  <p className="mb-1 text-xs font-semibold uppercase tracking-wider text-text-muted">准备填写的标题</p>
                  <p className="rounded-lg bg-surface-hover p-3 text-sm font-medium text-text-primary">{vp.generatedTitle}</p>
                </div>
                <div>
                  <p className="mb-1 text-xs font-semibold uppercase tracking-wider text-text-muted">准备填写的正文</p>
                  <p className="rounded-lg bg-surface-hover p-3 text-sm text-text-secondary whitespace-pre-wrap">{vp.contentPreview}</p>
                </div>
                <div className="flex gap-3 pt-2">
                  <button className="flex flex-1 items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-emerald-500 to-emerald-400 py-3 text-sm font-bold text-white shadow-lg shadow-emerald-500/20 transition-all hover:shadow-emerald-500/40">
                    <Check className="h-4 w-4" />
                    确认并继续发布
                  </button>
                  <button className="flex flex-1 items-center justify-center gap-2 rounded-xl border border-danger/50 bg-danger/10 py-3 text-sm font-bold text-danger transition-all hover:bg-danger/20">
                    <X className="h-4 w-4" />
                    放弃任务
                  </button>
                </div>
              </div>
            </div>
          </div>
        </motion.div>
      ) : null}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-1">
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="glass-card p-5">
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-text-muted">基本信息</h3>
            <div className="space-y-4">
              <div>
                <p className="mb-1 text-[11px] text-text-muted">当前进度反馈</p>
                <div className="flex items-start gap-2 rounded-lg bg-surface-hover p-3">
                  {task.status === "success" || task.status === "completed" ? (
                    <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-400" />
                  ) : task.status === "failed" ? (
                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-rose-400" />
                  ) : task.status === "running" ? (
                    <div className="mt-1 h-3 w-3 shrink-0 animate-spin rounded-full border-2 border-accent border-t-transparent" />
                  ) : (
                    <Clock className="mt-0.5 h-4 w-4 shrink-0 text-amber-400" />
                  )}
                  <p className="text-sm text-text-secondary">{task.message || "暂无日志"}</p>
                </div>
              </div>
              <div className="rounded-xl bg-surface-hover p-3">
                <p className="text-[11px] text-text-muted">桥接状态</p>
                <p className="mt-1 text-sm font-medium text-text-primary">
                  {workspace.bridge.origin} · {workspace.bridge.stage || "—"}
                </p>
                <p className="mt-2 text-sm leading-6 text-text-secondary">
                  本地状态 {workspace.bridge.localStatus || "—"}，最近同步 {formatDateTime(workspace.bridge.lastAgentSyncAt || workspace.runtime?.updatedAt)}
                </p>
              </div>
            </div>
          </motion.div>

          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 }} className="glass-card p-5">
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-text-muted">执行节点与分发链路</h3>
            <div className="space-y-4">
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-accent/10">
                  <MonitorSmartphone className="h-4 w-4 text-accent" />
                </div>
                <div>
                  <p className="text-[11px] text-text-muted">运行设备</p>
                  <p className="text-sm font-medium text-text-primary">{workspace.device?.name || "未知设备"}</p>
                </div>
              </div>
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-cyan/10">
                  <PlaySquare className="h-4 w-4 text-cyan" />
                </div>
                <div>
                  <p className="text-[11px] text-text-muted">分发平台账号</p>
                  <p className="text-sm font-medium text-text-primary">
                    {task.platform} · {task.accountName}
                  </p>
                  <p className="text-[10px] text-text-muted">ID: {workspace.account?.id || task.accountId || "—"}</p>
                </div>
              </div>
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-purple-500/10">
                  <RefreshCcw className="h-4 w-4 text-purple-400" />
                </div>
                <div>
                  <p className="text-[11px] text-text-muted">关联技能</p>
                  <p className="text-sm font-medium text-text-primary">{workspace.skill?.name || "直接发布"}</p>
                </div>
              </div>
            </div>
          </motion.div>

          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="glass-card p-5">
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-text-muted">时间线与准备度</h3>
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Calendar className="h-4 w-4 text-text-muted" />
                <span className="text-sm text-text-secondary">创建时间：</span>
                <span className="text-sm font-medium text-text-primary">{formatDateTime(task.createdAt)}</span>
              </div>
              <div className="flex items-center gap-2">
                <Calendar className="h-4 w-4 text-text-muted" />
                <span className="text-sm text-text-secondary">最近更新：</span>
                <span className="text-sm font-medium text-text-primary">{formatDateTime(task.updatedAt)}</span>
              </div>
              {task.runAt ? (
                <div className="flex items-center gap-2">
                  <Clock className="h-4 w-4 text-text-muted" />
                  <span className="text-sm text-text-secondary">执行时间：</span>
                  <span className="text-sm font-medium text-text-primary">{formatDateTime(task.runAt)}</span>
                </div>
              ) : null}
              {task.finishedAt ? (
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-4 w-4 text-emerald-400" />
                  <span className="text-sm text-text-secondary">完成时间：</span>
                  <span className="text-sm font-medium text-text-primary">{formatDateTime(task.finishedAt)}</span>
                </div>
              ) : null}
              <div className="rounded-xl bg-surface-hover p-3 text-sm leading-6 text-text-secondary">
                素材就绪 {workspace.readiness.availableMaterialCount}/{workspace.readiness.totalMaterialCount}，
                缺失 {workspace.readiness.missingMaterialCount}，
                技能同步 {workspace.readiness.skillSyncedToDevice ? "已完成" : "未完成"}。
              </div>
            </div>
          </motion.div>
        </div>

        <div className="space-y-6 lg:col-span-2">
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="glass-card overflow-hidden">
            <div className="border-b border-border/50 bg-surface-hover/30 p-5">
              <h3 className="flex items-center gap-2 text-sm font-semibold text-text-primary">
                <Package className="h-4 w-4 text-accent" />
                任务参数
              </h3>
            </div>
            <div className="grid grid-cols-1 gap-3 p-5 md:grid-cols-2">
              <DetailField label="skillName" value={skillName} />
              <DetailField label="platform" value={task.platform} />
              <DetailField label="title" value={task.title} />
              <DetailField label="accountName" value={task.accountName} />
              <DetailField label="requestedRun" value={requestedRun ? formatDateTime(requestedRun) : "—"} />
              <DetailField label="执行时间" value={executionTime ? formatDateTime(executionTime) : "—"} />
              <DetailField label="结果时间" value={resultTime ? formatDateTime(resultTime) : "—"} />
            </div>
          </motion.div>

          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 }} className="glass-card overflow-hidden">
            <div className="border-b border-border/50 bg-surface-hover/30 p-5">
              <h3 className="flex items-center gap-2 text-sm font-semibold text-text-primary">
                <Boxes className="h-4 w-4 text-cyan" />
                任务产物
              </h3>
            </div>
            <div className="space-y-4 p-5">
              {artifacts.length > 0 ? (
                artifacts.map((artifact) => <ArtifactCard key={artifact.id} artifact={artifact} />)
              ) : (
                <div className="flex items-center justify-center gap-2 rounded-xl border border-dashed border-border py-8 text-center text-text-muted">
                  <Package className="h-5 w-5" />
                  <span className="text-sm">当前还没有回流产物。</span>
                </div>
              )}
            </div>
          </motion.div>

          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="glass-card overflow-hidden">
            <div className="border-b border-border/50 bg-surface-hover/30 p-5">
              <h3 className="text-sm font-semibold text-text-primary">本地素材</h3>
            </div>
            <div className="space-y-4 p-5">
              {materials.length > 0 ? (
                materials.map((material) => <MaterialCard key={material.id} material={material} />)
              ) : (
                <p className="text-sm text-text-muted">当前任务没有挂接本地素材。</p>
              )}
            </div>
          </motion.div>
        </div>
      </div>
    </>
  );
}
