"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { motion } from "framer-motion";
import {
  AudioLines,
  CheckCircle2,
  CloudUpload,
  History,
  ImagePlus,
  LoaderCircle,
  Maximize2,
  Mic,
  Package,
  RefreshCw,
  ShoppingBag,
  Upload,
  UserRound,
  Video,
  X,
} from "lucide-react";
import { StatusBadge } from "@/components/ui/common";
import {
  DIGITAL_HUMAN_AUDIO_ACCEPT,
  DIGITAL_HUMAN_IMAGE_ACCEPT,
  DIGITAL_HUMAN_POLL_INTERVAL_MS,
  DIGITAL_HUMAN_REMINDER_TEXT,
  countUnicodeCharacters,
  formatDigitalHumanMode,
  isSuccessfulDigitalHumanTask,
  isTerminalDigitalHumanTask,
  validateDigitalHumanFile,
} from "@/lib/digital-human";
import {
  createDigitalHumanTask,
  getDigitalHumanBillingPreview,
  getDigitalHumanTask,
  listDigitalHumanModels,
  listDigitalHumanTasks,
} from "@/lib/services";
import type {
  DigitalHumanBillingPreview,
  DigitalHumanModelsResponse,
  DigitalHumanTask,
} from "@/lib/types";
import { formatDateTime } from "@/lib/workflow";

type FormErrors = Partial<
  Record<"characterImage" | "goodsImage" | "refAudio" | "goodsTitle" | "goodsText" | "modelName", string>
>;

/* ─── Hooks ─── */

function useObjectUrl(file: File | null) {
  const objectURL = useMemo(() => {
    if (!file) {
      return "";
    }
    return URL.createObjectURL(file);
  }, [file]);

  useEffect(() => {
    return () => {
      if (objectURL) {
        URL.revokeObjectURL(objectURL);
      }
    };
  }, [objectURL]);

  return objectURL;
}

/* ─── Helpers ─── */

function formatFileSize(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatCreditValue(value?: number | null) {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "0";
  }
  return value.toLocaleString("zh-CN", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 3,
  });
}

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

function resolveDefaultDigitalHumanModel(
  modelCatalog: DigitalHumanModelsResponse | undefined,
  mode: "digital" | "customize",
) {
  if (!modelCatalog) {
    return "";
  }
  const adminDefault = (modelCatalog.defaultModelByMode?.[mode] || "").trim();
  if (adminDefault) {
    return adminDefault;
  }
  if (modelCatalog.recommendedModelId?.trim()) {
    return modelCatalog.recommendedModelId.trim();
  }
  if (modelCatalog.currentModelId?.trim()) {
    return modelCatalog.currentModelId.trim();
  }
  return modelCatalog.models[0]?.id || "";
}

/* ─── Animation variants ─── */

const staggerContainer = {
  hidden: {},
  show: { transition: { staggerChildren: 0.06 } },
};

const fadeUp = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0, transition: { duration: 0.35, ease: "easeOut" as const } },
};

/* ─── DropZone Component ─── */

