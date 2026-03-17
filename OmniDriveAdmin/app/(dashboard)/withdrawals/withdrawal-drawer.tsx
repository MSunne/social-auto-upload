"use client";

import { useState } from "react";
import { useWithdrawalDetail, useApproveWithdrawal, useRejectWithdrawal, useMarkWithdrawalPaid } from "@/lib/hooks/useWithdrawals";
import { X, Loader2, CheckCircle2, XCircle, CreditCard, Banknote } from "lucide-react";

interface WithdrawalDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  withdrawalId: string | null;
  onSuccess: () => void;
}

export function WithdrawalDrawer({ isOpen, onClose, withdrawalId, onSuccess }: WithdrawalDrawerProps) {
  const [rejectReason, setRejectReason] = useState("");
  const [paymentRef, setPaymentRef] = useState("");
  const [proofUrl, setProofUrl] = useState(""); // Simplified for input, actual allows array

  const { data: detail, isLoading } = useWithdrawalDetail(isOpen ? withdrawalId : null);
  const approveM = useApproveWithdrawal();
  const rejectM = useRejectWithdrawal();
  const paidM = useMarkWithdrawalPaid();

  if (!isOpen) return null;

  const isPending = approveM.isPending || rejectM.isPending || paidM.isPending;

  const handleApprove = async () => {
    if (!withdrawalId) return;
    try {
      await approveM.mutateAsync({ withdrawalId });
      onSuccess();
      onClose();
    } catch {
      alert("审批通过失败");
    }
  };

  const handleReject = async () => {
    if (!withdrawalId || !rejectReason) return alert("请先填写驳回由于");
    try {
      await rejectM.mutateAsync({ withdrawalId, payload: { reason: rejectReason } });
      onSuccess();
      onClose();
    } catch {
      alert("驳回操作失败");
    }
  };

  const handleMarkPaid = async () => {
    if (!withdrawalId || !paymentRef) return alert("请填写支付流水号");
    try {
      await paidM.mutateAsync({ withdrawalId, payload: { paymentReference: paymentRef, proofUrls: proofUrl ? [proofUrl] : [] } });
      onSuccess();
      onClose();
    } catch {
      alert("标记打款失败");
    }
  };

  const renderStatus = (status: string) => {
    switch (status) {
      case "pending_review": return <span className="px-2.5 py-1 text-xs font-medium rounded-full bg-orange-500/10 text-orange-400 border border-orange-500/20">待审核</span>;
      case "approved": return <span className="px-2.5 py-1 text-xs font-medium rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20">待打款 (已批)</span>;
      case "rejected": return <span className="px-2.5 py-1 text-xs font-medium rounded-full bg-red-500/10 text-red-400 border border-red-500/20">已驳回</span>;
      case "paid": return <span className="px-2.5 py-1 text-xs font-medium rounded-full bg-green-500/10 text-green-400 border border-green-500/20">已打款</span>;
      default: return <span className="px-2.5 py-1 text-xs font-medium rounded-full bg-gray-500/10 text-gray-400 border border-gray-500/20">{status}</span>;
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/40 backdrop-blur-sm transition-opacity">
      <div className="w-full max-w-lg bg-[var(--color-bg-primary)] border-l border-[var(--color-border)] shadow-2xl flex flex-col h-full animate-in slide-in-from-right duration-300">
        <div className="flex items-center justify-between p-6 border-b border-[var(--color-border)]">
          <h2 className="text-lg font-medium flex items-center gap-2">提现审核详情</h2>
          <button onClick={onClose} className="p-2 text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors rounded-lg hover:bg-[var(--color-bg-secondary)]">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {isLoading ? (
            <div className="flex flex-col items-center justify-center py-20 text-[var(--color-text-secondary)]">
              <Loader2 className="h-8 w-8 animate-spin mb-4" />
              <p>正在读取提现申请...</p>
            </div>
          ) : detail ? (
            <>
              {/* Header Info */}
              <div className="p-5 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)]/50 space-y-4">
                <div className="flex items-center justify-between">
                  <div className="text-sm font-medium text-[var(--color-text-secondary)]">单号</div>
                  <div className="font-mono text-sm">{detail.record.id}</div>
                </div>
                <div className="flex items-center justify-between">
                  <div className="text-sm font-medium text-[var(--color-text-secondary)]">状态</div>
                  <div>{renderStatus(detail.record.status)}</div>
                </div>
                <div className="flex items-center justify-between">
                  <div className="text-sm font-medium text-[var(--color-text-secondary)]">提现金额</div>
                  <div className="text-2xl font-bold text-green-400">¥ {(detail.record.amountCents / 100).toFixed(2)}</div>
                </div>
                <div className="flex items-center justify-between">
                  <div className="text-sm font-medium text-[var(--color-text-secondary)]">申请时间</div>
                  <div className="text-sm">{new Date(detail.record.requestedAt).toLocaleString("zh-CN")}</div>
                </div>
              </div>

              {/* User Info */}
              <div className="space-y-3">
                <h3 className="text-sm font-medium flex items-center gap-2"><CreditCard className="h-4 w-4" /> 收款方信息</h3>
                <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] space-y-3 test-sm">
                  <div className="flex flex-col gap-1">
                    <span className="text-xs text-[var(--color-text-secondary)]">推广员</span>
                    <span>{detail.record.promoter.name} ({detail.record.promoter.email})</span>
                  </div>
                  <div className="flex flex-col gap-1">
                    <span className="text-xs text-[var(--color-text-secondary)]">出款渠道</span>
                    <span className="font-mono">{detail.record.payoutChannel || "线下转账"}</span>
                  </div>
                  <div className="flex flex-col gap-1">
                    <span className="text-xs text-[var(--color-text-secondary)]">打款账户</span>
                    <span className="font-mono text-[var(--color-primary)] font-medium p-2 bg-[var(--color-bg-secondary)] rounded-md border border-[var(--color-border)] mt-1">{detail.record.accountMasked || "未知账户"}</span>
                  </div>
                  {detail.note && (
                    <div className="flex flex-col gap-1">
                      <span className="text-xs text-[var(--color-text-secondary)]">用户备注</span>
                      <span className="p-2 bg-[var(--color-bg-secondary)] rounded-md text-sm">{detail.note}</span>
                    </div>
                  )}
                </div>
              </div>

              {/* Status Specific Actions/Info */}
              {detail.record.status === "pending_review" && (
                <div className="space-y-3 pb-6">
                  <h3 className="text-sm font-medium">财务审核</h3>
                  <div>
                    <label className="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">驳回原因 (仅驳回需填)</label>
                    <textarea value={rejectReason} onChange={e => setRejectReason(e.target.value)} rows={3} placeholder="如果选择驳回，请注明原因让用户知晓"
                      className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors" />
                  </div>
                  <div className="flex gap-3">
                    <button onClick={handleReject} disabled={isPending || !rejectReason} className="flex-1 py-2.5 bg-red-500/10 text-red-500 border border-red-500/20 rounded-lg text-sm font-medium hover:bg-red-500/20 transition-colors disabled:opacity-50 flex items-center justify-center gap-2">
                      {rejectM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <XCircle className="h-4 w-4" />}
                      驳回申请
                    </button>
                    <button onClick={handleApprove} disabled={isPending} className="flex-1 py-2.5 bg-blue-500 text-white rounded-lg text-sm font-medium hover:bg-blue-600 transition-colors disabled:opacity-50 flex items-center justify-center gap-2">
                      {approveM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
                      核批通过
                    </button>
                  </div>
                </div>
              )}

              {detail.record.status === "approved" && (
                <div className="space-y-4 pb-6">
                  <h3 className="text-sm font-medium flex items-center gap-2"><Banknote className="h-4 w-4 text-green-400" /> 财务打款确权</h3>
                  <div className="p-4 rounded-xl border border-green-500/30 bg-green-500/5 space-y-4">
                    <div>
                      <label className="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">支付流水号 / 交易单号</label>
                      <input type="text" value={paymentRef} onChange={e => setPaymentRef(e.target.value)} placeholder="如 微信转账单号、网银流水回单号"
                        className="w-full px-3 py-2 bg-[var(--color-bg-primary)] border border-green-500/20 rounded-lg text-sm focus:outline-none focus:border-green-500 transition-colors" />
                    </div>
                    <div>
                      <label className="block text-xs font-medium text-[var(--color-text-secondary)] mb-1">转账截图链接 (可选记录)</label>
                      <input type="text" value={proofUrl} onChange={e => setProofUrl(e.target.value)} placeholder="https://..."
                        className="w-full px-3 py-2 bg-[var(--color-bg-primary)] border border-green-500/20 rounded-lg text-sm focus:outline-none focus:border-green-500 transition-colors" />
                    </div>
                    <button onClick={handleMarkPaid} disabled={isPending || !paymentRef} className="w-full py-2.5 bg-green-500 text-white rounded-lg text-sm font-medium hover:bg-green-600 transition-colors disabled:opacity-50 flex items-center justify-center gap-2">
                      {paidM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
                      确认已打款 (下发给用户)
                    </button>
                  </div>
                </div>
              )}

              {detail.record.status === "paid" && (
                <div className="p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-[var(--color-text-secondary)]">平台支付单号</span>
                    <span className="font-mono text-sm">{detail.paymentReference || "无"}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-[var(--color-text-secondary)]">打款确认时间</span>
                    <span className="text-sm">{detail.record.paidAt ? new Date(detail.record.paidAt).toLocaleString("zh-CN") : "—"}</span>
                  </div>
                </div>
              )}
            </>
          ) : (
            <div className="text-center text-[var(--color-text-secondary)] py-10">获取数据失败。</div>
          )}
        </div>
      </div>
    </div>
  );
}
