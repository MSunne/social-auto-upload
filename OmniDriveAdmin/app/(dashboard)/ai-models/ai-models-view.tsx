"use client";

import { useState } from "react";
import { useAIModels, useUpdateAIModel } from "@/lib/hooks/useAIModels";
import { PageHeader } from "@/components/ui/common";
import { Search, Plus, Loader2, ChevronRight } from "lucide-react";
import { AIModel } from "@/lib/types";
import { AIModelDrawer } from "./ai-model-drawer";

export function AIModelsView() {
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState("");
  const [selectedModel, setSelectedModel] = useState<AIModel | null>(null);
  const [showCreateDrawer, setShowCreateDrawer] = useState(false);

  const { data, isLoading, error } = useAIModels({ page, pageSize: 20, query: query || undefined, status: status || undefined });
  const updateModel = useUpdateAIModel();

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setQuery(searchInput);
    setPage(1);
  };

  const handleToggleEnabled = async (model: AIModel) => {
    try {
      await updateModel.mutateAsync({ modelId: model.id, payload: { isEnabled: !model.isEnabled } });
    } catch {
      alert("操作失败，请重试");
    }
  };

  const getCategoryColor = (cat: string) => {
    switch (cat) {
      case "image": return "bg-blue-500/10 text-blue-400 border-blue-500/20";
      case "video": return "bg-purple-500/10 text-purple-400 border-purple-500/20";
      case "text": return "bg-green-500/10 text-green-400 border-green-500/20";
      case "audio": return "bg-orange-500/10 text-orange-400 border-orange-500/20";
      default: return "bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] border-[var(--color-border)]";
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <PageHeader
          title="AI 模型配置"
          subtitle="管理全平台的 AI 大模型接入、计费参数与启用状态。"
        />
        <button
          onClick={() => setShowCreateDrawer(true)}
          className="flex items-center gap-2 px-4 py-2 bg-[var(--color-primary)] text-white text-sm font-medium rounded-lg hover:bg-[var(--color-primary)]/90 transition-colors"
        >
          <Plus className="h-4 w-4" />
          新增模型
        </button>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center bg-[var(--color-bg-secondary)] p-4 rounded-xl border border-[var(--color-border)]">
        <form onSubmit={handleSearch} className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--color-text-secondary)]" />
          <input
            type="text"
            placeholder="搜索模型ID / 名称 / 厂商..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)] transition-all"
          />
        </form>
        <select
          value={status}
          onChange={(e) => { setStatus(e.target.value); setPage(1); }}
          className="px-3 py-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg text-sm focus:outline-none focus:border-[var(--color-primary)]"
        >
          <option value="">全部状态</option>
          <option value="active">已启用</option>
          <option value="inactive">已停用</option>
        </select>
      </div>

      {/* Table */}
      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden bg-[var(--color-bg-primary)]">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-[var(--color-text-secondary)] uppercase bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)]">
              <tr>
                <th className="px-6 py-4 font-medium">模型 ID / 名称</th>
                <th className="px-6 py-4 font-medium">厂商 (Vendor)</th>
                <th className="px-6 py-4 font-medium">分类</th>
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
                    <p className="mt-2 text-[var(--color-text-secondary)]">加载模型数据中...</p>
                  </td>
                </tr>
              )}
              {error && (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-red-500">加载失败，请重试</td>
                </tr>
              )}
              {data && data.data.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-[var(--color-text-secondary)]">未找到 AI 模型配置</td>
                </tr>
              )}
              {data && data.data.map((model) => (
                <tr key={model.id} className="hover:bg-[var(--color-bg-secondary)]/50 transition-colors">
                  <td className="px-6 py-4">
                    <div className="font-medium text-[var(--color-text-primary)]">{model.modelName}</div>
                    <div className="text-xs font-mono text-[var(--color-text-secondary)] mt-0.5">{model.id}</div>
                  </td>
                  <td className="px-6 py-4">
                    <span className="px-2 py-1 text-xs rounded bg-[var(--color-bg-secondary)] border border-[var(--color-border)]">{model.vendor}</span>
                  </td>
                  <td className="px-6 py-4">
                    <span className={`px-2 py-1 text-xs rounded-full border font-medium ${getCategoryColor(model.category)}`}>
                      {model.category}
                    </span>
                  </td>
                  <td className="px-6 py-4">
                    <button
                      onClick={() => handleToggleEnabled(model)}
                      className={`relative inline-flex items-center h-5 rounded-full w-10 transition-colors focus:outline-none ${model.isEnabled ? "bg-green-500" : "bg-[var(--color-border)]"}`}
                    >
                      <span className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform ${model.isEnabled ? "translate-x-5" : "translate-x-1"}`} />
                    </button>
                  </td>
                  <td className="px-6 py-4 text-xs text-[var(--color-text-secondary)]">
                    {new Date(model.updatedAt).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })}
                  </td>
                  <td className="px-6 py-4 text-right">
                    <button
                      onClick={() => setSelectedModel(model)}
                      className="inline-flex items-center gap-1 text-xs font-medium text-[var(--color-primary)] hover:underline"
                    >
                      编辑 <ChevronRight className="h-3 w-3" />
                    </button>
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
            共 <span className="font-medium">{data.pagination.total}</span> 个模型
          </p>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">上一页</button>
            <button onClick={() => setPage(p => Math.min(data.pagination.totalPages, p + 1))} disabled={page >= data.pagination.totalPages} className="px-3 py-1.5 text-sm border border-[var(--color-border)] rounded-lg disabled:opacity-50 hover:bg-[var(--color-bg-secondary)] transition-colors">下一页</button>
          </div>
        </div>
      )}

      {/* Edit Drawer */}
      <AIModelDrawer
        model={selectedModel}
        isOpen={!!selectedModel || showCreateDrawer}
        onClose={() => { setSelectedModel(null); setShowCreateDrawer(false); }}
        isCreate={showCreateDrawer && !selectedModel}
      />
    </div>
  );
}
