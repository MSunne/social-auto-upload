"use client";

import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import {
  Server,
  ListTodo,
  Cpu,
  Activity,
  ArrowUpRight,
  Zap,
  Clock,
} from "lucide-react";
import Link from "next/link";
import { listDevices, listTasks } from "@/lib/services";
import type { Device, Task } from "@/lib/types";
import { PageHeader, StatCard, StatusBadge, EmptyState } from "@/components/ui/common";

const fadeUp = {
  initial: { opacity: 0, y: 16 },
  animate: { opacity: 1, y: 0 },
};

export default function DashboardPage() {
  const { data: devices = [] } = useQuery<Device[]>({
    queryKey: ["devices"],
    queryFn: listDevices,
  });

  const { data: tasks = [] } = useQuery<Task[]>({
    queryKey: ["tasks"],
    queryFn: () => listTasks(),
  });

  const onlineDevices = devices.filter((d) => d.status === "online").length;
  const pendingTasks = tasks.filter(
    (t) => t.status === "pending" || t.status === "running",
  ).length;
  const verifyTasks = tasks.filter((t) => t.status === "needs_verify").length;

  return (
    <>
      <PageHeader
        title="控制面板"
        subtitle="实时查看 OmniBull 设备、AI 算力执行和分发任务状态"
      />

      {/* ── Stats Row ── */}
      <motion.div
        variants={fadeUp}
        initial="initial"
        animate="animate"
        transition={{ duration: 0.4, delay: 0.1 }}
        className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4"
      >
        <StatCard
          label="设备总数"
          value={devices.length}
          change={`${onlineDevices} 台在线`}
          changeType={onlineDevices > 0 ? "positive" : "neutral"}
          icon={<Server className="h-5 w-5" />}
        />
        <StatCard
          label="AI 算力效率"
          value="84.2%"
          change="稳定运行中"
          changeType="positive"
          icon={<Cpu className="h-5 w-5" />}
        />
        <StatCard
          label="待处理任务"
          value={pendingTasks}
          change={verifyTasks ? `${verifyTasks} 条待验证` : "全部正常"}
          changeType={verifyTasks > 0 ? "negative" : "positive"}
          icon={<ListTodo className="h-5 w-5" />}
        />
        <StatCard
          label="今日完成"
          value={tasks.filter((t) => t.status === "success").length}
          change="本日统计"
          changeType="neutral"
          icon={<Activity className="h-5 w-5" />}
        />
      </motion.div>

      {/* ── Content Grid ── */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Recent Devices */}
        <motion.div
          variants={fadeUp}
          initial="initial"
          animate="animate"
          transition={{ duration: 0.4, delay: 0.2 }}
          className="glass-card p-6"
        >
          <div className="mb-4 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Zap className="h-4 w-4 text-accent" />
              <h2 className="text-base font-semibold text-text-primary">
                OmniBull 设备状态
              </h2>
            </div>
            <Link
              href="/nodes"
              className="flex items-center gap-1 text-xs font-medium text-accent hover:text-accent-strong transition-colors"
            >
              查看全部 <ArrowUpRight className="h-3 w-3" />
            </Link>
          </div>

          {devices.length > 0 ? (
            <div className="space-y-3">
              {devices.slice(0, 5).map((device) => (
                <div
                  key={device.id}
                  className="flex items-center justify-between rounded-xl bg-surface-hover/50 px-4 py-3 transition-colors hover:bg-surface-hover"
                >
                  <div className="flex items-center gap-3">
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-accent/10">
                      <Server className="h-4 w-4 text-accent" />
                    </div>
                    <div>
                      <p className="text-sm font-medium text-text-primary">
                        {device.name}
                      </p>
                      <p className="text-xs text-text-muted">
                        {device.deviceCode}
                      </p>
                    </div>
                  </div>
                  <StatusBadge status={device.status} />
                </div>
              ))}
            </div>
          ) : (
            <EmptyState
              icon={<Server className="h-6 w-6" />}
              title="暂无设备"
              description="请先在本地启动 OmniBull 并发送心跳注册。"
            />
          )}
        </motion.div>

        {/* Recent Tasks */}
        <motion.div
          variants={fadeUp}
          initial="initial"
          animate="animate"
          transition={{ duration: 0.4, delay: 0.3 }}
          className="glass-card p-6"
        >
          <div className="mb-4 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Clock className="h-4 w-4 text-cyan" />
              <h2 className="text-base font-semibold text-text-primary">
                最近分发任务
              </h2>
            </div>
            <Link
              href="/tasks"
              className="flex items-center gap-1 text-xs font-medium text-accent hover:text-accent-strong transition-colors"
            >
              查看全部 <ArrowUpRight className="h-3 w-3" />
            </Link>
          </div>

          {tasks.length > 0 ? (
            <div className="space-y-3">
              {tasks.slice(0, 5).map((task) => (
                <div
                  key={task.id}
                  className="flex items-center justify-between rounded-xl bg-surface-hover/50 px-4 py-3 transition-colors hover:bg-surface-hover"
                >
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-text-primary">
                      {task.title}
                    </p>
                    <p className="text-xs text-text-muted">
                      {task.platform} · {task.accountName}
                    </p>
                  </div>
                  <StatusBadge status={task.status} />
                </div>
              ))}
            </div>
          ) : (
            <EmptyState
              icon={<ListTodo className="h-6 w-6" />}
              title="暂无任务"
              description="创建发布任务并分发到 OmniBull 设备。"
            />
          )}
        </motion.div>
      </div>

      {/* ── Quick Actions ── */}
      <motion.div
        variants={fadeUp}
        initial="initial"
        animate="animate"
        transition={{ duration: 0.4, delay: 0.4 }}
        className="mt-6 glass-card p-6"
      >
        <h2 className="mb-4 text-base font-semibold text-text-primary">
          快捷指令
        </h2>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <Link href="/creation/video">
            <div className="flex items-center gap-3 rounded-xl border border-border p-4 transition-all hover:border-accent/30 hover:bg-accent/5">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-accent/10">
                <Zap className="h-5 w-5 text-accent" />
              </div>
              <div>
                <p className="text-sm font-medium text-text-primary">
                  生成视频
                </p>
                <p className="text-xs text-text-muted">
                  基于 AI 文生的高质量合成
                </p>
              </div>
            </div>
          </Link>
          <Link href="/creation/image">
            <div className="flex items-center gap-3 rounded-xl border border-border p-4 transition-all hover:border-cyan/30 hover:bg-cyan/5">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-cyan/10">
                <Activity className="h-5 w-5 text-cyan" />
              </div>
              <div>
                <p className="text-sm font-medium text-text-primary">
                  创作艺术
                </p>
                <p className="text-xs text-text-muted">
                  高品质 AI 图片生成
                </p>
              </div>
            </div>
          </Link>
          <Link href="/chat">
            <div className="flex items-center gap-3 rounded-xl border border-border p-4 transition-all hover:border-info/30 hover:bg-info/5">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-info/10">
                <Cpu className="h-5 w-5 text-info" />
              </div>
              <div>
                <p className="text-sm font-medium text-text-primary">
                  开始对话
                </p>
                <p className="text-xs text-text-muted">
                  LLM 智能对话与头脑风暴
                </p>
              </div>
            </div>
          </Link>
        </div>
      </motion.div>
    </>
  );
}
