"use client";

import { FormEvent, useEffect, useState } from "react";
import { useAdminSkills, useUpdateAdminSkill } from "@/lib/hooks/useSkills";
import { getModelDisplayName } from "@/lib/model-display";
import { PageHeader } from "@/components/ui/common";
import { Search, Loader2, RefreshCw, Layers, CheckCircle, XCircle, PencilLine, Save, Sparkles } from "lucide-react";
import { AdminSkillSummary } from "@/lib/types";

type SkillEditorForm = {
  description: string;
  promptTemplate: string;
  storyboardPromptTemplate: string;
  publishPromptTemplate: string;
  publishIntroEnabled: boolean;
  topicsText: string;
  storyboardEnabled: boolean;
  isEnabled: boolean;
};

const DEFAULT_SKILL_STORYBOARD_PROMPT =
  "你是内容创作分镜与脚本优化助手。请结合用户目标、参考图片和参考文本，输出适合继续交给图片、视频或文本模型执行的精炼脚本。输出中需要保留主体、场景、镜头、风格、文案和节奏等关键信息。";

function buildEditorForm(skill: AdminSkillSummary | null): SkillEditorForm {
  return {
    description: skill?.description || "",
    promptTemplate: skill?.promptTemplate || "",
    storyboardPromptTemplate: skill?.storyboardPromptTemplate || DEFAULT_SKILL_STORYBOARD_PROMPT,
    publishPromptTemplate: skill?.publishPromptTemplate || "",
    publishIntroEnabled: skill?.publishIntroEnabled !== false,
    topicsText: (skill?.topics || []).join("，"),
    storyboardEnabled: skill?.storyboardEnabled !== false,
    isEnabled: Boolean(skill?.isEnabled),
  };
}

