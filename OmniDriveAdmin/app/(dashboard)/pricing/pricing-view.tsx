"use client";

import { useState } from "react";
import { usePricingPackages, usePricingRules, useUpdatePricingPackage } from "@/lib/hooks/usePricing";
import { PageHeader } from "@/components/ui/common";
import { Plus, Loader2, CheckCircle, XCircle, Edit2 } from "lucide-react";
import { BillingPackage } from "@/lib/types";
import { PricingPackageDrawer } from "./pricing-package-drawer";

export function PricingView() {
  const [selectedPackage, setSelectedPackage] = useState<BillingPackage | null>(null);
  const [showCreateDrawer, setShowCreateDrawer] = useState(false);
  const [activeTab, setActiveTab] = useState<"packages" | "rules">("packages");

  const { data: packagesData, isLoading: packagesLoading } = usePricingPackages();
  const { data: rulesData, isLoading: rulesLoading } = usePricingRules();
  const updatePackage = useUpdatePricingPackage();

  const handleTogglePackage = async (pkg: BillingPackage) => {
    try {
      await updatePackage.mutateAsync({ packageId: pkg.id, payload: { isEnabled: !pkg.isEnabled } });
    } catch {
      alert("操作失败，请重试");
    }
  };

  const getChannelBadge = (channels: string[]) =>
    channels.map(c => {
      const label = c === "alipay" ? "支付宝" : c === "wechatpay" ? "微信" : c === "manual_cs" ? "客服" : c;
      const color = c === "alipay" ? "text-[#1677FF] bg-[#1677FF]/10 border-[#1677FF]/20"
        : c === "wechatpay" ? "text-[#09B83E] bg-[#09B83E]/10 border-[#09B83E]/20"
        : "text-purple-400 bg-purple-500/10 border-purple-500/20";
      return <span key={c} className={`px-1.5 py-0.5 text-[10px] rounded border font-medium ${color}`}>{label}</span>;
    });

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="充值套餐与计费规则" subtitle="全局管理平台的充值套餐配置与积分计费计量逻辑。" />
        {activeTab === "packages" && (
          <button
            onClick={() => setShowCreateDrawer(true)}
            className="flex items-center gap-2 px-4 py-2 bg-[var(--color-primary)] text-white text-sm font-medium rounded-lg hover:bg-[var(--color-primary)]/90 transition-colors"
          >
            <Plus className="h-4 w-4" /> 新增套餐
          </button>
        )}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-[var(--color-border)]">
        {(["packages", "rules"] as const).map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            className={`px-5 py-2.5 text-sm font-medium border-b-2 transition-colors ${activeTab === tab ? "border-[var(--color-primary)] text-[var(--color-primary)]" : "border-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"}`}>
            {tab === "packages" ? "充值套餐" : "积分计费规则"}
          </button>
        ))}
      </div>

      {/* Packages Tab */}
      {activeTab === "packages" && (
        <div className="space-y-4">
          {packagesLoading && (
            <div className="flex justify-center py-12">
              <Loader2 className="h-6 w-6 animate-spin text-[var(--color-text-secondary)]" />
            </div>
          )}
          {packagesData && packagesData.items.length === 0 && (
            <p className="text-center text-[var(--color-text-secondary)] py-12">暂无套餐配置</p>
          )}
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {packagesData && packagesData.items.map(pkg => (
              <div key={pkg.id} className={`p-5 rounded-xl border transition-colors ${pkg.isEnabled ? "border-[var(--color-border)] bg-[var(--color-bg-primary)]" : "border-[var(--color-border)] bg-[var(--color-bg-secondary)]/50 opacity-60"}`}>
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      {pkg.badge && <span className="px-2 py-0.5 text-xs rounded bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">{pkg.badge}</span>}
                      <h3 className="font-semibold text-[var(--color-text-primary)] truncate">{pkg.name}</h3>
                    </div>
                    <p className="text-xs font-mono text-[var(--color-text-secondary)]">{pkg.id}</p>
                  </div>
                  <button onClick={() => handleTogglePackage(pkg)} className="ml-2 flex-shrink-0">
                    {pkg.isEnabled
                      ? <CheckCircle className="h-5 w-5 text-green-500" />
                      : <XCircle className="h-5 w-5 text-[var(--color-text-secondary)]" />}
                  </button>
                </div>

                <div className="mb-4">
                  <div className="flex items-baseline gap-1">
                    <span className="text-2xl font-bold text-[var(--color-text-primary)]">¥{(pkg.priceCents / 100).toFixed(0)}</span>
                    <span className="text-xs text-[var(--color-text-secondary)]">{pkg.currency?.toUpperCase()}</span>
                  </div>
                  <div className="text-sm text-[var(--color-text-secondary)] mt-1">
                    基础积分 <span className="text-[var(--color-text-primary)] font-medium">{pkg.creditAmount.toLocaleString()}</span>
                    {pkg.manualBonusCreditAmount > 0 && <> + 赠 <span className="text-amber-400 font-medium">{pkg.manualBonusCreditAmount.toLocaleString()}</span></>}
                  </div>
                </div>

                <div className="flex items-center justify-between">
                  <div className="flex gap-1 flex-wrap">{getChannelBadge(pkg.paymentChannels || [])}</div>
                  <button onClick={() => setSelectedPackage(pkg)} className="flex items-center gap-1 text-xs text-[var(--color-primary)] hover:underline font-medium">
                    <Edit2 className="h-3 w-3" /> 编辑
                  </button>
                </div>

                {pkg.entitlements && pkg.entitlements.length > 0 && (
                  <div className="mt-3 pt-3 border-t border-[var(--color-border)] space-y-1">
                    {pkg.entitlements.map(e => (
                      <div key={e.id} className="flex items-center justify-between text-xs text-[var(--color-text-secondary)]">
                        <span>{e.meterName || e.meterCode}</span>
                        <span className="font-mono text-[var(--color-text-primary)]">+{e.grantAmount} {e.unit || ""}</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Rules Tab */}
      {activeTab === "rules" && (
        <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
          <div className="overflow-x-auto">
            <table className="w-full text-sm text-left">
              <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
                <tr>
                  <th className="px-6 py-4 font-medium">规则名称</th>
                  <th className="px-6 py-4 font-medium">计量码 (MeterCode)</th>
                  <th className="px-6 py-4 font-medium">应用范围</th>
                  <th className="px-6 py-4 font-medium">扣费模式</th>
                  <th className="px-6 py-4 font-medium text-right">单位/扣额</th>
                  <th className="px-6 py-4 font-medium">状态</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-border)]">
                {rulesLoading && (
                  <tr>
                    <td colSpan={6} className="px-6 py-12 text-center">
                      <Loader2 className="h-5 w-5 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                    </td>
                  </tr>
                )}
                {rulesData && rulesData.items.map(rule => (
                  <tr key={rule.id} className={`hover:bg-[var(--color-bg-secondary)]/50 transition-colors ${!rule.isEnabled ? "opacity-50" : ""}`}>
                    <td className="px-6 py-4">
                      <div className="font-medium">{rule.name}</div>
                      {rule.description && <div className="text-xs text-[var(--color-text-secondary)] mt-0.5">{rule.description}</div>}
                    </td>
                    <td className="px-6 py-4 font-mono text-xs">{rule.meterCode}</td>
                    <td className="px-6 py-4 text-xs">
                      <div>{rule.appliesTo}</div>
                      {rule.modelName && <div className="text-[var(--color-text-secondary)]">{rule.modelName}</div>}
                    </td>
                    <td className="px-6 py-4">
                      <span className="px-2 py-0.5 text-xs rounded bg-[var(--color-bg-secondary)] border border-[var(--color-border)]">{rule.chargeMode}</span>
                    </td>
                    <td className="px-6 py-4 text-right font-mono text-xs">
                      <div>单位: {rule.unitSize}</div>
                      <div className="text-amber-400">扣: {rule.walletDebitAmount}</div>
                    </td>
                    <td className="px-6 py-4">
                      {rule.isEnabled
                        ? <span className="text-xs text-green-500 font-medium">● 启用</span>
                        : <span className="text-xs text-[var(--color-text-secondary)]">● 停用</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <PricingPackageDrawer
        pkg={selectedPackage}
        isOpen={!!selectedPackage || showCreateDrawer}
        onClose={() => { setSelectedPackage(null); setShowCreateDrawer(false); }}
        isCreate={showCreateDrawer && !selectedPackage}
      />
    </div>
  );
}
