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
  offline: "bg-danger/15 text-danger",
  failed: "bg-danger/15 text-danger",
  invalid: "bg-danger/15 text-danger",
  pending: "bg-info/15 text-info",
  running: "bg-info/15 text-info",
  awaiting_scan: "bg-warning/15 text-warning",
  awaiting_verification: "bg-warning/15 text-warning",
  needs_verify: "bg-warning/15 text-warning",
  unknown: "bg-text-muted/15 text-text-muted",
};

const statusLabels: Record<string, string> = {
  online: "在线",
  offline: "离线",
  active: "有效",
  invalid: "失效",
  pending: "等待中",
  running: "执行中",
  success: "已完成",
  failed: "失败",
  needs_verify: "待验证",
  awaiting_scan: "等待扫码",
  awaiting_verification: "等待验证",
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
          status === "online" || status === "active" || status === "running"
            ? "pulse-online"
            : "",
          status === "online" || status === "active" || status === "success"
            ? "bg-success"
            : "",
          status === "offline" || status === "failed" || status === "invalid"
            ? "bg-danger"
            : "",
          status === "pending" || status === "running" ? "bg-info" : "",
          status.includes("verify") || status.includes("awaiting")
            ? "bg-warning"
            : "",
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