function normalizeTopics(value: string) {
  return value
    .split(/[\n,，#\s]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function SkillsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [statusParam, setStatusParam] = useState("");
  const [selectedSkill, setSelectedSkill] = useState<AdminSkillSummary | null>(null);
  const [editorForm, setEditorForm] = useState<SkillEditorForm>(() => buildEditorForm(null));

  const { data, isLoading, error, refetch } = useAdminSkills({
    page,
    pageSize: 30,
    query: query || undefined,
    status: statusParam || undefined,
  });
  const updateM = useUpdateAdminSkill();

  useEffect(() => {
    if (!selectedSkill || !data) {
      return;
    }
    const nextSelected = data.items.find((item) => item.id === selectedSkill.id);
    if (nextSelected) {
      setSelectedSkill(nextSelected);
      setEditorForm(buildEditorForm(nextSelected));
    }
  }, [data, selectedSkill]);

  const handleSearch = (event: FormEvent) => {
    event.preventDefault();
    setQuery(searchInput);
    setPage(1);
  };

  const handleToggleStatus = async (skill: AdminSkillSummary) => {
    const nextStatus = !skill.isEnabled;
    if (!confirm(`确定要${nextStatus ? "启用" : "停用"}技能 "${skill.name}" 吗？`)) {
      return;
    }
    try {
      await updateM.mutateAsync({
        skillId: skill.id,
        payload: { isEnabled: nextStatus },
      });
      await refetch();
    } catch (saveError) {
      alert(saveError instanceof Error ? saveError.message : "操作失败，请重试");
    }
  };

  const openEditor = (skill: AdminSkillSummary) => {
    setSelectedSkill(skill);
    setEditorForm(buildEditorForm(skill));
  };

  const handleSaveEditor = async () => {
    if (!selectedSkill) {
      return;
    }
    if (!editorForm.description.trim()) {
      alert("简介不能为空");
      return;
    }

    try {
      await updateM.mutateAsync({
        skillId: selectedSkill.id,
        payload: {
          description: editorForm.description.trim(),
          promptTemplate: editorForm.promptTemplate.trim() || null,
          storyboardPromptTemplate: editorForm.storyboardPromptTemplate.trim() || null,
          publishPromptTemplate: editorForm.publishPromptTemplate.trim() || null,
          publishIntroEnabled: editorForm.publishIntroEnabled,
          topics: normalizeTopics(editorForm.topicsText),
          storyboardEnabled: editorForm.storyboardEnabled,
          isEnabled: editorForm.isEnabled,
        },
      });
      await refetch();
      alert("技能治理内容已保存");
    } catch (saveError) {
      alert(saveError instanceof Error ? saveError.message : "保存失败，请重试");
    }
  };

  const table = (
    <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
      <div className="overflow-x-auto">
        <table className="w-full text-sm text-left">
          <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
            <tr>
              <th className="px-5 py-3.5 font-medium">技能</th>
              <th className="px-5 py-3.5 font-medium">简介</th>
              <th className="px-5 py-3.5 font-medium">模型</th>
              <th className="px-5 py-3.5 font-medium text-center">状态</th>
              <th className="px-5 py-3.5 font-medium text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--color-border)]">
            {isLoading && (
              <tr>
                <td colSpan={5} className="px-6 py-12 text-center">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)]">加载技能列表...</p>
                </td>
              </tr>
            )}
            {error && (
              <tr>
                <td colSpan={5} className="px-6 py-10 text-center text-red-500 text-sm">加载失败，请重试</td>
              </tr>
            )}
            {data && data.items.length === 0 && (
              <tr>
                <td colSpan={5} className="px-6 py-12 text-center text-[var(--color-text-secondary)] text-sm">暂无技能配置</td>
              </tr>
            )}
            {data && data.items.map((row) => (
              <tr key={row.id} className={`transition-colors ${row.isEnabled ? "hover:bg-[var(--color-bg-secondary)]/50" : "bg-[var(--color-bg-secondary)]/30 opacity-75"}`}>
                <td className="px-5 py-4 align-top">
                  <div className="flex items-start gap-3">
                    <div className="w-8 h-8 rounded-lg bg-[var(--color-primary)]/10 flex items-center justify-center text-[var(--color-primary)]">
                      <Layers className="h-4 w-4" />
                    </div>
                    <div>
                      <div className="text-sm font-medium text-[var(--color-text-primary)]">{row.name}</div>
                      <div className="text-xs text-[var(--color-text-secondary)]">{row.outputType.toUpperCase()}</div>
                      <div className="mt-1 text-[11px] font-mono text-[var(--color-text-secondary)]">{row.id}</div>
                    </div>
                  </div>
                </td>
                <td className="px-5 py-4 align-top">
                  <p className="max-w-sm whitespace-pre-wrap text-xs leading-6 text-[var(--color-text-secondary)]">
                    {row.description || "暂无简介"}
                  </p>
                </td>
                <td className="px-5 py-4 align-top">
                  <div className="space-y-2">
                    <div className="inline-flex items-center px-2 py-1 rounded bg-[var(--color-bg-secondary)] border border-[var(--color-border)] text-xs font-mono">
                      {getModelDisplayName(row)}
                    </div>
                    <div className="flex flex-wrap gap-2 text-[11px] text-[var(--color-text-secondary)]">
                      <span className="rounded-full border border-[var(--color-border)] px-2 py-1">
                        分镜 {row.storyboardEnabled ? "开启" : "关闭"}
                      </span>
                      <span className="rounded-full border border-[var(--color-border)] px-2 py-1">
                        简介 AI {row.publishIntroEnabled ? "开启" : "关闭"}
                      </span>
                      <span className="rounded-full border border-[var(--color-border)] px-2 py-1">
                        标签 {(row.topics || []).length}
                      </span>
                    </div>
                  </div>
                </td>
                <td className="px-5 py-4 align-top text-center">
                  {row.isEnabled ? (
                    <span className="inline-flex items-center gap-1 text-green-500 text-xs font-medium">
                      <CheckCircle className="h-3.5 w-3.5" />
                      已启用
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-1 text-[var(--color-text-secondary)] text-xs">
                      <XCircle className="h-3.5 w-3.5" />
                      已停用
                    </span>
                  )}
                </td>
                <td className="px-5 py-4 align-top">
                  <div className="flex justify-end gap-2">
                    <button
                      onClick={() => openEditor(row)}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium border rounded-md transition-all border-[var(--color-border)] hover:bg-[var(--color-bg-secondary)]"
                    >
                      <PencilLine className="h-3.5 w-3.5" />
                      编辑
                    </button>
                    <button
                      onClick={() => handleToggleStatus(row)}
                      disabled={updateM.isPending}
                      className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium border rounded-md transition-all ${row.isEnabled ? "text-red-500 hover:bg-red-500/10 border-red-500/20" : "text-green-500 hover:bg-green-500/10 border-green-500/20"} disabled:opacity-50`}
                    >
                      {row.isEnabled ? "停用" : "启用"}
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <PageHeader
          title="产品技能配置"
          subtitle="管理系统所有对外技能。这里可以统一治理简介、任务说明、技能默认分镜、简介优化说明、简介 AI 开关、标签和分镜开关。"
        />
        <button
          onClick={() => refetch()}
          className="flex items-center gap-2 px-3 py-2 border border-[var(--color-border)] rounded-lg text-sm hover:bg-[var(--color-bg-secondary)] transition-colors"
        >
          <RefreshCw className="h-4 w-4" />
          <span className="hidden sm:inline">刷新</span>
        </button>
      </div>

      <div className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-4 text-sm text-[var(--color-text-secondary)]">
        <div className="flex items-start gap-3">
          <Sparkles className="mt-0.5 h-5 w-5 text-[var(--color-primary)]" />
          <div className="space-y-1">
            <p className="font-medium text-[var(--color-text-primary)]">当前执行逻辑</p>
            <p>1. 简介是发布到三方平台的基础文案。</p>
            <p>2. 任务说明会参与图片/视频生成；视文模式默认补“默认不要字幕”。</p>
            <p>3. 简介 AI 优化可按技能单独开关；开启后会结合“简介优化说明”生成每次不同但风格一致的发布简介。</p>
            <p>4. 分镜提示词现在按技能单独治理；管理员不修改时，系统使用默认分镜模板。</p>
          </div>
        </div>
      </div>

      <div className="flex flex-col sm:flex-row gap-3 justify-between items-start sm:items-center">
        <div className="flex gap-1 bg-[var(--color-bg-secondary)] p-1 rounded-lg border border-[var(--color-border)] overflow-x-auto w-full sm:w-auto">
          {[{ id: "", label: "全部技能" }, { id: "active", label: "已启用" }, { id: "inactive", label: "已停用" }].map((tab) => (
            <button
              key={tab.id}
              onClick={() => {
                setStatusParam(tab.id);
                setPage(1);
              }}
              className={`px-4 py-1.5 text-sm rounded-md whitespace-nowrap transition-colors ${statusParam === tab.id ? "bg-[var(--color-bg-primary)] text-[var(--color-text-primary)] shadow-sm border border-[var(--color-border)] opacity-100" : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-primary)]/50 border border-transparent opacity-80"}`}
            >
              {tab.label}
            </button>
          ))}
        </div>
        <form onSubmit={handleSearch} className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            placeholder="搜索技能名称..."
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-colors"
          />
        </form>
      </div>

      <div className={`grid gap-6 ${selectedSkill ? "xl:grid-cols-[minmax(0,1fr)_420px]" : ""}`}>
        <div className="space-y-4">
          {table}
          {data && data.pagination && data.pagination.totalPages > 1 && (
            <div className="flex items-center justify-between px-1">
              <p className="text-sm text-[var(--color-text-secondary)]">
                共 <span className="font-medium">{data.pagination.total}</span> 个技能
              </p>
              <div className="flex gap-2">
                <button
                  onClick={() => setPage((current) => Math.max(1, current - 1))}
                  disabled={page === 1}
                  className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors"
                >
                  上一页
                </button>
                <button
                  onClick={() => setPage((current) => Math.min(data.pagination.totalPages, current + 1))}
                  disabled={page >= data.pagination.totalPages}
                  className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors"
                >
                  下一页
                </button>
              </div>
            </div>
          )}
        </div>

        {selectedSkill ? (
          <aside className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-5 h-fit xl:sticky xl:top-6">
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-[var(--color-text-primary)]">{selectedSkill.name}</p>
                <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                  {selectedSkill.outputType.toUpperCase()} · {getModelDisplayName(selectedSkill)}
                </p>
              </div>
              <button
                onClick={() => setSelectedSkill(null)}
                className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
              >
                关闭
              </button>
            </div>

            <div className="mt-5 space-y-4">
              <label className="block space-y-1.5">
                <span className="text-sm font-medium text-[var(--color-text-primary)]">简介</span>
                <textarea
                  rows={5}
                  value={editorForm.description}
                  onChange={(event) => setEditorForm((current) => ({ ...current, description: event.target.value }))}
                  placeholder="发布到三方平台的基础简介"
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm leading-6 focus:border-[var(--color-primary)] focus:outline-none"
                />
              </label>

              <label className="block space-y-1.5">
                <span className="text-sm font-medium text-[var(--color-text-primary)]">任务说明</span>
                <textarea
                  rows={4}
                  value={editorForm.promptTemplate}
                  onChange={(event) => setEditorForm((current) => ({ ...current, promptTemplate: event.target.value }))}
                  placeholder="控制生成画面的镜头、节奏和边界。视文模式默认补“默认不要字幕”。"
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm leading-6 focus:border-[var(--color-primary)] focus:outline-none"
                />
              </label>

              <label className="block space-y-1.5">
                <span className="text-sm font-medium text-[var(--color-text-primary)]">默认分镜</span>
                <textarea
                  rows={6}
                  value={editorForm.storyboardPromptTemplate}
                  onChange={(event) => setEditorForm((current) => ({ ...current, storyboardPromptTemplate: event.target.value }))}
                  placeholder="控制该技能分镜优化时的镜头、节奏、场景、卖点和风格。未修改时会使用系统默认分镜模板。"
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm leading-6 focus:border-[var(--color-primary)] focus:outline-none"
                />
                <p className="text-xs leading-5 text-[var(--color-text-secondary)]">
                  这是该技能自己的分镜模板。现在运行时优先读取这里，不再使用全局统一分镜提示词。
                </p>
              </label>

              <label className="block space-y-1.5">
                <span className="text-sm font-medium text-[var(--color-text-primary)]">简介优化说明</span>
                <textarea
                  rows={4}
                  value={editorForm.publishPromptTemplate}
                  onChange={(event) => setEditorForm((current) => ({ ...current, publishPromptTemplate: event.target.value }))}
                  placeholder="控制每次发布简介的语气、互动方式、平台偏好和变化范围。"
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm leading-6 focus:border-[var(--color-primary)] focus:outline-none"
                />
                <p className="text-xs leading-5 text-[var(--color-text-secondary)]">
                  仅管理员可见。开启“简介 AI 优化”后，系统会按这里的规则在每次发布前重写简介；关闭后沿用用户原始简介。
                </p>
              </label>

              <label className="block space-y-1.5">
                <span className="text-sm font-medium text-[var(--color-text-primary)]">标签</span>
                <textarea
                  rows={3}
                  value={editorForm.topicsText}
                  onChange={(event) => setEditorForm((current) => ({ ...current, topicsText: event.target.value }))}
                  placeholder="多个标签用空格、逗号或换行分隔"
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm leading-6 focus:border-[var(--color-primary)] focus:outline-none"
                />
              </label>

              <div className="grid grid-cols-3 gap-3">
                <button
                  onClick={() => setEditorForm((current) => ({ ...current, publishIntroEnabled: !current.publishIntroEnabled }))}
                  className={`rounded-xl border px-3 py-3 text-left text-sm transition-colors ${editorForm.publishIntroEnabled ? "border-[var(--color-primary)] bg-[var(--color-primary)]/10 text-[var(--color-text-primary)]" : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-secondary)]"}`}
                >
                  <div className="font-medium">简介 AI 优化</div>
                  <div className="mt-1 text-xs">{editorForm.publishIntroEnabled ? "已开启" : "已关闭"}</div>
                </button>
                <button
                  onClick={() => setEditorForm((current) => ({ ...current, storyboardEnabled: !current.storyboardEnabled }))}
                  className={`rounded-xl border px-3 py-3 text-left text-sm transition-colors ${editorForm.storyboardEnabled ? "border-[var(--color-primary)] bg-[var(--color-primary)]/10 text-[var(--color-text-primary)]" : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-secondary)]"}`}
                >
                  <div className="font-medium">分镜优化</div>
                  <div className="mt-1 text-xs">{editorForm.storyboardEnabled ? "已开启" : "已关闭"}</div>
                </button>
                <button
                  onClick={() => setEditorForm((current) => ({ ...current, isEnabled: !current.isEnabled }))}
                  className={`rounded-xl border px-3 py-3 text-left text-sm transition-colors ${editorForm.isEnabled ? "border-green-500/30 bg-green-500/10 text-[var(--color-text-primary)]" : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-secondary)]"}`}
                >
                  <div className="font-medium">技能状态</div>
                  <div className="mt-1 text-xs">{editorForm.isEnabled ? "已启用" : "已停用"}</div>
                </button>
              </div>

              <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-3 text-xs leading-6 text-[var(--color-text-secondary)]">
                全局分镜模型和参考资料仍可在“分镜优化管理”维护；这里控制的是该技能自己的默认分镜、简介、任务说明、简介 AI 优化和分镜参与方式。
              </div>

              <button
                onClick={handleSaveEditor}
                disabled={updateM.isPending}
                className="w-full inline-flex items-center justify-center gap-2 rounded-lg bg-[var(--color-primary)] px-4 py-2.5 text-sm font-medium text-white transition-colors hover:brightness-110 disabled:opacity-50"
              >
                {updateM.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                保存技能治理内容
              </button>
            </div>
          </aside>
        ) : null}
      </div>
    </div>
  );
}
