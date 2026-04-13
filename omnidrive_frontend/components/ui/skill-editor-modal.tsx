"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AudioLines,
  ArrowDown,
  ArrowUp,
  Check,
  ChevronDown,
  ChevronUp,
  Cpu,
  FileText,
  Image as ImageIcon,
  Loader2,
  MessageSquareText,
  Mic,
  Package,
  Sparkles,
  Trash2,
  Upload,
  UserRound,
  Video,
  Wand2,
  X,
} from "lucide-react";
import {
  createSkill,
  deleteSkill,
  deleteSkillAsset,
  getSkillEditorDefaults,
  listAIModels,
  listDigitalHumanModels,
  listSkillAssets,
  updateSkill,
  uploadSkillAsset,
} from "@/lib/services";
import type {
  AIModel,
  DigitalHumanModelsResponse,
  Skill,
  SkillAsset,
  SkillEditorDefaults,
} from "@/lib/types";
import { getModelDisplayName } from "@/lib/model-display";
import { buildFileAccept, resolveSupportedFileTypes } from "@/lib/ai-file-types";
import { cn } from "@/lib/utils";
import {
  getModelReferenceLimit,
  isDigitalHumanSkillOutput,
  mapSkillOutputToModelCategory,
  normalizeSkillOutputLabel,
} from "@/lib/workflow";

type SkillEditorModalProps = {
  isOpen: boolean;
  deviceId: string;
  skill?: Skill | null;
  onClose: () => void;
  onSaved: () => void;
};

type SkillFormState = {
  name: string;
  description: string;
  promptTemplate: string;
  fixedDurationSeconds: string;
  publishIntroEnabled: boolean;
  topicsText: string;
  coverPromptTemplate: string;
  coverPromptUsesSystemDefault: boolean;
  outputType: string;
  modelName: string;
  digitalHumanMode: "digital" | "customize";
  digitalHumanGoodsTitle: string;
  digitalHumanGoodsText: string;
  storyboardEnabled: boolean;
  isEnabled: boolean;
};

type UploadAssetType =
  | "reference_image"
  | "reference_video"
  | "reference_text"
  | "digital_human_character_image"
  | "digital_human_goods_image"
  | "digital_human_ref_audio";

type UploadingAsset = {
  id: string;
  assetType: UploadAssetType;
  file: File;
};

type OutputOption = {
  value: SkillFormState["outputType"];
  label: string;
  hint: string;
  icon: React.ComponentType<{ className?: string }>;
  tone: string;
};

type SkillVideoDurationPreset = {
  seconds: number;
  label: string;
  specialPriceCredits?: number | null;
};

const OUTPUT_OPTIONS: OutputOption[] = [
  {
    value: "图文模式",
    label: "图文模式",
    hint: "海报、封面、种草图文",
    icon: ImageIcon,
    tone: "text-cyan",
  },
  {
    value: "视文模式",
    label: "视文模式",
    hint: "短视频、剧情、动态镜头",
    icon: Sparkles,
    tone: "text-accent",
  },
  {
    value: "文本格式",
    label: "文本格式",
    hint: "脚本、标题、文案",
    icon: MessageSquareText,
    tone: "text-amber-200",
  },
  {
    value: "真人口播",
    label: "真人口播",
    hint: "带货口播、人物口播视频",
    icon: Mic,
    tone: "text-emerald-300",
  },
];

const EMPTY_FORM: SkillFormState = {
  name: "",
  description: "",
  promptTemplate: "",
  fixedDurationSeconds: "",
  publishIntroEnabled: true,
  topicsText: "",
  coverPromptTemplate: "",
  coverPromptUsesSystemDefault: true,
  outputType: "图文模式",
  modelName: "",
  digitalHumanMode: "digital",
  digitalHumanGoodsTitle: "",
  digitalHumanGoodsText: "",
  storyboardEnabled: true,
  isEnabled: true,
};

function parseDigitalHumanConfig(referencePayload: Record<string, unknown> | null | undefined) {
  const base =
    referencePayload?.digitalHuman && typeof referencePayload.digitalHuman === "object"
      ? (referencePayload.digitalHuman as Record<string, unknown>)
      : referencePayload || {};
  const rawMode = typeof base.mode === "string" ? base.mode.trim() : "";
  return {
    mode: rawMode === "customize" ? "customize" : "digital",
    goodsTitle: typeof base.goodsTitle === "string" ? base.goodsTitle : "",
    goodsText: typeof base.goodsText === "string" ? base.goodsText : "",
  } as const;
}

function buildSkillFormState(skill?: Skill | null, coverPromptDefault = ""): SkillFormState {
  if (!skill) {
    return {
      ...EMPTY_FORM,
      coverPromptTemplate: coverPromptDefault,
      coverPromptUsesSystemDefault: true,
    };
  }
  const customCoverPrompt = (skill.coverPromptTemplate || "").trim();
  const digitalHumanConfig = parseDigitalHumanConfig(skill.referencePayload);
  return {
    name: skill.name || "",
    description: skill.description || "",
    promptTemplate: skill.promptTemplate || "",
    fixedDurationSeconds:
      typeof skill.fixedDurationSeconds === "number" && skill.fixedDurationSeconds > 0
        ? String(skill.fixedDurationSeconds)
        : "",
    publishIntroEnabled: skill.publishIntroEnabled !== false,
    topicsText: (skill.topics || []).join("，"),
    coverPromptTemplate: customCoverPrompt || coverPromptDefault,
    coverPromptUsesSystemDefault: customCoverPrompt.length === 0,
    outputType: normalizeSkillOutputLabel(skill.outputType),
    modelName: skill.modelName || "",
    digitalHumanMode: digitalHumanConfig.mode,
    digitalHumanGoodsTitle: digitalHumanConfig.goodsTitle,
    digitalHumanGoodsText: digitalHumanConfig.goodsText,
    storyboardEnabled: skill.storyboardEnabled !== false,
    isEnabled: Boolean(skill.isEnabled),
  };
}

const DEFAULT_VIDEO_TASK_NOTE = "默认不要字幕";
const DEFAULT_SKILL_VIDEO_BASE_SECONDS = 8;
const DEFAULT_SKILL_VIDEO_PRESET_MULTIPLIER_COUNT = 8;

function isImageAsset(asset: SkillAsset) {
  return (asset.mimeType || "").startsWith("image/") || asset.assetType.includes("image");
}

function isTextAsset(asset: SkillAsset) {
  const mimeType = (asset.mimeType || "").toLowerCase();
  return mimeType.startsWith("text/") || asset.assetType.includes("text");
}

function isVideoAsset(asset: SkillAsset) {
  return (asset.mimeType || "").startsWith("video/") || asset.assetType.includes("video");
}

function isReferenceMediaAsset(asset: SkillAsset) {
  return asset.assetType === "reference_image" || asset.assetType === "reference_video";
}

function hasSupportedFilePrefix(values: string[], prefix: string) {
  return values.some((value) => value === `${prefix}*` || value.startsWith(prefix));
}

function buildAcceptByPrefix(values: string[], prefix: string) {
  return buildFileAccept(values.filter((value) => value === `${prefix}*` || value.startsWith(prefix)));
}

function normalizeReferenceMediaOrder(value: unknown) {
  if (!Array.isArray(value)) {
    return [] as string[];
  }
  const seen = new Set<string>();
  const result: string[] = [];
  value.forEach((item) => {
    if (typeof item !== "string") {
      return;
    }
    const trimmed = item.trim();
    if (!trimmed || seen.has(trimmed)) {
      return;
    }
    seen.add(trimmed);
    result.push(trimmed);
  });
  return result;
}

function mergeReferenceMediaOrder(order: string[], assets: SkillAsset[]) {
  const seen = new Set<string>();
  const assetIds = new Set(assets.map((asset) => asset.id));
  const merged: string[] = [];
  order.forEach((id) => {
    if (!assetIds.has(id) || seen.has(id)) {
      return;
    }
    seen.add(id);
    merged.push(id);
  });
  assets.forEach((asset) => {
    if (seen.has(asset.id)) {
      return;
    }
    seen.add(asset.id);
    merged.push(asset.id);
  });
  return merged;
}

function sortMediaAssetsByOrder(assets: SkillAsset[], order: string[]) {
  const mergedOrder = mergeReferenceMediaOrder(order, assets);
  const orderIndex = new Map(mergedOrder.map((id, index) => [id, index]));
  return [...assets].sort((left, right) => {
    const leftIndex = orderIndex.get(left.id);
    const rightIndex = orderIndex.get(right.id);
    if (typeof leftIndex === "number" && typeof rightIndex === "number" && leftIndex !== rightIndex) {
      return leftIndex - rightIndex;
    }
    return new Date(left.createdAt).getTime() - new Date(right.createdAt).getTime();
  });
}

function buildReferencePayload(
  basePayload: Record<string, unknown> | null | undefined,
  referenceMediaOrder: string[],
  digitalHumanConfig?: {
    mode: "digital" | "customize";
    goodsTitle: string;
    goodsText: string;
  } | null,
) {
  const nextPayload: Record<string, unknown> = {
    ...(basePayload || {}),
  };
  delete nextPayload.mode;
  delete nextPayload.goodsTitle;
  delete nextPayload.goodsText;
  if (digitalHumanConfig) {
    delete nextPayload.referenceMediaOrder;
    nextPayload.digitalHuman = {
      mode: digitalHumanConfig.mode,
      goodsTitle: digitalHumanConfig.goodsTitle.trim(),
      goodsText: digitalHumanConfig.goodsText.trim(),
    };
  } else {
    delete nextPayload.digitalHuman;
    if (referenceMediaOrder.length > 0) {
      nextPayload.referenceMediaOrder = referenceMediaOrder;
    } else {
      delete nextPayload.referenceMediaOrder;
    }
  }
  return Object.keys(nextPayload).length > 0 ? nextPayload : null;
}

function describeSupportedMediaTypes(supportsImages: boolean, supportsVideos: boolean) {
  if (supportsImages && supportsVideos) {
    return "图片 / 视频";
  }
  if (supportsVideos) {
    return "仅视频";
  }
  if (supportsImages) {
    return "仅图片";
  }
  return "不支持";
}

function getSkillBillingAmount(model?: AIModel | null) {
  const amount = typeof model?.billingAmount === "number" ? model.billingAmount : model?.rawRate;
  if (typeof amount !== "number" || Number.isNaN(amount)) {
    return null;
  }
  return amount;
}

function formatSkillBillingAmount(model?: AIModel | null) {
  const amount = getSkillBillingAmount(model);
  if (amount === null) {
    return "待配置";
  }
  return `${amount.toFixed(2)} 积分 / 次`;
}

function formatBillingMode(mode?: string | null) {
  if (!mode) return "待配置";
  const map: Record<string, string> = {
    per_call: "按次计费",
    per_second: "按秒计费",
    per_token: "按 Token 计费",
    per_image: "按图计费",
  };
  return map[mode] || mode;
}

function getVendorColor(vendor?: string | null) {
  if (!vendor) return { border: "border-white/15", bg: "bg-white/8", text: "text-text-secondary" };
  const v = vendor.toLowerCase();
  if (v.includes("apiyi")) return { border: "border-sky-400/25", bg: "bg-sky-400/10", text: "text-sky-300" };
  if (v.includes("iconai")) return { border: "border-emerald-400/25", bg: "bg-emerald-400/10", text: "text-emerald-300" };
  if (v.includes("google")) return { border: "border-blue-400/25", bg: "bg-blue-400/10", text: "text-blue-300" };
  if (v.includes("openai")) return { border: "border-green-400/25", bg: "bg-green-400/10", text: "text-green-300" };
  return { border: "border-white/15", bg: "bg-white/8", text: "text-text-secondary" };
}

