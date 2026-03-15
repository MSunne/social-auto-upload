"use client";

import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import { ListTodo, ArrowUpRight } from "lucide-react";
import Link from "next/link";
import api from "@/lib/api";
import type { Task } from "@/lib/types";
import { PageHeader, StatusBadge, EmptyState } from "@/components/ui/common";
import { useState } from "react";
import { cn } from "@/lib/utils";

const statusTabs = [
  { key: "all", label: "全部任务" },
  { key: "video", label: "视频任务" },
  { key: "image", label: "图片任务" },
  { key: "chat", label: "聊天任务" },
];

export default function TasksPage() {
  const [activeTab, setActiveTab] = useState("all");
  const [statusFilter, setStatusFilter] = useState<string | null>(null);

  const { data: tasks = [] } = useQuery<Task[]>({
    queryKey: ["tasks"],
    queryFn: () => api.get("/tasks").then((r) => r.data),
  });

  const filteredTasks = tasks.filter((t) => {
    if (statusFilter && t.status !== statusFilter) return false;
    return true;
  });

  return (
    <>
      <PageHeader
        title="OpenClaw 任务"
        subtitle="管理和监控所有 AI 生成和分发任务的状态"
        actions={
          <button className="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2 text-sm font-semibold text-background transition-all hover:shadow-lg hover:shadow-accent/25">
            新建任务
          </button>
        }
      />

      {/* Tabs */}
      <div className="mb-6 flex items-center gap-1 rounded-xl bg-surface p-1">
        {statusTabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={cn(
              "rounded-lg px-4 py-2 text-sm font-medium transition-all",
              activeTab === tab.key
                ? "bg-accent/15 text-accent"
                : "text-text-muted hover:text-text-primary",
            )}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Status Filters */}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        {["pending", "running", "success", "failed", "needs_verify"].map(
          (s) => (
            <button
              key={s}
              onClick={() => setStatusFilter(statusFilter === s ? null : s)}
              className={cn(
                "rounded-full px-3 py-1 text-xs font-medium transition-all",
                statusFilter === s
                  ? "bg-accent/20 text-accent ring-1 ring-accent/40"
                  : "bg-surface text-text-muted hover:text-text-primary",
              )}
            >
              <StatusBadge status={s} size="sm" />
            </button>
          ),
        )}
      </div>

      {/* Task List */}
      {filteredTasks.length > 0 ? (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="glass-card overflow-hidden"
        >
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs font-semibold uppercase tracking-wider text-text-muted">
                  <th className="px-5 py-4">标题</th>
                  <th className="px-5 py-4">平台</th>
                  <th className="px-5 py-4">账号</th>
                  <th className="px-5 py-4">状态</th>
                  <th className="px-5 py-4">时间</th>
                  <th className="px-5 py-4">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filteredTasks.map((task) => (
                  <tr
                    key={task.id}
                    className="transition-colors hover:bg-surface-hover/50"
                  >
                    <td className="px-5 py-4">
                      <p className="max-w-[200px] truncate font-medium text-text-primary">
                        {task.title}
                      </p>
                    </td>
                    <td className="px-5 py-4 text-text-secondary">
                      {task.platform}
                    </td>
                    <td className="px-5 py-4 text-text-secondary">
                      {task.accountName}
                    </td>
                    <td className="px-5 py-4">
                      <StatusBadge status={task.status} />
                    </td>
                    <td className="px-5 py-4 text-text-muted">
                      {task.updatedAt
                        ? new Date(task.updatedAt).toLocaleString("zh-CN")
                        : "-"}
                    </td>
                    <td className="px-5 py-4">
                      <Link
                        href={`/tasks/${task.id}`}
                        className="flex items-center gap-1 text-xs font-medium text-accent hover:text-accent-strong transition-colors"
                      >
                        详情 <ArrowUpRight className="h-3 w-3" />
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </motion.div>
      ) : (
        <EmptyState
          icon={<ListTodo className="h-6 w-6" />}
          title="暂无任务"
          description="创建新的分发任务以开始自动化发布流程。"
        />
      )}
    </>
  );
}
