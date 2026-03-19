"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { PageHeader } from "@/components/ui/common";
import { useSystemConfig, useUpdateSystemConfig } from "@/lib/hooks/useSettings";
import { AdminSystemConfig } from "@/lib/types";
import {
  ArrowUpRight,
  Cpu,
  CreditCard,
  Loader2,
  MessageSquare,
  Save,
  Server,
} from "lucide-react";

export function SettingsView() {
  const { data: config, isLoading, error } = useSystemConfig();
  const updateM = useUpdateSystemConfig();
  const [formData, setFormData] = useState<Partial<AdminSystemConfig>>({});

  useEffect(() => {
    if (!config) {
      return;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps, react-hooks/set-state-in-effect
    setFormData(JSON.parse(JSON.stringify(config)));
  }, [config]);

  const handleSave = async () => {
    try {
      await updateM.mutateAsync(formData);
      window.alert("配置保存成功");
    } catch (saveError) {
      window.alert(saveError instanceof Error ? saveError.message : "配置保存失败");
    }
  };

  const handleManualSupportChange = (
    field: keyof NonNullable<AdminSystemConfig["billingManualSupport"]>,
    value: string
  ) => {
    setFormData((current) => ({
      ...current,
      billingManualSupport: {
        ...(current.billingManualSupport || { name: "", contact: "", qrCodeUrl: "", note: "" }),
        [field]: value,
      },
    }));
  };

  const togglePaymentChannel = (channel: string) => {
    setFormData((current) => {
      const channels = current.paymentChannels || [];
      if (channels.includes(channel)) {
        return { ...current, paymentChannels: channels.filter((item) => item !== channel) };
      }
      return { ...current, paymentChannels: [...channels, channel] };
    });
  };

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center p-20 text-[var(--color-text-secondary)]">
        <Loader2 className="mb-4 h-8 w-8 animate-spin" />
        <p>正在读取系统配置...</p>
      </div>
    );
  }

  if (error || !config) {
    return <div className="p-10 text-red-500">读取配置失败，请确保您有足够权限。</div>;
  }

  return (
    <div className="max-w-6xl space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader
          title="系统全局配置"
          subtitle="控制全平台的计费通道、AI 默认模型，以及分镜优化链路的管理员策略。"
        />
        <button
          onClick={handleSave}
          disabled={updateM.isPending}
          className="flex items-center gap-2 rounded-lg bg-[var(--color-primary)] px-4 py-2 font-medium text-white transition-colors hover:brightness-110 disabled:opacity-50"
        >
          {updateM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
          保存全站配置
        </button>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <div className="space-y-6">
          <div className="space-y-5 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-6">
            <h3 className="flex items-center gap-2 border-b border-[var(--color-border)] pb-2 text-base font-medium">
              <Server className="h-4 w-4 text-[var(--color-primary)]" />
              基础运行配置
            </h3>

            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <label className="block text-sm font-medium">全局 AI Agent 调度</label>
                  <p className="mt-0.5 text-xs text-[var(--color-text-secondary)]">
                    关闭后，云端将不再派发新的 AI 任务给所有节点。
                  </p>
                </div>
                <label className="relative inline-flex cursor-pointer items-center">
                  <input
                    type="checkbox"
                    checked={formData.aiWorkerEnabled || false}
                    onChange={(event) =>
                      setFormData((current) => ({ ...current, aiWorkerEnabled: event.target.checked }))
                    }
                    className="peer sr-only"
                  />
                  <div className="h-6 w-11 rounded-full bg-[var(--color-bg-secondary)] peer-checked:bg-[var(--color-primary)] peer-checked:after:translate-x-full after:absolute after:left-[2px] after:top-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-['']" />
                </label>
              </div>

              <div>
                <label className="mb-1 block text-sm font-medium">系统管理员联系邮箱</label>
                <input
                  type="email"
                  value={formData.adminEmail || ""}
                  onChange={(event) =>
                    setFormData((current) => ({ ...current, adminEmail: event.target.value }))
                  }
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
                />
              </div>
            </div>
          </div>

          <div className="space-y-5 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-6">
            <h3 className="flex items-center gap-2 border-b border-[var(--color-border)] pb-2 text-base font-medium">
              <Cpu className="h-4 w-4 text-[var(--color-primary)]" />
              默认 AI 模型
            </h3>

            <div className="space-y-4">
              <InputField
                label="默认语言大模型 (Chat)"
                value={formData.defaultChatModel || ""}
                onChange={(value) =>
                  setFormData((current) => ({ ...current, defaultChatModel: value }))
                }
                placeholder="e.g. gpt-4.1-mini"
              />
              <InputField
                label="默认生图模型 (Image)"
                value={formData.defaultImageModel || ""}
                onChange={(value) =>
                  setFormData((current) => ({ ...current, defaultImageModel: value }))
                }
                placeholder="e.g. imagen-4"
              />
              <InputField
                label="默认短视频模型 (Video)"
                value={formData.defaultVideoModel || ""}
                onChange={(value) =>
                  setFormData((current) => ({ ...current, defaultVideoModel: value }))
                }
                placeholder="e.g. veo-3-fast"
              />
            </div>
          </div>

          <div className="space-y-5 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-6">
            <h3 className="flex items-center gap-2 border-b border-[var(--color-border)] pb-2 text-base font-medium">
              <MessageSquare className="h-4 w-4 text-[var(--color-primary)]" />
              分镜优化治理
            </h3>
            <p className="text-sm text-[var(--color-text-secondary)]">
              分镜优化已经提升为独立管理入口，视频/通用分镜和图片分镜各自拥有独立的系统提示词、模型与参考文件。
            </p>
            <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p className="text-sm font-medium">独立入口</p>
                  <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                    在专门页面里统一维护视频/通用分镜与图片分镜的系统级策略，避免和基础系统配置混在一起。
                  </p>
                </div>
                <Link
                  href="/storyboards"
                  className="inline-flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2 text-sm transition-colors hover:border-[var(--color-primary)] hover:text-[var(--color-primary)]"
                >
                  打开分镜管理
                  <ArrowUpRight className="h-4 w-4" />
                </Link>
              </div>
            </div>
          </div>
        </div>

        <div className="space-y-6">
          <div className="space-y-5 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-6">
            <h3 className="flex items-center gap-2 border-b border-[var(--color-border)] pb-2 text-base font-medium">
              <CreditCard className="h-4 w-4 text-[var(--color-primary)]" />
              结算与充值通道
            </h3>

            <div>
              <label className="mb-2.5 block text-sm font-medium">开放的线上支付渠道</label>
              <div className="flex flex-wrap gap-4">
                {[
                  { id: "wechat", label: "微信支付" },
                  { id: "alipay", label: "支付宝" },
                  { id: "stripe", label: "Stripe 外卡" },
                ].map((channel) => {
                  const isActive = (formData.paymentChannels || []).includes(channel.id);
                  return (
                    <button
                      key={channel.id}
                      type="button"
                      onClick={() => togglePaymentChannel(channel.id)}
                      className={`rounded-lg border px-4 py-2 text-sm font-medium transition-colors ${
                        isActive
                          ? "border-[var(--color-primary)] bg-[var(--color-primary)]/10 text-[var(--color-primary)]"
                          : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-secondary)]"
                      }`}
                    >
                      {channel.label}
                    </button>
                  );
                })}
              </div>
            </div>
          </div>

          <div className="space-y-5 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-6">
            <h3 className="flex items-center gap-2 border-b border-[var(--color-border)] pb-2 text-base font-medium">
              <MessageSquare className="h-4 w-4 text-[var(--color-primary)]" />
              线下人工打款 / 客服配置
            </h3>
            <p className="text-sm text-[var(--color-text-secondary)]">
              当用户选择大额银行卡对公转账或人工充值时，客户端展示的收款信息。
            </p>

            <div className="space-y-4">
              <InputField
                label="收款方名称 / 户名"
                value={formData.billingManualSupport?.name || ""}
                onChange={(value) => handleManualSupportChange("name", value)}
                placeholder="例如：某某科技有限公司"
              />
              <InputField
                label="收款账号 / 联系方式"
                value={formData.billingManualSupport?.contact || ""}
                onChange={(value) => handleManualSupportChange("contact", value)}
                placeholder="银行账号或微信号"
              />
              <InputField
                label="收款二维码链接"
                value={formData.billingManualSupport?.qrCodeUrl || ""}
                onChange={(value) => handleManualSupportChange("qrCodeUrl", value)}
                placeholder="https://..."
              />
              <div>
                <label className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]">
                  转账备注意项说明
                </label>
                <textarea
                  value={formData.billingManualSupport?.note || ""}
                  onChange={(event) => handleManualSupportChange("note", event.target.value)}
                  rows={4}
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
                  placeholder="例如：转账时请备注用户 ID，处理时效为 1-2 个工作日。"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function InputField({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}) {
  return (
    <div>
      <label className="mb-1 block text-sm font-medium">{label}</label>
      <input
        type="text"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
      />
    </div>
  );
}