function isVideoTextOutput(outputType: string) {
  return normalizeSkillOutputLabel(outputType) === "视文模式";
}

function isDigitalHumanOutput(outputType: string) {
  return isDigitalHumanSkillOutput(outputType);
}

function resolveDigitalHumanDefaultModel(
  modelCatalog: DigitalHumanModelsResponse | undefined,
  mode: "digital" | "customize",
) {
  if (!modelCatalog) {
    return "";
  }
  const defaultModel = (modelCatalog.defaultModelByMode?.[mode] || "").trim();
  if (defaultModel) {
    return defaultModel;
  }
  if (modelCatalog.recommendedModelId?.trim()) {
    return modelCatalog.recommendedModelId.trim();
  }
  if (modelCatalog.currentModelId?.trim()) {
    return modelCatalog.currentModelId.trim();
  }
  return modelCatalog.models[0]?.id || "";
}

function parseSkillVideoDurationSeconds(value: string) {
  const match = value
    .trim()
    .toLowerCase()
    .match(/^(\d+)\s*s?$/);
  if (!match) {
    return null;
  }
  const seconds = Number(match[1]);
  return seconds > 0 ? seconds : null;
}

function resolveSkillVideoBaseSeconds(model?: AIModel | null) {
  const supportedDurations = model?.videoSupportedDurations || [];
  for (const item of supportedDurations) {
    const seconds = parseSkillVideoDurationSeconds(item);
    if (seconds) {
      return seconds;
    }
  }
  return DEFAULT_SKILL_VIDEO_BASE_SECONDS;
}

function buildSkillVideoDurationPresets(
  baseSeconds: number,
  durationRules: SkillEditorDefaults["videoTextDurationOptions"] = [],
) {
  const step = baseSeconds > 0 ? baseSeconds : DEFAULT_SKILL_VIDEO_BASE_SECONDS;
  const matchedRules = durationRules.filter((item) => item.durationSeconds > 0 && item.durationSeconds % step === 0);
  const secondsSet = new Set<number>();
  const presets: SkillVideoDurationPreset[] = [];

  for (let multiplier = 1; multiplier <= DEFAULT_SKILL_VIDEO_PRESET_MULTIPLIER_COUNT; multiplier += 1) {
    const seconds = step * multiplier;
    secondsSet.add(seconds);
  }
  matchedRules.forEach((item) => secondsSet.add(item.durationSeconds));

  Array.from(secondsSet)
    .sort((left, right) => left - right)
    .forEach((seconds) => {
      const matchedRule = matchedRules.find((item) => item.durationSeconds === seconds) ?? null;
      presets.push({
        seconds,
        label: `${seconds}s`,
        specialPriceCredits: matchedRule?.specialPriceCredits ?? null,
      });
    });

  return presets;
}

