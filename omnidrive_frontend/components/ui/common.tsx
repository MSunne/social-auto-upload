"use client";

import { motion } from "framer-motion";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
}

export function PageHeader({ title, subtitle, actions }: PageHeaderProps) {
  return (
    <motion.div
      initial={{ opacity: 0, y: -8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3 }}
      className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between"
    >
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-text-primary lg:text-3xl">
          {title}
        </h1>
        {subtitle && (
          <p className="mt-1 text-sm text-text-secondary">{subtitle}</p>
        )}
      </div>
      {actions && <div className="flex items-center gap-3">{actions}</div>}
    </motion.div>
  );
}

// ─── Stat Card ───
interface StatCardProps {
  label: string;
  value: string | number;
  change?: string;
  changeType?: "positive" | "negative" | "neutral";
  icon?: React.ReactNode;
}

export function StatCard({
  label,
  value,
  change,
  changeType = "neutral",
  icon,
}: StatCardProps) {
  return (
    <motion.div
      whileHover={{ y: -2, scale: 1.01 }}
      transition={{ duration: 0.2 }}
      className="glass-card glow-border p-5"
    >
      <div className="flex items-start justify-between">
        <div>
          <p className="text-xs font-medium uppercase tracking-wider text-text-muted">
            {label}
          </p>
          <p className="mt-2 text-3xl font-bold tracking-tight text-text-primary">
            {value}
          </p>
          {change && (
            <p
              className={cn(
                "mt-1 text-xs font-medium",
                changeType === "positive" && "text-success",
                changeType === "negative" && "text-danger",
                changeType === "neutral" && "text-text-muted",
              )}
            >
              {change}
            </p>
          )}
        </div>
        {icon && (
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-accent/10 text-accent">
            {icon}
          </div>
        )}
      </div>
    </motion.div>
  );
}

// ─── Status Badge ───
interface StatusBadgeProps {
  status: string;
  size?: "sm" | "md";
}

const statusColors: Record<string, string> = {
  online: "bg-success/15 text-success",
  active: "bg-success/15 text-success",
  success: "bg-success/15 text-success",
  completed: "bg-success/15 text-success",
  published: "bg-success/15 text-success",
  output_ready: "bg-success/15 text-success",
  imported: "bg-success/15 text-success",
  paid: "bg-success/15 text-success",
  credited: "bg-success/15 text-success",
  approved: "bg-success/15 text-success",
  offline: "bg-danger/15 text-danger",
  failed: "bg-danger/15 text-danger",
  publish_failed: "bg-danger/15 text-danger",
  invalid: "bg-danger/15 text-danger",
  rejected: "bg-danger/15 text-danger",
  invalidated: "bg-text-muted/15 text-text-muted",
  inactive: "bg-text-muted/15 text-text-muted",
  pending: "bg-info/15 text-info",
  running: "bg-info/15 text-info",
  generating: "bg-info/15 text-info",
  storyboarding: "bg-info/15 text-info",
  publishing: "bg-info/15 text-info",
  queued_generation: "bg-info/15 text-info",
  pending_payment: "bg-info/15 text-info",
  awaiting_scan: "bg-warning/15 text-warning",
  awaiting_verification: "bg-warning/15 text-warning",
  needs_verify: "bg-warning/15 text-warning",
  scheduled: "bg-warning/15 text-warning",
  publish_queued: "bg-warning/15 text-warning",
  cancel_requested: "bg-warning/15 text-warning",
  awaiting_submission: "bg-warning/15 text-warning",
  awaiting_manual_review: "bg-warning/15 text-warning",
  pending_review: "bg-warning/15 text-warning",
  processing: "bg-warning/15 text-warning",
  pending_consume: "bg-warning/15 text-warning",
  pending_settlement: "bg-info/15 text-info",
  requested: "bg-warning/15 text-warning",
  unknown: "bg-text-muted/15 text-text-muted",
  cancelled: "bg-text-muted/15 text-text-muted",
  closed: "bg-text-muted/15 text-text-muted",
};

const statusLabels: Record<string, string> = {
  online: "在线",
  offline: "离线",
  active: "有效",
  inactive: "已停用",
  invalid: "失效",
  invalidated: "已失效",
  pending: "等待中",
  running: "执行中",
  success: "已完成",
  completed: "已完成",
  published: "已发布",
  output_ready: "制作完成",
  imported: "已回流",
  paid: "已支付",
  credited: "已入账",
  approved: "已通过",
  failed: "失败",
  publish_failed: "发布失败",
  rejected: "已驳回",
  needs_verify: "待验证",
  awaiting_scan: "等待扫码",
  awaiting_verification: "等待验证",
  generating: "生成中",
  storyboarding: "优化分镜中",
  publishing: "发布中",
  queued_generation: "待生成",
  pending_payment: "待支付",
  scheduled: "未开始",
  publish_queued: "待发布",
  cancel_requested: "取消中",
  awaiting_submission: "待提交凭证",
  awaiting_manual_review: "待提交凭证",
  pending_review: "待人工审核",
  processing: "人工审核中",
  pending_consume: "待生效",
  pending_settlement: "待结算",
  requested: "待审核",
  cancelled: "已取消",
  closed: "已关闭",
  unknown: "未知",
};

export function StatusBadge({ status, size = "sm" }: StatusBadgeProps) {
  const colorClass = statusColors[status] ?? statusColors.unknown;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full font-medium",
        colorClass,
        size === "sm" && "px-2.5 py-1 text-xs",
        size === "md" && "px-3 py-1.5 text-sm",
      )}
    >
      <span
        className={cn(
          "h-1.5 w-1.5 rounded-full",
          status === "online" ||
          status === "active" ||
          status === "running" ||
          status === "generating" ||
          status === "storyboarding" ||
          status === "publishing"
            ? "pulse-online"
            : "",
          status === "online" ||
          status === "active" ||
          status === "success" ||
          status === "completed" ||
          status === "published" ||
          status === "output_ready" ||
          status === "imported" ||
          status === "paid" ||
          status === "credited" ||
          status === "approved"
            ? "bg-success"
            : "",
          status === "offline" ||
          status === "failed" ||
          status === "publish_failed" ||
          status === "invalid" ||
          status === "rejected"
            ? "bg-danger"
            : "",
          status === "invalidated" || status === "closed" || status === "cancelled"
            ? "bg-text-muted"
            : "",
          status === "pending" ||
          status === "running" ||
          status === "generating" ||
          status === "storyboarding" ||
          status === "publishing" ||
          status === "queued_generation" ||
          status === "pending_payment"
            ? "bg-info"
            : "",
          status.includes("verify") ||
          status.includes("awaiting") ||
          status === "scheduled" ||
          status === "publish_queued" ||
          status === "cancel_requested" ||
          status === "pending_review" ||
          status === "processing" ||
          status === "pending_consume" ||
          status === "requested"
            ? "bg-warning"
            : "",
          status === "pending_settlement" ? "bg-info" : "",
        )}
      />
      {statusLabels[status] ?? status}
    </span>
  );
}

// ─── Empty State ───
export function EmptyState({
  icon,
  title,
  description,
  action,
}: {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center rounded-2xl border border-dashed border-border py-16 text-center">
      {icon && (
        <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-accent/10 text-accent">
          {icon}
        </div>
      )}
      <h3 className="text-base font-semibold text-text-primary">{title}</h3>
      {description && (
        <p className="mt-1 max-w-sm text-sm text-text-muted">{description}</p>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
