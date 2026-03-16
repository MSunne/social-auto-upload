"use client";

import { use, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { motion, AnimatePresence } from "framer-motion";
import {
  Cpu,
  Plus,
  ChevronLeft,
  ChevronRight,
  ArrowLeft,
  Image as ImageIcon,
  Edit2,
  Trash2,
  Sparkles,
  Zap,
} from "lucide-react";
import Link from "next/link";
import { getDevice, listSkills } from "@/lib/services";
import type { Device, Skill } from "@/lib/types";
import { PageHeader, EmptyState } from "@/components/ui/common";

export default function NodeDetailPage({
  params,
}: {
  params: Promise<{ deviceId: string }>;
}) {
  const { deviceId } = use(params);
  const [hoveredImg, setHoveredImg] = useState<{ src: string; x: number; y: number } | null>(null);

  const { data: device } = useQuery<Device>({
    queryKey: ["device", deviceId],
    queryFn: () => getDevice(deviceId),
  });

  const { data: skills = [] } = useQuery<Skill[]>({
    queryKey: ["skills"],
    queryFn: listSkills,
  });

  if (!device)
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="skeleton h-8 w-48" />
      </div>
    );

  return (
    <>
      {/* Top Bar */}
      <div className="mb-6">
        <Link
          href="/nodes"
          className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-accent to-cyan px-5 py-2.5 text-sm font-bold text-white shadow-lg shadow-accent/25 transition-all hover:shadow-xl hover:shadow-accent/35 hover:-translate-y-0.5 active:translate-y-0"
        >
          <ArrowLeft className="h-4 w-4" /> 返回列表
        </Link>
      </div>

      {/* Skills Table */}
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.1, duration: 0.4 }}
        className="glass-card glow-border p-0 overflow-hidden"
      >
        {/* Table Header Bar */}
        <div className="flex items-center justify-between border-b border-border/50 px-6 py-5 bg-gradient-to-r from-surface-elevated/80 to-surface/50 backdrop-blur-md">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-cyan/10 border border-cyan/20">
              <Cpu className="h-5 w-5 text-cyan" />
            </div>
            <div>
              <h2 className="text-base font-bold text-text-primary uppercase tracking-wider">
                已配置技能
              </h2>
              <p className="text-xs text-text-muted mt-0.5">
                共 <span className="text-cyan font-semibold">{skills.length}</span> 个技能已激活
              </p>
            </div>
          </div>
          <button className="flex items-center gap-2 rounded-xl bg-gradient-to-r from-accent to-[#9333ea] px-5 py-2.5 text-sm font-bold text-white transition-all hover:shadow-[0_0_25px_rgba(177,73,255,0.5)] hover:-translate-y-0.5 active:translate-y-0">
            <Plus className="h-4 w-4" /> 新增技能
          </button>
        </div>

        {skills.length > 0 ? (
          <div className="overflow-x-auto w-full">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b border-border/50 bg-surface/30">
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted">
                    <div className="flex items-center gap-1.5">
                      <Sparkles className="h-3 w-3 text-accent/60" />
                      技能名称
                    </div>
                  </th>
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted">
                    <div className="flex items-center gap-1.5">
                      <ImageIcon className="h-3 w-3 text-accent/60" />
                      产品参考图
                    </div>
                  </th>
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted">
                    技能说明
                  </th>
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted">
                    <div className="flex items-center gap-1.5">
                      <Zap className="h-3 w-3 text-accent/60" />
                      生成内容
                    </div>
                  </th>
                  <th className="px-6 py-3.5 text-xs font-bold uppercase tracking-widest text-text-muted text-center">
                    操作
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/30">
                {skills.map((skill, idx) => (
                  <motion.tr
                    key={skill.id}
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: 0.05 * idx }}
                    className="group transition-colors hover:bg-accent/[0.03]"
                  >
                    {/* Skill Name */}
                    <td className="px-6 py-4">
                      <div className="flex flex-col">
                        <span className="font-semibold text-text-primary">
                          {skill.name}
                        </span>
                        <span className="text-[11px] text-text-muted mt-0.5 font-mono opacity-60">
                          #{skill.id.split("_").pop()}
                        </span>
                      </div>
                    </td>

                    {/* Reference Images */}
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-1.5">
                        {[1, 2, 3].map((i) => (
                          <div
                            key={i}
                            className="relative h-10 w-10 overflow-hidden rounded-lg border border-border/60 bg-surface-elevated shadow-sm cursor-pointer transition-all duration-200 hover:border-cyan/50 hover:shadow-[0_0_10px_rgba(0,245,212,0.2)] hover:scale-105"
                            onMouseEnter={(e) => {
                              const rect = e.currentTarget.getBoundingClientRect();
                              setHoveredImg({
                                src: `https://placehold.co/400x400/1a1a2e/00f5d4?text=产品${i}`,
                                x: rect.left + rect.width / 2,
                                y: rect.top,
                              });
                            }}
                            onMouseLeave={() => setHoveredImg(null)}
                          >
                            {/* eslint-disable-next-line @next/next/no-img-element */}
                            <img
                              src={`https://placehold.co/100x100/1a1a2e/00f5d4?text=Ref${i}`}
                              alt={`参考图${i}`}
                              className="h-full w-full object-cover"
                            />
                          </div>
                        ))}
                        <button className="flex h-10 w-10 items-center justify-center rounded-lg border border-dashed border-border/60 bg-transparent text-text-muted transition-all hover:border-accent/50 hover:text-accent hover:bg-accent/5">
                          <Plus className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </td>

                    {/* Description */}
                    <td className="px-6 py-4">
                      <p
                        className="max-w-[220px] truncate text-sm text-text-secondary"
                        title={skill.description}
                      >
                        {skill.description ||
                          "基于图像生成视频内容的标准技能配置"}
                      </p>
                    </td>

                    {/* Output Type */}
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <span className="inline-flex items-center gap-1 rounded-full bg-accent/10 px-2.5 py-1 text-xs font-semibold text-accent border border-accent/20">
                          <Zap className="h-3 w-3" />
                          {skill.outputType === "image_text"
                            ? "图+文"
                            : "视+文"}
                        </span>
                        <span className="text-xs text-text-muted">
                          {skill.modelName}
                        </span>
                      </div>
                    </td>

                    {/* Actions — always visible */}
                    <td className="px-6 py-4">
                      <div className="flex items-center justify-center gap-2">
                        <button className="flex h-8 w-8 items-center justify-center rounded-lg border border-border/60 bg-surface text-text-muted transition-all hover:border-cyan/50 hover:text-cyan hover:bg-cyan/10 hover:shadow-[0_0_8px_rgba(0,245,212,0.15)]">
                          <Edit2 className="h-3.5 w-3.5" />
                        </button>
                        <button className="flex h-8 w-8 items-center justify-center rounded-lg border border-border/60 bg-surface text-text-muted transition-all hover:border-danger/50 hover:text-danger hover:bg-danger/10 hover:shadow-[0_0_8px_rgba(239,68,68,0.15)]">
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </td>
                  </motion.tr>
                ))}
              </tbody>
            </table>

            {/* Pagination */}
            <div className="flex items-center justify-between border-t border-border/50 px-6 py-4 bg-surface/20">
              <span className="text-sm text-text-muted">
                第{" "}
                <span className="font-semibold text-text-primary">1</span>{" "}
                页，共{" "}
                <span className="font-semibold text-text-primary">1</span> 页
              </span>
              <div className="flex gap-2">
                <button
                  className="flex h-8 w-8 items-center justify-center rounded-lg border border-border bg-surface text-text-muted transition-colors hover:border-accent hover:text-accent disabled:opacity-40"
                  disabled
                >
                  <ChevronLeft className="h-4 w-4" />
                </button>
                <button
                  className="flex h-8 w-8 items-center justify-center rounded-lg border border-border bg-surface text-text-muted transition-colors hover:border-accent hover:text-accent disabled:opacity-40"
                  disabled
                >
                  <ChevronRight className="h-4 w-4" />
                </button>
              </div>
            </div>
          </div>
        ) : (
          <div className="p-8">
            <EmptyState
              icon={<Cpu className="h-6 w-6" />}
              title="暂无技能"
              description="创建产品技能后即可在此分配。"
            />
          </div>
        )}
      </motion.div>

      {/* Floating Image Preview Tooltip */}
      <AnimatePresence>
        {hoveredImg && (
          <motion.div
            initial={{ opacity: 0, scale: 0.85, y: 8 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.85, y: 8 }}
            transition={{ duration: 0.15 }}
            className="pointer-events-none fixed z-[9999] rounded-xl border border-cyan/30 bg-surface-elevated shadow-2xl shadow-cyan/10 overflow-hidden"
            style={{
              left: hoveredImg.x - 100,
              top: hoveredImg.y - 220,
            }}
          >
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={hoveredImg.src}
              alt="preview"
              className="h-[200px] w-[200px] object-cover"
            />
            <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 to-transparent px-3 py-2">
              <p className="text-[10px] text-white/80 font-medium">产品参考图预览</p>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