function DropZone({
  label,
  icon,
  accept,
  hint,
  kind,
  file,
  previewUrl,
  error,
  onSelect,
}: {
  label: string;
  icon: ReactNode;
  accept: string;
  hint: string;
  kind: "image" | "audio";
  file: File | null;
  previewUrl: string;
  error?: string;
  onSelect: (file: File | null) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [isDragOver, setIsDragOver] = useState(false);
  const [isLightboxOpen, setIsLightboxOpen] = useState(false);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(false);
  }, []);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setIsDragOver(false);
      const droppedFile = e.dataTransfer.files?.[0] || null;
      if (droppedFile) {
        onSelect(droppedFile);
      }
    },
    [onSelect],
  );

  const handleClick = useCallback(() => {
    inputRef.current?.click();
  }, []);

  const handleFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      onSelect(e.target.files?.[0] || null);
      // Reset so the same file can be re-selected
      if (inputRef.current) {
        inputRef.current.value = "";
      }
    },
    [onSelect],
  );

  const handleRemove = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      onSelect(null);
    },
    [onSelect],
  );

  // ─── Has File: show inline preview ───
  if (file && previewUrl) {
    return (
      <div className="group relative">
        <div
          className={`glass-card overflow-hidden p-0 transition-all duration-300 ${
            error ? "border-danger/40 shadow-[0_0_20px_rgba(255,59,92,0.15)]" : ""
          }`}
        >
          {/* Preview area */}
          {kind === "image" ? (
            <>
              <div
                className="relative h-36 cursor-pointer overflow-hidden bg-black/40"
                onClick={(e) => { e.stopPropagation(); setIsLightboxOpen(true); }}
                title="点击查看大图"
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={previewUrl}
                  alt={file.name}
                  className="h-full w-full object-contain p-2"
                />
                {/* Zoom hint on hover */}
                <div className="absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 transition-opacity duration-200 group-hover:opacity-100">
                  <div className="flex h-10 w-10 items-center justify-center rounded-full bg-black/60 text-white backdrop-blur-sm">
                    <Maximize2 className="h-4 w-4" />
                  </div>
                </div>
                {/* Bottom bar */}
                <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/70 to-transparent px-3 py-2">
                  <div className="flex items-center gap-2 text-xs font-medium text-white">
                    {icon}
                    <span>{label}</span>
                    <CheckCircle2 className="ml-auto h-3.5 w-3.5 text-success" />
                  </div>
                </div>
              </div>
              {/* Action buttons row */}
              <div className="flex items-center justify-end gap-1.5 px-3 py-1.5">
                <button
                  type="button"
                  onClick={handleClick}
                  className="flex h-7 w-7 items-center justify-center rounded-full bg-surface-hover text-text-secondary transition-colors hover:bg-accent/20 hover:text-accent"
                  title="更换文件"
                >
                  <RefreshCw className="h-3 w-3" />
                </button>
                <button
                  type="button"
                  onClick={handleRemove}
                  className="flex h-7 w-7 items-center justify-center rounded-full bg-surface-hover text-text-secondary transition-colors hover:bg-danger/20 hover:text-danger"
                  title="删除文件"
                >
                  <X className="h-3 w-3" />
                </button>
              </div>
              {/* Lightbox modal */}
              {isLightboxOpen && (
                <div
                  className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm"
                  onClick={() => setIsLightboxOpen(false)}
                >
                  <button
                    type="button"
                    onClick={() => setIsLightboxOpen(false)}
                    className="absolute right-4 top-4 flex h-10 w-10 items-center justify-center rounded-full bg-white/10 text-white transition-colors hover:bg-white/20"
                  >
                    <X className="h-5 w-5" />
                  </button>
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    src={previewUrl}
                    alt={file.name}
                    className="max-h-[85vh] max-w-[90vw] rounded-lg object-contain shadow-2xl"
                    onClick={(e) => e.stopPropagation()}
                  />
                </div>
              )}
            </>
          ) : (
            <div className="px-4 py-3">
              <div className="flex items-center gap-2">
                <div className="flex items-center gap-2 text-sm font-medium text-text-primary">
                  {icon}
                  <span>{label}</span>
                  <CheckCircle2 className="h-3.5 w-3.5 text-success" />
                </div>
                <audio controls src={previewUrl} className="mx-2 h-8 flex-1" style={{ minWidth: 0 }} />
                <div className="flex shrink-0 gap-1">
                  <button
                    type="button"
                    onClick={handleClick}
                    className="flex h-7 w-7 items-center justify-center rounded-full bg-surface-hover text-text-secondary transition-colors hover:bg-accent/20 hover:text-accent"
                    title="更换文件"
                  >
                    <RefreshCw className="h-3 w-3" />
                  </button>
                  <button
                    type="button"
                    onClick={handleRemove}
                    className="flex h-7 w-7 items-center justify-center rounded-full bg-surface-hover text-text-secondary transition-colors hover:bg-danger/20 hover:text-danger"
                    title="删除文件"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* File meta bar */}
          <div className="border-t border-border/50 px-4 py-2.5">
            <div className="flex items-center gap-2 text-xs text-text-muted">
              <span className="truncate">{file.name}</span>
              <span className="shrink-0 text-text-muted/60">·</span>
              <span className="shrink-0">{formatFileSize(file.size)}</span>
            </div>
          </div>
        </div>

        {error ? <p className="mt-2 text-xs text-danger">{error}</p> : null}

        <input
          ref={inputRef}
          type="file"
          accept={accept}
          className="hidden"
          onChange={handleFileChange}
        />
      </div>
    );
  }

  // ─── Empty: show drop zone ───
  return (
    <div className="relative">
      <button
        type="button"
        onClick={handleClick}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        className={`group/drop w-full cursor-pointer rounded-2xl border-2 border-dashed p-5 text-center transition-all duration-300 ${
          isDragOver
            ? "border-accent bg-accent/10 shadow-[0_0_30px_rgba(177,73,255,0.2)]"
            : error
              ? "border-danger/50 bg-danger/5 hover:border-danger/70"
              : "border-accent/30 bg-accent/[0.03] hover:border-accent/60 hover:bg-accent/[0.06] hover:shadow-[0_0_24px_rgba(177,73,255,0.12)]"
        }`}
      >
        <div
          className={`mx-auto mb-2.5 flex h-14 w-14 items-center justify-center rounded-2xl border transition-all duration-300 ${
            isDragOver
              ? "border-accent/40 bg-accent/20 text-accent scale-110"
              : "border-accent/20 bg-accent/10 text-accent group-hover/drop:border-accent/40 group-hover/drop:bg-accent/15 group-hover/drop:scale-105"
          }`}
        >
          {kind === "image" ? (
            <ImagePlus className="h-6 w-6" />
          ) : (
            <AudioLines className="h-6 w-6" />
          )}
        </div>

        <div className="text-sm font-semibold text-text-primary">{label}</div>
        <p className="mt-1 text-xs text-text-muted">
          拖拽文件到此处
        </p>
        <div className="mx-auto mt-3 inline-flex items-center gap-1.5 rounded-lg border border-accent/40 bg-accent/10 px-3 py-1.5 text-xs font-medium text-accent transition-all group-hover/drop:bg-accent/20 group-hover/drop:border-accent/60">
          <CloudUpload className="h-3.5 w-3.5" />
          点击选择文件
        </div>
        <p className="mt-2 text-[11px] text-text-muted/60">{hint}</p>
      </button>

      {error ? <p className="mt-2 text-xs text-danger">{error}</p> : null}

      <input
        ref={inputRef}
        type="file"
        accept={accept}
        className="hidden"
        onChange={handleFileChange}
      />
    </div>
  );
}

