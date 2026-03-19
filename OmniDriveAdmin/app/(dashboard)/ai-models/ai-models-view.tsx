"use client";

import { useState } from "react";
import {
  useAIModels,
  useDeleteAIModel,
  useUpdateAIModel,
} from "@/lib/hooks/useAIModels";
import { PageHeader } from "@/components/ui/common";
import { Search, Plus, Loader2, ChevronRight, Trash2 } from "lucide-react";
import { AIModel } from "@/lib/types";
import { AIModelDrawer } from "./ai-model-drawer";

const CATEGORY_OPTIONS = [
  { value: "", label: "全部类型" },
  { value: "image", label: "作图" },
  { value: "video", label: "做视频" },
  { value: "chat", label: "聊天" },
  { value: "music", label: "音乐" },
] as const;

const CATEGORY_LABELS: Record<string, string> = {
  image: "作图",
  video: "做视频",
  chat: "聊天",
  music: "音乐",
};

const BILLING_MODE_LABELS: Record<string, string> = {
  per_call: "按次计费",
  per_second: "按秒计费",
  per_token: "按 Token 计费",
};

function renderPricingSummary(model: AIModel) {
  if (model.billingMode === "per_token") {
    return (
      <div className="text-xs text-[var(--color-text-secondary)]">
        输入 原始 {model.chatInputRawRate ?? "—"} / 计费{" "}
        {model.chatInputBillingAmount ?? "—"} <br />
        输出 原始 {model.chatOutputRawRate ?? "—"} / 计费{" "}
        {model.chatOutputBillingAmount ?? "—"}
      </div>
    );
  }

  return (
    <div className="text-xs text-[var(--color-text-secondary)]">
      {model.billingMode === "per_second" ? "每秒" : "按次"} 原始{" "}
      {model.rawRate ?? "—"} / 计费 {model.billingAmount ?? "—"}
    </div>
  );
}

