"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Check,
  Clock3,
  Cpu,
  FileText,
  Image as ImageIcon,
  Loader2,
  MessageSquareText,
  Sparkles,
  Trash2,
  Upload,
  Wand2,
  X,
} from "lucide-react";
import {
  createSkill,
  deleteSkillAsset,
  listAIModels,
  listSkillAssets,
  updateSkill,
  uploadSkillAsset,
} from "@/lib/services";
import type { AIModel, Skill, SkillAsset } from "@/lib/types";
import { cn } from "@/lib/utils";
import {
  formatDateTime,
  getModelReferenceLimit,
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
  outputType: string;
  modelName: string;
  executionTime: string;
  repeatDaily: boolean;
  storyboardEnabled: boolean;
  isEnabled: boolean;
};

type OutputOption = {
  value: SkillFormState["outputType"];
  label: string;
  hint: string;
  icon: React.ComponentType<{ className?: string }>;
  tone: string;
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
];

const EMPTY_FORM: SkillFormState = {
  name: "",
  description: "",
  promptTemplate: "",
  outputType: "图文模式",
  modelName: "",
  executionTime: "",
  repeatDaily: false,
  storyboardEnabled: true,
  isEnabled: true,
};

function toTimeOfDayValue(value?: string | null) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const offset = date.getTimezoneOffset();
  const normalized = new Date(date.getTime() - offset * 60 * 1000);
  return normalized.toISOString().slice(11, 19);
}

function buildNextExecutionISOString(timeOfDay?: string | null) {
  const value = (timeOfDay || "").trim();
  if (!value) {
    return null;
  }
  const parts = value.split(":").map((item) => Number(item));
  const [hours, minutes, seconds = 0] = parts;
  if ([hours, minutes, seconds].some((item) => Number.isNaN(item))) {
    return null;
  }
  const target = new Date();
  target.setHours(hours, minutes, seconds, 0);
  if (target.getTime() <= Date.now()) {
    target.setDate(target.getDate() + 1);
  }
  return target.toISOString();
}

function formatTimeOfDayLabel(value?: string | null) {
  const normalized = (value || "").trim();
  if (!normalized) {
    return "未设置";
  }
  const parts = normalized.split(":");
  if (parts.length === 2) {
    return `${parts[0]}:${parts[1]}:00`;
  }
  return normalized;
}

function isImageAsset(asset: SkillAsset) {
  return (asset.mimeType || "").startsWith("image/") || asset.assetType.includes("image");
}

function isTextAsset(asset: SkillAsset) {
  const mimeType = (asset.mimeType || "").toLowerCase();
  return mimeType.startsWith("text/") || asset.assetType.includes("text");
}

