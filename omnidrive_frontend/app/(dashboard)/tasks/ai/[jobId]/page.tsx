"use client";

import { useQuery } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { motion } from "framer-motion";
import {
  ArrowLeft,
  Bot,
  Boxes,
  Calendar,
  Cpu,
  ExternalLink,
  Image as ImageIcon,
  Package,
  Wand2,
} from "lucide-react";
import { PageHeader, StatusBadge } from "@/components/ui/common";
import { getModelDisplayName } from "@/lib/model-display";
import { getAIJobWorkspace, listDevices } from "@/lib/services";
import type { AIJobArtifact, AIJobWorkspace, Device } from "@/lib/types";
import { buildAIJobTitle, formatDateTime, resolveAIJobStage } from "@/lib/workflow";

function sortByLatest<T extends { updatedAt?: string | null; createdAt?: string | null }>(items: T[]) {
  return [...items].sort((left, right) => {
    const leftTime = new Date(left.updatedAt || left.createdAt || 0).getTime();
    const rightTime = new Date(right.updatedAt || right.createdAt || 0).getTime();
    return rightTime - leftTime;
  });
}

function isImageArtifact(artifact: AIJobArtifact) {
  return (
    artifact.artifactType === "image" ||
    (artifact.mimeType || "").startsWith("image/") ||
    /\.(png|jpe?g|gif|webp|bmp)$/i.test(artifact.fileName || "")
  );
}

function isVideoArtifact(artifact: AIJobArtifact) {
  return (
    artifact.artifactType === "video" ||
    (artifact.mimeType || "").startsWith("video/") ||
    /\.(mp4|mov|webm|m4v)$/i.test(artifact.fileName || "")
  );
}

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

type LocalMaterialItem = {
  id: string;
  title: string;
  subtitle?: string;
  href?: string | null;
};

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : null;
}

function getString(value: unknown) {
  return typeof value === "string" && value.trim() ? value.trim() : "";
}

function extractLocalMaterials(inputPayload?: Record<string, unknown> | null): LocalMaterialItem[] {
  if (!inputPayload) {
    return [];
  }

  const candidates: LocalMaterialItem[] = [];
  const appendItems = (items: unknown[], sourceLabel: string) => {
    items.forEach((item, index) => {
      if (typeof item === "string" && item.trim()) {
        candidates.push({
          id: `${sourceLabel}-${index}-${item}`,
          title: item.trim(),
          subtitle: sourceLabel,
        });
        return;
      }
      const record = asRecord(item);
      if (!record) {
        return;
      }
      const title =
        getString(record.name) ||
        getString(record.fileName) ||
        getString(record.path) ||
        getString(record.relativePath) ||
        `${sourceLabel} ${index + 1}`;
      const root = getString(record.rootName) || getString(record.root);
      const path = getString(record.relativePath) || getString(record.path);
      const mimeType = getString(record.mimeType);
      const subtitle = [root && path ? `${root} / ${path}` : root || path, mimeType || sourceLabel]
        .filter(Boolean)
        .join(" · ");
      candidates.push({
        id: getString(record.id) || `${sourceLabel}-${index}-${title}`,
        title,
        subtitle: subtitle || sourceLabel,
        href: getString(record.publicUrl) || getString(record.url) || null,
      });
    });
  };

  const materialRefs = inputPayload.materialRefs;
  if (Array.isArray(materialRefs)) {
    appendItems(materialRefs, "本地素材");
  }
  const localMaterials = inputPayload.localMaterials;
  if (Array.isArray(localMaterials)) {
    appendItems(localMaterials, "本地素材");
  }
  const referenceImages = inputPayload.referenceImages;
  if (Array.isArray(referenceImages)) {
    appendItems(referenceImages, "参考图");
  }
  const attachments = inputPayload.attachments;
  if (Array.isArray(attachments)) {
    appendItems(attachments, "附件");
  }

  return candidates;
}