export function AIModelsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState("");
  const [category, setCategory] = useState("");
  const [selectedModel, setSelectedModel] = useState<AIModel | null>(null);
  const [showCreateDrawer, setShowCreateDrawer] = useState(false);

  const { data, isLoading, error } = useAIModels({
    page,
    pageSize: 20,
    query: query || undefined,
    status: status || undefined,
    category: category || undefined,
  });
  const updateModel = useUpdateAIModel();
  const deleteModel = useDeleteAIModel();

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setQuery(searchInput);
    setPage(1);
  };

  const handleToggleEnabled = async (model: AIModel) => {
    try {
      await updateModel.mutateAsync({
        modelId: model.id,
        payload: { isEnabled: !model.isEnabled },
      });
    } catch {
      alert("操作失败，请重试");
    }
  };

  const handleDeleteModel = async (model: AIModel) => {
    if (!confirm(`确定要删除模型 ${model.modelName} (${model.id}) 吗？删除后不可恢复！`)) return;
    try {
      await deleteModel.mutateAsync(model.id);
      if (selectedModel?.id === model.id) {
        setSelectedModel(null);
      }
    } catch {
      alert("删除失败，请重试");
    }
  };

  const getCategoryColor = (cat: string) => {
    switch (cat) {
      case "image":
        return "bg-blue-500/10 text-blue-400 border-blue-500/20";
      case "video":
        return "bg-purple-500/10 text-purple-400 border-purple-500/20";
      case "chat":
        return "bg-green-500/10 text-green-400 border-green-500/20";
      case "music":
        return "bg-orange-500/10 text-orange-400 border-orange-500/20";
      default:
        return "bg-[var(--color-surface)] text-[var(--color-text-secondary)] border-[var(--color-border)]";
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader
          title="模型管理"
          subtitle="管理 OmniDrive 可调用的模型类型、任务路由、计费参数和启用状态。"
        />
        <button
          onClick={() => setShowCreateDrawer(true)}
          className="flex items-center gap-2 px-4 py-2 bg-[var(--color-accent)] text-white text-sm font-medium rounded-lg hover:bg-[var(--color-accent)]/90 transition-colors"
        >
          <Plus className="h-4 w-4" />
          新增模型
        </button>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center bg-[var(--color-surface)] p-4 rounded-xl border border-[var(--color-border)]">
        <form onSubmit={handleSearch} className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            placeholder="搜索模型ID / 名称 / 厂商 / Base URL..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)] transition-all"
          />
        </form>
        <select
          value={category}
          onChange={(e) => {
            setCategory(e.target.value);
            setPage(1);
          }}
          className="px-3 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
        >
          {CATEGORY_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <select
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            setPage(1);
          }}
          className="px-3 py-2 bg-[var(--color-background)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-accent)]"
        >
          <option value="">全部状态</option>
          <option value="active">已启用</option>
          <option value="inactive">已停用</option>
        </select>
      </div>

      {/* Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-background)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-surface)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-6 py-4 font-medium">模型 ID / 名称</th>
                <th className="px-6 py-4 font-medium">厂商 / Base URL</th>
                <th className="px-6 py-4 font-medium">类型 / 计费</th>
                <th className="px-6 py-4 font-medium">状态</th>
                <th className="px-6 py-4 font-medium">更新时间</th>
                <th className="px-6 py-4 font-medium text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {isLoading && (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center">
                    <Loader2 className="h-6 w-6 animate-spin mx-auto text-[var(--color-text-secondary)]" />
                    <p className="mt-2 text-[var(--color-text-secondary)]">
                      加载模型数据中...
                    </p>
                  </td>
                </tr>
              )}
              {error && (
                <tr>
                  <td
                    colSpan={6}
                    className="px-6 py-12 text-center text-red-500"
                  >
                    加载失败，请重试
                  </td>
                </tr>
              )}
              {data && data.items.length === 0 && (
                <tr>
                  <td
                    colSpan={6}
                    className="px-6 py-12 text-center text-[var(--color-text-secondary)]"
                  >
                    未找到 AI 模型配置
                  </td>
                </tr>
              )}
              {data &&
                data.items.map((model) => (
                  <tr
                    key={model.id}
                    className="hover:bg-[var(--color-surface)]/50 transition-colors"
                  >
                    <td className="px-6 py-4">
                      <div className="font-medium text-[var(--color-text-primary)]">
                        {model.modelName}
                      </div>
                      <div className="text-xs font-mono text-[var(--color-text-secondary)] mt-0.5">
                        {model.id}
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <div className="space-y-1">
                        <span className="inline-flex px-2 py-1 text-xs rounded bg-[var(--color-surface)] border border-[var(--color-border)]">
                          {model.vendor}
                        </span>
                        <div
                          className="text-xs text-[var(--color-text-secondary)] max-w-[260px] truncate"
                          title={model.baseUrl || "未配置 Base URL"}
                        >
                          {model.baseUrl || "未配置 Base URL"}
                        </div>
                        <div className="text-xs text-[var(--color-text-secondary)]">
                          {model.apiKey ? "专用 Key 已配置" : "使用系统默认 Key"}
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <div className="space-y-1">
                        <span
                          className={`inline-flex px-2 py-1 text-xs rounded-full border font-medium ${getCategoryColor(model.category)}`}
                        >
                          {CATEGORY_LABELS[model.category] || model.category}
                        </span>
                        <div className="text-xs text-[var(--color-text-secondary)]">
                          {BILLING_MODE_LABELS[model.billingMode] ||
                            model.billingMode}
                        </div>
                        {renderPricingSummary(model)}
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <button
                        onClick={() => handleToggleEnabled(model)}
                        className={`relative inline-flex items-center h-5 rounded-full w-10 transition-colors focus:outline-none ${model.isEnabled ? "bg-green-500" : "bg-[var(--color-border)]"}`}
                      >
                        <span
                          className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform ${model.isEnabled ? "translate-x-5" : "translate-x-1"}`}
                        />
                      </button>
                    </td>
                    <td className="px-6 py-4 text-xs text-[var(--color-text-secondary)]">
                      {new Date(model.updatedAt).toLocaleString("zh-CN", {
                        month: "2-digit",
                        day: "2-digit",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </td>
                    <td className="px-6 py-4 text-right">
                      <div className="inline-flex items-center gap-3">
                        <button
                          onClick={() => setSelectedModel(model)}
                          className="inline-flex items-center gap-1 text-xs font-medium text-[var(--color-accent)] hover:underline"
                        >
                          编辑 <ChevronRight className="h-3 w-3" />
                        </button>
                        <button
                          onClick={() => void handleDeleteModel(model)}
                          className="inline-flex items-center gap-1 text-xs font-medium text-red-400 hover:underline disabled:opacity-50"
                          disabled={deleteModel.isPending}
                        >
                          <Trash2 className="h-3 w-3" />
                          删除
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Pagination */}
      {data && data.pagination && data.pagination.totalPages > 1 && (
        <div className="flex items-center justify-between px-1">
          <p className="text-sm text-[var(--color-text-secondary)]">
            共 <span className="font-medium">{data.pagination.total}</span>{" "}
            个模型
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
              className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-surface)] transition-colors"
            >
              上一页
            </button>
            <button
              onClick={() =>
                setPage((p) => Math.min(data.pagination.totalPages, p + 1))
              }
              disabled={page >= data.pagination.totalPages}
              className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-surface)] transition-colors"
            >
              下一页
            </button>
          </div>
        </div>
      )}

      {/* Edit Drawer */}
      <AIModelDrawer
        key={selectedModel?.id ?? (showCreateDrawer ? "create" : "closed")}
        model={selectedModel}
        isOpen={!!selectedModel || showCreateDrawer}
        onClose={() => {
          setSelectedModel(null);
          setShowCreateDrawer(false);
        }}
        isCreate={showCreateDrawer && !selectedModel}
      />
    </div>
  );
}
