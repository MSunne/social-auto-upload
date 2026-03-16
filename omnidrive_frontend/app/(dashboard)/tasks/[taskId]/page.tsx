"use client";

import { useQuery } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { motion } from "framer-motion";
import {
  ArrowLeft,
  Calendar,
  MonitorSmartphone,
  CheckCircle2,
  AlertTriangle,
  FileText,
  Clock,
  RefreshCcw,
  Check,
  X,
  PlaySquare,
  Image as ImageIcon,
} from "lucide-react";
import { listTasks, listDevices, listAccounts } from "@/lib/services";
import type { Task, Device, Account } from "@/lib/types";
import { PageHeader, StatusBadge } from "@/components/ui/common";

export default function TaskDetailPage() {
  const params = useParams();
  const router = useRouter();
  const taskId = params.taskId as string;

  // Fetch data
  const { data: tasks = [] } = useQuery<Task[]>({
    queryKey: ["tasks"],
    queryFn: () => listTasks(),
  });
  const { data: devices = [] } = useQuery<Device[]>({
    queryKey: ["devices"],
    queryFn: listDevices,
  });
  const { data: accounts = [] } = useQuery<Account[]>({
    queryKey: ["accounts"],
    queryFn: () => listAccounts(),
  });

  const task = tasks.find((t) => t.id === taskId);
  const device = devices.find((d) => d.id === task?.deviceId);
  const account = accounts.find((a) => a.id === task?.accountId);

  if (!task) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center text-text-muted">
        <div className="mb-4 h-8 w-8 animate-spin rounded-full border-2 border-accent border-t-transparent" />
        加载中或任务不存在...
      </div>
    );
  }

  const isNeedsVerify = task.status === "needs_verify";
  const vp = task.verificationPayload as Record<string, string> | null | undefined;

  return (
    <>
      <button
        onClick={() => router.back()}
        className="mb-4 flex items-center gap-2 text-sm font-medium text-text-muted transition-colors hover:text-text-primary"
      >
        <ArrowLeft className="h-4 w-4" />
        返回任务列表
      </button>

      <PageHeader
        title="任务详情"
        subtitle={`ID: ${task.id}`}
        actions={
          <div className="flex items-center gap-2">
            {task.status === "failed" && (
              <button className="flex items-center gap-1.5 rounded-xl border border-border bg-surface px-4 py-2 text-sm font-semibold text-text-primary transition-all hover:bg-surface-hover hover:text-accent">
                <RefreshCcw className="h-4 w-4" />
                重试任务
              </button>
            )}
            <StatusBadge status={task.status} />
          </div>
        }
      />

      {isNeedsVerify && vp && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="mb-6 overflow-hidden rounded-2xl border border-amber-500/30 bg-amber-500/10 shadow-lg shadow-amber-500/5"
        >
          <div className="flex items-center gap-3 border-b border-amber-500/20 bg-amber-500/15 p-4 text-amber-500">
            <AlertTriangle className="h-5 w-5" />
            <h3 className="font-bold">拦截人工验证：请确认内容无误后再发布</h3>
          </div>
          <div className="p-6">
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
              <div>
                <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-text-muted">
                  截屏预览 (OmniBull 自动捕获)
                </p>
                <div className="overflow-hidden rounded-xl border border-border bg-black">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    src={vp.screenshotUrl}
                    alt="验证预留图"
                    className="w-full object-contain"
                  />
                </div>
              </div>
              <div className="space-y-4">
                <div>
                  <p className="mb-1 text-xs font-semibold uppercase tracking-wider text-text-muted">
                    准备填写的标题
                  </p>
                  <p className="rounded-lg bg-surface-hover p-3 text-sm font-medium text-text-primary">
                    {vp.generatedTitle}
                  </p>
                </div>
                <div>
                  <p className="mb-1 text-xs font-semibold uppercase tracking-wider text-text-muted">
                    准备填写的正文
                  </p>
                  <p className="rounded-lg bg-surface-hover p-3 text-sm text-text-secondary whitespace-pre-wrap">
                    {vp.contentPreview}
                  </p>
                </div>
                <div className="flex gap-3 pt-2">
                  <button className="flex flex-1 items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-emerald-500 to-emerald-400 py-3 text-sm font-bold text-white shadow-lg shadow-emerald-500/20 transition-all hover:shadow-emerald-500/40">
                    <Check className="h-4 w-4" />
                    确认并继续发布
                  </button>
                  <button className="flex flex-1 items-center justify-center gap-2 rounded-xl border border-danger/50 bg-danger/10 py-3 text-sm font-bold text-danger transition-all hover:bg-danger/20">
                    <X className="h-4 w-4" />
                    放弃任务
                  </button>
                </div>
              </div>
            </div>
          </div>
        </motion.div>
      )}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* ── Left: Task Meta ── */}
        <div className="lg:col-span-1 space-y-6">
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1 }}
            className="glass-card p-5"
          >
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-text-muted">
              基本信息
            </h3>
            <div className="space-y-4">
              <div>
                <p className="mb-1 text-[11px] text-text-muted">任务标题</p>
                <p className="text-sm font-medium text-text-primary">
                  {task.title || "未命名任务"}
                </p>
              </div>
              <div>
                <p className="mb-1 text-[11px] text-text-muted">当前进度反馈</p>
                <div className="flex items-start gap-2 rounded-lg bg-surface-hover p-3">
                  {task.status === "success" ? (
                    <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-400" />
                  ) : task.status === "failed" ? (
                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-rose-400" />
                  ) : task.status === "running" ? (
                    <div className="mt-1 h-3 w-3 shrink-0 animate-spin rounded-full border-2 border-accent border-t-transparent" />
                  ) : (
                    <Clock className="mt-0.5 h-4 w-4 shrink-0 text-amber-400" />
                  )}
                  <p className="text-sm text-text-secondary">
                    {task.message || "暂无日志"}
                  </p>
                </div>
              </div>
            </div>
          </motion.div>

          {/* Device & Platform info */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.15 }}
            className="glass-card p-5"
          >
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-text-muted">
              执行节点与分发链路
            </h3>
            <div className="space-y-4">
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-accent/10">
                  <MonitorSmartphone className="h-4 w-4 text-accent" />
                </div>
                <div>
                  <p className="text-[11px] text-text-muted">运行设备</p>
                  <p className="text-sm font-medium text-text-primary">
                    {device?.name || "未知设备"}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-cyan/10">
                  <PlaySquare className="h-4 w-4 text-cyan" />
                </div>
                <div>
                  <p className="text-[11px] text-text-muted">分发平台账号</p>
                  <p className="text-sm font-medium text-text-primary">
                    {task.platform} · {task.accountName}
                  </p>
                  <p className="text-[10px] text-text-muted">
                    ID: {account?.id || task.accountId}
                  </p>
                </div>
              </div>
            </div>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2 }}
            className="glass-card p-5"
          >
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-text-muted">
              时间线
            </h3>
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Calendar className="h-4 w-4 text-text-muted" />
                <span className="text-sm text-text-secondary">创建时间：</span>
                <span className="text-sm font-medium text-text-primary">
                  {new Date(task.createdAt).toLocaleString("zh-CN")}
                </span>
              </div>
              {task.runAt && (
                <div className="flex items-center gap-2">
                  <Clock className="h-4 w-4 text-text-muted" />
                  <span className="text-sm text-text-secondary">执行时间：</span>
                  <span className="text-sm font-medium text-text-primary">
                    {new Date(task.runAt).toLocaleString("zh-CN")}
                  </span>
                </div>
              )}
              {task.finishedAt && (
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-4 w-4 text-emerald-400" />
                  <span className="text-sm text-text-secondary">完成时间：</span>
                  <span className="text-sm font-medium text-text-primary">
                    {new Date(task.finishedAt).toLocaleString("zh-CN")}
                  </span>
                </div>
              )}
            </div>
          </motion.div>
        </div>

        {/* ── Right: Payload Data ── */}
        <div className="lg:col-span-2 space-y-6">
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1 }}
            className="glass-card overflow-hidden"
          >
            <div className="border-b border-border/50 bg-surface-hover/30 p-5">
              <h3 className="flex items-center gap-2 text-sm font-semibold text-text-primary">
                <FileText className="h-4 w-4 text-accent" />
                发布内容载荷 (Payload)
              </h3>
            </div>
            <div className="p-5 space-y-6">
              {/* Text content */}
              <div>
                <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-text-muted">
                  正文案
                </p>
                {task.contentText ? (
                  <div className="rounded-xl border border-border bg-surface px-4 py-3 text-sm leading-relaxed text-text-secondary whitespace-pre-wrap">
                    {task.contentText}
                  </div>
                ) : (
                  <p className="text-sm text-text-muted italic">暂无文本内容</p>
                )}
              </div>

              {/* Media content */}
              <div>
                <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-text-muted">
                  媒体资产
                </p>
                {task.mediaPayload && (task.mediaPayload as Record<string, unknown>).images ? (
                  <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
                    {((task.mediaPayload as Record<string, unknown>).images as string[]).map((img: string, i: number) => (
                      <div
                        key={i}
                        className="group relative aspect-square overflow-hidden rounded-lg border border-border bg-surface-hover"
                      >
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img
                          src={img}
                          alt={`Media ${i}`}
                          className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-110"
                        />
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="flex items-center gap-2 rounded-xl border border-dashed border-border py-8 text-center text-text-muted justify-center">
                    <ImageIcon className="h-5 w-5" />
                    <span className="text-sm">暂无媒体内容或仅包含文本</span>
                  </div>
                )}
              </div>
            </div>
          </motion.div>
        </div>
      </div>
    </>
  );
}
