"use client";

import { use, useMemo, useState } from "react";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  Clock3,
  Cpu,
  FileText,
  Image as ImageIcon,
  Pencil,
  Plus,
  Trash2,
} from "lucide-react";
import Link from "next/link";
import { PageHeader, EmptyState, StatusBadge } from "@/components/ui/common";
import { SkillEditorModal } from "@/components/ui/skill-editor-modal";
import { deleteSkill, getDevice, listSkillAssets, listSkills } from "@/lib/services";
import type { Device, Skill, SkillAsset } from "@/lib/types";
import {
  formatDateTime,
  formatSkillSchedule,
  normalizeSkillOutputLabel,
} from "@/lib/workflow";

function isImageAsset(asset: SkillAsset) {
  return (asset.mimeType || "").startsWith("image/") || asset.assetType.includes("image");
}

function isTextAsset(asset: SkillAsset) {
  const mimeType = (asset.mimeType || "").toLowerCase();
  return mimeType.startsWith("text/") || asset.assetType.includes("text");
}

export default function NodeDetailPage({
  params,
}: {
  params: Promise<{ deviceId: string }>;
}) {
  const { deviceId } = use(params);
  const queryClient = useQueryClient();
  const [editingSkill, setEditingSkill] = useState<Skill | null>(null);
  const [creating, setCreating] = useState(false);

  const { data: device, isLoading: deviceLoading } = useQuery<Device>({
    queryKey: ["device", deviceId],
    queryFn: () => getDevice(deviceId),
  });

  const { data: skills = [], isLoading: skillsLoading } = useQuery<Skill[]>({
    queryKey: ["skills", deviceId],
    queryFn: () => listSkills(deviceId),
  });

  const assetQueries = useQueries({
    queries: skills.map((skill) => ({
      queryKey: ["skillAssets", skill.id],
      queryFn: () => listSkillAssets(skill.id),
      enabled: skills.length > 0,
    })),
  });

  const assetsBySkill = useMemo(() => {
    const results: Record<string, SkillAsset[]> = {};
    skills.forEach((skill, index) => {
      results[skill.id] = (assetQueries[index]?.data || []) as SkillAsset[];
    });
    return results;
  }, [assetQueries, skills]);

  const deleteMutation = useMutation({
    mutationFn: async (skill: Skill) => {
      await deleteSkill(skill.id);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["skills", deviceId] });
    },
    onError: (error) => {
      window.alert(error instanceof Error ? error.message : "删除技能失败，请稍后重试");
    },
  });

  const openCreateModal = () => {
    setEditingSkill(null);
    setCreating(true);
  };

  const openEditModal = (skill: Skill) => {
    setEditingSkill(skill);
    setCreating(false);
  };

  const closeModal = () => {
    setEditingSkill(null);
    setCreating(false);
  };

  if (deviceLoading || skillsLoading) {
    return (
      <div className="flex h-72 items-center justify-center">
        <div className="flex items-center gap-3 text-text-secondary">
          <div className="h-5 w-5 animate-spin rounded-full border-2 border-accent border-t-transparent" />
          正在读取节点与技能信息...
        </div>
      </div>
    );
  }

  if (!device) {
    return (
      <EmptyState
        icon={<Cpu className="h-6 w-6" />}
        title="节点不存在"
        description="当前设备可能尚未绑定到你的 OmniDrive 账号。"
      />
    );
  }

  return (
    <>
      <div className="mb-4">
        <Link
          href="/nodes"
          className="inline-flex items-center gap-2 rounded-xl border border-border bg-surface px-4 py-2 text-sm font-medium text-text-primary transition-colors hover:border-accent hover:text-accent"
        >
          <ArrowLeft className="h-4 w-4" />
          返回节点列表
        </Link>
      </div>

      <PageHeader
        title={`${device.name} · 技能中心`}
        subtitle={`设备编码 ${device.deviceCode}，在这里维护当前节点的生成技能、定时策略与参考素材。`}
        actions={
          <button
            type="button"
            onClick={openCreateModal}
            className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2 text-sm font-semibold text-background"
          >
            <Plus className="h-4 w-4" />
            新增技能
          </button>
        }
      />

      {skills.length === 0 ? (
        <EmptyState
          icon={<Cpu className="h-6 w-6" />}
          title="当前节点还没有技能"
          description="先为这个节点创建技能，再上传参考图、卖点文本和定时执行策略。"
          action={
            <button
              type="button"
              onClick={openCreateModal}
              className="rounded-xl bg-gradient-to-r from-accent to-cyan px-4 py-2 text-sm font-semibold text-background"
            >
              创建第一个技能
            </button>
          }
        />
      ) : (
        <div className="overflow-hidden rounded-3xl border border-border bg-surface">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border bg-surface-hover/40 text-xs uppercase tracking-wider text-text-muted">
                <tr>
                  <th className="px-5 py-4">技能</th>
                  <th className="px-5 py-4">输出与模型</th>
                  <th className="px-5 py-4">参考素材</th>
                  <th className="px-5 py-4">执行策略</th>
                  <th className="px-5 py-4">状态</th>
                  <th className="px-5 py-4 text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {skills.map((skill) => {
                  const assets = assetsBySkill[skill.id] || [];
                  const imageAssets = assets.filter(isImageAsset);
                  const textAssets = assets.filter(isTextAsset);
                  return (
                    <tr key={skill.id} className="align-top transition-colors hover:bg-surface-hover/20">
                      <td className="px-5 py-4">
                        <div>
                          <div className="font-semibold text-text-primary">{skill.name}</div>
                          <div className="mt-1 text-xs text-text-secondary">{skill.description}</div>
                          <div className="mt-2 text-[11px] font-mono text-text-muted">{skill.id}</div>
                        </div>
                      </td>
                      <td className="px-5 py-4">
                        <div className="space-y-2">
                          <div className="inline-flex rounded-full bg-accent/10 px-2.5 py-1 text-xs font-medium text-accent">
                            {normalizeSkillOutputLabel(skill.outputType)}
                          </div>
                          <div className="text-sm text-text-primary">{skill.modelName}</div>
                          {skill.promptTemplate ? (
                            <p className="max-w-xs text-xs leading-5 text-text-secondary">
                              {skill.promptTemplate.slice(0, 96)}
                              {skill.promptTemplate.length > 96 ? "..." : ""}
                            </p>
                          ) : (
                            <p className="text-xs text-text-muted">未单独配置提示词</p>
                          )}
                        </div>
                      </td>
                      <td className="px-5 py-4">
                        <div className="space-y-3">
                          <div className="flex items-center gap-2">
                            <div className="flex -space-x-2">
                              {imageAssets.slice(0, 4).map((asset) => (
                                <div
                                  key={asset.id}
                                  className="h-10 w-10 overflow-hidden rounded-xl border border-border bg-background"
                                >
                                  {asset.publicUrl ? (
                                    // eslint-disable-next-line @next/next/no-img-element
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
                              {!imageAssets.length ? (
                                <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-dashed border-border text-text-muted">
                                  <ImageIcon className="h-4 w-4" />
                                </div>
                              ) : null}
                            </div>
                            <span className="text-xs text-text-secondary">{imageAssets.length} 张图片</span>
                          </div>
                          <div className="flex items-center gap-2 text-xs text-text-secondary">
                            <FileText className="h-4 w-4 text-amber-400" />
                            {textAssets.length} 个文本参考
                          </div>
                        </div>
                      </td>
                      <td className="px-5 py-4">
                        <div className="space-y-2">
                          <div className="flex items-center gap-2 text-sm text-text-primary">
                            <Clock3 className="h-4 w-4 text-accent" />
                            {formatSkillSchedule(skill)}
                          </div>
                          <p className="text-xs text-text-secondary">
                            {skill.nextRunAt ? `下次发布时间：${formatDateTime(skill.nextRunAt)}` : "尚未进入定时发布"}
                          </p>
                          <p className="text-xs text-text-secondary">
                            {skill.storyboardEnabled ? "AI 分镜优化已启用" : "AI 分镜优化已关闭，执行时直接使用客户提交内容"}
                          </p>
                        </div>
                      </td>
                      <td className="px-5 py-4">
                        <div className="space-y-2">
                          <StatusBadge status={skill.isEnabled ? "active" : "inactive"} />
                          <p className="text-xs text-text-secondary">
                            {skill.repeatDaily ? "每天按时执行" : "按下一个时间点执行一次或手动执行"}
                          </p>
                        </div>
                      </td>
                      <td className="px-5 py-4">
                        <div className="flex justify-end gap-2">
                          <button
                            type="button"
                            onClick={() => openEditModal(skill)}
                            className="inline-flex items-center gap-1.5 rounded-xl border border-border px-3 py-2 text-sm text-text-primary transition-colors hover:border-accent hover:text-accent"
                          >
                            <Pencil className="h-4 w-4" />
                            编辑
                          </button>
                          <button
                            type="button"
                            onClick={() => {
                              if (!window.confirm(`确认删除技能「${skill.name}」吗？`)) {
                                return;
                              }
                              deleteMutation.mutate(skill);
                            }}
                            className="inline-flex items-center gap-1.5 rounded-xl border border-border px-3 py-2 text-sm text-text-primary transition-colors hover:border-danger hover:text-danger"
                          >
                            <Trash2 className="h-4 w-4" />
                            删除
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <SkillEditorModal
        isOpen={creating || Boolean(editingSkill)}
        deviceId={deviceId}
        skill={editingSkill}
        onClose={closeModal}
        onSaved={() => {
          queryClient.invalidateQueries({ queryKey: ["skills", deviceId] });
        }}
      />
    </>
  );
}