function ArtifactCard({ artifact }: { artifact: AIJobArtifact }) {
  const label = artifact.title || artifact.fileName || artifact.artifactKey || artifact.artifactType;

  return (
    <div className="overflow-hidden rounded-2xl border border-border bg-surface">
      <div className="flex items-start justify-between gap-3 border-b border-border/60 bg-surface-hover/30 px-4 py-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-text-primary">{label}</p>
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
            <img src={artifact.publicUrl} alt={label} className="max-h-[420px] w-full object-contain" />
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

export default function AIJobDetailPage() {
  const params = useParams();
  const router = useRouter();
  const jobId = params.jobId as string;

  const { data: workspace, isLoading } = useQuery<AIJobWorkspace>({
    queryKey: ["aiJobWorkspace", jobId],
    queryFn: () => getAIJobWorkspace(jobId),
    enabled: Boolean(jobId),
    refetchInterval: 5000,
  });
  const { data: devices = [] } = useQuery<Device[]>({
    queryKey: ["devices"],
    queryFn: listDevices,
  });

  if (isLoading && !workspace) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center text-text-muted">
        <div className="mb-4 h-8 w-8 animate-spin rounded-full border-2 border-accent border-t-transparent" />
        正在读取 AI 任务详情...
      </div>
    );
  }

  const job = workspace?.job;

  if (!job || !workspace) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center text-text-muted">
        <div className="mb-4 h-8 w-8 rounded-full border border-border bg-surface-hover" />
        任务不存在，或者你当前没有访问权限。
      </div>
    );
  }

  const deviceName =
    !job.deviceId ? "云端执行" : devices.find((item) => item.id === job.deviceId)?.name || job.deviceId;
  const artifacts = sortByLatest(workspace.artifacts || []);
  const publishTasks = sortByLatest(workspace.publishTasks || []);
  const stage = resolveAIJobStage(job);
  const inputPayload = asRecord(job.inputPayload);
  const localMaterials = extractLocalMaterials(inputPayload);
  const linkedTask = publishTasks[0];
  const skillName = workspace.skill?.name || getString(inputPayload?.skillName) || "—";
  const platform = linkedTask?.platform || getString(inputPayload?.platform) || "—";
  const title = getString(inputPayload?.title) || buildAIJobTitle(job);
  const accountName = linkedTask?.accountName || getString(inputPayload?.accountName) || "—";
  const requestedRun = getString(inputPayload?.publishAt) || job.runAt || null;
  const executionTime = workspace.bridge?.startedAt || null;
  const resultTime = job.finishedAt || workspace.bridge?.finishedAt || null;

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
        title={buildAIJobTitle(job)}
        subtitle={`AI 任务详情 · ID: ${job.id}`}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <div className="inline-flex rounded-full bg-surface-hover px-3 py-1 text-xs font-medium text-text-primary">
              {job.jobType}
            </div>
            <StatusBadge status={stage?.key || job.status} />
          </div>
        }
      />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-1">
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="glass-card p-5">
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-text-muted">基本信息</h3>
            <div className="space-y-4">
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-accent/10">
                  <Bot className="h-4 w-4 text-accent" />
                </div>
                <div>
                  <p className="text-[11px] text-text-muted">模型</p>
                  <p className="text-sm font-medium text-text-primary">{getModelDisplayName(job)}</p>
                </div>
              </div>
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-cyan/10">
                  <Cpu className="h-4 w-4 text-cyan" />
                </div>
                <div>
                  <p className="text-[11px] text-text-muted">执行节点</p>
                  <p className="text-sm font-medium text-text-primary">{deviceName}</p>
                </div>
              </div>
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-purple-500/10">
                  <Wand2 className="h-4 w-4 text-purple-400" />
                </div>
                <div>
                  <p className="text-[11px] text-text-muted">来源 / 技能</p>
                  <p className="text-sm font-medium text-text-primary">
                    {job.source}
                    {workspace.skill ? ` · ${workspace.skill.name}` : ""}
                  </p>
                </div>
              </div>
              <div className="rounded-xl bg-surface-hover p-3">
                <p className="text-[11px] text-text-muted">当前阶段</p>
                <p className="mt-1 text-sm font-semibold text-text-primary">{stage?.label || job.status}</p>
                <p className="mt-2 text-sm leading-6 text-text-secondary">
                  {stage?.description || job.deliveryMessage || job.message || "暂无额外说明"}
                </p>
              </div>
            </div>
          </motion.div>

          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 }} className="glass-card p-5">
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-text-muted">时间线</h3>
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Calendar className="h-4 w-4 text-text-muted" />
                <span className="text-sm text-text-secondary">创建时间：</span>
                <span className="text-sm font-medium text-text-primary">{formatDateTime(job.createdAt)}</span>
              </div>
              <div className="flex items-center gap-2">
                <Calendar className="h-4 w-4 text-text-muted" />
                <span className="text-sm text-text-secondary">最近更新：</span>
                <span className="text-sm font-medium text-text-primary">{formatDateTime(job.updatedAt)}</span>
              </div>
              {job.runAt ? (
                <div className="flex items-center gap-2">
                  <Calendar className="h-4 w-4 text-text-muted" />
                  <span className="text-sm text-text-secondary">计划执行：</span>
                  <span className="text-sm font-medium text-text-primary">{formatDateTime(job.runAt)}</span>
                </div>
              ) : null}
              {job.finishedAt ? (
                <div className="flex items-center gap-2">
                  <Calendar className="h-4 w-4 text-text-muted" />
                  <span className="text-sm text-text-secondary">完成时间：</span>
                  <span className="text-sm font-medium text-text-primary">{formatDateTime(job.finishedAt)}</span>
                </div>
              ) : null}
            </div>
          </motion.div>

          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="glass-card p-5">
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-text-muted">桥接状态</h3>
            <div className="space-y-3 text-sm text-text-secondary">
              <p>来源：<span className="font-medium text-text-primary">{workspace.bridge.origin || "cloud"}</span></p>
              <p>本地阶段：<span className="font-medium text-text-primary">{workspace.bridge.stage || "—"}</span></p>
              <p>投递状态：<span className="font-medium text-text-primary">{job.deliveryStatus || "—"}</span></p>
              <p>活跃租约：<span className="font-medium text-text-primary">{workspace.bridge.hasActiveLease ? "是" : "否"}</span></p>
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
              <DetailField label="platform" value={platform} />
              <DetailField label="title" value={title} />
              <DetailField label="accountName" value={accountName} />
              <DetailField label="requestedRun" value={requestedRun ? formatDateTime(requestedRun) : "—"} />
              <DetailField label="执行时间" value={executionTime ? formatDateTime(executionTime) : "—"} />
              <DetailField label="结果时间" value={resultTime ? formatDateTime(resultTime) : "—"} />
            </div>
          </motion.div>

          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 }} className="glass-card overflow-hidden">
            <div className="border-b border-border/50 bg-surface-hover/30 p-5">
              <h3 className="flex items-center gap-2 text-sm font-semibold text-text-primary">
                <Boxes className="h-4 w-4 text-cyan" />
                生成产物
              </h3>
            </div>
            <div className="space-y-4 p-5">
              {artifacts.length > 0 ? (
                artifacts.map((artifact) => <ArtifactCard key={artifact.id} artifact={artifact} />)
              ) : (
                <div className="flex items-center gap-2 rounded-xl border border-dashed border-border py-8 text-center text-text-muted justify-center">
                  <ImageIcon className="h-5 w-5" />
                  <span className="text-sm">当前还没有回流产物。</span>
                </div>
              )}
            </div>
          </motion.div>

          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="glass-card overflow-hidden">
            <div className="border-b border-border/50 bg-surface-hover/30 p-5">
              <h3 className="flex items-center gap-2 text-sm font-semibold text-text-primary">
                <Package className="h-4 w-4 text-purple-400" />
                本地素材
              </h3>
            </div>
            <div className="space-y-4 p-5">
              {localMaterials.length > 0 ? (
                localMaterials.map((item) => (
                  <div key={item.id} className="rounded-2xl border border-border bg-surface px-4 py-3">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-semibold text-text-primary">{item.title}</p>
                        <p className="mt-1 text-xs text-text-secondary">{item.subtitle || "本地素材"}</p>
                      </div>
                      {item.href ? (
                        <a
                          href={item.href}
                          target="_blank"
                          rel="noreferrer"
                          className="inline-flex shrink-0 items-center gap-1 rounded-lg border border-border px-2.5 py-1.5 text-xs font-medium text-text-primary transition-colors hover:border-accent hover:text-accent"
                        >
                          查看
                          <ExternalLink className="h-3 w-3" />
                        </a>
                      ) : null}
                    </div>
                  </div>
                ))
              ) : (
                <div className="flex items-center gap-2 rounded-xl border border-dashed border-border py-8 text-center text-text-muted justify-center">
                  <Package className="h-5 w-5" />
                  <span className="text-sm">当前没有本地素材。</span>
                </div>
              )}
            </div>
          </motion.div>
        </div>
      </div>
    </>
  );
}
