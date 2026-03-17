"use client";

import { useState, useEffect } from "react";
import { useSystemConfig, useUpdateSystemConfig } from "@/lib/hooks/useSettings";
import { PageHeader } from "@/components/ui/common";
import { Loader2, Save, Server, CreditCard, Cpu, MessageSquare } from "lucide-react";
import { AdminSystemConfig } from "@/lib/types";

export function SettingsView() {
  const { data: config, isLoading, error } = useSystemConfig();
  const updateM = useUpdateSystemConfig();
  
  const [formData, setFormData] = useState<Partial<AdminSystemConfig>>({});
  
  // Sync when data loads
  useEffect(() => {
    if (config) {
      // eslint-disable-next-line react-hooks/exhaustive-deps, react-hooks/set-state-in-effect
      setFormData(JSON.parse(JSON.stringify(config))); // Deep copy for complex objects
    }
  }, [config]);

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center p-20 text-[var(--color-text-secondary)]">
        <Loader2 className="h-8 w-8 animate-spin mb-4" />
        <p>正在读取系统配置...</p>
      </div>
    );
  }

  if (error || !config) {
    return <div className="text-red-500 p-10">读取配置失败，请确保您有足够权限。</div>;
  }

  const handleSave = async () => {
    try {
      await updateM.mutateAsync(formData);
      alert("配置保存成功");
    } catch {
      alert("配置保存失败，请检查填写内容或系统日志");
    }
  };

  const handleManualSupportChange = (field: string, value: string) => {
    setFormData(prev => ({
      ...prev,
      billingManualSupport: {
        ...(prev.billingManualSupport || { name: "", contact: "", qrCodeUrl: "", note: "" }),
        [field]: value
      }
    }));
  };

  const togglePaymentChannel = (channel: string) => {
    setFormData(prev => {
      const channels = prev.paymentChannels || [];
      if (channels.includes(channel)) {
        return { ...prev, paymentChannels: channels.filter(c => c !== channel) };
      }
      return { ...prev, paymentChannels: [...channels, channel] };
    });
  };

  return (
    <div className="space-y-6 max-w-5xl">
      <div className="flex items-start justify-between">
        <PageHeader title="系统全局配置" subtitle="控制全平台的计费通道、打款方式表单和 AI Agent 开关。" />
        <button 
          onClick={handleSave} 
          disabled={updateM.isPending}
          className="flex items-center gap-2 px-4 py-2 bg-[var(--color-primary)] text-white rounded-lg font-medium hover:brightness-110 transition-colors disabled:opacity-50">
          {updateM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
          保存全站配置
        </button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        
        {/* Core Settings */}
        <div className="space-y-6">
          <div className="p-6 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] space-y-5">
            <h3 className="text-base font-medium flex items-center gap-2 pb-2 border-b border-[var(--color-border)]">
              <Server className="h-4 w-4 text-[var(--color-primary)]" /> 基础运行配置
            </h3>
            
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <label className="block text-sm font-medium">全局 AI Agent 调度</label>
                  <p className="text-xs text-[var(--color-text-secondary)] mt-0.5">关闭后，云端将不再派发新的 AI 任务给所有节点。</p>
                </div>
                <label className="relative inline-flex items-center cursor-pointer">
                  <input type="checkbox" checked={formData.aiWorkerEnabled || false} onChange={e => setFormData({ ...formData, aiWorkerEnabled: e.target.checked })} className="sr-only peer" />
                  <div className="w-11 h-6 bg-[var(--color-bg-secondary)] peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[var(--color-primary)]"></div>
                </label>
              </div>

              <div>
                <label className="block text-sm font-medium mb-1">系统管理员联系邮箱</label>
                <input type="email" value={formData.adminEmail || ""} onChange={e => setFormData({ ...formData, adminEmail: e.target.value })}
                  className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors" />
                <p className="text-xs text-[var(--color-text-secondary)] mt-1.5">向用户展示的官方客服邮箱或发生严重错误时的报警接收邮箱。</p>
              </div>
            </div>
          </div>

          {/* AI Default Models */}
          <div className="p-6 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] space-y-5">
            <h3 className="text-base font-medium flex items-center gap-2 pb-2 border-b border-[var(--color-border)]">
              <Cpu className="h-4 w-4 text-[var(--color-primary)]" /> 默认 AI 模型选择
            </h3>
            
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-1">默认语言大模型 (Chat)</label>
                <input type="text" value={formData.defaultChatModel || ""} onChange={e => setFormData({ ...formData, defaultChatModel: e.target.value })}
                  className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none font-mono focus:border-[var(--color-primary)] transition-colors" placeholder="e.g. gpt-4o" />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1">默认生图模型 (Image)</label>
                <input type="text" value={formData.defaultImageModel || ""} onChange={e => setFormData({ ...formData, defaultImageModel: e.target.value })}
                  className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none font-mono focus:border-[var(--color-primary)] transition-colors" placeholder="e.g. dall-e-3" />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1">默认短视频模型 (Video)</label>
                <input type="text" value={formData.defaultVideoModel || ""} onChange={e => setFormData({ ...formData, defaultVideoModel: e.target.value })}
                  className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none font-mono focus:border-[var(--color-primary)] transition-colors" placeholder="e.g. veo2" />
              </div>
            </div>
          </div>
        </div>

        {/* Finance and Billing Settings */}
        <div className="space-y-6">
          <div className="p-6 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] space-y-5">
            <h3 className="text-base font-medium flex items-center gap-2 pb-2 border-b border-[var(--color-border)]">
              <CreditCard className="h-4 w-4 text-[var(--color-primary)]" /> 结算与充值通道
            </h3>
            
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-2.5">开放的线上支付渠道</label>
                <div className="flex gap-4">
                  {[
                    { id: "wechat", label: "微信支付" },
                    { id: "alipay", label: "支付宝" },
                    { id: "stripe", label: "Stripe 外卡" },
                  ].map(channel => {
                    const isActive = (formData.paymentChannels || []).includes(channel.id);
                    return (
                      <button 
                        key={channel.id}
                        onClick={() => togglePaymentChannel(channel.id)}
                        className={`px-4 py-2 border rounded-lg text-sm font-medium transition-colors ${isActive ? "border-[var(--color-primary)] bg-[var(--color-primary)]/10 text-[var(--color-primary)]" : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-secondary)]"}`}>
                        {channel.label}
                      </button>
                    );
                  })}
                </div>
              </div>
            </div>
          </div>

          <div className="p-6 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] space-y-5">
            <h3 className="text-base font-medium flex items-center gap-2 pb-2 border-b border-[var(--color-border)]">
              <MessageSquare className="h-4 w-4 text-[var(--color-primary)]" /> 线下人工打款/客服配置
            </h3>
            <p className="text-sm text-[var(--color-text-secondary)]">当用户选择大额银行卡对公转账或人工对接充值时，客户端展示的收款信息。</p>
            
            <div className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">收款方名称 / 户名</label>
                <input type="text" value={formData.billingManualSupport?.name || ""} onChange={e => handleManualSupportChange("name", e.target.value)}
                  className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors" placeholder="例如: 某某科技有限公司" />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">收款账号 / 联系方式</label>
                <input type="text" value={formData.billingManualSupport?.contact || ""} onChange={e => handleManualSupportChange("contact", e.target.value)}
                  className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors" placeholder="银行账号或微信号" />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">收款二维码链接 (可选)</label>
                <input type="text" value={formData.billingManualSupport?.qrCodeUrl || ""} onChange={e => handleManualSupportChange("qrCodeUrl", e.target.value)}
                  className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors" placeholder="https://..." />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">转账备注意项说明</label>
                <textarea value={formData.billingManualSupport?.note || ""} onChange={e => handleManualSupportChange("note", e.target.value)} rows={3}
                  className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors" placeholder="例如: 转账时请务必备注您的用户ID或邮箱，处理时效为 1-2 工作日..." />
              </div>
            </div>
          </div>
        </div>

      </div>
    </div>
  );
}
