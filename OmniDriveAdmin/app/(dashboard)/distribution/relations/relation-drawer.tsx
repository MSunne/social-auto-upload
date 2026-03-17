"use client";

import { useState } from "react";
import { useCreateDistributionRelation } from "@/lib/hooks/useDistribution";
import { X, Loader2 } from "lucide-react";

interface RelationDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export function RelationDrawer({ isOpen, onClose, onSuccess }: RelationDrawerProps) {
  const [formData, setFormData] = useState({
    promoterEmail: "",
    inviteeEmail: "",
    notes: "",
  });

  const createRelation = useCreateDistributionRelation();

  // Reset form when opened
  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.promoterEmail || !formData.inviteeEmail) {
      alert("推广员和受邀人邮箱均为必填项");
      return;
    }
    
    try {
      await createRelation.mutateAsync(formData);
      onSuccess();
      onClose();
    } catch (err: unknown) {
      if (err && typeof err === "object" && "response" in err) {
        const extErr = err as { response?: { data?: { error?: string } } };
        alert(extErr.response?.data?.error || "操作失败，请重试");
      } else if (err instanceof Error) {
        alert(err.message || "操作失败，请重试");
      } else {
        alert("操作失败，请重试");
      }
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/40 backdrop-blur-sm transition-opacity">
      <div className="w-full max-w-md bg-[var(--color-bg-primary)] border-l border-[var(--color-border)] shadow-2xl flex flex-col h-full animate-in slide-in-from-right duration-300">
        <div className="flex items-center justify-between p-6 border-b border-[var(--color-border)]">
          <h2 className="text-lg font-medium">人工绑定分销关系</h2>
          <button onClick={onClose} className="p-2 text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors rounded-lg hover:bg-[var(--color-bg-secondary)]">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-6">
          <div className="mb-6 p-4 rounded-xl border border-blue-500/20 bg-blue-500/5">
            <h3 className="text-sm font-medium text-blue-400 mb-1">管理员操作提示</h3>
            <p className="text-xs text-[var(--color-text-secondary)]">
              此功能用于在控制台手动为用户补绑上下级关系。操作生效后，后续受邀人的交易将开始计算佣金给推广员。
            </p>
          </div>

          <form id="relation-form" onSubmit={handleSubmit} className="space-y-5">
            <div>
              <label className="block text-sm font-medium mb-1.5">推广员 (上级) 邮箱</label>
              <input type="email" required value={formData.promoterEmail} onChange={e => setFormData({ ...formData, promoterEmail: e.target.value })}
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors"
                placeholder="promoter@example.com" />
            </div>

            <div>
              <label className="block text-sm font-medium mb-1.5">受邀人 (下级) 邮箱</label>
              <input type="email" required value={formData.inviteeEmail} onChange={e => setFormData({ ...formData, inviteeEmail: e.target.value })}
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors"
                placeholder="invitee@example.com" />
            </div>
            
            <div>
              <label className="block text-sm font-medium mb-1.5">绑定备注 (可选)</label>
              <textarea value={formData.notes} onChange={e => setFormData({ ...formData, notes: e.target.value })} rows={3}
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors"
                placeholder="例如: 客服申请人工补绑录入" />
            </div>
          </form>
        </div>

        <div className="p-6 border-t border-[var(--color-border)] bg-[var(--color-bg-secondary)]/50">
          <div className="flex gap-3">
            <button type="button" onClick={onClose} className="flex-1 px-4 py-2 border border-[var(--color-border)] rounded-lg text-sm font-medium hover:bg-[var(--color-bg-secondary)] transition-colors">取消</button>
            <button type="submit" form="relation-form" disabled={createRelation.isPending} className="flex-1 px-4 py-2 bg-blue-500 text-white rounded-lg text-sm font-medium hover:bg-blue-600 transition-all disabled:opacity-50 flex items-center justify-center gap-2">
              {createRelation.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              确认绑定
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
