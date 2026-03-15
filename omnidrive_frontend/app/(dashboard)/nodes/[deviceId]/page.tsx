"use client";

import { use } from "react";
import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import {
  Server,
  Cpu,
  HardDrive,
  Wifi,
  Users,
  Plus,
  ShieldCheck,
} from "lucide-react";
import api from "@/lib/api";
import type { Device, Account, Skill } from "@/lib/types";
import { PageHeader, StatusBadge, EmptyState } from "@/components/ui/common";

export default function NodeDetailPage({
  params,
}: {
  params: Promise<{ deviceId: string }>;
}) {
  const { deviceId } = use(params);

  const { data: device } = useQuery<Device>({
    queryKey: ["device", deviceId],
    queryFn: () => api.get(`/devices/${deviceId}`).then((r) => r.data),
  });

  const { data: accounts = [] } = useQuery<Account[]>({
    queryKey: ["accounts", deviceId],
    queryFn: () =>
      api.get(`/accounts?deviceId=${deviceId}`).then((r) => r.data),
  });

  const { data: skills = [] } = useQuery<Skill[]>({
    queryKey: ["skills"],
    queryFn: () => api.get("/skills").then((r) => r.data),
  });

  if (!device)
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="skeleton h-8 w-48" />
      </div>
    );

  return (
    <>
      <PageHeader
        title={device.name}
        subtitle={`设备编码: ${device.deviceCode}`}
      />

      {/* Device Info Grid */}
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4"
      >
        <div className="glass-card p-4">
          <div className="flex items-center gap-2 text-text-muted">
            <Wifi className="h-4 w-4" />
            <span className="text-xs font-medium">在线状态</span>
          </div>
          <div className="mt-2">
            <StatusBadge status={device.status} size="md" />
          </div>
        </div>
        <div className="glass-card p-4">
          <div className="flex items-center gap-2 text-text-muted">
            <Server className="h-4 w-4" />
            <span className="text-xs font-medium">IP 地址</span>
          </div>
          <p className="mt-2 font-mono text-sm text-text-primary">
            {device.localIp ?? "-"}
          </p>
        </div>
        <div className="glass-card p-4">
          <div className="flex items-center gap-2 text-text-muted">
            <Cpu className="h-4 w-4" />
            <span className="text-xs font-medium">推理模型</span>
          </div>
          <p className="mt-2 text-sm font-medium text-text-primary">
            {device.defaultReasoningModel ?? "默认"}
          </p>
        </div>
        <div className="glass-card p-4">
          <div className="flex items-center gap-2 text-text-muted">
            <HardDrive className="h-4 w-4" />
            <span className="text-xs font-medium">最后心跳</span>
          </div>
          <p className="mt-2 text-sm text-text-primary">
            {device.lastSeenAt
              ? new Date(device.lastSeenAt).toLocaleString("zh-CN")
              : "-"}
          </p>
        </div>
      </motion.div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Accounts */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="glass-card p-6"
        >
          <div className="mb-4 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Users className="h-4 w-4 text-accent" />
              <h2 className="text-base font-semibold text-text-primary">
                平台账号
              </h2>
            </div>
            <button className="flex items-center gap-1 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-text-secondary hover:border-accent/30 hover:text-accent transition-all">
              <Plus className="h-3 w-3" /> 添加账号
            </button>
          </div>

          {accounts.length > 0 ? (
            <div className="space-y-3">
              {accounts.map((acc) => (
                <div
                  key={acc.id}
                  className="flex items-center justify-between rounded-xl bg-surface-hover/50 px-4 py-3"
                >
                  <div>
                    <p className="text-sm font-medium text-text-primary">
                      {acc.accountName}
                    </p>
                    <p className="text-xs text-text-muted">{acc.platform}</p>
                  </div>
                  <div className="flex items-center gap-2">
                    <StatusBadge status={acc.status} />
                    <button className="rounded-lg p-1.5 text-text-muted hover:bg-accent/10 hover:text-accent transition-colors">
                      <ShieldCheck className="h-4 w-4" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState
              icon={<Users className="h-6 w-6" />}
              title="暂无同步账号"
              description="为此设备添加自媒体平台账号。"
            />
          )}
        </motion.div>

        {/* Skills */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="glass-card p-6"
        >
          <div className="mb-4 flex items-center gap-2">
            <Cpu className="h-4 w-4 text-cyan" />
            <h2 className="text-base font-semibold text-text-primary">
              已配置技能
            </h2>
          </div>

          {skills.length > 0 ? (
            <div className="space-y-3">
              {skills.map((skill) => (
                <div
                  key={skill.id}
                  className="rounded-xl bg-surface-hover/50 px-4 py-3"
                >
                  <p className="text-sm font-medium text-text-primary">
                    {skill.name}
                  </p>
                  <p className="mt-0.5 text-xs text-text-muted">
                    {skill.outputType === "image_text" ? "图+文" : "视+文"} ·{" "}
                    {skill.modelName}
                  </p>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState
              icon={<Cpu className="h-6 w-6" />}
              title="暂无技能"
              description="创建产品技能后即可在此分配。"
            />
          )}
        </motion.div>
      </div>
    </>
  );
}
