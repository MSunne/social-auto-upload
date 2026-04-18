"use client";

import { useParams } from "next/navigation";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, AudioLines, Clock3, Coins, Download, FileText, Film, Gauge, LoaderCircle, Scissors, Video } from "lucide-react";
import { PageHeader, StatusBadge } from "@/components/ui/common";
import { clampMixVideoTaskProgress, formatMixVideoCreditValue, getMixVideoTaskProgressMessage } from "@/lib/mix-video";
import { getMixVideoTaskPollInterval } from "@/lib/long-task-poll";
import { getMixVideoTask } from "@/lib/services";
import type { MixVideoAsset, MixVideoTask } from "@/lib/types";
import { formatDateTime } from "@/lib/workflow";

function AssetItem({ asset }: { asset: MixVideoAsset }) {
  const isCleaned = !asset.publicUrl;
  const isImage = asset.mimeType.startsWith("image/");
  const isVideo = asset.mimeType.startsWith("video/");

  return (
    <div className="glass-card overflow-hidden p-0">
      <div className="relative h-44 bg-black/40">
        {isCleaned ? (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-center text-text-muted">
            <Video className="h-6 w-6" />
            <span className="px-4 text-xs">源素材链接不可用</span>
          </div>
        ) : isImage ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={asset.publicUrl} alt={asset.fileName} className="h-full w-full object-cover" />
        ) : isVideo ? (
          <video src={asset.publicUrl} className="h-full w-full object-cover" muted controls />
        ) : (
          <div className="flex h-full items-center justify-center text-text-muted">
            <Video className="h-6 w-6" />
          </div>
        )}
      </div>
      <div className="px-4 py-3">
        <p className="truncate text-sm font-medium text-text-primary">{asset.fileName}</p>
        <p className="mt-1 text-xs text-text-muted">{asset.mimeType}</p>
      </div>
    </div>
  );
}

