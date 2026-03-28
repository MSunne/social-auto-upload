"use client";

import { useMemo, useRef, useState, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { motion, AnimatePresence } from "framer-motion";
import {
  AlertTriangle,
  Download,
  Filter,
  History,
  Play,
  Search,
  Video,
  X,
  Eye,
  CalendarDays,
  Clock,
  Box,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { getModelDisplayName } from "@/lib/model-display";
import {
  buildAIJobTitle,
  formatDateTime,
  resolveAIJobStage,
} from "@/lib/workflow";
import { getAIJobArtifacts, listAIJobs } from "@/lib/services";
import type { AIJob, AIJobArtifact } from "@/lib/types";

// --- START SHARED VIDEO LOGIC ---
type VideoPreviewItem = {
  id: string;
  artifactKey: string;
  artifactType: string;
  fileName?: string | null;
  mimeType?: string | null;
  publicUrl?: string | null;
};

function isTerminalJob(job?: AIJob | null) {
  if (!job) return false;
  return ["success", "completed", "failed", "cancelled"].includes(job.status);
}

function isSuccessJob(job?: AIJob | null) {
  if (!job) return false;
  return ["success", "completed"].includes(job.status);
}

function pickVideoArtifacts(items: VideoPreviewItem[]) {
  return items.filter((item) => {
    if (!item.publicUrl) return false;
    if (item.artifactType === "video") return true;
    if ((item.mimeType || "").startsWith("video/")) return true;
    return /\.(mp4|mov|webm)$/i.test(item.fileName || "");
  });
}

function extractVideoArtifactsFromPayload(job?: AIJob | null): VideoPreviewItem[] {
  const artifacts = (
    (job?.outputPayload?.artifacts as Record<string, unknown>[] | undefined) ||
    []
  ).map((item, index) => ({
    id: String(item.id || `${job?.id || "job"}_payload_${index}`),
    artifactKey: String(item.artifactKey || `video_${index}`),
    artifactType: String(item.artifactType || ""),
    fileName: typeof item.fileName === "string" ? item.fileName : null,
    mimeType: typeof item.mimeType === "string" ? item.mimeType : null,
    publicUrl: typeof item.publicUrl === "string" ? item.publicUrl : null,
  }));

  const videoPayload = (job?.outputPayload?.video as Record<string, unknown> | undefined) || {};
  const contentUrl =
    (typeof videoPayload.contentUrl === "string" && videoPayload.contentUrl.trim()) ||
    (typeof job?.outputPayload?.contentUrl === "string" && job.outputPayload.contentUrl.trim()) ||
    "";

  if (contentUrl && !artifacts.some((item) => item.publicUrl === contentUrl)) {
    artifacts.unshift({
      id: `${job?.id || "job"}_remote_video`,
      artifactKey: "remote-video",
      artifactType: "video",
      fileName: "remote-video.mp4",
      mimeType: "video/mp4",
      publicUrl: contentUrl,
    });
  }

  return pickVideoArtifacts(artifacts);
}

function buildVideoPreviewSource(url?: string | null) {
  const trimmed = (url || "").trim();
  if (!trimmed) return "";
  return trimmed.includes("#") ? trimmed : `${trimmed}#t=0.1`;
}

function VideoPreviewSurface({
  src,
  className,
  videoClassName,
  controls = false,
  compact = false,
}: {
  src: string;
  className?: string;
  videoClassName?: string;
  controls?: boolean;
  compact?: boolean;
}) {
  const previewSrc = useMemo(() => buildVideoPreviewSource(src), [src]);
  const [status, setStatus] = useState<"idle" | "loading" | "ready" | "failed">(
    compact ? "idle" : "loading",
  );
  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const retriedRef = useRef(false);
  const activeSrcRef = useRef("");

  useEffect(() => {
    if (!compact || !previewSrc) return;
    const container = containerRef.current;
    if (!container) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          observer.disconnect();
          setStatus("loading");
          const el = videoRef.current;
          if (el) {
            activeSrcRef.current = previewSrc;
            el.src = previewSrc;
            el.load();
          }
        }
      },
      { rootMargin: "100px" },
    );
    observer.observe(container);
    return () => observer.disconnect();
  }, [compact, previewSrc]);

  useEffect(() => {
    if (compact) return;
    activeSrcRef.current = previewSrc;
  }, [compact, previewSrc]);

  const handleReady = () => setStatus("ready");

  const handleError = () => {
    if (status === "idle") return;
    if (!retriedRef.current && activeSrcRef.current) {
      retriedRef.current = true;
      const el = videoRef.current;
      if (el) {
        const sep = activeSrcRef.current.includes("?") ? "&" : "?";
        const retrySrc = `${activeSrcRef.current}${sep}_r=${Date.now()}`;
        activeSrcRef.current = retrySrc;
        el.src = retrySrc;
        el.load();
        return;
      }
    }
    setStatus("failed");
  };

  return (
    <div ref={containerRef} className={cn("relative overflow-hidden bg-black", className)}>
      <video
        ref={videoRef}
        key={compact ? undefined : previewSrc}
        src={compact ? undefined : previewSrc}
        controls={controls}
        autoPlay={false}
        muted={!controls}
        loop={!compact}
        playsInline
        preload={compact ? "metadata" : "auto"}
        onLoadedMetadata={compact ? handleReady : undefined}
        onLoadedData={handleReady}
        onCanPlay={handleReady}
        onError={handleError}
        className={cn(
          "h-full w-full transition-opacity duration-700",
          controls ? "object-contain" : "object-cover",
          status === "ready" ? "opacity-100" : "opacity-0",
          videoClassName,
        )}
      />

      <div
        className={cn(
          "pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-2 transition-opacity duration-700",
          status === "ready" ? "opacity-0" : "opacity-100",
          compact && status === "failed"
            ? "bg-gradient-to-br from-surface-elevated via-surface to-surface-elevated text-text-muted/50"
            : "cyber-grid bg-surface-elevated/95 text-text-muted",
        )}
      >
          {compact && status === "failed" ? (
            <Play className="h-5 w-5" />
          ) : (
            <>
              <Video className={cn(compact ? "h-5 w-5" : "h-8 w-8", status === "loading" ? "animate-pulse" : "")} />
              {status !== "idle" ? (
                <span className={cn("tracking-wide", compact ? "text-[10px]" : "text-xs")}>
                  {status === "failed" ? "加载失败" : "加载中"}
                </span>
              ) : null}
            </>
          )}
        </div>
    </div>
  );
}

