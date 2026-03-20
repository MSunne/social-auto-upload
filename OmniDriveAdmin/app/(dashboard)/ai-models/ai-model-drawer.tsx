"use client";

import { useState, useEffect } from "react";
import { useCreateAIModel, useUpdateAIModel } from "@/lib/hooks/useAIModels";
import { AIModel } from "@/lib/types";
import { X } from "lucide-react";

interface AIModelDrawerProps {
  model: AIModel | null;
  isOpen: boolean;
  onClose: () => void;
  isCreate: boolean;
}

const DEFAULT_FORM = {
  id: "",
  vendor: "",
  modelName: "",
  category: "image",
  billingMode: "per_call",
  baseUrl: "",
  apiKey: "",
  rawRate: "",
  billingAmount: "",
  chatInputRawRate: "",
  chatOutputRawRate: "",
  chatInputBillingAmount: "",
  chatOutputBillingAmount: "",
  description: "",
  isEnabled: true,
  imageReferenceLimit: "",
  imageSupportedSizes: "auto, 1920x1080",
  videoReferenceLimit: "",
  videoSupportedResolutions: "",
  videoSupportedDurations: "",
};

const CATEGORY_LABELS: Record<string, string> = {
  image: "作图",
  video: "做视频",
  chat: "聊天",
  music: "音乐",
};

const BILLING_MODE_LABELS: Record<string, string> = {
  per_call: "按次计费",
  per_second: "按秒计费",
  per_token: "按 Token 计费",
};

function toCommaSeparated(values?: string[]) {
  return values && values.length > 0 ? values.join(", ") : "";
}

