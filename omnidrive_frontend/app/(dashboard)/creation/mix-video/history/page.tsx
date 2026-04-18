"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, Clock3, Coins, Film, History, LayoutGrid, Play, Scissors, Search, Video, XCircle } from "lucide-react";
import { PageHeader, StatCard, StatusBadge } from "@/components/ui/common";
import {
  clampMixVideoTaskProgress,
  formatMixVideoCreditValue,
  getMixVideoTaskProgressMessage,
  isTerminalMixVideoTask,
  truncateMixVideoText,
} from "@/lib/mix-video";
import { getMixVideoTaskPollInterval } from "@/lib/long-task-poll";
import { listMixVideoTasks } from "@/lib/services";
import type { MixVideoAsset, MixVideoTask } from "@/lib/types";
import { formatDateTime } from "@/lib/workflow";

type FilterKey = "all" | "running" | "completed" | "failed";

function formatDuration(seconds?: number | null) {
  if (typeof seconds !== "number" || !Number.isFinite(seconds) || seconds <= 0) return "";
  if (seconds < 60) return `${seconds.toFixed(0)}s`;
  const m = Math.floor(seconds / 60);
  const s = Math.round(seconds - m * 60);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

function pickPreviewAsset(task: MixVideoTask): { asset: MixVideoAsset | null; kind: "result" | "source" | "none" } {
  if (task.resultAsset?.publicUrl) return { asset: task.resultAsset, kind: "result" };
  const first = task.sourceAssets.find((a) => a.publicUrl);
  if (first) return { asset: first, kind: "source" };
  return { asset: null, kind: "none" };
}

function TaskCardThumbnail({ task }: { task: MixVideoTask }) {
  const { asset, kind } = pickPreviewAsset(task);
  const isVideo = asset?.mimeType.startsWith("video/");
  const isImage = asset?.mimeType.startsWith("image/");
  const progress = clampMixVideoTaskProgress(task);
  const running = !isTerminalMixVideoTask(task);
  const durationLabel = formatDuration(task.actualDurationSeconds ?? task.estimatedDurationSeconds);

  return (
    <div className="relative aspect-video w-full overflow-hidden rounded-t-xl bg-black">
      {asset && isVideo ? (
        <video
          src={`${asset.publicUrl}#t=0.5`}
          className="h-full w-full object-cover"
          muted
          playsInline
          preload="metadata"
        />
      ) : asset && isImage ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={asset.publicUrl} alt={asset.fileName} className="h-full w-full object-cover" />
      ) : (
        <div className="flex h-full flex-col items-center justify-center gap-2 bg-gradient-to-br from-surface/60 to-black/50 text-text-muted">
          <Film className="h-7 w-7" />
          <span className="text-xs">暂无预览</span>
        </div>
      )}

      {/* gradient overlay */}
      <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/70 via-black/10 to-transparent" />

      {/* kind chip */}
      <div className="absolute left-3 top-3 inline-flex items-center gap-1 rounded-full bg-black/55 px-2 py-0.5 text-[10px] text-white">
        {kind === "result" ? <CheckCircle2 className="h-3 w-3 text-success" /> : <Video className="h-3 w-3" />}
        {kind === "result" ? "成片" : kind === "source" ? "首帧素材" : "无预览"}
      </div>

      {/* status / duration */}
      <div className="absolute bottom-3 left-3 right-3 flex items-end justify-between gap-2">
        <StatusBadge status={task.status} />
        {durationLabel ? (
          <span className="rounded-md bg-black/60 px-2 py-0.5 text-[11px] text-white">{durationLabel}</span>
        ) : null}
      </div>

      {/* play hint if result video */}
      {kind === "result" && isVideo ? (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-black/50 text-white transition group-hover:scale-110">
            <Play className="ml-0.5 h-5 w-5 fill-current" />
          </div>
        </div>
      ) : null}

      {/* running progress overlay */}
      {running ? (
        <div className="absolute bottom-0 left-0 right-0 h-1 bg-black/50">
          <div
            className="h-full bg-accent transition-all duration-300"
            style={{ width: `${progress}%` }}
          />
        </div>
      ) : null}
    </div>
  );
}

