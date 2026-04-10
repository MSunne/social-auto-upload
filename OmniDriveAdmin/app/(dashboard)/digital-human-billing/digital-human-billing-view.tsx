"use client";

import { useEffect, useMemo, useState } from "react";
import { Loader2, Save, Video } from "lucide-react";
import { PageHeader, SectionCard } from "@/components/ui/common";
import { useSystemConfig, useUpdateSystemConfig } from "@/lib/hooks/useSettings";

type Notice = {
  tone: "success" | "error";
  text: string;
};

export function DigitalHumanBillingView() {
  const { data: config, isLoading, error } = useSystemConfig();
  const updateM = useUpdateSystemConfig();
  const [creditsPerSecond, setCreditsPerSecond] = useState("0");
  const [notice, setNotice] = useState<Notice | null>(null);

  useEffect(() => {
    if (!config) {
      return;
    }
    setCreditsPerSecond(String(config.digitalHumanCreditsPerSecond ?? 0));
  }, [config]);

  const currentValue = useMemo(() => {
    const trimmed = creditsPerSecond.trim();
    if (!/^\d+(\.\d{1,3})?$/.test(trimmed)) {
      return null;
    }
    return Number(trimmed);
  }, [creditsPerSecond]);

  const validationMessage = useMemo(() => {
    const trimmed = creditsPerSecond.trim();
    if (trimmed === "") {
      return "请输入每秒消耗积分。";
    }
    if (!/^\d+(\.\d{1,3})?$/.test(trimmed)) {
      return "只能填写 0 或最多 3 位小数的正数积分。";
    }
    return "";
  }, [creditsPerSecond]);

  async function handleSave() {
    setNotice(null);
    if (validationMessage || currentValue === null) {
      setNotice({ tone: "error", text: validationMessage || "请输入有效的积分数值。" });
      return;
    }

    try {
      const saved = await updateM.mutateAsync({
        digitalHumanCreditsPerSecond: currentValue,
      });
      setCreditsPerSecond(String(saved.digitalHumanCreditsPerSecond ?? 0));
      setNotice({ tone: "success", text: "数字人计费配置已保存。" });
    } catch (saveError) {
      setNotice({
        tone: "error",
        text: saveError instanceof Error ? saveError.message : "保存数字人计费配置失败",
      });
    }
  }

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center p-20 text-[var(--color-text-secondary)]">
        <Loader2 className="mb-4 h-8 w-8 animate-spin" />
        <p>正在读取数字人计费配置...</p>
      </div>
    );
  }

  if (error || !config) {
    return <div className="p-10 text-red-500">读取数字人计费配置失败，请确保您有足够权限。</div>;
  }

  return (
    <div className="max-w-5xl space-y-6">
      <PageHeader
        title="数字人计费"
        subtitle="为数字人视频配置全局按秒积分费率。用户侧预估时长固定按 4 字/秒估算，提交时会预扣预计积分，完成后再按成品视频实际秒数补差或退差。"
        actions={(
          <button
            type="button"
            onClick={handleSave}
            disabled={updateM.isPending}
            className="flex items-center gap-2 rounded-lg bg-[var(--color-primary)] px-4 py-2 font-medium text-white transition-colors hover:brightness-110 disabled:opacity-50"
          >
            {updateM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            保存配置
          </button>
        )}
      />

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.2fr)_320px]">
        <SectionCard
          title="每秒消耗积分"
          subtitle="设置为 0 表示关闭数字人视频计费，用户前台会显示“暂未开放计费配置”并禁止提交。"
        >
          <div className="space-y-5">
            <div>
              <label className="mb-2 block text-sm font-medium">全局费率</label>
              <div className="flex items-center gap-3">
                <input
                  type="number"
                  min={0}
                  step={0.001}
                  inputMode="decimal"
                  value={creditsPerSecond}
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
              {validationMessage ? (
                <p className="mt-2 text-xs text-red-500">{validationMessage}</p>
              ) : null}
            </div>

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
          subtitle="这里显示现在生效的数字人计费开关与费率说明。"
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

            <ul className="space-y-2 text-sm text-[var(--color-text-secondary)]">
              <li>创建任务时会先根据文案长度估算秒数并预扣积分。</li>
              <li>任务成功后按成品视频实际时长补扣或退回差额。</li>
              <li>任务失败时会退回全部预扣积分。</li>
            </ul>
          </div>
        </SectionCard>
      </div>
    </div>
  );
}