function parseCommaSeparated(input: string) {
  return input
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function toFormState(model: AIModel | null) {
  if (!model) {
    return DEFAULT_FORM;
  }
  return {
    id: model.id,
    vendor: model.vendor,
    modelName: model.modelName,
    category: model.category,
    billingMode: model.billingMode || (model.category === "chat" ? "per_token" : "per_call"),
    baseUrl: model.baseUrl || "",
    apiKey: model.apiKey || "",
    rawRate: model.rawRate !== undefined ? String(model.rawRate) : "",
    billingAmount:
      model.billingAmount !== undefined ? String(model.billingAmount) : "",
    chatInputRawRate:
      model.chatInputRawRate !== undefined
        ? String(model.chatInputRawRate)
        : "",
    chatOutputRawRate:
      model.chatOutputRawRate !== undefined
        ? String(model.chatOutputRawRate)
        : "",
    chatInputBillingAmount:
      model.chatInputBillingAmount !== undefined
        ? String(model.chatInputBillingAmount)
        : "",
    chatOutputBillingAmount:
      model.chatOutputBillingAmount !== undefined
        ? String(model.chatOutputBillingAmount)
        : "",
    description: model.description || "",
    isEnabled: model.isEnabled,
    imageReferenceLimit:
      model.imageReferenceLimit !== undefined
        ? String(model.imageReferenceLimit)
        : "",
    imageSupportedSizes: toCommaSeparated(model.imageSupportedSizes),
    videoReferenceLimit:
      model.videoReferenceLimit !== undefined
        ? String(model.videoReferenceLimit)
        : "",
    videoSupportedResolutions: toCommaSeparated(
      model.videoSupportedResolutions,
    ),
    videoSupportedDurations: toCommaSeparated(model.videoSupportedDurations),
  };
}

function parseOptionalNumber(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
}

export function AIModelDrawer({
  model,
  isOpen,
  onClose,
  isCreate,
}: AIModelDrawerProps) {
  const createModel = useCreateAIModel();
  const updateModel = useUpdateAIModel();

  const [form, setForm] = useState(() => toFormState(model));

  useEffect(() => {
    if (isOpen) {
      setForm(toFormState(model));
    }
  }, [isOpen, model]);

  const chatInputRawRate = parseOptionalNumber(form.chatInputRawRate);
  const chatOutputRawRate = parseOptionalNumber(form.chatOutputRawRate);
  const chatInputBillingAmount = parseOptionalNumber(
    form.chatInputBillingAmount,
  );
  const chatOutputBillingAmount = parseOptionalNumber(
    form.chatOutputBillingAmount,
  );

  const buildPricingPayload = () => {
    if (form.billingMode !== "per_token") {
      return {};
    }
    return {
      chatInputRawRate,
      chatOutputRawRate,
      chatInputBillingAmount,
      chatOutputBillingAmount,
    };
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const rawRate = parseOptionalNumber(form.rawRate);
    const billingAmount = parseOptionalNumber(form.billingAmount);
    const imageReferenceLimit = parseOptionalNumber(form.imageReferenceLimit);
    const videoReferenceLimit = parseOptionalNumber(form.videoReferenceLimit);

    const payload = {
      vendor: form.vendor.trim(),
      modelName: form.modelName.trim(),
      category: form.category,
      billingMode: form.billingMode,
      modelType: form.category,
      baseUrl: form.baseUrl.trim() || undefined,
      apiKey: isCreate ? form.apiKey.trim() || undefined : form.apiKey.trim(),
      rawRate: form.billingMode === "per_token" ? undefined : rawRate,
      billingAmount: form.billingMode === "per_token" ? undefined : billingAmount,
      description: form.description.trim() || undefined,
      pricingPayload: buildPricingPayload(),
      chatInputRawRate:
        form.billingMode === "per_token" ? chatInputRawRate : undefined,
      chatOutputRawRate:
        form.billingMode === "per_token" ? chatOutputRawRate : undefined,
      chatInputBillingAmount:
        form.billingMode === "per_token"
          ? chatInputBillingAmount
          : undefined,
      chatOutputBillingAmount:
        form.billingMode === "per_token"
          ? chatOutputBillingAmount
          : undefined,
      imageReferenceLimit: form.category === "image" ? imageReferenceLimit : 0,
      imageSupportedSizes:
        form.category === "image"
          ? parseCommaSeparated(form.imageSupportedSizes)
          : [],
      videoReferenceLimit: form.category === "video" ? videoReferenceLimit : 0,
      videoSupportedResolutions:
        form.category === "video"
          ? parseCommaSeparated(form.videoSupportedResolutions)
          : [],
      videoSupportedDurations:
        form.category === "video"
          ? parseCommaSeparated(form.videoSupportedDurations)
          : [],
      isEnabled: form.isEnabled,
    };

    try {
      if (isCreate) {
        await createModel.mutateAsync({
          id: form.id || undefined,
          ...payload,
        });
      } else if (model) {
        await updateModel.mutateAsync({
          modelId: model.id,
          payload,
        });
      }
      onClose();
    } catch (error) {
      alert(error instanceof Error ? error.message : "操作失败，请重试");
    }
  };

  if (!isOpen) return null;

  const isPending = createModel.isPending || updateModel.isPending;

  return (
    <>
      <div className="fixed inset-0 bg-black/50 z-40" onClick={onClose} />
      <div className="fixed inset-y-0 right-0 w-full max-w-lg bg-[var(--color-background)] border-l border-[var(--color-border)] shadow-2xl z-50 flex flex-col">
        <div className="flex items-center justify-between p-5 border-b border-[var(--color-border)] bg-[var(--color-background)]/90 backdrop-blur sticky top-0">
          <h2 className="text-lg font-semibold">
            {isCreate ? "新增 AI 模型配置" : `编辑模型: ${model?.modelName}`}
          </h2>
          <button
            onClick={onClose}
            className="p-2 hover:bg-[var(--color-surface)] rounded-lg transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <form
          onSubmit={handleSubmit}
          className="flex-1 overflow-y-auto p-5 space-y-5"
        >
          {isCreate && (
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                模型 ID <span className="text-xs">(留空自动生成)</span>
              </label>
              <input
                type="text"
                value={form.id}
                onChange={(e) => setForm((f) => ({ ...f, id: e.target.value }))}
                placeholder="e.g. flux-pro-v1"
                className="w-full px-3 py-2 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)] font-mono"
              />
            </div>
          )}

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                厂商 / 渠道 *
              </label>
              <input
                required
                type="text"
                value={form.vendor}
                onChange={(e) =>
                  setForm((f) => ({ ...f, vendor: e.target.value }))
                }
                placeholder="e.g. openai / fal / runway"
                className="w-full px-3 py-2 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
              />
            </div>
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                模型名称 *
              </label>
              <input
                required
                type="text"
                value={form.modelName}
                onChange={(e) =>
                  setForm((f) => ({ ...f, modelName: e.target.value }))
                }
                placeholder="e.g. gpt-4o"
                className="w-full px-3 py-2 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                模型类型 / 用途
              </label>
              <select
                value={form.category}
                onChange={(e) =>
                  setForm((f) => {
                    const nextCategory = e.target.value;
                    let nextBillingMode = f.billingMode;
                    if (nextCategory === "chat" && f.billingMode === "per_call") {
                      nextBillingMode = "per_token";
                    }
                    if (nextCategory !== "chat" && f.billingMode === "per_token") {
                      nextBillingMode = "per_call";
                    }
                    return {
                      ...f,
                      category: nextCategory,
                      billingMode: nextBillingMode,
                    };
                  })
                }
                className="w-full px-3 py-2 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
              >
                <option value="image">作图 (image)</option>
                <option value="video">做视频 (video)</option>
                <option value="chat">聊天 (chat)</option>
                <option value="music">音乐 (music)</option>
              </select>
            </div>
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                计费模式
              </label>
              <select
                value={form.billingMode}
                onChange={(e) =>
                  setForm((f) => ({ ...f, billingMode: e.target.value }))
                }
                className="w-full px-3 py-2 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
              >
                <option value="per_call">按次计费 (per_call)</option>
                <option value="per_second">按秒计费 (per_second)</option>
                <option value="per_token">按 Token 计费 (per_token)</option>
              </select>
            </div>
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                启用状态
              </label>
              <div className="flex items-center h-10">
                <button
                  type="button"
                  onClick={() =>
                    setForm((f) => ({ ...f, isEnabled: !f.isEnabled }))
                  }
                  className={`relative inline-flex items-center h-6 rounded-full w-11 transition-colors ${form.isEnabled ? "bg-green-500" : "bg-[var(--color-border)]"}`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${form.isEnabled ? "translate-x-6" : "translate-x-1"}`}
                  />
                </button>
                <span className="ml-2.5 text-sm">
                  {form.isEnabled ? "启用 (enabled)" : "停用 (disabled)"}
                </span>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="col-span-2">
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                Base URL *
              </label>
              <input
                required
                type="text"
                value={form.baseUrl}
                onChange={(e) =>
                  setForm((f) => ({ ...f, baseUrl: e.target.value }))
                }
                placeholder="https://api.example.com/v1/generate"
                className="w-full px-3 py-2 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)] font-mono"
              />
            </div>
            <div className="col-span-2">
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                模型 Key
              </label>
              <input
                type="text"
                value={form.apiKey}
                onChange={(e) =>
                  setForm((f) => ({ ...f, apiKey: e.target.value }))
                }
                placeholder="留空则使用系统默认固定 key"
                autoComplete="off"
                className="w-full px-3 py-2 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)] font-mono"
              />
              <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                为当前模型单独指定调用 key。留空时会回退到系统默认 key。
                {!isCreate && " 编辑时清空后保存，也会回退到系统默认 key。"}
              </p>
            </div>
            {form.billingMode !== "per_token" && (
              <>
                <div>
                  <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                    {form.billingMode === "per_second"
                      ? "每秒原始费率"
                      : "按次原始费率"}
                  </label>
                  <input
                    type="number"
                    step="0.000001"
                    min="0"
                    value={form.rawRate}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, rawRate: e.target.value }))
                    }
                    placeholder="例如 0.12"
                    className="w-full px-3 py-2 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
                  />
                </div>
                <div>
                  <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                    {form.billingMode === "per_second"
                      ? "每秒计费金额"
                      : "按次计费金额"}
                  </label>
                  <input
                    type="number"
                    step="0.000001"
                    min="0"
                    value={form.billingAmount}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, billingAmount: e.target.value }))
                    }
                    placeholder="例如 0.2"
                    className="w-full px-3 py-2 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
                  />
                </div>
              </>
            )}
          </div>

          {form.billingMode === "per_token" && (
            <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]/50 p-4 space-y-4">
              <div className="text-sm font-medium text-[var(--color-text-primary)]">
                {BILLING_MODE_LABELS[form.billingMode]}配置
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                    输入 Token 原始费率
                  </label>
                  <input
                    type="number"
                    step="0.000001"
                    min="0"
                    value={form.chatInputRawRate}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        chatInputRawRate: e.target.value,
                      }))
                    }
                    placeholder="例如 0.000001"
                    className="w-full px-3 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
                  />
                </div>
                <div>
                  <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                    输出 Token 原始费率
                  </label>
                  <input
                    type="number"
                    step="0.000001"
                    min="0"
                    value={form.chatOutputRawRate}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        chatOutputRawRate: e.target.value,
                      }))
                    }
                    placeholder="例如 0.000002"
                    className="w-full px-3 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
                  />
                </div>
                <div>
                  <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                    输入 Token 计费金额
                  </label>
                  <input
                    type="number"
                    step="0.000001"
                    min="0"
                    value={form.chatInputBillingAmount}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        chatInputBillingAmount: e.target.value,
                      }))
                    }
                    placeholder="例如 0.000001"
                    className="w-full px-3 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
                  />
                </div>
                <div>
                  <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                    输出 Token 计费金额
                  </label>
                  <input
                    type="number"
                    step="0.000001"
                    min="0"
                    value={form.chatOutputBillingAmount}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        chatOutputBillingAmount: e.target.value,
                      }))
                    }
                    placeholder="例如 0.000002"
                    className="w-full px-3 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
                  />
                </div>
              </div>
              <p className="text-xs text-[var(--color-text-secondary)]">
                按 Token 计费时，输入 / 输出 token 的原始费率和计费金额分开配置。
                当前只沉淀模型配置，真实扣费逻辑暂不变更。
              </p>
            </div>
          )}

          <div>
            <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
              描述备注
            </label>
            <textarea
              value={form.description}
              onChange={(e) =>
                setForm((f) => ({ ...f, description: e.target.value }))
              }
              placeholder="模型用途说明、限制事项等..."
              className="w-full px-3 py-2 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)] resize-none h-20"
            />
          </div>

          {form.category === "image" && (
            <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]/50 p-4 space-y-4">
              <div className="text-sm font-medium text-[var(--color-text-primary)]">
                {CATEGORY_LABELS[form.category]}模型扩展能力
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                    支持参考图数量
                  </label>
                  <input
                    type="number"
                    min="0"
                    value={form.imageReferenceLimit}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        imageReferenceLimit: e.target.value,
                      }))
                    }
                    placeholder="例如 4"
                    className="w-full px-3 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
                  />
                </div>
                <div className="col-span-2">
                  <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                    支持生成尺寸
                  </label>
                  <input
                    type="text"
                    value={form.imageSupportedSizes}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        imageSupportedSizes: e.target.value,
                      }))
                    }
                    placeholder="auto, 1920x1080, 1024x1024"
                    className="w-full px-3 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
                  />
                  <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                    多个尺寸用英文逗号分隔，例如 `auto, 1920x1080`。
                  </p>
                </div>
              </div>
            </div>
          )}

          {form.category === "video" && (
            <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]/50 p-4 space-y-4">
              <div className="text-sm font-medium text-[var(--color-text-primary)]">
                {CATEGORY_LABELS[form.category]}模型扩展能力
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                    支持参考图数量
                  </label>
                  <input
                    type="number"
                    min="0"
                    value={form.videoReferenceLimit}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        videoReferenceLimit: e.target.value,
                      }))
                    }
                    placeholder="例如 2"
                    className="w-full px-3 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
                  />
                </div>
                <div>
                  <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                    生成时长
                  </label>
                  <input
                    type="text"
                    value={form.videoSupportedDurations}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        videoSupportedDurations: e.target.value,
                      }))
                    }
                    placeholder="5s, 10s"
                    className="w-full px-3 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
                  />
                </div>
                <div className="col-span-2">
                  <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                    分辨率
                  </label>
                  <input
                    type="text"
                    value={form.videoSupportedResolutions}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        videoSupportedResolutions: e.target.value,
                      }))
                    }
                    placeholder="1280x720, 1920x1080"
                    className="w-full px-3 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
                  />
                  <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                    多个分辨率用英文逗号分隔。
                  </p>
                </div>
              </div>
            </div>
          )}

          <div className="pt-4 border-t border-[var(--color-border)] flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2.5 border border-[var(--color-border)] rounded-lg text-sm font-medium hover:bg-[var(--color-surface)] transition-colors"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={isPending}
              className="flex-1 px-4 py-2.5 bg-[var(--color-accent)] text-white rounded-lg text-sm font-medium hover:bg-[var(--color-accent)]/90 transition-colors disabled:opacity-50"
            >
              {isPending ? "保存中..." : isCreate ? "创建模型" : "保存更改"}
            </button>
          </div>
        </form>
      </div>
    </>
  );
}
