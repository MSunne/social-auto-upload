"use client";

import { type ReactNode, useEffect, useState } from "react";
import { PageHeader } from "@/components/ui/common";
import { useAIModels } from "@/lib/hooks/useAIModels";
import { useSystemConfig, useUpdateSystemConfig } from "@/lib/hooks/useSettings";
import { adminApi, adminPaths } from "@/lib/api";
import { AdminSystemConfig } from "@/lib/types";
import {
  FileImage,
  FileText,
  ImagePlus,
  Loader2,
  Save,
  Sparkles,
  Trash2,
  Upload,
  Video,
} from "lucide-react";

type StoryboardReference = NonNullable<AdminSystemConfig["storyboardReferences"]>[number];
type ReferenceField = "storyboardReferences" | "imageStoryboardReferences";
export function StoryboardsView() {
  const { data: config, isLoading, error } = useSystemConfig();
  const updateM = useUpdateSystemConfig();
  const { data: chatModels } = useAIModels({ pageSize: 200, category: "chat" });
  const [formData, setFormData] = useState<Partial<AdminSystemConfig>>({});
  const [uploadingField, setUploadingField] = useState<ReferenceField | null>(null);

  useEffect(() => {
    if (!config) {
      return;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps, react-hooks/set-state-in-effect
    setFormData(JSON.parse(JSON.stringify(config)));
  }, [config]);

  const modelOptions = (chatModels?.items || []).filter((item) => item.isEnabled);

  const appendReference = (field: ReferenceField, reference: StoryboardReference) => {
    setFormData((current) => ({
      ...current,
      [field]: [...((current[field] as StoryboardReference[] | undefined) || []), reference],
    }));
  };

  const removeReference = (field: ReferenceField, storageKey?: string, fileName?: string) => {
    setFormData((current) => ({
      ...current,
      [field]: (((current[field] as StoryboardReference[] | undefined) || []).filter((item) => {
        if (storageKey && item.storageKey) {
          return item.storageKey !== storageKey;
        }
        return item.fileName !== fileName;
      })),
    }));
  };

  const uploadStoryboardFiles = async (field: ReferenceField, files: FileList | null) => {
    const list = Array.from(files || []);
    if (list.length === 0) {
      return;
    }
    setUploadingField(field);
    try {
      for (const file of list) {
        const body = new FormData();
        body.append("file", file);
        const { data } = await adminApi.post(
          `${adminPaths.systemConfig}/storyboard-assets`,
          body,
          { headers: { "Content-Type": "multipart/form-data" } }
        );
        appendReference(field, data);
      }
    } catch (uploadError) {
      window.alert(uploadError instanceof Error ? uploadError.message : "上传分镜参考失败");
    } finally {
      setUploadingField(null);
    }
  };

  const handleSave = async () => {
    try {
      await updateM.mutateAsync({
        storyboardPrompt: formData.storyboardPrompt || "",
        storyboardModel: formData.storyboardModel || "",
        storyboardReferences: formData.storyboardReferences || [],
        imageStoryboardPrompt: formData.imageStoryboardPrompt || "",
        imageStoryboardModel: formData.imageStoryboardModel || "",
        imageStoryboardReferences: formData.imageStoryboardReferences || [],
      });
      window.alert("分镜配置保存成功");
    } catch (saveError) {
      window.alert(saveError instanceof Error ? saveError.message : "分镜配置保存失败");
    }
  };

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center p-20 text-[var(--color-text-secondary)]">
        <Loader2 className="mb-4 h-8 w-8 animate-spin" />
        <p>正在读取分镜配置...</p>
      </div>
    );
  }

  if (error || !config) {
    return <div className="p-10 text-red-500">读取分镜配置失败，请确保您有足够权限。</div>;
  }

  return (
    <div className="max-w-6xl space-y-6">
      <div className="flex items-start justify-between gap-4">
        <PageHeader
          title="分镜优化管理"
          subtitle="统一治理系统级分镜提示词。技能页只选择最终生成模型；若技能同时上传图片和文本参考，系统会先在这里完成分镜优化，再把优化结果交给技能的最终模型继续生成。"
        />
        <button
          onClick={handleSave}
          disabled={updateM.isPending}
          className="flex items-center gap-2 rounded-lg bg-[var(--color-primary)] px-4 py-2 font-medium text-white transition-colors hover:brightness-110 disabled:opacity-50"
        >
          {updateM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
          保存分镜配置
        </button>
      </div>

      <div className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-5">
        <div className="flex items-start gap-3">
          <Sparkles className="mt-0.5 h-5 w-5 text-[var(--color-primary)]" />
          <div className="space-y-1 text-sm text-[var(--color-text-secondary)]">
            <p className="font-medium text-[var(--color-text-primary)]">执行逻辑</p>
            <p>1. 技能页里的“最终生成模型”来自后端已启用模型列表，并按输出格式筛选。</p>
            <p>2. 如果技能资产里同时有参考图片和文本文件，系统会先在这里做分镜脚本优化。</p>
            <p>3. 优化后的脚本会继续传给技能所选的图片或视频模型执行生成。</p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
        <StoryboardSection
          icon={<Video className="h-4 w-4 text-[var(--color-primary)]" />}
          title="视频 / 通用分镜优化"
          description="用于视频生成链路，也作为图片分镜未单独配置时的默认回退。"
          promptValue={formData.storyboardPrompt || ""}
          modelValue={formData.storyboardModel || ""}
          references={formData.storyboardReferences || []}
          modelOptions={modelOptions.map((item) => item.modelName)}
          uploading={uploadingField === "storyboardReferences"}
          onPromptChange={(value) => setFormData((current) => ({ ...current, storyboardPrompt: value }))}
          onModelChange={(value) => setFormData((current) => ({ ...current, storyboardModel: value }))}
          onUpload={(files) => uploadStoryboardFiles("storyboardReferences", files)}
          onRemoveReference={(storageKey, fileName) => removeReference("storyboardReferences", storageKey, fileName)}
        />

        <StoryboardSection
          icon={<ImagePlus className="h-4 w-4 text-[var(--color-primary)]" />}
          title="图片分镜优化"
          description="用于图片生成链路；若为空，系统会自动回退到视频 / 通用分镜配置。"
          promptValue={formData.imageStoryboardPrompt || ""}
          modelValue={formData.imageStoryboardModel || ""}
          references={formData.imageStoryboardReferences || []}
          modelOptions={modelOptions.map((item) => item.modelName)}
          uploading={uploadingField === "imageStoryboardReferences"}
          onPromptChange={(value) => setFormData((current) => ({ ...current, imageStoryboardPrompt: value }))}
          onModelChange={(value) => setFormData((current) => ({ ...current, imageStoryboardModel: value }))}
          onUpload={(files) => uploadStoryboardFiles("imageStoryboardReferences", files)}
          onRemoveReference={(storageKey, fileName) => removeReference("imageStoryboardReferences", storageKey, fileName)}
        />
      </div>
    </div>
  );
}