export function SkillEditorModal({
  isOpen,
  deviceId,
  skill,
  onClose,
  onSaved,
}: SkillEditorModalProps) {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<SkillFormState>(EMPTY_FORM);
  const [pendingImages, setPendingImages] = useState<File[]>([]);
  const [pendingTexts, setPendingTexts] = useState<File[]>([]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    if (!skill) {
      setForm(EMPTY_FORM);
      setPendingImages([]);
      setPendingTexts([]);
      return;
    }
    setForm({
      name: skill.name || "",
      description: skill.description || "",
      promptTemplate: skill.promptTemplate || "",
      outputType: normalizeSkillOutputLabel(skill.outputType),
      modelName: skill.modelName || "",
      executionTime: toTimeOfDayValue(skill.executionTime),
      repeatDaily: Boolean(skill.repeatDaily),
      storyboardEnabled: skill.storyboardEnabled !== false,
      isEnabled: Boolean(skill.isEnabled),
    });
    setPendingImages([]);
    setPendingTexts([]);
  }, [isOpen, skill]);

  const modelCategory = mapSkillOutputToModelCategory(form.outputType);
  const { data: models = [], isLoading: modelsLoading } = useQuery<AIModel[]>({
    queryKey: ["aiModels", modelCategory],
    queryFn: () => listAIModels({ modelType: modelCategory }),
    enabled: isOpen,
  });

  const { data: assets = [], isLoading: assetsLoading } = useQuery<SkillAsset[]>({
    queryKey: ["skillAssets", skill?.id],
    queryFn: () => listSkillAssets(skill!.id),
    enabled: isOpen && Boolean(skill?.id),
  });

  const availableModels = useMemo(
    () => models.filter((item) => item.isEnabled && item.category === modelCategory),
    [modelCategory, models],
  );
  const selectedModel = useMemo(
    () => availableModels.find((item) => item.modelName === form.modelName) ?? null,
    [availableModels, form.modelName],
  );
  const selectedOutput = useMemo(
    () => OUTPUT_OPTIONS.find((item) => item.value === form.outputType) ?? OUTPUT_OPTIONS[0],
    [form.outputType],
  );

  const imageAssets = useMemo(() => assets.filter(isImageAsset), [assets]);
  const textAssets = useMemo(() => assets.filter(isTextAsset), [assets]);
  const imageLimit = getModelReferenceLimit(form.outputType, selectedModel ?? undefined);
  const totalImageCount = imageAssets.length + pendingImages.length;
  const totalTextCount = textAssets.length + pendingTexts.length;

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = {
        name: form.name.trim(),
        description: form.description.trim(),
        outputType: form.outputType,
        modelName: form.modelName.trim(),
        deviceId,
        promptTemplate: form.promptTemplate.trim() || null,
        executionTime: buildNextExecutionISOString(form.executionTime),
        repeatDaily: Boolean(form.executionTime && form.repeatDaily),
        storyboardEnabled: form.storyboardEnabled,
        isEnabled: form.isEnabled,
      };

      if (!payload.name || !payload.description || !payload.outputType || !payload.modelName) {
        throw new Error("请先填写完整的技能名称、说明、输出格式和模型");
      }

      const savedSkill = skill?.id
        ? await updateSkill(skill.id, payload)
        : await createSkill(payload);

      for (const file of pendingImages) {
        await uploadSkillAsset(savedSkill.id, file, "reference_image");
      }
      for (const file of pendingTexts) {
        await uploadSkillAsset(savedSkill.id, file, "reference_text");
      }

      return savedSkill;
    },
    onSuccess: async (savedSkill) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["skills"] }),
        queryClient.invalidateQueries({ queryKey: ["skills", deviceId] }),
        queryClient.invalidateQueries({ queryKey: ["skillAssets", savedSkill.id] }),
      ]);
      onSaved();
      onClose();
    },
    onError: (error) => {
      window.alert(error instanceof Error ? error.message : "保存技能失败，请稍后重试");
    },
  });

  const deleteAssetMutation = useMutation({
    mutationFn: async (asset: SkillAsset) => {
      if (!skill?.id) {
        throw new Error("技能尚未创建，无法删除已上传素材");
      }
      await deleteSkillAsset(skill.id, asset.id);
    },
    onSuccess: async () => {
      if (!skill?.id) {
        return;
      }
      await queryClient.invalidateQueries({ queryKey: ["skillAssets", skill.id] });
    },
    onError: (error) => {
      window.alert(error instanceof Error ? error.message : "删除素材失败，请稍后重试");
    },
  });

  if (!isOpen) {
    return null;
  }

  const handleOutputChange = (nextOutputType: string) => {
    const nextCategory = mapSkillOutputToModelCategory(nextOutputType);
    setForm((current) => ({
      ...current,
      outputType: nextOutputType,
      modelName:
        mapSkillOutputToModelCategory(current.outputType) === nextCategory ? current.modelName : "",
    }));
  };

  const handleImageSelection = (fileList: FileList | null) => {
    const nextFiles = Array.from(fileList || []);
    if (nextFiles.length === 0) {
      return;
    }
    const remaining =
      imageLimit > 0 ? Math.max(0, imageLimit - totalImageCount) : Number.MAX_SAFE_INTEGER;
    if (remaining <= 0) {
      window.alert(`当前模型最多支持 ${imageLimit} 张参考图，请先删除部分图片。`);
      return;
    }
    const accepted = nextFiles.slice(0, remaining);
    if (accepted.length < nextFiles.length) {
      window.alert(`当前模型最多支持 ${imageLimit} 张参考图，已仅保留前 ${accepted.length} 张。`);
    }
    setPendingImages((current) => [...current, ...accepted]);
  };

  const handleTextSelection = (fileList: FileList | null) => {
    const nextFiles = Array.from(fileList || []);
    if (nextFiles.length === 0) {
      return;
    }
    setPendingTexts((current) => [...current, ...nextFiles]);
  };

  const flowSteps = form.storyboardEnabled
    ? [
        "客户输入图文和提示词",
        "系统先做分镜优化",
        selectedModel?.modelName || "最终模型待选择",
        form.executionTime
          ? `每天 ${formatTimeOfDayLabel(form.executionTime)} 发布`
          : "手动触发",
      ]
    : [
        "客户输入图文和提示词",
        "跳过分镜，直接执行",
        selectedModel?.modelName || "最终模型待选择",
        form.executionTime
          ? `每天 ${formatTimeOfDayLabel(form.executionTime)} 发布`
          : "手动触发",
      ];

  return (
    <div className="fixed inset-0 z-[90] overflow-y-auto bg-[#050814]/85 px-4 py-4 backdrop-blur-xl sm:px-6 sm:py-6">
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className="absolute left-[-10%] top-[-8%] h-64 w-64 rounded-full bg-accent/14 blur-3xl" />
        <div className="absolute bottom-[-8%] right-[-5%] h-64 w-64 rounded-full bg-cyan/12 blur-3xl" />
      </div>

      <div className="relative mx-auto flex min-h-[calc(100vh-2rem)] max-w-[1260px] items-center">
        <div className="w-full overflow-hidden rounded-[30px] border border-white/10 bg-[#08111f]/96 shadow-[0_28px_100px_rgba(0,0,0,0.52)]">
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
                    先选产出和模型，再决定是否开启分镜，最后补时间和素材。
                  </p>
                </div>
                <div className="flex flex-wrap gap-2">
                  <BadgeChip label={`节点 ${deviceId.slice(0, 8)}`} />
                  <BadgeChip label={form.storyboardEnabled ? "分镜优化开启" : "分镜优化关闭"} active={form.storyboardEnabled} />
                  <BadgeChip label={form.isEnabled ? "技能启用中" : "技能已暂停"} active={form.isEnabled} tone="emerald" />
                </div>
              </div>

              <button
                type="button"
                onClick={onClose}
                className="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-white/5 text-text-muted transition-all hover:border-white/20 hover:bg-white/10 hover:text-white"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
          </div>

          <div className="max-h-[calc(100vh-14rem)] overflow-y-auto">
            <div className="grid gap-0 xl:grid-cols-[minmax(0,1fr)_292px]">
              <div className="px-6 py-6 sm:px-8">
                <div className="space-y-5">
                  <SectionCard
                    title="基础设定"
                    description="先把这条技能的目标和最终产出说清楚。"
                  >
                    <div className="grid gap-4 lg:grid-cols-[0.95fr_1.05fr]">
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
                        <div className="grid gap-3 sm:grid-cols-3">
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
                    </div>

                    <div className="grid gap-4 lg:grid-cols-[0.95fr_1.05fr]">
                      <label className="space-y-2.5">
                        <span className="text-sm font-medium text-white">技能说明</span>
                        <textarea
                          value={form.description}
                          onChange={(event) =>
                            setForm((current) => ({ ...current, description: event.target.value }))
                          }
                          rows={5}
                          placeholder="描述内容目标、受众、语气、场景和限制。"
                          className="w-full rounded-[24px] border border-white/10 bg-white/6 px-4 py-3 text-sm leading-6 text-white outline-none transition-all placeholder:text-text-muted focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                        />
                      </label>

                      <label className="space-y-2.5">
                        <span className="text-sm font-medium text-white">任务提示词</span>
                        <textarea
                          value={form.promptTemplate}
                          onChange={(event) =>
                            setForm((current) => ({ ...current, promptTemplate: event.target.value }))
                          }
                          rows={5}
                          placeholder="告诉系统重点表达什么，比如镜头感、文案节奏、品牌边界和禁用词。"
                          className="w-full rounded-[24px] border border-white/10 bg-white/6 px-4 py-3 text-sm leading-6 text-white outline-none transition-all placeholder:text-text-muted focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                        />
                      </label>
                    </div>
                  </SectionCard>

                  <SectionCard
                    title="执行方式"
                    description="这里决定技能是否生效，以及最终交给哪个模型执行。"
                  >
                    <div className="grid gap-4 lg:grid-cols-2">
                      <SwitchCard
                        title="AI 分镜优化"
                        description="开启后，系统会先整理图片、文本和提示词，再交给最终模型。"
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
                        description="关闭后保留配置，但不会进入自动调度。"
                        enabled={form.isEnabled}
                        enabledLabel="已启用"
                        disabledLabel="已暂停"
                        accent="emerald"
                        onToggle={() =>
                          setForm((current) => ({ ...current, isEnabled: !current.isEnabled }))
                        }
                      />
                    </div>

                      <div className="rounded-[24px] border border-white/10 bg-[#0d1729] p-4">
                        <div className="flex items-center gap-2 text-sm font-medium text-white">
                          <Cpu className="h-4 w-4 text-cyan" />
                          最终执行模型
                        </div>
                        <p className="mt-2 text-sm leading-6 text-text-secondary">
                          这里只选一个最终模型，方便按质量、速度和成本做取舍。分镜模型在系统侧单独配置。
                        </p>
                      </div>

                    {modelsLoading ? (
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
                          return (
                            <button
                              key={model.id}
                              type="button"
                              onClick={() =>
                                setForm((current) => ({ ...current, modelName: model.modelName }))
                              }
                              className={cn(
                                "rounded-[26px] border p-4 text-left transition-all",
                                selected
                                  ? "border-accent/50 bg-[linear-gradient(160deg,rgba(177,73,255,0.15),rgba(0,245,212,0.04))] shadow-[0_18px_45px_rgba(177,73,255,0.15)]"
                                  : "border-white/10 bg-white/[0.04] hover:border-white/18 hover:bg-white/[0.06]",
                              )}
                            >
                              <div className="flex items-start justify-between gap-3">
                                <div className="min-w-0">
                                  <div className="flex flex-wrap items-center gap-2">
                                    <p className="text-base font-semibold text-white">{model.modelName}</p>
                                    <MiniPill>{model.vendor}</MiniPill>
                                  </div>
                                  <p className="mt-2 text-sm leading-6 text-text-secondary">
                                    {model.description || "暂无模型说明，可在后台补充。"}
                                  </p>
                                </div>
                                <SelectionBadge selected={selected} />
                              </div>
                              <div className="mt-4 grid gap-3 sm:grid-cols-2">
                                <MetricBlock
                                  label="计费"
                                  value={model.billingMode || "待配置"}
                                />
                                <MetricBlock
                                  label="参考图"
                                  value={modelLimit > 0 ? `最多 ${modelLimit} 张` : "不限或未声明"}
                                />
                              </div>
                            </button>
                          );
                        })}
                      </div>
                    )}
                  </SectionCard>

                  <SectionCard
                    title="执行时间"
                    description="这里只填时分秒。重复的意思就是每天这个时间执行。"
                  >
                    <div className="grid gap-4 lg:grid-cols-[0.9fr_1.1fr]">
                      <label className="space-y-2.5">
                        <span className="text-sm font-medium text-white">每天执行时间</span>
                        <input
                          type="time"
                          step={1}
                          value={form.executionTime}
                          onChange={(event) =>
                            setForm((current) => ({ ...current, executionTime: event.target.value }))
                          }
                          className="w-full rounded-2xl border border-white/10 bg-white/6 px-4 py-3 text-sm text-white outline-none transition-all focus:border-accent/40 focus:bg-white/8 focus:ring-4 focus:ring-accent/10"
                        />
                        <p className="text-xs leading-5 text-text-secondary">
                          不填就是手动触发。填写后，系统会自动计算下一次执行时间。
                        </p>
                      </label>

                      <div className="space-y-4">
                        <SwitchCard
                          title="每天按时执行"
                          description="开启后每天这个时间执行；关闭则只跑下一次。"
                          enabled={form.repeatDaily}
                          enabledLabel="每日执行"
                          disabledLabel="只执行一次"
                          accent="cyan"
                          disabled={!form.executionTime}
                          onToggle={() =>
                            setForm((current) => ({ ...current, repeatDaily: !current.repeatDaily }))
                          }
                        />
                        <div className="rounded-[24px] border border-white/10 bg-[#0d1729] p-4">
                          <div className="flex items-center gap-2 text-sm font-medium text-white">
                            <Clock3 className="h-4 w-4 text-accent" />
                            当前时间策略
                          </div>
                          <div className="mt-3 space-y-2 text-sm leading-6 text-text-secondary">
                            {form.executionTime ? (
                              <>
                                <p>执行时间：{formatTimeOfDayLabel(form.executionTime)}</p>
                                <p>
                                  {form.repeatDaily
                                    ? "系统会每天按这个时间执行，OmniDrive 提前 30 分钟生成。"
                                    : "系统会按下一个到来的这个时间执行一次，OmniDrive 提前 30 分钟生成。"}
                                </p>
                              </>
                            ) : (
                              <p>当前未设置执行时间，这条技能只会作为手动执行能力存在。</p>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>
                  </SectionCard>

                  <SectionCard
                    title="参考素材"
                    description="图片约束画面风格，文本约束结构和卖点。"
                  >
                    <div className="grid gap-4 lg:grid-cols-2">
                      <UploadCard
                        title="参考图片"
                        hint={
                          imageLimit > 0
                            ? `当前模型最多支持 ${imageLimit} 张参考图，已准备 ${totalImageCount} 张。`
                            : `当前模型未声明参考图上限，已准备 ${totalImageCount} 张。`
                        }
                        icon={<ImageIcon className="h-5 w-5 text-cyan" />}
                      >
                        <label className="flex cursor-pointer items-center justify-center rounded-[22px] border border-dashed border-cyan/30 bg-cyan/10 px-4 py-5 text-center transition-all hover:border-cyan/50 hover:bg-cyan/14">
                          <div>
                            <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-2xl bg-cyan/15 text-cyan">
                              <Upload className="h-5 w-5" />
                            </div>
                            <p className="mt-3 text-sm font-semibold text-white">上传图片</p>
                            <p className="mt-1 text-xs text-text-secondary">支持多张，保存时一起提交。</p>
                          </div>
                          <input
                            type="file"
                            accept="image/*"
                            multiple
                            className="hidden"
                            onChange={(event) => handleImageSelection(event.target.files)}
                          />
                        </label>

                        <div className="mt-4 space-y-3">
                          {assetsLoading ? <InlineLoading label="正在读取图片素材..." /> : null}
                          {imageAssets.map((asset) => (
                            <AssetRow
                              key={asset.id}
                              asset={asset}
                              icon={<ImageIcon className="h-4 w-4 text-cyan" />}
                              deleting={deleteAssetMutation.isPending}
                              onDelete={() => deleteAssetMutation.mutate(asset)}
                            />
                          ))}
                          {pendingImages.map((file) => (
                            <PendingRow
                              key={`${file.name}-${file.size}`}
                              label={file.name}
                              icon={<ImageIcon className="h-4 w-4 text-cyan" />}
                              file={file}
                              onDelete={() =>
                                setPendingImages((current) =>
                                  current.filter(
                                    (item) => item.name !== file.name || item.size !== file.size,
                                  ),
                                )
                              }
                            />
                          ))}
                          {!imageAssets.length && !pendingImages.length ? (
                            <EmptyUploadState label="还没有图片素材。" />
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
                            onChange={(event) => handleTextSelection(event.target.files)}
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
                          {pendingTexts.map((file) => (
                            <PendingRow
                              key={`${file.name}-${file.size}`}
                              label={file.name}
                              icon={<FileText className="h-4 w-4 text-amber-200" />}
                              file={file}
                              onDelete={() =>
                                setPendingTexts((current) =>
                                  current.filter(
                                    (item) => item.name !== file.name || item.size !== file.size,
                                  ),
                                )
                              }
                            />
                          ))}
                          {!textAssets.length && !pendingTexts.length ? (
                            <EmptyUploadState label="还没有文本资料。" />
                          ) : null}
                        </div>
                      </UploadCard>
                    </div>
                  </SectionCard>
                </div>
              </div>

              <aside className="border-t border-white/10 bg-white/[0.03] px-6 py-6 xl:border-l xl:border-t-0 xl:px-5">
                <div className="space-y-4">
                  <SidebarCard title="执行概览" icon={<Sparkles className="h-4 w-4 text-accent" />}>
                    <div className="grid gap-3">
                      <SummaryLine label="输出类型" value={selectedOutput.label} />
                      <SummaryLine label="最终模型" value={selectedModel?.modelName || "未选择"} />
                      <SummaryLine label="执行时间" value={form.executionTime ? formatTimeOfDayLabel(form.executionTime) : "手动触发"} />
                      <SummaryLine label="参考素材" value={`${totalImageCount} 图 / ${totalTextCount} 文`} />
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

                  {skill?.nextRunAt ? (
                    <SidebarCard title="当前技能进度" icon={<Clock3 className="h-4 w-4 text-amber-200" />}>
                      <p className="text-sm leading-6 text-text-secondary">
                        下次执行时间是 <span className="font-medium text-white">{formatDateTime(skill.nextRunAt)}</span>，
                        系统会在这个时间点前 30 分钟进入生成链路。
                      </p>
                    </SidebarCard>
                  ) : null}
                </div>
              </aside>
            </div>
          </div>

          <div className="flex items-center justify-end gap-3 border-t border-white/10 bg-[#091221]/92 px-6 py-4 sm:px-8">
            <button
              type="button"
              onClick={onClose}
              className="rounded-2xl border border-white/10 bg-white/5 px-4 py-2.5 text-sm font-medium text-text-primary transition-all hover:border-white/20 hover:bg-white/8 hover:text-white"
            >
              取消
            </button>
            <button
              type="button"
              onClick={() => saveMutation.mutate()}
              disabled={saveMutation.isPending}
              className="inline-flex items-center gap-2 rounded-2xl bg-gradient-to-r from-accent via-pink to-cyan px-5 py-2.5 text-sm font-semibold text-white shadow-[0_16px_40px_rgba(177,73,255,0.22)] transition-all hover:scale-[1.01] hover:shadow-[0_20px_48px_rgba(177,73,255,0.28)] disabled:cursor-not-allowed disabled:opacity-60"
            >
              {saveMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {skill ? "保存技能" : "创建技能"}
            </button>
          </div>
        </div>
      </div>
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
      <div className="border-b border-white/10 bg-white/[0.03] px-5 py-5 sm:px-6">
        <h4 className="text-lg font-semibold text-white">{title}</h4>
        <p className="mt-2 text-sm leading-6 text-text-secondary">{description}</p>
      </div>
      <div className="px-5 py-5 sm:px-6">{children}</div>
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

function MetricBlock({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 px-3 py-3">
      <p className="text-[11px] uppercase tracking-[0.2em] text-white/45">{label}</p>
      <p className="mt-1 text-sm font-medium text-white">{value}</p>
    </div>
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

function AssetRow({
  asset,
  icon,
  deleting,
  onDelete,
}: {
  asset: SkillAsset;
  icon: React.ReactNode;
  deleting: boolean;
  onDelete: () => void;
}) {
  const image = isImageAsset(asset);

  return (
    <div className="flex items-center justify-between rounded-[22px] border border-white/10 bg-white/[0.04] px-4 py-3">
      <div className="flex min-w-0 items-center gap-3">
        {image && asset.publicUrl ? (
          <div className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-2xl border border-white/10 bg-black/30">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={asset.publicUrl} alt={asset.fileName} className="h-full w-full object-cover" />
          </div>
        ) : (
          <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-white/6">
            {icon}
          </div>
        )}
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-white">{asset.fileName}</p>
          <p className="mt-1 text-xs text-text-secondary">{asset.mimeType || asset.assetType}</p>
        </div>
      </div>

      <div className="flex items-center gap-2">
        {asset.publicUrl ? (
          <a
            href={asset.publicUrl}
            target="_blank"
            rel="noreferrer"
            className="rounded-full border border-white/10 bg-white/6 px-3 py-1.5 text-xs text-text-secondary transition-all hover:border-cyan/40 hover:text-cyan"
          >
            预览
          </a>
        ) : null}
        <button
          type="button"
          disabled={deleting}
          onClick={onDelete}
          className="inline-flex h-10 w-10 items-center justify-center rounded-2xl border border-white/10 bg-white/5 text-text-muted transition-all hover:border-danger hover:bg-danger/10 hover:text-danger disabled:opacity-50"
        >
          <Trash2 className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}

function PendingRow({
  label,
  icon,
  file,
  onDelete,
}: {
  label: string;
  icon: React.ReactNode;
  file?: File;
  onDelete: () => void;
}) {
  const [preview, setPreview] = useState<string | null>(null);

  useEffect(() => {
    if (!file || !file.type.startsWith("image/")) {
      setPreview(null);
      return;
    }
    const url = URL.createObjectURL(file);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  return (
    <div className="flex items-center justify-between rounded-[22px] border border-dashed border-white/12 bg-white/[0.03] px-4 py-3">
      <div className="flex min-w-0 items-center gap-3">
        {preview ? (
          <div className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-2xl border border-white/10 bg-black/30">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={preview} alt={label} className="h-full w-full object-cover" />
          </div>
        ) : (
          <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-white/6">
            {icon}
          </div>
        )}
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-white">{label}</p>
          <p className="mt-1 text-xs text-text-secondary">待上传，保存技能后会一并提交。</p>
        </div>
      </div>

      <button
        type="button"
        onClick={onDelete}
        className="inline-flex h-10 w-10 items-center justify-center rounded-2xl border border-white/10 bg-white/5 text-text-muted transition-all hover:border-danger hover:bg-danger/10 hover:text-danger"
      >
        <Trash2 className="h-4 w-4" />
      </button>
    </div>
  );
}
