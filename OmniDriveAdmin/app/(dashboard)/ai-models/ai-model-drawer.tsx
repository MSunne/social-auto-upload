"use client";

import { useState } from "react";
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
  description: "",
  isEnabled: true,
  pricingPayloadStr: "{}",
};

export function AIModelDrawer({ model, isOpen, onClose, isCreate }: AIModelDrawerProps) {
  const createModel = useCreateAIModel();
  const updateModel = useUpdateAIModel();

  const [form, setForm] = useState(() => model ? {
    id: model.id,
    vendor: model.vendor,
    modelName: model.modelName,
    category: model.category,
    description: model.description || "",
    isEnabled: model.isEnabled,
    pricingPayloadStr: model.pricingPayload ? JSON.stringify(model.pricingPayload, null, 2) : "{}",
  } : DEFAULT_FORM);
  const [jsonError, setJsonError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    let parsedPayload: unknown = {};
    try {
      parsedPayload = JSON.parse(form.pricingPayloadStr);
      setJsonError("");
    } catch {
      setJsonError("计费参数 JSON 格式错误，请检查");
      return;
    }

    try {
      if (isCreate) {
        await createModel.mutateAsync({
          id: form.id || undefined,
          vendor: form.vendor,
          modelName: form.modelName,
          category: form.category,
          description: form.description || undefined,
          pricingPayload: parsedPayload,
          isEnabled: form.isEnabled,
        });
      } else if (model) {
        await updateModel.mutateAsync({
          modelId: model.id,
          payload: {
            vendor: form.vendor,
            modelName: form.modelName,
            category: form.category,
            description: form.description || undefined,
            pricingPayload: parsedPayload,
            isEnabled: form.isEnabled,
          },
        });
      }
      onClose();
    } catch {
      alert("操作失败，请重试");
    }
  };

  if (!isOpen) return null;

  const isPending = createModel.isPending || updateModel.isPending;

  return (
    <>
      <div className="fixed inset-0 bg-black/50 z-40" onClick={onClose} />
      <div className="fixed inset-y-0 right-0 w-full max-w-lg bg-[var(--color-bg-primary)] border-l border-[var(--color-border)] shadow-2xl z-50 flex flex-col">
        <div className="flex items-center justify-between p-5 border-b border-[var(--color-border)] bg-[var(--color-bg-primary)]/90 backdrop-blur sticky top-0">
          <h2 className="text-lg font-semibold">{isCreate ? "新增 AI 模型配置" : `编辑模型: ${model?.modelName}`}</h2>
          <button onClick={onClose} className="p-2 hover:bg-[var(--color-bg-secondary)] rounded-lg transition-colors">
            <X className="h-5 w-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-5 space-y-5">
          {isCreate && (
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">模型 ID <span className="text-xs">(留空自动生成)</span></label>
              <input type="text" value={form.id} onChange={e => setForm(f => ({ ...f, id: e.target.value }))}
                placeholder="e.g. flux-pro-v1"
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] font-mono" />
            </div>
          )}

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">厂商 (Vendor) *</label>
              <input required type="text" value={form.vendor} onChange={e => setForm(f => ({ ...f, vendor: e.target.value }))}
                placeholder="e.g. openai / fal / runway"
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]" />
            </div>
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">模型名称 *</label>
              <input required type="text" value={form.modelName} onChange={e => setForm(f => ({ ...f, modelName: e.target.value }))}
                placeholder="e.g. gpt-4o"
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]" />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">输出类型分类</label>
              <select value={form.category} onChange={e => setForm(f => ({ ...f, category: e.target.value }))}
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]">
                <option value="image">图像 (image)</option>
                <option value="video">视频 (video)</option>
                <option value="text">文字 (text)</option>
                <option value="audio">音频 (audio)</option>
                <option value="other">其他 (other)</option>
              </select>
            </div>
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">启用状态</label>
              <div className="flex items-center h-10">
                <button type="button" onClick={() => setForm(f => ({ ...f, isEnabled: !f.isEnabled }))}
                  className={`relative inline-flex items-center h-6 rounded-full w-11 transition-colors ${form.isEnabled ? "bg-green-500" : "bg-[var(--color-border)]"}`}>
                  <span className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${form.isEnabled ? "translate-x-6" : "translate-x-1"}`} />
                </button>
                <span className="ml-2.5 text-sm">{form.isEnabled ? "启用 (enabled)" : "停用 (disabled)"}</span>
              </div>
            </div>
          </div>

          <div>
            <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">描述备注</label>
            <textarea value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
              placeholder="模型用途说明、限制事项等..."
              className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] resize-none h-20" />
          </div>

          <div>
            <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">计费参数 (pricingPayload) — JSON</label>
            <textarea
              value={form.pricingPayloadStr}
              onChange={e => { setForm(f => ({ ...f, pricingPayloadStr: e.target.value })); setJsonError(""); }}
              className={`w-full px-3 py-2 bg-[var(--color-bg-secondary)] border rounded-lg text-xs font-mono focus:outline-none resize-none h-36 ${jsonError ? "border-red-500 focus:border-red-500" : "border-[var(--color-border)] focus:border-[var(--color-primary)]"}`}
            />
            {jsonError && <p className="text-xs text-red-500 mt-1">{jsonError}</p>}
          </div>

          <div className="pt-4 border-t border-[var(--color-border)] flex gap-3">
            <button type="button" onClick={onClose} className="flex-1 px-4 py-2.5 border border-[var(--color-border)] rounded-lg text-sm font-medium hover:bg-[var(--color-bg-secondary)] transition-colors">
              取消
            </button>
            <button type="submit" disabled={isPending} className="flex-1 px-4 py-2.5 bg-[var(--color-primary)] text-white rounded-lg text-sm font-medium hover:bg-[var(--color-primary)]/90 transition-colors disabled:opacity-50">
              {isPending ? "保存中..." : isCreate ? "创建模型" : "保存更改"}
            </button>
          </div>
        </form>
      </div>
    </>
  );
}
