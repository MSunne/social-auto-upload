"use client";

import { useState } from "react";
import { Loader2, X } from "lucide-react";
import { useOpenPartnerProfile } from "@/lib/hooks/useDistribution";

interface PartnerDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export function PartnerDrawer({ isOpen, onClose, onSuccess }: PartnerDrawerProps) {
  const [userIdentifier, setUserIdentifier] = useState("");
  const openPartner = useOpenPartnerProfile();

  if (!isOpen) {
    return null;
  }

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!userIdentifier.trim()) {
      alert("请输入用户 ID、邮箱或手机号");
      return;
    }
    try {
      await openPartner.mutateAsync({ userIdentifier: userIdentifier.trim() });
      setUserIdentifier("");
      onSuccess();
      onClose();
    } catch (error) {
      if (error instanceof Error && error.message.trim()) {
        alert(error.message.trim());
        return;
      }
      alert("新增分销员失败，请稍后重试");
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/40 backdrop-blur-sm transition-opacity">
      <div className="flex h-full w-full max-w-md flex-col border-l border-[var(--color-border)] bg-[var(--color-bg-primary)] shadow-2xl animate-in slide-in-from-right duration-300">
        <div className="flex items-center justify-between border-b border-[var(--color-border)] p-6">
          <h2 className="text-lg font-medium">新增分销员</h2>
          <button
            onClick={onClose}
            className="rounded-lg p-2 text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)] hover:text-[var(--color-text-primary)]"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-6">
          <div className="mb-6 rounded-xl border border-blue-500/20 bg-blue-500/5 p-4">
            <h3 className="mb-1 text-sm font-medium text-blue-400">开通说明</h3>
            <p className="text-xs text-[var(--color-text-secondary)]">
              输入用户 ID、邮箱或手机号即可为现有用户开通分销员档案。开通后会自动生成专属合作码，后续绑定关系和佣金统计都会进入分销体系。
            </p>
          </div>

          <form id="partner-form" onSubmit={handleSubmit} className="space-y-5">
            <div>
              <label className="mb-1.5 block text-sm font-medium">用户 ID / 邮箱 / 手机号</label>
              <input
                type="text"
                value={userIdentifier}
                onChange={(event) => setUserIdentifier(event.target.value)}
                placeholder="user-id 或 promoter@example.com"
                className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm transition-colors focus:border-[var(--color-primary)] focus:outline-none"
              />
            </div>
          </form>
        </div>

        <div className="border-t border-[var(--color-border)] bg-[var(--color-bg-secondary)]/50 p-6">
          <div className="flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 rounded-lg border border-[var(--color-border)] px-4 py-2 text-sm font-medium transition-colors hover:bg-[var(--color-bg-secondary)]"
            >
              取消
            </button>
            <button
              type="submit"
              form="partner-form"
              disabled={openPartner.isPending}
              className="flex flex-1 items-center justify-center gap-2 rounded-lg bg-blue-500 px-4 py-2 text-sm font-medium text-white transition-all hover:bg-blue-600 disabled:opacity-50"
            >
              {openPartner.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              立即开通
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