// --- END SHARED VIDEO LOGIC ---

type FilterStatus = "all" | "processing" | "completed" | "failed";

export default function VideoHistoryPage() {
  const [filter, setFilter] = useState<FilterStatus>("all");
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);

  const { data: rawJobs = [], isLoading } = useQuery({
    queryKey: ["aiJobs", { jobType: "video", source: "omnidrive_cloud" }],
    queryFn: () => listAIJobs({ jobType: "video", source: "omnidrive_cloud", limit: 100 }),
    refetchInterval: (query) => {
      const active = query.state.data?.some((job) => !isTerminalJob(job));
      return active ? 3000 : false;
    },
  });

  const filteredJobs = useMemo(() => {
    return rawJobs.filter((job) => {
      const title = buildAIJobTitle(job).toLowerCase();
      const q = searchQuery.toLowerCase();
      if (q && !title.includes(q)) return false;

      const isTerminal = isTerminalJob(job);
      const isSuccess = isSuccessJob(job);

      if (filter === "processing" && isTerminal) return false;
      if (filter === "completed" && !isSuccess) return false;
      if (filter === "failed" && (!isTerminal || isSuccess)) return false;

      return true;
    });
  }, [rawJobs, filter, searchQuery]);

  const selectedJob = useMemo(() => {
    return rawJobs.find((j) => j.id === selectedJobId) || null;
  }, [rawJobs, selectedJobId]);

  const selectedPayloadPreview = getPrimaryPreviewFromJob(selectedJob);
  const { data: selectedArtifacts } = useQuery<AIJobArtifact[]>({
    queryKey: ["aiJobArtifacts", selectedJobId],
    queryFn: () => getAIJobArtifacts(selectedJobId!),
    enabled: !!selectedJobId && isTerminalJob(selectedJob) && isSuccessJob(selectedJob),
  });

  const selectedPreviewItems = useMemo(() => {
    if (selectedArtifacts && selectedArtifacts.length > 0) {
      return pickVideoArtifacts(selectedArtifacts.map((a) => ({
        id: a.id,
        artifactKey: a.artifactKey,
        artifactType: a.artifactType,
        fileName: a.fileName,
        mimeType: a.mimeType,
        publicUrl: a.publicUrl,
      })));
    }
    return selectedPayloadPreview ? [selectedPayloadPreview] : [];
  }, [selectedArtifacts, selectedPayloadPreview]);

  return (
    <div className="flex h-full flex-col p-6">
      <header className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight text-text-primary">
            <History className="h-6 w-6 text-accent" />
            视频历史
          </h1>
          <p className="mt-1 text-sm text-text-muted">管理您的视频生成任务和历史记录</p>
        </div>

        {/* Filters & Search */}
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted" />
            <input
              type="text"
              placeholder="搜索任务..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full rounded-xl border border-border bg-surface px-9 py-2 text-sm text-text-primary focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent sm:w-64"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery("")}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary"
              >
                <X className="h-3 w-3" />
              </button>
            )}
          </div>

          <div className="flex items-center gap-1 rounded-xl border border-border/50 p-1">
            {[
              { id: "all", label: "全部" },
              { id: "processing", label: "进行中" },
              { id: "completed", label: "已完成" },
              { id: "failed", label: "失败" },
            ].map((f) => (
              <button
                key={f.id}
                onClick={() => setFilter(f.id as FilterStatus)}
                className={cn(
                  "rounded-lg px-3 py-1.5 text-xs font-medium transition-all",
                  filter === f.id
                    ? "bg-accent/15 text-accent-strong shadow-sm"
                    : "text-text-secondary hover:bg-surface-hover hover:text-text-primary",
                )}
              >
                {f.label}
              </button>
            ))}
          </div>
        </div>
      </header>

      {/* List content */}
      <div className="custom-scrollbar flex-1 overflow-y-auto rounded-xl border border-border/50 bg-surface/30">
        {isLoading ? (
          <div className="flex h-40 items-center justify-center text-text-muted">加载中...</div>
        ) : filteredJobs.length === 0 ? (
          <div className="flex h-40 flex-col items-center justify-center text-text-muted">
            <Video className="mb-2 h-8 w-8 opacity-20" />
            <p>暂无符合条件的视频记录</p>
          </div>
        ) : (
          <div className="min-w-[800px]">
            <div className="grid grid-cols-[3fr_1.5fr_1.5fr_1.5fr_1fr] gap-4 border-b border-border/50 bg-surface-hover/50 px-6 py-3 text-xs font-semibold uppercase tracking-wider text-text-muted">
              <div>任务名称 / 提示词</div>
              <div>模型</div>
              <div>创建时间</div>
              <div>状态</div>
              <div className="text-right">操作</div>
            </div>
            <div className="flex flex-col">
              {filteredJobs.map((job) => {
                const stage = resolveAIJobStage(job);
                return (
                  <div
                    key={job.id}
                    className="grid grid-cols-[3fr_1.5fr_1.5fr_1.5fr_1fr] items-center gap-4 border-b border-border/10 px-6 py-4 transition-colors hover:bg-surface-hover/30"
                  >
                    <div className="flex items-center gap-3 overflow-hidden">
                      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-surface-elevated border border-border/50">
                        <Video className="h-5 w-5 text-accent/70" />
                      </div>
                      <div className="truncate">
                        <h4 className="truncate text-sm font-medium text-text-primary" title={buildAIJobTitle(job)}>
                          {buildAIJobTitle(job)}
                        </h4>
                        <p className="truncate text-xs text-text-muted">ID: {job.id}</p>
                      </div>
                    </div>
                    
                    <div className="flex items-center gap-2">
                      <Box className="h-3.5 w-3.5 text-text-muted" />
                      <span className="text-sm text-text-secondary">{getModelDisplayName(job)}</span>
                    </div>

                    <div className="flex items-center gap-2 text-sm text-text-secondary">
                      <CalendarDays className="h-3.5 w-3.5 text-text-muted" />
                      {formatDateTime(job.createdAt)}
                    </div>

                    <div>
                      <span className={cn(
                        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium border",
                        isSuccessJob(job) ? "border-success/20 bg-success/10 text-success" : 
                        isTerminalJob(job) ? "border-danger/20 bg-danger/10 text-danger" : "border-accent/20 bg-accent/10 text-accent"
                      )}>
                        {!isTerminalJob(job) && <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-accent" />}
                        {stage.label}
                      </span>
                    </div>

                    <div className="flex justify-end gap-2">
                      <button
                        onClick={() => setSelectedJobId(job.id)}
                        className="flex items-center gap-1.5 rounded-lg bg-surface-elevated px-3 py-1.5 text-sm font-medium text-text-primary transition-colors hover:bg-accent/20 hover:text-accent-strong"
                      >
                        <Eye className="h-4 w-4" />
                        查看
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>

      {/* Detail Modal */}
      <AnimatePresence>
        {selectedJob && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6">
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => setSelectedJobId(null)}
              className="absolute inset-0 bg-black/90"
            />
            <motion.div
              initial={{ opacity: 0, scale: 0.95, y: 20 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 20 }}
              className="flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-2xl border border-white/10 shadow-2xl"
              style={{ backgroundColor: "#121223" }}
            >
              <header className="flex shrink-0 items-center justify-between border-b border-white/10 px-6 py-4" style={{ transform: "translateZ(0)" }}>
                <h3 className="text-lg font-semibold flex items-center gap-2 drop-shadow-sm" style={{ color: "#f0f0ff" }}>
                  <Video className="h-5 w-5 text-accent" />
                  任务详情
                </h3>
                <button
                  onClick={() => setSelectedJobId(null)}
                  className="rounded-lg p-1.5 text-text-muted hover:bg-surface-hover hover:text-text-primary"
                >
                  <X className="h-5 w-5" />
                </button>
              </header>

              <div className="flex-1 overflow-y-auto p-4 sm:p-6 custom-scrollbar">
                <div className="flex flex-col gap-6 lg:flex-row">
                  {/* Left: Preview */}
                  <div className="flex flex-1 flex-col gap-4">
                    <div className="relative aspect-video w-full overflow-hidden rounded-xl border border-border/50 bg-black shadow-lg">
                      {selectedPreviewItems.length > 0 ? (
                        <>
                          <VideoPreviewSurface
                            src={selectedPreviewItems[0].publicUrl!}
                            controls
                            className="h-full w-full"
                          />
                          {/* Floating Status Indicator */}
                          <div className="pointer-events-none absolute left-4 top-4 flex items-center gap-2 rounded-lg border border-white/10 bg-black/90 px-3 py-1.5">
                            <span
                              className={cn(
                                "flex h-2 w-2 rounded-full",
                                isSuccessJob(selectedJob)
                                  ? "bg-success pulse-online"
                                  : isTerminalJob(selectedJob)
                                  ? "bg-danger"
                                  : "bg-warning",
                              )}
                            />
                            <span className="text-xs font-semibold text-white drop-shadow-md">
                              {resolveAIJobStage(selectedJob).label}
                            </span>
                          </div>
                        </>
                      ) : isTerminalJob(selectedJob) && !isSuccessJob(selectedJob) ? (
                        <div className="flex h-full w-full flex-col items-center justify-center gap-3 text-danger">
                          <AlertTriangle className="h-10 w-10" />
                          <span className="text-sm font-medium" style={{ color: "#ff3b5c" }}>生成失败，暂无视频</span>
                        </div>
                      ) : (
                        <div className="flex h-full w-full flex-col items-center justify-center gap-3" style={{ color: "#7a7a9e" }}>
                          <Video className="h-10 w-10 opacity-20" />
                          <span className="text-sm">尚未就绪，等待后端返回视频</span>
                        </div>
                      )}
                    </div>
                    
                    {/* Action Buttons */}
                    <div className="flex items-center gap-3">
                      {selectedPreviewItems.length > 0 && selectedPreviewItems[0].publicUrl && (
                        <a
                          href={selectedPreviewItems[0].publicUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex flex-1 items-center justify-center gap-2 rounded-xl px-4 py-3 text-sm font-bold text-white transition-all hover:opacity-80 drop-shadow-md"
                          style={{ backgroundColor: "#b149ff", boxShadow: "0 0 20px rgba(177,73,255,0.4)", transform: "translateZ(0)" }}
                        >
                          <Download className="h-4 w-4" />
                          下载源视频
                        </a>
                      )}
                    </div>
                  </div>

                  {/* Right: Info */}
                    <div className="flex w-full flex-col gap-4 lg:w-[320px] shrink-0" style={{ transform: "translateZ(0)" }}>
                      <div className="flex flex-col gap-2 rounded-xl border p-5 shadow-xl" style={{ backgroundColor: "rgba(30, 30, 50, 0.95)", borderColor: "rgba(255, 255, 255, 0.3)", transform: "translateZ(0)" }}>
                        <h4 className="flex items-center gap-2 mb-1 text-sm font-extrabold uppercase drop-shadow-sm" style={{ color: "rgba(255, 255, 255, 0.95)" }}>
                          <Box className="h-4 w-4" style={{ color: "#d084ff" }} /> 提示词
                        </h4>
                        <p className="text-base font-semibold leading-relaxed break-all drop-shadow-sm" style={{ color: "#ffffff" }}>
                          {buildAIJobTitle(selectedJob) || "生成视频的提示词为空"}
                        </p>
                      </div>

                    <div className="grid grid-cols-2 gap-4 lg:grid-cols-1">
                      <div className="flex flex-col gap-1.5 rounded-xl border p-4 shadow-lg" style={{ backgroundColor: "rgba(30, 30, 50, 0.95)", borderColor: "rgba(255, 255, 255, 0.3)", transform: "translateZ(0)" }}>
                        <span className="text-xs font-extrabold uppercase drop-shadow-sm" style={{ color: "#00f5d4" }}>模型引擎</span>
                        <span className="text-base font-bold drop-shadow-sm" style={{ color: "#ffffff" }}>{getModelDisplayName(selectedJob)}</span>
                      </div>
                      <div className="flex flex-col gap-1.5 rounded-xl border p-4 shadow-lg" style={{ backgroundColor: "rgba(30, 30, 50, 0.95)", borderColor: "rgba(255, 255, 255, 0.3)", transform: "translateZ(0)" }}>
                        <span className="text-xs font-extrabold uppercase drop-shadow-sm" style={{ color: "rgba(255, 255, 255, 0.9)" }}>创建时间</span>
                        <span className="text-sm font-bold flex items-center drop-shadow-sm" style={{ color: "#ffffff" }}>{formatDateTime(selectedJob.createdAt)}</span>
                      </div>
                      <div className="flex flex-col gap-1.5 rounded-xl border p-4 shadow-lg" style={{ backgroundColor: "rgba(30, 30, 50, 0.95)", borderColor: "rgba(255, 255, 255, 0.3)", transform: "translateZ(0)" }}>
                        <span className="text-xs font-extrabold uppercase drop-shadow-sm" style={{ color: "rgba(255, 255, 255, 0.9)" }}>更新时间</span>
                        <span className="text-sm font-bold flex items-center drop-shadow-sm" style={{ color: "#ffffff" }}>{formatDateTime(selectedJob.updatedAt)}</span>
                      </div>
                    </div>

                    {selectedJob.message && (
                      <div className="mt-2 rounded-xl border p-4 shadow-md" style={{ backgroundColor: "rgba(255, 59, 92, 0.15)", borderColor: "rgba(255, 59, 92, 0.4)", transform: "translateZ(0)" }}>
                        <h4 className="mb-1.5 text-xs font-extrabold uppercase flex items-center gap-1.5 drop-shadow-sm" style={{ color: "#ff3b5c" }}>
                          <AlertTriangle className="h-3.5 w-3.5" /> 系统消息
                        </h4>
                        <p className="text-sm font-semibold leading-relaxed drop-shadow-sm" style={{ color: "#ffa5b5" }}>{selectedJob.message}</p>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </div>
  );
}

function getPrimaryPreviewFromJob(job?: AIJob | null) {
  return extractVideoArtifactsFromPayload(job)[0] || null;
}
