"use client";

import { useState } from "react";
import { useBulkActionDevices, useDevices, useUpdateDeviceActivation } from "@/lib/hooks/useDevices";
import type { AdminDeviceRow } from "@/lib/types";
import { PageHeader } from "@/components/ui/common";
import {
  Search,
  Loader2,
  Monitor,
  Wifi,
  WifiOff,
  PowerOff,
  Power,
  KeyRound,
  X,
} from "lucide-react";

const formatLastSeen = (ts?: string) => {
  if (!ts) return "从未在线";
  const diff = Date.now() - new Date(ts).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 2) return "刚刚在线";
  if (mins < 60) return `${mins} 分钟前`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs} 小时前`;
  return `${Math.floor(hrs / 24)} 天前`;
};

const formatActivationStatus = (row: AdminDeviceRow) => {
  if (!row.activation) {
    return {
      label: "未配置",
      className: "text-[var(--color-text-secondary)] border-[var(--color-border)] bg-[var(--color-bg-secondary)]",
    };
  }
  if (row.activation.status === "disabled") {
    return {
      label: "已禁用",
      className: "text-red-400 border-red-500/30 bg-red-500/10",
    };
  }
  if (row.activation.status === "activated") {
    return {
      label: "已激活",
      className: "text-green-400 border-green-500/30 bg-green-500/10",
    };
  }
  return {
    label: "待激活",
    className: "text-amber-400 border-amber-500/30 bg-amber-500/10",
  };
};

export function DevicesView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [activationDevice, setActivationDevice] = useState<AdminDeviceRow | null>(null);
  const [activationOrderNo, setActivationOrderNo] = useState("");
  const [activationCode, setActivationCode] = useState("");
  const [activationStatus, setActivationStatus] = useState("ready");
  const [activationNotes, setActivationNotes] = useState("");

  const { data, isLoading, error } = useDevices({
    page,
    pageSize: 20,
    query: query || undefined,
    status: status || undefined,
  });
  const bulkAction = useBulkActionDevices();
  const updateActivation = useUpdateDeviceActivation();

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setQuery(searchInput);
    setPage(1);
    setSelected(new Set());
  };

  const toggleSelect = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });

  const toggleSelectAll = () => {
    if (!data) return;
    const allIds = data.items.map((row) => row.device.id);
    setSelected(selected.size === allIds.length ? new Set() : new Set(allIds));
  };

  const openActivationModal = (row: AdminDeviceRow) => {
    setActivationDevice(row);
    setActivationOrderNo(row.activation?.orderNo ?? "");
    setActivationCode("");
    setActivationStatus(row.activation?.status === "disabled" ? "disabled" : "ready");
    setActivationNotes(row.activation?.notes ?? "");
  };

  const closeActivationModal = () => {
    if (updateActivation.isPending) return;
    setActivationDevice(null);
    setActivationOrderNo("");
    setActivationCode("");
    setActivationStatus("ready");
    setActivationNotes("");
  };

  const handleActivationSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!activationDevice) return;
    try {
      await updateActivation.mutateAsync({
        deviceId: activationDevice.device.id,
        orderNo: activationOrderNo.trim() || undefined,
        activationCode: activationCode.trim() || undefined,
        status: activationStatus,
        notes: activationNotes.trim() || undefined,
      });
      closeActivationModal();
    } catch (mutationError) {
      const message = mutationError instanceof Error ? mutationError.message : "保存激活配置失败，请重试";
      alert(message);
    }
  };

  const handleBulkAction = async (action: "disable" | "enable") => {
    if (selected.size === 0) return;
    const label = action === "disable" ? "停用" : "启用";
    if (!confirm(`确认对 ${selected.size} 台设备执行「${label}」操作？`)) return;
    try {
      await bulkAction.mutateAsync({ ids: Array.from(selected), action });
      setSelected(new Set());
    } catch {
      alert("操作失败，请重试");
    }
  };

  const getStatusIndicator = (deviceStatus: string, isEnabled: boolean) => {
    if (!isEnabled) {
      return (
        <span className="flex items-center gap-1.5 text-xs text-[var(--color-text-secondary)]">
          <PowerOff className="h-3.5 w-3.5" /> 已停用
        </span>
      );
    }
    if (deviceStatus === "online") {
      return (
        <span className="flex items-center gap-1.5 text-xs text-green-400">
          <Wifi className="h-3.5 w-3.5" /> 在线
        </span>
      );
    }
    return (
      <span className="flex items-center gap-1.5 text-xs text-[var(--color-text-secondary)]">
        <WifiOff className="h-3.5 w-3.5" /> 离线
      </span>
    );
  };

  return (
    <div className="space-y-6">
      <PageHeader title="设备管理" subtitle="监控设备在线状态，并维护客户激活所需的设备码与激活码配置。" />

      <div className="flex flex-col items-start gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4 sm:flex-row sm:items-center">
        <form onSubmit={handleSearch} className="relative max-w-md flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            placeholder="搜索设备名称 / DeviceCode / IP..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] py-2 pl-9 pr-4 text-sm transition-all focus:border-[var(--color-primary)] focus:outline-none"
          />
        </form>
        <select
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            setPage(1);
            setSelected(new Set());
          }}
          className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
        >
          <option value="">全部设备</option>
          <option value="online">在线</option>
          <option value="offline">离线</option>
        </select>

        {selected.size > 0 && (
          <div className="ml-auto flex items-center gap-2">
            <span className="text-sm text-[var(--color-text-secondary)]">已选 {selected.size} 台</span>
            <button
              onClick={() => handleBulkAction("enable")}
              className="flex items-center gap-1.5 rounded-lg border border-green-500/30 px-3 py-1.5 text-xs font-medium text-green-400 transition-colors hover:bg-green-500/10"
            >
              <Power className="h-3.5 w-3.5" /> 批量启用
            </button>
            <button
              onClick={() => handleBulkAction("disable")}
              className="flex items-center gap-1.5 rounded-lg border border-red-500/30 px-3 py-1.5 text-xs font-medium text-red-400 transition-colors hover:bg-red-500/10"
            >
              <PowerOff className="h-3.5 w-3.5" /> 批量停用
            </button>
          </div>
        )}
      </div>

      <div className="overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-xs uppercase text-[var(--color-text-secondary)]">
              <tr>
                <th className="px-4 py-4">
                  <input
                    type="checkbox"
                    className="rounded"
                    checked={data ? selected.size === data.items.length && data.items.length > 0 : false}
                    onChange={toggleSelectAll}
                  />
                </th>
                <th className="px-4 py-4 font-medium">设备信息</th>
                <th className="px-4 py-4 font-medium">归属用户</th>
                <th className="px-4 py-4 font-medium">状态</th>
                <th className="px-4 py-4 font-medium">激活配置</th>
                <th className="px-4 py-4 font-medium">账号 / 任务负载</th>
                <th className="px-4 py-4 font-medium">IP 地址</th>
                <th className="px-4 py-4 font-medium">最后在线</th>
                <th className="px-4 py-4 font-medium">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr>
                  <td colSpan={9} className="px-6 py-12 text-center">
                    <Loader2 className="mx-auto h-6 w-6 animate-spin text-[var(--color-text-secondary)]" />
                    <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载设备数据中...</p>
                  </td>
                </tr>
              )}
              {error && (
                <tr>
                  <td colSpan={9} className="px-6 py-10 text-center text-sm text-red-500">
                    加载失败，请重试
                  </td>
                </tr>
              )}
              {data && data.items.length === 0 && (
                <tr>
                  <td colSpan={9} className="px-6 py-12 text-center text-sm text-[var(--color-text-secondary)]">
                    未找到设备
                  </td>
                </tr>
              )}
              {data?.items.map((row) => {
                const activationMeta = formatActivationStatus(row);
                return (
                  <tr
                    key={row.device.id}
                    className={`transition-colors hover:bg-[var(--color-bg-secondary)]/50 ${
                      selected.has(row.device.id) ? "bg-[var(--color-primary)]/5" : ""
                    }`}
                  >
                    <td className="px-4 py-4">
                      <input
                        type="checkbox"
                        className="rounded"
                        checked={selected.has(row.device.id)}
                        onChange={() => toggleSelect(row.device.id)}
                      />
                    </td>
                    <td className="px-4 py-4">
                      <div className="flex items-center gap-3">
                        <div
                          className={`flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg ${
                            row.device.status === "online" && row.device.isEnabled
                              ? "bg-green-500/10"
                              : "bg-[var(--color-bg-secondary)]"
                          }`}
                        >
                          <Monitor
                            className={`h-4 w-4 ${
                              row.device.status === "online" && row.device.isEnabled
                                ? "text-green-400"
                                : "text-[var(--color-text-secondary)]"
                            }`}
                          />
                        </div>
                        <div>
                          <div className="font-medium">{row.device.name}</div>
                          <div className="mt-0.5 text-xs font-mono text-[var(--color-text-secondary)]">
                            {row.device.deviceCode}
                          </div>
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-4">
                      {row.owner ? (
                        <div>
                          <div className="text-sm">{row.owner.name}</div>
                          <div className="text-xs text-[var(--color-text-secondary)]">{row.owner.email}</div>
                        </div>
                      ) : (
                        <span className="text-xs text-[var(--color-text-secondary)]">— 未绑定</span>
                      )}
                    </td>
                    <td className="px-4 py-4">{getStatusIndicator(row.device.status, row.device.isEnabled)}</td>
                    <td className="px-4 py-4">
                      <div className="space-y-1">
                        <span className={`inline-flex rounded-full border px-2.5 py-1 text-xs font-medium ${activationMeta.className}`}>
                          {activationMeta.label}
                        </span>
                        <div className="text-xs text-[var(--color-text-secondary)]">
                          订单号: {row.activation?.orderNo || "—"}
                        </div>
                        <div className="text-xs text-[var(--color-text-secondary)]">
                          提示: {row.activation?.activationCodeHint || "未设置"}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-4">
                      <div className="text-sm">
                        {row.device.load.activeAccountCount}/{row.device.load.accountCount} 账号
                      </div>
                      <div className="mt-0.5 flex gap-2 text-xs text-[var(--color-text-secondary)]">
                        <span className="text-amber-400">{row.device.load.runningTaskCount} 运行</span>
                        <span>{row.device.load.pendingTaskCount} 等待</span>
                        {row.device.load.failedTaskCount > 0 && (
                          <span className="text-red-400">{row.device.load.failedTaskCount} 失败</span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-4 font-mono text-xs text-[var(--color-text-secondary)]">
                      <div>公网: {row.device.publicIp || "—"}</div>
                      <div className="mt-0.5">内网: {row.device.localIp || "—"}</div>
                    </td>
                    <td className="px-4 py-4 text-xs text-[var(--color-text-secondary)]">
                      {formatLastSeen(row.device.lastSeenAt)}
                    </td>
                    <td className="px-4 py-4">
                      <button
                        type="button"
                        onClick={() => openActivationModal(row)}
                        className="inline-flex items-center gap-1.5 rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-xs font-medium transition-colors hover:bg-[var(--color-bg-secondary)]"
                      >
                        <KeyRound className="h-3.5 w-3.5" />
                        {row.activation ? "更新激活配置" : "配置激活码"}
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {data?.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">
            共 <span className="font-medium">{data.pagination.total}</span> 台设备
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page === 1}
              className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm transition-colors hover:bg-[var(--color-bg-secondary)] disabled:opacity-50"
            >
              上一页
            </button>
            <button
              onClick={() => setPage((current) => Math.min(data.pagination.totalPages, current + 1))}
              disabled={page >= data.pagination.totalPages}
              className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm transition-colors hover:bg-[var(--color-bg-secondary)] disabled:opacity-50"
            >
              下一页
            </button>
          </div>
        </div>
      )}

      {activationDevice && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4">
          <div className="w-full max-w-lg rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] shadow-2xl">
            <div className="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-4">
              <div>
                <h3 className="text-base font-semibold">设备激活配置</h3>
                <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                  {activationDevice.device.name} / {activationDevice.device.deviceCode}
                </p>
              </div>
              <button
                type="button"
                onClick={closeActivationModal}
                className="rounded-full p-2 text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)]"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <form onSubmit={handleActivationSubmit} className="space-y-4 px-5 py-5">
              <div className="grid gap-4 sm:grid-cols-2">
                <label className="space-y-2">
                  <span className="text-xs font-medium text-[var(--color-text-secondary)]">订单号</span>
                  <input
                    value={activationOrderNo}
                    onChange={(e) => setActivationOrderNo(e.target.value)}
                    placeholder="例如 SO-20260327-001"
                    className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
                  />
                </label>
                <label className="space-y-2">
                  <span className="text-xs font-medium text-[var(--color-text-secondary)]">状态</span>
                  <select
                    value={activationStatus}
                    onChange={(e) => setActivationStatus(e.target.value)}
                    className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
                  >
                    <option value="ready">待激活</option>
                    <option value="disabled">已禁用</option>
                  </select>
                </label>
              </div>

              <label className="space-y-2">
                <span className="text-xs font-medium text-[var(--color-text-secondary)]">激活码</span>
                <input
                  value={activationCode}
                  onChange={(e) => setActivationCode(e.target.value)}
                  placeholder={activationDevice.activation ? "如需轮换激活码，请重新输入" : "首次配置必须填写激活码"}
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm uppercase focus:border-[var(--color-primary)] focus:outline-none"
                />
                <p className="text-xs text-[var(--color-text-secondary)]">
                  系统只保存激活码校验信息和尾号提示，不会在列表里回显完整激活码。
                </p>
              </label>

              <label className="space-y-2">
                <span className="text-xs font-medium text-[var(--color-text-secondary)]">备注</span>
                <textarea
                  value={activationNotes}
                  onChange={(e) => setActivationNotes(e.target.value)}
                  rows={3}
                  placeholder="例如：由供应商 03-27 回执导入，客服 Alice 已通知客户。"
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
                />
              </label>

              <div className="flex items-center justify-end gap-3 border-t border-[var(--color-border)] pt-4">
                <button
                  type="button"
                  onClick={closeActivationModal}
                  disabled={updateActivation.isPending}
                  className="rounded-lg px-4 py-2 text-sm text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)]"
                >
                  取消
                </button>
                <button
                  type="submit"
                  disabled={updateActivation.isPending}
                  className="rounded-lg bg-[var(--color-primary)] px-4 py-2 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {updateActivation.isPending ? "保存中..." : "保存配置"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
