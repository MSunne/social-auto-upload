"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/lib/store";
import { motion } from "framer-motion";
import {
  LayoutDashboard,
  Image,
  Video,
  MessageSquare,
  Server,
  ListTodo,
  Wallet,
  CreditCard,
  LogOut,
  ChevronRight,
  Zap,
  Layers,
} from "lucide-react";

const navGroups = [
  {
    label: "主控台",
    items: [
      { href: "/dashboard", label: "控制面板", icon: LayoutDashboard },
    ],
  },
  {
    label: "AI 创作",
    items: [
      { href: "/creation/image", label: "图片制作", icon: Image },
      { href: "/creation/video", label: "视频制作", icon: Video },
      { href: "/chat", label: "聊天助手", icon: MessageSquare },
    ],
  },
  {
    label: "管理中心",
    items: [
      { href: "/nodes", label: "OpenClaw 配置", icon: Server },
      { href: "/skills", label: "产品技能库", icon: Layers },
      { href: "/tasks", label: "OpenClaw 任务", icon: ListTodo },
    ],
  },
  {
    label: "财务",
    items: [
      { href: "/finance", label: "财务管理", icon: Wallet },
      { href: "/top-up", label: "充值中心", icon: CreditCard },
    ],
  },
];

export function Sidebar() {
  const pathname = usePathname();
  const { user, logout } = useAuthStore();

  return (
    <aside className="fixed left-0 top-0 z-40 flex h-screen w-[260px] flex-col border-r border-accent/10 bg-surface/60 backdrop-blur-2xl" style={{ boxShadow: 'inset -1px 0 0 rgba(177,73,255,0.08), 4px 0 40px rgba(177,73,255,0.04)' }}>
      {/* Brand */}
      <div className="flex items-center gap-3 px-5 py-6">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-accent via-pink to-cyan shadow-lg shadow-accent/30">
          <Zap className="h-5 w-5 text-background" />
        </div>
        <div>
          <h1 className="text-base font-bold tracking-tight text-text-primary">
            OmniDrive
          </h1>
          <p className="text-xs text-text-muted">Cloud Console</p>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto px-3 pb-4">
        {navGroups.map((group) => (
          <div key={group.label} className="mb-4">
            <p className="mb-2 px-3 text-[11px] font-semibold uppercase tracking-[0.2em] text-text-muted">
              {group.label}
            </p>
            <div className="space-y-0.5">
              {group.items.map((item) => {
                const isActive =
                  pathname === item.href ||
                  (item.href !== "/dashboard" &&
                    pathname.startsWith(item.href));
                return (
                  <Link key={item.href} href={item.href}>
                    <motion.div
                      whileHover={{ x: 2 }}
                      className={cn(
                        "group relative flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all duration-200",
                        isActive
                          ? "bg-accent/15 text-accent-strong shadow-[0_0_20px_rgba(177,73,255,0.12)]"
                          : "text-text-secondary hover:bg-surface-hover hover:text-text-primary hover:shadow-[0_0_12px_rgba(177,73,255,0.06)]",
                      )}
                    >
                      {isActive && (
                        <motion.div
                          layoutId="navIndicator"
                          className="absolute left-0 top-1/2 h-5 w-[3px] -translate-y-1/2 rounded-r-full bg-gradient-to-b from-accent to-cyan shadow-[0_0_12px_var(--color-accent-glow)]"
                          transition={{
                            type: "spring",
                            stiffness: 350,
                            damping: 30,
                          }}
                        />
                      )}
                      <item.icon className="h-[18px] w-[18px] shrink-0" />
                      <span className="truncate">{item.label}</span>
                      {isActive && (
                        <ChevronRight className="ml-auto h-3.5 w-3.5 opacity-50" />
                      )}
                    </motion.div>
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      {/* User panel */}
      <div className="border-t border-border p-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5 min-w-0">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-accent/40 to-cyan/40 text-xs font-bold text-text-primary shadow-sm shadow-accent/20">
              {user?.name?.charAt(0)?.toUpperCase() ?? "U"}
            </div>
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-text-primary">
                {user?.name ?? "未登录"}
              </p>
              <p className="truncate text-xs text-text-muted">
                {user?.phone ?? user?.email ?? ""}
              </p>
            </div>
          </div>
          <button
            onClick={logout}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-danger/15 hover:text-danger"
            title="退出登录"
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      </div>
    </aside>
  );
}
