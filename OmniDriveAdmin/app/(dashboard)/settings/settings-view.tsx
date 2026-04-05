"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { PageHeader } from "@/components/ui/common";
import { useAIModels } from "@/lib/hooks/useAIModels";
import { useSystemConfig, useUpdateSystemConfig } from "@/lib/hooks/useSettings";
import { getModelDisplayName, getModelDisplayNameByName } from "@/lib/model-display";
import { AIModel, AdminSystemConfig } from "@/lib/types";
import {
  ArrowUpRight,
  Cpu,
  CreditCard,
  Loader2,
  MessageSquare,
  Save,
  Server,
} from "lucide-react";

type SaveNotice = {
  tone: "success" | "error";
  text: string;
};

export function SettingsView() {
  const { data: config, isLoading, error } = useSystemConfig();
  const { data: aiModelsData } = useAIModels({ page: 1, pageSize: 500 });
  const updateM = useUpdateSystemConfig();
  const [formData, setFormData] = useState<Partial<AdminSystemConfig>>({});
  const [saveNotice, setSaveNotice] = useState<SaveNotice | null>(null);

  const aiModels = useMemo(() => aiModelsData?.items || [], [aiModelsData?.items]);
  const chatModelOptions = useMemo(
    () => buildModelOptions(aiModels, "chat", formData.defaultChatModel),
    [aiModels, formData.defaultChatModel],
  );
  const imageModelOptions = useMemo(
    () => buildModelOptions(aiModels, "image", formData.defaultImageModel),
    [aiModels, formData.defaultImageModel],
  );
  const videoModelOptions = useMemo(
    () => buildModelOptions(aiModels, "video", formData.defaultVideoModel),
    [aiModels, formData.defaultVideoModel],
  );

  useEffect(() => {
    if (!config) {
      return;
    }
    const nextValue = JSON.parse(JSON.stringify(config));
    const timer = window.setTimeout(() => setFormData(nextValue), 0);
    return () => window.clearTimeout(timer);
  }, [config]);

  const handleSave = async () => {
    setSaveNotice(null);
    try {
      const savedConfig = await updateM.mutateAsync(buildSystemConfigUpdatePayload(formData));
      setFormData(JSON.parse(JSON.stringify(savedConfig)));
      setSaveNotice({ tone: "success", text: "系统配置已保存。" });
    } catch (saveError) {
      setSaveNotice({
        tone: "error",
        text: saveError instanceof Error ? saveError.message : "配置保存失败",
      });
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

  const handleSMSRegistrationChange = (
    field: keyof NonNullable<AdminSystemConfig["smsRegistration"]>,
    value: string | boolean | number
  ) => {
    setFormData((current) => ({
      ...current,
      smsRegistration: {
        ...(current.smsRegistration || {
          enabled: false,
          provider: "aliyun_dypnsapi",
          endpoint: "dypnsapi.aliyuncs.com",
          accessKeyId: "",
          accessKeySecret: "",
          signName: "",
          templateCode: "",
          templateParam: '{"code":"##code##"}',
          schemeName: "",
          defaultCountryCode: "86",
          validMinutes: 10,
          cooldownSeconds: 60,
          dailyLimit: 10,
          codeLength: 6,
        }),
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
        <div className="flex flex-col items-end gap-2">
          <button
            onClick={handleSave}
            disabled={updateM.isPending}
            className="flex items-center gap-2 rounded-lg bg-[var(--color-primary)] px-4 py-2 font-medium text-white transition-colors hover:brightness-110 disabled:opacity-50"
          >
            {updateM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            保存全站配置
          </button>
          {saveNotice ? (
            <p
              className={`text-sm ${
                saveNotice.tone === "success"
                  ? "text-emerald-600"
                  : "text-red-500"
              }`}
            >
              {saveNotice.text}
            </p>
          ) : null}
        </div>
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
                  readOnly
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
                />
                <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                  当前值来自服务启动环境变量，暂不支持在此页持久化修改。
                </p>
              </div>
            </div>
          </div>

          <div className="space-y-5 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-6">
            <h3 className="flex items-center gap-2 border-b border-[var(--color-border)] pb-2 text-base font-medium">
              <Cpu className="h-4 w-4 text-[var(--color-primary)]" />
              默认 AI 模型
            </h3>

            <div className="space-y-4">
              <ModelSelectField
                label="默认语言大模型 (Chat)"
                value={formData.defaultChatModel || ""}
                onChange={(value) =>
                  setFormData((current) => ({ ...current, defaultChatModel: value }))
                }
                options={chatModelOptions}
                displayValue={getModelDisplayNameByName(aiModels, formData.defaultChatModel)}
              />
              <ModelSelectField
                label="默认生图模型 (Image)"
                value={formData.defaultImageModel || ""}
                onChange={(value) =>
                  setFormData((current) => ({ ...current, defaultImageModel: value }))
                }
                options={imageModelOptions}
                displayValue={getModelDisplayNameByName(aiModels, formData.defaultImageModel)}
              />
              <ModelSelectField
                label="默认短视频模型 (Video)"
                value={formData.defaultVideoModel || ""}
                onChange={(value) =>
                  setFormData((current) => ({ ...current, defaultVideoModel: value }))
                }
                options={videoModelOptions}
                displayValue={getModelDisplayNameByName(aiModels, formData.defaultVideoModel)}
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

          <div className="space-y-5 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-6">
            <h3 className="flex items-center gap-2 border-b border-[var(--color-border)] pb-2 text-base font-medium">
              <MessageSquare className="h-4 w-4 text-[var(--color-primary)]" />
              短信注册 / 阿里云短信
            </h3>
            <p className="text-sm text-[var(--color-text-secondary)]">
              注册页发送验证码时会读取这里的配置。模板参数里请使用 <code>##code##</code> 作为验证码占位符。
            </p>

            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <label className="block text-sm font-medium">启用短信注册</label>
                  <p className="mt-0.5 text-xs text-[var(--color-text-secondary)]">
                    开启后，用户注册必须先获取并校验短信验证码。
                  </p>
                </div>
                <label className="relative inline-flex cursor-pointer items-center">
                  <input
                    type="checkbox"
                    checked={formData.smsRegistration?.enabled || false}
                    onChange={(event) => handleSMSRegistrationChange("enabled", event.target.checked)}
                    className="peer sr-only"
                  />
                  <div className="h-6 w-11 rounded-full bg-[var(--color-bg-secondary)] peer-checked:bg-[var(--color-primary)] peer-checked:after:translate-x-full after:absolute after:left-[2px] after:top-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-['']" />
                </label>
              </div>

              <InputField
                label="Provider"
                value={formData.smsRegistration?.provider || "aliyun_dypnsapi"}
                onChange={(value) => handleSMSRegistrationChange("provider", value)}
                placeholder="aliyun_dypnsapi 或 aliyun_dysmsapi"
              />
              <p className="-mt-2 text-xs text-[var(--color-text-secondary)]">
                `aliyun_dypnsapi` 适合号码认证赠送签名和赠送模板；`aliyun_dysmsapi` 适合自定义签名和 `SMS_` 模板。
              </p>
              <InputField
                label="Endpoint"
                value={formData.smsRegistration?.endpoint || ""}
                onChange={(value) => handleSMSRegistrationChange("endpoint", value)}
                placeholder="dypnsapi.aliyuncs.com / dysmsapi.aliyuncs.com"
              />
              <InputField
                label="AccessKey ID"
                value={formData.smsRegistration?.accessKeyId || ""}
                onChange={(value) => handleSMSRegistrationChange("accessKeyId", value)}
                placeholder="LTAI..."
              />
              <InputField
                label="AccessKey Secret"
                type="password"
                value={formData.smsRegistration?.accessKeySecret || ""}
                onChange={(value) => handleSMSRegistrationChange("accessKeySecret", value)}
                placeholder="阿里云短信 AccessKey Secret"
              />
              <InputField
                label="签名名称"
                value={formData.smsRegistration?.signName || ""}
                onChange={(value) => handleSMSRegistrationChange("signName", value)}
                placeholder="例如：速通互联验证码"
              />
              <InputField
                label="模板编码"
                value={formData.smsRegistration?.templateCode || ""}
                onChange={(value) => handleSMSRegistrationChange("templateCode", value)}
                placeholder="例如：SMS_100001"
              />
              <InputField
                label="Scheme Name（选填）"
                value={formData.smsRegistration?.schemeName || ""}
                onChange={(value) => handleSMSRegistrationChange("schemeName", value)}
                placeholder="阿里云短信服务名，可留空"
              />
              <InputField
                label="默认国家区号"
                value={formData.smsRegistration?.defaultCountryCode || "86"}
                onChange={(value) => handleSMSRegistrationChange("defaultCountryCode", value)}
                placeholder="86"
              />
              <div>
                <label className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]">
                  模板参数
                </label>
                <textarea
                  value={formData.smsRegistration?.templateParam || '{"code":"##code##"}'}
                  onChange={(event) => handleSMSRegistrationChange("templateParam", event.target.value)}
                  rows={3}
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
                  placeholder='{"code":"##code##"}'
                />
                <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                  统一使用 `##code##` 作为验证码占位符，系统会根据 Provider 自动改成云端校验或本地安全校验。
                </p>
              </div>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <InputField
                  label="验证码有效期（分钟）"
                  type="number"
                  value={String(formData.smsRegistration?.validMinutes ?? 10)}
                  onChange={(value) => handleSMSRegistrationChange("validMinutes", Number(value) || 0)}
                  placeholder="10"
                />
                <InputField
                  label="发送冷却（秒）"
                  type="number"
                  value={String(formData.smsRegistration?.cooldownSeconds ?? 60)}
                  onChange={(value) => handleSMSRegistrationChange("cooldownSeconds", Number(value) || 0)}
                  placeholder="60"
                />
                <InputField
                  label="单日上限"
                  type="number"
                  value={String(formData.smsRegistration?.dailyLimit ?? 10)}
                  onChange={(value) => handleSMSRegistrationChange("dailyLimit", Number(value) || 0)}
                  placeholder="10"
                />
                <InputField
                  label="验证码长度"
                  type="number"
                  value={String(formData.smsRegistration?.codeLength ?? 6)}
                  onChange={(value) => handleSMSRegistrationChange("codeLength", Number(value) || 0)}
                  placeholder="6"
                />
              </div>
              <div className="flex items-center justify-end border-t border-[var(--color-border)] pt-4">
                <button
                  type="button"
                  onClick={handleSave}
                  disabled={updateM.isPending}
                  className="inline-flex items-center gap-2 rounded-lg bg-[var(--color-primary)] px-4 py-2 text-sm font-medium text-white transition-colors hover:brightness-110 disabled:opacity-50"
                >
                  {updateM.isPending ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <Save className="h-4 w-4" />
                  )}
                  保存当前配置
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="flex justify-end border-t border-[var(--color-border)] pt-4">
        <button
          type="button"
          onClick={handleSave}
          disabled={updateM.isPending}
          className="inline-flex items-center gap-2 rounded-lg bg-[var(--color-primary)] px-4 py-2 font-medium text-white transition-colors hover:brightness-110 disabled:opacity-50"
        >
          {updateM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
          保存系统设置
        </button>
      </div>
    </div>
  );
}

function buildSystemConfigUpdatePayload(formData: Partial<AdminSystemConfig>) {
  return {
    aiWorkerEnabled: Boolean(formData.aiWorkerEnabled),
    paymentChannels: [...(formData.paymentChannels ?? [])],
    billingManualSupport: {
      name: formData.billingManualSupport?.name ?? "",
      contact: formData.billingManualSupport?.contact ?? "",
      qrCodeUrl: formData.billingManualSupport?.qrCodeUrl ?? "",
      note: formData.billingManualSupport?.note ?? "",
    },
    smsRegistration: {
      enabled: Boolean(formData.smsRegistration?.enabled),
      provider: formData.smsRegistration?.provider ?? "aliyun_dypnsapi",
      endpoint: formData.smsRegistration?.endpoint ?? "",
      accessKeyId: formData.smsRegistration?.accessKeyId ?? "",
      accessKeySecret: formData.smsRegistration?.accessKeySecret ?? "",
      signName: formData.smsRegistration?.signName ?? "",
      templateCode: formData.smsRegistration?.templateCode ?? "",
      templateParam: formData.smsRegistration?.templateParam ?? '{"code":"##code##"}',
      schemeName: formData.smsRegistration?.schemeName ?? "",
      defaultCountryCode: formData.smsRegistration?.defaultCountryCode ?? "86",
      validMinutes: formData.smsRegistration?.validMinutes ?? 10,
      cooldownSeconds: formData.smsRegistration?.cooldownSeconds ?? 60,
      dailyLimit: formData.smsRegistration?.dailyLimit ?? 10,
      codeLength: formData.smsRegistration?.codeLength ?? 6,
    },
    defaultChatModel: formData.defaultChatModel ?? "",
    defaultImageModel: formData.defaultImageModel ?? "",
    defaultVideoModel: formData.defaultVideoModel ?? "",
  };
}

function InputField({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: string;
}) {
  return (
    <div>
      <label className="mb-1 block text-sm font-medium">{label}</label>
      <input
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
      />
    </div>
  );
}

function buildModelOptions(models: AIModel[], category: string, currentValue?: string) {
  const matched = models
    .filter((item) => item.category === category)
    .sort((left, right) => getModelDisplayName(left).localeCompare(getModelDisplayName(right)))
    .map((item) => ({
      value: item.modelName,
      label: getModelDisplayName(item),
    }));

  const normalized = currentValue?.trim();
  if (normalized && !matched.some((item) => item.value === normalized)) {
    return [{ value: normalized, label: `${normalized}（未注册模型）` }, ...matched];
  }
  return matched;
}

function ModelSelectField({
  label,
  value,
  onChange,
  options,
  displayValue,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Array<{ value: string; label: string }>;
  displayValue: string;
}) {
  return (
    <div>
      <label className="mb-1 block text-sm font-medium">{label}</label>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
      >
        <option value="">请选择模型</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      {value ? (
        <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
          当前显示别名：{displayValue}，实际保存值：{value}
        </p>
      ) : null}
    </div>
  );
}