function StoryboardSection({
  icon,
  title,
  description,
  promptValue,
  modelValue,
  references,
  modelOptions,
  uploading,
  onPromptChange,
  onModelChange,
  onUpload,
  onRemoveReference,
}: {
  icon: ReactNode;
  title: string;
  description: string;
  promptValue: string;
  modelValue: string;
  references: StoryboardReference[];
  modelOptions: string[];
  uploading: boolean;
  onPromptChange: (value: string) => void;
  onModelChange: (value: string) => void;
  onUpload: (files: FileList | null) => void;
  onRemoveReference: (storageKey?: string, fileName?: string) => void;
}) {
  return (
    <div className="space-y-5 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-6">
      <h3 className="flex items-center gap-2 border-b border-[var(--color-border)] pb-2 text-base font-medium">
        {icon}
        {title}
      </h3>
      <p className="text-sm text-[var(--color-text-secondary)]">{description}</p>

      <div className="space-y-4">
        <div>
          <label className="mb-1 block text-sm font-medium">系统分镜模型</label>
          <select
            value={modelValue}
            onChange={(event) => onModelChange(event.target.value)}
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
          >
            <option value="">默认使用系统 Chat 模型</option>
            {modelOptions.map((item) => (
              <option key={item} value={item}>
                {item}
              </option>
            ))}
          </select>
          <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
            下拉列表来自后端已启用的 Chat 模型。
          </p>
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium">系统提示词</label>
          <textarea
            value={promptValue}
            onChange={(event) => onPromptChange(event.target.value)}
            rows={8}
            placeholder="描述希望大模型如何整合图片、文本、镜头、文案和节奏。"
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm focus:border-[var(--color-primary)] focus:outline-none"
          />
        </div>

        <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="text-sm font-medium">参考文件</p>
              <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                支持图片和文本文件，保存后会作为系统级分镜参考。
              </p>
            </div>
            <label className="inline-flex cursor-pointer items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2 text-sm transition-colors hover:border-[var(--color-primary)] hover:text-[var(--color-primary)]">
              {uploading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
              上传图文参考
              <input
                type="file"
                multiple
                accept="image/*,.txt,.md,.json,.csv,.xml,text/plain,text/markdown,application/json"
                className="hidden"
                onChange={(event) => onUpload(event.target.files)}
              />
            </label>
          </div>

          <div className="mt-4 space-y-3">
            {references.length === 0 ? (
              <div className="rounded-lg border border-dashed border-[var(--color-border)] px-3 py-4 text-sm text-[var(--color-text-secondary)]">
                暂无系统级参考文件。
              </div>
            ) : (
              references.map((item) => (
                <div
                  key={item.storageKey || item.fileName}
                  className="flex items-center justify-between rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-3"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-[var(--color-bg-secondary)]">
                      {item.kind === "image" ? (
                        <FileImage className="h-4 w-4 text-sky-400" />
                      ) : (
                        <FileText className="h-4 w-4 text-amber-400" />
                      )}
                    </div>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{item.fileName}</p>
                      <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                        {item.mimeType || item.kind || "reference"}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    {item.publicUrl ? (
                      <a
                        href={item.publicUrl}
                        target="_blank"
                        rel="noreferrer"
                        className="text-xs text-[var(--color-primary)] hover:underline"
                      >
                        预览
                      </a>
                    ) : null}
                    <button
                      type="button"
                      onClick={() => onRemoveReference(item.storageKey, item.fileName)}
                      className="rounded-lg border border-[var(--color-border)] p-2 text-[var(--color-text-secondary)] transition-colors hover:border-red-500/40 hover:text-red-500"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
