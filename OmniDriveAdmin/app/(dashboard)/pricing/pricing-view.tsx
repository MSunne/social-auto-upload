"use client";

import { useState } from "react";
import {
  useCreateWorkflowDurationRule,
  usePricingPackages,
  usePricingRules,
  useUpdatePricingPackage,
  useUpdateWorkflowDurationRule,
  useWorkflowDurationRules,
} from "@/lib/hooks/usePricing";
import { PageHeader } from "@/components/ui/common";
import { Plus, Loader2, CheckCircle, XCircle, Edit2 } from "lucide-react";
import { BillingPackage, WorkflowDurationRule } from "@/lib/types";
import { getModelDisplayName } from "@/lib/model-display";
import { PricingPackageDrawer } from "./pricing-package-drawer";

type WorkflowRuleFormState = {
  id?: string;
  workflowCode: string;
  outputType: string;
  durationSeconds: string;
  segmentSeconds: string;
  specialPriceCredits: string;
  sortOrder: string;
  description: string;
  isEnabled: boolean;
};

const EMPTY_WORKFLOW_RULE_FORM: WorkflowRuleFormState = {
  workflowCode: "video_text",
  outputType: "视文模式",
  durationSeconds: "8",
  segmentSeconds: "8",
  specialPriceCredits: "",
  sortOrder: "100",
  description: "",
  isEnabled: true,
};

function buildWorkflowRuleForm(rule?: WorkflowDurationRule | null): WorkflowRuleFormState {
  if (!rule) {
    return { ...EMPTY_WORKFLOW_RULE_FORM };
  }
  return {
    id: rule.id,
    workflowCode: rule.workflowCode,
    outputType: rule.outputType,
    durationSeconds: String(rule.durationSeconds),
    segmentSeconds: String(rule.segmentSeconds),
    specialPriceCredits:
      typeof rule.specialPriceCredits === "number" && rule.specialPriceCredits > 0
        ? String(rule.specialPriceCredits)
        : "",
    sortOrder: String(rule.sortOrder),
    description: rule.description || "",
    isEnabled: Boolean(rule.isEnabled),
  };
}

