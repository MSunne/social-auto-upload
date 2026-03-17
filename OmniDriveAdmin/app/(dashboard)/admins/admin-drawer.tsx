"use client";

import { useEffect, useState } from "react";
import { useCreateAdmin, useUpdateAdmin } from "@/lib/hooks/useAdmins";
import { AdminIdentity, AdminRole } from "@/lib/types";
import { X, Loader2 } from "lucide-react";

interface AdminDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  admin?: AdminIdentity | null;
  roles: AdminRole[];
}

export function AdminDrawer({ isOpen, onClose, admin, roles }: AdminDrawerProps) {
  const [formData, setFormData] = useState({
    email: "",
    name: "",
    password: "",
    isActive: true,
    roleIds: [] as string[],
  });

  const createAdmin = useCreateAdmin();
  const updateAdmin = useUpdateAdmin();

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (isOpen) {
      if (admin) {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setFormData({
          email: admin.email,
          name: admin.name,
          password: "", 
          isActive: admin.isActive,
          roleIds: admin.roleIds || [],
        });
      } else {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setFormData({
          email: "",
          name: "",
          password: "",
          isActive: true,
          roleIds: [],
        });
      }
    }
  }, [isOpen, admin]);

  if (!isOpen) return null;

  const isPending = createAdmin.isPending || updateAdmin.isPending;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (admin) {
        // Edit mode
        const payload: Record<string, unknown> = { id: admin.id, name: formData.name, isActive: formData.isActive, roleIds: formData.roleIds };
        if (formData.email !== admin.email) payload.email = formData.email;
        if (formData.password) payload.password = formData.password;
        await updateAdmin.mutateAsync(payload as Parameters<typeof updateAdmin.mutateAsync>[0]);
      } else {
        // Create mode
        if (!formData.password) {
          alert("新建管理员必须设置初始密码");
          return;
        }
        await createAdmin.mutateAsync({ ...formData, password: formData.password });
      }
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

  const toggleRole = (roleId: string) => {
    setFormData(prev => {
      const ids = new Set(prev.roleIds);
      if (ids.has(roleId)) {
        ids.delete(roleId);
      } else {
        ids.add(roleId);
      }
      return { ...prev, roleIds: Array.from(ids) };
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/40 backdrop-blur-sm transition-opacity">
      <div className="w-full max-w-md bg-[var(--color-bg-primary)] border-l border-[var(--color-border)] shadow-2xl flex flex-col h-full animate-in slide-in-from-right duration-300">
        <div className="flex items-center justify-between p-6 border-b border-[var(--color-border)]">
          <h2 className="text-lg font-medium">{admin ? "编辑管理员" : "新建管理员"}</h2>
          <button onClick={onClose} className="p-2 text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors rounded-lg hover:bg-[var(--color-bg-secondary)]">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-6">
          <form id="admin-form" onSubmit={handleSubmit} className="space-y-5">
            <div>
              <label className="block text-sm font-medium mb-1.5">邮箱账号</label>
              <input type="email" required value={formData.email} onChange={e => setFormData({ ...formData, email: e.target.value })}
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors"
                placeholder="admin@example.com" />
            </div>

            <div>
              <label className="block text-sm font-medium mb-1.5">管理员花名</label>
              <input type="text" required value={formData.name} onChange={e => setFormData({ ...formData, name: e.target.value })}
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors"
                placeholder="如: 吴彦祖" />
            </div>

            <div>
              <label className="block text-sm font-medium mb-1.5">{admin ? "重置密码 (可选)" : "初始密码"}</label>
              <input type="text" minLength={8} required={!admin} value={formData.password} onChange={e => setFormData({ ...formData, password: e.target.value })}
                className="w-full px-3 py-2 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors"
                placeholder={admin ? "留空则不修改密码" : "至少 8 位数字/字母"} />
            </div>

            <div>
              <label className="block text-sm font-medium mb-3">账号状态</label>
              <label className="flex items-center gap-3 cursor-pointer">
                <input type="checkbox" checked={formData.isActive} onChange={e => setFormData({ ...formData, isActive: e.target.checked })}
                  className="w-4 h-4 rounded border-[var(--color-border)] text-[var(--color-primary)] focus:ring-[var(--color-primary)]" />
                <span className="text-sm">允许登录控制台</span>
              </label>
            </div>

            <div className="pt-2">
              <label className="block text-sm font-medium mb-3">分配角色</label>
              <div className="space-y-2">
                {roles.map(role => (
                  <label key={role.id} className="flex items-start gap-3 p-3 rounded-lg border border-[var(--color-border)] hover:bg-[var(--color-bg-secondary)] cursor-pointer transition-colors">
                    <input type="checkbox" checked={formData.roleIds.includes(role.id)} onChange={() => toggleRole(role.id)}
                      className="mt-0.5 w-4 h-4 rounded border-[var(--color-border)] text-[var(--color-primary)] focus:ring-[var(--color-primary)] bg-[var(--color-bg-primary)]" />
                    <div>
                      <div className="text-sm font-medium flex items-center gap-2">
                        {role.name}
                        {role.isSystem && <span className="px-1.5 py-0.5 text-[10px] rounded bg-purple-500/10 text-purple-400 border border-purple-500/20">系统内置</span>}
                      </div>
                      {role.description && <div className="text-xs text-[var(--color-text-secondary)] mt-0.5">{role.description}</div>}
                    </div>
                  </label>
                ))}
              </div>
            </div>
          </form>
        </div>

        <div className="p-6 border-t border-[var(--color-border)] bg-[var(--color-bg-secondary)]/50">
          <div className="flex gap-3">
            <button type="button" onClick={onClose} className="flex-1 px-4 py-2 border border-[var(--color-border)] rounded-lg text-sm font-medium hover:bg-[var(--color-bg-secondary)] transition-colors">取消</button>
            <button type="submit" form="admin-form" disabled={isPending} className="flex-1 px-4 py-2 bg-[var(--color-primary)] text-white rounded-lg text-sm font-medium hover:brightness-110 transition-all disabled:opacity-50 flex items-center justify-center gap-2">
              {isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              {admin ? "保存修改" : "确认创建"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