/* ─── Main Page ─── */

export default function DigitalHumanCreationPage() {
  const queryClient = useQueryClient();
  const [mode, setMode] = useState<"digital" | "customize">("digital");
  const [characterImage, setCharacterImage] = useState<File | null>(null);
  const [goodsImage, setGoodsImage] = useState<File | null>(null);
  const [refAudio, setRefAudio] = useState<File | null>(null);
  const [goodsTitle, setGoodsTitle] = useState("");
  const [goodsText, setGoodsText] = useState("");
  const [selectedModelByMode, setSelectedModelByMode] = useState<Record<"digital" | "customize", string>>({
    digital: "",
    customize: "",
  });
  const [errors, setErrors] = useState<FormErrors>({});
  const [submitError, setSubmitError] = useState("");
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [debouncedGoodsText, setDebouncedGoodsText] = useState("");

  const characterPreview = useObjectUrl(characterImage);
  const goodsPreview = useObjectUrl(goodsImage);
  const audioPreview = useObjectUrl(refAudio);

  const titleLength = countUnicodeCharacters(goodsTitle);
  const textLength = countUnicodeCharacters(goodsText);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedGoodsText(goodsText.trim());
    }, 400);
    return () => window.clearTimeout(timer);
  }, [goodsText]);

  const { data: digitalHumanModels, isLoading: digitalHumanModelsLoading } = useQuery<DigitalHumanModelsResponse>({
    queryKey: ["digitalHumanModels"],
    queryFn: () => listDigitalHumanModels(),
  });

  const {
    data: billingPreview,
    error: billingPreviewError,
    isFetching: isBillingPreviewFetching,
  } = useQuery<DigitalHumanBillingPreview>({
    queryKey: ["digitalHumanBillingPreview", debouncedGoodsText],
    queryFn: () => getDigitalHumanBillingPreview(debouncedGoodsText),
    refetchOnWindowFocus: false,
  });

  const { data: recentTasks = [] } = useQuery<DigitalHumanTask[]>({
    queryKey: ["digitalHumanTasks", "recent"],
    queryFn: () => listDigitalHumanTasks({ limit: 6 }),
    refetchInterval: ({ state }) => {
      const tasks = state.data as DigitalHumanTask[] | undefined;
      return tasks?.some((task) => !isTerminalDigitalHumanTask(task))
        ? DIGITAL_HUMAN_POLL_INTERVAL_MS
        : false;
    },
  });

  const resolvedActiveTaskId = activeTaskId ?? recentTasks[0]?.id ?? null;

  const { data: activeTask } = useQuery<DigitalHumanTask>({
    queryKey: ["digitalHumanTask", resolvedActiveTaskId],
    queryFn: () => getDigitalHumanTask(resolvedActiveTaskId as string),
    enabled: Boolean(resolvedActiveTaskId),
    refetchInterval: ({ state }) => {
      const task = state.data as DigitalHumanTask | undefined;
      return task && !isTerminalDigitalHumanTask(task)
        ? DIGITAL_HUMAN_POLL_INTERVAL_MS
        : false;
    },
  });

  const createMutation = useMutation({
    mutationFn: (formData: FormData) => createDigitalHumanTask(formData),
    onSuccess: async (task) => {
      setActiveTaskId(task.id);
      setSubmitError("");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["digitalHumanTasks"] }),
        queryClient.invalidateQueries({ queryKey: ["digitalHumanTasks", "recent"] }),
      ]);
      queryClient.setQueryData(["digitalHumanTask", task.id], task);
    },
  });

  const latestTaskItems = useMemo(
    () => recentTasks.slice(0, 3),
    [recentTasks],
  );
  const hasGoodsText = goodsText.trim().length > 0;
  const selectedModelName =
    selectedModelByMode[mode] || resolveDefaultDigitalHumanModel(digitalHumanModels, mode);
  const selectedModelOption = digitalHumanModels?.models.find((item) => item.id === selectedModelName) ?? null;
  const billingDisabled = Boolean(billingPreview && billingPreview.creditsPerSecond <= 0);
  const billingInsufficient = Boolean(
    hasGoodsText &&
      billingPreview &&
      billingPreview.creditsPerSecond > 0 &&
      !billingPreview.canAfford,
  );
  const submitDisabled = createMutation.isPending || billingDisabled || billingInsufficient || !selectedModelName.trim();

  function handleModeChange(nextMode: "digital" | "customize") {
    setMode(nextMode);
    if (nextMode === "customize") {
      setGoodsImage(null);
      setGoodsTitle("");
      setErrors((current) => ({
        ...current,
        goodsImage: "",
        goodsTitle: "",
      }));
    }
    setErrors((current) => ({
      ...current,
      modelName: "",
    }));
  }

  function validateForm() {
    const nextErrors: FormErrors = {};
    const currentModelName = selectedModelName.trim();

    if (!currentModelName) {
      nextErrors.modelName = "请选择执行模型";
    } else if (
      digitalHumanModels &&
      !digitalHumanModels.models.some((item) => item.id === currentModelName)
    ) {
      nextErrors.modelName = "当前模型已不可用，请重新选择";
    }

    if (!characterImage) {
      nextErrors.characterImage = "请上传人物照片";
    } else {
      const fileError = validateDigitalHumanFile(characterImage, "image");
      if (fileError) {
        nextErrors.characterImage = fileError;
      }
    }

    if (!refAudio) {
      nextErrors.refAudio = "请上传参考语音";
    } else {
      const fileError = validateDigitalHumanFile(refAudio, "audio");
      if (fileError) {
        nextErrors.refAudio = fileError;
      }
    }

    if (mode === "digital") {
      if (!goodsImage) {
        nextErrors.goodsImage = "带货模式必须上传参考产品图";
      } else {
        const fileError = validateDigitalHumanFile(goodsImage, "image");
        if (fileError) {
          nextErrors.goodsImage = fileError;
        }
      }
      if (!goodsTitle.trim()) {
        nextErrors.goodsTitle = "请填写产品标题";
      } else if (titleLength > 20) {
        nextErrors.goodsTitle = "产品标题最多 20 个字";
      }
    }

    if (!goodsText.trim()) {
      nextErrors.goodsText = "请填写文案内容";
    } else if (textLength > 1000) {
      nextErrors.goodsText = "文案内容最多 1000 个字";
    }

    setErrors(nextErrors);
    return Object.keys(nextErrors).length === 0;
  }

  function handleImageSelect(file: File | null, field: "characterImage" | "goodsImage") {
    if (!file) {
      if (field === "characterImage") {
        setCharacterImage(null);
      } else {
        setGoodsImage(null);
      }
      setErrors((current) => ({ ...current, [field]: "" }));
      return;
    }

    const fileError = validateDigitalHumanFile(file, "image");
    if (field === "characterImage") {
      setCharacterImage(fileError ? null : file);
    } else {
      setGoodsImage(fileError ? null : file);
    }
    setErrors((current) => ({ ...current, [field]: fileError }));
  }

  function handleAudioSelect(file: File | null) {
    if (!file) {
      setRefAudio(null);
      setErrors((current) => ({ ...current, refAudio: "" }));
      return;
    }
    const fileError = validateDigitalHumanFile(file, "audio");
    setRefAudio(fileError ? null : file);
    setErrors((current) => ({ ...current, refAudio: fileError }));
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitError("");

    if (!validateForm()) {
      return;
    }
    if (billingDisabled) {
      setSubmitError("数字人计费暂未开放，请稍后再试");
      return;
    }
    if (billingPreview && !billingPreview.canAfford) {
      setSubmitError(`当前积分不足，预计需要 ${formatCreditValue(billingPreview.estimatedCredits)} 积分，还差 ${formatCreditValue(billingPreview.shortfallCredits)} 积分`);
      return;
    }

    const formData = new FormData();
    formData.append("mode", mode);
    formData.append("modelName", selectedModelName.trim());
    formData.append("goodsText", goodsText.trim());
    formData.append("characterImage", characterImage as File);
    formData.append("refAudio", refAudio as File);

    if (mode === "digital") {
      formData.append("goodsTitle", goodsTitle.trim());
      formData.append("goodsImage", goodsImage as File);
    }

    try {
      await createMutation.mutateAsync(formData);
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : "数字人任务创建失败，请稍后重试");
    }
  }

  const shouldShowReminder = createMutation.isPending || Boolean(activeTask);
  const progressValue = clampProgress(activeTask);
  const submitButtonText = createMutation.isPending
    ? "正在提交中..."
    : billingDisabled
      ? "计费未开放"
      : billingInsufficient
        ? "积分不足"
        : "开始制作";

  const modeOptions = [
    {
      key: "digital" as const,
      title: "数字带货",
      icon: <ShoppingBag className="h-4 w-4" />,
    },
    {
      key: "customize" as const,
      title: "自定义口播",
      icon: <Mic className="h-4 w-4" />,
    },
  ];

  return (
    <>
      <div className="grid gap-5 xl:grid-cols-[minmax(0,1.15fr)_420px]">
        {/* ─── Left: Form ─── */}
        <motion.form
          onSubmit={handleSubmit}
          variants={staggerContainer}
          initial="hidden"
          animate="show"
          className="space-y-4"
        >
          {/* ─── Mode Toggle (compact pill slider) ─── */}
          <motion.div variants={fadeUp} className="flex items-center gap-3">
            <span className="text-sm font-medium text-text-secondary">制作模式</span>
            <div className="relative inline-flex rounded-full border border-border/60 bg-surface/50 p-1">
              {/* Sliding indicator */}
              <div
                className="absolute top-1 bottom-1 rounded-full bg-gradient-to-r from-accent to-[#7c3aed] shadow-[0_0_12px_rgba(177,73,255,0.3)] transition-all duration-300"
                style={{
                  left: mode === "digital" ? "4px" : "50%",
                  width: "calc(50% - 4px)",
                }}
              />
              {modeOptions.map((item) => (
                <button
                  key={item.key}
                  type="button"
                  onClick={() => handleModeChange(item.key)}
                  className={`relative z-10 inline-flex items-center gap-1.5 rounded-full px-4 py-1.5 text-sm font-medium transition-colors duration-200 ${
                    mode === item.key
                      ? "text-white"
                      : "text-text-muted hover:text-text-primary"
                  }`}
                >
                  {item.icon}
                  {item.title}
                </button>
              ))}
            </div>
          </motion.div>

          <motion.section variants={fadeUp} className="glass-card p-5">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="text-sm font-semibold text-text-primary">执行模型</div>
                <p className="mt-1 text-xs text-text-muted">
                  {mode === "digital" ? "带货模式默认模型来自后台设置" : "口播模式默认模型来自后台设置"}
                </p>
              </div>
              {selectedModelOption?.isRecommended ? (
                <span className="rounded-full bg-emerald-400/12 px-2.5 py-1 text-xs font-medium text-emerald-300">
                  推荐模型
                </span>
              ) : null}
            </div>
            <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto]">
              <select
                value={selectedModelName}
                onChange={(event) => {
                  const nextValue = event.target.value;
                  setSelectedModelByMode((current) => ({
                    ...current,
                    [mode]: nextValue,
                  }));
                  setErrors((current) => ({ ...current, modelName: "" }));
                }}
                className="w-full rounded-2xl border border-border bg-surface/50 px-4 py-3 text-sm text-text-primary outline-none transition-all focus:border-accent focus:shadow-[0_0_16px_rgba(177,73,255,0.1)]"
                disabled={digitalHumanModelsLoading}
              >
                {!digitalHumanModels?.models.length ? <option value="">暂无可用模型</option> : null}
                {digitalHumanModels?.models.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.id}
                    {item.isRecommended ? "（推荐）" : ""}
                    {item.isCurrent ? "（当前）" : ""}
                  </option>
                ))}
              </select>
              <div className="flex flex-wrap items-center gap-2 text-xs text-text-muted">
                {digitalHumanModels?.provider ? (
                  <span className="rounded-full bg-surface-hover px-2.5 py-1">
                    {digitalHumanModels.provider}
                  </span>
                ) : null}
                {selectedModelOption?.isCurrent ? (
                  <span className="rounded-full bg-cyan/10 px-2.5 py-1 text-cyan">
                    当前服务模型
                  </span>
                ) : null}
              </div>
            </div>
            {errors.modelName ? <p className="mt-2 text-xs text-danger">{errors.modelName}</p> : null}
          </motion.section>

          {/* ─── Upload Section ─── */}
          <motion.section variants={fadeUp} className="glass-card p-5">
            <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-text-primary">
              <CloudUpload className="h-4 w-4 text-accent" />
              素材上传
            </div>

            {/* Images row: character + goods side by side */}
            <div className={`grid gap-4 ${mode === "digital" ? "lg:grid-cols-2" : ""}`}>
              <DropZone
                label="人物照片"
                icon={<UserRound className="h-4 w-4 text-accent" />}
                accept={DIGITAL_HUMAN_IMAGE_ACCEPT}
                hint="支持 JPG / PNG / WebP，用于生成数字人形象"
                kind="image"
                file={characterImage}
                previewUrl={characterPreview}
                error={errors.characterImage}
                onSelect={(f) => handleImageSelect(f, "characterImage")}
              />

              {mode === "digital" ? (
                <DropZone
                  label="参考产品图"
                  icon={<Package className="h-4 w-4 text-accent" />}
                  accept={DIGITAL_HUMAN_IMAGE_ACCEPT}
                  hint="带货模式必填，展示产品外观"
                  kind="image"
                  file={goodsImage}
                  previewUrl={goodsPreview}
                  error={errors.goodsImage}
                  onSelect={(f) => handleImageSelect(f, "goodsImage")}
                />
              ) : null}
            </div>

            {/* Audio: compact single-row */}
            <div className="mt-4">
              <DropZone
                label="参考语音"
                icon={<AudioLines className="h-4 w-4 text-accent" />}
                accept={DIGITAL_HUMAN_AUDIO_ACCEPT}
                hint="支持 M4A / MP3 / WAV / AAC，用于拟合音色"
                kind="audio"
                file={refAudio}
                previewUrl={audioPreview}
                error={errors.refAudio}
                onSelect={handleAudioSelect}
              />
            </div>
          </motion.section>

          {/* ─── Product Title + Script Section (grouped) ─── */}
          <motion.section variants={fadeUp} className="glass-card p-5 space-y-4">
            {mode === "digital" ? (
              <div>
                <label className="flex items-center justify-between text-sm font-medium text-text-primary">
                  <div className="flex items-center gap-2">
                    <Package className="h-4 w-4 text-accent" />
                    <span>产品标题</span>
                  </div>
                  <span
                    className={`rounded-full px-2 py-0.5 text-xs ${
                      titleLength > 20
                        ? "bg-danger/15 text-danger"
                        : "bg-surface-hover text-text-muted"
                    }`}
                  >
                    {titleLength}/20
                  </span>
                </label>
                <input
                  type="text"
                  value={goodsTitle}
                  onChange={(event) => {
                    setGoodsTitle(event.target.value);
                    setErrors((current) => ({ ...current, goodsTitle: "" }));
                  }}
                  placeholder="例如：老廖牌香薰"
                  className="mt-2 w-full rounded-2xl border border-accent/20 bg-accent/[0.03] px-4 py-3 text-sm text-text-primary outline-none transition-all duration-300 focus:border-accent/50 focus:bg-accent/[0.05] focus:shadow-[0_0_16px_rgba(177,73,255,0.1)] placeholder:text-text-muted/50"
                />
                <p className="mt-1.5 text-xs text-text-muted">一句话描述产品，帮助AI理解带货重点。</p>
                {errors.goodsTitle ? (
                  <p className="mt-1.5 text-xs text-danger">{errors.goodsTitle}</p>
                ) : null}
              </div>
            ) : null}

          {/* ─── Script / Text Section (same card as 产品标题 to keep them grouped) ─── */}
            <div>
              <label className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-sm font-semibold text-text-primary">
                  <Upload className="h-4 w-4 text-accent" />
                  口播文案
                </div>
                <span
                  className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                    textLength > 1000
                      ? "bg-danger/15 text-danger"
                      : "bg-surface-hover text-text-muted"
                  }`}
                >
                  {textLength}/1000
                </span>
              </label>
              <textarea
                value={goodsText}
                onChange={(event) => {
                  setGoodsText(event.target.value);
                  setErrors((current) => ({ ...current, goodsText: "" }));
                }}
                rows={5}
                placeholder="请输入数字人口播文案，系统将根据文案内容生成对应时长的口播视频..."
                className="mt-2 w-full resize-y rounded-2xl border border-accent/20 bg-accent/[0.03] px-4 py-3 text-sm leading-7 text-text-primary outline-none transition-all duration-300 focus:border-accent/50 focus:bg-accent/[0.05] focus:shadow-[0_0_16px_rgba(177,73,255,0.1)] placeholder:text-text-muted/50"
                style={{ minHeight: "120px" }}
              />
              <p className="mt-1.5 text-xs text-text-muted">
                文案越长，生成的视频越长。建议精心撰写文案以获得最佳效果。
              </p>
              <div className="mt-3 rounded-2xl border border-accent/15 bg-accent/[0.04] px-4 py-3">
                {billingPreviewError ? (
                  <p className="text-xs text-warning">
                    预计消耗读取失败，提交时服务端仍会重新校验积分。
                  </p>
                ) : billingDisabled ? (
                  <div className="space-y-1">
                    <p className="text-sm font-medium text-warning">暂未开放计费配置</p>
                    <p className="text-xs text-text-muted">
                      管理后台尚未设置数字人视频每秒积分，当前无法提交任务。
                    </p>
                  </div>
                ) : hasGoodsText && billingPreview ? (
                  <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-text-secondary">
                    <span>
                      预计时长 <span className="font-semibold text-text-primary">{billingPreview.estimatedDurationSeconds}</span> 秒
                    </span>
                    <span>
                      预计消耗 <span className="font-semibold text-accent">{formatCreditValue(billingPreview.estimatedCredits)}</span> 积分
                    </span>
                    <span>
                      当前余额 <span className="font-semibold text-text-primary">{formatCreditValue(billingPreview.creditBalance)}</span> 积分
                    </span>
                    <span className="text-xs text-text-muted">按 4 字/秒估算，实际结算以成品视频时长为准</span>
                    {billingInsufficient ? (
                      <span className="rounded-full bg-danger/10 px-2 py-0.5 text-xs font-medium text-danger">
                        余额不足，还差 {formatCreditValue(billingPreview.shortfallCredits)} 积分
                      </span>
                    ) : (
                      <span className="rounded-full bg-success/10 px-2 py-0.5 text-xs font-medium text-success">
                        积分充足
                      </span>
                    )}
                  </div>
                ) : (
                  <div className="flex flex-wrap items-center gap-3 text-xs text-text-muted">
                    <span>
                      当前费率 {formatCreditValue(billingPreview?.creditsPerSecond ?? 0)} 积分/秒
                    </span>
                    <span>按 4 字/秒估算，输入文案后将展示预计消耗</span>
                    {isBillingPreviewFetching ? <span className="text-accent">正在计算...</span> : null}
                  </div>
                )}
              </div>
              {errors.goodsText ? <p className="mt-1.5 text-xs text-danger">{errors.goodsText}</p> : null}
            </div>
          </motion.section>

          {/* ─── Submit ─── */}
          <motion.div variants={fadeUp} className="flex flex-wrap items-center gap-4">
            <button
              type="submit"
              disabled={submitDisabled}
              className={`group relative inline-flex items-center gap-2.5 overflow-hidden rounded-2xl px-6 py-3.5 text-sm font-semibold text-white transition-all duration-300 ${
                submitDisabled
                  ? "bg-gradient-to-r from-accent/60 to-[#7c3aed]/60 cursor-not-allowed shadow-none"
                  : "bg-gradient-to-r from-accent to-[#7c3aed] shadow-[0_0_24px_rgba(177,73,255,0.25)] hover:shadow-[0_0_36px_rgba(177,73,255,0.35)] hover:brightness-110"
              }`}
            >
              {/* Shimmer animation while loading */}
              {createMutation.isPending ? (
                <div className="absolute inset-0 overflow-hidden">
                  <div className="absolute inset-0 -translate-x-full animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-transparent via-white/15 to-transparent" />
                </div>
              ) : (
                <div className="absolute inset-0 bg-gradient-to-r from-cyan/20 to-accent/20 opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
              )}
              <span className="relative flex items-center gap-2">
                {createMutation.isPending ? (
                  <LoaderCircle className="h-4.5 w-4.5 animate-spin" />
                ) : (
                  <Video className="h-4.5 w-4.5" />
                )}
                {submitButtonText}
              </span>
            </button>
            {createMutation.isPending ? (
              <span className="text-xs text-accent animate-pulse">
                正在上传素材并启动任务，请勿关闭页面...
              </span>
            ) : billingDisabled ? (
              <span className="text-xs text-warning">
                后台尚未配置数字人视频每秒积分，暂时无法提交。
              </span>
            ) : billingInsufficient && billingPreview ? (
              <span className="text-xs text-danger">
                当前积分不足，还差 {formatCreditValue(billingPreview.shortfallCredits)} 积分。
              </span>
            ) : (
              <span className="text-xs text-text-muted">
                素材将上传至云端后自动启动数字人视频生成
              </span>
            )}
          </motion.div>

          {submitError ? (
            <motion.div
              initial={{ opacity: 0, y: -8 }}
              animate={{ opacity: 1, y: 0 }}
              className="rounded-2xl border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger"
            >
              {submitError}
            </motion.div>
          ) : null}
        </motion.form>

        {/* ─── Right: Tasks Panel ─── */}
        <motion.div
          variants={staggerContainer}
          initial="hidden"
          animate="show"
          className="space-y-4"
        >
          {/* Current Task */}
          <motion.section variants={fadeUp} className="glass-card-elevated p-5">
            <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-text-primary">
              <Video className="h-4 w-4 text-accent" />
              当前任务
            </div>

            {shouldShowReminder ? (
              <div className="mt-4 rounded-2xl border border-warning/20 bg-warning/8 px-4 py-3 text-sm leading-6 text-text-secondary">
                <span className="mr-1.5 text-warning">💡</span>
                {DIGITAL_HUMAN_REMINDER_TEXT}
              </div>
            ) : null}

            {activeTask ? (
              <div className="mt-5 space-y-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="text-sm font-semibold text-text-primary">
                      {activeTask.goodsTitle?.trim() || "自定义口播任务"}
                    </p>
                    <p className="mt-1 text-xs text-text-muted">
                      {formatDigitalHumanMode(activeTask.mode)} · {activeTask.modelName || "未记录模型"} · {formatDateTime(activeTask.updatedAt)}
                    </p>
                  </div>
                  <StatusBadge status={activeTask.status} />
                </div>

                {/* Progress */}
                <div>
                  <div className="flex items-center justify-between text-xs text-text-secondary">
                    <span>{activeTask.progress?.message || "等待生成进度..."}</span>
                    <span className="font-mono font-medium text-accent">{progressValue.toFixed(0)}%</span>
                  </div>
                  <div className="mt-2 h-2 overflow-hidden rounded-full bg-background/60">
                    <motion.div
                      className="h-full rounded-full bg-gradient-to-r from-accent to-cyan"
                      initial={{ width: 0 }}
                      animate={{ width: `${progressValue}%` }}
                      transition={{ duration: 0.5, ease: "easeOut" }}
                      style={{
                        boxShadow: progressValue > 0 ? "0 0 12px rgba(177,73,255,0.4)" : "none",
                      }}
                    />
                  </div>
                </div>

                {activeTask.errorMessage ? (
                  <div className="rounded-2xl border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">
                    {activeTask.errorMessage}
                  </div>
                ) : null}

                <div className="flex flex-wrap items-center gap-2.5">
                  <Link
                    href={`/creation/digital-human/${activeTask.id}`}
                    className="btn-neon inline-flex items-center gap-2 rounded-xl border border-border px-3.5 py-2 text-sm font-medium text-text-primary transition-all hover:border-accent hover:text-accent"
                  >
                    查看详情
                  </Link>
                  {activeTask.resultAsset?.publicUrl ? (
                    <a
                      href={activeTask.resultAsset.publicUrl}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-accent/20 to-cyan/20 px-3.5 py-2 text-sm font-medium text-accent transition-all hover:from-accent/30 hover:to-cyan/30"
                    >
                      <CloudUpload className="h-3.5 w-3.5" />
                      下载成品
                    </a>
                  ) : null}
                </div>
              </div>
            ) : (
              <div className="mt-5 flex flex-col items-center justify-center rounded-2xl border border-dashed border-border/60 py-10 text-center">
                <div className="mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-accent/10 text-accent/60">
                  <Video className="h-5 w-5" />
                </div>
                <p className="text-sm text-text-muted">提交任务后，将在这里跟踪生成进度</p>
              </div>
            )}
          </motion.section>

          {/* Recent Tasks */}
          <motion.section variants={fadeUp} className="glass-card p-5">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 text-sm font-semibold text-text-primary">
                <History className="h-4 w-4 text-accent" />
                最近任务
              </div>
              {latestTaskItems.length > 0 && (
                <Link
                  href="/creation/digital-human/history"
                  className="text-xs text-text-muted transition-colors hover:text-accent"
                >
                  查看全部 →
                </Link>
              )}
            </div>

            <div className="mt-4 space-y-2.5">
              {latestTaskItems.length > 0 ? (
                latestTaskItems.map((task, i) => (
                  <motion.button
                    key={task.id}
                    type="button"
                    initial={{ opacity: 0, x: 8 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: i * 0.05, duration: 0.25 }}
                    whileHover={{ x: 2 }}
                    onClick={() => setActiveTaskId(task.id)}
                    className={`group w-full rounded-xl border px-4 py-3 text-left transition-all duration-200 ${
                      resolvedActiveTaskId === task.id
                        ? "border-accent/40 bg-accent/8 shadow-[0_0_16px_rgba(177,73,255,0.08)]"
                        : "border-border/50 bg-surface/30 hover:border-accent/25 hover:bg-surface/50"
                    }`}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-medium text-text-primary">
                          {task.goodsTitle?.trim() || "自定义口播任务"}
                        </p>
                        <p className="mt-0.5 truncate text-xs text-text-muted">
                          {formatDigitalHumanMode(task.mode)} · {task.modelName || "未记录模型"} · {formatDateTime(task.updatedAt)}
                        </p>
                      </div>
                      <StatusBadge status={task.status} />
                    </div>
                  </motion.button>
                ))
              ) : (
                <div className="rounded-xl border border-dashed border-border/60 py-8 text-center text-sm text-text-muted">
                  还没有数字人任务记录
                </div>
              )}
            </div>
          </motion.section>
        </motion.div>
      </div>
    </>
  );
}
