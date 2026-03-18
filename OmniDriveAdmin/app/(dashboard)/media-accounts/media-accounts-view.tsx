"use client";

import { type FormEvent, useEffect, useMemo, useState } from "react";
import Image from "next/image";
import { Loader2, Plus, RefreshCw, Search, ShieldCheck, Trash2, X } from "lucide-react";

import { PageHeader } from "@/components/ui/common";
import { adminApi, adminPaths } from "@/lib/api";
import { useDevices } from "@/lib/hooks/useDevices";
import { useBulkActionMediaAccounts, useMediaAccounts } from "@/lib/hooks/useMediaAccounts";
import type { LoginSession } from "@/lib/types";

const PLATFORMS = ["", "douyin", "bilibili", "xiaohongshu", "kuaishou", "wechat_channel", "baijiahao", "tiktok"];

const PLATFORM_LABELS: Record<string, string> = {
  "": "全部",
  douyin: "抖音",
  bilibili: "Bilibili",
  xiaohongshu: "小红书",
  kuaishou: "快手",
  wechat_channel: "视频号",
  baijiahao: "百家号",
  tiktok: "TikTok",
};

const PLATFORM_ALIASES: Record<string, string> = {
  抖音: "douyin",
  小红书: "xiaohongshu",
  快手: "kuaishou",
  视频号: "wechat_channel",
};

const REMOTE_LOGIN_PLATFORM_OPTIONS = [
  { value: "douyin", cloudPlatform: "抖音" },
  { value: "xiaohongshu", cloudPlatform: "小红书" },
  { value: "kuaishou", cloudPlatform: "快手" },
  { value: "wechat_channel", cloudPlatform: "视频号" },
];

const SESSION_FINAL_STATUSES = new Set(["success", "failed", "cancelled"]);

type NoticeTone = "info" | "success" | "error";

type SessionMeta = {
  sourceKey: string;
  deviceName?: string;
  deviceCode?: string;
};

function normalizePlatformKey(platform: string): string {
  return PLATFORM_ALIASES[platform] || platform;
}

function getPlatformLabel(platform: string): string {
  const key = normalizePlatformKey(platform);
  return PLATFORM_LABELS[key] || platform;
}

function getPlatformColor(platform: string) {
  const key = normalizePlatformKey(platform);
  const map: Record<string, string> = {
    douyin: "bg-black text-white",
    bilibili: "bg-[#00A1D6]/10 text-[#00A1D6] border border-[#00A1D6]/20",
    xiaohongshu: "bg-red-500/10 text-red-400 border border-red-500/20",
    kuaishou: "bg-yellow-500/10 text-yellow-400 border border-yellow-500/20",
    wechat_channel: "bg-green-600/10 text-green-400 border border-green-600/20",
    tiktok: "bg-[#010101]/10 text-[var(--color-text-secondary)] border border-[var(--color-border)]",
    baijiahao: "bg-blue-500/10 text-blue-400 border border-blue-500/20",
  };
  return map[key] || "bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border border-[var(--color-border)]";
}

function getStatusBadge(status: string) {
  const map: Record<string, string> = {
    active: "bg-green-500/10 text-green-400 border-green-500/20",
    inactive: "bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border-[var(--color-border)]",
    needs_login: "bg-orange-500/10 text-orange-400 border-orange-500/20",
    locked: "bg-red-500/10 text-red-400 border-red-500/20",
    banned: "bg-red-700/10 text-red-500 border-red-700/20",
  };
  const labels: Record<string, string> = {
    active: "正常",
    inactive: "未激活",
    needs_login: "需重登",
    locked: "已锁定",
    banned: "已封号",
  };
  const cls = map[status] ?? "bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border-[var(--color-border)]";
  return <span className={`px-2 py-0.5 text-xs rounded-full border font-medium ${cls}`}>{labels[status] || status}</span>;
}

function getSessionStatusTone(status: string): NoticeTone {
  if (status === "success") {
    return "success";
  }
  if (status === "failed" || status === "cancelled") {
    return "error";
  }
  return "info";
}

function getSessionStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    pending: "等待拉起",
    running: "登录中",
    verification_required: "等待验证",
    success: "已完成",
    failed: "失败",
    cancelled: "已取消",
  };
  return labels[status] || status;
}

function formatSessionSummary(session: LoginSession, meta: SessionMeta | null): string {
  const platformLabel = getPlatformLabel(session.platform);
  const statusLabel = getSessionStatusLabel(session.status);
  const accountName = session.accountName || "未命名账号";
  const deviceLabel = meta?.deviceName ? `，设备 ${meta.deviceName}` : "";
  return `${platformLabel} / ${accountName}${deviceLabel}，当前状态：${statusLabel}`;
}

function formatErrorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

export function MediaAccountsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [platform, setPlatform] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [actionLoadingKey, setActionLoadingKey] = useState<string | null>(null);
  const [notice, setNotice] = useState<{ tone: NoticeTone; text: string } | null>(null);
  const [showAddDialog, setShowAddDialog] = useState(false);
  const [addAccountName, setAddAccountName] = useState("");
  const [addPlatform, setAddPlatform] = useState("douyin");
  const [addDeviceId, setAddDeviceId] = useState("");
  const [activeSession, setActiveSession] = useState<LoginSession | null>(null);
  const [activeSessionMeta, setActiveSessionMeta] = useState<SessionMeta | null>(null);
  const [sessionActionLoading, setSessionActionLoading] = useState<string | null>(null);

  const { data, isLoading, error, refetch } = useMediaAccounts({
    page,
    pageSize: 20,
    query: query || undefined,
    platform: platform || undefined,
  });
  const { data: devicesData, isLoading: isDevicesLoading } = useDevices({ page: 1, pageSize: 100 });
  const bulkAction = useBulkActionMediaAccounts();

  const availableDevices = useMemo(
    () => (devicesData?.items ?? []).filter((row) => row.device.isEnabled),
    [devicesData]
  );
  const effectiveAddDeviceId = addDeviceId || availableDevices[0]?.device.id || "";
  const selectedDevice = availableDevices.find((row) => row.device.id === effectiveAddDeviceId) || null;
  const isSessionFinal = activeSession ? SESSION_FINAL_STATUSES.has(activeSession.status) : false;
  const activeSessionSourceKey = !isSessionFinal ? activeSessionMeta?.sourceKey : null;

  useEffect(() => {
    if (!activeSession?.id || SESSION_FINAL_STATUSES.has(activeSession.status)) {
      return;
    }

    let cancelled = false;

    const pollSession = async () => {
      try {
        const { data: session } = await adminApi.get<LoginSession>(`${adminPaths.loginSessions}/${activeSession.id}`);
        if (cancelled) {
          return;
        }
        setActiveSession(session);
        setActionLoadingKey((current) => (current === activeSessionMeta?.sourceKey ? null : current));

        if (SESSION_FINAL_STATUSES.has(session.status)) {
          setSessionActionLoading(null);
          const tone = getSessionStatusTone(session.status);
          setNotice({
            tone,
            text: session.message || formatSessionSummary(session, activeSessionMeta),
          });
          if (session.status === "success") {
            void refetch();
          }
        }
      } catch (pollError) {
        if (cancelled) {
          return;
        }
        setNotice({
          tone: "error",
          text: formatErrorMessage(pollError, "获取登录会话状态失败，请稍后刷新页面查看最新状态。"),
        });
      }
    };

    void pollSession();
    const intervalId = window.setInterval(() => {
      void pollSession();
    }, 1000);

    return () => {
      cancelled = true;
      window.clearInterval(intervalId);
    };
  }, [activeSession?.id, activeSession?.status, activeSessionMeta, refetch]);

  const handleSearch = (event: FormEvent) => {
    event.preventDefault();
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
    if (!data?.items) {
      return;
    }
    const allIds = data.items.map((row) => row.account.id);
    setSelected(selected.size === allIds.length ? new Set() : new Set(allIds));
  };

  const beginSessionTracking = (session: LoginSession, meta: SessionMeta, openingText: string) => {
    setActiveSession(session);
    setActiveSessionMeta(meta);
    setActionLoadingKey(meta.sourceKey);
    setNotice({
      tone: "info",
      text: openingText,
    });
  };

  const handleBulkDelete = async () => {
    if (selected.size === 0) {
      return;
    }
    if (!confirm(`确认永久删除 ${selected.size} 个媒体账号？此操作不可撤销！`)) {
      return;
    }
    try {
      await bulkAction.mutateAsync({ ids: Array.from(selected), action: "delete" });
      setSelected(new Set());
      setNotice({ tone: "success", text: "媒体账号已删除。" });
      void refetch();
    } catch (bulkError) {
      setNotice({
        tone: "error",
        text: formatErrorMessage(bulkError, "删除失败，请重试。"),
      });
    }
  };

  const handleRefreshQRCode = async () => {
    if (!activeSession || SESSION_FINAL_STATUSES.has(activeSession.status)) {
      return;
    }

    setSessionActionLoading("refresh_qr");
    try {
      await adminApi.post(`${adminPaths.loginSessions}/${activeSession.id}/actions`, {
        actionType: "refresh_qr",
      });
      setNotice({
        tone: "info",
        text: "已请求本地 SAU 刷新二维码，新的二维码会在几秒内同步回来。",
      });
      setActiveSession((current) => (
        current
          ? {
              ...current,
              message: "已下发刷新二维码请求，等待本地 SAU 返回最新二维码。",
            }
          : current
      ));
    } catch (refreshError) {
      setNotice({
        tone: "error",
        text: formatErrorMessage(refreshError, "刷新二维码失败，请稍后重试。"),
      });
    } finally {
      setSessionActionLoading(null);
    }
  };

  const handleValidate = async (accountId: string) => {
    const row = data?.items.find((item) => item.account.id === accountId);
    if (!row) {
      setNotice({ tone: "error", text: "未找到对应账号，无法发起验证。" });
      return;
    }

    const sourceKey = `validate:${row.account.id}`;
    setActionLoadingKey(sourceKey);
    try {
      const { data: session } = await adminApi.post<LoginSession>(`${adminPaths.mediaAccounts}/${accountId}/validate`);
      beginSessionTracking(session, {
        sourceKey,
        deviceName: row.device.name,
        deviceCode: row.device.deviceCode,
      }, `已创建 ${getPlatformLabel(row.account.platform)} 账号验证会话，等待设备 ${row.device.name} 上的 SAU 拉起登录窗口。`);
      void refetch();
    } catch (validateError) {
      setActionLoadingKey(null);
      setNotice({
        tone: "error",
        text: formatErrorMessage(validateError, "发起验证失败，请重试。"),
      });
    }
  };

  const handleAddAccount = async (event: FormEvent) => {
    event.preventDefault();

    const trimmedAccountName = addAccountName.trim();
    if (!effectiveAddDeviceId) {
      setNotice({ tone: "error", text: "请选择要执行登录的设备。" });
      return;
    }
    if (!trimmedAccountName) {
      setNotice({ tone: "error", text: "请输入账号名。" });
      return;
    }

    const platformConfig = REMOTE_LOGIN_PLATFORM_OPTIONS.find((item) => item.value === addPlatform);
    if (!platformConfig) {
      setNotice({ tone: "error", text: "当前平台暂不支持本地 SAU 登录拉起。" });
      return;
    }

    const sourceKey = `add:${effectiveAddDeviceId}:${platformConfig.value}:${trimmedAccountName}`;
    setActionLoadingKey(sourceKey);
    try {
      const { data: session } = await adminApi.post<LoginSession>(`${adminPaths.mediaAccounts}/remote-login`, {
        deviceId: effectiveAddDeviceId,
        platform: platformConfig.cloudPlatform,
        accountName: trimmedAccountName,
      });
      beginSessionTracking(session, {
        sourceKey,
        deviceName: selectedDevice?.device.name,
        deviceCode: selectedDevice?.device.deviceCode,
      }, `已创建 ${getPlatformLabel(platformConfig.value)} 账号登录会话，等待设备 ${selectedDevice?.device.name || "本地 SAU"} 拉起浏览器登录。`);
      setShowAddDialog(false);
      setAddAccountName("");
      void refetch();
    } catch (addError) {
      setActionLoadingKey(null);
      setNotice({
        tone: "error",
        text: formatErrorMessage(addError, "添加账号失败，请重试。"),
      });
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader title="媒体账号管理" subtitle="查看平台账号状态、任务负载，并执行批量运维操作。" />
        <div className="flex items-center gap-2">
          <button
            onClick={() => {
              if (!availableDevices.length) {
                setNotice({ tone: "error", text: "当前没有可用设备，无法拉起本地 SAU 登录。" });
                return;
              }
              setAddPlatform("douyin");
              setAddAccountName("");
              setAddDeviceId((current) => current || availableDevices[0].device.id);
              setShowAddDialog(true);
            }}
            className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm border border-[var(--color-primary)]/30 text-[var(--color-primary)] hover:bg-[var(--color-primary)]/10 transition-colors"
          >
            <Plus className="h-4 w-4" /> 添加账号
          </button>
          <button
            onClick={() => refetch()}
            className="flex items-center gap-2 px-3 py-2 border border-[var(--color-border)] rounded-lg text-sm hover:bg-[var(--color-bg-secondary)] transition-colors"
          >
            <RefreshCw className="h-4 w-4" /> 刷新
          </button>
        </div>
      </div>

      {notice && (
        <div
          className={`flex items-start justify-between gap-3 rounded-xl border px-4 py-3 text-sm ${
            notice.tone === "success"
              ? "border-green-500/30 bg-green-500/10 text-green-300"
              : notice.tone === "error"
                ? "border-red-500/30 bg-red-500/10 text-red-300"
                : "border-blue-500/30 bg-blue-500/10 text-blue-200"
          }`}
        >
          <p className="leading-6">{notice.text}</p>
          <button
            type="button"
            onClick={() => setNotice(null)}
            className="rounded-md p-1 hover:bg-black/10 transition-colors"
            aria-label="关闭提示"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      )}

      {activeSession && (
        <div className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-5 shadow-sm">
          <div className="flex items-start justify-between gap-4">
            <div>
              <div className="flex items-center gap-2">
                <span className={`px-2 py-0.5 text-xs rounded font-medium ${getPlatformColor(activeSession.platform)}`}>
                  {getPlatformLabel(activeSession.platform)}
                </span>
                <span className="text-xs text-[var(--color-text-secondary)]">{getSessionStatusLabel(activeSession.status)}</span>
              </div>
              <h3 className="mt-2 text-base font-semibold text-[var(--color-text-primary)]">{activeSession.accountName}</h3>
              <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
                {activeSession.message || formatSessionSummary(activeSession, activeSessionMeta)}
              </p>
              {(activeSessionMeta?.deviceName || activeSessionMeta?.deviceCode) && (
                <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                  执行设备：{activeSessionMeta?.deviceName || "未知设备"}
                  {activeSessionMeta?.deviceCode ? ` (${activeSessionMeta.deviceCode})` : ""}
                </p>
              )}
            </div>
            <div className="flex items-center gap-2">
              {!SESSION_FINAL_STATUSES.has(activeSession.status) && (
                <button
                  type="button"
                  onClick={() => void handleRefreshQRCode()}
                  disabled={sessionActionLoading === "refresh_qr"}
                  className="inline-flex items-center gap-2 rounded-lg border border-[var(--color-primary)]/30 px-3 py-2 text-xs text-[var(--color-primary)] hover:bg-[var(--color-primary)]/10 transition-colors disabled:opacity-50"
                >
                  <RefreshCw className={`h-3.5 w-3.5 ${sessionActionLoading === "refresh_qr" ? "animate-spin" : ""}`} />
                  刷新二维码
                </button>
              )}
              <button
                type="button"
                onClick={() => {
                  setActiveSession(null);
                  setActiveSessionMeta(null);
                  setActionLoadingKey(null);
                  setSessionActionLoading(null);
                }}
                className="rounded-lg p-2 text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-secondary)] transition-colors"
                aria-label="关闭登录会话面板"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          </div>

          {(activeSession.qrData || activeSession.verificationPayload?.screenshotData) && (
            <div className="mt-4 grid gap-4 lg:grid-cols-2">
              {activeSession.qrData && (
                <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4">
                  <p className="text-sm font-medium text-[var(--color-text-primary)]">本地 SAU 返回的登录二维码</p>
                  <Image
                    src={activeSession.qrData}
                    alt="登录二维码"
                    width={320}
                    height={320}
                    unoptimized
                    className="mt-3 w-full max-w-[280px] rounded-lg border border-[var(--color-border)] bg-white"
                  />
                </div>
              )}
              {activeSession.verificationPayload?.screenshotData && (
                <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4">
                  <p className="text-sm font-medium text-[var(--color-text-primary)]">
                    {activeSession.verificationPayload.title || "登录验证截图"}
                  </p>
                  {activeSession.verificationPayload.message && (
                    <p className="mt-1 text-xs leading-5 text-[var(--color-text-secondary)]">
                      {activeSession.verificationPayload.message}
                    </p>
                  )}
                  <Image
                    src={activeSession.verificationPayload.screenshotData}
                    alt="登录验证截图"
                    width={1200}
                    height={900}
                    unoptimized
                    className="mt-3 w-full rounded-lg border border-[var(--color-border)]"
                  />
                </div>
              )}
            </div>
          )}
        </div>
      )}

      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center bg-[var(--color-bg-secondary)] p-4 rounded-xl border border-[var(--color-border)]">
        <form onSubmit={handleSearch} className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            placeholder="搜索账号名称 / 用户 / 设备..."
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]"
          />
        </form>
        <div className="flex gap-1 flex-wrap">
          {PLATFORMS.map((item) => (
            <button
              key={item}
              onClick={() => {
                setPlatform(item);
                setPage(1);
                setSelected(new Set());
              }}
              className={`px-3 py-1.5 text-xs rounded-lg border transition-colors ${
                platform === item
                  ? "bg-[var(--color-primary)]/10 border-[var(--color-primary)]/50 text-[var(--color-primary)]"
                  : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-primary)]"
              }`}
            >
              {PLATFORM_LABELS[item]}
            </button>
          ))}
        </div>
        {selected.size > 0 && (
          <div className="flex items-center gap-2 ml-auto">
            <span className="text-xs text-[var(--color-text-secondary)]">已选 {selected.size}</span>
            <button
              onClick={handleBulkDelete}
              className="flex items-center gap-1 px-2.5 py-1.5 text-xs text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/10 transition-colors"
            >
              <Trash2 className="h-3 w-3" /> 批量删除
            </button>
          </div>
        )}
      </div>

      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-4 py-3.5">
                  <input
                    type="checkbox"
                    className="rounded"
                    checked={data ? selected.size === data.items.length && data.items.length > 0 : false}
                    onChange={toggleSelectAll}
                  />
                </th>
                <th className="px-4 py-3.5 font-medium">账号信息</th>
                <th className="px-4 py-3.5 font-medium">归属用户</th>
                <th className="px-4 py-3.5 font-medium">运行设备</th>
                <th className="px-4 py-3.5 font-medium">状态</th>
                <th className="px-4 py-3.5 font-medium">任务负载</th>
                <th className="px-4 py-3.5 font-medium">最近验证</th>
                <th className="px-4 py-3.5 font-medium text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr>
                  <td colSpan={8} className="px-6 py-12 text-center">
                    <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                    <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载账号数据中...</p>
                  </td>
                </tr>
              )}
              {error && (
                <tr>
                  <td colSpan={8} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td>
                </tr>
              )}
              {data && data.items.length === 0 && (
                <tr>
                  <td colSpan={8} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">未找到媒体账号</td>
                </tr>
              )}
              {data && data.items.map((row) => {
                const validateSourceKey = `validate:${row.account.id}`;
                const isValidating = activeSessionSourceKey === validateSourceKey || actionLoadingKey === validateSourceKey;
                return (
                  <tr
                    key={row.account.id}
                    className={`hover:bg-[var(--color-bg-secondary)]/50 transition-colors ${
                      selected.has(row.account.id) ? "bg-[var(--color-primary)]/5" : ""
                    }`}
                  >
                    <td className="px-4 py-3.5">
                      <input
                        type="checkbox"
                        className="rounded"
                        checked={selected.has(row.account.id)}
                        onChange={() => toggleSelect(row.account.id)}
                      />
                    </td>
                    <td className="px-4 py-3.5">
                      <div className="flex items-center gap-2.5">
                        <span className={`px-2 py-0.5 text-xs rounded font-medium ${getPlatformColor(row.account.platform)}`}>
                          {getPlatformLabel(row.account.platform)}
                        </span>
                        <div>
                          <div className="font-medium text-[var(--color-text-primary)]">{row.account.accountName}</div>
                          {row.account.lastMessage && (
                            <div
                              className="text-xs text-[var(--color-text-secondary)] mt-0.5 max-w-[180px] truncate"
                              title={row.account.lastMessage}
                            >
                              {row.account.lastMessage}
                            </div>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3.5">
                      {row.owner ? (
                        <>
                          <div className="text-sm">{row.owner.name}</div>
                          <div className="text-xs text-[var(--color-text-secondary)]">{row.owner.email}</div>
                        </>
                      ) : (
                        <span className="text-xs text-[var(--color-text-secondary)]">—</span>
                      )}
                    </td>
                    <td className="px-4 py-3.5">
                      <div className="text-sm font-medium">{row.device.name}</div>
                      <div className="text-xs font-mono text-[var(--color-text-secondary)]">{row.device.deviceCode}</div>
                    </td>
                    <td className="px-4 py-3.5">{getStatusBadge(row.account.status)}</td>
                    <td className="px-4 py-3.5">
                      <div className="text-xs space-y-0.5">
                        <div className="flex gap-2">
                          {row.account.load.runningTaskCount > 0 && <span className="text-blue-400">{row.account.load.runningTaskCount} 运行</span>}
                          {row.account.load.pendingTaskCount > 0 && <span className="text-[var(--color-text-secondary)]">{row.account.load.pendingTaskCount} 等待</span>}
                          {row.account.load.needsVerifyTaskCount > 0 && <span className="text-purple-400">{row.account.load.needsVerifyTaskCount} 待核</span>}
                          {row.account.load.failedTaskCount > 0 && <span className="text-red-400">{row.account.load.failedTaskCount} 失败</span>}
                          {row.account.load.taskCount === 0 && <span className="text-[var(--color-text-secondary)]">无任务</span>}
                        </div>
                        {row.account.load.activeLoginSessionCount > 0 && (
                          <div className="text-orange-400">{row.account.load.activeLoginSessionCount} 登录会话</div>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3.5 text-xs text-[var(--color-text-secondary)]">
                      {row.account.lastAuthenticatedAt
                        ? new Date(row.account.lastAuthenticatedAt).toLocaleString("zh-CN", {
                            month: "2-digit",
                            day: "2-digit",
                            hour: "2-digit",
                            minute: "2-digit",
                          })
                        : "从未"}
                    </td>
                    <td className="px-4 py-3.5 text-right">
                      {row.actions.canValidate && (
                        <button
                          onClick={() => handleValidate(row.account.id)}
                          disabled={isValidating}
                          className="inline-flex items-center gap-1 px-2.5 py-1.5 text-xs text-[var(--color-primary)] border border-[var(--color-primary)]/30 rounded-lg hover:bg-[var(--color-primary)]/10 transition-colors disabled:opacity-50"
                        >
                          <ShieldCheck className="h-3 w-3" />
                          {isValidating ? "进行中" : "验证"}
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">
            共 <span className="font-medium">{data.pagination.total}</span> 个媒体账号
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page === 1}
              className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors"
            >
              上一页
            </button>
            <button
              onClick={() => setPage((current) => Math.min(data.pagination.totalPages, current + 1))}
              disabled={page >= data.pagination.totalPages}
              className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors"
            >
              下一页
            </button>
          </div>
        </div>
      )}

      {showAddDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/55 px-4">
          <div className="w-full max-w-lg rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] shadow-2xl">
            <div className="flex items-start justify-between gap-4 border-b border-[var(--color-border)] px-6 py-5">
              <div>
                <h2 className="text-lg font-semibold text-[var(--color-text-primary)]">添加账号</h2>
                <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
                  这会先在 OmniDrive 创建登录会话，再由目标设备上的本地 SAU 拉起浏览器登录。
                </p>
              </div>
              <button
                type="button"
                onClick={() => setShowAddDialog(false)}
                className="rounded-lg p-2 text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-secondary)] transition-colors"
                aria-label="关闭添加账号弹窗"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleAddAccount} className="space-y-4 px-6 py-5">
              <div className="rounded-xl border border-blue-500/20 bg-blue-500/8 px-4 py-3 text-sm text-blue-200">
                当前已接通真实本地登录链路的平台：{REMOTE_LOGIN_PLATFORM_OPTIONS.map((item) => getPlatformLabel(item.value)).join(" / ")}
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-[var(--color-text-primary)]">执行设备</label>
                <select
                  value={effectiveAddDeviceId}
                  onChange={(event) => setAddDeviceId(event.target.value)}
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2.5 text-sm focus:outline-none focus:border-[var(--color-primary)]"
                >
                  {availableDevices.length === 0 && <option value="">暂无可用设备</option>}
                  {availableDevices.map((row) => (
                    <option key={row.device.id} value={row.device.id}>
                      {row.device.name} ({row.device.deviceCode})
                    </option>
                  ))}
                </select>
                {isDevicesLoading && (
                  <p className="text-xs text-[var(--color-text-secondary)]">设备列表加载中...</p>
                )}
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-[var(--color-text-primary)]">平台</label>
                <select
                  value={addPlatform}
                  onChange={(event) => setAddPlatform(event.target.value)}
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2.5 text-sm focus:outline-none focus:border-[var(--color-primary)]"
                >
                  {REMOTE_LOGIN_PLATFORM_OPTIONS.map((item) => (
                    <option key={item.value} value={item.value}>
                      {getPlatformLabel(item.value)}
                    </option>
                  ))}
                </select>
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-[var(--color-text-primary)]">账号名</label>
                <input
                  type="text"
                  value={addAccountName}
                  onChange={(event) => setAddAccountName(event.target.value)}
                  placeholder="例如：brand_official"
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2.5 text-sm focus:outline-none focus:border-[var(--color-primary)]"
                />
              </div>

              <div className="flex items-center justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddDialog(false)}
                  className="rounded-lg border border-[var(--color-border)] px-4 py-2 text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-secondary)] transition-colors"
                >
                  取消
                </button>
                <button
                  type="submit"
                  disabled={actionLoadingKey === `add:${effectiveAddDeviceId}:${addPlatform}:${addAccountName.trim()}` || availableDevices.length === 0}
                  className="inline-flex items-center gap-2 rounded-lg border border-[var(--color-primary)]/30 bg-[var(--color-primary)]/12 px-4 py-2 text-sm text-[var(--color-primary)] hover:bg-[var(--color-primary)]/18 transition-colors disabled:opacity-50"
                >
                  <Plus className="h-4 w-4" /> 创建登录会话
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
