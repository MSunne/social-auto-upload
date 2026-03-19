"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { motion, AnimatePresence } from "framer-motion";
import {
  AlertTriangle,
  Check,
  ChevronDown,
  Copy,
  Download,
  ImagePlus,
  Info,
  Layers,
  Sparkles,
  Upload,
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

type ReferenceImage = {
  id: string;
  previewUrl: string;
  dataUrl: string;
  fileName: string;
  mimeType: string;
};

type ImageSizeOption = {
  aspectRatio: string;
  resolution: string;
  width: number;
  height: number;
};

type ImagePreviewItem = {
  id: string;
  artifactKey: string;
  artifactType: string;
  fileName?: string | null;
  mimeType?: string | null;
  publicUrl?: string | null;
  textContent?: string | null;
};

const DEFAULT_SIZE_OPTIONS: ImageSizeOption[] = [
  { aspectRatio: "1:1", resolution: "1024x1024", width: 1024, height: 1024 },
  { aspectRatio: "16:9", resolution: "1344x768", width: 1344, height: 768 },
  { aspectRatio: "9:16", resolution: "768x1344", width: 768, height: 1344 },
  { aspectRatio: "4:3", resolution: "1152x896", width: 1152, height: 896 },
  { aspectRatio: "3:4", resolution: "896x1152", width: 896, height: 1152 },
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

function parseSizeOption(value: string): ImageSizeOption | null {
  const normalized = value.trim().toLowerCase().replace("*", "x");
  const match = normalized.match(/^(\d+)\s*x\s*(\d+)$/);
  if (!match) {
    return null;
  }
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (!width || !height) {
    return null;
  }
  return {
    aspectRatio: toAspectRatio(width, height),
    resolution: `${width}x${height}`,
    width,
    height,
  };
}

function buildImageSizeOptions(model?: AIModel | null): ImageSizeOption[] {
  const supported = (model?.imageSupportedSizes || [])
    .map((item) => parseSizeOption(item))
    .filter((item): item is ImageSizeOption => Boolean(item));
  if (supported.length > 0) {
    return supported;
  }
  return DEFAULT_SIZE_OPTIONS;
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

function pickImageArtifacts(items: ImagePreviewItem[]) {
  return items.filter((item) => {
    if (!item.publicUrl) {
      return false;
    }
    if (item.artifactType === "image") {
      return true;
    }
    return (item.mimeType || "").startsWith("image/");
  });
}

function extractImageArtifactsFromPayload(job?: AIJob | null): ImagePreviewItem[] {
  const artifacts = (job?.outputPayload?.artifacts as Record<string, unknown>[] | undefined) || [];
  return artifacts
    .map((item, index) => ({
      id: String(item.id || `${job?.id || "job"}_payload_${index}`),
      artifactKey: String(item.artifactKey || `artifact_${index}`),
      artifactType: String(item.artifactType || ""),
      fileName: typeof item.fileName === "string" ? item.fileName : null,
      mimeType: typeof item.mimeType === "string" ? item.mimeType : null,
      publicUrl: typeof item.publicUrl === "string" ? item.publicUrl : null,
      textContent: typeof item.textContent === "string" ? item.textContent : null,
    }))
    .filter((item) => item.publicUrl || item.textContent);
}

function toPreviewItem(artifact: AIJobArtifact): ImagePreviewItem {
  return {
    id: artifact.id,
    artifactKey: artifact.artifactKey,
    artifactType: artifact.artifactType,
    fileName: artifact.fileName,
    mimeType: artifact.mimeType,
    publicUrl: artifact.publicUrl,
    textContent: artifact.textContent,
  };
}

function sortJobsByUpdatedAt(items: AIJob[]) {
  return [...items].sort((left, right) => {
    return new Date(right.updatedAt || 0).getTime() - new Date(left.updatedAt || 0).getTime();
  });
}

function buildProgress(job?: AIJob | null) {
  if (!job) {
    return {
      value: 0,
      label: "等待开始",
      tone: "idle" as const,
      hint: "提交图片制作任务后，这里会显示云端进度。",
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
      value: 22,
      label: stage.label,
      tone: "progress" as const,
      hint: stage.description || "任务已经入队，等待云端消费。",
    };
  }
  if (stage.key === "storyboarding") {
    return {
      value: 45,
      label: stage.label,
      tone: "progress" as const,
      hint: stage.description || "正在优化分镜和提示词。",
    };
  }
  if (stage.key === "generating") {
    return {
      value: 78,
      label: stage.label,
      tone: "progress" as const,
      hint: stage.description || "模型正在生成图片。",
    };
  }
  if (stage.key === "output_ready" || stage.key === "imported" || isSuccessJob(job)) {
    return {
      value: 100,
      label: stage.label,
      tone: "success" as const,
      hint: stage.description || "图片生成完成，可以直接预览。",
    };
  }
  if (stage.key === "publish_failed" || job.status === "failed") {
    return {
      value: 100,
      label: "生成失败",
      tone: "danger" as const,
      hint: stage.description || job.message || "生成任务执行失败。",
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

function extractTextResult(job?: AIJob | null, artifacts: AIJobArtifact[] = []) {
  const artifactText = artifacts.find((item) => item.artifactType === "text" && item.textContent?.trim());
  if (artifactText?.textContent?.trim()) {
    return artifactText.textContent.trim();
  }

  const outputText = typeof job?.outputPayload?.text === "string" ? job.outputPayload.text.trim() : "";
  if (outputText) {
    return outputText;
  }

  const payloadArtifacts = (job?.outputPayload?.artifacts as Record<string, unknown>[] | undefined) || [];
  for (const artifact of payloadArtifacts) {
    if (artifact.artifactType === "text" && typeof artifact.textContent === "string" && artifact.textContent.trim()) {
      return artifact.textContent.trim();
    }
  }

  return "";
}

function buildPromptOptimizationMessages(prompt: string, referenceImages: ReferenceImage[]) {
  const userContent: Array<Record<string, unknown>> = [
    {
      type: "text",
      text:
        "请把下面的图片创作需求优化成一段可以直接用于 AI 作图的专业中文提示词。保留用户真实诉求，不要擅自改主题。",
    },
    {
      type: "text",
      text: `原始提示词：${prompt.trim()}`,
    },
  ];

  if (referenceImages.length > 0) {
    userContent.push({
      type: "text",
      text: "以下是用户上传的参考图，请结合主体、材质、构图、灯光、色彩和风格特征一起优化。",
    });
    referenceImages.forEach((image) => {
      userContent.push({
        type: "image_url",
        image_url: {
          url: image.dataUrl,
        },
      });
    });
  }

  userContent.push({
    type: "text",
    text:
      "输出要求：1. 只返回最终优化后的提示词正文。2. 不要加标题、编号、解释、引号或多余客套。3. 需要补全构图、镜头、主体细节、材质、光线、背景、色彩、风格和画质描述。4. 如果参考图与文字有冲突，优先保留文字目标，再吸收参考图的视觉语言。",
  });

  return [
    {
      role: "system",
      content:
        "你是专业的 AI 图片提示词优化师，擅长把朴素需求整理成适合图片生成模型执行的高质量中文提示词。",
    },
    {
      role: "user",
      content: userContent,
    },
  ];
}

export default function ImageCreationPage() {
  const [prompt, setPrompt] = useState("");
  const [selectedModel, setSelectedModel] = useState("");
  const [selectedSize, setSelectedSize] = useState("");
  const [count, setCount] = useState(1);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState("");
  const [optimizingPrompt, setOptimizingPrompt] = useState(false);
  const [optimizeError, setOptimizeError] = useState("");
  const [optimizedPrompt, setOptimizedPrompt] = useState("");
  const [optimizedPromptApplied, setOptimizedPromptApplied] = useState(false);
  const [refImages, setRefImages] = useState<ReferenceImage[]>([]);
  const [isModelDropdownOpen, setIsModelDropdownOpen] = useState(false);
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  const [currentJobId, setCurrentJobId] = useState<string | null>(null);
  const [currentOptimizeJobId, setCurrentOptimizeJobId] = useState<string | null>(null);
  const [previewIndex, setPreviewIndex] = useState(0);
  const [copied, setCopied] = useState(false);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const previewSectionRef = useRef<HTMLDivElement>(null);
  const autoPreviewedJobIdRef = useRef<string | null>(null);
  const resolvedOptimizeJobIdRef = useRef<string | null>(null);
  const latestRefImagesRef = useRef<ReferenceImage[]>([]);

  const { data: allModels = [], isLoading: modelsLoading } = useQuery<AIModel[]>({
    queryKey: ["aiModels", "image"],
    queryFn: () => listAIModels({ category: "image" }),
  });
  const { data: allChatModels = [], isLoading: chatModelsLoading } = useQuery<AIModel[]>({
    queryKey: ["aiModels", "chat"],
    queryFn: () => listAIModels({ category: "chat" }),
  });

  const imageModels = useMemo(() => {
    const filtered = allModels.filter((item) => item.category === "image" && item.isEnabled);
    return filtered.length > 0 ? filtered : allModels.filter((item) => item.category === "image");
  }, [allModels]);
  const chatModels = useMemo(() => {
    const filtered = allChatModels.filter((item) => item.category === "chat" && item.isEnabled);
    return filtered.length > 0 ? filtered : allChatModels.filter((item) => item.category === "chat");
  }, [allChatModels]);

  const activeModel = useMemo(() => {
    return imageModels.find((item) => item.modelName === selectedModel) || imageModels[0] || null;
  }, [imageModels, selectedModel]);
  const activeOptimizeModel = useMemo(() => chatModels[0] || null, [chatModels]);

  const sizeOptions = useMemo(() => buildImageSizeOptions(activeModel), [activeModel]);
  const selectedSizeOption = useMemo(() => {
    return sizeOptions.find((item) => item.resolution === selectedSize) || sizeOptions[0] || null;
  }, [selectedSize, sizeOptions]);

  const maxRefImages = useMemo(() => {
    const limit = Number(activeModel?.imageReferenceLimit || 4) || 4;
    return Math.max(1, Math.min(4, limit));
  }, [activeModel]);

  const {
    data: imageJobs = [],
    refetch: refetchImageJobs,
  } = useQuery<AIJob[]>({
    queryKey: ["aiJobs", "image"],
    queryFn: () => listAIJobs({ jobType: "image", limit: 20 }),
    refetchInterval: currentJobId ? 4000 : false,
  });

  const { data: currentJob } = useQuery<AIJob>({
    queryKey: ["aiJob", currentJobId],
    queryFn: () => getAIJob(currentJobId as string),
    enabled: Boolean(currentJobId),
    refetchInterval: (query) => {
      const job = query.state.data as AIJob | undefined;
      return currentJobId && !isTerminalJob(job) ? 2000 : false;
    },
  });
  const { data: currentOptimizeJob } = useQuery<AIJob>({
    queryKey: ["aiJob", "prompt-optimize", currentOptimizeJobId],
    queryFn: () => getAIJob(currentOptimizeJobId as string),
    enabled: Boolean(currentOptimizeJobId),
    refetchInterval: (query) => {
      const job = query.state.data as AIJob | undefined;
      return currentOptimizeJobId && !isTerminalJob(job) ? 2000 : false;
    },
  });
  const { data: currentOptimizeArtifacts = [] } = useQuery<AIJobArtifact[]>({
    queryKey: ["aiJobArtifacts", "prompt-optimize", currentOptimizeJobId],
    queryFn: () => getAIJobArtifacts(currentOptimizeJobId as string),
    enabled: Boolean(currentOptimizeJobId),
    refetchInterval:
      currentOptimizeJobId && currentOptimizeJob && !isTerminalJob(currentOptimizeJob) ? 2000 : false,
  });

  const mergedJobs = useMemo(() => {
    if (!currentJob) {
      return sortJobsByUpdatedAt(imageJobs);
    }
    return sortJobsByUpdatedAt([currentJob, ...imageJobs.filter((item) => item.id !== currentJob.id)]);
  }, [currentJob, imageJobs]);

  const selectedJob = useMemo(() => {
    if (selectedJobId && currentJob && selectedJobId === currentJob.id) {
      return currentJob;
    }
    if (selectedJobId) {
      return mergedJobs.find((item) => item.id === selectedJobId) || null;
    }
    return mergedJobs[0] || null;
  }, [currentJob, mergedJobs, selectedJobId]);

  const {
    data: selectedJobArtifacts = [],
  } = useQuery<AIJobArtifact[]>({
    queryKey: ["aiJobArtifacts", selectedJob?.id],
    queryFn: () => getAIJobArtifacts(selectedJob?.id as string),
    enabled: Boolean(selectedJob?.id),
    refetchInterval:
      selectedJob?.id && currentJobId && selectedJob.id === currentJobId && !isTerminalJob(currentJob)
        ? 2000
        : false,
  });

  const selectedPreviewItems = useMemo(() => {
    const serverArtifacts = pickImageArtifacts(selectedJobArtifacts.map((item) => toPreviewItem(item)));
    if (serverArtifacts.length > 0) {
      return serverArtifacts;
    }
    return pickImageArtifacts(extractImageArtifactsFromPayload(selectedJob));
  }, [selectedJob, selectedJobArtifacts]);

  const selectedPreviewItem = selectedPreviewItems[previewIndex] || selectedPreviewItems[0] || null;
  const progress = buildProgress(currentJob || selectedJob);
  const optimizeProgress = buildProgress(currentOptimizeJob);
  const generating = submitting || Boolean(currentJob && !isTerminalJob(currentJob));
  const optimizing = optimizingPrompt || Boolean(currentOptimizeJob && !isTerminalJob(currentOptimizeJob));

  useEffect(() => {
    if (!selectedModel && imageModels.length > 0) {
      setSelectedModel(imageModels[0].modelName);
      return;
    }
    if (selectedModel && !imageModels.some((item) => item.modelName === selectedModel) && imageModels.length > 0) {
      setSelectedModel(imageModels[0].modelName);
    }
  }, [imageModels, selectedModel]);

  useEffect(() => {
    if (!selectedSize && sizeOptions.length > 0) {
      setSelectedSize(sizeOptions[0].resolution);
      return;
    }
    if (selectedSize && !sizeOptions.some((item) => item.resolution === selectedSize) && sizeOptions.length > 0) {
      setSelectedSize(sizeOptions[0].resolution);
    }
  }, [selectedSize, sizeOptions]);

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
    latestRefImagesRef.current = refImages;
  }, [refImages]);

  useEffect(() => {
    return () => {
      latestRefImagesRef.current.forEach((item) => URL.revokeObjectURL(item.previewUrl));
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
    void refetchImageJobs();
    previewSectionRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  }, [currentJob, refetchImageJobs]);

  useEffect(() => {
    if (!copied) {
      return;
    }
    const timer = window.setTimeout(() => setCopied(false), 1500);
    return () => window.clearTimeout(timer);
  }, [copied]);

  useEffect(() => {
    if (!currentOptimizeJob || !isTerminalJob(currentOptimizeJob)) {
      return;
    }
    if (resolvedOptimizeJobIdRef.current === currentOptimizeJob.id) {
      return;
    }
    if (currentOptimizeJob.status === "failed") {
      resolvedOptimizeJobIdRef.current = currentOptimizeJob.id;
      setOptimizeError(currentOptimizeJob.message || "AI 优化失败，请稍后再试");
      return;
    }

    const nextPrompt = extractTextResult(currentOptimizeJob, currentOptimizeArtifacts);
    if (!nextPrompt) {
      return;
    }

    resolvedOptimizeJobIdRef.current = currentOptimizeJob.id;
    setOptimizedPrompt(nextPrompt);
    setOptimizedPromptApplied(false);
    setOptimizeError("");
  }, [currentOptimizeArtifacts, currentOptimizeJob]);

  async function handleGenerate() {
    if (!prompt.trim() || !activeModel || !selectedSizeOption) {
      return;
    }

    setSubmitError("");
    setSubmitting(true);

    try {
      const payload: CreateAIJobRequest = {
        jobType: "image",
        modelName: activeModel.modelName,
        prompt: prompt.trim(),
        source: "omnidrive_cloud",
        inputPayload: {
          prompt: prompt.trim(),
          aspectRatio: selectedSizeOption.aspectRatio,
          resolution: selectedSizeOption.resolution,
          size: selectedSizeOption.resolution,
          count,
          referenceImages: refImages.map((item) => ({
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
      await refetchImageJobs();
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : "图片生成请求失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleOptimizePrompt() {
    if (!prompt.trim()) {
      setOptimizeError("请先输入要优化的提示词");
      return;
    }
    if (!activeOptimizeModel) {
      setOptimizeError("后台还没有启用可用的 AI 优化模型");
      return;
    }

    setOptimizeError("");
    setOptimizedPrompt("");
    setOptimizedPromptApplied(false);
    setOptimizingPrompt(true);
    resolvedOptimizeJobIdRef.current = null;

    try {
      const payload: CreateAIJobRequest = {
        jobType: "chat",
        modelName: activeOptimizeModel.modelName,
        prompt: prompt.trim(),
        source: "omnidrive_cloud",
        inputPayload: {
          prompt: prompt.trim(),
          temperature: 0.4,
          maxTokens: 600,
          messages: buildPromptOptimizationMessages(prompt, refImages),
        },
      };
      const job = await createAIJob(payload);
      setCurrentOptimizeJobId(job.id);
    } catch (error) {
      setOptimizeError(error instanceof Error ? error.message : "AI 优化请求失败");
    } finally {
      setOptimizingPrompt(false);
    }
  }

  async function handleFileSelect(event: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files || []);
    if (files.length === 0) {
      return;
    }

    const remaining = Math.max(0, maxRefImages - refImages.length);
    const selectedFiles = files.slice(0, remaining);
    if (selectedFiles.length === 0) {
      event.target.value = "";
      return;
    }

    const nextImages = await Promise.all(
      selectedFiles.map(async (file) => ({
        id: `${file.name}_${file.size}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
        previewUrl: URL.createObjectURL(file),
        dataUrl: await readFileAsDataUrl(file),
        fileName: file.name,
        mimeType: file.type || "image/png",
      })),
    );

    setRefImages((previous) => [...previous, ...nextImages].slice(0, maxRefImages));
    event.target.value = "";
  }

  function removeRefImage(id: string) {
    setRefImages((previous) => {
      const target = previous.find((item) => item.id === id);
      if (target) {
        URL.revokeObjectURL(target.previewUrl);
      }
      return previous.filter((item) => item.id !== id);
    });
  }

  async function copyPreviewLink() {
    if (!selectedPreviewItem?.publicUrl) {
      return;
    }
    try {
      await navigator.clipboard.writeText(selectedPreviewItem.publicUrl);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  function applyOptimizedPrompt() {
    if (!optimizedPrompt.trim()) {
      return;
    }
    setPrompt(optimizedPrompt.trim());
    setOptimizedPromptApplied(true);
    setOptimizeError("");
  }

  function dismissOptimizedPrompt() {
    setOptimizedPrompt("");
    setOptimizedPromptApplied(false);
    setOptimizeError("");
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
              参考图 (最多 {maxRefImages} 张)
            </label>
            <span className="text-[10px] text-text-muted">
              {refImages.length}/{maxRefImages}
            </span>
          </div>

          <div className="grid grid-cols-4 gap-2">
            <AnimatePresence>
              {refImages.map((image) => (
                <motion.div
                  key={image.id}
                  initial={{ opacity: 0, scale: 0.8 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.8 }}
                  className="group relative aspect-square rounded-lg border border-border bg-surface-hover"
                >
                  <div className="h-full w-full overflow-hidden rounded-lg">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img src={image.previewUrl} alt={image.fileName} className="h-full w-full object-cover" />
                  </div>
                  <button
                    type="button"
                    onClick={() => removeRefImage(image.id)}
                    className="absolute -right-2 -top-2 z-10 flex h-6 w-6 items-center justify-center rounded-full bg-danger text-white opacity-0 shadow-md transition-opacity group-hover:opacity-100"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </motion.div>
              ))}
            </AnimatePresence>

            {refImages.length < maxRefImages && (
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
            创作提示词
          </label>
          <textarea
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
            placeholder="描述你想要生成的图片内容，例如：一座未来科技城市的黄昏景色，霓虹灯光穿梭..."
            rows={5}
            className="w-full resize-none rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text-primary outline-none transition-all placeholder:text-text-muted focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
          />
          <div className="mt-2 flex items-center justify-between">
            <span className="text-xs text-text-muted">{prompt.length} / 2000</span>
            <div className="flex items-center gap-3">
              <span className="text-[11px] text-text-muted">
                {activeOptimizeModel
                  ? `优化模型：${activeOptimizeModel.modelName}`
                  : chatModelsLoading
                    ? "正在加载优化模型..."
                    : "未配置 AI 优化模型"}
              </span>
              <button
                type="button"
                onClick={() => void handleOptimizePrompt()}
                disabled={!prompt.trim() || !activeOptimizeModel || optimizing}
                className="inline-flex items-center gap-1 rounded-full border border-accent/40 bg-accent/10 px-3 py-1.5 text-xs font-medium text-accent transition-colors hover:bg-accent/15 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {optimizing ? (
                  <div className="h-3.5 w-3.5 animate-spin rounded-full border border-accent/30 border-t-accent" />
                ) : (
                  <Sparkles className="h-3.5 w-3.5" />
                )}
                {optimizing ? "AI 优化中..." : "AI优化"}
              </button>
            </div>
          </div>

          {(optimizing || optimizeError || optimizedPrompt) && (
            <div className="mt-3 rounded-2xl border border-border/60 bg-surface-hover/60 p-3">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-text-primary">AI 优化提示词</p>
                  <p className="mt-1 text-xs text-text-muted">
                    会把当前提示词和参考图发给 AI，生成一版更适合作图的专业描述。
                  </p>
                </div>
                {optimizedPrompt ? (
                  <span
                    className={cn(
                      "rounded-full px-2.5 py-1 text-[11px]",
                      optimizedPromptApplied ? "bg-success/15 text-success" : "bg-accent/15 text-accent",
                    )}
                  >
                    {optimizedPromptApplied ? "已引用" : "待选择"}
                  </span>
                ) : null}
              </div>

              {optimizing ? (
                <div className="mt-3 rounded-xl border border-border/50 bg-surface px-3 py-3">
                  <div className="flex items-center justify-between text-xs text-text-secondary">
                    <span>{optimizeProgress.label}</span>
                    <span>{optimizeProgress.value}%</span>
                  </div>
                  <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-background/60">
                    <div
                      className="h-full rounded-full bg-gradient-to-r from-cyan to-accent transition-all duration-500"
                      style={{ width: `${optimizeProgress.value}%` }}
                    />
                  </div>
                  <p className="mt-2 text-xs text-text-muted">
                    {currentOptimizeJob?.message || optimizeProgress.hint || "AI 正在整理更专业的图片提示词。"}
                  </p>
                </div>
              ) : null}

              {optimizeError ? (
                <div className="mt-3 flex items-start gap-2 rounded-xl border border-danger/30 bg-danger/10 px-3 py-3 text-sm text-danger">
                  <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                  <span>{optimizeError}</span>
                </div>
              ) : null}

              {optimizedPrompt ? (
                <div className="mt-3 space-y-3">
                  <div className="rounded-xl border border-border/50 bg-surface px-3 py-3 text-sm leading-6 text-text-primary">
                    {optimizedPrompt}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <button
                      type="button"
                      onClick={applyOptimizedPrompt}
                      className="rounded-xl bg-accent px-3 py-2 text-sm font-medium text-white transition-transform hover:scale-[1.01]"
                    >
                      {optimizedPromptApplied ? "再次引用到提示词" : "引用到提示词"}
                    </button>
                    <button
                      type="button"
                      onClick={dismissOptimizedPrompt}
                      className="rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text-secondary transition-colors hover:border-border-strong hover:text-text-primary"
                    >
                      保留原文
                    </button>
                    {optimizedPromptApplied ? (
                      <span className="text-xs text-success">提示词已回填，现在可以直接开始构图。</span>
                    ) : null}
                  </div>
                </div>
              ) : null}
            </div>
          )}
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="glass-card p-4"
        >
          <label className="mb-3 block text-xs font-semibold uppercase tracking-wider text-text-muted">
            生成模型
          </label>
          <div className="relative">
            <button
              type="button"
              onClick={() => setIsModelDropdownOpen((open) => !open)}
              className="flex w-full items-center justify-between rounded-xl border border-border bg-surface px-4 py-3 text-sm font-medium transition-all hover:border-accent/50 focus:border-accent"
            >
              <div className="flex items-center gap-2">
                <span className="text-glow text-text-primary">
                  {activeModel?.modelName || (modelsLoading ? "加载中..." : "暂无可用模型")}
                </span>
              </div>
              <ChevronDown
                className={cn("h-4 w-4 text-text-muted transition-transform", isModelDropdownOpen && "rotate-180")}
              />
            </button>

            <AnimatePresence>
              {isModelDropdownOpen && imageModels.length > 0 && (
                <motion.div
                  initial={{ opacity: 0, y: -5 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -5 }}
                  className="absolute left-0 right-0 top-full z-50 mt-2 overflow-hidden rounded-xl border border-border-strong bg-surface-elevated shadow-xl backdrop-blur-3xl"
                >
                  {imageModels.map((model) => (
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
          className="glass-card p-4"
        >
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="mb-2 block text-xs font-semibold uppercase tracking-wider text-text-muted">
                画幅比例
              </label>
              <select
                value={selectedSizeOption?.resolution || ""}
                onChange={(event) => setSelectedSize(event.target.value)}
                className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text-primary outline-none focus:border-accent"
              >
                {sizeOptions.map((size) => (
                  <option key={size.resolution} value={size.resolution}>
                    {size.aspectRatio} ({size.resolution})
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="mb-2 block text-xs font-semibold uppercase tracking-wider text-text-muted">
                生成数量
              </label>
              <select
                value={count}
                onChange={(event) => setCount(Number(event.target.value))}
                className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text-primary outline-none focus:border-accent"
              >
                {[1, 2, 4].map((value) => (
                  <option key={value} value={value}>
                    {value} 张
                  </option>
                ))}
              </select>
            </div>
          </div>
        </motion.div>

        {(currentJob || submitError) && (
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2 }}
            className={cn(
              "glass-card p-4",
              submitError && !currentJob && "border border-danger/30",
            )}
          >
            <div className="mb-3 flex items-center justify-between">
              <span className="text-xs font-semibold uppercase tracking-wider text-text-muted">任务进度</span>
              {currentJob ? (
                <span className="text-[11px] text-text-secondary">{progress.label}</span>
              ) : null}
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
                  {currentJob.message ? (
                    <p className="text-xs text-text-muted">{currentJob.message}</p>
                  ) : null}
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
                <span className="tracking-wider">生成中...</span>
              </div>
            ) : (
              <div className="flex items-center justify-center gap-2 text-white">
                <Wand2 className="h-5 w-5 drop-shadow-[0_0_6px_rgba(255,255,255,0.6)]" />
                <span className="tracking-[0.15em] text-[15px] drop-shadow-[0_0_8px_rgba(255,255,255,0.4)]">
                  开始构图
                </span>
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
                className="group relative h-full w-full bg-black/30"
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={selectedPreviewItem.publicUrl}
                  alt={selectedPreviewItem.fileName || "Preview"}
                  className="h-full w-full object-contain p-2"
                />

                <div className="absolute left-6 top-6 flex items-center gap-2 rounded-lg border border-white/10 bg-black/40 px-3 py-1.5 backdrop-blur-md">
                  <span
                    className={cn(
                      "flex h-2 w-2 rounded-full",
                      isSuccessJob(selectedJob) ? "bg-success pulse-online" : "bg-warning",
                    )}
                  />
                  <span className="text-xs font-medium text-white">
                    {selectedJob ? `${selectedJob.modelName} • ${progress.label}` : "预览结果"}
                  </span>
                </div>

                <div className="absolute bottom-4 left-1/2 flex -translate-x-1/2 items-center gap-2 rounded-2xl border border-white/10 bg-black/40 p-2 opacity-0 shadow-2xl backdrop-blur-xl transition-opacity group-hover:opacity-100">
                  <a
                    href={selectedPreviewItem.publicUrl || "#"}
                    download={selectedPreviewItem.fileName || undefined}
                    target="_blank"
                    rel="noreferrer"
                    className="flex h-9 w-9 items-center justify-center rounded-xl text-white transition-colors hover:bg-white/20"
                    title="下载原图"
                  >
                    <Download className="h-4 w-4" />
                  </a>
                  <div className="h-5 w-px bg-white/20" />
                  <button
                    type="button"
                    onClick={() => void copyPreviewLink()}
                    className="flex h-9 w-9 items-center justify-center rounded-xl text-white transition-colors hover:bg-white/20"
                    title="复制链接"
                  >
                    <Copy className="h-4 w-4" />
                  </button>
                  <div className="min-w-12 text-center text-[10px] text-white/80">
                    {copied ? "已复制" : "复制链接"}
                  </div>
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
                  <div className="absolute inset-0 animate-[spin_4s_linear_infinite] rounded-full border-[1px] border-accent/20" />
                  <div className="absolute inset-2 animate-[spin_2s_ease-in-out_infinite] rounded-full border-2 border-transparent border-t-accent border-b-cyan" />
                  <div className="absolute inset-6 animate-[spin_3s_reverse_infinite] rounded-full border-[1px] border-dashed border-cyan/40" />
                  <Sparkles className="z-10 h-8 w-8 animate-pulse text-accent drop-shadow-[0_0_8px_rgba(177,73,255,0.8)]" />
                </div>
                <div className="space-y-3 text-center">
                  <h3 className="text-glow text-2xl font-black uppercase tracking-[0.2em] text-accent">
                    Rendering
                  </h3>
                  <div className="flex flex-col items-center gap-2">
                    <p className="text-xs tracking-wider text-text-muted">{progress.hint}</p>
                    <div className="h-1 w-56 overflow-hidden rounded-full bg-surface">
                      <div
                        className="h-full rounded-full bg-gradient-to-r from-cyan to-accent shadow-[0_0_10px_rgba(177,73,255,0.5)] transition-all duration-500"
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
                  <h3 className="text-xl font-bold text-text-primary">这次生成失败了</h3>
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
                <ImagePlus className="mb-4 h-16 w-16" />
                <p>等待模型接收指令输出</p>
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
                  "overflow-hidden rounded-xl border-2 transition-all",
                  previewIndex === index ? "border-accent shadow-[0_0_15px_rgba(177,73,255,0.12)]" : "border-border",
                )}
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={item.publicUrl || ""}
                  alt={item.fileName || `preview-${index + 1}`}
                  className="h-20 w-20 object-cover"
                />
              </button>
            ))}
          </div>
        ) : null}
      </div>

      <div className="flex h-full flex-col gap-4 overflow-hidden pb-4 lg:col-span-2 lg:border-l lg:border-border/50 lg:pl-5 xl:col-span-2">
        <h3 className="flex shrink-0 items-center gap-2 border-b border-border/50 pb-3 text-sm font-semibold uppercase tracking-widest text-text-secondary">
          <Layers className="h-4 w-4 text-accent" /> 最近图片任务
        </h3>

        <div className="custom-scrollbar flex-1 space-y-3 overflow-y-auto pr-1">
          {mergedJobs.length === 0 ? (
            <div className="rounded-xl border border-dashed border-border px-4 py-8 text-center text-sm text-text-muted">
              还没有图片生成记录
            </div>
          ) : (
            mergedJobs.map((job) => {
              const stage = resolveAIJobStage(job);
              const thumbnail = pickImageArtifacts(extractImageArtifactsFromPayload(job))[0];
              const isSelected = selectedJob?.id === job.id;

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
                  <div className="relative aspect-square w-full shrink-0 border-b border-border/50 bg-black">
                    {thumbnail?.publicUrl ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img
                        src={thumbnail.publicUrl}
                        className="h-full w-full object-cover opacity-80 transition-opacity group-hover:opacity-100"
                        alt={thumbnail.fileName || buildAIJobTitle(job)}
                      />
                    ) : (
                      <div className="cyber-grid flex h-full w-full items-center justify-center bg-surface-elevated">
                        {isTerminalJob(job) ? (
                          <ImagePlus className="h-6 w-6 text-text-muted/50" />
                        ) : (
                          <div className="h-5 w-5 animate-spin rounded-full border-2 border-accent/30 border-t-accent" />
                        )}
                      </div>
                    )}

                    <div className="absolute bottom-1 right-1 rounded bg-black/80 px-1.5 py-0.5 text-[9px] text-white backdrop-blur">
                      {stage.label}
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
