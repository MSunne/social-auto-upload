"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { motion, AnimatePresence } from "framer-motion";
import {
  AlertTriangle,
  Download,
  History,
  Image as ImageIcon,
  Search,
  X,
  Eye,
  CalendarDays,
  Box,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { getModelDisplayName } from "@/lib/model-display";
import {
  buildAIJobTitle,
  formatDateTime,
  resolveAIJobStage,
} from "@/lib/workflow";
import { getAIJob, getAIJobArtifacts, listAIJobs } from "@/lib/services";
import type { AIJob, AIJobArtifact } from "@/lib/types";

const IMAGE_HISTORY_PAGE_SIZE = 20;

function isTerminalJob(job?: AIJob | null) {
  if (!job) return false;
  return ["success", "completed", "failed", "cancelled"].includes(job.status);
}

function isSuccessJob(job?: AIJob | null) {
  if (!job) return false;
  return ["success", "completed"].includes(job.status);
}

function extractImageArtifacts(job?: AIJob | null, artifacts: AIJobArtifact[] = []) {
  const allArtifacts = [
    ...artifacts.map((artifact) => ({
      id: artifact.id,
      publicUrl: artifact.publicUrl,
      fileName: artifact.fileName,
      mimeType: artifact.mimeType,
      artifactType: artifact.artifactType,
    })),
    ...(((job?.outputPayload as Record<string, unknown> | null)?.artifacts as Record<string, unknown>[] | undefined) || []).map(
      (item, index) => ({
        id: String(item.id || `payload_${index}`),
        publicUrl: typeof item.publicUrl === "string" ? item.publicUrl : null,
        fileName: typeof item.fileName === "string" ? item.fileName : null,
        mimeType: typeof item.mimeType === "string" ? item.mimeType : null,
        artifactType: String(item.artifactType || ""),
      }),
    ),
  ];

  return allArtifacts.filter((artifact) => {
    if (!artifact.publicUrl) return false;
    if (artifact.artifactType === "image") return true;
    return (artifact.mimeType || "").startsWith("image/");
  });
}

function getJobTimelineTime(job: AIJob) {
  return (
    new Date(job.runAt || 0).getTime() ||
    new Date(job.createdAt || 0).getTime() ||
    new Date(job.updatedAt || 0).getTime()
  );
}

function sortJobsByLatest(left: AIJob, right: AIJob) {
  const timeDiff = getJobTimelineTime(right) - getJobTimelineTime(left);
  if (timeDiff !== 0) {
    return timeDiff;
  }
  const updatedDiff = new Date(right.updatedAt || 0).getTime() - new Date(left.updatedAt || 0).getTime();
  if (updatedDiff !== 0) {
    return updatedDiff;
  }
  return right.id.localeCompare(left.id);
}

function mergeJobsByID(...groups: AIJob[][]) {
  const jobMap = new Map<string, AIJob>();
  groups.flat().forEach((job) => {
    jobMap.set(job.id, job);
  });
  return Array.from(jobMap.values()).sort(sortJobsByLatest);
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) {
    return error.message.trim();
  }
  return fallback;
}

function getPromptDetail(job?: AIJob | null) {
  if (!job) return "";
  const inputPayload =
    job.inputPayload && typeof job.inputPayload === "object" && !Array.isArray(job.inputPayload)
      ? (job.inputPayload as Record<string, unknown>)
      : null;
  const payloadPrompt =
    typeof inputPayload?.prompt === "string" && inputPayload.prompt.trim()
      ? inputPayload.prompt.trim()
      : "";
  if (payloadPrompt) {
    return payloadPrompt;
  }
  const prompt = typeof job.prompt === "string" && job.prompt.trim() ? job.prompt.trim() : "";
  if (prompt) {
    return prompt;
  }
  return buildAIJobTitle(job);
}

type FilterStatus = "all" | "processing" | "completed" | "failed";

