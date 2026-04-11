"use client";

import { useMemo, useState } from "react";
import { Loader2, Save, Video } from "lucide-react";
import { PageHeader, SectionCard } from "@/components/ui/common";
import { useDigitalHumanModels } from "@/lib/hooks/useDigitalHumanModels";
import { useSystemConfig, useUpdateSystemConfig } from "@/lib/hooks/useSettings";

type Notice = {
  tone: "success" | "error";
  text: string;
};

function formatModelLabel(modelId: string, options: Array<{ id: string; isRecommended: boolean; isCurrent: boolean }>) {
  const option = options.find((item) => item.id === modelId);
  if (!option) {
    return modelId || "未配置";
  }
  return `${option.id}${option.isRecommended ? "（推荐）" : ""}${option.isCurrent ? "（当前）" : ""}`;
}

export function DigitalHumanBillingView() {
  const { data: config, isLoading, error } = useSystemConfig();
  const { data: modelCatalog, isLoading: modelsLoading } = useDigitalHumanModels();
  const updateM = useUpdateSystemConfig();
  const [creditsPerSecond, setCreditsPerSecond] = useState<string | null>(null);
  const [shoppingDefaultModel, setShoppingDefaultModel] = useState<string | null>(null);
  const [speechDefaultModel, setSpeechDefaultModel] = useState<string | null>(null);
  const [notice, setNotice] = useState<Notice | null>(null);
  const creditsInputValue =
    creditsPerSecond ?? String(config?.digitalHumanCreditsPerSecond ?? 0);
  const shoppingModelValue =
    shoppingDefaultModel
    ?? config?.digitalHumanShoppingDefaultModel
    ?? modelCatalog?.defaultModelByMode.digital
    ?? "";
  const speechModelValue =
    speechDefaultModel
    ?? config?.digitalHumanSpeechDefaultModel
    ?? modelCatalog?.defaultModelByMode.customize
    ?? "";

  const currentValue = useMemo(() => {
    const trimmed = creditsInputValue.trim();
    if (!/^\d+(\.\d{1,3})?$/.test(trimmed)) {
      return null;
    }
    return Number(trimmed);
  }, [creditsInputValue]);

  const validationMessage = useMemo(() => {
    const trimmed = creditsInputValue.trim();
    if (trimmed === "") {
      return "请输入每秒消耗积分。";
    }
    if (!/^\d+(\.\d{1,3})?$/.test(trimmed)) {
      return "只能填写 0 或最多 3 位小数的正数积分。";
    }
    if (!shoppingModelValue.trim()) {
      return "请选择带货模式默认模型。";
    }
    if (!speechModelValue.trim()) {
      return "请选择口播模式默认模型。";
    }
    if (
      modelCatalog &&
      !modelCatalog.models.some((item) => item.id === shoppingModelValue.trim())
    ) {
      return "带货模式默认模型不在当前可用模型列表中。";
    }
    if (
      modelCatalog &&
      !modelCatalog.models.some((item) => item.id === speechModelValue.trim())
    ) {
      return "口播模式默认模型不在当前可用模型列表中。";
    }
    return "";
  }, [creditsInputValue, modelCatalog, shoppingModelValue, speechModelValue]);

  async function handleSave() {
    setNotice(null);
    if (validationMessage || currentValue === null) {
      setNotice({ tone: "error", text: validationMessage || "请输入有效的配置项。" });
      return;
    }

    try {
      const saved = await updateM.mutateAsync({
        digitalHumanCreditsPerSecond: currentValue,
        digitalHumanShoppingDefaultModel: shoppingModelValue.trim(),
        digitalHumanSpeechDefaultModel: speechModelValue.trim(),
      });
      setCreditsPerSecond(String(saved.digitalHumanCreditsPerSecond ?? 0));
      setShoppingDefaultModel(saved.digitalHumanShoppingDefaultModel || "");
      setSpeechDefaultModel(saved.digitalHumanSpeechDefaultModel || "");
      setNotice({ tone: "success", text: "数字人设置已保存。" });
    } catch (saveError) {
      setNotice({
        tone: "error",
        text: saveError instanceof Error ? saveError.message : "保存数字人设置失败",
      });
    }
  }

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center p-20 text-[var(--color-text-secondary)]">
        <Loader2 className="mb-4 h-8 w-8 animate-spin" />
        <p>正在读取数字人设置...</p>
      </div>
    );
  }

  if (error || !config) {
    return <div className="p-10 text-red-500">读取数字人设置失败，请确保您有足够权限。</div>;
  }

  return (
    <div className="max-w-5xl space-y-6">
      <PageHeader
        title="数字人设置"
        subtitle="统一配置数字人费率和两种模式的默认执行模型。前台与技能编辑器都可以覆盖默认模型，后台只负责提供默认值和校验可用性。"
        actions={(
          <button
            type="button"
            onClick={handleSave}
            disabled={updateM.isPending || modelsLoading}
            className="flex items-center gap-2 rounded-lg bg-[var(--color-primary)] px-4 py-2 font-medium text-white transition-colors hover:brightness-110 disabled:opacity-50"
          >
            {updateM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            保存配置
          </button>
        )}
      />

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.25fr)_320px]">
        <SectionCard
          title="全局配置"
          subtitle="这里定义数字人视频的按秒费率，以及带货模式和口播模式首次进入时各自预填的默认模型。"
        >
          <div className="space-y-5">
            <div>
              <label className="mb-2 block text-sm font-medium">每秒消耗积分</label>
              <div className="flex items-center gap-3">
                <input
                  type="number"
                  min={0}
                  step={0.001}
                  inputMode="decimal"
                  value={creditsInputValue}
                  onChange={(event) => {
                    setCreditsPerSecond(event.target.value);
                    setNotice(null);
                  }}
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
                  placeholder="例如 0.5"
                />
                <span className="shrink-0 text-sm text-[var(--color-text-secondary)]">积分 / 秒</span>
              </div>
              <p className="mt-2 text-xs text-[var(--color-text-secondary)]">
                预计时长按 4 字/秒估算，实际结算以成品视频时长向上取整到秒为准。
              </p>
            </div>

            <div className="grid gap-4 lg:grid-cols-2">
              <label className="space-y-2">
                <span className="block text-sm font-medium">带货模式默认模型</span>
                <select
                  value={shoppingModelValue}
                  onChange={(event) => {
                    setShoppingDefaultModel(event.target.value);
                    setNotice(null);
                  }}
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
                  disabled={modelsLoading}
                >
                  {!modelCatalog?.models.length ? <option value="">暂无可用模型</option> : null}
                  {modelCatalog?.models.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.id}
                      {item.isRecommended ? "（推荐）" : ""}
                      {item.isCurrent ? "（当前）" : ""}
                    </option>
                  ))}
                </select>
                <p className="text-xs text-[var(--color-text-secondary)]">
                  用户在带货模式初次进入时，会优先预填这个模型。
                </p>
              </label>

              <label className="space-y-2">
                <span className="block text-sm font-medium">口播模式默认模型</span>
                <select
                  value={speechModelValue}
                  onChange={(event) => {
                    setSpeechDefaultModel(event.target.value);
                    setNotice(null);
                  }}
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
                  disabled={modelsLoading}
                >
                  {!modelCatalog?.models.length ? <option value="">暂无可用模型</option> : null}
                  {modelCatalog?.models.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.id}
                      {item.isRecommended ? "（推荐）" : ""}
                      {item.isCurrent ? "（当前）" : ""}
                    </option>
                  ))}
                </select>
                <p className="text-xs text-[var(--color-text-secondary)]">
                  用户在口播模式初次进入时，会优先预填这个模型。
                </p>
              </label>
            </div>

            {validationMessage ? (
              <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-600">
                {validationMessage}
              </div>
            ) : null}

            {notice ? (
              <div
                className={`rounded-lg border px-4 py-3 text-sm ${
                  notice.tone === "success"
                    ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                    : "border-red-200 bg-red-50 text-red-600"
                }`}
              >
                {notice.text}
              </div>
            ) : null}
          </div>
        </SectionCard>

        <SectionCard
          title="当前状态"
          subtitle="这里展示当前生效的默认模型和计费开关。"
        >
          <div className="space-y-4">
            <div className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] p-4">
              <div className="flex items-center gap-3">
                <div className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-3 text-[var(--color-accent)]">
                  <Video className="h-5 w-5" />
                </div>
                <div>
                  <p className="text-sm font-medium">
                    {(currentValue ?? config.digitalHumanCreditsPerSecond) > 0 ? "已启用按秒计费" : "计费已关闭"}
                  </p>
                  <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                    当前配置 {(currentValue ?? config.digitalHumanCreditsPerSecond) || 0} 积分 / 秒
                  </p>
                </div>
              </div>
            </div>

            <div className="space-y-3 rounded-2xl border border-[var(--color-border)] bg-[var(--color-panel-muted)] p-4 text-sm">
              <div>
                <p className="text-xs text-[var(--color-text-secondary)]">带货模式默认模型</p>
                <p className="mt-1 font-medium">
                  {formatModelLabel(shoppingDefaultModel || config.digitalHumanShoppingDefaultModel, modelCatalog?.models || [])}
                </p>
              </div>
              <div>
                <p className="text-xs text-[var(--color-text-secondary)]">口播模式默认模型</p>
                <p className="mt-1 font-medium">
                  {formatModelLabel(speechDefaultModel || config.digitalHumanSpeechDefaultModel, modelCatalog?.models || [])}
                </p>
              </div>
            </div>

            <ul className="space-y-2 text-sm text-[var(--color-text-secondary)]">
              <li>`qvq-max` 始终会在前台标记为推荐模型。</li>
              <li>默认模型只负责预填，前台和技能编辑器都允许用户手动改模型。</li>
              <li>任务失败时会退回全部预扣积分，成功后按实际成片时长结算。</li>
            </ul>
          </div>
        </SectionCard>
      </div>
    </div>
  );
}
