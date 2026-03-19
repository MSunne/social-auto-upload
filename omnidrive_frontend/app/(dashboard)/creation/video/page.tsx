"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { motion, AnimatePresence } from "framer-motion";
import {
  AlertTriangle,
  Check,
  ChevronDown,
  Download,
  Info,
  Layers,
  Play,
  Upload,
  Video,
  Wand2,
  X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { buildAIJobTitle, formatDateTime, resolveAIJobStage } from "@/lib/workflow";
import {
  createAIJob,
  getAIJob,
  getAIJobArtifacts,
  listAIJobs,
  listAIModels,
} from "@/lib/services";
import type { AIJob, AIJobArtifact, AIModel, CreateAIJobRequest } from "@/lib/types";

type ReferenceFrame = {
  id: string;
  previewUrl: string;
  dataUrl: string;
  fileName: string;
  mimeType: string;
};

type VideoDurationOption = {
  label: string;
  seconds: number;
};

type VideoResolutionOption = {
  label: string;
  resolution: string;
  width: number;
  height: number;
  aspectRatio: string;
};

type VideoPreviewItem = {
  id: string;
  artifactKey: string;
  artifactType: string;
  fileName?: string | null;
  mimeType?: string | null;
  publicUrl?: string | null;
};

type VideoPreviewSurfaceProps = {
  src: string;
  className?: string;
  videoClassName?: string;
  controls?: boolean;
  compact?: boolean;
};

type VideoPreviewSurfaceInnerProps = {
  previewSrc: string;
  className?: string;
  videoClassName?: string;
  controls?: boolean;
  compact?: boolean;
};

const DEFAULT_DURATION_OPTIONS: VideoDurationOption[] = [
  { label: "8s", seconds: 8 },
  { label: "12s", seconds: 12 },
];

const DEFAULT_RESOLUTION_OPTIONS: VideoResolutionOption[] = [
  { label: "16:9", resolution: "1280x720", width: 1280, height: 720, aspectRatio: "16:9" },
  { label: "9:16", resolution: "720x1280", width: 720, height: 1280, aspectRatio: "9:16" },
];

function greatestCommonDivisor(a: number, b: number): number {
  let left = Math.abs(a);
  let right = Math.abs(b);
  while (right !== 0) {
    const next = left % right;
    left = right;
    right = next;
  }
  return left || 1;
}

function toAspectRatio(width: number, height: number): string {
  const divisor = greatestCommonDivisor(width, height);
  return `${Math.round(width / divisor)}:${Math.round(height / divisor)}`;
}

function parseDurationOption(value: string): VideoDurationOption | null {
  const match = value.trim().toLowerCase().match(/^(\d+)\s*s?$/);
  if (!match) {
    return null;
  }
  const seconds = Number(match[1]);
  if (!seconds) {
    return null;
  }
  return {
    label: `${seconds}s`,
    seconds,
  };
}

function parseResolutionOption(value: string): VideoResolutionOption | null {
  const normalized = value.trim().toLowerCase().replaceAll("×", "x").replace("*", "x");
  const match = normalized.match(/^(\d+)\s*x\s*(\d+)$/);
  if (!match) {
    return null;
  }
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (!width || !height) {
    return null;
  }
  const aspectRatio = toAspectRatio(width, height);
  return {
    label: aspectRatio,
    resolution: `${width}x${height}`,
    width,
    height,
    aspectRatio,
  };
}

function buildVideoDurationOptions(model?: AIModel | null) {
  const supported = (model?.videoSupportedDurations || [])
    .map((item) => parseDurationOption(item))
    .filter((item): item is VideoDurationOption => Boolean(item));
  if (supported.length > 0) {
    return supported;
  }
  return DEFAULT_DURATION_OPTIONS;
}

function buildVideoResolutionOptions(model?: AIModel | null) {
  const supported = (model?.videoSupportedResolutions || [])
    .map((item) => parseResolutionOption(item))
    .filter((item): item is VideoResolutionOption => Boolean(item));
  if (supported.length > 0) {
    return supported;
  }
  return DEFAULT_RESOLUTION_OPTIONS;
}

function formatResolutionLabel(option: VideoResolutionOption) {
  return `${option.label} (${option.resolution})`;
}

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string") {
        resolve(reader.result);
        return;
      }
      reject(new Error("文件读取失败"));
    };
    reader.onerror = () => reject(new Error("文件读取失败"));
    reader.readAsDataURL(file);
  });
}

function isTerminalJob(job?: AIJob | null) {
  if (!job) {
    return false;
  }
  return ["success", "completed", "failed", "cancelled"].includes(job.status);
}

function isSuccessJob(job?: AIJob | null) {
  if (!job) {
    return false;
  }
  return ["success", "completed"].includes(job.status);
}