export default function MixVideoTaskDetailPage() {
  const params = useParams<{ taskId: string }>();
  const taskId = Array.isArray(params.taskId) ? params.taskId[0] : params.taskId;

  const taskQuery = useQuery<MixVideoTask>({
    queryKey: ["mixVideoTask", taskId],
    queryFn: () => getMixVideoTask(taskId),
    enabled: Boolean(taskId),
    refetchInterval: ({ state }) => {
      const item = state.data as MixVideoTask | undefined;
      return getMixVideoTaskPollInterval(item);
    },
  });

  const task = taskQuery.data;
  const progressValue = clampMixVideoTaskProgress(task);
  const progressMessage = getMixVideoTaskProgressMessage(task);

  return (
    <div className="space-y-6">
      <PageHeader
        title="混剪详情"
        subtitle="查看任务状态、素材清单、文案和最终计费。"
        actions={
          <div className="flex items-center gap-2">
            <Link href="/creation/mix-video/history" className="btn-neon inline-flex items-center gap-2 rounded-lg border border-border px-3.5 py-2 text-sm text-text-primary">
              <ArrowLeft className="h-4 w-4" />
              历史
            </Link>
            <Link href="/creation/mix-video" className="btn-neon inline-flex items-center gap-2 rounded-lg border border-border px-3.5 py-2 text-sm text-text-primary">
              <Scissors className="h-4 w-4" />
              新建
            </Link>
          </div>
        }
      />

      {taskQuery.isLoading ? (
        <div className="glass-card p-8 text-center text-sm text-text-muted">正在读取混剪任务详情...</div>
      ) : !task ? (
        <div className="glass-card p-8 text-center text-sm text-danger">混剪任务不存在或读取失败。</div>
      ) : (
        <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
          <section className="space-y-5">
            <div className="glass-card overflow-hidden p-0">
              {task.resultAsset?.publicUrl ? (
                <>
                  <video src={task.resultAsset.publicUrl} controls className="w-full bg-black" style={{ maxHeight: "520px", objectFit: "contain" }} />
                  <div className="flex items-center justify-between border-t border-border/30 px-5 py-3">
                    <div className="flex items-center gap-2 text-sm font-medium text-text-primary">
                      <Video className="h-4 w-4 text-accent" />
                      成片视频
                    </div>
                    <a href={task.resultAsset.publicUrl} target="_blank" rel="noreferrer" className="inline-flex items-center gap-2 rounded-lg bg-accent/10 px-3 py-2 text-sm text-accent">
                      <Download className="h-4 w-4" />
                      下载
                    </a>
                  </div>
                </>
              ) : (
                <div className="flex h-72 flex-col items-center justify-center gap-3 bg-gradient-to-br from-surface/60 to-black/40 text-sm text-text-muted">
                  <div className="flex h-14 w-14 items-center justify-center rounded-full bg-surface/60">
                    {task.status === "failed" || task.status === "cancelled" ? (
                      <Film className="h-7 w-7" />
                    ) : (
                      <LoaderCircle className="h-7 w-7 animate-spin text-accent" />
                    )}
                  </div>
                  <p>{task.status === "failed" ? "混剪任务失败" : task.status === "cancelled" ? "任务已取消" : "混剪生成中，请稍候"}</p>
                  {task.status !== "failed" && task.status !== "cancelled" ? (
                    <p className="text-xs text-text-muted/80">{progressMessage} · {progressValue.toFixed(0)}%</p>
                  ) : null}
                </div>
              )}
            </div>

            <div className="glass-card p-5">
              <div className="mb-4 flex items-center gap-2 text-base font-semibold text-text-primary">
                <FileText className="h-4 w-4 text-accent" />
                文案
              </div>
              <div className="rounded-lg bg-surface/30 p-4 text-sm leading-7 text-text-secondary whitespace-pre-wrap">{task.scriptText}</div>
            </div>

            <div className="glass-card p-5">
              <div className="mb-4 flex items-center gap-2 text-base font-semibold text-text-primary">
                <Video className="h-4 w-4 text-accent" />
                素材列表
              </div>
              <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                {task.sourceAssets.map((asset, index) => (
                  <AssetItem key={`${asset.fileName}-${index}`} asset={asset} />
                ))}
              </div>
            </div>

            <div className="glass-card p-5">
              <div className="mb-4 flex items-center gap-2 text-base font-semibold text-text-primary">
                <AudioLines className="h-4 w-4 text-accent" />
                参考音频
              </div>
              {task.refAudioAsset.publicUrl ? (
                <audio controls src={task.refAudioAsset.publicUrl} className="w-full" />
              ) : (
                <div className="rounded-lg bg-surface/30 px-4 py-3 text-sm text-text-muted">参考音频链接不可用。</div>
              )}
            </div>
          </section>

          <aside className="space-y-5">
            <div className="glass-card p-5">
              <div className="flex items-center justify-between">
                <h2 className="text-base font-semibold text-text-primary">任务状态</h2>
                <StatusBadge status={task.status} />
              </div>
              <div className="mt-4 space-y-3 text-sm text-text-secondary">
                <div className="rounded-lg border border-accent/15 bg-accent/[0.04] p-4">
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2 text-text-primary">
                      <Gauge className="h-4 w-4 text-accent" />
                      <span>{progressMessage}</span>
                    </div>
                    <span className="font-mono font-medium text-accent">{progressValue.toFixed(0)}%</span>
                  </div>
                  <div className="mt-3 h-2 overflow-hidden rounded-full bg-background/60">
                    <div className="h-full rounded-full bg-accent transition-all duration-300" style={{ width: `${progressValue}%` }} />
                  </div>
                </div>
                <div className="flex items-center justify-between"><span>计费状态</span><span className="font-medium text-text-primary">{task.billingStatus}</span></div>
                <div className="flex items-center justify-between"><span>创建时间</span><span className="font-medium text-text-primary">{formatDateTime(task.createdAt)}</span></div>
                <div className="flex items-center justify-between"><span>更新时间</span><span className="font-medium text-text-primary">{formatDateTime(task.updatedAt)}</span></div>
                {task.errorMessage ? <div className="rounded-lg bg-danger/10 px-3 py-2 text-danger">{task.errorMessage}</div> : null}
              </div>
            </div>

            <div className="glass-card p-5">
              <h2 className="text-base font-semibold text-text-primary">计费信息</h2>
              <div className="mt-4 space-y-3 text-sm">
                <div className="flex items-center justify-between text-text-secondary"><span className="inline-flex items-center gap-2"><Clock3 className="h-4 w-4" />预计时长</span><span className="font-medium text-text-primary">{task.estimatedDurationSeconds} 秒</span></div>
                <div className="flex items-center justify-between text-text-secondary"><span className="inline-flex items-center gap-2"><Coins className="h-4 w-4" />预计消耗</span><span className="font-medium text-text-primary">{formatMixVideoCreditValue(task.estimatedCredits)} 积分</span></div>
                <div className="flex items-center justify-between text-text-secondary"><span>实际时长</span><span className="font-medium text-text-primary">{task.actualDurationSeconds != null ? `${task.actualDurationSeconds} 秒` : "—"}</span></div>
                <div className="flex items-center justify-between text-text-secondary"><span>最终结算</span><span className="font-medium text-text-primary">{task.finalCredits != null ? `${formatMixVideoCreditValue(task.finalCredits)} 积分` : "—"}</span></div>
              </div>
            </div>
          </aside>
        </div>
      )}
    </div>
  );
}