export default function MixVideoHistoryPage() {
  const [filter, setFilter] = useState<FilterKey>("all");
  const [query, setQuery] = useState("");

  const tasksQuery = useQuery<MixVideoTask[]>({
    queryKey: ["mixVideoTasks", "history"],
    queryFn: () => listMixVideoTasks({ limit: 50 }),
    refetchInterval: ({ state }) => {
      const items = state.data as MixVideoTask[] | undefined;
      const activeTask = items?.find((task) => !isTerminalMixVideoTask(task));
      return getMixVideoTaskPollInterval(activeTask);
    },
  });

  const tasks = useMemo(() => tasksQuery.data || [], [tasksQuery.data]);
  const normalizedQuery = query.trim().toLowerCase();

  const filteredTasks = useMemo(() => {
    return tasks.filter((task) => {
      if (filter === "running" && !["queued", "running"].includes(task.status)) return false;
      if (filter === "completed" && task.status !== "completed") return false;
      if (filter === "failed" && !["failed", "cancelled"].includes(task.status)) return false;
      if (!normalizedQuery) return true;
      return (
        task.scriptText.toLowerCase().includes(normalizedQuery) ||
        task.id.toLowerCase().includes(normalizedQuery)
      );
    });
  }, [filter, normalizedQuery, tasks]);

  const filterTabs: { key: FilterKey; label: string }[] = [
    { key: "all", label: "全部" },
    { key: "running", label: "进行中" },
    { key: "completed", label: "已完成" },
    { key: "failed", label: "失败 / 取消" },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        title="混剪历史"
        subtitle="查看预估时长、预扣积分、最终结算和成片结果。"
        actions={
          <Link
            href="/creation/mix-video"
            className="btn-neon inline-flex items-center gap-2 rounded-lg border border-border px-3.5 py-2 text-sm text-text-primary"
          >
            <Scissors className="h-4 w-4" />
            新建混剪
          </Link>
        }
      />

      <div className="grid gap-4 md:grid-cols-3">
        <StatCard label="全部任务" value={tasks.length} icon={<LayoutGrid className="h-5 w-5" />} />
        <StatCard
          label="进行中"
          value={tasks.filter((task) => ["queued", "running"].includes(task.status)).length}
          icon={<History className="h-5 w-5" />}
        />
        <StatCard
          label="已完成"
          value={tasks.filter((task) => task.status === "completed").length}
          icon={<Video className="h-5 w-5" />}
        />
      </div>

      <div className="glass-card flex flex-col gap-3 p-4 md:flex-row md:items-center md:justify-between">
        <div className="flex flex-wrap gap-1.5">
          {filterTabs.map((item) => (
            <button
              key={item.key}
              type="button"
              onClick={() => setFilter(item.key)}
              className={`rounded-lg px-3 py-1.5 text-sm transition ${
                filter === item.key
                  ? "bg-accent text-background"
                  : "bg-surface/40 text-text-secondary hover:text-text-primary"
              }`}
            >
              {item.label}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-2 rounded-lg border border-border bg-surface/30 px-3 py-1.5">
          <Search className="h-4 w-4 text-text-muted" />
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="搜索文案或任务 ID"
            className="w-48 bg-transparent text-sm text-text-primary outline-none"
          />
        </div>
      </div>

      {filteredTasks.length > 0 ? (
        <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
          {filteredTasks.map((task) => {
            const running = !isTerminalMixVideoTask(task);
            const failed = ["failed", "cancelled"].includes(task.status);
            return (
              <Link
                key={task.id}
                href={`/creation/mix-video/${task.id}`}
                className="group glass-card overflow-hidden p-0 transition hover:border-accent/40"
              >
                <TaskCardThumbnail task={task} />

                <div className="space-y-3 p-4">
                  <div className="min-h-[2.75rem]">
                    <p className="text-sm font-semibold leading-6 text-text-primary line-clamp-2">
                      {truncateMixVideoText(task.scriptText, 90) || "混剪任务"}
                    </p>
                  </div>

                  {running ? (
                    <div>
                      <div className="flex items-center justify-between text-[11px] text-text-muted">
                        <span className="truncate">{getMixVideoTaskProgressMessage(task)}</span>
                        <span className="ml-2 shrink-0 font-mono font-medium text-accent">
                          {clampMixVideoTaskProgress(task).toFixed(0)}%
                        </span>
                      </div>
                      <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-background/50">
                        <div
                          className="h-full rounded-full bg-accent transition-all duration-300"
                          style={{ width: `${clampMixVideoTaskProgress(task)}%` }}
                        />
                      </div>
                    </div>
                  ) : failed ? (
                    <div className="inline-flex items-center gap-1.5 rounded-md bg-danger/10 px-2 py-1 text-xs text-danger">
                      <XCircle className="h-3.5 w-3.5" />
                      {task.errorMessage ? truncateMixVideoText(task.errorMessage, 30) : "任务失败"}
                    </div>
                  ) : null}

                  <div className="grid grid-cols-2 gap-2 text-xs">
                    <div className="rounded-lg bg-surface/30 px-3 py-2">
                      <div className="inline-flex items-center gap-1 text-[10px] text-text-muted">
                        <Clock3 className="h-3 w-3" />
                        预计时长
                      </div>
                      <div className="mt-0.5 font-medium text-text-primary">
                        {task.estimatedDurationSeconds}
                        <span className="text-[10px] text-text-muted"> 秒</span>
                      </div>
                    </div>
                    <div className="rounded-lg bg-surface/30 px-3 py-2">
                      <div className="inline-flex items-center gap-1 text-[10px] text-text-muted">
                        <Coins className="h-3 w-3" />
                        预估消耗
                      </div>
                      <div className="mt-0.5 font-medium text-text-primary">
                        {formatMixVideoCreditValue(task.estimatedCredits)}
                      </div>
                    </div>
                    <div className="rounded-lg bg-surface/30 px-3 py-2">
                      <div className="text-[10px] text-text-muted">实际时长</div>
                      <div className="mt-0.5 font-medium text-text-primary">
                        {task.actualDurationSeconds != null ? `${task.actualDurationSeconds} 秒` : "—"}
                      </div>
                    </div>
                    <div className="rounded-lg bg-surface/30 px-3 py-2">
                      <div className="text-[10px] text-text-muted">最终结算</div>
                      <div className="mt-0.5 font-medium text-text-primary">
                        {task.finalCredits != null
                          ? `${formatMixVideoCreditValue(task.finalCredits)} 积分`
                          : "—"}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center justify-between text-[11px] text-text-muted">
                    <span>素材 {task.sourceAssets.length} 个</span>
                    <span>{formatDateTime(task.updatedAt)}</span>
                  </div>
                </div>
              </Link>
            );
          })}
        </div>
      ) : !tasksQuery.isLoading ? (
        <div className="glass-card p-10 text-center text-sm text-text-muted">暂无符合条件的混剪任务。</div>
      ) : (
        <div className="glass-card p-10 text-center text-sm text-text-muted">正在加载混剪历史...</div>
      )}
    </div>
  );
}