export function SkillEditorModal({
  isOpen,
  deviceId,
  skill,
  onClose,
  onSaved,
}: SkillEditorModalProps) {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<SkillFormState>(() => buildSkillFormState(skill));
  const [draftSkillId, setDraftSkillId] = useState<string | null>(null);
  const [draftNeedsCleanup, setDraftNeedsCleanup] = useState(false);
  const [uploadingAssets, setUploadingAssets] = useState<UploadingAsset[]>([]);
  const [referenceMediaOrder, setReferenceMediaOrder] = useState<string[]>([]);
  const [coverPromptWarningOpen, setCoverPromptWarningOpen] = useState(false);
  const [coverPromptUnlockCountdown, setCoverPromptUnlockCountdown] = useState(0);
  const [coverPromptUnlocked, setCoverPromptUnlocked] = useState(false);
  const [digitalHumanModelExpanded, setDigitalHumanModelExpanded] = useState(false);
  const draftCreationRef = useRef<Promise<Skill> | null>(null);
  const currentSkillId = skill?.id ?? draftSkillId;
  const isDigitalHumanOutputType = isDigitalHumanOutput(form.outputType);

  const modelCategory = mapSkillOutputToModelCategory(form.outputType);
  const { data: models = [], isLoading: modelsLoading } = useQuery<AIModel[]>({
    queryKey: ["aiModels", modelCategory],
    queryFn: () => listAIModels({ modelType: modelCategory }),
    enabled: isOpen && !isDigitalHumanOutputType,
  });
  const { data: digitalHumanModels, isLoading: digitalHumanModelsLoading } = useQuery<DigitalHumanModelsResponse>({
    queryKey: ["digitalHumanModels"],
    queryFn: () => listDigitalHumanModels(),
    enabled: isOpen && isDigitalHumanOutputType,
  });

  const { data: assets = [], isLoading: assetsLoading } = useQuery<SkillAsset[]>({
    queryKey: ["skillAssets", currentSkillId],
    queryFn: () => listSkillAssets(currentSkillId!),
    enabled: isOpen && Boolean(currentSkillId),
  });
  const { data: skillEditorDefaults } = useQuery<SkillEditorDefaults>({
    queryKey: ["skillEditorDefaults"],
    queryFn: () => getSkillEditorDefaults(),
    enabled: isOpen,
  });
  const coverPromptDefault = (skillEditorDefaults?.coverPromptTemplateDefault || "").trim();
  const videoTextDurationOptions = useMemo(
    () => skillEditorDefaults?.videoTextDurationOptions || [],
    [skillEditorDefaults?.videoTextDurationOptions],
  );

  const availableModels = useMemo(
    () => models.filter((item) => item.isEnabled && item.category === modelCategory),
    [modelCategory, models],
  );
  const availableDigitalHumanModels = useMemo(
    () => digitalHumanModels?.models || [],
    [digitalHumanModels?.models],
  );
  const selectedModel = useMemo(
    () => availableModels.find((item) => item.modelName === form.modelName) ?? null,
    [availableModels, form.modelName],
  );
  const selectedDigitalHumanModel = useMemo(
    () => availableDigitalHumanModels.find((item) => item.id === form.modelName) ?? null,
    [availableDigitalHumanModels, form.modelName],
  );
  const selectedOutput = useMemo(
    () => OUTPUT_OPTIONS.find((item) => item.value === form.outputType) ?? OUTPUT_OPTIONS[0],
    [form.outputType],
  );
  const selectedVideoBaseSeconds = useMemo(
    () => resolveSkillVideoBaseSeconds(selectedModel),
    [selectedModel],
  );
  const videoDurationPresets = useMemo(
    () => buildSkillVideoDurationPresets(selectedVideoBaseSeconds, videoTextDurationOptions),
    [selectedVideoBaseSeconds, videoTextDurationOptions],
  );
  const matchedDurationRule = useMemo(() => {
    const durationSeconds = Number(form.fixedDurationSeconds);
    if (!Number.isFinite(durationSeconds) || durationSeconds <= 0) {
      return null;
    }
    return videoTextDurationOptions.find((item) => item.durationSeconds === durationSeconds) ?? null;
  }, [form.fixedDurationSeconds, videoTextDurationOptions]);

  const mediaAssets = useMemo(() => assets.filter(isReferenceMediaAsset), [assets]);
  const imageAssets = useMemo(() => mediaAssets.filter(isImageAsset), [mediaAssets]);
  const videoAssets = useMemo(() => mediaAssets.filter(isVideoAsset), [mediaAssets]);
  const textAssets = useMemo(() => assets.filter((asset) => asset.assetType === "reference_text" || isTextAsset(asset)), [assets]);
  const digitalHumanCharacterAssets = useMemo(
    () => assets.filter((asset) => asset.assetType === "digital_human_character_image"),
    [assets],
  );
  const digitalHumanGoodsAssets = useMemo(
    () => assets.filter((asset) => asset.assetType === "digital_human_goods_image"),
    [assets],
  );
  const digitalHumanAudioAssets = useMemo(
    () => assets.filter((asset) => asset.assetType === "digital_human_ref_audio"),
    [assets],
  );
  const uploadingImages = useMemo(
    () => uploadingAssets.filter((item) => item.assetType === "reference_image"),
    [uploadingAssets],
  );
  const uploadingVideos = useMemo(
    () => uploadingAssets.filter((item) => item.assetType === "reference_video"),
    [uploadingAssets],
  );
  const uploadingMedia = useMemo(
    () => uploadingAssets.filter((item) => item.assetType !== "reference_text"),
    [uploadingAssets],
  );
  const uploadingTexts = useMemo(
    () => uploadingAssets.filter((item) => item.assetType === "reference_text"),
    [uploadingAssets],
  );
  const uploadingDigitalHumanCharacters = useMemo(
    () => uploadingAssets.filter((item) => item.assetType === "digital_human_character_image"),
    [uploadingAssets],
  );
  const uploadingDigitalHumanGoods = useMemo(
    () => uploadingAssets.filter((item) => item.assetType === "digital_human_goods_image"),
    [uploadingAssets],
  );
  const uploadingDigitalHumanAudios = useMemo(
    () => uploadingAssets.filter((item) => item.assetType === "digital_human_ref_audio"),
    [uploadingAssets],
  );
  const orderedMediaAssets = useMemo(
    () => sortMediaAssetsByOrder(mediaAssets, referenceMediaOrder),
    [mediaAssets, referenceMediaOrder],
  );
  const supportedFileTypes = useMemo(
    () => resolveSupportedFileTypes(selectedModel ?? undefined),
    [selectedModel],
  );
  const supportsReferenceImages = hasSupportedFilePrefix(supportedFileTypes, "image/");
  const supportsReferenceVideos = hasSupportedFilePrefix(supportedFileTypes, "video/");
  const imageAccept = buildAcceptByPrefix(supportedFileTypes, "image/");
  const videoAccept = buildAcceptByPrefix(supportedFileTypes, "video/");
  const mediaLimit = getModelReferenceLimit(form.outputType, selectedModel ?? undefined);
  const totalImageCount = imageAssets.length + uploadingImages.length;
  const totalVideoCount = videoAssets.length + uploadingVideos.length;
  const totalMediaCount = orderedMediaAssets.length + uploadingMedia.length;
  const totalTextCount = textAssets.length + uploadingTexts.length;
  const visibleModelName = isDigitalHumanOutputType
    ? selectedDigitalHumanModel?.id || form.modelName || "未选择"
    : getModelDisplayName(selectedModel, "未选择");

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    setForm(buildSkillFormState(skill, coverPromptDefault));
    setDraftSkillId(null);
    setDraftNeedsCleanup(false);
    setUploadingAssets([]);
    setReferenceMediaOrder(normalizeReferenceMediaOrder(skill?.referencePayload?.referenceMediaOrder));
    setCoverPromptWarningOpen(false);
    setCoverPromptUnlockCountdown(0);
    setCoverPromptUnlocked(false);
    draftCreationRef.current = null;
    // `coverPromptDefault` is hydrated separately below so late-loaded defaults do not clobber in-progress edits.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, skill]);

  useEffect(() => {
    if (!isOpen || !coverPromptDefault) {
      return;
    }
    setForm((current) => {
      if (!current.coverPromptUsesSystemDefault) {
        return current;
      }
      if (current.coverPromptTemplate === coverPromptDefault) {
        return current;
      }
      return {
        ...current,
        coverPromptTemplate: coverPromptDefault,
      };
    });
  }, [isOpen, coverPromptDefault]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    setReferenceMediaOrder((current) => {
      const next = mergeReferenceMediaOrder(current, mediaAssets);
      if (next.length === current.length && next.every((item, index) => item === current[index])) {
        return current;
      }
      return next;
    });
  }, [isOpen, mediaAssets]);

  useEffect(() => {
    if (!isOpen || !isVideoTextOutput(form.outputType)) {
      return;
    }
    setForm((current) => {
      if (!isVideoTextOutput(current.outputType)) {
        return current;
      }
      const durationSeconds = Number(current.fixedDurationSeconds);
      if (Number.isFinite(durationSeconds) && durationSeconds > 0 && durationSeconds % selectedVideoBaseSeconds === 0) {
        return current;
      }
      return {
        ...current,
        fixedDurationSeconds: String(selectedVideoBaseSeconds),
      };
    });
  }, [form.outputType, isOpen, selectedVideoBaseSeconds]);

  useEffect(() => {
    if (!isOpen || !isDigitalHumanOutputType || !digitalHumanModels) {
      return;
    }
    const nextModelName = resolveDigitalHumanDefaultModel(digitalHumanModels, form.digitalHumanMode);
    const modelExists = availableDigitalHumanModels.some((item) => item.id === form.modelName);
    if (!nextModelName || (form.modelName && modelExists)) {
      return;
    }
    setForm((current) => {
      if (!isDigitalHumanOutput(current.outputType)) {
        return current;
      }
      if (current.modelName.trim()) {
        const exists = availableDigitalHumanModels.some((item) => item.id === current.modelName);
        if (exists) {
          return current;
        }
      }
      return {
        ...current,
        modelName: nextModelName,
      };
    });
  }, [
    availableDigitalHumanModels,
    digitalHumanModels,
    form.digitalHumanMode,
    form.modelName,
    isDigitalHumanOutputType,
    isOpen,
  ]);

  useEffect(() => {
    if (!coverPromptWarningOpen) {
      setCoverPromptUnlockCountdown(0);
      return;
    }
    setCoverPromptUnlockCountdown(4);
    const timer = window.setInterval(() => {
      setCoverPromptUnlockCountdown((current) => {
        if (current <= 1) {
          window.clearInterval(timer);
          return 0;
        }
        return current - 1;
      });
    }, 1000);
    return () => window.clearInterval(timer);
  }, [coverPromptWarningOpen]);

  const buildSkillPayload = () => ({
    name: form.name.trim(),
    description: form.description.trim(),
    outputType: form.outputType,
    modelName: form.modelName.trim(),
    deviceId,
    promptTemplate: form.promptTemplate.trim() || null,
    fixedDurationSeconds: isVideoTextOutput(form.outputType) && form.fixedDurationSeconds
      ? Number(form.fixedDurationSeconds)
      : null,
    publishIntroEnabled: form.publishIntroEnabled,
    coverPromptTemplate: form.coverPromptUsesSystemDefault ? null : form.coverPromptTemplate.trim() || null,
    topics: form.topicsText
      .split(/[\n,，#\s]+/)
      .map((item) => item.trim())
      .filter(Boolean),
    referencePayload: buildReferencePayload(
      skill?.referencePayload,
      referenceMediaOrder,
      isDigitalHumanOutput(form.outputType)
        ? {
            mode: form.digitalHumanMode,
            goodsTitle: form.digitalHumanGoodsTitle,
            goodsText: form.digitalHumanGoodsText,
          }
        : null,
    ),
    storyboardEnabled: form.storyboardEnabled,
    isEnabled: form.isEnabled,
  });

  const ensureSkillPayloadReady = (
    payload: ReturnType<typeof buildSkillPayload>,
    options?: { requireDigitalHumanAssets?: boolean },
  ) => {
    if (!payload.name || !payload.description || !payload.outputType || !payload.modelName) {
      throw new Error("请先填写完整的技能名称、简介、输出格式和模型，再上传素材");
    }
    if (isDigitalHumanOutput(payload.outputType)) {
      const goodsText = form.digitalHumanGoodsText.trim();
      if (!goodsText) {
        throw new Error("请先填写真人口播文案");
      }
      if (form.digitalHumanMode === "digital" && !form.digitalHumanGoodsTitle.trim()) {
        throw new Error("带货模式请先填写产品标题");
      }
      if (options?.requireDigitalHumanAssets) {
        if (digitalHumanCharacterAssets.length === 0) {
          throw new Error("请先上传人物主图");
        }
        if (digitalHumanAudioAssets.length === 0) {
          throw new Error("请先上传参考音频");
        }
        if (form.digitalHumanMode === "digital" && digitalHumanGoodsAssets.length === 0) {
          throw new Error("带货模式请先上传商品主图");
        }
      }
      return;
    }
    if (isVideoTextOutput(payload.outputType)) {
      const durationSeconds = Number(payload.fixedDurationSeconds || 0);
      if (!Number.isFinite(durationSeconds) || durationSeconds <= 0) {
        throw new Error("请先填写有效的视频固定时长");
      }
      if (durationSeconds % selectedVideoBaseSeconds !== 0) {
        throw new Error(`当前模型基础时长为 ${selectedVideoBaseSeconds} 秒，请填写 ${selectedVideoBaseSeconds} 的倍数`);
      }
    }
  };

  const invalidateSkillQueries = async (skillId: string) => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["skills"] }),
      queryClient.invalidateQueries({ queryKey: ["skills", deviceId] }),
      queryClient.invalidateQueries({ queryKey: ["skillAssets", skillId] }),
    ]);
  };

  const ensureUploadTargetSkillId = async () => {
    if (currentSkillId) {
      return currentSkillId;
    }
    if (draftCreationRef.current) {
      const draft = await draftCreationRef.current;
      return draft.id;
    }

    const payload = buildSkillPayload();
    ensureSkillPayloadReady(payload);

    const createPromise = createSkill(payload);
    draftCreationRef.current = createPromise;
    try {
      const createdSkill = await createPromise;
      setDraftSkillId(createdSkill.id);
      setDraftNeedsCleanup(true);
      return createdSkill.id;
    } finally {
      draftCreationRef.current = null;
    }
  };

  const uploadFiles = async (files: File[], assetType: UploadAssetType) => {
    if (files.length === 0) {
      return;
    }

    const nextUploads = files.map((file) => ({
      id: `${assetType}-${file.name}-${file.size}-${Date.now()}-${Math.random()}`,
      assetType,
      file,
    }));
    setUploadingAssets((current) => [...current, ...nextUploads]);

    try {
      const skillId = await ensureUploadTargetSkillId();
      for (const entry of nextUploads) {
        await uploadSkillAsset(skillId, entry.file, assetType);
      }
      await invalidateSkillQueries(skillId);
    } catch (error) {
      window.alert(error instanceof Error ? error.message : "上传素材失败，请稍后重试");
    } finally {
      const uploadIds = new Set(nextUploads.map((item) => item.id));
      setUploadingAssets((current) => current.filter((item) => !uploadIds.has(item.id)));
    }
  };

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = buildSkillPayload();
      ensureSkillPayloadReady(payload, { requireDigitalHumanAssets: true });
      return currentSkillId
        ? updateSkill(currentSkillId, payload)
        : createSkill(payload);
    },
    onSuccess: async (savedSkill) => {
      setDraftSkillId(savedSkill.id);
      setDraftNeedsCleanup(false);
      await invalidateSkillQueries(savedSkill.id);
      onSaved();
      onClose();
    },
    onError: (error) => {
      window.alert(error instanceof Error ? error.message : "保存技能失败，请稍后重试");
    },
  });

  const deleteAssetMutation = useMutation({
    mutationFn: async (asset: SkillAsset) => {
      if (!currentSkillId) {
        throw new Error("技能尚未创建，无法删除已上传素材");
      }
      await deleteSkillAsset(currentSkillId, asset.id);
    },
    onSuccess: async () => {
      if (!currentSkillId) {
        return;
      }
      await queryClient.invalidateQueries({ queryKey: ["skillAssets", currentSkillId] });
    },
    onError: (error) => {
      window.alert(error instanceof Error ? error.message : "删除素材失败，请稍后重试");
    },
  });

  if (!isOpen) {
    return null;
  }

  const isBusy = saveMutation.isPending || uploadingAssets.length > 0;

  const handleClose = async () => {
    if (isBusy) {
      window.alert("素材还在上传或技能正在保存，请稍候片刻。");
      return;
    }
    if (!skill?.id && draftNeedsCleanup && draftSkillId) {
      try {
        await deleteSkill(draftSkillId);
        await queryClient.invalidateQueries({ queryKey: ["skills", deviceId] });
      } catch (error) {
        window.alert(error instanceof Error ? error.message : "清理未完成的技能失败，请稍后重试");
        return;
      }
    }
    onClose();
  };

  const handleOutputChange = (nextOutputType: string) => {
    const nextCategory = mapSkillOutputToModelCategory(nextOutputType);
    const nextIsDigitalHuman = isDigitalHumanOutput(nextOutputType);
    const nextDefaultDigitalHumanModel = nextIsDigitalHuman
      ? resolveDigitalHumanDefaultModel(digitalHumanModels, form.digitalHumanMode)
      : "";
    setForm((current) => ({
      ...current,
      outputType: nextOutputType,
      promptTemplate:
        nextOutputType === "视文模式" && !current.promptTemplate.trim()
          ? DEFAULT_VIDEO_TASK_NOTE
          : current.promptTemplate,
      fixedDurationSeconds:
        nextOutputType === "视文模式"
          ? current.fixedDurationSeconds || String(selectedVideoBaseSeconds)
          : "",
      modelName:
        mapSkillOutputToModelCategory(current.outputType) === nextCategory
          ? current.modelName
          : nextDefaultDigitalHumanModel,
    }));
  };

  const handleDigitalHumanModeChange = (nextMode: "digital" | "customize") => {
    const nextModelName = resolveDigitalHumanDefaultModel(digitalHumanModels, nextMode);
    setForm((current) => ({
      ...current,
      digitalHumanMode: nextMode,
      modelName: nextModelName || current.modelName,
      digitalHumanGoodsTitle: nextMode === "customize" ? "" : current.digitalHumanGoodsTitle,
    }));
  };

  const moveMediaAsset = (assetId: string, direction: -1 | 1) => {
    setReferenceMediaOrder((current) => {
      const next = mergeReferenceMediaOrder(current, mediaAssets);
      const index = next.findIndex((item) => item === assetId);
      const swapIndex = index + direction;
      if (index < 0 || swapIndex < 0 || swapIndex >= next.length) {
        return current;
      }
      const reordered = [...next];
      [reordered[index], reordered[swapIndex]] = [reordered[swapIndex], reordered[index]];
      return reordered;
    });
  };

  const handleImageSelection = (fileList: FileList | null) => {
    const nextFiles = Array.from(fileList || []);
    if (nextFiles.length === 0) {
      return;
    }
    if (!supportsReferenceImages) {
      window.alert("当前模型不支持参考图片，请先切换到支持图片参考的模型。");
      return;
    }
    const remaining =
      mediaLimit > 0 ? Math.max(0, mediaLimit - totalMediaCount) : Number.MAX_SAFE_INTEGER;
    if (remaining <= 0) {
      window.alert(`当前模型最多支持 ${mediaLimit} 个参考媒体，请先删除部分素材。`);
      return;
    }
    const accepted = nextFiles.slice(0, remaining);
    if (accepted.length < nextFiles.length) {
      window.alert(`当前模型最多支持 ${mediaLimit} 个参考媒体，已仅保留前 ${accepted.length} 个。`);
    }
    void uploadFiles(accepted, "reference_image");
  };

  const handleVideoSelection = (fileList: FileList | null) => {
    const nextFiles = Array.from(fileList || []);
    if (nextFiles.length === 0) {
      return;
    }
    if (!supportsReferenceVideos) {
      window.alert("当前模型不支持参考视频，请先切换到支持视频参考的模型。");
      return;
    }
    const remaining =
      mediaLimit > 0 ? Math.max(0, mediaLimit - totalMediaCount) : Number.MAX_SAFE_INTEGER;
    if (remaining <= 0) {
      window.alert(`当前模型最多支持 ${mediaLimit} 个参考媒体，请先删除部分素材。`);
      return;
    }
    const accepted = nextFiles.slice(0, remaining);
    if (accepted.length < nextFiles.length) {
      window.alert(`当前模型最多支持 ${mediaLimit} 个参考媒体，已仅保留前 ${accepted.length} 个。`);
    }
    void uploadFiles(accepted, "reference_video");
  };

  const handleTextSelection = (fileList: FileList | null) => {
    const nextFiles = Array.from(fileList || []);
    if (nextFiles.length === 0) {
      return;
    }
    void uploadFiles(nextFiles, "reference_text");
  };

  const handleDigitalHumanAssetSelection = (
    fileList: FileList | null,
    assetType: Extract<
      UploadAssetType,
      "digital_human_character_image" | "digital_human_goods_image" | "digital_human_ref_audio"
    >,
  ) => {
    const nextFile = Array.from(fileList || [])[0];
    if (!nextFile) {
      return;
    }
    void uploadFiles([nextFile], assetType);
  };

  const requestCoverPromptEditing = () => {
    if (coverPromptUnlocked) {
      return;
    }
    setCoverPromptWarningOpen(true);
  };

  const resetCoverPromptToDefault = () => {
    setForm((current) => ({
      ...current,
      coverPromptTemplate: coverPromptDefault,
      coverPromptUsesSystemDefault: true,
    }));
    setCoverPromptUnlocked(false);
  };

  const flowSteps = isDigitalHumanOutputType
    ? [
        "账号执行技能时自动复用人物主图、商品主图和参考音频",
        form.digitalHumanMode === "digital" ? "按带货模式创建真人任务" : "按口播模式创建真人任务",
        visibleModelName || "最终模型待选择",
      ]
    : form.storyboardEnabled
      ? [
          "客户输入素材、任务说明和简介",
          "系统先做分镜优化",
          getModelDisplayName(selectedModel, "最终模型待选择"),
        ]
      : [
          "客户输入素材、任务说明和简介",
          "跳过分镜，直接执行",
          getModelDisplayName(selectedModel, "最终模型待选择"),
        ];

  return (
    <div className="fixed inset-0 z-[90] bg-[#050814]/85 px-4 py-4 backdrop-blur-xl sm:px-6 sm:py-6">
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className="absolute left-[-10%] top-[-8%] h-64 w-64 rounded-full bg-accent/14 blur-3xl" />
        <div className="absolute bottom-[-8%] right-[-5%] h-64 w-64 rounded-full bg-cyan/12 blur-3xl" />
      </div>

      <div className="relative mx-auto flex h-[calc(100vh-2rem)] max-w-[1260px] items-stretch sm:h-[calc(100vh-3rem)]">
        <div className="flex w-full flex-col overflow-hidden rounded-[30px] border border-white/10 bg-[#08111f]/96 shadow-[0_28px_100px_rgba(0,0,0,0.52)]">
          <div className="border-b border-white/10 bg-[linear-gradient(135deg,rgba(177,73,255,0.14),rgba(0,245,212,0.05)_40%,rgba(8,17,31,0)_72%)] px-6 py-6 sm:px-8">
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-3">
                <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/6 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.24em] text-text-muted">
                  <Wand2 className="h-3.5 w-3.5 text-accent" />
                  OpenClaw Skill
                </div>
                <div>
                  <h3 className="text-2xl font-semibold tracking-tight text-white sm:text-[30px]">
                    {skill ? "编辑技能" : "新增技能"}
                  </h3>
                  <p className="mt-2 max-w-3xl text-sm leading-6 text-text-secondary">
                    先选产出和模型，再决定是否开启分镜，然后上传参考素材。
                  </p>
                </div>
                <div className="mt-1 flex flex-wrap gap-2">
                  <BadgeChip label={`节点 ${deviceId.slice(0, 8)}`} />
                  <BadgeChip label={form.storyboardEnabled ? "分镜优化开启" : "分镜优化关闭"} active={form.storyboardEnabled} />
                  <BadgeChip label={form.isEnabled ? "技能启用中" : "技能已暂停"} active={form.isEnabled} tone="emerald" />
                </div>
              </div>

              <button
                type="button"
                onClick={() => {
                  void handleClose();
                }}
                disabled={isBusy}
                className="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-white/5 text-text-muted transition-all hover:border-white/20 hover:bg-white/10 hover:text-white"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto">
            <div className="grid gap-0 xl:grid-cols-[minmax(0,1fr)_292px]">
              <div className="px-6 py-8 sm:px-8">
                <div className="space-y-8">
                  <SectionCard
                    title="基础设定"
                    description="先定义这条技能的简介、任务说明和标签。简介如何优化由管理员统一配置，你只决定是否启用。"
                  >
                    <label className="space-y-2.5">
                      <span className="text-sm font-medium text-white">技能名称</span>
                      <input
                        value={form.name}
                        onChange={(event) =>
                          setForm((current) => ({ ...current, name: event.target.value }))
                        }
                        placeholder="例如：新品种草短视频"
                        className="w-full rounded-2xl border border-white/10 bg-white/6 px-4 py-3 text-sm text-white outline-none transition-all placeholder:text-text-muted focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                      />
                    </label>

                    <div className="space-y-2.5">
                      <span className="text-sm font-medium text-white">输出类型</span>
                      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                        {OUTPUT_OPTIONS.map((option) => {
                          const Icon = option.icon;
                          const selected = option.value === form.outputType;
                          return (
                            <button
                              key={option.value}
                              type="button"
                              onClick={() => handleOutputChange(option.value)}
                              className={cn(
                                "rounded-2xl border px-4 py-4 text-left transition-all",
                                selected
                                  ? "border-accent/45 bg-accent/12 shadow-[0_12px_35px_rgba(177,73,255,0.16)]"
                                  : "border-white/10 bg-white/[0.04] hover:border-white/18 hover:bg-white/[0.06]",
                              )}
                            >
                              <div className="flex items-start justify-between gap-2">
                                <div className={cn("flex h-9 w-9 items-center justify-center rounded-2xl bg-white/8", option.tone)}>
                                  <Icon className="h-4 w-4" />
                                </div>
                                <SelectionBadge selected={selected} />
                              </div>
                              <p className="mt-3 text-sm font-semibold text-white">{option.label}</p>
                              <p className="mt-1 text-xs leading-5 text-text-secondary">{option.hint}</p>
                            </button>
                          );
                        })}
                      </div>
                    </div>

                    <div className="grid gap-4 lg:grid-cols-2">
                      <label className="space-y-2.5">
                        <span className="text-sm font-medium text-white">简介</span>
                        <textarea
                          value={form.description}
                          onChange={(event) =>
                            setForm((current) => ({ ...current, description: event.target.value }))
                          }
                          rows={4}
                          placeholder="填写发布到三方平台的基础简介，说明主题、受众、语气、卖点和风格基线。"
                          className="w-full rounded-2xl border border-white/10 bg-white/6 px-4 py-3 text-sm leading-6 text-white outline-none transition-all placeholder:text-text-muted focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                        />
                      </label>

                      <label className="space-y-2.5">
                        <span className="text-sm font-medium text-white">任务说明</span>
                        <textarea
                          value={form.promptTemplate}
                          onChange={(event) =>
                            setForm((current) => ({ ...current, promptTemplate: event.target.value }))
                          }
                          rows={4}
                          placeholder="告诉系统重点表达什么，比如镜头感、节奏、品牌边界和禁用词。视文模式默认会补充“默认不要字幕”。"
                          className="w-full rounded-2xl border border-white/10 bg-white/6 px-4 py-3 text-sm leading-6 text-white outline-none transition-all placeholder:text-text-muted focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                        />
                      </label>
                    </div>

                    <label className="space-y-2.5">
                      <span className="text-sm font-medium text-white">标签</span>
                      <textarea
                        value={form.topicsText}
                        onChange={(event) =>
                          setForm((current) => ({ ...current, topicsText: event.target.value }))
                        }
                        rows={4}
                        placeholder="例如：小红书种草，抖音短视频，品牌口播。多个标签可用空格、逗号或换行分隔。"
                        className="w-full rounded-2xl border border-white/10 bg-white/6 px-4 py-3 text-sm leading-6 text-white outline-none transition-all placeholder:text-text-muted focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                      />
                      <p className="text-xs leading-5 text-text-secondary">
                        这些标签会跟随技能进入账号发布任务，并继续传给 SAU 上传器。
                      </p>
                    </label>



                    {isVideoTextOutput(form.outputType) ? (
                      <div className="space-y-3 rounded-[24px] border border-white/10 bg-white/[0.04] p-4">
                        <div className="flex flex-wrap items-start justify-between gap-3">
                          <div className="space-y-1">
                            <span className="text-sm font-medium text-white">封面提示词</span>
                            <p className="text-xs leading-5 text-text-secondary">
                              仅视文模式生效。系统会把这段话和客户原始图片、参考资料一起交给
                              {" "}系统封面模型重新设计视频封面首帧；客户上传多张图时会补做尾帧。
                            </p>
                          </div>
                          {form.coverPromptUsesSystemDefault ? (
                            <MiniPill>当前使用系统默认</MiniPill>
                          ) : (
                            <button
                              type="button"
                              onClick={resetCoverPromptToDefault}
                              className="rounded-full border border-white/10 px-3 py-1 text-xs font-medium text-text-secondary transition-all hover:border-white/20 hover:text-white"
                            >
                              恢复系统默认
                            </button>
                          )}
                        </div>
                        <textarea
                          value={form.coverPromptTemplate}
                          readOnly={!coverPromptUnlocked}
                          onClick={requestCoverPromptEditing}
                          onFocus={requestCoverPromptEditing}
                          onChange={(event) =>
                            setForm((current) => ({
                              ...current,
                              coverPromptTemplate: event.target.value,
                              coverPromptUsesSystemDefault: false,
                            }))
                          }
                          rows={7}
                          placeholder="描述希望系统如何围绕真实产品重新设计封面。"
                          className={cn(
                            "w-full rounded-2xl border px-4 py-3 text-sm leading-6 text-white outline-none transition-all placeholder:text-text-muted",
                            coverPromptUnlocked
                              ? "border-white/10 bg-white/6 focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                              : "cursor-pointer border-white/8 bg-white/[0.03]",
                          )}
                        />
                        <div className="flex flex-wrap items-center justify-between gap-2 text-xs leading-5 text-text-secondary">
                          <p>
                            不修改时不会把默认值固化到当前技能；后续 Admin 更新系统默认值后，这里会自动跟随。
                          </p>
                          {!coverPromptUnlocked ? (
                            <button
                              type="button"
                              onClick={requestCoverPromptEditing}
                              className="rounded-full border border-accent/30 px-3 py-1 font-medium text-accent transition-all hover:border-accent/50"
                            >
                              修改封面提示词
                            </button>
                          ) : null}
                        </div>
                      </div>
                    ) : null}
                  </SectionCard>

                  {isDigitalHumanOutputType ? (
                    <SectionCard
                      title="口播设定"
                      description="配置真人口播模式、产品信息和文案脚本。素材和文案将保存在技能自身，账号执行时会直接按当前配置创建真人任务。"
                    >
                      <div className="flex flex-wrap gap-2">
                        {[
                          { value: "digital" as const, label: "带货模式", icon: <Package className="h-3.5 w-3.5" /> },
                          { value: "customize" as const, label: "口播模式", icon: <Mic className="h-3.5 w-3.5" /> },
                        ].map((option) => (
                          <button
                            key={option.value}
                            type="button"
                            onClick={() => handleDigitalHumanModeChange(option.value)}
                            className={cn(
                              "inline-flex items-center gap-2 rounded-full border px-3.5 py-2 text-sm transition-all",
                              form.digitalHumanMode === option.value
                                ? "border-emerald-400/35 bg-emerald-400/12 text-white"
                                : "border-white/10 bg-white/[0.04] text-text-secondary hover:border-white/20 hover:text-white",
                            )}
                          >
                            {option.icon}
                            {option.label}
                          </button>
                        ))}
                      </div>

                      {form.digitalHumanMode === "digital" ? (
                        <label className="space-y-2.5">
                          <span className="text-sm font-medium text-white">产品标题</span>
                          <input
                            value={form.digitalHumanGoodsTitle}
                            onChange={(event) =>
                              setForm((current) => ({ ...current, digitalHumanGoodsTitle: event.target.value }))
                            }
                            placeholder="例如：老廖牌香薰"
                            className="w-full rounded-2xl border border-white/10 bg-white/6 px-4 py-3 text-sm text-white outline-none transition-all placeholder:text-text-muted focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                          />
                        </label>
                      ) : null}

                      <label className="space-y-2.5">
                        <span className="text-sm font-medium text-white">口播文案</span>
                        <textarea
                          value={form.digitalHumanGoodsText}
                          onChange={(event) =>
                            setForm((current) => ({ ...current, digitalHumanGoodsText: event.target.value }))
                          }
                          rows={5}
                          placeholder="填写真人口播文案，系统会按这段文案生成最终视频。"
                          className="w-full rounded-2xl border border-white/10 bg-white/6 px-4 py-3 text-sm leading-6 text-white outline-none transition-all placeholder:text-text-muted focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                        />
                      </label>
                    </SectionCard>
                  ) : null}

                  <SectionCard
                    title={isDigitalHumanOutputType ? "执行模型" : "模型与时长"}
                    description={isDigitalHumanOutputType ? "选择真人口播的执行模型。系统已根据当前模式推荐最优模型，通常无需手动更换。" : "选择最终执行模型并设置视频时长。模型和时长紧密耦合，在同一区域方便对照。"}
                  >
                    {isDigitalHumanOutputType ? (
                      digitalHumanModelsLoading ? (
                        <InlineLoading label="正在读取真人模型..." />
                      ) : availableDigitalHumanModels.length === 0 ? (
                        <div className="rounded-[24px] border border-dashed border-white/10 bg-white/[0.03] px-5 py-12 text-center text-sm text-text-secondary">
                          当前没有可用的真人模型。
                        </div>
                      ) : !digitalHumanModelExpanded ? (
                        /* ── Collapsed: show only selected model summary ── */
                        <div className="space-y-3">
                          {(() => {
                            const activeModel = availableDigitalHumanModels.find((m) => m.id === form.modelName);
                            return (
                              <div className="rounded-[26px] border border-emerald-400/45 bg-[linear-gradient(160deg,rgba(16,185,129,0.14),rgba(12,18,32,0.08))] p-5 shadow-[0_0_0_1px_rgba(16,185,129,0.18),0_18px_45px_rgba(16,185,129,0.12)]">
                                <div className="flex items-start justify-between gap-3">
                                  <div className="min-w-0">
                                    <div className="flex flex-wrap items-center gap-2">
                                      <p className="text-base font-semibold text-white">{form.modelName || "未选择"}</p>
                                      {activeModel?.isRecommended ? (
                                        <span className="inline-flex items-center rounded-full border border-emerald-400/30 bg-emerald-400/12 px-2.5 py-0.5 text-[11px] font-semibold text-emerald-300">✦ 推荐</span>
                                      ) : null}
                                      {activeModel?.isCurrent ? (
                                        <span className="inline-flex items-center rounded-full border border-white/10 bg-white/6 px-2.5 py-0.5 text-[11px] font-semibold text-text-secondary">当前服务模型</span>
                                      ) : null}
                                    </div>
                                    <p className="mt-2 text-sm leading-6 text-text-secondary">
                                      {activeModel?.isRecommended
                                        ? "系统推荐模型，适合作为当前模式的默认执行模型。"
                                        : "已固定为该模型，保存后作为真人口播默认执行模型。"}
                                    </p>
                                  </div>
                                  <SelectionBadge selected />
                                </div>
                              </div>
                            );
                          })()}
                          <button
                            type="button"
                            onClick={() => setDigitalHumanModelExpanded(true)}
                            className="inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-white/[0.04] px-4 py-2 text-xs font-medium text-text-secondary transition-all hover:border-white/20 hover:text-white"
                          >
                            <ChevronDown className="h-3.5 w-3.5" />
                            更换模型（共 {availableDigitalHumanModels.length} 个）
                          </button>
                        </div>
                      ) : (
                        /* ── Expanded: full model list with recommended first ── */
                        <div className="space-y-3">
                          <div className="grid gap-4 lg:grid-cols-2">
                            {[...availableDigitalHumanModels]
                              .sort((a, b) => {
                                if (a.isRecommended && !b.isRecommended) return -1;
                                if (!a.isRecommended && b.isRecommended) return 1;
                                if (a.isCurrent && !b.isCurrent) return -1;
                                if (!a.isCurrent && b.isCurrent) return 1;
                                return 0;
                              })
                              .map((model) => {
                              const selected = model.id === form.modelName;
                              return (
                                <button
                                  key={model.id}
                                  type="button"
                                  onClick={() => {
                                    setForm((current) => ({ ...current, modelName: model.id }));
                                    setDigitalHumanModelExpanded(false);
                                  }}
                                  className={cn(
                                    "group relative rounded-[26px] border p-5 text-left transition-all duration-200",
                                    selected
                                      ? "border-emerald-400/45 bg-[linear-gradient(160deg,rgba(16,185,129,0.14),rgba(12,18,32,0.08))] shadow-[0_0_0_1px_rgba(16,185,129,0.18),0_18px_45px_rgba(16,185,129,0.12)]"
                                      : "border-white/10 bg-white/[0.04] hover:border-white/18 hover:bg-white/[0.06]",
                                  )}
                                >
                                  <div className="flex items-start justify-between gap-3">
                                    <div className="min-w-0">
                                      <div className="flex flex-wrap items-center gap-2">
                                        <p className="text-base font-semibold text-white">{model.id}</p>
                                        {model.isRecommended ? (
                                          <span className="inline-flex items-center rounded-full border border-emerald-400/30 bg-emerald-400/12 px-2.5 py-0.5 text-[11px] font-semibold text-emerald-300">✦ 推荐</span>
                                        ) : null}
                                        {model.isCurrent ? <MiniPill>当前服务模型</MiniPill> : null}
                                      </div>
                                      <p className="mt-2 text-sm leading-6 text-text-secondary">
                                        {model.isRecommended
                                          ? "推荐模型，适合作为真人口播默认执行模型。"
                                          : "可作为真人口播执行模型，由技能在保存时固定。"}
                                      </p>
                                    </div>
                                    <SelectionBadge selected={selected} />
                                  </div>
                                </button>
                              );
                            })}
                          </div>
                          <button
                            type="button"
                            onClick={() => setDigitalHumanModelExpanded(false)}
                            className="inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-white/[0.04] px-4 py-2 text-xs font-medium text-text-secondary transition-all hover:border-white/20 hover:text-white"
                          >
                            <ChevronUp className="h-3.5 w-3.5" />
                            收起模型列表
                          </button>
                        </div>
                      )
                    ) : modelsLoading ? (
                      <InlineLoading label="正在读取可用模型..." />
                    ) : availableModels.length === 0 ? (
                      <div className="rounded-[24px] border border-dashed border-white/10 bg-white/[0.03] px-5 py-12 text-center text-sm text-text-secondary">
                        当前没有匹配这个输出类型的已启用模型。
                      </div>
                    ) : (
                      <div className="grid gap-4 lg:grid-cols-2">
                        {availableModels.map((model) => {
                          const selected = model.modelName === form.modelName;
                          const modelLimit = getModelReferenceLimit(form.outputType, model);
                          const modelFileTypes = resolveSupportedFileTypes(model);
                          const modelSupportsImages = hasSupportedFilePrefix(modelFileTypes, "image/");
                          const modelSupportsVideos = hasSupportedFilePrefix(modelFileTypes, "video/");
                          const vendorColor = getVendorColor(model.vendor);
                          return (
                            <button
                              key={model.id}
                              type="button"
                              onClick={() =>
                                setForm((current) => ({ ...current, modelName: model.modelName }))
                              }
                              className={cn(
                                "group relative rounded-[26px] border p-5 text-left transition-all duration-200",
                                selected
                                  ? "border-accent/50 bg-[linear-gradient(160deg,rgba(177,73,255,0.12),rgba(0,245,212,0.04))] shadow-[0_0_0_1px_rgba(177,73,255,0.15),0_18px_45px_rgba(177,73,255,0.12)]"
                                  : "border-white/10 bg-white/[0.04] hover:border-white/18 hover:bg-white/[0.06]",
                              )}
                            >
                              {selected ? (
                                <span className="absolute left-3 top-3 flex h-3 w-3">
                                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-accent/60 opacity-75" />
                                  <span className="relative inline-flex h-3 w-3 rounded-full bg-accent" />
                                </span>
                              ) : null}
                              <div className="flex items-start justify-between gap-3">
                                <div className="min-w-0">
                                  <div className="flex flex-wrap items-center gap-2">
                                    <p className="text-base font-semibold text-white">{getModelDisplayName(model)}</p>
                                    <span className={cn("rounded-full border px-2.5 py-0.5 text-[11px] font-semibold", vendorColor.border, vendorColor.bg, vendorColor.text)}>
                                      {model.vendor}
                                    </span>
                                  </div>
                                  <p className="mt-2 line-clamp-2 text-sm leading-6 text-text-secondary">
                                    {model.description || "暂无模型说明，可在后台补充。"}
                                  </p>
                                </div>
                                <SelectionBadge selected={selected} />
                              </div>
                              <div className="mt-4 flex flex-wrap items-center gap-2">
                                <span className="inline-flex items-center rounded-full border border-accent/20 bg-accent/8 px-2.5 py-1 text-xs font-semibold text-accent">
                                  {formatSkillBillingAmount(model)}
                                </span>
                                <span className="inline-flex items-center rounded-full border border-white/10 bg-white/6 px-2.5 py-1 text-xs font-medium text-text-secondary">
                                  {formatBillingMode(model.billingMode)}
                                </span>
                                <span className="inline-flex items-center rounded-full border border-white/10 bg-white/6 px-2.5 py-1 text-xs font-medium text-text-secondary">
                                  {modelSupportsImages && modelSupportsVideos ? "🖼️🎬 " : modelSupportsVideos ? "🎬 " : modelSupportsImages ? "🖼️ " : ""}
                                  {describeSupportedMediaTypes(modelSupportsImages, modelSupportsVideos)}
                                </span>
                                {modelLimit > 0 ? (
                                  <span className="inline-flex items-center rounded-full border border-white/10 bg-white/6 px-2.5 py-1 text-xs font-medium text-text-secondary">
                                    最多{modelLimit}个
                                  </span>
                                ) : null}
                              </div>
                            </button>
                          );
                        })}
                      </div>
                    )}

                    {!isDigitalHumanOutputType ? (
                      <div className="rounded-[24px] border border-white/10 bg-[#0d1729] p-4">
                        <div className="flex items-center gap-2 text-sm font-medium text-white">
                          <Cpu className="h-4 w-4 text-cyan" />
                          当前模型
                        </div>
                        <div className="mt-3 grid gap-3 sm:grid-cols-2">
                          <SummaryLine
                            label="执行模型"
                            value={visibleModelName}
                          />
                          <SummaryLine
                            label="单次扣费"
                            value={formatSkillBillingAmount(selectedModel)}
                          />
                        </div>
                        <p className="mt-3 text-xs leading-5 text-text-secondary">
                          创建前即可看到当前技能单次预计扣费，实际扣费以任务入账结果为准。
                        </p>
                      </div>
                    ) : null}

                    {isDigitalHumanOutputType ? (
                      <div className="flex items-center justify-between rounded-[24px] border border-white/10 bg-[#0d1729] p-4">
                        <div>
                          <p className="text-sm font-medium text-white">技能状态</p>
                          <p className="mt-1 text-xs text-text-secondary">关闭后保留配置，账号侧不能继续使用。</p>
                        </div>
                        <button
                          type="button"
                          onClick={() => setForm((current) => ({ ...current, isEnabled: !current.isEnabled }))}
                          className={cn(
                            "relative inline-flex h-7 w-12 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/20",
                            form.isEnabled ? "bg-emerald-500" : "bg-white/15",
                          )}
                        >
                          <span
                            className={cn(
                              "pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow-lg ring-0 transition-transform duration-200",
                              form.isEnabled ? "translate-x-5" : "translate-x-0.5",
                            )}
                          />
                        </button>
                      </div>
                    ) : null}

                    {isVideoTextOutput(form.outputType) ? (
                      <div className="space-y-3">
                        <span className="text-sm font-medium text-white">固定视频时长</span>
                        <input
                          type="number"
                          min={selectedVideoBaseSeconds}
                          step={selectedVideoBaseSeconds}
                          value={form.fixedDurationSeconds}
                          onChange={(event) =>
                            setForm((current) => ({ ...current, fixedDurationSeconds: event.target.value }))
                          }
                          className="w-full rounded-2xl border border-white/10 bg-white/6 px-4 py-3 text-sm text-white outline-none transition-all focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                          placeholder={`请输入 ${selectedVideoBaseSeconds} 的倍数`}
                        />
                        <div className="flex flex-wrap gap-2">
                          {videoDurationPresets.map((option) => {
                            const presetSelected = Number(form.fixedDurationSeconds) === option.seconds;
                            return (
                              <button
                                key={option.seconds}
                                type="button"
                                onClick={() =>
                                  setForm((current) => ({
                                    ...current,
                                    fixedDurationSeconds: String(option.seconds),
                                  }))
                                }
                                className={cn(
                                  "rounded-full border px-3 py-1.5 text-xs font-medium transition-all",
                                  presetSelected
                                    ? "border-accent/45 bg-accent/12 text-white"
                                    : "border-white/10 bg-white/[0.04] text-text-secondary hover:border-white/20 hover:text-white",
                                )}
                              >
                                {option.label}
                                {typeof option.specialPriceCredits === "number" && option.specialPriceCredits > 0
                                  ? ` · 特价 ${option.specialPriceCredits}`
                                  : ""}
                              </button>
                            );
                          })}
                        </div>
                        <p className="text-xs leading-5 text-text-secondary">
                          当前模型基础时长为 {selectedVideoBaseSeconds} 秒，只支持 {selectedVideoBaseSeconds} 的倍数。超过单段时长后，系统会自动生成首帧并在多段时以上一段末帧续接，未命中特价时按首帧/分镜和每段视频实际步骤累计计费。
                        </p>
                        <p className="text-xs leading-5 text-text-secondary">
                          {matchedDurationRule && typeof matchedDurationRule.specialPriceCredits === "number" && matchedDurationRule.specialPriceCredits > 0
                            ? `当前时长命中特价 ${matchedDurationRule.specialPriceCredits} 积分。`
                            : "当前时长未命中特价套餐，将按实际模型步骤计费。"}
                        </p>
                      </div>
                    ) : null}
                  </SectionCard>

                  {!isDigitalHumanOutputType ? (
                    <SectionCard
                      title="执行方式"
                      description="决定技能是否生效、是否启用简介 AI 优化和分镜优化。"
                    >
                      <div className="grid gap-4 lg:grid-cols-3">
                        <SwitchCard
                          title="简介 AI 优化"
                          description="开启后，在每次发布前自动改写简介；关闭后沿用你填写的原始简介。"
                          enabled={form.publishIntroEnabled}
                          enabledLabel="已启用"
                          disabledLabel="已关闭"
                          accent="cyan"
                          onToggle={() =>
                            setForm((current) => ({
                              ...current,
                              publishIntroEnabled: !current.publishIntroEnabled,
                            }))
                          }
                        />
                        <SwitchCard
                          title="AI 分镜优化"
                          description="开启后，系统会先统一优化分镜、任务说明和发布简介，再交给最终模型。"
                          enabled={form.storyboardEnabled}
                          enabledLabel="已启用"
                          disabledLabel="已关闭"
                          accent="accent"
                          onToggle={() =>
                            setForm((current) => ({
                              ...current,
                              storyboardEnabled: !current.storyboardEnabled,
                            }))
                          }
                        />
                        <SwitchCard
                          title="技能状态"
                          description="关闭后保留配置，但账号侧不能继续用这条技能创建新任务。"
                          enabled={form.isEnabled}
                          enabledLabel="已启用"
                          disabledLabel="已暂停"
                          accent="emerald"
                          onToggle={() =>
                            setForm((current) => ({ ...current, isEnabled: !current.isEnabled }))
                          }
                        />
                      </div>
                    </SectionCard>
                  ) : null}


                  <SectionCard
                    title="参考素材"
                    description={
                      isDigitalHumanOutputType
                        ? "真人口播会把人物主图、商品主图和参考音频保存在技能资产中，执行时自动复用。"
                        : "图片和视频共用一条有序参考链，文本继续补充结构、卖点和限制条件。"
                    }
                  >
                    {isDigitalHumanOutputType ? (
                      <div className="grid gap-4 lg:grid-cols-3">
                        <UploadCard
                          title="人物主图"
                          hint="口播模式和带货模式都必填，用于生成真人形象。"
                          icon={<UserRound className="h-5 w-5 text-emerald-300" />}
                        >
                          {!digitalHumanCharacterAssets.length && !uploadingDigitalHumanCharacters.length ? (
                            <label className="flex cursor-pointer items-center justify-center rounded-[22px] border border-dashed border-emerald-400/30 bg-emerald-400/10 px-4 py-5 text-center transition-all hover:border-emerald-400/50 hover:bg-emerald-400/14">
                              <div>
                                <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-2xl bg-emerald-400/15 text-emerald-300">
                                  <Upload className="h-5 w-5" />
                                </div>
                                <p className="mt-3 text-sm font-semibold text-white">上传人物主图</p>
                                <p className="mt-1 text-xs text-text-secondary">单张上传，替换前请先删除旧素材。</p>
                              </div>
                              <input
                                type="file"
                                accept="image/*"
                                className="hidden"
                                onChange={(event) => {
                                  handleDigitalHumanAssetSelection(event.target.files, "digital_human_character_image");
                                  event.target.value = "";
                                }}
                              />
                            </label>
                          ) : null}
                          <div className="mt-4 space-y-3">
                            {digitalHumanCharacterAssets.map((asset) => (
                              <AssetRow
                                key={asset.id}
                                asset={asset}
                                icon={<UserRound className="h-4 w-4 text-emerald-300" />}
                                deleting={deleteAssetMutation.isPending}
                                onDelete={() => deleteAssetMutation.mutate(asset)}
                              />
                            ))}
                            {uploadingDigitalHumanCharacters.map((item) => (
                              <PendingRow
                                key={item.id}
                                label={item.file.name}
                                icon={<UserRound className="h-4 w-4 text-emerald-300" />}
                                file={item.file}
                                helperText="正在上传到云端..."
                                onDelete={() => undefined}
                                hideDelete
                              />
                            ))}
                            {!digitalHumanCharacterAssets.length && !uploadingDigitalHumanCharacters.length ? (
                              <EmptyUploadState label="还没有人物主图。" />
                            ) : null}
                          </div>
                        </UploadCard>

                        <UploadCard
                          title="商品主图"
                          hint={form.digitalHumanMode === "digital" ? "带货模式必填，用于商品展示。" : "口播模式可留空。"}
                          icon={<Package className="h-5 w-5 text-cyan" />}
                        >
                          {!digitalHumanGoodsAssets.length && !uploadingDigitalHumanGoods.length ? (
                            <label className="flex cursor-pointer items-center justify-center rounded-[22px] border border-dashed border-cyan/30 bg-cyan/10 px-4 py-5 text-center transition-all hover:border-cyan/50 hover:bg-cyan/14">
                              <div>
                                <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-2xl bg-cyan/15 text-cyan">
                                  <Upload className="h-5 w-5" />
                                </div>
                                <p className="mt-3 text-sm font-semibold text-white">上传商品主图</p>
                                <p className="mt-1 text-xs text-text-secondary">仅带货模式会真正执行到成片。</p>
                              </div>
                              <input
                                type="file"
                                accept="image/*"
                                className="hidden"
                                onChange={(event) => {
                                  handleDigitalHumanAssetSelection(event.target.files, "digital_human_goods_image");
                                  event.target.value = "";
                                }}
                              />
                            </label>
                          ) : null}
                          <div className="mt-4 space-y-3">
                            {digitalHumanGoodsAssets.map((asset) => (
                              <AssetRow
                                key={asset.id}
                                asset={asset}
                                icon={<Package className="h-4 w-4 text-cyan" />}
                                deleting={deleteAssetMutation.isPending}
                                onDelete={() => deleteAssetMutation.mutate(asset)}
                              />
                            ))}
                            {uploadingDigitalHumanGoods.map((item) => (
                              <PendingRow
                                key={item.id}
                                label={item.file.name}
                                icon={<Package className="h-4 w-4 text-cyan" />}
                                file={item.file}
                                helperText="正在上传到云端..."
                                onDelete={() => undefined}
                                hideDelete
                              />
                            ))}
                            {!digitalHumanGoodsAssets.length && !uploadingDigitalHumanGoods.length ? (
                              <EmptyUploadState label="还没有商品主图。" />
                            ) : null}
                          </div>
                        </UploadCard>

                        <UploadCard
                          title="参考音频"
                          hint="口播模式和带货模式都必填，用于拟合音色。"
                          icon={<AudioLines className="h-5 w-5 text-amber-200" />}
                        >
                          {!digitalHumanAudioAssets.length && !uploadingDigitalHumanAudios.length ? (
                            <label className="flex cursor-pointer items-center justify-center rounded-[22px] border border-dashed border-amber-300/30 bg-amber-300/10 px-4 py-5 text-center transition-all hover:border-amber-300/50 hover:bg-amber-300/14">
                              <div>
                                <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-2xl bg-amber-300/15 text-amber-200">
                                  <Upload className="h-5 w-5" />
                                </div>
                                <p className="mt-3 text-sm font-semibold text-white">上传参考音频</p>
                                <p className="mt-1 text-xs text-text-secondary">支持 mp3、wav、m4a、aac。</p>
                              </div>
                              <input
                                type="file"
                                accept="audio/*,.m4a,.mp3,.wav,.aac"
                                className="hidden"
                                onChange={(event) => {
                                  handleDigitalHumanAssetSelection(event.target.files, "digital_human_ref_audio");
                                  event.target.value = "";
                                }}
                              />
                            </label>
                          ) : null}
                          <div className="mt-4 space-y-3">
                            {digitalHumanAudioAssets.map((asset) => (
                              <AssetRow
                                key={asset.id}
                                asset={asset}
                                icon={<AudioLines className="h-4 w-4 text-amber-200" />}
                                deleting={deleteAssetMutation.isPending}
                                onDelete={() => deleteAssetMutation.mutate(asset)}
                              />
                            ))}
                            {uploadingDigitalHumanAudios.map((item) => (
                              <PendingRow
                                key={item.id}
                                label={item.file.name}
                                icon={<AudioLines className="h-4 w-4 text-amber-200" />}
                                file={item.file}
                                helperText="正在上传到云端..."
                                onDelete={() => undefined}
                                hideDelete
                              />
                            ))}
                            {!digitalHumanAudioAssets.length && !uploadingDigitalHumanAudios.length ? (
                              <EmptyUploadState label="还没有参考音频。" />
                            ) : null}
                          </div>
                        </UploadCard>
                      </div>
                    ) : (
                      <div className="grid gap-4 lg:grid-cols-2">
                        <UploadCard
                          title="参考媒体"
                          hint={
                            !selectedModel
                              ? "先选择最终模型，系统才知道你可以上传图片、视频还是两者都支持。"
                              : mediaLimit > 0
                                ? `当前模型支持 ${describeSupportedMediaTypes(supportsReferenceImages, supportsReferenceVideos)}，最多 ${mediaLimit} 个参考媒体，已准备 ${totalMediaCount} 个。`
                                : `当前模型支持 ${describeSupportedMediaTypes(supportsReferenceImages, supportsReferenceVideos)}，已准备 ${totalMediaCount} 个参考媒体。`
                          }
                          icon={<Video className="h-5 w-5 text-cyan" />}
                        >
                          <div className="grid gap-3 sm:grid-cols-2">
                            {supportsReferenceImages ? (
                              <label className="flex cursor-pointer items-center justify-center rounded-[22px] border border-dashed border-cyan/30 bg-cyan/10 px-4 py-5 text-center transition-all hover:border-cyan/50 hover:bg-cyan/14">
                                <div>
                                  <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-2xl bg-cyan/15 text-cyan">
                                    <Upload className="h-5 w-5" />
                                  </div>
                                  <p className="mt-3 text-sm font-semibold text-white">上传参考图片</p>
                                  <p className="mt-1 text-xs text-text-secondary">支持多张，选中后立即上传到云端。</p>
                                </div>
                                <input
                                  type="file"
                                  accept={imageAccept || "image/*"}
                                  multiple
                                  className="hidden"
                                  onChange={(event) => {
                                    handleImageSelection(event.target.files);
                                    event.target.value = "";
                                  }}
                                />
                              </label>
                            ) : (
                              <DisabledUploadState label="当前模型不支持参考图片" />
                            )}

                            {supportsReferenceVideos ? (
                              <label className="flex cursor-pointer items-center justify-center rounded-[22px] border border-dashed border-cyan/30 bg-cyan/10 px-4 py-5 text-center transition-all hover:border-cyan/50 hover:bg-cyan/14">
                                <div>
                                  <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-2xl bg-cyan/15 text-cyan">
                                    <Upload className="h-5 w-5" />
                                  </div>
                                  <p className="mt-3 text-sm font-semibold text-white">上传参考视频</p>
                                  <p className="mt-1 text-xs text-text-secondary">仅支持当前模型允许的视频格式，顺序会原样保留。</p>
                                </div>
                                <input
                                  type="file"
                                  accept={videoAccept || "video/*"}
                                  multiple
                                  className="hidden"
                                  onChange={(event) => {
                                    handleVideoSelection(event.target.files);
                                    event.target.value = "";
                                  }}
                                />
                              </label>
                            ) : (
                              <DisabledUploadState label="当前模型不支持参考视频" />
                            )}
                          </div>

                          <div className="mt-4 space-y-3">
                            {assetsLoading ? <InlineLoading label="正在读取参考媒体..." /> : null}
                            {orderedMediaAssets.map((asset, index) => (
                              <AssetRow
                                key={asset.id}
                                asset={asset}
                                icon={
                                  isVideoAsset(asset) ? (
                                    <Video className="h-4 w-4 text-cyan" />
                                  ) : (
                                    <ImageIcon className="h-4 w-4 text-cyan" />
                                  )
                                }
                                sequence={index + 1}
                                deleting={deleteAssetMutation.isPending}
                                moveUpDisabled={index === 0}
                                moveDownDisabled={index === orderedMediaAssets.length - 1}
                                onMoveUp={() => moveMediaAsset(asset.id, -1)}
                                onMoveDown={() => moveMediaAsset(asset.id, 1)}
                                onDelete={() => deleteAssetMutation.mutate(asset)}
                              />
                            ))}
                            {uploadingMedia.map((item, index) => (
                              <PendingRow
                                key={item.id}
                                label={item.file.name}
                                icon={
                                  item.assetType === "reference_video" ? (
                                    <Video className="h-4 w-4 text-cyan" />
                                  ) : (
                                    <ImageIcon className="h-4 w-4 text-cyan" />
                                  )
                                }
                                file={item.file}
                                sequence={orderedMediaAssets.length + index + 1}
                                helperText="正在上传到云端..."
                                onDelete={() => undefined}
                                hideDelete
                              />
                            ))}
                            {!orderedMediaAssets.length && !uploadingMedia.length ? (
                              <EmptyUploadState label="还没有参考媒体。" />
                            ) : null}
                          </div>
                        </UploadCard>

                        <UploadCard
                          title="参考文本"
                          hint={
                            form.storyboardEnabled
                              ? "这些文本会先参与分镜优化，再进入最终模型。"
                              : "这些文本会直接进入最终模型。"
                          }
                          icon={<FileText className="h-5 w-5 text-amber-200" />}
                        >
                          <label className="flex cursor-pointer items-center justify-center rounded-[22px] border border-dashed border-amber-300/30 bg-amber-300/10 px-4 py-5 text-center transition-all hover:border-amber-300/50 hover:bg-amber-300/14">
                            <div>
                              <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-2xl bg-amber-300/15 text-amber-200">
                                <Upload className="h-5 w-5" />
                              </div>
                              <p className="mt-3 text-sm font-semibold text-white">上传文本</p>
                              <p className="mt-1 text-xs text-text-secondary">支持 txt、md、json、csv、xml。</p>
                            </div>
                            <input
                              type="file"
                              accept=".txt,.md,.json,.csv,.xml,text/plain,text/markdown,application/json"
                              multiple
                              className="hidden"
                              onChange={(event) => {
                                handleTextSelection(event.target.files);
                                event.target.value = "";
                              }}
                            />
                          </label>

                          <div className="mt-4 space-y-3">
                            {textAssets.map((asset) => (
                              <AssetRow
                                key={asset.id}
                                asset={asset}
                                icon={<FileText className="h-4 w-4 text-amber-200" />}
                                deleting={deleteAssetMutation.isPending}
                                onDelete={() => deleteAssetMutation.mutate(asset)}
                              />
                            ))}
                            {uploadingTexts.map((item) => (
                              <PendingRow
                                key={item.id}
                                label={item.file.name}
                                icon={<FileText className="h-4 w-4 text-amber-200" />}
                                file={item.file}
                                helperText="正在上传到云端..."
                                onDelete={() => undefined}
                                hideDelete
                              />
                            ))}
                            {!textAssets.length && !uploadingTexts.length ? (
                              <EmptyUploadState label="还没有文本资料。" />
                            ) : null}
                          </div>
                        </UploadCard>
                      </div>
                    )}
                  </SectionCard>
                </div>
              </div>

              <aside className="border-t border-white/10 bg-white/[0.03] px-6 py-8 xl:border-l xl:border-t-0 xl:px-5">
                <div className="space-y-4">
                  <SidebarCard title="执行概览" icon={<Sparkles className="h-4 w-4 text-accent" />}>
                    <div className="grid gap-3">
                      <SummaryLine label="输出类型" value={selectedOutput.label} />
                      <SummaryLine label="最终模型" value={visibleModelName} />
                      <SummaryLine
                        label="参考素材"
                        value={
                          isDigitalHumanOutputType
                            ? `${digitalHumanCharacterAssets.length} 人物 / ${digitalHumanGoodsAssets.length} 商品 / ${digitalHumanAudioAssets.length} 音频`
                            : `${totalMediaCount} 媒体 / ${totalTextCount} 文`
                        }
                      />
                      <SummaryLine
                        label={isDigitalHumanOutputType ? "当前模式" : "媒体构成"}
                        value={
                          isDigitalHumanOutputType
                            ? form.digitalHumanMode === "digital"
                              ? "带货模式"
                              : "口播模式"
                            : `${totalImageCount} 图 / ${totalVideoCount} 视频`
                        }
                      />
                    </div>
                    <div className="mt-4 space-y-3">
                      {flowSteps.map((step, index) => (
                        <div key={step} className="flex items-start gap-3">
                          <div className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-white/8 text-xs font-semibold text-white">
                            {index + 1}
                          </div>
                          <p className="text-sm leading-6 text-text-secondary">{step}</p>
                        </div>
                      ))}
                    </div>
                  </SidebarCard>
                </div>
              </aside>
            </div>
          </div>

          <div className="flex shrink-0 items-center justify-end gap-3 border-t border-white/10 bg-[linear-gradient(to_right,#091221,#0b1628)] px-6 py-5 shadow-[0_-12px_40px_rgba(0,0,0,0.4)] sm:px-8">
            <button
              type="button"
              onClick={() => {
                void handleClose();
              }}
              disabled={isBusy}
              className="rounded-2xl border border-white/10 bg-white/5 px-4 py-2.5 text-sm font-medium text-text-primary transition-all hover:border-white/20 hover:bg-white/8 hover:text-white"
            >
              取消
            </button>
            <button
              type="button"
              onClick={() => saveMutation.mutate()}
              disabled={isBusy}
              className="inline-flex items-center gap-2 rounded-2xl bg-gradient-to-r from-accent via-pink to-cyan px-5 py-2.5 text-sm font-semibold text-white shadow-[0_16px_40px_rgba(177,73,255,0.22)] transition-all hover:scale-[1.01] hover:shadow-[0_20px_48px_rgba(177,73,255,0.28)] disabled:cursor-not-allowed disabled:opacity-60"
            >
              {isBusy ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {skill || draftSkillId ? "保存技能" : "创建技能"}
            </button>
          </div>
        </div>
      </div>
      {coverPromptWarningOpen ? (
        <div className="absolute inset-0 z-[120] flex items-center justify-center bg-[#050814]/82 px-4 backdrop-blur-sm">
          <div className="w-full max-w-xl rounded-[28px] border border-white/10 bg-[#091321] p-6 shadow-[0_28px_100px_rgba(0,0,0,0.58)]">
            <div className="space-y-3">
              <h4 className="text-lg font-semibold text-white">修改封面提示词</h4>
              <p className="text-sm leading-6 text-text-secondary">
                修改封面提示词会直接影响视频首帧效果，也可能导致尾帧风格失衡。建议在技术人员协助下修改，避免偏离真实产品、卖点和使用场景。
              </p>
              <div className="rounded-2xl border border-white/10 bg-white/[0.04] p-4 text-sm leading-6 text-text-secondary">
                当前解锁后，你修改的是技能级覆盖值。若只是想继续跟随系统默认，不要修改，直接关闭即可。
              </div>
            </div>
            <div className="mt-6 flex flex-wrap items-center justify-end gap-3">
              <button
                type="button"
                onClick={() => setCoverPromptWarningOpen(false)}
                className="rounded-2xl border border-white/10 px-4 py-2 text-sm font-medium text-text-secondary transition-all hover:border-white/20 hover:text-white"
              >
                取消
              </button>
              <button
                type="button"
                disabled={coverPromptUnlockCountdown > 0}
                onClick={() => {
                  setCoverPromptUnlocked(true);
                  setCoverPromptWarningOpen(false);
                }}
                className="rounded-2xl bg-accent px-4 py-2 text-sm font-medium text-white transition-all hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-55"
              >
                {coverPromptUnlockCountdown > 0
                  ? `${coverPromptUnlockCountdown}s 后可继续修改`
                  : "我已知晓，继续修改"}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function SectionCard({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <section className="overflow-hidden rounded-[28px] border border-white/10 bg-[linear-gradient(180deg,rgba(255,255,255,0.05),rgba(255,255,255,0.03))]">
      <div className="border-b border-white/10 bg-white/[0.03] px-5 py-4 sm:px-6">
        <h4 className="text-lg font-semibold text-white">{title}</h4>
        <p className="mt-1.5 text-sm leading-6 text-text-secondary">{description}</p>
      </div>
      <div className="space-y-5 px-5 py-5 sm:px-6">{children}</div>
    </section>
  );
}

function SwitchCard({
  title,
  description,
  enabled,
  enabledLabel,
  disabledLabel,
  accent,
  disabled,
  onToggle,
}: {
  title: string;
  description: string;
  enabled: boolean;
  enabledLabel: string;
  disabledLabel: string;
  accent: "accent" | "emerald" | "cyan";
  disabled?: boolean;
  onToggle: () => void;
}) {
  const accentClass =
    accent === "emerald"
      ? enabled
        ? "border-emerald-400/28 bg-emerald-400/10"
        : "border-white/10 bg-white/[0.03]"
      : accent === "cyan"
        ? enabled
          ? "border-cyan/30 bg-cyan/10"
          : "border-white/10 bg-white/[0.03]"
        : enabled
          ? "border-accent/30 bg-accent/10"
          : "border-white/10 bg-white/[0.03]";

  const knobClass =
    accent === "emerald"
      ? enabled
        ? "translate-x-5 bg-emerald-300"
        : "translate-x-0 bg-white/70"
      : accent === "cyan"
        ? enabled
          ? "translate-x-5 bg-cyan"
          : "translate-x-0 bg-white/70"
        : enabled
          ? "translate-x-5 bg-accent"
          : "translate-x-0 bg-white/70";

  return (
    <button
      type="button"
      onClick={onToggle}
      disabled={disabled}
      className={cn(
        "w-full rounded-[24px] border px-4 py-4 text-left transition-all",
        accentClass,
        disabled ? "cursor-not-allowed opacity-45" : "hover:border-white/18 hover:bg-white/[0.06]",
      )}
    >
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-sm font-semibold text-white">{title}</p>
          <p className="mt-2 text-sm leading-6 text-text-secondary">{description}</p>
        </div>

        <div className="flex shrink-0 items-center gap-3">
          <span className="text-xs font-medium text-text-secondary">
            {enabled ? enabledLabel : disabledLabel}
          </span>
          <span
            className={cn(
              "relative inline-flex h-7 w-12 rounded-full border border-white/10 bg-white/10 p-1 transition-all",
              enabled ? "shadow-[0_0_0_4px_rgba(255,255,255,0.04)]" : "",
            )}
            aria-hidden="true"
          >
            <span
              className={cn(
                "h-5 w-5 rounded-full transition-transform duration-200",
                knobClass,
              )}
            />
          </span>
        </div>
      </div>
    </button>
  );
}

function UploadCard({
  title,
  hint,
  icon,
  children,
}: {
  title: string;
  hint: string;
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="overflow-hidden rounded-[26px] border border-white/10 bg-white/[0.04]">
      <div className="border-b border-white/10 px-5 py-5">
        <div className="flex items-center gap-3">
          <div className="flex h-11 w-11 items-center justify-center rounded-2xl border border-white/10 bg-white/6">
            {icon}
          </div>
          <div>
            <p className="text-base font-semibold text-white">{title}</p>
            <p className="mt-1 text-xs leading-5 text-text-secondary">{hint}</p>
          </div>
        </div>
      </div>
      <div className="px-5 py-5">{children}</div>
    </div>
  );
}

function SidebarCard({
  title,
  icon,
  children,
}: {
  title: string;
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-[26px] border border-white/10 bg-white/[0.04] p-5">
      <div className="flex items-center gap-2 text-sm font-semibold text-white">
        {icon}
        {title}
      </div>
      <div className="mt-4">{children}</div>
    </div>
  );
}

function SelectionBadge({ selected }: { selected: boolean }) {
  return (
    <span
      className={cn(
        "inline-flex h-8 w-8 items-center justify-center rounded-full border transition-all",
        selected
          ? "border-white/20 bg-white text-[#08111f]"
          : "border-white/10 bg-white/5 text-transparent",
      )}
    >
      <Check className="h-4 w-4" />
    </span>
  );
}

function BadgeChip({
  label,
  active,
  tone = "accent",
}: {
  label: string;
  active?: boolean;
  tone?: "accent" | "emerald";
}) {
  const className =
    tone === "emerald"
      ? active
        ? "border-emerald-400/24 bg-emerald-400/10 text-emerald-300"
        : "border-white/10 bg-white/6 text-text-secondary"
      : active
        ? "border-accent/24 bg-accent/10 text-accent"
        : "border-white/10 bg-white/6 text-text-secondary";

  return <span className={cn("rounded-full border px-3 py-1 text-xs font-medium", className)}>{label}</span>;
}

function MiniPill({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center rounded-full border border-white/10 bg-white/6 px-2.5 py-1 text-[11px] font-medium text-text-secondary">
      {children}
    </span>
  );
}

function SummaryLine({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/[0.04] px-4 py-3">
      <p className="text-[11px] uppercase tracking-[0.2em] text-white/45">{label}</p>
      <p className="mt-1 text-sm font-medium text-white">{value}</p>
    </div>
  );
}

function InlineLoading({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-2 rounded-2xl border border-white/10 bg-white/[0.04] px-4 py-3 text-sm text-text-secondary">
      <Loader2 className="h-4 w-4 animate-spin text-accent" />
      {label}
    </div>
  );
}

function EmptyUploadState({ label }: { label: string }) {
  return (
    <div className="rounded-[22px] border border-dashed border-white/10 bg-white/[0.03] px-4 py-6 text-sm text-text-secondary">
      {label}
    </div>
  );
}

function DisabledUploadState({ label }: { label: string }) {
  return (
    <div className="flex items-center justify-center rounded-[22px] border border-dashed border-white/10 bg-white/[0.03] px-4 py-5 text-center text-sm text-text-secondary">
      {label}
    </div>
  );
}

function AssetRow({
  asset,
  icon,
  sequence,
  deleting,
  moveUpDisabled,
  moveDownDisabled,
  onMoveUp,
  onMoveDown,
  onDelete,
}: {
  asset: SkillAsset;
  icon: React.ReactNode;
  sequence?: number;
  deleting: boolean;
  moveUpDisabled?: boolean;
  moveDownDisabled?: boolean;
  onMoveUp?: () => void;
  onMoveDown?: () => void;
  onDelete: () => void;
}) {
  const image = isImageAsset(asset);
  const video = isVideoAsset(asset);
  const audio = (asset.mimeType || "").startsWith("audio/") || asset.assetType.includes("audio");
  const hasReorder = typeof onMoveUp === "function" || typeof onMoveDown === "function";
  const hasSequence = typeof sequence === "number";

  return (
    <div className="overflow-hidden rounded-[22px] border border-white/10 bg-white/[0.04]">
      {asset.publicUrl && image ? (
        <div className="overflow-hidden border-b border-white/10 bg-black/30">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={asset.publicUrl} alt={asset.fileName} className="h-40 w-full object-cover" />
        </div>
      ) : null}
      {asset.publicUrl && video ? (
        <div className="overflow-hidden border-b border-white/10 bg-black/50">
          <video src={asset.publicUrl} controls preload="metadata" className="h-40 w-full bg-black object-cover" />
        </div>
      ) : null}
      {asset.publicUrl && audio ? (
        <div className="border-b border-white/10 bg-black/20 px-4 py-3">
          <audio src={asset.publicUrl} controls preload="metadata" className="h-8 w-full" />
        </div>
      ) : null}
      <div className="px-4 py-3">
        <div className="flex min-w-0 items-start gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-white/10 bg-white/6">
            {icon}
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium text-white">{asset.fileName}</p>
            <p className="mt-0.5 text-xs text-text-secondary">{asset.mimeType || asset.assetType}</p>
          </div>
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          {hasReorder ? (
            <>
              <button
                type="button"
                disabled={moveUpDisabled}
                onClick={onMoveUp}
                className="inline-flex h-8 w-8 items-center justify-center rounded-xl border border-white/10 bg-white/5 text-text-muted transition-all hover:border-white/20 hover:text-white disabled:opacity-40"
              >
                <ArrowUp className="h-3.5 w-3.5" />
              </button>
              <button
                type="button"
                disabled={moveDownDisabled}
                onClick={onMoveDown}
                className="inline-flex h-8 w-8 items-center justify-center rounded-xl border border-white/10 bg-white/5 text-text-muted transition-all hover:border-white/20 hover:text-white disabled:opacity-40"
              >
                <ArrowDown className="h-3.5 w-3.5" />
              </button>
            </>
          ) : null}
          {hasSequence ? (
            <span className="rounded-full border border-white/10 bg-white/6 px-2 py-0.5 text-[10px] font-semibold text-text-secondary">
              #{sequence}
            </span>
          ) : null}
          <span className="rounded-full border border-white/10 bg-white/6 px-2 py-0.5 text-[10px] font-semibold text-text-secondary">
            {audio ? "音频" : video ? "视频" : image ? "图片" : "素材"}
          </span>
          <div className="ml-auto flex items-center gap-2">
            <button
              type="button"
              disabled={deleting}
              onClick={onDelete}
              className="inline-flex h-8 w-8 items-center justify-center rounded-xl border border-white/10 bg-white/5 text-text-muted transition-all hover:border-danger hover:bg-danger/10 hover:text-danger disabled:opacity-50"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function PendingRow({
  label,
  icon,
  file,
  sequence,
  helperText,
  hideDelete,
  onDelete,
}: {
  label: string;
  icon: React.ReactNode;
  file?: File;
  sequence?: number;
  helperText?: string;
  hideDelete?: boolean;
  onDelete: () => void;
}) {
  const preview = useMemo(() => {
    if (!file || (!file.type.startsWith("image/") && !file.type.startsWith("video/"))) {
      return null;
    }
    return URL.createObjectURL(file);
  }, [file]);

  useEffect(() => {
    return () => {
      if (preview) {
        URL.revokeObjectURL(preview);
      }
    };
  }, [preview]);

  return (
    <div className="overflow-hidden rounded-[22px] border border-dashed border-white/12 bg-white/[0.03]">
      {preview && file?.type.startsWith("image/") ? (
        <div className="overflow-hidden border-b border-white/10 bg-black/30">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={preview} alt={label} className="h-40 w-full object-cover" />
        </div>
      ) : null}
      {preview && file?.type.startsWith("video/") ? (
        <div className="overflow-hidden border-b border-white/10 bg-black/50">
          <video src={preview} controls preload="metadata" className="h-40 w-full bg-black object-cover" />
        </div>
      ) : null}
      <div className="px-4 py-3">
        <div className="flex min-w-0 items-start gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-white/10 bg-white/6">
            {icon}
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium text-white">{label}</p>
            <p className="mt-0.5 text-xs text-text-secondary">{helperText || "正在上传到云端..."}</p>
          </div>
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          {typeof sequence === "number" ? (
            <span className="rounded-full border border-white/10 bg-white/6 px-2 py-0.5 text-[10px] font-semibold text-text-secondary">
              #{sequence}
            </span>
          ) : null}
          <span className="rounded-full border border-white/10 bg-white/6 px-2 py-0.5 text-[10px] font-semibold text-text-secondary">
            {file?.type.startsWith("video/") ? "视频" : file?.type.startsWith("image/") ? "图片" : file?.type.startsWith("audio/") ? "音频" : "素材"}
          </span>
          <div className="ml-auto">
            {hideDelete ? (
              <span className="inline-flex items-center rounded-full border border-white/10 bg-white/5 px-2.5 py-1 text-[10px] font-medium text-text-secondary">
                上传中
              </span>
            ) : (
              <button
                type="button"
                onClick={onDelete}
                className="inline-flex h-8 w-8 items-center justify-center rounded-xl border border-white/10 bg-white/5 text-text-muted transition-all hover:border-danger hover:bg-danger/10 hover:text-danger"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
