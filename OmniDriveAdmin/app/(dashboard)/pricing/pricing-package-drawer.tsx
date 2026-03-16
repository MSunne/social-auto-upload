"use client";

import { useState } from "react";
import { useCreatePricingPackage, useUpdatePricingPackage } from "@/lib/hooks/usePricing";
import { BillingPackage } from "@/lib/types";
import { X, Plus, Trash2 } from "lucide-react";

interface PricingPackageDrawerProps {
  pkg: BillingPackage | null;
  isOpen: boolean;
  onClose: () => void;
  isCreate: boolean;
}

interface EntitlementInput {
  meterCode: string;
  grantAmount: number;
  grantMode: string;
  description: string;
}

const PAYMENT_CHANNELS = [
  { value: "alipay", label: "支付宝" },
  { value: "wechatpay", label: "微信支付" },
  { value: "manual_cs", label: "客服充值" },
];

export function PricingPackageDrawer({ pkg, isOpen, onClose, isCreate }: PricingPackageDrawerProps) {
  const createPackage = useCreatePricingPackage();
  const updatePackage = useUpdatePricingPackage();

  // Derive initial state synchronously from props - avoids setState-in-effect
  const [form, setForm] = useState(() => pkg ? {
    id: pkg.id,
    name: pkg.name,
    packageType: pkg.packageType || "standard",
    currency: pkg.currency || "cny",
    priceCents: pkg.priceCents,
    creditAmount: pkg.creditAmount,
    manualBonusCreditAmount: pkg.manualBonusCreditAmount || 0,
    badge: pkg.badge || "",
    description: pkg.description || "",
    isEnabled: pkg.isEnabled,
    sortOrder: pkg.sortOrder || 0,
    paymentChannels: pkg.paymentChannels || [],
  } : {
    id: "", name: "", packageType: "standard", currency: "cny",
    priceCents: 0, creditAmount: 0, manualBonusCreditAmount: 0,
    badge: "", description: "", isEnabled: true, sortOrder: 0,
    paymentChannels: ["alipay", "wechatpay"],
  });

  const [entitlements, setEntitlements] = useState<EntitlementInput[]>(
    () => (pkg?.entitlements || []).map(e => ({
      meterCode: e.meterCode,
      grantAmount: e.grantAmount,
      grantMode: e.grantMode,
      description: e.description || "",
    }))
  );

  if (!isOpen) return null;

  const toggleChannel = (ch: string) => {
    setForm(f => ({
      ...f,
      paymentChannels: f.paymentChannels.includes(ch)
        ? f.paymentChannels.filter(c => c !== ch)
        : [...f.paymentChannels, ch],
    }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      // Build a plain object with entitlements in the shape the API needs
      const basePayload = { ...form };
      if (isCreate) {
        await createPackage.mutateAsync({ ...basePayload, entitlements } as Parameters<typeof createPackage.mutateAsync>[0]);
      } else if (pkg) {
        await updatePackage.mutateAsync({
          packageId: pkg.id,
          payload: { ...basePayload, entitlements } as Parameters<typeof updatePackage.mutateAsync>[0]["payload"],
        });
      }
      onClose();
    } catch {
      alert("操作失败，请重试");
    }
  };

  const isPending = createPackage.isPending || updatePackage.isPending;

  return (
    <>
      <div className="fixed inset-0 bg-black/50 z-40" onClick={onClose} />
      <div className="fixed inset-y-0 right-0 w-full max-w-xl bg-[var(--color-bg-primary)] border-l border-[var(--color-border)] shadow-2xl z-50 flex flex-col">
        <div className="flex items-center justify-between p-5 border-b border-[var(--color-border)] sticky top-0 bg-[var(--color-bg-primary)]/90 backdrop-blur">
          <h2 className="text-lg font-semibold">{isCreate ? "新增充值套餐" : `编辑: ${pkg?.name}`}</h2>
          <button onClick={onClose} className="p-2 hover:bg-[var(--color-bg-secondary)] rounded-lg transition-colors">
            <X className="h-5 w-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-5 space-y-5">
          {isCreate && (
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">
                套餐 ID <span className="text-xs">(留空自动生成)</span>
              </label>
              <input
                type="text"
                value={form.id}
                onChange={e => setForm(f => ({ ...f, id: e.target.value }))}
                placeholder="e.g. pkg-standard-99"
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm font-mono focus:outline-none focus:border-[var(--color-primary)]"
              />
            </div>
          )}

          <div>
            <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">套餐名称 *</label>
            <input
              required
              type="text"
              value={form.name}
              onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
              placeholder="e.g. 基础版 99 元套餐"
              className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">价格 (分)</label>
              <input
                type="number" min={0}
                value={form.priceCents}
                onChange={e => setForm(f => ({ ...f, priceCents: +e.target.value }))}
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]"
              />
              <p className="text-xs text-[var(--color-text-secondary)] mt-1">= ¥{(form.priceCents / 100).toFixed(2)}</p>
            </div>
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">基础积分</label>
              <input
                type="number" min={0}
                value={form.creditAmount}
                onChange={e => setForm(f => ({ ...f, creditAmount: +e.target.value }))}
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]"
              />
            </div>
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">赠送积分</label>
              <input
                type="number" min={0}
                value={form.manualBonusCreditAmount}
                onChange={e => setForm(f => ({ ...f, manualBonusCreditAmount: +e.target.value }))}
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]"
              />
            </div>
            <div>
              <label className="block text-sm text-[var(--color-text-secondary)] mb-1.5">徽章标签 (badge)</label>
              <input
                type="text"
                value={form.badge}
                onChange={e => setForm(f => ({ ...f, badge: e.target.value }))}
                placeholder="e.g. 热门 / 推荐"
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm text-[var(--color-text-secondary)] mb-2">支持的支付渠道</label>
            <div className="flex gap-2 flex-wrap">
              {PAYMENT_CHANNELS.map(ch => (
                <button
                  type="button"
                  key={ch.value}
                  onClick={() => toggleChannel(ch.value)}
                  className={`px-3 py-1.5 text-sm rounded-lg border transition-colors ${form.paymentChannels.includes(ch.value)
                    ? "bg-[var(--color-primary)]/10 border-[var(--color-primary)]/50 text-[var(--color-primary)]"
                    : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-secondary)]"}`}
                >
                  {ch.label}
                </button>
              ))}
            </div>
          </div>

          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={() => setForm(f => ({ ...f, isEnabled: !f.isEnabled }))}
              className={`relative inline-flex h-6 w-11 rounded-full transition-colors ${form.isEnabled ? "bg-green-500" : "bg-[var(--color-border)]"}`}
            >
              <span className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${form.isEnabled ? "translate-x-6" : "translate-x-1"}`} />
            </button>
            <span className="text-sm">{form.isEnabled ? "对外上架展示" : "暂时下架"}</span>
          </div>

          {/* Entitlements */}
          <div className="pt-4 border-t border-[var(--color-border)]">
            <div className="flex items-center justify-between mb-3">
              <label className="text-sm font-medium text-[var(--color-text-secondary)]">权益赠送 (Entitlements)</label>
              <button
                type="button"
                onClick={() => setEntitlements(es => [...es, { meterCode: "", grantAmount: 0, grantMode: "one_time", description: "" }])}
                className="flex items-center gap-1 text-xs text-[var(--color-primary)] hover:underline"
              >
                <Plus className="h-3 w-3" /> 添加权益项
              </button>
            </div>
            {entitlements.map((ent, idx) => (
              <div key={idx} className="grid grid-cols-[1fr_auto_auto_auto] gap-2 mb-2 items-center">
                <input
                  placeholder="meterCode (e.g. ai_credits)"
                  value={ent.meterCode}
                  onChange={e => setEntitlements(es => es.map((x, i) => i === idx ? { ...x, meterCode: e.target.value } : x))}
                  className="px-2 py-1.5 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded text-xs font-mono focus:outline-none focus:border-[var(--color-primary)]"
                />
                <input
                  type="number" placeholder="数量"
                  value={ent.grantAmount}
                  onChange={e => setEntitlements(es => es.map((x, i) => i === idx ? { ...x, grantAmount: +e.target.value } : x))}
                  className="w-20 px-2 py-1.5 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded text-xs focus:outline-none focus:border-[var(--color-primary)]"
                />
                <select
                  value={ent.grantMode}
                  onChange={e => setEntitlements(es => es.map((x, i) => i === idx ? { ...x, grantMode: e.target.value } : x))}
                  className="px-2 py-1.5 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded text-xs focus:outline-none"
                >
                  <option value="one_time">一次</option>
                  <option value="monthly">按月</option>
                </select>
                <button
                  type="button"
                  onClick={() => setEntitlements(es => es.filter((_, i) => i !== idx))}
                  className="p-1.5 text-red-500 hover:bg-red-500/10 rounded transition-colors"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
          </div>

          <div className="pt-4 border-t border-[var(--color-border)] flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2.5 border border-[var(--color-border)] rounded-lg text-sm font-medium hover:bg-[var(--color-bg-secondary)] transition-colors"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={isPending}
              className="flex-1 px-4 py-2.5 bg-[var(--color-primary)] text-white rounded-lg text-sm font-medium hover:bg-[var(--color-primary)]/90 transition-colors disabled:opacity-50"
            >
              {isPending ? "保存中..." : isCreate ? "创建套餐" : "保存更改"}
            </button>
          </div>
        </form>
      </div>
    </>
  );
}
