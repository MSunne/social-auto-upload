"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { motion } from "framer-motion";
import {
  Layers,
  Plus,
  Trash2,
  FileImage,
  Video,
  Settings2,
  MessageSquareText,
  Search,
  Image as ImageIcon,
  FileText,
  ChevronDown,
  ChevronUp,
} from "lucide-react";
import { deleteSkill, listSkills, listSkillAssets } from "@/lib/services";
import type { Skill, SkillAsset } from "@/lib/types";
import { PageHeader, EmptyState } from "@/components/ui/common";
import { SkillEditorModal } from "@/components/ui/skill-editor-modal";
import { cn } from "@/lib/utils";

function SkillAssetPreview({ skillId }: { skillId: string }) {
  const [expanded, setExpanded] = useState(false);
  const { data: assets = [], isLoading } = useQuery<SkillAsset[]>({
    queryKey: ["skillAssets", skillId],
    queryFn: () => listSkillAssets(skillId),
  });

  const imageAssets = useMemo(
    () => assets.filter((a) => (a.mimeType || "").startsWith("image/") || a.assetType.includes("image")),
    [assets],
  );
  const textAssets = useMemo(
    () => assets.filter((a) => (a.mimeType || "").toLowerCase().startsWith("text/") || a.assetType.includes("text")),
    [assets],
  );

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-xs text-text-muted">
        <div className="h-3 w-3 animate-spin rounded-full border border-accent border-t-transparent" />
        读取素材...
      </div>
    );
  }

  if (assets.length === 0) {
    return (
      <p className="text-xs text-text-muted italic">暂无素材</p>
    );
  }

  const previewLimit = 6;
  const visibleImages = expanded ? imageAssets : imageAssets.slice(0, previewLimit);
  const hasMore = imageAssets.length > previewLimit;

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-3 text-xs text-text-secondary">
        {imageAssets.length > 0 ? (
          <span className="inline-flex items-center gap-1">
            <ImageIcon className="h-3 w-3 text-cyan" />
            {imageAssets.length} 张图片
          </span>
        ) : null}
        {textAssets.length > 0 ? (
          <span className="inline-flex items-center gap-1">
            <FileText className="h-3 w-3 text-amber-200" />
            {textAssets.length} 个文本
          </span>
        ) : null}
      </div>

      {imageAssets.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {visibleImages.map((asset) => (
            <div
              key={asset.id}
              className="h-10 w-10 overflow-hidden rounded-lg border border-border/50 bg-surface-hover"
              title={asset.fileName}
            >
              {asset.publicUrl ? (
                <img
                  src={asset.publicUrl}
                  alt={asset.fileName}
                  className="h-full w-full object-cover"
                />
              ) : (
                <div className="flex h-full w-full items-center justify-center">
                  <ImageIcon className="h-4 w-4 text-text-muted" />
                </div>
              )}
            </div>
          ))}
          {hasMore && !expanded ? (
            <button
              onClick={() => setExpanded(true)}
              className="flex h-10 w-10 items-center justify-center rounded-lg border border-dashed border-border/50 bg-surface text-text-muted transition-colors hover:border-accent/30 hover:text-accent"
            >
              <span className="text-[10px] font-semibold">+{imageAssets.length - previewLimit}</span>
            </button>
          ) : null}
        </div>
      ) : null}

      {hasMore && expanded ? (
        <button
          onClick={() => setExpanded(false)}
          className="inline-flex items-center gap-1 text-[11px] text-accent hover:underline"
        >
          <ChevronUp className="h-3 w-3" />
          收起
        </button>
      ) : null}

      {textAssets.length > 0 ? (
        <div className="space-y-1">
          {textAssets.slice(0, expanded ? textAssets.length : 3).map((asset) => (
            <div
              key={asset.id}
              className="flex items-center gap-2 rounded-lg border border-border/40 bg-surface-hover/30 px-2.5 py-1.5 text-xs text-text-secondary"
            >
              <FileText className="h-3 w-3 shrink-0 text-amber-200" />
              <span className="truncate">{asset.fileName}</span>
            </div>
          ))}
          {!expanded && textAssets.length > 3 ? (
            <button
              onClick={() => setExpanded(true)}
              className="inline-flex items-center gap-1 text-[11px] text-accent hover:underline"
            >
              <ChevronDown className="h-3 w-3" />
              查看全部 {textAssets.length} 个文本
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

export default function SkillsPage() {
  const queryClient = useQueryClient();
  const { data: skills = [] } = useQuery<Skill[]>({
    queryKey: ["skills"],
    queryFn: () => listSkills(),
  });

  const [search, setSearch] = useState("");
  const [editingSkill, setEditingSkill] = useState<Skill | null>(null);
  const [creating, setCreating] = useState(false);

  const deleteMutation = useMutation({
    mutationFn: async (skill: Skill) => {
      await deleteSkill(skill.id);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["skills"] });
    },
    onError: (error) => {
      window.alert(error instanceof Error ? error.message : "删除技能失败，请稍后重试");
    },
  });

  const openEditModal = (skill: Skill) => {
    setEditingSkill(skill);
    setCreating(false);
  };

  const openCreateModal = () => {
    setEditingSkill(null);
    setCreating(true);
  };

  const closeModal = () => {
    setEditingSkill(null);
    setCreating(false);
  };

  const filteredSkills = skills.filter((s) =>
    s.name.toLowerCase().includes(search.toLowerCase()) ||
    (s.description || "").toLowerCase().includes(search.toLowerCase())
  );

  const getOutputIcon = (outputType: string) => {
    if (outputType === "video_text" || outputType === "视文模式") {
      return <Video className="h-5 w-5 text-accent" />;
    }
    if (outputType === "文本格式") {
      return <MessageSquareText className="h-5 w-5 text-amber-200" />;
    }
    return <FileImage className="h-5 w-5 text-cyan" />;
  };

  const getOutputLabel = (outputType: string) => {
    if (outputType === "video_text" || outputType === "视文模式") return "视文模式";
    if (outputType === "文本格式") return "纯文本";
    return "图文模式";
  };

  const getOutputColor = (outputType: string) => {
    if (outputType === "video_text" || outputType === "视文模式") return "from-accent/15 border-accent/25";
    if (outputType === "文本格式") return "from-amber-300/15 border-amber-300/25";
    return "from-cyan/15 border-cyan/25";
  };

  return (
    <>
      <PageHeader
        title="产品技能库"
        subtitle="管理 OmniBull 的多模态技能模版、执行工作流与提示词约束"
        actions={
          <div className="flex items-center gap-2">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted" />
              <input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="搜索技能..."
                className="w-52 rounded-xl border border-border bg-surface py-2 pl-9 pr-3 text-sm text-text-primary placeholder-text-muted outline-none transition-all focus:border-accent/50 focus:ring-2 focus:ring-accent/20"
              />
            </div>
            <button
              onClick={openCreateModal}
              className="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2 text-sm font-semibold text-background transition-all hover:shadow-lg hover:shadow-accent/25"
            >
              <Plus className="h-4 w-4" />
              新建技能
            </button>
          </div>
        }
      />

      {filteredSkills.length > 0 ? (
        <div className="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3">
          {filteredSkills.map((skill, index) => (
            <motion.div
              initial={{ opacity: 0, y: 15 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: index * 0.04 }}
              key={skill.id}
              className="group flex flex-col overflow-hidden rounded-2xl border border-border bg-surface transition-all hover:border-accent/30 hover:shadow-xl hover:shadow-accent/8"
            >
              {/* Gradient Top Accent */}
              <div className={cn("h-1 w-full bg-gradient-to-r to-transparent", getOutputColor(skill.outputType))} />

              {/* Header */}
              <div className="flex items-start justify-between p-5 pb-0">
                <div className="flex items-start gap-3">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-white/6">
                    {getOutputIcon(skill.outputType)}
                  </div>
                  <div className="min-w-0">
                    <h3 className="text-base font-bold text-text-primary line-clamp-1">
                      {skill.name}
                    </h3>
                    <div className="mt-1 flex items-center gap-2">
                      <span className={cn(
                        "inline-flex h-1.5 w-1.5 rounded-full",
                        skill.isEnabled
                          ? "bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.5)]"
                          : "bg-text-muted",
                      )} />
                      <span className="text-[11px] text-text-muted">
                        {skill.isEnabled ? "已启用" : "已暂停"}
                      </span>
                    </div>
                  </div>
                </div>
              </div>

              {/* Body */}
              <div className="flex-1 space-y-4 p-5">
                <p className="text-sm leading-relaxed text-text-secondary line-clamp-2">
                  {skill.description || "暂无描述"}
                </p>

                <div className="flex flex-wrap gap-2">
                  <span className="inline-flex items-center gap-1 rounded-lg bg-white/6 px-2 py-1 text-[11px] font-medium text-text-secondary">
                    产出 <span className="text-text-primary">{getOutputLabel(skill.outputType)}</span>
                  </span>
                  <span className="inline-flex items-center gap-1 rounded-lg bg-white/6 px-2 py-1 text-[11px] font-medium text-text-secondary">
                    模型 <span className="text-text-primary truncate max-w-[120px]">{skill.modelName || "未配置"}</span>
                  </span>
                  <span className="inline-flex items-center gap-1 rounded-lg bg-white/6 px-2 py-1 text-[11px] font-medium text-text-secondary">
                    分镜 <span className="text-text-primary">{skill.storyboardEnabled !== false ? "启用" : "关闭"}</span>
                  </span>
                </div>

                {/* Asset Preview */}
                <div className="rounded-xl border border-border/50 bg-surface-hover/30 p-3">
                  <p className="mb-2 text-[10px] font-semibold uppercase tracking-wide text-text-muted">
                    参考素材
                  </p>
                  <SkillAssetPreview skillId={skill.id} />
                </div>
              </div>

              {/* Footer */}
              <div className="flex items-center justify-between border-t border-border/40 px-5 py-3">
                <span className="text-[11px] text-text-muted">
                  {new Date(skill.updatedAt).toLocaleDateString("zh-CN")} 更新
                </span>
                <div className="flex items-center gap-1">
                  <button
                    title="编辑技能"
                    onClick={() => openEditModal(skill)}
                    className="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted transition-all hover:bg-accent/10 hover:text-accent"
                  >
                    <Settings2 className="h-4 w-4" />
                  </button>
                  <button
                    title="删除"
                    onClick={() => {
                      if (!window.confirm(`确认删除技能「${skill.name}」吗？此操作不可恢复。`)) {
                        return;
                      }
                      deleteMutation.mutate(skill);
                    }}
                    disabled={deleteMutation.isPending}
                    className="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted transition-all hover:bg-danger/10 hover:text-danger"
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

      <SkillEditorModal
        key={editingSkill?.id || (creating ? "create" : "closed")}
        isOpen={creating || Boolean(editingSkill)}
        deviceId={editingSkill?.deviceId || ""}
        skill={editingSkill}
        onClose={closeModal}
        onSaved={() => {
          queryClient.invalidateQueries({ queryKey: ["skills"] });
        }}
      />
    </>
  );
}