export function PricingView() {
  const [selectedPackage, setSelectedPackage] = useState<BillingPackage | null>(null);
  const [showCreateDrawer, setShowCreateDrawer] = useState(false);
  const [activeTab, setActiveTab] = useState<"packages" | "rules" | "workflow">("packages");
  const [workflowRuleForm, setWorkflowRuleForm] = useState<WorkflowRuleFormState>(EMPTY_WORKFLOW_RULE_FORM);

  const { data: packagesData, isLoading: packagesLoading } = usePricingPackages();
  const { data: rulesData, isLoading: rulesLoading } = usePricingRules();
  const { data: workflowRulesData, isLoading: workflowRulesLoading } = useWorkflowDurationRules();
  const updatePackage = useUpdatePricingPackage();
  const createWorkflowRule = useCreateWorkflowDurationRule();
  const updateWorkflowRule = useUpdateWorkflowDurationRule();

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

  const handleWorkflowRuleSubmit = async () => {
    const payload = {
      workflowCode: workflowRuleForm.workflowCode.trim(),
      outputType: workflowRuleForm.outputType.trim(),
      durationSeconds: Number(workflowRuleForm.durationSeconds),
      segmentSeconds: Number(workflowRuleForm.segmentSeconds),
      specialPriceCredits: workflowRuleForm.specialPriceCredits.trim()
        ? Number(workflowRuleForm.specialPriceCredits)
        : undefined,
      sortOrder: Number(workflowRuleForm.sortOrder || "100"),
      description: workflowRuleForm.description.trim() || undefined,
      isEnabled: workflowRuleForm.isEnabled,
    };
    if (!payload.workflowCode || !payload.outputType || payload.durationSeconds <= 0 || payload.segmentSeconds <= 0) {
      alert("请先填写完整的工作流编码、输出类型、固定时长和分段时长。");
      return;
    }
    try {
      if (workflowRuleForm.id) {
        await updateWorkflowRule.mutateAsync({ ruleId: workflowRuleForm.id, payload });
      } else {
        await createWorkflowRule.mutateAsync(payload);
      }
      setWorkflowRuleForm({ ...EMPTY_WORKFLOW_RULE_FORM });
    } catch (error) {
      alert(error instanceof Error ? error.message : "保存时长规则失败，请重试");
    }
  };

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
        {(["packages", "rules", "workflow"] as const).map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            className={`px-5 py-2.5 text-sm font-medium border-b-2 transition-colors ${activeTab === tab ? "border-[var(--color-primary)] text-[var(--color-primary)]" : "border-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"}`}>
            {tab === "packages" ? "充值套餐" : tab === "rules" ? "积分计费规则" : "视文模式时长策略"}
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
                      {rule.modelName && (
                        <div className="text-[var(--color-text-secondary)]">
                          {getModelDisplayName(rule, rule.modelName)}
                        </div>
                      )}
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

      {activeTab === "workflow" && (
        <div className="space-y-4">
          <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-5">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">视文模式固定时长策略</h3>
                <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
                  用于控制技能可选时长，以及是否命中全局任务特价。未配置特价时，回退为按实际模型步骤累计计费。
                </p>
              </div>
              {workflowRuleForm.id ? (
                <button
                  type="button"
                  onClick={() => setWorkflowRuleForm({ ...EMPTY_WORKFLOW_RULE_FORM })}
                  className="rounded-lg border border-[var(--color-border)] px-3 py-2 text-xs font-medium text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
                >
                  新建规则
                </button>
              ) : null}
            </div>
            <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              <label className="space-y-1 text-sm">
                <span className="text-[var(--color-text-secondary)]">工作流编码</span>
                <input value={workflowRuleForm.workflowCode} onChange={(event) => setWorkflowRuleForm((current) => ({ ...current, workflowCode: event.target.value }))} className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]" />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-[var(--color-text-secondary)]">输出类型</span>
                <input value={workflowRuleForm.outputType} onChange={(event) => setWorkflowRuleForm((current) => ({ ...current, outputType: event.target.value }))} className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]" />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-[var(--color-text-secondary)]">固定时长（秒）</span>
                <input value={workflowRuleForm.durationSeconds} onChange={(event) => setWorkflowRuleForm((current) => ({ ...current, durationSeconds: event.target.value }))} className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]" />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-[var(--color-text-secondary)]">分段时长（秒）</span>
                <input value={workflowRuleForm.segmentSeconds} onChange={(event) => setWorkflowRuleForm((current) => ({ ...current, segmentSeconds: event.target.value }))} className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]" />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-[var(--color-text-secondary)]">任务特价（积分）</span>
                <input value={workflowRuleForm.specialPriceCredits} onChange={(event) => setWorkflowRuleForm((current) => ({ ...current, specialPriceCredits: event.target.value }))} placeholder="留空表示按步骤计费" className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]" />
              </label>
              <label className="space-y-1 text-sm">
                <span className="text-[var(--color-text-secondary)]">排序</span>
                <input value={workflowRuleForm.sortOrder} onChange={(event) => setWorkflowRuleForm((current) => ({ ...current, sortOrder: event.target.value }))} className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]" />
              </label>
              <label className="space-y-1 text-sm md:col-span-2">
                <span className="text-[var(--color-text-secondary)]">说明</span>
                <input value={workflowRuleForm.description} onChange={(event) => setWorkflowRuleForm((current) => ({ ...current, description: event.target.value }))} className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]" />
              </label>
            </div>
            <div className="mt-4 flex flex-wrap items-center gap-3">
              <label className="inline-flex items-center gap-2 text-sm text-[var(--color-text-primary)]">
                <input type="checkbox" checked={workflowRuleForm.isEnabled} onChange={(event) => setWorkflowRuleForm((current) => ({ ...current, isEnabled: event.target.checked }))} />
                启用当前规则
              </label>
              <button
                type="button"
                onClick={() => void handleWorkflowRuleSubmit()}
                disabled={createWorkflowRule.isPending || updateWorkflowRule.isPending}
                className="rounded-lg bg-[var(--color-primary)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--color-primary)]/90 disabled:opacity-60"
              >
                {createWorkflowRule.isPending || updateWorkflowRule.isPending ? "保存中..." : workflowRuleForm.id ? "更新规则" : "创建规则"}
              </button>
            </div>
          </div>

          <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
            <div className="overflow-x-auto">
              <table className="w-full text-sm text-left">
                <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
                  <tr>
                    <th className="px-6 py-4 font-medium">工作流</th>
                    <th className="px-6 py-4 font-medium">时长 / 分段</th>
                    <th className="px-6 py-4 font-medium">任务特价</th>
                    <th className="px-6 py-4 font-medium">排序</th>
                    <th className="px-6 py-4 font-medium">状态</th>
                    <th className="px-6 py-4 font-medium text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--color-border)]">
                  {workflowRulesLoading && (
                    <tr>
                      <td colSpan={6} className="px-6 py-12 text-center">
                        <Loader2 className="h-5 w-5 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                      </td>
                    </tr>
                  )}
                  {workflowRulesData?.items.map((rule) => (
                    <tr key={rule.id} className={`hover:bg-[var(--color-bg-secondary)]/50 transition-colors ${!rule.isEnabled ? "opacity-50" : ""}`}>
                      <td className="px-6 py-4">
                        <div className="font-medium text-[var(--color-text-primary)]">{rule.workflowCode}</div>
                        <div className="mt-1 text-xs text-[var(--color-text-secondary)]">{rule.outputType}</div>
                        {rule.description ? <div className="mt-1 text-xs text-[var(--color-text-secondary)]">{rule.description}</div> : null}
                      </td>
                      <td className="px-6 py-4 font-mono text-xs">
                        {rule.durationSeconds}s / {rule.segmentSeconds}s
                      </td>
                      <td className="px-6 py-4 text-xs">
                        {typeof rule.specialPriceCredits === "number" && rule.specialPriceCredits > 0 ? (
                          <span className="text-amber-400 font-medium">{rule.specialPriceCredits} 积分</span>
                        ) : (
                          <span className="text-[var(--color-text-secondary)]">按步骤计费</span>
                        )}
                      </td>
                      <td className="px-6 py-4 text-xs">{rule.sortOrder}</td>
                      <td className="px-6 py-4">
                        {rule.isEnabled
                          ? <span className="text-xs text-green-500 font-medium">● 启用</span>
                          : <span className="text-xs text-[var(--color-text-secondary)]">● 停用</span>}
                      </td>
                      <td className="px-6 py-4 text-right">
                        <button
                          type="button"
                          onClick={() => setWorkflowRuleForm(buildWorkflowRuleForm(rule))}
                          className="inline-flex items-center gap-1 text-xs font-medium text-[var(--color-primary)] hover:underline"
                        >
                          <Edit2 className="h-3 w-3" /> 编辑
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      <PricingPackageDrawer
        key={selectedPackage?.id ?? (showCreateDrawer ? "__create__" : "__closed__")}
        pkg={selectedPackage}
        isOpen={!!selectedPackage || showCreateDrawer}
        onClose={() => { setSelectedPackage(null); setShowCreateDrawer(false); }}
        isCreate={showCreateDrawer && !selectedPackage}
      />
    </div>
  );
}