function toPreviewItem(artifact: AIJobArtifact): VideoPreviewItem {
  return {
    id: artifact.id,
    artifactKey: artifact.artifactKey,
    artifactType: artifact.artifactType,
    fileName: artifact.fileName,
    mimeType: artifact.mimeType,
    publicUrl: artifact.publicUrl,
  };
}

function pickVideoArtifacts(items: VideoPreviewItem[]) {
  return items.filter((item) => {
    if (!item.publicUrl) {
      return false;
    }
    if (item.artifactType === "video") {
      return true;
    }
    if ((item.mimeType || "").startsWith("video/")) {
      return true;
    }
    return /\.(mp4|mov|webm)$/i.test(item.fileName || "");
  });
}

function extractVideoArtifactsFromPayload(job?: AIJob | null): VideoPreviewItem[] {
  const artifacts = ((job?.outputPayload?.artifacts as Record<string, unknown>[] | undefined) || []).map((item, index) => ({
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
  if (!trimmed) {
    return "";
  }
  return trimmed.includes("#") ? trimmed : `${trimmed}#t=0.1`;
}

function VideoPreviewSurfaceInner({
  previewSrc,
  className,
  videoClassName,
  controls = false,
  compact = false,
}: VideoPreviewSurfaceInnerProps) {
  const [status, setStatus] = useState<"loading" | "ready" | "failed">("loading");

  return (
    <div className={cn("relative overflow-hidden bg-black", className)}>
      <video
        key={previewSrc}
        src={previewSrc}
        controls={controls}
        autoPlay
        muted
        loop
        playsInline
        preload={compact ? "metadata" : "auto"}
        onLoadedData={() => setStatus("ready")}
        onCanPlay={() => setStatus("ready")}
        onError={() => setStatus("failed")}
        className={cn(
          "h-full w-full transition-opacity duration-300",
          controls ? "object-contain" : "object-cover",
          status === "ready" ? "opacity-100" : "opacity-0",
          videoClassName,
        )}
      />

      {status !== "ready" ? (
        <div className="cyber-grid absolute inset-0 flex flex-col items-center justify-center gap-2 bg-surface-elevated/95 text-text-muted">
          <Video className={cn(compact ? "h-5 w-5" : "h-8 w-8", status === "loading" ? "animate-pulse" : "")} />
          <span className={cn("tracking-wide", compact ? "text-[10px]" : "text-xs")}>
            {status === "failed" ? "预览加载失败" : "正在加载预览"}
          </span>
        </div>
      ) : null}
    </div>
  );
}

function VideoPreviewSurface({
  src,
  className,
  videoClassName,
  controls = false,
  compact = false,
}: VideoPreviewSurfaceProps) {
  const previewSrc = useMemo(() => buildVideoPreviewSource(src), [src]);

  return (
    <VideoPreviewSurfaceInner
      key={previewSrc}
      previewSrc={previewSrc}
      className={className}
      videoClassName={videoClassName}
      controls={controls}
      compact={compact}
    />
  );
}

function getPrimaryVideoPreviewItem(job?: AIJob | null) {
  return extractVideoArtifactsFromPayload(job)[0] || null;
}

function sortJobsByUpdatedAt(items: AIJob[]) {
  return [...items].sort((left, right) => {
    return new Date(right.updatedAt || 0).getTime() - new Date(left.updatedAt || 0).getTime();
  });
}

function buildVideoProgress(job?: AIJob | null) {
  if (!job) {
    return {
      value: 0,
      label: "等待开始",
      tone: "idle" as const,
      hint: "提交视频制作任务后，这里会显示云端进度。",
    };
  }

  const stage = resolveAIJobStage(job);
  if (stage.key === "scheduled") {
    return {
      value: 10,
      label: stage.label,
      tone: "progress" as const,
      hint: stage.description || "任务已排队，等待开始执行。",
    };
  }
  if (stage.key === "queued_generation") {
    return {
      value: 18,
      label: stage.label,
      tone: "progress" as const,
      hint: stage.description || "任务已创建，等待到易侧接收。",
    };
  }
  if (stage.key === "storyboarding") {
    return {
      value: 40,
      label: stage.label,
      tone: "progress" as const,
      hint: stage.description || "正在优化脚本和镜头描述。",
    };
  }
  if (stage.key === "generating") {
    return {
      value: 76,
      label: stage.label,
      tone: "progress" as const,
      hint: stage.description || "到易正在合成视频镜头，请稍候。",
    };
  }
  if (stage.key === "output_ready" || stage.key === "imported" || isSuccessJob(job)) {
    return {
      value: 100,
      label: stage.label,
      tone: "success" as const,
      hint: stage.description || "视频已经生成完成，可以直接预览。",
    };
  }
  if (stage.key === "publish_failed" || job.status === "failed") {
    return {
      value: 100,
      label: "生成失败",
      tone: "danger" as const,
      hint: stage.description || job.message || "视频任务执行失败。",
    };
  }
  if (stage.key === "cancelled") {
    return {
      value: 100,
      label: "已取消",
      tone: "danger" as const,
      hint: stage.description || "任务已取消。",
    };
  }

  return {
    value: 35,
    label: stage.label,
    tone: "progress" as const,
    hint: stage.description || job.message || "正在处理任务。",
  };
}

function extractDurationLabel(job?: AIJob | null) {
  const payload = (job?.inputPayload || {}) as Record<string, unknown>;
  const rawDuration = payload.durationSeconds ?? payload.duration;
  const seconds = Number(rawDuration || 0);
  return Number.isFinite(seconds) && seconds > 0 ? `${seconds}s` : "";
}

export default function VideoCreationPage() {
  const [prompt, setPrompt] = useState("");
  const [selectedModel, setSelectedModel] = useState("");
  const [selectedDuration, setSelectedDuration] = useState("");
  const [selectedResolution, setSelectedResolution] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState("");
  const [referenceFrames, setReferenceFrames] = useState<ReferenceFrame[]>([]);
  const [isModelDropdownOpen, setIsModelDropdownOpen] = useState(false);
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  const [currentJobId, setCurrentJobId] = useState<string | null>(null);
  const [previewIndex, setPreviewIndex] = useState(0);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const previewSectionRef = useRef<HTMLDivElement>(null);
  const autoPreviewedJobIdRef = useRef<string | null>(null);
  const latestReferenceFramesRef = useRef<ReferenceFrame[]>([]);

  const { data: allModels = [], isLoading: modelsLoading } = useQuery<AIModel[]>({
    queryKey: ["aiModels", "video"],
    queryFn: () => listAIModels({ category: "video" }),
  });

  const videoModels = useMemo(() => {
    const filtered = allModels.filter((item) => item.category === "video" && item.isEnabled);
    return filtered.length > 0 ? filtered : allModels.filter((item) => item.category === "video");
  }, [allModels]);

  const activeModel = useMemo(() => {
    return videoModels.find((item) => item.modelName === selectedModel) || videoModels[0] || null;
  }, [selectedModel, videoModels]);

  const durationOptions = useMemo(() => buildVideoDurationOptions(activeModel), [activeModel]);
  const resolutionOptions = useMemo(() => buildVideoResolutionOptions(activeModel), [activeModel]);

  const selectedDurationOption = useMemo(() => {
    return durationOptions.find((item) => item.label === selectedDuration) || durationOptions[0] || null;
  }, [durationOptions, selectedDuration]);
  const selectedResolutionOption = useMemo(() => {
    return resolutionOptions.find((item) => item.resolution === selectedResolution) || resolutionOptions[0] || null;
  }, [resolutionOptions, selectedResolution]);

  const maxReferenceFrames = useMemo(() => {
    const limit = Number(activeModel?.videoReferenceLimit || 1) || 1;
    return Math.max(1, Math.min(4, limit));
  }, [activeModel]);

  const estimatedCredits = useMemo(() => {
    const amount = Number(activeModel?.billingAmount ?? activeModel?.rawRate ?? 0);
    if (!amount) {
      return null;
    }
    if (activeModel?.billingMode === "per_second" && selectedDurationOption) {
      return Math.round(amount * selectedDurationOption.seconds);
    }
    return Math.round(amount);
  }, [activeModel, selectedDurationOption]);

  const {
    data: videoJobs = [],
    refetch: refetchVideoJobs,
  } = useQuery<AIJob[]>({
    queryKey: ["aiJobs", "video"],
    queryFn: () => listAIJobs({ jobType: "video", limit: 20 }),
    refetchInterval: currentJobId ? 4000 : false,
  });

  const { data: currentJob } = useQuery<AIJob>({
    queryKey: ["aiJob", "video", currentJobId],
    queryFn: () => getAIJob(currentJobId as string),
    enabled: Boolean(currentJobId),
    refetchInterval: (query) => {
      const job = query.state.data as AIJob | undefined;
      return currentJobId && !isTerminalJob(job) ? 3000 : false;
    },
  });

  const mergedJobs = useMemo(() => {
    if (!currentJob) {
      return sortJobsByUpdatedAt(videoJobs);
    }
    return sortJobsByUpdatedAt([currentJob, ...videoJobs.filter((item) => item.id !== currentJob.id)]);
  }, [currentJob, videoJobs]);

  const selectedJob = useMemo(() => {
    if (selectedJobId && currentJob && selectedJobId === currentJob.id) {
      return currentJob;
    }
    if (selectedJobId) {
      return mergedJobs.find((item) => item.id === selectedJobId) || null;
    }
    return mergedJobs[0] || null;
  }, [currentJob, mergedJobs, selectedJobId]);

  const { data: selectedJobArtifacts = [] } = useQuery<AIJobArtifact[]>({
    queryKey: ["aiJobArtifacts", "video", selectedJob?.id],
    queryFn: () => getAIJobArtifacts(selectedJob?.id as string),
    enabled: Boolean(selectedJob?.id),
    refetchInterval:
      selectedJob?.id && currentJobId && selectedJob.id === currentJobId && !isTerminalJob(currentJob)
        ? 3000
        : false,
  });

  const selectedPreviewItems = useMemo(() => {
    const serverArtifacts = pickVideoArtifacts(selectedJobArtifacts.map((item) => toPreviewItem(item)));
    if (serverArtifacts.length > 0) {
      return serverArtifacts;
    }
    return extractVideoArtifactsFromPayload(selectedJob);
  }, [selectedJob, selectedJobArtifacts]);

  const selectedPreviewItem = selectedPreviewItems[previewIndex] || selectedPreviewItems[0] || null;
  const progress = buildVideoProgress(currentJob || selectedJob);
  const generating = submitting || Boolean(currentJob && !isTerminalJob(currentJob));

  useEffect(() => {
    if (!selectedModel && videoModels.length > 0) {
      setSelectedModel(videoModels[0].modelName);
      return;
    }
    if (selectedModel && !videoModels.some((item) => item.modelName === selectedModel) && videoModels.length > 0) {
      setSelectedModel(videoModels[0].modelName);
    }
  }, [selectedModel, videoModels]);

  useEffect(() => {
    if (!selectedDuration && durationOptions.length > 0) {
      setSelectedDuration(durationOptions[0].label);
      return;
    }
    if (selectedDuration && !durationOptions.some((item) => item.label === selectedDuration) && durationOptions.length > 0) {
      setSelectedDuration(durationOptions[0].label);
    }
  }, [durationOptions, selectedDuration]);

  useEffect(() => {
    if (!selectedResolution && resolutionOptions.length > 0) {
      setSelectedResolution(resolutionOptions[0].resolution);
      return;
    }
    if (
      selectedResolution &&
      !resolutionOptions.some((item) => item.resolution === selectedResolution) &&
      resolutionOptions.length > 0
    ) {
      setSelectedResolution(resolutionOptions[0].resolution);
    }
  }, [resolutionOptions, selectedResolution]);

  useEffect(() => {
    if (!selectedJobId && mergedJobs.length > 0) {
      setSelectedJobId(mergedJobs[0].id);
    }
  }, [mergedJobs, selectedJobId]);

  useEffect(() => {
    if (!selectedPreviewItems.length) {
      setPreviewIndex(0);
      return;
    }
    if (previewIndex >= selectedPreviewItems.length) {
      setPreviewIndex(0);
    }
  }, [previewIndex, selectedPreviewItems.length]);

  useEffect(() => {
    latestReferenceFramesRef.current = referenceFrames;
  }, [referenceFrames]);

  useEffect(() => {
    return () => {
      latestReferenceFramesRef.current.forEach((item) => URL.revokeObjectURL(item.previewUrl));
    };
  }, []);

  useEffect(() => {
    if (!currentJob || !isSuccessJob(currentJob)) {
      return;
    }
    if (autoPreviewedJobIdRef.current === currentJob.id) {
      return;
    }
    autoPreviewedJobIdRef.current = currentJob.id;
    setSelectedJobId(currentJob.id);
    setPreviewIndex(0);
    void refetchVideoJobs();
    previewSectionRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  }, [currentJob, refetchVideoJobs]);

  async function handleGenerate() {
    if (!prompt.trim() || !activeModel || !selectedDurationOption || !selectedResolutionOption) {
      return;
    }

    setSubmitError("");
    setSubmitting(true);

    try {
      const payload: CreateAIJobRequest = {
        jobType: "video",
        modelName: activeModel.modelName,
        prompt: prompt.trim(),
        source: "omnidrive_cloud",
        inputPayload: {
          prompt: prompt.trim(),
          aspectRatio: selectedResolutionOption.aspectRatio,
          resolution: selectedResolutionOption.resolution,
          durationSeconds: selectedDurationOption.seconds,
          referenceImages: referenceFrames.map((item) => ({
            data: item.dataUrl.split(",")[1] || "",
            fileName: item.fileName,
            mimeType: item.mimeType,
          })),
        },
      };
      const job = await createAIJob(payload);
      setCurrentJobId(job.id);
      setSelectedJobId(job.id);
      setPreviewIndex(0);
      await refetchVideoJobs();
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : "视频生成请求失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleFileSelect(event: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files || []);
    if (files.length === 0) {
      return;
    }

    const remaining = Math.max(0, maxReferenceFrames - referenceFrames.length);
    const selectedFiles = files.slice(0, remaining);
    if (selectedFiles.length === 0) {
      event.target.value = "";
      return;
    }

    const nextFrames = await Promise.all(
      selectedFiles.map(async (file) => ({
        id: `${file.name}_${file.size}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
        previewUrl: URL.createObjectURL(file),
        dataUrl: await readFileAsDataUrl(file),
        fileName: file.name,
        mimeType: file.type || "image/png",
      })),
    );

    setReferenceFrames((previous) => [...previous, ...nextFrames].slice(0, maxReferenceFrames));
    event.target.value = "";
  }

  function removeReferenceFrame(id: string) {
    setReferenceFrames((previous) => {
      const target = previous.find((item) => item.id === id);
      if (target) {
        URL.revokeObjectURL(target.previewUrl);
      }
      return previous.filter((item) => item.id !== id);
    });
  }

  return (
    <div className="grid min-h-[calc(100vh-theme(spacing.16))] grid-cols-1 gap-5 pb-4 lg:h-[calc(100vh-theme(spacing.16))] lg:grid-cols-12">
      <div className="flex h-full flex-col gap-3 overflow-y-auto pr-1 pb-4 lg:col-span-4 xl:col-span-3">
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="glass-card p-4"
        >
          <div className="mb-2 flex items-center justify-between">
            <label className="text-xs font-semibold uppercase tracking-wider text-text-muted">
              参考帧 (最多 {maxReferenceFrames} 张)
            </label>
            <span className="text-[10px] text-text-muted">
              {referenceFrames.length}/{maxReferenceFrames}
            </span>
          </div>

          <div className="grid grid-cols-4 gap-2">
            <AnimatePresence>
              {referenceFrames.map((image) => (
                <motion.div
                  key={image.id}
                  initial={{ opacity: 0, scale: 0.8 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.8 }}
                  className="group relative aspect-square overflow-hidden rounded-lg border border-border bg-surface-hover"
                >
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src={image.previewUrl} alt={image.fileName} className="h-full w-full object-cover" />
                  <button
                    type="button"
                    onClick={() => removeReferenceFrame(image.id)}
                    className="absolute -right-2 -top-2 flex h-6 w-6 items-center justify-center rounded-full bg-danger text-white opacity-0 shadow-md transition-opacity group-hover:opacity-100"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </motion.div>
              ))}
            </AnimatePresence>

            {referenceFrames.length < maxReferenceFrames && (
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                className="flex aspect-square flex-col items-center justify-center gap-1 rounded-lg border-2 border-dashed border-border bg-surface-hover/50 text-text-muted transition-colors hover:border-accent/50 hover:bg-accent/5 hover:text-accent"
              >
                <Upload className="h-4 w-4" />
                <span className="text-[10px]">上传</span>
              </button>
            )}
            <input
              ref={fileInputRef}
              type="file"
              multiple
              accept="image/*"
              className="hidden"
              onChange={handleFileSelect}
            />
          </div>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.05 }}
          className="glass-card p-4"
        >
          <label className="mb-2 block text-xs font-semibold uppercase tracking-wider text-text-muted">
            视频描述指令
          </label>
          <textarea
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
            placeholder="描述镜头运动、主体动作、场景变化与动态细节..."
            rows={5}
            className="w-full resize-none rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary outline-none transition-all placeholder:text-text-muted focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
          />
          <div className="mt-2 flex items-center justify-between">
            <span className="text-xs text-text-muted">{prompt.length} / 4000</span>
            <span className="text-[11px] text-text-muted">
              {activeModel?.vendor ? `提供方：${activeModel.vendor}` : "视频任务将直接发送到真实后端"}
            </span>
          </div>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="glass-card p-4"
        >
          <label className="mb-3 block text-xs font-semibold uppercase tracking-wider text-text-muted">
            视频引擎
          </label>
          <div className="relative">
            <button
              type="button"
              onClick={() => setIsModelDropdownOpen((open) => !open)}
              className="flex w-full items-center justify-between rounded-xl border border-border bg-surface px-4 py-3 text-sm font-medium transition-all hover:border-accent/50 focus:border-accent"
            >
              <span className="text-text-primary">
                {activeModel?.modelName || (modelsLoading ? "加载中..." : "暂无可用模型")}
              </span>
              <ChevronDown
                className={cn("h-4 w-4 text-text-muted transition-transform", isModelDropdownOpen && "rotate-180")}
              />
            </button>

            <AnimatePresence>
              {isModelDropdownOpen && videoModels.length > 0 && (
                <motion.div
                  initial={{ opacity: 0, y: -5 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -5 }}
                  className="absolute left-0 right-0 top-full z-50 mt-2 overflow-hidden rounded-xl border border-border-strong bg-surface-elevated shadow-xl backdrop-blur-3xl"
                >
                  {videoModels.map((model) => (
                    <button
                      key={model.id}
                      type="button"
                      onClick={() => {
                        setSelectedModel(model.modelName);
                        setIsModelDropdownOpen(false);
                      }}
                      className="flex w-full items-center justify-between border-b border-border/50 px-4 py-3 text-left transition-colors last:border-0 hover:bg-surface-hover"
                    >
                      <span
                        className={cn(
                          "text-sm",
                          selectedModel === model.modelName ? "font-bold text-accent" : "text-text-primary",
                        )}
                      >
                        {model.modelName}
                      </span>
                      {selectedModel === model.modelName && <Check className="h-4 w-4 text-accent" />}
                    </button>
                  ))}
                </motion.div>
              )}
            </AnimatePresence>
          </div>

          <div className="mt-3 flex items-start gap-2 rounded-lg border border-border/50 bg-surface-hover p-3 text-xs text-text-muted">
            <Info className="mt-0.5 h-3.5 w-3.5 shrink-0 text-cyan" />
            <p className="leading-snug">
              {activeModel?.description || "模型说明将从后端接口实时读取。"}
            </p>
          </div>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.15 }}
          className="glass-card space-y-4 p-4"
        >
          <div>
            <label className="mb-2 block text-xs font-semibold uppercase tracking-wider text-text-muted">
              视频时长
            </label>
            <div className="grid grid-cols-3 gap-2">
              {durationOptions.map((option) => (
                <button
                  key={option.label}
                  type="button"
                  onClick={() => setSelectedDuration(option.label)}
                  className={cn(
                    "rounded-lg border px-2 py-2 text-xs font-medium transition-all",
                    selectedDurationOption?.label === option.label
                      ? "border-accent/50 bg-accent/10 text-accent"
                      : "border-border text-text-muted hover:border-accent/30 hover:text-text-primary",
                  )}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="mb-2 block text-xs font-semibold uppercase tracking-wider text-text-muted">
              输出规格
            </label>
            <div className="grid grid-cols-1 gap-2">
              {resolutionOptions.map((option) => (
                <button
                  key={option.resolution}
                  type="button"
                  onClick={() => setSelectedResolution(option.resolution)}
                  className={cn(
                    "rounded-lg border px-3 py-2 text-left transition-all",
                    selectedResolutionOption?.resolution === option.resolution
                      ? "border-accent/50 bg-accent/10"
                      : "border-border bg-surface hover:border-accent/30 hover:bg-surface-hover",
                  )}
                >
                  <div className="text-sm font-medium text-text-primary">{formatResolutionLabel(option)}</div>
                  <div className="text-[11px] text-text-muted">{option.resolution}</div>
                </button>
              ))}
            </div>
          </div>

          <div className="rounded-xl border border-border/50 bg-surface-hover px-3 py-3 text-xs text-text-muted">
            <div className="flex items-center justify-between gap-2">
              <span>计费方式</span>
              <span className="font-medium text-text-primary">
                {activeModel?.billingMode === "per_second" ? "按秒计费" : activeModel?.billingMode === "per_call" ? "按次计费" : "实时计算"}
              </span>
            </div>
            <div className="mt-2 flex items-center justify-between gap-2">
              <span>预计积分</span>
              <span className="font-medium text-accent">{estimatedCredits ? `${estimatedCredits}` : "提交后由后端计算"}</span>
            </div>
          </div>
        </motion.div>

        {(currentJob || submitError) && (
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2 }}
            className={cn("glass-card p-4", submitError && !currentJob && "border border-danger/30")}
          >
            <div className="mb-3 flex items-center justify-between">
              <span className="text-xs font-semibold uppercase tracking-wider text-text-muted">任务进度</span>
              {currentJob ? <span className="text-[11px] text-text-secondary">{progress.label}</span> : null}
            </div>

            {currentJob ? (
              <>
                <div className="h-2 overflow-hidden rounded-full bg-surface">
                  <div
                    className={cn(
                      "h-full rounded-full transition-all duration-500",
                      progress.tone === "success" && "bg-gradient-to-r from-emerald-400 to-cyan",
                      progress.tone === "danger" && "bg-gradient-to-r from-rose-500 to-orange-400",
                      progress.tone === "progress" && "bg-gradient-to-r from-cyan to-accent",
                      progress.tone === "idle" && "bg-border",
                    )}
                    style={{ width: `${progress.value}%` }}
                  />
                </div>
                <div className="mt-3 space-y-2 text-sm">
                  <p className="font-medium text-text-primary">{progress.hint}</p>
                  <p className="text-xs text-text-secondary">任务 ID: {currentJob.id}</p>
                  {currentJob.message ? <p className="text-xs text-text-muted">{currentJob.message}</p> : null}
                </div>
              </>
            ) : null}

            {submitError ? (
              <div className="mt-3 flex items-start gap-2 rounded-xl border border-danger/30 bg-danger/10 px-3 py-3 text-sm text-danger">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>{submitError}</span>
              </div>
            ) : null}
          </motion.div>
        )}

        <div className="flex-1" />

        <motion.button
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.25 }}
          type="button"
          onClick={handleGenerate}
          disabled={!prompt.trim() || !activeModel || generating}
          className="group relative mt-2 w-full shrink-0 overflow-hidden rounded-2xl bg-gradient-to-r from-accent via-pink to-cyan py-5 text-sm font-bold transition-all hover:scale-[1.02] hover:shadow-[0_0_40px_rgba(177,73,255,0.5),0_0_80px_rgba(0,245,212,0.25)] active:scale-[0.98] disabled:opacity-40"
        >
          <div className="absolute inset-[1px] rounded-2xl bg-background/60 backdrop-blur-xl" />
          <div className="relative z-10">
            {generating ? (
              <div className="flex items-center justify-center gap-2 text-white">
                <div className="h-5 w-5 animate-spin rounded-full border-2 border-white/30 border-t-white" />
                <span className="tracking-wider">视频合成中...</span>
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center gap-1 text-white">
                <div className="flex items-center gap-2">
                  <Wand2 className="h-5 w-5 drop-shadow-[0_0_6px_rgba(255,255,255,0.6)]" />
                  <span className="tracking-[0.15em] text-[15px] drop-shadow-[0_0_8px_rgba(255,255,255,0.4)]">
                    启动视频制作
                  </span>
                </div>
                <span className="text-[10px] text-white/70">提交到真实后端并同步到到易视频引擎</span>
              </div>
            )}
          </div>
        </motion.button>
      </div>

      <div
        ref={previewSectionRef}
        className="flex min-h-[480px] flex-col pb-4 lg:col-span-6 lg:min-h-0 xl:col-span-7"
      >
        <motion.div
          initial={{ opacity: 0, scale: 0.98 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ delay: 0.1 }}
          className="glow-border cyber-grid relative flex flex-1 items-center justify-center overflow-hidden rounded-2xl border border-border bg-surface-elevated shadow-2xl"
        >
          <AnimatePresence mode="wait">
            {selectedPreviewItem?.publicUrl ? (
              <motion.div
                key={selectedPreviewItem.id}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="relative h-full w-full bg-black/50 p-3"
              >
                <VideoPreviewSurface
                  src={selectedPreviewItem.publicUrl}
                  controls
                  className="h-full w-full rounded-xl"
                  videoClassName="rounded-xl"
                />

                <div className="absolute left-8 top-8 flex items-center gap-2 rounded-lg border border-white/10 bg-black/40 px-3 py-1.5 backdrop-blur-md">
                  <span
                    className={cn(
                      "flex h-2 w-2 rounded-full",
                      isSuccessJob(selectedJob) ? "bg-success pulse-online" : "bg-warning",
                    )}
                  />
                  <span className="text-xs font-medium text-white">
                    {selectedJob ? `${selectedJob.modelName} • ${progress.label}` : "视频预览"}
                  </span>
                </div>

                <div className="absolute bottom-8 right-8 flex items-center gap-2 rounded-xl border border-white/10 bg-black/40 px-3 py-2 backdrop-blur-md">
                  <Play className="h-4 w-4 text-white" />
                  <a
                    href={selectedPreviewItem.publicUrl}
                    download={selectedPreviewItem.fileName || undefined}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 text-xs font-medium text-white hover:text-cyan"
                  >
                    <Download className="h-3.5 w-3.5" />
                    下载视频
                  </a>
                </div>
              </motion.div>
            ) : selectedJob && !isTerminalJob(selectedJob) ? (
              <motion.div
                key="generating"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="flex flex-col items-center justify-center gap-6"
              >
                <div className="relative flex h-32 w-32 items-center justify-center">
                  <div className="absolute inset-0 animate-[spin_4s_linear_infinite] rounded-full border-[1px] border-cyan/20" />
                  <div className="absolute inset-2 animate-[spin_2s_ease-in-out_infinite] rounded-full border-2 border-transparent border-t-cyan border-b-accent" />
                  <div className="absolute inset-6 animate-[spin_3s_reverse_infinite] rounded-full border-[1px] border-dashed border-accent/40" />
                  <Video className="z-10 h-8 w-8 animate-pulse text-cyan drop-shadow-[0_0_8px_rgba(0,245,212,0.8)]" />
                </div>
                <div className="space-y-3 text-center">
                  <h3 className="text-2xl font-black uppercase tracking-[0.2em] text-cyan">
                    Rendering
                  </h3>
                  <div className="flex flex-col items-center gap-2">
                    <p className="text-xs tracking-wider text-text-muted">{progress.hint}</p>
                    <div className="h-1 w-56 overflow-hidden rounded-full bg-surface">
                      <div
                        className="h-full rounded-full bg-gradient-to-r from-cyan to-accent shadow-[0_0_10px_rgba(0,245,212,0.5)] transition-all duration-500"
                        style={{ width: `${progress.value}%` }}
                      />
                    </div>
                    <span className="text-xs text-text-secondary">{progress.value}%</span>
                  </div>
                </div>
              </motion.div>
            ) : selectedJob?.status === "failed" ? (
              <motion.div
                key="failed"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="flex flex-col items-center justify-center gap-4 px-6 text-center"
              >
                <AlertTriangle className="h-14 w-14 text-rose-400" />
                <div className="space-y-2">
                  <h3 className="text-xl font-bold text-text-primary">这次视频生成失败了</h3>
                  <p className="max-w-md text-sm text-text-secondary">
                    {selectedJob.message || "后端没有返回更多信息，请检查模型配置或服务状态后重试。"}
                  </p>
                </div>
              </motion.div>
            ) : (
              <motion.div
                key="empty"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="flex flex-col items-center justify-center opacity-50 text-text-muted"
              >
                <Video className="mb-4 h-16 w-16" />
                <p>等待模型返回视频结果</p>
              </motion.div>
            )}
          </AnimatePresence>
        </motion.div>

        {selectedPreviewItems.length > 1 ? (
          <div className="mt-3 flex gap-3 overflow-x-auto pb-1">
            {selectedPreviewItems.map((item, index) => (
              <button
                key={item.id}
                type="button"
                onClick={() => setPreviewIndex(index)}
                className={cn(
                  "rounded-xl border px-3 py-2 text-xs transition-all",
                  previewIndex === index ? "border-accent bg-accent/10 text-accent" : "border-border text-text-muted",
                )}
              >
                结果 {index + 1}
              </button>
            ))}
          </div>
        ) : null}
      </div>

      <div className="flex h-full flex-col gap-4 overflow-hidden pb-4 lg:col-span-2 lg:border-l lg:border-border/50 lg:pl-5 xl:col-span-2">
        <h3 className="flex shrink-0 items-center gap-2 border-b border-border/50 pb-3 text-sm font-semibold uppercase tracking-widest text-text-secondary">
          <Layers className="h-4 w-4 text-accent" /> 最近视频任务
        </h3>

        <div className="custom-scrollbar flex-1 space-y-3 overflow-y-auto pr-1">
          {mergedJobs.length === 0 ? (
            <div className="rounded-xl border border-dashed border-border px-4 py-8 text-center text-sm text-text-muted">
              还没有视频生成记录
            </div>
          ) : (
            mergedJobs.map((job) => {
              const stage = resolveAIJobStage(job);
              const durationLabel = extractDurationLabel(job);
              const isSelected = selectedJob?.id === job.id;
              const cardPreview = getPrimaryVideoPreviewItem(job);

              return (
                <button
                  key={job.id}
                  type="button"
                  onClick={() => {
                    setSelectedJobId(job.id);
                    setPreviewIndex(0);
                  }}
                  className={cn(
                    "group relative flex w-full flex-col overflow-hidden rounded-xl border-2 bg-surface text-left transition-all",
                    isSelected
                      ? "border-accent bg-accent/5 shadow-[0_0_15px_rgba(177,73,255,0.1)]"
                      : "border-border hover:border-accent/40 hover:bg-surface-hover",
                  )}
                >
                  <div className="relative aspect-video w-full shrink-0 border-b border-border/50 bg-black">
                    {cardPreview?.publicUrl ? (
                      <VideoPreviewSurface
                        src={cardPreview.publicUrl}
                        compact
                        className="h-full w-full"
                        videoClassName="h-full w-full"
                      />
                    ) : (
                      <div className="cyber-grid flex h-full w-full items-center justify-center bg-surface-elevated">
                        <Video className="h-6 w-6 text-text-muted/60" />
                      </div>
                    )}
                    <div className="absolute bottom-1 right-1 rounded bg-black/80 px-1.5 py-0.5 text-[9px] text-white backdrop-blur">
                      {durationLabel || stage.label}
                    </div>
                  </div>

                  <div className="flex-1 p-2.5">
                    <p className="mb-1.5 line-clamp-2 text-[11px] leading-tight text-text-primary">
                      {buildAIJobTitle(job)}
                    </p>
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate text-[9px] uppercase text-text-muted">{job.modelName}</span>
                      {isSuccessJob(job) ? (
                        <span className="text-[9px] text-success">已完成</span>
                      ) : job.status === "failed" ? (
                        <span className="text-[9px] text-danger">失败</span>
                      ) : (
                        <span className="flex items-center gap-1 text-[9px] text-warning">
                          <span className="h-1 w-1 animate-pulse rounded-full bg-warning" />
                          执行中
                        </span>
                      )}
                    </div>
                    <p className="mt-1 text-[9px] text-text-muted">{formatDateTime(job.updatedAt)}</p>
                  </div>
                </button>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}
