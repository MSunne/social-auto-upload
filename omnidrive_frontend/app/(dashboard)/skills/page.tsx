"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import {
  Layers,
  Plus,
  Pencil,
  Trash2,
  FileImage,
  Video,
  Settings2,
  Eye,
} from "lucide-react";
import { listSkills } from "@/lib/services";
import type { Skill } from "@/lib/types";
import { PageHeader, EmptyState, StatusBadge } from "@/components/ui/common";

export default function SkillsPage() {
  const { data: skills = [] } = useQuery<Skill[]>({
    queryKey: ["skills"],
    queryFn: () => listSkills(),
  });

  const [search, setSearch] = useState("");

  const filteredSkills = skills.filter((s) =>
    s.name.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <>
      <PageHeader
        title="产品技能库"
        subtitle="管理 OmniBull 的多模态技能模版、执行工作流与提示词约束"
        actions={
          <div className="flex items-center gap-2">
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="搜索技能名称..."
              className="w-48 rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text-primary placeholder-text-muted outline-none focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
            />
            <button className="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2 text-sm font-semibold text-background transition-all hover:shadow-lg hover:shadow-accent/25">
              <Plus className="h-4 w-4" />
              新建技能
            </button>
          </div>
        }
      />

      {filteredSkills.length > 0 ? (
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3">
          {filteredSkills.map((skill, index) => (
            <motion.div
              initial={{ opacity: 0, y: 15 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: index * 0.05 }}
              key={skill.id}
              className="glass-card flex flex-col overflow-hidden transition-all hover:border-accent/30 hover:shadow-lg hover:shadow-accent/10"
            >
              {/* Header */}
              <div className="flex items-start justify-between border-b border-border/50 bg-surface-hover/30 p-5">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-accent/10">
                    {skill.outputType === "video_text" ? (
                      <Video className="h-5 w-5 text-accent" />
                    ) : (
                      <FileImage className="h-5 w-5 text-cyan" />
                    )}
                  </div>
                  <div>
                    <h3 className="text-base font-bold text-text-primary line-clamp-1">
                      {skill.name}
                    </h3>
                    <div className="mt-0.5 flex items-center gap-2">
                      <span className="text-xs font-mono text-text-muted">
                        ID: {skill.id}
                      </span>
                      <span
                        className={`inline-block h-1.5 w-1.5 rounded-full ${
                          skill.isEnabled ? "bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.5)]" : "bg-text-muted"
                        }`}
                      />
                    </div>
                  </div>
                </div>
              </div>

              {/* Body */}
              <div className="flex-1 p-5">
                <p className="mb-4 text-sm leading-relaxed text-text-secondary line-clamp-3 h-[60px]">
                  {skill.description || "暂无描述"}
                </p>

                <div className="grid grid-cols-2 gap-3">
                  <div className="rounded-lg bg-surface p-3 border border-border/50">
                    <p className="mb-1 text-[10px] font-semibold uppercase text-text-muted tracking-wide">
                      产出类型
                    </p>
                    <p className="text-sm font-medium text-text-primary">
                      {skill.outputType === "video_text"
                        ? "视频 + 图文"
                        : "图片 + 图文"}
                    </p>
                  </div>
                  <div className="rounded-lg bg-surface p-3 border border-border/50">
                    <p className="mb-1 text-[10px] font-semibold uppercase text-text-muted tracking-wide">
                      推理底座
                    </p>
                    <p className="text-sm font-medium text-text-primary truncate">
                      {skill.modelName || "未配置"}
                    </p>
                  </div>
                </div>
              </div>

              {/* Footer Actions */}
              <div className="flex items-center justify-between border-t border-border/50 bg-surface-hover/30 px-5 py-3">
                <span className="text-xs text-text-muted">
                  {new Date(skill.updatedAt).toLocaleDateString("zh-CN")} 更新
                </span>
                <div className="flex items-center gap-2">
                  <button
                    title="预览提示词"
                    className="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface hover:text-cyan"
                  >
                    <Eye className="h-4 w-4" />
                  </button>
                  <button
                    title="编辑技能"
                    className="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface hover:text-accent"
                  >
                    <Settings2 className="h-4 w-4" />
                  </button>
                  <button
                    title="删除"
                    className="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-danger/10 hover:text-danger"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>
            </motion.div>
          ))}
        </div>
      ) : (
        <EmptyState
          icon={<Layers className="h-6 w-6" />}
          title="未找到技能"
          description="暂时没有配置任何能力模版，点击上方按钮新建。"
        />
      )}
    </>
  );
}
