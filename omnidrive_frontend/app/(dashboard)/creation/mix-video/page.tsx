"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AudioLines,
  CheckCircle2,
  Clock3,
  Coins,
  FileText,
  Film,
  LoaderCircle,
  MoveDown,
  MoveUp,
  Play,
  Plus,
  Scissors,
  Video as VideoIcon,
  Wallet,
  X,
} from "lucide-react";
import { PageHeader, StatusBadge } from "@/components/ui/common";
import {
  MIX_VIDEO_ASSET_ACCEPT,
  MIX_VIDEO_AUDIO_ACCEPT,
  buildMixVideoUploadEntries,
  clampMixVideoTaskProgress,
  countMixVideoUnicodeCharacters,
  formatMixVideoCreditValue,
  getMixVideoTaskProgressMessage,
  resolveMixVideoLocalPreviewKind,
  resolveMixVideoUploadProgress,
  validateMixVideoFile,
} from "@/lib/mix-video";
import { createMixVideoTask, getMixVideoBillingPreview, listMixVideoTasks } from "@/lib/services";
import type { MixVideoBillingPreview, MixVideoTask } from "@/lib/types";
import { formatDateTime } from "@/lib/workflow";

function formatBytes(bytes?: number | null) {
  if (typeof bytes !== "number" || !Number.isFinite(bytes) || bytes <= 0) {
    return "0 B";
  }
  if (bytes < 1024) {
    return `${bytes.toFixed(0)} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function useObjectUrl(file: File | null) {
  const url = useMemo(() => (file ? URL.createObjectURL(file) : ""), [file]);
  useEffect(() => {
    if (!url) return;
    return () => URL.revokeObjectURL(url);
  }, [url]);
  return url;
}

type FirstFrameState = { url: string; dataUrl: string; failed: boolean };

function useVideoFirstFrame(url: string, enabled: boolean) {
  const [state, setState] = useState<FirstFrameState>({ url: "", dataUrl: "", failed: false });

  useEffect(() => {
    if (!enabled || !url) return;

    let active = true;
    let captured = false;
    let seeked = false;
    let frameReqId = 0;
    const video = document.createElement("video");
    video.preload = "auto";
    video.muted = true;
    video.playsInline = true;
    video.crossOrigin = "anonymous";

    const capture = () => {
      if (!active || captured || video.videoWidth <= 0 || video.videoHeight <= 0) return;
      captured = true;
      const canvas = document.createElement("canvas");
      canvas.width = video.videoWidth;
      canvas.height = video.videoHeight;
      const ctx = canvas.getContext("2d");
      if (!ctx) return;
      try {
        ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
        const dataUrl = canvas.toDataURL("image/jpeg", 0.85);
        setState({ url, dataUrl, failed: false });
      } catch {
        setState({ url, dataUrl: "", failed: true });
      }
    };

    const trySeek = () => {
      if (seeked) return;
      const dur = Number.isFinite(video.duration) ? video.duration : 0;
      const target = dur > 1.5 ? Math.min(1.0, Math.max(0.15, dur * 0.15)) : 0;
      if (target <= 0) {
        capture();
        return;
      }
      seeked = true;
      try {
        video.currentTime = target;
      } catch {
        capture();
      }
    };

    const onMeta = () => {
      if (video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) trySeek();
    };
    const onData = () => {
      capture();
      trySeek();
    };
    const onSeeked = () => capture();
    const onErr = () => {
      if (active) setState({ url, dataUrl: "", failed: true });
    };

    video.addEventListener("loadedmetadata", onMeta);
    video.addEventListener("loadeddata", onData);
    video.addEventListener("seeked", onSeeked);
    video.addEventListener("error", onErr);
    video.src = url;
    video.load();

    if ("requestVideoFrameCallback" in video) {
      frameReqId = (video as HTMLVideoElement & {
        requestVideoFrameCallback: (cb: () => void) => number;
      }).requestVideoFrameCallback(() => capture());
    }

    const timeout = window.setTimeout(() => {
      if (!captured && active) setState({ url, dataUrl: "", failed: true });
    }, 4000);

    return () => {
      active = false;
      window.clearTimeout(timeout);
      if (frameReqId && "cancelVideoFrameCallback" in video) {
        (video as HTMLVideoElement & {
          cancelVideoFrameCallback: (h: number) => void;
        }).cancelVideoFrameCallback(frameReqId);
      }
      video.removeEventListener("loadedmetadata", onMeta);
      video.removeEventListener("loadeddata", onData);
      video.removeEventListener("seeked", onSeeked);
      video.removeEventListener("error", onErr);
      video.pause();
      video.removeAttribute("src");
      video.load();
    };
  }, [url, enabled]);

  const active = state.url === url && enabled;
  return {
    dataUrl: active ? state.dataUrl : "",
    failed: active ? state.failed : false,
  };
}

function FileThumbnail({ file, size = "strip" }: { file: File; size?: "strip" | "tiny" }) {
  const url = useObjectUrl(file);
  const kind = useMemo(() => resolveMixVideoLocalPreviewKind(file), [file]);
  const { dataUrl, failed } = useVideoFirstFrame(url, kind === "video");

  const dims = size === "tiny" ? "h-10 w-10" : "h-14 w-20";
  const iconSize = size === "tiny" ? "h-3 w-3" : "h-4 w-4";

  if (kind === "image" && url) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img src={url} alt={file.name} className={`${dims} rounded-lg object-cover`} />
    );
  }

  if (kind === "video") {
    if (dataUrl) {
      return (
        <div className={`relative ${dims} overflow-hidden rounded-lg bg-black`}>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={dataUrl} alt={file.name} className="h-full w-full object-cover" />
          <div className="absolute inset-0 flex items-center justify-center bg-black/25">
            <div className={`flex items-center justify-center rounded-full bg-black/55 text-white ${size === "tiny" ? "h-4 w-4" : "h-7 w-7"}`}>
              <Play className={`${iconSize} ml-0.5 fill-current`} />
            </div>
          </div>
        </div>
      );
    }
    if (failed) {
      return (
        <div className={`flex flex-col items-center justify-center gap-1 rounded-lg bg-surface-hover ${dims} text-text-muted`}>
          <VideoIcon className={iconSize} />
          {size !== "tiny" ? <span className="text-[10px]">无法预览</span> : null}
        </div>
      );
    }
    return (
      <div className={`flex items-center justify-center rounded-lg bg-surface-hover ${dims} text-text-muted`}>
        <LoaderCircle className={`${iconSize} animate-spin`} />
      </div>
    );
  }

  return (
    <div className={`flex items-center justify-center rounded-lg bg-surface-hover ${dims} text-text-muted`}>
      <Film className={iconSize} />
    </div>
  );
}

function HeroVideoPreview({ file }: { file: File | null }) {
  const url = useObjectUrl(file);
  if (!file) {
    return (
      <div className="relative h-[220px] w-full overflow-hidden rounded-xl border border-border/60 bg-gradient-to-br from-surface/60 to-black/40">
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 text-text-muted">
          <div className="flex h-11 w-11 items-center justify-center rounded-full bg-surface/60">
            <Film className="h-5 w-5" />
          </div>
          <p className="text-sm">选择视频素材后将在此预览首帧</p>
          <p className="text-xs text-text-muted/80">支持 mp4、mov、webm 多文件混剪</p>
        </div>
      </div>
    );
  }
  const isVideo = resolveMixVideoLocalPreviewKind(file) === "video";
  return (
    <div className="relative h-[220px] w-full overflow-hidden rounded-xl border border-border/60 bg-black">
      {isVideo && url ? (
        <video
          key={url}
          src={url}
          controls
          muted
          playsInline
          preload="metadata"
          className="h-full w-full object-contain"
        />
      ) : (
        <div className="flex h-full items-center justify-center text-text-muted">
          <Film className="h-8 w-8" />
        </div>
      )}
      <div className="pointer-events-none absolute left-3 top-3 inline-flex items-center gap-1.5 rounded-full bg-black/55 px-2.5 py-1 text-[11px] text-white">
        <VideoIcon className="h-3 w-3" />
        预览
      </div>
      <div className="pointer-events-none absolute bottom-3 left-3 right-3 flex items-end justify-between gap-3">
        <div className="max-w-[70%] truncate rounded-md bg-black/55 px-2 py-1 text-xs text-white">{file.name}</div>
        <div className="rounded-md bg-black/55 px-2 py-1 text-[11px] text-white">{formatBytes(file.size)}</div>
      </div>
    </div>
  );
}

function AudioPreview({ file }: { file: File }) {
  const url = useObjectUrl(file);
  if (!url) return null;
  return <audio controls src={url} className="w-full" preload="metadata" />;
}

type UploadPhase = "idle" | "uploading" | "creating" | "done";
type UploadProgressState = {
  loadedBytes: number;
  totalBytes: number;
  percentage: number;
};

const UPLOAD_VISUAL_MAX_PERCENT = 96;
const CREATE_VISUAL_START_PERCENT = 97;
const CREATE_VISUAL_MAX_PERCENT = 99;

function UploadProgressOverlay({
  phase,
  progress,
  assets,
  refAudio,
  itemProgress,
  errorMessage,
  displayPercentage,
}: {
  phase: UploadPhase;
  progress: UploadProgressState | null;
  assets: File[];
  refAudio: File | null;
  itemProgress: ReturnType<typeof resolveMixVideoUploadProgress>;
  errorMessage: string;
  displayPercentage: number;
}) {
  if (phase === "idle") return null;

  const phaseLabel =
    phase === "done"
      ? "任务已创建，即将跳转详情"
      : phase === "creating"
        ? "素材上传完成，正在创建任务"
        : "正在上传素材";

  const pct = displayPercentage;
  const loaded = progress?.loadedBytes ?? 0;
  const total = progress?.totalBytes ?? 0;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm">
      <div className="glass-card relative w-[min(560px,calc(100%-2rem))] space-y-5 p-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {phase === "done" ? (
              <CheckCircle2 className="h-5 w-5 text-success" />
            ) : (
              <LoaderCircle className="h-5 w-5 animate-spin text-accent" />
            )}
            <div>
              <p className="text-sm font-medium text-text-primary">{phaseLabel}</p>
              <p className="text-xs text-text-muted">
                {assets.length} 个素材 + {refAudio ? "1 个参考音频" : "0 个参考音频"}
              </p>
            </div>
          </div>
          <span className="font-mono text-lg font-semibold text-accent">{pct.toFixed(0)}%</span>
        </div>

        <div className="h-2.5 overflow-hidden rounded-full bg-background/60">
          <div
            className={`h-full rounded-full transition-all duration-300 ${phase === "done" ? "bg-success" : "bg-accent"}`}
            style={{ width: `${pct}%` }}
          />
        </div>

        <div className="flex items-center justify-between text-xs text-text-muted">
          <span>{formatBytes(loaded)} / {formatBytes(total)}</span>
          <span>
            {phase === "uploading" ? "请勿关闭窗口" : phase === "creating" ? "任务创建中" : "已完成"}
          </span>
        </div>

        <div className="max-h-[240px] space-y-2 overflow-y-auto rounded-lg border border-border/50 bg-surface/30 p-3">
          {itemProgress.map((item) => {
            const file = item.kind === "audio" ? refAudio : assets[parseInt(item.id.replace("asset-", ""), 10)];
            const tone =
              item.state === "completed" ? "text-success" : item.state === "uploading" ? "text-accent" : "text-text-muted";
            const stateLabel = item.state === "completed" ? "已上传" : item.state === "uploading" ? "上传中" : "等待";
            return (
              <div key={item.id} className="flex items-center gap-3">
                {file ? <FileThumbnail file={file} size="tiny" /> : null}
                <div className="min-w-0 flex-1">
                  <div className="flex items-center justify-between gap-2 text-xs">
                    <span className="truncate text-text-primary">{item.label}</span>
                    <span className={`shrink-0 ${tone}`}>
                      {stateLabel} · {item.percentage.toFixed(0)}%
                    </span>
                  </div>
                  <div className="mt-1 h-1 overflow-hidden rounded-full bg-background/60">
                    <div
                      className={`h-full rounded-full ${item.state === "completed" ? "bg-success" : "bg-accent"}`}
                      style={{ width: `${item.percentage}%` }}
                    />
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        {errorMessage ? (
          <div className="rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-xs text-danger">{errorMessage}</div>
        ) : null}
      </div>
    </div>
  );
}

export default function MixVideoCreatePage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [assets, setAssets] = useState<File[]>([]);
  const [activeIndex, setActiveIndex] = useState(0);
  const [refAudio, setRefAudio] = useState<File | null>(null);
  const [scriptText, setScriptText] = useState("");
  const [debouncedScriptText, setDebouncedScriptText] = useState("");
  const [errorMessage, setErrorMessage] = useState("");
  const [uploadProgress, setUploadProgress] = useState<UploadProgressState | null>(null);
  const [uploadPhase, setUploadPhase] = useState<UploadPhase>("idle");
  const [displayProgressPercentage, setDisplayProgressPercentage] = useState(0);

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedScriptText(scriptText.trim()), 400);
    return () => window.clearTimeout(timer);
  }, [scriptText]);

  const safeActiveIndex = assets.length === 0 ? 0 : Math.min(activeIndex, assets.length - 1);

  const previewQuery = useQuery<MixVideoBillingPreview>({
    queryKey: ["mixVideoBillingPreview", debouncedScriptText],
    queryFn: () => getMixVideoBillingPreview(debouncedScriptText),
    enabled: debouncedScriptText.length > 0,
  });

  const recentTasksQuery = useQuery<MixVideoTask[]>({
    queryKey: ["mixVideoTasks", "recent"],
    queryFn: () => listMixVideoTasks({ limit: 5 }),
    staleTime: 10_000,
  });

  const selectedUploadBytes = useMemo(() => {
    const assetBytes = assets.reduce((sum, file) => sum + file.size, 0);
    const audioBytes = refAudio?.size || 0;
    return assetBytes + audioBytes;
  }, [assets, refAudio]);

  const uploadEntries = useMemo(() => buildMixVideoUploadEntries(assets, refAudio), [assets, refAudio]);
  const uploadItemProgress = useMemo(() => {
    if (!uploadProgress) return [];
    return resolveMixVideoUploadProgress(uploadEntries, uploadProgress.loadedBytes);
  }, [uploadEntries, uploadProgress]);

  useEffect(() => {
    if (uploadPhase === "idle") {
      setDisplayProgressPercentage(0);
      return;
    }
    if (uploadPhase === "done") {
      setDisplayProgressPercentage(100);
      return;
    }
    if (uploadPhase === "uploading") {
      const rawPercentage = uploadProgress?.percentage ?? 0;
      const scaledPercentage = rawPercentage >= 100
        ? UPLOAD_VISUAL_MAX_PERCENT
        : Math.min(UPLOAD_VISUAL_MAX_PERCENT, rawPercentage * (UPLOAD_VISUAL_MAX_PERCENT / 100));
      setDisplayProgressPercentage(scaledPercentage);
      return;
    }
    if (uploadPhase !== "creating") {
      return;
    }

    setDisplayProgressPercentage((current) => Math.max(current, CREATE_VISUAL_START_PERCENT));
    const timer = window.setInterval(() => {
      setDisplayProgressPercentage((current) => {
        if (current >= CREATE_VISUAL_MAX_PERCENT) {
          return CREATE_VISUAL_MAX_PERCENT;
        }
        const remaining = CREATE_VISUAL_MAX_PERCENT - current;
        const increment = Math.max(0.2, remaining * 0.2);
        return Math.min(CREATE_VISUAL_MAX_PERCENT, current + increment);
      });
    }, 320);

    return () => window.clearInterval(timer);
  }, [uploadPhase, uploadProgress]);

  const createMutation = useMutation({
    mutationFn: async () => {
      const formData = new FormData();
      assets.forEach((file) => formData.append("assets[]", file));
      if (refAudio) {
        formData.append("refAudio", refAudio);
      }
      formData.append("scriptText", scriptText.trim());
      return createMixVideoTask(formData, {
        onUploadProgress: ({ loaded, total, percentage }) => {
          const nextTotal = total ?? selectedUploadBytes;
          const safeTotal = nextTotal > 0 ? nextTotal : Math.max(loaded, 1);
          const safePercentage = safeTotal > 0
            ? Math.max(0, Math.min(100, percentage > 0 ? percentage : (loaded / safeTotal) * 100))
            : 0;
          setUploadProgress({
            loadedBytes: Math.max(loaded, 0),
            totalBytes: safeTotal,
            percentage: safePercentage,
          });
          if (safePercentage >= 100) {
            setUploadPhase("creating");
          } else {
            setUploadPhase("uploading");
          }
        },
      });
    },
    onMutate: () => {
      setErrorMessage("");
      setUploadPhase("uploading");
      setDisplayProgressPercentage(0);
      setUploadProgress({
        loadedBytes: 0,
        totalBytes: Math.max(selectedUploadBytes, 1),
        percentage: 0,
      });
    },
    onSuccess: async (task) => {
      setUploadPhase("done");
      setDisplayProgressPercentage(100);
      setUploadProgress((current) =>
        current
          ? { ...current, loadedBytes: current.totalBytes, percentage: 100 }
          : { loadedBytes: 1, totalBytes: 1, percentage: 100 },
      );
      await queryClient.invalidateQueries({ queryKey: ["mixVideoTasks"] });
      window.setTimeout(() => {
        router.push(`/creation/mix-video/${task.id}`);
      }, 1200);
    },
    onError: (error) => {
      setUploadPhase("idle");
      setUploadProgress(null);
      setErrorMessage(error instanceof Error ? error.message : "创建混剪任务失败");
    },
  });

  const preview = previewQuery.data;
  const missingReasons: string[] = [];
  if (assets.length === 0) missingReasons.push("请上传至少 1 个视频素材");
  if (!refAudio) missingReasons.push("请上传参考音频");
  if (!scriptText.trim()) missingReasons.push("请输入文案");
  const insufficientBalance = Boolean(preview && !preview.canAfford);
  if (insufficientBalance && preview) {
    missingReasons.push(`余额不足 ${formatMixVideoCreditValue(preview.shortfallCredits)} 积分`);
  }
  const submitDisabled =
    missingReasons.length > 0 || createMutation.isPending || previewQuery.isFetching;

  const activeFile = assets[safeActiveIndex] || null;

  const handleAssetSelect = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files || []);
    if (files.length === 0) return;

    const nextFiles: File[] = [];
    for (const file of files) {
      const message = validateMixVideoFile(file, "asset");
      if (message) {
        setErrorMessage(message);
        continue;
      }
      nextFiles.push(file);
    }
    if (nextFiles.length > 0) {
      setAssets((current) => {
        const merged = [...current, ...nextFiles];
        if (current.length === 0) setActiveIndex(0);
        return merged;
      });
      setErrorMessage("");
    }
    event.target.value = "";
  }, []);

  const handleAudioSelect = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0] || null;
    if (!file) return;
    const message = validateMixVideoFile(file, "audio");
    if (message) {
      setErrorMessage(message);
      return;
    }
    setRefAudio(file);
    setErrorMessage("");
    event.target.value = "";
  }, []);

  const moveAsset = useCallback((index: number, direction: -1 | 1) => {
    setAssets((current) => {
      const next = [...current];
      const target = index + direction;
      if (target < 0 || target >= next.length) return current;
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });
    setActiveIndex((current) => {
      if (current === index) return Math.max(0, Math.min(assets.length - 1, index + direction));
      if (current === index + direction) return index;
      return current;
    });
  }, [assets.length]);

  const removeAsset = useCallback((index: number) => {
    setAssets((current) => current.filter((_, i) => i !== index));
  }, []);

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="混剪"
          subtitle="上传素材、参考音频和文案，系统先预估时长与积分，再异步生成成片。"
          actions={
            <Link
              href="/creation/mix-video/history"
              className="btn-neon inline-flex items-center gap-2 rounded-lg border border-border px-3.5 py-2 text-sm text-text-primary"
            >
              <Scissors className="h-4 w-4" />
              历史
            </Link>
          }
        />

        <div className="glass-card flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 text-xs">
          {[
            { label: "视频素材", done: assets.length > 0, hint: assets.length > 0 ? `${assets.length} 段` : "必选" },
            { label: "参考音频", done: Boolean(refAudio), hint: refAudio ? "已选择" : "必选" },
            { label: "文案", done: scriptText.trim().length > 0, hint: scriptText.trim() ? `${countMixVideoUnicodeCharacters(scriptText)} 字` : "必填" },
          ].map((step, index) => (
            <div key={step.label} className="flex items-center gap-2">
              <span
                className={`flex h-5 w-5 items-center justify-center rounded-full text-[10px] font-semibold ${step.done ? "bg-success/20 text-success" : "bg-surface/60 text-text-muted"}`}
              >
                {step.done ? <CheckCircle2 className="h-3 w-3" /> : index + 1}
              </span>
              <span className={step.done ? "text-text-primary" : "text-text-secondary"}>{step.label}</span>
              <span className="text-text-muted">· {step.hint}</span>
            </div>
          ))}
        </div>

        <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
          <section className="space-y-5">
            {/* Hero preview + thumbnail strip */}
            <div className="glass-card space-y-4 p-5">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-sm font-medium text-text-primary">
                  <VideoIcon className="h-4 w-4 text-accent" />
                  素材预览
                </div>
                <label className="inline-flex cursor-pointer items-center gap-1.5 rounded-lg bg-accent/10 px-3 py-1.5 text-xs font-medium text-accent transition hover:bg-accent/15">
                  <Plus className="h-3.5 w-3.5" />
                  添加素材
                  <input
                    type="file"
                    accept={MIX_VIDEO_ASSET_ACCEPT}
                    multiple
                    className="hidden"
                    onChange={handleAssetSelect}
                  />
                </label>
              </div>

              <HeroVideoPreview file={activeFile} />

              {assets.length > 0 ? (
                <div className="space-y-3">
                  <div className="flex items-center justify-between text-xs text-text-muted">
                    <span>
                      共 {assets.length} 段 · 当前第 {safeActiveIndex + 1} 段
                    </span>
                    <span>点击缩略图切换预览，按顺序拼接</span>
                  </div>
                  <div className="flex gap-3 overflow-x-auto pb-2">
                    {assets.map((file, index) => {
                      const active = index === safeActiveIndex;
                      return (
                        <div
                          key={`${file.name}-${index}-${file.size}`}
                          className={`relative shrink-0 rounded-xl border p-2 transition ${active ? "border-accent bg-accent/10" : "border-border/50 bg-surface/30 hover:border-accent/40"}`}
                        >
                          <button
                            type="button"
                            onClick={() => setActiveIndex(index)}
                            className="flex flex-col items-center gap-2 text-left"
                            title={file.name}
                          >
                            <FileThumbnail file={file} size="strip" />
                            <div className="w-20 min-w-0">
                              <p className="truncate text-[10px] font-medium text-text-primary">{file.name}</p>
                              <p className="text-[10px] text-text-muted">{formatBytes(file.size)}</p>
                            </div>
                          </button>
                          <div className="mt-2 flex items-center justify-center gap-1">
                            <button
                              type="button"
                              onClick={() => moveAsset(index, -1)}
                              disabled={index === 0}
                              className="rounded p-1 text-text-muted hover:bg-surface-hover hover:text-text-primary disabled:opacity-30"
                              title="上移"
                            >
                              <MoveUp className="h-3.5 w-3.5" />
                            </button>
                            <button
                              type="button"
                              onClick={() => moveAsset(index, 1)}
                              disabled={index === assets.length - 1}
                              className="rounded p-1 text-text-muted hover:bg-surface-hover hover:text-text-primary disabled:opacity-30"
                              title="下移"
                            >
                              <MoveDown className="h-3.5 w-3.5" />
                            </button>
                            <button
                              type="button"
                              onClick={() => removeAsset(index)}
                              className="rounded p-1 text-text-muted hover:bg-danger/10 hover:text-danger"
                              title="删除"
                            >
                              <X className="h-3.5 w-3.5" />
                            </button>
                          </div>
                          <span className="absolute -left-1 -top-1 flex h-5 w-5 items-center justify-center rounded-full bg-accent text-[10px] font-semibold text-background">
                            {index + 1}
                          </span>
                        </div>
                      );
                    })}
                  </div>
                </div>
              ) : (
                <label className="flex cursor-pointer items-center justify-center gap-2 rounded-lg border border-dashed border-border bg-surface/40 px-4 py-5 text-sm text-text-secondary transition hover:border-accent/40 hover:text-text-primary">
                  <Plus className="h-4 w-4" />
                  点击选择视频素材（mp4 / mov / webm，可多选）
                  <input
                    type="file"
                    accept={MIX_VIDEO_ASSET_ACCEPT}
                    multiple
                    className="hidden"
                    onChange={handleAssetSelect}
                  />
                </label>
              )}
            </div>

            {/* Audio + script row */}
            <div className="grid gap-5 lg:grid-cols-2">
              <div className="glass-card space-y-3 p-5">
                <div className="flex items-center gap-2 text-sm font-medium text-text-primary">
                  <AudioLines className="h-4 w-4 text-accent" />
                  参考音频
                </div>
                <label className="flex cursor-pointer items-center justify-center rounded-lg border border-dashed border-border bg-surface/40 px-4 py-4 text-sm text-text-secondary transition hover:border-accent/40 hover:text-text-primary">
                  <input
                    type="file"
                    accept={MIX_VIDEO_AUDIO_ACCEPT}
                    className="hidden"
                    onChange={handleAudioSelect}
                  />
                  {refAudio ? `已选择：${refAudio.name}` : "上传参考音频（m4a / mp3 / wav / aac）"}
                </label>
                {refAudio ? (
                  <div className="rounded-lg border border-border/50 bg-surface/30 p-3">
                    <AudioPreview file={refAudio} />
                    <div className="mt-2 flex items-center justify-between text-[11px] text-text-muted">
                      <span className="truncate">{refAudio.name}</span>
                      <div className="flex items-center gap-2">
                        <span>{formatBytes(refAudio.size)}</span>
                        <button
                          type="button"
                          onClick={() => setRefAudio(null)}
                          className="rounded p-1 hover:bg-danger/10 hover:text-danger"
                          title="移除"
                        >
                          <X className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>
                  </div>
                ) : null}
              </div>

              <div className="glass-card space-y-3 p-5">
                <div className="flex items-center gap-2 text-sm font-medium text-text-primary">
                  <FileText className="h-4 w-4 text-accent" />
                  文案
                </div>
                <textarea
                  value={scriptText}
                  onChange={(event) => setScriptText(event.target.value)}
                  placeholder="输入混剪文案，系统按 4 字/秒估算时长，最终按成片时长结算。"
                  rows={8}
                  className="w-full rounded-lg border border-border bg-surface/40 px-4 py-3 text-sm text-text-primary outline-none transition focus:border-accent/50"
                />
                <div className="flex items-center justify-between text-xs text-text-muted">
                  <span>Unicode 字符数：{countMixVideoUnicodeCharacters(scriptText)}</span>
                  <span>防抖 400ms 自动预估</span>
                </div>
              </div>
            </div>

            {errorMessage ? (
              <div className="rounded-lg border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">
                {errorMessage}
              </div>
            ) : null}
          </section>

          <aside className="space-y-5">
            {/* Billing highlight */}
            <section className="glass-card p-5">
              <h2 className="mb-3 text-base font-semibold text-text-primary">计费预估</h2>
              <div className="rounded-lg bg-gradient-to-br from-accent/10 via-accent/5 to-transparent p-4">
                <div className="flex items-end justify-between gap-3">
                  <div>
                    <p className="text-[11px] text-text-muted">预计时长</p>
                    <p className="mt-1 text-2xl font-semibold text-text-primary">
                      {preview?.estimatedDurationSeconds ?? 0}
                      <span className="ml-1 text-xs font-normal text-text-muted">秒</span>
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="text-[11px] text-text-muted">预计扣费</p>
                    <p className="mt-1 text-2xl font-semibold text-accent">
                      {formatMixVideoCreditValue(preview?.estimatedCredits)}
                      <span className="ml-1 text-xs font-normal text-text-muted">积分</span>
                    </p>
                  </div>
                </div>
              </div>
              <div className="mt-4 space-y-2 text-sm">
                <div className="flex items-center justify-between text-text-secondary">
                  <span className="inline-flex items-center gap-2">
                    <Wallet className="h-3.5 w-3.5" />
                    当前余额
                  </span>
                  <span className="font-medium text-text-primary">
                    {formatMixVideoCreditValue(preview?.creditBalance)} 积分
                  </span>
                </div>
                <div className="flex items-center justify-between text-text-secondary">
                  <span className="inline-flex items-center gap-2">
                    <Clock3 className="h-3.5 w-3.5" />
                    费率
                  </span>
                  <span className="font-medium text-text-primary">
                    {formatMixVideoCreditValue(preview?.creditsPerSecond)} 积分/秒
                  </span>
                </div>
                {insufficientBalance ? (
                  <div className="flex items-center justify-between rounded-md bg-danger/10 px-2 py-1 text-danger">
                    <span>差额</span>
                    <span className="font-medium">
                      {formatMixVideoCreditValue(preview?.shortfallCredits)} 积分
                    </span>
                  </div>
                ) : null}
                {previewQuery.isFetching ? (
                  <p className="text-[11px] text-text-muted">正在更新预估...</p>
                ) : null}
              </div>
            </section>

            {/* Submit CTA */}
            <section className="glass-card space-y-3 p-5">
              <button
                type="button"
                disabled={submitDisabled}
                onClick={() => createMutation.mutate()}
                className="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-accent px-4 py-3 text-sm font-semibold text-background transition hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {createMutation.isPending ? (
                  <LoaderCircle className="h-4 w-4 animate-spin" />
                ) : (
                  <Scissors className="h-4 w-4" />
                )}
                立即创建混剪任务
              </button>
              {missingReasons.length > 0 ? (
                <ul className="space-y-1 text-xs text-text-muted">
                  {missingReasons.map((reason) => (
                    <li key={reason} className="flex items-center gap-1.5">
                      <span className="h-1.5 w-1.5 rounded-full bg-text-muted" />
                      {reason}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-xs text-success">准备就绪，可直接提交</p>
              )}
            </section>

            {/* Recent tasks */}
            <section className="glass-card p-5">
              <div className="mb-4 flex items-center justify-between">
                <h2 className="text-base font-semibold text-text-primary">最近任务</h2>
                <Link href="/creation/mix-video/history" className="text-xs text-accent">
                  查看全部
                </Link>
              </div>
              <div className="space-y-3">
                {(recentTasksQuery.data || []).map((task) => (
                  <Link
                    key={task.id}
                    href={`/creation/mix-video/${task.id}`}
                    className="block rounded-lg border border-border/50 bg-surface/30 p-3 transition hover:border-accent/40"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <p className="truncate text-sm font-medium text-text-primary">
                        {task.scriptText.slice(0, 24) || "混剪任务"}
                      </p>
                      <StatusBadge status={task.status} />
                    </div>
                    <p className="mt-1 text-xs text-text-muted">{formatDateTime(task.updatedAt)}</p>
                    {!["completed", "failed", "cancelled"].includes(task.status) ? (
                      <div className="mt-2">
                        <div className="flex items-center justify-between text-[11px] text-text-muted">
                          <span className="truncate">{getMixVideoTaskProgressMessage(task)}</span>
                          <span className="ml-2 shrink-0 font-mono text-accent">
                            {clampMixVideoTaskProgress(task).toFixed(0)}%
                          </span>
                        </div>
                        <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-background/50">
                          <div
                            className="h-full rounded-full bg-accent transition-all duration-300"
                            style={{ width: `${clampMixVideoTaskProgress(task)}%` }}
                          />
                        </div>
                      </div>
                    ) : null}
                  </Link>
                ))}
                {recentTasksQuery.data?.length === 0 ? (
                  <p className="text-sm text-text-muted">还没有混剪任务。</p>
                ) : null}
              </div>
            </section>

            <div className="rounded-lg bg-surface/40 p-3 text-[11px] leading-5 text-text-muted">
              <p className="mb-1 inline-flex items-center gap-1 text-text-primary">
                <Coins className="h-3 w-3" />
                结算说明
              </p>
              <p>按 4 字/秒估算时长，最终按成片真实时长结算积分。</p>
            </div>
          </aside>
        </div>
      </div>

      <UploadProgressOverlay
        phase={uploadPhase}
        progress={uploadProgress}
        assets={assets}
        refAudio={refAudio}
        itemProgress={uploadItemProgress}
        errorMessage={errorMessage}
        displayPercentage={displayProgressPercentage}
      />
    </>
  );
}