export default function ImageHistoryPage() {
  const [filter, setFilter] = useState<FilterStatus>("all");
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  const [olderJobs, setOlderJobs] = useState<AIJob[]>([]);
  const [hasMore, setHasMore] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [loadMoreError, setLoadMoreError] = useState("");

  const {
    data: latestJobs = [],
    isLoading,
    isError,
    error,
    isFetching,
    isRefetchError,
    refetch,
  } = useQuery<AIJob[]>({
    queryKey: ["aiJobs", { jobType: "image", source: "omnidrive_cloud", scope: "history-recent" }],
    queryFn: () =>
      listAIJobs({
        jobType: "image",
        source: "omnidrive_cloud",
        payloadMode: "summary",
        limit: IMAGE_HISTORY_PAGE_SIZE,
      }),
    refetchInterval: (query) => {
      const active = query.state.data?.some((job) => !isTerminalJob(job));
      return active ? 10_000 : false;
    },
    staleTime: 10_000,
  });

  useEffect(() => {
    if (olderJobs.length === 0) {
      setHasMore(latestJobs.length === IMAGE_HISTORY_PAGE_SIZE);
    }
  }, [latestJobs, olderJobs.length]);

  const rawJobs = useMemo(() => mergeJobsByID(latestJobs, olderJobs), [latestJobs, olderJobs]);

  const filteredJobs = useMemo(() => {
    const normalizedQuery = searchQuery.toLowerCase().trim();
    return rawJobs
      .filter((job) => {
        const title = buildAIJobTitle(job).toLowerCase();
        if (normalizedQuery && !title.includes(normalizedQuery)) return false;

        const isTerminal = isTerminalJob(job);
        const isSuccess = isSuccessJob(job);

        if (filter === "processing" && isTerminal) return false;
        if (filter === "completed" && !isSuccess) return false;
        if (filter === "failed" && (!isTerminal || isSuccess)) return false;

        return true;
      })
      .sort(sortJobsByLatest);
  }, [rawJobs, filter, searchQuery]);

  const selectedJob = useMemo(() => {
    return rawJobs.find((job) => job.id === selectedJobId) || null;
  }, [rawJobs, selectedJobId]);

  const { data: selectedJobDetail } = useQuery<AIJob>({
    queryKey: ["aiJobDetail", selectedJobId],
    queryFn: () => getAIJob(selectedJobId!),
    enabled: !!selectedJobId,
    staleTime: 10_000,
  });

  const selectedJobView = selectedJobDetail || selectedJob;

  const { data: selectedArtifacts = [] } = useQuery<AIJobArtifact[]>({
    queryKey: ["aiJobArtifacts", selectedJobId],
    queryFn: () => getAIJobArtifacts(selectedJobId!),
    enabled: !!selectedJobId && isTerminalJob(selectedJobView) && isSuccessJob(selectedJobView),
  });

  const selectedPreviewItems = useMemo(() => {
    return extractImageArtifacts(selectedJobView, selectedArtifacts);
  }, [selectedJobView, selectedArtifacts]);

  const initialErrorMessage = getErrorMessage(error, "图片历史加载失败，请稍后重试");

  async function handleLoadMore() {
    if (isLoadingMore || !hasMore || rawJobs.length === 0) {
      return;
    }
    const cursorJob = rawJobs[rawJobs.length - 1];
    if (!cursorJob?.updatedAt) {
      setHasMore(false);
      return;
    }

    setIsLoadingMore(true);
    setLoadMoreError("");
    try {
      const nextJobs = await listAIJobs({
        jobType: "image",
        source: "omnidrive_cloud",
        payloadMode: "summary",
        limit: IMAGE_HISTORY_PAGE_SIZE,
        beforeUpdatedAt: cursorJob.updatedAt,
        beforeId: cursorJob.id,
      });
      setOlderJobs((current) => mergeJobsByID(current, nextJobs));
      setHasMore(nextJobs.length === IMAGE_HISTORY_PAGE_SIZE);
    } catch (loadError) {
      setLoadMoreError(getErrorMessage(loadError, "加载更多失败，请稍后重试"));
    } finally {
      setIsLoadingMore(false);
    }
  }

  return (
    <div className="flex h-full flex-col p-6">
      <header className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight text-text-primary">
            <History className="h-6 w-6 text-accent" />
            图片历史
          </h1>
          <p className="mt-1 text-sm text-text-muted">管理您的图片生成任务和历史记录</p>
        </div>

        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted" />
            <input
              type="text"
              placeholder="搜索已加载记录..."
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              className="w-full rounded-xl border border-border bg-surface px-9 py-2 text-sm text-text-primary focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent sm:w-64"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery("")}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary"
              >
                <X className="h-3 w-3" />
              </button>
            )}
          </div>

          <div className="flex items-center gap-1 rounded-xl border border-border/50 p-1">
            {[
              { id: "all", label: "全部" },
              { id: "processing", label: "进行中" },
              { id: "completed", label: "已完成" },
              { id: "failed", label: "失败" },
            ].map((item) => (
              <button
                key={item.id}
                onClick={() => setFilter(item.id as FilterStatus)}
                className={cn(
                  "rounded-lg px-3 py-1.5 text-xs font-medium transition-all",
                  filter === item.id
                    ? "bg-accent/15 text-accent-strong shadow-sm"
                    : "text-text-secondary hover:bg-surface-hover hover:text-text-primary",
                )}
              >
                {item.label}
              </button>
            ))}
          </div>
        </div>
      </header>

      <div className="custom-scrollbar flex-1 overflow-y-auto rounded-xl border border-border/50 bg-surface/30">
        {isLoading && rawJobs.length === 0 ? (
          <div className="flex h-40 items-center justify-center text-text-muted">加载中...</div>
        ) : isError && rawJobs.length === 0 ? (
          <div className="flex h-56 flex-col items-center justify-center gap-3 px-6 text-center">
            <AlertTriangle className="h-9 w-9 text-danger" />
            <div className="space-y-1">
              <p className="text-sm font-medium text-text-primary">图片历史暂时无法加载</p>
              <p className="text-xs text-text-muted">{initialErrorMessage}</p>
            </div>
            <button
              onClick={() => void refetch()}
              className="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-strong"
            >
              重试
            </button>
          </div>
        ) : filteredJobs.length === 0 ? (
          <div className="flex h-40 flex-col items-center justify-center text-text-muted">
            <ImageIcon className="mb-2 h-8 w-8 opacity-20" />
            <p>暂无符合条件的图片记录</p>
          </div>
        ) : (
          <div className="min-w-[800px]">
            {(isRefetchError || loadMoreError) && (
              <div className="border-b border-warning/20 bg-warning/10 px-6 py-3 text-xs text-warning">
                {isRefetchError ? "图片历史刷新失败，已保留上次成功结果。" : loadMoreError}
              </div>
            )}

            <div className="grid grid-cols-[3fr_1.5fr_1.5fr_1.5fr_1fr] gap-4 border-b border-border/50 bg-surface-hover/50 px-6 py-3 text-xs font-semibold uppercase tracking-wider text-text-muted">
              <div>任务名称 / 提示词</div>
              <div>模型</div>
              <div>创建时间</div>
              <div>状态</div>
              <div className="text-right">操作</div>
            </div>

            <div className="flex flex-col">
              {filteredJobs.map((job) => {
                const stage = resolveAIJobStage(job);
                return (
                  <div
                    key={job.id}
                    className="grid grid-cols-[3fr_1.5fr_1.5fr_1.5fr_1fr] items-center gap-4 border-b border-border/10 px-6 py-4 transition-colors hover:bg-surface-hover/30"
                  >
                    <div className="flex items-center gap-3 overflow-hidden">
                      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-border/50 bg-surface-elevated">
                        <ImageIcon className="h-5 w-5 text-accent/70" />
                      </div>
                      <div className="truncate">
                        <h4 className="truncate text-sm font-medium text-text-primary" title={buildAIJobTitle(job)}>
                          {buildAIJobTitle(job)}
                        </h4>
                        <p className="truncate text-xs text-text-muted">ID: {job.id}</p>
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      <Box className="h-3.5 w-3.5 text-text-muted" />
                      <span className="text-sm text-text-secondary">{getModelDisplayName(job)}</span>
                    </div>

                    <div className="flex items-center gap-2 text-sm text-text-secondary">
                      <CalendarDays className="h-3.5 w-3.5 text-text-muted" />
                      {formatDateTime(job.createdAt)}
                    </div>

                    <div>
                      <span
                        className={cn(
                          "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium",
                          isSuccessJob(job)
                            ? "border-success/20 bg-success/10 text-success"
                            : isTerminalJob(job)
                              ? "border-danger/20 bg-danger/10 text-danger"
                              : "border-accent/20 bg-accent/10 text-accent",
                        )}
                      >
                        {!isTerminalJob(job) && <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-accent" />}
                        {stage.label}
                      </span>
                    </div>

                    <div className="flex justify-end gap-2">
                      <button
                        onClick={() => setSelectedJobId(job.id)}
                        className="flex items-center gap-1.5 rounded-lg bg-surface-elevated px-3 py-1.5 text-sm font-medium text-text-primary transition-colors hover:bg-accent/20 hover:text-accent-strong"
                      >
                        <Eye className="h-4 w-4" />
                        查看
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>

            <div className="flex items-center justify-between gap-3 border-t border-border/30 px-6 py-4">
              <p className="text-xs text-text-muted">
                已加载 {rawJobs.length} 条记录
                {isFetching && !isLoading ? "，正在刷新最新结果..." : ""}
              </p>
              {hasMore ? (
                <button
                  onClick={() => void handleLoadMore()}
                  disabled={isLoadingMore}
                  className="rounded-lg border border-border bg-surface-elevated px-4 py-2 text-sm font-medium text-text-primary transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {isLoadingMore ? "加载中..." : "加载更多"}
                </button>
              ) : (
                <span className="text-xs text-text-muted">已加载全部可用记录</span>
              )}
            </div>
          </div>
        )}
      </div>

      <AnimatePresence>
        {selectedJobView && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6">
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => setSelectedJobId(null)}
              className="absolute inset-0 bg-background/80 backdrop-blur-sm"
            />
            <motion.div
              initial={{ opacity: 0, scale: 0.95, y: 20 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 20 }}
              className="glass-card flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden"
            >
              <header className="flex shrink-0 items-center justify-between border-b border-border/50 px-6 py-4">
                <h3 className="flex items-center gap-2 text-lg font-semibold text-text-primary">
                  <ImageIcon className="h-5 w-5 text-accent" />
                  图片详情
                </h3>
                <button
                  onClick={() => setSelectedJobId(null)}
                  className="rounded-lg p-1.5 text-text-muted hover:bg-surface-hover hover:text-text-primary"
                >
                  <X className="h-5 w-5" />
                </button>
              </header>

              <div className="custom-scrollbar flex-1 overflow-y-auto p-4 sm:p-6">
                <div className="flex flex-col gap-6 lg:flex-row">
                  <div className="flex flex-1 flex-col gap-4">
                    <div className="relative aspect-square w-full overflow-hidden rounded-xl border border-border/50 bg-black shadow-lg">
                      {selectedPreviewItems.length > 0 ? (
                        <>
                          {/* eslint-disable-next-line @next/next/no-img-element */}
                          <img
                            src={selectedPreviewItems[0].publicUrl!}
                            alt="preview"
                            className="h-full w-full object-contain"
                          />
                          <div className="pointer-events-none absolute left-4 top-4 flex items-center gap-2 rounded-lg border border-white/10 bg-black/40 px-3 py-1.5 backdrop-blur-md">
                            <span
                              className={cn(
                                "flex h-2 w-2 rounded-full",
                                isSuccessJob(selectedJobView)
                                  ? "bg-success pulse-online"
                                  : isTerminalJob(selectedJobView)
                                    ? "bg-danger"
                                    : "bg-warning",
                              )}
                            />
                            <span className="text-xs font-semibold text-white drop-shadow-md">
                              {resolveAIJobStage(selectedJobView).label}
                            </span>
                          </div>
                        </>
                      ) : isTerminalJob(selectedJobView) && !isSuccessJob(selectedJobView) ? (
                        <div className="flex h-full w-full flex-col items-center justify-center gap-3 text-danger">
                          <AlertTriangle className="h-10 w-10" />
                          <span className="text-sm font-medium">生成失败，暂无图片</span>
                        </div>
                      ) : (
                        <div className="flex h-full w-full flex-col items-center justify-center gap-3 text-text-muted">
                          <ImageIcon className="h-10 w-10 opacity-20" />
                          <span className="text-sm">尚未就绪，等待后端返回图片</span>
                        </div>
                      )}
                    </div>

                    <div className="flex items-center gap-3">
                      {selectedPreviewItems.length > 0 && selectedPreviewItems[0].publicUrl && (
                        <a
                          href={selectedPreviewItems[0].publicUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex flex-1 items-center justify-center gap-2 rounded-xl bg-accent px-4 py-3 text-sm font-semibold text-white shadow-[0_0_20px_rgba(177,73,255,0.3)] transition-all hover:bg-accent-strong hover:shadow-[0_0_25px_rgba(177,73,255,0.4)]"
                        >
                          <Download className="h-4 w-4" />
                          下载源图片
                        </a>
                      )}
                    </div>
                  </div>

                  <div className="flex w-full shrink-0 flex-col gap-4 lg:w-[320px]">
                    <div className="flex flex-col gap-1.5 rounded-xl border border-border/30 bg-surface-hover/50 p-4">
                      <h4 className="mb-1 flex items-center gap-1.5 text-xs font-bold uppercase text-text-muted">
                        <Box className="h-3.5 w-3.5" /> 提示词
                      </h4>
                      <p className="break-all text-sm leading-relaxed text-text-primary">
                        {getPromptDetail(selectedJobView)}
                      </p>
                    </div>

                    <div className="grid grid-cols-2 gap-3 lg:grid-cols-1">
                      <div className="flex flex-col gap-1 rounded-xl border border-border/30 bg-surface-hover/30 p-3">
                        <span className="text-[11px] font-semibold uppercase text-text-muted">模型引擎</span>
                        <span className="text-sm font-medium text-text-primary">{getModelDisplayName(selectedJobView)}</span>
                      </div>
                      <div className="flex flex-col gap-1 rounded-xl border border-border/30 bg-surface-hover/30 p-3">
                        <span className="text-[11px] font-semibold uppercase text-text-muted">创建时间</span>
                        <span className="text-[13px] text-text-primary">{formatDateTime(selectedJobView.createdAt)}</span>
                      </div>
                      <div className="flex flex-col gap-1 rounded-xl border border-border/30 bg-surface-hover/30 p-3">
                        <span className="text-[11px] font-semibold uppercase text-text-muted">更新时间</span>
                        <span className="text-[13px] text-text-primary">{formatDateTime(selectedJobView.updatedAt)}</span>
                      </div>
                    </div>

                    {selectedJobView.message && (
                      <div className="mt-2 rounded-xl border border-danger/30 bg-danger/10 p-4">
                        <h4 className="mb-1.5 flex items-center gap-1.5 text-xs font-bold uppercase text-danger">
                          <AlertTriangle className="h-3.5 w-3.5" /> 系统消息
                        </h4>
                        <p className="text-xs leading-normal text-danger-strong">{selectedJobView.message}</p>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </div>
  );
}
