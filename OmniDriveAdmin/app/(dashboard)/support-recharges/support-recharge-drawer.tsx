"use client";

import { useState } from "react";
import { 
  useSupportRechargeDetail, 
  useCreditSupportRecharge, 
  useRejectSupportRecharge 
} from "@/lib/hooks/useFinance";
import { X, Loader2, ExternalLink, CheckCircle, XCircle } from "lucide-react";

interface SupportRechargeDrawerProps {
  orderId: string | null;
  onClose: () => void;
}

export function SupportRechargeDrawer({ orderId, onClose }: SupportRechargeDrawerProps) {
  const { data, isLoading, error } = useSupportRechargeDetail(orderId);
  const creditMutation = useCreditSupportRecharge();
  const rejectMutation = useRejectSupportRecharge();

  const [rejectReason, setRejectReason] = useState("");
  const [creditNote, setCreditNote] = useState("");
  const [isRejecting, setIsRejecting] = useState(false);

  if (!orderId) return null;

  const handleCredit = async () => {
    try {
      await creditMutation.mutateAsync({ orderId, payload: { note: creditNote } });
      onClose();
    } catch (e: unknown) {
      alert("入账失败：" + ((e as { response?: { data?: { message?: string } } }).response?.data?.message || (e as Error).message));
    }
  };

  const handleReject = async () => {
    if (!rejectReason) {
      alert("驳回必须填写原因");
      return;
    }
    try {
      await rejectMutation.mutateAsync({ orderId, payload: { reason: rejectReason } });
      onClose();
    } catch (e: unknown) {
      alert("驳回失败：" + ((e as { response?: { data?: { message?: string } } }).response?.data?.message || (e as Error).message));
    }
  };

  return (
    <>
      <div 
        className="fixed inset-0 bg-black/50 z-40 transition-opacity" 
        onClick={onClose}
      />
      <div className="fixed inset-y-0 right-0 w-full max-w-xl bg-[var(--color-bg-primary)] border-l border-[var(--color-border)] shadow-2xl z-50 overflow-y-auto flex flex-col transform transition-transform">
        <div className="flex items-center justify-between p-4 border-b border-[var(--color-border)] sticky top-0 bg-[var(--color-bg-primary)]/90 backdrop-blur z-10">
          <h2 className="text-lg font-medium text-[var(--color-text-primary)]">工单详情</h2>
          <button onClick={onClose} className="p-2 hover:bg-[var(--color-bg-secondary)] rounded-lg transition-colors">
            <X className="h-5 w-5 text-[var(--color-text-secondary)]" />
          </button>
        </div>

        <div className="p-6 flex-1 space-y-8">
          {isLoading && (
            <div className="flex flex-col items-center justify-center py-20">
              <Loader2 className="h-8 w-8 animate-spin text-[var(--color-primary)]" />
              <p className="mt-4 text-sm text-[var(--color-text-secondary)]">正在加载详情...</p>
            </div>
          )}

          {error && (
            <div className="p-4 bg-red-500/10 border border-red-500/20 rounded-xl text-red-500 text-sm">
              加载失败，请检查网络或重试。
            </div>
          )}

          {data && (
            <>
              {/* Header Info */}
              <div>
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-sm font-medium text-[var(--color-text-secondary)] uppercase tracking-wider">基础信息</h3>
                  <span className={`px-2 py-1 text-xs font-medium rounded-full ${
                    data.record.status === "pending_review" ? "bg-amber-500/10 text-amber-500 border border-amber-500/20" :
                    data.record.status === "credited" ? "bg-green-500/10 text-green-500 border border-green-500/20" :
                    "bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border border-[var(--color-border)]"
                  }`}>
                    {data.record.status}
                  </span>
                </div>
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <p className="text-[var(--color-text-secondary)] mb-1">订单号</p>
                    <p className="font-mono text-xs break-all">{data.record.orderNo}</p>
                  </div>
                  <div>
                    <p className="text-[var(--color-text-secondary)] mb-1">用户邮箱</p>
                    <p className="break-all">{data.user.email}</p>
                  </div>
                  <div>
                    <p className="text-[var(--color-text-secondary)] mb-1">请求金额 (人民币)</p>
                    <p className="font-medium text-green-500">¥ {(data.record.amountCents / 100).toFixed(2)}</p>
                  </div>
                  <div>
                    <p className="text-[var(--color-text-secondary)] mb-1">预计核发积分</p>
                    <p className="font-mono">{data.record.baseCredits} + <span className="text-amber-500">{data.record.bonusCredits} (赠)</span></p>
                  </div>
                </div>
              </div>

              {/* Submission Details */}
              <div className="pt-6 border-t border-[var(--color-border)]">
                <h3 className="text-sm font-medium text-[var(--color-text-secondary)] uppercase tracking-wider mb-4">提交凭证</h3>
                
                {data.submission ? (
                  <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-4 text-sm">
                      {data.submission.contactChannel && (
                        <div>
                          <p className="text-[var(--color-text-secondary)] mb-1">联系渠道</p>
                          <p>{data.submission.contactChannel}: {data.submission.contactHandle}</p>
                        </div>
                      )}
                      {data.submission.paymentReference && (
                        <div>
                          <p className="text-[var(--color-text-secondary)] mb-1">支付流水号</p>
                          <p className="font-mono text-xs">{data.submission.paymentReference}</p>
                        </div>
                      )}
                    </div>

                    {data.submission.customerNote && (
                      <div>
                        <p className="text-[var(--color-text-secondary)] text-sm mb-1">用户备注</p>
                        <p className="text-sm p-3 bg-[var(--color-bg-secondary)] rounded-lg text-white">
                          {data.submission.customerNote}
                        </p>
                      </div>
                    )}

                    {data.submission.proofUrls && data.submission.proofUrls.length > 0 && (
                      <div>
                        <p className="text-[var(--color-text-secondary)] text-sm mb-2">打款截图 / 凭证</p>
                        <div className="grid grid-cols-2 gap-2">
                          {data.submission.proofUrls.map((url, idx) => (
                            <a 
                              key={idx} 
                              href={url} 
                              target="_blank" 
                              rel="noreferrer"
                              className="group relative aspect-video bg-[var(--color-bg-secondary)] rounded-lg border border-[var(--color-border)] overflow-hidden flex items-center justify-center hover:border-[var(--color-primary)] transition-colors"
                            >
                              <img src={url} alt={`凭证 ${idx + 1}`} className="object-cover w-full h-full opacity-80 group-hover:opacity-100 transition-opacity" />
                              <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
                                <ExternalLink className="h-5 w-5 text-white" />
                              </div>
                            </a>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ) : (
                  <p className="text-sm text-[var(--color-text-secondary)]">暂未提交凭单信息</p>
                )}
              </div>

              {/* Review Result */}
              {data.review && data.review.status !== "pending" && (
                <div className="pt-6 border-t border-[var(--color-border)]">
                  <h3 className="text-sm font-medium text-[var(--color-text-secondary)] uppercase tracking-wider mb-4">审核结果</h3>
                  <div className={`p-4 rounded-xl border ${data.review.status === "credited" ? "bg-green-500/5 border-green-500/20" : "bg-red-500/5 border-red-500/20"}`}>
                    <div className="flex items-start gap-3">
                      {data.review.status === "credited" ? <CheckCircle className="h-5 w-5 text-green-500 mt-0.5" /> : <XCircle className="h-5 w-5 text-red-500 mt-0.5" />}
                      <div>
                        <p className="font-medium text-[var(--color-text-primary)]">
                          {data.review.status === "credited" ? "已入账并下发积分" : "已驳回充值请求"}
                        </p>
                        <p className="text-xs text-[var(--color-text-secondary)] mt-1">处理人: {data.review.operatorEmail || "系统"}</p>
                        {data.review.note && (
                          <p className="mt-2 text-sm text-[var(--color-text-primary)] bg-black/20 p-2 rounded-lg break-all">
                            审批备注: {data.review.note}
                          </p>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer Actions */}
        {data && data.actions && (data.actions.canCredit || data.actions.canReject) && (
          <div className="border-t border-[var(--color-border)] p-4 bg-[var(--color-bg-secondary)]/50 backdrop-blur sticky bottom-0">
            {isRejecting ? (
              <div className="space-y-4">
                <div>
                  <label className="text-sm text-[var(--color-text-secondary)] mb-1 block">驳回原因 (必填)</label>
                  <textarea 
                    value={rejectReason}
                    onChange={(e) => setRejectReason(e.target.value)}
                    className="w-full bg-[var(--color-bg-primary)] border border-red-500/50 rounded-lg p-3 text-sm focus:outline-none focus:border-red-500 focus:ring-1 focus:ring-red-500/50 resize-none h-24"
                    placeholder="请输入驳回原因，例如：凭证不清晰、打款金额不匹配..."
                  />
                </div>
                <div className="flex gap-3">
                  <button 
                    onClick={() => setIsRejecting(false)} 
                    className="flex-1 px-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm font-medium hover:bg-[var(--color-bg-secondary)] transition-colors"
                  >
                    取消
                  </button>
                  <button 
                    onClick={handleReject} 
                    disabled={!rejectReason || rejectMutation.isPending}
                    className="flex-1 px-4 py-2 bg-red-500 text-white rounded-lg text-sm font-medium hover:bg-red-600 transition-colors disabled:opacity-50"
                  >
                    {rejectMutation.isPending ? "驳回中..." : "确认驳回并通知用户"}
                  </button>
                </div>
              </div>
            ) : (
               <div className="space-y-4">
                 {data.actions.canCredit && (
                    <div>
                      <label className="text-sm text-[var(--color-text-secondary)] mb-1 block">内部审批备注 (选填)</label>
                      <input 
                        type="text"
                        value={creditNote}
                        onChange={(e) => setCreditNote(e.target.value)}
                        className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-green-500"
                        placeholder="例如：对公户打款收款已确认..."
                      />
                    </div>
                 )}
                 
                 <div className="flex gap-3">
                    {data.actions.canReject && (
                      <button 
                        onClick={() => setIsRejecting(true)} 
                        className="flex-1 px-4 py-2 bg-[var(--color-bg-primary)] border border-red-500/50 text-red-500 rounded-lg text-sm font-medium hover:bg-red-500/10 transition-colors"
                      >
                        驳回请求
                      </button>
                    )}
                    {data.actions.canCredit && (
                      <button 
                        onClick={handleCredit} 
                        disabled={creditMutation.isPending}
                        className="flex-1 px-4 py-2 bg-green-500 text-white rounded-lg text-sm font-medium hover:bg-green-600 transition-colors disabled:opacity-50"
                      >
                        {creditMutation.isPending ? "入账下发中..." : "核验无误，通过并入账"}
                      </button>
                    )}
                 </div>
               </div>
            )}
          </div>
        )}
      </div>
    </>
  );
}
